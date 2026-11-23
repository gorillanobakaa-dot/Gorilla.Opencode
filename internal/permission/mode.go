package permission

// GORILLA OVERRIDE (2026-10-11): the standing permission mode, applied inside
// Request. The modes and the reasoning are in config/permissionmode.go; this
// file holds what each one does to a request.

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

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

// Refused is the error a tool returns when its permission request was not
// granted. A person's Deny (or a refusal by a rule) is ErrorPermissionDenied,
// which the agent loop treats as the end of the batch. Plan mode refuses
// without asking anyone, so it is an ordinary tool error carrying the plan-mode
// explanation: the model reads it, the other calls in the same message still
// run, and the turn is not ended (found by audit 2026-11-23: one refused edit
// cancelled every other call as "canceled by user").
func Refused(toolName string) error {
	if config.PermissionMode() == config.PermissionPlan {
		return errors.New(PlanModeRefusal(toolName))
	}
	return ErrorPermissionDenied
}

// isPlanDocument reports whether path is the plan document itself (PLAN.md,
// *plan*.md, roadmap/strategy/rfc/todo notes). Plan mode lets the agent record
// its plan there; source files stay refused.
func isPlanDocument(path string) bool {
	if path == "" {
		return false
	}
	clean := filepath.Clean(path)
	base := strings.ToLower(filepath.Base(clean))
	if strings.ToLower(filepath.Ext(clean)) != ".md" {
		return false
	}
	for _, w := range []string{"plan", "roadmap", "strategy", "rfc", "todo"} {
		if strings.Contains(base, w) {
			return true
		}
	}
	return false
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
		// The plan document itself may be written; source code may not.
		if (opts.ToolName == "write" || opts.ToolName == "edit") && isPlanDocument(opts.Path) && s.mustAskAnyway(opts, root) == "" {
			return true, true
		}
		return true, false
	case config.PermissionAcceptEdits:
		if editTools[opts.ToolName] && s.mustAskAnyway(opts, root) == "" {
			return true, true
		}
	}
	return false, false
}
