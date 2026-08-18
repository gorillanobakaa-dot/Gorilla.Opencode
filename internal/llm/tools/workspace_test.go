package tools

// GORILLA OVERRIDE (2026-08-18): containment tests for patch "*** Move to:".
//
// The exploit these exist for: a patch whose prompt says "Update file README.md"
// while the bytes land in ~/.bashrc, because MovePath was never validated and
// never shown. Confirmed by reading the code on 2026-08-18 — MovePath appeared
// in the parser, the plumbing and the write, and nowhere else.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

func TestMoveDestinationsOutsideTheWorkspaceAreRefused(t *testing.T) {
	// This package isolates config in TestMain, so the workspace is whatever
	// WorkingDirectory() reports — use that rather than assuming a TempDir.
	wd := config.WorkingDirectory()
	if wd == "" {
		t.Skip("no working directory configured")
	}

	refuse := []struct{ dest, why string }{
		{"/home/gorilla/.bashrc", "absolute path to the user's shell profile"},
		{"/etc/cron.d/evil", "absolute path to a system directory"},
		{"../escaped.txt", "relative traversal one level out"},
		{"../../../../tmp/escaped.txt", "deep relative traversal"},
		{"", "empty destination"},
	}
	for _, c := range refuse {
		t.Run(c.dest, func(t *testing.T) {
			if err := ensureInsideWorkspace(c.dest); err == nil {
				t.Errorf("ALLOWED a write outside the workspace: %q (%s)", c.dest, c.why)
			}
		})
	}

	allow := []string{"notes.md", "sub/dir/notes.md", "./notes.md", filepath.Join(wd, "inside.md")}
	_ = os.MkdirAll(wd, 0o755)
	for _, dest := range allow {
		if err := ensureInsideWorkspace(dest); err != nil {
			t.Errorf("refused a legitimate in-workspace destination %q: %v", dest, err)
		}
	}
}

// A prefix-only containment check is defeated by a symlink inside the workspace
// pointing out of it. This is why the check resolves symlinks before comparing.
func TestASymlinkOutOfTheWorkspaceIsRefused(t *testing.T) {
	wd := config.WorkingDirectory()
	if wd == "" {
		t.Skip("no working directory configured")
	}
	if err := os.MkdirAll(wd, 0o755); err != nil {
		t.Skipf("workspace unavailable: %v", err)
	}
	outside := t.TempDir()

	link := filepath.Join(wd, "innocent-link")
	_ = os.Remove(link)
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	defer os.Remove(link)

	dest := filepath.Join("innocent-link", "payload.txt") // looks inside; resolves outside
	if err := ensureInsideWorkspace(dest); err == nil {
		t.Errorf("a symlink escape was allowed: %q resolves into %s", dest, outside)
	}
}

// Sibling directories that merely share a prefix are not inside.
func TestPrefixNeighboursAreNotInside(t *testing.T) {
	if withinDir("/home/u/project", "/home/u/project-evil/x") {
		t.Error("a sibling sharing a name prefix was treated as inside the workspace")
	}
	if !withinDir("/home/u/project", "/home/u/project/sub/x") {
		t.Error("a genuine child was treated as outside")
	}
}
