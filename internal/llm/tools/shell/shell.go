package shell

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/osutil"
	"github.com/opencode-ai/opencode/internal/procjob"
)

type PersistentShell struct {
	cmd          *exec.Cmd
	stdin        *os.File
	isAlive      bool
	cwd          string
	mu           sync.Mutex
	commandQueue chan *commandExecution
	isPowerShell bool
	// closeQueue closes commandQueue exactly once. Two goroutines closed it
	// (the panic handler and the process waiter); the second close panics.
	closeQueue sync.Once
	// job holds the shell and everything it starts (Windows; nil elsewhere
	// or when no job could be made). Closing it kills the whole tree.
	job *procjob.Job
	// stop interrupts a running PowerShell pipeline without ending the
	// session (Windows PowerShell only; nil otherwise).
	stop *stopSignal
}

type commandExecution struct {
	command    string
	timeout    time.Duration
	resultChan chan commandResult
	ctx        context.Context
}

type commandResult struct {
	stdout      string
	stderr      string
	exitCode    int
	interrupted bool
	err         error
}

var (
	shellInstance *PersistentShell
	// GORILLA OVERRIDE: was a sync.Once, which cannot be reset — so
	// ResetPersistentShell (needed by /cd) had no way to force a respawn. A
	// mutex plus a nil check gives the same once-only behaviour AND allows a
	// deliberate teardown.
	shellMu sync.Mutex
)

// ResetPersistentShell tears down the current shell so the next GetPersistentShell
// starts a fresh one in newCwd.
//
// GORILLA OVERRIDE: the shell is a process-lifetime singleton holding its OWN
// working directory (cmd.Dir, set once at spawn). Repointing config.WorkingDir
// with /cd therefore did NOT move the shell — every bash call kept running in the
// old directory while the rest of the app believed it had moved. That is the
// trap: a relative `make` or `ls` would silently operate on the previous project.
//
// Killing and respawning is the honest fix. Shell state the user set up by hand
// (exported vars, activated venv, cd'd subdirectory) is lost — that is inherent
// to changing the working directory, and losing it loudly beats keeping a shell
// pointed somewhere the user no longer means.
func ResetPersistentShell(newCwd string) {
	shellMu.Lock()
	old := shellInstance
	shellInstance = nil
	shellMu.Unlock()

	if old != nil {
		old.Close()
	}
	// Next GetPersistentShell call spawns in newCwd. Done lazily rather than
	// eagerly so /cd does not pay for a shell the user may never use.
	_ = newCwd
}

func GetPersistentShell(workingDir string) *PersistentShell {
	shellMu.Lock()
	defer shellMu.Unlock()

	if shellInstance == nil {
		shellInstance = newPersistentShell(workingDir)
	} else if !shellInstance.isAlive {
		// Respawn where the dead shell was, not where the caller happens to be.
		shellInstance = newPersistentShell(shellInstance.cwd)
	}
	return shellInstance
}

func newPersistentShell(cwd string) *PersistentShell {
	// Get shell configuration from config
	cfg := config.Get()

	// Default to environment variable if config is not set or nil
	var shellPath string
	var shellArgs []string

	if cfg != nil {
		shellPath = cfg.Shell.Path
		shellArgs = cfg.Shell.Args
	}

	if shellPath == "" {
		shellPath = resolveShellPath()
	}

	isPowerShell := looksLikePowerShell(shellPath)

	// Default shell args
	if len(shellArgs) == 0 {
		if isPowerShell {
			shellArgs = PowerShellArgs("-Command", "-")
		} else {
			shellArgs = []string{"-l"}
		}
	}

	// GORILLA OVERRIDE (2026-09-01): a working directory we cannot enter must
	// not cost us the shell.
	//
	// Measured failure this repairs: s.cwd picked up a BOM from the old
	// PowerShell wrapper, every respawn died with `chdir C:\...: The filename,
	// directory name, or volume label syntax is incorrect`, newPersistentShell
	// returned nil, and the bash tool reported "Shell could not be started" for
	// the rest of the session (and before the nil-check existed, panicked). The
	// directory is now validated up front, and a failed start is retried once
	// with the process's own directory, because a shell in the wrong folder is
	// still enormously more useful than no shell at all.
	if cwd != "" {
		if st, err := os.Stat(cwd); err != nil || !st.IsDir() {
			fmt.Fprintf(os.Stderr, "Shell working directory %q is unusable (%v); starting in the current directory instead\n", cwd, err)
			cwd = ""
		}
	}

	cmd, job, stdinPipe, err := startShell(shellPath, shellArgs, cwd)
	if err != nil && cwd != "" {
		fmt.Fprintf(os.Stderr, "Failed to start shell %s in %q: %v — retrying without a working directory\n", shellPath, cwd, err)
		cwd = ""
		cmd, job, stdinPipe, err = startShell(shellPath, shellArgs, "")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start shell %s: %v\n", shellPath, err)
		return nil
	}

	// The session setup (UTF-8 console, the stdin guard) goes down the pipe
	// before any command, so it runs first and at global scope.
	var stop *stopSignal
	if isPowerShell && runtime.GOOS == "windows" {
		stop = newStopSignal()
		if _, err := stdinPipe.Write([]byte(psSessionSetup + stop.armScript())); err != nil {
			fmt.Fprintf(os.Stderr, "PowerShell session setup failed: %v\n", err)
		}
	}

	shell := &PersistentShell{
		cmd:          cmd,
		stdin:        stdinPipe,
		isAlive:      true,
		cwd:          cwd,
		commandQueue: make(chan *commandExecution, 10),
		isPowerShell: isPowerShell,
		job:          job,
		stop:         stop,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Panic in shell command processor: %v\n", r)
				shell.isAlive = false
				shell.closeQueue.Do(func() { close(shell.commandQueue) })
			}
		}()
		shell.processCommands()
	}()

	go func() {
		err := cmd.Wait()
		if err != nil {
			// Log the error if needed
		}
		shell.isAlive = false
		// Whatever the dead shell started dies with it, not later and not
		// never (gap #9).
		shell.job.Close()
		shell.stop.close()
		shell.closeQueue.Do(func() { close(shell.commandQueue) })
	}()

	return shell
}

