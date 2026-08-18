package tools

// GORILLA OVERRIDE (2026-08-18): tests for honest write destinations.
//
// The defect these exist for is CONSENT INTEGRITY, not sandboxing. This codebase
// has no sandbox by design — roots.go says so outright — so the fix is not a new
// refusal (which would break working across /add-dir roots) but a permission
// prompt that names the path the bytes actually land on.
//
// Two ways the old prompts lied:
//   - patch "*** Move to:" named the SOURCE while writing to the destination;
//   - write/edit named a SYMLINK while writing to its target.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// A symlink inside the workspace pointing elsewhere must be reported by its
// TARGET, and flagged as outside. Naming the link is the lie.
func TestASymlinkIsDescribedByWhereItActuallyGoes(t *testing.T) {
	wd := config.WorkingDirectory()
	if wd == "" {
		t.Skip("no workspace")
	}
	if err := os.MkdirAll(wd, 0o755); err != nil {
		t.Skipf("workspace unavailable: %v", err)
	}
	outside := t.TempDir()

	link := filepath.Join(wd, "innocent-link")
	_ = os.Remove(link)
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	defer os.Remove(link)

	dest := filepath.Join("innocent-link", "payload.txt")

	resolved, inside, err := ResolveWriteTarget(dest)
	if err != nil {
		t.Fatalf("ResolveWriteTarget: %v", err)
	}
	if !strings.HasPrefix(resolved, outside) {
		t.Errorf("the symlink was not followed: %q resolved to %q, expected under %q", dest, resolved, outside)
	}
	if inside {
		t.Error("a path resolving outside every root was reported as inside")
	}

	desc := DescribeWriteTarget(dest)
	if !strings.Contains(desc, outside) {
		t.Errorf("the prompt would not show the real destination: %q", desc)
	}
	if !strings.Contains(desc, "OUTSIDE") {
		t.Errorf("the prompt does not warn that it leaves the project: %q", desc)
	}
}

// Ordinary in-workspace paths must be described plainly — no scary noise on the
// overwhelmingly common case, or the warning stops meaning anything.
func TestOrdinaryPathsAreDescribedPlainly(t *testing.T) {
	wd := config.WorkingDirectory()
	if wd == "" {
		t.Skip("no workspace")
	}
	_ = os.MkdirAll(wd, 0o755)

	for _, dest := range []string{"notes.md", "sub/dir/notes.md", "./notes.md"} {
		desc := DescribeWriteTarget(dest)
		if strings.Contains(desc, "OUTSIDE") || strings.Contains(desc, "resolves to") {
			t.Errorf("an ordinary in-workspace path was decorated with a warning: %q -> %q", dest, desc)
		}
	}
}

// CAPABILITY GUARD. This is a coding agent: moving a file inside the project is
// ordinary work. Every one of these must resolve inside a root, so nothing here
// is ever flagged or impeded.
func TestLegitimateInWorkspaceDestinationsStayInside(t *testing.T) {
	wd := config.WorkingDirectory()
	if wd == "" {
		t.Skip("no workspace")
	}
	_ = os.MkdirAll(filepath.Join(wd, "pkg"), 0o755)

	for _, dest := range []string{
		"pkg/greet.go",
		"renamed.go",
		"./also-fine.go",
		"a/b/c/deep.go",
		filepath.Join(wd, "abs.go"),
	} {
		_, inside, err := ResolveWriteTarget(dest)
		if err != nil {
			t.Errorf("REGRESSION: legitimate destination errored: %q -> %v", dest, err)
			continue
		}
		if !inside {
			t.Errorf("REGRESSION: legitimate in-workspace destination reported OUTSIDE: %q", dest)
		}
	}
}

// An empty destination is an error, not a silent pass.
func TestEmptyDestinationIsAnError(t *testing.T) {
	if _, _, err := ResolveWriteTarget("   "); err == nil {
		t.Error("an empty destination was accepted")
	}
}

// Sibling directories that merely share a name prefix are not inside.
func TestPrefixNeighboursAreNotInside(t *testing.T) {
	if withinDir("/home/u/project", "/home/u/project-evil/x") {
		t.Error("a sibling sharing a name prefix was treated as inside")
	}
	if !withinDir("/home/u/project", "/home/u/project/sub/x") {
		t.Error("a genuine child was treated as outside")
	}
}
