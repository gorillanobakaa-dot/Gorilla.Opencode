// GORILLA OVERRIDE: this file did not exist upstream. It implements multiple
// workspace roots (/add-dir) and repointing the primary one (/cd).
//
// Design note — why primary + extras rather than a flat []string:
// 31 call sites resolve relative paths against config.WorkingDirectory(). If the
// primary root were ambiguous, "edit foo.go" would have no single correct
// answer. WorkingDir therefore stays THE root for relative-path resolution and
// AdditionalDirs extends four other behaviours: which context files load, how
// permissions are scoped, what the env block advertises, and which directories
// LSP watches.
//
// This does NOT grant new access. There is no sandbox in this codebase — tools
// accept absolute paths anywhere and only consult the working directory to
// resolve relative paths and to pick a permission scope. Adding a root makes a
// directory a first-class part of the workspace; it does not unlock it.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// containsPath reports whether path is root itself or lies beneath it.
//
// Uses filepath.Rel rather than strings.HasPrefix. HasPrefix is wrong for paths:
// with root "/tmp/foo" it happily matches "/tmp/foobar/x.go", because the string
// prefix ignores the component boundary. That bug is live in the tree today at
// edit.go and write.go, which is why permission scoping there can attribute a
// file to a sibling directory's root.
func containsPath(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// normaliseDir expands "~", makes the path absolute, resolves symlinks, and
// verifies it is an existing directory. Symlink resolution matters: without it
// the same directory reached by two paths would register as two roots, and
// containsPath would fail to match files opened through the other name.
func normaliseDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("no directory given")
	}

	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		dir = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(dir, "~"), "/"))
	}

	if !filepath.IsAbs(dir) {
		base := "."
		if cfg != nil && cfg.WorkingDir != "" {
			base = cfg.WorkingDir
		}
		dir = filepath.Join(base, dir)
	}
	dir = filepath.Clean(dir)

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s does not exist", dir)
		}
		return "", fmt.Errorf("cannot read %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is a file, not a directory", dir)
	}

	// Best effort: a symlink we cannot resolve is still usable as given.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return dir, nil
}

// Roots returns the primary working directory first, then every additional
// root, skipping empties. Always non-empty when config is loaded. Order is
// significant: RootFor returns the first match, so the primary root wins.
func Roots() []string {
	if cfg == nil {
		panic("config not loaded")
	}
	roots := make([]string, 0, 1+len(cfg.AdditionalDirs))
	if cfg.WorkingDir != "" {
		roots = append(roots, cfg.WorkingDir)
	}
	for _, d := range cfg.AdditionalDirs {
		if d != "" {
			roots = append(roots, d)
		}
	}
	return roots
}

// RootFor returns the workspace root containing path. Callers use it to scope a
// permission request to a whole root, so one "allow for session" covers it,
// instead of prompting per sub-directory.
func RootFor(path string) (string, bool) {
	if cfg == nil || path == "" {
		return "", false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cfg.WorkingDir, path)
	}
	path = filepath.Clean(path)
	for _, root := range Roots() {
		if containsPath(root, path) {
			return root, true
		}
	}
	return "", false
}

// AddDir registers an additional workspace root and persists it. Returns the
// cleaned absolute path actually stored.
//
// Rejects a directory already covered by an existing root, and a directory that
// would CONTAIN an existing root. The latter matters: adding a parent would make
// the child root unreachable, since RootFor returns the first match and would
// attribute the child's files to the parent — silently widening the permission
// scope the user had deliberately kept narrow.
func AddDir(dir string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config not loaded")
	}
	clean, err := normaliseDir(dir)
	if err != nil {
		return "", err
	}

	for _, root := range Roots() {
		switch {
		case root == clean:
			return "", fmt.Errorf("%s is already a workspace root", clean)
		case containsPath(root, clean):
			// GORILLA OVERRIDE: this used to be a flat refusal, which was the wrong
			// answer to the question actually being asked. A user pointing at a
			// SUBDIRECTORY of the current root is almost never trying to add a
			// redundant extra root — they are trying to NARROW the workspace to
			// that directory, because the current root is too broad. The canonical
			// case: the primary root is a home directory holding a kernel tree and
			// a browser tree, and the agent's file discovery walks millions of
			// files. Telling them it is "already covered" is both true and useless.
			return "", fmt.Errorf(
				"%s is inside the existing root %s — to work in just that directory, "+
					"narrow the workspace to it instead of adding it (press p in the add form, "+
					"or run /cd %s)", clean, root, clean)
		case containsPath(clean, root):
			return "", fmt.Errorf("%s contains the existing root %s — adding it would shadow that root and widen its permission scope; remove %s first", clean, root, root)
		}
	}

	set := func(c *Config) {
		for _, d := range c.AdditionalDirs {
			if d == clean {
				return
			}
		}
		c.AdditionalDirs = append(c.AdditionalDirs, clean)
	}
	set(cfg)
	if err := updateCfgFile(set); err != nil {
		return "", err
	}
	return clean, nil
}

