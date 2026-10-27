//go:build windows

package shell

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/opencode-ai/opencode/internal/procjob"
	"golang.org/x/sys/windows"
)

// Gap #9 (Job Objects) and gap #10 (an interrupt keeps the session), measured
// on real processes. The sleeper is ping, not timeout.exe: timeout refuses to
// run with its input redirected, and nothing here has a console to give it.
// Every process a test starts is killed by pid in its cleanup if it is still
// there, so a failing test leaves nothing behind.

const sleeperExe = "ping.exe"

func sleeperArgs(seconds int) string { return fmt.Sprintf("ping -n %d 127.0.0.1 >nul 2>nul", seconds) }

type procInfo struct {
	pid, ppid int
	exe       string
}

func snapshot(t *testing.T) []procInfo {
	t.Helper()
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		t.Fatalf("CreateToolhelp32Snapshot: %v", err)
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var out []procInfo
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		out = append(out, procInfo{int(e.ProcessID), int(e.ParentProcessID), strings.ToLower(windows.UTF16ToString(e.ExeFile[:]))})
	}
	return out
}

// findDescendant waits for a process named exe below root.
func findDescendant(t *testing.T, root int, exe string, within time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		procs := snapshot(t)
		below := map[int]bool{root: true}
		for grew := true; grew; {
			grew = false
			for _, p := range procs {
				if below[p.ppid] && !below[p.pid] && p.pid != p.ppid {
					below[p.pid] = true
					grew = true
				}
			}
		}
		for _, p := range procs {
			if below[p.pid] && p.pid != root && p.exe == exe {
				return p.pid
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no %s appeared below pid %d within %v", exe, root, within)
	return 0
}

func alive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}

func waitDead(pid int, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if !alive(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return !alive(pid)
}

// killPid is the cleanup: by pid, never by name (the owner runs ping too).
func killPid(pid int) {
	if h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid)); err == nil {
		windows.TerminateProcess(h, 1)
		windows.CloseHandle(h)
	}
}

// startTree starts cmd -> cmd -> ping and returns the top cmd and the ping.
func startTree(t *testing.T, inJob bool) (*exec.Cmd, *procjob.Job, int) {
	t.Helper()
	cmd := exec.Command("cmd.exe", "/c", "cmd /c "+sleeperArgs(120))
	var job *procjob.Job
	var err error
	if inJob {
		job, err = procjob.Start(cmd)
		if err == nil && job == nil {
			t.Fatal("procjob.Start ran the process without a job")
		}
	} else {
		err = cmd.Start()
	}
	if err != nil {
		t.Fatalf("start the tree: %v", err)
	}
	t.Cleanup(func() { killPid(cmd.Process.Pid); _ = cmd.Wait() })
	grandchild := findDescendant(t, cmd.Process.Pid, sleeperExe, 10*time.Second)
	t.Cleanup(func() { killPid(grandchild) })
	return cmd, job, grandchild
}

// The "before": a bare Process.Kill (what lsp/client.go and mcp-tools.go
// did) ends the process it names and nothing below it.
func TestGrandchildSurvivesWithoutJobObject(t *testing.T) {
	cmd, _, grandchild := startTree(t, false)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill: %v", err)
	}
	_ = cmd.Wait()
	time.Sleep(500 * time.Millisecond)
	if !alive(grandchild) {
		t.Fatalf("the grandchild %d died with its grandparent: the baseline this job fixes is gone, re-measure", grandchild)
	}
}

// The "after": closing the job kills the whole tree, grandchild included.
func TestJobObjectKillsGrandchild(t *testing.T) {
	_, job, grandchild := startTree(t, true)
	pids, err := job.Pids()
	if err != nil {
		t.Fatalf("Pids: %v", err)
	}
	if !containsPid(pids, grandchild) {
		t.Fatalf("the grandchild %d is not in the job %v: it started outside it", grandchild, pids)
	}
	if err := job.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !waitDead(grandchild, 5*time.Second) {
		t.Fatalf("the grandchild %d outlived the job", grandchild)
	}
}

func containsPid(pids []int, pid int) bool {
	for _, p := range pids {
		if p == pid {
			return true
		}
	}
	return false
}

const jobHelperEnv = "GORILLA_JOBOBJECT_HELPER"

