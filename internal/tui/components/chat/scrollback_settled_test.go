package chat

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// GORILLA: these tests pin the fix for tool-call invisibility, found live on
// the crocodile search (2026-08-17). An assistant message reports finished when
// its STREAM ends, but its tool results arrive in a later message. Printing at
// that moment baked "Waiting for response..." into the terminal's permanent
// scrollback and the real result never appeared anywhere on screen — the user
// could not see what any tool did.

func crocodileTurn(withResult bool) []message.Message {
	const at int64 = 1786942707
	msgs := []message.Message{
		{
			ID: "a1", Role: message.Assistant, CreatedAt: at,
			Parts: []message.ContentPart{
				message.TextContent{Text: "I'll search for the word crocodile."},
				message.ToolCall{ID: "c1", Name: "find", Input: `{"query":"crocodile"}`, Finished: true},
				message.Finish{Reason: message.FinishReasonToolUse},
			},
		},
	}
	if withResult {
		msgs = append(msgs, message.Message{
			ID: "t1", Role: message.Tool, CreatedAt: at + 1,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: "c1",
					Content:    "ipr.c:181 PCI_DEVICE_ID_IBM_CROCODILE",
				},
			},
		})
	}
	return msgs
}

// A tool-using turn must NOT print while its results are still outstanding —
// printed output cannot be corrected afterwards.
func TestToolTurnIsHeldUntilItsResultsExist(t *testing.T) {
	msgs := crocodileTurn(false)

	if ScrollbackSettled(msgs[0], msgs) {
		t.Fatal("assistant message with an unanswered tool call reports settled; " +
			"printing it now would bake 'Waiting for response...' into the terminal forever")
	}

	m := printerFor(t, 100, msgs...)
	out := strings.Join(plainLines(m.printPending()), "\n")
	if strings.Contains(out, "Waiting for response") {
		t.Errorf("'Waiting for response...' reached permanent scrollback:\n%s", out)
	}
}

// Once the result lands, the turn prints WITH it — the user sees what the tool
// was asked and what came back, in the terminal's real history.
func TestToolTurnPrintsWithItsResult(t *testing.T) {
	msgs := crocodileTurn(true)

	if !ScrollbackSettled(msgs[0], msgs) {
		t.Fatal("assistant message with all results present must be settled")
	}

	m := printerFor(t, 100, msgs...)
	out := strings.Join(plainLines(m.printPending()), "\n")
	if !strings.Contains(out, "crocodile") {
		t.Errorf("the tool call is missing from scrollback:\n%s", out)
	}
	if !strings.Contains(out, "PCI_DEVICE_ID_IBM_CROCODILE") {
		t.Errorf("the tool RESULT is missing from scrollback — the user cannot see what the tool returned:\n%s", out)
	}
	if strings.Contains(out, "Waiting for response") {
		t.Errorf("result exists but the placeholder still printed:\n%s", out)
	}
}

// The anti-stall belt: a turn that ended abnormally may never get results for
// every call, and the transcript must not wait forever behind it. This is the
// 2026-07-30 corpse class — a printer that stops permanently looks exactly
// like a hung provider.
func TestAbnormallyFinishedTurnsAreNotHeld(t *testing.T) {
	for _, reason := range []message.FinishReason{
		message.FinishReasonCanceled,
		message.FinishReasonError,
		message.FinishReasonPermissionDenied,
	} {
		msg := message.Message{
			ID: "a1", Role: message.Assistant, CreatedAt: 1786942707,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "c1", Name: "find", Input: "{}", Finished: true},
				message.Finish{Reason: reason},
			},
		}
		if !ScrollbackSettled(msg, []message.Message{msg}) {
			t.Errorf("finish reason %q with an unanswered call holds the transcript; "+
				"an abnormal turn must print immediately with whatever state it reached", reason)
		}
	}
}
