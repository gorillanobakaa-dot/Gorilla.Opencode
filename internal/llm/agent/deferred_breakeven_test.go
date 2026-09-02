package agent

import (
	"sort"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
)

// Where deferral stops paying, counted in TOOLS rather than percentages.
//
// "It stops paying once 83% of the withheld schema is loaded" is true and
// useless to someone deciding whether to switch it on. What they can act on is
// "after the Nth tool you are losing money", so that is what this prints.
//
// It matters because the loss is permanent for the rest of that conversation
// and invisible while it happens. Somebody on a metered connection or a free
// tier's daily quota does not get told; they just run out sooner.
func TestHowManyDiscoveriesBeforeDeferralStopsPaying(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	config.ResetLoadout()

	if !config.LoadoutEnabled(config.ToolSearchComponentID) {
		config.ToggleLoadout(config.ToolSearchComponentID)
	}
	all := CoderAgentTools(nil, nil, nil, nil, nil)
	floor, _ := config.LoadoutTokenRange()

	config.ToggleLoadout(config.ToolSearchComponentID)
	CoderAgentTools(nil, nil, nil, nil, nil)
	off, _ := config.LoadoutTokenRange()

	// Cost of each deferrable tool, dearest first: a model looking for a
	// capability tends to reach for the big specialised ones, so this is the
	// pessimistic order and the honest one to quote.
	type tc struct {
		name string
		cost int
	}
	var deferred []tc
	for _, tl := range all {
		if tools.IsDeferrable(tl.Info().Name) {
			deferred = append(deferred, tc{tl.Info().Name, toolTokens(tl)})
		}
	}
	sort.Slice(deferred, func(i, j int) bool { return deferred[i].cost > deferred[j].cost })

	t.Logf("deferral OFF costs %d tokens/turn. ON starts at %d.", off, floor)
	t.Logf("")
	t.Logf("%-4s %-14s %6s   %8s   %s", "n", "discovered", "cost", "per turn", "vs OFF")

	running := floor
	flipped := 0
	for i, d := range deferred {
		running += d.cost
		delta := running - off
		verdict := "saving"
		if delta > 0 {
			verdict = "LOSING"
			if flipped == 0 {
				flipped = i + 1
			}
		}
		t.Logf("%-4d %-14s %6d   %8d   %+6d  %s", i+1, d.name, d.cost, running, delta, verdict)
	}

	t.Logf("")
	if flipped == 0 {
		t.Logf("VERDICT: deferral saves tokens even with every tool loaded.")
	} else {
		t.Logf("VERDICT: deferral saves money until the %d%s tool is discovered.", flipped, ordinal(flipped))
		t.Logf("From then on it costs more than having had every schema from the start.")
		t.Logf("There are %d deferrable tools, so the losing zone is the last %d.",
			len(deferred), len(deferred)-flipped+1)
	}

	// A mechanism that flips to a loss after one or two discoveries would be
	// worse than useless -- most real sessions reach that. Fail loudly if a
	// future edit takes it there.
	if flipped > 0 && flipped <= 2 {
		t.Errorf("deferral starts LOSING after only %d discovered tool(s); "+
			"a normal session would reach that, so this costs users money", flipped)
	}
}

func ordinal(n int) string {
	switch {
	case n%100 >= 11 && n%100 <= 13:
		return "th"
	case n%10 == 1:
		return "st"
	case n%10 == 2:
		return "nd"
	case n%10 == 3:
		return "rd"
	}
	return "th"
}
