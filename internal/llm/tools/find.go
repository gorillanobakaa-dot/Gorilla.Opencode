package tools

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// GORILLA OVERRIDE: this tool did not exist upstream. It replaces the separate
// ls, glob and grep tools with one call backed by pfind
// (Scripts.For.Work/pfind), and it exists for two measured reasons.
//
// (1) Description cost. Those three tools carried 1,162 tokens of prose plus
// their schemas — ~1,485 tokens on EVERY turn, whether or not the agent
// searched anything. Three descriptions repeating WHEN TO USE / HOW TO USE /
// LIMITATIONS / TIPS is three times the boilerplate for one job: "where is
// this thing".
//
// (2) The round trip, which is the larger cost by an order of magnitude. grep
// returned PATHS ONLY, so it could never answer a question — only say where to
// look. The agent then had to call view, and view returns the whole file.
// Measured on internal/llm/tools: grep's answer was 16 tokens, the view that
// had to follow was 1,829. This tool returns the matching lines with context
// (132 tokens for the same question) and the second turn does not happen at
// all. At ~9,935 tokens of standing context per turn, a turn avoided is worth
// far more than any description saving.
//
// The three old tools remain in the tree as *.go.retired files — renamed so
// they no longer compile, not deleted. Reverting is renaming them back and
// re-registering (see internal/llm/agent/tools.go).

type FindParams struct {
	Query        string `json:"query"`
	Path         string `json:"path"`
	Glob         string `json:"glob"`
	Type         string `json:"type"`
	Regex        bool   `json:"regex"`
	Fuzzy        bool   `json:"fuzzy"`
	Recent       bool   `json:"recent"`
	ModifiedOnly bool   `json:"modified_only"`
	View         string `json:"view"`
	FilesOnly    bool   `json:"files_only"`
}

type FindResponseMetadata struct {
	Engine    string `json:"engine"`
	Truncated bool   `json:"truncated"`
	Bytes     int    `json:"bytes"`
	DurationS string `json:"duration_s"`
}

type findTool struct{}

