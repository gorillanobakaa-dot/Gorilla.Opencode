package dialog

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
)

// The fit test in research_test.go covers the unpriced shape only. With a
// priced helper model, supervised, at the helper maximum, the dialog was 33
// rows on a 32-row screen (found 2026-10-06 while re-pricing the forecast).
// The footer is the part that gets cut, and the footer holds the keys.
//
// Both shapes are rendered: no run measured on this computer, and runs
// measured. Without the explicit cache the test read the maintainer's own run
// record and passed on Windows while failing on a clean Linux runner.
func TestAPricedSupervisedDialogFitsTheScreenToo(t *testing.T) {
	pricedConfig(t)
	// The chat model differs from the helper model here, which adds the
	// HELPERS RUN ON block: the tallest shape (48 rows before 2026-10-09).
	for _, measured := range []bool{false, true} {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		if measured {
			for _, in := range []int64{60000, 90000, 120000} {
				config.RecordResearchRun(4, in, 3000, 20, 90*time.Second)
			}
		}
		for _, sz := range []struct{ w, h int }{{176, 48}, {120, 40}, {100, 32}, {90, 30}} {
			m := dialogAt("supervised", agent.ResearchMaxAgents)
			m.SetSize(sz.w, sz.h)
			v := m.View()
			if got := lipgloss.Height(v); got > sz.h {
				t.Errorf("%dx%d (measured=%v): priced dialog is %d rows, taller than the screen\n%s", sz.w, sz.h, measured, got, v)
			}
			if !strings.Contains(v, "enter: go") || !strings.Contains(v, "esc: cancel") {
				t.Errorf("%dx%d (measured=%v): key hints missing", sz.w, sz.h, measured)
			}
		}
	}
}
