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

// isPlanDocument reports whether path is a plan document: a note file
// (.md .txt .me .json .yaml .yml) whose name holds plan/roadmap/strategy/rfc/
// todo/improvement, or any note file inside a folder named like that
// (GORILLA FIX 2026-11-23: the owner's plan file is fix.me in an
// "Improvements" folder, which the old .md-only rule refused). Source code
// extensions never match.
func isPlanDocument(path string) bool {
	if path == "" {
		return false
	}
	clean := filepath.Clean(path)
	switch strings.ToLower(filepath.Ext(clean)) {
	case ".md", ".txt", ".me", ".json", ".yaml", ".yml":
	default:
		return false
	}
	words := []string{"plan", "roadmap", "strategy", "rfc", "todo", "improvement"}
	parts := strings.Split(strings.ToLower(filepath.ToSlash(clean)), "/")
	for _, part := range parts {
		for _, w := range words {
			if strings.Contains(part, w) {
				return true
			}
		}
	}
	return false
}

// modeDecision applies the standing mode to a request BEFORE grants are
// consulted. decided=false means "carry on with the normal path".
//
//   - plan refuses file changes outright (except the plan document), without a
//     prompt. Egress (a web read) and shell commands still go to the prompt:
//     research is part of planning, and the person decides it.
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
		// GORILLA FIX (2026-11-23): write and edit put the WORKSPACE ROOT in
		// Path (permissionScope) and the file itself in GrantKey, so testing
		// Path alone never matched a plan file and every plan write was
		// refused: /plan could not write its own plan. Test the file.
		if (opts.ToolName == "write" || opts.ToolName == "edit") &&
			(isPlanDocument(opts.GrantKey) || isPlanDocument(opts.Path)) {
			if s.mustAskAnyway(opts, root) == "" {
				return true, true
			}
			// Outside the workspace (e.g. an Improvements folder on the
			// Desktop): the person decides, as in ask mode, instead of a refusal.
			return false, false
		}
		// GORILLA FIX (2026-11-23): shell commands ask instead of being
		// refused outright. Read-only ones already run without a dialog; the
		// rest (python -m fieldkit size find, OCR helpers) were blocked, which
		// left the model unable to read anything and was called sabotage.
		if opts.ToolName == "bash" {
			return false, false
		}
		s.markPolicyRefusal(root, PlanModeRefusal(opts.ToolName))
		return true, false
	case config.PermissionAcceptEdits:
		if editTools[opts.ToolName] && s.mustAskAnyway(opts, root) == "" {
			return true, true
		}
	}
	return false, false
}