const (
	FindToolName = "find"

	// findMaxFiles bounds how many files pfind may report.
	findMaxFiles = 15

	// findMaxPerFile bounds how many MATCHES are shown per file, with their
	// context. Without it one kernel file with 36 hits emitted every one of
	// them: the same query returned 35 KB before this cap and 10.5 KB after.
	// pfind prints "… N more match(es) in this file" when it bites, so the
	// agent can tell a capped result from a complete one.
	findMaxPerFile = 3

	// findMaxBytes bounds the RESULT, in bytes, which is the unit of the
	// resource actually at risk.
	//
	// This is deliberately far tighter than the 400 KB backstop in
	// NewTextResponse. That backstop exists to stop a catastrophe; this exists
	// to stop a slow bleed. Every tool result is appended to the message
	// history and re-sent on every later turn, so a 200 KB search result is not
	// a one-off — it is 200 KB added to the bill for the rest of the
	// conversation. The old grep tool capped MATCHES at 100 and returned
	// 2.4 MB by matching inside minified JSON; counting items is a proxy that
	// fails exactly when items are unusual. 32 KB is roughly 8,000 tokens: big
	// enough for a real answer with context, small enough that a pathological
	// match cannot quietly double a conversation.
	findMaxBytes = 32 * 1024

	// findMaxLineChars truncates individual output lines. 500 characters is
	// several code lines' worth; anything longer is minified/generated data
	// whose tail carries no information the head did not.
	findMaxLineChars = 500

	// findTimeout caps a single search. pfind searched a 354 MB tree in 131 ms
	// on the reference hardware; 30 s is a hang guard, not a budget — though a
	// no-path search of an entire home directory measured 27.8 s, so it is a
	// guard that real calls can approach.
	findTimeout = 30 * time.Second

	findDescription = `Search code and find files. THIS TOOL REPLACES grep, glob, ls, rg, ripgrep, list_dir, search_files and file_search — there are no separate tools for those here. Any time you want to search text, find a file, or see what is in a folder, call find. It works on very large trees (a 3.4 GB, 120,000-file kernel checkout searches in about 1.6 seconds).

WHAT IT RETURNS
By default it returns each matching line WITH 2 LINES OF CONTEXT either side, grouped under the file path. That is usually enough to answer the question, so do NOT follow it with a call to read the whole file unless you actually need more.

Search results are RANKED, best first: matches on the file's NAME, matches in its CONTENT, and how recently it changed are combined, so the likeliest file comes first even when you do not know the filename. The query text itself matches EXACTLY: query="media decoder" finds that exact phrase, not the two words separately. Directory listings and glob-only file finds are plain lists, not ranked.

HOW TO USE IT
- Search inside files:      query="ath9k_hw_init"
- Search a specific tree:   query="rate control", path="/home/me/linux-7.1.2"
- Narrow by language:       query="kmalloc", type="c"
- Narrow by filename:       query="init", glob="*.h"
- Find files by name only:  glob="*.rs"        (no query needed)
- List what is in a folder:  path="src/audio"  (no query, no glob)
- Just the file paths:      query="TODO", files_only=true
- Regular expression:       query="ath9k_.*_init", regex=true
- Misremembered a name:     query="buldparser", fuzzy=true   (typo-tolerant name matching)
- Freshest results first:   query="decoder", recent=true     (recently-modified files rank first)
- Only work in progress:    query="TODO", modified_only=true (files with uncommitted git changes)

EXPLORING (the view argument)
- view="tree"  directory structure as a tree — the fastest way to understand a project's layout
- view="long"  files with permissions, size, modified time and git status badges (M=modified, ?=untracked); add recent=true to sort newest first
- view="code"  lines-of-code summary by language for a whole tree — instantly answers "what is this project written in and how big is it" (query is ignored in this view)
All views work with a path alone; tree and long also work with a query, showing the matches inside that view.

CHOOSING GOOD ARGUMENTS ON A BIG TREE
A kernel or browser checkout holds hundreds of thousands of files. An unnarrowed query there returns a truncated, low-value result. Narrow it:
- type is the strongest filter and the easiest to get right. Valid values include: c, cpp, rust, python, js, ts, go, java, sh, html, css, json, yaml, toml, md, sql, asm, make, cmake.
- path is the next strongest. If you know the subsystem, point at it.
- glob accepts *.ext, src/**/*.ts, and !*.min.js to EXCLUDE.

IMPORTANT
- query is a LITERAL string unless you set regex=true. Search for "foo(bar)" or "a.b" directly; you do not need to escape anything.
- Skipped automatically: hidden files, anything .gitignore excludes, and common build/dependency junk (node_modules, build, dist, minified/compiled files). Search those explicitly by path if you really need them.
- Output is capped. If the result says TRUNCATED, "more matches in this file", or "only N shown", it is INCOMPLETE — narrow it with type, path or glob and search again. Do not draw conclusions from a cut-off result.
- "No matches found" means the search RAN and nothing matched — try a shorter or less specific query before concluding the code is absent. A failed search says FAILED instead; do not treat a failure as absence.`
)

// countSummary parses pfind's --count stderr line:
// "--- 27 file(s) matched, showing 15 ---". The "showing" clause only appears
// when the cap actually cut something.
var countSummary = regexp.MustCompile(`--- (\d+) file\(s\) matched(?:, showing (\d+))? ---`)

// normaliseGlob makes slash-containing globs behave the way every example in
// the description promises. pfind (like ripgrep overrides) matches a glob with
// a "/" in it against the full path of each candidate — which here is always
// absolute, so "src/**/*.ts" could never match anything. Anchoring it as
// "**/src/**/*.ts" matches that subtree at any depth, which is what the
// pattern means to the person typing it. Globs that already start with "**",
// "/" or "~" are left alone; a leading "!" (exclude) is preserved.
func normaliseGlob(g string) string {
	neg := ""
	if strings.HasPrefix(g, "!") {
		neg, g = "!", g[1:]
	}
	if strings.Contains(g, "/") && !strings.HasPrefix(g, "**") &&
		!strings.HasPrefix(g, "/") && !strings.HasPrefix(g, "~") {
		g = "**/" + g
	}
	return neg + g
}

func NewFindTool() BaseTool {
	return &findTool{}
}

