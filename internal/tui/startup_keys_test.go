package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
	"github.com/opencode-ai/opencode/internal/tui/page"
)

// idleCoder is never busy; the editor's send() asks nothing else of it.
type idleCoder struct{ agent.Service }

func (idleCoder) IsSessionBusy(string) bool { return false }
func (idleCoder) IsBusy() bool              { return false }

// startupRun is the program as it is ~10 ms after launch: sized, and about to
// get the "Initialize Project" offer that Init sends on a project without the
// flag file. Its clock is the test's. As in bubbletea, every command runs on its
// own goroutine and its message comes back through the event loop, so keys that
// were queued together reach the program together, ahead of any timer. Every
// message is kept so the test can see what would have been sent.
type startupRun struct {
	t    *testing.T
	a    appModel
	now  time.Time
	out  chan tea.Msg
	seen []tea.Msg
}

func newStartupRun(t *testing.T) *startupRun {
	r := &startupRun{t: t, now: time.Date(2026, 10, 25, 19, 0, 0, 0, time.UTC), out: make(chan tea.Msg, 4096)}
	r.a = *New(&app.App{CoderAgent: idleCoder{}}).(*appModel)
	r.a.now = func() time.Time { return r.now }
	r.send(tea.WindowSizeMsg{Width: 120, Height: 30})
	return r
}

func (r *startupRun) deliver(m tea.Msg) {
	r.seen = append(r.seen, m)
	if _, ok := m.(chat.SendMsg); ok {
		return // it would reach the model; kept for sent(), not delivered
	}
	next, cmd := r.a.Update(m)
	r.a = next.(appModel)
	go runCmd(cmd, r.out)
}

// runCmd executes a command the way bubbletea does, batches included. The
// cursor blink re-arms itself forever and changes nothing here: not followed.
func runCmd(cmd tea.Cmd, out chan<- tea.Msg) {
	if cmd == nil {
		return
	}
	switch m := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range m {
			go runCmd(c, out)
		}
	default:
		if !strings.HasPrefix(fmt.Sprintf("%T", m), "cursor.") {
			out <- m
		}
	}
}

// settle runs the event loop until nothing has arrived for a while.
func (r *startupRun) settle() {
	for n := 0; ; n++ {
		if n > 5000 {
			r.t.Fatal("the program never went quiet")
		}
		select {
		case m := <-r.out:
			r.deliver(m)
		case <-time.After(300 * time.Millisecond):
			return
		}
	}
}

// send delivers msg and then everything it leads to.
func (r *startupRun) send(msg tea.Msg) {
	r.deliver(msg)
	r.settle()
}

// keys delivers text as the Windows console does: one KeyMsg per character, every
// line break a plain Enter, all queued at the same instant (a paste, or typing
// that was waiting while the program started).
func (r *startupRun) keys(text string) {
	for _, c := range text {
		if c == '\n' {
			r.deliver(tea.KeyMsg{Type: tea.KeyEnter})
		} else {
			r.deliver(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c}})
		}
	}
	r.settle()
}

// boxText is what the chat box holds.
func (r *startupRun) boxText() string {
	return r.a.pages[page.ChatPage].(interface{ EditorText() string }).EditorText()
}

// sent lists what would have left the box: chat messages, and an answer that
// runs the project initializer (which sends a prompt to the model).
func (r *startupRun) sent() []string {
	var out []string
	for _, m := range r.seen {
		switch m := m.(type) {
		case chat.SendMsg:
			out = append(out, "message "+fmt.Sprintf("%q", m.Text))
		case dialog.CloseInitDialogMsg:
			if m.Initialize {
				out = append(out, "initialize project")
			}
		}
	}
	return out
}

