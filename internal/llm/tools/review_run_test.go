package tools

// GORILLA FIX (2026-10-05): tests for what the `review` tool says about its own
// run.
//
// The older tests in review_test.go check the SUMMARY: given a report, is it
// rendered honestly. None of them checked that the report was the one that had
// been asked for, or what happened when there was no report at all — and that
// is where every defect found on 2026-10-05 lived:
//
//   - "quick" sent a flag that ran the security stages, and the summary said
//     they had been skipped;
//   - logs were written into the folder under review;
//   - the three ordinary ways a review ends without a report (nothing changed,
//     no such path, not a git repository) all reached the model as a bare
//     failure, because stderr was thrown away;
//   - a readiness check that crashed counted as ready;
//   - nothing said which analysers reach the network.
//
// The pure functions are tested without Python. The end-to-end cases run the
// embedded toolkit for real and are skipped where there is no Python to run it.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/commands"
)

// R1. "quick" must ask the toolkit for its linters-and-formatters-only mode.
// --no-stage3 is the flag it used to send, and that flag runs stages 1 AND 2.
func TestQuickAsksTheToolkitForARealQuickPass(t *testing.T) {
	got := strings.Join(focusArgs("quick"), " ")
	if got != "--quick" {
		t.Errorf("focus=quick sends %q; it must send --quick", got)
	}
	if strings.Contains(got, "no-stage3") {
		t.Error("focus=quick sends --no-stage3, which still runs bandit, gosec, semgrep and gitleaks")
	}
	for _, f := range []string{"security", "full"} {
		if strings.Join(focusArgs(f), " ") != "--deep" {
			t.Errorf("focus=%s no longer forces the deep stage", f)
		}
	}
	if len(focusArgs("")) != 0 {
		t.Errorf("the standard pass sends depth flags: %v", focusArgs(""))
	}
}

// R1. The sentence "security was skipped entirely" is a claim about the run, so
// it may only be printed when the RUN says so. A report with no depth block —
// an older toolkit, or one that ignored the flag — must not have it asserted on
// its behalf.
func TestQuickClaimIsOnlyMadeWhenTheRunConfirmsIt(t *testing.T) {
	unconfirmed := mustJSON(t, map[string]any{
		"target": "/src", "findings": []map[string]any{}, "corroborated": []map[string]any{},
		"trust": map[string]any{"tools_ran": []string{"gosec", "pylint"}},
	})
	out, err := summariseReview(unconfirmed, "quick")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "SKIPPED ENTIRELY") || strings.Contains(out, "cannot have found") {
		t.Errorf("claimed the security checks were skipped on a report that does not say so "+
			"(and whose tools_ran lists gosec):\n%s", out)
	}
	if !strings.Contains(out, "did not confirm") {
		t.Errorf("a quick request the run did not confirm is not flagged as unconfirmed:\n%s", out)
	}

	standardRun := mustJSON(t, map[string]any{
		"target": "/src", "findings": []map[string]any{}, "corroborated": []map[string]any{},
		"trust": map[string]any{"tools_ran": []string{"gosec"}},
		"depth": map[string]any{"mode": "standard"},
	})
	out, err = summariseReview(standardRun, "quick")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "SKIPPED ENTIRELY") {
		t.Errorf("claimed a quick pass over a run that reports itself as standard:\n%s", out)
	}
}

// R5. Exit 0 with nothing on stdout is "nothing was in scope" — the normal
// result of diff=HEAD on an unchanged tree. It was reported as "The review
// failed to run: the review produced no output".
func TestNothingInScopeIsSaidAsSuchNotAsAFailure(t *testing.T) {
	text, isErr := interpretReviewRun(nil, []byte("Files in scope: 0\nNo files found in scope. Nothing to do.\n"), nil, "")
	if isErr {
		t.Error("an empty scope was reported as an error; it is a result")
	}
	if !strings.Contains(text, "NOTHING WAS IN SCOPE") {
		t.Errorf("an empty scope is not named as one:\n%s", text)
	}
	if strings.Contains(strings.ToLower(text), "failed to run") {
		t.Errorf("an empty scope is still described as a failure:\n%s", text)
	}
	if !strings.Contains(text, "No files found in scope") {
		t.Errorf("the toolkit's own explanation was dropped:\n%s", text)
	}
	if !strings.Contains(text, "Do not describe the code as reviewed") {
		t.Errorf("an empty scope could be read as a clean review:\n%s", text)
	}
}

