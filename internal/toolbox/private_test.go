package toolbox

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/opencode-ai/opencode/internal/pathutil/pathutiltest"
)

// The CI failure (2026-10-27): the runner's cache under TEMP is spelled
// C:\Users\RUNNER~1\..., its profile C:\Users\runneradmin, and the find tool
// refused to run its own engine from "outside your user profile". A cache
// reached through a second spelling of a folder inside the profile is inside.
func TestCacheDirIsPrivateShortPathInsideTheProfile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("elsewhere the check is the directory's mode bits, not its location")
	}
	home := pathutiltest.LongDir(t)
	cacheLong := filepath.Join(home, "AppData Local")
	if err := os.MkdirAll(cacheLong, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", home)
	for _, mk := range []func(*testing.T, string) (string, pathutiltest.Kind){pathutiltest.ShortAlias, pathutiltest.LinkAlias} {
		alias, kind := mk(t, cacheLong)
		if err := CacheDirIsPrivate(filepath.Join(alias, "gorilla-opencode", "toolbox")); err != nil {
			t.Errorf("through a %s (%s): %v", kind, alias, err)
		}
	}
}

// CAPABILITY GUARD: a junction INSIDE the profile that leads OUT of it is
// outside. The helper about to be executed lands where the junction points.
func TestCacheDirIsPrivateShortPathJunctionOutIsRefused(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("elsewhere the check is the directory's mode bits, not its location")
	}
	home := pathutiltest.LongDir(t)
	elsewhere := pathutiltest.LongDir(t)
	t.Setenv("USERPROFILE", home)
	link, kind := pathutiltest.LinkAlias(t, elsewhere)
	inner := filepath.Join(home, "cache")
	if err := os.Rename(link, inner); err != nil {
		t.Skipf("cannot move the %s into the profile: %v", kind, err)
	}
	if err := CacheDirIsPrivate(filepath.Join(inner, "gorilla-opencode")); err == nil {
		t.Errorf("a %s in the profile pointing at %s was accepted as private", kind, elsewhere)
	}
}
