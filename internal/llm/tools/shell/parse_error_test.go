package shell

// GORILLA FIX (2026-10-11): a syntax error must fail at once, with the
// parser's message, and leave the shell alive.
//
// Measured before the fix on Windows PowerShell 5.1.26100: `cd C:\x && python
// -m fieldkit --help` — three times in one Nemotron session — reported its
// parse error on the shell's own stderr at 320 ms, discarded the whole chunk
// including the status-file write, waited out the 60 s timeout, and was
// reported as "timed out". stopRunning then killed the shell, so the next
// command started in a fresh one with the session's state gone.

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/pathutil"
	"github.com/opencode-ai/opencode/internal/pathutil/pathutiltest"
)

func psOnly(t *testing.T) *PersistentShell {
	t.Helper()
	s := newTestShell(t)
	if !s.isPowerShell {
		t.Skip("PowerShell-specific: the bash wrapper has no parse-then-run step")
	}
	// GORILLA FIX (2026-10-21): the first command in a new PowerShell pays for
	// its start-up, which under a full parallel `go test ./...` took 7.6 s once
	// and failed the 5 s bound below with no bug present. Warmed here, so the
	// timings measure the parse error only.
	exec1(t, s, "Write-Output warm")
	return s
}

// parseBound is how long a parse error may take. The bug these tests guard
// against waits out exec1's whole 25 s timeout; a bound at half of it catches
// that, and is not tripped by a loaded machine.
const parseBound = 12 * time.Second

// psMajor is the major version of the PowerShell the session runs.
// resolveShellPath prefers pwsh.exe (7+) when it is installed, as on the
// windows-latest CI runner, and Windows PowerShell 5.1 otherwise.
func psMajor(t *testing.T, s *PersistentShell) int {
	t.Helper()
	out, _, _ := exec1(t, s, "$PSVersionTable.PSVersion.Major")
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("PowerShell version unreadable: %q", out)
	}
	return n
}

