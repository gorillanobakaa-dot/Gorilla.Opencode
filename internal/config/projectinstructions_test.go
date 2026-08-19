package config

// GORILLA OVERRIDE (2026-08-19): AGENTS.md is a real gap and the most
// dangerous three-line change available, so it ships with guards and the
// guards ship with tests.
//
// The asymmetry that decides every case below: wrongly SKIPPING costs the user
// a notice and a copy-paste; wrongly LOADING puts a stranger's text into the
// model's instructions before anyone has typed anything. So the gate is biased
// toward not loading, and the tests assert that bias.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRepo(t *testing.T, remote, name, email string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if remote != "" {
		run("remote", "add", "origin", remote)
	}
	if name != "" {
		run("config", "user.name", name)
	}
	if email != "" {
		run("config", "user.email", email)
	}
	if err := os.WriteFile(filepath.Join(dir, AgentsFile), []byte("# rules\nBe nice.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOwnRepositoryLoadsItsInstructions(t *testing.T) {
	dir := gitRepo(t, "https://github.com/gorillanobakaa-dot/Gorilla.Opencode",
		"gorillanobakaa-dot", "gorillanobakaa@gmail.com")
	v := AutoLoadProjectInstructions(dir)
	if !v.Loaded {
		t.Fatalf("own repository was refused: %s", v.Reason)
	}
	if v.Bytes == 0 {
		t.Error("byte count not reported — guard 2 needs it to tell the user what was loaded")
	}
}

// git clone && cd must not be prompt injection.
func TestSomeoneElsesRepositoryDoesNotLoadAutomatically(t *testing.T) {
	dir := gitRepo(t, "https://github.com/attacker/totally-normal-lib",
		"gorillanobakaa-dot", "gorillanobakaa@gmail.com")
	v := AutoLoadProjectInstructions(dir)
	if v.Loaded {
		t.Fatal("a cloned stranger's AGENTS.md was spliced into the system prompt with no prompt and no tool call")
	}
	if v.Bytes == 0 {
		t.Error("the refusal did not report the size; the user cannot judge what they are missing")
	}
	if v.Reason == "" {
		t.Error("refused without saying why — a file that silently did not load is indistinguishable from one that is not there")
	}
}

// A directory you made is not a directory you cloned.
func TestANonGitDirectoryCountsAsYours(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, AgentsFile), []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v := AutoLoadProjectInstructions(dir)
	if !v.Loaded {
		t.Fatalf("a plain directory was treated as untrusted: %s", v.Reason)
	}
}

func TestNoFileIsNotAnError(t *testing.T) {
	v := AutoLoadProjectInstructions(t.TempDir())
	if v.Loaded || v.Bytes != 0 {
		t.Fatalf("reported something for a directory with no %s: %+v", AgentsFile, v)
	}
}

func TestRemoteOwnerParsesTheCommonSpellings(t *testing.T) {
	for _, tc := range []struct{ remote, want string }{
		{"https://github.com/owner/repo.git", "owner"},
		{"https://github.com/owner/repo", "owner"},
		{"git@github.com:owner/repo.git", "owner"},
		{"ssh://git@gitlab.com/group/repo.git", "group"},
	} {
		got, ok := remoteOwner(tc.remote)
		if !ok || got != tc.want {
			t.Errorf("remoteOwner(%q) = %q, %v; want %q", tc.remote, got, ok, tc.want)
		}
	}
}

// The identity match is fuzzy on purpose — accounts and git identities diverge
// in small ways an equality test would miss — but it must not be so fuzzy that
// it matches anyone.
func TestIdentityMatchingIsNotJustAnythingGoes(t *testing.T) {
	dir := gitRepo(t, "https://github.com/microsoft/vscode", "gorillanobakaa-dot", "gorillanobakaa@gmail.com")
	if v := AutoLoadProjectInstructions(dir); v.Loaded {
		t.Fatal("an unrelated account matched the local git identity")
	}
}
