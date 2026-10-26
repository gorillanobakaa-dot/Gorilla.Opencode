// Version: 1.1.0 · updated 26-10-23-02-33
// GORILLA OVERRIDE (2026-10-23): what a shell command IS, for the permission
// rules: read-only (never asks), a hard stop (asks even under auto-approve),
// or able to reach the network (asks under auto-approve only after the turn
// read untrusted content).
//
// # WHY THIS FILE EXISTS
//
// Measured on the MacBook Air on 2026-10-23 (v0.1.149-dev, Gemini 3.8 Flash
// High, auto-approve ON), installing Fieldkit as a stranger: a probe of
// /Library/Developer/CommandLineTools and `python3 --version && git --version`
// each raised a security dialog. Neither can change or send anything. The
// first asked because `2>/dev/null` counted as a write (any `>` did) and the
// path was outside the workspace; the second because `python3` was not on the
// read-only list, so after a web page had been read every shell command asked.
//
// The rules below are tables (data), each row with a test in
// commandclass_test.go. commandgate.go still decides HOW a line is split and
// what is opaque; nothing here relaxes "a command is not its first word":
// every segment is judged, and a single unknown segment loses the line its
// read-only standing.
package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
)

// readOnlyAdditions extend safeReadOnlyCommands (bash.go). Each one reads and
// prints; the flags that would make one write, delete or run a program are in
// unsafeFlags (commandgate.go). Reading a credential file through one of these
// is caught by namesSensitivePath, the same guard the view tool uses, and what
// reaches the transcript passes through the after-tool secret mask.
var readOnlyAdditions = []string{
	"cat", "head", "tail", "wc", "file", "stat", "find", "grep", "egrep", "fgrep", "rg",
	"tree", "realpath", "readlink", "basename", "dirname", "test", "[",
	"command -v", "diff", "cmp", "md5sum", "sha256sum", "sha1sum", "shasum", "md5",
	"sort", "cut", "nproc", "arch", "locale", "tty", "lsb_release", "lscpu",
	// macOS: how an agent learns which toolchain is there.
	"sw_vers", "sysctl -n", "xcode-select -p", "xcode-select --print-path",
	"pkgutil --pkg-info", "pkgutil --pkgs", "brew --prefix", "brew list", "brew config",
	"pip list", "pip3 list", "pip show", "pip3 show", "npm ls", "npm list",
}

// unixOnlyReadOnly are read-only in bash and something else in PowerShell:
// `type` is Get-Content there, which read a credentials file unasked
// (2026-10-05, see bash.go). In PowerShell it is listed again below, as the
// Get-Content alias it is, now that namesSensitivePath expands $env: paths.
var unixOnlyReadOnly = []string{"type"}

// powerShellReadOnly are the cmdlets (and their built-in aliases) that only
// read: the inspection the tool guidance itself tells the model to use, and
// which asked permission every time (gap #7). A pipeline is read-only only if
// EVERY stage is (commandgate.go splits at |), so a Rename-Item, Remove-Item
// or Set-* tail still asks. Script blocks { }, sub-expressions and
// redirection stay opaque.
var powerShellReadOnly = []string{
	"Get-ChildItem", "gci", "dir", "ls",
	"Get-Item", "gi",
	"Get-ItemProperty", "gp",
	"Get-Content", "gc", "cat", "type",
	"Test-Path",
	"Get-Command", "gcm",
	"Get-Location", "gl", "pwd",
	"Resolve-Path", "rvpa",
	"Get-FileHash",
	"Get-Acl",
	"Select-String", "sls",
	"Measure-Object", "measure",
	"Select-Object", "select", "Sort-Object", "sort",
	"Format-Table", "ft", "Format-List", "fl", "Out-String",
	"Get-Date", "date",
}

