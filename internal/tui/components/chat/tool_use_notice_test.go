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

// The joke is a FEATURE, not a stray string. It is the owner's wording and it is
// meant to be permanent, so a future tidy-up that removes it should have to
// delete a test that says why rather than silently dropping a line it read as
// noise.
//
// Both halves are asserted. The explanation carries the 2026-08-04 fix (this
// used to say "Finished without output" and looked like a crash); the joke
// carries the credit. Losing either one is a regression of a different kind.
func TestTheSilentToolTurnKeepsBothItsExplanationAndItsJoke(t *testing.T) {
	const at int64 = 1785228225

	assistantWithCall := message.Message{
		ID: "a1", Role: message.Assistant, CreatedAt: at,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "c1", Name: "bash", Input: "{}", Finished: true},
			message.Finish{Reason: message.FinishReasonToolUse},
		},
	}
	toolResult := message.Message{
		ID: "t1", Role: message.Tool, CreatedAt: at,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Name: "bash", Content: "ok"},
		},
	}

	m := printerFor(t, 200, assistantWithCall, toolResult)
	out := strings.Join(plainLines(m.printPending()), "\n")

	if !strings.Contains(out, "went straight to running a tool") {
		t.Error("the explanation is gone. It exists so this does not read as a failure.")
	}
	if !strings.Contains(out, "with science") {
		t.Error("the Pete Holmes line is gone. It is the owner's wording and deliberate; " +
			"see the credit in getToolAction and the v0.1.118 release page.")
	}
	// Directive 1 bans em-dashes. Scoped to the NOTICE LINE, not the whole
	// rendered turn: the first version of this assertion checked `out` and
	// tripped on decoration elsewhere in the printer, which is a real finding
	// but a different one, and an assertion that fires for something other than
	// its own subject is a test that will be deleted rather than read.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "went straight to running a tool") &&
			strings.Contains(line, "—") {
			t.Errorf("an em-dash is back in the silent-tool notice:\n%s", line)
		}
	}
}
