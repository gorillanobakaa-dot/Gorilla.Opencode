package prompt

import (
	"context"
	_ "embed"
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
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE: the modern base prompt. Replaces the two 2023-era
// prompts (baseOpenAICoderPrompt / baseAnthropicCoderPrompt, ~2,003
// tokens, heavy ALL-CAPS + threat-toned emphasis). Neutral and
// declarative — no shouty caps, no "IMPORTANT"/"NEVER" stacking —
// synthesised from the SOTA agentic-prompting research (faithful
// outcome reporting, anti-hallucination, loop discipline) and
// specialised for building large systems (Firefox/mach, Linux kernel,
// Windows internals).
//
// 2026-07-29: reworked against Anthropic's Claude Fable 5
// prompting guidance, which is written for exactly this shape of
// workload — long-horizon autonomous runs where nobody is watching.
// Three sections are new (scope, delegation, memory) and honesty,
// verification, output and conduct were expanded. Measured cost:
// ~464 -> ~1058 estimated tokens per turn. That is a real regression in
// per-turn overhead and it buys grounded progress claims, an explicit
// stop rule, and no unrequested actions; every section remains
// individually switchable in /context. See system-prompts/README.md for
// the rationale and RESEARCH-SOURCES.md for the citations.
//
// Editable: internal/llm/prompt/coder-modern.txt; study copy in
// system-prompts/current/coder-modern.md.
//
//go:embed coder-modern.txt
var baseModernCoderPrompt string

// BaseCoderPrompt is the base instructions only (no env/LSP blocks).
// GORILLA OVERRIDE: exported so the context loadout can measure it.
// Provider-neutral now — the modern prompt works across providers.
func BaseCoderPrompt(provider models.ModelProvider) string {
	// GORILLA OVERRIDE: assembled from the sections the /context loadout leaves
	// enabled, over the ACTIVE prompt text (user override if present, else the
	// embedded factory copy). Was a flat return of the embedded constant.
	return assembleCoderPrompt()
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
// has the find tool for deep exploration on demand.
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
%s</env>
<project_summary>
%s
</project_summary>
%s`, cwd, boolToYesNo(isGit), platform, userDirs(), summary, extra)
}

// userDirs tells the model where this machine actually keeps pictures,
// downloads and documents.
//
// GORILLA OVERRIDE (2026-08-19), from a real run: asked to look in "my
// screenshots folder", the agent ran a home-wide find for *screenshot*, got a
// page of matches from inside other projects' documentation trees, tried a
// second search that TIMED OUT AT 30 SECONDS, and finally fell back to
// guessing /home/gorilla/Pictures through bash — about two minutes and three
// tool calls to find a directory the operating system could have named
// instantly.
//
// That was read as the model being limited. It is fairer to say the program
// never told it. These paths are not a convention to be inferred, they are
// configuration: XDG writes them to ~/.config/user-dirs.dirs and any desktop
// Linux can print them. Localised installs make guessing actively wrong —
// "Pictures" is "Bilder", "Imágenes", "画像" depending on the user's language,
// so an English guess fails on exactly the machines this project is built for.
//
// Deliberately FACTS, not instructions: this block states where things are and
// says nothing about what to do, which is what keeps it cheap. Measured at
// roughly 30 tokens, only for the directories that actually exist, and it
// rides the prompt.env row so anyone who does not want it can switch it off.
func userDirs() string {
	// Read the XDG config rather than shelling out to xdg-user-dir: no process
	// per prompt render, and it works when xdg-utils is not installed.
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(home, ".config", "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	// Three, not six. The full XDG set measured 66 tokens on every turn and
	// Videos and Music are not a coding agent's business — if it ever needs
	// them it can ask, and asking costs nothing until it happens. These three
	// are the ones that prevent the failure that prompted this: where
	// screenshots land, where downloads land, and where the user's own work
	// lives. Measured at ~35 tokens.
	want := map[string]string{
		"XDG_DOWNLOAD_DIR":  "Downloads",
		"XDG_DOCUMENTS_DIR": "Documents",
		"XDG_PICTURES_DIR":  "Pictures (screenshots land here)",
	}
	order := []string{"XDG_DOCUMENTS_DIR", "XDG_DOWNLOAD_DIR", "XDG_PICTURES_DIR"}
	found := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if _, wanted := want[strings.TrimSpace(k)]; !wanted {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"`)
		v = strings.Replace(v, "$HOME", home, 1)
		// Only claim a directory that is really there. A path the model is
		// told about and cannot open is worse than silence.
		if st, err := os.Stat(v); err != nil || !st.IsDir() {
			continue
		}
		found[strings.TrimSpace(k)] = v
	}
	if len(found) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("This machine's standard folders:\n")
	for _, k := range order {
		if v, ok := found[k]; ok {
			fmt.Fprintf(&b, "  %s: %s\n", want[k], v)
		}
	}
	return b.String()
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
	b.WriteString("Top-level (depth 1, not a full tree — use the find tool for deeper paths):\n")
	b.WriteString(listTopLevelBrief(cwd, maxTopLevelEntries))
	// GORILLA OVERRIDE (2026-09-01): the branch NAME stays; `git status` goes.
	//
	// See getEnvironmentInfo for the measurement behind this. The branch is
	// stable for hours at a time and worth its handful of tokens. The
	// working-tree status changes on every edit the agent itself makes, and it
	// sits near the front of a ~6,500-token prompt, so each change threw away
	// the KV cache for everything after it.
	//
	// It was also the most misleading line in the prompt. It is captured once,
	// when the system prompt is built, and is stale the moment the agent writes
	// a file — so a model reading it late in a turn was being told the state of
	// the tree from before its own last several edits, presented as current.
	//
	// Nothing is lost that cannot be had accurately: `git status` through the
	// shell tool answers for the moment it is asked.
	if isGit {
		if br := gitBranchBrief(cwd); br != "" {
			b.WriteString("\n")
			b.WriteString(br)
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
	var dirs, files []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name+"/")
		} else {
			files = append(files, name)
		}
	}
	if len(dirs)+len(files) == 0 {
		return "(empty or only hidden entries)"
	}
	sort.Strings(dirs)
	files = collapseVersionFamilies(files)
	sort.Strings(files)

	// GORILLA OVERRIDE (2026-08-09): directories first, then files.
	//
	// This was a plain sort.Strings over everything, which is ASCII, which puts
	// CAPITALS first. In this very repo that meant the 25 slots were consumed by
	// GITHUB-RELEASE-NOTES-0.1.65 ... 0.1.77 and SHA256SUMS-*, and NOT ONE source
	// entry reached the model - no cmd/, no internal/, no go.mod. The block whose
	// entire job is "where am I" was describing a pile of old builds.
	//
	// It is not a naming problem: lowercasing every filename puts internal/ at
	// position 78, because 47 .deb artifacts sort before it either way. The root
	// simply had 96 entries and a 25-entry budget.
	//
	// Directories are the orientation signal; loose files in a repo root are
	// mostly output. Listing dirs first means the answer to "what is this
	// project" survives any amount of accumulated detritus.
	names := append(dirs, files...)
	shown := names
	extra := 0
	if len(shown) > limit {
		extra = len(shown) - limit
		shown = shown[:limit]
	}
	out := strings.Join(shown, "\n")
	if extra > 0 {
		out += fmt.Sprintf("\n... +%d more (not listed)", extra)
	}
	return out
}

