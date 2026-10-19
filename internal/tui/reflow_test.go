package tui

import (
	"strings"
	"testing"
)

// The frame the owner saw at 138 columns, shrunk to 80: a full-width divider,
// the prompt, and the status bar.
func TestExtraRowsCountsWhatTheTerminalRewrapped(t *testing.T) {
	divider := strings.Repeat("─", 138)
	frame := []string{divider, " >", " ctrl+? help" + strings.Repeat(" ", 100) + "AUTO"}
	if got := extraRows(frame, 138, 80); got != 1 {
		t.Errorf("138 -> 80: %d extra rows, want 1 (the divider wraps to 2 rows; the last line is below the cursor)", got)
	}
	if got := extraRows(frame, 138, 60); got != 2 {
		t.Errorf("138 -> 60: %d extra rows, want 2", got)
	}
	if got := extraRows(frame, 80, 138); got != 0 {
		t.Errorf("growing the window must not move up: got %d", got)
	}
	if got := extraRows(frame, 138, 138); got != 0 {
		t.Errorf("same width: got %d", got)
	}
}

func TestTheRepairIsSentOnceAndOnlyWhereTheTerminalRewraps(t *testing.T) {
	f := &frameTracker{}
	old := strings.Repeat("─", 138) + "\n >\n status"
	f.frame(old, 138)

	f.resized(80, false)
	if got := f.frame("─────\n >\n status", 80); strings.HasPrefix(got, "\x1b[") {
		t.Fatalf("a terminal that does not rewrap got a cursor move, which would erase real lines: %q", got[:10])
	}

	f.frame(old, 138)
	f.resized(80, true)
	next := "─────\n >\n status"
	got := f.frame(next, 80)
	if !strings.HasPrefix(got, "\x1b[1A") {
		t.Fatalf("after a shrink the frame must start by moving up over the rewrapped row: %q", got)
	}
	if again := f.frame(next, 80); again != got {
		t.Errorf("the same first line must keep the same prefix (the renderer skips it unchanged)")
	}
	if changed := f.frame("other first line\n >\n status", 80); strings.HasPrefix(changed, "\x1b[") {
		t.Errorf("once the first line changes, the prefix must be gone, or it would move up again")
	}
}

func TestTheRewrapOverrideWins(t *testing.T) {
	t.Setenv("GORILLA_TERMINAL_REWRAPS", "0")
	if terminalRewraps() {
		t.Error("GORILLA_TERMINAL_REWRAPS=0 must switch the repair off")
	}
	t.Setenv("GORILLA_TERMINAL_REWRAPS", "1")
	if !terminalRewraps() {
		t.Error("GORILLA_TERMINAL_REWRAPS=1 must switch the repair on")
	}
}
