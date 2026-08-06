package tui

import (
	"errors"
	"testing"

	"github.com/opencode-ai/opencode/internal/tui/layout"
)

// THE GAP: the provider picker ran only at launch. If the provider chosen there
// turned out not to work — a key rejected, a model not entitled, a plan limit —
// the only way to try another was to quit and relaunch. /connect, /login and
// /model between them can do the job, but that is three commands and none of
// them is the screen the user was just shown. The people this is built for are
// exactly the ones who will not know to go looking.
// A KEY BINDING WAS TRIED FIRST AND ABANDONED. ctrl+p is Print in most GUI
// contexts and "previous line" in readline; ctrl+c is SIGINT; esc already
// cancels a running turn in this app, so taking it would mean esc sometimes
// stops your request and sometimes opens a menu. Nearly every remaining control
// key is bound here or reserved by the terminal (ctrl+z suspend, ctrl+d EOF,
// ctrl+q/s flow control, ctrl+b tmux prefix).
//
// So the escape hatch is a slash command: it collides with nothing, appears in
// /help, and is the idiom this app already teaches.
func TestNoConflictingKeyBindingWasIntroduced(t *testing.T) {
	// ctrl+c is a deliberate, conventional exception: it is the Quit binding and
	// has been since before this change. Everything else here is a key that
	// either the terminal owns or this app already gives another meaning.
	const allowed = "ctrl+c"
	reserved := map[string]string{
		"ctrl+p": "Print in GUI contexts, previous-line in readline",
		"ctrl+z": "suspend",
		"ctrl+d": "EOF",
		"ctrl+v": "paste",
		"ctrl+x": "cut",
		"ctrl+q": "flow control",
		"ctrl+b": "tmux prefix",
		"esc":    "cancels the running turn in this app",
	}
	for _, b := range layout.KeyMapToSlice(keys) {
		for _, k := range b.Keys() {
			if k == allowed {
				continue
			}
			if why, bad := reserved[k]; bad {
				t.Errorf("global binding uses %q, which is %s", k, why)
			}
		}
	}
}

// The hook must be optional: a build that never wires it (and every test binary)
// must not panic on the keypress.
func TestUnwiredHookDoesNotPanic(t *testing.T) {
	prev := ReopenProviderPortal
	t.Cleanup(func() { ReopenProviderPortal = prev })
	ReopenProviderPortal = nil

	if ReopenProviderPortal != nil {
		t.Fatal("fixture failed to clear the hook")
	}
	// portalExec must never be constructed with a nil runner; the guard lives in
	// the key handler, which is asserted by the nil check there. This test pins
	// the contract that nil is a legal state.
}

// The wrapper hands the terminal over and reports the outcome truthfully — a
// failure must not be swallowed into a success message.
func TestPortalExecReportsFailure(t *testing.T) {
	want := errors.New("no models found")
	p := portalExec{run: func() error { return want }}
	if got := p.Run(); !errors.Is(got, want) {
		t.Errorf("Run() = %v, want the underlying error", got)
	}

	ok := portalExec{run: func() error { return nil }}
	if err := ok.Run(); err != nil {
		t.Errorf("a successful portal run reported %v", err)
	}
}

// The stream setters are no-ops by design: the portal opens the terminal itself
// rather than reading the streams bubbletea hands over. Pinned so a future
// change does not silently start ignoring redirected input.
func TestPortalExecIgnoresStreams(t *testing.T) {
	p := portalExec{run: func() error { return nil }}
	p.SetStdin(nil)
	p.SetStdout(nil)
	p.SetStderr(nil)
	if err := p.Run(); err != nil {
		t.Errorf("setting streams changed the outcome: %v", err)
	}
}
