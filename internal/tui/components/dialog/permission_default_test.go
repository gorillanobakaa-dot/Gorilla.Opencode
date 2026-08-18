package dialog

// GORILLA FIX (2026-08-18): the permission dialog is the only security boundary
// in this program, so its default answer and its reset behaviour are security
// properties, not cosmetics.

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/permission"
)

// Landing on "Allow" makes the dangerous answer the reflex one, and a prompt
// that appears mid-typing can be accepted by an Enter meant for something else.
func TestTheDialogDefaultsToDeny(t *testing.T) {
	p := NewPermissionDialogCmp().(*permissionDialogCmp)
	if p.selectedOption != 2 {
		t.Errorf("dialog opens on option %d; it must default to Deny (2) so an "+
			"accidental Enter is harmless", p.selectedOption)
	}
}

// The dialog is reused between requests. If the highlight persists, a single
// Enter answers a question the user has not read.
func TestTheSelectionResetsForEachNewRequest(t *testing.T) {
	p := NewPermissionDialogCmp().(*permissionDialogCmp)

	p.selectedOption = 0 // user allowed the previous one
	p.SetPermissions(permission.PermissionRequest{
		ToolName: "bash", Action: "execute", Path: "/work", GrantKey: "rm -rf /",
	})

	if p.selectedOption != 2 {
		t.Errorf("a new request inherited the previous answer (option %d); consent "+
			"must be given per request", p.selectedOption)
	}
}
