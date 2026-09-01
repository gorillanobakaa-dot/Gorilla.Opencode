package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindTool_Info(t *testing.T) {
	info := NewFindTool().Info()

	assert.Equal(t, FindToolName, info.Name)
	for _, p := range []string{"query", "path", "glob", "type", "regex", "fuzzy", "recent", "modified_only", "view", "files_only"} {
		assert.Contains(t, info.Parameters, p, "schema must expose %q", p)
	}
	// Nothing is required: a bare path is a directory listing, a bare glob is a
	// filename search. Requiring a query would make those two impossible.
	assert.Empty(t, info.Required)
}

// TestFindDescriptionTeachesNarrowing guards the reason this description is not
// minimal. Smaller models (NVIDIA NIM among them) could not search a kernel or
// browser checkout: they issued unnarrowed queries and got truncated noise. The
// fix was to teach the narrowing arguments in the description itself. If someone
// later trims it for tokens, this fails and says why.
func TestFindDescriptionTeachesNarrowing(t *testing.T) {
	d := NewFindTool().Info().Description

	for _, must := range []string{"type", "path", "glob", "files_only", "regex", "fuzzy", "recent", "modified_only", "tree", "code"} {
		assert.Contains(t, d, must, "description must name the %q argument", must)
	}
	// The literal-by-default rule: models otherwise escape strings that need no
	// escaping, and then match nothing.
	assert.Contains(t, strings.ToLower(d), "literal",
		"description must say query is literal unless regex=true")
	// Truncation honesty: a capped result must not be read as a whole answer.
	assert.Contains(t, strings.ToUpper(d), "TRUNCATED")
	// Concrete language values, because "type" alone does not tell a weak model
	// that "c" or "rust" are the accepted tokens.
	assert.Contains(t, d, "rust")
}

// retiredTrioDescriptionAndSchemaChars is what ls+glob+grep cost per turn,
// measured 2026-08-17 immediately before they were retired (description text
// plus JSON-marshalled parameter schemas; the sources now live in the
// *.go.retired files beside this one). It is a constant, not a live
// measurement, because the point of retiring them is that they no longer
// compile.
const retiredTrioDescriptionAndSchemaChars = 5554

// TestFindDescriptionCostsLessThanTheThreeItReplaced is the whole economic
// premise. ls+glob+grep were ~1,388 tokens of description and schema on EVERY
// turn. find is allowed to be generous — it teaches its own use, which is what
// lets small models drive it — but it must stay well under what it replaced or
// the change was pointless.
func TestFindDescriptionCostsLessThanTheThreeItReplaced(t *testing.T) {
	info := NewFindTool().Info()
	schema, err := json.Marshal(info.Parameters)
	require.NoError(t, err)
	find := len(info.Name) + len(info.Description) + len(schema)

	// 2026-08-17, owner's decision: expose pfind's full arsenal (fuzzy,
	// recency, git-dirty, tree/long/code views) rather than a minimal schema —
	// a failed search that must be relaunched costs a ~10k-token round trip,
	// far more than the richer description. The guard that remains: find must
	// still cost LESS than the three tools it replaced.
	assert.Less(t, find, retiredTrioDescriptionAndSchemaChars,
		"find (%d chars) must still cost less than the %d chars of ls+glob+grep",
		find, retiredTrioDescriptionAndSchemaChars)
}

// TestFindDescriptionNamesTheToolsItReplaces: models are TRAINED on harnesses
// that have grep/glob/ls, and a small model looks for those names first. The
// description must say, in its first line, that this tool IS those tools.
func TestFindDescriptionNamesTheToolsItReplaces(t *testing.T) {
	d := NewFindTool().Info().Description
	firstPara := strings.SplitN(d, "\n", 2)[0]
	for _, name := range []string{"grep", "glob", "ls", "ripgrep", "list_dir"} {
		assert.Contains(t, firstPara, name,
			"the FIRST paragraph must name %q as replaced — small models will not read further to find out", name)
	}
	assert.Contains(t, firstPara, "REPLACES")
}

