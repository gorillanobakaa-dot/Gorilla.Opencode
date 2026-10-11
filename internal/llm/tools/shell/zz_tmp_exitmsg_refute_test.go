package shell

// TEMPORARY reviewer probe — deleted after the run. Not part of the tree.

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestZZTmpExitMsgRefute(t *testing.T) {
	s := newTestShell(t)
	if !s.isPowerShell {
		t.Skip("PowerShell only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	stdout, stderr, code, interrupted, err := s.Exec(ctx, "Write-Output before_exit; exit 5", 25000)
	elapsed := time.Since(start)
	time.Sleep(300 * time.Millisecond)

	t.Logf("elapsed=%s", elapsed)
	t.Logf("stdout=%q", stdout)
	t.Logf("stderr=%q", stderr)
	t.Logf("code=%d interrupted=%v err=%v isAlive=%v", code, interrupted, err, s.isAlive)
	t.Logf("stdout contains before_exit: %v", strings.Contains(stdout, "before_exit"))
	t.Logf("stderr claims 'Any output above is real': %v", strings.Contains(stderr, "Any output above is real"))
	t.Logf("stderr recommends `false`: %v", strings.Contains(stderr, "use `false`"))
	if s.cmd != nil && s.cmd.ProcessState != nil {
		t.Logf("powershell.exe process state: exited=%v exitcode=%d", s.cmd.ProcessState.Exited(), s.cmd.ProcessState.ExitCode())
	} else {
		t.Logf("powershell.exe process state: not yet available (process still running?)")
	}

	// Second, independent shell: is `false` a PowerShell command?
	s2 := newTestShell(t)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()
	o2, e2, c2, _, _ := s2.Exec(ctx2, "false", 25000)
	t.Logf("`false`: stdout=%q stderr=%q code=%d isAlive=%v", o2, e2, c2, s2.isAlive)
}