func (s *PersistentShell) processCommands() {
	for cmd := range s.commandQueue {
		result := s.execCommand(cmd.command, cmd.timeout, cmd.ctx)
		cmd.resultChan <- result
	}
}

func (s *PersistentShell) execCommand(command string, timeout time.Duration, ctx context.Context) commandResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isAlive {
		return commandResult{
			stderr:   "Shell is not alive",
			exitCode: 1,
			err:      errors.New("shell is not alive"),
		}
	}

	tempDir := os.TempDir()
	stdoutFile := filepath.Join(tempDir, fmt.Sprintf("gorilla-opencode-stdout-%d", time.Now().UnixNano()))
	stderrFile := filepath.Join(tempDir, fmt.Sprintf("gorilla-opencode-stderr-%d", time.Now().UnixNano()))
	statusFile := filepath.Join(tempDir, fmt.Sprintf("gorilla-opencode-status-%d", time.Now().UnixNano()))
	cwdFile := filepath.Join(tempDir, fmt.Sprintf("gorilla-opencode-cwd-%d", time.Now().UnixNano()))

	defer func() {
		osutil.Remove(stdoutFile)
		osutil.Remove(stderrFile)
		osutil.Remove(statusFile)
		osutil.Remove(cwdFile)
	}()

	var fullCommand string
	if s.isPowerShell {
		// GORILLA OVERRIDE (2026-09-01, rev 2): the PowerShell wrapper.
		//
		// Rev 1 of this wrapper was wrong in four independent ways, each of
		// which was measured on real Windows PowerShell 5.1 before this
		// rewrite. They are listed here because every one of them is a trap
		// that looks correct in review:
		//
		//  1. `Out-File -Encoding utf8` on PowerShell 5.1 writes a UTF-8 BOM.
		//     The BOM landed at the head of EVERY captured file. In the status
		//     file it made `Sscanf("%d")` fail, so every command reported exit
		//     0 — a failing build looked like a passing one. In the cwd file it
		//     produced "C:\..." which was stored as s.cwd, and the next
		//     respawn died with `chdir: The filename, directory name, or volume
		//     label syntax is incorrect`. That is the "Shell could not be
		//     started" the user saw, and the nil dereference behind three
		//     recorded panics. WriteAllText with UTF8Encoding($false) is the
		//     only reliable way to get BOM-less UTF-8 out of 5.1.
		//
		//  2. `& {...} | Out-File x 2> y` binds the `2>` to Out-File, not to
		//     the block, so the block's errors went to the shell's own stderr —
		//     which is discarded. The stderr file was ALWAYS empty. Merging
		//     with 2>&1 and re-splitting on ErrorRecord is what actually
		//     separates the two streams.
		//
		//  3. `$LASTEXITCODE` is set only by native executables, and it PERSISTS
		//     across commands in a shell this long-lived. Left alone, command N
		//     inherits command N-1's exit code. It is reset to $null first, and
		//     only trusted when a native command actually set it; a pure-cmdlet
		//     failure is detected from the error stream instead.
		//
		//  4. Object output (Get-ChildItem and friends) is rendered by the
		//     default formatter at the host's console width, which truncates
		//     columns. `Out-String -Width 4096` renders it wide instead.
		//
		// The status file is written LAST, on purpose: the watcher below treats
		// its existence as "the command is done", so anything written after it
		// would be a race.
		//
		// GORILLA FIX (2026-10-11, rev 3): the command is no longer pasted into
		// the wrapper's own text. It travels base64-encoded, is PARSED first,
		// and only then runs as a script block. Measured on Windows PowerShell
		// 5.1.26100 with `cd C:\x && python -m fieldkit --help`, three times in
		// one Nemotron session:
		//
		//   - `&&` is a parse error in 5.1. PowerShell reported it at 320 ms —
		//     on the shell's OWN stderr, which nobody reads — and then threw
		//     away the whole chunk it had been handed, the four WriteAllText
		//     lines included. No status file ever appeared, the watcher waited
		//     the full 60 s timeout, stopRunning found no child to kill and
		//     killed the shell itself, and the model was told "timed out",
		//     which is the one diagnosis that was not true.
		//   - an unterminated quote is worse: the parser waits for the rest of
		//     the string for ever, so every later chunk is swallowed into it.
		//   - a terminating error (`throw`, a .NET exception) at the top of the
		//     block aborted the chunk the same way: output lost, no status.
		//
		// With the command parsed from a string, a syntax error is an ordinary
		// error record with the parser's own message, exit code 2, at once; a
		// throw is caught, the output collected before it is kept, and the
		// shell lives on with its working directory intact. Set-Location inside
		// a script block still moves the session, exactly as `& { }` did.
		//
		// GORILLA FIX (2026-10-27, rev 4, gap #10): the whole wrapper is one
		// try/finally. An interrupt sets the session's stop event, GorillaStop
		// stops the running pipeline (what Ctrl+C does at a console), and the
		// finally block still writes the four files, with exit code 130 and
		// only .NET calls, because a cmdlet called while the pipeline is
		// stopping is itself stopped. The session survives with its variables,
		// venv and directory; $__stopped tells the two ends apart.
		fullCommand = fmt.Sprintf(`
$__stopped = $true
$__enc = New-Object System.Text.UTF8Encoding $false
$__err = New-Object System.Collections.ArrayList
$__outl = New-Object System.Collections.ArrayList
try {
$ErrorActionPreference = 'Continue'
$global:LASTEXITCODE = $null
if ('GorillaStop' -as [type]) { [GorillaStop]::Begin() }
$__src = $__enc.GetString([System.Convert]::FromBase64String('%s'))
$__tok = $null
$__perr = $null
[void][System.Management.Automation.Language.Parser]::ParseInput($__src, [ref]$__tok, [ref]$__perr)
if ($__perr -ne $null -and $__perr.Count -gt 0) {
  foreach ($__e in $__perr) {
    [void]$__err.Add(("PowerShell could not parse the command (line " + $__e.Extent.StartLineNumber + ", column " + $__e.Extent.StartColumnNumber + "): " + $__e.Message + [char]10))
  }
  $__code = 2
} else {
  $__code = $null
  $__in = [IntPtr]::Zero
  try {
    $__sb = [scriptblock]::Create($__src)
    if ('GorillaStdin' -as [type]) { $__in = [GorillaStdin]::Detach() }
    & $__sb 2>&1 | ForEach-Object {
      if ($_ -is [System.Management.Automation.ErrorRecord]) {
        [void]$__err.Add(($_ | Out-String -Width 4096))
      } else { [void]$__outl.Add($_) }
    }
  } catch {
    [void]$__err.Add(($_ | Out-String -Width 4096))
    $__code = 1
  } finally {
    if ($__in -ne [IntPtr]::Zero) { [GorillaStdin]::Restore($__in) }
  }
  if ($__code -eq $null) {
    $__code = if ($LASTEXITCODE -ne $null) { $LASTEXITCODE } elseif ($__err.Count -gt 0) { 1 } else { 0 }
  }
}
$__o = ($__outl | Out-String -Width 4096)
$__stopped = $false
} finally {
$__req = $false
if ('GorillaStop' -as [type]) { $__req = [GorillaStop]::Requested(); [GorillaStop]::End() }
if ($__stopped) {
  $__o = [string]::Join([string][char]10, [object[]]$__outl.ToArray())
  if ($__req) {
    [void]$__err.Add("`+stoppedMarker+`")
    $__code = 130
  } else { $__code = '`+unwoundStatus+`' }
}
[System.IO.File]::WriteAllText(%s, $__o, $__enc)
[System.IO.File]::WriteAllText(%s, [string]::Join('', [object[]]$__err.ToArray()), $__enc)
[System.IO.File]::WriteAllText(%s, $ExecutionContext.SessionState.Path.CurrentLocation.Path, $__enc)
[System.IO.File]::WriteAllText(%s, [string]$__code, $__enc)
}
`,
			base64.StdEncoding.EncodeToString([]byte(command)),
			shellQuotePS(stdoutFile),
			shellQuotePS(stderrFile),
			shellQuotePS(cwdFile),
			shellQuotePS(statusFile),
		)
	} else {
		// GORILLA OVERRIDE (2026-09-01, rev 2): the bash wrapper, restored.
		//
		// Rev 1 added `set -e` and dropped `< /dev/null`. Both were regressions
		// on the platform that already worked, and both are measured:
		//
		//   set -e     — this shell is non-interactive and reads its commands
		//                from a pipe, so the FIRST non-zero exit terminates the
		//                shell itself. A `grep` that finds nothing (exit 1) killed
		//                the session, the status file was never written, and the
		//                NEXT command never ran either. The repo's own
		//                TestOrdinaryFailuresReportTheRealErrorAndExitCode caught
		//                this: it expects exit 2 and 127 and got 1 with "the shell
		//                session ended".
		//
		//   < /dev/null — without it, a command that reads stdin reads the SHELL's
		//                stdin, which is the command stream. A bare `cat` swallowed
		//                the `pwd`/`echo $?` protocol lines that follow it and the
		//                session desynchronised. It was load-bearing.
		//
		// The `{ ... }` group (rather than rev 0's `eval`) is kept: it is what
		// lets a multi-line command work without re-quoting, and the group's
		// status is still the command's status.
		fullCommand = fmt.Sprintf(`
{
%s
} < /dev/null > %s 2> %s
EXEC_EXIT_CODE=$?
pwd > %s
echo $EXEC_EXIT_CODE > %s
`,
			command,
			shellQuote(stdoutFile),
			shellQuote(stderrFile),
			shellQuote(cwdFile),
			shellQuote(statusFile),
		)
	}

	// GORILLA FIX (2026-10-05): a command that was cancelled while it waited in
	// the queue must not be run. The context was only looked at AFTER the
	// command had been written to the shell.
	if ctx.Err() != nil {
		return commandResult{stderr: "The command was cancelled before it started.", exitCode: 1, interrupted: true}
	}
	_, err := s.stdin.Write([]byte(fullCommand + "\n"))
	if err != nil {
		return commandResult{
			stderr:   fmt.Sprintf("Failed to write command to shell: %v", err),
			exitCode: 1,
			err:      err,
		}
	}

	interrupted := false
	sessionKept := false
	// GORILLA FIX (2026-08-19), tool audit: a command containing `exit` ends
	// THIS shell, not a subshell.
	//
	// The command runs through `eval`, which executes in the current shell —
	// so `exit 1`, or a script ending `|| exit 1`, terminates the persistent
	// shell itself. The status file is then never written, the watcher below
	// polls for it until the FULL TIMEOUT elapses (one minute by default), and
	// the result comes back as "Command execution timed out or was
	// interrupted".
	//
	// Every part of that is wrong except the delay. The command did not time
	// out, it was not interrupted, and it very often succeeded — its output is
	// sitting in the stdout file. A model told "timed out" retries, or raises
	// the timeout, or reports a hang to the user. It fixes the wrong thing,
	// having waited a minute to be misled.
	//
	// Measured before this: `echo out; exit 1` took the full timeout and
	// reported a timeout. After: it returns at once and says what happened.
	shellExited := false

	startTime := time.Now()

	done := make(chan bool)
	go func() {
		for {
			select {
			case <-ctx.Done():
				sessionKept = s.stopRunning(statusFile)
				interrupted = true
				done <- true
				return

			case <-time.After(10 * time.Millisecond):
				if fileExists(statusFile) && fileSize(statusFile) > 0 {
					done <- true
					return
				}
				// The shell died before writing its status. Stop waiting for a
				// file that is never coming.
				if !s.isAlive {
					shellExited = true
					done <- true
					return
				}

				if timeout > 0 {
					elapsed := time.Since(startTime)
					if elapsed > timeout {
						sessionKept = s.stopRunning(statusFile)
						interrupted = true
						done <- true
						return
					}
				}
			}
		}
	}()

	<-done

	stdout := readFileOrEmpty(stdoutFile)
	stderr := readFileOrEmpty(stderrFile)
	exitCodeStr := readFileOrEmpty(statusFile)
	newCwd := readFileOrEmpty(cwdFile)

	// The wrapper was left without a stop being asked for: `exit` (which
	// ends this shell; the finally block still runs on the way out) or a
	// top-level break/continue (which does not). Which one shows in whether
	// the shell is still there a moment later.
	if strings.TrimSpace(exitCodeStr) == unwoundStatus {
		exitCodeStr = ""
		for deadline := time.Now().Add(unwoundGrace); s.isAlive && time.Now().Before(deadline); {
			time.Sleep(25 * time.Millisecond)
		}
		if s.isAlive {
			exitCodeStr = "1"
			stderr += "\nThe command left early (a break or continue outside a loop); the session is unchanged.\n"
		} else {
			shellExited = true
		}
	}

	exitCode := 0
	if exitCodeStr != "" {
		fmt.Sscanf(exitCodeStr, "%d", &exitCode)
	} else if shellExited {
		// Named accurately. The output above is real and complete up to the
		// point the shell ended; only the exit code was lost with it.
		exitCode = 1
		stderr += "\nThe shell session ended while running this command — the usual cause is an " +
			"`exit` in the command itself, which ends this shell rather than a subshell. " +
			"Any output above is real. The exit code could not be recorded. " +
			"Drop the `exit` (use `false`, or let the last command's own status stand) and a " +
			"fresh shell will be started for the next command."
	} else if interrupted {
		exitCode = 143
		stderr += "\nCommand execution timed out or was interrupted"
	}
	// Gap #10: say whether the interrupt cost the session, so the model
	// knows whether to set its variables or venv up again.
	if interrupted {
		stderr += "\n" + interruptNote(sessionKept)
	}
	// The parser names the token; the model needs to be told what to write
	// instead, or it tries `&&` again with different spacing.
	if s.isPowerShell && strings.Contains(stderr, "is not a valid statement separator") {
		stderr += "This shell is Windows PowerShell 5.1: separate commands with ';' — '&&' and '||' do not exist here.\n"
	}

	// GORILLA OVERRIDE (2026-09-01): only believe a working directory that is
	// actually a directory.
	//
	// s.cwd is not just reported — it is where the shell is RESPAWNED after it
	// dies. One unusable value here (a BOM, a truncated write, a half-flushed
	// file) turns a recoverable death into a permanent one: every respawn fails
	// with chdir, GetPersistentShell hands back nil, and the bash tool is dead
	// for the rest of the session. Keeping the last known-good directory is
	// always better than adopting a broken one.
	if cleaned := strings.TrimSpace(newCwd); cleaned != "" {
		if st, err := os.Stat(cleaned); err == nil && st.IsDir() {
			s.cwd = cleaned
		}
	}

	return commandResult{
		stdout:      stdout,
		stderr:      stderr,
		exitCode:    exitCode,
		interrupted: interrupted,
	}
}