// GORILLA FIX (2026-10-27): `&&` is a parse error only in Windows PowerShell
// 5.1. In PowerShell 7 it is the pipeline-chain operator, so on the CI
// runner (pwsh 7 installed) `cd C:\ && echo hi` ran and printed hi, exit 0:
// correct for that shell, and this test asserted 5.1 regardless. It now
// checks what each version must do; under 7 that includes the chain
// stopping at a failure, which is what && means.
func TestAnAmpersandChainFailsAtOnceWithTheRealReason(t *testing.T) {
	s := psOnly(t)
	if v := psMajor(t, s); v >= 7 {
		stdout, stderr, code := exec1(t, s, `cd C:\ && echo hi`)
		if code != 0 || !strings.Contains(stdout, "hi") {
			t.Errorf("PowerShell %d: the chain did not run: code=%d stdout=%q stderr=%q", v, code, stdout, stderr)
		}
		stdout, _, code = exec1(t, s, `Get-Item C:\no-such-dir-xyz && echo after`)
		if code == 0 || strings.Contains(stdout, "after") {
			t.Errorf("PowerShell %d: && ran past a failure: code=%d stdout=%q", v, code, stdout)
		}
		if !s.isAlive {
			t.Fatal("the shell died on a chain")
		}
		return
	}
	start := time.Now()
	stdout, stderr, code := exec1(t, s, `cd C:\ && echo hi`)
	if d := time.Since(start); d > parseBound {
		t.Fatalf("took %s: the parse error is still waiting for the timeout", d)
	}
	if code == 0 {
		t.Errorf("exit code 0 for a command that could not be parsed; stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "&&") {
		t.Errorf("stderr does not name the offending token:\n%s", stderr)
	}
	if !strings.Contains(stderr, "separate commands with ';'") {
		t.Errorf("stderr does not tell the model what to write instead:\n%s", stderr)
	}
	if !s.isAlive {
		t.Fatal("the shell died on a parse error")
	}
	if out, _, c := exec1(t, s, "Write-Output still_alive"); !strings.Contains(out, "still_alive") || c != 0 {
		t.Errorf("the next command did not run in the same shell: out=%q code=%d", out, c)
	}
}

func TestAnUnterminatedQuoteFailsAtOnce(t *testing.T) {
	s := psOnly(t)
	start := time.Now()
	_, stderr, code := exec1(t, s, `Write-Output 'never closed`)
	if d := time.Since(start); d > parseBound {
		t.Fatalf("took %s: the parser is still waiting for the rest of the string", d)
	}
	if code == 0 || !strings.Contains(strings.ToLower(stderr), "could not parse") {
		t.Errorf("code=%d stderr=%q", code, stderr)
	}
	if out, _, _ := exec1(t, s, "Write-Output after_quote"); !strings.Contains(out, "after_quote") {
		t.Errorf("the command after the unterminated quote was swallowed: %q", out)
	}
}

func TestAThrowKeepsTheOutputBeforeItAndTheShell(t *testing.T) {
	s := psOnly(t)
	stdout, stderr, code := exec1(t, s, `Write-Output before; throw 'boom'`)
	if !strings.Contains(stdout, "before") {
		t.Errorf("output printed before the throw was lost: %q", stdout)
	}
	if code == 0 || !strings.Contains(stderr, "boom") {
		t.Errorf("the throw was not reported: code=%d stderr=%q", code, stderr)
	}
	if out, _, _ := exec1(t, s, "Write-Output after_throw"); !strings.Contains(out, "after_throw") {
		t.Errorf("the shell did not survive the throw: %q", out)
	}
}

// Set-Location inside the script block must still move the session, exactly
// as it did inside `& { }`.
func TestChangingDirectoryStillPersists(t *testing.T) {
	s := psOnly(t)
	dir := t.TempDir()
	exec1(t, s, "Set-Location '"+dir+"'")
	// PowerShell reports the folder by its long name: t.TempDir() is
	// C:\Users\RUNNER~1\... on the CI runner, and the session says
	// C:\Users\runneradmin\.... The same folder, so compared with pathutil.
	out, _, _ := exec1(t, s, "(Get-Location).Path")
	if !pathutil.Same(strings.TrimSpace(out), dir) {
		t.Errorf("cwd after Set-Location = %q, want %q", strings.TrimSpace(out), dir)
	}
	if !pathutil.Same(s.cwd, dir) {
		t.Errorf("tracked cwd = %q, want %q", s.cwd, dir)
	}
}

// The command travels base64-encoded: non-ASCII, quotes of both kinds and
// newlines must all come out the other side intact.
func TestTheCommandSurvivesTheEncoding(t *testing.T) {
	s := psOnly(t)
	cmd := "Write-Output 'äöüßé — \"double\" and ''single'''\nWrite-Output second_line"
	out, stderr, code := exec1(t, s, cmd)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"äöüßé", `"double"`, "'single'", "second_line"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q: %q", want, out)
		}
	}
}

func TestANativeExitCodeIsStillTheCommandsOwn(t *testing.T) {
	s := psOnly(t)
	if _, _, code := exec1(t, s, `cmd /c "exit 9"`); code != 9 {
		t.Errorf("exit code = %d, want 9", code)
	}
	if _, _, code := exec1(t, s, `Write-Output ok`); code != 0 {
		t.Errorf("a later success inherited an exit code: %d", code)
	}
}

// The CI case on every machine: the session moved to a folder by its 8.3
// short name still tracks that folder (PowerShell reports the long name).
func TestChangingDirectoryShortPathStillPersists(t *testing.T) {
	s := psOnly(t)
	long := pathutiltest.LongDir(t)
	alias, kind := pathutiltest.ShortAlias(t, long)
	exec1(t, s, "Set-Location '"+alias+"'")
	if !pathutil.Same(s.cwd, long) {
		t.Errorf("moved to %s (%s of %s); tracked cwd = %q", alias, kind, long, s.cwd)
	}
	if out, _, _ := exec1(t, s, "(Get-Location).Path"); !pathutil.Same(strings.TrimSpace(out), long) {
		t.Errorf("the session is in %q", strings.TrimSpace(out))
	}
}
