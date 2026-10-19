package tui

// GORILLA FIX (2026-10-19): the chat frame left debris when the window was made
// narrower. The owner drags the window on an 11-inch MacBook Air (Terminal from
// 169x43 down to 80x24 and back), and every shrink left the old, wider status
// bar and divider behind (measured with scripts/model-tests/tui_sizes.py
// --resize).
//
// Why: the chat is drawn inline, below the conversation, not in the alternate
// screen (the conversation is ordinary terminal output on purpose). bubbletea's
// inline renderer erases its last frame by moving the cursor up by the number of
// LOGICAL lines it drew. A terminal that rewraps text on resize (macOS Terminal,
// iTerm2, the Windows console, Windows Terminal, VTE, Konsole, kitty, tmux...)
// turns each old line wider than the new width into several rows, so the cursor
// stops short of the old frame's top and the rest stays on screen. See also
// internal/tui/startup/workspace.go, where the same mechanism was fixed for the
// start-up screens by moving them to the alternate screen.
//
// The repair: right after a resize, the next frame's first line starts by moving
// up over the extra rows and erasing to the end of the screen. The count is
// computed from the frame that was on screen and the new width. On a terminal
// that does NOT rewrap (plain xterm, the Linux console) there are no extra rows,
// and moving up would erase real conversation lines, so it is only done where
// the terminal is known to rewrap. GORILLA_TERMINAL_REWRAPS=1 or 0 overrides the
// guess either way.

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// frameTracker remembers the last frame View produced, so a resize can work out
// what the terminal did to it. A pointer, because View has a value receiver.
type frameTracker struct {
	lines []string
	width int
	// prefix is put in front of the first line of the next frames, and line0 is
	// the first line it was computed for. Once the first line changes, the
	// renderer rewrites it without the prefix; while it is unchanged the renderer
	// skips it, so the prefix is sent exactly once.
	prefix string
	line0  string
}

// extraRows is how many more rows than logical lines the frame `lines`, drawn at
// width `from`, takes once the terminal has rewrapped it to width `to`. The last
// line is not counted: the cursor sits at its start, so its own wrap lies below
// the cursor and is erased by the erase-to-end that follows.
func extraRows(lines []string, from, to int) int {
	if to <= 0 || from <= 0 || to >= from || len(lines) < 2 {
		return 0
	}
	extra := 0
	for _, l := range lines[:len(lines)-1] {
		w := ansi.StringWidth(l)
		if w > from {
			w = from // the renderer cut it to the old width
		}
		if rows := (w + to - 1) / to; rows > 1 {
			extra += rows - 1
		}
	}
	return extra
}

// terminalRewraps reports whether this terminal rewraps lines on resize.
func terminalRewraps() bool {
	switch os.Getenv("GORILLA_TERMINAL_REWRAPS") {
	case "1":
		return true
	case "0":
		return false
	}
	if runtime.GOOS == "windows" {
		return true // conhost and Windows Terminal both rewrap
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "Apple_Terminal", "iTerm.app", "WezTerm", "vscode", "ghostty", "Tabby", "Hyper", "tmux":
		return true
	}
	for _, v := range []string{"VTE_VERSION", "KONSOLE_VERSION", "KITTY_WINDOW_ID", "ALACRITTY_WINDOW_ID",
		"ALACRITTY_SOCKET", "WT_SESSION", "TMUX", "WEZTERM_EXECUTABLE"} {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}

// resized is called with the new width before the next frame is drawn.
func (f *frameTracker) resized(width int, rewraps bool) {
	if f == nil {
		return
	}
	if n := extraRows(f.lines, f.width, width); n > 0 && rewraps {
		f.prefix = fmt.Sprintf("\x1b[%dA%s", n, ansi.EraseScreenBelow)
		f.line0 = ""
	}
}

// frame records the frame about to be drawn and returns it with the repair
// prefix when one is pending.
func (f *frameTracker) frame(view string, width int) string {
	if f == nil {
		return view
	}
	lines := strings.Split(view, "\n")
	f.lines, f.width = lines, width
	if f.prefix == "" {
		return view
	}
	if f.line0 == "" {
		f.line0 = lines[0]
	}
	if lines[0] != f.line0 {
		f.prefix, f.line0 = "", ""
		return view
	}
	return f.prefix + view
}
