//go:build unix

package agent

import (
	"os/exec"
	"syscall"
)

// prepareHookProcess puts the hook in its own process group, so a timeout can
// stop everything it started and not only the shell.
func prepareHookProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup kills the hook's whole process group.
func killProcessGroup(cmd *exec.Cmd) error {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err == nil {
		return nil
	}
	return cmd.Process.Kill()
}
