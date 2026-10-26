// Test names end in a lowercase token (_utf, _stdin ...): `go test -run`
// splits a pattern at each top-level "|" and compiles every part on its own,
// so the goal check's leading (?i) covers only its first alternative.
package shell

import (
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestExecutionPolicyBypassArgs: every PowerShell we start bypasses the
// execution policy for its own process (gap #3) and never changes it.
func TestExecutionPolicyBypassArgs_executionpolicy(t *testing.T) {
	args := PowerShellArgs("-Command", "-")
	i := slices.Index(args, "-ExecutionPolicy")
	if i < 0 || i+1 >= len(args) || args[i+1] != "Bypass" {
		t.Fatalf("PowerShellArgs = %q, want -ExecutionPolicy Bypass", args)
	}
	if args[len(args)-2] != "-Command" || args[len(args)-1] != "-" {
		t.Errorf("PowerShellArgs lost the caller's arguments: %q", args)
	}
	if strings.Contains(psSessionSetup, "Set-ExecutionPolicy") {
		t.Error("the session setup changes the machine's execution policy")
	}
}

// TestExecutionPolicyLiveSession: in the shell the bash tool drives, the
// effective policy is Bypass, so a .ps1 runs even where the machine says
// Restricted.
func TestExecutionPolicyLiveSession_executionpolicy(t *testing.T) {
	s := newTestShell(t)
	if !s.isPowerShell {
		t.Skip("PowerShell only")
	}
	out, _, _ := exec1(t, s, "Get-ExecutionPolicy -Scope Process")
	if strings.TrimSpace(out) != "Bypass" {
		t.Errorf("process execution policy = %q, want Bypass", strings.TrimSpace(out))
	}
	ps1 := t.TempDir() + `\hello.ps1`
	if err := os.WriteFile(ps1, []byte("Write-Output 'script ran'\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := exec1(t, s, "& '"+ps1+"'")
	if code != 0 || !strings.Contains(out, "script ran") {
		t.Errorf("a .ps1 did not run: code %d, out %q, err %q", code, out, errOut)
	}
}

// TestUTF8ChildEnv: Python children write UTF-8 (gap #4); a value the user
// set is kept; nothing is added outside Windows.
func TestUTF8ChildEnv_utf(t *testing.T) {
	env := shellChildEnv([]string{"PATH=x"}, true)
	for _, want := range []string{"PYTHONUTF8=1", "PYTHONIOENCODING=utf-8"} {
		if !slices.Contains(env, want) {
			t.Errorf("child env %q lacks %s", env, want)
		}
	}
	kept := shellChildEnv([]string{"PYTHONIOENCODING=latin-1"}, true)
	if slices.Contains(kept, "PYTHONIOENCODING=utf-8") {
		t.Errorf("the user's PYTHONIOENCODING was overridden: %q", kept)
	}
	if got := shellChildEnv([]string{"PATH=x"}, false); len(got) != 1 {
		t.Errorf("non-Windows env changed: %q", got)
	}
}

// TestUTF8NativeOutputIsNotMojibake: git and Python print non-ASCII
// through the PowerShell session intact. Before the fix git's UTF-8 came
// back decoded as the OEM code page ("cafÃ©") and Python wrote cp1252.
func TestUTF8NativeOutputIsNotMojibake_utf(t *testing.T) {
	s := newTestShell(t)
	if !s.isPowerShell {
		t.Skip("PowerShell only")
	}
	const word = "café 日本 Ωmega"
	ran := false
	if _, err := exec.LookPath("git"); err == nil {
		ran = true
		out, errOut, code := exec1(t, s,
			"git init -q . ; git -c user.name=t -c user.email=t@t commit -q --allow-empty -m '"+word+"' ; git log -1 --format=%s")
		if code != 0 || !strings.Contains(out, word) {
			t.Errorf("git log printed %q (code %d, err %q), want %q", out, code, errOut, word)
		}
	}
	if py, err := exec.LookPath("python"); err == nil && !strings.Contains(strings.ToLower(py), `\windowsapps\`) {
		ran = true
		out, errOut, code := exec1(t, s, `python -c "print('café 日本 Ωmega')"`)
		if code != 0 || !strings.Contains(out, word) {
			t.Errorf("python printed %q (code %d, err %q), want %q", out, code, errOut, word)
		}
	}
	if !ran {
		t.Skip("neither git nor python on PATH")
	}
}

// TestStdinNativeReaderDoesNotEatCommands (gap #5): a native program that
// reads stdin gets end-of-file, not the shell's command stream. Before the
// fix findstr blocked on the pipe until the timeout, and python read the
// next commands as its own input.
func TestStdinNativeReaderDoesNotEatCommands_stdin(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the POSIX wrapper has < /dev/null; see TestACommandThatReadsStdinDoesNotEatTheSession")
	}
	s := newTestShell(t)
	if !s.isPowerShell {
		t.Skip("PowerShell only")
	}
	_, _, code := exec1(t, s, `findstr /R "x"`)
	if code == 0 {
		t.Errorf("findstr found a match on what should be an empty stdin")
	}
	out, _, _ := exec1(t, s, "Write-Output survived")
	if !strings.Contains(out, "survived") {
		t.Errorf("the next command returned %q after a stdin reader", out)
	}
	// Pipeline input into a native program still works: only the inherited
	// handle is swapped, not PowerShell's own piping.
	out, _, _ = exec1(t, s, `"abc","xyz" | findstr x`)
	if strings.TrimSpace(out) != "xyz" {
		t.Errorf("piped input to findstr = %q, want xyz", out)
	}
}
