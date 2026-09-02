package dialog

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// A component that exists but never renders is unswitchable, and the only
// symptom is a tool the agent silently does not have -- which is exactly how
// tool.review was lost for days.
func TestEveryComponentIsReachableInContext(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	rows := sortedLoadout()
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.ID] = true
	}
	for _, c := range config.LoadoutComponents {
		if !seen[c.ID] {
			t.Errorf("%s exists but never appears in /context — unswitchable", c.ID)
		}
	}
	t.Logf("%d components, all present in /context", len(rows))

	// And each has the three things a user needs to decide.
	for _, c := range config.LoadoutComponents {
		if strings.TrimSpace(c.Name) == "" {
			t.Errorf("%s has no display name", c.ID)
		}
		if strings.TrimSpace(c.Tradeoff) == "" {
			t.Errorf("%s does not say what you lose by switching it off", c.ID)
		}
		if c.Tokens <= 0 {
			t.Errorf("%s has no token cost (%d)", c.ID, c.Tokens)
		}
	}
}

// The undo is the way back from the low-bandwidth preset. If its key stops
// being bound, the preset is one-way again and nothing else would say so.
func TestUndoKeyIsBoundAndDiscoverable(t *testing.T) {
	if len(loadoutKeys.UndoLowBW.Keys()) == 0 {
		t.Fatal("the low-bandwidth undo has no key bound")
	}
	if got := loadoutKeys.UndoLowBW.Keys()[0]; got != "u" {
		t.Errorf("undo bound to %q, expected u", got)
	}
	if h := loadoutKeys.UndoLowBW.Help().Desc; !strings.Contains(strings.ToLower(h), "undo") {
		t.Errorf("undo help text does not mention undo: %q", h)
	}
}
