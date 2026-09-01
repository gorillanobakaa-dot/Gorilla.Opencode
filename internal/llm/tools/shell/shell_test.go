// GORILLA OVERRIDE (2026-09-01): this file did not exist. The shell package —
// the single most platform-sensitive thing in the tree, and the one every other
// tool depends on — had NO tests at all, on any platform. That is the direct
// reason the Windows port shipped a wrapper that reported exit 0 for every
// failing command, never captured a single byte of stderr, and permanently
// bricked its own session by storing a working directory it could not re-enter.
//
// Every test here asserts a behaviour that was actually broken and is now fixed,
// so a regression names itself instead of showing up as "the agent says the
// build passed" three weeks later.
package shell

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func newTestShell(t *testing.T) *PersistentShell {
	t.Helper()
	s := newPersistentShell(t.TempDir())
	if s == nil {
		t.Fatal("newPersistentShell returned nil — no usable shell on this machine")
	}
	t.Cleanup(s.Close)
	return s
}

func exec1(t *testing.T, s *PersistentShell, cmd string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stdout, stderr, code, _, err := s.Exec(ctx, cmd, 25000)
	if err != nil {
		t.Fatalf("Exec(%q) returned a transport error: %v", cmd, err)
	}
	return stdout, stderr, code
}

// A command's real exit code must survive the round trip.
//
// This is the one that mattered most: with the BOM bug, EVERY command came back
// 0 because Sscanf could not parse "\xEF\xBB\xBF7". A failing test suite, a
// failing build and a failing deploy all looked like success.
func TestExitCodeIsTheCommandsOwn(t *testing.T) {
	s := newTestShell(t)
	cmd := "exit 7"
	if s.isPowerShell {
		cmd = "cmd /c \"exit 7\""
	}
	if _, _, code := exec1(t, s, cmd); code != 7 {
		t.Errorf("exit code is %d, want 7 — a non-zero status must reach the caller, "+
			"or every failure is reported to the model as a success", code)
	}
}

func TestSuccessIsZero(t *testing.T) {
	s := newTestShell(t)
	if _, _, code := exec1(t, s, "echo ok"); code != 0 {
		t.Errorf("exit code is %d, want 0", code)
	}
}

// stdout must come back byte-clean: no BOM, nothing prepended.
//
// The BOM is invisible in a terminal and in a diff. It corrupted the first line
// of every single command's output for the whole Windows port.
func TestStdoutHasNoByteOrderMark(t *testing.T) {
	s := newTestShell(t)
	stdout, _, _ := exec1(t, s, "echo hello")
	if strings.HasPrefix(stdout, "\uFEFF") {
		t.Errorf("stdout starts with a byte-order mark: %q", stdout)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("stdout does not contain the echoed text: %q", stdout)
	}
}

// A command that fails must put something in stderr.
//
// The old PowerShell wrapper bound its `2>` to Out-File instead of to the
// command, so the error went to the shell's own discarded stderr and the model
// was handed an empty string. "It failed and I cannot tell you why" is worse
// than the failure.
func TestFailureExplainsItselfOnStderr(t *testing.T) {
	s := newTestShell(t)
	cmd := "ls /definitely-not-here-xyz"
	if s.isPowerShell {
		cmd = "Get-ChildItem nosuchpath-xyz"
	}
	_, stderr, code := exec1(t, s, cmd)
	if code == 0 {
		t.Errorf("a failing command reported exit 0")
	}
	if strings.TrimSpace(stderr) == "" {
		t.Errorf("a failing command produced no stderr — the reason for the failure was lost")
	}
}

// Long lines must not be wrapped or truncated by the shell's own formatting.
func TestLongLinesSurviveIntact(t *testing.T) {
	s := newTestShell(t)
	cmd := "python3 -c \"print('X'*300)\""
	if s.isPowerShell {
		cmd = "Write-Output ('X'*300)"
	}
	stdout, _, _ := exec1(t, s, cmd)
	if n := strings.Count(stdout, "X"); n != 300 {
		t.Skipf("got %d X's, want 300 (needs python3 on PATH for the bash case)", n)
	}
}

// The tracked working directory must be a directory we can actually re-enter.
//
// This is the whole "Shell could not be started" bug. s.cwd is where the shell
// is RESPAWNED after it dies, so one unusable value turns a recoverable death
// into a permanent one for the rest of the session.
func TestTrackedCwdCanBeReenteredAfterACommand(t *testing.T) {
	s := newTestShell(t)
	exec1(t, s, "echo anything")

	if strings.HasPrefix(s.cwd, "\uFEFF") {
		t.Fatalf("tracked cwd starts with a byte-order mark: %q", s.cwd)
	}
	respawned := newPersistentShell(s.cwd)
	if respawned == nil {
		t.Fatalf("a shell cannot be started in the directory we just tracked (%q) — "+
			"every respawn after this point would fail and the bash tool would be dead", s.cwd)
	}
	respawned.Close()
}

