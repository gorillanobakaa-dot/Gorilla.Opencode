package chat

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// A turn that produced reasoning and then died must not print NOTHING.
func TestFinishedMessageWithNoAnswerStillPrintsSomething(t *testing.T) {
	for _, reason := range []message.FinishReason{
		message.FinishReasonError,
		message.FinishReasonCanceled,
		message.FinishReasonMaxTokens,
	} {
		msg := message.Message{
			ID: "a", SessionID: "s", Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ReasoningContent{Thinking: "I was thinking about it and then"},
				message.Finish{Reason: reason},
			},
		}
		out := RenderForScrollback(msg, 0, []message.Message{msg}, nil, 100)
		if strings.TrimSpace(out) == "" {
			t.Errorf("finish=%q rendered NOTHING: the user sees thinking, then silence, "+
				"with no indication the turn failed", reason)
			continue
		}
		t.Logf("finish=%q -> %q", reason, strings.TrimSpace(out))
	}
}
