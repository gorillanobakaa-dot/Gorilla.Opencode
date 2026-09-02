//go:build windows

// GORILLA OVERRIDE: this file did not exist upstream, and without it the
// interface is drawn for the wrong size on every Windows machine whose window
// is not exactly the size it had at launch.
//
// bubbletea does not watch for a resize on Windows. Its own source says so:
//
//	//go:build windows
//	// listenForResize is not available on windows because windows does not
//	// implement syscall.SIGWINCH.
//	func (p *Program) listenForResize(done chan struct{}) { close(done) }
//
// and its renderer repeats the warning where it truncates lines: "on Windows we
// only get the width of the window on program initialization, so after a resize
// this won't perform correctly". So the size is read ONCE, at startup, and never
// again.
//
// What that costs is not subtle. A console launched from the Windows shortcut
// starts at conhost's stored default -- 120 columns by 30 rows on the owner's
// machine -- and is then maximised to about 239 by 64. The program goes on
// drawing for 120 by 30 inside a window more than twice that size, so:
//
//   - every printed line wraps at column 120 and the right half of the window
//     stays empty;
//   - the frame is 30 rows tall in a 64-row window, so the prompt ends up
//     somewhere in the middle of the screen with output scrolling past it.
//
// The owner reported both, separately, before either was understood: "the print
// does not make full use of the terminal window's width" and "the print seem to
// be scrolling below under and the rest of the prompt is not visible anymore ...
// On linux it does not do that." Linux has SIGWINCH, so bubbletea keeps up there
// and the fault is invisible.
//
// Polling is the only option. Windows can deliver window-resize records through
// the console input handle, but bubbletea owns that handle for keyboard input
// and a second reader would steal keystrokes from it. Asking for the size is
// cheap and touches nothing bubbletea is using.
package tui

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// resizePollInterval is how often the console is asked for its size.
//
// Chosen to be unnoticeable to a person dragging a window edge while costing
// nothing measurable: GetConsoleScreenBufferInfo is a single syscall against a
// handle the process already holds. Faster would be wasted -- a human cannot
// resize a window in less than this -- and slower would leave the interface
// visibly drawn at the old size after a maximise.
const resizePollInterval = 250 * time.Millisecond

// WatchTerminalResize sends a tea.WindowSizeMsg whenever the console's size
// changes, until stop is closed.
//
// It reports only CHANGES. bubbletea has already delivered the size at startup,
// and re-sending an unchanged size every quarter second would make every
// component recompute its layout forever.
func WatchTerminalResize(p *tea.Program, stop <-chan struct{}) {
	if p == nil {
		return
	}
	fd := int(os.Stdout.Fd())

	lastW, lastH, err := term.GetSize(fd)
	if err != nil {
		// Not a terminal: output is redirected to a file or a pipe. There is no
		// size to watch and nothing to correct, so this exits rather than
		// polling a handle that will never answer.
		return
	}

	ticker := time.NewTicker(resizePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			w, h, err := term.GetSize(fd)
			if err != nil {
				// A transient failure is not worth acting on: reporting a size
				// we could not read would be worse than keeping the last one we
				// could. Try again on the next tick.
				continue
			}
			if w == lastW && h == lastH {
				continue
			}
			// Guard against a zero size. A console being restored from minimised
			// briefly reports 0x0 on some Windows builds, and a frame drawn for a
			// zero-width terminal is not recoverable -- every component divides
			// by it.
			if w <= 0 || h <= 0 {
				continue
			}
			lastW, lastH = w, h
			p.Send(tea.WindowSizeMsg{Width: w, Height: h})
		}
	}
}
