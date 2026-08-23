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
