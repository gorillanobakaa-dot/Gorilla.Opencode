package permission

// GORILLA OVERRIDE (2026-11-21): /yolo typed while a turn is running.
//
// Until now YOLO only affected requests raised AFTER it was switched on, and
// the editor refused /yolo while the agent worked. A tool already parked on a
// permission question therefore stayed parked. These tests pin the new rules:
// turning YOLO on answers what is waiting, repeating it changes nothing, and
// the carve-outs still ask.

import (
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

func newLiveService(t *testing.T, wait time.Duration) *permissionService {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	t.Cleanup(PermissionWaitForTest(wait))
	return NewPermissionService().(*permissionService)
}

func pendingCount(s *permissionService) int {
	n := 0
	s.pendingRequests.Range(func(_, _ any) bool { n++; return true })
	return n
}

// waitPending blocks until a question is registered, so the test turns YOLO on
// while the tool is genuinely parked, not before it asked.
func waitPending(t *testing.T, s *permissionService) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for pendingCount(s) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the request never reached the waiting state")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func ordinaryEdit() CreatePermissionRequest {
	return CreatePermissionRequest{
		SessionID: "conversation", ToolName: "edit", Action: "write",
		Path: config.WorkingDirectory(),
	}
}

func TestYoloTurnedOnMidRunAnswersTheWaitingQuestion(t *testing.T) {
	s := newLiveService(t, 5*time.Second)
	got := make(chan bool, 1)
	go func() { got <- s.Request(ordinaryEdit()) }()
	waitPending(t, s)

	s.AutoApproveSession("conversation")

	select {
	case ok := <-got:
		if !ok {
			t.Fatal("the waiting request was refused; turning YOLO on should have approved it")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the waiting request was not released by /yolo — the tool is still parked on a question YOLO covers")
	}
}

func TestYoloMidRunStillAsksAboutWhatCannotBeUndone(t *testing.T) {
	s := newLiveService(t, 300*time.Millisecond)
	req := ordinaryEdit()
	req.ToolName = "bash"
	req.Action = "execute"
	req.Irreversible = "it deletes the whole disk"
	got := make(chan bool, 1)
	go func() { got <- s.Request(req) }()
	waitPending(t, s)

	s.AutoApproveSession("conversation")

	// Nobody answers, so the question times out and is refused. The point is
	// that it was NOT approved by YOLO.
	if ok := <-got; ok {
		t.Fatal("YOLO turned on mid-run approved an irreversible command without asking")
	}
}

func TestYoloMidRunStillAsksOutsideEveryRoot(t *testing.T) {
	s := newLiveService(t, 300*time.Millisecond)
	req := ordinaryEdit()
	req.Path = "/definitely/not/a/workspace/root/file.txt"
	got := make(chan bool, 1)
	go func() { got <- s.Request(req) }()
	waitPending(t, s)

	s.AutoApproveSession("conversation")

	if ok := <-got; ok {
		t.Fatal("YOLO turned on mid-run approved a write outside every workspace root without asking")
	}
}

func TestYoloIsIdempotentAndOffStaysOff(t *testing.T) {
	s := newLiveService(t, 50*time.Millisecond)
	s.AutoApproveSession("conversation")
	s.AutoApproveSession("conversation")
	if n := len(s.autoApproveSessions); n != 1 {
		t.Fatalf("repeating /yolo stored the conversation %d times; it must change nothing", n)
	}
	if !s.IsAutoApproved("conversation") {
		t.Fatal("YOLO should be on")
	}
	s.RevokeAutoApprove("conversation")
	s.RevokeAutoApprove("conversation")
	if s.IsAutoApproved("conversation") {
		t.Fatal("YOLO should be off, and repeating /yolo off must keep it off")
	}
}

// A question already queued behind the one on screen is dropped, not shown,
// once YOLO covers it; one it does not cover is still shown.
func TestIsCoveredFollowsYolo(t *testing.T) {
	s := newLiveService(t, 50*time.Millisecond)
	ordinary := PermissionRequest{SessionID: "conversation", ToolName: "edit", Action: "write", Path: config.WorkingDirectory()}
	danger := ordinary
	danger.ToolName, danger.Action, danger.Irreversible = "bash", "execute", "it deletes the whole disk"

	if s.IsCovered(ordinary) {
		t.Fatal("nothing covers this before YOLO is on")
	}
	s.AutoApproveSession("conversation")
	if !s.IsCovered(ordinary) {
		t.Fatal("a queued ordinary question should be covered once YOLO is on")
	}
	if s.IsCovered(danger) {
		t.Fatal("YOLO must not cover an irreversible command")
	}
	s.RevokeAutoApprove("conversation")
	if s.IsCovered(ordinary) {
		t.Fatal("after /yolo off the same question is asked again")
	}
}
