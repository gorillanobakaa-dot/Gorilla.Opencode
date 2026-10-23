package permission

// GORILLA FIX (2026-10-23): MacBook Air, v0.1.150. A question nobody answered
// timed out after ten minutes and the turn ended as "permission denied", but
// the dialog stayed on screen; an Allow pressed later reached nobody. Every
// settled question is now announced (DeletedEvent), and a timeout is told apart
// from a person's refusal.

import (
	"context"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

func TestASettledQuestionIsAnnouncedAndATimeoutIsNotADeny(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	s := NewPermissionService().(*permissionService)
	restore := PermissionWaitForTest(50 * time.Millisecond)
	defer restore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := s.Subscribe(ctx)

	ask := func(key string) PermissionRequest {
		go s.Request(CreatePermissionRequest{SessionID: "conversation", ToolName: "bash",
			Action: "execute", Path: config.WorkingDirectory(), GrantKey: key})
		ev := <-events
		if ev.Type != pubsub.CreatedEvent {
			t.Fatalf("first event %v, want created", ev.Type)
		}
		return ev.Payload
	}
	settled := func(id string) {
		t.Helper()
		select {
		case ev := <-events:
			if ev.Type != pubsub.DeletedEvent || ev.Payload.ID != id {
				t.Fatalf("got %v for %s, want deleted for %s", ev.Type, ev.Payload.ID, id)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("a settled question was never announced; its dialog would stay on screen")
		}
	}

	// Nobody answers: announced, and recorded as no answer, not as a Deny.
	p := ask("python3 -c 'print(1)'")
	settled(p.ID)
	if TakePersonDenial("conversation") {
		t.Error("a timeout was recorded as the person's Deny")
	}
	if !TakeNoAnswer("conversation") {
		t.Error("a timeout was not recorded as no answer")
	}

	// The person answers: announced too, and not a timeout.
	p = ask("python3 -c 'print(2)'")
	s.Deny(p)
	settled(p.ID)
	if TakeNoAnswer("conversation") {
		t.Error("a Deny was recorded as no answer")
	}
	if !TakePersonDenial("conversation") {
		t.Error("a Deny was not recorded as the person's")
	}
}
