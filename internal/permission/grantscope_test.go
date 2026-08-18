package permission

// GORILLA FIX (2026-08-18): "Allow for session" must grant what the user was
// shown, not the whole tool.
//
// The dialog renders ONE command in a fenced block and offers "Allow for
// session". Upstream then stored a grant matched on
// ToolName+Action+SessionID+Path — Params was carried and never compared — so
// approving `cat README.md` silently authorised every later bash command in the
// session tree, including ones the user never saw.

import "testing"

func TestASessionGrantDoesNotCoverADifferentCommand(t *testing.T) {
	svc := NewPermissionService().(*permissionService)
	const sess = "s1"

	// The user approves one specific command for the session.
	svc.GrantPersistant(PermissionRequest{
		SessionID: sess,
		ToolName:  "bash",
		Action:    "execute",
		Path:      "/work",
		GrantKey:  "cat README.md",
	})

	// The same command again: covered, no prompt.
	if !svc.hasGrant(sess, "bash", "execute", "/work", "cat README.md") {
		t.Error("the approved command was not remembered — the grant is useless")
	}

	// A DIFFERENT command must NOT be covered.
	if svc.hasGrant(sess, "bash", "execute", "/work", "rm -rf /home/gorilla/Documents") {
		t.Error("approving one command authorised a completely different one — " +
			"this is the blanket-grant defect")
	}
}

// A tool that offers no key keeps the old tool-wide behaviour, so nothing that
// relied on it silently stops working.
func TestAKeylessGrantStillCoversTheTool(t *testing.T) {
	svc := NewPermissionService().(*permissionService)
	svc.GrantPersistant(PermissionRequest{
		SessionID: "s1", ToolName: "fetch", Action: "read", Path: "/work",
	})
	if !svc.hasGrant("s1", "fetch", "read", "/work", "") {
		t.Error("a keyless grant no longer covers its tool")
	}
}
