package procjob

import (
	"os/exec"
	"sync/atomic"
)

// StartContext is Start for a command made with exec.CommandContext: when
// the context ends, the whole job is terminated, and fallback runs only if
// there is no job (outside Windows, or none could be made). Cancel is set
// before the start, so the context watcher never sees a half-made value.
func StartContext(cmd *exec.Cmd, fallback func() error) (*Job, error) {
	var holder atomic.Pointer[Job]
	cmd.Cancel = func() error {
		if j := holder.Load(); j != nil && j.Terminate() == nil {
			return nil
		}
		return fallback()
	}
	job, err := Start(cmd)
	holder.Store(job)
	return job, err
}