// notInPowerShell are read-only list entries that are neither a PowerShell
// alias nor a Windows program, dropped when the shell is PowerShell: a name
// that does not exist there can only resolve to whatever .exe of that name a
// PATH directory happens to hold. Their PowerShell equivalents are in
// powerShellReadOnly (head/tail: Select-Object -First/-Last, wc:
// Measure-Object, grep: Select-String, stat/file: Get-Item, which/command -v:
// Get-Command, md5sum/sha256sum: Get-FileHash, realpath: Resolve-Path,
// test/[: Test-Path).
var notInPowerShell = map[string]bool{
	"cal": true, "uptime": true, "id": true, "groups": true, "which": true, "whereis": true,
	"whatis": true, "uname": true, "df": true, "du": true, "free": true,
	"head": true, "tail": true, "wc": true, "file": true, "stat": true,
	"grep": true, "egrep": true, "fgrep": true, "realpath": true, "readlink": true,
	"basename": true, "dirname": true, "test": true, "[": true, "command -v": true,
	"cmp": true, "md5sum": true, "sha256sum": true, "sha1sum": true, "shasum": true, "md5": true,
	"cut": true, "nproc": true, "arch": true, "locale": true, "tty": true,
	"lsb_release": true, "lscpu": true, "sw_vers": true, "sysctl -n": true,
	"xcode-select -p": true, "xcode-select --print-path": true,
	"pkgutil --pkg-info": true, "pkgutil --pkgs": true,
	"brew --prefix": true, "brew list": true, "brew config": true,
}

// propertyRead is a whole segment of the form (Get-Item x).Attributes: a
// parenthesised plain command followed only by property names. Nothing that
// can hide another command may be inside, and nothing (an `=`, a method call)
// may follow, so (Get-Item x).IsReadOnly = $false and (Get-Item x).Delete()
// stay opaque.
var propertyRead = regexp.MustCompile(`^\(\s*([^(){}$` + "`" + `<>;|&\r\n=]+?)\s*\)((?:\.[A-Za-z_][A-Za-z0-9_]*)+)$`)

// unwrapPropertyReads replaces each segment of that form by its inner
// command, so the inner command is judged like any other.
func unwrapPropertyReads(cmd string) string {
	if !strings.Contains(cmd, ").") {
		return cmd
	}
	var out strings.Builder
	start := 0
	flush := func(end int) {
		seg := cmd[start:end]
		trimmed := strings.TrimSpace(seg)
		if m := propertyRead.FindStringSubmatch(trimmed); m != nil {
			out.WriteString(" " + m[1] + " ")
		} else {
			out.WriteString(seg)
		}
	}
	for i := 0; i < len(cmd); i++ {
		switch cmd[i] {
		case ';', '|', '&', '\n', '\r':
			flush(i)
			out.WriteByte(cmd[i])
			start = i + 1
		}
	}
	flush(len(cmd))
	return out.String()
}

// versionFlags may follow a bare program name to make a read-only query.
var versionFlags = map[string]bool{"--version": true, "-version": true, "--help": true, "version": true}

