//go:build !windows

package shell

// childPIDs is the Windows-only helper's counterpart. On Unix, killChildren
// uses pkill -P / pgrep -P directly, so nothing needs it here; it exists so the
// call site does not have to be written twice.
func childPIDs(ppid int) []int { return nil }
