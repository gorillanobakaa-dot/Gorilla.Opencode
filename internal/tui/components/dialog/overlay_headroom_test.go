// GORILLA OVERRIDE (2026-09-03): overlays must leave room for what is under them.
//
// Both of the big full-width screens hit the same fault, one after the other,
// and the second hit it BECAUSE the first was fixed alone.
//
// These dialogs are drawn by placeOverlay. In scrollback mode that grows the
// canvas by the overlay's FULL height and then renders the prompt and the footer
// BELOW it. So a frame exactly as tall as the terminal overflows by precisely
// the height of that footer, the view scrolls, and the rows that leave the
// screen are the ones at the TOP.
//
// That is the cruel part: the BOTTOM of the dialog looks perfect, because the
// bottom is what survives. What is lost is the title and the state line. On
// /help it was "Commands - what each one does" and the command count. On
// /context it was the cost line and the red LOW-BANDWIDTH MODE banner, which
// existed specifically so that a mode could not be invisible, and was then made
// invisible by a layout bug. Both were reported from screenshots of the running
// program. Neither was caught here, because every existing test asserted on the
// list of switches, and the list is the part that survives.
//
// Deliberately driven from the SAME sizedDialogs registry the overflow ratchet
// uses, so a new full-window dialog inherits the check instead of having to
// remember it.
package dialog

import (
	"strings"
	"testing"
)

// overlayHeaders are the dialogs drawn as overlays, and the text that has to
// still be on screen. The string is from the TOP of each frame, because the top
// is what a too-tall overlay loses.
var overlayHeaders = map[string]string{
	"/help": "Commands",
	// The cost line is explanation and compact mode drops it first, by design.
	// The TITLE is not explanation: it is what tells you which screen you are on,
	// and it is always rendered.
	"/context": "Context loadout",
}

// The frame must be STRICTLY shorter than the terminal. Equality is the bug: the
// footer drawn underneath is what pushes the top off the screen.
func TestOverlaysLeaveRoomForWhatIsUnderThem(t *testing.T) {
	for name, build := range sizedDialogs(t) {
		if _, ok := overlayHeaders[name]; !ok {
			continue
		}
		for _, size := range terminalSizes {
			if size.h < standardHeight {
				continue // the small-terminal ratchet owns those
			}
			d := build()
			d.SetSize(size.w, size.h)
			got := len(strings.Split(d.View(), "\n"))
			if got >= size.h {
				t.Errorf("%s at %dx%d: frame is %d rows in a %d-row terminal.\n"+
					"  placeOverlay draws the prompt and footer BELOW this, so a frame that "+
					"uses the whole terminal scrolls the view, and the TITLE is what leaves "+
					"the screen.", name, size.w, size.h, got, size.h)
			}
		}
	}
}

// And the thing that actually goes missing must actually be there.
func TestOverlaysKeepTheirHeaderOnScreen(t *testing.T) {
	for name, build := range sizedDialogs(t) {
		want, ok := overlayHeaders[name]
		if !ok {
			continue
		}
		for _, size := range terminalSizes {
			if size.h < standardHeight {
				continue
			}
			d := build()
			d.SetSize(size.w, size.h)
			if !strings.Contains(d.View(), want) {
				t.Errorf("%s at %dx%d: %q is not in the frame, so the header has been "+
					"pushed off the top", name, size.w, size.h, want)
			}
		}
	}
}
