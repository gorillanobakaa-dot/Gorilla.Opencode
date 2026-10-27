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
	// PowerToys and Sysinternals are taught only when found.
	for _, absent := range []string{"PowerToys", "Sysinternals"} {
		if strings.Contains(got, absent) {
			t.Errorf("%s named although not found:\n%s", absent, got)
		}
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
