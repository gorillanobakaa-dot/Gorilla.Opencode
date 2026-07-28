package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// PlaceOverlay had no test at all, which is why the clamp below could be added and
// then "verified" by deleting it and seeing nothing fail. It is the last line of
// defence for every dialog in the program, present and future.
//
// The consequence of a miss is not cosmetic: a foreground taller than the
// background pushes the composed frame past the terminal, the terminal scrolls, and
// the layout is wrecked for the rest of the session. Losing the bottom of a dialog
// is strictly better than that.
func TestOverlayTallerThanBackgroundIsClamped(t *testing.T) {
	// The overlay must be NARROWER than the background here, so this exercises the
	// row loop rather than the early return below.
	bg := strings.Repeat("background row is wide\n", 10)
	fg := strings.Repeat("dialog\n", 25)

	out := PlaceOverlay(0, 0, strings.TrimSuffix(fg, "\n"), strings.TrimSuffix(bg, "\n"), false)

	if got := lipgloss.Height(out); got > 10 {
		t.Errorf("a 25-row overlay on a 10-row background produced %d rows — the frame now exceeds the terminal, which scrolls it and destroys the layout", got)
	}
}

// The path that actually needed the clamp, and the one my first test missed.
//
// PlaceOverlay short-circuits when the overlay is BOTH wider and taller than the
// background:
//
//	if fgWidth >= bgWidth && fgHeight >= bgHeight {
//		// FIXME: return fg or bg?
//		return fg
//	}
//
// That returned the oversized overlay verbatim, so the composed frame exceeded the
// terminal in both directions — the terminal scrolls and the layout is wrecked. When
// the overlay is merely taller, the loop over background rows bounds it, which is
// why a narrower test case passed with or without the clamp and reported a false
// all-clear. This is the case that distinguishes them.
func TestOverlayBothTallerAndWiderIsClamped(t *testing.T) {
	bg := strings.TrimSuffix(strings.Repeat(strings.Repeat("b", 30)+"\n", 10), "\n")
	fg := strings.TrimSuffix(strings.Repeat(strings.Repeat("f", 60)+"\n", 25), "\n")

	out := PlaceOverlay(0, 0, fg, bg, false)

	if got := lipgloss.Height(out); got > 10 {
		t.Errorf("height %d on a 10-row background — an overlay both taller AND wider was returned unclamped", got)
	}
	for i, l := range strings.Split(out, "\n") {
		if got := lipgloss.Width(l); got > 30 {
			t.Errorf("row %d is %d columns on a 30-column background", i, got)
		}
	}
}

func TestOverlayWiderThanBackgroundIsClamped(t *testing.T) {
	bg := strings.Repeat(strings.Repeat("b", 30)+"\n", 5)
	fg := strings.Repeat("f", 100)

	out := PlaceOverlay(0, 0, fg, strings.TrimSuffix(bg, "\n"), false)

	for i, l := range strings.Split(out, "\n") {
		if got := lipgloss.Width(l); got > 30 {
			t.Errorf("row %d is %d columns on a 30-column background; the right-hand side is clipped at the screen edge", i, got)
		}
	}
}

// An overlay that fits must pass through untouched — clamping must not shrink or
// reflow a dialog that was already the right size.
func TestOverlayThatFitsIsUnchanged(t *testing.T) {
	bg := strings.TrimSuffix(strings.Repeat(strings.Repeat("b", 40)+"\n", 20), "\n")
	fg := strings.TrimSuffix(strings.Repeat("dialog\n", 6), "\n")

	out := PlaceOverlay(2, 2, fg, bg, false)

	if got := lipgloss.Height(out); got != 20 {
		t.Errorf("composed height %d, want the background's 20", got)
	}
	// Every one of the six dialog rows must still be present.
	if n := strings.Count(out, "dialog"); n != 6 {
		t.Errorf("found %d dialog rows in the composition, want 6 — a fitting overlay was altered", n)
	}
}

// The degenerate sizes a resize actually produces, none of which may panic.
func TestOverlayDegenerateSizes(t *testing.T) {
	cases := []struct{ name, fg, bg string }{
		{"empty fg", "", "bg row\nbg row"},
		{"empty bg", "dialog", ""},
		{"both empty", "", ""},
		{"single cell bg", "a very long dialog line", "x"},
		{"fg with trailing newline", "dialog\n", "bg\nbg"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_ = PlaceOverlay(0, 0, c.fg, c.bg, false)
		})
	}
}
