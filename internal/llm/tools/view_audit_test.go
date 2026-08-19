package tools

// GORILLA OVERRIDE (2026-08-19): the search-strategy audit, extended to view.
//
// Same shape hunted as in find: every way this tool can return something that
// LOOKS like a fact about the file when it is really a fact about the call.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func viewOf(t *testing.T, input string) ToolResponse {
	t.Helper()
	r, err := NewViewTool(nil).Run(context.Background(), ToolCall{Input: input})
	require.NoError(t, err)
	return r
}

// MEASURED before the fix: viewing a 7-byte binary returned
//
//	<file>     1|\x00\x01\x02\x03\x00\xff\x00</file>
//
// Raw bytes, rendered as if they were source. The size limit is 5 MB, so a
// real binary could put five megabytes of garbage into the context — where it
// is then RE-SENT ON EVERY LATER TURN. The description already claimed the
// tool "cannot display binary files". It displayed them, badly.
func TestABinaryFileIsRefusedRatherThanDumped(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string][]byte{
		"thing.bin": {0, 1, 2, 3, 0, 255, 0},
		"prog":      []byte("\x7fELF\x02\x01\x01\x00 and then some"),
		"a.deb":     []byte("!<arch>\ndebian-binary   "),
		"doc.pdf":   []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3"),
		"db.sqlite": []byte("SQLite format 3\x00 rest"),
	} {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, body, 0o644))
		r := viewOf(t, `{"file_path":"`+p+`"}`)
		assert.True(t, r.IsError, "%s was not refused: %q", name, r.Content)
		assert.NotContains(t, r.Content, "\x00", "%s: raw bytes reached the conversation", name)
		assert.Contains(t, r.Content, "binary", "%s: the refusal does not say why", name)
		assert.Contains(t, r.Content, "strings", "%s: no route offered for reading it anyway", name)
	}
}

// A false positive here refuses a readable file, which is worse than the bytes
// it prevents. The test is narrow on purpose.
func TestOrdinaryTextFilesAreNotMistakenForBinary(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"a.go":      "package main\n\nfunc main() {}\n",
		"utf8.txt":  "Grüße — 日本語 — العربية — 🦍\n",
		"long.md":   strings.Repeat("# heading\n\ntext here\n\n", 200),
		"tabs.tsv":  "a\tb\tc\n1\t2\t3\n",
		"crlf.txt":  "line one\r\nline two\r\n",
		"noeol.txt": "no trailing newline",
	} {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
		r := viewOf(t, `{"file_path":"`+p+`"}`)
		assert.False(t, r.IsError, "%s was wrongly refused: %s", name, r.Content)
	}
}

// MEASURED before the fix: an empty file returned "<file>\n\n</file>" — a shape
// that looks like a failed read. A model reports it as "the file appears to be
// empty" whether it was empty or the read went wrong, and those are different.
func TestAnEmptyFileSaysItIsEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(p, nil, 0o644))

	r := viewOf(t, `{"file_path":"`+p+`"}`)
	assert.False(t, r.IsError, "an empty file is not an error")
	assert.Contains(t, r.Content, "empty")
	assert.Contains(t, r.Content, "0 bytes")
	assert.Contains(t, r.Content, "read succeeded",
		"it must distinguish 'nothing in the file' from 'the read failed'")
}

// MEASURED before the fix: offset 9999 into a 100-line file returned
// "<file>\n\n</file>" — indistinguishable from an empty file.
func TestAnOffsetPastTheEndSaysHowLongTheFileIs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "l.txt")
	require.NoError(t, os.WriteFile(p, []byte(strings.Repeat("line\n", 100)), 0o644))

	r := viewOf(t, `{"file_path":"`+p+`","offset":9999}`)
	assert.True(t, r.IsError)
	assert.Contains(t, r.Content, "100 line", "it must report the real length")
	assert.Contains(t, r.Content, "past the end")
	assert.Contains(t, r.Content, "NOT an empty file",
		"the whole point is telling this apart from an empty file")
}

// The cases that were already sound, pinned so they stay that way.
func TestViewStillReportsTheObviousFailuresClearly(t *testing.T) {
	dir := t.TempDir()
	r := viewOf(t, `{"file_path":"`+dir+`"}`)
	assert.True(t, r.IsError)
	assert.Contains(t, r.Content, "directory")

	r = viewOf(t, `{"file_path":"`+filepath.Join(dir, "nope.txt")+`"}`)
	assert.True(t, r.IsError)
}
