package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

type PersistentShell struct {
	cmd          *exec.Cmd
	stdin        *os.File
	isAlive      bool
	cwd          string
	mu           sync.Mutex
	commandQueue chan *commandExecution
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
		shellPath = os.Getenv("SHELL")
		if shellPath == "" {
			shellPath = "/bin/bash"
		}
	}

	// Default shell args
	if len(shellArgs) == 0 {
		shellArgs = []string{"-l"}
	}

	cmd := exec.Command(shellPath, shellArgs...)
	cmd.Dir = cwd

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}

	cmd.Env = append(os.Environ(), "GIT_EDITOR=true")

	err = cmd.Start()
	if err != nil {
		return nil
	}

	shell := &PersistentShell{
		cmd:          cmd,
		stdin:        stdinPipe.(*os.File),
		isAlive:      true,
		cwd:          cwd,
		commandQueue: make(chan *commandExecution, 10),
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
		os.Remove(stdoutFile)
		os.Remove(stderrFile)
		os.Remove(statusFile)
		os.Remove(cwdFile)
	}()

	fullCommand := fmt.Sprintf(`
eval %s < /dev/null > %s 2> %s
EXEC_EXIT_CODE=$?
pwd > %s
echo $EXEC_EXIT_CODE > %s
`,
		shellQuote(command),
		shellQuote(stdoutFile),
		shellQuote(stderrFile),
		shellQuote(cwdFile),
		shellQuote(statusFile),
	)

	_, err := s.stdin.Write([]byte(fullCommand + "\n"))
	if err != nil {
		return commandResult{
			stderr:   fmt.Sprintf("Failed to write command to shell: %v", err),
			exitCode: 1,
			err:      err,
		}
	}

	interrupted := false

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
	} else if interrupted {
		exitCode = 143
		stderr += "\nCommand execution timed out or was interrupted"
	}

	if newCwd != "" {
		s.cwd = strings.TrimSpace(newCwd)
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

func readFileOrEmpty(path string) string {
	content, err := os.ReadFile(path)
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
