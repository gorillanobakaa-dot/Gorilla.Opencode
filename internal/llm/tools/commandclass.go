// Version: 1.0.0 · updated 26-10-23-02-25
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
// (2026-10-05, see bash.go).
var unixOnlyReadOnly = []string{"type"}

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
		if home != "" && (w == "~" || strings.HasPrefix(w, "~/") || strings.HasPrefix(w, "$HOME")) {
			w = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(w, "$HOME"), "~"))
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
	{words: []string{"pip", "install"}, unless: []string{"--user", "--target", "-t", "--prefix"}, why: "it installs Python packages outside a project (pip)"},
	{words: []string{"pip3", "install"}, unless: []string{"--user", "--target", "-t", "--prefix"}, why: "it installs Python packages outside a project (pip3)"},
	{words: []string{"*", "-m", "pip", "install"}, unless: []string{"--user", "--target", "-t", "--prefix"}, why: "it installs Python packages outside a project (pip)"},
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
			excused := false
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

// localPrograms change only local files and start nothing else: after
// untrusted content they still run under auto-approve. Anything NOT here is
// treated as able to reach the network (fail closed): an interpreter, a
// package manager, ssh, make, a script by path, a program this list does not
// know. A second program reached through an argument (find -exec, xargs) is
// caught by unsafeFlags or is simply not on this list.
var localPrograms = map[string]bool{
	"mkdir": true, "cp": true, "mv": true, "touch": true, "rm": true, "rmdir": true,
	"ln": true, "chmod": true, "chown": true, "tar": true, "unzip": true, "zip": true,
	"gzip": true, "gunzip": true, "xz": true, "bzip2": true, "cd": true, "pwd": true,
	"echo": true, "true": true, "false": true, "export": true, "ls": true, "cat": true,
	"head": true, "tail": true, "wc": true, "sort": true, "uniq": true, "cut": true,
	"tr": true, "diff": true, "patch": true, "mktemp": true, "test": true, "[": true,
	"grep": true, "find": true, "stat": true, "file": true, "du": true, "df": true,
	"which": true, "basename": true, "dirname": true, "realpath": true, "date": true,
}

// runsAProgram are flags by which a program on localPrograms starts another
// program: tar's --to-command and checkpoint actions, a compressor named with
// -I, zip's test command, find's -exec family.
var runsAProgram = []string{
	"--to-command", "--checkpoint-action", "--use-compress-program", "-I",
	"-TT", "--unzip-command", "-exec", "-execdir", "-ok", "-okdir",
}

// localGitSubcommands are the git subcommands that never contact a remote.
var localGitSubcommands = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true, "add": true, "commit": true,
	"checkout": true, "switch": true, "restore": true, "stash": true, "init": true,
	"mv": true, "rm": true, "merge": true, "rebase": true, "reset": true, "branch": true,
	"tag": true, "rev-parse": true, "ls-files": true, "blame": true, "grep": true,
	"config": true, "describe": true, "cherry-pick": true, "revert": true, "clean": true,
}

// MayReachNetwork reports whether a command can send data off the machine or
// run code that can. Used only when the turn has read untrusted content: that
// is the shell's exfiltration path, and the one place taint still makes the
// shell ask under auto-approve.
func MayReachNetwork(cmd string) bool {
	if strings.Contains(cmd, "://") || hasOpaqueConstruct(stripBenignRedirects(cmd)) {
		return true
	}
	for _, seg := range splitShellCommands(stripBenignRedirects(cmd)) {
		w := segmentWords(seg)
		if len(w) == 0 {
			continue
		}
		if w[0] == "git" {
			if len(w) < 2 || !localGitSubcommands[w[1]] {
				return true
			}
			continue
		}
		if !localPrograms[w[0]] {
			return true
		}
		for _, x := range w[1:] {
			for _, f := range runsAProgram {
				if x == f || strings.HasPrefix(x, f+"=") {
					return true
				}
			}
		}
	}
	return false
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
	out := append(append([]string{}, safeReadOnlyCommands...), readOnlyAdditions...)
	if !shell.IsWindowsPowerShell() {
		out = append(out, unixOnlyReadOnly...)
	}
	return out
}
