package config

import (
	"os"
	"path/filepath"
	"testing"
)

// containsPath replaces strings.HasPrefix for path containment. The sibling
// case is the reason it exists: with root "/tmp/foo", HasPrefix matches
// "/tmp/foobar/x.go" because a string prefix ignores the component boundary.
// That live bug at edit.go:201,312,432 and write.go:166 lets a file be
// attributed to a sibling directory's root, widening its permission scope.
func TestContainsPath(t *testing.T) {
	for _, tc := range []struct {
		name, root, path string
		want             bool
	}{
		{"root itself", "/tmp/foo", "/tmp/foo", true},
		{"direct child", "/tmp/foo", "/tmp/foo/a.go", true},
		{"deep child", "/tmp/foo", "/tmp/foo/a/b/c.go", true},
		{"sibling sharing a string prefix", "/tmp/foo", "/tmp/foobar/x.go", false},
		{"sibling exactly", "/tmp/foo", "/tmp/foobar", false},
		{"parent", "/tmp/foo", "/tmp", false},
		{"unrelated", "/tmp/foo", "/var/log/x", false},
		{"escaping via ..", "/tmp/foo", "/tmp/foo/../bar", false},
		{"empty root", "", "/tmp/foo", false},
		{"empty path", "/tmp/foo", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsPath(tc.root, tc.path); got != tc.want {
				t.Errorf("containsPath(%q, %q) = %v, want %v", tc.root, tc.path, got, tc.want)
			}
		})
	}
}

// withConfig gives each test an isolated loaded config rooted at a temp dir.
// config.Load early-returns once cfg is set, so tests share the global and must
// reset the fields they touch.
func withConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	real, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = real
	}
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := Get()
	prevWD, prevAdd := c.WorkingDir, c.AdditionalDirs
	c.WorkingDir = dir
	c.AdditionalDirs = nil
	t.Cleanup(func() { c.WorkingDir, c.AdditionalDirs = prevWD, prevAdd })
	return dir
}

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}

func TestRootsReturnsPrimaryFirst(t *testing.T) {
	primary := withConfig(t)
	extra := mkdir(t, t.TempDir(), "extra")

	if got := Roots(); len(got) != 1 || got[0] != primary {
		t.Fatalf("with no extras, Roots() = %v, want [%s]", got, primary)
	}

	if _, err := AddDir(extra); err != nil {
		t.Fatalf("AddDir: %v", err)
	}
	got := Roots()
	if len(got) != 2 || got[0] != primary || got[1] != extra {
		t.Errorf("Roots() = %v, want [%s %s] with the primary first", got, primary, extra)
	}
}

func TestAddDirRejections(t *testing.T) {
	primary := withConfig(t)

	t.Run("non-existent", func(t *testing.T) {
		if _, err := AddDir(filepath.Join(primary, "nope")); err == nil {
			t.Error("expected an error for a directory that does not exist")
		}
	})

	t.Run("a file, not a directory", func(t *testing.T) {
		f := filepath.Join(primary, "file.txt")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := AddDir(f); err == nil {
			t.Error("expected an error for a file")
		}
	})

	t.Run("already the primary root", func(t *testing.T) {
		if _, err := AddDir(primary); err == nil {
			t.Error("expected an error when re-adding the primary root")
		}
	})

	t.Run("subdir already covered by the primary", func(t *testing.T) {
		sub := mkdir(t, primary, "sub")
		if _, err := AddDir(sub); err == nil {
			t.Error("expected an error: a subdirectory of an existing root is already covered")
		}
	})

	t.Run("duplicate add", func(t *testing.T) {
		extra := mkdir(t, t.TempDir(), "dup")
		if _, err := AddDir(extra); err != nil {
			t.Fatalf("first AddDir: %v", err)
		}
		if _, err := AddDir(extra); err == nil {
			t.Error("expected an error on the second add")
		}
		n := 0
		for _, d := range Get().AdditionalDirs {
			if d == extra {
				n++
			}
		}
		if n != 1 {
			t.Errorf("root listed %d times, want exactly 1", n)
		}
	})

	// A parent would shadow the child: RootFor returns the first match, so the
	// child's files would be attributed to the parent, silently widening a
	// permission scope the user kept narrow.
	t.Run("parent of an existing root", func(t *testing.T) {
		base := t.TempDir()
		child := mkdir(t, base, "child")
		if _, err := AddDir(child); err != nil {
			t.Fatalf("AddDir child: %v", err)
		}
		if _, err := AddDir(base); err == nil {
			t.Error("expected an error: adding a parent of an existing root would shadow it")
		}
	})
}

