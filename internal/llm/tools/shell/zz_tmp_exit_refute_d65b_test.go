package shell

// TEMPORARY refutation probe — deleted after the run.

import (
	"context"
	"os"
	"testing"
	"time"
)

func zzExitProbe(t *testing.T, s *PersistentShell, cmd string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	stdout, stderr, code, interrupted, err := s.Exec(ctx, cmd, 25000)
	t.Logf("cmd=%q elapsed=%s", cmd, time.Since(start))
	t.Logf("stdout=%q", stdout)
	t.Logf("stderr=%q", stderr)
	t.Logf("code=%d interrupted=%v err=%v isAlive=%v isPowerShell=%v", code, interrupted, err, s.isAlive, s.isPowerShell)
	time.Sleep(300 * time.Millisecond)
	if s.cmd != nil && s.cmd.ProcessState != nil {
		t.Logf("shell process state: %v exitcode=%d", s.cmd.ProcessState, s.cmd.ProcessState.ExitCode())
	} else {
		t.Logf("shell process still running (ProcessState nil)")
	}
}

func TestZZTmpExitRefutePowerShell(t *testing.T) {
	s := newTestShell(t)
	if !s.isPowerShell {
		t.Skip("powershell only")
	}
	zzExitProbe(t, s, "Write-Output before_exit; exit 5")
}

func TestZZTmpExitRefuteBash(t *testing.T) {
	bash := `C:\Program Files\Git\bin\bash.exe`
	if _, err := os.Stat(bash); err != nil {
		t.Skip("no Git bash")
	}
	t.Setenv("SHELL", "")
	t.Setenv("SHELL_FALLBACK", bash)
	t.Setenv("PATH", `C:\Windows\System32\does-not-exist`)
	s := newPersistentShell(t.TempDir())
	if s == nil {
		t.Fatal("no bash shell")
	}
	t.Cleanup(s.Close)
	if s.isPowerShell {
		t.Fatal("got powershell, wanted bash")
	}
	zzExitProbe(t, s, "echo before_exit; exit 5")
}
