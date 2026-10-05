package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests come from one real conversation on 2026-10-05, in which a
// 550-billion-parameter model was asked two plain questions about a home
// folder and was handed three wrong answers by this tool.

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
}

func bothEngines(t *testing.T, check func(t *testing.T, engine string)) {
	t.Helper()
	check(t, "ripgrep if installed")
	t.Setenv("PFIND_NO_RG", "1")
	check(t, "pure Python")
}

// Asked for "**/gemini.md", the tool said "No matches found" while fifteen
// files named GEMINI.md existed.
func TestANameIsFoundWhateverItsCapitals(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"project/GEMINI.md":   "# rules\n",
		"project/src/main.go": "package main\n",
	})
	bothEngines(t, func(t *testing.T, engine string) {
		for _, glob := range []string{"**/gemini.md", "gemini.md", "GEMINI.MD", "Gemini*"} {
			resp := runFind(t, FindParams{Path: dir, Glob: glob})
			assert.Contains(t, resp.Content, "GEMINI.md", "%s: glob %q did not find GEMINI.md", engine, glob)
			assert.NotContains(t, resp.Content, "main.go", "%s: glob %q", engine, glob)
		}
	})
}

// glob="**/gemini.md" with type="md" returned every Markdown file on the
// machine: 120,765 of them.
func TestANamePatternAndATypeMeanBothNotEither(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"a/GEMINI.md":  "needle here\n",
		"a/README.md":  "needle here\n",
		"a/gemini.txt": "needle here\n",
		"b/notes.md":   "needle here\n",
	})
	bothEngines(t, func(t *testing.T, engine string) {
		list := runFind(t, FindParams{Path: dir, Glob: "**/gemini.md", Type: "md"})
		assert.Contains(t, list.Content, "GEMINI.md", engine)
		assert.NotContains(t, list.Content, "README.md", "%s: the type widened the name pattern", engine)
		assert.NotContains(t, list.Content, "notes.md", engine)

		wide := runFind(t, FindParams{Path: dir, Glob: "gemini*", Type: "md"})
		assert.Contains(t, wide.Content, "GEMINI.md", engine)
		assert.NotContains(t, wide.Content, "gemini.txt", "%s: the type did not narrow the name pattern", engine)

		content := runFind(t, FindParams{Path: dir, Query: "needle", Glob: "gemini*", Type: "md"})
		assert.Contains(t, content.Content, "GEMINI.md", engine)
		assert.NotContains(t, content.Content, "README.md", "%s: content search", engine)
		assert.NotContains(t, content.Content, "gemini.txt", "%s: content search", engine)
	})
}

// Asked for the folders of a home directory, the tree view drew the first 40
// files of the walk, showed Documents as holding one file, and said nothing.
func TestATreeTooLargeToDrawShowsFoldersAndSaysSo(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"top.txt": "x\n"}
	for i := 0; i < 30; i++ {
		files[fmt.Sprintf("alpha/f%02d.txt", i)] = "x\n"
		files[fmt.Sprintf("beta/deep/f%02d.txt", i)] = "x\n"
	}
	files["gamma/only.txt"] = "x\n"
	writeTree(t, dir, files)

	bothEngines(t, func(t *testing.T, engine string) {
		resp := runFind(t, FindParams{Path: dir, View: "tree"})
		for _, want := range []string{"alpha", "beta", "gamma", "deep"} {
			assert.Contains(t, resp.Content, want, "%s: folder %s missing from the overview", engine, want)
		}
		assert.Contains(t, resp.Content, "62 files", "%s: the total must be stated", engine)
		assert.Contains(t, resp.Content, "(30 files)", "%s: each folder carries its count", engine)
		assert.Contains(t, resp.Content, "NOT the whole tree", "%s: an overview must not pass for the tree", engine)
		assert.NotContains(t, resp.Content, "f07.txt", "%s: individual files do not belong in the overview", engine)
	})
}

// A small tree is still drawn in full.
func TestASmallTreeIsStillDrawnInFull(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"src/main.go": "package main\n", "docs/guide.md": "# g\n"})
	resp := runFind(t, FindParams{Path: dir, View: "tree"})
	assert.Contains(t, resp.Content, "main.go")
	assert.Contains(t, resp.Content, "guide.md")
	assert.NotContains(t, resp.Content, "NOT the whole tree")
}

// "raise --limit" names a switch the model cannot reach.
func TestACutListDoesNotNameASwitchTheModelCannotUse(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{}
	for i := 0; i < findMaxFiles+10; i++ {
		files[fmt.Sprintf("n%03d.txt", i)] = "x\n"
	}
	writeTree(t, dir, files)
	resp := runFind(t, FindParams{Path: dir, Glob: "*.txt"})
	assert.NotContains(t, resp.Content, "--limit")
	assert.Contains(t, resp.Content, "INCOMPLETE")
}
