package dialog

// GORILLA OVERRIDE (2026-11-21): Shift+Y on the permission dialog turns YOLO on
// for the conversation. It is the one place a person can do that while the
// agent is parked on a question, because the editor is not reachable from here.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/opencode-ai/opencode/internal/permission"
)

func respondTo(t *testing.T, key tea.KeyMsg) (PermissionResponseMsg, bool) {
	t.Helper()
	p := NewPermissionDialogCmp().(*permissionDialogCmp)
	p.SetPermissions(permission.PermissionRequest{ID: "p1", SessionID: "conversation", ToolName: "edit", Action: "write", Path: "/work"})
	_, cmd := p.Update(key)
	if cmd == nil {
		return PermissionResponseMsg{}, false
	}
	m, ok := cmd().(PermissionResponseMsg)
	return m, ok
}

func TestShiftYAnswersWithYolo(t *testing.T) {
	m, ok := respondTo(t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	if !ok || m.Action != PermissionYolo {
		t.Fatalf("Shift+Y answered %+v (ok=%v); want PermissionYolo", m, ok)
	}
	if m.Permission.SessionID != "conversation" {
		t.Errorf("the response lost the session: %+v", m.Permission)
	}
}

// Handing over every later prompt must never be reachable by typing a lowercase
// "y" for yes, and must never be a button that Enter can land on.
func TestLowercaseYDoesNotTurnYoloOn(t *testing.T) {
	if m, ok := respondTo(t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}); ok && m.Action == PermissionYolo {
		t.Fatal("lowercase y turned YOLO on")
	}
	p := NewPermissionDialogCmp().(*permissionDialogCmp)
	p.SetPermissions(permission.PermissionRequest{ID: "p1", SessionID: "c", ToolName: "edit", Action: "write", Path: "/work"})
	if p.hasOption(PermissionYolo) {
		t.Fatal("YOLO is a button; it must be a key only, so Enter cannot select it")
	}
}
