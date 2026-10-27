# Job Objects, interrupts that keep the session, and a real console - dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers job 02 of the goal "windows-capabilities" (2026-10-27): gaps #9, #10 and #11 of the goal's REPORT.md.

---

# Track one: plain language

## What happened

When Gorilla OpenCode runs a command on Windows, that command can start other programs, and those can start more. Until now, stopping the command, closing a language server or Gorilla crashing left some of those programs running, invisible, using memory and holding files open. Stopping a long PowerShell command also threw away everything the session had set up. And a command that asks a question (`Read-Host`, "are you sure? y/n") just hung until the time ran out.

Windows has a built-in feature for the first problem, the Job Object: a container for a group of programs that the system ends all at once. Every program Gorilla starts now goes into one. The other two problems were fixed in our own code: a stop now stops only the command, and there is a new "console mode" for commands that need to ask something.

## Honest state of play

Done and tested on this laptop (Windows 11, PowerShell 5.1), with real programs: a program started by a program started by the command is ended when the command is stopped, when its session closes, and when Gorilla itself is killed without warning. The same holds for language servers, MCP servers and hooks. Stopping a running PowerShell command keeps the session. `Read-Host` answers in console mode.

Not tried: PowerShell 7 (not installed here), a real language server or MCP server that starts its own helpers (the tests use `cmd` and `ping` to build the same family tree), Linux and macOS (nothing changes there; the Mac build compiles).

## Worst case if something is wrong

- Something you asked Gorilla to start in the background (a development server, a file watcher) now stops when Gorilla's session for it ends: when you stop a later command, change folder with /cd, or quit Gorilla. Before, it ran on unseen. If you want a program to outlive Gorilla, start it yourself outside Gorilla.
- If Windows refuses to make a Job Object (it has never happened here), commands still run, and stopping falls back to the old way, which can miss a program whose parent already ended.
- If a future PowerShell changes the internal function used to stop a command, a stop ends the whole shell again, and the answer says so: "variables and an activated venv set up earlier are gone".

## What changed for you

**Stopping a command (Esc, or a timeout)**
- Before: Programs the command had started could keep running, and a PowerShell command such as `Start-Sleep` or a long `Get-ChildItem -Recurse` could only be stopped by ending the whole shell, losing its variables and virtual environment.
- After: Everything the command started is ended. The shell stays, with its variables, virtual environment and folder, and the answer says "The shell session was kept".
- Affects: Everyone on Windows

**Gorilla crashing or being killed**
- Before: Every program it had started stayed behind.
- After: Windows ends them all, because the container dies with Gorilla.
- Affects: Everyone on Windows

**Language servers, MCP servers, hooks**
- Before: Closing one ended only the server itself, not the helpers it had started.
- After: The server and everything it started end together.
- Affects: Anyone using code diagnostics, MCP servers or hooks

**Commands that ask a question**
- Before: `Read-Host`, a "y/n" prompt, or a program that checks whether it is in a real console hung until the time ran out; the AI was told never to use them.
- After: The AI can run such a command in console mode and type the answers. It runs as a separate program in the same folder, so it does not see variables set earlier in the session.
- Affects: Anyone running installers, scripts with prompts, or console programs

## What you can do now

- Ask: "run the setup script, and answer yes when it asks". The AI uses console mode and types `yes`.
- Press Esc on a long command and carry on in the same session: your virtual environment is still active.
- Quit Gorilla while a test run is going: nothing is left running in Task Manager.

## What is still missing

- Console mode is for one command at a time; a full interactive program (an editor, a game) is not supported.
- PowerShell 7 was not available to test.

## How to check it works on your computer

Open PowerShell in the program's source folder and run:

```powershell
cd $HOME\Documents\Gorilla.Opencode
go test ./internal/llm/tools/shell/... -run '(?i)jobobject|grandchild|conpty' -v
```

Every line that starts with `--- PASS` is one check that passed. A line with `--- FAIL` names the check that did not.

## Should you be concerned?

No new door was opened: nothing is downloaded, nothing is sent anywhere, no setting of the computer is changed. Code was copied (not downloaded at run time) from Microsoft's hcsshim project under its MIT license; none of its logging or telemetry was taken.

## Glossary

- **Job Object**: a Windows container for a group of programs; closing it ends every program in it.
- **Grandchild**: a program started by a program the command started.
- **ConPTY (pseudoconsole)**: Windows' way to give a program a real console window that another program reads and types into.
- **Pipeline**: the command PowerShell is running right now.

