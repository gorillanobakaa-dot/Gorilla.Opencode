package startup

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// 2026-10-19: on the owner's 11-inch MacBook Air Terminal opens at 80x24, and
// the portal (17 providers) was taller than that: "Choose a provider" scrolled
// off the top. Measured with the tui-sizes tool (Fieldkit) on v0.1.148.
func TestThePortalFitsTheAirsTerminalSizes(t *testing.T) {
	rows := make([]ProviderRow, 17)
	for i := range rows {
		rows[i] = ProviderRow{ID: fmt.Sprint(i), Name: fmt.Sprintf("Provider %d - a label of ordinary length", i),
			What: "A description of three sentences, as the real rows have. It wraps on a narrow screen. " +
				"It must still leave the list and the title visible."}
	}
	rows[16].Warning = "A warning line that will genuinely surprise someone, wrapped like the rest."
	// One model per cursor position, RESIZED through every size in turn: the
	// owner drags the window, so each frame after a resize must fit it, whether
	// it got bigger or smaller.
	sizes := [][2]int{{169, 43}, {80, 24}, {138, 37}, {100, 30}, {144, 33}, {60, 18}, {169, 43}, {80, 24}}
	for _, sel := range []int{0, 8, 16} {
		m := NewProviderModel(rows, true)
		m.sel = sel
		for _, size := range sizes {
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			// not trimmed: the frame's trailing newline is a row on screen too
			lines := strings.Split(m.View(), "\n")
			if len(lines) > size[1] {
				t.Errorf("%dx%d, cursor on row %d: %d lines, taller than the window", size[0], size[1], sel, len(lines))
			}
			if !strings.Contains(lines[0], "Choose a provider") {
				t.Errorf("%dx%d, cursor on row %d: the title is not the first line: %q", size[0], size[1], sel, lines[0])
			}
			if !strings.Contains(m.View(), fmt.Sprintf("> Provider %d ", sel)) {
				t.Errorf("%dx%d: the selected row %d is not on screen", size[0], size[1], sel)
			}
		}
	}
}

func TestVisibleWindowKeepsTheCursorAndCountsTheRest(t *testing.T) {
	for _, c := range []struct{ n, sel, room, first, last int }{
		{17, 0, 40, 0, 17},  // everything fits
		{17, 0, 10, 0, 9},   // top: one "more" line below
		{17, 16, 10, 8, 17}, // bottom: one "more" line above
		{17, 8, 10, 4, 12},  // middle: both
		{17, 8, 1, 8, 9},    // no room: the selected row is still drawn
	} {
		f, l := visibleWindow(c.n, c.sel, c.room)
		if f != c.first || l != c.last {
			t.Errorf("visibleWindow(%d, %d, %d) = %d, %d; want %d, %d", c.n, c.sel, c.room, f, l, c.first, c.last)
		}
		if c.sel < f || c.sel >= l {
			t.Errorf("visibleWindow(%d, %d, %d) left the cursor out", c.n, c.sel, c.room)
		}
	}
}
