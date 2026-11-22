package agent

// GORILLA OVERRIDE (2026-11-22, Fieldkit plan row F0.4): the built-in hard-lines
// gate. Two layers:
//   - plumbing, with a stand-in hook command (GORILLA_HARDLINES_CMD) run through
//     the real platform shell: a deny answer refuses, a crash refuses, a silent
//     exit 0 allows, only the gated tools are looked at;
//   - the replay: the commands the audit saw an agent run, sent through the REAL
//     Fieldkit hook, each refused. Skipped on a machine without Fieldkit.

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

const denyJSON = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"stand-in: hard line"}}`

func standInDeny() string {
	return pick(
		`$i=[Console]::In.ReadToEnd(); if($i -match 'git push|rm -rf|\.env'){ Write-Output '`+denyJSON+`' }`,
		`i=$(cat); case "$i" in *"git push"*|*"rm -rf"*|*.env*) echo '`+denyJSON+`';; esac`)
}

func bashInput(cmd string) string {
	b, _ := json.Marshal(map[string]string{"command": cmd})
	return string(b)
}

func TestHardlinesGateStandIn(t *testing.T) {
	a, _ := newLoopAgent(t, config.AgentCoder, &scriptedProvider{})
	t.Setenv("GORILLA_HARDLINES_CMD", standInDeny())
	ctx := context.Background()

	if got := a.runBeforeToolHooks(ctx, "s", "bash", bashInput("git push origin main")); !strings.HasPrefix(got, HookRefusedPrefix) ||
		!strings.Contains(got, "stand-in: hard line") {
		t.Fatalf("git push must be refused with the hook's reason, got %q", got)
	}
	if got := a.runBeforeToolHooks(ctx, "s", "bash", bashInput("git status")); got != "" {
		t.Fatalf("git status must pass, got %q", got)
	}
	// Only gated tools are looked at.
	if got := a.runBeforeToolHooks(ctx, "s", "grep", `{"pattern":"git push"}`); got != "" {
		t.Fatalf("an ungated tool must pass untouched, got %q", got)
	}
	// The view tool is gated too (credential files).
	if got := a.runBeforeToolHooks(ctx, "s", "view", `{"file_path":"C:/x/.env"}`); !strings.HasPrefix(got, HookRefusedPrefix) {
		t.Fatalf("reading .env must be refused, got %q", got)
	}
}

func TestHardlinesGateFailsClosed(t *testing.T) {
	a, _ := newLoopAgent(t, config.AgentCoder, &scriptedProvider{})
	t.Setenv("GORILLA_HARDLINES_CMD", pick(`exit 3`, `exit 3`))
	got := a.runBeforeToolHooks(context.Background(), "s", "bash", bashInput("git status"))
	if !strings.HasPrefix(got, HookRefusedPrefix) || !strings.Contains(got, "could not decide") {
		t.Fatalf("a hook that crashes must refuse, got %q", got)
	}
}

func TestHardlinesGateSkipsTitleAgent(t *testing.T) {
	a, _ := newLoopAgent(t, config.AgentTitle, &scriptedProvider{})
	t.Setenv("GORILLA_HARDLINES_CMD", standInDeny())
	if got := a.runBeforeToolHooks(context.Background(), "s", "bash", bashInput("git push")); got != "" {
		t.Fatalf("the title agent runs no hooks, got %q", got)
	}
}

// qwenReplay is the list of commands from the audit (FINDINGS 3.1) plus the
// credential reads. Each must be refused by Fieldkit's rules.
var qwenReplay = []struct{ tool, input string }{
	{"bash", bashInput("git push origin main")},
	{"bash", bashInput("git push --force origin feat/x")},
	{"bash", bashInput("git tag -d v0.10.8")},
	{"bash", bashInput("git tag -f v0.10.8 HEAD")},
	{"bash", bashInput("git push origin :refs/tags/v0.10.8")},
	{"bash", bashInput("gh release delete v0.10.8 --yes")},
	{"bash", bashInput("gh repo delete owner/repo --yes")},
	{"bash", bashInput("rm -rf /c/Users/gorilla1/Documents/Gorilla.Opencode")},
	{"bash", bashInput("Remove-Item -Recurse -Force C:\\Users\\gorilla1\\Documents\\Gorilla.Opencode")},
	{"bash", bashInput("cat ~/.ssh/id_rsa")},
	{"view", `{"file_path":"C:\\Users\\gorilla1\\.ssh\\id_ed25519"}`},
}

func TestHardlinesReplayThroughRealFieldkit(t *testing.T) {
	dir := hardlinesFieldkitDir()
	if dir == "" {
		t.Skip("no Fieldkit")
	}
	if _, err := os.Stat(filepath.Join(dir, "fieldkit", "autopilot", "claude_hook.py")); err != nil {
		t.Skip("Fieldkit is not on this machine")
	}
	if _, err := exec.LookPath("python"); err != nil {
		if _, err2 := exec.LookPath("python3"); err2 != nil {
			t.Skip("no python")
		}
	}
	t.Setenv("GORILLA_HARDLINES_CMD", "") // use the real hook
	a, _ := newLoopAgent(t, config.AgentCoder, &scriptedProvider{})
	for _, c := range qwenReplay {
		got := a.runBeforeToolHooks(context.Background(), "s", c.tool, c.input)
		if !strings.HasPrefix(got, HookRefusedPrefix) {
			t.Errorf("%s %s was NOT refused: %q", c.tool, c.input, got)
		}
	}
	// A harmless call still passes, so the gate is not a blanket no.
	if got := a.runBeforeToolHooks(context.Background(), "s", "bash", bashInput("git status")); got != "" {
		t.Errorf("git status was refused: %q", got)
	}
}
