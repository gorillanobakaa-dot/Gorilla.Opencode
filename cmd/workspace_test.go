package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// isolate points config at a scratch XDG_CONFIG_HOME and writes the given
// config.json, so nothing here can touch the real one. This guard exists
// because a test in this repo has twice overwritten the user's live config —
// once rewriting "wd" to a deleted temp dir, once clobbering a provider key.
func isolate(t *testing.T, cfg map[string]any) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, "gorilla-opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := config.GorillaConfigFile(); filepath.Dir(got) != dir {
		t.Fatalf("config isolation failed — this test would write the real config at %s", got)
	}
	if cfg == nil {
		return
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.GorillaConfigFile(), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// The bug this whole path exists to fix. The desktop entry is
// `Exec=gorilla-opencode launch` with no Path=, so an icon click starts the
// process in $HOME — which on this machine holds a kernel tree and a browser
// tree, over a million files. A saved workspace has to win over that inherited
// cwd, or every icon launch scopes the agent to everything.
func TestSavedWorkspaceBeatsAnInheritedHomeCwd(t *testing.T) {
	project := t.TempDir()
	isolate(t, map[string]any{"wd": project, "skipWorkspacePrompt": true})

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	restore := chdir(t, home)
	defer restore()

	// skipWorkspacePrompt is set, so this is the silent path — exactly what an
	// icon launch does once the user has ticked "don't ask again".
	got, _, remember, err := resolveWorkspace("", false)
	if err != nil {
		t.Fatalf("resolveWorkspace: %v", err)
	}
	if got == home {
		t.Fatalf("resolved to $HOME (%s) despite a saved workspace; this is the million-file scan", home)
	}
	if got != project {
		t.Errorf("resolved to %q, want the saved %q", got, project)
	}
	if remember {
		t.Error("remember should only be true when the user just ticked the box")
	}
}

// An explicit --cwd is the user being specific. It must win over everything,
// including a saved workspace, and must never trigger a prompt — scripts pass it.
func TestExplicitCwdFlagWins(t *testing.T) {
	saved, explicit := t.TempDir(), t.TempDir()
	isolate(t, map[string]any{"wd": saved})

	got, _, _, err := resolveWorkspace(explicit, false)
	if err != nil {
		t.Fatalf("resolveWorkspace: %v", err)
	}
	if got != explicit {
		t.Errorf("resolved to %q, want the flag value %q", got, explicit)
	}
}

// A bad --cwd must fail loudly rather than silently falling back, which would
// run the agent somewhere the user did not ask for.
func TestBadCwdFlagIsAnError(t *testing.T) {
	isolate(t, nil)

	if _, _, _, err := resolveWorkspace(filepath.Join(t.TempDir(), "nope"), false); err == nil {
		t.Error("accepted a --cwd that does not exist")
	}
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := resolveWorkspace(file, false); err == nil {
		t.Error("accepted a file as --cwd")
	}
}

// -p is a scripted run: there is no one to answer a prompt, and asking would
// hang the script forever.
func TestNonInteractiveNeverPrompts(t *testing.T) {
	project := t.TempDir()
	// Note: askWorkspaceOnStartup is left ON (skipWorkspacePrompt absent), so
	// only the nonInteractive argument can be suppressing the prompt here.
	isolate(t, map[string]any{"wd": project})

	done := make(chan string, 1)
	go func() {
		dir, _, _, err := resolveWorkspace("", true)
		if err != nil {
			t.Error(err)
		}
		done <- dir
	}()

	select {
	case got := <-done:
		if got != project {
			t.Errorf("resolved to %q, want %q", got, project)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("resolveWorkspace blocked with -p — it tried to prompt a script")
	}
}

// A saved workspace that has since been deleted must not strand the session in
// a directory that is not there.
func TestVanishedSavedWorkspaceFallsBackToCwd(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "deleted-project")
	isolate(t, map[string]any{"wd": gone, "skipWorkspacePrompt": true})

	here := t.TempDir()
	restore := chdir(t, here)
	defer restore()

	got, _, _, err := resolveWorkspace("", false)
	if err != nil {
		t.Fatalf("resolveWorkspace: %v", err)
	}
	if got != here {
		t.Errorf("resolved to %q, want the real cwd %q", got, here)
	}
}

// With nothing saved at all, a fresh install must fall back to the real cwd.
func TestFreshInstallUsesCwd(t *testing.T) {
	isolate(t, map[string]any{"skipWorkspacePrompt": true})

	here := t.TempDir()
	restore := chdir(t, here)
	defer restore()

	got, _, _, err := resolveWorkspace("", false)
	if err != nil {
		t.Fatalf("resolveWorkspace: %v", err)
	}
	if got != here {
		t.Errorf("resolved to %q, want %q", got, here)
	}
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Resolve symlinks: on macOS /var is /private/var and t.TempDir returns the
	// unresolved form, so a plain comparison against Getwd would fail.
	real, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = real
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Chdir(prev) }
}
