//go:build !windows

package procjob

import (
	"os/exec"
	"testing"
)

func TestJobObjectStartWithoutJobsStillStarts(t *testing.T) {
	cmd := exec.Command("true")
	j, err := Start(cmd)
	if err != nil {
		t.Skipf("no true(1): %v", err)
	}
	if j != nil {
		t.Fatal("a job outside Windows")
	}
	_ = cmd.Wait()
}
