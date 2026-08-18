package tools

// GORILLA OVERRIDE (2026-08-18): the `review` tool — point it at a folder, a
// file or a diff and it drives ~30 real static analysers, then hands back
// normalised, position-verified findings and an honest account of what did NOT
// run.
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
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools/codereview"
	"github.com/opencode-ai/opencode/internal/permission"
)

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
	// Deep enables the toolkit's slower, deeper pass.
	Deep bool `json:"deep"`
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

Drives around thirty real analysers appropriate to the languages actually present (C/C++, Go, Python, JavaScript/TypeScript, Rust, shell, CSS and more), normalises every tool's output into one shape, verifies that each reported line really says what the tool claims, and reports what did NOT run.

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
			"deep": map[string]any{
				"type":        "boolean",
				"description": "Run the slower, deeper pass. Off by default.",
			},
		},
		Required: []string{},
	}
}

func (r *reviewTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ReviewParams
	if call.Input != "" {
		if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
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
	doctor, ready := runDoctor(ctx, script, target)
	if !ready {
		return NewTextResponse("The review did NOT run, because no analyser for this " +
			"code is installed on this machine. An empty result would have looked exactly " +
			"like a clean report, so nothing was run at all.\n\n" + doctor +
			"\n\nTell the user which analysers are missing and the command that installs " +
			"them. Do not describe the code as reviewed."), nil
	}

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
		Description: "Run static analysis and security tools over " + shown,
		Params:      params,
	}) {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	args := []string{script, target, "--audience", "agent"}
	if params.Diff != "" {
		args = append(args, "--diff", params.Diff)
	}
	if params.Deep {
		args = append(args, "--deep")
	}

	runCtx, cancel := context.WithTimeout(ctx, reviewTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "python3", args...)
	cmd.Dir = filepath.Dir(script)
	out, runErr := cmd.Output()
	if len(out) == 0 {
		msg := "the review produced no output"
		if runErr != nil {
			msg = runErr.Error()
		}
		return NewTextErrorResponse("The review failed to run: " + msg +
			"\n\nDo not describe the code as reviewed."), nil
	}

	summary, err := summariseReview(out)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("the review ran but its output could not be read: %s", err)), nil
	}
	return NewTextResponse(summary), nil
}

// runDoctor asks whether this machine can review that target at all.
func runDoctor(ctx context.Context, script, target string) (string, bool) {
	dctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(dctx, "python3", script, target, "--doctor")
	cmd.Dir = filepath.Dir(script)
	out, err := cmd.Output()
	text := strings.TrimSpace(string(out))

	// Exit code 3 is the toolkit's documented "no analyser installed" signal.
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {
		return text, false
	}
	if err != nil && text == "" {
		return "the doctor could not run: " + err.Error(), false
	}
	return text, true
}

// agentReport is the subset of schema code-review/agent/1 this tool reads.
type agentReport struct {
	Target       string   `json:"target"`
	Profile      string   `json:"profile"`
	FilesScanned int      `json:"files_scanned"`
	Languages    []string `json:"languages"`
	ResultsDir   string   `json:"results_dir"`

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

	ManualSteps []string `json:"manual_steps"`
}

// summariseReview turns the report into something a model can act on, bounded.
//
// Order is deliberate and is the opposite of every review tool the author has
// seen: what did NOT run comes first, the corroborated findings second, and the
// long tail last, truncated. A reader who stops after two paragraphs still
// leaves with the two things that stop them drawing a false conclusion.
func summariseReview(raw []byte) (string, error) {
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

	fmt.Fprintf(&b, "## All findings: %d\n\n", len(findings))
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
			fmt.Fprintf(&b, "- %s\n", oneLineOf(m, 200))
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
		return string([]rune(s)[:max]) + "…"
	}
	return s
}
