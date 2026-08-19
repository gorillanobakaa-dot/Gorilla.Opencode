package permission

// GORILLA OVERRIDE (2026-08-17): a grant belongs to the CONVERSATION, not to
// whichever helper happened to ask for it.
//
// Reported from a live run: "Allow for session" was answered for a web search,
// and the same search asked again minutes later, and again after that. Every
// research helper runs in its own session (one per lane), and the grant was
// matched on that session id — so approving in helper a3 did nothing for a1,
// a2 or a4, and a ten-helper run could ask the same question ten times.

import (
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

func req(session, tool string) CreatePermissionRequest {
	return CreatePermissionRequest{
		SessionID: session,
		ToolName:  tool,
		Action:    "search",
		Path:      "/tmp/project",
	}
}

// grantIn answers one pending request for a session, the way the UI does when
// the user presses "allow for session".
func grantIn(s *permissionService, sessionID, tool string) {
	s.mu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, PermissionRequest{
		SessionID: s.rootSession(sessionID),
		ToolName:  tool,
		Action:    "search",
		Path:      "/tmp/project",
	})
	s.mu.Unlock()
}

func TestGrantCoversSiblingHelpers(t *testing.T) {
	s := NewPermissionService().(*permissionService)
	s.RegisterChildSession("helper-a1", "conversation")
	s.RegisterChildSession("helper-a2", "conversation")
	s.RegisterChildSession("helper-a3", "conversation")

	// The user approves once, while helper a3 is the one asking.
	grantIn(s, "helper-a3", "web_search")

	// Every sibling — and the conversation itself — is now covered. Without
	// the fix each of these blocks forever waiting on a prompt, which is what
	// the report described.
	for _, id := range []string{"helper-a1", "helper-a2", "helper-a3", "conversation"} {
		if !s.Request(req(id, "web_search")) {
			t.Errorf("%s was not covered by the grant — it would prompt again", id)
		}
	}
}

// The fix must not widen what was approved. A different tool still asks.
func TestGrantDoesNotLeakToOtherTools(t *testing.T) {
	s := NewPermissionService().(*permissionService)
	s.RegisterChildSession("helper-a1", "conversation")
	grantIn(s, "helper-a1", "web_search")

	done := make(chan bool, 1)
	go func() { done <- s.Request(req("helper-a1", "bash")) }()

	// Nothing may auto-approve this: it must still be waiting on the user.
	// The wait is the assertion — checking immediately would pass before the
	// goroutine had even started, which is a test that cannot fail.
	select {
	case got := <-done:
		t.Errorf("a grant for web_search silently approved bash (returned %v)", got)
	case <-time.After(150 * time.Millisecond):
	}
}

// A grant made in one conversation must not cover a different conversation.
func TestGrantDoesNotCrossConversations(t *testing.T) {
	s := NewPermissionService().(*permissionService)
	s.RegisterChildSession("helper-a1", "conversation-one")
	s.RegisterChildSession("helper-b1", "conversation-two")
	grantIn(s, "helper-a1", "web_search")

	done := make(chan bool, 1)
	go func() { done <- s.Request(req("helper-b1", "web_search")) }()
	select {
	case got := <-done:
		t.Errorf("a grant in one conversation approved another (returned %v)", got)
	case <-time.After(150 * time.Millisecond):
	}
}

// YOLO covers the whole tree, and can be switched back off.
func TestYoloCoversTheTreeAndRevokes(t *testing.T) {
	// AMENDED 2026-08-19: this used req(), whose Path is the fixed literal
	// "/tmp/project". That was fine while auto-approve was unconditional. It
	// is not fine now: an out-of-root path is one of the three carve-outs
	// (see carveout_test.go), so the request correctly raised a prompt and
	// this test blocked on it for the full permission wait.
	//
	// The path is changed rather than the carve-out, because what this test is
	// FOR is tree coverage and revocation, not path scope. Using a real
	// workspace root keeps it testing that.
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	inRoot := func(session, tool string) CreatePermissionRequest {
		r := req(session, tool)
		r.Path = config.WorkingDirectory()
		return r
	}

	s := NewPermissionService().(*permissionService)
	s.RegisterChildSession("helper-a1", "conversation")

	if s.IsAutoApproved("conversation") {
		t.Fatal("auto-approve must start off")
	}
	s.AutoApproveSession("conversation")

	if !s.IsAutoApproved("helper-a1") {
		t.Errorf("a helper does not see the conversation's YOLO stance")
	}
	if !s.Request(inRoot("helper-a1", "bash")) {
		t.Errorf("YOLO did not approve a helper's request")
	}

	s.RevokeAutoApprove("conversation")
	if s.IsAutoApproved("conversation") || s.IsAutoApproved("helper-a1") {
		t.Errorf("YOLO survived being revoked")
	}
}

// Registering a cycle must not hang the caller: a tool call is blocked on this.
func TestRootSessionSurvivesACycle(t *testing.T) {
	s := NewPermissionService().(*permissionService)
	s.RegisterChildSession("a", "b")
	s.RegisterChildSession("b", "a")
	done := make(chan string, 1)
	go func() { done <- s.rootSession("a") }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Errorf("rootSession hung on a cycle — a tool call waits on this")
	}
}

// A prompt nobody answers must eventually release the tool, not park it
// forever.
//
// GORILLA OVERRIDE (2026-08-17): the code carried the comment "Wait for the
// response with a timeout" above a bare channel receive with no timeout at all.
// Observed live: three helpers stopped mid-flow and were still reported as
// "running" sixteen minutes later, with no network connections and nothing
// written. This test drives the same path with a wait short enough to assert.
func TestUnansweredRequestIsReleasedNotParked(t *testing.T) {
	s := NewPermissionService().(*permissionService)

	restore := PermissionWaitForTest(50 * time.Millisecond)
	defer restore()

	done := make(chan bool, 1)
	go func() { done <- s.Request(req("conversation", "web_search")) }()

	select {
	case granted := <-done:
		if granted {
			t.Errorf("an unanswered request was APPROVED; it must be denied so the tool fails loudly")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the request never returned — this is the hang that stranded three helpers")
	}
}

// Killing a helper must release whatever it was blocked on immediately, rather
// than leaving it to wait out the timeout.
func TestCancelSessionReleasesOnlyThatConversation(t *testing.T) {
	s := NewPermissionService().(*permissionService)
	s.RegisterChildSession("helper-a1", "doomed")

	mine := make(chan bool, 1)
	other := make(chan bool, 1)
	go func() { mine <- s.Request(req("helper-a1", "web_search")) }()
	go func() { other <- s.Request(req("innocent", "web_search")) }()
	time.Sleep(100 * time.Millisecond) // let both park

	if n := s.CancelSession("doomed"); n != 1 {
		t.Errorf("CancelSession released %d requests, want exactly 1 (its own)", n)
	}
	select {
	case granted := <-mine:
		if granted {
			t.Errorf("a cancelled request came back approved")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("killing the helper did not release the tool waiting inside it")
	}
	select {
	case <-other:
		t.Errorf("cancelling one conversation released a DIFFERENT conversation's prompt")
	case <-time.After(200 * time.Millisecond):
	}
}
