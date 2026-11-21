package permission

// GORILLA FIX (2026-11-21): session persistence and approval-flood regressions.
//
// Pins the four bugs that caused repeated prompts after "Allow for session" or
// "Allow family" and 10+ consecutive popups on parallel write/screenshot runs:
//   1. A parallel burst of writes in subdirectories settles in ONE click.
//   2. A family grant on an outside-workspace file (e.g. shot1.png) scopes to
//      its folder so sibling files (shot2.png..shot10.png) do not re-prompt.
//   3. Approving an outside-root carve-out under YOLO extends extraRoots
//      synchronously before the UI drains the queue, avoiding the goroutine race.
//   4. An edit/write/patch session grant covers subdirectories inside the
//      approved folder, matching SessionScope's "every file change inside <dir>".

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/pubsub"
)

func TestAllowForSessionCoversParallelWriteFloodInOneClick(t *testing.T) {
	s := newLiveService(t, 5*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	events := s.Subscribe(ctx)

	base := t.TempDir()
	const n = 10
	results := make(chan bool, n)
	for i := 0; i < n; i++ {
		sub := filepath.Join(base, fmt.Sprintf("sub%d", i))
		req := CreatePermissionRequest{
			SessionID: "conversation",
			ToolName:  "write",
			Action:    "write",
			Path:      base,
			GrantKey:  filepath.Join(sub, "file.txt"),
		}
		go func(r CreatePermissionRequest) { results <- s.Request(r) }(req)
	}

	// Wait until all 10 parallel requests are parked, then answer ONLY the first
	// with "Allow for session". All 10 must be released immediately.
	deadline := time.Now().Add(2 * time.Second)
	for pendingCount(s) < n {
		if time.Now().After(deadline) {
			t.Fatalf("only %d/%d requests reached waiting state", pendingCount(s), n)
		}
		time.Sleep(5 * time.Millisecond)
	}

	var first PermissionRequest
	select {
	case ev := <-events:
		if ev.Type == pubsub.CreatedEvent {
			first = ev.Payload
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no CreatedEvent published")
	}
	s.GrantPersistant(first)

	for i := 0; i < n; i++ {
		select {
		case ok := <-results:
			if !ok {
				t.Fatalf("parallel write %d was denied after Allow for session", i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("parallel write %d stayed parked after Allow for session (flood not drained)", i)
		}
	}
}

func TestFamilyGrantOnOutsideFileCoversSiblingFilesInSameFolder(t *testing.T) {
	s := familyService(t)
	shots := filepath.Join(t.TempDir(), "screenshots")
	other := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(shots, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	shot1 := filepath.Join(shots, "shot1.png")
	shot2 := filepath.Join(shots, "shot2.png")
	otherFile := filepath.Join(other, "secret.png")

	s.GrantPersistant(PermissionRequest{
		ID:           "p1",
		SessionID:    "conversation",
		ToolName:     familyTool,
		Action:       "execute",
		Path:         shot1,
		GrantKey:     "python -I -m fieldkit ocr " + shot1,
		SessionClass: "python -I -m fieldkit",
	})

	req2 := familyReq("python -I -m fieldkit ocr " + shot2)
	req2.Path = shot2
	if !s.Request(req2) {
		t.Fatal("sibling screenshot in the same folder still prompted after Allow for session")
	}

	reqOther := familyReq("python -I -m fieldkit ocr " + otherFile)
	reqOther.Path = otherFile
	if s.Request(reqOther) {
		t.Fatal("grant on screenshots folder leaked to an unrelated folder")
	}

	if got := SessionScope(PermissionRequest{
		ToolName: familyTool, Path: shot1, SessionClass: "python -I -m fieldkit",
	}); got != "every `python -I -m fieldkit ...` command run in "+shots+" (one command each, no chaining)" {
		t.Errorf("SessionScope did not disclose the folder scope: %q", got)
	}
}

func TestGrantOnOutsideRootExtendsExtraRootSynchronouslyForQueueDrain(t *testing.T) {
	s := newLiveService(t, 5*time.Second)
	s.AutoApproveSession("conversation")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	events := s.Subscribe(ctx)

	shots := filepath.Join(t.TempDir(), "shots")
	if err := os.MkdirAll(shots, 0o755); err != nil {
		t.Fatal(err)
	}
	shot1 := filepath.Join(shots, "shot1.png")
	shot2 := filepath.Join(shots, "shot2.png")

	got1 := make(chan bool, 1)
	go func() {
		got1 <- s.Request(CreatePermissionRequest{
			SessionID: "conversation", ToolName: "bash", Action: "execute",
			Path: shot1, GrantKey: "ocr " + shot1,
		})
	}()
	waitPending(t, s)

	var first PermissionRequest
	select {
	case ev := <-events:
		first = ev.Payload
	case <-time.After(2 * time.Second):
		t.Fatal("expected CreatedEvent")
	}

	// Grant #1, then IMMEDIATELY check IsCovered for queued #2 on the same
	// goroutine (exactly as tui.go's drainPermissionQueue does).
	s.Grant(first)
	queued2 := PermissionRequest{
		SessionID: "conversation", ToolName: "bash", Action: "execute",
		Path: shot2, GrantKey: "ocr " + shot2,
	}
	if !s.IsCovered(queued2) {
		t.Fatal("queued sibling in the same outside folder was not covered synchronously after Grant")
	}
	if ok := <-got1; !ok {
		t.Fatal("first request was denied")
	}
}

func TestEditSessionGrantCoversSubdirectoriesInsideApprovedFolder(t *testing.T) {
	s := familyService(t)
	outside := filepath.Join(t.TempDir(), "export")
	sub := filepath.Join(outside, "nested", "dir")
	other := filepath.Join(t.TempDir(), "unrelated")

	s.GrantPersistant(PermissionRequest{
		SessionID: "conversation", ToolName: "write", Action: "write",
		Path: outside, GrantKey: filepath.Join(outside, "a.txt"),
	})

	if !s.Request(CreatePermissionRequest{
		SessionID: "conversation", ToolName: "write", Action: "write",
		Path: sub, GrantKey: filepath.Join(sub, "b.txt"),
	}) {
		t.Fatal("write in subdirectory of approved folder still prompted")
	}
	if s.Request(CreatePermissionRequest{
		SessionID: "conversation", ToolName: "write", Action: "write",
		Path: other, GrantKey: filepath.Join(other, "c.txt"),
	}) {
		t.Fatal("write outside approved folder was covered")
	}
}