// R5. A run that stops must say WHY. The reason is on stderr and nowhere else.
func TestAStoppedRunCarriesTheToolkitsReason(t *testing.T) {
	text, isErr := interpretReviewRun(nil,
		[]byte("ERROR: --diff was given but /src is not a git repository.\n"),
		errors.New("exit status 1"), "")
	if !isErr {
		t.Error("a run that exited non-zero with no report was not reported as an error")
	}
	if !strings.Contains(text, "not a git repository") {
		t.Errorf("the reason was lost; the model would see only an exit status:\n%s", text)
	}
	if !strings.Contains(text, "Do not describe the code as reviewed") {
		t.Errorf("a stopped run could be read as a review:\n%s", text)
	}
}

// R5. stderr is bounded: it is a tool result and is re-sent on every turn.
func TestTheToolkitsReasonIsBounded(t *testing.T) {
	var noisy strings.Builder
	for i := 0; i < 4000; i++ {
		noisy.WriteString("progress line that is not the reason\n")
	}
	noisy.WriteString("ERROR: the real reason\n")
	tail := stderrTail([]byte(noisy.String()))
	if !strings.Contains(tail, "the real reason") {
		t.Error("the last line — the one that says what went wrong — was cut")
	}
	if len(tail) > 2000 {
		t.Errorf("stderr tail is %d bytes; it must stay small", len(tail))
	}
	if stderrTail([]byte("  \n\n")) != "" {
		t.Error("blank stderr produced a heading with nothing under it")
	}
}

// exitErrorWithCode produces a real *exec.ExitError carrying the given code, by
// re-running this test binary. classifyDoctor inspects the concrete type, so a
// hand-made error would test nothing.
func exitErrorWithCode(t *testing.T, code string) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestReviewHelperProcessExit$")
	cmd.Env = append(os.Environ(), "GORILLA_REVIEW_HELPER_EXIT="+code)
	err := cmd.Run()
	if err == nil {
		t.Fatalf("helper process exited 0; wanted %s", code)
	}
	return err
}

// Not a test: the child half of exitErrorWithCode.
func TestReviewHelperProcessExit(t *testing.T) {
	switch os.Getenv("GORILLA_REVIEW_HELPER_EXIT") {
	case "1":
		os.Exit(1)
	case "3":
		os.Exit(3)
	}
}

// R6. Ready means the doctor exited 0. It used to mean "did not exit 3 and
// printed something", so a doctor that crashed after its first line passed.
func TestOnlyACleanDoctorCountsAsReady(t *testing.T) {
	if _, st := classifyDoctor([]byte("all good"), nil, nil); st != doctorReady {
		t.Error("a doctor that exited 0 is not ready")
	}
	if _, st := classifyDoctor([]byte("nothing installed"), nil, exitErrorWithCode(t, "3")); st != doctorNoAnalysers {
		t.Error("exit 3 — the documented 'no analyser installed' — was not recognised")
	}
	text, st := classifyDoctor([]byte("Checking analysers..."), []byte("Traceback (most recent call last):\nKeyError: 'ready'\n"),
		exitErrorWithCode(t, "1"))
	if st != doctorFailed {
		t.Fatalf("a doctor that crashed with partial output counted as state %d; it must be a failed check", st)
	}
	if !strings.Contains(text, "KeyError") {
		t.Errorf("the crash reason was dropped:\n%s", text)
	}
	if _, st := classifyDoctor(nil, nil, errors.New("exec: python: not found")); st != doctorFailed {
		t.Error("a doctor that could not start counted as something other than failed")
	}
}