// TestJobObjectHelperProcess is not a test: TestJobObjectGrandchildDiesOnParentCrash
// runs this binary again with jobHelperEnv set, and this is that process. It
// starts a tree in a job, prints the grandchild's pid and waits to be killed.
func TestJobObjectHelperProcess(t *testing.T) {
	if os.Getenv(jobHelperEnv) != "1" {
		t.Skip("helper process for TestJobObjectGrandchildDiesOnParentCrash")
	}
	cmd := exec.Command("cmd.exe", "/c", "cmd /c "+sleeperArgs(120))
	job, err := procjob.Start(cmd)
	if err != nil || job == nil {
		fmt.Printf("ERROR %v\n", err)
		os.Exit(3)
	}
	gc := findDescendant(t, cmd.Process.Pid, sleeperExe, 10*time.Second)
	fmt.Printf("GRANDCHILD %d\n", gc)
	time.Sleep(60 * time.Second) // killed long before this
	os.Exit(4)
}

// A crash: the process holding the job is killed outright (TerminateProcess,
// no deferred cleanup runs). The kernel closes its handles, the job's last
// handle with them, and KILL_ON_JOB_CLOSE takes the tree down.
func TestJobObjectGrandchildDiesOnParentCrash(t *testing.T) {
	helper := exec.Command(os.Args[0], "-test.run", "^TestJobObjectHelperProcess$", "-test.v")
	helper.Env = append(os.Environ(), jobHelperEnv+"=1")
	out, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killPid(helper.Process.Pid); _ = helper.Wait() })

	got := make(chan int, 1)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if rest, ok := strings.CutPrefix(sc.Text(), "GRANDCHILD "); ok {
				n, _ := strconv.Atoi(strings.TrimSpace(rest))
				got <- n
				return
			}
		}
		got <- 0
	}()
	var grandchild int
	select {
	case grandchild = <-got:
	case <-time.After(30 * time.Second):
		t.Fatal("the helper never reported its grandchild")
	}
	if grandchild == 0 {
		t.Fatal("the helper could not start its tree in a job")
	}
	t.Cleanup(func() { killPid(grandchild) })
	if !alive(grandchild) {
		t.Fatalf("the grandchild %d is not running before the crash", grandchild)
	}

	if err := helper.Process.Kill(); err != nil { // TerminateProcess: a crash, not an exit
		t.Fatalf("kill the helper: %v", err)
	}
	if !waitDead(grandchild, 5*time.Second) {
		t.Fatalf("the grandchild %d survived its parent's crash", grandchild)
	}
}

