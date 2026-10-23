package permission

// GORILLA OVERRIDE (2026-08-19): auto-approve used to be total.
//
// `Request()` opened with an unconditional `return true` for an auto-approved
// session — before grant matching, before anything. So every "it's fine, the
// user gets asked" claim elsewhere in the codebase was false the moment YOLO
// was switched on, and the program had no answer at all for the case that
// matters most: the model has just read a hostile web page, and the next thing
// it wants to do is send a request somewhere.

import (
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

func autoApproved(t *testing.T) *permissionService {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	s := NewPermissionService().(*permissionService)
	s.AutoApproveSession("conversation")
	return s
}

// answerNext denies the next prompt, so a test can tell "was approved without
// asking" (true, immediately) from "a prompt was raised" (false, denied).
func denyPrompts(t *testing.T, s *permissionService) {
	t.Helper()
	restore := PermissionWaitForTest(50 * 1000000) // 50ms
	t.Cleanup(restore)
}

func TestAutoApproveStillCoversOrdinaryWork(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	ok := s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "edit",
		Action:    "write",
		Path:      config.WorkingDirectory(),
	})
	if !ok {
		t.Fatal("YOLO stopped covering an ordinary in-workspace edit — the carve-outs are too wide to be usable")
	}
}

// Egress under auto-approve.
//
// GORILLA OVERRIDE (2026-10-23, owner's order): auto-approve covers egress,
// whatever the turn has read. Earlier the same day a new host asked after a
// web page had been read; before that every egress asked. The owner: "from
// the source code GET RID OF THAT BULLSHIT ... every bloody 3 seconds there's
// a dialog." Egress still asks in ask mode and reaches a person in plan mode.
func TestAutoApproveCoversEgress(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	for _, host := range []string{"https://github.com", "https://raw.githubusercontent.com", "https://evil.example"} {
		if !s.Request(CreatePermissionRequest{
			SessionID: "conversation",
			ToolName:  "web_fetch",
			Action:    "fetch",
			Path:      config.WorkingDirectory(),
			GrantKey:  host,
			Egress:    true,
		}) {
			t.Errorf("a fetch of %s asked under auto-approve", host)
		}
	}
}

// But it must fall THROUGH to grant matching, not deny outright — otherwise
// the user is asked once per call rather than once per host, and a prompt that
// fires constantly is a prompt that gets answered without being read.
func TestAnExistingGrantStillCoversEgressUnderAutoApprove(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	s.mu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, PermissionRequest{
		SessionID: "conversation",
		ToolName:  "web_fetch",
		Action:    "fetch",
		Path:      config.WorkingDirectory(),
		GrantKey:  "https://docs.python.org",
	})
	s.mu.Unlock()

	if !s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "web_fetch",
		Action:    "fetch",
		Path:      config.WorkingDirectory(),
		GrantKey:  "https://docs.python.org",
		Egress:    true,
	}) {
		t.Fatal("an already-approved host was asked about again")
	}
}

func TestAutoApproveDoesNotCoverPathsOutsideEveryRoot(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	if s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "write",
		Action:    "write",
		Path:      "/etc",
	}) {
		t.Fatal("YOLO authorised a write outside every workspace root without asking")
	}
}

// GORILLA OVERRIDE (2026-10-23, owner's order): this was
// TestTaintMakesTheNextActionAsk. With the taint mechanism gone, auto-approve
// covers a shell command and an edit inside the roots, network or not; a path
// outside the roots still asks.
func TestAutoApproveCoversWorkInsideTheRoots(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	for _, cmd := range []string{`python3 -c "import urllib.request; print('urllib ok')"`, "scp notes.txt me@host:", "mkdir -p build"} {
		if !s.Request(CreatePermissionRequest{
			SessionID: "conversation",
			ToolName:  "bash",
			Action:    "execute",
			Path:      config.WorkingDirectory(),
			GrantKey:  cmd,
		}) {
			t.Errorf("%q asked under auto-approve", cmd)
		}
	}
	if !s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "edit",
		Action:    "write",
		Path:      config.WorkingDirectory(),
		GrantKey:  config.WorkingDirectory() + "/main.go",
	}) {
		t.Fatal("an in-project edit asked under auto-approve")
	}
	outside := t.TempDir()
	if s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "edit",
		Action:    "write",
		Path:      outside,
		GrantKey:  outside + "/x",
	}) {
		t.Fatal("an edit OUTSIDE the roots went ahead unasked")
	}
}

// TestTaintInAHelperReachesTheConversation and TestANewUserTurnClearsTaint
// stood here until 2026-10-23; the taint mechanism they tested was removed on
// the owner's order.

// GORILLA OVERRIDE (2026-08-19): the carve-outs assume a prompt can be
// ANSWERED. In a -p run or a cron job there is no TUI subscribed to the
// permission broker, so a carve-out would publish a question into an empty
// room, wait the full ten-minute PermissionWait, and then deny — turning a
// working headless script into one that hangs for ten minutes and then fails.
//
// This is the test that would have caught it. Every other test in this package
// answers its own prompts, which is exactly why the gap was invisible.
func TestUnattendedRunsAreNotBlockedByACarveOut(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	s.SetUnattended(true)

	done := make(chan bool, 1)
	go func() {
		done <- s.Request(CreatePermissionRequest{
			SessionID: "conversation",
			ToolName:  "web_fetch",
			Action:    "fetch",
			Path:      config.WorkingDirectory(),
			GrantKey:  "https://docs.python.org",
			Egress:    true,
		})
	}()

	select {
	case ok := <-done:
		if !ok {
			t.Fatal("a headless fetch was denied by a carve-out nobody could answer")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a headless fetch blocked on a permission prompt with no subscriber to answer it")
	}
}

// Unattended must not become a way to disable the carve-outs for a real user.
func TestUnattendedIsOffForAnInteractiveSession(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	if s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "bash",
		Action:    "execute",
		Path:      config.WorkingDirectory(),
		GrantKey:  "sudo make install",
		HardStop:  "it runs as the administrator (sudo)",
	}) {
		t.Fatal("an interactive session skipped the sudo hard stop without SetUnattended")
	}
}
