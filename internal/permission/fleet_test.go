package permission

// GORILLA OVERRIDE (2026-08-23): ROADMAP item 4.
//
// The owner, after being walked through why ten helpers meant ten prompts:
// "this is getting ridiculous. in order to get a web search either i have to
// click ten or twenty times if the search has to restart or what? most of the
// models will just allow you to accept once and that accept will work for all
// the batched models."
//
// The 2026-08-17 fix closed the SESSION axis and these tests do not re-test it.
// They pin the GRANT KEY axis, which is the one that was still open, and, just
// as importantly, they pin the three bounds that keep a fleet grant from being
// a hole: one tool, one run, and nothing inherited by anybody else.

import "testing"

func webSearch(session, query string) CreatePermissionRequest {
	return CreatePermissionRequest{
		SessionID: session,
		ToolName:  "web_search",
		Action:    "search",
		GrantKey:  query,
		Egress:    true,
	}
}

// THE REPORTED PROBLEM. Ten helpers, ten different queries, one approval.
//
// Without the fleet grant every one of these blocks on a prompt, so the test
// would hang rather than fail. PermissionWaitForTest keeps that honest: an
// unapproved request gives up quickly and returns false, and the assertion
// below reads "was asked again", not "timed out".
func TestOneApprovalCoversEveryQueryInTheRun(t *testing.T) {
	restore := PermissionWaitForTest(50 * 1000 * 1000) // 50ms
	defer restore()

	s := NewPermissionService().(*permissionService)
	const root = "conversation"
	for i := 0; i < 10; i++ {
		s.RegisterChildSession(helperID(i), root)
	}

	s.GrantFleet(root, []string{"web_search", "web_fetch"})

	queries := []string{
		"ath9k rfkill regression", "debian 6.12 kernel config", "vaio sve firmware",
		"lynx searxng headers", "codex rate limit headers", "pacman zst signing",
		"go embed directive", "bubbletea z order", "chroma retrieval header",
		"opus context window",
	}
	for i, q := range queries {
		if !s.Request(webSearch(helperID(i), q)) {
			t.Fatalf("helper %d was asked again for %q. One approval must cover the "+
				"whole run: being asked once per query is the exact complaint.", i, q)
		}
	}
}

// BOUND ONE: by tool. A fleet grant for searching must not approve a shell
// command, a file write, or anything else that happens to run in the same tree.
func TestAFleetGrantCoversOnlyTheToolsItNamed(t *testing.T) {
	restore := PermissionWaitForTest(50 * 1000 * 1000)
	defer restore()

	s := NewPermissionService().(*permissionService)
	const root = "conversation"
	s.RegisterChildSession("helper", root)
	s.GrantFleet(root, []string{"web_search"})

	if !s.Request(webSearch("helper", "anything")) {
		t.Fatal("the named tool was not covered")
	}
	if s.Request(CreatePermissionRequest{
		SessionID: "helper", ToolName: "bash", Action: "run", GrantKey: "rm -rf /",
	}) {
		t.Error("a fleet grant for web_search approved a bash command. It is scoped " +
			"to the tools named on the dialog, or it is just YOLO with extra steps.")
	}
	if s.Request(CreatePermissionRequest{
		SessionID: "helper", ToolName: "web_fetch", Action: "fetch", GrantKey: "http://x",
		Egress: true,
	}) {
		t.Error("web_fetch was approved but only web_search was granted")
	}
}

// BOUND TWO: by run. The research tool opens one and defers the close. If the
// close does not work, the widening outlives the thing it was granted for and
// every later search in that conversation is silently approved.
func TestRevokingEndsTheWidening(t *testing.T) {
	restore := PermissionWaitForTest(50 * 1000 * 1000)
	defer restore()

	s := NewPermissionService().(*permissionService)
	const root = "conversation"
	s.RegisterChildSession("helper", root)

	s.GrantFleet(root, []string{"web_search"})
	if !s.Request(webSearch("helper", "during the run")) {
		t.Fatal("not covered during the run")
	}

	s.RevokeFleet(root)
	if s.Request(webSearch("helper", "after the run")) {
		t.Error("still approving searches after the run ended. The grant has outlived " +
			"the run it was granted for, so the conversation is now permanently open.")
	}
}

// BOUND THREE: by conversation. Another conversation must not ride on it.
func TestAFleetGrantDoesNotLeakToAnotherConversation(t *testing.T) {
	restore := PermissionWaitForTest(50 * 1000 * 1000)
	defer restore()

	s := NewPermissionService().(*permissionService)
	s.RegisterChildSession("helper-a", "conversation-a")
	s.RegisterChildSession("helper-b", "conversation-b")
	s.GrantFleet("conversation-a", []string{"web_search"})

	if !s.Request(webSearch("helper-a", "q")) {
		t.Fatal("the granting conversation was not covered")
	}
	if s.Request(webSearch("helper-b", "q")) {
		t.Error("a grant made in one conversation approved a search in another")
	}
}