## Claim Sources

- Job Objects and JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE: Microsoft Learn, "Job Objects" and JOBOBJECT_BASIC_LIMIT_INFORMATION.
- ConPTY: Microsoft Learn, "Creating a Pseudoconsole session".
- Every "after" above: the tests named in Track two, run 2026-10-27 on this laptop.

---

# Track two: developer

## Problem Being Solved

REPORT.md gaps: #9 no Job Objects (grandchildren survive, nothing is cleaned up on a crash, LSP/MCP children leak through a bare `Process.Kill`); #10 Ctrl-C on a cmdlet kills the whole shell; #11 no ConPTY, so `Read-Host` hangs and console-needing programs fail.

## Approach Taken

- New package `internal/procjob`: a Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, adapted from hcsshim v0.15.0 `internal/jobobject` (MIT, copied with its notice, pinned in `third_party/PINS.md`). `Start(cmd)` creates the process `CREATE_SUSPENDED`, assigns it, then resumes it (`NtResumeProcess`), so no child can be started outside the job. `StartContext` also sets `exec.Cmd.Cancel` to terminate the job. `KillAllBut(pid)` kills every job member except `pid` and the `conhost.exe`/`OpenConsole.exe` that serves it. The handle is never inheritable, so the only handle dies with this process and a crash kills the tree. Outside Windows every method is a no-op and `Start` is `cmd.Start()`.
- Shell (`shell.go`): started with `procjob.Start`; an interrupt calls `job.KillAllBut(shellPid)` (fallback: the old `taskkill /T` of direct children); the process waiter closes the job when the shell ends for any reason (`Close`, `exit`, a kill).
- Gap #10: `psSessionSetup` adds `GorillaStop` (Add-Type): a background thread waits on a per-shell named auto-reset event (`Local\gorilla-opencode-stop-<pid>-<n>`, created by Go in `stop_windows.go`) and, while the wrapper has called `Begin()`, invokes the internal `Runspace.GetCurrentlyRunningPipeline()` by reflection and `StopAsync()`s it, which is what Ctrl+C does at a console. The wrapper is now one `try/finally`; on a stop the `finally` writes the four files with .NET calls only (a cmdlet called while the pipeline stops is itself stopped), exit code 130 and "The command was stopped before it finished.". `stopRunning` returns whether the status file appeared within 2 s (`stopGrace`); `interruptNote` tells the model "kept" or "gone". The `finally` also runs when the wrapper is left without a stop: `exit` (PowerShell then ends) or a top-level `break`/`continue` (it does not). `GorillaStop.Requested()` tells a stop from those; otherwise the status is `unwound`, and Go waits up to 2 s (`unwoundGrace`) to see whether the shell died: dead is reported as before ("the shell session ended ... `exit`"), alive as exit code 1, "the command left early".
- LSP (`lsp/client.go`): `procjob.StartContext`; `Close` closes the job after a clean exit and on the 2 s timeout (instead of `Process.Kill`). `CloseAllFiles` read `config.Get().DebugLSP` and panicked when no config was loaded; it uses `debugLSPEnabled()` now.
- MCP (`agent/mcp-tools.go`): mcp-go starts the process itself, so `newStdioMCPClient` assigns it right after the start (a launcher does not start a child in that first moment, but it is not race-free) and keeps the job in `mcpJobs`; `abandonMCPClient` closes the job (was `Process.Kill`), `closeMCPClient` closes the client then the job.
- Hooks (`agent/hooks.go`): `procjob.StartContext` + `Wait`, job closed when the hook returns.
- ConPTY (`shell/conpty_windows.go`): `RunInConsole` adapted from hcsshim `internal/conpty`: `CreatePseudoConsole` (200x50), `CreateProcess` with `EXTENDED_STARTUPINFO_PRESENT`, the `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE` attribute, `STARTF_USESTDHANDLES` with empty handles (otherwise the child of a process with redirected output writes to those redirections), `CREATE_SUSPENDED` + job + `ResumeThread`. PowerShell gets the command as `-EncodedCommand`. Input is typed with CR line ends. Output is drained continuously, `ClosePseudoConsole` runs while it drains, VT sequences are stripped (CSI, OSC, two-byte ESC), capped at 1 MiB with a note.
- Bash tool: `console` and `input` parameters, Windows only (`shell.ConsoleSupported()`); the PowerShell guidance now says to use them instead of "never use Read-Host".

