package agent

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

func lastRequestHas(p *scriptedProvider, i int, needle string) bool {
	for _, m := range p.history[i] {
		if strings.Contains(m.Content().String(), needle) {
			return true
		}
	}
	return false
}

// A message typed while a tool runs reaches the model at the next boundary,
// marked as the owner's steer, and the next request carries it.
func TestSteerTypedMidRunIsInjectedAtTheToolBoundary(t *testing.T) {
	var a *agent
	tool := &scriptedTool{name: "slow_tool"}
	tool.do = func() (tools.ToolResponse, error) {
		QueueUserMessage(a, "steer1", "use dmesg instead")
		return tools.NewTextResponse("ran slow_tool"), nil
	}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{call("c1", "slow_tool")}},
		{text: "switching to dmesg"},
	}}
	a, _ = newLoopAgent(t, config.AgentCoder, p, tool)
	ev := runTurn(t, a, "steer1", "start")
	if ev.Error != nil {
		t.Fatal(ev.Error)
	}
	if p.asked != 2 {
		t.Fatalf("model called %d times", p.asked)
	}
	if !lastRequestHas(p, 1, "use dmesg instead") || !lastRequestHas(p, 1, "OWNER STEER") {
		t.Error("the second model call did not carry the marked steer")
	}
}

// Esc stops the running tool only: the run goes on and the model is told to ask.
func TestEscStopsTheToolNotTheRun(t *testing.T) {
	var sid = "esc1"
	tool := &scriptedTool{name: "slow_tool"}
	tool.do = func() (tools.ToolResponse, error) {
		if !InterruptTool(sid) {
			t.Error("no running tool to interrupt")
		}
		return tools.NewTextResponse("partial"), nil
	}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{call("c1", "slow_tool"), call("c2", "slow_tool")}},
		{text: "continue or change?"},
	}}
	a, _ := newLoopAgent(t, config.AgentCoder, p, tool)
	ev := runTurn(t, a, sid, "go")
	if ev.Error != nil {
		t.Fatal(ev.Error)
	}
	if tool.count() != 1 {
		t.Errorf("tool ran %d times; the second call must not run after Esc", tool.count())
	}
	found := false
	for _, m := range p.history[1] {
		for _, r := range m.ToolResults() {
			found = found || strings.Contains(r.Content, "owner pressed Esc")
		}
	}
	if !found {
		t.Error("the model was not told Esc stopped the tool")
	}
}

func TestStopWord(t *testing.T) {
	for _, s := range []string{"stop", "Stop.", " STOP! ", "stop it"} {
		if !IsStopWord(s) {
			t.Errorf("%q must stop", s)
		}
	}
	for _, s := range []string{"stop using grep", "don't stop", "stopwatch"} {
		if IsStopWord(s) {
			t.Errorf("%q is a steer, not a stop", s)
		}
	}
}
