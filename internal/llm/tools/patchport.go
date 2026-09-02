// GORILLA (2026-09-02): the patch-porting half of the embedded toolkit.
//
// patch_port.py shipped inside the binary before this file existed, and was
// unreachable — nothing unpacked it, nothing ran it, and it appeared in no
// command list. A tool that ships but cannot be invoked is not a feature.
//
// Routed through the agent like /review, and for the same reason: applying a
// patch is the easy half. Whether the result is CORRECT is a judgement about
// code, and the honest answer usually needs a human or the model to read the
// resulting diff. This tool reports how each patch landed and refuses to
// present a fuzzed application as if it were a clean one.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools/codereview"
	"github.com/opencode-ai/opencode/internal/permission"
)

const (
	PatchPortToolName = "patch_port"

	// Porting is git work, not analysis: bounded by how long a rebase and an
	// optional build take, not by dozens of analysers.
	patchPortTimeout = 30 * time.Minute

	// How much of the toolkit's own log travels back. The log is the part that
	// names per-patch outcomes, so it is worth carrying, but a 200-patch series
	// would otherwise flood the context.
	patchPortMaxLogLines = 120
)

type PatchPortParams struct {
	// Operation is what to do: forward-port, backport, rebase, refresh,
	// port-series, or inspect. Empty means inspect, which changes nothing.
	Operation string `json:"operation"`
	// Tree is the git checkout to work in. Defaults to the working directory.
	Tree string `json:"tree"`
	// Patch is a single .patch/.diff file.
	Patch string `json:"patch"`
	// Series is a directory of numbered patches, as produced by
	// `git format-patch -o <dir>`.
	Series string `json:"series"`
	// Onto is the base to move the work onto: a tag, branch or commit.
	Onto string `json:"onto"`
	// Build is a command that proves the result still compiles. Without one,
	// nothing is verified and the report says so.
	Build string `json:"build"`
	// Test is a command that proves the result still behaves.
	Test string `json:"test"`
}

type patchPortTool struct {
	permissions permission.Service
}

func NewPatchPortTool(permissions permission.Service) BaseTool {
	return &patchPortTool{permissions: permissions}
}

func (p *patchPortTool) Info() ToolInfo {
	return ToolInfo{
		Name: PatchPortToolName,
		Description: `Move patches between versions of a codebase: forward-port, backport, rebase, refresh, or port a whole series.

Use this when the user has changes that were written against one version of a tree and needs them on another — the classic kernel and Firefox workflow: old tree, existing patch series, new upstream version, rebase, conflicts, resolve, build, test.

OPERATIONS
  inspect       read the tree and the patches, change nothing. Start here when unsure.
  forward-port  carry patches onto a NEWER base
  backport      carry patches onto an OLDER base
  rebase        move a branch's own commits onto a new base
  refresh       regenerate a patch so it applies cleanly again
  port-series   a whole numbered series, in order, stopping at the first real conflict

HOW TO READ THE RESULT — this is the part that matters
Each patch reports HOW it applied, and the four outcomes are NOT equivalent:
  applied-clean       matched exactly where it said it would. Trustworthy.
  applied-three-way   context had moved; git merged it using the real blob
                      history. Usually right, but READ IT.
  applied-with-fuzz   the hunk was RELOCATED by searching for surrounding
                      context. This is the one that can be silently WRONG: if
                      that context appears twice in the file, the change can
                      land in the wrong place and still report success. Always
                      show the user the resulting diff for these.
  already-present     the tree already has this change. Drop the patch from the
                      series; do not force it in a second time.
  conflict            a real decision is needed. The files are named.

Exit 0 means the patches applied and any build/test command PASSED. If no build
command was given, nothing was compiled and the port is UNVERIFIED — say so
rather than implying the result is known good.

Never describe a port as complete on the strength of patches applying. Applying
is mechanical; correctness is not.`,
		Parameters: map[string]any{
			"operation": map[string]any{
				"type": "string",
				"enum": []string{"inspect", "forward-port", "backport", "rebase",
					"refresh", "port-series"},
				"description": "What to do. 'inspect' changes nothing and is the safe default.",
			},
			"tree": map[string]any{
				"type":        "string",
				"description": "Git checkout to work in. Defaults to the working directory.",
			},
			"patch": map[string]any{
				"type":        "string",
				"description": "A single .patch or .diff file.",
			},
			"series": map[string]any{
				"type":        "string",
				"description": "A directory of numbered patches (git format-patch output).",
			},
			"onto": map[string]any{
				"type":        "string",
				"description": "Base to move onto: a tag, branch or commit.",
			},
			"build": map[string]any{
				"type":        "string",
				"description": "Command proving the result compiles, e.g. 'make -j8'. Without one the port is unverified.",
			},
			"test": map[string]any{
				"type":        "string",
				"description": "Command proving the result behaves, e.g. 'make check'.",
			},
		},
		Required: []string{"operation"},
	}
}

