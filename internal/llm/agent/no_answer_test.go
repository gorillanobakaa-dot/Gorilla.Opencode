package agent

// GORILLA FIX (2026-10-23): MacBook Air, v0.1.150. A permission question nobody
// answered timed out; the screen then said "Permission denied, so nothing was
// run" (nobody had denied anything) and the unanswered call showed "Waiting for
// response..." for good, because the message was marked finished before the
// tool results existed and the inline printer printed it at that moment.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
)

// orderedMessages records, when an assistant message is first marked
// permission-denied, whether a tool message already exists.
type orderedMessages struct{ *memMessages }

var (
	resultsSeen bool
	markSeen    bool
)

func (o orderedMessages) Update(ctx context.Context, msg message.Message) error {
	if msg.FinishReason() == message.FinishReasonPermissionDenied && !markSeen {
		all, _ := o.memMessages.List(ctx, msg.SessionID)
		for _, m := range all {
			resultsSeen = resultsSeen || m.Role == message.Tool
		}
		markSeen = true
	}
	return o.memMessages.Update(ctx, msg)
}

func TestAnUnansweredQuestionSaysSoAndStoresResultsFirst(t *testing.T) {
	restore := permission.PermissionWaitForTest(50 * time.Millisecond)
	defer restore()
	a, store := newLoopAgent(t, config.AgentCoder, &scriptedProvider{})
	a.messages = orderedMessages{memMessages: store}
	svc := permission.NewPermissionService() // nobody subscribes: nobody answers
	asking := &scriptedTool{name: "probe_tool", do: func() (tools.ToolResponse, error) {
		if !svc.Request(permission.CreatePermissionRequest{SessionID: "noanswer", ToolName: "probe_tool",
			Action: "execute", Path: config.WorkingDirectory(), GrantKey: "probe"}) {
			return tools.ToolResponse{}, permission.ErrorPermissionDenied
		}
		return tools.NewTextResponse("ran"), nil
	}}
	a.tools = []tools.BaseTool{asking}
	a.provider = &scriptedProvider{script: []reply{{calls: []message.ToolCall{call("c1", "probe_tool")}}}}
	markSeen, resultsSeen = false, false

	runTurn(t, a, "noanswer", "install fieldkit")

	all, _ := store.List(context.Background(), "noanswer")
	got := ""
	for _, m := range all {
		for _, r := range m.ToolResults() {
			if r.ToolCallID == "c1" {
				got = r.Content
			}
		}
	}
	if !strings.Contains(got, "nobody answered") || strings.HasPrefix(strings.ToLower(got), "permission denied") {
		t.Errorf("the result of an unanswered question reads %q; it must say nobody answered", got)
	}
	if !markSeen {
		t.Fatal("the turn was not marked as stopped at a permission question")
	}
	if !resultsSeen {
		t.Error("the message was marked permission-denied before its tool results were stored; the inline printer shows the call as Waiting for response... for good")
	}
}