// R3. The permission text is built from the toolkit's report: installed tools
// that run at this depth are named with what they fetch; the rest are not.
func TestNetworkNoteNamesWhatWillReachTheNetwork(t *testing.T) {
	rep := mustJSON(t, map[string]any{"tools": []map[string]any{
		{"id": "semgrep-fast", "label": "semgrep (p/ci ruleset)", "network": "downloads its rule packs from semgrep.dev.", "installed": true, "runs": true},
		{"id": "cargo-audit", "label": "cargo audit", "network": "downloads the advisory database.", "installed": false, "runs": true},
		{"id": "gosec", "label": "gosec (Go security)", "network": "the Go toolchain downloads modules.", "installed": true, "runs": false},
	}})
	n := describeNetwork(rep)
	if !n.egress {
		t.Error("an installed analyser that fetches from the network did not mark the request as egress")
	}
	if !strings.Contains(n.text, "semgrep") || !strings.Contains(n.text, "semgrep.dev") {
		t.Errorf("the installed network-using analyser is not named with what it does:\n%s", n.text)
	}
	if strings.Contains(n.text, "cargo audit") {
		t.Errorf("named an analyser that is not installed and so cannot contact anything:\n%s", n.text)
	}
	if strings.Contains(n.text, "gosec") {
		t.Errorf("named an analyser the chosen depth does not run:\n%s", n.text)
	}

	none := describeNetwork(mustJSON(t, map[string]any{"tools": []map[string]any{
		{"id": "cargo-audit", "label": "cargo audit", "network": "x", "installed": false, "runs": true},
	}}))
	if none.egress {
		t.Error("marked egress when no network-using analyser is installed")
	}
	if !strings.Contains(none.text, "none of the analysers") {
		t.Errorf("did not say plainly that nothing contacts the network:\n%s", none.text)
	}

	// Not knowing is not the same as "nothing will".
	if bad := describeNetwork([]byte("not json")); !bad.egress || !strings.Contains(bad.text, "Assume some will") {
		t.Errorf("an unreadable report was treated as 'no network use': %+v", bad)
	}
}

// R2. Results live under the program's cache directory, never in the target.
func TestResultsAreNeverWrittenIntoTheReviewedFolder(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	target := t.TempDir()

	dir, err := newReviewResultsDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(filepath.Clean(dir), filepath.Clean(cache)) {
		t.Errorf("results dir %s is not under the cache dir %s", dir, cache)
	}
	if rel, err := filepath.Rel(target, dir); err == nil && !strings.HasPrefix(rel, "..") {
		t.Errorf("results dir %s is inside the reviewed folder %s", dir, target)
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Errorf("creating the results dir wrote into the reviewed folder: %v", entries)
	}

	// Two runs in the same second must not share a folder.
	dir2, err := newReviewResultsDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if dir2 == dir {
		t.Error("two runs were given the same results folder")
	}
}

// R2. Old result folders are pruned, and the one just created never is.
func TestOldResultsArePrunedAndTheNewOneKept(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := reviewResultsRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < reviewKeepResults+7; i++ {
		// Older than anything a real run would be named.
		name := "19990101_0000" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + "-old"
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	dir, err := newReviewResultsDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) > reviewKeepResults {
		t.Errorf("%d result folders kept; the cap is %d", len(entries), reviewKeepResults)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the folder for the run in progress was pruned: %v", err)
	}
}

// ── end to end, against the embedded toolkit ─────────────────────────────

func toolkitForTest(t *testing.T) toolkitRunner {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("codereview", "toolkit", "code_review.py"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("script not present: %v", err)
	}
	py, pre, err := getPythonBinary()
	if err != nil {
		t.Skipf("no python: %v", err)
	}
	return toolkitRunner{python: py, preArgs: pre, script: script}
}

func writeReviewFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// R1 + R2, for real: a quick run reports itself as quick, lists no security,
// secrets or static-analysis tool in ANY state, and leaves the target untouched.
func TestARealQuickRunSchedulesNoSecurityToolAndLeavesTheTargetAlone(t *testing.T) {
	tk := toolkitForTest(t)
	target := t.TempDir()
	writeReviewFixture(t, filepath.Join(target, "a.py"), "import os\nx = 1\n")
	writeReviewFixture(t, filepath.Join(target, "a.go"), "package main\n\nfunc main() {}\n")
	writeReviewFixture(t, filepath.Join(target, "go.mod"), "module example.test/x\n\ngo 1.21\n")
	results := t.TempDir()

	args := append([]string{target, "--audience", "agent", "--results-dir", results, "--skip-preflight"},
		focusArgs("quick")...)
	stdout, stderr, err := tk.run(context.Background(), args...)
	if err != nil {
		t.Fatalf("quick run failed: %v\n%s", err, stderr)
	}
	var rep agentReport
	if err := json.Unmarshal(stdout, &rep); err != nil {
		t.Fatalf("report did not decode: %v\n%s", err, stdout)
	}
	if rep.Depth == nil || rep.Depth.Mode != "quick" {
		t.Fatalf("the run does not report itself as quick: %+v", rep.Depth)
	}
	if len(rep.Depth.ToolsSkipped) == 0 {
		t.Error("a quick pass over Python and Go named no analyser it left out")
	}

	// Every tool the run touched, in whatever state, must be one a quick pass
	// is allowed to run. Before the fix this list held bandit, gosec,
	// staticcheck, mypy, semgrep-fast and gitleaks-worktree.
	touched := map[string]bool{}
	for _, list := range [][]string{rep.Trust.ToolsRan, rep.Trust.ToolsMissing, rep.Trust.ToolsErrored, rep.Trust.ToolsTimedOut} {
		for _, id := range list {
			touched[id] = true
		}
	}
	for id := range securityToolIDs {
		if touched[id] {
			t.Errorf("a QUICK pass scheduled the security tool %s", id)
		}
	}
	for _, id := range rep.Depth.ToolsSkipped {
		if touched[id] {
			t.Errorf("%s is reported both as left out by the depth and as scheduled", id)
		}
	}

	entries, _ := os.ReadDir(target)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "a.go,a.py,go.mod" {
		t.Errorf("the review wrote into the folder it was reviewing: %v", names)
	}
	if rep.ResultsDir == "" || !strings.HasPrefix(filepath.Clean(rep.ResultsDir), filepath.Clean(results)) {
		t.Errorf("results went to %q, not the directory the tool passed (%s)", rep.ResultsDir, results)
	}

	// And the summary built from this real report makes the claim, with names.
	summary, err := summariseReview(stdout, "quick")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "SKIPPED ENTIRELY") || !strings.Contains(summary, rep.Depth.ToolsSkipped[0]) {
		t.Errorf("the summary of a confirmed quick run does not say what was left out:\n%s", summary)
	}
}

// R5, for real: a path that does not exist.
func TestAMissingPathIsReportedWithItsReason(t *testing.T) {
	tk := toolkitForTest(t)
	missing := filepath.Join(t.TempDir(), "no-such-folder")
	stdout, stderr, err := tk.run(context.Background(), missing, "--audience", "agent", "--results-dir", t.TempDir())
	text, isErr := interpretReviewRun(stdout, stderr, err, "")
	if !isErr {
		t.Errorf("a review of a path that does not exist was not an error:\n%s", text)
	}
	if !strings.Contains(text, "does not exist") {
		t.Errorf("the model is not told the path does not exist:\n%s", text)
	}
}

// R5, for real: diff=HEAD on a tree with nothing changed.
func TestDiffOnAnUnchangedTreeIsNothingInScope(t *testing.T) {
	tk := toolkitForTest(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("no git: %v", err)
	}
	repo := t.TempDir()
	writeReviewFixture(t, filepath.Join(repo, "a.py"), "x = 1\n")
	for _, a := range [][]string{
		{"init", "-q"},
		{"add", "a.py"},
		{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "one"},
	} {
		cmd := exec.Command(git, a...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v failed, cannot build the fixture: %v\n%s", a, err, out)
		}
	}
	stdout, stderr, runErr := tk.run(context.Background(), repo, "--audience", "agent",
		"--results-dir", t.TempDir(), "--diff", "HEAD")
	text, isErr := interpretReviewRun(stdout, stderr, runErr, "")
	if isErr {
		t.Errorf("an unchanged tree was reported as a failed review:\n%s", text)
	}
	if !strings.Contains(text, "NOTHING WAS IN SCOPE") {
		t.Errorf("an unchanged tree is not reported as nothing in scope:\n%s", text)
	}
}

