// GORILLA OVERRIDE: this file did not exist upstream.
//
// AGENTS.md is a real gap and the most dangerous three-line change available.
//
// THE GAP: defaultContextPaths listed .cursorrules, .cursor/rules/, CLAUDE.md
// and SIX case-variants of opencode.md — and not AGENTS.md, which was
// formalised in August 2025, donated to the Linux Foundation's Agentic AI
// Foundation that December, and is honoured by Codex, Cursor, Gemini CLI,
// Copilot, VS Code, Devin, Amp, Factory and Jules across 60,000+ repositories.
// So this tool read a competitor's proprietary dotfile and three spellings of
// its own name, but not the file most repositories actually use. A user opening
// a mainstream checkout silently got no project instructions, and silence and
// success looked identical.
//
// THE DANGER: a context file is spliced into the system prompt automatically,
// on entering a directory, before the user has typed anything, with no tool
// call and no permission prompt. `git clone && cd` therefore becomes prompt
// injection. The CLAUDE.md paths already have this shape, but they are at
// least this project's own convention; AGENTS.md is a PUBLIC STANDARD an
// attacker knows is honoured.
//
// So it is added with guards rather than added plainly.
package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AgentsFile is the open-standard project instructions filename.
//
// It is ordered AHEAD of .cursorrules in defaultContextPaths so the open
// standard wins when a repository carries both.
const AgentsFile = "AGENTS.md"

// ProjectInstructionsVerdict is why a project instructions file was or was not
// loaded. It exists so the answer can be SHOWN rather than only acted on: a
// file that silently did not load is indistinguishable from a file that is not
// there, which is the failure this whole file is about.
type ProjectInstructionsVerdict struct {
	Path    string
	Bytes   int
	Loaded  bool
	Reason  string
	Remote  string
	IsOwned bool
}

// AutoLoadProjectInstructions decides whether dir's AGENTS.md may be spliced
// into the system prompt without asking.
//
// The rule: a repository you wrote is your own words; a repository you cloned
// is a stranger's. Ownership is inferred from the origin remote against the
// local git identity — imperfect, and deliberately biased toward NOT loading,
// because the cost of wrongly skipping is that the user reads a notice and
// copies a file, while the cost of wrongly loading is a stranger's text in the
// system prompt.
//
// A directory that is not a git repository at all counts as owned: it is
// somewhere the user made, not somewhere they cloned.
func AutoLoadProjectInstructions(dir string) ProjectInstructionsVerdict {
	v := ProjectInstructionsVerdict{Path: filepath.Join(dir, AgentsFile)}

	info, err := os.Stat(v.Path)
	if err != nil || info.IsDir() {
		v.Reason = "no " + AgentsFile + " here"
		return v
	}
	v.Bytes = int(info.Size())

	remote, ok := originRemote(dir)
	if !ok {
		v.IsOwned = true
		v.Loaded = true
		v.Reason = "not a git checkout — this is a directory you made"
		return v
	}
	v.Remote = remote

	if owner, ok := remoteOwner(remote); ok && ownerLooksLikeUs(dir, owner) {
		v.IsOwned = true
		v.Loaded = true
		v.Reason = "origin belongs to you (" + owner + ")"
		return v
	}

	v.Reason = "this is someone else's repository (" + remote + "), and " + AgentsFile +
		" would go straight into the model's instructions without being asked about"
	return v
}

func originRemote(dir string) (string, bool) {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	// A repository with no origin, or no git installed, is not an error worth
	// reporting — it just means ownership cannot be established this way.
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// remoteOwner pulls the account segment out of the common remote spellings:
// https://host/owner/repo(.git) and git@host:owner/repo(.git).
func remoteOwner(remote string) (string, bool) {
	r := strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	if i := strings.Index(r, "@"); i >= 0 && !strings.Contains(r[:i], "/") {
		r = r[i+1:] // git@github.com:owner/repo
	}
	if i := strings.Index(r, "://"); i >= 0 {
		r = r[i+3:]
	}
	r = strings.ReplaceAll(r, ":", "/")
	parts := strings.Split(strings.Trim(r, "/"), "/")
	if len(parts) < 3 {
		return "", false
	}
	// parts[0] is the host; the account is next.
	return parts[len(parts)-2], true
}

// ownerLooksLikeUs compares the remote's account against the local git
// identity. Substring matching in BOTH directions, case-folded, because
// accounts and names diverge in small ways ("gorillanobakaa-dot" against
// "gorillanobakaa@gmail.com") that an equality test would miss.
func ownerLooksLikeUs(dir, owner string) bool {
	owner = strings.ToLower(owner)
	if owner == "" {
		return false
	}
	for _, key := range []string{"user.name", "user.email"} {
		out, err := exec.Command("git", "-C", dir, "config", "--get", key).Output()
		if err != nil {
			continue
		}
		id := strings.ToLower(strings.TrimSpace(string(out)))
		if id == "" {
			continue
		}
		if i := strings.Index(id, "@"); i > 0 {
			id = id[:i]
		}
		id = strings.NewReplacer(" ", "", "-", "", "_", "", ".", "").Replace(id)
		flat := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "").Replace(owner)
		if id == "" || flat == "" {
			continue
		}
		if strings.Contains(flat, id) || strings.Contains(id, flat) {
			return true
		}
	}
	return false
}
