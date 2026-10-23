package agent

// GORILLA FIX (2026-10-23): a person pressing Deny is an answer the model
// reads, not the end of the turn. On the MacBook Air one Deny on a harmless
// probe stopped the stranger install dead: the turn finished as "permission
// denied", the model never learned what the person wanted, and the screen
// waited until the person typed again.

import (
	"context"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

func TestAPersonsDenyIsAnAnswerAndTheTurnCarriesOn(t *testing.T) {
	a, store := newLoopAgent(t, config.AgentCoder, &scriptedProvider{})
	svc := permission.NewPermissionService()
	// The person: denies every prompt that appears, as the dialog's Deny does.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := svc.Subscribe(ctx)
	go func() {
		for ev := range ch {
			if ev.Type == pubsub.CreatedEvent {
				svc.Deny(ev.Payload)
			}
		}
	}()
	asking := &scriptedTool{name: "probe_tool", do: func() (tools.ToolResponse, error) {
		if !svc.Request(permission.CreatePermissionRequest{SessionID: "deny", ToolName: "probe_tool",
			Action: "execute", Path: config.WorkingDirectory(), GrantKey: "probe"}) {
			return tools.ToolResponse{}, permission.ErrorPermissionDenied
		}
		return tools.NewTextResponse("ran"), nil
	}}
	second := &scriptedTool{name: "send_things"}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{call("c1", "probe_tool"), call("c2", "send_things")}},
		{text: "understood, I wrote the question in QUESTIONS.txt and carried on"},
	}}
	a.provider = p
	a.tools = []tools.BaseTool{asking, second}

	ev := runTurn(t, a, "deny", "install fieldkit")
	if ev.Error != nil {
		t.Fatalf("turn failed: %v", ev.Error)
	}
	if p.asked != 2 {
		t.Fatalf("the model was called %d times; after a person's Deny it must be called again with the answer", p.asked)
	}
	if second.count() != 0 {
		t.Fatal("a call sent in the same message as the denied one ran anyway")
	}
	got := ""
	for _, m := range p.history[1] {
		for _, r := range m.ToolResults() {
			if r.ToolCallID == "c1" {
				got = r.Content
			}
		}
	}
	if !strings.HasPrefix(got, "Denied by the person") || !strings.Contains(got, "Do not retry") {
		t.Errorf("the model was told %q, want the person's refusal and not to retry", got)
	}
	if ev.Message.FinishReason() == message.FinishReasonPermissionDenied {
		t.Error("the turn still finished as permission denied")
	}
	all, _ := store.List(context.Background(), "deny")
	if last := all[len(all)-1]; last.Role != message.Assistant || !strings.Contains(last.Content().String(), "carried on") {
		t.Errorf("the turn did not end on the model's answer: %+v", last.Content().String())
	}
}
