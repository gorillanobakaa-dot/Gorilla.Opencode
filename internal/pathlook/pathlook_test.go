package pathlook

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// stubLookPath makes exec.LookPath answer from onPath and counts the calls.
func stubLookPath(t *testing.T, onPath map[string]string) *int {
	t.Helper()
	old := lookPath
	t.Cleanup(func() { lookPath = old })
	psMu.Lock()
	psLast = nil
	psMu.Unlock()
	n := 0
	lookPath = func(name string) (string, error) {
		n++
		if p, ok := onPath[name]; ok {
			return p, nil
		}
		return "", exec.ErrNotFound
	}
	return &n
}

// The 2026-10-27 storm: every settings load and every render asked exec.LookPath
// for pwsh.exe and powershell.exe again. Resolved once, it is one probe for the
// whole process however often it is asked.
func TestPowerShellResolvedOncePerPath(t *testing.T) {
	calls := stubLookPath(t, map[string]string{"pwsh.exe": `C:\PS7\pwsh.exe`})
	t.Setenv("PATH", `C:\PS7`)
	for i := 0; i < 200; i++ {
		name, full, ok := PowerShell()
		if !ok || name != "pwsh.exe" || full != `C:\PS7\pwsh.exe` {
			t.Fatalf("PowerShell() = %q, %q, %v", name, full, ok)
		}
	}
	if *calls != 1 {
		t.Fatalf("200 questions cost %d LookPath calls, want 1", *calls)
	}

	// A process that changes PATH gets a fresh answer (the bash-fallback test
	// in the shell package does exactly that).
	t.Setenv("PATH", `C:\elsewhere`)
	PowerShell()
	if *calls != 2 {
		t.Fatalf("after a PATH change: %d LookPath calls, want 2", *calls)
	}
}

func TestPowerShellFallsBackToWindowsPowerShell(t *testing.T) {
	calls := stubLookPath(t, map[string]string{"powershell.exe": `C:\WINDOWS\powershell.exe`})
	t.Setenv("PATH", `C:\WINDOWS`)
	for i := 0; i < 50; i++ {
		if name, _, ok := PowerShell(); !ok || name != "powershell.exe" {
			t.Fatalf("PowerShell() = %q, %v; want powershell.exe", name, ok)
		}
	}
	if *calls != 2 {
		t.Fatalf("50 questions cost %d LookPath calls, want 2 (pwsh, then powershell, once)", *calls)
	}
	stubLookPath(t, nil)
	if _, _, ok := PowerShell(); ok {
		t.Fatal("PowerShell() reported one on a machine that has none")
	}
}