func (f *findTool) Info() ToolInfo {
	return ToolInfo{
		Name:        FindToolName,
		Description: findDescription,
		Parameters: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Text or regex to find inside files. Omit to list files instead of searching them.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory or file to search (defaults to the working directory).",
			},
			"glob": map[string]any{
				"type":        "string",
				"description": "Only consider files matching this glob, e.g. '*.go' or 'src/**/*.ts'. Prefix with ! to exclude.",
			},
			"type": map[string]any{
				"type":        "string",
				"description": "Restrict to one language: c, cpp, rust, python, js, ts, go, java, sh, html, css, json, yaml, toml, md, sql, asm, make, cmake. The best way to narrow a search on a large tree.",
			},
			"regex": map[string]any{
				"type":        "boolean",
				"description": "Treat query as a regular expression. Default false, meaning query is matched literally.",
			},
			"fuzzy": map[string]any{
				"type":        "boolean",
				"description": "Typo-tolerant filename matching: query='buldparser' still finds build_parser.py. Use when unsure of exact spelling.",
			},
			"recent": map[string]any{
				"type":        "boolean",
				"description": "Rank or sort recently-modified files first. Best for 'where did I just change X'.",
			},
			"modified_only": map[string]any{
				"type":        "boolean",
				"description": "Only files with uncommitted git changes (listings); in a search, rank them first. 'My work in progress'.",
			},
			"view": map[string]any{
				"type":        "string",
				"description": "Presentation: 'tree' = directory tree, 'long' = permissions/size/date/git-status table, 'code' = lines-of-code per language. Empty = normal search output.",
			},
			"files_only": map[string]any{
				"type":        "boolean",
				"description": "Return matching paths only, without the matching lines. Ignored when view is set.",
			},
		},
		Required: []string{},
	}
}

// The engine itself rides inside the binary, so a bare `curl`-installed
// gorilla-opencode can search on any machine with python3 — no package, no
// config, no path that only exists on the developer's computer. The vendored
// copy is kept hash-identical to its upstream (see TestVendoredPfindMatchesDevCopy);
// both are the same author's work, shipped under this repo's terms with the
// embedded copy's AGPL header intact.
//
//go:embed pfind.py
var embeddedPfind []byte

// findPfindPath locates the pfind engine, most-explicit first:
//
//  1. GORILLA_PFIND — an explicit override. If it is set but unusable that is
//     an error, not a fall-through: silently ignoring a typo'd override would
//     run a different engine than the one the user pointed at.
//  2. `pfind` on PATH — a system install (the .deb ships /usr/bin/pfind).
//  3. The embedded copy, extracted once into the user cache.
//
// GORILLA OVERRIDE: when python3 is missing this REFUSES rather than falling
// back to a degraded search. An agent told "no matches" cannot tell that apart
// from "the search never ran", and will conclude the code does not exist.
func findPfindPath() (string, []string, error) {
	if env := os.Getenv("GORILLA_PFIND"); env != "" {
		st, err := os.Stat(env)
		if err != nil || st.IsDir() {
			return "", nil, fmt.Errorf("GORILLA_PFIND is set to %q but that is not a readable file", env)
		}
		python, err := exec.LookPath("python3")
		if err != nil {
			return "", nil, fmt.Errorf("GORILLA_PFIND points at %s but python3 is not installed", env)
		}
		return python, []string{env}, nil
	}
	if p, err := exec.LookPath("pfind"); err == nil {
		return p, nil, nil
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		return "", nil, fmt.Errorf(
			"the find tool needs python3 (its search engine is a Python program embedded in this binary). " +
				"Install python3, or install a 'pfind' executable on PATH")
	}
	script, err := extractEmbeddedPfind()
	if err != nil {
		return "", nil, fmt.Errorf("could not extract the embedded search engine: %w", err)
	}
	return python, []string{script}, nil
}

