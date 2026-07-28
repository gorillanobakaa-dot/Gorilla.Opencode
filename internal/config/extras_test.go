package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The one setting that spends must default to off, and the free ones must default
// on. A fresh install must not be paying for output nobody asked for, and must not
// be hiding information that costs nothing to show.
func TestExtraDefaultsMatchTheirCost(t *testing.T) {
	paid := 0
	for _, e := range Extras {
		switch e.Cost {
		case CostGeneration:
			paid++
			if e.Default {
				t.Errorf("%s costs extra but defaults ON — a fresh install would spend before anyone was asked", e.ID)
			}
		case CostFree:
			if !e.Default {
				t.Errorf("%s is free but defaults OFF — that hides information at no saving", e.ID)
			}
		}
	}
	if paid != 1 {
		t.Errorf("%d extras are marked as costing something; exactly one (reasoning generation) actually does. "+
			"Marking a free one as costly would push users to disable it for an imaginary saving", paid)
	}
}

// Absent must mean "the registry default", NOT the loadout convention of
// absent-means-enabled. That rule would silently switch on the only setting here
// that spends money.
func TestAbsentSettingFallsBackToTheDefaultNotToEnabled(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prev := cfg.Extras
	t.Cleanup(func() { cfg.Extras = prev })
	cfg.Extras = nil // nothing ever chosen

	if ExtraEnabled("extras-reasoning-generate") {
		t.Error("with no stored choice, the paid extra reads as ON — absent must mean the default, and its default is off")
	}
	if !ExtraEnabled("extras-timestamps-show") {
		t.Error("with no stored choice, a free extra reads as OFF despite defaulting on")
	}
}

// An explicit "off" has to survive a round trip through the file. This is the
// omitempty trap that has bitten this config twice: a bool field with omitempty
// drops a false and reads back as the default, so "I turned this off" becomes "I
// never chose". A false VALUE inside a present map is preserved, which is why
// Extras is a map.
func TestAnExplicitOffSurvivesBeingWrittenAndReadBack(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prev := cfg.Extras
	t.Cleanup(func() { cfg.Extras = prev })

	// Turn a default-ON extra explicitly off.
	if err := SetExtra("extras-timestamps-show", false); err != nil {
		t.Fatalf("SetExtra: %v", err)
	}

	raw, err := os.ReadFile(GorillaConfigFile())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var onDisk Config
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("parse config: %v", err)
	}

	v, present := onDisk.Extras["extras-timestamps-show"]
	if !present {
		t.Fatalf("the explicit off was dropped from the file — it would read back as ON next launch:\n%s", raw)
	}
	if v {
		t.Error("the stored value is true; the choice was inverted")
	}
}

// Turning the paid one on must also persist, since it has billing consequences and
// a setting that silently reverts is worse than one that does not exist.
func TestEnablingThePaidExtraPersists(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prev := cfg.Extras
	t.Cleanup(func() { cfg.Extras = prev })

	if err := SetExtra("extras-reasoning-generate", true); err != nil {
		t.Fatalf("SetExtra: %v", err)
	}
	if !ExtraEnabled("extras-reasoning-generate") {
		t.Error("the live config does not reflect the change")
	}

	raw, _ := os.ReadFile(GorillaConfigFile())
	var onDisk Config
	json.Unmarshal(raw, &onDisk)
	if !onDisk.Extras["extras-reasoning-generate"] {
		t.Errorf("the choice did not reach the file:\n%s", raw)
	}
}

func TestSetExtraRejectsUnknownIDs(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SetExtra("extra.does.not.exist", true); err == nil {
		t.Error("an unknown extra was accepted, so a typo would persist a setting nothing reads")
	}
}

// The explanation must describe real resource use and must never invent a price.
// We hold zero pricing for local models, and a NIM free tier or local Ollama bills
// no money — a warning that is false on the user's own machine trains them to
// ignore the ones that are true.
func TestCostExplanationIsHonestAboutHavingNoPrice(t *testing.T) {
	paid := ExtraCostExplanation(CostGeneration)

	for _, forbidden := range []string{"$", "USD", "€", "cents"} {
		if strings.Contains(paid, forbidden) {
			t.Errorf("the explanation quotes a currency (%q) we have no data for", forbidden)
		}
	}
	for _, want := range []string{"allowance", "CPU", "longer", "No price is shown"} {
		if !strings.Contains(paid, want) {
			t.Errorf("the explanation omits %q, so the real cost is not conveyed", want)
		}
	}

	free := ExtraCostExplanation(CostFree)
	if !strings.Contains(free, "saves you nothing") {
		t.Error("the free explanation does not say that hiding it saves nothing — the user would switch it off expecting a saving")
	}
}