// patchPortReport mirrors the JSON that patch_port.py emits under --json.
type patchPortReport struct {
	Operation     string   `json:"operation"`
	Tree          string   `json:"tree"`
	TreeKind      string   `json:"tree_kind"`
	Platform      string   `json:"platform"`
	Base          string   `json:"base"`
	WorktreeClean bool     `json:"worktree_clean"`
	Problems      []string `json:"problems"`
	Patches       []struct {
		Name    string   `json:"name"`
		Subject string   `json:"subject"`
		Hunks   int      `json:"hunks"`
		Files   []string `json:"files"`
		CRLF    bool     `json:"crlf"`
	} `json:"patches"`
	ExitCode int      `json:"exit_code"`
	Verified bool     `json:"verified"`
	Caveat   string   `json:"caveat"`
	Log      []string `json:"log"`
}

func (p *patchPortTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params PatchPortParams
	if call.Input != "" {
		if err := UnmarshalToolInput(call.Input, &params); err != nil {
			return NewTextErrorResponse(fmt.Sprintf("could not read the parameters: %s", err)), nil
		}
	}

	op := strings.ToLower(strings.TrimSpace(params.Operation))
	if op == "" {
		op = "inspect"
	}
	switch op {
	case "inspect", "forward-port", "backport", "rebase", "refresh", "port-series":
	default:
		return NewTextErrorResponse(fmt.Sprintf(
			"unknown operation %q. Use one of: inspect, forward-port, backport, "+
				"rebase, refresh, port-series.", params.Operation)), nil
	}

	tree := strings.TrimSpace(params.Tree)
	if tree == "" {
		tree = config.WorkingDirectory()
	}
	if !filepath.IsAbs(tree) {
		tree = filepath.Join(config.WorkingDirectory(), tree)
	}

	script, err := codereview.Unpack(config.Get().Data.Directory)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("could not unpack the toolkit: %s", err)), nil
	}
	// Unpack returns code_review.py; patch_port.py is its neighbour.
	porter := filepath.Join(filepath.Dir(script), "patch_port.py")

	python, preArgs, err := getPythonBinary()
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}

	args := append(append([]string{}, preArgs...), porter, "--tree", tree, "--op", op, "--json")
	if params.Patch != "" {
		args = append(args, "--patch", params.Patch)
	}
	if params.Series != "" {
		args = append(args, "--series", params.Series)
	}
	if params.Onto != "" {
		args = append(args, "--onto", params.Onto)
	}
	if params.Build != "" {
		args = append(args, "--build", params.Build)
	}
	if params.Test != "" {
		args = append(args, "--test", params.Test)
	}

	// inspect only reads. Everything else rewrites a git tree — checking out a
	// different base, applying patches, possibly leaving conflict markers — so
	// it asks first, and the description says plainly what is about to happen
	// to which tree. A user who says yes to "inspect" has not said yes to
	// "rebase my kernel".
	if op != "inspect" {
		sid, mid := GetContextValues(ctx)
		if sid == "" || mid == "" {
			return ToolResponse{}, fmt.Errorf("session or message id missing from the context")
		}
		what := op + " in " + tree
		if params.Onto != "" {
			what += ", onto " + params.Onto
		}
		if params.Series != "" {
			what += ", series " + params.Series
		} else if params.Patch != "" {
			what += ", patch " + params.Patch
		}
		if !p.permissions.Request(permission.CreatePermissionRequest{
			SessionID:   sid,
			Path:        tree,
			ToolName:    PatchPortToolName,
			Action:      op,
			Description: "Modify this git tree: " + what,
			GrantKey:    tree + "|" + op,
			Params:      params,
		}) {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, patchPortTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, python, args...)
	cmd.Dir = tree
	out, runErr := cmd.Output()

	if runCtx.Err() == context.DeadlineExceeded {
		return NewTextResponse(fmt.Sprintf(
			"The %s timed out after %s and was stopped. The tree may be part-way "+
				"through the operation — check `git status` in %s before doing "+
				"anything else.", op, patchPortTimeout, tree)), nil
	}

	var rep patchPortReport
	if err := json.Unmarshal(out, &rep); err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" && runErr != nil {
			detail = runErr.Error()
		}
		return NewTextErrorResponse(fmt.Sprintf(
			"the porting tool did not return a readable report: %s\n\n%s",
			err, oneLineOf(detail, 800))), nil
	}

	return NewTextResponse(summarisePatchPort(&rep, op, params)), nil
}

