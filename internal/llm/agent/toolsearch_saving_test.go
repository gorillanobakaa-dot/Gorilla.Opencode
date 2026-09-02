package agent

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
)

// What deferral actually saves, measured rather than asserted.
//
// The whole feature is a trade: fewer tokens per turn, paid for with an extra
// round trip whenever the model needs something it has not loaded. If the
// saving were small the trade would be bad, so it is worth keeping a number
// here that fails loudly if it stops being true.
func TestDeferredLoadingSavesWhatItClaims(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	config.ResetLoadout()
	for _, c := range config.LoadoutComponents {
		if !config.LoadoutEnabled(c.ID) {
			config.ToggleLoadout(c.ID)
		}
	}

	all := CoderAgentTools(nil, nil, nil, nil, nil)
	full := 0
	for _, tl := range all {
		full += toolTokens(tl)
	}

	const sid = "measure"
	tools.ForgetSession(sid)
	defer tools.ForgetSession(sid)

	visible := tools.VisibleTools(all, sid, true)
	deferredCost := 0
	for _, tl := range visible {
		deferredCost += toolTokens(tl)
	}

	index := len(tools.DeferredCatalogueBlock(all)) / 4 // ~4 chars per token
	saved := full - deferredCost - index

	t.Logf("tools: %d total, %d sent up front", len(all), len(visible))
	t.Logf("every schema:        %5d tokens", full)
	t.Logf("deferred + index:    %5d tokens (%d schemas + ~%d index)", deferredCost+index, deferredCost, index)
	t.Logf("saved per turn:      %5d tokens (%.0f%%)", saved, float64(saved)/float64(full)*100)

	if saved <= 0 {
		t.Errorf("deferral saves nothing (%d); the extra round trip buys the user no context back", saved)
	}
	// The index must not eat the saving. If it ever approaches the schemas it
	// replaces, the mechanism has stopped being worth its complexity.
	if index > deferredCost {
		t.Errorf("the catalogue index (%d) costs more than the schemas sent (%d)", index, deferredCost)
	}
}

// The name of the sub-agent tool is written as a literal in the tools package,
// because importing this package from there would be a cycle. If the constant
// ever changes, that literal silently stops deferring anything.
func TestSubAgentToolNameStaysInSyncWithTheDeferredList(t *testing.T) {
	if !tools.IsDeferrable(AgentToolName) {
		t.Errorf("AgentToolName is %q, which is not in the deferred list — "+
			"the literal in toolsearch.go has drifted from this constant", AgentToolName)
	}
}
