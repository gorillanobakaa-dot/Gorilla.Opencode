package config

// GORILLA OVERRIDE (2026-08-23): a new tool must be DECIDED about for the
// low-bandwidth preset, not merely forgotten.
//
// lowBandwidthOff was last edited on 2026-08-14. tool.review landed on
// 2026-08-18, default ON, 759 measured tokens, and nobody came back to this
// list. So the preset built for somebody on a satellite link kept shipping a
// 30-analyser static-review schema on every single turn, while its own docstring
// said it exists to drop whatever is "not required for core edit/build loops".
//
// Nothing failed. The preset still ran, still switched things off, still
// reported a smaller total. It was simply leaving 759 tokens on the table, and
// only counting the schemas directly (schema_cost_test.go, the same day) made it
// visible. That is the same shape as the Arch package vanishing from four
// consecutive releases: a new thing lands, an existing list is not updated, and
// every check that exists still passes.
//
// The fix is not "remember harder". It is to make silence impossible: a
// default-ON, non-critical component must appear in lowBandwidthOff (dropped) or
// in lowBandwidthKeep (kept, with a written reason). Absence from both is now a
// failing test rather than an invisible omission.

import (
	"strings"
	"testing"
)

// TestEveryOptionalComponentHasALowBandwidthDecision is the guard.
func TestEveryOptionalComponentHasALowBandwidthDecision(t *testing.T) {
	var undecided []string
	for _, c := range LoadoutComponents {
		// Per-server LSP rows are generated from the user's config, not shipped.
		if strings.HasPrefix(c.ID, "lsp.") {
			continue
		}
		// Critical components are never dropped, by design: the preset trims
		// cost, it does not lobotomise the agent.
		if c.Critical {
			if lowBandwidthOff[c.ID] {
				t.Errorf("%s is marked Critical but the low-bandwidth preset drops it. "+
					"The preset saves tokens; it must not remove core capability.", c.ID)
			}
			continue
		}
		// A component that ships OFF costs nothing by default, so the preset has
		// nothing to decide about it.
		if !c.Default {
			continue
		}
		if lowBandwidthOff[c.ID] {
			continue
		}
		if reason := lowBandwidthKeep[c.ID]; strings.TrimSpace(reason) != "" {
			continue
		}
		undecided = append(undecided, c.ID)
	}

	if len(undecided) > 0 {
		t.Errorf("these ship ON by default, are not critical, and the low-bandwidth "+
			"preset has no recorded decision about them:\n  %s\n\n"+
			"  Add each to lowBandwidthOff (dropped on a metered link) or to\n"+
			"  lowBandwidthKeep with a reason (kept on purpose). Absence from both is\n"+
			"  indistinguishable from nobody having looked, which is how tool.review\n"+
			"  sat unconsidered for four days at 759 tokens per turn.",
			strings.Join(undecided, "\n  "))
	}
}

// A reason has to be a reason. "yes" or "TODO" would satisfy a non-empty check
// and defeat the point of the map, which is to record WHY.
func TestEveryLowBandwidthKeepGivesARealReason(t *testing.T) {
	for id, reason := range lowBandwidthKeep {
		if len(strings.Fields(reason)) < 8 {
			t.Errorf("lowBandwidthKeep[%q] is %d words. It is there to record why a "+
				"component earns its place on a metered link, so it needs an argument, "+
				"not a label.", id, len(strings.Fields(reason)))
		}
	}
	// A component cannot be in both halves of the decision.
	for id := range lowBandwidthKeep {
		if lowBandwidthOff[id] {
			t.Errorf("%s is in BOTH lowBandwidthOff and lowBandwidthKeep", id)
		}
	}
}

// Every id in either map must actually exist. A typo would silently do nothing,
// which is the failure mode this whole file is about.
func TestLowBandwidthListsNameRealComponents(t *testing.T) {
	known := map[string]bool{}
	for _, c := range LoadoutComponents {
		known[c.ID] = true
	}
	for id := range lowBandwidthOff {
		if !known[id] {
			t.Errorf("lowBandwidthOff names %q, which is not a registered component. "+
				"A typo here switches nothing off and reports success.", id)
		}
	}
	for id := range lowBandwidthKeep {
		if !known[id] {
			t.Errorf("lowBandwidthKeep names %q, which is not a registered component", id)
		}
	}
}

