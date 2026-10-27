package dialog

// GORILLA FIX (2026-10-27): the PATH-lookup ceiling for this package.
//
// Cacheable `go test ./internal/tui/components/dialog` wrote a testlog of
// 1,289,552 entries (1,267,273 stats): PowerShell resolved again on every
// render and /arsenal walking PATH x PATHEXT once per name per open. go.exe
// hashes every testlog entry after the tests pass, so the run sat at 100-250 %
// CPU for hours. These tests hold the number of probes to a ceiling on a fake
// PATH of 20 directories, whatever this machine has installed.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
	"github.com/opencode-ai/opencode/internal/pathlook"
)

// fakePATH points PATH at 20 empty directories, the last holding a PowerShell.
func fakePATH(t *testing.T) {
	t.Helper()
	var dirs []string
	for i := 0; i < 20; i++ {
		dirs = append(dirs, t.TempDir())
	}
	if err := os.WriteFile(filepath.Join(dirs[19], "pwsh.exe"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", strings.Join(dirs, string(os.PathListSeparator)))
	t.Setenv("SHELL", "")
}

func TestSettingsAndShellQuestionStayUnderTheLookupCeiling(t *testing.T) {
	fakePATH(t)
	before := pathlook.Counted()
	for i := 0; i < 200; i++ {
		d := NewSettingsDialogCmp()
		d.Init()
		_ = d.View()
		ps := shell.IsWindowsPowerShell()
		if runtime.GOOS == "windows" && !ps {
			t.Fatal("pwsh.exe is on the fake PATH but the shell question says no PowerShell")
		}
	}
	after := pathlook.Counted()
	// Measured: 1 LookPath on Windows (pwsh.exe, once), 0 elsewhere. Before the
	// fix: 2 per IsWindowsPowerShell call = 400, each 20 dirs x 13 names stat-ed.
	if n := after.LookPaths - before.LookPaths; n >= 50 {
		t.Fatalf("200 settings renders + shell questions made %d LookPath calls, ceiling 50", n)
	}
	if n := after.DirLists - before.DirLists; n != 0 {
		t.Fatalf("settings path listed %d PATH directories, want 0", n)
	}
}

func TestArsenalOpensListPathOnceEach(t *testing.T) {
	fakePATH(t)
	const opens = 20
	before := pathlook.Counted()
	for i := 0; i < opens; i++ {
		NewArsenalCmp()
	}
	after := pathlook.Counted()
	// Every open looks at PATH again (one stat per directory, so a tool
	// installed meanwhile shows up), re-reads a directory only when it changed
	// since the last read - so each of the 20 once in all - and probes only
	// hits: none here. Before the fix one open was ~70 names x 20 dirs x
	// 13 stats.
	if n := after.DirLists - before.DirLists; n > 20 {
		t.Fatalf("%d opens read %d directories, ceiling 20 (each once)", opens, n)
	}
	if n := after.DirStats - before.DirStats; n > opens*20 {
		t.Fatalf("%d opens looked at directories %d times, ceiling %d (20 per open)", opens, n, opens*20)
	}
	if n := after.LookPaths - before.LookPaths; n != 0 {
		t.Fatalf("%d opens made %d LookPath calls on a PATH with no arsenal tool, want 0", opens, n)
	}
}
