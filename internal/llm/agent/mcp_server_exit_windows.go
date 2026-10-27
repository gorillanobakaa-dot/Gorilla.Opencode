package agent

import "github.com/opencode-ai/opencode/internal/procjob"

// processRunning reports whether pid is still running (it reads the exit code,
// so an exited process that still has open handles counts as gone).
func processRunning(pid int) bool { return procjob.Alive(pid) }
