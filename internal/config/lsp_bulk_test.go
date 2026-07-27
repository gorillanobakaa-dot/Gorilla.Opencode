package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// registerTestLSPs gives the test a hermetic loadout: ONLY the named language
// servers, with both the component registry and the loadout state restored
// afterwards.
//
// It replaces LoadoutComponents rather than appending, and that matters. Load()
// auto-registers every language server it finds configured — clangd among them —
// so appending leaves those rows in the registry, SetAllLSPs correctly switches
// them off too, and the write lands in process-global state that later tests
// inherit. That is precisely how this file broke TestLSPEnabledHonoursBothSwitches
// and TestEnabledLSPNamesIsSortedAndFiltered while passing in isolation.
func registerTestLSPs(t *testing.T, names ...string) {
	t.Helper()

	initLoadout()

	prevComponents := LoadoutComponents
	loadoutMu.RLock()
	prevState := make(map[string]bool, len(loadoutState))
	for k, v := range loadoutState {
		prevState[k] = v
	}
	loadoutMu.RUnlock()

	t.Cleanup(func() {
		LoadoutComponents = prevComponents
		loadoutMu.Lock()
		loadoutState = prevState
		loadoutMu.Unlock()
	})

	// Keep only non-LSP rows, then add exactly the ones this test asked for.
	kept := make([]LoadoutComponent, 0, len(prevComponents))
	for _, c := range prevComponents {
		if !strings.HasPrefix(c.ID, "lsp.") {
			kept = append(kept, c)
		}
	}
	LoadoutComponents = kept

	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = false // map value is "disabled"
	}
	RegisterLSPComponents(m)

	// Start from a known state: every requested server on, and no stale entry
	// for one of these ids left over from another test.
	loadoutMu.Lock()
	for _, n := range names {
		loadoutState[LSPComponentID(n)] = true
	}
	loadoutMu.Unlock()
}

// With nine servers configured, a quiet session took nine separate toggles.
// The granular control the user asked for is only usable with a bulk switch
// beside it.
func TestSetAllLSPs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	registerTestLSPs(t, "c", "cpp", "go", "rust")

	on, off := LSPLoadoutCounts()
	if on != 4 || off != 0 {
		t.Fatalf("setup: on=%d off=%d, want 4/0", on, off)
	}

	if n := SetAllLSPs(false); n != 4 {
		t.Errorf("SetAllLSPs(false) changed %d rows, want 4", n)
	}
	for _, name := range []string{"c", "cpp", "go", "rust"} {
		if LSPEnabled(name) {
			t.Errorf("%s still enabled after switching all off", name)
		}
	}
	if on, off = LSPLoadoutCounts(); on != 0 || off != 4 {
		t.Errorf("counts after all-off: on=%d off=%d, want 0/4", on, off)
	}

	// Idempotent: nothing left to change, so nothing is reported as changed.
	if n := SetAllLSPs(false); n != 0 {
		t.Errorf("second SetAllLSPs(false) reported %d changes, want 0", n)
	}

	if n := SetAllLSPs(true); n != 4 {
		t.Errorf("SetAllLSPs(true) changed %d rows, want 4", n)
	}
	for _, name := range []string{"c", "cpp", "go", "rust"} {
		if !LSPEnabled(name) {
			t.Errorf("%s still disabled after switching all on", name)
		}
	}
}

// The bulk switch must not be a hidden reset. "No language servers" is a normal
// working mode; "no tools" is not, and losing the tool loadout to an LSP key
// would be a nasty surprise.
func TestSetAllLSPsLeavesOtherComponentsAlone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	registerTestLSPs(t, "c", "go")

	// Pick a non-LSP component and flip it away from its default.
	var victim string
	var want bool
	for _, c := range LoadoutComponents {
		if !strings.HasPrefix(c.ID, "lsp.") && !c.Critical {
			victim = c.ID
			want = !LoadoutEnabled(c.ID)
			break
		}
	}
	if victim == "" {
		t.Skip("no non-LSP, non-critical component registered")
	}
	ToggleLoadout(victim)
	if LoadoutEnabled(victim) != want {
		t.Fatalf("setup: could not flip %s", victim)
	}

	SetAllLSPs(false)

	if got := LoadoutEnabled(victim); got != want {
		t.Errorf("%s changed from %v to %v — the bulk LSP switch touched an unrelated component", victim, want, got)
	}
}

// The change has to survive a restart, or the switch is theatre — the failure
// the user suspected: toggles that appear to work and are gone next launch.
//
// Asserted by reading the file rather than by resetting the in-memory state.
// loadoutOnce/loadoutState are process-global, and clearing them mid-suite made
// later tests re-initialise against a different XDG_CONFIG_HOME, which broke two
// unrelated LSP tests. What actually needs proving is that the choice reached
// disk, and the file says that directly.
func TestSetAllLSPsPersists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	registerTestLSPs(t, "c", "go", "rust")
	SetAllLSPs(false)

	data, err := os.ReadFile(loadoutPath())
	if err != nil {
		t.Fatalf("loadout file was never written: %v", err)
	}
	var saved map[string]bool
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("loadout file is not valid JSON: %v", err)
	}
	for _, name := range []string{"c", "go", "rust"} {
		id := LSPComponentID(name)
		v, ok := saved[id]
		if !ok {
			t.Errorf("%s is absent from the saved loadout, so a restart would read it as enabled", id)
			continue
		}
		if v {
			t.Errorf("%s was saved as enabled after switching all off", id)
		}
	}
}

// A component with no saved entry must turn OFF on the first press.
//
// It did not. LoadoutEnabled reports an absent key as enabled, but ToggleLoadout
// read the raw map — zero value false — and flipped it to true, so the first
// press moved it from "on" to on. Every language-server row is in exactly this
// state, because those components are registered from the config at Load time,
// after the loadout state has been read. One press, nothing happens: precisely
// the "I disabled them and they are still there" report.
func TestToggleTurnsOffAComponentWithNoSavedEntry(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	registerTestLSPs(t, "zetalang")

	id := LSPComponentID("zetalang")

	// Remove any entry, so the id is genuinely absent — a freshly registered row.
	loadoutMu.Lock()
	delete(loadoutState, id)
	loadoutMu.Unlock()

	if !LoadoutEnabled(id) {
		t.Fatalf("setup: an absent id should read as enabled")
	}

	ToggleLoadout(id)
	if LoadoutEnabled(id) {
		t.Error("still enabled after one press — the toggle flipped the zero value instead of the reported value")
	}
	if LSPEnabled("zetalang") {
		t.Error("LSPEnabled still true, so the server would start anyway")
	}

	ToggleLoadout(id)
	if !LoadoutEnabled(id) {
		t.Error("a second press did not turn it back on")
	}
}