## Before

- `cmd -> cmd -> ping`, top `cmd` killed with `Process.Kill`: the ping keeps running (`TestGrandchildSurvivesWithoutJobObject`, which stays in the suite as the baseline).
- An interrupt ran `taskkill /T` on the shell's direct children; a grandchild whose parent had already exited (`cmd /c start /b ...`) was in no tree it walked.
- A busy cmdlet on interrupt: the shell was killed after 750 ms, the model was told only "timed out or was interrupted".

## After

| Gap | Change | Tests |
|---|---|---|
| #9 shell | shell in a job; interrupt = `KillAllBut`; job closed when the shell ends | `TestJobObjectInterruptKillsOrphanedGrandchildAndKeepsSession`, `TestJobObjectShellCloseKillsBackgroundGrandchild`, `TestJobObjectShellExitKillsBackgroundGrandchild` |
| #9 crash | KILL_ON_JOB_CLOSE, handle not inheritable | `TestJobObjectGrandchildDiesOnParentCrash` (a helper test process holding the job is killed with TerminateProcess) |
| #9 procjob | Start, Terminate, Pids, KillAllBut, Close | `TestJobObjectKillsGrandchild`, `TestJobObjectStartPutsTheWholeTreeInTheJob`, `TestJobObjectKillAllButKeepsTheNamedProcess`, `TestJobObjectCloseIsSafeTwiceAndOnNil` |
| #9 LSP | job; Close and context cancel kill the tree | `TestJobObjectLSPCloseKillsGrandchild`, `TestJobObjectLSPContextCancelKillsGrandchild` |
| #9 MCP | job per stdio server; abandon closes it | `TestJobObjectMCPAbandonKillsGrandchild` |
| #9 hooks | job per hook | `TestJobObjectHookBackgroundGrandchildDies` |
| #10 | GorillaStop + try/finally wrapper; note says kept/lost; exit/break told apart | `TestACommandThatCallsExitIsNotReportedAsATimeout` (existing), `TestABreakOutsideALoopKeepsTheSession`, `TestJobObjectInterruptKillsOrphanedGrandchildAndKeepsSession` (Start-Sleep is a cmdlet; `$env:` set before survives), `TestJobObjectInterruptNoteSaysWhenTheSessionIsLost` |
| #11 | `RunInConsole`, bash `console`/`input` | `TestReadHostInConsole_conpty` (Read-Host returns the typed line in ~0.4 s), `TestConsoleIsARealConsole_conpty` (`[Console]::IsOutputRedirected` False), `TestConsoleTimeoutStopsTheCommand_conpty`, `TestConsoleOutputDropsEscapes_conpty`, `TestBashToolConsoleMode_conpty` |

