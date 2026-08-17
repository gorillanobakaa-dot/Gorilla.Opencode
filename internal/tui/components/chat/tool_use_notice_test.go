package chat

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// THE BUG: a model that answers with a tool call and no prose is the single most
// common shape of a coding turn, and it was reported as "Finished without
// output" — the wording reserved for turns nobody can explain.
//
// Observed 2026-08-04: "yo lama" produced *Finished without output* followed
// immediately by a bash call and its result. Nothing had failed; the model had
// simply gone straight to running something. The label read like a crash and
// sent the user hunting for a bug that did not exist.
func TestATurnThatGoesStraightToAToolIsNotReportedAsEmpty(t *testing.T) {
	const at int64 = 1785228225

	assistantWithCall := message.Message{
		ID: "a1", Role: message.Assistant, CreatedAt: at,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "c1", Name: "bash", Input: "{}", Finished: true},
			message.Finish{Reason: message.FinishReasonToolUse},
		},
	}
	// The result must exist for the turn to print at all — scrollback now waits
	// for it (ScrollbackSettled), because printing earlier baked
	// "Waiting for response..." into the terminal's history.
	toolResult := message.Message{
		ID: "t1", Role: message.Tool, CreatedAt: at + 1,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Content: "ok"},
		},
	}

	m := printerFor(t, 100, assistantWithCall, toolResult)
	out := strings.Join(plainLines(m.printPending()), "\n")

	if strings.Contains(out, "Finished without output") {
		t.Errorf("a normal tool-using turn is still labelled with the catch-all "+
			"\"Finished without output\", which reads as a failure:\n%s", out)
	}
	if !strings.Contains(out, "went straight to running a tool") {
		t.Errorf("the turn does not say what actually happened:\n%s", out)
	}
}

// The catch-all must SURVIVE for finish reasons that genuinely have no better
// explanation, or removing it would hide real anomalies behind a reassuring
// message about tools.
func TestTheCatchAllStillCoversGenuinelyUnexplainedTurns(t *testing.T) {
	const at int64 = 1785228225

	unexplained := message.Message{
		ID: "a1", Role: message.Assistant, CreatedAt: at,
		Parts: []message.ContentPart{
			message.Finish{Reason: message.FinishReasonUnknown},
		},
	}

	m := printerFor(t, 100, unexplained)
	out := strings.Join(plainLines(m.printPending()), "\n")

	if !strings.Contains(out, "Finished without output") {
		t.Errorf("an unexplained finish reason no longer reports anything, so a "+
			"real anomaly would render as silence:\n%s", out)
	}
	if strings.Contains(out, "went straight to running a tool") {
		t.Errorf("an unexplained turn is being described as a tool call:\n%s", out)
	}
}
