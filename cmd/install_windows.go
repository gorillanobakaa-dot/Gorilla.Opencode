//go:build windows

// GORILLA OVERRIDE (2026-09-01): the Windows half of `install` / `uninstall`.
//
// There was none. `install` was the Linux implementation with no build tag, so
// on Windows it wrote a `.desktop` file — a Linux application-grid entry, in a
// format nothing on Windows reads — into ~/.local/share/applications, unpacked
// hicolor icon directories nothing would look at, and reported success. The
// binary stayed wherever the user had downloaded it, with no shortcut anywhere
// and nothing on PATH.
//
// What Windows actually needs is four things, none of which requires
// administrator rights:
//
//   - a permanent home for the executable, under %LOCALAPPDATA%\Programs
//   - a Desktop shortcut
//   - a Start menu entry, so it is reachable by typing its name
//   - the install directory on the user's PATH
//
// All of it is per-user on purpose. Installing for all users means elevation,
// and this program deliberately no longer asks for that (see winres.json —
// the manifest went from requireAdministrator to asInvoker in this same
// session). A per-user install is also the one a person can undo.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	appDisplayName = "Gorilla OpenCode"
	appExeName     = appBinName + ".exe"
	appIcoName     = appBinName + ".ico"
)

// installDir is where the executable and its icon live.
func installDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(base, "Programs", appDisplayName)
}

// desktopShortcut is the .lnk on the user's Desktop.
//
// USERPROFILE\Desktop is not always right — OneDrive-redirected profiles put the
// real Desktop under the OneDrive folder — so the redirected location is
// preferred when it exists. Getting this wrong means writing a shortcut to a
// folder the user never sees, which looks exactly like the install silently
// failing.
func desktopShortcut() string {
	home, _ := os.UserHomeDir()
	candidates := []string{}
	if od := os.Getenv("OneDrive"); od != "" {
		candidates = append(candidates, filepath.Join(od, "Desktop"))
	}
	candidates = append(candidates, filepath.Join(home, "Desktop"))
	for _, dir := range candidates {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return filepath.Join(dir, appDisplayName+".lnk")
		}
	}
	return filepath.Join(home, "Desktop", appDisplayName+".lnk")
}

// startMenuShortcut is the .lnk that makes the app findable by typing its name.
func startMenuShortcut() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs",
		appDisplayName+".lnk")
}

// conhostLauncher returns the target and arguments a shortcut should use.
//
// Falls back to running the exe directly when conhost is missing, which should
// not happen on any supported Windows but costs one Stat to be certain of: a
// shortcut pointing at a file that is not there is worse than one without an
// icon.
func conhostLauncher(exe string) (target, args string) {
	conhost := filepath.Join(os.Getenv("SystemRoot"), "System32", "conhost.exe")
	if os.Getenv("SystemRoot") == "" {
		conhost = `C:\Windows\System32\conhost.exe`
	}
	if st, err := os.Stat(conhost); err != nil || st.IsDir() {
		return exe, ""
	}
	return conhost, `"` + exe + `"`
}

