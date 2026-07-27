package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// The project context used to be cached with sync.Once, which cannot be reset.
// That made /add-dir, /cd and contextPaths edits silent no-ops until restart:
// the control reported success and changed nothing the model saw.
//
// These assertions pin both halves of the contract — the cache must HOLD (so we
// are not re-reading every file on every turn) and it must RELEASE on demand.
// The middle assertion is the one that matters: it is the bug reproduced.
func TestContextCacheHoldsThenInvalidates(t *testing.T) {
	tmpDir := t.TempDir()
	if _, err := config.Load(tmpDir, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	cfg.WorkingDir = tmpDir
	cfg.ContextPaths = []string{"CLAUDE.md"}

	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(body), 0o644); err != nil {
			t.Fatalf("write CLAUDE.md: %v", err)
		}
	}

	write("ORIGINAL-MARKER")
	InvalidateContextCache()

	if got := getContextFromPaths(); !strings.Contains(got, "ORIGINAL-MARKER") {
		t.Fatalf("first read did not pick up the context file, got:\n%s", got)
	}

	// Cache must hold: the file changed but nothing invalidated it.
	write("REPLACED-MARKER")
	got := getContextFromPaths()
	if strings.Contains(got, "REPLACED-MARKER") {
		t.Error("cache did not hold — context was re-read without invalidation, so every turn pays for a full re-read")
	}
	if !strings.Contains(got, "ORIGINAL-MARKER") {
		t.Errorf("cache held but lost its content, got:\n%s", got)
	}

	// Cache must release on demand. This is what /add-dir and /cd rely on;
	// with sync.Once it was impossible.
	InvalidateContextCache()
	if got := getContextFromPaths(); !strings.Contains(got, "REPLACED-MARKER") {
		t.Errorf("InvalidateContextCache did not take effect — /add-dir and /cd would silently do nothing, got:\n%s", got)
	}
}

// A changed contextPaths list must be honoured after invalidation, not just a
// changed file. This is the /settings path: editing contextPaths has to change
// what the model receives.
func TestContextCachePicksUpChangedContextPaths(t *testing.T) {
	tmpDir := t.TempDir()
	if _, err := config.Load(tmpDir, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	cfg.WorkingDir = tmpDir

	for name, body := range map[string]string{
		"FIRST.md":  "FIRST-BODY",
		"SECOND.md": "SECOND-BODY",
	} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	cfg.ContextPaths = []string{"FIRST.md"}
	InvalidateContextCache()
	got := getContextFromPaths()
	if !strings.Contains(got, "FIRST-BODY") || strings.Contains(got, "SECOND-BODY") {
		t.Fatalf("expected only FIRST.md, got:\n%s", got)
	}

	cfg.ContextPaths = []string{"SECOND.md"}
	InvalidateContextCache()
	got = getContextFromPaths()
	if !strings.Contains(got, "SECOND-BODY") {
		t.Errorf("contextPaths change not honoured after invalidation, got:\n%s", got)
	}
	if strings.Contains(got, "FIRST-BODY") {
		t.Errorf("dropped context path still present — removing an entry must remove its content, got:\n%s", got)
	}
}

// processContextPaths runs one goroutine per context path. Output order must
// follow the configured path order, not goroutine completion order, or the
// system prompt reshuffles between identical runs.
func TestProcessContextPathsIsDeterministic(t *testing.T) {
	tmpDir := t.TempDir()

	// Enough paths, with the cheap single files listed AFTER a directory walk,
	// that arrival order and configured order genuinely differ.
	if err := os.MkdirAll(filepath.Join(tmpDir, "rules"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	files := map[string]string{
		"rules/a.md": "RULES-A",
		"rules/b.md": "RULES-B",
		"one.md":     "ONE",
		"two.md":     "TWO",
		"three.md":   "THREE",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	paths := []string{"rules/", "one.md", "two.md", "three.md"}

	first := processContextPaths(tmpDir, paths)
	for i := 0; i < 50; i++ {
		if got := processContextPaths(tmpDir, paths); got != first {
			t.Fatalf("output changed between runs (iteration %d)\nfirst:\n%s\ngot:\n%s", i, first, got)
		}
	}

	// And the order must be the CONFIGURED order, not merely stable.
	for _, want := range []string{"RULES-A", "RULES-B", "ONE", "TWO", "THREE"} {
		if !strings.Contains(first, want) {
			t.Fatalf("missing %s from output:\n%s", want, first)
		}
	}
	if idxOf(first, "ONE") < idxOf(first, "RULES-A") {
		t.Error("single files sorted before the directory path that precedes them in contextPaths")
	}
	if idxOf(first, "TWO") < idxOf(first, "ONE") || idxOf(first, "THREE") < idxOf(first, "TWO") {
		t.Error("single files are not in configured order")
	}
}

func idxOf(haystack, needle string) int { return strings.Index(haystack, needle) }
