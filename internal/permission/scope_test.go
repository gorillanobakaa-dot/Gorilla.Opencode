package permission

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// The path a request is recorded against is the scope of an "allow for this
// session" grant. It must be exactly what the caller asked for.
//
// This was filepath.Dir(opts.Path) — the PARENT of a path callers had already
// resolved to a directory — so every grant was one level too wide.
func TestNormalisePermissionPathDoesNotWidenScope(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	for _, tc := range []struct {
		name, in, want string
	}{
		{"a workspace root is kept as-is", "/home/user/project", "/home/user/project"},
		{"a nested directory is kept as-is", "/home/user/project/sub", "/home/user/project/sub"},
		{"a sibling stays distinct from its neighbour", "/home/user/other", "/home/user/other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalisePermissionPath(tc.in); got != tc.want {
				t.Errorf("normalisePermissionPath(%q) = %q, want %q — taking the parent widens every grant by one level", tc.in, got, tc.want)
			}
		})
	}
}

// The specific consequence of the old behaviour: two sibling directories
// collapsed to the same recorded path, so a grant for one silently covered the
// other. The session-permission check compares Path exactly, so equal paths mean
// a shared grant.
func TestSiblingDirectoriesDoNotShareAGrant(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	project := normalisePermissionPath("/home/user/project")
	sibling := normalisePermissionPath("/home/user/other")

	if project == sibling {
		t.Errorf("both %q and %q record as %q — one grant would cover both", "/home/user/project", "/home/user/other", project)
	}
	// And neither may collapse to their shared parent.
	if parent := filepath.Dir("/home/user/project"); project == parent || sibling == parent {
		t.Errorf("a directory recorded as its parent %q — the grant reaches everything beside it", parent)
	}
}

// A path with no directory component must still fall back to the working
// directory rather than being recorded as "." or empty.
func TestEmptyPathFallsBackToWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	if _, err := config.Load(dir, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prev := cfg.WorkingDir
	cfg.WorkingDir = dir
	t.Cleanup(func() { cfg.WorkingDir = prev })

	for _, in := range []string{"", "."} {
		if got := normalisePermissionPath(in); got != dir {
			t.Errorf("normalisePermissionPath(%q) = %q, want the working directory %q", in, got, dir)
		}
	}
}

// A session grant must still cover the directory it was given for, or every edit
// re-prompts and "allow for this session" means nothing.
//
// Request BLOCKS on the user when no grant matches, so this runs it in a
// goroutine with a deadline. Asserting on a bare Request call would HANG rather
// than fail when the scoping regresses — which is exactly what happened while
// verifying this test against the old behaviour, and a hanging test is worse than
// a failing one because CI reports a timeout instead of the reason.
func TestSessionGrantCoversTheDirectoryItWasGivenFor(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	const session = "s1"
	project := "/home/user/project"

	s := NewPermissionService().(*permissionService)
	s.sessionPermissions = append(s.sessionPermissions, PermissionRequest{
		SessionID: session,
		ToolName:  "edit",
		Action:    "write",
		Path:      project,
	})

	granted := make(chan bool, 1)
	go func() {
		granted <- s.Request(CreatePermissionRequest{
			SessionID: session,
			ToolName:  "edit",
			Action:    "write",
			Path:      project,
		})
	}()

	select {
	case ok := <-granted:
		if !ok {
			t.Error("a second edit scoped to the same directory was denied despite the session grant")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Request blocked waiting for a user decision — the session grant did not match the directory it was given for, so allow-for-session would re-prompt on every edit")
	}
}