func runFind(t *testing.T, params FindParams) ToolResponse {
	t.Helper()
	if _, _, err := findPfindPath(); err != nil {
		t.Skipf("pfind not available: %v", err)
	}
	body, err := json.Marshal(params)
	require.NoError(t, err)
	resp, err := NewFindTool().Run(context.Background(), ToolCall{Input: string(body)})
	require.NoError(t, err)
	return resp
}

// TestFindReturnsMatchingLinesNotJustPaths is the behaviour the replacement
// exists for. The old grep tool returned paths only, so the agent always needed
// a second turn and a whole-file view to see any code. If this ever regresses to
// bare paths, the round-trip saving is gone and this test fails.
func TestFindReturnsMatchingLinesNotJustPaths(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "sample.go")
	require.NoError(t, os.WriteFile(src, []byte(
		"package main\n\nfunc UniqueMarkerSymbol() int {\n\treturn 42\n}\n"), 0o644))

	resp := runFind(t, FindParams{Query: "UniqueMarkerSymbol", Path: dir})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "sample.go", "must report the file")
	assert.Contains(t, resp.Content, "func UniqueMarkerSymbol", "must return the matching LINE, not just the path")
	assert.Contains(t, resp.Content, "return 42", "must return surrounding context")
}

// TestFindFilesOnlyOmitsLines is the inverse: when the agent only wants
// locations it must not be charged for the code.
func TestFindFilesOnlyOmitsLines(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sample.go"), []byte(
		"package main\n\nfunc UniqueMarkerSymbol() int {\n\treturn 42\n}\n"), 0o644))

	resp := runFind(t, FindParams{Query: "UniqueMarkerSymbol", Path: dir, FilesOnly: true})

	assert.Contains(t, resp.Content, "sample.go")
	assert.NotContains(t, resp.Content, "return 42", "files_only must not carry file contents")
}

func TestFindLiteralQueryNeedsNoEscaping(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.c"), []byte(
		"int v = compute(a.b);\n"), 0o644))

	// '.' and '(' are regex metacharacters. Literal mode must match them as-is,
	// which is what the description promises.
	resp := runFind(t, FindParams{Query: "compute(a.b)", Path: dir})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "x.c")
}

func TestFindTypeFilterNarrows(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.c"), []byte("int marker;\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skip.md"), []byte("marker\n"), 0o644))

	resp := runFind(t, FindParams{Query: "marker", Path: dir, Type: "c"})

	assert.Contains(t, resp.Content, "keep.c")
	assert.NotContains(t, resp.Content, "skip.md", "type=c must exclude markdown")
}

func TestFindListsDirectoryWithoutQuery(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alpha.go"), []byte("package a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "beta.go"), []byte("package b\n"), 0o644))

	resp := runFind(t, FindParams{Path: dir})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "alpha.go")
	assert.Contains(t, resp.Content, "beta.go")
}

func TestFindNoMatchesIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644))

	resp := runFind(t, FindParams{Query: "zzz_definitely_absent_zzz", Path: dir})

	assert.False(t, resp.IsError, "no matches is a valid answer, not a failure")
	assert.Contains(t, resp.Content, "No matches")
}

// TestFindCrashIsNotReportedAsNoMatches is the finding the adversarial review
// confirmed: pfind crashes used to exit 1 (Python's default), the same code as
// "no matches", so a broken regex told the agent the code did not exist. The
// engine now exits 2 on crashes and the tool must surface FAILURE, not absence.
func TestFindCrashIsNotReportedAsNoMatches(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644))

	// An unbalanced group is a guaranteed regex compile error in the engine.
	resp := runFind(t, FindParams{Query: "bad(regex", Path: dir, Regex: true})

	assert.True(t, resp.IsError, "a crashed search must be an error, got: %s", resp.Content)
	assert.NotContains(t, resp.Content, "No matches found")
	assert.Contains(t, resp.Content, "FAILED")
}

