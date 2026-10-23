package permission

// GORILLA OVERRIDE (2026-10-11): the standing permission mode (/mode,
// Shift+Tab). Each mode is asserted against the request it must approve, the
// request it must still ask about, and the carve-outs it must never undo.

import (
	"context"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

func withMode(t *testing.T, mode string) *permissionService {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if err := config.SetPermissionMode(mode); err != nil {
		t.Fatalf("SetPermissionMode(%s): %v", mode, err)
	}
	t.Cleanup(func() { _ = config.SetPermissionMode(config.PermissionAsk) })
	s := NewPermissionService().(*permissionService)
	denyPrompts(t, s)
	return s
}

// countPrompts counts prompts actually published while the test runs.
func countPrompts(t *testing.T, s *permissionService) *int {
	t.Helper()
	n := new(int)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ch := s.Subscribe(ctx)
	ready := make(chan struct{})
	go func() {
		close(ready)
		for ev := range ch {
			if ev.Type == pubsub.CreatedEvent {
				*n++
			}
		}
	}()
	<-ready
	return n
}

func editReq() CreatePermissionRequest {
	return CreatePermissionRequest{SessionID: "conversation", ToolName: "edit", Action: "write",
		Path: config.WorkingDirectory(), GrantKey: config.WorkingDirectory() + "/main.go"}
}

func bashReq() CreatePermissionRequest {
	return CreatePermissionRequest{SessionID: "conversation", ToolName: "bash", Action: "execute",
		Path: config.WorkingDirectory(), GrantKey: "go build ./..."}
}

func TestAskModeChangesNothing(t *testing.T) {
	s := withMode(t, config.PermissionAsk)
	if s.Request(editReq()) || s.Request(bashReq()) {
		t.Fatal("ask mode approved something without a prompt")
	}
}

func TestAcceptEditsApprovesEditsInsideTheRootsOnly(t *testing.T) {
	s := withMode(t, config.PermissionAcceptEdits)
	if !s.Request(editReq()) {
		t.Error("an in-workspace edit still asked under accept-edits")
	}
	if s.Request(bashReq()) {
		t.Error("a shell command was approved under accept-edits")
	}
	outside := editReq()
	outside.Path = t.TempDir()
	outside.GrantKey = outside.Path + "/x"
	if s.Request(outside) {
		t.Error("an edit outside every workspace root was approved under accept-edits")
	}
	// Owner decision "B": an in-project edit still goes through after
	// untrusted content; a shell command does not.
	MarkTainted("conversation", "read a web page")
	t.Cleanup(func() { ClearTaint("conversation") })
	if !s.Request(editReq()) {
		t.Error("an in-project edit in a tainted turn asked under accept-edits (decision B)")
	}
}

func TestPlanModeRefusesWithoutAskingButStillAsksForTheWeb(t *testing.T) {
	s := withMode(t, config.PermissionPlan)
	prompts := countPrompts(t, s)
	if s.Request(editReq()) || s.Request(bashReq()) {
		t.Fatal("plan mode approved a change")
	}
	if *prompts != 0 {
		t.Fatalf("plan mode published %d prompts; it must refuse without asking", *prompts)
	}
	web := CreatePermissionRequest{SessionID: "conversation", ToolName: "web_fetch", Action: "fetch",
		Path: config.WorkingDirectory(), GrantKey: "https://example.test", Egress: true}
	s.Request(web)
	if *prompts != 1 {
		t.Fatalf("a web read in plan mode did not reach a person (prompts=%d)", *prompts)
	}
}

func TestAutoModeIsYoloForEverySessionWithTheSameCarveOuts(t *testing.T) {
	s := withMode(t, config.PermissionAuto)
	for _, sid := range []string{"conversation", "another-conversation"} {
		r := bashReq()
		r.SessionID = sid
		if !s.Request(r) {
			t.Errorf("session %s still asked under auto", sid)
		}
	}
	r := bashReq()
	r.Irreversible = "it deletes everything"
	if s.Request(r) {
		t.Error("auto approved an irreversible command")
	}
	// GORILLA OVERRIDE (2026-10-23): a web read on its own runs under auto;
	// after untrusted content, a place not used before asks.
	web := CreatePermissionRequest{SessionID: "conversation", ToolName: "web_fetch", Action: "fetch",
		Path: config.WorkingDirectory(), GrantKey: "https://example.test", Egress: true}
	if !s.Request(web) {
		t.Error("auto asked for an ordinary web read")
	}
	MarkTainted("conversation", "web page fetched from https://example.test")
	t.Cleanup(func() { ClearTaint("conversation") })
	web.GrantKey = "https://evil.example"
	if s.Request(web) {
		t.Error("auto approved a new destination after untrusted content")
	}
	ClearTaint("conversation")
	outside := editReq()
	outside.Path = t.TempDir()
	if s.Request(outside) {
		t.Error("auto approved a path outside every root")
	}
	if !s.IsAutoApproved("any-session") || EffectiveMode("any-session") != config.PermissionAuto {
		t.Error("auto mode is not reported as auto-approved for display")
	}
}

func TestModeCycleAndSpellings(t *testing.T) {
	if config.NextPermissionMode(config.PermissionAuto) != config.PermissionAsk ||
		config.NextPermissionMode(config.PermissionAsk) != config.PermissionAcceptEdits {
		t.Error("Shift+Tab cycle order is wrong")
	}
	for in, want := range map[string]string{"yolo": "auto", "Accept Edits": "accept-edits", "readonly": "plan", "default": "ask", "bogus": ""} {
		if got := config.NormalisePermissionMode(in); got != want {
			t.Errorf("NormalisePermissionMode(%q) = %q, want %q", in, got, want)
		}
	}
}
