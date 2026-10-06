package tools

// GORILLA OVERRIDE (2026-08-18): the `review` tool — point it at a folder, a
// file or a diff and it drives the real static analysers installed on this
// machine, then hands back normalised, position-verified findings and an honest
// account of what did NOT run.
//
// GORILLA FIX (2026-10-05): seven statements this tool made about itself were
// not true, found by reading what the embedded toolkit actually does:
//
//   - "quick" claimed the security stages were skipped. They ran. It mapped to
//     --no-stage3, which only stops the deep stage. It is now --quick, a real
//     linters-and-formatters-only run, and the sentence is read off the report.
//   - Results were written into <target>/.code_review/, inside the folder under
//     review, on every run. They now go under the program's cache directory.
//   - "Nothing is downloaded." semgrep fetched rule packs and sent metrics;
//     cargo audit and the Go tools fetch too. Metrics are off, and the
//     permission prompt names every network-using analyser that is installed.
//   - stderr was discarded, so "no files changed", "no such path" and "not a
//     git repository" all reached the model as "exit status 1" or as a failure.
//   - A doctor that crashed with partial output counted as "ready".
//   - The analyser count was typed (30) and wrong (the registry holds more, a
//     given language gets a handful). No count is stated any more.
//   - A timeout killed Python and orphaned the analysers it had started.
//
// WHY THIS IS NOT "ASK THE MODEL TO READ THE CODE". Those are different jobs
// and both are needed. An analyser finds the buffer overrun, the shell
// injection, the leaked credential, the unchecked error — mechanically, on
// every file, without getting bored. A model finds the wrong logic, the broken
// invariant, the swallowed error, the thing that is technically fine and
// completely wrong for this codebase. Neither substitutes for the other, and a
// review that claims to be complete having done only one of them is lying.
//
// THE THING THAT MAKES IT SAFE FOR A SMALL MODEL. A report full of MISSING
// looks exactly like a report that found nothing. That confusion is the worst
// way for a review tool to fail, and a 2-billion-parameter model has no way to
// spot it. So the toolkit's `trust` block travels FIRST in this tool's output,
// before a single finding, and the tool refuses outright when no analyser for
// the target's languages is installed rather than returning an empty list that
// reads like a pass.
//
// OUTPUT IS BOUNDED IN THE UNIT THAT MATTERS. A full review of a large tree is
// megabytes of JSON, and every tool result is re-sent on every later turn — the
// same trap that took a conversation from 15.9K to 675K tokens in one turn when
// grep capped MATCHES instead of BYTES. This caps what it returns and always
// says what it left out, with the path to the complete report on disk.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools/codereview"
	"github.com/opencode-ai/opencode/internal/permission"
)

// getPythonBinary reports a Python 3 interpreter to run the toolkit with.
//
// GORILLA OVERRIDE (2026-09-01, rev 2): this delegates to the find tool's
// detector rather than keeping a second one.
//
// Rev 1 wrote its own, and the two disagreed in the way that mattered. find.go's
// version rejects Windows' App Execution Alias — the zero-byte %LOCALAPPDATA%\
// Microsoft\WindowsApps\python3.exe stub that opens the Microsoft Store instead
// of running anything. This one did not, so `review` would pick the stub, run
// it, and get back "Python was not found; run without arguments to install from
// the Microsoft Store" as if it were toolkit output. The local session database
// has six recorded failures of exactly that shape.
//
// It also verified py.exe with `-3` and then invoked it WITHOUT `-3`, so the
// check proved something about an interpreter it then did not use.
//
// One detector, one answer. See findPythonExe in find.go.
func getPythonBinary() (string, []string, error) {
	python, preArgs, err := findPythonExe()
	if err == nil {
		return python, preArgs, nil
	}
	return "", nil, fmt.Errorf("Python 3 not found. Install Python 3:\n" +
		"  Windows: https://python.org — tick \"Add python.exe to PATH\" in the installer,\n" +
		"           or install from the Microsoft Store and re-open your terminal\n" +
		"  Linux:   sudo apt install python3   (Arch: sudo pacman -S python)\n" +
		"  macOS:   brew install python3")
}

const (
	ReviewToolName = "review"

	// reviewMaxFindings bounds how many individual findings travel back. The
	// corroborated ones and the trust block are never truncated: they are small,
	// and they are what the reader needs to judge everything else.
	reviewMaxFindings = 60

	// reviewTimeout is generous because a real review of a large tree runs
	// dozens of analysers. The toolkit has its own per-tool timeouts underneath.
	reviewTimeout = 20 * time.Minute
)

type ReviewParams struct {
	// Path to review. A directory, or a single file. Defaults to the working
	// directory.
	Path string `json:"path"`
	// Diff limits the review to what changed against a git ref, e.g. "HEAD~1"
	// or "origin/main". Strongly preferred on a large tree.
	Diff string `json:"diff"`
	// Deep enables the toolkit's slower, deeper pass. Prefer Focus.
	Deep bool `json:"deep"`
	// Focus selects how much of the pipeline runs: "quick", "security" or
	// "full". Empty means the default — stages 0-2 with automatic escalation.
	Focus string `json:"focus"`
	// MaxFiles bounds a very large tree. 0 means the toolkit's own default.
	MaxFiles int `json:"max_files"`
	// Profile overrides project detection: firefox, linux-kernel, go, rust,
	// python, generic.
	Profile string `json:"profile"`
}

