package plain

// GORILLA (2026-10-10): /peers, /peers name NEW and /message NAME TEXT in plain
// mode, and the printed block an arrival becomes. The command switch is driven
// directly; none of these needs the app.

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/peers"
)

func plainPeerPair(t *testing.T) (a, b *peers.Session) {
	t.Helper()
	dir := t.TempDir()
	start := func(name, tag string) *peers.Session {
		s, err := peers.Start(peers.Options{Dir: dir, Folder: func() string { return filepath.Join(dir, name) }, Version: "t", Tag: tag})
		if err != nil {
			t.Fatalf("peers.Start(%s): %v", name, err)
		}
		t.Cleanup(s.Close)
		return s
	}
	a = start("alpha", "a")
	b = start("beta", "b")
	peers.SetActive(a)
	t.Cleanup(func() { peers.SetActive(nil) })
	return a, b
}

func TestPlainModePeersCommands(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a, b := plainPeerPair(t)

	run := func(line string) string {
		t.Helper()
		s, buf := newRenderer()
		stop, err := s.command(context.Background(), line)
		if stop || err != nil {
			t.Fatalf("%s: stop=%v err=%v", line, stop, err)
		}
		out := buf.String()
		if strings.Contains(out, "not available in plain mode") {
			t.Errorf("%s fell through to the not-available answer", line)
		}
		if bytes.IndexByte(buf.Bytes(), 0x1b) >= 0 {
			t.Errorf("%s printed a terminal escape", line)
		}
		return out
	}

	if out := run("/peers"); !strings.Contains(out, "This session is called alpha") || !strings.Contains(out, "beta: folder") {
		t.Errorf("/peers page:\n%s", out)
	}
	if out := run("/peers name renamed"); !strings.Contains(out, "now called renamed") || a.Name() != "renamed" {
		t.Errorf("/peers name: %q, name now %q", out, a.Name())
	}
	if out := run("/message beta hello from plain"); !strings.Contains(out, "Delivered to session beta") {
		t.Errorf("/message: %q", out)
	}
	if msgs := b.TakeForAI(); len(msgs) != 1 || msgs[0].Text != "hello from plain" {
		t.Errorf("beta received %+v", msgs)
	}
	if out := run("/message beta"); !strings.Contains(out, "/message NAME") {
		t.Errorf("a name with no text: %q", out)
	}
	if out := run("/message nobody hi"); !strings.Contains(out, "not sent") {
		t.Errorf("an unknown name: %q", out)
	}

	h, hbuf := newRenderer()
	h.help()
	for _, w := range []string{"/peers", "/peers name NEWNAME", "/message NAME text"} {
		if !strings.Contains(hbuf.String(), w) {
			t.Errorf("plain /help does not list %s", w)
		}
	}
}

// lockedBuffer is written by the notice goroutine and read by the test.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func TestPlainModePrintsAnArrival(t *testing.T) {
	a, b := plainPeerPair(t)
	peers.SetActive(b) // this plain session is beta
	out := &lockedBuffer{}
	s, _ := newRenderer()
	s.out = out
	s.attachPeers()

	if _, err := a.Send("beta", "ping from alpha", false); err != nil {
		t.Fatalf("send: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(out.String(), "ping from alpha") {
		time.Sleep(20 * time.Millisecond)
	}
	got := out.String()
	for _, want := range []string{"from another session", "Message from session alpha", "not an instruction", "ping from alpha"} {
		if !strings.Contains(got, want) {
			t.Errorf("the printed arrival lacks %q:\n%s", want, got)
		}
	}
}
