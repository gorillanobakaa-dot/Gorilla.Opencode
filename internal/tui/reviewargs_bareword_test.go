package tui

import (
	"strings"
	"testing"
)

// The reported failure: `/review quick` built a prompt saying path="quick", so
// the model went looking for a folder of that name and reviewed nothing. The
// dash is the only difference between --quick and quick, and nobody types it
// reliably for a word that reads as an adverb.
func TestBareDepthWordIsADepthNotAFolder(t *testing.T) {
	// Nothing on disk, so a depth word can only be a depth.
	restore := reviewPathExists
	reviewPathExists = func(string) bool { return false }
	defer func() { reviewPathExists = restore }()

	for _, tc := range []struct{ in, focus string }{
		{"quick", "quick"},
		{"QUICK", "quick"},
		{"fast", "quick"},
		{"security", "security"},
		{"sec", "security"},
		{"full", "full"},
		{"deep", "full"},
	} {
		got := parseReviewArgs(tc.in)
		if got.Focus != tc.focus {
			t.Errorf("/review %s -> Focus=%q, want %q", tc.in, got.Focus, tc.focus)
		}
		if got.Path != "" {
			t.Errorf("/review %s -> Path=%q; the word was the depth, not a folder", tc.in, got.Path)
		}
	}
}

// A folder that really exists always wins. "full" and "audit" are perfectly
// plausible directory names, and reviewing the wrong thing is worse than
// needing a dash.
func TestARealFolderBeatsTheDepthReading(t *testing.T) {
	restore := reviewPathExists
	reviewPathExists = func(p string) bool { return p == "audit" }
	defer func() { reviewPathExists = restore }()

	got := parseReviewArgs("audit")
	if got.Path != "audit" {
		t.Errorf("Path=%q, want %q: a directory that exists must not be read as a depth", got.Path, "audit")
	}
	if got.Focus != "" {
		t.Errorf("Focus=%q; it should be empty when the word names a real folder", got.Focus)
	}
}

// The existing behaviour must not move.
func TestBareWordRuleDoesNotDisturbNormalUse(t *testing.T) {
	restore := reviewPathExists
	reviewPathExists = func(string) bool { return false }
	defer func() { reviewPathExists = restore }()

	// A real path is still a path.
	if got := parseReviewArgs("internal/auth"); got.Path != "internal/auth" || got.Focus != "" {
		t.Errorf("path arg: Path=%q Focus=%q", got.Path, got.Focus)
	}
	// Flags still win, and the bare word after one stays a path.
	if got := parseReviewArgs("--security internal/auth"); got.Focus != "security" || got.Path != "internal/auth" {
		t.Errorf("flag+path: Path=%q Focus=%q", got.Path, got.Focus)
	}
	// An explicit flag plus a depth-looking word: the word is the path,
	// because the depth was already given.
	if got := parseReviewArgs("--quick full"); got.Focus != "quick" || got.Path != "full" {
		t.Errorf("--quick full: Path=%q Focus=%q; the depth was already set", got.Path, got.Focus)
	}
	// Two positionals are not a depth word.
	if got := parseReviewArgs("quick brown"); got.Focus != "" || got.Path != "quick brown" {
		t.Errorf("two words: Path=%q Focus=%q", got.Path, got.Focus)
	}
	// Nothing at all is still nothing.
	if got := parseReviewArgs(""); got.Path != "" || got.Focus != "" {
		t.Errorf("empty: Path=%q Focus=%q", got.Path, got.Focus)
	}
}

// The prompt is what the model actually sees, so assert on it too: a depth
// must not arrive as a path there either.
func TestPromptForBareDepthAsksForDepthNotPath(t *testing.T) {
	restore := reviewPathExists
	reviewPathExists = func(string) bool { return false }
	defer func() { reviewPathExists = restore }()

	p := reviewPrompt(parseReviewArgs("quick"))
	if strings.Contains(p, `path="quick"`) {
		t.Error(`the prompt still says path="quick" — the exact text the user saw`)
	}
	if !strings.Contains(p, `focus="quick"`) {
		t.Errorf("the prompt does not ask for the quick depth:\n%s", p)
	}
}
