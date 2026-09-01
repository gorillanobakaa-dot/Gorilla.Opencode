// GORILLA OVERRIDE: this file did not exist upstream.
//
// It answers one question — "is any dialog on screen right now?" — and it answers
// it by reflection rather than by a hand-written list.
//
// The reason is that the answer decides whether the program is on the alternate
// screen. Dialogs assume they can paint a whole screen, and outside the alternate
// screen they cannot, so each one is shown by briefly switching buffers and
// switching back when it closes. A list of the ~18 show* flags maintained by hand
// would work today and rot the moment someone adds the nineteenth dialog: their
// new dialog would draw into the inline footer, overflow it, and corrupt the
// terminal — a failure with no obvious connection to the flag they forgot to add.
//
// So the flags are discovered from the struct. Adding a dialog field named show*
// is enough; nothing else has to be remembered.
package tui

import (
	"reflect"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// overlayFields holds the indices of every bool field on appModel whose name
// begins with "show". Computed once: the set cannot change at runtime.
var overlayFields = sync.OnceValue(func() []int {
	t := reflect.TypeFor[appModel]()
	var idx []int
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Type.Kind() == reflect.Bool && len(f.Name) > 4 && f.Name[:4] == "show" {
			idx = append(idx, i)
		}
	}
	return idx
})

// anyOverlayOpen reports whether anything is being drawn over the conversation.
//
// It covers the discovered show* flags plus the two overlays that are not
// booleans: a pending sign-in URL, and the summarising notice. Both paint over
// the full view in View(), so both need the alternate screen for the same reason
// a dialog does.
func (a appModel) anyOverlayOpen() bool {
	if a.loginURL != "" || a.isCompacting {
		return true
	}
	v := reflect.ValueOf(a)
	for _, i := range overlayFields() {
		if v.Field(i).Bool() {
			return true
		}
	}
	return false
}

// bufferCmd used to switch the terminal between the main and alternate screen
// buffers as dialogs opened and closed. It now never switches, and returns nil.
//
// GORILLA FIX (2026-09-01). The switch was destroying the conversation, and it
// could not be made safe from outside bubbletea.
//
// bubbletea's standard renderer tracks r.linesRendered and assigns it in exactly
// one place: r.linesRendered = len(newLines), the height of the frame it last
// drew. That counter is shared by both buffers. exitAltScreen calls repaint(),
// which clears lastRender and lastRenderedLines but NOT linesRendered. So after
// a dialog had been drawn full-screen on the alternate buffer, the first inline
// flush back on the main buffer ran
//
//	} else if r.linesRendered > 1 {
//	        buf.WriteString(ansi.CursorUp(r.linesRendered - 1))
//
// with linesRendered still holding the alt-screen height — walking the cursor
// sixty-odd rows UP into the printed conversation and drawing the short footer
// over it. The owner reported it as a blank screen at every permission prompt
// followed by "the past messages are gone", and that is exactly what it was.
//
// There is no public lever that resets linesRendered; clearScreen() only calls
// repaint() too. So the fix is not to switch buffers at all. Dialogs are now
// composited inline by placeOverlay in tui.go, on a canvas grown to the dialog's
// own height. Inline growth scrolls the conversation up into the terminal's
// scrollback intact, and the shrink back erases only rows the frame itself drew,
// because linesRendered then describes rows that really are the frame's.
//
// Kept as a function, rather than deleted with its call site, because this is
// where the decision is documented and where a future exception would have to be
// argued for. The tests below pin the rule.
//
// See TO.DO.TO.FIX/BUG-ALTSCREEN-ERASE.md for the full write-up.
func (a appModel) bufferCmd(wasOpen bool) tea.Cmd {
	return nil
}

// overlayFlagNames is used by tests to report what was discovered, so a failure
// can say which flags are covered rather than only that the count was wrong.
func overlayFlagNames() []string {
	t := reflect.TypeFor[appModel]()
	names := make([]string, 0, len(overlayFields()))
	for _, i := range overlayFields() {
		names = append(names, t.Field(i).Name)
	}
	return names
}
