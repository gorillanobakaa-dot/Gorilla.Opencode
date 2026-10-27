// Package pathutiltest makes a second spelling of a folder, so tests can prove
// a check treats both spellings as the same place.
//
// The CI runner's TEMP is spelled with an 8.3 short name (C:\Users\RUNNER~1);
// a laptop whose profile name is 8 characters or fewer has none, and some
// volumes do not generate them at all, so the failure never reproduced there.
// ShortAlias gives every machine a second spelling: the real 8.3 name where
// the volume made one, otherwise a junction (Windows) or symlink (elsewhere)
// to the same folder.
package pathutiltest

import (
	"os"
	"path/filepath"
	"testing"
)

// Kind says which alias ShortAlias made.
type Kind string

const (
	ShortName Kind = "8.3 short name"
	Junction  Kind = "junction"
	Symlink   Kind = "symlink"
)

// LongDir creates a folder with a name too long for 8.3 under a fresh
// t.TempDir(), spelled the long way, and returns it.
func LongDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(longTemp(t), "a folder name longer than eight")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ShortAlias returns another spelling of dir (an existing folder) and what it
// is. The alias, when it is a link, lives in its own t.TempDir() OUTSIDE dir,
// so a check that compares strings cannot pass by accident.
func ShortAlias(t *testing.T, dir string) (string, Kind) {
	t.Helper()
	if s, ok := shortName(dir); ok && s != dir {
		return s, ShortName
	}
	alias := filepath.Join(longTemp(t), "alias")
	return alias, link(t, dir, alias)
}

// LinkAlias always makes the link form (junction on Windows, symlink
// elsewhere), for the machines where ShortAlias found an 8.3 name.
func LinkAlias(t *testing.T, dir string) (string, Kind) {
	t.Helper()
	alias := filepath.Join(longTemp(t), "alias")
	return alias, link(t, dir, alias)
}
