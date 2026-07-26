package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// overflowPanel deliberately renders WIDER and TALLER than it was allotted —
// simulating a growing textarea. The layout must absorb this without shifting
// or clipping its neighbour.
type overflowPanel struct {
	w, h       int
	ch         string
	extraW     int
	extraLines int
}

func (m *overflowPanel) Init() tea.Cmd                       { return nil }
func (m *overflowPanel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m *overflowPanel) SetSize(w, h int) tea.Cmd            { m.w, m.h = w, h; return nil }
func (m *overflowPanel) GetSize() (int, int)                 { return m.w, m.h }
func (m *overflowPanel) BindingKeys() []key.Binding          { return nil }
func (m *overflowPanel) View() string {
	line := strings.Repeat(m.ch, m.w+m.extraW)
	rows := make([]string, m.h+m.extraLines)
	for i := range rows {
		rows[i] = line
	}
	return strings.Join(rows, "\n")
}

// The invariant: whatever children do, the composed layout is exactly
// width x height, every row full width, and the sidebar occupies the
// rightmost columns on EVERY row (top to bottom).
func TestSplitLayoutInvariantWithOverflowingEditor(t *testing.T) {
	const W, H = 100, 40

	for _, tc := range []struct {
		name               string
		extraW, extraLines int
	}{
		{"well-behaved", 0, 0},
		{"editor 2 cols too wide", 2, 0},
		{"editor 6 lines too tall", 0, 6},
		{"editor too wide AND tall", 3, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left := &overflowPanel{ch: "L"}
			right := &overflowPanel{ch: "R"}
			bottom := &overflowPanel{ch: "B", extraW: tc.extraW, extraLines: tc.extraLines}

			l := NewSplitPane(WithLeftPanel(left), WithRightPanel(right))
			l.SetBottomPanel(bottom)
			l.SetSize(W, H)

			lines := strings.Split(l.View(), "\n")

			if len(lines) != H {
				t.Errorf("height: got %d rows, want %d", len(lines), H)
			}
			for i, ln := range lines {
				if w := lipgloss.Width(ln); w != W {
					t.Errorf("row %d width: got %d, want %d", i, w, W)
					break
				}
			}
			// Sidebar must be present on the first and last row.
			if !strings.Contains(lines[0], "R") {
				t.Error("sidebar missing from TOP row")
			}
			if !strings.Contains(lines[len(lines)-1], "R") {
				t.Error("sidebar missing from BOTTOM row")
			}
		})
	}
}
