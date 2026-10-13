package agent

// GORILLA OVERRIDE (2026-10-12): calls the PROVIDER already ran.
//
// The Claude CLI provider spawns a child that runs its own agent loop, so its
// tool calls are finished before the turn ends. They come back in
// ProviderResponse.ExecutedToolCalls rather than ToolCalls, because ToolCalls
// means "please run these" and running them again would do everything twice.
//
// THE BUG THESE EXIST FOR. EventComplete calls SetToolCalls(Response.ToolCalls),
// which OVERWRITES whatever the streamed tool-use events recorded. With
// ToolCalls deliberately empty, every call the child had made was wiped from the
// message before it was stored. Measured 2026-10-12 through the built binary:
// the model read note.txt, quoted its contents correctly, and the receipt
// printed "No tool was called. The answer above was written without running,
// reading or checking anything."
//
// The receipt is the one thing in this program the model cannot write. A receipt
// that denies work that happened is worse than no receipt, because it is
// believed.

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
)

// Calls the provider already ran must reach the stored session, with their
// results, so the receipt can report them.
func TestExecutedToolCallsAreRecorded(t *testing.T) {
	p := &scriptedProvider{script: []reply{{
		text:        "The secret word is PINEAPPLE.",
		executed:    []message.ToolCall{call("bridge_view_1", "view")},
		executedRes: []message.ToolResult{{ToolCallID: "bridge_view_1", Name: "view", Content: "the secret word is PINEAPPLE"}},
	}}}
	a, msgs := newLoopAgent(t, config.AgentCoder, p)

	runTurn(t, a, "s1", "read the note")

	stored, err := msgs.List(t.Context(), "s1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var gotCalls []message.ToolCall
	var gotResults []message.ToolResult
	for _, m := range stored {
		gotCalls = append(gotCalls, m.ToolCalls()...)
		gotResults = append(gotResults, m.ToolResults()...)
	}

	if len(gotCalls) != 1 {
		t.Fatalf("the session recorded %d tool calls, want 1; the receipt would say nothing ran", len(gotCalls))
	}
	if gotCalls[0].Name != "view" {
		t.Errorf("recorded call is %q, want view", gotCalls[0].Name)
	}
	if len(gotResults) != 1 {
		t.Fatalf("the session recorded %d tool results, want 1; the receipt would show no outcome", len(gotResults))
	}
	if gotResults[0].ToolCallID != gotCalls[0].ID {
		t.Errorf("the result does not pair with the call: %q vs %q", gotResults[0].ToolCallID, gotCalls[0].ID)
	}
}

// They must be RECORDED, never DISPATCHED. The child already ran them; running
// them again would repeat every edit and every command in the turn.
func TestExecutedToolCallsAreNotRunAgain(t *testing.T) {
	tool := &scriptedTool{name: "view"}
	p := &scriptedProvider{script: []reply{{
		text:        "done",
		executed:    []message.ToolCall{call("bridge_view_1", "view")},
		executedRes: []message.ToolResult{{ToolCallID: "bridge_view_1", Name: "view", Content: "contents"}},
	}}}
	a, _ := newLoopAgent(t, config.AgentCoder, p, tool)

	runTurn(t, a, "s2", "read the note")

	if n := tool.count(); n != 0 {
		t.Errorf("the tool ran %d times in this process; the child had already run it, so this is every edit and command done twice", n)
	}
	// One pass only: a second trip to the model would mean the loop treated the
	// finished calls as outstanding work.
	p.mu.Lock()
	asked := p.asked
	p.mu.Unlock()
	if asked != 1 {
		t.Errorf("the model was called %d times, want 1; the loop treated finished calls as work to do", asked)
	}
}
