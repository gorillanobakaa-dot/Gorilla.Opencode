package agent

// GORILLA OVERRIDE (2026-10-09): runs the user's lifecycle hooks.
//
// The hooks themselves are declared and validated in internal/config/hooks.go,
// which also says who can act through them (only whoever can write
// config.json) and why a malformed one stops the load.
//
// # WHAT RUNS, WHERE
//
// One command line through the platform shell — Windows: powershell.exe
// -NoProfile -NonInteractive -Command; elsewhere: sh -c — in the working
// directory, with the agent's own environment plus GORILLA_HOOK_* variables.
// The full raw tool input is also written to the hook's stdin, because an
// environment variable has a size limit (32767 characters per variable on
// Windows) and a gate that saw only part of the input could pass what it
// would have refused. The variable is cut at hookEnvInputMax and
// GORILLA_HOOK_TOOL_INPUT_TRUNCATED=1 says so.
//
// # GATING, FAIL CLOSED
//
// A before_tool hook that exits non-zero REFUSES the call: the tool is not
// run, and the model is told so in a result that names the hook. A hook that
// times out or cannot start refuses too — a gate that could not decide has not
// said yes. after_tool and turn_end hooks cannot change anything; their exit
// codes are logged.
//
// # WHICH AGENTS
//
// The coder, sub-agents (task) and research helpers share this loop and all
// run hooks. The title and summarizer agents never do: they make no tool calls
// and are not turns the user asked for.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/message"
)

const (
	hookOutputMax   = 8 * 1024  // bytes of stdout+stderr kept (the tail)
	hookResultMax   = 8 * 1024  // bytes of tool result put in GORILLA_HOOK_TOOL_RESULT
	hookEnvInputMax = 24 * 1024 // bytes of tool input put in GORILLA_HOOK_TOOL_INPUT
	hookRefusalTail = 2000      // characters of hook output quoted to the model
)

// HookRefusedPrefix opens every result of a call refused by a before_tool
// hook. The receipt (internal/app/receipt.go) reads it.
const HookRefusedPrefix = "Refused by the user's before_tool hook"

// hookOutcome is what one hook run produced.
type hookOutcome struct {
	exitCode int    // -1 when it did not exit normally
	output   string // tail of stdout+stderr, at most hookOutputMax bytes
	timedOut bool
	startErr error // the command could not be started at all
}

// passed reports whether a gating hook said yes. Only a clean exit 0 does.
func (o hookOutcome) passed() bool {
	return o.startErr == nil && !o.timedOut && o.exitCode == 0
}

// hooksApply says whether this agent runs hooks at all.
func (a *agent) hooksApply() bool {
	return a.agentName != config.AgentTitle && a.agentName != config.AgentSummarizer
}

// hooksFor returns the configured hooks for an event (and tool, for tool
// events), in config order.
func hooksFor(event, tool string) []config.Hook {
	var out []config.Hook
	for _, h := range config.Hooks() {
		if h.Event != event {
			continue
		}
		if event != config.HookTurnEnd && !h.AppliesToTool(tool) {
			continue
		}
		out = append(out, h)
	}
	return out
}

// hookShell returns the platform shell and its arguments for one command line.
func hookShell(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell.exe", []string{"-NoProfile", "-NonInteractive", "-Command", command}
	}
	return "sh", []string{"-c", command}
}

