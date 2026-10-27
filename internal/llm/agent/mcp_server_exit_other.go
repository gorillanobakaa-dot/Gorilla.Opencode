//go:build !windows

package agent

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// processRunning reports whether pid is still running. Signal 0 finds a
// process that exists; on Linux a zombie (exited, not yet reaped: mcp-go reaps
// it only at Close) also counts as gone. Elsewhere a zombie still answers
// signal 0, so there the watch ends no call early and changes nothing.
func processRunning(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return true
	}
	s := string(stat)
	if i := strings.LastIndexByte(s, ')'); i >= 0 && i+2 < len(s) {
		return s[i+2] != 'Z'
	}
	return true
}
