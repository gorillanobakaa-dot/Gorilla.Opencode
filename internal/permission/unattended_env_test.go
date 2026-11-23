package permission

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// Owner 2026-11-23: /plan could not write its plan. write/edit carry the
// workspace root in Path and the file in GrantKey.
func TestPlanModeWritesThePlanFileAsTheToolsAskForIt(t *testing.T) {
	s := withMode(t, config.PermissionPlan)
	root := config.WorkingDirectory()
	if !s.Request(CreatePermissionRequest{SessionID: "c", ToolName: "write", Action: "write",
		Path: root, GrantKey: root + "/docs/PLAN.md"}) {
		t.Fatal("plan mode refused to write PLAN.md in the workspace")
	}
	if s.Request(CreatePermissionRequest{SessionID: "c", ToolName: "write", Action: "write",
		Path: root, GrantKey: root + "/main.go"}) {
		t.Fatal("plan mode wrote source code")
	}
	if _, ok := TakePolicyRefusal("c"); !ok {
		t.Fatal("a plan-mode refusal must leave a per-call mark")
	}
}

// GORILLA_UNATTENDED=1 asks nothing, outside the roots too, and refuses an
// irreversible action per call.
func TestEnvUnattendedAsksNothing(t *testing.T) {
	s := withMode(t, config.PermissionAsk)
	n := countPrompts(t, s)
	t.Setenv("GORILLA_UNATTENDED", "1")
	if !s.Request(CreatePermissionRequest{SessionID: "c", ToolName: "view", Action: "read",
		Path: t.TempDir(), GrantKey: "shot.png"}) {
		t.Fatal("an outside-root read was refused while unattended")
	}
	if s.Request(CreatePermissionRequest{SessionID: "c", ToolName: "bash", Action: "execute",
		Path: config.WorkingDirectory(), GrantKey: "wipe", Irreversible: "it erases a disk"}) {
		t.Fatal("an irreversible action ran unattended")
	}
	if why, ok := TakePolicyRefusal("c"); !ok || why == "" {
		t.Fatal("the refusal left no reason for the model")
	}
	if *n != 0 {
		t.Fatalf("%d prompts raised while unattended", *n)
	}
}
