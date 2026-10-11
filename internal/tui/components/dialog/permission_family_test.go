package dialog

// GORILLA FIX (2026-10-11): the "Allow family" button, and the rule that it
// appears only when there is a family to show.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/permission"
)

const famCmd = `python -c "import sys; from fieldkit.cli import main; main(['cards', 'list'])"`
const famSuggested = `python -c "import sys; from fieldkit.cli import main;`

func familyRequest() permission.PermissionRequest {
	return permission.PermissionRequest{
		ID: "r1", ToolName: "bash", Action: "execute", Path: "/work", GrantKey: famCmd,
		FamilySuggestion: famSuggested,
		Params:           tools.BashPermissionsParams{Command: famCmd},
	}
}

func keyPress(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestTheFamilyButtonAppearsOnlyWithASuggestionAndDenyStaysDefault(t *testing.T) {
	p := NewPermissionDialogCmp().(*permissionDialogCmp)
	p.SetPermissions(familyRequest())
	if len(p.options) != 4 || p.options[2] != PermissionAllowFamily || p.options[3] != PermissionDeny {
		t.Fatalf("options = %v, want allow, session, family, deny", p.options)
	}
	if p.currentAction() != PermissionDeny {
		t.Fatalf("default is %v, must be Deny", p.currentAction())
	}
	if p.family() != famSuggested {
		t.Fatalf("family shown = %q, want the suggestion", p.family())
	}

	plain := familyRequest()
	plain.FamilySuggestion = ""
	p.SetPermissions(plain)
	if len(p.options) != 3 || p.currentAction() != PermissionDeny {
		t.Fatalf("without a suggestion: options = %v default %v", p.options, p.currentAction())
	}
}

func TestPressingFSendsTheFamilyThePersonRead(t *testing.T) {
	p := NewPermissionDialogCmp().(*permissionDialogCmp)
	p.SetPermissions(familyRequest())
	_, cmd := p.Update(keyPress("f"))
	msg, ok := cmd().(PermissionResponseMsg)
	if !ok || msg.Action != PermissionAllowFamily || msg.Family != famSuggested {
		t.Fatalf("got %#v", cmd())
	}
}

func TestTheBracketsMoveTheFamilyOneWordAtATime(t *testing.T) {
	p := NewPermissionDialogCmp().(*permissionDialogCmp)
	p.SetPermissions(familyRequest())
	before := p.family()
	p.Update(keyPress("["))
	shorter := p.family()
	if len(shorter) >= len(before) || !tools.BashFamilyCovers(famCmd, shorter) {
		t.Fatalf("[ did not shorten to a usable family: %q -> %q", before, shorter)
	}
	p.Update(keyPress("]"))
	if p.family() != before {
		t.Fatalf("] did not restore the family: %q", p.family())
	}
	// Whatever is shown is what f sends.
	p.Update(keyPress("["))
	_, cmd := p.Update(keyPress("f"))
	if msg := cmd().(PermissionResponseMsg); msg.Family != shorter {
		t.Fatalf("f sent %q, the dialog showed %q", msg.Family, shorter)
	}
}

func TestFWithoutAFamilyNeverWidens(t *testing.T) {
	p := NewPermissionDialogCmp().(*permissionDialogCmp)
	plain := familyRequest()
	plain.FamilySuggestion = ""
	p.SetPermissions(plain)
	_, cmd := p.Update(keyPress("f"))
	if cmd != nil {
		t.Fatalf("f produced an answer with no family on offer: %#v", cmd())
	}
}
