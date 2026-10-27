package agent

import "golang.org/x/sys/unix"

// szomb is SZOMB in <sys/proc.h>: the process has exited and waits to be
// reaped.
const szomb = 5

// isZombie asks the kernel's process table (sysctl kern.proc.pid), the same
// source ps uses for its Z state.
//
// GORILLA FIX (2026-10-27, job 09): macOS had no zombie check, so an MCP call
// to a stdio server that died mid-call still waited for ever there.
func isZombie(pid int) bool {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return false
	}
	return kp.Proc.P_stat == szomb
}
