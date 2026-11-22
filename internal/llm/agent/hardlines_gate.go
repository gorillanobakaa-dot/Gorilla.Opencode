package agent

// GORILLA OVERRIDE (2026-11-22, Fieldkit plan row F0.4): the built-in hard-lines
// gate.
//
// WHY. The Fieldkit audit of 2026-11-14..22 found that OpenCode agents pushed,
// moved tags, deleted and read credential files, which Claude sessions are
// stopped from doing by Fieldkit's hard-lines hook. The user's own before_tool
// hooks (hooks.go) cannot cover this: they exist only if someone wrote them.
//
// WHAT. Before every bash / write / edit / patch / view call this gate runs
// Fieldkit's own hook, `python -m fieldkit.autopilot.claude_hook`, the same
// program Claude Code runs. Fieldkit's rules (hardlines.py, toolrouter.py) stay
// the single source: nothing is copied here. The call is handed over in
// Claude's hook shape ({tool_name, tool_input, cwd, session_id}) on stdin; a
// {"hookSpecificOutput":{"permissionDecision":"deny",...}} answer refuses.
//
// FAIL CLOSED. Once the gate is on, a hook that cannot start, times out, exits
// non-zero or prints something unreadable refuses the call, like a user hook.
//
// WHEN ON. GORILLA_HARDLINES_CMD (a full command line, used as is) or a
// Fieldkit checkout: GORILLA_FIELDKIT_DIR, else <home>/Documents/Scripts/Fieldkit.
// Neither present means a machine without Fieldkit: the gate is off, because
// there are no rules to call. There is no switch to turn it off when it is on.
// The setting is read from the process environment, which the agent's bash
// tool cannot change.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/logging"
)

const hardlinesTimeoutSeconds = 30

// hardlinesToolName maps an OpenCode tool to the Claude tool name Fieldkit's
// rules know. "" means the gate does not look at that tool.
func hardlinesToolName(tool string) string {
	switch tool {
	case "bash":
		return "Bash"
	case "write":
		return "Write"
	case "edit", "patch":
		return "Edit"
	case "view":
		return "Read"
	}
	return ""
}

func hardlinesFieldkitDir() string {
	if dir := os.Getenv("GORILLA_FIELDKIT_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Documents", "Scripts", "Fieldkit")
}

// hardlinesCommand returns the command line that runs Fieldkit's hook, or ""
// when the gate is off on this machine.
func hardlinesCommand() string {
	if c := strings.TrimSpace(os.Getenv("GORILLA_HARDLINES_CMD")); c != "" {
		return c
	}
	dir := hardlinesFieldkitDir()
	if dir == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(dir, "fieldkit", "autopilot", "claude_hook.py")); err != nil {
		return ""
	}
	py := os.Getenv("GORILLA_HARDLINES_PYTHON")
	if py == "" {
		for _, n := range []string{"python", "python3", "py"} {
			if p, err := exec.LookPath(n); err == nil {
				py = p
				break
			}
		}
	}
	if py == "" {
		// Fieldkit is here but cannot be run: refuse, never skip.
		return "exit 127"
	}
	if runtime.GOOS == "windows" {
		return "& '" + strings.ReplaceAll(py, "'", "''") + "' -m fieldkit.autopilot.claude_hook"
	}
	return "'" + strings.ReplaceAll(py, "'", "'\\''") + "' -m fieldkit.autopilot.claude_hook"
}

// hardlinesVerdict reads the hook's answer: deny plus the reason.
func hardlinesVerdict(out string) (deny bool, reason string) {
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var v struct {
			H struct {
				D string `json:"permissionDecision"`
				R string `json:"permissionDecisionReason"`
			} `json:"hookSpecificOutput"`
		}
		if json.Unmarshal([]byte(line), &v) != nil {
			continue
		}
		if v.H.D == "deny" {
			return true, v.H.R
		}
	}
	return false, ""
}

// runHardlinesGate returns "" when the call may run, else the refusal text.
func (a *agent) runHardlinesGate(ctx context.Context, sessionID, tool, input string) string {
	claudeTool := hardlinesToolName(tool)
	if claudeTool == "" {
		return ""
	}
	cmdline := hardlinesCommand()
	if cmdline == "" {
		return ""
	}
	var args map[string]any
	if json.Unmarshal([]byte(input), &args) != nil || args == nil {
		args = map[string]any{"raw": input}
	}
	payload, _ := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       claudeTool,
		"tool_input":      args,
		"cwd":             hookWorkingDir(),
		"session_id":      sessionID,
	})
	env := hookToolEnv(config.HookBeforeTool, sessionID, tool, input)
	if dir := hardlinesFieldkitDir(); dir != "" {
		env = append(env, "PYTHONPATH="+dir)
	}
	h := config.Hook{Event: config.HookBeforeTool, Command: cmdline, TimeoutSeconds: hardlinesTimeoutSeconds}
	o := runHook(ctx, h, env, string(payload))
	if deny, why := hardlinesVerdict(o.output); deny {
		logging.Warn("hard-lines gate refused the call", "tool", tool, "reason", why, "agent", string(a.agentName))
		return fmt.Sprintf("%s (Fieldkit hard lines): %s The %s tool was NOT run. Do not retry another way: use `fieldkit ship push` to push, `fieldkit size recycle PATH` to delete, and never read credential files.",
			HookRefusedPrefix, why, tool)
	}
	if !o.passed() {
		why := fmt.Sprintf("exit code %d", o.exitCode)
		switch {
		case o.timedOut:
			why = fmt.Sprintf("no answer within %d seconds", hardlinesTimeoutSeconds)
		case o.startErr != nil:
			why = fmt.Sprintf("could not start (%v)", o.startErr)
		}
		logging.Warn("hard-lines gate could not decide; refused", "tool", tool, "reason", why)
		return fmt.Sprintf("%s (Fieldkit hard lines): the gate could not decide (%s), so the call is refused. The %s tool was NOT run. Output:\n%s",
			HookRefusedPrefix, why, tool, strings.TrimSpace(o.output))
	}
	return ""
}
