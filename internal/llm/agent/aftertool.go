package agent

// GORILLA OVERRIDE (2026-10-04): the one place every tool result passes through
// before it is stored and sent.
//
// Two things happen here that must not depend on a tool remembering to do
// them: credentials are masked (tools/secretmask.go), and the call is recorded
// for loop detection (stuck.go). Putting them at the single choke point is the
// same reasoning as tools.MaxToolResponseBytes: a rule kept in twelve places is
// forgotten in the thirteenth.

import (
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

// stuck is shared by every agent in the process. Session IDs are unique, and a
// helper agent runs in its own session, so one tracker serves them all.
var stuck = newStuckTracker()

// finishToolResults masks and records every result whose tool actually ran.
// ran[i] is false for a call that was cancelled or refused, which is not the
// model repeating itself and has no output to mask.
//
// It returns the sentence for the person if the turn must stop, or "".
func finishToolResults(sessionID string, calls []message.ToolCall, results []message.ToolResult, ran []bool) string {
	stop := ""
	for i := range results {
		if i >= len(calls) || i >= len(ran) || !ran[i] {
			continue
		}
		masked, n := tools.MaskSecrets(calls[i].Name, results[i].Content)

		// Recorded BEFORE any note is appended: the note changes with the
		// count, and a result that differs every time is never a repeat.
		verdict := stuck.Record(sessionID, calls[i].Name, calls[i].Input, masked, results[i].IsError)

		results[i].Content = masked + tools.MaskNotice(n) + verdict.Note
		if verdict.Stop != "" && stop == "" {
			stop = verdict.Stop
		}
	}
	return stop
}
