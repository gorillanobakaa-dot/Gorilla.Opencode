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

// GORILLA OVERRIDE (2026-10-23): "Allow for session" remembers a class plus a
// place. A bash class covers the same program and subcommand in the same place
// with a plain tail, and nothing wider: not another place, not a chained
// second command (the bash matcher refuses it).
func TestAllowForSessionRemembersAClassAndAPlace(t *testing.T) {
	s := familyService(t)
	RegisterFamilyMatcher("classtool", func(key, fam string) bool {
		return key == fam || (len(key) > len(fam) && key[:len(fam)+1] == fam+" " && !containsAny(key, ";&|"))
	})
	first := PermissionRequest{SessionID: "conversation", ToolName: "classtool", Action: "execute",
		Path: "/work", GrantKey: "pip3 install --user rich", SessionClass: "pip3 install"}
	s.GrantPersistant(first)
	same := classReq("pip3 install --user typer")
	same.Path = "/work"
	if !s.Request(same) {
		t.Error("the class the dialog printed did not cover the next pip3 install in the same place")
	}
	elsewhere := classReq("pip3 install --user typer")
	elsewhere.Path = "/elsewhere"
	if s.Request(elsewhere) {
		t.Error("the class covered a different place")
	}
	chained := classReq("pip3 install x; rm -rf ~")
	chained.Path = "/work"
	if s.Request(chained) {
		t.Error("the class covered a chained second command")
	}
}

// File changes: one "Allow for session" covers every change inside the same
// root, and none outside it.
func TestAllowForSessionOnAnEditCoversTheRootNotBeyond(t *testing.T) {
	s := familyService(t)
	s.GrantPersistant(PermissionRequest{SessionID: "conversation", ToolName: "edit", Action: "write",
		Path: "/work", GrantKey: "/work/a.go"})
	if !s.Request(CreatePermissionRequest{SessionID: "conversation", ToolName: "write", Action: "write",
		Path: "/work", GrantKey: "/work/sub/b.go"}) {
		t.Error("an edit grant did not cover another file change inside the same root")
	}
	if s.Request(CreatePermissionRequest{SessionID: "conversation", ToolName: "write", Action: "write",
		Path: "/other", GrantKey: "/other/b.go"}) {
		t.Error("an edit grant covered a file outside its root")
	}
	if got := SessionScope(PermissionRequest{ToolName: "edit", Path: "/work"}); got != "every file change inside /work" {
		t.Errorf("SessionScope = %q", got)
	}
}

func containsAny(s, chars string) bool {
	for _, c := range chars {
		for _, r := range s {
			if r == c {
				return true
			}
		}
	}
	return false
}

func classReq(key string) CreatePermissionRequest {
	r := familyReq(key)
	r.ToolName = "classtool"
	return r
}