// TestFindSlashGlobMatchesLikeTheExamples: the description advertises
// glob="src/**/*.ts". Search paths are absolute, and path-relative globs can
// never match absolute candidates — normaliseGlob anchors them with "**/" so
// the documented examples actually work.
func TestFindSlashGlobMatchesLikeTheExamples(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src", "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "sub", "a.ts"), []byte("marker\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "top.ts"), []byte("marker\n"), 0o644))

	resp := runFind(t, FindParams{Glob: "src/**/*.ts", Path: dir})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "a.ts", "the description's own example pattern must match")
	assert.NotContains(t, resp.Content, "top.ts", "files outside the globbed subtree must not match")
}

func TestNormaliseGlob(t *testing.T) {
	for in, want := range map[string]string{
		"*.go":          "*.go", // no slash: basename match, leave alone
		"src/**/*.ts":   "**/src/**/*.ts",
		"!src/*.min.js": "!**/src/*.min.js",
		"**/x/*.c":      "**/x/*.c", // already anchored
		"/abs/*.c":      "/abs/*.c", // explicitly absolute
	} {
		assert.Equal(t, want, normaliseGlob(in), "normaliseGlob(%q)", in)
	}
}

// TestFindSaysWhenTheFileCapCut: more matching files than findMaxFiles must be
// announced in the body, not silently dropped — the file-level twin of the
// per-file "… N more match(es)" marker.
func TestFindSaysWhenTheFileCapCut(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < findMaxFiles+10; i++ {
		name := filepath.Join(dir, fmt.Sprintf("f%02d.txt", i))
		require.NoError(t, os.WriteFile(name, []byte("capmarker\n"), 0o644))
	}

	resp := runFind(t, FindParams{Query: "capmarker", Path: dir, FilesOnly: true})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "only", "the body must say the file list was cut")
	assert.Contains(t, resp.Content, "shown")
}

// TestFindLongLinesAreTruncatedPerLine reproduces the crocodile-over-$HOME
// call: a single wordlist line thousands of characters long must not eat the
// byte budget that could hold other files' matches. The line is kept as a
// bounded preview with an explicit omission marker.
func TestFindLongLinesAreTruncatedPerLine(t *testing.T) {
	dir := t.TempDir()
	long := "linemarker " + strings.Repeat("\"word\",", 3000) // ~21,000 chars on one line
	require.NoError(t, os.WriteFile(filepath.Join(dir, "wordlist.txt"), []byte(long+"\n"), 0o644))

	resp := runFind(t, FindParams{Query: "linemarker", Path: dir})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Less(t, len(resp.Content), 2*findMaxLineChars,
		"one long line must come back bounded, not verbatim")
	assert.Contains(t, resp.Content, "omitted", "the cut must be announced on the line itself")
}

func TestFindRejectsMissingPath(t *testing.T) {
	resp := runFind(t, FindParams{Query: "x", Path: "/nonexistent/path/for/test"})
	assert.True(t, resp.IsError)
	assert.Contains(t, resp.Content, "does not exist")
}

// TestFindResultIsBoundedInBytes encodes the lesson from the grep incident:
// the cap must be in the unit of the resource at risk. A file with thousands of
// matches must not be able to put a megabyte into the message history, where it
// is re-sent on every later turn.
func TestFindResultIsBoundedInBytes(t *testing.T) {
	// The original incident's shape (single enormous lines) is now defused
	// upstream by --max-columns, so this fixture builds its volume the way a
	// real code tree does: many files, many matches, every line under the
	// per-line cap. The byte cap is the last line of defence and must still
	// hold when each individual line is legal.
	dir := t.TempDir()
	match := "marker_token " + strings.Repeat("y", 400) + "\n"
	filler := strings.Repeat("z", 400) + "\n"
	var sb strings.Builder
	for j := 0; j < 6; j++ { // matches spaced out: each keeps a full -C 2 window
		sb.WriteString(match)
		for k := 0; k < 6; k++ {
			sb.WriteString(filler)
		}
	}
	content := sb.String() // ≈ 3 surviving blocks × 5 long lines per file
	for i := 0; i < 30; i++ {
		name := filepath.Join(dir, fmt.Sprintf("bulk%02d.txt", i))
		require.NoError(t, os.WriteFile(name, []byte(content), 0o644))
	}

	resp := runFind(t, FindParams{Query: "marker_token", Path: dir})

	assert.LessOrEqual(t, len(resp.Content), findMaxBytes+512,
		"result must be bounded in BYTES, not in match count")
	assert.Contains(t, resp.Content, "TRUNCATED",
		"a cut-off result must say so; a silent fragment reads as the whole answer")
}

