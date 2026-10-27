//go:build !windows

package agent

import "syscall"

// processRunning reports whether pid is still running. Signal 0 finds a
// process that exists; a zombie (exited, not yet reaped: mcp-go reaps it only
// at Close) answers signal 0 too, so it is asked for by name: /proc on Linux,
// the kernel's process table on macOS. Elsewhere a zombie still counts as
// running, so there the watch ends no call early and changes nothing.
func processRunning(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	return !isZombie(pid)
}