// stopRunning ends whatever the shell is doing, for a cancel or a timeout.
//
// GORILLA FIX (2026-10-05): killing the shell's CHILD processes stops a program
// the shell started. It does nothing to work the shell is doing ITSELF: a
// PowerShell cmdlet (Remove-Item -Recurse, Get-ChildItem -Recurse, Start-Sleep)
// or a bash builtin loop has no child to kill. The person pressed Esc, was told
// "interrupted", and the delete carried on; the next command then queued behind
// it and burned its own timeout waiting.
//
// So: kill the children, give the shell a moment to write its status, and if it
// has not, the shell itself is what is busy. It is killed and marked dead, and
// the next command gets a fresh one (GetPersistentShell). The working directory
// is carried over; shell variables set earlier are lost, which is the price of
// the stop actually stopping.
//
// GORILLA FIX (2026-10-27, gap #10): on Windows PowerShell the stop event is
// set first, which stops a busy cmdlet the way Ctrl+C does and keeps the
// session. Only a shell that still does not answer is killed, with its whole
// job. The result says which of the two happened.
func (s *PersistentShell) stopRunning(statusFile string) (sessionKept bool) {
	s.killChildren()
	s.stop.fire()
	deadline := time.Now().Add(stopGrace)
	for time.Now().Before(deadline) {
		if !s.isAlive {
			return false
		}
		if fileExists(statusFile) && fileSize(statusFile) > 0 {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	s.isAlive = false
	if s.job.Terminate() != nil && s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return false
}

// stopGrace is how long an interrupted shell has to finish its own wrapper.
// A stopped PowerShell pipeline needs a few hundred milliseconds to unwind.
const stopGrace = 2 * time.Second

// stoppedMarker is the line the PowerShell wrapper adds when its pipeline was
// stopped rather than finished.
const stoppedMarker = "The command was stopped before it finished."

// unwoundStatus is what the PowerShell wrapper writes as its status when it
// was left without a stop: an `exit` or a stray break/continue.
const unwoundStatus = "unwound"

// unwoundGrace is how long an `exit` takes to end PowerShell after the
// wrapper's finally block has run.
const unwoundGrace = 2 * time.Second

// interruptNote tells the model whether an interrupt cost the session.
func interruptNote(kept bool) string {
	if kept {
		return "The shell session was kept: variables, an activated venv and the working directory are unchanged."
	}
	return "The shell itself was busy and had to be ended: the next command starts a fresh shell in the same " +
		"directory, but variables and an activated venv set up earlier are gone."
}

func (s *PersistentShell) killChildren() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}

	if runtime.GOOS == "windows" {
		// GORILLA OVERRIDE (2026-09-01, rev 2): kill the CHILDREN, not the shell.
		//
		// Rev 1 ran `taskkill /T /F /PID <shell>`, and /T kills the named process
		// along with its tree — so every interrupt and every timeout destroyed the
		// persistent shell itself. Everything the session had set up went with it:
		// exported variables, an activated virtualenv, the directory the user had
		// cd'd into. On Unix the same function is careful to signal only children
		// (pkill -P), so Ctrl+C there costs you a command and here it cost you the
		// session. One Ctrl+C should not be a factory reset.
		//
		// GORILLA FIX (2026-10-27, gap #9): with the shell in a Job Object,
		// everything in the job but the shell (and its console host) is
		// killed. That reaches a grandchild whose parent already exited,
		// which a walk down from the shell's children cannot find.
		if _, err := s.job.KillAllBut(s.cmd.Process.Pid); err == nil {
			return
		}
		// No job (it could not be made): children are enumerated directly
		// rather than shelled out to, because this runs on the interrupt
		// path where a 300ms process launch is felt.
		for _, pid := range childPIDs(s.cmd.Process.Pid) {
			exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", pid)).Run()
		}
		return
	}

	// GORILLA OVERRIDE (2026-09-01): On Unix, use pkill -P for modern systems,
	// fallback to pgrep for older systems that don't have pkill.
	if _, err := exec.LookPath("pkill"); err == nil {
		exec.Command("pkill", "-P", fmt.Sprintf("%d", s.cmd.Process.Pid)).Run()
		return
	}

	pgrepCmd := exec.Command("pgrep", "-P", fmt.Sprintf("%d", s.cmd.Process.Pid))
	output, err := pgrepCmd.Output()
	if err != nil {
		return
	}

	for pidStr := range strings.SplitSeq(string(output), "\n") {
		if pidStr = strings.TrimSpace(pidStr); pidStr != "" {
			var pid int
			fmt.Sscanf(pidStr, "%d", &pid)
			if pid > 0 {
				proc, err := os.FindProcess(pid)
				if err == nil {
					proc.Signal(syscall.SIGTERM)
				}
			}
		}
	}
}