// R5, for real: --diff outside a git repository.
func TestDiffOutsideARepositorySaysSo(t *testing.T) {
	tk := toolkitForTest(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git: %v", err)
	}
	dir := t.TempDir()
	writeReviewFixture(t, filepath.Join(dir, "a.py"), "x = 1\n")
	stdout, stderr, runErr := tk.run(context.Background(), dir, "--audience", "agent",
		"--results-dir", t.TempDir(), "--diff", "HEAD")
	text, isErr := interpretReviewRun(stdout, stderr, runErr, "")
	if !isErr {
		// A temp dir nested inside some outer repository is a possible host
		// layout; there the toolkit legitimately finds a repository.
		t.Skipf("the temp dir appears to be inside a git repository:\n%s", text)
	}
	if !strings.Contains(text, "not a git repository") && !strings.Contains(text, "git diff") {
		t.Errorf("the model is not told why --diff could not be used:\n%s", text)
	}
}

// R3, for real: the report the permission prompt is built from decodes, and on
// a quick pass marks the security-category network tools as not running.
func TestTheRealNetworkReportDecodes(t *testing.T) {
	tk := toolkitForTest(t)
	target := t.TempDir()
	writeReviewFixture(t, filepath.Join(target, "a.go"), "package main\n\nfunc main() {}\n")
	writeReviewFixture(t, filepath.Join(target, "a.py"), "x = 1\n")

	read := func(extra ...string) []networkTool {
		stdout, stderr, err := tk.run(context.Background(), append([]string{target, "--network-report"}, extra...)...)
		if err != nil {
			t.Fatalf("--network-report failed: %v\n%s", err, stderr)
		}
		var rep struct {
			Tools []networkTool `json:"tools"`
		}
		if err := json.Unmarshal(stdout, &rep); err != nil {
			t.Fatalf("network report did not decode: %v\n%s", err, stdout)
		}
		return rep.Tools
	}

	std := read()
	if len(std) == 0 {
		t.Fatal("no network-using analyser reported for a Go and Python tree; semgrep and the Go tools apply")
	}
	for _, tl := range std {
		if tl.ID == "" || tl.Label == "" || tl.Network == "" {
			t.Errorf("incomplete entry: %+v", tl)
		}
		if !tl.Runs {
			t.Errorf("%s is marked as not running on a standard pass", tl.ID)
		}
	}
	for _, tl := range read(focusArgs("quick")...) {
		if securityToolIDs[tl.ID] && tl.Runs {
			t.Errorf("the security tool %s is marked as running on a quick pass", tl.ID)
		}
	}
}

// R11. The tools treated as security-only are the registry's own security and
// secrets analysers — no stale id, none missing.
func TestSecurityToolIDsAreTheRegistrysOwn(t *testing.T) {
	tk := toolkitForTest(t)
	code := "import json, tools_registry as r; print(json.dumps(sorted(t.id for t in r.TOOLS " +
		"if t.category in ('security', 'secrets') and t.scope in ('auto-file', 'auto-project'))))"
	cmd := exec.Command(tk.python, append(append([]string{}, tk.preArgs...), "-c", code)...)
	cmd.Dir = filepath.Dir(tk.script)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("could not read the registry: %v", err)
	}
	var ids []string
	if err := json.Unmarshal(out, &ids); err != nil {
		t.Fatalf("registry ids did not decode: %v\n%s", err, out)
	}
	inRegistry := map[string]bool{}
	for _, id := range ids {
		inRegistry[id] = true
		if !securityToolIDs[id] {
			t.Errorf("%s is a security or secrets analyser in the registry and is missing from securityToolIDs; "+
				"its findings would be dropped from a security-focused report unless their wording matched", id)
		}
	}
	for id := range securityToolIDs {
		if !inRegistry[id] {
			t.Errorf("securityToolIDs names %q, which the toolkit never emits", id)
		}
	}
}

