package config

import (
	"slices"
	"testing"
)

// Each configured language server must get its own /context row, so clangd can
// be turned off on a Go-only day without also losing gopls. Before this the
// only control was the single bulk "LSP info" row.
func TestRegisterLSPComponentsAddsOneRowPerServer(t *testing.T) {
	orig := LoadoutComponents
	t.Cleanup(func() { LoadoutComponents = orig })

	RegisterLSPComponents(map[string]bool{
		"clangd":        false, // enabled in config
		"gopls":         false,
		"rust-analyzer": true, // disabled in config
	})

	for _, name := range []string{"clangd", "gopls", "rust-analyzer"} {
		id := LSPComponentID(name)
		idx := slices.IndexFunc(LoadoutComponents, func(c LoadoutComponent) bool { return c.ID == id })
		if idx < 0 {
			t.Fatalf("no loadout row for %q (id %q)", name, id)
		}
		c := LoadoutComponents[idx]
		if c.Name == "" || c.Tradeoff == "" {
			t.Errorf("row %q has an empty Name or Tradeoff — the /context menu would render a blank line", id)
		}
		if c.Critical {
			t.Errorf("row %q is marked Critical; no language server is load-bearing enough to warrant the warning marker", id)
		}
	}

	// A server disabled in config.json must start as an OFF row rather than
	// silently re-enabling itself through the loadout default.
	idx := slices.IndexFunc(LoadoutComponents, func(c LoadoutComponent) bool {
		return c.ID == LSPComponentID("rust-analyzer")
	})
	if LoadoutComponents[idx].Default {
		t.Error("a server disabled in config.json defaulted to ON in the loadout — the config setting was overridden")
	}
}

// Called twice (config reload, or Load running again in a test binary) it must
// not duplicate rows — the /context menu would show clangd twice.
func TestRegisterLSPComponentsIsIdempotent(t *testing.T) {
	orig := LoadoutComponents
	t.Cleanup(func() { LoadoutComponents = orig })

	in := map[string]bool{"clangd": false, "gopls": false}
	RegisterLSPComponents(in)
	after := len(LoadoutComponents)
	RegisterLSPComponents(in)
	if len(LoadoutComponents) != after {
		t.Errorf("second call added %d duplicate rows", len(LoadoutComponents)-after)
	}
}

// LSPEnabled ANDs the config flag with the loadout toggle. Either switch alone
// must be able to turn a server off; neither may override the other.
func TestLSPEnabledHonoursBothSwitches(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := Get()
	prevLSP, prevComponents := c.LSP, LoadoutComponents
	t.Cleanup(func() { c.LSP, LoadoutComponents = prevLSP, prevComponents })

	c.LSP = map[string]LSPConfig{
		"clangd":   {Command: "clangd"},
		"disabled": {Command: "x", Disabled: true},
	}
	RegisterLSPComponents(map[string]bool{"clangd": false, "disabled": true})

	if !LSPEnabled("clangd") {
		t.Error("clangd enabled in config and loadout should be enabled")
	}
	if LSPEnabled("disabled") {
		t.Error("a server Disabled in config must stay off regardless of the loadout row")
	}

	// Now turn clangd off via the loadout only; config still says enabled.
	if LoadoutEnabled(LSPComponentID("clangd")) {
		ToggleLoadout(LSPComponentID("clangd"))
	}
	if LSPEnabled("clangd") {
		t.Error("clangd disabled via the /context loadout should be off even though config allows it")
	}
	// Restore for other tests in this binary.
	ToggleLoadout(LSPComponentID("clangd"))

	// An unconfigured name is enabled by default, matching LoadoutEnabled's
	// unknown-id rule — a newly added server is never silently dropped.
	if !LSPEnabled("never-configured") {
		t.Error("an unknown server should default to enabled")
	}
}

// EnabledLSPNames feeds the prompt's LSP block. It must list only running
// servers, sorted, so the block is deterministic across runs.
func TestEnabledLSPNamesIsSortedAndFiltered(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := Get()
	prevLSP, prevComponents := c.LSP, LoadoutComponents
	t.Cleanup(func() { c.LSP, LoadoutComponents = prevLSP, prevComponents })

	c.LSP = map[string]LSPConfig{
		"rust-analyzer": {Command: "rust-analyzer"},
		"clangd":        {Command: "clangd"},
		"off-in-config": {Command: "x", Disabled: true},
	}
	RegisterLSPComponents(map[string]bool{
		"rust-analyzer": false, "clangd": false, "off-in-config": true,
	})

	got := EnabledLSPNames()
	want := []string{"clangd", "rust-analyzer"}
	if !slices.Equal(got, want) {
		t.Errorf("EnabledLSPNames() = %v, want %v (sorted, config-disabled excluded)", got, want)
	}
}