func (s *PersistentShell) Exec(ctx context.Context, command string, timeoutMs int) (string, string, int, bool, error) {
	if !s.isAlive {
		return "", "Shell is not alive", 1, false, errors.New("shell is not alive")
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond

	resultChan := make(chan commandResult)
	// The shell can die between the check above and this send (a command that
	// ran `exit`); sending on the closed queue would panic the whole program.
	if !s.enqueue(&commandExecution{
		command:    command,
		timeout:    timeout,
		resultChan: resultChan,
		ctx:        ctx,
	}) {
		return "", "Shell is not alive", 1, false, errors.New("shell is not alive")
	}

	result := <-resultChan
	return result.stdout, result.stderr, result.exitCode, result.interrupted, result.err
}

func (s *PersistentShell) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isAlive {
		return
	}

	s.stdin.Write([]byte("exit\n"))

	// The process waiter in newPersistentShell then closes the shell's job,
	// which takes the tree with it (gap #9): a server it started in the
	// background does not outlive the session.
	s.cmd.Process.Kill()
	s.isAlive = false
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func shellQuotePS(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// looksLikePowerShell reports whether a shell path names PowerShell.
func looksLikePowerShell(shellPath string) bool {
	p := strings.ToLower(shellPath)
	return strings.Contains(p, "powershell") || strings.Contains(p, "pwsh")
}

// resolveShellPath picks the shell to drive when config names none.
//
// GORILLA OVERRIDE (2026-09-01): on Windows, PowerShell comes FIRST. The
// upstream order consulted $SHELL and fell back to /bin/bash, which on Windows
// means the bash tool tried to start a path that does not exist. $SHELL is still
// honoured on Windows, but only when it actually names PowerShell — Git Bash and
// MSYS both export a Unix-shaped $SHELL that has nothing to do with what this
// process can execute, and letting that win is how the port ended up sending
// bash syntax to PowerShell.
//
// Split out of newPersistentShell so IsWindowsPowerShell can ask which shell
// WOULD be used without starting one.
func resolveShellPath() string {
	if runtime.GOOS != "windows" {
		if s := os.Getenv("SHELL"); s != "" {
			return s
		}
		return "/bin/bash"
	}

	// 1. An explicit PowerShell in $SHELL.
	if s := os.Getenv("SHELL"); s != "" && looksLikePowerShell(s) {
		return s
	}
	// 2. PowerShell itself: 7+ if present, else the 5.1 that ships with Windows.
	if _, err := exec.LookPath("pwsh.exe"); err == nil {
		return "pwsh.exe"
	}
	if _, err := exec.LookPath("powershell.exe"); err == nil {
		return "powershell.exe"
	}
	// 3. An explicit opt-out, for people who really do want bash on Windows.
	if s := os.Getenv("SHELL_FALLBACK"); s != "" {
		return s
	}
	// 4. Git Bash, as a last resort.
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, "scoop", "apps", "git", "current", "bin", "bash.exe"),
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("bash.exe"); err == nil {
		return p
	}
	// Nothing found. Return PowerShell's name anyway so the failure message
	// names something real rather than /bin/bash.
	return "powershell.exe"
}

// startShell launches the shell process and returns its stdin pipe. Split out of
// newPersistentShell so the caller can retry with a different directory.
//
// On Windows the shell starts inside its own Job Object (gap #9): everything
// it starts, and everything those start, is in the job, so an interrupt can
// kill the whole tree but the shell, and closing the job (or this program
// crashing) kills all of it. The job is nil where none could be made.
func startShell(shellPath string, shellArgs []string, cwd string) (*exec.Cmd, *procjob.Job, *os.File, error) {
	cmd := exec.Command(shellPath, shellArgs...)
	cmd.Dir = cwd
	cmd.Env = shellChildEnv(append(os.Environ(), "GIT_EDITOR=true"), runtime.GOOS == "windows")

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	// StdinPipe returns an *os.File wrapped so Close is idempotent; the shell
	// wants the raw file to write to. Assert rather than cast so a future Go
	// change surfaces here instead of as a nil write later.
	f, ok := stdinPipe.(*os.File)
	if !ok {
		type filer interface{ File() *os.File }
		if wrapped, okw := stdinPipe.(filer); okw {
			f = wrapped.File()
		} else {
			// Not started yet, so there is no process to kill.
			return nil, nil, nil, fmt.Errorf("shell stdin is %T, not an *os.File", stdinPipe)
		}
	}
	job, err := procjob.Start(cmd)
	if err != nil {
		return nil, nil, nil, err
	}
	return cmd, job, f, nil
}

// PowerShellArgs is how every PowerShell we start begins: no profile, and
// the execution policy bypassed for THIS process only (gap #3). Under the
// default Restricted policy the .ps1 shims that npm, npx, a venv's
// Activate.ps1 and build.ps1 scripts are made of refuse to run. The machine
// and user policy are never changed (no Set-ExecutionPolicy); a policy set
// by Group Policy still wins, as it should.
func PowerShellArgs(rest ...string) []string {
	return append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass"}, rest...)
}

// shellChildEnv adds what every child of the shell needs on Windows (gap
// #4): Python writes UTF-8 to pipes instead of the ANSI code page
// (PYTHONUTF8, PEP 540; PYTHONIOENCODING for Python before 3.7). A value
// the user set already is kept.
func shellChildEnv(env []string, windows bool) []string {
	if !windows {
		return env
	}
	for _, kv := range []string{"PYTHONUTF8=1", "PYTHONIOENCODING=utf-8"} {
		key := kv[:strings.IndexByte(kv, '=')+1]
		set := false
		for _, e := range env {
			if len(e) >= len(key) && strings.EqualFold(e[:len(key)], key) {
				set = true
				break
			}
		}
		if !set {
			env = append(env, kv)
		}
	}
	return env
}

// psSessionSetup runs once, before the first command, in every Windows
// PowerShell session the bash tool drives.
//
// UTF-8 (gap #4): PowerShell decodes a native program's output with
// [Console]::OutputEncoding, which defaults to the OEM code page (437/850),
// so git's and cargo's UTF-8 came out as mojibake; $OutputEncoding is what
// it encodes text piped INTO a native program with (ASCII on 5.1).
//
// stdin (gap #5): this shell reads its commands from its stdin, and a
// native program inherits that handle. Measured on 5.1.26100: `python -c
// "import sys; sys.stdin.read()"` swallowed every command sent after it.
// `$null |` in front of the block does not help (the native command inside
// is not the pipeline's first element). GorillaStdin swaps the process's
// standard input for NUL around each command (the wrapper's try/finally)
// and puts it back before PowerShell reads its next line; PowerShell
// fetches the handle on every read, which is also why the swap must sit
// inside the one statement the wrapper is.
//
// Interrupt (gap #10): GorillaStop waits on the session's stop event (see
// stop_windows.go) on a background thread and, while a command runs, stops
// the running pipeline. Runspace.GetCurrentlyRunningPipeline is internal in
// PowerShell 5.1, so it is reached by reflection; if a PowerShell version
// lacks it, nothing is stopped and stopRunning ends the shell as before and
// says so.
const psSessionSetup = `try { [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding $false } catch { }
try { [Console]::InputEncoding = New-Object System.Text.UTF8Encoding $false } catch { }
$global:OutputEncoding = New-Object System.Text.UTF8Encoding $false
if (-not ('GorillaStdin' -as [type])) { try { Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public static class GorillaStdin { [StructLayout(LayoutKind.Sequential)] struct SA { public int nLength; public IntPtr sd; public int inherit; } [DllImport("kernel32.dll")] static extern IntPtr GetStdHandle(int n); [DllImport("kernel32.dll")] static extern bool SetStdHandle(int n, IntPtr h); [DllImport("kernel32.dll", CharSet = CharSet.Unicode)] static extern IntPtr CreateFileW(string name, uint access, uint share, ref SA sa, uint disposition, uint flags, IntPtr template); static IntPtr nul = IntPtr.Zero; public static IntPtr Detach() { if (nul == IntPtr.Zero) { SA sa = new SA(); sa.nLength = Marshal.SizeOf(sa); sa.inherit = 1; IntPtr h = CreateFileW("NUL", 0x80000000, 3, ref sa, 3, 0, IntPtr.Zero); if (h == new IntPtr(-1)) { return IntPtr.Zero; } nul = h; } IntPtr old = GetStdHandle(-10); if (!SetStdHandle(-10, nul)) { return IntPtr.Zero; } return old; } public static void Restore(IntPtr old) { SetStdHandle(-10, old); } }' } catch { } }
if (-not ('GorillaStop' -as [type])) { try { Add-Type -TypeDefinition 'using System; using System.Reflection; using System.Threading; using System.Management.Automation.Runspaces; public static class GorillaStop { static volatile bool busy; static Runspace rs; static EventWaitHandle ev; public static void Arm(Runspace r, string name) { if (ev != null) { return; } rs = r; ev = new EventWaitHandle(false, EventResetMode.AutoReset, name); Thread t = new Thread(Loop); t.IsBackground = true; t.Start(); } static volatile bool requested; public static void Begin() { requested = false; busy = true; } public static void End() { busy = false; } public static bool Requested() { return requested; } static void Loop() { MethodInfo m = typeof(Runspace).GetMethod("GetCurrentlyRunningPipeline", BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic); while (true) { ev.WaitOne(); if (!busy || m == null) { continue; } try { Pipeline p = m.Invoke(rs, null) as Pipeline; if (p != null) { requested = true; p.StopAsync(); } } catch { } } } }' } catch { } }
`

// HookPowerShell is the PowerShell a lifecycle hook runs in (gap #18): the
// bash tool's own when that is PowerShell 7 (pwsh), so a hook written for the
// shell the user chose runs in it; otherwise Windows PowerShell 5.1.
func HookPowerShell() string {
	p := ""
	if cfg := config.Get(); cfg != nil {
		p = cfg.Shell.Path
	}
	if p == "" {
		p = resolveShellPath()
	}
	if strings.Contains(strings.ToLower(filepath.Base(p)), "pwsh") {
		return p
	}
	return "powershell.exe"
}

// IsWindowsPowerShell reports whether the bash tool will drive PowerShell.
//
// GORILLA OVERRIDE (2026-09-01): this must NOT start a shell. Its only caller is
// bashDescription(), which runs while the tool set is being built — so the first
// version of this function spawned a PowerShell process as a side effect of
// composing a help string, at a point where config.WorkingDirectory() may not be
// resolved yet. GetPersistentShell caches the directory it is first called with
// for the life of the process, so that side effect could pin the session's shell
// to the wrong folder. Deciding from the configured/detected path answers the
// same question without launching anything.
func IsWindowsPowerShell() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	// A shell already running is the authoritative answer, but only if one
	// exists — asking must not create it.
	shellMu.Lock()
	existing := shellInstance
	shellMu.Unlock()
	if existing != nil && existing.isAlive {
		return existing.isPowerShell
	}
	return looksLikePowerShell(resolveShellPath())
}

