package tools

// GORILLA OVERRIDE (2026-10-04): name the commands that can ruin a machine.
//
// # WHAT THIS ADDS
//
// commandgate.go answers "is this command banned?" and "is it read-only?".
// Everything else falls to one permission prompt that looks the same for
// `go build ./...` and for `curl http://x | sh`. A prompt that always looks the
// same is approved by habit, and in auto-approve mode it is not shown at all.
//
// This file recognises the small set of shapes where a wrong yes cannot be taken
// back, so the caller can (a) say so in the prompt in plain words and (b) ask
// even when auto-approve is on. It never blocks: a person who means it can still
// say yes. See permission.CreatePermissionRequest.Risk.
//
// # WHERE THE LIST CAME FROM
//
// The idea and about half the shapes are from goose's threat patterns
// (aaif-goose/goose, crates/goose/src/security/patterns.rs, Apache-2.0). The
// regular expressions are rewritten: several upstream ones have an alternation
// that escapes its group (`a.*b|c` matching any line containing c), which would
// make this fire on ordinary work. The Windows and git shapes are ours.
//
// # THE RULE THIS FILE KEEPS
//
// Every pattern is matched against the WHOLE command line, never its first
// word, for the reason commandgate.go spends a page on.

import (
	"regexp"
	"strings"
)

// DangerousCommand is one recognised shape and the sentence shown to the person.
type DangerousCommand struct {
	Name string
	Why  string
	re   *regexp.Regexp
}

func danger(name, why, pattern string) DangerousCommand {
	return DangerousCommand{Name: name, Why: why, re: regexp.MustCompile(pattern)}
}

// rmRecursive matches rm at a command position (not inside a quoted search
// string) with any run of options that includes a recursive one.
const rmRecursive = `(?:^|[;&|(]\s*|\bsudo\s+|\bthen\s+|\bdo\s+)rm\s+(?:-[a-zA-Z]+\s+|--[a-z-]+\s+)*(?:-[a-zA-Z]*[rR][a-zA-Z]*|--recursive)(?:\s+-[a-zA-Z]+|\s+--[a-z-]+)*\s+`

