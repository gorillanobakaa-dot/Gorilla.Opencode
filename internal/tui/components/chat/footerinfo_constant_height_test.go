package chat

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/session"
)

// Audit EVERY state the info block passes through in a real session. A bound is
// not enough: the frame must be the SAME height every render, not merely under
// a limit.
func TestCompactViewHeightIsConstantNotMerelyBounded(t *testing.T) {
	states := []struct {
		name string
		sess session.Session
	}{
		{"fresh session", session.Session{}},
		{"small counts", session.Session{PromptTokens: 900, CompletionTokens: 120, Cost: 0.01}},
		{"large counts", session.Session{PromptTokens: 494300, CompletionTokens: 13000, Cost: 0.78}},
		{"huge cost", session.Session{PromptTokens: 1200000, CompletionTokens: 900000, Cost: 1234.56}},
	}

	for _, width := range []int{80, 120, 200} {
		var first int
		for i, s := range states {
			m := &sidebarCmp{session: s.sess}
			h := lipgloss.Height(m.CompactView(width))
			if i == 0 {
				first = h
				t.Logf("width=%d %-16s height=%d", width, s.name, h)
				continue
			}
			t.Logf("width=%d %-16s height=%d", width, s.name, h)
			if h != first {
				t.Errorf("width=%d: %q renders %d rows but %q renders %d — the frame "+
					"changes height as the numbers change, which makes bubbletea's "+
					"cursor-up erase land in the wrong place",
					width, states[0].name, first, s.name, h)
			}
		}
	}
}