// TestFindRefusesWhenEngineUnavailable proves the tool does not silently
// degrade. An agent told "no matches" cannot distinguish that from "the search
// never ran", and will conclude the code does not exist.
func TestFindRefusesWhenEngineUnavailable(t *testing.T) {
	t.Run("broken GORILLA_PFIND override is an error, not a fall-through", func(t *testing.T) {
		t.Setenv("GORILLA_PFIND", filepath.Join(t.TempDir(), "absent.py"))
		_, _, err := findPfindPath()
		require.Error(t, err, "a typo'd override must not silently run a different engine")
		assert.Contains(t, err.Error(), "GORILLA_PFIND")
	})
	t.Run("no python3 anywhere refuses and says what to install", func(t *testing.T) {
		t.Setenv("GORILLA_PFIND", "")
		t.Setenv("PATH", t.TempDir()) // neither pfind nor python3 findable
		_, _, err := findPfindPath()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "python3")
	})
}

// TestFindEmbeddedEngineWorksOnACleanMachine is the portability claim itself:
// no pfind on PATH, no override, an empty cache — the tool must extract its
// embedded engine and complete a real search using nothing but python3.
func TestFindEmbeddedEngineWorksOnACleanMachine(t *testing.T) {
	if _, _, err := findPythonExe(); err != nil {
		t.Skip("no working python 3 interpreter installed")
	}
	t.Setenv("GORILLA_PFIND", "")
	// GORILLA OVERRIDE (2026-09-01): "a clean machine" cannot be spelled
	// "/usr/bin:/bin". That PATH has no meaning on Windows, so this test — and
	// the portability claim it exists to prove — could only ever fail there,
	// which is precisely the platform whose portability was in doubt.
	// systemOnlyPATH() names the same idea in the local dialect.
	t.Setenv("PATH", systemOnlyPATH(t))
	if _, err := exec.LookPath("pfind"); err == nil {
		t.Skip("a system pfind exists on the system PATH; embedded path not reachable")
	}
	cache := t.TempDir()
	setUserCacheDir(t, cache)

	bin, prefix, err := findPfindPath()
	require.NoError(t, err)
	assert.True(t, looksLikePython(bin), "engine must be run by a python interpreter, got %q", bin)
	require.NotEmpty(t, prefix)
	assert.Contains(t, prefix[len(prefix)-1], cache, "engine must be extracted into the cache, not run from a repo path")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a // clean_machine_marker\n"), 0o644))
	body, _ := json.Marshal(FindParams{Query: "clean_machine_marker", Path: dir})
	resp, err := NewFindTool().Run(context.Background(), ToolCall{Input: string(body)})
	require.NoError(t, err)
	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "a.go")
}

// TestFindQueryStartingWithDashIsNotAFlag: a model searching for "-v" or
// "--help" must get a literal search, not have its query eaten by the engine's
// own option parser.
func TestFindQueryStartingWithDashIsNotAFlag(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "flags.txt"), []byte(
		"use -v for verbose\nplain line\n"), 0o644))

	resp := runFind(t, FindParams{Query: "-v", Path: dir})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "flags.txt")
	assert.Contains(t, resp.Content, "use -v for verbose")
}