var dangerousCommands = []DangerousCommand{
	// --- deleting more than a project ---------------------------------------
	danger("delete-root-or-home",
		"it deletes the whole disk or your whole home folder, and that cannot be undone",
		`(?i)`+rmRecursive+`['"]?(?:/|~|\$HOME|\$\{HOME\}|/home|/root|/\*|~/\*)['"]?(?:\s|[;&|]|$)`),
	danger("windows-delete-drive-or-profile",
		"it deletes a whole drive or your whole user folder, and that cannot be undone",
		`(?i)(?:remove-item|rd|rmdir|del|erase)\b[^|;&]*(?:-recurse|/s)\b[^|;&]*\s['"]?(?:[a-z]:\\?|\$env:userprofile|%userprofile%|~)['"]?(?:\s|[;&|]|$)`),
	danger("overwrite-disk",
		"it writes directly over a disk, destroying everything on it",
		`(?i)\bdd\b[^|;&]*\bof=/dev/(?:sd|hd|nvme|mmcblk|disk)`),
	danger("format-disk",
		"it formats a disk, destroying everything on it",
		`(?i)(?:\bmkfs(?:\.[a-z0-9]+)?\s+/dev/|\bformat(?:\.com)?\s+[a-z]:|\bformat-volume\b|\bclear-disk\b|\bdiskpart\b)`),

	// --- running code fetched from the network ------------------------------
	danger("download-and-run",
		"it downloads a script from the internet and runs it without anyone reading it first",
		`(?i)\b(?:curl|wget)\b[^;&]*\|\s*(?:sudo\s+)?(?:[a-z0-9_./\\:-]*[/\\])?(?:bash|sh|zsh|dash|ksh|fish|python[23]?|perl|ruby|node)(?:\.exe)?\b`),
	danger("download-and-run",
		"it downloads a script from the internet and runs it without anyone reading it first",
		`(?i)\b(?:bash|sh|zsh)\s*<\s*\(\s*(?:curl|wget)\b`),
	danger("powershell-download-and-run",
		"it downloads a script from the internet and runs it without anyone reading it first",
		`(?i)(?:\b(?:iwr|irm|invoke-webrequest|invoke-restmethod|curl|wget)\b[^;&]*\|\s*(?:iex|invoke-expression)\b|downloadstring[^;&]*(?:iex|invoke-expression)|(?:iex|invoke-expression)\b[^;&]*(?:downloadstring|invoke-webrequest|invoke-restmethod|\biwr\b|\birm\b))`),
	danger("decode-and-run",
		"it decodes hidden text and runs it as a command, which is how a harmful command is disguised",
		`(?i)\bbase64\s+(?:-d|--decode|-D)\b[^;&]*\|\s*(?:sudo\s+)?(?:bash|sh|zsh)\b`),
	danger("decode-and-run",
		"it runs an encoded PowerShell command, which is how a harmful command is disguised",
		`(?i)\b(?:powershell|pwsh)(?:\.exe)?\b[^;&|]*\s-(?:e|ec|enc|encodedcommand)\s+[a-z0-9+/=]{16,}`),

	// --- sending credentials away, or opening a way in ----------------------
	danger("send-credentials",
		"it sends a private key, a password file or your command history to another computer",
		`(?i)\b(?:curl|wget|nc|ncat|netcat|scp|rsync)\b[^;&]*(?:\.ssh[/\\]id_[a-z0-9]+|/etc/shadow|\.aws[/\\]credentials|\.netrc|\.bash_history|\.zsh_history|\.gnupg)`),
	danger("reverse-shell",
		"it gives another computer a command line on this one",
		`(?i)(?:\b(?:nc|ncat|netcat)\b[^;&|]*\s-e\s*\S*(?:sh|bash|cmd|powershell)|/dev/tcp/[^\s/]+/\d+|\bbash\s+-i\b[^;&]*>&)`),

	// --- changes that outlive the session -----------------------------------
	danger("passwordless-sudo",
		"it lets any program become administrator without a password",
		`(?i)nopasswd[^;&]*sudoers|sudoers[^;&]*nopasswd`),
	danger("setuid",
		"it marks a program to run as administrator for every user",
		`(?i)\bchmod\s+(?:-[a-zA-Z]+\s+)*(?:[2467][0-7]{3}|[ugoa]*\+[a-z]*s[a-z]*)\b`),
	danger("kernel-module",
		"it loads or removes a kernel module, which can crash or compromise the system",
		`(?i)\b(?:insmod|rmmod)\b|\bmodprobe\s+-r\b`),
	danger("disable-protection",
		"it switches off the system's malware protection or firewall",
		`(?i)(?:set-mppreference\b[^;&|]*-disable|netsh\s+advfirewall\s+set\b[^;&|]*state\s+off|\bufw\s+disable\b|\bsetenforce\s+0\b)`),
	danger("boot-or-registry",
		"it changes how Windows starts or deletes part of the system registry",
		`(?i)(?:\bbcdedit\b[^;&|]*/(?:set|delete)|\breg(?:\.exe)?\s+delete\s+['"]?hk(?:lm|ey_local_machine)\b)`),

	// --- losing work in a repository ----------------------------------------
	danger("discard-uncommitted-work",
		"it throws away changes that were never committed, and git cannot bring them back",
		`(?i)\bgit\b[^;&|]*\b(?:reset\s+(?:[^;&|]*\s)?--hard|clean\s+(?:[^;&|]*\s)?-[a-z]*f[a-z]*|checkout\s+(?:--\s+)?\.(?:\s|$)|restore\s+(?:--[a-z-]+\s+)*\.(?:\s|$))`),
	danger("rewrite-published-history",
		"it overwrites history on the shared repository, which can destroy other people's commits",
		`(?i)\bgit\b[^;&|]*\bpush\b[^;&|]*(?:\s--force(?:\s|$)|\s-f(?:\s|$)|\s--mirror\b|\s\+[^\s]+)`),
}

// DangerousPatternIn returns the first dangerous shape found anywhere in the
// command line, or nil. The whole line is searched: a harmless first command
// does not excuse what is chained after it.
func DangerousPatternIn(cmd string) *DangerousCommand {
	if strings.TrimSpace(cmd) == "" {
		return nil
	}
	for i := range dangerousCommands {
		if dangerousCommands[i].re.MatchString(cmd) {
			return &dangerousCommands[i]
		}
	}
	// Recursive deletes are also PARSED, because a pattern only knows the
	// spellings somebody thought of. See dangerous_delete.go.
	if deletesSomethingHuge(cmd) {
		return &bigDeleteWhy
	}
	return nil
}
