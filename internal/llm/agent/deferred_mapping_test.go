package agent

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
)

// A deferrable tool with no loadout row is invisible to the cost accounting,
// so /context would quote the price of a schema nobody sends. The two names
// genuinely differ (web_fetch is tool.fetch), so this cannot be a naming rule
// and has to be checked.
func TestEveryDeferrableToolMapsToARealRow(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	rows := map[string]bool{}
	for _, c := range config.LoadoutComponents {
		rows[c.ID] = true
	}
	all := CoderAgentTools(nil, nil, nil, nil, nil)
	for _, tl := range all {
		n := tl.Info().Name
		if !tools.IsDeferrable(n) {
			continue
		}
		id, ok := loadoutIDForTool(n)
		if !ok {
			t.Errorf("%s is deferrable but maps to no loadout row: /context will over-report the cost", n)
			continue
		}
		if !rows[id] {
			t.Errorf("%s maps to %q, which is not a real component", n, id)
		}
	}
}