// isVersionQuery reports `<program> --version` and its kin: two words, the
// program found on PATH (no directory in its name, so not a file the model
// may have just written), the second word a version or help flag.
func isVersionQuery(segment string) bool {
	f := strings.Fields(segment)
	if len(f) != 2 || !versionFlags[f[1]] {
		return false
	}
	return !strings.ContainsAny(f[0], `/\`) && f[0] != "." && f[0] != "source"
}

// benignRedirects only discard or merge output: `2>/dev/null`, `>/dev/null`,
// `&>/dev/null`, `2>&1`, and PowerShell's `2>$null`. They are removed before
// the read-only analysis, so `ls x 2>/dev/null` is judged as `ls x`. Any other
// `>` still makes a line opaque (it writes a file).
var benignRedirects = regexp.MustCompile(`(^|\s)([12]?>>?|&>)\s*(/dev/null|\$null)\b|(^|\s)[12]?>&[12]\b`)

func stripBenignRedirects(cmd string) string {
	return benignRedirects.ReplaceAllString(cmd, " ")
}

// namesSensitivePath reports whether any word of the command is a path the
// credential guard refuses to read (an SSH key, .aws/credentials, a .env, this
// program's own configuration) or a process environment file. A read-only
// command naming one is asked about, never waved through.
func namesSensitivePath(cmd string) bool {
	home, _ := os.UserHomeDir()
	for _, w := range strings.Fields(cmd) {
		w = strings.Trim(w, `"'`)
		if w == "" || strings.HasPrefix(w, "-") {
			continue
		}
		if strings.Contains(w, "environ") && strings.Contains(w, "/proc/") {
			return true
		}
		// $env:USERPROFILE\.aws\credentials was judged as a RELATIVE path,
		// i.e. inside the workspace and exempt, so `cat` (Get-Content in
		// PowerShell) read it with no prompt. Expanded first now.
		if x := expandShellPathWord(w, home); x != w {
			w = filepath.Clean(x)
		}
		if !strings.ContainsAny(w, `/\`) && !strings.HasPrefix(w, ".") {
			continue
		}
		if RefuseSensitiveRead(w) != "" {
			return true
		}
	}
	return false
}

// hardStop is one row of the "asks even under auto-approve" table.
type hardStop struct {
	// words is the command's leading words (program first). A row matches a
	// segment whose words begin with these; "*" matches any one word.
	words []string
	// unless names words whose presence anywhere in the segment means the
	// row does not apply (`pip install --user` is not system-wide).
	unless []string
	// venvOK: the row does not apply when the program is run by path from a
	// virtual environment or from inside a workspace root (`.venv/bin/python
	// -m pip install`): that installs into the project, not the machine.
	venvOK bool
	why    string
}

// hardStops are the actions auto-approve still stops for although they can
// be undone: running as the administrator and installing software for the
// whole machine. The plan (PERMISSIONS-REWORK-PLAN.md) names these, and the
// owner's "done" says the stranger install must still stop here.
var hardStops = []hardStop{
	{words: []string{"sudo"}, why: "it runs as the administrator (sudo)"},
	{words: []string{"doas"}, why: "it runs as the administrator (doas)"},
	{words: []string{"su"}, why: "it switches to another user (su)"},
	{words: []string{"pkexec"}, why: "it runs as the administrator (pkexec)"},
	{words: []string{"runas"}, why: "it runs as another user (runas)"},
	{words: []string{"brew", "install"}, why: "it installs software for the whole machine (brew)"},
	{words: []string{"brew", "reinstall"}, why: "it installs software for the whole machine (brew)"},
	{words: []string{"brew", "upgrade"}, why: "it upgrades software for the whole machine (brew)"},
	{words: []string{"brew", "uninstall"}, why: "it removes software from the whole machine (brew)"},
	{words: []string{"port", "install"}, why: "it installs software for the whole machine (MacPorts)"},
	{words: []string{"apt", "install"}, why: "it installs software for the whole machine (apt)"},
	{words: []string{"apt-get", "install"}, why: "it installs software for the whole machine (apt-get)"},
	{words: []string{"apt", "remove"}, why: "it removes software from the whole machine (apt)"},
	{words: []string{"apt-get", "remove"}, why: "it removes software from the whole machine (apt-get)"},
	{words: []string{"dnf", "install"}, why: "it installs software for the whole machine (dnf)"},
	{words: []string{"yum", "install"}, why: "it installs software for the whole machine (yum)"},
	{words: []string{"pacman", "-S"}, why: "it installs software for the whole machine (pacman)"},
	{words: []string{"pacman", "-U"}, why: "it installs software for the whole machine (pacman)"},
	{words: []string{"zypper", "install"}, why: "it installs software for the whole machine (zypper)"},
	{words: []string{"snap", "install"}, why: "it installs software for the whole machine (snap)"},
	{words: []string{"flatpak", "install"}, why: "it installs software for the whole machine (flatpak)"},
	{words: []string{"dpkg", "-i"}, why: "it installs a package for the whole machine (dpkg)"},
	{words: []string{"installer"}, why: "it installs a package for the whole machine (macOS installer)"},
	{words: []string{"softwareupdate"}, why: "it changes the operating system (softwareupdate)"},
	{words: []string{"xcode-select", "--install"}, why: "it installs the Command Line Tools for the whole machine"},
	{words: []string{"pip", "install"}, unless: []string{"--user", "--target", "-t", "--prefix"}, venvOK: true, why: "it installs Python packages outside a project (pip)"},
	{words: []string{"pip3", "install"}, unless: []string{"--user", "--target", "-t", "--prefix"}, venvOK: true, why: "it installs Python packages outside a project (pip3)"},
	{words: []string{"*", "-m", "pip", "install"}, unless: []string{"--user", "--target", "-t", "--prefix"}, venvOK: true, why: "it installs Python packages outside a project (pip)"},
	{words: []string{"npm", "install", "-g"}, why: "it installs a program for the whole machine (npm -g)"},
	{words: []string{"npm", "i", "-g"}, why: "it installs a program for the whole machine (npm -g)"},
	{words: []string{"npm", "install", "--global"}, why: "it installs a program for the whole machine (npm --global)"},
	{words: []string{"gem", "install"}, unless: []string{"--user-install"}, why: "it installs Ruby gems outside a project (gem)"},
	{words: []string{"cargo", "install"}, why: "it installs a program (cargo install)"},
}

// segmentWords is a segment's words with a leading path stripped from the
// program, so /usr/bin/sudo is sudo.
func segmentWords(seg string) []string {
	f := strings.Fields(seg)
	for len(f) > 0 && strings.Contains(f[0], "=") && !strings.HasPrefix(f[0], "-") {
		f = f[1:] // FOO=bar cmd
	}
	if len(f) > 0 {
		f[0] = strings.Trim(f[0], `"'`)
		if i := strings.LastIndexAny(f[0], `/\`); i >= 0 {
			f[0] = f[0][i+1:]
		}
		f[0] = strings.TrimSuffix(strings.ToLower(f[0]), ".exe")
	}
	return f
}

// projectLocalProgram reports whether a segment's program is named by a path
// inside a virtual environment or a workspace root, so whatever it installs
// lands in the project. Measured 2026-10-23 (run 2, row 22):
// `.venv/bin/python -m pip install pytest` asked as if it were system-wide.
func projectLocalProgram(seg string) bool {
	f := strings.Fields(seg)
	for len(f) > 0 && strings.Contains(f[0], "=") && !strings.HasPrefix(f[0], "-") {
		f = f[1:]
	}
	if len(f) == 0 {
		return false
	}
	prog := strings.Trim(f[0], `"'`)
	if !strings.ContainsAny(prog, `/\`) {
		return false
	}
	low := strings.ToLower(filepath.ToSlash(prog))
	for _, v := range []string{"/.venv/", "/venv/", "/.virtualenvs/", ".venv/", "venv/"} {
		if strings.Contains(low, v) {
			return true
		}
	}
	if filepath.IsAbs(prog) {
		_, inside := config.RootFor(filepath.Clean(prog))
		return inside
	}
	return false
}

// HardStopIn returns why auto-approve must still ask about this command, or
// "" when nothing in it is a hard stop. Every segment is checked.
func HardStopIn(cmd string) string {
	for _, seg := range splitShellCommands(cmd) {
		w := segmentWords(seg)
		for _, h := range hardStops {
			if len(w) < len(h.words) {
				continue
			}
			match := true
			for i, hw := range h.words {
				if hw != "*" && !strings.EqualFold(w[i], hw) {
					match = false
					break
				}
			}
			if !match {
				continue
			}
			excused := h.venvOK && projectLocalProgram(seg)
			for _, u := range h.unless {
				for _, x := range w[len(h.words):] {
					if x == u || strings.HasPrefix(x, u+"=") {
						excused = true
					}
				}
			}
			if !excused {
				return h.why
			}
		}
	}
	return ""
}

// HISTORY (2026-10-23, the same day): the first version of this check was
// fail-closed. Any program not on a localPrograms table (mkdir, cp, tar, ls,
// grep...) counted as able to reach the network: every interpreter, every
// package manager, every script by path, every unknown program. Run 2 on the
// MacBook Air (PERMISSIONS-REWORK-PLAN.md, "Evidence, run 2") showed what that
// costs: one install raised 26 dialogs after a web page was read, one per
// `python3 -m fieldkit...` and `.venv/bin/fieldkit ...`, every one inside
// ~/Stranger-Test, none risky. Under that rule all 26 would still have asked.
// The owner's direction the same night: Claude Code's model, which has no
// taint at all, is the reference; this program is allowed to be stricter only
// where data actually leaves the machine. So the table is now of SENDERS.

// networkSenders send to another machine by design. After untrusted content,
// a shell command naming one asks under auto-approve.
var networkSenders = map[string]bool{
	"ssh": true, "scp": true, "sftp": true, "rsync": true, "ftp": true, "mosh": true,
	"socat": true, "ncat": true, "open": true, "xdg-open": true, "gh": true,
	"aws": true, "gcloud": true, "az": true, "rclone": true, "s3cmd": true,
	"mail": true, "sendmail": true, "mutt": true, "smbclient": true,
}

// remoteGitSubcommands contact a remote.
var remoteGitSubcommands = map[string]bool{
	"push": true, "fetch": true, "pull": true, "clone": true, "ls-remote": true,
	"submodule": true, "send-email": true, "request-pull": true,
}

// pushSubcommands of container tools upload an image.
var pushSubcommands = map[string]bool{"docker": true, "podman": true, "buildah": true}

// inlineNetworkCode marks interpreter code passed on the command line (-c,
// -e) that names a networking library. A script FILE is not read: that is a
// residual, stated in docs/PERMISSIONS.md.
var inlineNetworkCode = regexp.MustCompile(`(?i)\b(urllib|requests|http\.client|httpx|aiohttp|socket|smtplib|ftplib|fetch\(|XMLHttpRequest|net/http|Net::HTTP|LWP|IO::Socket|Invoke-WebRequest|Invoke-RestMethod|WebClient)\b`)

// MayReachNetwork reports whether a command explicitly sends something to
// another machine: a sender program, a remote git subcommand, an image push,
// a URL anywhere in the line, or inline interpreter code naming a network
// library. Used only when the turn has read untrusted content.
func MayReachNetwork(cmd string) bool {
	if strings.Contains(cmd, "://") {
		return true
	}
	for _, seg := range splitShellCommands(stripBenignRedirects(cmd)) {
		w := segmentWords(seg)
		if len(w) == 0 {
			continue
		}
		switch {
		case networkSenders[w[0]]:
			return true
		case w[0] == "git" && len(w) > 1 && remoteGitSubcommands[w[1]]:
			return true
		case pushSubcommands[w[0]] && len(w) > 1 && w[1] == "push":
			return true
		}
		// rsync/scp-style host:path targets on any program.
		for _, x := range w[1:] {
			if i := strings.Index(x, ":"); i > 0 && strings.Contains(x, "@") && i > strings.Index(x, "@") {
				return true
			}
		}
	}
	// The splitter cuts inside quotes, so the code is checked on the whole line.
	return inlineNetworkCode.MatchString(cmd) && regexp.MustCompile(`(^|\s)-[ce](\s|$)`).MatchString(cmd)
}

// CommandPurpose is the one plain line the dialog prints above a command:
// what it will do, in words a person who cannot read the command can judge.
func CommandPurpose(cmd string) string {
	if d := DangerousPatternIn(cmd); d != nil {
		return "Cannot be undone: " + d.Why + "."
	}
	if h := HardStopIn(cmd); h != "" {
		return "Asks because " + h + "."
	}
	// GORILLA OVERRIDE (2026-10-25): see trash.go.
	if PermanentDeleteIn(cmd) != "" {
		return "Permanent delete: it " + PermanentDeleteWhy + "."
	}
	segs := splitShellCommands(cmd)
	if len(segs) == 0 {
		return ""
	}
	w := segmentWords(segs[0])
	if len(w) == 0 {
		return ""
	}
	what := "Runs " + w[0]
	if MayReachNetwork(cmd) {
		what += "; it can reach the network or run a script"
	}
	return what + "."
}

// readOnlyList is safeReadOnlyCommands plus the additions above, for the
// shell this session actually drives.
func readOnlyList() []string {
	if shell.IsWindowsPowerShell() {
		return powerShellReadOnlyList()
	}
	out := append(append([]string{}, safeReadOnlyCommands...), readOnlyAdditions...)
	return append(out, unixOnlyReadOnly...)
}

// powerShellReadOnlyList is the read-only list when the shell is PowerShell.
func powerShellReadOnlyList() []string {
	var out []string
	for _, c := range append(append([]string{}, safeReadOnlyCommands...), readOnlyAdditions...) {
		if !notInPowerShell[c] {
			out = append(out, c)
		}
	}
	return append(out, powerShellReadOnly...)
}