type reviewTool struct {
	permissions permission.Service
}

func NewReviewTool(permissions permission.Service) BaseTool {
	return &reviewTool{permissions: permissions}
}

func (r *reviewTool) Info() ToolInfo {
	return ToolInfo{
		Name: ReviewToolName,
		Description: `Run a professional static-analysis and security review over a folder, a file, or a set of changes.

Drives the real analysers installed on this machine that suit the languages actually present (C/C++, Go, Python, JavaScript/TypeScript, Rust, shell, CSS and more), normalises every tool's output into one shape, verifies that each reported line really says what the tool claims, and reports what did NOT run. Needs Python 3. Logs and the full report are written under the program's cache directory, never into the folder being reviewed. Some analysers fetch rule packs or dependencies over the network; the permission prompt names the ones that apply before anything runs.

WHEN TO USE IT
  - Before committing, to check your own changes: pass diff="HEAD" or diff="origin/main".
  - When asked to review, audit or check code, a patch, or a repository.
  - When you want the mechanical findings — memory errors, injection, leaked secrets, unchecked errors — that reading cannot reliably produce.

WHAT IT DOES NOT DO
  It finds no semantic bugs. Wrong logic, broken invariants, swallowed errors and bad design are invisible to static analysers. YOU must still read the changed code. Treat this as one half of a review and say so in your answer.

READ THE trust BLOCK FIRST. It is returned before any finding. An empty findings list is NOT the same as clean code: a tool listed in tools_missing never ran at all. Only call a language reviewed if its analysers appear in tools_ran.

START FROM corroborated. Those are lines flagged independently by two or more different tools — computed, not guessed, and the highest-confidence material in the report.`,
		Parameters: map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory or file to review. Defaults to the working directory.",
			},
			"diff": map[string]any{
				"type":        "string",
				"description": "Limit the review to changes against this git ref, e.g. 'HEAD', 'HEAD~1', 'origin/main'. Strongly preferred on a large repository — without it every tracked file is reviewed.",
			},
			"focus": map[string]any{
				"type": "string",
				"enum": []string{"quick", "security", "full"},
				"description": "How much to run. 'quick' = linters and formatters only, for a fast sanity check: " +
					"no static analysis, no security tools, no secret scan. " +
					"'security' = ONLY the secret scanners, security analysers and static analysers, with the deep " +
					"pass forced over every file; no linters or formatters run at all, and only security, secrets and " +
					"static-analysis findings are listed. 'full' = every analyser of every kind, deep pass forced over " +
					"every file. " +
					"OMIT THIS for the normal review: by default the fast and standard stages run, and the deep security " +
					"pass escalates ON ITS OWN for any file whose output mentions CWE, CVE, overflow, use-after-free, " +
					"injection, a hardcoded secret, a race, a path traversal or a format string. That is usually what you want.",
			},
			"max_files": map[string]any{
				"type":        "integer",
				"description": "Cap the number of files reviewed. Use on a very large tree — prefer 'diff' first.",
			},
			"profile": map[string]any{
				"type":        "string",
				"enum":        []string{"firefox", "linux-kernel", "go", "rust", "python", "generic"},
				"description": "Override project detection. Only set this when auto-detection is visibly wrong: the correct profile makes the toolkit prefer a tree's own native tooling (./mach lint for Firefox, checkpatch and sparse for a kernel) over noisier generic tools.",
			},
			"deep": map[string]any{
				"type":        "boolean",
				"description": "Deprecated — use focus='full'. Kept so an older call still works.",
			},
		},
		Required: []string{},
	}
}

