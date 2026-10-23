package tui

// GORILLA FIX (2026-10-23): a settled question takes its dialog down. On the
// MacBook Air (v0.1.150) a dialog whose question had timed out stayed on
// screen, and an Allow pressed on it reached nobody.

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
)

func TestASettledQuestionTakesItsDialogDown(t *testing.T) {
	a := appModel{permissions: dialog.NewPermissionDialogCmp()}
	shown := permission.PermissionRequest{ID: "p1", ToolName: "bash"}
	queued := permission.PermissionRequest{ID: "p2", ToolName: "bash"}

	m, _ := a.Update(pubsub.Event[permission.PermissionRequest]{Type: pubsub.CreatedEvent, Payload: shown})
	m, _ = m.(appModel).Update(pubsub.Event[permission.PermissionRequest]{Type: pubsub.CreatedEvent, Payload: queued})
	a = m.(appModel)
	if !a.showPermissions || a.permissions.Current().ID != "p1" || len(a.permissionQueue) != 1 {
		t.Fatalf("setup: show=%v current=%q queue=%d", a.showPermissions, a.permissions.Current().ID, len(a.permissionQueue))
	}

	// The question on screen times out: its dialog goes, the next one shows.
	m, _ = a.Update(pubsub.Event[permission.PermissionRequest]{Type: pubsub.DeletedEvent, Payload: shown})
	a = m.(appModel)
	if !a.showPermissions || a.permissions.Current().ID != "p2" || len(a.permissionQueue) != 0 {
		t.Fatalf("after p1 settled: show=%v current=%q queue=%d", a.showPermissions, a.permissions.Current().ID, len(a.permissionQueue))
	}
	// A settled question that is not on screen changes nothing.
	m, _ = a.Update(pubsub.Event[permission.PermissionRequest]{Type: pubsub.DeletedEvent, Payload: shown})
	a = m.(appModel)
	if !a.showPermissions || a.permissions.Current().ID != "p2" {
		t.Fatal("a stale settle took down a live dialog")
	}
	// The last one settles: no dialog left.
	m, _ = a.Update(pubsub.Event[permission.PermissionRequest]{Type: pubsub.DeletedEvent, Payload: queued})
	if m.(appModel).showPermissions {
		t.Fatal("the dialog of a settled question stayed on screen")
	}
}