func TestRootForAttributesToTheOwningRoot(t *testing.T) {
	primary := withConfig(t)
	extraBase := t.TempDir()
	extra := mkdir(t, extraBase, "foo")
	// Sibling sharing a string prefix with `extra` — the HasPrefix trap.
	sibling := mkdir(t, extraBase, "foobar")

	if _, err := AddDir(extra); err != nil {
		t.Fatalf("AddDir: %v", err)
	}

	if root, ok := RootFor(filepath.Join(extra, "a", "b.go")); !ok || root != extra {
		t.Errorf("RootFor(under extra) = (%q, %v), want (%q, true)", root, ok, extra)
	}
	if root, ok := RootFor(filepath.Join(primary, "main.go")); !ok || root != primary {
		t.Errorf("RootFor(under primary) = (%q, %v), want (%q, true)", root, ok, primary)
	}

	// The assertion that pins the bug fix.
	if root, ok := RootFor(filepath.Join(sibling, "x.go")); ok {
		t.Errorf("RootFor(%q) matched root %q — a sibling sharing a string prefix must NOT match", filepath.Join(sibling, "x.go"), root)
	}

	if _, ok := RootFor("/definitely/not/a/root/x.go"); ok {
		t.Error("RootFor matched a path outside every root")
	}

	// A relative path resolves against the primary root.
	if root, ok := RootFor("main.go"); !ok || root != primary {
		t.Errorf("RootFor(relative) = (%q, %v), want (%q, true)", root, ok, primary)
	}
}

func TestRemoveDir(t *testing.T) {
	primary := withConfig(t)
	extra := mkdir(t, t.TempDir(), "gone")
	if _, err := AddDir(extra); err != nil {
		t.Fatalf("AddDir: %v", err)
	}

	if err := RemoveDir(primary); err == nil {
		t.Error("expected an error: the primary root must be changed with /cd, not removed")
	}
	if err := RemoveDir(extra); err != nil {
		t.Fatalf("RemoveDir: %v", err)
	}
	if len(Roots()) != 1 {
		t.Errorf("Roots() = %v after removal, want just the primary", Roots())
	}
	if err := RemoveDir(extra); err == nil {
		t.Error("expected an error when removing a root that is not registered")
	}
}

// A root whose directory has since been deleted must still be removable, or the
// user is stuck with a permanently broken entry.
func TestRemoveDirWorksAfterTheDirectoryIsDeleted(t *testing.T) {
	withConfig(t)
	base := t.TempDir()
	extra := mkdir(t, base, "vanishing")
	if _, err := AddDir(extra); err != nil {
		t.Fatalf("AddDir: %v", err)
	}
	if err := os.RemoveAll(extra); err != nil {
		t.Fatal(err)
	}
	if err := RemoveDir(extra); err != nil {
		t.Errorf("RemoveDir on a deleted directory failed: %v", err)
	}
}

func TestSetWorkingDir(t *testing.T) {
	t.Run("keepOld retains the previous primary as an extra", func(t *testing.T) {
		old := withConfig(t)
		next := mkdir(t, t.TempDir(), "next")

		got, err := SetWorkingDir(next, true)
		if err != nil {
			t.Fatalf("SetWorkingDir: %v", err)
		}
		if got != next || Get().WorkingDir != next {
			t.Errorf("primary = %q, want %q", Get().WorkingDir, next)
		}
		roots := Roots()
		if len(roots) != 2 || roots[0] != next || roots[1] != old {
			t.Errorf("Roots() = %v, want [%s %s]", roots, next, old)
		}
	})

	t.Run("without keepOld the previous primary is dropped", func(t *testing.T) {
		withConfig(t)
		next := mkdir(t, t.TempDir(), "next2")
		if _, err := SetWorkingDir(next, false); err != nil {
			t.Fatalf("SetWorkingDir: %v", err)
		}
		if roots := Roots(); len(roots) != 1 || roots[0] != next {
			t.Errorf("Roots() = %v, want [%s]", roots, next)
		}
	})

	t.Run("promoting an existing extra does not duplicate it", func(t *testing.T) {
		withConfig(t)
		extra := mkdir(t, t.TempDir(), "promote")
		if _, err := AddDir(extra); err != nil {
			t.Fatalf("AddDir: %v", err)
		}
		if _, err := SetWorkingDir(extra, false); err != nil {
			t.Fatalf("SetWorkingDir: %v", err)
		}
		roots := Roots()
		if len(roots) != 1 || roots[0] != extra {
			t.Errorf("Roots() = %v, want exactly [%s] with no duplicate", roots, extra)
		}
	})

	t.Run("non-existent target is rejected and the primary is unchanged", func(t *testing.T) {
		primary := withConfig(t)
		if _, err := SetWorkingDir(filepath.Join(primary, "nope"), false); err == nil {
			t.Error("expected an error for a non-existent directory")
		}
		if Get().WorkingDir != primary {
			t.Errorf("primary changed to %q despite the error", Get().WorkingDir)
		}
	})
}

func TestNormaliseDirExpandsTilde(t *testing.T) {
	withConfig(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	got, err := normaliseDir("~")
	if err != nil {
		t.Fatalf("normaliseDir(~): %v", err)
	}
	want := home
	if real, err := filepath.EvalSymlinks(home); err == nil {
		want = real
	}
	if got != want {
		t.Errorf("normaliseDir(~) = %q, want %q", got, want)
	}
}
