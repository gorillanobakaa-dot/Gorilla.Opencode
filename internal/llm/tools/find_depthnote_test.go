package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Job 07 (GPT-6-Luna, 2026-10-27): "What is in the folder Folder?" was answered
// without sub2, whose only file sits at sub2\deep\d.txt: the depth-2 plain
// listing lists files only and said nothing about what it cut.
func TestAPlainListingNamesTheFoldersItDidNotEnter(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a.txt", "sub1/c.txt", "sub2/deep/d.txt"} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("x\n"), 0o644))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755))

	resp := runFind(t, FindParams{Path: dir})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "a.txt")
	sep := string(filepath.Separator)
	assert.Contains(t, resp.Content, "folders here: sub1"+sep+", sub2"+sep)
	assert.Contains(t, resp.Content, "files deeper than that are not listed")
	assert.NotContains(t, resp.Content, ".git", "a dot-folder is not named, as pfind does not list it")
}

// A folder that holds only folders is not "No matches found".
func TestAListingOfOnlyFoldersSaysSo(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "only", "deep", "x.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("x\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "empty"), 0o755))

	resp := runFind(t, FindParams{Path: dir})

	assert.False(t, resp.IsError, "content: %s", resp.Content)
	assert.Contains(t, resp.Content, "only"+string(filepath.Separator))
	assert.False(t, strings.HasPrefix(resp.Content, "No matches found"), "content: %s", resp.Content)
}

// A search with a query is not a listing: no folder note.
func TestASearchCarriesNoFolderNote(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package marker\n"), 0o644))
	resp := runFind(t, FindParams{Query: "marker", Path: dir})
	assert.NotContains(t, resp.Content, "folders here")
}
