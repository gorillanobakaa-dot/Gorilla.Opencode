// GORILLA OVERRIDE (2026-09-01): the Windows first-run helper.
//
// Rev 1 of this file ran unconditionally from main(), before cobra had even
// parsed the command line. Three things were wrong with that, and all three were
// user-visible:
//
//   - It printed a multi-line doctor banner to STDOUT on every single launch —
//     including `--version`, `--help`, and `-p "..."`. Anything piping this
//     binary's output got the banner mixed into the payload, and the TUI's first
//     paint had to fight it.
//   - It called fmt.Scanln, so a non-interactive run (CI, a pipe, a scheduled
//     task) could block on a prompt nobody would ever answer.
//   - Answering "y" to the Scoop prompt ran os.RemoveAll on ~/scoop, described
//     in the comment as clearing an "orphaned directory". If Scoop was merely
//     absent from PATH — a fresh terminal, a PATH edit, a shim problem — that
//     deleted a working Scoop install and every application in it.
//
// It now runs only on a genuine first interactive launch, writes to stderr, and
// never deletes anything. `gorilla-opencode doctor` runs the checks on demand.
package bootstrap

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/term"
)

// requiredTools maps the command we probe for to the Scoop package providing it.
var requiredTools = []struct{ cmd, pkg string }{
	{"python", "python"},
	{"git", "git"},
	{"go", "go"},
	{"make", "make"},
	{"cmake", "cmake"},
	{"ninja", "ninja"},
	{"gcc", "gcc"},
	{"rg", "ripgrep"},
	{"fd", "fd"},
	{"eza", "eza"},
	{"fzf", "fzf"},
	{"jq", "jq"},
	{"yq", "yq"},
	{"qsv", "qsv"},
	{"pandoc", "pandoc"},
	{"tesseract", "tesseract"},
	{"magick", "imagemagick"},
	{"ffmpeg", "ffmpeg"},
	{"7z", "7zip"},
	{"curl", "curl"},
}

// markerPath is the file whose existence means "we have already offered this".
func markerPath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(base, "gorilla-opencode", "windows-setup-offered")
}

// interactive reports whether there is a real person on the other end.
//
// Both ends matter: stdin must be a terminal or a prompt can never be answered,
// and stdout must be a terminal or our chatter is corrupting somebody's pipeline.
func interactive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// EnsureWindowsDependencies offers to install missing tooling, once, on the
// first interactive launch. It is deliberately silent in every other case.
func EnsureWindowsDependencies() {
	if runtime.GOOS != "windows" {
		return
	}
	// Any argument at all means this is a specific command (--version, --help,
	// -p, a subcommand) rather than the bare interactive launch this helper is
	// for. Staying quiet is the whole point.
	if len(os.Args) > 1 {
		return
	}
	if !interactive() {
		return
	}
	marker := markerPath()
	if marker == "" {
		return
	}
	if _, err := os.Stat(marker); err == nil {
		return // already offered; never nag
	}

	missing := missingTools()
	if len(missing) == 0 {
		writeMarker(marker)
		return
	}

	fmt.Fprintf(os.Stderr, "\n[Gorilla OpenCode] Optional tools not found on PATH:\n  %s\n",
		strings.Join(missing, ", "))

	if _, err := exec.LookPath("scoop"); err != nil {
		// GORILLA OVERRIDE: rev 1 offered to install Scoop here and, on "y",
		// ran os.RemoveAll(~/scoop) first. Deleting a package manager's entire
		// root — and every tool the user installed with it — is not a repair
		// this program gets to make on its own. It prints the command instead.
		fmt.Fprintf(os.Stderr,
			"\nScoop is not installed. It is the easiest way to get these without admin rights:\n"+
				"  https://scoop.sh\n"+
				"Then: scoop install %s\n\n", strings.Join(missing, " "))
		writeMarker(marker)
		return
	}

	fmt.Fprintf(os.Stderr, "Install them with Scoop now? [y/N]: ")
	if !readsYes() {
		fmt.Fprintf(os.Stderr, "Skipped. Run 'gorilla-opencode doctor' to see this again.\n\n")
		writeMarker(marker)
		return
	}

	// GORILLA OVERRIDE: rev 1 forced SCOOP_ALLOW_ADMIN=true and passed
	// -RunAsAdmin to the installer. Scoop warns against admin installs because
	// they leave a root that any user can write to, and this program no longer
	// requires elevation at all (the manifest asks for asInvoker), so there is
	// nothing left to work around.
	if out, err := exec.Command("scoop", "bucket", "add", "extras").CombinedOutput(); err != nil {
		// A bucket that already exists is not an error worth reporting.
		if !strings.Contains(string(out), "already exists") {
			fmt.Fprintf(os.Stderr, "  (could not add the 'extras' bucket: %v)\n", err)
		}
	}
	cmd := exec.Command("scoop", append([]string{"install"}, missing...)...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nScoop install did not complete: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "\nDone. Restart your terminal so PATH changes apply.\n\n")
	}
	writeMarker(marker)
}

