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
	"os/exec"
	"path/filepath"
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

	out, err := summariseReview(raw, "")
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
	out, err := summariseReview(raw, "")
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

	out, err := summariseReview(raw, "")
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
	out, err := summariseReview(raw, "")
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
	out, err := summariseReview(raw, "")
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

// A quick pass that does not say it skipped the security stage is the same lie
// as an analyser that was never installed: the reader is left believing the
// code was checked for things nobody looked for.
func TestEachDepthDeclaresWhatItSkipped(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"target": "/src", "findings": []map[string]any{}, "corroborated": []map[string]any{},
		"trust": map[string]any{"tools_ran": []string{"gofmt"}},
	})

	// The quick report carries the run's own depth block. Since 2026-10-05 the
	// "skipped entirely" sentence is only printed when the run confirms it —
	// see TestQuickClaimIsOnlyMadeWhenTheRunConfirmsIt for the other half.
	rawQuick := mustJSON(t, map[string]any{
		"target": "/src", "findings": []map[string]any{}, "corroborated": []map[string]any{},
		"trust": map[string]any{"tools_ran": []string{"gofmt"}},
		"depth": map[string]any{
			"mode": "quick", "categories_run": []string{"recon", "lint", "format"},
			"categories_skipped":     []string{"secrets", "security", "static-analysis"},
			"tools_skipped_by_depth": []string{"gosec", "staticcheck"},
		},
	})
	quick, err := summariseReview(rawQuick, "quick")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DEPTH: quick", "SKIPPED ENTIRELY", "cannot have found", "gosec", "staticcheck"} {
		if !strings.Contains(quick, want) {
			t.Errorf("a quick pass does not admit what it skipped — missing %q", want)
		}
	}
	// And it must be inside the trust block, where a reader looking for "what
	// was not checked" will find it.
	if strings.Index(quick, "DEPTH: quick") > strings.Index(quick, "## Corroborated") {
		t.Errorf("the depth note is below the findings; it belongs with the other 'what was not checked' facts")
	}

	sec, err := summariseReview(raw, "security")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sec, "deliberately left out") {
		t.Errorf("focus=security narrows the report without saying what it dropped")
	}

	std, err := summariseReview(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(std, "escalating automatically") {
		t.Errorf("the default depth does not explain that the deep pass self-escalates")
	}
}

// focus=security narrows the list but must never hide the real total.
func TestSecurityFocusNarrowsTheListAndStatesTheTotal(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"target": "/src",
		"findings": []map[string]any{
			{"tool": "gosec", "file": "a.go", "line": 1, "severity": "high", "message": "hardcoded credentials"},
			{"tool": "cppcheck", "file": "b.c", "line": 2, "severity": "error", "message": "buffer overrun (CWE-120)"},
			{"tool": "gofmt", "file": "c.go", "line": 3, "severity": "style", "message": "formatting"},
			{"tool": "pylint", "file": "d.py", "line": 4, "severity": "warning", "message": "unused variable"},
		},
		"corroborated": []map[string]any{},
		"trust":        map[string]any{"tools_ran": []string{"gosec"}},
	})

	out, err := summariseReview(raw, "security")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Security findings: 2 (of 4 total") {
		t.Errorf("the narrowing is not stated with the real total:\n%s", out)
	}
	if !strings.Contains(out, "hardcoded credentials") || !strings.Contains(out, "buffer overrun") {
		t.Errorf("a real security finding was filtered out")
	}
	if strings.Contains(out, "unused variable") || strings.Contains(out, "- [style]") {
		t.Errorf("style noise survived a security-focused report")
	}
}

