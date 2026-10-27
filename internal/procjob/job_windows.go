//go:build windows

// Parts of this file (the job object wrapper: Assign, Terminate, Pids and the
// JOBOBJECT_BASIC_PROCESS_ID_LIST layout) are adapted from microsoft/hcsshim
// internal/jobobject and internal/winapi, tag v0.15.0, commit
// 900183417d29765de2947de22a6e72c22c195550 (third_party/PINS.md).
//
//	The MIT License (MIT)
//	Copyright (c) 2015 Microsoft
//	Full text: third_party/hcsshim/LICENSE
//
// Only the plain Win32 calls were taken. hcsshim's notification queue, silo,
// file binding, logging and tracing code were not.

package procjob

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Job is a Windows Job Object created with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.
// Every process assigned to it, and every process those start, belongs to it;
// closing the last handle (Close, or this program dying in any way) kills
// them all. The handle is not inheritable, so no child can keep it open.
type Job struct {
	mu     sync.Mutex
	handle windows.Handle
}

// Supported reports whether Job Objects back this platform's Job.
func Supported() bool { return true }

// New creates an empty job that kills its processes when it is closed.
func New() (*Job, error) {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("CreateJobObject: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("SetInformationJobObject: %w", err)
	}
	return &Job{handle: h}, nil
}

var errClosed = errors.New("job object is closed")

// Handle is the raw job handle, for a CreateProcess attribute list.
func (j *Job) Handle() windows.Handle {
	if j == nil {
		return 0
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.handle
}

// AssignPid puts a running process (and so everything it starts from now on)
// in the job.
func (j *Job) AssignPid(pid int) error {
	if j == nil {
		return errClosed
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.handle == 0 {
		return errClosed
	}
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	p, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("OpenProcess %d: %w", pid, err)
	}
	defer windows.CloseHandle(p)
	return windows.AssignProcessToJobObject(j.handle, p)
}

// Terminate kills every process in the job, the whole tree at once.
func (j *Job) Terminate() error {
	if j == nil {
		return errClosed
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.handle == 0 {
		return errClosed
	}
	return windows.TerminateJobObject(j.handle, 1)
}

// Close releases the handle. With KILL_ON_JOB_CLOSE that kills whatever is
// still running in the job. Safe to call twice and on nil.
func (j *Job) Close() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(j.handle)
	j.handle = 0
	return err
}

// jobObjectBasicProcessIDList is JOBOBJECT_BASIC_PROCESS_ID_LIST (hcsshim
// internal/winapi): a header and a variable-length array of ULONG_PTR pids.
type jobObjectBasicProcessIDList struct {
	NumberOfAssignedProcesses uint32
	NumberOfProcessIdsInList  uint32
	ProcessIdList             [1]uintptr
}

// Pids lists the processes in the job now.
func (j *Job) Pids() ([]int, error) {
	if j == nil {
		return nil, errClosed
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.handle == 0 {
		return nil, errClosed
	}
	// hcsshim sizes the second query from the first; a process started in
	// between makes that one too small, so this grows and retries instead.
	n := 64
	for range 8 {
		buf := make([]uintptr, n+2)
		err := windows.QueryInformationJobObject(j.handle, windows.JobObjectBasicProcessIdList,
			uintptr(unsafe.Pointer(&buf[0])), uint32(len(buf))*uint32(unsafe.Sizeof(buf[0])), nil)
		if err == windows.ERROR_MORE_DATA {
			n *= 4
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("QueryInformationJobObject: %w", err)
		}
		info := (*jobObjectBasicProcessIDList)(unsafe.Pointer(&buf[0]))
		ids := unsafe.Slice(&info.ProcessIdList[0], info.NumberOfProcessIdsInList)
		pids := make([]int, len(ids))
		for i, p := range ids {
			pids[i] = int(p)
		}
		return pids, nil
	}
	return nil, errors.New("QueryInformationJobObject: the process list kept growing")
}

// KillAllBut kills every process in the job except keep and the console host
// (conhost.exe / OpenConsole.exe) that serves keep, because killing that host
// takes keep's console, and with it keep, down too. It returns how many were
// killed. This is what an interrupt uses: the shell lives, everything it
// started dies, including grandchildren whose parent already exited and that
// a walk of the parent chain (taskkill /T) can no longer find.
func (j *Job) KillAllBut(keep int) (int, error) {
	pids, err := j.Pids()
	if err != nil {
		return 0, err
	}
	hosts := consoleHostsOf(keep)
	killed := 0
	for _, pid := range pids {
		if pid == keep || hosts[pid] {
			continue
		}
		p, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
		if err != nil {
			continue // already gone
		}
		if windows.TerminateProcess(p, 1) == nil {
			killed++
		}
		windows.CloseHandle(p)
	}
	return killed, nil
}

// consoleHostsOf returns the console hosts whose parent is pid.
func consoleHostsOf(pid int) map[int]bool {
	hosts := map[int]bool{}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return hosts
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if int(e.ParentProcessID) != pid {
			continue
		}
		name := strings.ToLower(windows.UTF16ToString(e.ExeFile[:]))
		if name == "conhost.exe" || name == "openconsole.exe" {
			hosts[int(e.ProcessID)] = true
		}
	}
	return hosts
}

var procNtResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

// Start starts cmd inside a new job, with no window in which the process can
// start a child outside it: the process is created suspended, assigned, and
// only then resumed. If the job cannot be made or joined, the process runs
// anyway and Start returns a nil job (callers fall back to killing by pid);
// running without a job beats not running.
func Start(cmd *exec.Cmd) (*Job, error) {
	job, jerr := New()
	if jerr == nil {
		attr := &syscall.SysProcAttr{}
		if cmd.SysProcAttr != nil {
			c := *cmd.SysProcAttr
			attr = &c
		}
		attr.CreationFlags |= windows.CREATE_SUSPENDED
		cmd.SysProcAttr = attr
	}
	if err := cmd.Start(); err != nil {
		job.Close()
		return nil, err
	}
	if jerr != nil {
		return nil, nil
	}
	aerr := job.AssignPid(cmd.Process.Pid)
	if rerr := resume(cmd.Process.Pid); rerr != nil {
		// A process left suspended would hang its caller for ever.
		_ = cmd.Process.Kill()
		job.Close()
		return nil, fmt.Errorf("resume the started process: %w", rerr)
	}
	if aerr != nil {
		job.Close()
		return nil, nil
	}
	return job, nil
}

func resume(pid int) error {
	p, err := windows.OpenProcess(windows.PROCESS_SUSPEND_RESUME, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(p)
	if r, _, _ := procNtResumeProcess.Call(uintptr(p)); r != 0 {
		return fmt.Errorf("NtResumeProcess: NTSTATUS 0x%x", r)
	}
	return nil
}

// ImageName is the lower-case file name of a running process's program, or
// "" when it cannot be read (gone, or not ours to query).
func ImageName(pid int) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) != nil {
		return ""
	}
	full := windows.UTF16ToString(buf[:n])
	return strings.ToLower(full[strings.LastIndexAny(full, `\/`)+1:])
}

// Alive reports whether pid is a running process.
func Alive(pid int) bool {
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
