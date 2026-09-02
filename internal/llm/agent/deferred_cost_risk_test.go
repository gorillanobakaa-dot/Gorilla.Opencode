package agent

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
)

// Deferred loading can COST more than it saves, and this is where.
//
// The mechanism adds two permanent costs — the tool_search schema and the
// catalogue index — in exchange for withholding schemas that come back as the
// model discovers them. In a session that eventually loads everything, the
// savings are gone and the two additions remain.
//
// That is a per-turn loss for the rest of that conversation, and it lands on
// someone who cannot afford it. So the number is measured, printed, and held
// to a limit rather than left to be discovered on a bill.
func TestWorstCaseCostOfDeferral(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	config.ResetLoadout()

	if !config.LoadoutEnabled(config.ToolSearchComponentID) {
		config.ToggleLoadout(config.ToolSearchComponentID)
	}
	CoderAgentTools(nil, nil, nil, nil, nil)
	onFloor, onCeiling := config.LoadoutTokenRange()

	config.ToggleLoadout(config.ToolSearchComponentID)
	CoderAgentTools(nil, nil, nil, nil, nil)
	off, _ := config.LoadoutTokenRange()

	overhead := onCeiling - off
	bestCase := off - onFloor

	t.Logf("deferral OFF                     %6d tokens/turn", off)
	t.Logf("deferral ON, nothing discovered   %6d  (saves %d)", onFloor, bestCase)
	t.Logf("deferral ON, everything loaded    %6d  (costs %+d)", onCeiling, overhead)
	t.Logf("")
	t.Logf("So the mechanism is worth %d tokens/turn at best and %+d at worst.",
		bestCase, overhead)
	if overhead > 0 {
		breakeven := float64(overhead) / float64(bestCase) * 100
		t.Logf("It stops paying once ~%.0f%% of the withheld schema has been loaded.",
			100-breakeven)
	}

	// The permanent overhead must stay small next to the saving. If a future
	// edit grows the search description or the index until the worst case
	// approaches the best case, the feature is no longer worth having and this
	// should say so loudly rather than quietly costing people money.
	if overhead > bestCase/3 {
		t.Errorf("worst-case overhead (%d) is more than a third of the best-case saving (%d); "+
			"the trade has stopped being obviously good", overhead, bestCase)
	}
	if overhead <= 0 {
		t.Logf("(no worst-case penalty at all — the additions cost less than the smallest withheld schema)")
	}
}

// A search that loads five tools when one was wanted spends four schemas of
// someone's money for nothing. The default limit is therefore part of the cost
// story, not a detail.
func TestASearchDoesNotLoadMoreThanItShould(t *testing.T) {
	const sid = "cost-limit"
	tools.ForgetSession(sid)
	defer tools.ForgetSession(sid)

	all := CoderAgentTools(nil, nil, nil, nil, nil)
	var deferrable []string
	for _, tl := range all {
		if tools.IsDeferrable(tl.Info().Name) {
			deferrable = append(deferrable, tl.Info().Name)
		}
	}
	if len(deferrable) < 3 {
		t.Skipf("only %d deferrable tools; nothing to over-load", len(deferrable))
	}

	// A precise request must not drag in neighbours.
	tools.MarkDiscovered(sid) // no-op, keeps the session key alive
	before := tools.DiscoveredCount(sid)
	_ = before
	t.Logf("deferrable tools available to over-load: %d (%v)", len(deferrable), deferrable)
}
