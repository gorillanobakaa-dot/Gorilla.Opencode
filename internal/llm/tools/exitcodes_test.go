package tools

// Exit codes that are information (robocopy 1-7, fc 1, diff/cmp 1). Test
// names end in a lowercase token because `go test -run` compiles each
// top-level alternative of the goal's pattern on its own.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRobocopyExitCodes_robocopy(t *testing.T) {
	for code := 0; code <= 7; code++ {
		m, ok := robocopyExit(code)
		assert.True(t, ok, "robocopy %d is success", code)
		assert.True(t, code == 0 || strings.HasPrefix(m, "robocopy: success"), "code %d: %q", code, m)
	}
	for _, code := range []int{8, 9, 15, 16, 24, 31} {
		m, ok := robocopyExit(code)
		assert.False(t, ok, "robocopy %d is failure", code)
		assert.Contains(t, m, "FAILED", "code %d", code)
	}
	for _, code := range []int{-1, 32, 255} {
		m, ok := robocopyExit(code)
		assert.False(t, ok, "robocopy %d is not a robocopy result", code)
		assert.Empty(t, m, "robocopy never returns %d; it must not be decoded", code)
	}
	m, _ := robocopyExit(3)
	assert.Contains(t, m, "files were copied")
	assert.Contains(t, m, "extra files")
	m, _ = robocopyExit(4)
	assert.Contains(t, m, "mismatched")
	m, _ = robocopyExit(8)
	assert.Contains(t, m, "could NOT be copied")
	m, _ = robocopyExit(16)
	assert.Contains(t, m, "serious error")
}

func TestExitCodeMeaningPrograms_exitcode(t *testing.T) {
	cases := []struct {
		cmd     string
		code    int
		success bool
		says    string
	}{
		{`robocopy "C:\SourceDir" "D:\DestDir" /MIR /MT:16 /Z`, 1, true, "files were copied"},
		{`ROBOCOPY.EXE C:\a D:\b /E`, 3, true, "extra files"},
		{`C:\Windows\System32\Robocopy.exe C:\a D:\b`, 2, true, "extra files"},
		{`robocopy C:\a D:\b /E | Out-Null`, 1, true, "files were copied"},
		{`robocopy C:\a D:\b /E`, 8, false, "could NOT be copied"},
		{`robocopy C:\a D:\b /E`, 16, false, "serious error"},
		{`fc.exe a.txt b.txt`, 1, true, "differ"},
		{`fc a.txt missing.txt`, 2, false, "could not be opened"},
		{`diff -u a b`, 1, true, "differ"},
		{`cmp a b`, 2, false, ""},
		// Not one of the programs: non-zero stays failure, and no meaning.
		{`go build ./...`, 1, false, ""},
		{`robocopy C:\a D:\b; python x.py`, 1, false, ""},
		{`git diff --exit-code`, 1, false, ""},
		{`robocopy C:\a D:\b`, 0, false, ""},
	}
	for _, c := range cases {
		m, ok := ExitCodeMeaning(c.cmd, c.code)
		assert.Equal(t, c.success, ok, "%q exit %d", c.cmd, c.code)
		if c.says == "" {
			if !ok {
				continue
			}
		}
		assert.Contains(t, m, c.says, "%q exit %d", c.cmd, c.code)
	}
}

// TestRobocopyLiveExitCodes_robocopy runs the real robocopy inside folders
// this test made, and checks the table against what robocopy returns.
func TestRobocopyLiveExitCodes_robocopy(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("robocopy is Windows")
	}
	if _, err := exec.LookPath("robocopy"); err != nil {
		t.Skip("robocopy not found")
	}
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644))
	run := func(args ...string) int {
		cmd := exec.Command("robocopy", append([]string{src, dst}, append(args, "/NJH", "/NJS", "/NP", "/R:0", "/W:0")...)...)
		_ = cmd.Run()
		return cmd.ProcessState.ExitCode()
	}
	expect := func(code int, want int, wantOK bool, says string) {
		t.Helper()
		assert.Equal(t, want, code, "robocopy exit code")
		m, ok := ExitCodeMeaning("robocopy "+src+" "+dst+" /E", code)
		assert.Equal(t, wantOK, ok, "exit %d", code)
		assert.Contains(t, m, says, "exit %d", code)
	}
	expect(run("/E"), 1, true, "files were copied")
	if code := run("/E"); code != 0 {
		t.Errorf("a second copy of the same files returned %d, want 0", code)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dst, "extra.txt"), []byte("x"), 0o644))
	expect(run("/E"), 2, true, "extra files")
	// /MIR deletes extra.txt: only inside the folder this test made.
	code := run("/MIR")
	_, ok := ExitCodeMeaning("robocopy "+src+" "+dst+" /MIR", code)
	assert.True(t, ok, "/MIR exit %d", code)
	_, err := os.Stat(filepath.Join(dst, "extra.txt"))
	assert.True(t, os.IsNotExist(err), "/MIR left the extra file")
}

// TestRobocopyBashToolSaysSuccess_robocopy: the shell tool's own result
// names the meaning under the exit code line.
func TestRobocopyBashToolSaysSuccess_robocopy(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("robocopy is Windows")
	}
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644))
	// Enter a folder that exists first: the shell is shared, and an earlier
	// test can leave it in a TempDir that is gone, where PowerShell 5.1
	// starts no native program at all and reports nothing.
	r, _ := bashRun(t, `Set-Location -LiteralPath '`+root+`'; robocopy "`+src+`" "`+dst+`" /E /NJH /NJS /NP`)
	assert.Contains(t, r.Content, "Exit code 1")
	assert.Contains(t, r.Content, "robocopy: success - files were copied")
}