// extractEmbeddedPfind writes the embedded engine to the user cache, once per
// engine version. The filename carries a content hash, so a new binary with a
// new engine never runs a stale extract, and re-running never rewrites.
func extractEmbeddedPfind() (string, error) {
	sum := sha256.Sum256(embeddedPfind)
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		// No HOME/XDG (cron, containers). The world-writable temp dir gets a
		// per-uid subdirectory so another local user cannot pre-plant a file at
		// the predictable path and have this process execute it.
		cacheRoot = filepath.Join(os.TempDir(), fmt.Sprintf("gorilla-opencode-%d", os.Getuid()))
	}
	dir := filepath.Join(cacheRoot, "gorilla-opencode")
	target := filepath.Join(dir, fmt.Sprintf("pfind-%x.py", sum[:8]))
	// An existing file only counts if its BYTES are ours — a name and a size
	// are claims, not proof, and this file is about to be executed.
	if existing, err := os.ReadFile(target); err == nil && sha256.Sum256(existing) == sum {
		return target, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if st, err := os.Stat(dir); err != nil || st.Mode().Perm()&0o022 != 0 {
		if err == nil {
			return "", fmt.Errorf("cache dir %s is writable by other users; refusing to execute from it", dir)
		}
		return "", err
	}
	// Write-then-rename so a concurrent session never executes a half-written file.
	tmp, err := os.CreateTemp(dir, "pfind-*.tmp")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(embeddedPfind); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return target, nil
}