// TestVendoredPfindMatchesDevCopy guards the two-copy problem: the vendored
// engine in this repo and the development copy in Scripts.For.Work must stay
// byte-identical, or fixes land in one and silently miss the other. The test
// only runs on a machine that HAS the dev copy; everywhere else it skips.
func TestVendoredPfindMatchesDevCopy(t *testing.T) {
	dev := filepath.Join(os.Getenv("HOME"), "Documents", "Scripts.For.Work", "pfind", "pfind.py")
	devBytes, err := os.ReadFile(dev)
	if err != nil {
		t.Skip("no development copy on this machine; nothing to drift against")
	}
	assert.Equal(t, sha256.Sum256(devBytes), sha256.Sum256(embeddedPfind),
		"vendored pfind.py has drifted from %s — re-sync whichever direction is current", dev)
}

func TestFindHonoursGorillaPfindEnv(t *testing.T) {
	if _, _, err := findPythonExe(); err != nil {
		t.Skip("no working python 3 interpreter installed")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "pfind.py")
	require.NoError(t, os.WriteFile(fake, []byte("print('ok')\n"), 0o644))

	// no 'pfind' executable in dir, but the python3 lookup needs a real PATH
	t.Setenv("PATH", dir+string(os.PathListSeparator)+systemOnlyPATH(t))
	t.Setenv("GORILLA_PFIND", fake)

	bin, prefix, err := findPfindPath()
	require.NoError(t, err)
	assert.True(t, looksLikePython(bin), "engine must be run by a python interpreter, got %q", bin)
	require.NotEmpty(t, prefix)
	assert.Equal(t, fake, prefix[len(prefix)-1])
}

// ── The full-arsenal surface (owner's decision, 2026-08-17) ──────────────────

func TestFindFuzzyFindsMisspelledName(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "build_parser.py"), []byte("x = 1\n"), 0o644))

	resp := runFind(t, FindParams{Query: "buldparser", Path: dir, Fuzzy: true})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "build_parser.py", "fuzzy must survive the missing letter")
}

func TestFindTreeViewShowsStructure(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src", "audio"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "audio", "mix.c"), []byte("int x;\n"), 0o644))

	resp := runFind(t, FindParams{Path: dir, View: "tree"})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "── ", "tree view must render tree connectors")
	assert.Contains(t, resp.Content, "mix.c")
}

func TestFindLongViewShowsMetadata(t *testing.T) {
	dir := t.TempDir()
	// not .bin — that extension is in pfind's default exclude list
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.txt"), []byte(strings.Repeat("x", 2048)), 0o644))

	resp := runFind(t, FindParams{Path: dir, View: "long"})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "rw-", "long view must show permissions")
	assert.Contains(t, resp.Content, "KiB", "long view must show sizes")
}

func TestFindCodeViewSummarisesLanguages(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.py"), []byte("# c\nx = 1\n\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.rs"), []byte("fn main() {}\n"), 0o644))

	resp := runFind(t, FindParams{Path: dir, View: "code"})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "Python")
	assert.Contains(t, resp.Content, "Rust")
	assert.Contains(t, resp.Content, "Language")
}

func TestFindInvalidViewTeaches(t *testing.T) {
	resp := runFind(t, FindParams{Path: ".", View: "fancy"})
	assert.True(t, resp.IsError)
	assert.Contains(t, resp.Content, "tree", "the error must list the valid views")
}

func TestFindRecentListingSortsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "older.txt")
	newer := filepath.Join(dir, "newer.txt")
	require.NoError(t, os.WriteFile(older, []byte("o\n"), 0o644))
	require.NoError(t, os.WriteFile(newer, []byte("n\n"), 0o644))
	past := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(older, past, past))

	resp := runFind(t, FindParams{Path: dir, Recent: true})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	iNew := strings.Index(resp.Content, "newer.txt")
	iOld := strings.Index(resp.Content, "older.txt")
	require.True(t, iNew >= 0 && iOld >= 0, "both files must be listed: %s", resp.Content)
	assert.Less(t, iNew, iOld, "recent=true must put the newer file first")
}

