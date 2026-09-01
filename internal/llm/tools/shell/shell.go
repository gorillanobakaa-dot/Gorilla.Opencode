package shell

import (
	"context"
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
)

type PersistentShell struct {
	cmd          *exec.Cmd
	stdin        *os.File
	isAlive      bool
	cwd          string
	mu           sync.Mutex
	commandQueue chan *commandExecution
	isPowerShell bool
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
			shellArgs = []string{"-NoProfile", "-Command", "-"}
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

	cmd, stdinPipe, err := startShell(shellPath, shellArgs, cwd)
	if err != nil && cwd != "" {
		fmt.Fprintf(os.Stderr, "Failed to start shell %s in %q: %v — retrying without a working directory\n", shellPath, cwd, err)
		cwd = ""
		cmd, stdinPipe, err = startShell(shellPath, shellArgs, "")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start shell %s: %v\n", shellPath, err)
		return nil
	}

	shell := &PersistentShell{
		cmd:          cmd,
		stdin:        stdinPipe,
		isAlive:      true,
		cwd:          cwd,
		commandQueue: make(chan *commandExecution, 10),
		isPowerShell: isPowerShell,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Panic in shell command processor: %v\n", r)
				shell.isAlive = false
				close(shell.commandQueue)
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
		close(shell.commandQueue)
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
		fullCommand = fmt.Sprintf(`
$ErrorActionPreference = 'Continue'
$global:LASTEXITCODE = $null
$__enc = New-Object System.Text.UTF8Encoding $false
$__err = New-Object System.Collections.ArrayList
$__out = & {
%s
} 2>&1 | ForEach-Object {
  if ($_ -is [System.Management.Automation.ErrorRecord]) {
    [void]$__err.Add(($_ | Out-String -Width 4096))
  } else { $_ }
}
$__code = if ($LASTEXITCODE -ne $null) { $LASTEXITCODE } elseif ($__err.Count -gt 0) { 1 } else { 0 }
[System.IO.File]::WriteAllText(%s, ($__out | Out-String -Width 4096), $__enc)
[System.IO.File]::WriteAllText(%s, ($__err -join ''), $__enc)
[System.IO.File]::WriteAllText(%s, (Get-Location).Path, $__enc)
[System.IO.File]::WriteAllText(%s, [string]$__code, $__enc)
`,
			command,
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

	_, err := s.stdin.Write([]byte(fullCommand + "\n"))
	if err != nil {
		return commandResult{
			stderr:   fmt.Sprintf("Failed to write command to shell: %v", err),
			exitCode: 1,
			err:      err,
		}
	}

	interrupted := false
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
				s.killChildren()
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
						s.killChildren()
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
		// Children are enumerated directly rather than shelled out to, because
		// this runs on the interrupt path where a 300ms process launch is felt.
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
	s.commandQueue <- &commandExecution{
		command:    command,
		timeout:    timeout,
		resultChan: resultChan,
		ctx:        ctx,
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
func startShell(shellPath string, shellArgs []string, cwd string) (*exec.Cmd, *os.File, error) {
	cmd := exec.Command(shellPath, shellArgs...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "GIT_EDITOR=true")

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdin pipe: %w", err)
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
			cmd.Process.Kill()
			return nil, nil, fmt.Errorf("shell stdin is %T, not an *os.File", stdinPipe)
		}
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return cmd, f, nil
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