// The preset must actually reduce the bill, and it must leave the agent able to
// read, write and run things. Both halves, because either alone is satisfiable
// by a preset that is wrong.
func TestLowBandwidthPresetCutsCostWithoutRemovingCoreCapability(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	before := LoadoutActiveTokens()
	after := ApplyLowBandwidthLoadout()

	if after >= before {
		t.Errorf("the low-bandwidth preset did not reduce the per-turn cost: %d -> %d", before, after)
	}
	t.Logf("per-turn active tokens: %d -> %d (saves %d, %.0f%%)",
		before, after, before-after, float64(before-after)/float64(before)*100)

	for _, c := range LoadoutComponents {
		if c.Critical && !LoadoutEnabled(c.ID) {
			t.Errorf("%s is Critical and the preset switched it off; someone on a "+
				"satellite link still needs to edit and build", c.ID)
		}
	}
}

// THE GUARANTEE. Pressing the low-bandwidth key must never raise the bill.
//
// It used to reset every component not on the drop-list back to its shipped
// default, so on a hand-trimmed loadout it switched things back ON. Measured
// live by the owner on his own setup, 2026-08-23: ~8,802 to ~8,138, only 8%,
// because the web tools went but bash, the edit tool, environment info and four
// language servers all returned. He had switched those off on purpose. A button
// that undoes your economies and then reports a smaller number is worse than no
// button, especially on the connection it exists for.
//
// The starting state here is deliberately awkward: several default-ON components
// switched OFF by hand, which is exactly the shape that exposed the bug.
func TestLowBandwidthOnlyEverSubtracts(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Trim by hand first, the way a user on a slow link would.
	var trimmed []string
	for _, c := range LoadoutComponents {
		if c.Default && !lowBandwidthOff[c.ID] && !strings.HasPrefix(c.ID, "lsp.") {
			if LoadoutEnabled(c.ID) {
				ToggleLoadout(c.ID)
			}
			trimmed = append(trimmed, c.ID)
		}
	}
	if len(trimmed) == 0 {
		t.Skip("no default-ON components outside the drop-list to trim")
	}
	before := LoadoutActiveTokens()
	after := ApplyLowBandwidthLoadout()

	if after > before {
		t.Errorf("the low-bandwidth key RAISED the per-turn cost: %d -> %d.\n"+
			"  It must only ever subtract. Resetting non-listed rows to their\n"+
			"  defaults is what made it switch a hand-trimmed loadout back on.",
			before, after)
	}
	// Nothing the user switched off may come back.
	for _, id := range trimmed {
		if LoadoutEnabled(id) {
			t.Errorf("%s was switched off by hand and the low-bandwidth key turned it "+
				"back ON. A user pressing this wants less, not their choices undone.", id)
		}
	}
	// And it must still do its job: everything on the drop-list is off.
	for id := range lowBandwidthOff {
		if LoadoutEnabled(id) {
			t.Errorf("%s is on the drop-list and survived the preset", id)
		}
	}
}

// THE BASE PROMPT MUST NOT BE COUNTED TWICE. Second instance of one trap.
//
// LoadoutActiveTokens() opens with `total := basePromptTokens`, so it ALREADY
// includes the system prompt. Adding LoadoutBaseTokens() on top counts it again.
//
// Found and fixed on 2026-08-14 in ResearchBasisTokens, where it inflated every
// dollar figure on the /research screen by 3,000 tokens (28%) and made a line
// labelled "MEASURED" disagree with what /context printed for the same quantity.
// A warning comment was left at loadout.go:938 saying exactly this.
//
// Repeated on 2026-08-23, in the analysis written to CORRECT a different bad
// number, and published: the release notes quoted a 12,174-token default and a
// 37% saving, when the true figures are 10,386 and 43%. The absolute saving
// (4,465) was right both times, which is why nothing looked wrong.
//
// The comment did not prevent it, so this is a test instead.
func TestBasePromptIsNotCountedTwice(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	SetBasePromptTokens(1000)

	active := LoadoutActiveTokens()
	sum := 0
	for _, c := range LoadoutComponents {
		if LoadoutEnabled(c.ID) {
			sum += ComponentTokens(c)
		}
	}
	if want := sum + LoadoutBaseTokens(); active != want {
		t.Fatalf("LoadoutActiveTokens()=%d, want components(%d) + base(%d) = %d",
			active, sum, LoadoutBaseTokens(), want)
	}
	// The property that matters to a caller: the base is in there ONCE, so
	// anyone writing ActiveTokens()+BaseTokens() is double counting.
	SetBasePromptTokens(2000)
	if grew := LoadoutActiveTokens() - active; grew != 1000 {
		t.Errorf("raising the base prompt by 1000 moved the active total by %d; "+
			"it must move by exactly 1000, or the base is counted a number of "+
			"times other than once", grew)
	}
}
