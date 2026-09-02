package agent

import (
	"sort"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
)

// THE GUARANTEE: deferral never costs more than not deferring.
//
// Not "usually", and not "we documented the exception". This program is used
// on prepaid data and free-tier quotas, where a few hundred wasted tokens a
// turn is somebody's afternoon. A mechanism that silently flips from saving to
// costing after the fourth discovery is a leak, and a leak nobody is told
// about is the worst kind.
//
// So the invariant is checked at EVERY point along the discovery path, not
// just at the ends. It walks a session tool by tool -- the dearest first,
// which is the pessimistic order -- and asserts the cost never exceeds what
// the same session would have cost with the feature switched off.
func TestDeferralNeverCostsMoreThanNotDeferring(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	config.ResetLoadout()
	if !config.LoadoutEnabled(config.ToolSearchComponentID) {
		config.ToggleLoadout(config.ToolSearchComponentID)
	}

	a := &agent{}
	all := CoderAgentTools(nil, nil, nil, nil, nil)
	a.tools = all

	// What the same toolset costs with deferral off: every tool, no search
	// tool, no index.
	offCost := 0
	for _, tl := range withoutToolSearch(all) {
		offCost += toolTokens(tl)
	}

	// Deferrable tools, dearest first.
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

	const sid = "never-costs-more"
	tools.ForgetSession(sid)
	defer tools.ForgetSession(sid)

	cost := func() int {
		n := 0
		for _, tl := range a.visibleTools(sid) {
			n += toolTokens(tl)
		}
		// The index rides every turn while the search tool is still offered.
		for _, tl := range a.visibleTools(sid) {
			if tl.Info().Name == tools.ToolSearchToolName {
				n += indexTokensForTest()
				break
			}
		}
		return n
	}

	t.Logf("deferral OFF would cost %d tokens of schema per turn", offCost)
	t.Logf("")
	t.Logf("%-4s %-14s %8s %8s  %s", "n", "discovered", "ON", "vs OFF", "")
	worst := -1 << 30
	for i := 0; ; i++ {
		c := cost()
		delta := c - offCost
		if delta > worst {
			worst = delta
		}
		label := "(nothing yet)"
		if i > 0 {
			label = deferred[i-1].name
		}
		t.Logf("%-4d %-14s %8d %+8d", i, label, c, delta)
		if delta > 0 {
			t.Errorf("after %d discovery/discoveries deferral costs %d MORE tokens per turn "+
				"than not deferring -- that is a silent leak", i, delta)
		}
		if i >= len(deferred) {
			break
		}
		tools.MarkDiscovered(sid, deferred[i].name)
	}
	t.Logf("")
	t.Logf("worst point across the whole discovery path: %+d tokens vs deferral off", worst)
}

// indexTokensForTest mirrors what the agent charges for the catalogue block.
func indexTokensForTest() int {
	return len(catalogueForTest()) / 4
}

func catalogueForTest() string {
	all := CoderAgentTools(nil, nil, nil, nil, nil)
	return tools.DeferredCatalogueBlock(all)
}

// The switch-off must not strand the model: when deferral stops, every tool it
// could have discovered has to be present, or a capability vanishes.
func TestWhenDeferralSwitchesOffEveryToolIsPresent(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	config.ResetLoadout()
	if !config.LoadoutEnabled(config.ToolSearchComponentID) {
		config.ToggleLoadout(config.ToolSearchComponentID)
	}
	a := &agent{}
	all := CoderAgentTools(nil, nil, nil, nil, nil)
	a.tools = all

	const sid = "switch-off"
	tools.ForgetSession(sid)
	defer tools.ForgetSession(sid)

	// Discover everything, which is what forces the switch.
	for _, tl := range all {
		if tools.IsDeferrable(tl.Info().Name) {
			tools.MarkDiscovered(sid, tl.Info().Name)
		}
	}
	visible := a.visibleTools(sid)

	have := map[string]bool{}
	for _, tl := range visible {
		have[tl.Info().Name] = true
	}
	for _, tl := range all {
		n := tl.Info().Name
		if n == tools.ToolSearchToolName {
			if have[n] {
				t.Error("tool_search is still being sent after deferral switched off; " +
					"it can find nothing and costs its whole schema every turn")
			}
			continue
		}
		if !have[n] {
			t.Errorf("%s disappeared when deferral switched off", n)
		}
	}
}