// mkfile creates an executable file (on Windows the extension is what counts).
func mkfile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// A Snapshot must give exec.LookPath's answer: same directory order, same
// extension order, directories and (outside Windows) non-executables skipped.
func TestSnapshotAgreesWithExecLookPath(t *testing.T) {
	a, b, c := t.TempDir(), t.TempDir(), t.TempDir()
	names := []string{"one", "two", "three", "four", "absent", "Mixed", "dot.ted"}
	if runtime.GOOS == "windows" {
		mkfile(t, a, "one.cmd")
		mkfile(t, b, "one.exe") // later directory loses to a's .cmd
		mkfile(t, a, "two.bat")
		mkfile(t, a, "two.exe") // .exe comes before .bat in PATHEXT
		if err := os.Mkdir(filepath.Join(a, "three.exe"), 0o755); err != nil {
			t.Fatal(err)
		}
		mkfile(t, c, "three.exe") // a directory of that name is not a program
		mkfile(t, b, "FOUR.EXE")  // case does not matter
		mkfile(t, c, "mixed.exe")
		mkfile(t, b, "dot.ted.exe")
		names = append(names, "one.exe", "four.exe")
		t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	} else {
		mkfile(t, a, "one")
		mkfile(t, b, "one")
		if err := os.WriteFile(filepath.Join(a, "two"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		mkfile(t, b, "two") // a's is not executable
		if err := os.Mkdir(filepath.Join(a, "three"), 0o755); err != nil {
			t.Fatal(err)
		}
		mkfile(t, c, "three")
		mkfile(t, c, "four")
	}
	t.Setenv("PATH", strings.Join([]string{a, "", "relative-dir", b, c}, string(os.PathListSeparator)))
	t.Chdir(t.TempDir()) // nothing in the current directory to find

	s := NewSnapshot()
	for _, n := range names {
		want, wantErr := exec.LookPath(n)
		got, gotErr := s.LookPath(n)
		if (wantErr == nil) != (gotErr == nil) || !strings.EqualFold(got, want) {
			t.Errorf("%s: snapshot = %q, %v; exec.LookPath = %q, %v", n, got, gotErr, want, wantErr)
		}
		if gotErr != nil && !errors.Is(gotErr, exec.ErrNotFound) {
			t.Errorf("%s: error %v is not exec.ErrNotFound", n, gotErr)
		}
	}
}

// The ceiling: a batch of lookups lists each PATH directory once, and probes
// only names the listing shows, once each. Fake PATH of 20 directories, 200 lookups.
func TestSnapshotListsEachDirectoryOnce(t *testing.T) {
	var dirs []string
	for i := 0; i < 20; i++ {
		dirs = append(dirs, t.TempDir())
	}
	prog := "tool"
	if runtime.GOOS == "windows" {
		prog = "tool.exe"
	}
	mkfile(t, dirs[19], prog)
	t.Setenv("PATH", strings.Join(dirs, string(os.PathListSeparator)))

	before := Counted()
	s := NewSnapshot()
	found := 0
	for i := 0; i < 200; i++ {
		if _, err := s.LookPath("tool"); err == nil {
			found++
		}
		if _, err := s.LookPath("nothere"); err == nil {
			t.Fatal("found a program that does not exist")
		}
	}
	d := Counted()
	lists, probes := d.DirLists-before.DirLists, d.LookPaths-before.LookPaths
	if looks := d.DirStats - before.DirStats; looks != 20 {
		t.Errorf("looked at directories %d times in one snapshot, want 20 (each once)", looks)
	}
	if found != 200 {
		t.Fatalf("found tool %d of 200 times", found)
	}
	// 20 listings + 1 confirming probe of the one hit; repeats are remembered.
	// exec.LookPath would have made 400 x 20 x (1+len(PATHEXT)) stats.
	if lists != 20 {
		t.Errorf("listed %d directories, want 20 (each once)", lists)
	}
	if probes != 1 {
		t.Errorf("%d confirming probes, want 1 (one per hit, none per miss, none per repeat)", probes)
	}
}

// Later snapshots reuse a directory's listing while the directory is unchanged,
// and see a program installed into it since.
func TestListingReusedUntilTheDirectoryChanges(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	t.Setenv("PATH", a+string(os.PathListSeparator)+b)
	prog := "late"
	if runtime.GOOS == "windows" {
		prog = "late.exe"
	}

	before := Counted().DirLists
	if _, err := NewSnapshot().LookPath("late"); err == nil {
		t.Fatal("found a program not installed yet")
	}
	if n := Counted().DirLists - before; n != 2 {
		t.Fatalf("first snapshot read %d directories, want 2", n)
	}
	for i := 0; i < 50; i++ {
		NewSnapshot().LookPath("late")
	}
	if n := Counted().DirLists - before; n != 2 {
		t.Fatalf("50 more snapshots of an unchanged PATH read %d directories in all, want still 2", n)
	}

	mkfile(t, b, prog)
	p, err := NewSnapshot().LookPath("late")
	if err != nil || !strings.EqualFold(p, filepath.Join(b, prog)) {
		t.Fatalf("after installing into %s: %q, %v", b, p, err)
	}
	if n := Counted().DirLists - before; n != 3 {
		t.Fatalf("after one directory changed: %d reads in all, want 3 (only that one re-read)", n)
	}
}
