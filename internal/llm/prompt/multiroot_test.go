package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// This is the assertion that proves /add-dir does anything at all. A root whose
// CLAUDE.md is never read is just a directory the agent could already open by
// absolute path — the whole feature is "this directory's project instructions
// now reach the model".
func TestContextLoadsFromEveryRoot(t *testing.T) {
	primary := t.TempDir()
	extra := t.TempDir()
	if real, err := filepath.EvalSymlinks(extra); err == nil {
		extra = real
	}

	write := func(dir, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(primary, "PRIMARY-ROOT-MARKER")
	write(extra, "EXTRA-ROOT-MARKER")

	if _, err := config.Load(primary, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prevWD, prevAdd, prevPaths := cfg.WorkingDir, cfg.AdditionalDirs, cfg.ContextPaths
	t.Cleanup(func() {
		cfg.WorkingDir, cfg.AdditionalDirs, cfg.ContextPaths = prevWD, prevAdd, prevPaths
		InvalidateContextCache()
	})
	cfg.WorkingDir = primary
	cfg.ContextPaths = []string{"CLAUDE.md"}

	// Primary only, to start.
	cfg.AdditionalDirs = nil
	InvalidateContextCache()
	got := getContextFromPaths()
	if !strings.Contains(got, "PRIMARY-ROOT-MARKER") {
		t.Fatalf("primary root context missing:\n%s", got)
	}
	if strings.Contains(got, "EXTRA-ROOT-MARKER") {
		t.Fatalf("extra root content present before it was added:\n%s", got)
	}

	// Add the extra root — this is what /add-dir does.
	cfg.AdditionalDirs = []string{extra}
	InvalidateContextCache()
	got = getContextFromPaths()
	if !strings.Contains(got, "PRIMARY-ROOT-MARKER") {
		t.Errorf("primary root context lost after adding a second root:\n%s", got)
	}
	if !strings.Contains(got, "EXTRA-ROOT-MARKER") {
		t.Errorf("added root's CLAUDE.md never reached the prompt — /add-dir would be cosmetic:\n%s", got)
	}

	// Removing it must take the content away again, or /add-dir is one-way.
	cfg.AdditionalDirs = nil
	InvalidateContextCache()
	got = getContextFromPaths()
	if strings.Contains(got, "EXTRA-ROOT-MARKER") {
		t.Errorf("removed root's content still in the prompt:\n%s", got)
	}
}

// The env block must name additional roots, or the model does not know they
// exist and will never look in them.
func TestEnvBlockAdvertisesAdditionalRoots(t *testing.T) {
	primary := t.TempDir()
	base := t.TempDir()
	extra := filepath.Join(base, "second-root")
	if err := os.MkdirAll(filepath.Join(extra, "somedir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if real, err := filepath.EvalSymlinks(extra); err == nil {
		extra = real
	}

	if _, err := config.Load(primary, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prevWD, prevAdd := cfg.WorkingDir, cfg.AdditionalDirs
	t.Cleanup(func() { cfg.WorkingDir, cfg.AdditionalDirs = prevWD, prevAdd })
	cfg.WorkingDir = primary

	cfg.AdditionalDirs = nil
	if got := EnvironmentInfoBlock(); strings.Contains(got, "Additional workspace roots") {
		t.Errorf("env block mentions additional roots when there are none:\n%s", got)
	}

	cfg.AdditionalDirs = []string{extra}
	got := EnvironmentInfoBlock()
	if !strings.Contains(got, "Additional workspace roots") {
		t.Errorf("env block does not announce the extra root:\n%s", got)
	}
	if !strings.Contains(got, extra) {
		t.Errorf("env block does not name the extra root %q:\n%s", extra, got)
	}
	if !strings.Contains(got, "somedir") {
		t.Errorf("env block does not list the extra root's contents:\n%s", got)
	}
	// The primary must still be the one reported as the working directory.
	if !strings.Contains(got, "Working directory: "+primary) {
		t.Errorf("primary root is no longer reported as the working directory:\n%s", got)
	}
}

// Adding roots must not silently balloon the per-turn prompt. Extras are capped
// at maxExtraRootEntries and get no git-status shell-out.
func TestExtraRootCostIsBounded(t *testing.T) {
	primary := t.TempDir()
	extra := t.TempDir()
	// More entries than the cap.
	for i := 0; i < maxExtraRootEntries*3; i++ {
		if err := os.WriteFile(filepath.Join(extra, "file"+string(rune('a'+i%26))+string(rune('0'+i/26))+".txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if real, err := filepath.EvalSymlinks(extra); err == nil {
		extra = real
	}

	if _, err := config.Load(primary, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prevWD, prevAdd := cfg.WorkingDir, cfg.AdditionalDirs
	t.Cleanup(func() { cfg.WorkingDir, cfg.AdditionalDirs = prevWD, prevAdd })
	cfg.WorkingDir = primary

	cfg.AdditionalDirs = nil
	before := len(EnvironmentInfoBlock())
	cfg.AdditionalDirs = []string{extra}
	after := len(EnvironmentInfoBlock())

	added := after - before
	// A capped listing of 12 short names plus a header and a git marker. If the
	// cap regressed to "list everything" this blows well past it.
	if added > 1200 {
		t.Errorf("one extra root added %d bytes to a block that ships EVERY turn; the entry cap is not being applied", added)
	}
	if added <= 0 {
		t.Errorf("extra root added %d bytes — it is not being rendered at all", added)
	}

	// Extras must not trigger a git shell-out.
	if strings.Count(EnvironmentInfoBlock(), "Git status") > 1 {
		t.Error("git status rendered for an extra root; that is a 2s shell-out per root per render")
	}
}

// TaskPrompt embeds the env block, so it inherits multi-root awareness. Guard
// against a future refactor that gives the task agent a different env view.
func TestTaskPromptSeesAdditionalRoots(t *testing.T) {
	primary := t.TempDir()
	extra := t.TempDir()
	if real, err := filepath.EvalSymlinks(extra); err == nil {
		extra = real
	}
	if _, err := config.Load(primary, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prevWD, prevAdd := cfg.WorkingDir, cfg.AdditionalDirs
	t.Cleanup(func() { cfg.WorkingDir, cfg.AdditionalDirs = prevWD, prevAdd })
	cfg.WorkingDir = primary
	cfg.AdditionalDirs = []string{extra}

	if got := TaskPrompt(models.ProviderLocal); !strings.Contains(got, extra) {
		t.Errorf("task agent's prompt does not mention the extra root:\n%s", got)
	}
}