// The toolkit's own check of the same contract, from the Python side.
func TestToolkitDepthAndResolutionChecksHold(t *testing.T) {
	tk := toolkitForTest(t)
	script := filepath.Join(filepath.Dir(tk.script), "tests", "test_depth.py")
	out, err := exec.Command(tk.python, append(append([]string{}, tk.preArgs...), script)...).CombinedOutput()
	if err != nil {
		t.Errorf("the toolkit's own check test_depth.py failed: %v\n%s", err, out)
	}
}

// Found while fixing R5 (2026-10-05): a C tree could never be summarised.
//
// manual_steps is a list of objects and the Go struct declared []string, so the
// decode failed on every report that carried a manual step — every C or C++
// review — after the analysers had already run. The same report's logs_dir was
// being read as results_dir, so the path to the full report was never shown.
func TestACTreeReportDecodesAndSaysWhereTheFullReportIs(t *testing.T) {
	tk := toolkitForTest(t)
	target := t.TempDir()
	writeReviewFixture(t, filepath.Join(target, "a.c"), "int main(void) { return 0; }\n")
	results := t.TempDir()

	stdout, stderr, err := tk.run(context.Background(), target, "--audience", "agent",
		"--results-dir", results, "--skip-preflight")
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, stderr)
	}
	var rep agentReport
	if err := json.Unmarshal(stdout, &rep); err != nil {
		t.Fatalf("a C tree's report does not decode with the real struct: %v", err)
	}
	if len(rep.ManualSteps) == 0 || rep.ManualSteps[0].Label == "" {
		t.Errorf("manual steps did not decode: %+v", rep.ManualSteps)
	}
	if !strings.HasPrefix(filepath.Clean(rep.ResultsDir), filepath.Clean(results)) {
		t.Errorf("the report's own results path decoded as %q; the run wrote to %s", rep.ResultsDir, results)
	}
	summary, err := summariseReview(stdout, "")
	if err != nil {
		t.Fatalf("summariseReview: %v", err)
	}
	if !strings.Contains(summary, "Full report") || !strings.Contains(summary, filepath.Base(results)) {
		t.Errorf("the summary does not say where the full report is:\n%s", summary)
	}
	if !strings.Contains(summary, rep.ManualSteps[0].Label) {
		t.Errorf("the manual steps are not offered:\n%s", summary)
	}
}

// In agent mode stdout is the JSON report or it is empty. The preflight used to
// print the doctor's page of prose there, so a refused run looked like a
// corrupt report.
func TestARefusedRunPutsNothingOnStdout(t *testing.T) {
	tk := toolkitForTest(t)
	target := t.TempDir()
	// A language no analyser on any machine handles without being installed,
	// run WITHOUT --skip-preflight so the gate is what answers.
	writeReviewFixture(t, filepath.Join(target, "a.c"), "int main(void) { return 0; }\n")
	stdout, stderr, err := tk.run(context.Background(), append([]string{target, "--audience", "agent",
		"--results-dir", t.TempDir()}, focusArgs("quick")...)...)
	if err == nil {
		// A C linter is installed here; the gate passed. The stdout contract
		// still holds and is the thing under test.
		var rep agentReport
		if jerr := json.Unmarshal(stdout, &rep); jerr != nil {
			t.Fatalf("stdout is not the JSON report: %v", jerr)
		}
		return
	}
	if len(strings.TrimSpace(string(stdout))) != 0 {
		t.Errorf("a refused run wrote %d bytes of non-report text to stdout:\n%.300s", len(stdout), stdout)
	}
	if !strings.Contains(string(stderr), "PREFLIGHT FAILED") {
		t.Errorf("the refusal is not explained on stderr:\n%s", stderr)
	}
	text, isErr := interpretReviewRun(stdout, stderr, err, "quick")
	if !isErr || !strings.Contains(text, "without focus=\"quick\"") {
		t.Errorf("a quick pass refused for want of a linter does not say a standard review may work:\n%s", text)
	}
}