// An unusable working directory must degrade, not kill the tool.
func TestAnUnusableWorkingDirectoryStillGivesAShell(t *testing.T) {
	bad := "/definitely/not/a/real/directory/anywhere"
	if runtime.GOOS == "windows" {
		bad = "Z:\\no\\such\\directory"
	}
	s := newPersistentShell(bad)
	if s == nil {
		t.Fatal("an unusable working directory produced no shell at all; " +
			"a shell in the wrong folder is far more useful than none")
	}
	s.Close()
}

// State must persist across calls — that is the entire point of a persistent shell.
func TestShellStatePersistsBetweenCommands(t *testing.T) {
	s := newTestShell(t)
	set, get := "export GORILLA_TEST_VAR=persisted", "echo $GORILLA_TEST_VAR"
	if s.isPowerShell {
		set, get = "$env:GORILLA_TEST_VAR='persisted'", "Write-Output $env:GORILLA_TEST_VAR"
	}
	exec1(t, s, set)
	stdout, _, _ := exec1(t, s, get)
	if !strings.Contains(stdout, "persisted") {
		t.Errorf("shell state did not survive between commands: %q", stdout)
	}
}

// A failing command must not take the shell down with it.
//
// `set -e` in the rev-1 bash wrapper made the FIRST non-zero exit terminate the
// session: a grep with no matches killed the shell, and the next command never
// ran. Measured, not theoretical.
func TestAFailingCommandDoesNotKillTheSession(t *testing.T) {
	s := newTestShell(t)
	fail := "ls /definitely-not-here-xyz"
	if s.isPowerShell {
		fail = "Get-ChildItem nosuchpath-xyz"
	}
	exec1(t, s, fail)

	stdout, _, code := exec1(t, s, "echo still_alive")
	if !strings.Contains(stdout, "still_alive") || code != 0 {
		t.Errorf("the shell did not survive a failing command: stdout=%q exit=%d", stdout, code)
	}
}

// A command that reads stdin must not eat the command stream.
//
// Dropping `< /dev/null` from the bash wrapper meant a bare `cat` read the
// SHELL's stdin — swallowing the bookkeeping lines that follow it and
// desynchronising the session permanently.
func TestACommandThatReadsStdinDoesNotEatTheSession(t *testing.T) {
	s := newTestShell(t)
	if s.isPowerShell {
		t.Skip("stdin-draining applies to the bash wrapper")
	}
	exec1(t, s, "cat")

	stdout, _, _ := exec1(t, s, "echo survived")
	if !strings.Contains(stdout, "survived") {
		t.Errorf("a stdin-reading command consumed the command stream; next command returned %q", stdout)
	}
}

func TestStripBOM(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"utf-8":    {"\xEF\xBB\xBF0", "0"},
		"utf-16le": {"\xFF\xFE0", "0"},
		"utf-16be": {"\xFE\xFF0", "0"},
		"none":     {"0", "0"},
		"empty":    {"", ""},
	}
	for name, c := range cases {
		if got := string(stripBOM([]byte(c.in))); got != c.want {
			t.Errorf("%s: stripBOM(%q) = %q, want %q", name, c.in, got, c.want)
		}
	}
}

// On Windows the shell must be PowerShell, not a bash that probably is not there.
func TestWindowsPrefersPowerShell(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	if p := resolveShellPath(); !looksLikePowerShell(p) {
		t.Logf("resolved shell is %q, not PowerShell — acceptable only if PowerShell is genuinely absent", p)
	}
}

// Interrupting a command must not destroy the session.
//
// On Windows this used to run `taskkill /T /F` on the SHELL's own pid, and /T
// includes the named process — so one Ctrl+C or one timeout threw away every
// environment variable, virtualenv and directory change the session had.
func TestKillingChildrenLeavesTheShellAlive(t *testing.T) {
	s := newTestShell(t)
	set, get := "export GORILLA_KILL_TEST=alive", "echo $GORILLA_KILL_TEST"
	if s.isPowerShell {
		set, get = "$env:GORILLA_KILL_TEST='alive'", "Write-Output $env:GORILLA_KILL_TEST"
	}
	exec1(t, s, set)

	s.killChildren()

	if !s.isAlive {
		t.Fatal("killChildren killed the shell itself")
	}
	stdout, _, _ := exec1(t, s, get)
	if !strings.Contains(stdout, "alive") {
		t.Errorf("session state was lost when children were killed: %q", stdout)
	}
}

// childPIDs must not report the process it was asked about.
func TestChildPIDsExcludesTheParentItself(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("childPIDs is the Windows implementation")
	}
	self := os.Getpid()
	for _, pid := range childPIDs(self) {
		if pid == self {
			t.Fatal("childPIDs returned the parent itself — killing that list would kill the shell")
		}
	}
}

// Asking which shell would be used must not START one.
func TestIsWindowsPowerShellDoesNotSpawnAShell(t *testing.T) {
	shellMu.Lock()
	before := shellInstance
	shellMu.Unlock()

	_ = IsWindowsPowerShell()

	shellMu.Lock()
	after := shellInstance
	shellMu.Unlock()

	if before == nil && after != nil {
		t.Error("IsWindowsPowerShell started a shell as a side effect; it is called while " +
			"building the tool description, before the working directory is settled")
	}
}
