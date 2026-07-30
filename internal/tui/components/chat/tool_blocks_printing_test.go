package chat

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// THE BUG: a tool message stopped the transcript forever.
//
// ScrollbackReady returned false for anything that was not a User or Assistant
// message, and printPending BREAKS on the first message that is not ready — so
// the first tool result halted printing permanently. Every later message,
// including the model's finished answer, was generated, stored and never shown.
//
// Observed 2026-07-30: a bash call returned in 2 seconds, the model answered
// fully, and the screen sat on "Waiting for response..." for fifteen minutes.
// It looked like the provider had hung. Nothing had hung; the printer had.
func TestAToolResultDoesNotStopTheTranscript(t *testing.T) {
	const at int64 = 1785228225

	// The exact shape of a tool-using turn.
	assistantWithCall := message.Message{
		ID: "a1", Role: message.Assistant, CreatedAt: at,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "c1", Name: "bash", Input: "{}", Finished: true},
			message.Finish{Reason: message.FinishReasonToolUse},
		},
	}
	toolResult := message.Message{
		ID: "t1", Role: message.Tool, CreatedAt: at + 1,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Content: "/usr/bin/rg"},
		},
	}
	finalAnswer := message.Message{
		ID: "a2", Role: message.Assistant, CreatedAt: at + 2,
		Parts: []message.ContentPart{
			message.TextContent{Text: "ripgrep is installed at /usr/bin/rg"},
			message.Finish{Reason: message.FinishReasonEndTurn},
		},
	}

	m := printerFor(t, 100, assistantWithCall, toolResult, finalAnswer)
	out := strings.Join(plainLines(m.printPending()), "\n")

	if !strings.Contains(out, "ripgrep is installed") {
		t.Fatalf("the model's finished answer was never printed — a tool result "+
			"halted the transcript.\nprinted:\n%s", out)
	}
	if !m.printed["a2"] {
		t.Error("the final answer was not marked printed, so it never will be")
	}
}

// A tool result must be CONSUMED, not merely skipped: it has to be marked
// printed, or the loop reconsiders it on every update forever.
func TestToolResultsAreConsumedRatherThanBlocking(t *testing.T) {
	const at int64 = 1785228225
	tool := message.Message{
		ID: "t1", Role: message.Tool, CreatedAt: at,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Content: "output"},
		},
	}
	if !ScrollbackReady(tool) {
		t.Fatal("a tool result is a finished artefact the moment it exists; " +
			"treating it as unsettled blocks everything behind it")
	}

	m := printerFor(t, 100, tool)
	m.printPending()
	if !m.printed["t1"] {
		t.Error("tool result not marked printed; it will be reconsidered forever")
	}
}

// It must not print anything ITSELF — tool output reaches the screen through the
// assistant message that owns it, so printing it here would duplicate it.
func TestToolResultsPrintNothingOfTheirOwn(t *testing.T) {
	const at int64 = 1785228225
	tool := message.Message{
		ID: "t1", Role: message.Tool, CreatedAt: at,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Content: "UNIQUE-TOOL-OUTPUT"},
		},
	}
	m := printerFor(t, 100, tool)
	out := strings.Join(plainLines(m.printPending()), "\n")
	if strings.Contains(out, "UNIQUE-TOOL-OUTPUT") {
		t.Error("the tool result printed itself; it would appear twice, since the " +
			"owning assistant message renders it too")
	}
}
