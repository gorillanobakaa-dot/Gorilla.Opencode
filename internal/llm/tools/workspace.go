// GORILLA OVERRIDE (2026-08-18): workspace containment for write destinations.
//
// # WHY THIS EXISTS
//
// The patch tool's "*** Move to:" directive wrote content to a path taken
// straight from the patch text and then deleted the original
// (internal/diff/patch.go:638-644), while the permission prompt named only the
// SOURCE file. So a patch reading
//
//	*** Update File: README.md
//	*** Move to: /home/gorilla/.bashrc
//
// asked "Update file README.md?" and, on approval, wrote model-chosen content to
// the user's shell profile. MovePath was never validated: it appeared in the
// parser, the plumbing and the write, and nowhere else in the codebase.
//
// A permission dialog that names the wrong path is worse than no dialog at all,
// because it converts the user's caution into consent.
//
// # WHY SYMLINKS ARE RESOLVED FIRST
//
// A containment check that compares strings is defeated by a symlink: a link
// inside the workspace pointing at /etc or ~/.ssh passes a prefix test while the
// bytes land outside. EvalSymlinks resolves what will ACTUALLY be written. For a
// destination that does not exist yet — the normal case for a move — the nearest
// existing ancestor is resolved instead, because that is the directory the file
// will be created in.
package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
)

// ensureInsideWorkspace reports an error if dest resolves outside the working
// directory. Relative paths are taken as relative to the workspace, which is
// what every other tool here assumes.
//
// It fails CLOSED: if the path cannot be resolved, it is refused rather than
// allowed, because an unresolvable destination is not evidence of safety.
func ensureInsideWorkspace(dest string) error {
	wd := config.WorkingDirectory()
	if strings.TrimSpace(dest) == "" {
		return fmt.Errorf("refusing an empty destination path")
	}

	abs := dest
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(wd, abs)
	}
	abs = filepath.Clean(abs)

	realDest, err := resolveExistingAncestor(abs)
	if err != nil {
		return fmt.Errorf("refusing to write to %s: its location could not be resolved (%v)", dest, err)
	}
	realWD, err := filepath.EvalSymlinks(wd)
	if err != nil {
		realWD = filepath.Clean(wd)
	}

	if !withinDir(realWD, realDest) {
		return fmt.Errorf(
			"refusing to write to %s: it resolves to %s, which is outside this project (%s). "+
				"A patch may not move a file out of the workspace",
			dest, realDest, realWD)
	}
	return nil
}

// resolveExistingAncestor resolves symlinks for a path that may not exist yet,
// by walking up to the nearest ancestor that does and re-attaching the tail.
// That is what decides where the bytes actually land.
func resolveExistingAncestor(abs string) (string, error) {
	tail := ""
	cur := abs
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			if tail == "" {
				return resolved, nil
			}
			return filepath.Join(resolved, tail), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached the root without finding anything that exists.
			return "", fmt.Errorf("no existing ancestor for %s", abs)
		}
		tail = filepath.Join(filepath.Base(cur), tail)
		cur = parent
	}
}

// withinDir reports whether target is dir itself or lies beneath it. Compared on
// cleaned paths with a separator guard, so /home/user/project-evil is not
// treated as inside /home/user/project.
func withinDir(dir, target string) bool {
	dir = filepath.Clean(dir)
	target = filepath.Clean(target)
	if target == dir {
		return true
	}
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
