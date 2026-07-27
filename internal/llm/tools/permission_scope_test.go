package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// permissionScope decides how wide a "allow for this session" grant reaches.
// Getting it wrong is a security question, not a cosmetic one: scoping a file to
// a root the user did not intend means a grant they gave for their project also
// covers edits somewhere else.
func TestPermissionScope(t *testing.T) {
	base := t.TempDir()
	if real, err := filepath.EvalSymlinks(base); err == nil {
		base = real
	}

	primary := filepath.Join(base, "foo")
	sibling := filepath.Join(base, "foobar") // shares a string prefix with primary
	extra := filepath.Join(base, "extra")
	outside := filepath.Join(base, "elsewhere")
	for _, d := range []string{primary, sibling, extra, outside} {
		if err := os.MkdirAll(filepath.Join(d, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := config.Load(primary, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prevWD, prevAdd := cfg.WorkingDir, cfg.AdditionalDirs
	cfg.WorkingDir = primary
	cfg.AdditionalDirs = []string{extra}
	t.Cleanup(func() { cfg.WorkingDir, cfg.AdditionalDirs = prevWD, prevAdd })

	for _, tc := range []struct {
		name string
		file string
		want string
		why  string
	}{
		{
			name: "file directly in the primary root",
			file: filepath.Join(primary, "main.go"),
			want: primary,
			why:  "one grant should cover the workspace",
		},
		{
			name: "file deep inside the primary root",
			file: filepath.Join(primary, "sub", "deep.go"),
			want: primary,
			why:  "a sub-directory must not need its own grant",
		},
		{
			name: "file inside an added root",
			file: filepath.Join(extra, "sub", "notes.go"),
			want: extra,
			why:  "the point of /add-dir is that its files scope to that root",
		},
		{
			// THE BUG. strings.HasPrefix("/base/foobar/x.go", "/base/foo") is
			// true, so this file used to be attributed to the primary root and
			// inherit any session-wide grant given for the workspace.
			name: "sibling directory sharing a string prefix with the primary root",
			file: filepath.Join(sibling, "x.go"),
			want: sibling,
			why:  "a sibling is NOT inside the root; scope must stay at its own directory",
		},
		{
			name: "sibling sub-directory sharing a string prefix",
			file: filepath.Join(sibling, "sub", "y.go"),
			want: filepath.Join(sibling, "sub"),
			why:  "still outside every root, so the narrowest scope applies",
		},
		{
			name: "file outside every root",
			file: filepath.Join(outside, "z.go"),
			want: outside,
			why:  "an unrelated path must not borrow a root's scope",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := permissionScope(tc.file); got != tc.want {
				t.Errorf("permissionScope(%q) = %q, want %q — %s", tc.file, got, tc.want, tc.why)
			}
		})
	}
}

// Removing an added root must immediately narrow the scope again, or a grant
// keeps reaching a directory the user has just detached.
func TestPermissionScopeFollowsRootRemoval(t *testing.T) {
	base := t.TempDir()
	if real, err := filepath.EvalSymlinks(base); err == nil {
		base = real
	}
	primary := filepath.Join(base, "primary")
	extra := filepath.Join(base, "extra")
	for _, d := range []string{primary, filepath.Join(extra, "sub")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := config.Load(primary, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prevWD, prevAdd := cfg.WorkingDir, cfg.AdditionalDirs
	cfg.WorkingDir = primary
	cfg.AdditionalDirs = []string{extra}
	t.Cleanup(func() { cfg.WorkingDir, cfg.AdditionalDirs = prevWD, prevAdd })

	file := filepath.Join(extra, "sub", "a.go")
	if got := permissionScope(file); got != extra {
		t.Fatalf("with %q registered, scope = %q, want %q", extra, got, extra)
	}

	cfg.AdditionalDirs = nil
	if got, want := permissionScope(file), filepath.Join(extra, "sub"); got != want {
		t.Errorf("after removing the root, scope = %q, want %q — a stale root would keep widening the grant", got, want)
	}
}