func (r *reviewTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ReviewParams
	if call.Input != "" {
		if err := UnmarshalToolInput(call.Input, &params); err != nil {
			return NewTextErrorResponse(fmt.Sprintf("could not read the parameters: %s", err)), nil
		}
	}

	target := strings.TrimSpace(params.Path)
	if target == "" {
		target = config.WorkingDirectory()
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(config.WorkingDirectory(), target)
	}

	script, err := codereview.Unpack(config.Get().Data.Directory)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("could not unpack the review toolkit: %s", err)), nil
	}

	// The doctor runs first, always. It is fast, it needs no permission because
	// it only inspects, and it is the difference between "clean" and "nothing
	// ran". Refusing here is the whole point of the feature.
	python, preArgs, err := getPythonBinary()
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("The review did NOT run - Python 3 not found: %s"+
			"\n\nDo not describe the code as reviewed.", err.Error())), nil
	}
	tk := toolkitRunner{python: python, preArgs: preArgs, script: script}

	doctor, state := runDoctor(ctx, tk, target)
	switch state {
	case doctorNoAnalysers:
		return NewTextResponse("The review did NOT run, because no analyser for this " +
			"code is installed on this machine. An empty result would have looked exactly " +
			"like a clean report, so nothing was run at all.\n\n" + doctor +
			"\n\nTell the user which analysers are missing and the command that installs " +
			"them. Do not describe the code as reviewed."), nil
	case doctorFailed:
		// Not "no analyser installed": the check itself did not finish, so
		// nothing is known about the analysers either way. Saying which of the
		// two happened is the whole point of having a doctor.
		return NewTextErrorResponse("The review did NOT run, because the readiness check " +
			"could not complete. This is not a statement about which analysers are " +
			"installed.\n\n" + doctor + "\n\nDo not describe the code as reviewed."), nil
	}

	focus := strings.ToLower(strings.TrimSpace(params.Focus))
	if focus == "" && params.Deep {
		focus = "full" // the old boolean, still honoured
	}

	// What will touch the network, asked of the toolkit BEFORE the user is
	// asked to approve anything. A failure here is reported in the prompt
	// rather than hidden: "could not find out" is not "nothing will".
	network := networkNote(ctx, tk, target, focus)

	sid, mid := GetContextValues(ctx)
	if sid == "" || mid == "" {
		return ToolResponse{}, fmt.Errorf("session or message id missing from the context")
	}
	shown := target
	if params.Diff != "" {
		shown += "  (changes against " + params.Diff + ")"
	}
	if !r.permissions.Request(permission.CreatePermissionRequest{
		SessionID:   sid,
		Path:        target,
		ToolName:    ReviewToolName,
		Action:      "run",
		Description: "Run static analysis and security tools over " + shown + "\n\n" + network.text,
		// Grant covers THIS target, not every path in the session.
		GrantKey: target,
		Params:   params,
		// Only when an installed analyser really will reach another machine,
		// or when that could not be established.
		Egress: network.egress,
	}) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	// GORILLA FIX (2026-10-05): results go under the program's own cache
	// directory. The toolkit's default is <target>/.code_review/<timestamp>,
	// which put a new untracked folder of logs inside the user's repository on
	// every review — the thing /osint goes out of its way never to do, and one
	// `git add -A` away from being committed.
	resultsDir, err := newReviewResultsDir(target)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("The review did NOT run: it has nowhere to write "+
			"its report (%s). Do not describe the code as reviewed.", err)), nil
	}

	args := []string{target, "--audience", "agent", "--results-dir", resultsDir}
	if params.Diff != "" {
		args = append(args, "--diff", params.Diff)
	}
	if params.MaxFiles > 0 {
		args = append(args, "--max-files", fmt.Sprint(params.MaxFiles))
	}
	if params.Profile != "" {
		args = append(args, "--profile", params.Profile)
	}

	// focus maps onto the toolkit's four stages. See its tools_registry.py:
	// stage 0 recon, 1 fast linters, 2 static analysis and security, 3 deep —
	// where stage 3 normally escalates by itself on files whose earlier output
	// looked security-shaped.
	args = append(args, focusArgs(focus)...)

	runCtx, cancel := context.WithTimeout(ctx, reviewTimeout)
	defer cancel()

	stdout, stderr, runErr := tk.run(runCtx, args...)
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return NewTextErrorResponse(fmt.Sprintf("The review was stopped after %s without finishing, "+
			"and the analysers it had started were stopped with it. Nothing below that point was "+
			"checked. Narrow it — pass diff, a smaller path, or focus=\"quick\" — and run it again."+
			"\n\nDo not describe the code as reviewed.%s", reviewTimeout, stderrTail(stderr))), nil
	}
	text, isErr := interpretReviewRun(stdout, stderr, runErr, focus)
	if isErr {
		return NewTextErrorResponse(text), nil
	}
	return NewTextResponse(text), nil
}

// focusArgs maps a depth onto the toolkit's own flags.
//
// GORILLA FIX (2026-10-05): "quick" was --no-stage3, with a comment here saying
// "Stages 0-1 only". It was neither. --no-stage3 only stops the deep stage, so
// stages 1 AND 2 ran: bandit, gosec, cargo audit, clang-tidy, semgrep, gitleaks.
// The summary then told the model the security stages had been "SKIPPED
// ENTIRELY" and that the pass "cannot have found" an injection or a leaked
// credential, in the trust block, which is the part it is told to believe.
//
// --quick is a real mode in the toolkit: linters and formatters by CATEGORY,
// nothing else, never escalating. The claim was kept and made true.
//
// GORILLA FIX (2026-10-06): "security" and "full" both sent --deep. The same
// analysers ran, for the same time; the only difference was that the summary
// of a "security" run dropped the style findings afterwards. --security is now
// a real mode in the toolkit too, the mirror of --quick: the secrets, security
// and static-analysis categories with the deep stage forced on every file, and
// no linter or formatter. --deep is left to "full", which runs everything.
func focusArgs(focus string) []string {
	switch focus {
	case "quick":
		return []string{"--quick"}
	case "security":
		return []string{"--security"}
	case "full":
		return []string{"--deep"}
	}
	return nil
}

// toolkitRunner starts the embedded toolkit. One place, so the doctor, the
// network report and the review itself cannot each grow their own idea of how
// to find Python, where to run, or what to do with stderr.
type toolkitRunner struct {
	python  string
	preArgs []string
	script  string
}