func TestFindModifiedOnlyFiltersToDirtyFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "clean.txt"), []byte("c\n"), 0o644))
	run("add", "clean.txt")
	run("commit", "-q", "-m", "clean")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("d\n"), 0o644))

	resp := runFind(t, FindParams{Path: dir, ModifiedOnly: true})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "dirty.txt", "the uncommitted file must be listed")
	assert.NotContains(t, resp.Content, "clean.txt", "committed-and-unchanged files must be filtered out")
}

// GORILLA OVERRIDE (2026-08-19), from a real run.
//
// Asked to look in "my screenshots folder", the agent searched the whole home
// directory, hit this timeout, was told only "narrow it with path or glob",
// and fell back to guessing a path through bash. Two minutes and three tool
// calls for a directory the operating system can name instantly.
//
// "Narrow it" is true and useless. The caller has just told us the one thing
// that would let us help — where it was looking. An error that says what to do
// next is worth more than one that says what went wrong.
func TestTheHomeDirectoryTimeoutSaysWhatToDoNext(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	if !withinHome(home, home) {
		t.Fatal("a search rooted at the home directory was not recognised as one")
	}
	if withinHome(filepath.Join(home, "Documents"), home) {
		t.Error("a search inside a subdirectory was treated as a whole-home search")
	}
	if withinHome("", home) {
		t.Error("an empty path was treated as a home search")
	}
	if withinHome("/etc", home) {
		t.Error("/etc was treated as a home search")
	}
}

// GORILLA OVERRIDE (2026-08-19): fail fast on the search that cannot work.
//
// From a real run: asked "what is in my screenshots folder", a model ran a
// CONTENT search rooted at the home directory — reading inside every file on
// the machine to find a folder name. Thirty seconds, no answer, then a
// fallback to guessing paths through bash.
//
// The owner's objection to the alternative is the point: hand-feeding a model
// where Pictures lives does not scale, because there is always another
// convention. Making the wrong call FAIL FAST AND SAY WHY costs zero tokens,
// works for every model including ones not yet released, and never has to be
// repeated.
func TestAContentSearchOverTheWholeHomeDirectoryIsRefusedImmediately(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	why := doomedContentSearch(FindParams{Query: "screenshot"}, home)
	if why == "" {
		t.Fatal("a content search over the whole home directory was allowed; it will time out")
	}
	for _, want := range []string{"glob", "INSIDE every file", "environment block"} {
		if !strings.Contains(why, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, why)
		}
	}
}

// A NAME search over the same tree is cheap and must stay allowed — that is
// how you actually find a folder.
func TestANameSearchOverTheHomeDirectoryIsStillAllowed(t *testing.T) {
	home, _ := os.UserHomeDir()
	if why := doomedContentSearch(FindParams{Glob: "*screenshot*"}, home); why != "" {
		t.Errorf("a name search was refused: %s", why)
	}
}

// It must refuse only the shape nobody wants. A guess that blocks a legitimate
// search is worse than a timeout.
func TestContentSearchesInsideAProjectAreUntouched(t *testing.T) {
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, "Documents", "Gorilla.Opencode"),
		filepath.Join(home, "Documents"),
		"/tmp/whatever",
	} {
		if why := doomedContentSearch(FindParams{Query: "func main"}, p); why != "" {
			t.Errorf("refused a legitimate content search in %s: %s", p, why)
		}
	}
}

// ── search-strategy audit, 2026-08-19 ────────────────────────────────────
//
// Prompted by the owner: pfind replaced grep, glob and ls, so it is now on
// nearly every turn, and its parameter surface is large enough that today's
// bug is unlikely to be the only one of its kind. The audit looked for one
// shape specifically — A FILTER THAT MISFIRES AND LOOKS LIKE AN EMPTY RESULT —
// because that is what makes a model report a false fact about the codebase
// rather than retry.

