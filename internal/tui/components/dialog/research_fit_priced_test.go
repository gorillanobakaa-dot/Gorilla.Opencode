package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/llm/agent"
)

// The fit test above covers the unpriced shape only. With a priced helper
// model, supervised, at the helper maximum, the dialog was 33 rows on a 32-row
// screen (found 2026-10-06 while re-pricing the forecast). The footer is the
// part that gets cut, and the footer holds the keys.
func TestAPricedSupervisedDialogFitsTheScreenToo(t *testing.T) {
	pricedConfig(t)
	for _, sz := range []struct{ w, h int }{{176, 48}, {120, 40}, {100, 32}, {90, 30}} {
		m := dialogAt("supervised", agent.ResearchMaxAgents)
		m.SetSize(sz.w, sz.h)
		v := m.View()
		if got := lipgloss.Height(v); got > sz.h {
			t.Errorf("%dx%d: priced dialog is %d rows, taller than the screen\n%s", sz.w, sz.h, got, v)
		}
		if !strings.Contains(v, "enter: go") || !strings.Contains(v, "esc: cancel") {
			t.Errorf("%dx%d: key hints missing", sz.w, sz.h)
		}
	}
	_ = fmt.Sprint
}
