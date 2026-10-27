package hostprobe

import (
	"runtime"
	"strings"
	"testing"
)

func winFacts() Facts {
	return Facts{
		OS: "windows", Arch: "amd64", OSName: "Windows 11 Home 25H2 (build 26200.1)", Admin: "no", CPUs: 12,
		Shell:       "Windows PowerShell 5.1.26100.1 (powershell.exe)",
		PowerShells: []string{"PowerShell 7.5.0 (pwsh.exe)", "Windows PowerShell 5.1.26100.1 (powershell.exe)"},
		LongPaths:   "on", DevMode: "off", Git: true, MSVC: "Visual Studio Build Tools 2022 17.14.40", Terminal: true,
		Toolbox: []string{"pfind", "sd"},
	}
}

func TestWindowsLinesNameShellCPUsRobocopyAndToolbox(t *testing.T) {
	got := strings.Join(winFacts().Lines(), "\n")
	for _, want := range []string{
		"OS: Windows 11 Home 25H2 (build 26200.1), amd64; not administrator",
		"Shell (the bash tool): Windows PowerShell 5.1.26100.1 (powershell.exe); also installed: PowerShell 7.5.0 (pwsh.exe)",
		"CPUs: 12 logical; robocopy /MT:n takes 1-128 threads",
		"Windows: long paths on, Developer Mode off",
		"Installed: git, MSVC (Visual Studio Build Tools 2022 17.14.40), Windows Terminal (wt)",
		"Embedded toolbox (callable by name in the shell): pfind, sd",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("lines lack %q:\n%s", want, got)
		}
	}
	// Sysinternals is taught only when found. PowerToys is named either way on
	// Windows (job 07: a missing line read as "not checked", not as "no").
	if strings.Contains(got, "Sysinternals") {
		t.Errorf("Sysinternals named although not found:\n%s", got)
	}
	if !strings.Contains(got, "PowerToys: not installed") {
		t.Errorf("PowerToys' absence is not said:\n%s", got)
	}
	if strings.Contains(got, "MSVC build:") {
		t.Errorf("an MSVC build line without a VsDevCmd.bat:\n%s", got)
	}
}

// Job 07: GPT-6-Luna knew MSVC was installed but failed twice to quote
// cmd /c "...VsDevCmd.bat..." from PowerShell 5.1. The line names the file and
// the one way that needs no nested quoting.
func TestWindowsLinesTeachTheMSVCDevShellThroughACmdFile(t *testing.T) {
	f := winFacts()
	f.MSVCDevCmd = `C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\Common7\Tools\VsDevCmd.bat`
	got := strings.Join(f.Lines(), "\n")
	want := `MSVC build: write a .cmd file (line 1: @call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\Common7\Tools\VsDevCmd.bat" -arch=amd64 -no_logo; then cl lines) and run cmd /c FILE.cmd`
	if !strings.Contains(got, want) {
		t.Errorf("lines lack %q:\n%s", want, got)
	}
}

func TestPowerToysAndSysinternalsAppearOnlyWhenFound(t *testing.T) {
	f := winFacts()
	f.PowerToys = `C:\Users\u\AppData\Local\PowerToys`
	f.PowerToysCLIs = []string{"FileLocksmithCLI.exe", "PowerToys.Awake.exe"}
	f.Sysinternals = []string{"handle", "sigcheck"}
	got := strings.Join(f.Lines(), "\n")
	for _, want := range []string{
		`PowerToys: installed at C:\Users\u\AppData\Local\PowerToys; command-line: FileLocksmithCLI.exe, PowerToys.Awake.exe`,
		"Sysinternals on PATH: handle, sigcheck",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("lines lack %q:\n%s", want, got)
		}
	}
}

func TestLinuxLinesCarryNoWindowsFacts(t *testing.T) {
	f := Facts{OS: "linux", Arch: "amd64", OSName: "Debian GNU/Linux 13 (trixie)", CPUs: 8, Shell: "bash", Git: true}
	got := strings.Join(f.Lines(), "\n")
	for _, absent := range []string{"robocopy", "PowerShell", "Developer Mode", "MSVC", "administrator"} {
		if strings.Contains(got, absent) {
			t.Errorf("Linux lines mention %q:\n%s", absent, got)
		}
	}
	if !strings.Contains(got, "OS: Debian GNU/Linux 13 (trixie), amd64") || !strings.Contains(got, "CPUs: 8 logical") ||
		!strings.Contains(got, "Embedded toolbox: none in this build") {
		t.Errorf("Linux lines incomplete:\n%s", got)
	}
}

func TestShellEditionByName(t *testing.T) {
	if got := ShellEdition("/bin/bash"); got != "bash" {
		t.Errorf("bash: %q", got)
	}
	if got := ShellEdition(`C:\Program Files\PowerShell\7\pwsh.exe`); !strings.HasPrefix(got, "PowerShell 7") || !strings.HasSuffix(got, "(pwsh.exe)") {
		t.Errorf("pwsh: %q", got)
	}
	if got := ShellEdition("powershell.exe"); !strings.HasPrefix(got, "Windows PowerShell 5.1") {
		t.Errorf("powershell: %q", got)
	}
}

// The live probe on the machine running the tests: cached, and true to it.
func TestGetIsCachedAndMatchesThisMachine(t *testing.T) {
	a, b := Get(), Get()
	if strings.Join(a.Lines(), "\n") != strings.Join(b.Lines(), "\n") {
		t.Fatal("two renders of the probe differ: the prompt would not be cache-stable")
	}
	if a.CPUs != runtime.NumCPU() || a.OS != runtime.GOOS {
		t.Fatalf("probe says %d CPUs on %s, the runtime %d on %s", a.CPUs, a.OS, runtime.NumCPU(), runtime.GOOS)
	}
	if runtime.GOOS == "windows" {
		if !strings.Contains(a.Shell, "PowerShell") || !strings.HasPrefix(a.OSName, "Windows") || a.Admin == "" {
			t.Fatalf("Windows probe incomplete: %+v", a)
		}
	}
}
