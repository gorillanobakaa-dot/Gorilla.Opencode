package agent

// GORILLA OVERRIDE (2026-10-09): lifecycle hooks, driven through whole turns by
// the stand-in model of loop_harness_test.go and REAL commands through the
// platform shell (powershell.exe on Windows, sh elsewhere). No seam: what is
// tested is what runs.

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

func onWindows() bool { return runtime.GOOS == "windows" }

// pick returns the PowerShell form on Windows and the sh form elsewhere.
func pick(ps, sh string) string {
	if onWindows() {
		return ps
	}
	return sh
}

// setHooks installs hooks on the loaded config for one test. Call AFTER
// newLoopAgent, which loads the config.
func setHooks(t *testing.T, hooks ...config.Hook) {
	t.Helper()
	c := config.Get()
	prev, prevWD := c.Hooks, c.WorkingDir
	c.Hooks = hooks
	// config.Load is cached for the whole test binary, so its working
	// directory is the first test's TempDir, long since deleted. Hooks run
	// there; give them a directory that exists.
	c.WorkingDir = t.TempDir()
	t.Cleanup(func() { c.Hooks, c.WorkingDir = prev, prevWD })
}

// hookLog points $HOOK_OUT at a fresh file the hook commands append to.
func hookLog(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "hook.out")
	t.Setenv("HOOK_OUT", p)
	return p
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

func resultFor(t *testing.T, store *memMessages, sessionID, callID string) message.ToolResult {
	t.Helper()
	all, _ := store.List(context.Background(), sessionID)
	for _, m := range all {
		for _, r := range m.ToolResults() {
			if r.ToolCallID == callID {
				return r
			}
		}
	}
	t.Fatalf("no result stored for %s", callID)
	return message.ToolResult{}
}

func callWith(id, name, input string) message.ToolCall {
	return message.ToolCall{ID: id, Name: name, Input: input, Type: "function", Finished: true}
}

// exit 0 is a yes: the tool runs, and the hook saw the call's input on stdin.
func TestABeforeToolHookThatExitsZeroLetsTheToolRun(t *testing.T) {
	tool := &scriptedTool{name: "bash"}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{callWith("c1", "bash", `{"command":"ls"}`)}},
		{text: "done"},
	}}
	a, store := newLoopAgent(t, config.AgentCoder, p, tool)
	out := hookLog(t)
	setHooks(t, config.Hook{Event: config.HookBeforeTool, Command: pick(
		`$input | Add-Content -Path $env:HOOK_OUT; Add-Content -Path $env:HOOK_OUT -Value ($env:GORILLA_HOOK_EVENT + '|' + $env:GORILLA_HOOK_TOOL + '|' + $env:GORILLA_HOOK_TOOL_INPUT + '|' + $env:GORILLA_HOOK_SESSION); exit 0`,
		`cat >> "$HOOK_OUT"; echo >> "$HOOK_OUT"; echo "$GORILLA_HOOK_EVENT|$GORILLA_HOOK_TOOL|$GORILLA_HOOK_TOOL_INPUT|$GORILLA_HOOK_SESSION" >> "$HOOK_OUT"; exit 0`)})

	if ev := runTurn(t, a, "allow", "list"); ev.Error != nil {
		t.Fatalf("turn failed: %v", ev.Error)
	}
	if tool.count() != 1 {
		t.Fatalf("the tool ran %d times after a passing hook, want 1", tool.count())
	}
	if r := resultFor(t, store, "allow", "c1"); r.IsError || r.Content != "ran bash" {
		t.Errorf("result = %+v", r)
	}
	lines := readLines(t, out)
	want := []string{`{"command":"ls"}`, `before_tool|bash|{"command":"ls"}|allow`}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("the hook saw:\n%s\nwant (stdin, then env):\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

// Non-zero is a no: the tool never runs, the model is told why in the hook's
// own words, and the turn goes on.
func TestABeforeToolHookThatExitsNonZeroRefusesTheCall(t *testing.T) {
	tool := &scriptedTool{name: "bash"}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{callWith("c1", "bash", `{"command":"rm -rf /"}`)}},
		{text: "understood, not doing that"},
	}}
	a, store := newLoopAgent(t, config.AgentCoder, p, tool)
	const cmd = "echo blocked-by-the-gate; exit 3"
	setHooks(t, config.Hook{Event: config.HookBeforeTool, Command: cmd})

	if ev := runTurn(t, a, "refuse", "wipe it"); ev.Error != nil {
		t.Fatalf("turn failed: %v", ev.Error)
	}
	if tool.count() != 0 {
		t.Fatalf("the refused tool RAN %d time(s)", tool.count())
	}
	r := resultFor(t, store, "refuse", "c1")
	if !r.IsError {
		t.Error("a refused call is not flagged as an error result")
	}
	for _, want := range []string{HookRefusedPrefix + " `" + cmd + "`", "exited with code 3", "blocked-by-the-gate", "NOT run"} {
		if !strings.Contains(r.Content, want) {
			t.Errorf("refusal %q does not contain %q", r.Content, want)
		}
	}
	if !strings.HasPrefix(r.Content, HookRefusedPrefix) {
		t.Errorf("the refusal must START with the prefix the receipt reads: %q", r.Content)
	}
	if p.asked != 2 {
		t.Errorf("the model was asked %d times; a hook refusal is reported to it and the turn goes on", p.asked)
	}
}