// MEASURED before the fix: glob=".github/**" returned "No matches found" while
// .github/workflows/ held build.yml, ci.yml and release.yml. A model asked
// "does this project have CI?" is told no, and says no.
//
// Skipping hidden files is right as a DEFAULT and indefensible when the
// request names the hidden thing. Typing ".github/**" is not ambiguous.
func TestExplicitlyAskingForAHiddenPathFindsIt(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, ".github", "workflows")
	require.NoError(t, os.MkdirAll(hidden, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hidden, "ci.yml"),
		[]byte("jobs:\n  build:\n    runs-on: ubuntu-latest\n"), 0o644))

	resp := runFind(t, FindParams{Path: dir, Glob: ".github/**"})
	assert.False(t, resp.IsError, resp.Content)
	assert.Contains(t, resp.Content, "ci.yml",
		"a glob that names a dot-directory must find it; otherwise the answer is "+
			"indistinguishable from the project having no CI")

	// And a content search rooted inside it.
	resp = runFind(t, FindParams{Path: hidden, Query: "runs-on"})
	assert.Contains(t, resp.Content, "runs-on")
}

// An ordinary search must NOT start dredging up .git objects and caches: the
// default exists for a reason and the fix is intent-driven, not a blanket flag.
func TestAnOrdinarySearchStillSkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".secretcache"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".secretcache", "junk.txt"),
		[]byte("UNIQUEHIDDENMARKER\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "real.txt"),
		[]byte("UNIQUEHIDDENMARKER\n"), 0o644))

	resp := runFind(t, FindParams{Path: dir, Query: "UNIQUEHIDDENMARKER"})
	assert.Contains(t, resp.Content, "real.txt")
	assert.NotContains(t, resp.Content, "junk.txt",
		"an ordinary search pulled in a hidden directory; the default is there so "+
			"caches and .git do not flood every result")
}

// MEASURED before the fix: `pfind -t notalanguage` exits 0 with no output. So
// type="pyton" produced "No matches found", which a model reads and reports as
// "this project has no Python in it".
//
// A filter that misfires must never be indistinguishable from an empty result.
func TestAnUnknownTypeIsRefusedRatherThanReturningNothing(t *testing.T) {
	for _, bad := range []string{"pyton", "golang", "typescrip", "notalanguage"} {
		why := checkType(bad)
		require.NotEmpty(t, why, "type %q was accepted", bad)
		assert.Contains(t, why, "NOT run",
			"the message must say the search did not happen, not merely that the type is odd")
		assert.Contains(t, why, "Valid types:")
	}
	// The realistic typos should get a suggestion.
	assert.Contains(t, checkType("pyton"), "Did you mean")
	assert.Contains(t, checkType("golang"), "Did you mean")
}

func TestEveryValidTypeIsAccepted(t *testing.T) {
	for ok := range knownTypes() {
		assert.Empty(t, checkType(ok), "valid type %q was rejected", ok)
		assert.Empty(t, checkType(strings.ToUpper(ok)), "type matching must be case-insensitive")
	}
	assert.Empty(t, checkType(""), "an absent type is not an error")
}

// The audit's own guard: validTypes must not drift from what the engine
// actually accepts. A list that quietly goes stale reintroduces exactly the
// bug it was written to prevent — a valid type refused, or an invalid one let
// through to return a misleading nothing.
func TestTheTypeListMatchesTheEngine(t *testing.T) {
	bin, prefix, err := findPfindPath()
	if err != nil {
		t.Skipf("pfind not available: %v", err)
	}
	out, err := exec.Command(bin, append(append([]string{}, prefix...), "--type-list")...).CombinedOutput()
	if err != nil {
		t.Skipf("could not list types: %v", err)
	}
	engine := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && !strings.HasSuffix(f[0], ":") {
			engine[f[0]] = true
		}
	}
	if len(engine) == 0 {
		t.Skip("could not parse the engine's type list")
	}
	for v := range knownTypes() {
		if !engine[v] {
			t.Errorf("we accept type %q but the engine does not know it — a search with it "+
				"returns nothing, which reads as 'no such code here'", v)
		}
	}
}