// RemoveDir drops an additional workspace root and persists the change.
// Refuses to remove the primary working directory — that is what /cd is for,
// and a workspace with no primary root has no way to resolve a relative path.
func RemoveDir(dir string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	clean, err := normaliseDir(dir)
	if err != nil {
		// Allow removing a root whose directory has since been deleted or
		// renamed: fall back to the literal string so the user is never stuck
		// with an un-removable entry.
		clean = filepath.Clean(dir)
	}

	if clean == cfg.WorkingDir {
		return fmt.Errorf("%s is the primary working directory — use /cd to change it, not /add-dir to remove it", clean)
	}

	found := false
	remaining := make([]string, 0, len(cfg.AdditionalDirs))
	for _, d := range cfg.AdditionalDirs {
		if d == clean {
			found = true
			continue
		}
		remaining = append(remaining, d)
	}
	if !found {
		return fmt.Errorf("%s is not a workspace root", clean)
	}

	set := func(c *Config) { c.AdditionalDirs = remaining }
	set(cfg)
	return updateCfgFile(set)
}

// SetWorkingDir repoints the PRIMARY workspace root, for /cd.
//
// keepOld asks to retain the previous primary as an additional root. It is
// HONOURED ONLY when the new primary is not inside the old one.
//
// GORILLA OVERRIDE — the narrowing case. When the new primary IS inside the old
// (the common /cd: from a home directory down into one project), keeping the old
// as an extra root would defeat the entire operation. The user narrows precisely
// to stop the agent seeing the wider tree — its context files, its permission
// scope, its @-completion file walk. Retaining the parent would leave all of that
// in place while reporting success. So a narrowing /cd drops the old primary and
// says so.
//
// Widening or moving sideways still honours keepOld, because there the old root
// is genuinely separate work the user may not want to lose.
//
// Callers MUST also invalidate derived state — prompt.InvalidateContextCache(),
// and the persistent bash shell, which holds its own cwd for the process
// lifetime and will otherwise keep running commands in the old directory.
func SetWorkingDir(dir string, keepOld bool) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config not loaded")
	}
	clean, err := normaliseDir(dir)
	if err != nil {
		return "", err
	}
	if clean == cfg.WorkingDir {
		return clean, nil
	}

	old := cfg.WorkingDir

	// Two separate rules, because conflating them either keeps the broad tree or
	// throws away unrelated work:
	//
	//  1. If the OLD PRIMARY contains the new one, this is a narrowing and
	//     keepOld is ignored. Retaining the parent would leave the wide tree in
	//     scope — its context files, its permission scope, its @-completion file
	//     walk — while reporting success.
	//  2. Any EXTRA root that contains the new primary is dropped regardless,
	//     since leaving a parent in the extras list is the same defeat by another
	//     route. Unrelated extras are separate work and survive.
	if old != "" && old != clean && containsPath(old, clean) {
		keepOld = false
	}

	// The new primary must not remain in the extras list, or it would appear
	// twice. Rule 2 above: an extra that CONTAINS the new primary is dropped.
	extras := make([]string, 0, len(cfg.AdditionalDirs)+1)
	for _, d := range cfg.AdditionalDirs {
		if d == clean || containsPath(d, clean) {
			continue
		}
		extras = append(extras, d)
	}
	if keepOld && old != "" && old != clean {
		alreadyListed := false
		for _, d := range extras {
			if d == old {
				alreadyListed = true
				break
			}
		}
		if !alreadyListed {
			extras = append(extras, old)
		}
	}

	set := func(c *Config) {
		c.WorkingDir = clean
		c.AdditionalDirs = extras
	}
	set(cfg)
	if err := updateCfgFile(set); err != nil {
		return "", err
	}
	return clean, nil
}
