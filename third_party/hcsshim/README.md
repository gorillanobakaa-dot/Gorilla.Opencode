# hcsshim (Microsoft, MIT)

Pattern source for Gorilla OpenCode's Windows Job Objects and ConPTY (windows-capabilities job 02).
Pinned in third_party/PINS.md: v0.15.0, 900183417d29765de2947de22a6e72c22c195550.

Copied and adapted (each file carries the MIT notice and points here):
- internal/jobobject/jobobject.go (Assign, Terminate, Pids) and the JOBOBJECT_BASIC_PROCESS_ID_LIST
  layout from internal/winapi -> internal/procjob/job_windows.go
- internal/conpty/conpty.go (Create, Close) -> internal/llm/tools/shell/conpty_windows.go

Not taken: the module itself (internal packages cannot be imported, and hcsshim as a whole carries
OpenTelemetry and ETW plumbing), the job notification queue, silos, file binding, logging.
