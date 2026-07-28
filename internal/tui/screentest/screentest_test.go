package screentest

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// A harness is only worth having if it catches the bug it was built for. This is
// the /help defect reproduced exactly: a row styled with the same colour for
// foreground and background. The text is in the output and invisible on screen.
func TestLegibleCatchesForegroundEqualToBackground(t *testing.T) {
	const grey = "#323232"
	invisible := lipgloss.NewStyle().Foreground(lipgloss.Color(grey)).Background(lipgloss.Color(grey))
	visible := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color(grey))

	view := strings.Join([]string{
		visible.Render("/clear   start a new conversation"),
		invisible.Render("/export  save this conversation"), // the selected row
		visible.Render("/cd      change folder"),
	}, "\n")

	s := Render(view, 40, 3)

	// String matching cannot tell these apart — proving why the grid is needed.
	if !s.Contains("/export") {
		t.Fatal("the text is not on the grid at all; this test is not exercising the right thing")
	}

	if s.RowLegible(1) {
		t.Error("a row with foreground equal to background was reported legible — this is the /help bug and the harness must catch it")
	}
	for _, y := range []int{0, 2} {
		if !s.RowLegible(y) {
			t.Errorf("row %d has contrasting colours but was reported illegible", y)
		}
	}
}

// Default colours must not be mistaken for a collision, or every unstyled
// component would fail.
func TestUnstyledTextIsLegible(t *testing.T) {
	s := Render("plain text with no styling at all", 40, 1)
	if !s.RowLegible(0) {
		t.Error("unstyled text reported illegible; nil foreground and nil background is the terminal default and is readable")
	}
}

// One colour set and the other left default cannot collide, because we do not know
// what the terminal's default is. Claiming otherwise would produce false failures.
func TestOneColourSetIsTreatedAsLegible(t *testing.T) {
	fgOnly := lipgloss.NewStyle().Foreground(lipgloss.Color("#323232")).Render("only fg set")
	if !Render(fgOnly, 40, 1).RowLegible(0) {
		t.Error("foreground-only styling reported illegible")
	}
	bgOnly := lipgloss.NewStyle().Background(lipgloss.Color("#323232")).Render("only bg set")
	if !Render(bgOnly, 40, 1).RowLegible(0) {
		t.Error("background-only styling reported illegible")
	}
}

// Blank space is not "readable content", so a padded row of nothing must not pass
// a legibility check that a caller uses to mean "this row shows something".
func TestBlankRowIsNotLegible(t *testing.T) {
	s := Render(lipgloss.NewStyle().Width(20).Render(""), 20, 1)
	if s.RowLegible(0) {
		t.Error("an empty padded row was reported legible, so an assertion that a row is visible would pass on a blank")
	}
}

// The overflow the /help dialog shipped: more rows emitted than the terminal has.
// The grid must report it rather than silently absorbing it.
func TestOverflowIsReportedNotHidden(t *testing.T) {
	view := strings.Repeat("a row of content\n", 15)

	tall := Render(view, 40, 20)
	if tall.Overflows() {
		t.Errorf("15 rows in a 20-row terminal reported as overflowing (wanted %d)", tall.RowsWanted())
	}

	short := Render(view, 40, 10)
	if !short.Overflows() {
		t.Errorf("15 rows in a 10-row terminal did NOT report overflow (wanted %d, height 10) — this is exactly the /help bug", short.RowsWanted())
	}
	if short.RowsWanted() != 15 {
		t.Errorf("RowsWanted = %d, want 15 — the requested height must survive so the failure message can state it", short.RowsWanted())
	}
	// And the grid itself is still only as tall as the terminal.
	if got := len(short.Lines()); got != 10 {
		t.Errorf("the grid holds %d rows, want 10 — it must model the terminal, not the wish", got)
	}
}

// Content wider than the terminal is clipped by a real terminal, and the grid must
// clip it the same way — otherwise a width bug looks fine in the test.
func TestContentWiderThanTheTerminalIsClipped(t *testing.T) {
	s := Render(strings.Repeat("x", 100), 20, 1)

	cols, _ := s.WidestRow()
	if cols > 20 {
		t.Errorf("widest row is %d columns in a 20-column grid; the grid is not clipping like a terminal", cols)
	}
}

// FindRow is what lets a test say "the selected row, wherever it is, must be
// legible" without hard-coding an index that shifts when a list changes.
func TestFindRowLocatesContent(t *testing.T) {
	s := Render("first\nsecond\nthird", 20, 3)

	if got := s.FindRow("second"); got != 1 {
		t.Errorf("FindRow(second) = %d, want 1", got)
	}
	if got := s.FindRow("nowhere"); got != -1 {
		t.Errorf("FindRow(nowhere) = %d, want -1", got)
	}
}

// The grid must survive the awkward inputs real components produce, rather than
// panicking inside a test suite.
func TestDegenerateInputsDoNotPanic(t *testing.T) {
	cases := []struct {
		name          string
		view          string
		width, height int
	}{
		{"empty view", "", 20, 5},
		{"single newline", "\n", 20, 5},
		{"zero height", "content", 20, 0},
		{"one by one", "x", 1, 1},
		{"wide runes", "日本語のテキスト", 10, 1},
		{"trailing newlines", "a\n\n\n", 20, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := Render(c.view, c.width, c.height)
			_ = s.String()
			_ = s.Overflows()
			if c.height > 0 {
				_ = s.RowLegible(0)
			}
		})
	}
}

// Double-width characters occupy two cells. Reading them back must not duplicate or
// drop them, or any assertion about a Japanese or emoji label is meaningless.
func TestWideRunesReadBackIntact(t *testing.T) {
	s := Render("日本", 10, 1)
	if got := s.Text(0); !strings.Contains(got, "日本") {
		t.Errorf("wide runes did not survive the grid: %q", got)
	}
}
