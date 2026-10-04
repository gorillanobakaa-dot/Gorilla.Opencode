package permission

// GORILLA OVERRIDE (2026-10-04): auto-approve does not cover what cannot be
// undone, and an unattended run refuses it outright.

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

func TestAutoApproveDoesNotCoverAnIrreversibleAction(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	if s.Request(CreatePermissionRequest{
		SessionID:    "conversation",
		ToolName:     "bash",
		Action:       "execute",
		Path:         config.WorkingDirectory(),
		GrantKey:     "rm -rf ~",
		Irreversible: "it deletes your whole home folder",
	}) {
		t.Fatal("YOLO approved an irreversible command without asking")
	}
}

// The same request without the marker is ordinary work and stays covered, so
// the carve-out is keyed on the marker and on nothing else.
func TestTheSameRequestWithoutTheMarkerIsStillCovered(t *testing.T) {
	s := autoApproved(t)
	denyPrompts(t, s)
	if !s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "bash",
		Action:    "execute",
		Path:      config.WorkingDirectory(),
		GrantKey:  "go build ./...",
	}) {
		t.Fatal("YOLO stopped covering an ordinary command")
	}
}

// Nobody is watching: the other carve-outs log and proceed, this one refuses.
func TestAnUnattendedRunRefusesAnIrreversibleAction(t *testing.T) {
	s := autoApproved(t)
	s.unattended = true
	if s.Request(CreatePermissionRequest{
		SessionID:    "conversation",
		ToolName:     "bash",
		Action:       "execute",
		Path:         config.WorkingDirectory(),
		GrantKey:     "git reset --hard",
		Irreversible: "it throws away changes that were never committed",
	}) {
		t.Fatal("an unattended run carried out an irreversible command")
	}
	if !s.Request(CreatePermissionRequest{
		SessionID: "conversation",
		ToolName:  "bash",
		Action:    "execute",
		Path:      config.WorkingDirectory(),
		GrantKey:  "go test ./...",
	}) {
		t.Fatal("an unattended run stopped doing ordinary work")
	}
}
