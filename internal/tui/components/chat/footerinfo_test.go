package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/screentest"
)

func infoFor(sess session.Session, mods map[string]struct {
	additions int
	removals  int
}) *sidebarCmp {
	return &sidebarCmp{session: sess, modFiles: mods}
}

// The footer is redrawn in place, and outside the alternate screen bubbletea erases
// its last frame by counting LOGICAL lines. A block that grows past its budget does
// not merely look wrong — every later erase lands in the wrong place, which is the
// stale-copy corruption the startup picker used to show. So the height is a
// contract, checked across widths narrow enough to force truncation.
func TestCompactViewNeverExceedsItsRowBudget(t *testing.T) {
	mods := map[string]struct {
		additions int
		removals  int
	}{
		"internal/tui/tui.go":  {additions: 120, removals: 8},
		"internal/config/x.go": {additions: 3, removals: 44},
	}
	sess := session.Session{
		Title: "a session title long enough to be a nuisance on a narrow terminal",
		Cost:  1.23, PromptTokens: 91_234, CompletionTokens: 7_775,
	}
	cmp := infoFor(sess, mods)

	for _, width := range []int{20, 40, 60, 80, 100, 200} {
		got := cmp.CompactView(width)
		if strings.TrimSpace(got) == "" {
			t.Errorf("width %d rendered nothing", width)
			continue
		}
		if rows := lipgloss.Height(got); rows > footerInfoRows {
			t.Errorf("width %d rendered %d rows, budget is %d. A line that WRAPS instead "+
				"of truncating is how this happens, and it corrupts every later redraw",
				width, rows, footerInfoRows)
		}
		for i, line := range strings.Split(got, "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line %d is %d columns wide; the terminal would wrap it "+
					"and add a row that the renderer does not know about", width, i, w)
			}
		}
	}
}

// An unknown width must render nothing rather than guess. The footer is drawn on
// every frame, including the ones before the first size message arrives.
func TestCompactViewRefusesUnknownWidth(t *testing.T) {
	cmp := infoFor(session.Session{Cost: 1}, nil)
	for _, width := range []int{0, -10} {
		if got := cmp.CompactView(width); got != "" {
			t.Errorf("width %d rendered %q; nothing can be laid out without a width", width, got)
		}
	}
}

// The numbers must actually be the session's. A footer that renders a pleasing
// layout of stale or zeroed values is worse than no footer, because it is believed.
func TestCompactViewReportsTheSessionsRealNumbers(t *testing.T) {
	sess := session.Session{Cost: 4.56, PromptTokens: 12_000, CompletionTokens: 3_400}
	got := infoFor(sess, map[string]struct {
		additions int
		removals  int
	}{"a.go": {additions: 7, removals: 2}})

	view := got.CompactView(200)
	for _, want := range []string{"$4.56", "12.0K", "3.4K", "1 files", "+7", "-2"} {
		if !strings.Contains(view, want) {
			t.Errorf("footer does not report %q; rendered:\n%s", want, view)
		}
	}
}

// A session with nothing in it must not claim files were changed. "no files" and
// "1 files" are different statements and the empty case is the common one.
func TestCompactViewDistinguishesAnUntouchedSession(t *testing.T) {
	empty := infoFor(session.Session{}, nil).CompactView(200)
	if !strings.Contains(empty, "no files") {
		t.Errorf("an untouched session does not say so; rendered:\n%s", empty)
	}

	touched := infoFor(session.Session{}, map[string]struct {
		additions int
		removals  int
	}{"a.go": {additions: 1}}).CompactView(200)
	if strings.Contains(touched, "no files") {
		t.Errorf("a session with a modified file claims none were changed; rendered:\n%s", touched)
	}
}

// Every row must be painted edge to edge in ONE colour.
//
// This is the defect that made the footer look unfinished: the key/value pairs were
// styled but the separators between them were raw strings, so they inherited the
// terminal's own background. Outside the alternate screen that is not a shade
// difference, it is a hole — measured as a background break at column 19 of a
// 100-column line, seen as black rectangles punched through a coloured bar.
//
// A width assertion cannot catch this. The row was exactly the right length.
func TestCompactViewIsPaintedEdgeToEdge(t *testing.T) {
	cmp := infoFor(session.Session{
		Cost: 0.01, PromptTokens: 9_600, CompletionTokens: 98,
	}, map[string]struct {
		additions int
		removals  int
	}{"internal/tui/tui.go": {additions: 12, removals: 3}})

	for _, width := range []int{40, 80, 100, 160} {
		view := cmp.CompactView(width)
		if strings.TrimSpace(view) == "" {
			t.Fatalf("width %d rendered nothing", width)
		}
		rows := lipgloss.Height(view)
		s := screentest.Render(view, width, rows)

		for y := 0; y < rows; y++ {
			if col := s.BackgroundBreak(y); col >= 0 {
				t.Errorf("width %d row %d: background changes at column %d, so the row is "+
					"painted in patches. The terminal's own background shows through the gap.\n  row: %q",
					width, y, col, s.Text(y))
			}
		}
		// And it must reach the right-hand edge: a row that stops at its last
		// character leaves the remainder unpainted for the same reason.
		for y := 0; y < rows; y++ {
			if w := lipgloss.Width(strings.Split(view, "\n")[y]); w != width {
				t.Errorf("width %d row %d is %d columns wide; a short row leaves the rest of "+
					"the line showing the terminal background", width, y, w)
			}
		}
	}
}
