package chat

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
)

func historyEditor(sent ...string) *editorCmp {
	ta := textarea.New()
	ta.Focus()
	m := &editorCmp{textarea: ta}
	for _, s := range sent {
		m.textarea.SetValue(s)
		m.rememberSent(s)
		m.textarea.Reset()
	}
	return m
}

// Up recalls the previous message, like every shell since 1978.
func TestUpRecallsPreviousMessagesNewestFirst(t *testing.T) {
	m := historyEditor("first", "second", "third")

	for _, want := range []string{"third", "second", "first"} {
		if !m.recallHistory(true) {
			t.Fatalf("Up was not handled while walking back to %q", want)
		}
		if got := m.textarea.Value(); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
	// At the oldest, Up stays put rather than falling through to the textarea.
	if !m.recallHistory(true) {
		t.Error("Up at the oldest entry was not swallowed; it would move the cursor")
	}
	if got := m.textarea.Value(); got != "first" {
		t.Errorf("moved past the oldest entry: %q", got)
	}
}

// THE property that makes it feel native rather than hijacked: a half-typed
// line is not destroyed by browsing away from it.
func TestBrowsingAwayAndBackReturnsTheDraft(t *testing.T) {
	m := historyEditor("earlier message")
	m.textarea.SetValue("half typed thing")

	m.recallHistory(true)
	if got := m.textarea.Value(); got != "earlier message" {
		t.Fatalf("Up did not recall: %q", got)
	}
	m.recallHistory(false)
	if got := m.textarea.Value(); got != "half typed thing" {
		t.Errorf("the draft was lost; got %q", got)
	}
}

// Up must still MOVE THE CURSOR inside a multi-line message. Taking it over
// unconditionally would make multi-line editing impossible.
func TestUpMovesTheCursorWhenThereIsALineAbove(t *testing.T) {
	m := historyEditor("some history")
	m.textarea.SetValue("line one\nline two")

	if m.textarea.Line() == 0 {
		t.Skip("textarea reports line 0 for multi-line input; cursor model differs")
	}
	if m.recallHistory(true) {
		t.Error("Up was hijacked while the cursor had a line above it — this would " +
			"make it impossible to edit the top line of a multi-line message")
	}
	if got := m.textarea.Value(); got != "line one\nline two" {
		t.Errorf("the message was replaced: %q", got)
	}
}

// Consecutive duplicates collapse, as in a shell.
func TestRepeatedSendsAreStoredOnce(t *testing.T) {
	m := historyEditor("same", "same", "same")
	if len(m.history) != 1 {
		t.Errorf("history holds %d entries for one repeated message: %v",
			len(m.history), m.history)
	}
}

// Blank sends are not history.
func TestWhitespaceIsNotRemembered(t *testing.T) {
	m := historyEditor("real message", "   ", "\n\n")
	if len(m.history) != 1 {
		t.Errorf("whitespace entered the history: %v", m.history)
	}
}

// Down with nothing to go forward to must fall through to the textarea rather
// than being swallowed.
func TestDownIsNotHijackedWhenNotBrowsing(t *testing.T) {
	m := historyEditor("something")
	if m.recallHistory(false) {
		t.Error("Down was swallowed while not browsing history")
	}
}