Mutation plan `tests/mutations-jobobjects-conpty.yaml`: 27 rules (see the PLAN row for the run's result).

Schema cost (`TestDefaultToolSchemaCost`, Windows): tool.bash 1,594 -> 1,552, default-ON tools 10,713 -> 10,671. The two parameters were paid for by cutting the generic steps 3-5 of the bash description to one line; the 1,600 per-tool cap was 6 tokens away before.

## Known Alternatives

- `PROC_THREAD_ATTRIBUTE_JOB_LIST` (hcsshim's `UpdateProcThreadAttribute`) puts a process in a job at creation, but `os/exec` takes no attribute list; suspended-assign-resume gives the same guarantee with `exec.Cmd` kept.
- Putting Gorilla itself in a job would catch every child without a race, but one job for everything cannot stop one shell's tree without the others.
- Gap #10 via `GenerateConsoleCtrlEvent`: needs a shared console and a process group, and a group started with `CREATE_NEW_PROCESS_GROUP` ignores Ctrl+C unless it re-enables it; no console exists in a headless run.
- ConPTY as the default transport: rejected for now. Its output is a screen drawing (wrapped at the console width, cursor moves, escape sequences stripped by pattern), and the persistent session's file-based protocol would have to be rebuilt on top of it. Pipes stay the default; console mode is opt-in per command.

## Files Changed

- `internal/procjob/` (new): `doc.go`, `job_windows.go`, `job_other.go`, `cancel.go`, tests
- `internal/llm/tools/shell/shell.go`, `stop_windows.go`, `stop_other.go`, `conpty_windows.go`, `conpty_other.go` (new files), tests `jobobject_*_test.go`, `conpty_*_test.go`
- `internal/llm/tools/bash.go`, `bash_console_windows_test.go`
- `internal/lsp/client.go`, `jobobject_windows_test.go`
- `internal/llm/agent/mcp-tools.go`, `hooks.go`, `mcp_jobobject_windows_test.go`, `schema_cost_test.go`
- `third_party/hcsshim/LICENSE`, `third_party/hcsshim/README.md`, `third_party/PINS.md` (new)
- `docs/WHAT-A-CURIOUS-USER-COSTS.md`, `tests/mutations-jobobjects-conpty.yaml`

## Decisions Made

- Background programs die with the session. A tree that outlives the agent invisibly is the bug being fixed; a server the user wants to keep is started outside Gorilla.
- `KillAllBut` spares the shell's console host: killing `conhost.exe` takes the shell's console and the shell with it.
- Test names: `go test -run` splits the pattern at top-level `|` and compiles each part separately, so in `(?i)jobobject|grandchild|conpty` only `jobobject` is case-insensitive. Tests carry `JobObject` or a lowercase `_conpty` suffix so the goal's command selects every one (measured: `TestGrandchildDiesOnParentCrash` and `TestConPTY*` were silently not run before the rename).
- The ping sleeper, not `timeout.exe`: `timeout` refuses to run with redirected input, and no test has a console.

## Tried and Abandoned

- The first try/finally wrapper wrote "stopped, exit 130" for an `exit` too, because PowerShell runs `finally` on the way out of `exit`; the full suite caught it (`TestACommandThatCallsExitIsNotReportedAsATimeout`), hence `Requested()`.
- Starting a hook in the process's own folder when the configured one is gone: reverted, hooks fail closed there on purpose (`TestABeforeToolHookThatCannotRunRefuses`). Console mode does fall back, as the persistent shell does.
- `cmd /c "start /b ping ..."` as the background program in the close test: the started ping inherits the capture pipe and the command waits for it (25 s, measured); `Start-Process -WindowStyle Hidden` is used there.
- `unsafe.Pointer(hpc)` for the pseudoconsole attribute (hcsshim's form): `go vet` rejects it; the handle is read through a pointer instead.

## ⚠ Claimed But Not Verified

- The suspended start closes the race of a child started before the assignment; no test can trigger that race on demand, so it is not in the mutation plan.
- MCP servers are assigned after mcp-go starts them; a server that spawns a child in its first microseconds would escape the job.
- `GetCurrentlyRunningPipeline` on PowerShell 7 (not installed here). If absent, the stop falls back to ending the shell and says so.
- Real language servers and MCP launchers (gopls, npx, uvx) were not run; their trees are modeled with cmd and ping.
- Linux and macOS: the darwin build compiles; the Unix paths are unchanged and their tests skip with a reason.

## Open Items

- The variables test uses `$env:`: plain `$x = 1` never persisted between commands, because the wrapper runs each command as a script block (child scope). Unchanged by this job; job 06 owns the guidance.
- Console mode has no resize and no interactive back-and-forth; answers are typed up front.

## How to verify this work is correct

```powershell
cd $HOME\Documents\Gorilla.Opencode
go test ./internal/llm/tools/shell/... ./internal/procjob/... ./internal/lsp/... ./internal/llm/agent/... ./internal/llm/tools/ -run '(?i)jobobject|(?i)grandchild|(?i)conpty' -v
$env:PYTHONUTF8 = "1"; python -m fieldkit mutate --plan tests/mutations-jobobjects-conpty.yaml --cwd . -- go test ./internal/procjob/ ./internal/llm/tools/shell/ ./internal/lsp/ ./internal/llm/agent/ ./internal/llm/tools/ -count=1 -run "(?i)jobobject|(?i)grandchild|(?i)conpty|ExitIsNotReported|BreakOutsideALoop"
```

## Technical Debt

- `GorillaStop` reaches an internal PowerShell method by reflection.
- The VT stripping is pattern-based; a program that redraws heavily (progress bars) leaves its final screen text, not a log.

## Claim Sources

- hcsshim v0.15.0, commit 900183417d29765de2947de22a6e72c22c195550: `internal/jobobject/jobobject.go`, `internal/conpty/conpty.go`, `internal/winapi`.
- learn.microsoft.com/windows/win32/procthread/job-objects; learn.microsoft.com/windows/console/creating-a-pseudoconsole-session
- Measurements on this laptop, 2026-10-27: the tests above; the `-run` alternation split (`(?i)crash|conptyisa` selected only the crash test).
