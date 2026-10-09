package app

// GORILLA OVERRIDE (2026-10-09): a call refused by the user's before_tool hook
// never ran. The receipt must say so, not list it as a tool that failed.

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/message"
)

func TestACallRefusedByAHookIsListedAsRefusedNotFailed(t *testing.T) {
	refusal := agent.HookRefusedPrefix + " `./gate.sh`: it exited with code 1. The bash tool was NOT run. Hook output:\nno rm here"
	r := BuildReceipt([]message.Message{
		assistant("", call("a", "bash", `{"command":"rm -rf build"}`)),
		toolMsg(result("a", refusal, true)),
	})
	if got := r.Lines[0].Outcome; got != "refused by hook, not run" {
		t.Fatalf("a hook refusal was listed as %q", got)
	}
	if strings.Contains(r.Text(), "failed") {
		t.Errorf("the receipt calls a refused call a failure:\n%s", r.Text())
	}
	// Any other tool, same answer: the refusal is the loop's, not the tool's.
	if got := outcomeFor("edit", message.ToolResult{Content: refusal, IsError: true}); got != "refused by hook, not run" {
		t.Errorf("an edit refused by a hook was listed as %q", got)
	}
}
