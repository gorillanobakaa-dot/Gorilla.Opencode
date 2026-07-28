package tui

import (
	"fmt"
	"strings"
	"testing"
)

// printedLines reports how many newlines a pin command would emit, or -1 for no
// command. tea.Println appends one newline of its own, which is why the count is
// the payload's newlines plus one.
func printedLines(a *appModel, rows int) int {
	cmd := a.pinCmd(rows)
	if cmd == nil {
		return -1
	}
	// bubbletea's printLineMessage type is unexported, so the payload is read by
	// formatting the message. %v on the struct exposes the string it carries, and
	// its newlines are what determine where the cursor ends up.
	return strings.Count(fmt.Sprintf("%v", cmd()), "\n") + 1
}

// The prompt must be put on the last row exactly ONCE. Pinning on every resize
// would scroll the conversation out of view each time the window changed, and
// pinning never leaves the prompt drifting mid-screen — the reported symptom.
func TestTheFrameIsPinnedOnceNotOnEveryResize(t *testing.T) {
	a := &appModel{scrollback: true}

	if n := printedLines(a, 40); n != 39 {
		t.Errorf("first pin emitted %d lines for a 40-row window, want 39 (enough to "+
			"put the cursor on the last row)", n)
	}
	// Same size again: nothing. This is the case that would scroll the conversation
	// away on every redraw if it were wrong.
	if n := printedLines(a, 40); n != -1 {
		t.Errorf("re-pinned %d lines for an unchanged window; the conversation would "+
			"be scrolled out of view on every resize event", n)
	}
	// Shrinking needs nothing: the terminal scrolls to keep the cursor visible, so
	// the frame is already at the bottom.
	if n := printedLines(a, 24); n != -1 {
		t.Errorf("a smaller window emitted %d lines; shrinking already leaves the "+
			"frame at the bottom", n)
	}
}

// Growing the window adds rows BELOW the frame, which is exactly the gap this
// exists to remove — but only the new rows need scrolling, not a whole screen.
func TestGrowingTheWindowScrollsOnlyTheNewRows(t *testing.T) {
	a := &appModel{scrollback: true}
	printedLines(a, 24) // establish the bottom

	n := printedLines(a, 30)
	if n == -1 {
		t.Fatal("growing the window emitted nothing; the frame would sit 6 rows above " +
			"the bottom with dead space beneath it")
	}
	// Six new rows appeared below the frame, so the cursor must move down six rows:
	// six newlines, not seven. One newline per row is the whole arithmetic, and
	// getting it wrong by one leaves either a blank line or a one-row gap forever.
	if n != 6 {
		t.Errorf("growing 24->30 emitted %d lines, want 6 (one per new row). A whole "+
			"screen would push the conversation out of view for no reason", n)
	}
}

// With the alternate screen on, the frame owns the whole window already. Scrolling
// would corrupt it.
func TestNothingIsPinnedOnTheAlternateScreen(t *testing.T) {
	a := &appModel{scrollback: false}
	if n := printedLines(a, 40); n != -1 {
		t.Errorf("emitted %d lines while drawing on the alternate screen, where the "+
			"frame already occupies the whole window", n)
	}
}

// A degenerate size must not emit anything. Height 1 or 0 shows up during startup
// and on odd terminals, and "scroll by -1 lines" is not a thing.
func TestDegenerateWindowSizesArePinnedSafely(t *testing.T) {
	for _, rows := range []int{-5, 0, 1} {
		a := &appModel{scrollback: true}
		if n := printedLines(a, rows); n != -1 {
			t.Errorf("rows=%d emitted %d lines; nothing can be pinned in a window that "+
				"has no room for a frame", rows, n)
		}
		if a.pinnedOnce {
			t.Errorf("rows=%d marked the frame as pinned, so the real first size "+
				"message would be ignored and the prompt would never reach the bottom", rows)
		}
	}
}
