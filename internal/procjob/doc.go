// Package procjob keeps every process Gorilla OpenCode starts in a Windows
// Job Object (gap #9), so a whole tree dies together: on an interrupt, when a
// server is stopped, and when this program crashes or is killed, because the
// job is created with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE and the only handle
// to it dies with the process. Outside Windows it is a no-op and callers keep
// their process-group code.
package procjob
