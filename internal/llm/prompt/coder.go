package prompt

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE: the modern base prompt. Replaces the two 2023-era
// prompts (baseOpenAICoderPrompt / baseAnthropicCoderPrompt, ~2,003
// tokens, heavy ALL-CAPS + markdown headers). This one is ~924 tokens
// of plain declarative prose — no shouty caps, no "#" scaffolding for a
// swarm to echo — synthesised from the modern Claude Code prompt plus
// the SOTA agentic-prompting research (neutral/imperative, faithful
// outcome reporting, anti-hallucination, loop discipline) and
// specialised for building large systems (Firefox/mach, Linux kernel,
// Windows internals). Editable: internal/llm/prompt/coder-modern.txt;
// study copy in system-prompts/proposed/.
//
//go:embed coder-modern.txt
var baseModernCoderPrompt string

// BaseCoderPrompt is the base instructions only (no env/LSP blocks).
// GORILLA OVERRIDE: exported so the context loadout can measure it.
// Provider-neutral now — the modern prompt works across providers.
func BaseCoderPrompt(provider models.ModelProvider) string {
	return strings.TrimSpace(baseModernCoderPrompt)
}

// EnvironmentInfoBlock / LSPInfoBlock expose the switchable prompt blocks
// so the loadout can price them. GORILLA OVERRIDE.
func EnvironmentInfoBlock() string { return getEnvironmentInfo() }
func LSPInfoBlock() string         { return lspInformation() }

func CoderPrompt(provider models.ModelProvider) string {
	// GORILLA OVERRIDE: env and LSP context blocks are switchable via the
	// context loadout (/context menu). Off = fewer tokens per turn, at the
	// cost of the agent knowing your cwd/OS/git and active LSPs. Because
	// this is re-evaluated whenever the system prompt is (re)built, a
	// toggle takes effect on the next rebuild (see ReloadCoderTools).
	envInfo := ""
	if config.LoadoutEnabled("prompt.env") {
		envInfo = getEnvironmentInfo()
	}
	lspInfo := ""
	if config.LoadoutEnabled("prompt.lsp") {
		lspInfo = lspInformation()
	}

	return fmt.Sprintf("%s\n\n%s\n%s", BaseCoderPrompt(provider), envInfo, lspInfo)
}

// GORILLA OVERRIDE: two 2023-era prompt constants (baseOpenAICoderPrompt at
// 4,194 bytes and baseAnthropicCoderPrompt at 8,014 bytes) used to sit here,
// declared and never referenced — BaseCoderPrompt returned only the modern
// prompt. They were verified stripped from the compiled binary by the linker's
// dead-code elimination (probed with `strings /usr/bin/gorilla-opencode`), so
// removing them is a code-hygiene fix, not a token or binary-size fix. The risk
// they closed is real: a future edit rewiring one of them back into the
// provider-selection switch would silently ship a 2023 prompt again. Study
// copies remain in system-prompts/current/coder-{anthropic,openai}.md.

// GORILLA OVERRIDE: environment block used to call the recursive LS tool
// with MaxLSFiles=1000 and dump the whole tree into every system prompt.
// On large trees (home dir, Firefox, kernel) that alone was ~10k–30k tokens
// per turn — the dominant fixed overhead, and hostile to metered / satellite
// links. Now: depth-1 listing (capped) + short git status. The agent still
// has ls/glob/grep tools for deep exploration on demand.
const (
	maxTopLevelEntries = 25
	maxGitStatusLines  = 10
	// GORILLA OVERRIDE: extra /add-dir roots get a shallower listing than the
	// primary. Budget ~40-60 tokens per added root in a block that ships every
	// turn.
	maxExtraRootEntries = 12
)