// The security filter reads the finding, not the tool. The same analyser emits
// both a formatting nit and a command injection.
func TestSecurityFilterJudgesTheFindingNotOnlyTheTool(t *testing.T) {
	if !looksSecurity("warning", "possible command injection in exec call", "", "cppcheck") {
		t.Error("an injection reported by a general-purpose linter was not treated as security")
	}
	if !looksSecurity("error", "buffer overrun", "CWE-120", "clang-tidy") {
		t.Error("a CWE-tagged finding was not treated as security")
	}
	if looksSecurity("style", "line too long", "", "pylint") {
		t.Error("a formatting nit was treated as security")
	}
	// Dedicated security tools count regardless of wording.
	if !looksSecurity("low", "anything at all", "", "gitleaks-worktree") {
		t.Error("a dedicated secrets scanner's finding was dropped")
	}
}

// GORILLA OVERRIDE (2026-10-04): the verdict is stated, not left to be derived.
//
// The report that prompted this said 18 analysers ran, 17 were not installed,
// and found nothing — with the same names in both lists. A model reading that
// has to notice the overlap to learn that nothing was reviewed. The coverage
// block says it in the first line of the trust section.
func TestNothingRanIsSaidOutrightBeforeTheLists(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"target": "/src/proj", "files_scanned": 40,
		"findings": []map[string]any{}, "corroborated": []map[string]any{},
		"trust": map[string]any{"tools_ran": []string{}, "tools_missing": []string{"bandit", "gosec"}},
		"coverage": map[string]any{
			"terminal_state": "nothing-ran", "jobs_planned": 17,
			"jobs":                 map[string]int{"completed": 0, "missing": 17},
			"languages_unreviewed": []string{"go", "python"},
		},
	})
	out, err := summariseReview(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	iVerdict := strings.Index(out, "NOTHING WAS REVIEWED")
	iLists := strings.Index(out, "Analysers that ran")
	if iVerdict < 0 {
		t.Fatalf("a run in which no analyser completed is not called what it is:\n%s", out)
	}
	if iLists < 0 || iVerdict > iLists {
		t.Errorf("the verdict comes after the lists; it has to be the first thing read")
	}
}

func TestAPartialReviewNamesTheLanguagesNothingLookedAt(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"target": "/src/proj", "files_scanned": 9,
		"findings": []map[string]any{}, "corroborated": []map[string]any{},
		"trust": map[string]any{"tools_ran": []string{"flake8"}},
		"coverage": map[string]any{
			"terminal_state": "partial", "jobs_planned": 5,
			"jobs":                 map[string]int{"completed": 2, "missing": 3},
			"languages_unreviewed": []string{"go"},
		},
	})
	out, err := summariseReview(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"PARTIAL REVIEW", "2 of 5", "NO completed analyser: go"} {
		if !strings.Contains(out, want) {
			t.Errorf("the summary never says %q:\n%s", want, out)
		}
	}
}

// A report from a toolkit that predates the coverage block must still render.
func TestAReportWithoutCoverageStillRenders(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"target": "/src/proj", "files_scanned": 1,
		"findings": []map[string]any{}, "corroborated": []map[string]any{},
		"trust": map[string]any{"tools_ran": []string{"gofmt"}},
	})
	out, err := summariseReview(raw, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "NOTHING WAS REVIEWED") || strings.Contains(out, "PARTIAL REVIEW") {
		t.Errorf("a verdict was invented for a report that carried no coverage block:\n%s", out)
	}
}

// The Python side of the same contract: a tool that was only scheduled is never
// listed as having run. Runs the toolkit's own check; skipped without Python.
func TestToolkitCoverageAccountingHolds(t *testing.T) {
	py, err := exec.LookPath("python")
	if err != nil {
		if py, err = exec.LookPath("python3"); err != nil {
			t.Skipf("no python: %v", err)
		}
	}
	for _, script := range []string{"test_coverage.py", "test_rules.py"} {
		out, err := exec.Command(py, filepath.Join("codereview", "toolkit", "tests", script)).CombinedOutput()
		if err != nil {
			t.Errorf("the toolkit's own check %s failed: %v\n%s", script, err, out)
		}
	}
}
