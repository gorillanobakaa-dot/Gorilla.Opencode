package app

// GORILLA (2026-10-10): StartPeers registers this session only when the
// /context row is on, stops cleanly, and is never reached by a one-shot -p run.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/peers"
)

func isolatePeers(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return peers.DefaultDir(config.StateBase())
}

func TestStartPeersRegistersAndStops(t *testing.T) {
	dir := isolatePeers(t)
	if !config.LoadoutEnabled(config.PeersComponentID) {
		t.Fatal("session messaging must be ON by default")
	}
	stop := (&App{}).StartPeers(nil)
	s := peers.Active()
	if s == nil {
		stop()
		t.Fatal("StartPeers left no active session")
	}
	list, err := peers.Registry{Dir: dir}.List()
	if err != nil || len(list) != 1 || list[0].PID != os.Getpid() {
		t.Errorf("register after start: %+v, %v", list, err)
	}
	stop()
	if peers.Active() != nil {
		t.Error("the session is still active after stop")
	}
	if list, _ := (peers.Registry{Dir: dir}).List(); len(list) != 0 {
		t.Errorf("the entry survived stop: %+v", list)
	}
}

func TestStartPeersDoesNothingWhenSwitchedOff(t *testing.T) {
	dir := isolatePeers(t)
	if config.LoadoutEnabled(config.PeersComponentID) {
		config.ToggleLoadout(config.PeersComponentID)
	}
	t.Cleanup(func() {
		if !config.LoadoutEnabled(config.PeersComponentID) {
			config.ToggleLoadout(config.PeersComponentID)
		}
	})
	stop := (&App{}).StartPeers(nil)
	defer stop()
	if peers.Active() != nil {
		t.Error("a session with messaging switched off became active")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		if list, _ := (peers.Registry{Dir: dir}).List(); len(list) != 0 {
			t.Errorf("a session with messaging switched off registered: %+v", list)
		}
	}
}

// A one-shot -p run returns before StartPeers is reached, and the editor mode
// starts it. Read from the source, the way the command-registry tests read the
// dispatch switch: there is no other link between the two files.
func TestOnlyInteractiveAndEditorSessionsStartPeers(t *testing.T) {
	root, err := os.ReadFile(filepath.Join("..", "..", "cmd", "root.go"))
	if err != nil {
		t.Skipf("cannot read cmd/root.go: %v", err)
	}
	src := string(root)
	oneShot := strings.Index(src, "app.RunNonInteractive(")
	start := strings.Index(src, "StartPeers(")
	if oneShot < 0 || start < 0 {
		t.Fatalf("could not find RunNonInteractive (%d) or StartPeers (%d) in cmd/root.go", oneShot, start)
	}
	if start < oneShot {
		t.Error("StartPeers runs before the -p branch returns: one-shot runs would register")
	}
	acp, err := os.ReadFile(filepath.Join("..", "..", "cmd", "acp.go"))
	if err != nil {
		t.Skipf("cannot read cmd/acp.go: %v", err)
	}
	if !strings.Contains(string(acp), "StartPeers(") {
		t.Error("editor mode (acp) does not start session messaging")
	}
}
