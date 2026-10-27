//go:build windows

package procjob

import (
	"os/exec"
	"testing"
	"time"
)

// findInJob waits for a process with the given image name in the job.
func findInJob(t *testing.T, j *Job, exe string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		pids, err := j.Pids()
		if err != nil {
			t.Fatalf("Pids: %v", err)
		}
		for _, p := range pids {
			if ImageName(p) == exe {
				return p
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no %s in the job within 10s", exe)
	return 0
}

func waitGone(pid int) bool {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !Alive(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func TestJobObjectStartPutsTheWholeTreeInTheJob(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "cmd /c ping -n 60 127.0.0.1 >nul 2>nul")
	j, err := Start(cmd)
	if err != nil || j == nil {
		t.Fatalf("Start: job=%v err=%v", j, err)
	}
	defer j.Close()
	gc := findInJob(t, j, "ping.exe")
	if gc == cmd.Process.Pid {
		t.Fatal("the grandchild is the top process")
	}
	if err := j.Terminate(); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	if !waitGone(gc) || !waitGone(cmd.Process.Pid) {
		t.Fatal("Terminate left part of the tree running")
	}
	_ = cmd.Wait()
}

func TestJobObjectKillAllButKeepsTheNamedProcess(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping -n 60 127.0.0.1 >nul 2>nul & ping -n 60 127.0.0.1 >nul 2>nul")
	j, err := Start(cmd)
	if err != nil || j == nil {
		t.Fatalf("Start: job=%v err=%v", j, err)
	}
	defer j.Close()
	gc := findInJob(t, j, "ping.exe")
	if _, err := j.KillAllBut(cmd.Process.Pid); err != nil {
		t.Fatalf("KillAllBut: %v", err)
	}
	if !waitGone(gc) {
		t.Fatal("the child survived KillAllBut")
	}
	if !Alive(cmd.Process.Pid) {
		t.Fatal("KillAllBut killed the process it was told to keep")
	}
}

func TestJobObjectCloseIsSafeTwiceAndOnNil(t *testing.T) {
	j, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if j.Close() != nil || j.Close() != nil {
		t.Fatal("Close twice must not fail")
	}
	var nilJob *Job
	if nilJob.Close() != nil || nilJob.Terminate() == nil {
		t.Fatal("a nil job closes quietly and refuses to terminate")
	}
}
