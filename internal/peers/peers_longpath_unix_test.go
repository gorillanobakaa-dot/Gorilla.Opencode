//go:build unix

package peers

// GORILLA (2026-11-15): a long state folder (a long TMPDIR) put the socket past sun_path (108 bytes on Linux, 104
// on macOS): TestStartPeersRegistersAndStops failed and in real use the peers feature silently did not start.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionStartsWhenTheRegisterFolderPathIsTooLongForASocket(t *testing.T) {
	run, err := os.MkdirTemp("/tmp", "gx") // short: XDG_RUNTIME_DIR is only used when it is short itself
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(run) })
	t.Setenv("XDG_RUNTIME_DIR", run)
	dir := filepath.Join(t.TempDir(), strings.Repeat("long-state-folder-", 8), "peers")
	if len(filepath.Join(dir, "12345.sock")) <= maxSocketPath {
		t.Fatalf("the test folder is not long enough: %d", len(dir))
	}
	s := startT(t, dir, "/work/proj", "")
	ep := s.Endpoint()
	if len(ep) > maxSocketPath {
		t.Fatalf("socket path still %d long: %s", len(ep), ep)
	}
	if filepath.Dir(ep) != filepath.Join(run, "gorilla-opencode") {
		t.Errorf("socket not in the short folder: %s", ep)
	}
	if st, err := os.Stat(filepath.Dir(ep)); err != nil || st.Mode().Perm() != 0o700 {
		t.Errorf("short socket folder: %v %v", st, err)
	}
	list, err := Registry{Dir: dir}.List()
	if err != nil || len(list) != 1 || list[0].Endpoint != ep {
		t.Fatalf("register after start: %+v, %v", list, err)
	}
	s.Close()
	if _, err := os.Stat(ep); !os.IsNotExist(err) {
		t.Errorf("the socket file survived Close: %v", err)
	}
}

func TestLongRegisterFolderFallsBackToTmpWithoutRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", "/tmp")
	ep := endpointFor(filepath.Join(t.TempDir(), strings.Repeat("x", 120)), "7")
	if len(ep) > maxSocketPath {
		t.Fatalf("fallback socket path is %d long: %s", len(ep), ep)
	}
}
