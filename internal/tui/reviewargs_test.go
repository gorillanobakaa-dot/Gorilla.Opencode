package tui

// GORILLA OVERRIDE (2026-08-18): the test that would have caught /review
// treating "--deep" as a folder name.
//
// The existing guard, TestEverySlashCommandNamedInProseActuallyExists, checks
// that a command mentioned in prose is in the registry. It says nothing about
// what a command does with its ARGUMENTS, which is how the same defect —
// a flag read as content — got written twice in one day.
//
// So this types what a person types.

import (
	"strings"
	"testing"
)

func TestReviewArgumentsAreParsedNotTreatedAsAPath(t *testing.T) {
	for _, c := range []struct {
		in    string
		path  string
		focus string
		diff  string
	}{
		// The original bug, in every spelling.
		{"--deep", "", "full", ""},
		{"-deep", "", "full", ""},
		{"--security", "", "security", ""},
		{"--quick", "", "quick", ""},

		// Flag plus path, in both orders — people write it both ways.
		{"--security internal/auth", "internal/auth", "security", ""},
		{"internal/auth --security", "internal/auth", "security", ""},

		// Explicit forms.
		{"--focus=security", "", "security", ""},
		{"--focus security", "", "security", ""},
		{"--level quick", "", "quick", ""},
		{"--depth=full", "", "full", ""},

		// Aliases someone would reasonably reach for.
		{"--sec", "", "security", ""},
		{"--audit", "", "security", ""},
		{"--fast", "", "quick", ""},
		{"--thorough", "", "full", ""},

		// Diff scoping.
		{"--diff HEAD~3", "", "", "HEAD~3"},
		{"--diff=origin/main", "", "", "origin/main"},
		{"--changes", "", "", "HEAD"}, // bare --diff means "what I changed"
		{"--diff HEAD internal/tui", "internal/tui", "", "HEAD"},

		// Everything at once.
		{"--security --diff origin/main internal/llm", "internal/llm", "security", "origin/main"},

		// And the ordinary cases must not regress.
		{"", "", "", ""},
		{"internal/llm/tools", "internal/llm/tools", "", ""},
		{"  internal/tui  ", "internal/tui", "", ""},
	} {
		got := parseReviewArgs(c.in)
		if got.Path != c.path || got.Focus != c.focus || got.Diff != c.diff {
			t.Errorf("parseReviewArgs(%q) = path=%q focus=%q diff=%q; want path=%q focus=%q diff=%q",
				c.in, got.Path, got.Focus, got.Diff, c.path, c.focus, c.diff)
		}
		if strings.HasPrefix(got.Path, "-") {
			t.Errorf("parseReviewArgs(%q) produced a path beginning with a dash (%q) — "+
				"that is the /osint --recover bug again", c.in, got.Path)
		}
	}
}

// A directory really can be called "security". A bare word is a path; only a
// dash makes it a flag.
func TestABareWordIsAPathEvenWhenItLooksLikeAFlagName(t *testing.T) {
	for _, word := range []string{"security", "quick", "deep", "audit"} {
		got := parseReviewArgs(word)
		if got.Path != word {
			t.Errorf("%q was read as %q rather than as a path", word, got.Path)
		}
		if got.Focus != "" {
			t.Errorf("%q set focus=%q; a bare word must not be a flag", word, got.Focus)
		}
	}
}

// A mistyped flag must be reported, never silently dropped and never used as
// the thing to review.
func TestAnUnrecognisedFlagIsReportedAndNeverBecomesThePath(t *testing.T) {
	got := parseReviewArgs("--secrutiy internal/auth")
	if got.Path != "internal/auth" {
		t.Errorf("the real path was lost: %q", got.Path)
	}
	if len(got.Unknown) != 1 || got.Unknown[0] != "--secrutiy" {
		t.Errorf("the typo was not reported back: %v", got.Unknown)
	}
	if got.Focus != "" {
		t.Errorf("a typo silently selected focus=%q", got.Focus)
	}

	// A flag whose VALUE is nonsense is also a typo, not a silent default.
	bad := parseReviewArgs("--focus=banana")
	if len(bad.Unknown) == 0 {
		t.Errorf("--focus=banana was accepted silently; focus=%q", bad.Focus)
	}
}
