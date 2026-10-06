package tui

// GORILLA FIX (2026-10-05): three ways /review misread what was typed.
//
// The earlier tests cover a flag read as a path and a depth word read as a
// folder. These are the remaining members of the same family, each one a case
// where the parser was confident and wrong:
//
//   - `--diff internal/auth` took the folder as the git ref;
//   - `--focus banana` blamed the option instead of the value;
//   - `"my project"` kept its quote characters and split in two.

import (
	"strings"
	"testing"
)

// stubReviewDisk replaces the two lookups the parser makes, so a test states
// what is on disk and what git knows instead of arranging it.
func stubReviewDisk(t *testing.T, paths, refs map[string]bool) {
	t.Helper()
	oldPath, oldRef := reviewPathExists, reviewRefExists
	reviewPathExists = func(p string) bool { return paths[p] }
	reviewRefExists = func(r string) bool { return refs[r] }
	t.Cleanup(func() { reviewPathExists, reviewRefExists = oldPath, oldRef })
}

// `--diff` on its own is documented as "what I changed". A folder after it is
// therefore the folder to review, not a ref called after a folder.
func TestAFolderAfterDiffIsThePathNotTheRef(t *testing.T) {
	stubReviewDisk(t, map[string]bool{"internal/auth": true}, nil)

	got := parseReviewArgs("--diff internal/auth")
	if got.Diff != "HEAD" {
		t.Errorf("diff=%q; a bare --diff means HEAD, and internal/auth is a folder, not a ref", got.Diff)
	}
	if got.Path != "internal/auth" {
		t.Errorf("path=%q; the folder typed after --diff was lost", got.Path)
	}
	if len(got.Unknown) != 0 {
		t.Errorf("reported unknown options for a valid command: %v", got.Unknown)
	}
}

// A ref stays a ref: nothing on disk is called HEAD~3 or origin/main, and the
// documented forms must keep working exactly as before.
func TestARefAfterDiffIsStillARef(t *testing.T) {
	stubReviewDisk(t, map[string]bool{"internal/tui": true}, map[string]bool{"HEAD": true, "origin/main": true})

	for _, c := range []struct{ in, path, diff string }{
		{"--diff HEAD", "", "HEAD"},
		{"--diff HEAD~3", "", "HEAD~3"},
		{"--diff origin/main internal/tui", "internal/tui", "origin/main"},
		{"--diff=internal/tui", "", "internal/tui"}, // explicit = is never second-guessed
		{"internal/tui --diff", "internal/tui", "HEAD"},
	} {
		got := parseReviewArgs(c.in)
		if got.Path != c.path || got.Diff != c.diff {
			t.Errorf("parseReviewArgs(%q) = path=%q diff=%q; want path=%q diff=%q",
				c.in, got.Path, got.Diff, c.path, c.diff)
		}
	}
}

// A word that is BOTH a folder and a branch is the ref: it was typed after
// --diff, and git can resolve it.
func TestAWordThatIsBothAFolderAndABranchIsTheRef(t *testing.T) {
	stubReviewDisk(t, map[string]bool{"main": true}, map[string]bool{"main": true})

	got := parseReviewArgs("--diff main")
	if got.Diff != "main" || got.Path != "" {
		t.Errorf("path=%q diff=%q; `main` resolves as a ref and must be used as one", got.Path, got.Diff)
	}
}

// Git is not consulted for the ordinary case. Asking would start a process on
// every `/review --diff HEAD`.
func TestGitIsOnlyAskedWhenTheWordIsAPath(t *testing.T) {
	asked := 0
	oldPath, oldRef := reviewPathExists, reviewRefExists
	reviewPathExists = func(string) bool { return false }
	reviewRefExists = func(string) bool { asked++; return true }
	t.Cleanup(func() { reviewPathExists, reviewRefExists = oldPath, oldRef })

	parseReviewArgs("--diff HEAD~1 --security")
	if asked != 0 {
		t.Errorf("git was asked %d time(s) about a word that is not a path on disk", asked)
	}
}

