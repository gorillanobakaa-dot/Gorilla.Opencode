package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/pathutil"
)

// TestDeletedCwdNativeCommandReportsTheRealFailure: the session's folder is
// deleted by another process while the shell sits in it. Windows PowerShell
// 5.1 then starts no native program and sets no $LASTEXITCODE, and the
// wrapper used to report "no output (exit code 0)": a build that never ran
// looked like a build that passed. The result must fail, name the folder,
// and the session must move to a folder that exists, so the next command
// runs.
func TestDeletedCwdNativeCommandReportsTheRealFailure(t *testing.T) {
	s := newTestShell(t)
	gone := filepath.Join(t.TempDir(), "will-be-deleted")
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := exec1(t, s, "cd '"+gone+"'"); code != 0 {
		t.Fatalf("cd into %s failed: %d %q", gone, code, errOut)
	}
	if !pathutil.Same(s.Cwd(), gone) {
		t.Fatalf("session cwd = %q, want %q", s.Cwd(), gone)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatalf("could not delete the session folder from outside: %v", err)
	}

	native := "git --version"
	if !s.isPowerShell {
		native = "ls"
	}
	out, errOut, code := exec1(t, s, native)
	if code == 0 {
		t.Fatalf("a native command in a deleted folder reported success: out %q, err %q", out, errOut)
	}
	if !strings.Contains(errOut, "will-be-deleted") || !strings.Contains(strings.ToLower(errOut), "no longer exists") {
		t.Errorf("the failure does not say the folder is gone: %q", errOut)
	}
	if _, err := os.Stat(s.Cwd()); err != nil {
		t.Errorf("session cwd %q was not recovered to a folder that exists: %v", s.Cwd(), err)
	}

	// The next command runs normally in the recovered folder.
	out, errOut, code = exec1(t, s, "echo recovered")
	if code != 0 || !strings.Contains(out, "recovered") {
		t.Errorf("the session did not recover: code %d, out %q, err %q", code, out, errOut)
	}
}

// TestDeletedCwdRecoveryPicksTheNearestExistingParent: the session moves to
// the nearest folder above the deleted one that still exists, not to some
// unrelated place.
func TestDeletedCwdRecoveryPicksTheNearestExistingParent(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if got := nearestExistingDir(deep); !pathutil.Same(got, root) {
		t.Errorf("nearestExistingDir(%q) = %q, want %q", deep, got, root)
	}
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := nearestExistingDir(deep); !pathutil.Same(got, filepath.Join(root, "a")) {
		t.Errorf("nearestExistingDir(%q) = %q, want %q", deep, got, filepath.Join(root, "a"))
	}
}
