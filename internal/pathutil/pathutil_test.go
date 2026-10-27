package pathutil_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/pathutil"
	"github.com/opencode-ai/opencode/internal/pathutil/pathutiltest"
)

// The CI failure itself, on every machine: a second spelling of a folder (8.3
// where the volume makes one, a junction or symlink otherwise) is the same
// place, and what lies under one spelling lies inside the other.
func TestShortPathAliasIsTheSameFolder(t *testing.T) {
	long := pathutiltest.LongDir(t)
	alias, kind := pathutiltest.ShortAlias(t, long)
	t.Logf("alias (%s): %s -> %s", kind, alias, long)

	if !pathutil.Same(alias, long) {
		t.Errorf("Same(%q, %q) = false; Canonical gives %q and %q",
			alias, long, pathutil.Canonical(alias), pathutil.Canonical(long))
	}
	file := filepath.Join(alias, "config.json") // does not exist yet: the write case
	if !pathutil.Within(long, file) {
		t.Errorf("%s is not inside %s", file, long)
	}
	if !pathutil.Within(alias, filepath.Join(long, "sub", "x")) {
		t.Errorf("a path under the long spelling is not inside the alias")
	}
	if got := pathutil.Canonical(file); !strings.EqualFold(got, filepath.Join(long, "config.json")) {
		t.Errorf("Canonical(%q) = %q, want the long spelling", file, got)
	}
}

// CAPABILITY GUARD: a neighbor whose name only starts the same is not inside,
// and neither is the parent.
func TestShortPathPrefixNeighborIsNotInside(t *testing.T) {
	long := pathutiltest.LongDir(t)
	for _, p := range []string{long + "-evil", filepath.Dir(long), filepath.Join(long, "..", "other")} {
		if pathutil.Within(long, p) || pathutil.WithinLexical(long, p) {
			t.Errorf("%s counted as inside %s", p, long)
		}
	}
	if pathutil.Within("", long) || pathutil.WithinLexical(long, "") {
		t.Error("an empty path counted as inside")
	}
}

// Long changes the spelling only: a link stays a link, so the lexical check
// does not follow it (permission scopes are about the path as written).
func TestShortPathLongDoesNotFollowLinks(t *testing.T) {
	long := pathutiltest.LongDir(t)
	alias, kind := pathutiltest.ShortAlias(t, long)
	if kind == pathutiltest.ShortName {
		if got := pathutil.Long(alias); !strings.EqualFold(got, long) {
			t.Errorf("Long(%q) = %q, want %q", alias, got, long)
		}
		return
	}
	if got := pathutil.Long(alias); !strings.EqualFold(got, alias) {
		t.Errorf("Long followed the %s: %q -> %q", kind, alias, got)
	}
	if pathutil.WithinLexical(long, filepath.Join(alias, "x")) {
		t.Errorf("WithinLexical followed the %s", kind)
	}
}

// GetLongPathName itself, on a real 8.3 name. Skipped only where the volume
// generates none (this laptop's profile name fits 8.3, and 8.3 generation
// may be off); the CI runner's TEMP is one, so it runs there.
func TestShortPathGetLongPathNameExpands83(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("8.3 short names exist only on Windows")
	}
	long := pathutiltest.LongDir(t)
	short, ok := pathutiltest.ShortNameOf(long)
	if !ok {
		if s, ok2 := pathutiltest.ShortNameOf(os.TempDir()); ok2 {
			short, long = s, pathutil.Long(os.TempDir())
		} else {
			t.Skipf("this volume generated no 8.3 name for %s (8.3 generation is off)", long)
		}
	}
	if !strings.Contains(short, "~") {
		t.Fatalf("GetShortPathName gave %q, not an 8.3 name", short)
	}
	if got := pathutil.Long(short); !strings.EqualFold(got, long) {
		t.Errorf("Long(%q) = %q, want %q", short, got, long)
	}
	if got := pathutil.Long(filepath.Join(short, "not-yet", "file.txt")); !strings.EqualFold(got, filepath.Join(long, "not-yet", "file.txt")) {
		t.Errorf("a path that does not exist yet kept its short head: %q", got)
	}
	if !pathutil.WithinLexical(long, filepath.Join(short, "x")) {
		t.Errorf("WithinLexical: %s is not inside %s", short, long)
	}
}

// The link form of the alias, run everywhere: a junction or symlink to a
// folder is inside it for Within, and outside it for WithinLexical.
func TestShortPathLinkAliasIsFollowedOnlyByCanonical(t *testing.T) {
	long := pathutiltest.LongDir(t)
	alias, kind := pathutiltest.LinkAlias(t, long)
	if !pathutil.Same(alias, long) || !pathutil.Within(long, filepath.Join(alias, "f")) {
		t.Errorf("the %s %s was not followed to %s", kind, alias, long)
	}
	if pathutil.WithinLexical(long, filepath.Join(alias, "f")) {
		t.Errorf("WithinLexical followed the %s", kind)
	}
}
