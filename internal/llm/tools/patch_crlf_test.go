package tools

// Gap #16: the patch tool's pipeline (diff.TextToPatch then
// diff.PatchToCommit, exactly as patch.go calls them) keeps a file's line
// endings. Before the fix a CRLF file came out with the patched lines in
// bare LF and the rest in CRLF.

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/diff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func applyPatchText(t *testing.T, patchText string, files map[string]string) map[string]string {
	t.Helper()
	patch, fuzz, err := diff.TextToPatch(patchText, files)
	require.NoError(t, err)
	assert.Equal(t, 0, fuzz, "context lines must match exactly, not by fuzz")
	commit, err := diff.PatchToCommit(patch, files)
	require.NoError(t, err)
	out := map[string]string{}
	require.NoError(t, diff.ApplyCommit(commit, func(p, c string) error { out[p] = c; return nil }, func(string) error { return nil }))
	return out
}

const crlfPatch = `*** Begin Patch
*** Update File: build.ps1
@@
 $a = 1
-$b = 2
+$b = 3
+$c = 4
 $d = 5
*** End Patch`

func TestPatchKeepsCRLF_crlf(t *testing.T) {
	orig := "# build\r\n$a = 1\r\n$b = 2\r\n$d = 5\r\n"
	out := applyPatchText(t, crlfPatch, map[string]string{"build.ps1": orig})
	got := out["build.ps1"]
	assert.Equal(t, "# build\r\n$a = 1\r\n$b = 3\r\n$c = 4\r\n$d = 5\r\n", got)
	assert.Equal(t, strings.Count(got, "\n"), strings.Count(got, "\r\n"), "no bare LF in a CRLF file")
}

// A patch the model sends with CRLF itself applies too, and an LF file
// stays LF.
func TestPatchCRLFPatchTextLFFile_crlf(t *testing.T) {
	orig := "# build\n$a = 1\n$b = 2\n$d = 5\n"
	out := applyPatchText(t, strings.ReplaceAll(crlfPatch, "\n", "\r\n"), map[string]string{"build.ps1": orig})
	assert.Equal(t, "# build\n$a = 1\n$b = 3\n$c = 4\n$d = 5\n", out["build.ps1"])
	assert.ElementsMatch(t, []string{"build.ps1"}, diff.IdentifyFilesNeeded(strings.ReplaceAll(crlfPatch, "\n", "\r\n")),
		"a CRLF patch must not name the file \"build.ps1\\r\"")
}
