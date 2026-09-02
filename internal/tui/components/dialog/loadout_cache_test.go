package dialog

// GORILLA OVERRIDE (2026-09-02): "the provider said nothing" and "nothing was
// cached" must never render the same way.
//
// Prompt caching was worth 8m14s down to 15s on this project's own hardware,
// measured, and nothing in the interface reported it. The fault that had been
// destroying it -- a timestamp inside the cached prefix -- survived unnoticed
// for exactly that reason.
//
// The obvious fix is a "cached tokens" figure. On this machine that would read
// zero forever: LM Studio sends no prompt_tokens_details at all (measured
// 2026-09-02, two identical requests, no such field in either usage object)
// while demonstrably reusing the prefix. So a bare zero would report working
// caching as broken, and the person reading it would go looking for a fault
// that is not there.
//
// internal/llm/prompt/prompt_stability_test.go is the guard that stops the
// regression. This is the guard that stops the REPORT of it from lying.

import (
	"strings"
	"testing"
)

// render returns the cache line for a given state, by driving the same code the
// dialog uses rather than reimplementing the wording here. A test that restated
// the strings would pass while the screen said something else.
func cacheLineFor(t *testing.T, input, read, create int64, reported bool) string {
	t.Helper()
	m := &loadoutDialogCmp{termWidth: 100}
	m.SetLastUsage(input, read, create, reported)
	out := m.renderAt(6, false)
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "cache reuse") {
			return line
		}
	}
	return ""
}

func TestAnUnreportingProviderIsNotCalledZero(t *testing.T) {
	got := cacheLineFor(t, 6000, 0, 0, false)
	if got == "" {
		t.Fatal("no cache line rendered at all")
	}
	if strings.Contains(got, "none") || strings.Contains(got, "0%") {
		t.Errorf("a provider that reports nothing was rendered as nothing cached:\n  %q\n"+
			"LM Studio reports no cache figures while reusing the prefix, so this "+
			"would call working caching broken and send someone hunting a fault "+
			"that does not exist.", strings.TrimSpace(got))
	}
	if !strings.Contains(got, "does not report") {
		t.Errorf("the line does not say the figure is unavailable:\n  %q", strings.TrimSpace(got))
	}
}

// A provider that DOES report, and genuinely cached nothing, must say so
// plainly. That is the state a regression actually produces, and it has to be
// distinguishable from the one above.
func TestAGenuineCacheMissSaysSo(t *testing.T) {
	got := cacheLineFor(t, 6000, 0, 0, true)
	if !strings.Contains(got, "none") {
		t.Errorf("a real cache miss did not report itself:\n  %q", strings.TrimSpace(got))
	}
	if strings.Contains(got, "does not report") {
		t.Errorf("a real cache miss was rendered as an unreported one:\n  %q\n"+
			"these are opposite conditions and must not share wording",
			strings.TrimSpace(got))
	}
}

// The working case has to show the actual saving, because a number that only
// ever appears when something is wrong teaches nobody what right looks like.
func TestAWorkingCacheShowsTheSaving(t *testing.T) {
	got := cacheLineFor(t, 76, 6408, 0, true)
	for _, want := range []string{"6,408", "cache"} {
		if !strings.Contains(got, want) {
			t.Errorf("the cache line does not mention %q:\n  %q", want, strings.TrimSpace(got))
		}
	}
	// 6408 of 6484 is 98%. The exact figure matters less than it being high and
	// present; a percentage that silently read 0 would be the original fault in
	// a new costume.
	if strings.Contains(got, "0%") {
		t.Errorf("a 6,408-token cache read rendered as 0%%:\n  %q", strings.TrimSpace(got))
	}
}
