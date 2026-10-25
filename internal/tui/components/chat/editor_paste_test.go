package chat

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/llm/agent"
)

// idleAgent is never busy; send() asks nothing else of it.
type idleAgent struct{ agent.Service }

func (idleAgent) IsSessionBusy(string) bool { return false }

// pasteEditor is a focused chat editor whose clock the test moves and whose
// Enter-settle timer the test fires, so timing is exact and nothing sleeps.
func pasteEditor() (m *editorCmp, clock *time.Time, settled *[]tea.Msg) {
	t0 := time.Date(2026, 10, 25, 14, 0, 0, 0, time.UTC)
	clock = &t0
	settled = &[]tea.Msg{}
	m = &editorCmp{app: &app.App{CoderAgent: idleAgent{}}}
	m.textarea = CreateTextArea(nil)
	m.textarea.Focus()
	m.SetSize(120, 1)
	m.now = func() time.Time { return *clock }
	m.settleAfter = func(_ time.Duration, msg tea.Msg) tea.Cmd {
		*settled = append(*settled, msg)
		return nil
	}
	return m, clock, settled
}

// consoleKeys is a paste as bubbletea v1 delivers it from the Windows console
// (key_windows.go): one KeyMsg per character, Paste false, and every line
// break a plain Enter. Explorer's "Copy as path" separates paths with CRLF and
// the console turns each CRLF into one Enter.
func consoleKeys(text string) []tea.KeyMsg {
	var keys []tea.KeyMsg
	for _, r := range strings.ReplaceAll(text, "\r\n", "\r") {
		if r == '\r' {
			keys = append(keys, tea.KeyMsg{Type: tea.KeyEnter})
		} else {
			keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	return keys
}

func copiedPaths(n int) string {
	paths := make([]string, n)
	for i := range paths {
		paths[i] = fmt.Sprintf(`"C:\Users\gorilla1\Pictures\Screenshots\Screenshot 2026-10-17 %06d.png"`, i)
	}
	return strings.Join(paths, "\r\n")
}

// Owner's report, 2026-10-25: ~200 paths pasted into the chat box were sent in
// batches, a bare path per message, while the rest stayed queued in the box.
// A paste must land whole and send nothing until Enter is pressed.
func TestAConsolePasteSendsNothingAndLandsWhole(t *testing.T) {
	m, clock, settled := pasteEditor()
	text := copiedPaths(200)

	// The whole paste is queued at once: every key arrives at the same instant.
	start := time.Now()
	for _, k := range consoleKeys(text) {
		m.Update(k)
	}
	// Fed to the textarea one key at a time this took 187 s (it re-hashes
	// every line per key); buffered it takes well under a second.
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("15K pasted keys took %s to process; the paste would land minutes late", took.Round(time.Millisecond))
	}
	// Then the paste is over and every waiting timer fires.
	*clock = clock.Add(time.Second)
	for _, msg := range *settled {
		m.Update(msg)
	}
	*settled = nil

	if len(m.history) != 0 {
		t.Fatalf("the paste sent %d message(s) by itself, the first %q", len(m.history), m.history[0])
	}
	want := strings.ReplaceAll(text, "\r\n", "\n")
	if got := m.textarea.Value(); got != want {
		t.Fatalf("the box does not hold the paste as pasted: %d lines / %d chars, want 200 lines / %d chars",
			strings.Count(got, "\n")+1, len(got), len(want))
	}

	// One Enter, pressed a second later and followed by silence, sends it all.
	*clock = clock.Add(time.Second)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.history) != 0 {
		t.Fatal("Enter sent before its settle time; it cannot yet know it was not pasted")
	}
	for _, msg := range *settled {
		m.Update(msg)
	}
	if len(m.history) != 1 || m.history[0] != want {
		t.Fatalf("the pressed Enter did not send the paste as one message (%d sent)", len(m.history))
	}
	if m.textarea.Value() != "" {
		t.Error("the box was not emptied after sending")
	}
}

// A paste can arrive in pieces (the console reader takes 16 records at a time),
// so a line break can be the LAST key of one piece, with a pause before the
// next. The key that follows still turns it into a line break.
func TestAnEnterFollowedByMoreKeysIsALineBreak(t *testing.T) {
	m, clock, settled := pasteEditor()
	for _, k := range consoleKeys("first line") {
		m.Update(k)
	}
	*clock = clock.Add(20 * time.Millisecond) // the piece ended; Enter comes alone
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	*clock = clock.Add(5 * time.Millisecond) // next piece, before the Enter settled
	for _, k := range consoleKeys("second line") {
		m.Update(k)
	}
	*clock = clock.Add(time.Second)
	for _, msg := range *settled { // the stale timer fires late
		m.Update(msg)
	}

	if len(m.history) != 0 {
		t.Fatalf("a line break inside a paste was sent: %q", m.history[0])
	}
	if got := m.textarea.Value(); got != "first line\nsecond line" {
		t.Fatalf("got %q", got)
	}
}

// Typing stays as it was: a word, a human pause, Enter, and it is sent.
func TestATypedEnterStillSends(t *testing.T) {
	m, clock, settled := pasteEditor()
	for _, k := range consoleKeys("hello") {
		*clock = clock.Add(120 * time.Millisecond)
		m.Update(k)
	}
	*clock = clock.Add(150 * time.Millisecond)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for _, msg := range *settled {
		m.Update(msg)
	}
	if len(m.history) != 1 || m.history[0] != "hello" {
		t.Fatalf("typed Enter did not send: history %q, box %q", m.history, m.textarea.Value())
	}
}