func getEnvironmentInfo() string {
	roots := config.Roots()
	cwd := roots[0]
	isGit := isGitRepo(cwd)
	platform := runtime.GOOS
	date := time.Now().Format("1/2/2006")
	summary := projectSummary(cwd, isGit)

	// GORILLA OVERRIDE: additional roots from /add-dir are advertised here, or
	// the model has no idea they are in play and will not look in them. Kept
	// deliberately shallower than the primary root — this block rides EVERY
	// turn, and extra roots are the one place /add-dir can quietly cost tokens.
	// gitStatusBrief is NOT called for extras: it shells out to git with a 2s
	// timeout, and doing that per root per render is a latency risk on a slow
	// link.
	extra := ""
	if len(roots) > 1 {
		var b strings.Builder
		b.WriteString("\nAdditional workspace roots (also yours to work in):\n")
		for _, r := range roots[1:] {
			fmt.Fprintf(&b, "- %s (git repo: %s)\n", r, boolToYesNo(isGitRepo(r)))
			for _, line := range strings.Split(listTopLevelBrief(r, maxExtraRootEntries), "\n") {
				fmt.Fprintf(&b, "    %s\n", line)
			}
		}
		extra = b.String()
	}

	return fmt.Sprintf(`Here is useful information about the environment you are running in:
<env>
Working directory: %s
Is directory a git repo: %s
Platform: %s
Today's date: %s
</env>
<project_summary>
%s
</project_summary>
%s`, cwd, boolToYesNo(isGit), platform, date, summary, extra)
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// projectSummary is a shallow, token-cheap sketch of the workspace:
// top-level names (max maxTopLevelEntries) and, if a git repo, a short
// status snippet (max maxGitStatusLines). No recursive walk.
func projectSummary(cwd string, isGit bool) string {
	var b strings.Builder
	b.WriteString("Top-level (depth 1, not a full tree — use ls/glob/grep for deeper paths):\n")
	b.WriteString(listTopLevelBrief(cwd, maxTopLevelEntries))
	if isGit {
		if g := gitStatusBrief(cwd, maxGitStatusLines); g != "" {
			b.WriteString("\nGit status (short, capped):\n")
			b.WriteString(g)
		}
	}
	return strings.TrimSpace(b.String())
}

// listTopLevelBrief lists only the immediate children of dir. Hidden
// names (leading '.') are skipped. Directories are marked with a trailing
// '/'. Output is capped at limit entries (plus a "+N more" line).
func listTopLevelBrief(dir string, limit int) string {
	if limit <= 0 {
		limit = maxTopLevelEntries
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Sprintf("(could not list directory: %v)", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "(empty or only hidden entries)"
	}
	shown := names
	extra := 0
	if len(shown) > limit {
		extra = len(shown) - limit
		shown = shown[:limit]
	}
	out := strings.Join(shown, "\n")
	if extra > 0 {
		out += fmt.Sprintf("\n… +%d more (not listed)", extra)
	}
	return out
}

// gitStatusBrief returns `git status --short` (plus branch name) with a
// hard line cap. Failures are silent — missing git must not break the
// system prompt.
func gitStatusBrief(dir string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = maxGitStatusLines
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	branch := ""
	if out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}

	out, err := exec.CommandContext(ctx, "git", "-C", dir, "status", "--short").Output()
	if err != nil {
		if branch != "" {
			return "branch: " + branch
		}
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		if branch != "" {
			return "branch: " + branch + "\nclean working tree"
		}
		return "clean working tree"
	}
	extra := 0
	if len(lines) > maxLines {
		extra = len(lines) - maxLines
		lines = lines[:maxLines]
	}
	body := strings.Join(lines, "\n")
	if extra > 0 {
		body += fmt.Sprintf("\n… +%d more changed paths (not listed)", extra)
	}
	if branch != "" {
		return "branch: " + branch + "\n" + body
	}
	return body
}

func lspInformation() string {
	// GORILLA OVERRIDE: name the servers that are actually running instead of a
	// generic paragraph. The model can then reason about which languages have
	// live diagnostics — previously it was told diagnostics "may" appear with no
	// way to know for which files, so it either ignored them or assumed
	// coverage it did not have.
	active := config.EnabledLSPNames()
	if len(active) == 0 {
		return ""
	}
	return fmt.Sprintf(`# LSP Information
Active language servers: %s. Edits to files they cover come back with live
compiler/linter diagnostics in <file_diagnostics></file_diagnostics> and
<project_diagnostics></project_diagnostics> tags.
- Fix diagnostics your change caused, in the same turn.
- Ignore diagnostics in files you did not touch unless asked.
- A language not listed above has NO live diagnostics: verify those edits by
  building or running tests instead of assuming they are clean.
`, strings.Join(active, ", "))
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
