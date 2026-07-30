package tools

import (
	"strings"
	"testing"
)

// A search that reported "100 matches, truncated" still returned 2,438,026
// bytes on 2026-07-30, because the match COUNT was capped and the byte size was
// not. That result took a conversation from 15.9K tokens to 675K in one turn.
// Capping matches alone is not a bound.
func TestOneAbsurdLineCannotBlowUpTheResult(t *testing.T) {
	// The real shape: a source file embedded as one escaped JSON string.
	line := `"after": "package tui\n\nimport (` + strings.Repeat("x", 66000)
	got := clampMatchLine(line)

	if len(got) >= len(line) {
		t.Fatalf("a %d-byte line came back at %d bytes; it was not clamped",
			len(line), len(got))
	}
	if len(got) > maxGrepLineBytes+64 {
		t.Errorf("clamped to %d bytes, want about %d", len(got), maxGrepLineBytes)
	}
	// The model must be able to tell it received a fragment. Silently cutting a
	// line makes it reason about the fragment as if it were the whole thing.
	if !strings.Contains(got, "truncated") {
		t.Errorf("clamped line does not say it was truncated: %q", got[len(got)-60:])
	}
	if !strings.Contains(got, "66") {
		t.Errorf("clamped line does not report the original size: %q", got[len(got)-60:])
	}
}

func TestShortLinesAreUntouched(t *testing.T) {
	for _, s := range []string{"", "func main() {", strings.Repeat("a", maxGrepLineBytes)} {
		if got := clampMatchLine(s); got != s {
			t.Errorf("a %d-byte line was altered", len(s))
		}
	}
}

// Per-line clamping alone is not enough: a thousand merely-large lines still
// add up. Both limits exist because either one alone leaks.
func TestTheTwoLimitsCoverDifferentFailures(t *testing.T) {
	if maxGrepOutputBytes <= maxGrepLineBytes {
		t.Fatal("the total cap must exceed the per-line cap or it is meaningless")
	}
	worst := maxGrepOutputBytes + maxGrepLineBytes + 128
	if worst > 200*1024 {
		t.Errorf("worst-case output is %d bytes; that is still enough to hurt", worst)
	}
	t.Logf("worst case: ~%d KB, versus the 2,438,026 bytes actually observed",
		worst/1024)
}