// Every extra must appear in /settings, generated from the registry rather than
// hand-copied, or the three surfaces drift and one ends up stating something we
// have measured to be false.
func TestEveryExtraHasAGeneratedSettingsRow(t *testing.T) {
	byID := map[string]Setting{}
	for _, s := range Settings {
		byID[s.ID] = s
	}

	for _, e := range Extras {
		s, ok := byID[e.ID]
		if !ok {
			t.Errorf("%s has no /settings row, so it is invisible there", e.ID)
			continue
		}
		if s.Group != GroupExtras {
			t.Errorf("%s is in group %q, not the extras group", e.ID, s.Group)
		}
		if s.Kind != KindBool {
			t.Errorf("%s is not a bool row", e.ID)
		}
		if s.WhenOn == "" || s.WhenOff == "" {
			t.Errorf("%s does not explain both directions", e.ID)
		}
		if s.Get == nil || s.Set == nil {
			t.Fatalf("%s row is not wired to the registry", e.ID)
		}

		// The costly row must say so; the free rows must say the opposite, in the
		// place where the decision is actually made.
		if e.Cost == CostGeneration {
			if !strings.Contains(s.WhenOn, "more") {
				t.Errorf("%s does not warn that it generates more: %q", e.ID, s.WhenOn)
			}
		} else if !strings.Contains(s.WhenOff, "NOTHING") {
			t.Errorf("%s does not state that hiding it saves nothing: %q", e.ID, s.WhenOff)
		}
	}
}

// The generated row must actually drive the setting, not just describe it.
func TestSettingsRowRoundTripsThroughTheRegistry(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prev := cfg.Extras
	t.Cleanup(func() { cfg.Extras = prev })

	var row *Setting
	for i := range Settings {
		if Settings[i].ID == "extras-reasoning-generate" {
			row = &Settings[i]
			break
		}
	}
	if row == nil {
		t.Fatal("no settings row for the paid extra")
	}

	if err := row.Set(true); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := row.Get(); got != true {
		t.Errorf("Get returned %v after setting true", got)
	}
	if !ExtraEnabled("extras-reasoning-generate") {
		t.Error("the /settings row did not change the underlying setting")
	}
}

// The summary must call out the paid one being on, since that is the state with
// consequences.
func TestSummaryFlagsWhenTheExpensiveOneIsOn(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prev := cfg.Extras
	t.Cleanup(func() { cfg.Extras = prev })

	SetExtra("extras-reasoning-generate", false)
	if strings.Contains(ExtrasSummary(), "extra tokens") {
		t.Error("the summary warns about token use while the paid extra is off")
	}
	SetExtra("extras-reasoning-generate", true)
	if !strings.Contains(ExtrasSummary(), "extra tokens") {
		t.Errorf("the summary does not mention that thinking is on and costing: %q", ExtrasSummary())
	}
}

// Mouse reporting must default OFF. Asking the terminal for mouse events takes
// drag-to-select away from the user — in most terminals you then need Shift — and the
// only modes available report one event per cell crossed, which is what flooded the
// event loop badly enough to leak raw escape codes into the editor.
func TestMouseReportingDefaultsOff(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prev := cfg.MouseWheel
	t.Cleanup(func() { cfg.MouseWheel = prev })
	cfg.MouseWheel = false

	if MouseWheelEnabled() {
		t.Error("mouse reporting is on by default — text selection would be broken out of the box")
	}

	var row *Setting
	for i := range Settings {
		if Settings[i].ID == "mouseWheel" {
			row = &Settings[i]
		}
	}
	if row == nil {
		t.Fatal("no /settings row for mouse reporting, so the trade-off is invisible")
	}
	if row.Default != false {
		t.Error("the settings row advertises a default of on")
	}
	if !row.Restart {
		t.Error("not marked Restart — mouse mode is requested once at startup, so a change cannot take effect mid-session")
	}
	// The row must name the cost, not just the feature.
	if !strings.Contains(row.WhenOn, "Shift") {
		t.Errorf("WhenOn does not mention needing Shift to select: %q", row.WhenOn)
	}
	if !strings.Contains(row.WhenOff, "PageUp") {
		t.Errorf("WhenOff does not say how to scroll instead: %q", row.WhenOff)
	}
}

func TestMouseReportingPersists(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prev := cfg.MouseWheel
	t.Cleanup(func() { cfg.MouseWheel = prev })

	if err := SetMouseWheel(true); err != nil {
		t.Fatalf("SetMouseWheel: %v", err)
	}
	raw, _ := os.ReadFile(GorillaConfigFile())
	var onDisk Config
	json.Unmarshal(raw, &onDisk)
	if !onDisk.MouseWheel {
		t.Errorf("the choice did not reach the file:\n%s", raw)
	}
}
