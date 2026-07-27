package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// setupTree builds parent/child/sibling under a temp dir and loads config with
// parent as the primary root.
func setupTree(t *testing.T) (parent, child, sibling string) {
	t.Helper()
	base := t.TempDir()
	if real, err := filepath.EvalSymlinks(base); err == nil {
		base = real
	}
	parent = base
	child = filepath.Join(base, "project", "deep")
	sibling = filepath.Join(base, "..", "elsewhere")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	sibling = t.TempDir()
	if real, err := filepath.EvalSymlinks(sibling); err == nil {
		sibling = real
	}

	if _, err := Load(parent, false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := Get()
	prevWD, prevAdd := c.WorkingDir, c.AdditionalDirs
	t.Cleanup(func() { c.WorkingDir, c.AdditionalDirs = prevWD, prevAdd })
	c.WorkingDir = parent
	c.AdditionalDirs = nil
	return parent, child, sibling
}

// The operation the whole roots feature exists for. An agent started in a
// directory holding a kernel tree and a browser tree walks millions of files;
// the fix is to point it at ONE project. Narrowing must therefore actually
// narrow — keeping the parent as an extra root would leave its context files,
// its permission scope and its @-completion file walk all in place while
// reporting success.
func TestNarrowingDropsTheParentRoot(t *testing.T) {
	parent, child, _ := setupTree(t)
	c := Get()

	got, err := SetWorkingDir(child, true) // keepOld=true is deliberately IGNORED here
	if err != nil {
		t.Fatalf("SetWorkingDir: %v", err)
	}
	if got != child {
		t.Errorf("primary = %q, want %q", got, child)
	}

	roots := Roots()
	if len(roots) != 1 {
		t.Errorf("after narrowing, Roots() = %v — want exactly the new root; the parent being retained makes the narrowing cosmetic", roots)
	}
	if slices.Contains(roots, parent) {
		t.Errorf("the parent %q survived a narrowing operation; the wide tree is still in scope", parent)
	}
	if c.WorkingDir != child {
		t.Errorf("cfg.WorkingDir = %q, want %q", c.WorkingDir, child)
	}
}

// An extra root that CONTAINS the new primary must also be dropped — leaving a
// parent behind in the extras list is the same defeat by another route.
func TestNarrowingDropsAContainingExtraRoot(t *testing.T) {
	parent, child, sibling := setupTree(t)
	c := Get()

	// Start with the parent as an EXTRA and the sibling as primary, so the
	// parent is only reachable via AdditionalDirs.
	c.WorkingDir = sibling
	c.AdditionalDirs = []string{parent}

	if _, err := SetWorkingDir(child, true); err != nil {
		t.Fatalf("SetWorkingDir: %v", err)
	}

	roots := Roots()
	if slices.Contains(roots, parent) {
		t.Errorf("extra root %q contains the new primary %q and should have been dropped; roots = %v", parent, child, roots)
	}
	// The unrelated sibling is legitimate separate work and must survive.
	if !slices.Contains(roots, sibling) {
		t.Errorf("unrelated root %q was dropped by a narrowing operation; roots = %v", sibling, roots)
	}
}

// Moving sideways is not narrowing, so keepOld must still be honoured — the old
// root there is genuinely separate work the user may not want to lose.
func TestSidewaysMoveStillHonoursKeepOld(t *testing.T) {
	parent, _, sibling := setupTree(t)

	if _, err := SetWorkingDir(sibling, true); err != nil {
		t.Fatalf("SetWorkingDir: %v", err)
	}
	roots := Roots()
	if roots[0] != sibling {
		t.Errorf("primary = %q, want %q", roots[0], sibling)
	}
	if !slices.Contains(roots, parent) {
		t.Errorf("keepOld=true on a sideways move dropped %q; roots = %v", parent, roots)
	}
}

func TestSidewaysMoveWithoutKeepOldDropsIt(t *testing.T) {
	parent, _, sibling := setupTree(t)

	if _, err := SetWorkingDir(sibling, false); err != nil {
		t.Fatalf("SetWorkingDir: %v", err)
	}
	if roots := Roots(); slices.Contains(roots, parent) {
		t.Errorf("keepOld=false kept %q; roots = %v", parent, roots)
	}
}

// AddDir still refuses a redundant extra — adding a subdirectory of an existing
// root genuinely does nothing for context loading or permission scoping. But the
// error must point at narrowing, because that is what the user is actually
// trying to do. A flat "already covered" is true and useless.
func TestAddDirRefusalPointsAtNarrowing(t *testing.T) {
	_, child, _ := setupTree(t)

	_, err := AddDir(child)
	if err == nil {
		t.Fatal("AddDir accepted a subdirectory of an existing root")
	}
	msg := err.Error()
	for _, want := range []string{"narrow", "/cd"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal message does not tell the user how to get what they want (missing %q): %s", want, msg)
		}
	}
}

// Narrowing to a subdirectory must work even though ADDING it is refused. This
// is the pairing that was broken: the only route to a subdirectory was
// add-then-promote, and the add step refused.
func TestNarrowingWorksWhereAddingIsRefused(t *testing.T) {
	_, child, _ := setupTree(t)

	if _, err := AddDir(child); err == nil {
		t.Fatal("expected AddDir to refuse a subdirectory")
	}
	if _, err := SetWorkingDir(child, false); err != nil {
		t.Errorf("SetWorkingDir refused the same path AddDir refused, leaving no way to narrow: %v", err)
	}
	if Get().WorkingDir != child {
		t.Errorf("WorkingDir = %q, want %q", Get().WorkingDir, child)
	}
}

// Narrowing must genuinely shrink what @-completion walks, since that is the
// expensive automatic scan. Roots() is what feeds ripgrep.
func TestNarrowingShrinksTheSearchScope(t *testing.T) {
	parent, child, _ := setupTree(t)
	c := Get()
	c.AdditionalDirs = nil

	before := Roots()
	if len(before) != 1 || before[0] != parent {
		t.Fatalf("setup: Roots() = %v, want [%s]", before, parent)
	}

	if _, err := SetWorkingDir(child, true); err != nil {
		t.Fatal(err)
	}
	after := Roots()
	if len(after) != 1 || after[0] != child {
		t.Fatalf("Roots() = %v, want exactly [%s] — ripgrep is handed these paths", after, child)
	}
	// The new scope must be strictly inside the old one, not merely different.
	if !containsPath(parent, after[0]) {
		t.Errorf("new root %q is not inside the old %q; this is not a narrowing", after[0], parent)
	}
}
