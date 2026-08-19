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
func TestAutoApproveDoesNotCoverEgress(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	if s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "web_fetch",
		Action:    "fetch",
		Path:      config.WorkingDirectory(),
		GrantKey:  "https://evil.example",
		Egress:    true,
	}) {
		t.Fatal("a fetch left the machine under YOLO with no question asked")
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

	if s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "edit",
		Action:    "write",
		Path:      config.WorkingDirectory(),
	}) {
		t.Fatal("an edit went ahead unasked in a turn that had already read untrusted content")
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