// The tools list narrows a hook. A gate on bash must not touch view.
func TestTheToolsFilterIsHonoured(t *testing.T) {
	bash := &scriptedTool{name: "bash"}
	view := &scriptedTool{name: "view"}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{call("c1", "view"), call("c2", "bash")}},
		{text: "done"},
	}}
	a, store := newLoopAgent(t, config.AgentCoder, p, bash, view)
	setHooks(t, config.Hook{Event: config.HookBeforeTool, Tools: []string{"bash"}, Command: "exit 1"})

	runTurn(t, a, "filter", "go")
	if view.count() != 1 {
		t.Errorf("view ran %d times; a hook limited to bash must not gate it", view.count())
	}
	if bash.count() != 0 {
		t.Errorf("bash ran %d times; its gate said no", bash.count())
	}
	if r := resultFor(t, store, "filter", "c2"); !strings.HasPrefix(r.Content, HookRefusedPrefix) {
		t.Errorf("bash result = %q", r.Content)
	}
}

// A gate that cannot decide has not said yes.
func TestABeforeToolHookThatTimesOutRefusesTheCall(t *testing.T) {
	tool := &scriptedTool{name: "bash"}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{call("c1", "bash")}},
		{text: "done"},
	}}
	a, store := newLoopAgent(t, config.AgentCoder, p, tool)
	setHooks(t, config.Hook{Event: config.HookBeforeTool, TimeoutSeconds: 1,
		Command: pick("Start-Sleep -Seconds 30", "sleep 30")})

	start := time.Now()
	runTurn(t, a, "slow", "go")
	if took := time.Since(start); took > 15*time.Second {
		t.Errorf("the hook was not stopped at its 1 s timeout; the turn took %s", took)
	}
	if tool.count() != 0 {
		t.Fatal("the tool ran after its gate timed out")
	}
	if r := resultFor(t, store, "slow", "c1"); !strings.Contains(r.Content, "did not finish within 1 seconds") {
		t.Errorf("result = %q", r.Content)
	}
}

// A gate that cannot run refuses too. Two ways: the shell starts but the
// program it names does not exist (a non-zero exit), and the hook process
// cannot be started at all (here: its working directory has gone).
func TestABeforeToolHookThatCannotRunRefuses(t *testing.T) {
	a, _ := newLoopAgent(t, config.AgentCoder, &scriptedProvider{})
	setHooks(t, config.Hook{Event: config.HookBeforeTool, Command: "gorilla-no-such-program-20261009"})
	if got := a.runBeforeToolHooks(context.Background(), "s", "bash", "{}"); !strings.HasPrefix(got, HookRefusedPrefix) {
		t.Errorf("a hook naming a missing program let the call through: %q", got)
	}

	setHooks(t, config.Hook{Event: config.HookBeforeTool, Command: "exit 0"})
	config.Get().WorkingDir = filepath.Join(t.TempDir(), "gone")
	got := a.runBeforeToolHooks(context.Background(), "s", "bash", "{}")
	if !strings.HasPrefix(got, HookRefusedPrefix) || !strings.Contains(got, "could not be started") {
		t.Errorf("a hook that could not start let the call through: %q", got)
	}
}

