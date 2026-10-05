package tools

// GORILLA OVERRIDE (2026-10-05): recursive deletes are judged by PARSING the
// command, not by a pattern over its text.
//
// The two patterns in dangerous.go (delete-root-or-home and
// windows-delete-drive-or-profile) expected one spelling each. An audit listed
// what walked past them, every one auto-approved in an unattended run, where
// this check is the only refusal there is:
//
//	Remove-Item C:\ -Recurse -Force        the path before the flags
//	Remove-Item -Recurse -Force C:\Users\gorilla1     the profile, written out
//	rd /s /q C:\Users\gorilla1
//	Remove-Item -Recurse $HOME             del /s /q C:\*
//	rm -rf ~/      rm -rf $HOME/      rm -rf /home/me      rm -rf -- /
//
// A pattern has to anticipate word order, quoting and trailing slashes. A parse
// does not: find the delete command, see whether it is recursive, then look at
// each thing it is pointed at, wherever on the line it sits.

import (
	"os"
	"path/filepath"
	"strings"
)

var deleteVerbs = map[string]bool{
	"rm": true, "remove-item": true, "ri": true, "rd": true, "rmdir": true, "del": true, "erase": true,
}

var bigDeleteWhy = DangerousCommand{
	Name: "delete-root-or-home",
	Why:  "it deletes a whole disk, the system, or a whole user folder, and that cannot be undone",
}

// deletesSomethingHuge reports a recursive delete aimed at a disk, a system
// folder, the folder that holds every user, or a user's own folder.
func deletesSomethingHuge(cmd string) bool {
	for _, seg := range splitShellCommands(cmd) {
		words := shellWords(seg)
		// Skip what may stand in front of the command word.
		for len(words) > 0 {
			w := strings.ToLower(words[0])
			if w == "sudo" || w == "then" || w == "do" || w == "command" || w == "&" || strings.Contains(w, "=") {
				words = words[1:]
				continue
			}
			break
		}
		if len(words) < 2 {
			continue
		}
		verb := strings.TrimSuffix(strings.ToLower(filepath.Base(strings.ReplaceAll(words[0], `\`, "/"))), ".exe")
		if !deleteVerbs[verb] {
			continue
		}
		recursive := false
		var targets []string
		for _, w := range words[1:] {
			low := strings.ToLower(w)
			switch {
			case low == "--":
			case low == "/s" || low == "-recurse" || low == "-r" || low == "--recursive" || strings.HasPrefix(low, "-rec"):
				recursive = true
			case strings.HasPrefix(low, "--"):
			case strings.HasPrefix(low, "-") && !strings.ContainsAny(low, `/\.`):
				// A bundle of short options: -rf, -fr, -Rf.
				if verb == "rm" && strings.ContainsAny(w, "rR") {
					recursive = true
				}
			case len(low) == 2 && low[0] == '/':
				// A cmd.exe switch such as /q.
			default:
				targets = append(targets, w)
			}
		}
		if !recursive {
			continue
		}
		for _, t := range targets {
			if isHugeTarget(t) {
				return true
			}
		}
	}
	return false
}

// shellWords splits on spaces, keeping a quoted run together and dropping the
// quotes. Crude on purpose: it only has to find words.
func shellWords(s string) []string {
	var out []string
	var cur strings.Builder
	quote := rune(0)
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t' || r == ',':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// isHugeTarget judges one path argument. It is deliberately about the SHAPE of
// the path and never touches the disk.
func isHugeTarget(t string) bool {
	p := strings.ToLower(strings.TrimSpace(t))
	p = strings.ReplaceAll(p, `\`, "/")
	// "everything inside X" is X.
	for strings.HasSuffix(p, "/*") || strings.HasSuffix(p, "/.") || (strings.HasSuffix(p, "/") && len(p) > 1) {
		p = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(p, "*"), "."), "/")
		if p == "" {
			return true // it was "/", "/*" or "/."
		}
	}
	if p == "*" {
		return false // everything in the CURRENT folder: a project clean, not this
	}
	// Spellings of "my home folder".
	switch p {
	case "/", "~", "$home", "${home}", "$env:userprofile", "${env:userprofile}", "%userprofile%",
		"$env:homepath", "%homepath%", "%homedrive%", "$env:systemdrive", "%systemdrive%",
		"$env:systemroot", "%systemroot%", "$env:windir", "%windir%", "$env:programfiles", "%programfiles%":
		return true
	}
	// A drive: "c:".
	if len(p) == 2 && p[1] == ':' && p[0] >= 'a' && p[0] <= 'z' {
		return true
	}
	if len(p) > 2 && p[1] == ':' {
		p = p[2:] // judge the rest like a rooted path
	}
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if !strings.HasPrefix(p, "/") {
		return false // relative: inside wherever the command runs
	}
	top := parts[0]
	switch top {
	case "home", "users":
		if len(parts) <= 2 {
			return true // /home, /home/me, C:\Users, C:\Users\me
		}
		// The folders inside a home that hold a person's own things.
		if len(parts) == 3 {
			switch parts[2] {
			case "documents", "desktop", "downloads", "pictures", "music", "videos", "onedrive",
				"appdata", ".ssh", ".config", ".gnupg", ".local", "library":
				return true
			}
		}
		return false
	case "root", "etc", "usr", "var", "bin", "sbin", "lib", "lib64", "boot", "opt", "sys", "proc", "dev",
		"windows", "program files", "program files (x86)", "programdata", "system32", "library", "system", "applications":
		return len(parts) == 1
	}
	// This machine's real home, however it is spelled, and the folder above it.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		h := strings.ToLower(strings.ReplaceAll(home, `\`, "/"))
		if len(h) > 2 && h[1] == ':' {
			h = h[2:]
		}
		h = strings.TrimRight(h, "/")
		if p == h || p == filepath.ToSlash(filepath.Dir(h)) {
			return true
		}
	}
	return false
}
