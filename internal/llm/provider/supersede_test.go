package provider

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

func bigFind(id, args string) message.Message {
	return message.Message{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: id, Name: "find", Input: args},
		},
	}
}

func findResult(id, content string) message.Message {
	return message.Message{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: id, Name: "find", Content: content},
		},
	}
}

// A read-only call repeated with identical arguments: the earlier result is
// collapsed to a stub, the later one is kept in full, and the tool_call/
// tool_result pairing survives (every result still has its call, no orphans).
func TestSupersedeCollapsesIdenticalReadResult(t *testing.T) {
	big := strings.Repeat("x", 3000)
	msgs := []message.Message{
		bigFind("c1", `{"path":"."}`),
		findResult("c1", big),
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "and again"}}},
		bigFind("c2", `{"path":"."}`), // byte-identical arguments
		findResult("c2", big),
	}
	out := supersedeStaleReads(msgs)

	first := out[1].ToolResults()[0].Content
	last := out[4].ToolResults()[0].Content
	if !strings.Contains(first, "superseded") {
		t.Errorf("earlier identical result should be stubbed, got %d chars", len(first))
	}
	if last != big {
		t.Error("the newest copy must be kept in full")
	}
	// Pairing: every tool_result still maps to a tool_call id present in history.
	callIDs := map[string]bool{"c1": true, "c2": true}
	for _, m := range out {
		for _, tr := range m.ToolResults() {
			if !callIDs[tr.ToolCallID] {
				t.Errorf("orphaned tool_result %q — pairing broken", tr.ToolCallID)
			}
		}
	}
}

// Different arguments are not duplicates: nothing is collapsed.
func TestSupersedeKeepsDifferentArguments(t *testing.T) {
	big := strings.Repeat("y", 3000)
	msgs := []message.Message{
		bigFind("c1", `{"path":".","view":"long"}`),
		findResult("c1", big),
		bigFind("c2", `{"path":".","view":"tree"}`), // different view flag
		findResult("c2", big),
	}
	out := supersedeStaleReads(msgs)
	if out[1].ToolResults()[0].Content != big {
		t.Error("different arguments must not be treated as a duplicate")
	}
}

// Side-effectful tools (bash/edit/write) are never collapsed, even when the
// arguments are byte-identical — the output is not reproducible from the args.
func TestSupersedeNeverTouchesBash(t *testing.T) {
	big := strings.Repeat("z", 3000)
	call := message.Message{Role: message.Assistant, Parts: []message.ContentPart{
		message.ToolCall{ID: "b1", Name: "bash", Input: `{"command":"date"}`}}}
	res := message.Message{Role: message.Tool, Parts: []message.ContentPart{
		message.ToolResult{ToolCallID: "b1", Name: "bash", Content: big}}}
	call2 := message.Message{Role: message.Assistant, Parts: []message.ContentPart{
		message.ToolCall{ID: "b2", Name: "bash", Input: `{"command":"date"}`}}}
	res2 := message.Message{Role: message.Tool, Parts: []message.ContentPart{
		message.ToolResult{ToolCallID: "b2", Name: "bash", Content: big}}}
	out := supersedeStaleReads([]message.Message{call, res, call2, res2})
	if out[1].ToolResults()[0].Content != big {
		t.Error("bash output must never be collapsed — it is not reproducible from its arguments")
	}
}

// The stored input is not mutated: supersede must copy Parts, never write
// through the shared slice into the session store.
func TestSupersedeDoesNotMutateInput(t *testing.T) {
	big := strings.Repeat("x", 3000)
	msgs := []message.Message{
		bigFind("c1", `{"path":"."}`),
		findResult("c1", big),
		bigFind("c2", `{"path":"."}`),
		findResult("c2", big),
	}
	_ = supersedeStaleReads(msgs)
	if msgs[1].ToolResults()[0].Content != big {
		t.Error("supersede mutated the caller's message — must operate on a copy")
	}
}