// run executes the toolkit and returns stdout and stderr SEPARATELY.
//
// GORILLA FIX (2026-10-05): this used cmd.Output(), which drops stderr. In
// agent mode the toolkit sends every word that is not the final JSON to
// stderr — including the only explanation it ever gives for stopping. So
// "No files found in scope", "<path> does not exist" and "--diff was given but
// this is not a git repository" all arrived here as empty output plus "exit
// status 1", and the model was told the review had failed with no reason.
func (t toolkitRunner) run(ctx context.Context, args ...string) (stdout, stderr []byte, err error) {
	full := append(append(append([]string{}, t.preArgs...), t.script), args...)
	cmd := exec.CommandContext(ctx, t.python, full...)
	cmd.Dir = filepath.Dir(t.script)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se

	// GORILLA FIX (2026-10-05): stop the whole tree, not just Python.
	//
	// CommandContext kills the process it started. On Windows that leaves every
	// analyser Python launched still running — a cancelled or timed-out review
	// went on consuming the machine with nothing left to read its output.
	// `taskkill /T` walks the tree; the same approach as the shell tool.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if runtime.GOOS == "windows" {
			if exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run() == nil {
				return nil
			}
		}
		return cmd.Process.Kill()
	}
	// A child that inherited the pipes can hold them open after Python is
	// gone; without a bound, Wait would block on them indefinitely.
	cmd.WaitDelay = 5 * time.Second

	err = cmd.Run()
	return so.Bytes(), se.Bytes(), err
}

// stderrTail is the last few lines of what the toolkit said on stderr, for
// showing to the model. Bounded: a tool result is re-sent on every later turn.
func stderrTail(stderr []byte) string {
	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(string(stderr), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, strings.TrimRight(l, " \t"))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	const keep = 12
	if len(lines) > keep {
		lines = lines[len(lines)-keep:]
	}
	return "\n\nWhat the toolkit said:\n" + oneBlockOf(strings.Join(lines, "\n"), 1500)
}

func oneBlockOf(s string, max int) string {
	if r := []rune(s); len(r) > max {
		return "..." + string(r[len(r)-max:])
	}
	return s
}

// interpretReviewRun turns what the toolkit produced into what the model is
// told. It is separate from Run so each outcome can be tested without Python.
//
// Three outcomes used to be one ("The review failed to run"):
//
//   - exit 0, nothing on stdout: the run had NOTHING IN SCOPE. The usual cause
//     is diff="HEAD" on a tree with no changes — a result, not a failure, and
//     /review's own prompt steers the model into it.
//   - non-zero exit, nothing on stdout: it stopped, and stderr says why.
//   - JSON on stdout: a report, whatever the exit code.
func interpretReviewRun(stdout, stderr []byte, runErr error, focus string) (text string, isError bool) {
	if len(bytes.TrimSpace(stdout)) == 0 {
		if runErr == nil {
			return "NOTHING WAS IN SCOPE, so nothing was reviewed. This is not a failure and " +
				"it is not a clean result: no file was looked at. If you passed diff, there " +
				"are no changed files against that ref — say so, and offer a review of the " +
				"folder without diff if the user wants one." + stderrTail(stderr) +
				"\n\nDo not describe the code as reviewed.", false
		}
		// Exit 3 is the toolkit's preflight: nothing that this depth runs is
		// installed. The readiness check before the permission prompt asks
		// about a standard review, so this is reached by a quick pass on a
		// machine with security tools and no linter. Its explanation is at the
		// TOP of a long stderr, which the tail would cut, so it is said here.
		var ee *exec.ExitError
		if errors.As(runErr, &ee) && ee.ExitCode() == 3 {
			msg := "The review did NOT run: none of the analysers this depth uses is installed " +
				"for this code, so it would have inspected nothing."
			switch focus {
			case "quick":
				msg += " A quick pass runs linters and formatters only. A standard review may " +
					"still be possible: run it again without focus=\"quick\"."
			case "security":
				msg += " A security pass runs secret scanners, security analysers and static " +
					"analysers only. A standard review may still be possible: run it again " +
					"without focus=\"security\"."
			}
			return msg + "\n\nDo not describe the code as reviewed.", true
		}
		return "The review did NOT run: " + runErr.Error() + "." + stderrTail(stderr) +
			"\n\nDo not describe the code as reviewed.", true
	}
	summary, err := summariseReview(stdout, focus)
	if err != nil {
		return fmt.Sprintf("The review ran but its output could not be read: %s.%s"+
			"\n\nDo not describe the code as reviewed.", err, stderrTail(stderr)), true
	}
	return summary, false
}

// reviewKeepResults is how many past result folders are kept. Each holds every
// analyser's raw log for one run; they are for reading after a review, not an
// archive, and nothing else ever removes them.
const reviewKeepResults = 20

var reviewSlugUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// reviewResultsRoot is where every review's logs and reports live.
func reviewResultsRoot() string {
	return filepath.Join(config.CacheBase(), "code-review", "results")
}

// newReviewResultsDir creates a fresh, private folder for one run and prunes
// the oldest beyond reviewKeepResults.
//
// 0700 because the raw logs quote the code under review, and a review of a
// private repository should not be readable by other accounts on the machine.
func newReviewResultsDir(target string) (string, error) {
	root := reviewResultsRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	slug := strings.Trim(reviewSlugUnsafe.ReplaceAllString(filepath.Base(target), "-"), "-.")
	if slug == "" {
		slug = "review"
	}
	if len(slug) > 40 {
		slug = slug[:40]
	}
	// The timestamp sorts; MkdirTemp's suffix keeps two runs in one second apart.
	dir, err := os.MkdirTemp(root, time.Now().Format("20060102_150405")+"-"+slug+"-")
	if err != nil {
		return "", err
	}
	pruneReviewResults(root, dir)
	return dir, nil
}

