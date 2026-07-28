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

// bufferCmd returns the command that keeps the terminal buffer in step with the
// dialogs, given whether an overlay was open before this update.
//
// This is deliberately a named function rather than inline logic in Update: it is
// the whole rule, and a test that reimplemented it would pass while the program
// was wrong. Tests call this.
func (a appModel) bufferCmd(wasOpen bool) tea.Cmd {
	if !a.scrollback {
		return nil
	}
	nowOpen := a.anyOverlayOpen()
	if nowOpen == wasOpen {
		// No change. Returning a command here would switch buffers on every
		// keystroke, which flickers and loses the printed conversation from view.
		return nil
	}
	if nowOpen {
		return tea.EnterAltScreen
	}
	// Leaving discards everything drawn in the alternate screen, which is exactly
	// right: the dialog leaves no trace, and the printed conversation underneath was
	// never in that buffer to begin with.
	return tea.ExitAltScreen
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
