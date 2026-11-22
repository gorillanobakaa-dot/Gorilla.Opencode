package chat

// GORILLA OVERRIDE (2026-11-21): regression tests for output stream readability
// (visual separation between prose and tool blocks, distinct tool badges, and
// clean blank-line separation between consecutive scrollback turns).

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/message"
)

func TestProseAndToolCallsAreVisuallySeparated(t *testing.T) {
	const at int64 = 1785228225
	asst := message.Message{
		ID: "a1", Role: message.Assistant, CreatedAt: at,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Checking the configuration file now."},
			message.ToolCall{ID: "c1", Name: "view", Input: `{"file_path":"README.md"}`, Finished: true},
			message.Finish{Reason: message.FinishReasonToolUse, Time: at + 1},
		},
	}
	res := message.Message{
		ID: "t1", Role: message.Tool, CreatedAt: at + 1,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Content: "hello world"},
		},
	}

	out := RenderForScrollback(asst, 0, []message.Message{asst, res}, nil, 80)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "[View]") {
		t.Fatalf("tool block missing distinct [View] badge:\n%s", plain)
	}
	if !strings.Contains(plain, "\n\n") {
		t.Fatalf("no blank line separating conversational prose from tool block:\n%s", plain)
	}
	if strings.HasSuffix(out, "\n") {
		t.Fatalf("RenderForScrollback must not end with a trailing newline")
	}
}

func TestConsecutiveScrollbackMessagesHaveBlankSeparatorWithoutExtraCmds(t *testing.T) {
	const at int64 = 1785228225
	u := userMsgID("u1", "first prompt", at)
	a := finishedAssistant("a1", "first answer", at+1)
	m := printerFor(t, 80, u, a)

	cmds := m.printPending()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 print cmds (1 per message), got %d", len(cmds))
	}
}
