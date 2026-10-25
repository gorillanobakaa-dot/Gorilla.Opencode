package chat

// GORILLA OVERRIDE (2026-10-25): a claimed action with no tool call behind it
// is flagged on screen, under the answer, which is left as written.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/message"
)

func claimTurn(withCall bool) []message.Message {
	msgs := []message.Message{{ID: "u", SessionID: "s", Role: message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "delete the pngs"}}}}
	if withCall {
		msgs = append(msgs,
			message.Message{ID: "a1", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{
				message.ToolCall{ID: "c1", Name: "bash", Input: `{"command":"del /Q *.png"}`, Finished: true},
				message.Finish{Reason: message.FinishReasonToolUse}}},
			message.Message{ID: "t1", SessionID: "s", Role: message.Tool, Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "c1", Name: "bash", Content: ""}}})
	}
	return append(msgs, message.Message{ID: "a2", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{
		message.TextContent{Text: "All files were deleted."},
		message.Finish{Reason: message.FinishReasonEndTurn}}})
}

func TestTheScreenFlagsAClaimWithNoToolCall(t *testing.T) {
	msgs := claimTurn(false)
	last := len(msgs) - 1
	out := ansi.Strip(RenderForScrollback(msgs[last], last, msgs, nil, 220))
	if !strings.Contains(out, app.ClaimWarningPrefix+"The answer says files were deleted, but no tool ran this turn: nothing was deleted.") {
		t.Fatalf("no warning on screen:\n%s", out)
	}
	if !strings.Contains(out, "All files were deleted.") {
		t.Fatalf("the answer itself must stay on screen:\n%s", out)
	}
	if strings.Index(out, "All files were deleted.") > strings.Index(out, app.ClaimWarningPrefix) {
		t.Fatalf("the warning must come after the answer:\n%s", out)
	}
}

func TestTheScreenDoesNotFlagAClaimBackedByACall(t *testing.T) {
	msgs := claimTurn(true)
	last := len(msgs) - 1
	if out := ansi.Strip(RenderForScrollback(msgs[last], last, msgs, nil, 220)); strings.Contains(out, app.ClaimWarningPrefix) {
		t.Fatalf("a turn whose call succeeded was flagged:\n%s", out)
	}
}

func TestOnlyTheMessageThatEndsTheTurnIsChecked(t *testing.T) {
	m := message.Message{ID: "a", SessionID: "s", Role: message.Assistant, Parts: []message.ContentPart{
		message.TextContent{Text: "All files were deleted."},
		message.Finish{Reason: message.FinishReasonCanceled}}}
	if line := claimWarningLine(m, 0, []message.Message{m}); line != "" {
		t.Fatalf("a cancelled message was checked: %q", line)
	}
	// A stale index is found again by id.
	msgs := claimTurn(false)
	if line := claimWarningLine(msgs[len(msgs)-1], 0, msgs); line == "" {
		t.Fatal("a wrong index lost the warning")
	}
}