// summarisePatchPort turns the report into something a model cannot casually
// misread as success. The ordering is deliberate: what was NOT proven comes
// before what was done.
func summarisePatchPort(rep *patchPortReport, op string, params PatchPortParams) string {
	var b strings.Builder

	ok := rep.ExitCode == 0
	fmt.Fprintf(&b, "patch_port: %s — %s\n", op,
		map[bool]string{true: "completed", false: "did NOT complete"}[ok])
	fmt.Fprintf(&b, "tree: %s (%s, base %s)\n", rep.Tree, rep.TreeKind, rep.Base)
	if !rep.WorktreeClean {
		b.WriteString("worktree was NOT clean when this ran.\n")
	}
	for _, p := range rep.Problems {
		fmt.Fprintf(&b, "problem: %s\n", p)
	}

	// The verification state is the single most misreadable thing here, so it
	// goes near the top rather than in a footnote.
	if op != "inspect" {
		if rep.Verified {
			b.WriteString("\nVERIFIED: the build/test command given was run and passed.\n")
		} else {
			b.WriteString("\nUNVERIFIED: no build or test command was given, so NOTHING was " +
				"compiled or run. The patches applying is not evidence the result is correct. " +
				"Say this to the user; do not call the port done.\n")
		}
	}

	if len(rep.Patches) > 0 {
		fmt.Fprintf(&b, "\npatches (%d):\n", len(rep.Patches))
		for _, p := range capped(rep.Patches, 40) {
			crlf := ""
			if p.CRLF {
				crlf = "  [CRLF patch]"
			}
			fmt.Fprintf(&b, "  %s — %s (%d hunk(s), %d file(s))%s\n",
				p.Name, oneLineOf(p.Subject, 90), p.Hunks, len(p.Files), crlf)
		}
		if len(rep.Patches) > 40 {
			fmt.Fprintf(&b, "  ... and %d more\n", len(rep.Patches)-40)
		}
	}

	// The log carries the per-patch outcome lines, which are the substance.
	if len(rep.Log) > 0 {
		b.WriteString("\nwhat happened:\n")
		for _, line := range capped(rep.Log, patchPortMaxLogLines) {
			b.WriteString("  " + line + "\n")
		}
		if len(rep.Log) > patchPortMaxLogLines {
			fmt.Fprintf(&b, "  ... %d more lines\n", len(rep.Log)-patchPortMaxLogLines)
		}
	}

	// Surface the dangerous outcomes explicitly rather than hoping the model
	// notices them in the log.
	joined := strings.Join(rep.Log, "\n")
	if strings.Contains(joined, "applied-with-fuzz") || strings.Contains(joined, "WITH FUZZ") {
		b.WriteString("\nFUZZ WAS USED. At least one hunk was relocated by searching for its " +
			"context rather than matched at its recorded line. That can place a change in the " +
			"WRONG part of the file and still report success. Show the user `git diff` for the " +
			"affected files and say which ones were fuzzed.\n")
	}
	if strings.Contains(joined, "three-way") {
		b.WriteString("\nA patch was merged three-way: context had moved and git merged it " +
			"using blob history. Read the result before trusting it.\n")
	}
	if rep.Caveat != "" {
		b.WriteString("\n" + rep.Caveat + "\n")
	}
	return b.String()
}