// The preflight's refusal (exit 3) is named, not left as "exit status 3".
func TestAPreflightRefusalIsExplained(t *testing.T) {
	text, isErr := interpretReviewRun(nil, []byte("PREFLIGHT FAILED\n"), exitErrorWithCode(t, "3"), "")
	if !isErr {
		t.Error("a refused run was not an error")
	}
	if !strings.Contains(text, "none of the analysers") || !strings.Contains(text, "Do not describe the code as reviewed") {
		t.Errorf("the refusal is not explained:\n%s", text)
	}
	if strings.Contains(text, "focus=\"quick\"") {
		t.Errorf("advice about quick was given on a run that was not quick:\n%s", text)
	}
}

// R3. The /review help names the analysers that reach the network. That list
// is prose, so it is checked against the registry: a tool given a network note
// later, and not mentioned to the user, fails here.
func TestTheHelpNamesEveryAnalyserThatReachesTheNetwork(t *testing.T) {
	tk := toolkitForTest(t)
	code := "import json, tools_registry as r; print(json.dumps(sorted(set(" +
		"(t.check_cmd[0] if t.check_cmd else t.id) for t in r.TOOLS if t.network))))"
	cmd := exec.Command(tk.python, append(append([]string{}, tk.preArgs...), "-c", code)...)
	cmd.Dir = filepath.Dir(tk.script)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("could not read the registry: %v", err)
	}
	var programs []string
	if err := json.Unmarshal(out, &programs); err != nil {
		t.Fatalf("did not decode: %v\n%s", err, out)
	}
	if len(programs) == 0 {
		t.Fatal("the registry lists no analyser that reaches the network; semgrep does")
	}
	review := commands.ByName("review")
	if review == nil {
		t.Fatal("/review is not in the command registry")
	}
	help := strings.ToLower(review.Detail)
	// The help speaks of toolchains, not of each binary that goes through one:
	// golangci-lint, staticcheck and gosec all download through Go.
	family := map[string]string{
		"semgrep": "semgrep", "cargo": "cargo audit",
		"go": "go and rust tools", "golangci-lint": "go and rust tools",
		"staticcheck": "go and rust tools", "gosec": "go and rust tools",
	}
	for _, p := range programs {
		want, known := family[p]
		if !known {
			t.Errorf("%s reaches the network (it has a network note in the registry) and the /review "+
				"help has no wording for it; add it to the help and to this table", p)
			continue
		}
		if !strings.Contains(help, want) {
			t.Errorf("%s reaches the network and the /review help does not mention %q", p, want)
		}
	}
	for _, gone := range []string{"nothing is downloaded", "thirty", "30 real"} {
		if strings.Contains(help, gone) {
			t.Errorf("the /review help still says %q", gone)
		}
	}
	for _, must := range []string{"python 3", "cache folder", "nothing is written into the folder being reviewed"} {
		if !strings.Contains(help, must) {
			t.Errorf("the /review help no longer says %q", must)
		}
	}
}

// R7. No typed analyser count in what the model is told about the tool.
func TestTheToolDescriptionCarriesNoTypedAnalyserCount(t *testing.T) {
	desc := strings.ToLower((&reviewTool{}).Info().Description)
	for _, typed := range []string{"thirty", "30 ", "~30"} {
		if strings.Contains(desc, typed) {
			t.Errorf("the review tool description states an analyser count (%q); the registry "+
				"is the only place that number is true, and it changes", typed)
		}
	}
	for _, must := range []string{"python 3", "network", "never into the folder being reviewed"} {
		if !strings.Contains(desc, must) {
			t.Errorf("the review tool description no longer says %q", must)
		}
	}
}