func writeMarker(path string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte("offered\n"), 0o600)
}

func readsYes() bool {
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(line), "y")
}

// missingTools returns the Scoop package names for tools that are not usable.
func missingTools() []string {
	var missing []string
	seen := map[string]bool{}
	for _, t := range requiredTools {
		path, err := exec.LookPath(t.cmd)
		if err != nil || isStoreStub(t.cmd, path) {
			if !seen[t.pkg] {
				missing = append(missing, t.pkg)
				seen[t.pkg] = true
			}
		}
	}
	return missing
}

// isStoreStub reports whether a resolved path is one of Windows' App Execution
// Aliases — a zero-byte stub that opens the Microsoft Store instead of running
// anything. The session database has six recorded search failures reading
// "Python was not found; run without arguments to install from the Microsoft
// Store", which is exactly this stub being treated as a real interpreter.
func isStoreStub(cmd, path string) bool {
	if cmd != "python" && cmd != "python3" {
		return false
	}
	return strings.Contains(strings.ToLower(path), "windowsapps")
}

// RunDoctor reports Windows settings that affect how well this program works.
// Exposed so it can be run on demand rather than shouted on every launch.
func RunDoctor() {
	if runtime.GOOS != "windows" {
		fmt.Println("The Windows doctor only has anything to say on Windows.")
		return
	}
	fmt.Println("\n[Gorilla OpenCode] Windows system checks")

	if out, err := exec.Command("reg", "query",
		`HKLM\SYSTEM\CurrentControlSet\Control\FileSystem`, "/v", "LongPathsEnabled").Output(); err == nil {
		if !strings.Contains(string(out), "0x1") {
			fmt.Println("  [!] LongPathsEnabled is off — paths over 260 characters may fail.")
			fmt.Println("      Fix (as admin): reg add \"HKLM\\SYSTEM\\CurrentControlSet\\Control\\FileSystem\" /v LongPathsEnabled /t REG_DWORD /d 1 /f")
		} else {
			fmt.Println("  [ok] Long paths enabled.")
		}
	}

	if out, err := exec.Command("reg", "query",
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock`, "/v",
		"AllowDevelopmentWithoutDevLicense").Output(); err == nil {
		if !strings.Contains(string(out), "0x1") {
			fmt.Println("  [!] Developer Mode is off — creating symlinks will need elevation.")
		} else {
			fmt.Println("  [ok] Developer Mode on.")
		}
	}

	if missing := missingTools(); len(missing) > 0 {
		fmt.Printf("  [!] Optional tools missing: %s\n", strings.Join(missing, ", "))
		fmt.Printf("      scoop install %s\n", strings.Join(missing, " "))
	} else {
		fmt.Println("  [ok] All optional tools found.")
	}

	cwd, _ := os.Getwd()
	if strings.Contains(strings.ToLower(cwd), "onedrive") {
		fmt.Println("  [!] This project is inside OneDrive. Cloud-only placeholder files")
		fmt.Println("      stall reads; make sure the folder is set to 'Always keep on this device'.")
	}
	fmt.Printf("  [i] For speed, exclude this folder from Defender:\n"+
		"      Add-MpPreference -ExclusionPath %q   (in an elevated PowerShell)\n\n", cwd)
}
