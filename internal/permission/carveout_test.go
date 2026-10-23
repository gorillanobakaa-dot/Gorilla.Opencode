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

// Egress is the sink. Whatever a hostile page talked the model into, it has to
// leave the machine to matter.
//
// GORILLA OVERRIDE (2026-10-23): the sink that matters is a NEW place after
// untrusted content. Until this date every egress asked under auto-approve,
// so reading the README the person named raised a dialog (MacBook Air,
// evidence row 2). The same fear is still tested: after a page was read, a
// host this conversation has not used asks; the host already used does not
// (evidence row 3, INSTALL.md on the same site).
func TestAutoApproveDoesNotCoverEgress(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	fetch := func(host string) bool {
		return s.Request(CreatePermissionRequest{
			SessionID: "conversation",
			ToolName:  "web_fetch",
			Action:    "fetch",
			Path:      config.WorkingDirectory(),
			GrantKey:  host,
			Egress:    true,
		})
	}
	if !fetch("https://github.com") {
		t.Fatal("the first fetch of a turn asked under auto-approve (evidence row 2)")
	}
	MarkTainted("conversation", "web page fetched from https://github.com")
	t.Cleanup(func() { ClearTaint("conversation") })
	if !fetch("https://github.com") {
		t.Fatal("a second page on the same site asked after the first was read (evidence row 3)")
	}
	if fetch("https://evil.example") {
		t.Fatal("a fetch to a new place left the machine under YOLO after untrusted content, with no question asked")
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

// The taint bit is what converts "the model was tricked" into "the model was
// tricked and could not act on it".
func TestTaintMakesTheNextActionAsk(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	MarkTainted("conversation", "web page fetched from https://evil.example")
	t.Cleanup(func() { ClearTaint("conversation") })

	// GORILLA OVERRIDE (2026-10-11, owner decision "B"): a shell command asks;
	// an edit inside the roots no longer does (it is reviewed in the diff).
	//
	// GORILLA OVERRIDE (2026-10-23): a shell command asks when it can reach the
	// network or run a script (tools.MayReachNetwork); a local one does not.
	if s.Request(CreatePermissionRequest{
		SessionID:       "conversation",
		ToolName:        "bash",
		Action:          "execute",
		Path:            config.WorkingDirectory(),
		GrantKey:        "python3 send.py",
		MayReachNetwork: true,
	}) {
		t.Fatal("a shell command that can reach the network went ahead unasked in a turn that had already read untrusted content")
	}
	if !s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "bash",
		Action:    "execute",
		Path:      config.WorkingDirectory(),
		GrantKey:  "mkdir -p build",
	}) {
		t.Fatal("a local shell command asked after untrusted content; only the network path should")
	}
	if !s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "edit",
		Action:    "write",
		Path:      config.WorkingDirectory(),
		GrantKey:  config.WorkingDirectory() + "/main.go",
	}) {
		t.Fatal("an in-project edit asked after untrusted content; decision B lets it through")
	}
	outside := t.TempDir()
	if s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "edit",
		Action:    "write",
		Path:      outside,
		GrantKey:  outside + "/x",
	}) {
		t.Fatal("an edit OUTSIDE the roots went ahead unasked in a tainted turn")
	}
}

// A helper session cannot launder untrusted content past the conversation that
// spawned it: taint is keyed on the session tree root, like grants are.
func TestTaintInAHelperReachesTheConversation(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	s.RegisterChildSession("helper-a1", "conversation")
	MarkTainted("helper-a1", "web search results")
	t.Cleanup(func() { ClearTaint("conversation") })

	if !IsTainted("conversation") {
		t.Fatal("a helper read untrusted content and the parent conversation did not know")
	}
}

// Taint that never cleared would be set permanently after the first web search
// and every egress would prompt forever — which is how a control gets switched
// off in practice while still looking present in the source.
func TestANewUserTurnClearsTaint(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	MarkTainted("conversation", "web page")
	ClearTaint("conversation")

	if !s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "edit",
		Action:    "write",
		Path:      config.WorkingDirectory(),
	}) {
		t.Fatal("taint outlived the turn that caused it")
	}
}

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
	MarkTainted("conversation", "web page")
	t.Cleanup(func() { ClearTaint("conversation") })
	if s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "web_fetch",
		Action:    "fetch",
		Path:      config.WorkingDirectory(),
		Egress:    true,
	}) {
		t.Fatal("an interactive session skipped the egress carve-out without SetUnattended")
	}
}