// MEASURED before the fix: modified_only outside a git repository returned
// "No matches found" — the identical answer you get inside a repository whose
// tree is clean.
//
// Those are different facts. One says "nothing has been edited"; the other
// says "this question cannot be asked here". A model told the first will
// report a clean working tree for a directory with no version control at all.
func TestModifiedOnlyOutsideGitSaysSoRatherThanNoMatches(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o644))

	resp := runFind(t, FindParams{Path: dir, ModifiedOnly: true})
	assert.True(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "not inside a git repository")
	assert.Contains(t, resp.Content, "NOT the same as",
		"the message must distinguish it from an empty result, which is the whole point")
}

func TestInsideGitRepoDetection(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".git"), 0o755))
	sub := filepath.Join(repo, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	assert.True(t, insideGitRepo(repo), "the repository root")
	assert.True(t, insideGitRepo(sub), "a nested directory")

	// A worktree or submodule has .git as a FILE, not a directory.
	wt := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /elsewhere\n"), 0o644))
	assert.True(t, insideGitRepo(wt), "a worktree, where .git is a file")

	assert.False(t, insideGitRepo(t.TempDir()), "a plain directory")
}

// GORILLA OVERRIDE (2026-09-01): cross-platform helpers for the tests above.
//
// They used to hardcode "/usr/bin:/bin" for "a machine with python and nothing
// else", and to assert the interpreter's path contained the literal "python3".
// Neither survives contact with Windows, where the interpreter can legitimately
// be py.exe invoked as `py -3`, and where those directories do not exist. The
// tests were therefore guaranteed failures on Windows rather than checks of it.

// systemOnlyPATH is a PATH holding a Python interpreter and the OS's own tool
// directories, and nothing else the user has added — the local spelling of "a
// machine with python3 and no user-installed pfind".
//
// It is derived from where the interpreter actually is rather than hardcoded,
// because there is no fixed directory that answers this on every platform:
// Python may be in /usr/bin, in a Homebrew prefix, in a Scoop shim directory, or
// beside py.exe in the Windows directory. Hardcoding "/usr/bin:/bin" made the
// portability tests unrunnable on the one platform whose portability was in
// question.
func systemOnlyPATH(t *testing.T) string {
	t.Helper()
	python, _, err := findPythonExe()
	if err != nil {
		t.Skip("no working python 3 interpreter installed")
	}
	dirs := []string{filepath.Dir(python)}
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = filepath.Join("C:", "Windows")
		}
		dirs = append(dirs, filepath.Join(root, "System32"), root)
	} else {
		dirs = append(dirs, "/usr/bin", "/bin")
	}
	return strings.Join(dirs, string(os.PathListSeparator))
}

// looksLikePython reports whether a resolved binary path is a Python
// interpreter, under any of the names one legitimately goes by.
func looksLikePython(bin string) bool {
	b := strings.ToLower(filepath.Base(bin))
	return strings.HasPrefix(b, "python") || strings.HasPrefix(b, "py.") || b == "py"
}

// setUserCacheDir points os.UserCacheDir at dir for the duration of a test.
//
// GORILLA OVERRIDE (2026-09-01): setting XDG_CACHE_HOME alone was a Unix
// assumption. On Windows os.UserCacheDir reads %LocalAppData% and ignores XDG
// entirely, so the test asserted the engine had been extracted into a temporary
// directory while it was in fact being extracted into the user's real cache —
// failing for a reason that had nothing to do with what it was testing.
func setUserCacheDir(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("LocalAppData", dir)
		return
	}
	t.Setenv("XDG_CACHE_HOME", dir)
}
