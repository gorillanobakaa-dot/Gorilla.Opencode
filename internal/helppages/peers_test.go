package helppages

// GORILLA (2026-10-10): the /peers page states this session's name, lists the
// others or says none are running, tells the reader how to rename, how to send,
// what a message can and cannot do, and how to switch the feature off.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/peers"
)

func TestPeersPageWithOthersRunning(t *testing.T) {
	text := pageText(PeersPage(PeersState{
		Running: true, SwitchedOn: true, Self: "proj", Folder: `C:\work\proj`,
		Others: []peers.Entry{
			{Name: "website", Folder: `C:\work\website`, Busy: true, Version: "v0.1.145"},
			{Name: "notes", Folder: `C:\notes`, Version: "v0.1.145"},
		},
		Recent: []peers.Message{
			{From: peers.Identity{Name: "website"}, At: time.Now(), Text: "release is done", Kind: peers.KindMessage},
			{From: peers.Identity{Name: "notes"}, At: time.Now(), Kind: peers.KindIdle},
		},
		Pending: 1,
	}))
	for _, want := range []string{
		"This session is called proj", `C:\work\proj`,
		"website: folder C:\\work\\website, busy", "notes: folder C:\\notes, idle",
		"from website: release is done", "finished its turn", "1 of these waits",
		"/peers name NEWNAME", "/message NAME your text",
		"not its person's instruction", "16 KB", "20 a minute",
		"Answer a permission question, switch on /yolo, or change a setting",
		"Make the other AI start working", "nothing else", "only your own account",
		"Claude Code", "/context", "Session messaging — /peers", "The row is ON now",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the /peers page does not contain %q:\n%s", want, text)
		}
	}
}

func TestPeersPageSaysNoneRunning(t *testing.T) {
	text := pageText(PeersPage(PeersState{Running: true, SwitchedOn: true, Self: "proj", Folder: "/home/u/proj"}))
	if !strings.Contains(text, "None running") {
		t.Errorf("no plain statement that no other session is running:\n%s", text)
	}
	if strings.Contains(text, "Received lately") {
		t.Error("an empty inbox still printed a 'received' section")
	}
}

func TestPeersPageWhenNotRunningSaysWhyAndHow(t *testing.T) {
	text := pageText(PeersPage(PeersState{Running: false, SwitchedOn: false}))
	for _, want := range []string{peers.NotRunning, "The row is OFF now", "/peers name NEWNAME"} {
		if !strings.Contains(text, want) {
			t.Errorf("not-running page lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Other sessions running now") {
		t.Error("a session that is not taking part listed other sessions")
	}
}

// The live page reads the real session.
func TestLivePeersPageShowsTheActiveSession(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	dir := t.TempDir()
	s, err := peers.Start(peers.Options{Dir: dir, Folder: func() string { return filepath.Join(dir, "livepage") }, Version: "t", Tag: "page"})
	if err != nil {
		t.Fatalf("peers.Start: %v", err)
	}
	t.Cleanup(s.Close)
	peers.SetActive(s)
	t.Cleanup(func() { peers.SetActive(nil) })

	p, ok := ByName("peers")
	if !ok || p.Name != "peers" {
		t.Fatalf("ByName(peers) = %q, %v", p.Name, ok)
	}
	if text := pageText(p); !strings.Contains(text, "This session is called livepage") {
		t.Errorf("the live page does not name the active session:\n%s", text)
	}
}

func TestPeersPageCodeLinesFitEightyColumns(t *testing.T) {
	for _, l := range codeLines(PeersPage(PeersState{Running: true})) {
		if w := ansi.StringWidth(l); w > CodeWidthMax {
			t.Errorf("code line is %d columns: %q", w, l)
		}
	}
}