// tailBuffer keeps the last max bytes written to it. Safe for the two copier
// goroutines exec starts for stdout and stderr.
type tailBuffer struct {
	mu  sync.Mutex
	max int
	b   []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if over := len(t.b) - t.max; over > 0 {
		t.b = append(t.b[:0:0], t.b[over:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.ToValidUTF8(string(t.b), "")
}

// runHook runs one hook command and waits for it, bounded by the hook's
// timeout. The whole process tree is killed when the time runs out.
func runHook(ctx context.Context, h config.Hook, env []string, stdin string) hookOutcome {
	timeout := time.Duration(h.Timeout()) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell, args := hookShell(h.Command)
	cmd := exec.CommandContext(ctx, shell, args...)
	if wd := hookWorkingDir(); wd != "" {
		cmd.Dir = wd
	}
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	out := &tailBuffer{max: hookOutputMax}
	cmd.Stdout, cmd.Stderr = out, out
	prepareHookProcess(cmd)
	cmd.Cancel = func() error { return killHookTree(cmd) }
	// A grandchild that inherited the pipes can hold them open after the shell
	// is gone; without a bound, Wait would block on them indefinitely.
	cmd.WaitDelay = 3 * time.Second

	err := cmd.Run()
	o := hookOutcome{exitCode: -1, output: out.String()}
	if ctx.Err() == context.DeadlineExceeded {
		o.timedOut = true
		return o
	}
	if err == nil {
		o.exitCode = 0
		return o
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		o.exitCode = exitErr.ExitCode()
		return o
	}
	o.startErr = err
	return o
}

func hookWorkingDir() string {
	defer func() { _ = recover() }() // config not loaded: run in the process's own directory
	return config.WorkingDirectory()
}

// hookToolEnv builds the GORILLA_HOOK_* variables shared by the tool events.
func hookToolEnv(event, sessionID, tool, input string) []string {
	envInput, truncated := input, false
	if len(envInput) > hookEnvInputMax {
		envInput, truncated = strings.ToValidUTF8(envInput[:hookEnvInputMax], ""), true
	}
	env := []string{
		"GORILLA_HOOK_EVENT=" + event,
		"GORILLA_HOOK_TOOL=" + tool,
		"GORILLA_HOOK_TOOL_INPUT=" + envInput,
		"GORILLA_HOOK_SESSION=" + sessionID,
	}
	if truncated {
		env = append(env, "GORILLA_HOOK_TOOL_INPUT_TRUNCATED=1")
	}
	return env
}

// runBeforeToolHooks runs every matching before_tool hook in config order. It
// returns "" when all passed, or the refusal text for the model at the first
// one that did not. Later hooks are not run once one has refused.
func (a *agent) runBeforeToolHooks(ctx context.Context, sessionID, tool, input string) string {
	if !a.hooksApply() {
		return ""
	}
	for _, h := range hooksFor(config.HookBeforeTool, tool) {
		o := runHook(ctx, h, hookToolEnv(config.HookBeforeTool, sessionID, tool, input), input)
		if o.passed() {
			logging.Info("before_tool hook allowed the call", "tool", tool, "command", h.Command, "agent", string(a.agentName))
			continue
		}
		why := ""
		switch {
		case o.timedOut:
			why = fmt.Sprintf("it did not finish within %d seconds and was stopped", h.Timeout())
		case o.startErr != nil:
			why = fmt.Sprintf("it could not be started (%v)", o.startErr)
		default:
			why = fmt.Sprintf("it exited with code %d", o.exitCode)
		}
		logging.Warn("before_tool hook refused the call", "tool", tool, "command", h.Command,
			"reason", why, "agent", string(a.agentName))
		tail := strings.TrimSpace(o.output)
		if r := []rune(tail); len(r) > hookRefusalTail {
			tail = "..." + string(r[len(r)-hookRefusalTail:])
		}
		if tail == "" {
			tail = "(the hook printed nothing)"
		}
		return fmt.Sprintf("%s `%s`: %s. The %s tool was NOT run. Hook output:\n%s",
			HookRefusedPrefix, h.Command, why, tool, tail)
	}
	return ""
}

// runAfterToolHooks runs every matching after_tool hook. Exit codes are logged
// only: the call has already happened.
func (a *agent) runAfterToolHooks(ctx context.Context, sessionID, tool, input, result string, isError bool) {
	if !a.hooksApply() {
		return
	}
	hs := hooksFor(config.HookAfterTool, tool)
	if len(hs) == 0 {
		return
	}
	// Credentials are masked by the same rule that masks them for the model:
	// a notification hook may forward this text somewhere.
	result, _ = tools.MaskSecrets(tool, result)
	if len(result) > hookResultMax {
		result = strings.ToValidUTF8(result[:hookResultMax], "")
	}
	errFlag := "0"
	if isError {
		errFlag = "1"
	}
	env := append(hookToolEnv(config.HookAfterTool, sessionID, tool, input),
		"GORILLA_HOOK_TOOL_RESULT="+result,
		"GORILLA_HOOK_TOOL_ERROR="+errFlag)
	for _, h := range hs {
		a.logHookOutcome(config.HookAfterTool, h, runHook(ctx, h, env, input))
	}
}

// runTurnEndHooks runs every turn_end hook once. Its own context: the turn's
// is already cancelled by the time this runs.
func (a *agent) runTurnEndHooks(sessionID, finishReason string) {
	if !a.hooksApply() {
		return
	}
	env := []string{
		"GORILLA_HOOK_EVENT=" + config.HookTurnEnd,
		"GORILLA_HOOK_SESSION=" + sessionID,
		"GORILLA_HOOK_FINISH_REASON=" + finishReason,
	}
	for _, h := range hooksFor(config.HookTurnEnd, "") {
		a.logHookOutcome(config.HookTurnEnd, h, runHook(context.Background(), h, env, ""))
	}
}

// turnFinishReason is GORILLA_HOOK_FINISH_REASON for a finished turn: the
// final message's finish reason, or "canceled" / "error" when the turn failed.
func turnFinishReason(result AgentEvent) string {
	if result.Error != nil {
		if errors.Is(result.Error, ErrRequestCancelled) || errors.Is(result.Error, context.Canceled) {
			return string(message.FinishReasonCanceled)
		}
		return string(message.FinishReasonError)
	}
	if r := result.Message.FinishReason(); r != "" {
		return string(r)
	}
	return string(message.FinishReasonEndTurn)
}

func (a *agent) logHookOutcome(event string, h config.Hook, o hookOutcome) {
	switch {
	case o.timedOut:
		logging.Warn("hook timed out and was stopped", "event", event, "command", h.Command, "agent", string(a.agentName))
	case o.startErr != nil:
		logging.Warn("hook could not be started", "event", event, "command", h.Command, "error", o.startErr, "agent", string(a.agentName))
	case o.exitCode != 0:
		logging.Warn("hook exited non-zero", "event", event, "command", h.Command, "exit", o.exitCode, "agent", string(a.agentName))
	default:
		logging.Debug("hook ran", "event", event, "command", h.Command, "agent", string(a.agentName))
	}
}

// killHookTree stops the hook and everything it started.
func killHookTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		// Same approach as the review tool's toolkit runner.
		if exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run() == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
	return killProcessGroup(cmd)
}
