package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// loadCfg points config at a real directory so the environment block renders.
func loadCfg(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(wd, false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
}

// GORILLA OVERRIDE (2026-09-01): the system prompt must not change while the
// user works.
//
// Every provider that caches prompts does it by common PREFIX: the first byte
// that differs from the cached copy invalidates everything after it. Measured
// on a local Qwen3-Coder-30B with a ~5,700-token prompt:
//
//	identical prompt, sent again ..............   0.3 s
//	one character changed at the END ..........  57.7 s
//	one character changed at the START ........ 285.3 s
//
// The environment block sat about 1,800 tokens into a ~6,500-token prompt and
// carried `git status` and today's date. So every file the agent itself edited
// changed the prompt, threw away the cache for the remaining ~4,700 tokens, and
// cost minutes of prompt processing on a local model. On a cloud model the same
// thing silently pays full price for tokens that could have been cached.
//
// These tests fail if volatile content comes back.

func TestEnvironmentBlockIsStableAcrossWorkingTreeChanges(t *testing.T) {
	loadCfg(t)

	// The scenario that actually matters: the agent EDITS AN EXISTING FILE.
	// That changes `git status` and nothing else, and it happens dozens of times
	// in a single turn. Creating a brand-new top-level file legitimately changes
	// the directory listing, so that is not what is asserted here.
	f := filepath.Join(".", "prompt_stability_scratch.tmp")
	if err := os.WriteFile(f, []byte("original\n"), 0o644); err != nil {
		t.Skipf("cannot write into the working tree: %v", err)
	}
	t.Cleanup(func() { os.Remove(f) })

	before := EnvironmentInfoBlock()
	if err := os.WriteFile(f, []byte("edited by the agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := EnvironmentInfoBlock()

	if before != after {
		t.Errorf("the environment block changed after an existing file was edited.\n"+
			"Anything volatile here invalidates the prompt cache for every token that follows "+
			"it - measured at minutes per turn on a local model.\n\nbefore:\n%s\n\nafter:\n%s",
			before, after)
	}
}

// Two renders with nothing changing at all must be byte-identical. Catches
// anything time-based creeping back in.
func TestEnvironmentBlockIsStableAcrossConsecutiveRenders(t *testing.T) {
	loadCfg(t)
	if a, b := EnvironmentInfoBlock(), EnvironmentInfoBlock(); a != b {
		t.Errorf("two consecutive renders differ:\n%s\n---\n%s", a, b)
	}
}

// A date changes at midnight and invalidates every cached prompt at that
// moment, for nothing the shell cannot answer on demand.
func TestNoDateInTheEnvironmentBlock(t *testing.T) {
	loadCfg(t)
	env := EnvironmentInfoBlock()
	for _, marker := range []string{"Today's date", "Todays date"} {
		if strings.Contains(env, marker) {
			t.Errorf("the environment block still carries %q", marker)
		}
	}
}

// git status changes on every edit the agent makes, and is stale the moment it
// does: it is captured when the prompt is built, so a model reading it late in
// a turn sees the tree from before its own last several writes, presented as
// current. The branch NAME is stable for hours and stays.
func TestNoGitStatusInTheEnvironmentBlock(t *testing.T) {
	loadCfg(t)
	env := EnvironmentInfoBlock()
	if strings.Contains(env, "Git status") {
		t.Error("the environment block still carries `git status`, which changes on every edit")
	}
}