// The option exists; the value does not. The message must say which.
func TestABadDepthValueIsBlamedOnTheValue(t *testing.T) {
	stubReviewDisk(t, nil, nil)

	got := parseReviewArgs("--focus banana src")
	if len(got.Unknown) != 1 {
		t.Fatalf("unknown = %v; want exactly the bad depth", got.Unknown)
	}
	if got.Path != "src" {
		t.Errorf("path=%q; the path after the bad value was lost", got.Path)
	}
	msg := unknownReviewOptionMessage(got.Unknown)
	if strings.Contains(msg, "Don't know the option") {
		t.Errorf("--focus is a real option, but the message says it is unknown:\n%s", msg)
	}
	if !strings.Contains(msg, "banana") || !strings.Contains(msg, "quick, security or full") {
		t.Errorf("the message does not name the bad value and the good ones:\n%s", msg)
	}

	// No value at all is its own message.
	none := parseReviewArgs("--focus")
	if len(none.Unknown) != 1 {
		t.Fatalf("--focus with nothing after it was accepted: %+v", none)
	}
	if msg := unknownReviewOptionMessage(none.Unknown); !strings.Contains(msg, "needs a depth") {
		t.Errorf("--focus with no value does not say a depth is needed:\n%s", msg)
	}

	// And a flag that really does not exist keeps the old wording.
	if msg := unknownReviewOptionMessage([]string{"--secrutiy"}); !strings.Contains(msg, "Don't know the option --secrutiy") {
		t.Errorf("a genuinely unknown flag lost its message:\n%s", msg)
	}
}

// Quotes group a path with spaces and are not part of it.
func TestAQuotedPathIsOnePathWithoutItsQuotes(t *testing.T) {
	stubReviewDisk(t, nil, nil)

	for _, c := range []struct{ in, path, focus string }{
		{`"my project"`, "my project", ""},
		{`'my project' --quick`, "my project", "quick"},
		{`--security "C:\Users\me\My Code"`, `C:\Users\me\My Code`, "security"},
		{`src/don't-panic`, "src/don't-panic", ""}, // an apostrophe inside a word is a character
	} {
		got := parseReviewArgs(c.in)
		if got.Path != c.path || got.Focus != c.focus {
			t.Errorf("parseReviewArgs(%s) = path=%q focus=%q; want path=%q focus=%q",
				c.in, got.Path, got.Focus, c.path, c.focus)
		}
		if strings.ContainsAny(got.Path, `"`) {
			t.Errorf("parseReviewArgs(%s) left a quote character in the path: %q", c.in, got.Path)
		}
		if len(got.Unknown) != 0 {
			t.Errorf("parseReviewArgs(%s) reported unknown options: %v", c.in, got.Unknown)
		}
	}

	// Quoting is how to say "this is a folder" for a word that spells a depth
	// or looks like a flag.
	if got := parseReviewArgs(`"quick"`); got.Path != "quick" || got.Focus != "" {
		t.Errorf(`"quick" in quotes was read as a depth: %+v`, got)
	}
	if got := parseReviewArgs(`"--odd-folder"`); got.Path != "--odd-folder" || len(got.Unknown) != 0 {
		t.Errorf(`a quoted folder beginning with dashes was read as a flag: %+v`, got)
	}
}

// What the model is told about a quick pass must match what a quick pass is.
func TestTheQuickPromptDescribesWhatQuickReallyRuns(t *testing.T) {
	p := reviewPrompt(reviewRequest{Focus: "quick"})
	for _, want := range []string{`focus="quick"`, "linters and formatters only", "no security"} {
		if !strings.Contains(p, want) {
			t.Errorf("the quick prompt does not say %q:\n%s", want, p)
		}
	}
	// And the default prompt tells the model what to do when diff finds nothing,
	// because it is this prompt that sends it there.
	if d := reviewPrompt(reviewRequest{}); !strings.Contains(d, "nothing was in scope") {
		t.Errorf("the default prompt steers to diff=HEAD without covering an unchanged tree:\n%s", d)
	}
}

// GORILLA FIX (2026-10-06): security and full are different runs, and the
// prompt for each says what it runs. Until today both said only "pass
// focus=...", because both were the same --deep run underneath.
func TestTheSecurityAndFullPromptsSayWhatEachReallyRuns(t *testing.T) {
	sec := reviewPrompt(reviewRequest{Focus: "security"})
	for _, want := range []string{`focus="security"`, "secret scanners", "static", "deep pass", "no linters, no formatters", "left out"} {
		if !strings.Contains(sec, want) {
			t.Errorf("the security prompt does not say %q:\n%s", want, sec)
		}
	}
	full := reviewPrompt(reviewRequest{Focus: "full"})
	for _, want := range []string{`focus="full"`, "every analyser of every kind", "linters", "secret scan", "deep pass"} {
		if !strings.Contains(full, want) {
			t.Errorf("the full prompt does not say %q:\n%s", want, full)
		}
	}
	if strings.Contains(full, "no linters") {
		t.Errorf("the full prompt says linters are skipped:\n%s", full)
	}
}