func pruneReviewResults(root, keep string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs) // names start with the timestamp, so this is oldest first
	for len(dirs) > reviewKeepResults {
		victim := filepath.Join(root, dirs[0])
		dirs = dirs[1:]
		if victim == keep {
			continue
		}
		_ = os.RemoveAll(victim)
	}
}

// networkTool is one entry of the toolkit's --network-report.
type networkTool struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Category  string `json:"category"`
	Stage     int    `json:"stage"`
	Network   string `json:"network"`
	Installed bool   `json:"installed"`
	// Runs is the toolkit's own answer to "does this tool run at the depth
	// asked for". Decided there, by the rule that schedules the jobs, so this
	// file holds no second copy of what a quick pass includes.
	Runs bool `json:"runs"`
}

type networkInfo struct {
	text   string
	egress bool
}

// networkNote asks the toolkit which analysers will contact another machine
// during this run, and turns the answer into the paragraph the user approves.
//
// GORILLA FIX (2026-10-05): the prompt said "Run static analysis and security
// tools over <path>" and the help said "nothing is downloaded when you run it",
// while semgrep fetched rule packs from semgrep.dev and reported usage metrics,
// cargo audit cloned an advisory database, and the Go tools could download
// modules. The list is not typed here. It is read from the registry entry of
// each tool (Tool.network), filtered to the languages in THIS target, to the
// tools actually installed, and to the depth asked for.
func networkNote(ctx context.Context, tk toolkitRunner, target, focus string) networkInfo {
	nctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	args := append([]string{target, "--network-report"}, focusArgs(focus)...)
	stdout, stderr, err := tk.run(nctx, args...)
	if err != nil {
		return networkInfo{egress: true, text: "NETWORK: could not establish which analysers " +
			"contact the network (" + err.Error() + "). Assume some will." + stderrTail(stderr)}
	}
	return describeNetwork(stdout)
}