// versionRunRe matches a run of digits and dots - the part that varies across
// release artifacts (0.1.65, 1.2.3, 20260809).
var versionRunRe = regexp.MustCompile(`[0-9][0-9.]*`)

// collapseVersionFamilies folds files that differ only by a version number into
// a single line: 47 copies of gorilla-opencode_0.1.NN_amd64.deb become
// "gorilla-opencode_*_amd64.deb (47 files)".
//
// GORILLA OVERRIDE (2026-08-09): this is the honest fix for a truncated
// listing. The alternative - raising the cap - re-introduces the 10k-30k
// tokens/turn this block was written to eliminate. Collapsing instead makes the
// listing SMALLER while showing MORE: the fact that 47 .deb files exist is one
// line of information, not 47, and the 46 slots freed go to entries that
// actually distinguish this project from any other.
//
// It also removes an accidental prompt injection. Thirteen consecutive
// GITHUB-RELEASE-NOTES-0.1.65...0.1.77 lines read as a sequence, and on 2026-08-09
// a model given the input "oi" and nothing else continued that sequence: it ran
// `git tag -a v0.1.78`, and the tag was really created. Nothing was
// hallucinated - the context contained a monotonic counter and an idle agent.
// One collapsed line carries the same fact and invites nothing.
func collapseVersionFamilies(files []string) []string {
	const minFamily = 3

	type fam struct {
		pattern string
		count   int
		first   int
	}
	order := []string{}
	byKey := map[string]*fam{}

	for i, f := range files {
		loc := versionRunRe.FindStringIndex(f)
		if loc == nil {
			continue
		}
		key := f[:loc[0]] + "\x00" + f[loc[1]:]
		if fm, ok := byKey[key]; ok {
			fm.count++
		} else {
			byKey[key] = &fam{pattern: f[:loc[0]] + "*" + f[loc[1]:], count: 1, first: i}
			order = append(order, key)
		}
	}

	collapsed := map[string]bool{}
	out := make([]string, 0, len(files))
	for _, key := range order {
		if byKey[key].count >= minFamily {
			collapsed[key] = true
		}
	}
	seen := map[string]bool{}
	for _, f := range files {
		loc := versionRunRe.FindStringIndex(f)
		key := ""
		if loc != nil {
			key = f[:loc[0]] + "\x00" + f[loc[1]:]
		}
		if key != "" && collapsed[key] {
			if !seen[key] {
				seen[key] = true
				fm := byKey[key]
				out = append(out, fmt.Sprintf("%s (%d files)", fm.pattern, fm.count))
			}
			continue
		}
		out = append(out, f)
	}
	return out
}

// gitBranchBrief returns just the current branch name.
//
// GORILLA OVERRIDE (2026-09-01): split out of gitStatusBrief so the STABLE half
// of that information can ride the prompt while the volatile half does not. See
// projectSummary. Failures are silent: no git, a detached HEAD or a fresh repo
// with no commits must never break the system prompt.
func gitBranchBrief(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	if b := strings.TrimSpace(string(out)); b != "" {
		return "branch: " + b
	}
	return ""
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
		body += fmt.Sprintf("\n... +%d more changed paths (not listed)", extra)
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
