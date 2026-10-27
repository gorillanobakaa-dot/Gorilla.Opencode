// Package hostprobe tells the model what machine it is on, in a few short
// lines of the system prompt.
//
// GORILLA (2026-10-27, job 06): the owner found that local models did not know
// the tools on this computer, robocopy's 128 threads, which PowerShell they were
// driving, or that PowerToys was there. The prompt said only "Platform:
// windows". These are FACTS read once per process from the registry, the file
// system and PATH: no process is started, nothing goes on the network, and the
// answer is cached so every render of the prompt is byte-identical (a changing
// line near the front of the prompt throws the KV cache away).
package hostprobe

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
	"github.com/opencode-ai/opencode/internal/toolbox"
)

// RobocopyMaxThreads is the highest /MT:n robocopy accepts.
const RobocopyMaxThreads = 128

// Facts is what the probe found. An empty field was not found or does not
// apply to this OS; Lines leaves it out rather than guessing.
type Facts struct {
	OS     string // runtime.GOOS
	Arch   string // runtime.GOARCH
	OSName string // "Windows 11 Home 25H2 (build 26200.6899)", "Debian GNU/Linux 13 (trixie)"
	Admin  string // "yes", "no" or "" (unknown)
	CPUs   int    // logical processors

	Shell       string   // what the bash tool drives, with its edition
	PowerShells []string // every PowerShell installed (Windows)

	LongPaths string // "on", "off" or ""
	DevMode   string // "on", "off" or ""

	Git      bool
	MSVC     string // "Visual Studio Build Tools 2022 17.14.40"
	Terminal bool   // Windows Terminal (wt.exe)

	PowerToys     string   // install folder, "" when not installed
	PowerToysCLIs []string // command-line programs found in that folder

	Sysinternals []string // Sysinternals programs found on PATH
	Toolbox      []string // helpers embedded in this binary (internal/toolbox)
}

var (
	once   sync.Once
	cached Facts
)

// Get probes once per process and returns the same answer after that.
func Get() Facts {
	once.Do(func() { cached = probe() })
	return cached
}

func probe() Facts {
	f := Facts{OS: runtime.GOOS, Arch: runtime.GOARCH, CPUs: runtime.NumCPU()}
	_, err := exec.LookPath("git")
	f.Git = err == nil
	f.Toolbox = toolbox.Names()
	probeOS(&f)
	return f
}

// ShellEdition names the shell at path the way a person would: "Windows
// PowerShell 5.1.26100.1 (powershell.exe)", "PowerShell 7.5.0 (pwsh.exe)",
// "bash". The version is read from the registry on Windows, never by starting
// the shell.
func ShellEdition(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(base, "pwsh"):
		v := pwshVersion()
		if v == "" {
			v = "7+"
		}
		return "PowerShell " + v + " (pwsh.exe)"
	case strings.Contains(base, "powershell"):
		v := winPowerShellVersion()
		if v == "" {
			v = "5.1"
		}
		return "Windows PowerShell " + v + " (powershell.exe)"
	}
	return strings.TrimSuffix(filepath.Base(path), ".exe")
}

// IsPowerShell7 reports whether the bash tool drives PowerShell 7+ (pwsh),
// which, unlike Windows PowerShell 5.1, accepts && and ||.
func IsPowerShell7() bool {
	return runtime.GOOS == "windows" && strings.Contains(strings.ToLower(filepath.Base(shell.ShellPath())), "pwsh")
}

// sysinternalsNames are looked up on PATH, never downloaded or installed.
var sysinternalsNames = []string{
	"procexp", "procexp64", "procmon", "procmon64", "handle", "handle64", "sigcheck", "sigcheck64",
	"autorunsc", "autorunsc64", "psexec", "psexec64", "pslist", "pskill", "listdlls", "tcpvcon", "accesschk", "strings64",
}

func lookSysinternals() []string {
	var out []string
	for _, n := range sysinternalsNames {
		if _, err := exec.LookPath(n + ".exe"); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// Lines is the probe as a few short lines for the <env> block. Facts that are
// absent are left out; PowerToys and Sysinternals appear only when found.
func (f Facts) Lines() []string {
	var l []string
	name := f.OSName
	if name == "" {
		name = f.OS
	}
	first := fmt.Sprintf("OS: %s, %s", name, f.Arch)
	switch f.Admin {
	case "yes":
		first += "; running as administrator"
	case "no":
		first += "; not administrator"
	}
	l = append(l, first)
	if f.Shell != "" {
		s := "Shell (the bash tool): " + f.Shell
		if f.OS == "windows" {
			if others := without(f.PowerShells, f.Shell); len(others) > 0 {
				s += "; also installed: " + strings.Join(others, ", ")
			}
		}
		l = append(l, s)
	}
	cpu := fmt.Sprintf("CPUs: %d logical", f.CPUs)
	if f.OS == "windows" {
		cpu += fmt.Sprintf("; robocopy /MT:n takes 1-%d threads (default 8), the files tool drives it", RobocopyMaxThreads)
	}
	l = append(l, cpu)
	if f.OS == "windows" {
		var w []string
		if f.LongPaths != "" {
			w = append(w, "long paths "+f.LongPaths)
		}
		if f.DevMode != "" {
			w = append(w, "Developer Mode "+f.DevMode)
		}
		if len(w) > 0 {
			l = append(l, "Windows: "+strings.Join(w, ", "))
		}
	}
	var found []string
	if f.Git {
		found = append(found, "git")
	}
	if f.MSVC != "" {
		found = append(found, "MSVC ("+f.MSVC+")")
	}
	if f.Terminal {
		found = append(found, "Windows Terminal (wt)")
	}
	if len(found) > 0 {
		l = append(l, "Installed: "+strings.Join(found, ", "))
	}
	if f.PowerToys != "" {
		s := "PowerToys: installed at " + f.PowerToys
		if len(f.PowerToysCLIs) > 0 {
			s += "; command-line: " + strings.Join(f.PowerToysCLIs, ", ")
		}
		l = append(l, s)
	}
	if len(f.Sysinternals) > 0 {
		l = append(l, "Sysinternals on PATH: "+strings.Join(f.Sysinternals, ", "))
	}
	if len(f.Toolbox) > 0 {
		l = append(l, "Embedded toolbox (callable by name in the shell): "+strings.Join(f.Toolbox, ", "))
	} else {
		l = append(l, "Embedded toolbox: none in this build")
	}
	return l
}

// without returns list minus the entry that names the same program as shell.
func without(list []string, shell string) []string {
	var out []string
	for _, s := range list {
		if !strings.EqualFold(s, shell) {
			out = append(out, s)
		}
	}
	return out
}

// osRelease reads PRETTY_NAME from /etc/os-release (Linux).
func osRelease() string {
	raw, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
}

func sortedUnique(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}
