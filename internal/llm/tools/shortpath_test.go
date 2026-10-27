package tools

// GORILLA FIX (2026-10-27): the windows-latest CI runner's TEMP is
// C:\Users\RUNNER~1\... (an 8.3 short name) and its profile
// C:\Users\runneradmin. Every guard below compared the two spellings as
// strings, so the workspace was not the workspace. These tests give every
// machine a second spelling (pathutiltest.ShortAlias) and run the real checks.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/pathutil"
	"github.com/opencode-ai/opencode/internal/pathutil/pathutiltest"
)

// loadWorkspace makes dir the only root for one test. config.Load runs once
// per binary (TestMain), so the loaded config is changed in place and put
// back after, as permission_scope_test.go does.
func loadWorkspace(t *testing.T, dir string) {
	t.Helper()
	cfg := config.Get()
	prevWD, prevAdd := cfg.WorkingDir, cfg.AdditionalDirs
	cfg.WorkingDir, cfg.AdditionalDirs = dir, nil
	t.Cleanup(func() { cfg.WorkingDir, cfg.AdditionalDirs = prevWD, prevAdd })
}

// A workspace opened by its short spelling, a file named by its long one (the
// CI case), and the other way round: the write stays inside and the prompt
// raises no alarm.
func TestWorkspaceShortPathWriteStaysInside(t *testing.T) {
	long := pathutiltest.LongDir(t)
	alias, kind := pathutiltest.ShortAlias(t, long)
	if kind != pathutiltest.ShortName {
		t.Skipf("no 8.3 name here (made a %s); TestWorkspaceShortPathLinkIsNamedByItsTarget covers links", kind)
	}
	for _, c := range []struct{ root, dest string }{
		{alias, filepath.Join(long, "new.txt")},
		{long, filepath.Join(alias, "new.txt")},
	} {
		loadWorkspace(t, c.root)
		resolved, inside, err := ResolveWriteTarget(c.dest)
		if err != nil {
			t.Fatal(err)
		}
		if !inside || !pathutil.Within(long, resolved) {
			t.Errorf("root %s: %s resolved to %s, inside=%v", c.root, c.dest, resolved, inside)
		}
		if d := DescribeWriteTarget(c.dest); d != c.dest {
			t.Errorf("root %s: the prompt for %s says %q; the same file spelled short is not a redirection", c.root, c.dest, d)
		}
	}
}

// A junction (or symlink) into the workspace from outside it is named by
// where it writes, and that is inside. Go 1.23+ EvalSymlinks does not follow
// junctions; before pathutil.Canonical this named the junction itself.
func TestWorkspaceShortPathLinkIsNamedByItsTarget(t *testing.T) {
	long := pathutiltest.LongDir(t)
	alias, kind := pathutiltest.LinkAlias(t, long)
	loadWorkspace(t, long)
	dest := filepath.Join(alias, "new.txt")
	resolved, inside, err := ResolveWriteTarget(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !inside || !pathutil.Equal(resolved, filepath.Join(long, "new.txt")) {
		t.Errorf("through a %s: %s resolved to %s, inside=%v", kind, dest, resolved, inside)
	}
	if d := DescribeWriteTarget(dest); !strings.Contains(d, resolved) || strings.Contains(d, "OUTSIDE") {
		t.Errorf("the prompt for a %s into the project says %q", kind, d)
	}
}

// The credential guard's workspace exemption (inWorkspace), which refused a
// project's own file on the CI runner: the root spelled short, the resolved
// file spelled long, and the other ways round.
func TestSensitiveShortPathWorkspaceExemptionHolds(t *testing.T) {
	long := pathutiltest.LongDir(t)
	alias, kind := pathutiltest.ShortAlias(t, long)
	if kind != pathutiltest.ShortName {
		t.Skipf("no 8.3 name here (made a %s); a link is a different path as written", kind)
	}
	for _, c := range []struct{ root, file string }{
		{alias, filepath.Join(long, "notes.txt")},
		{long, filepath.Join(alias, "notes.txt")},
		{alias, filepath.Join(alias, "notes.txt")},
	} {
		loadWorkspace(t, c.root)
		lexical := resolveForGuard(c.file)
		if !inWorkspace(lexical, realForGuard(lexical)) {
			t.Errorf("root %s: %s (resolved %s) is not in the workspace", c.root, c.file, realForGuard(lexical))
		}
	}
}

// CAPABILITY GUARD: a home folder opened by its short spelling is still the
// home folder, so the exemption never covers it.
func TestSensitiveShortPathHomeIsStillHome(t *testing.T) {
	home := pathutiltest.LongDir(t)
	alias, kind := pathutiltest.ShortAlias(t, home)
	if kind != pathutiltest.ShortName {
		t.Skipf("no 8.3 name here (made a %s)", kind)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if !isHomeOrAbove(alias) || !isHomeOrAbove(filepath.Dir(alias)) {
		t.Errorf("%s is the home folder %s spelled short, and was not treated as home", alias, home)
	}
	if isHomeOrAbove(filepath.Join(alias, "project")) {
		t.Error("a folder inside home counted as home or above")
	}
	loadWorkspace(t, alias)
	secret := filepath.Join(alias, sensitiveDirSegments[0], "notes.txt")
	if why := RefuseSensitiveRead(secret); why == "" {
		t.Errorf("ALLOWED %s in a home-folder workspace opened as %s", secret, alias)
	}
}

// A junction inside the project pointing at a credential FOLDER is read for
// what it is: an innocent file name under it is still refused. Go 1.23+
// EvalSymlinks does not follow junctions, so before pathutil.Canonical this
// read as a project file. Only empty folders are made.
func TestSensitiveShortPathJunctionToACredentialFolderIsRefused(t *testing.T) {
	project := pathutiltest.LongDir(t)
	creds := filepath.Join(pathutiltest.LongDir(t), sensitiveDirSegments[0])
	if err := os.MkdirAll(creds, 0o700); err != nil {
		t.Fatal(err)
	}
	link, kind := pathutiltest.LinkAlias(t, creds)
	inner := filepath.Join(project, "keys")
	if err := os.Rename(link, inner); err != nil {
		t.Skipf("cannot move the %s into the project: %v", kind, err)
	}
	loadWorkspace(t, project)
	if why := RefuseSensitiveRead(filepath.Join(inner, "notes.txt")); why == "" {
		t.Errorf("ALLOWED a read inside %s through a %s in the project", creds, kind)
	}
}

// This program's own configuration is refused however either side is
// spelled: XDG_CONFIG_HOME short and the file long, and the other way round.
// (Found when the whole package ran with a short TEMP: Canonical spells abs
// long, and the configuration folder was compared as written.)
func TestSensitiveShortPathOwnConfigIsStillRefused(t *testing.T) {
	long := pathutiltest.LongDir(t)
	alias, kind := pathutiltest.ShortAlias(t, long)
	for _, c := range []struct{ xdg, file string }{
		{alias, filepath.Join(long, "gorilla-opencode", "settings.json")},
		{long, filepath.Join(alias, "gorilla-opencode", "settings.json")},
	} {
		t.Setenv("XDG_CONFIG_HOME", c.xdg)
		if why := RefuseSensitiveRead(c.file); why == "" {
			t.Errorf("config home %s (%s): ALLOWED reading %s", c.xdg, kind, c.file)
		}
	}
}
