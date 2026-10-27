//go:build !windows

package procjob

import (
	"errors"
	"os/exec"
)

// Job is a no-op outside Windows: Unix callers keep their process groups and
// pkill. Every method is safe on a nil Job.
type Job struct{}

var errUnsupported = errors.New("job objects are Windows-only")

// Supported reports whether Job Objects back this platform's Job.
func Supported() bool { return false }

func New() (*Job, error)                   { return nil, errUnsupported }
func (j *Job) AssignPid(int) error         { return errUnsupported }
func (j *Job) Terminate() error            { return errUnsupported }
func (j *Job) Close() error                { return nil }
func (j *Job) Pids() ([]int, error)        { return nil, errUnsupported }
func (j *Job) KillAllBut(int) (int, error) { return 0, errUnsupported }

// Start starts cmd. There is no job here, so the returned Job is nil.
func Start(cmd *exec.Cmd) (*Job, error) { return nil, cmd.Start() }

// ImageName is Windows-only; it returns "" here.
func ImageName(int) string { return "" }

// Alive is Windows-only; it returns false here.
func Alive(int) bool { return false }
