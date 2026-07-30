package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// NOTHING IN THE FRAME MAY BE WIDER THAN THE TERMINAL.
//
// Bubbletea's inline renderer moves the cursor up by the count of LOGICAL lines
// it last drew. A line wider than the terminal takes two physical rows but
// counts as one, so the erase under-reaches and strands footer debris in the
// transcript — the marching, jumping footer. The reproduction lives in
// internal/tui/inline/scroll_boundary_test.go; this pins the fix.
func TestClampToWidthLeavesNoLineWiderThanTheTerminal(t *testing.T) {
	cases := []struct {
		name  string
		view  string
		width int
	}{
		{"plain over-wide", strings.Repeat("x", 200), 80},
		{"exactly at width", strings.Repeat("x", 80), 80},
		{"under width", "short line", 80},
		{"multi-line mixed", "ok\n" + strings.Repeat("y", 300) + "\nalso ok", 100},
		{"styled over-wide", lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Render(strings.Repeat("z", 250)), 60},
		{"unicode", strings.Repeat("🦍", 100), 40},
		{"empty", "", 80},
	}

	for _, c := range cases {
		got := clampToWidth(c.view, c.width)
		for i, line := range strings.Split(got, "\n") {
			if w := ansi.StringWidth(line); w > c.width {
				t.Errorf("%s: line %d is %d cells wide, terminal is %d — this line "+
					"wraps and breaks the renderer's cursor-up arithmetic",
					c.name, i, w, c.width)
			}
		}
	}
}

// Truncating must not silently eat content that already fits.
func TestClampToWidthLeavesFittingLinesAlone(t *testing.T) {
	view := "model X · context 1.2K\ntokens 900 in / 300 out\n> "
	if got := clampToWidth(view, 200); got != view {
		t.Errorf("a frame that already fits was altered:\n got %q\nwant %q", got, view)
	}
}

// A zero or negative width means the size is not known yet. Truncating to zero
// would blank the frame, which looks like a crash.
func TestClampToWidthIsInertBeforeTheSizeIsKnown(t *testing.T) {
	view := "something"
	for _, w := range []int{0, -1} {
		if got := clampToWidth(view, w); got != view {
			t.Errorf("width=%d altered the frame: %q", w, got)
		}
	}
}

// Proves the assertion above is not vacuous: the UNCLAMPED string really does
// exceed the width, so the first test would fail without the fix.
func TestTheOverWideCaseIsGenuinelyOverWide(t *testing.T) {
	raw := strings.Repeat("x", 200)
	if ansi.StringWidth(raw) <= 80 {
		t.Fatal("the fixture is not actually over-wide; the test proves nothing")
	}
	if ansi.StringWidth(clampToWidth(raw, 80)) != 80 {
		t.Fatal("clamping did not bring the line to the terminal width")
	}
}
