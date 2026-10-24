package agent

// GORILLA FIX (2026-10-24): what happens when a tool round would overflow the
// model's window. Measured on Gemma 4 12B (20K window, LM Studio): the turn
// was refused on an estimate of 18K when the server's own count put the next
// request at ~14.1K, and the person was left with a tool result and no answer.

import (
	"fmt"

	"github.com/opencode-ai/opencode/internal/message"
)

// elidedStub replaces an old tool output that was left out to make room. It
// says what was there, so the model does not take the gap for an empty result.
const elidedStub = "[left out to fit the model's context window: %d characters of output from an earlier turn. Run the tool again if it is needed.]"

// elideKeep is the size under which an old tool output is kept as it is: a
// stub would save nothing.
const elideKeep = 400

// estimateRequest is the whole-history estimate for this agent and session.
func (a *agent) estimateRequest(sessionID string, msgs []message.Message) int {
	return EstimateRequestTokens(a.prov().SystemPrompt(), a.wireView(msgs), a.visibleTools(sessionID))
}

// overflowAfterTools is the overflow message for the request that would carry
// toolMsg, or "". The server's measurement of the call that just ended is used
// when there is one; it is checked against the whole-history estimate only
// when the server reported nothing.
func (a *agent) overflowAfterTools(sessionID string, msgs []message.Message, toolMsg message.Message) string {
	est := 0
	if s, ok := LastSpeed(sessionID); ok {
		if n, ok := MeasuredNextRequest(speedUsage(s), toolMsg); ok {
			est = n
		}
	}
	if est == 0 {
		est = a.estimateRequest(sessionID, msgs)
	}
	return ContextOverflowMessage(est, a.prov().Model())
}

// elideOldToolResults returns a copy of msgs in which the tool outputs of
// messages before turnStart are replaced by a stub, and how many were. The
// messages themselves (and the database) are not changed: only what is sent.
func elideOldToolResults(msgs []message.Message, turnStart int) ([]message.Message, int) {
	out := make([]message.Message, len(msgs))
	copy(out, msgs)
	dropped := 0
	for i := 0; i < turnStart && i < len(out); i++ {
		if out[i].Role != message.Tool {
			continue
		}
		parts := make([]message.ContentPart, len(out[i].Parts))
		copy(parts, out[i].Parts)
		changed := false
		for j, p := range parts {
			tr, ok := p.(message.ToolResult)
			if !ok || len(tr.Content) <= elideKeep {
				continue
			}
			tr.Content = fmt.Sprintf(elidedStub, len(tr.Content))
			tr.Metadata = ""
			parts[j] = tr
			changed = true
			dropped++
		}
		if changed {
			out[i].Parts = parts
		}
	}
	return out, dropped
}