func (f *findTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params FindParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	searchPath := params.Path
	if searchPath == "" {
		searchPath = config.WorkingDirectory()
	}
	if !filepath.IsAbs(searchPath) {
		searchPath = filepath.Join(config.WorkingDirectory(), searchPath)
	}
	if _, err := os.Stat(searchPath); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("path does not exist: %s", searchPath)), nil
	}

	// GORILLA OVERRIDE (2026-08-18): find PRINTS MATCHING LINES, so searching a
	// credential directory puts the key itself into the transcript, the model's
	// context and the session database. Refuse the search root outright; see
	// sensitive.go. A blocklist, deliberately not a sandbox.
	if why := RefuseSensitiveRead(searchPath); why != "" {
		return NewTextErrorResponse(why), nil
	}

	if params.Query == "" && params.Glob == "" {
		// Listing mode still needs a shape; a bare directory listing is what
		// the old ls tool did, so keep that behaviour rather than erroring.
		params.FilesOnly = true
	}

	bin, prefix, err := findPfindPath()
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}

	switch params.View {
	case "", "tree", "long", "code":
	default:
		return NewTextErrorResponse(fmt.Sprintf(
			"invalid view %q — valid values: tree, long, code (or omit it for normal search output)", params.View)), nil
	}
	// view="code" aggregates a whole tree; a query has no meaning there and
	// would be misread as a path by the engine's positional parser.
	if params.View == "code" {
		params.Query = ""
	}

	// Tree rows and long-table rows are short and information-dense, so the
	// views get a higher entry budget than context-mode search output. The
	// byte cap still bounds everything.
	limit := findMaxFiles
	switch params.View {
	case "tree":
		limit = 40
	case "long":
		limit = 20
	}

	// All options go BEFORE the "--" separator; query and path go after it.
	// Without the separator a query like "-v" or "--help" would be parsed as a
	// pfind flag instead of searched for.
	args := append([]string{}, prefix...)
	args = append(args,
		"--no-color",
		"--count",
		"--limit", strconv.Itoa(limit),
		"--max", strconv.Itoa(findMaxPerFile),
		// One pathological line must not eat the byte budget. The crocodile
		// test (2026-08-17) matched a spell-checker wordlist: a single
		// multi-thousand-character line consumed space that could have held
		// ten more files' worth of real matches inside the 32 KB cap.
		// --max-columns-preview keeps the line's head with an explicit
		// "[… omitted end of long line]" marker instead of dropping it.
		"--max-columns", strconv.Itoa(findMaxLineChars),
		"--max-columns-preview",
	)
	if params.Glob != "" {
		args = append(args, "--glob", normaliseGlob(params.Glob))
	}
	if params.Type != "" {
		args = append(args, "--type", params.Type)
	}
	if params.Regex {
		args = append(args, "--regex")
	}
	if params.Fuzzy {
		args = append(args, "--fuzzy")
	}
	if params.ModifiedOnly {
		// Listings FILTER to uncommitted files; searches add the git signal to
		// the ranking fusion (pfind does not filter search results by git
		// state — the boost is what its RRF design provides).
		args = append(args, "--git-dirty")
		if params.Query != "" {
			args = append(args, "--dirty-boost")
		}
	}
	if params.Recent {
		if params.Query != "" {
			args = append(args, "--recency-boost")
		} else {
			args = append(args, "--sortr", "modified")
		}
	}

	switch params.View {
	case "tree":
		args = append(args, "--tree")
	case "long":
		args = append(args, "--long")
	case "code":
		args = append(args, "--code")
	default:
		if params.Query == "" {
			if params.Recent || params.ModifiedOnly {
				// Sorted/filtered listings go through pfind's exploratory
				// mode (its --files fast path neither sorts nor consults
				// git); one path per line keeps it compact.
				args = append(args, "--oneline")
			} else {
				// Plain listing: --files lists what would be searched. Depth-
				// limit it like ls would, so a kernel-sized tree does not
				// answer with its first 15 files in traversal order. pfind
				// caps the count and prints "… and N more" when it cuts.
				args = append(args, "--files")
				if params.Glob == "" {
					args = append(args, "--max-depth", "2")
				}
			}
		} else if params.FilesOnly {
			args = append(args, "--files-only")
		} else {
			// The whole point of replacing grep: return enough to answer the
			// question so the agent does not need a follow-up view call.
			args = append(args, "-C", "2")
		}
	}
	args = append(args, "--")
	if params.Query != "" {
		args = append(args, params.Query)
	}
	args = append(args, searchPath)

	runCtx, cancel := context.WithTimeout(ctx, findTimeout)
	defer cancel()

	started := time.Now()
	cmd := exec.CommandContext(runCtx, bin, args...)
	cmd.Dir = config.WorkingDirectory()
	var stdout, stderrBuf strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderrBuf
	runErr := cmd.Run()
	elapsed := time.Since(started)
	stderr := strings.TrimSpace(stderrBuf.String())

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return NewTextErrorResponse(fmt.Sprintf(
			"search timed out after %s. Narrow it with path or glob", findTimeout)), nil
	}
	// Exit-code contract (grep convention, enforced in pfind.py's own __main__
	// handler): 0 = matches, 1 = no matches, anything else = the search FAILED.
	// The two must never be conflated — an agent told "no matches" when the
	// search never ran concludes the code does not exist. A Traceback on
	// exit 1 means an older engine crashed through Python's default handler
	// (which also exits 1), so that shape is treated as a failure too.
	var exitErr *exec.ExitError
	if runErr != nil && errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1 &&
		!strings.Contains(stderr, "Traceback") {
		return WithResponseMetadata(
			NewTextResponse("No matches found."),
			FindResponseMetadata{Engine: "pfind", DurationS: elapsed.Round(time.Millisecond).String()},
		), nil
	}
	if runErr != nil {
		if stderr == "" {
			stderr = runErr.Error()
		}
		return NewTextErrorResponse(fmt.Sprintf("search FAILED (it did not run to completion — this is not \"no matches\"): %s", stderr)), nil
	}

	body := stdout.String()
	// pfind's --count summary goes to stderr as "--- N file(s) matched, showing M ---".
	// When more files matched than the cap allowed, say so in the result body;
	// a silently capped list reads as the complete answer.
	if m := countSummary.FindStringSubmatch(stderr); m != nil && m[2] != "" {
		body += fmt.Sprintf("\n[%s files matched; only %s shown — narrow with type, path or glob]", m[1], m[2])
	}
	rawBytes := len(body)
	truncated := false
	if rawBytes > findMaxBytes {
		// Truncation always SAYS so: a model handed a silent fragment reasons
		// about the fragment as though it were the whole answer.
		body = body[:findMaxBytes] + fmt.Sprintf(
			"\n\n[TRUNCATED: %d bytes matched, %d kept. This result is INCOMPLETE — "+
				"narrow the search with path or glob rather than concluding from it.]",
			rawBytes, findMaxBytes)
		truncated = true
	}
	if strings.TrimSpace(body) == "" {
		body = "No matches found."
	}

	return WithResponseMetadata(
		NewTextResponse(body),
		FindResponseMetadata{
			Engine:    "pfind",
			Truncated: truncated,
			Bytes:     rawBytes,
			DurationS: elapsed.Round(time.Millisecond).String(),
		},
	), nil
}
