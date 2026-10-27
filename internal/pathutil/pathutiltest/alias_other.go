//go:build !windows

package pathutiltest

import (
	"os"
	"path/filepath"
	"testing"
)

// ShortNameOf: only Windows has 8.3 names.
func ShortNameOf(string) (string, bool) { return "", false }

func shortName(string) (string, bool) { return "", false }

// longTemp is t.TempDir() with its links followed (macOS: /var is
// /private/var), so the only second spelling is the one ShortAlias makes.
func longTemp(t *testing.T) string {
	d := t.TempDir()
	if r, err := filepath.EvalSymlinks(d); err == nil {
		return r
	}
	return d
}

func link(t *testing.T, target, alias string) Kind {
	t.Helper()
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	return Symlink
}
