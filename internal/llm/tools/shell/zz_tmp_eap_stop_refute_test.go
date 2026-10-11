package shell

// TEMPORARY refutation probe for the $ErrorActionPreference='Stop' claim.
// Delete after use.

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func zzEapRun(t *testing.T, s *PersistentShell, cmd string) (string, string, int, bool, time.Duration) {
	t.Helper()
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stdout, stderr, code, interrupted, err := s.Exec(ctx, cmd, 15000)
	el := time.Since(start)
	t.Logf("CMD %q\n  elapsed=%s code=%d interrupted=%v err=%v alive=%v\n  stdout=%q\n  stderr=%q",
		cmd, el, code, interrupted, err, s.isAlive, stdout, stderr)
	return stdout, stderr, code, interrupted, el
}

// Baseline: the same native command WITHOUT the preference change.
func TestZZEap_BaselineNoStop(t *testing.T) {
	s := psOnly(t)
	out, _, code, _, _ := zzEapRun(t, s, `cmd /c "echo err 1>&2 & echo out & exit 0"; Write-Output after`)
	if code != 0 || !strings.Contains(out, "out") || !strings.Contains(out, "after") {
		t.Errorf("BASELINE BROKEN: code=%d out=%q", code, out)
	}
}

// The claim's exact failing input.
func TestZZEap_ExactClaimInput(t *testing.T) {
	s := psOnly(t)
	out, stderr, code, interrupted, el := zzEapRun(t, s, `$ErrorActionPreference='Stop'; cmd /c "echo err 1>&2 & echo out & exit 0"; Write-Output after`)
	t.Logf("RESULT exact: elapsed=%s code=%d interrupted=%v out=%q stderr=%q", el, code, interrupted, out, stderr)
	hasOut := strings.Contains(out, "out")
	hasAfter := strings.Contains(out, "after")
	t.Logf("CLAIM CHECK: exit1=%v stdoutEmpty=%v hasOut=%v hasAfter=%v nativeCommandError=%v",
		code == 1, strings.TrimSpace(out) == "", hasOut, hasAfter, strings.Contains(stderr, "NativeCommandError"))
	// Does the shell live on and is the preference reset for the next chunk?
	out2, _, code2, _, _ := zzEapRun(t, s, `cmd /c "echo err2 1>&2 & echo out2 & exit 0"; Write-Output after2`)
	t.Logf("NEXT CHUNK: code=%d out=%q", code2, out2)
}

// Real shape: git checkout -b writes "Switched to a new branch" on stderr and exits 0.
func TestZZEap_RealShapeGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	s := psOnly(t)
	// set up a repo in the shell's cwd (t.TempDir())
	zzEapRun(t, s, `git init -q .; git -c user.email=a@b -c user.name=a commit -q --allow-empty -m init`)
	out, stderr, code, _, _ := zzEapRun(t, s, `$ErrorActionPreference='Stop'; git checkout -b feature; git status`)
	t.Logf("RESULT git: code=%d out=%q stderr=%q", code, out, stderr)
	t.Logf("CLAIM CHECK git: exit1=%v hasStatusOutput=%v", code == 1, strings.Contains(out, "On branch"))
	// Did the branch switch actually happen (the program ran and succeeded)?
	outB, _, _, _, _ := zzEapRun(t, s, `git rev-parse --abbrev-ref HEAD`)
	t.Logf("branch now: %q", strings.TrimSpace(outB))
}

// Diagnostic: spawn powershell.exe directly (same args as the product) and run
// the claim input WITHOUT the wrapper's 2>&1 merge, to see whether the
// termination is caused by the wrapper's redirect or by PowerShell alone.
func TestZZEap_DirectSpawnDiagnostic(t *testing.T) {
	type variant struct {
		name   string
		script string
	}
	src := `$ErrorActionPreference='Stop'; cmd /c "echo err 1>&2 & echo out & exit 0"; Write-Output after`
	variants := []variant{
		{"no-redirect, top-level", src + "\nWrite-Output ('LASTEXITCODE=' + $LASTEXITCODE)\n"},
		{"scriptblock, no 2>&1", "$sb=[scriptblock]::Create('" + strings.ReplaceAll(src, "'", "''") + "'); try { & $sb | ForEach-Object { $_ } } catch { Write-Output ('CAUGHT: ' + $_) }\nWrite-Output done\n"},
		{"scriptblock, 2>&1 (wrapper shape)", "$sb=[scriptblock]::Create('" + strings.ReplaceAll(src, "'", "''") + "'); try { & $sb 2>&1 | ForEach-Object { if ($_ -is [System.Management.Automation.ErrorRecord]) { Write-Output ('ERR: ' + $_) } else { $_ } } } catch { Write-Output ('CAUGHT: ' + $_) }\nWrite-Output done\n"},
		{"rev2 shape: & { } 2>&1", "$o = & {\n" + src + "\n} 2>&1 | ForEach-Object { if ($_ -is [System.Management.Automation.ErrorRecord]) { 'ERR: ' + $_ } else { $_ } }\nWrite-Output ('AFTER-BLOCK o=' + ($o -join '|'))\n"},
	}
	for _, v := range variants {
		for _, stderrNull := range []bool{true, false} {
			cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", "-")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			if !stderrNull {
				cmd.Stderr = &stderr
			}
			cmd.Stdin = strings.NewReader(v.script + "\nWrite-Output END\n")
			start := time.Now()
			err := cmd.Run()
			t.Logf("VARIANT %q (child stderr -> %s): elapsed=%s err=%v\n  STDOUT=%q\n  STDERR=%q",
				v.name, map[bool]string{true: "null", false: "pipe"}[stderrNull], time.Since(start), err, stdout.String(), stderr.String())
		}
	}
}
