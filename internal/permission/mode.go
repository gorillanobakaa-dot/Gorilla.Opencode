package permission

// GORILLA OVERRIDE (2026-10-11): the standing permission mode, applied inside
// Request. The modes and the reasoning are in config/permissionmode.go; this
// file holds what each one does to a request.

import (
	"fmt"

	"github.com/opencode-ai/opencode/internal/config"
)

// editTools are the tools accept-edits waves through: they change files and
// nothing else. A shell command can change anything, so it is not here.
var editTools = map[string]bool{
	"edit": true, "write": true, "patch": true, "sparse": true,
}

// EffectiveMode is what a session is actually running under: the standing
// mode, or "auto" when /yolo armed this one conversation.
func EffectiveMode(sessionID string) string {
	mode := config.PermissionMode()
	if mode != config.PermissionAuto && SessionAutoApproved(sessionID) {
		return config.PermissionAuto
	}
	return mode
}

// PlanModeRefusal is the tool result the model reads when plan mode refused
// a tool. It says that nobody was asked, so the model does not wait for an
// answer that is not coming, and what to do instead.
func PlanModeRefusal(toolName string) string {
	return fmt.Sprintf("Plan mode is on: the %s tool would change something or run a command, "+
		"so it was not run and nobody was asked. Describe what you would do and why, as a plan "+
		"the user can read and approve. The user switches modes with /mode or Shift+Tab.", toolName)
}

// modeDecision applies the standing mode to a request BEFORE grants are
// consulted. decided=false means "carry on with the normal path".
//
//   - plan refuses every non-egress request outright, without publishing a
//     prompt. Egress (a web read) still goes to the prompt: research is part
//     of planning, and the person decides it as they always did.
//   - accept-edits approves an edit tool inside the roots, unless a carve-out
//     applies (mustAskAnyway), in which case the normal path asks.
//   - auto joins the auto-approve path in Request, carve-outs included; ask
//     changes nothing.
func (s *permissionService) modeDecision(opts CreatePermissionRequest, root string) (decided bool, answer bool) {
	switch config.PermissionMode() {
	case config.PermissionPlan:
		if opts.Egress {
			return false, false
		}
		return true, false
	case config.PermissionAcceptEdits:
		if editTools[opts.ToolName] && s.mustAskAnyway(opts, root) == "" {
			return true, true
		}
	}
	return false, false
}
