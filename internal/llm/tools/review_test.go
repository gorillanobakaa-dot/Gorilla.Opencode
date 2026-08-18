package tools

// GORILLA OVERRIDE (2026-08-18): tests for the `review` tool.
//
// One property matters more than the rest and every test here circles it: a
// review that found nothing because nothing was INSTALLED must never read like
// a review that found nothing because the code is clean. A model with two
// billion parameters cannot tell those apart from a findings list, so the
// distinction has to be structural — in the ordering, in the wording, and in
// the refusal.

import (
	"encoding/json"
	"strings"
	"testing"
)

// A report where half the analysers never ran must SAY so before it says
// anything else.
func TestTrustBlockComesBeforeAnyFinding(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"target": "/src/proj", "profile": "go", "files_scanned": 12,
		"languages": []string{"go"},
		"findings": []map[string]any{
			{"tool": "gosec", "file": "a.go", "line": 4, "severity": "high", "message": "hardcoded credentials"},
		},
		"corroborated": []map[string]any{},
		"trust": map[string]any{
			"tools_ran":     []string{"gofmt"},
			"tools_missing": []string{"gosec", "staticcheck", "golangci-lint"},
			"tools_errored": []string{"semgrep"},
		},
	})

	out, err := summariseReview(raw)
	if err != nil {
		t.Fatal(err)
	}

	iTrust := strings.Index(out, "## Trust")
	iFind := strings.Index(out, "## All findings")
	if iTrust < 0 || iFind < 0 || iTrust > iFind {
		t.Errorf("the trust block does not precede the findings; a reader could stop early and conclude the wrong thing")
	}
	for _, want := range []string{
		"NOT INSTALLED",
		"UNREVIEWED",
		"Failed to run",
		"do not find semantic bugs",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the summary never says %q", want)
		}
	}
}

// The sentence that stops the worst misreading.
func TestNoFindingsIsNotReportedAsClean(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"target": "/src/proj", "files_scanned": 3,
		"findings": []map[string]any{}, "corroborated": []map[string]any{},
		"trust": map[string]any{"tools_ran": []string{"gofmt"}},
	})
	out, err := summariseReview(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "not the same as clean") {
		t.Errorf("an empty corroborated list is presented without the warning that it is not a pass:\n%s", out)
	}
}

// Every tool result is re-sent on every later turn, so an unbounded result is a
// recurring bill — the grep lesson, in a different tool.
func TestFindingsAreBoundedAndTruncationIsAnnounced(t *testing.T) {
	var findings []map[string]any
	for i := range 500 {
		findings = append(findings, map[string]any{
			"tool": "cppcheck", "file": "x.c", "line": i, "severity": "style",
			"message": strings.Repeat("a very long analyser message ", 40),
		})
	}
	raw := mustJSON(t, map[string]any{
		"target": "/src", "findings": findings, "corroborated": []map[string]any{},
		"trust": map[string]any{"tools_ran": []string{"cppcheck"}},
	})

	out, err := summariseReview(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, "- [style]"); n > reviewMaxFindings {
		t.Errorf("emitted %d findings, cap is %d", n, reviewMaxFindings)
	}
	if !strings.Contains(out, "further findings were not listed") {
		t.Errorf("findings were dropped without saying so — a model reasons about a silent fragment as if it were complete")
	}
	if len(out) > 60_000 {
		t.Errorf("summary is %d bytes; it rides every later turn", len(out))
	}
	// The total must still be honest even though the list is cut.
	if !strings.Contains(out, "All findings: 500") {
		t.Errorf("the real total is not stated")
	}
}

// Severity ordering: a reader who only reads the first few lines must get the
// dangerous ones.
func TestMostSevereFindingsComeFirst(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"target": "/src",
		"findings": []map[string]any{
			{"tool": "a", "file": "f", "line": 1, "severity": "style", "message": "spacing"},
			{"tool": "b", "file": "f", "line": 2, "severity": "error", "message": "buffer overrun"},
			{"tool": "c", "file": "f", "line": 3, "severity": "warning", "message": "unused"},
		},
		"corroborated": []map[string]any{},
		"trust":        map[string]any{"tools_ran": []string{"a"}},
	})
	out, err := summariseReview(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(out, "buffer overrun") > strings.Index(out, "spacing") {
		t.Errorf("a style nit is listed above a buffer overrun")
	}
}

// Corroborated findings are the highest-confidence material and are never cut.
func TestCorroboratedFindingsAreNeverTruncated(t *testing.T) {
	var corr []map[string]any
	for i := range 120 {
		corr = append(corr, map[string]any{
			"file": "f.c", "line": i, "tools": []string{"cppcheck", "clang-tidy"},
			"messages": []string{"null dereference"},
		})
	}
	raw := mustJSON(t, map[string]any{
		"target": "/src", "findings": []map[string]any{}, "corroborated": corr,
		"trust": map[string]any{"tools_ran": []string{"cppcheck", "clang-tidy"}},
	})
	out, err := summariseReview(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, "[cppcheck+clang-tidy]"); n != 120 {
		t.Errorf("emitted %d of 120 corroborated findings; these are the ones worth reading and must not be cut", n)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