// describeNetwork renders a --network-report. Separate so it is testable.
func describeNetwork(report []byte) networkInfo {
	var rep struct {
		Tools []networkTool `json:"tools"`
	}
	if err := json.Unmarshal(report, &rep); err != nil {
		return networkInfo{egress: true, text: "NETWORK: could not read the toolkit's account " +
			"of which analysers contact the network (" + err.Error() + "). Assume some will."}
	}
	var lines []string
	for _, t := range rep.Tools {
		if !t.Installed || !t.Runs {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", t.Label, t.Network))
	}
	if len(lines) == 0 {
		return networkInfo{text: "NETWORK: none of the analysers that will run here contact " +
			"another machine. Everything is read from disk."}
	}
	return networkInfo{egress: true, text: "NETWORK: these installed analysers contact other " +
		"machines when they run. What each one requests is visible to the service it asks:\n" +
		strings.Join(lines, "\n")}
}

type doctorState int

const (
	doctorReady doctorState = iota
	doctorNoAnalysers
	doctorFailed
)

// runDoctor asks whether this machine can review that target at all.
//
// GORILLA FIX (2026-10-05): this returned ready=true for ANY exit code other
// than 3 as long as something had been printed. A doctor that crashed halfway,
// or was pointed at a path that does not exist, therefore passed the gate; the
// user was asked to approve a review that then failed. Ready is exit 0 and
// nothing else. Exit 3 is the toolkit's documented "no analyser installed";
// every other outcome is a failed check, reported as one.
func runDoctor(ctx context.Context, tk toolkitRunner, target string) (string, doctorState) {
	dctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	stdout, stderr, err := tk.run(dctx, target, "--doctor")
	return classifyDoctor(stdout, stderr, err)
}

func classifyDoctor(stdout, stderr []byte, err error) (string, doctorState) {
	text := strings.TrimSpace(string(stdout))
	if err == nil {
		return text, doctorReady
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 3 {
		return text, doctorNoAnalysers
	}
	msg := "The readiness check stopped: " + err.Error() + "."
	if text != "" {
		msg += "\n\n" + oneBlockOf(text, 1500)
	}
	return msg + stderrTail(stderr), doctorFailed
}

// agentReport is the subset of schema code-review/agent/1 this tool reads.
type agentReport struct {
	Target       string   `json:"target"`
	Profile      string   `json:"profile"`
	FilesScanned int      `json:"files_scanned"`
	Languages    []string `json:"languages"`
	// GORILLA FIX (2026-10-05): the key is logs_dir. This read "results_dir",
	// which the toolkit has never written, so the line that tells the reader
	// where the full report is was never printed — while the truncation notice
	// went on saying the missing findings "are all in the full report".
	ResultsDir string `json:"logs_dir"`

	Findings []struct {
		Tool     string `json:"tool"`
		File     string `json:"file"`
		Line     int    `json:"line"`
		Severity string `json:"severity"`
		Message  string `json:"message"`
		Rule     string `json:"rule"`
		Excerpt  string `json:"excerpt"`
	} `json:"findings"`

	Corroborated []struct {
		File     string   `json:"file"`
		Line     int      `json:"line"`
		Tools    []string `json:"tools"`
		Messages []string `json:"messages"`
	} `json:"corroborated"`

	Trust struct {
		ToolsRan      []string `json:"tools_ran"`
		ToolsErrored  []string `json:"tools_errored"`
		ToolsMissing  []string `json:"tools_missing"`
		ToolsTimedOut []string `json:"tools_timed_out"`
		NoParser      []string `json:"tools_without_parser"`
		PositionCheck bool     `json:"position_checked"`
		Dropped       int      `json:"findings_dropped_by_position_check"`
		Caveat        string   `json:"caveat"`
	} `json:"trust"`

	// Depth is the run's OWN statement of how deep it went and which analysers
	// it left out because of that. Absent from a toolkit older than 2026-10-05,
	// in which case nothing is claimed about what a quick pass skipped.
	Depth *struct {
		Mode              string   `json:"mode"`
		CategoriesRun     []string `json:"categories_run"`
		CategoriesSkipped []string `json:"categories_skipped"`
		ToolsSkipped      []string `json:"tools_skipped_by_depth"`
	} `json:"depth"`

	// Coverage is the sealed account of the run: every scheduled job in exactly
	// one state, and each language reviewed only if an analyser for it finished.
	// Absent from reports written by a toolkit older than 2026-10-04.
	Coverage *struct {
		TerminalState string         `json:"terminal_state"`
		JobsPlanned   int            `json:"jobs_planned"`
		Jobs          map[string]int `json:"jobs"`
		Languages     map[string]struct {
			State string `json:"state"`
		} `json:"languages"`
		Unreviewed []string `json:"languages_unreviewed"`
	} `json:"coverage"`

	// GORILLA FIX (2026-10-05): manual_steps is a list of OBJECTS. This was
	// []string, so json.Unmarshal failed on the whole report whenever the
	// toolkit offered a manual step — which it does for every C or C++ tree
	// (valgrind, scan-build) and every kernel or Firefox profile. Those reviews
	// ran to completion and then reported "its output could not be read". The
	// contract test never saw it because it decodes a Go-only directory.
	ManualSteps []struct {
		ID      string `json:"id"`
		Label   string `json:"label"`
		Why     string `json:"why"`
		Command string `json:"command"`
	} `json:"manual_steps"`
}

// summariseReview turns the report into something a model can act on, bounded.
//
// Order is deliberate and is the opposite of every review tool the author has
// seen: what did NOT run comes first, the corroborated findings second, and the
// long tail last, truncated. A reader who stops after two paragraphs still
// leaves with the two things that stop them drawing a false conclusion.
func summariseReview(raw []byte, focus string) (string, error) {
	var rep agentReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Code review — %s\n\n", rep.Target)
	fmt.Fprintf(&b, "%d files scanned · languages: %s · profile: %s\n\n",
		rep.FilesScanned, strings.Join(rep.Languages, ", "), rep.Profile)

	// 1. WHAT DID NOT RUN.
	b.WriteString("## Trust — read this before the findings\n\n")
	// GORILLA OVERRIDE (2026-10-04): the verdict comes before the lists.
	//
	// A small model was handed "Analysers that ran: 18", "NOT INSTALLED: 17" and
	// "All findings: 0" and had to work out for itself that nothing had been
	// reviewed. The sealed coverage block (idea from alibaba/open-code-review's
	// run manifest) states the conclusion outright, so it is read, not derived.
	if c := rep.Coverage; c != nil {
		switch c.TerminalState {
		case "nothing-ran":
			b.WriteString("- **NOTHING WAS REVIEWED.** No analyser completed a single job. " +
				"The empty findings list below means nothing ran, not that the code is clean. " +
				"Do not report this code as reviewed.\n")
		case "partial":
			// GORILLA FIX (2026-10-06), first end-to-end run on a real model:
			// "6 of 17 scheduled jobs completed" read as eleven failures, and
			// the receipt listed the call as "did not succeed". The eleven were
			// analysers that are not installed. The breakdown is stated so a
			// gap in installed tools is never mistaken for a tool that broke.
			fmt.Fprintf(&b, "- **PARTIAL REVIEW.** %d of %d scheduled jobs completed (%d not installed, %d failed, %d timed out).",
				c.Jobs["completed"], c.JobsPlanned, c.Jobs["missing"], c.Jobs["errored"], c.Jobs["timed_out"])
			if len(c.Unreviewed) > 0 {
				fmt.Fprintf(&b, " **Languages with NO completed analyser: %s** — say so in your answer.",
					strings.Join(c.Unreviewed, ", "))
			}
			b.WriteString("\n")
		case "complete":
			fmt.Fprintf(&b, "- Coverage: all %d scheduled jobs completed, every language in scope had an analyser finish.\n",
				c.JobsPlanned)
		}
	}
	fmt.Fprintf(&b, "- Analysers that ran: %d (%s)\n", len(rep.Trust.ToolsRan), joinCapped(rep.Trust.ToolsRan, 14))
	if n := len(rep.Trust.ToolsMissing); n > 0 {
		fmt.Fprintf(&b, "- **NOT INSTALLED, so they never ran: %d (%s)** — the code they cover is UNREVIEWED\n",
			n, joinCapped(rep.Trust.ToolsMissing, 14))
	}
	if n := len(rep.Trust.ToolsErrored); n > 0 {
		fmt.Fprintf(&b, "- **Failed to run: %d (%s)** — produced nothing parseable\n", n, joinCapped(rep.Trust.ToolsErrored, 14))
	}
	if n := len(rep.Trust.ToolsTimedOut); n > 0 {
		fmt.Fprintf(&b, "- Timed out: %d (%s)\n", n, joinCapped(rep.Trust.ToolsTimedOut, 14))
	}
	if len(rep.Trust.NoParser) > 0 {
		fmt.Fprintf(&b, "- Ran without a parser, so line numbers may be imprecise: %s\n", joinCapped(rep.Trust.NoParser, 10))
	}
	if rep.Trust.PositionCheck {
		fmt.Fprintf(&b, "- Every reported line was checked against the file; %d findings were dropped as stale.\n", rep.Trust.Dropped)
	}
	// What the CHOSEN DEPTH skipped belongs in the trust block, beside what was
	// missing from the machine. Both answer the same question — what did this
	// review not look at — and a quick pass that does not say it skipped the
	// security stage is the same lie as an uninstalled analyser.
	switch focus {
	case "quick":
		// GORILLA FIX (2026-10-05): this sentence is now read off the report.
		//
		// It used to be printed whenever "quick" had been ASKED for, whatever
		// the toolkit then did — and what it did was run the security stages.
		// The claim is only made when the run itself says it was a quick one,
		// and it names the analysers that were left out, so a reader can check
		// it against the list of what ran two lines above.
		if d := rep.Depth; d != nil && d.Mode == "quick" {
			b.WriteString("- **DEPTH: quick.** Only linters and formatters ran. Static analysis, " +
				"security tools and the secret scan were SKIPPED ENTIRELY — this pass cannot " +
				"have found a buffer overrun, an injection, or a leaked credential, and says " +
				"nothing about whether one is there.\n")
			if len(d.CategoriesSkipped) > 0 {
				fmt.Fprintf(&b, "  - Kinds of check not run at all: %s\n", strings.Join(d.CategoriesSkipped, ", "))
			}
			if len(d.ToolsSkipped) > 0 {
				fmt.Fprintf(&b, "  - Analysers that apply to this code and were left out by the depth: %s\n",
					joinCapped(d.ToolsSkipped, 20))
			}
		} else {
			b.WriteString("- **DEPTH: quick was asked for, but the run did not confirm it.** " +
				"Treat the lists above as the only account of what ran; do not tell the user " +
				"which stages were skipped.\n")
		}
	case "security":
		// GORILLA FIX (2026-10-06): read off the report, like quick. Until
		// today this said "the deep pass was forced over every file" and that
		// style findings "exist and were deliberately left out" — true, because
		// the run was identical to a full one and every linter had run. Now the
		// linters do not run, so the sentence that was true of the old run
		// would be a lie about the new one; the run says what it did.
		if d := rep.Depth; d != nil && d.Mode == "security" {
			b.WriteString("- **DEPTH: security.** Only the secret scanners, security analysers and " +
				"static analysers ran, with the deep pass forced over every file. Linters and " +
				"formatters were SKIPPED ENTIRELY — this pass says nothing about style, " +
				"formatting or dead code, and the list below is narrowed to security-shaped " +
				"findings.\n")
			if len(d.CategoriesSkipped) > 0 {
				fmt.Fprintf(&b, "  - Kinds of check not run at all: %s\n", strings.Join(d.CategoriesSkipped, ", "))
			}
			if len(d.ToolsSkipped) > 0 {
				fmt.Fprintf(&b, "  - Analysers that apply to this code and were left out by the depth: %s\n",
					joinCapped(d.ToolsSkipped, 20))
			}
		} else {
			b.WriteString("- **DEPTH: security was asked for, but the run did not confirm it.** " +
				"Treat the lists above as the only account of what ran; do not tell the user " +
				"which kinds of check were skipped. The list below is still narrowed to " +
				"security-shaped findings.\n")
		}
	case "full":
		if d := rep.Depth; d != nil && d.Mode == "deep" {
			b.WriteString("- **DEPTH: full.** Every analyser of every kind ran, with the deep pass " +
				"forced over every file. Nothing was left out by the depth.\n")
		} else {
			b.WriteString("- **DEPTH: full was asked for, but the run did not confirm it.** " +
				"Treat the lists above as the only account of what ran.\n")
		}
	default:
		b.WriteString("- Depth: standard — fast and static-analysis stages, with the deep " +
			"security pass escalating automatically on any file whose output looked " +
			"security-shaped.\n")
	}

	b.WriteString("\nStatic analysers do not find semantic bugs — wrong logic, broken " +
		"invariants, swallowed errors. Read the changed code yourself as well, and say " +
		"in your answer that you did.\n\n")

	// 2. CORROBORATED — never truncated.
	fmt.Fprintf(&b, "## Corroborated: %d (flagged by two or more DIFFERENT tools)\n\n", len(rep.Corroborated))
	if len(rep.Corroborated) == 0 {
		b.WriteString("None. That is not the same as clean — see the trust block above.\n\n")
	}
	for _, c := range rep.Corroborated {
		fmt.Fprintf(&b, "- `%s:%d` [%s] %s\n", c.File, c.Line,
			strings.Join(c.Tools, "+"), firstOf(c.Messages))
	}
	b.WriteString("\n")

	// 3. THE REST, most severe first, bounded.
	sev := map[string]int{"error": 0, "critical": 0, "high": 1, "warning": 2, "medium": 2, "low": 3, "style": 4, "info": 4}
	findings := rep.Findings
	sort.SliceStable(findings, func(i, j int) bool {
		si, ok1 := sev[strings.ToLower(findings[i].Severity)]
		sj, ok2 := sev[strings.ToLower(findings[j].Severity)]
		if !ok1 {
			si = 3
		}
		if !ok2 {
			sj = 3
		}
		return si < sj
	})

	// A security review lists security findings. The others still exist and the
	// count says so, so nothing is hidden — it is narrowed, and the narrowing
	// is stated.
	total := len(findings)
	if focus == "security" {
		var kept []struct {
			Tool     string `json:"tool"`
			File     string `json:"file"`
			Line     int    `json:"line"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
			Rule     string `json:"rule"`
			Excerpt  string `json:"excerpt"`
		}
		for _, f := range findings {
			if looksSecurity(f.Severity, f.Message, f.Rule, f.Tool) {
				kept = append(kept, f)
			}
		}
		// GORILLA FIX (2026-10-06): the omitted findings are no longer "style
		// and formatting" — no linter ran. What is dropped now is a static
		// analyser's non-security output (a type error from mypy, an unused
		// variable from cppcheck), and the heading says so.
		fmt.Fprintf(&b, "## Security findings: %d (of %d total; the rest are non-security findings from the static analysers, omitted by focus=security)\n\n",
			len(kept), total)
		findings = kept
	} else {
		fmt.Fprintf(&b, "## All findings: %d\n\n", total)
	}
	shown := findings
	if len(shown) > reviewMaxFindings {
		shown = shown[:reviewMaxFindings]
	}
	for _, f := range shown {
		fmt.Fprintf(&b, "- [%s] `%s:%d` (%s", f.Severity, f.File, f.Line, f.Tool)
		if f.Rule != "" {
			fmt.Fprintf(&b, " %s", f.Rule)
		}
		fmt.Fprintf(&b, ") %s\n", oneLineOf(f.Message, 200))
	}
	if len(findings) > len(shown) {
		fmt.Fprintf(&b, "\n**%d further findings were not listed here** to keep this result "+
			"small — every later turn re-sends it. They are all in the full report.\n",
			len(findings)-len(shown))
	}

	if len(rep.ManualSteps) > 0 {
		b.WriteString("\n## Cannot be automated safely — run these by hand if it matters\n\n")
		for _, m := range capped(rep.ManualSteps, 8) {
			fmt.Fprintf(&b, "- %s", oneLineOf(m.Label, 120))
			if m.Command != "" {
				fmt.Fprintf(&b, ": `%s`", oneLineOf(m.Command, 200))
			}
			b.WriteString("\n")
		}
	}
	if rep.ResultsDir != "" {
		fmt.Fprintf(&b, "\nFull report, and every tool's unedited output: `%s`\n", rep.ResultsDir)
	}
	return b.String(), nil
}

func joinCapped(s []string, n int) string {
	if len(s) == 0 {
		return "none"
	}
	if len(s) <= n {
		return strings.Join(s, ", ")
	}
	return strings.Join(s[:n], ", ") + fmt.Sprintf(", +%d more", len(s)-n)
}

func capped[T any](s []T, n int) []T {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func firstOf(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return oneLineOf(s[0], 200)
}

func oneLineOf(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > max {
		return string([]rune(s)[:max]) + "..."
	}
	return s
}

// looksSecurity decides whether a finding belongs in a security-focused report.
//
// Matched on the evidence in the finding itself rather than on a tool
// allow-list: the same analyser produces both a formatting nit and a command
// injection, so filtering by tool would drop real findings and keep noise. The
// vocabulary is the toolkit's own escalation keyword set, which is what decides
// the deep stage in the first place — one definition of "security-shaped",
// used in both places.
//
// securityToolIDs are the analysers whose every finding is a security finding:
// the registry's auto-run tools of category "security" or "secrets".
//
// GORILLA FIX (2026-10-05): the list named "npm-audit" and "semgrep", neither
// of which is an id the toolkit has ever emitted, so two of its nine entries
// could never match. TestSecurityToolIDsAreTheRegistrysOwn now compares it with
// the registry, in both directions.
var securityToolIDs = map[string]bool{
	"gosec": true, "bandit": true, "bandit-deep": true, "semgrep-deep": true,
	"gitleaks-worktree": true, "gitleaks-history": true, "cargo-audit": true,
}

func looksSecurity(severity, message, rule, tool string) bool {
	if securityToolIDs[strings.ToLower(tool)] {
		return true
	}
	hay := strings.ToLower(message + " " + rule)
	for _, k := range []string{
		"cwe-", "cve-", "overflow", "use-after-free", "double-free", "uaf",
		"injection", "unsanitized", "unsanitised", "hardcoded", "hard-coded",
		"secret", "credential", "race condition", "deserializ", "ssti",
		"path traversal", "format string", "null pointer dereference",
		"buffer overrun", "security", "unsafe", "tainted", "xss", "csrf",
	} {
		if strings.Contains(hay, k) {
			return true
		}
	}
	return strings.EqualFold(severity, "critical")
}