// Fieldkit chat_paste_probe.py, 2026-10-25: the first 35 characters of a paste
// made right after start-up never reached the chat box. The program read all of
// them; the "Initialize Project" offer, open ~10 ms after launch and drawn above
// the box, took every key until the "n" of "Screens" answered it No. A "y" or an
// Enter (its default is Yes) would have run the initializer and sent a prompt.
// Keys that arrive while the offer is coming up were typed for the chat.
func TestKeysTypedAsTheChatComesUpAllLandInTheBoxAndSendNothing(t *testing.T) {
	r := newStartupRun(t)
	r.send(dialog.ShowInitDialogMsg{Show: true}) // Init's offer, before any key
	text := "yes, look at this\nand this too\n"
	r.keys(text)
	r.send(tea.WindowSizeMsg{Width: 120, Height: 30}) // any later event: the box is drawn

	if got := r.sent(); len(got) != 0 {
		t.Fatalf("keys typed during start-up sent something: %v", got)
	}
	if r.a.showInitDialog {
		t.Error("the offer is still open and would take the next key")
	}
	if got, want := r.boxText(), text; got != want {
		t.Fatalf("the box holds %q, want every key in order %q", got, want)
	}
}

// Typing that began before the offer arrived: it is not shown at all.
func TestTheOfferDoesNotOpenOverTypingThatHasBegun(t *testing.T) {
	r := newStartupRun(t)
	r.keys("hi")
	r.send(dialog.ShowInitDialogMsg{Show: true})
	if r.a.showInitDialog {
		t.Fatal("the offer opened over a box already being typed in")
	}
	r.keys(" there")
	r.send(tea.WindowSizeMsg{Width: 120, Height: 30})
	if got := r.boxText(); got != "hi there" {
		t.Fatalf("the box holds %q", got)
	}
}

// Typing at a person's pace, one key landing just as the offer appears: it was
// on its way before the offer could be read, so it is not an answer.
func TestAKeyTypedAsTheOfferAppearsIsNotAnAnswer(t *testing.T) {
	r := newStartupRun(t)
	r.send(dialog.ShowInitDialogMsg{Show: true})
	r.now = r.now.Add(100 * time.Millisecond)
	r.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}) // then a pause
	if got := r.sent(); len(got) != 0 {
		t.Fatalf("a key typed as the offer appeared answered it: %v", got)
	}
	if got := r.boxText(); got != "y" || r.a.showInitDialog {
		t.Fatalf("box %q, offer open %v; want the key in the box and the offer put away", got, r.a.showInitDialog)
	}
}

// A paste into an offer that has been on screen for a while is still a paste:
// its first key waits to see whether more follow, and they do.
func TestAPasteIntoTheOpenOfferGoesToTheBox(t *testing.T) {
	r := newStartupRun(t)
	r.send(dialog.ShowInitDialogMsg{Show: true})
	r.now = r.now.Add(10 * time.Second)
	r.keys("no\nyes\n")
	r.send(tea.WindowSizeMsg{Width: 120, Height: 30})
	if got := r.sent(); len(got) != 0 {
		t.Fatalf("the paste answered the offer or sent: %v", got)
	}
	if got := r.boxText(); got != "no\nyes\n" {
		t.Fatalf("the box holds %q", got)
	}
}

// The offer still works for someone who reads it and answers.
func TestADeliberateAnswerStillAnswersTheOffer(t *testing.T) {
	for _, tc := range []struct {
		key  tea.KeyMsg
		init bool
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}, true},
		{tea.KeyMsg{Type: tea.KeyEnter}, true}, // Yes is preselected
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, false},
	} {
		r := newStartupRun(t)
		r.send(dialog.ShowInitDialogMsg{Show: true})
		r.now = r.now.Add(2 * time.Second) // read it, then pressed one key
		r.send(tc.key)
		var answered, initialize bool
		for _, m := range r.seen {
			if c, ok := m.(dialog.CloseInitDialogMsg); ok {
				answered, initialize = true, c.Initialize
			}
		}
		if !answered || initialize != tc.init || r.a.showInitDialog {
			t.Errorf("%q: answered=%v initialize=%v open=%v, want an answer initialize=%v",
				tc.key.String(), answered, initialize, r.a.showInitDialog, tc.init)
		}
		if got := r.boxText(); got != "" {
			t.Errorf("%q: the answer also went into the box: %q", tc.key.String(), got)
		}
	}
}