// after_tool sees the result and whether the tool errored; its exit code
// changes nothing.
func TestAnAfterToolHookReceivesTheResult(t *testing.T) {
	good := &scriptedTool{name: "view"}
	bad := &scriptedTool{name: "edit", do: func() (tools.ToolResponse, error) {
		return tools.NewTextErrorResponse("no such file"), nil
	}}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{call("c1", "view"), call("c2", "edit")}},
		{text: "done"},
	}}
	a, store := newLoopAgent(t, config.AgentCoder, p, good, bad)
	out := hookLog(t)
	setHooks(t, config.Hook{Event: config.HookAfterTool, Command: pick(
		`Add-Content -Path $env:HOOK_OUT -Value ($env:GORILLA_HOOK_EVENT + '|' + $env:GORILLA_HOOK_TOOL + '|' + $env:GORILLA_HOOK_TOOL_ERROR + '|' + $env:GORILLA_HOOK_TOOL_RESULT); exit 7`,
		`echo "$GORILLA_HOOK_EVENT|$GORILLA_HOOK_TOOL|$GORILLA_HOOK_TOOL_ERROR|$GORILLA_HOOK_TOOL_RESULT" >> "$HOOK_OUT"; exit 7`)})

	if ev := runTurn(t, a, "after", "go"); ev.Error != nil {
		t.Fatalf("turn failed: %v", ev.Error)
	}
	want := "after_tool|view|0|ran view\nafter_tool|edit|1|no such file"
	if got := strings.Join(readLines(t, out), "\n"); got != want {
		t.Errorf("after_tool hook saw:\n%s\nwant:\n%s", got, want)
	}
	if r := resultFor(t, store, "after", "c1"); r.Content != "ran view" || r.IsError {
		t.Errorf("an after_tool hook's exit 7 changed the result: %+v", r)
	}
}

// turn_end runs once per turn — not once per round trip — with the reason.
func TestTurnEndRunsOncePerTurn(t *testing.T) {
	tool := &scriptedTool{name: "view"}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{call("c1", "view")}},
		{calls: []message.ToolCall{call("c2", "view")}},
		{text: "first turn done"},
		{text: "second turn done"},
	}}
	a, _ := newLoopAgent(t, config.AgentCoder, p, tool)
	out := hookLog(t)
	setHooks(t, config.Hook{Event: config.HookTurnEnd, Tools: []string{"ignored-for-turn-end"}, Command: pick(
		`Add-Content -Path $env:HOOK_OUT -Value ($env:GORILLA_HOOK_EVENT + '|' + $env:GORILLA_HOOK_SESSION + '|' + $env:GORILLA_HOOK_FINISH_REASON)`,
		`echo "$GORILLA_HOOK_EVENT|$GORILLA_HOOK_SESSION|$GORILLA_HOOK_FINISH_REASON" >> "$HOOK_OUT"`)})

	runTurn(t, a, "turns", "first")
	if got := readLines(t, out); len(got) != 1 || got[0] != "turn_end|turns|end_turn" {
		t.Fatalf("after one turn of three round trips the hook ran: %q", got)
	}
	runTurn(t, a, "turns", "second")
	if got := readLines(t, out); len(got) != 2 {
		t.Fatalf("after two turns the hook ran %d times", len(got))
	}
}

// The title and summarizer agents never run hooks; helpers sharing the loop do.
func TestHooksRunForHelpersButNotForTitleOrSummarizer(t *testing.T) {
	for _, tc := range []struct {
		name  config.AgentName
		gated bool
	}{
		{config.AgentTask, true},
		{config.AgentResearch, true},
		{config.AgentTitle, false},
		{config.AgentSummarizer, false},
	} {
		tool := &scriptedTool{name: "view"}
		p := &scriptedProvider{script: []reply{{calls: []message.ToolCall{call("c1", "view")}}, {text: "done"}}}
		a, _ := newLoopAgent(t, tc.name, p, tool)
		setHooks(t, config.Hook{Event: config.HookBeforeTool, Command: "exit 1"})
		runTurn(t, a, "agent-"+string(tc.name), "go")
		if ran := tool.count() == 1; ran == tc.gated {
			t.Errorf("%s: tool ran=%v, want gated=%v", tc.name, ran, tc.gated)
		}
	}
}