// stripBOM removes a leading byte-order mark.
//
// GORILLA OVERRIDE (2026-09-01): defence in depth for the Windows shell. The
// wrapper above now writes BOM-less UTF-8 itself, but a user-configured shell
// (config `shell.path`), a PowerShell profile, or any future edit that reaches
// for Out-File can put one back. A BOM is invisible in a terminal and in a diff,
// and it silently broke both the exit code and the working directory for the
// entire Windows port — so it is stripped on the way in as well, where it cannot
// be reintroduced by accident.
//
// UTF-16 marks are matched too: PowerShell 5.1's plain `>` redirection emits
// UTF-16LE, which is how the err.txt in the project root came to be unreadable.
// Those files are not decoded here, but detecting the mark means the caller sees
// mojibake rather than a stray control character at the head of the string.
func stripBOM(b []byte) []byte {
	switch {
	case len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF:
		return b[3:]
	case len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE:
		return b[2:]
	case len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF:
		return b[2:]
	}
	return b
}

func readFileOrEmpty(path string) string {
	content, err := os.ReadFile(path)
	content = stripBOM(content)
	if err != nil {
		return ""
	}
	return string(content)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// Cwd is the session's working directory as last reported by the shell.
// It waits for a running command to finish, because that command may move it.
func (s *PersistentShell) Cwd() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cwd
}
