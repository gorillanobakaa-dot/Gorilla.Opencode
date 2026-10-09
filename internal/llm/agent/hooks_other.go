//go:build !unix

package agent

import "os/exec"

// On Windows the tree is stopped with taskkill /T in killHookTree; there is no
// process group to set up here.
func prepareHookProcess(*exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error { return cmd.Process.Kill() }
