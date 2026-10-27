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
	"os"
	"path/filepath"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/pathutil"
)

// ResolveWriteTarget returns the path bytes will ACTUALLY be written to, after
// following any symlink, and whether that path lies inside a configured root.
//
// It does not refuse. This codebase has no sandbox by design — roots.go states
// it outright: "tools accept absolute paths anywhere and only consult the
// working directory to resolve relative paths and to pick a permission scope."
// Adding /add-dir roots makes a directory first-class; it does not unlock it,
// because nothing was locked.
//
// So the defect these fixes address is NOT that a write can leave the project.
// It is that the permission prompt named the wrong path — the link rather than
// its target, the source rather than the move destination. A dialog that names
// the wrong file converts the user's caution into consent. The remedy is an
// HONEST prompt, not a new refusal that would break working across roots.
func ResolveWriteTarget(dest string) (resolved string, insideRoot bool, err error) {
	if strings.TrimSpace(dest) == "" {
		return "", false, fmt.Errorf("empty destination path")
	}

	abs := dest
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(config.WorkingDirectory(), abs)
	}
	abs = filepath.Clean(abs)

	resolved, err = resolveExistingAncestor(abs)
	if err != nil {
		// Unresolvable: report the cleaned path and let the caller show it. Not
		// knowing where it lands is exactly when the user should be asked.
		return abs, false, nil
	}

	if _, ok := config.RootFor(resolved); ok {
		return resolved, true, nil
	}
	// Fall back to the primary working directory for the common single-root case.
	return resolved, withinDir(pathutil.Canonical(config.WorkingDirectory()), resolved), nil
}

// DescribeWriteTarget renders the destination for a permission prompt, naming
// the resolved location whenever it differs from what was asked for — which is
// precisely the symlink and move cases that made the old prompts dishonest.
func DescribeWriteTarget(dest string) string {
	resolved, inside, err := ResolveWriteTarget(dest)
	if err != nil {
		return dest
	}

	// Compare against the ABSOLUTE form of what was asked for, not the raw
	// string. Every relative path "resolves to" its absolute form, so comparing
	// raw would decorate ordinary prompts with a note on every single write —
	// and a warning that fires constantly is one nobody reads. Only a genuine
	// redirection (a symlink) should be called out.
	asked := dest
	if !filepath.IsAbs(asked) {
		asked = filepath.Join(config.WorkingDirectory(), asked)
	}
	// Respelled with Long first, so a short (8.3) spelling of the same file
	// is not called a redirection.
	redirected := !pathutil.Equal(resolved, pathutil.Long(asked))

	switch {
	case redirected && !inside:
		return fmt.Sprintf("%s (actually writes to %s — OUTSIDE this project)", dest, resolved)
	case redirected:
		return fmt.Sprintf("%s (actually writes to %s)", dest, resolved)
	case !inside:
		return fmt.Sprintf("%s (OUTSIDE this project)", dest)
	default:
		return dest
	}
}

// resolveExistingAncestor resolves symlinks for a path that may not exist yet,
// by walking up to the nearest ancestor that does and re-attaching the tail.
// That is what decides where the bytes actually land.
//
// GORILLA FIX (2026-10-27): resolved with pathutil.Canonical. Since Go 1.23
// filepath.EvalSymlinks no longer follows Windows junctions, which any user
// can make without privilege, so a junction in the project was named, not
// where it wrote.
func resolveExistingAncestor(abs string) (string, error) {
	for cur := abs; ; {
		if _, err := os.Lstat(cur); err == nil {
			return pathutil.Canonical(abs), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached the root without finding anything that exists.
			return "", fmt.Errorf("no existing ancestor for %s", abs)
		}
		cur = parent
	}
}

// withinDir reports whether target is dir itself or lies beneath it, with a
// separator guard, so /home/user/project-evil is not treated as inside
// /home/user/project. Both are already resolved; pathutil also expands 8.3
// short names (a working directory given as C:\Users\RUNNER~1\... and a
// target resolved to C:\Users\runneradmin\... are the same folder).
func withinDir(dir, target string) bool {
	return pathutil.WithinLexical(dir, target)
}