// IsCovered must answer for both kinds of grant, because the queue drain uses
// it to decide what NOT to ask, and a wrong "no" there is a duplicate prompt
// for something already approved.
func TestIsCoveredSeesBothKindsOfGrant(t *testing.T) {
	s := NewPermissionService().(*permissionService)
	s.RegisterChildSession("helper", "root")

	req := PermissionRequest{
		SessionID: "root", ToolName: "web_search", Action: "search", GrantKey: "q",
	}
	if s.IsCovered(req) {
		t.Fatal("covered with no grant of any kind")
	}

	s.GrantPersistant(req)
	if !s.IsCovered(req) {
		t.Error("an exact session grant was not recognised")
	}

	other := req
	other.GrantKey = "a different query"
	if s.IsCovered(other) {
		t.Error("a session grant for one query covered a different query. That is the " +
			"b333c23 property and it must survive the fleet grant existing.")
	}
	s.GrantFleet("root", []string{"web_search"})
	if !s.IsCovered(other) {
		t.Error("a fleet grant was not recognised by IsCovered, so the queue will ask " +
			"again for something the user just approved for the whole run")
	}
}

func helperID(i int) string {
	return string(rune('a'+i)) + "-helper"
}

// The grant is FILED under the tree root, because that is what Request resolves
// to before looking it up. Filing it under whatever id the caller held would put
// it somewhere nothing reads, and the symptom is the prompt storm it exists to
// stop, with no error to explain why. Research can be launched from inside a
// sub-agent, so the two ids are not always the same.
func TestAGrantOpenedFromInsideASubAgentStillCovers(t *testing.T) {
	restore := PermissionWaitForTest(50 * 1000 * 1000)
	defer restore()

	s := NewPermissionService().(*permissionService)
	s.RegisterChildSession("orchestrator", "conversation")
	s.RegisterChildSession("helper", "orchestrator")

	// Granted with the id the caller holds, which is NOT the root.
	s.GrantFleet("orchestrator", []string{"web_search"})

	if !s.Request(webSearch("helper", "q")) {
		t.Error("the helper was asked despite a fleet grant opened one level down. " +
			"GrantFleet must resolve to the same root Request does.")
	}
	s.RevokeFleet("orchestrator")
	if s.Request(webSearch("helper", "q2")) {
		t.Error("revoking with the caller's id did not clear the grant filed at the root")
	}
}

// TOOL-WIDE GRAIN. The dialog has three buttons and the per-query key made the
// middle one useless: "allow for session, for this exact string" is a grant
// nobody wants, because searches are not repeated word for word. So Allow was
// the only usable answer and the user paid for it once per search.
//
// With GrantWholeTool the two lifetimes people actually want land on the two
// buttons: this one search, or searching for the rest of the session.
func TestOneAllowForSessionCoversEverySearch(t *testing.T) {
	restore := PermissionWaitForTest(50 * 1000 * 1000)
	defer restore()

	s := NewPermissionService().(*permissionService)

	search := func(query string) CreatePermissionRequest {
		return CreatePermissionRequest{
			SessionID: "conversation", ToolName: "web_search", Action: "search",
			Path: "/work", GrantKey: GrantWholeTool, Egress: true,
			Description: "Search for: " + query,
		}
	}

	// The user answers ONE prompt with "allow for session".
	first := search("ath9k rfkill regression")
	s.GrantPersistant(PermissionRequest{
		SessionID: "conversation", ToolName: first.ToolName, Action: first.Action,
		Path: first.Path, GrantKey: first.GrantKey,
	})

	for _, q := range []string{
		"debian 6.12 kernel config", "vaio sve firmware", "searxng lynx headers",
	} {
		if !s.Request(search(q)) {
			t.Fatalf("asked again for %q after allow-for-session. The middle button has "+
				"to mean searching, or it means nothing and Allow is the only answer.", q)
		}
	}
}

// The grain is per TOOL, and web_fetch keeps its own. Allowing searching must
// not allow opening arbitrary sites: reaching a server is the thing a poisoned
// result would want, and it is still asked, per host.
func TestAllowingSearchDoesNotAllowOpeningPages(t *testing.T) {
	restore := PermissionWaitForTest(50 * 1000 * 1000)
	defer restore()

	s := NewPermissionService().(*permissionService)
	s.GrantPersistant(PermissionRequest{
		SessionID: "conversation", ToolName: "web_search", Action: "search",
		Path: "/work", GrantKey: GrantWholeTool,
	})

	if s.Request(CreatePermissionRequest{
		SessionID: "conversation", ToolName: "web_fetch", Action: "fetch",
		Path: "/work", GrantKey: "https://evil.example", Egress: true,
	}) {
		t.Error("a session-wide search grant also authorised fetching a page. The two " +
			"tools have different grains on purpose: search sends words out, fetch " +
			"chooses which server answers.")
	}
}
