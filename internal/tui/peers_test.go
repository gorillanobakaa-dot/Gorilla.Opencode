package tui

// GORILLA (2026-10-10): /peers, /peers name NEW and /message NAME TEXT in the
// full interface, driven through Update with the message the editor sends, and
// the notice an arrival becomes.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/peers"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

func infoOf(t *testing.T, cmd tea.Cmd) util.InfoMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command returned")
	}
	m, ok := cmd().(util.InfoMsg)
	if !ok {
		t.Fatalf("the command produced %T, want a notice", cmd())
	}
	return m
}

func TestPeersCommandsInTheFullInterface(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	start := func(name, tag string) *peers.Session {
		s, err := peers.Start(peers.Options{Dir: dir, Folder: func() string { return filepath.Join(dir, name) }, Version: "t", Tag: tag})
		if err != nil {
			t.Fatalf("peers.Start(%s): %v", name, err)
		}
		t.Cleanup(s.Close)
		return s
	}
	a := start("alpha", "a")
	b := start("beta", "b")
	peers.SetActive(a)
	t.Cleanup(func() { peers.SetActive(nil) })

	// /peers opens the live page.
	next, _ := appModel{width: 100, height: 30}.Update(chat.SlashCommandMsg{Name: "peers"})
	got := next.(appModel)
	if !got.showInfoPage || got.infoPage.Page().Name != "peers" {
		t.Fatalf("/peers did not open its page")
	}
	var page strings.Builder
	for _, l := range got.infoPage.Page().Lines {
		page.WriteString(l.Text + "\n")
	}
	for _, want := range []string{"This session is called alpha", "beta: folder"} {
		if !strings.Contains(page.String(), want) {
			t.Errorf("the page lacks %q:\n%s", want, page.String())
		}
	}

	// /peers name NEW renames this session.
	_, cmd := appModel{}.Update(chat.SlashCommandMsg{Name: "peers", Args: "name renamed"})
	if m := infoOf(t, cmd); m.Type != util.InfoTypeInfo || !strings.Contains(m.Msg, "renamed") {
		t.Errorf("rename notice %+v", m)
	}
	if a.Name() != "renamed" {
		t.Errorf("the session is called %q after /peers name renamed", a.Name())
	}
	_, cmd = appModel{}.Update(chat.SlashCommandMsg{Name: "peers", Args: "name beta"})
	if m := infoOf(t, cmd); m.Type != util.InfoTypeError {
		t.Errorf("renaming onto a live session's name was not refused: %+v", m)
	}

	// /message NAME TEXT sends from the person.
	_, cmd = appModel{}.Update(chat.SlashCommandMsg{Name: "message", Args: "beta hello there"})
	if m := infoOf(t, cmd); m.Type != util.InfoTypeInfo || !strings.Contains(m.Msg, "Delivered to session beta") {
		t.Errorf("send notice %+v", m)
	}
	if msgs := b.TakeForAI(); len(msgs) != 1 || msgs[0].Text != "hello there" || msgs[0].From.Name != "renamed" {
		t.Errorf("beta received %+v", msgs)
	}

	// Mistakes are explained, not sent.
	_, cmd = appModel{}.Update(chat.SlashCommandMsg{Name: "message", Args: "beta"})
	if m := infoOf(t, cmd); m.Type != util.InfoTypeWarn || !strings.Contains(m.Msg, "/message NAME") {
		t.Errorf("a name with no text: %+v", m)
	}
	_, cmd = appModel{}.Update(chat.SlashCommandMsg{Name: "message", Args: "nobody hi"})
	if m := infoOf(t, cmd); m.Type != util.InfoTypeError || !strings.Contains(m.Msg, "beta") {
		t.Errorf("an unknown name: %+v", m)
	}

	// Not running: said plainly.
	peers.SetActive(nil)
	_, cmd = appModel{}.Update(chat.SlashCommandMsg{Name: "message", Args: "beta hi"})
	if m := infoOf(t, cmd); !strings.Contains(m.Msg, "not running") {
		t.Errorf("with messaging off: %+v", m)
	}
}

func TestAPeerArrivalIsANoticeThatReachesTheTranscript(t *testing.T) {
	n := peerNotice(peers.Message{From: peers.Identity{Name: "beta", Folder: "f"}, At: time.Now(), Text: "build finished", Kind: peers.KindMessage})
	if !n.Echo {
		t.Error("the notice is not echoed into the conversation; the status bar alone cuts it")
	}
	if n.Type != util.InfoTypeWarn {
		t.Errorf("notice type %v, want a warning so it stands out", n.Type)
	}
	for _, want := range []string{"beta", "build finished", "not an instruction"} {
		if !strings.Contains(n.Msg, want) {
			t.Errorf("notice lacks %q: %s", want, n.Msg)
		}
	}
}