// The shell starts in a job, and an interrupt kills a grandchild whose parent
// has already exited (cmd /c start /b): the old taskkill /T walk from the
// shell's children cannot find it. The interrupted Start-Sleep is a cmdlet,
// so the session survives only if the pipeline was stopped rather than the
// shell killed (gap #10), and the result must say which.
func TestJobObjectInterruptKillsOrphanedGrandchildAndKeepsSession(t *testing.T) {
	s := newTestShell(t)
	if !s.isPowerShell {
		t.Skip("the Windows shell under test is PowerShell; this machine resolved another shell")
	}
	if s.job == nil {
		t.Fatal("the shell was started without a Job Object")
	}
	exec1(t, s, "$env:GORILLA_KEPT = 'kept-42'")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type res struct {
		stderr      string
		interrupted bool
	}
	done := make(chan res, 1)
	go func() {
		_, stderr, _, interrupted, _ := s.Exec(ctx, `cmd /c "start /b `+sleeperArgs(120)+`"; Start-Sleep -Seconds 60`, 90000)
		done <- res{stderr, interrupted}
	}()

	// The orphan: in the shell's job, its parent cmd already gone.
	var orphan int
	deadline := time.Now().Add(15 * time.Second)
	for orphan == 0 && time.Now().Before(deadline) {
		pids, _ := s.job.Pids()
		for _, p := range snapshot(t) {
			if p.exe == sleeperExe && containsPid(pids, p.pid) && !alive(p.ppid) {
				orphan = p.pid
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if orphan == 0 {
		t.Fatal("no orphaned ping appeared in the shell's job")
	}
	t.Cleanup(func() { killPid(orphan) })
	for _, c := range childPIDs(s.cmd.Process.Pid) {
		if c == orphan {
			t.Fatalf("the orphan %d is still a child of the shell; the test no longer shows the orphan case", orphan)
		}
	}

	cancel()
	var r res
	select {
	case r = <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the interrupted command did not return")
	}
	if !r.interrupted {
		t.Error("the result was not marked interrupted")
	}
	if !waitDead(orphan, 5*time.Second) {
		t.Errorf("the orphaned grandchild %d survived the interrupt", orphan)
	}
	if !s.isAlive {
		t.Fatalf("the interrupt ended the shell; result said: %q", r.stderr)
	}
	if !strings.Contains(r.stderr, interruptNote(true)) {
		t.Errorf("the result does not tell the model the session was kept: %q", r.stderr)
	}
	stdout, _, _ := exec1(t, s, "Write-Output $env:GORILLA_KEPT")
	if !strings.Contains(stdout, "kept-42") {
		t.Errorf("a variable set before the interrupt is gone: %q", stdout)
	}
}

// When the shell cannot be stopped without ending it, the result says so.
func TestJobObjectInterruptNoteSaysWhenTheSessionIsLost(t *testing.T) {
	lost, kept := interruptNote(false), interruptNote(true)
	if lost == kept || !strings.Contains(lost, "gone") || !strings.Contains(kept, "kept") {
		t.Fatalf("the two notes must differ and say what happened: kept=%q lost=%q", kept, lost)
	}
}

// Closing the shell kills what it left running in the background.
func TestJobObjectShellCloseKillsBackgroundGrandchild(t *testing.T) {
	s := newPersistentShell(t.TempDir())
	if s == nil {
		t.Fatal("no shell")
	}
	if s.job == nil {
		s.Close()
		t.Fatal("the shell was started without a Job Object")
	}
	if !s.isPowerShell {
		s.Close()
		t.Skip("PowerShell syntax below")
	}
	// Start-Process returns at once and leaves ping running on its own, the
	// way a dev server is started in the background. (cmd's start /b would
	// hand ping the capture pipe and the command would wait for it.)
	exec1(t, s, `Start-Process -FilePath ping.exe -ArgumentList '-n','120','127.0.0.1' -WindowStyle Hidden`)
	var bg int
	pids, _ := s.job.Pids()
	for _, p := range snapshot(t) {
		if p.exe == sleeperExe && containsPid(pids, p.pid) {
			bg = p.pid
		}
	}
	if bg == 0 {
		s.Close()
		t.Fatal("the background ping is not in the shell's job")
	}
	t.Cleanup(func() { killPid(bg) })
	s.Close()
	if !waitDead(bg, 5*time.Second) {
		t.Fatalf("the background grandchild %d outlived the closed shell", bg)
	}
}

// A shell that ends by itself (an `exit` in a command) takes its background
// processes with it: the process waiter closes the job.
func TestJobObjectShellExitKillsBackgroundGrandchild(t *testing.T) {
	s := newTestShell(t)
	if !s.isPowerShell || s.job == nil {
		t.Fatalf("want PowerShell in a job, got powershell=%v job=%v", s.isPowerShell, s.job != nil)
	}
	stdout, _, _ := exec1(t, s, `(Start-Process -FilePath ping.exe -ArgumentList '-n','120','127.0.0.1' -WindowStyle Hidden -PassThru).Id`)
	bg, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		t.Fatalf("no pid: %q", stdout)
	}
	t.Cleanup(func() { killPid(bg) })
	exec1(t, s, "exit")
	if !waitDead(bg, 10*time.Second) {
		t.Fatalf("the background process %d outlived the shell's exit", bg)
	}
}

// The wrapper's finally also runs for a top-level break, which does not end
// PowerShell: the result must come back at once, not at the timeout, and
// must not be mistaken for an interrupt or for `exit`.
func TestABreakOutsideALoopKeepsTheSession(t *testing.T) {
	s := newTestShell(t)
	if !s.isPowerShell {
		t.Skip("PowerShell wrapper")
	}
	start := time.Now()
	_, stderr, code := exec1(t, s, "Write-Output before; break")
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("took %v", took)
	}
	if !s.isAlive || code != 1 || !strings.Contains(stderr, "left early") || strings.Contains(stderr, stoppedMarker) {
		t.Fatalf("alive=%v code=%d stderr=%q", s.isAlive, code, stderr)
	}
	if out, _, _ := exec1(t, s, "Write-Output after"); !strings.Contains(out, "after") {
		t.Fatalf("the session did not survive: %q", out)
	}
}