// psQuote renders a Go string as a PowerShell single-quoted literal.
//
// Single quotes, not double: PowerShell expands $variables and backticks inside
// double quotes, and these strings are filesystem paths that can legitimately
// contain a $ (a user named their folder that) — which would otherwise be
// silently replaced with nothing and write the shortcut somewhere else.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// createShortcut writes a Windows .lnk.
//
// Shelled out to PowerShell's WScript.Shell rather than done in Go, deliberately:
// a .lnk is an OLE structured-storage document, and writing one directly means
// either a cgo dependency or hand-rolling a binary format whose corner cases
// (icon indices, working directories, UNC targets) are exactly where it would
// break. WScript.Shell is present on every Windows install and is the same API
// every other installer uses.
func createShortcut(lnkPath, target, iconPath, description string) error {
	if err := os.MkdirAll(filepath.Dir(lnkPath), 0o755); err != nil {
		return err
	}

	// GORILLA OVERRIDE (2026-09-01): launch through conhost so the app owns its
	// own window — and therefore its own taskbar icon.
	//
	// Windows 11 defaults the terminal host to Windows Terminal. A console
	// program started there does not get a window at all: it becomes a TAB
	// inside Windows Terminal's window, so the taskbar button belongs to
	// Windows Terminal and shows Windows Terminal's icon. Measured directly —
	// the process reports MainWindowHandle = 0 under Windows Terminal and a real
	// handle under conhost.
	//
	// This is why the icon appeared to be broken and was not. It was correctly
	// embedded at all seven sizes and the shell read a real 256x256 out of the
	// exe; there was simply no window of ours for Windows to put it on.
	//
	// Naming conhost explicitly is a per-SHORTCUT choice. It does not touch the
	// machine's default terminal setting, so every other console program —
	// Git Bash, python, whatever else — is unaffected. Someone who prefers
	// Windows Terminal can point the shortcut back at the exe directly and get
	// the old behaviour, icon included in the trade.
	target, args := conhostLauncher(target)

	// The working directory is the USER'S OWN, not the install directory.
	// Starting in %LOCALAPPDATA%\Programs meant the first thing the app asked
	// was whether to work in its own installation folder — which is never the
	// answer for a coding tool, and is a confusing first question to be asked.
	workDir := os.Getenv("USERPROFILE")
	if workDir == "" {
		workDir, _ = os.UserHomeDir()
	}

	script := strings.Join([]string{
		"$ErrorActionPreference = 'Stop'",
		"$s = (New-Object -ComObject WScript.Shell).CreateShortcut(" + psQuote(lnkPath) + ")",
		"$s.TargetPath = " + psQuote(target),
		"$s.Arguments = " + psQuote(args),
		"$s.WorkingDirectory = " + psQuote(workDir),
		"$s.Description = " + psQuote(description),
		// GORILLA FIX (2026-09-02): open maximised.
		//
		// A console launched from a shortcut starts at conhost stored default,
		// which on the owner machine is 120 columns by 30 rows. bubbletea reads
		// the terminal size ONCE on Windows -- it has no SIGWINCH and its
		// listenForResize is an empty function there -- so the program then drew
		// for 120x30 for the whole session no matter how large the window was.
		// internal/tui/resize_windows.go now corrects that after the fact; this
		// stops it happening in the first place, so the very first frame is the
		// right size rather than being fixed a quarter of a second later.
		//
		// 3 is SW_SHOWMAXIMIZED. The two other values a .lnk accepts are 1
		// (normal) and 7 (minimised); there is no "remember what the user last
		// did", so this is a choice between a small window every launch and a
		// large one. A terminal application with a footer, a prompt and a
		// transcript wants the room.
		"$s.WindowStyle = 3",
		// ",0" selects the FIRST icon group in the file. The resource built by
		// winres puts 256x256 first, which is what Windows 11 wants for large
		// tiles; it picks the smaller entries out of the same group by itself.
		"$s.IconLocation = " + psQuote(iconPath+",0"),
		"$s.Save()",
	}, "; ")

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating shortcut %s: %v: %s", lnkPath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// addToUserPath appends dir to the user's PATH, if it is not already there.
//
// Uses the .NET environment API through PowerShell rather than `setx`, which
// TRUNCATES the value at 1024 characters — a real and well-known way to destroy
// somebody's PATH. It also reads the User scope specifically: reading the
// process PATH (machine + user combined) and writing it back into the user scope
// is the other classic way to wreck it, duplicating every system entry into the
// user's own and making the damage permanent.
func addToUserPath(dir string) (added bool, err error) {
	script := strings.Join([]string{
		"$ErrorActionPreference = 'Stop'",
		"$d = " + psQuote(dir),
		"$p = [Environment]::GetEnvironmentVariable('Path','User')",
		"if ($null -eq $p) { $p = '' }",
		"$parts = $p -split ';' | Where-Object { $_ -ne '' }",
		"if ($parts -contains $d) { Write-Output 'already'; exit 0 }",
		"$new = (@($parts) + $d) -join ';'",
		"[Environment]::SetEnvironmentVariable('Path', $new, 'User')",
		"Write-Output 'added'",
	}, "; ")

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("updating PATH: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.Contains(string(out), "added"), nil
}

func removeFromUserPath(dir string) {
	script := strings.Join([]string{
		"$ErrorActionPreference = 'SilentlyContinue'",
		"$d = " + psQuote(dir),
		"$p = [Environment]::GetEnvironmentVariable('Path','User')",
		"if ($null -eq $p) { exit 0 }",
		"$parts = $p -split ';' | Where-Object { $_ -ne '' -and $_ -ne $d }",
		"[Environment]::SetEnvironmentVariable('Path', ($parts -join ';'), 'User')",
	}, "; ")
	_ = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Run()
}

func platformInstall() error {
	dir := installDir()
	exePath := filepath.Join(dir, appExeName)
	icoPath := filepath.Join(dir, appIcoName)

	copied, err := copySelf(exePath)
	if err != nil {
		return err
	}
	if copied {
		fmt.Println("installed:", exePath)
	} else {
		fmt.Println("already running from:", exePath)
	}

	// The .ico carries all seven sizes (256 down to 16) so Windows has a real
	// image for every context — the 256 for large tiles and the Alt-Tab view,
	// the 16 for the title bar — instead of downscaling one big PNG badly.
	if err := installAsset("icons/"+appIcoName, icoPath); err != nil {
		return err
	}
	fmt.Println("installed icon:", icoPath, "(256/128/64/48/32/24/16)")

	// The shortcut points at the EXE, not at "exe launch". Double-clicking a
	// console program is how a terminal app is opened on Windows, and cobra's
	// mousetrap guard is already disabled in main.go for exactly this.
	desc := "Terminal AI coding agent - bring your own API keys"
	if err := createShortcut(desktopShortcut(), exePath, icoPath, desc); err != nil {
		return err
	}
	fmt.Println("created Desktop shortcut:", desktopShortcut())

	if err := createShortcut(startMenuShortcut(), exePath, icoPath, desc); err != nil {
		return err
	}
	fmt.Println("created Start menu entry:", startMenuShortcut())

	switch added, err := addToUserPath(dir); {
	case err != nil:
		// Not fatal. The shortcuts work regardless, and a PATH we could not
		// update is worth saying out loud rather than failing the whole install.
		fmt.Println("could not add to PATH:", err)
	case added:
		fmt.Println("added to your PATH:", dir)
		fmt.Println("  (open a NEW terminal before `gorilla-opencode` works there)")
	default:
		fmt.Println("already on your PATH:", dir)
	}

	fmt.Println()
	fmt.Println("Done. Look for", appDisplayName, "on your Desktop or in the Start menu.")
	return nil
}

func platformUninstall() error {
	dir := installDir()
	removed := 0
	for _, p := range []string{
		desktopShortcut(),
		startMenuShortcut(),
		filepath.Join(dir, appIcoName),
	} {
		if err := os.Remove(p); err == nil {
			fmt.Println("removed:", p)
			removed++
		}
	}

	// The executable is left in place when it is the one currently running:
	// Windows locks a running image, and deleting it would fail anyway.
	exePath := filepath.Join(dir, appExeName)
	if self, err := os.Executable(); err == nil && sameFile(self, exePath) {
		fmt.Println("left in place (it is running right now):", exePath)
		fmt.Println("  delete it yourself once this process exits, or run uninstall from a copy elsewhere")
	} else if err := os.Remove(exePath); err == nil {
		fmt.Println("removed:", exePath)
		removed++
	}

	removeFromUserPath(dir)
	fmt.Println("removed from PATH:", dir)

	if removed == 0 {
		fmt.Println("nothing to remove (was it installed for this user?)")
	}
	return nil
}
