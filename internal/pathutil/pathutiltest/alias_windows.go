//go:build windows

package pathutiltest

import (
	"os/exec"
	"testing"

	"github.com/opencode-ai/opencode/internal/pathutil"
	"golang.org/x/sys/windows"
)

// ShortNameOf returns the 8.3 spelling Windows has for path, and false when
// the volume made none (8.3 generation off, or a name that already fits).
func ShortNameOf(path string) (string, bool) { return shortName(path) }

func shortName(path string) (string, bool) {
	u, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", false
	}
	buf := make([]uint16, 520)
	n, err := windows.GetShortPathName(u, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) >= len(buf) {
		return "", false
	}
	s := windows.UTF16ToString(buf[:n])
	return s, s != path
}

// longTemp is t.TempDir() spelled long, so the folder made inside it has one
// spelling only unless ShortAlias makes another.
func longTemp(t *testing.T) string { return pathutil.Long(t.TempDir()) }

// link makes a junction: unlike a symlink it needs no privilege, so it works
// for an ordinary user and on the CI runner alike.
func link(t *testing.T, target, alias string) Kind {
	t.Helper()
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", alias, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J %s %s: %v\n%s", alias, target, err, out)
	}
	return Junction
}
