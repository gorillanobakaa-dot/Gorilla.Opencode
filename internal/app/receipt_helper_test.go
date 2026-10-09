package app

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// Run on Gemini Flash with role=coder (2026-10-09): the helper rewrote app.py
// and ran python, and the receipt said only "agent ... -> ok".
func TestAHelpersEditsAndCommandsAreOnTheReceipt(t *testing.T) {
	parent := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "call-agent", Name: "agent", Input: `{"prompt":"fix app.py","role":"coder"}`},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "call-agent", Name: "agent", Content: "done"},
		}},
	}
	helper := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "h1", Name: "edit", Input: `{"file_path":"C:\\p\\app.py"}`},
			message.ToolCall{ID: "h2", Name: "bash", Input: `{"command":"python -c \"import app\""}`},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "h1", Name: "edit", Content: "edited"},
			message.ToolResult{ToolCallID: "h2", Name: "bash", Content: "Exit code 1"},
		}},
	}
	r := BuildReceiptWithHelpers(parent, func(id string) []message.Message {
		if id == "call-agent" {
			return helper
		}
		return nil
	})
	if r.Calls != 1 || r.HelperCalls != 2 {
		t.Fatalf("calls %d helper %d, want 1 and 2", r.Calls, r.HelperCalls)
	}
	if r.Problems != 1 {
		t.Errorf("the helper's failed command must count as a problem: %d", r.Problems)
	}
	text := r.Text()
	for _, want := range []string{
		"1 tool call plus 2 by helpers, 1 did not succeed:",
		"  agent  fix app.py  -> ok",
		"    | edit   C:\\p\\app.py  -> ok",
		"    | bash   python -c \"import app\"  -> exit code 1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("receipt lacks %q:\n%s", want, text)
		}
	}
	if plain := BuildReceipt(parent).Text(); strings.Contains(plain, "helpers") {
		t.Errorf("without a lookup the receipt must not invent helpers:\n%s", plain)
	}
}
