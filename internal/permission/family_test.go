package permission

// GORILLA FIX (2026-10-11): "Allow family" — one answer for one program's many
// invocations, and the list of things it must never cover.
//
// Every test here is a protection that existed before families did, asserted
// AGAINST a family grant that would cover the request if the protection were
// gone. The measured motivation is in tools/commandfamily.go.

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// wordPrefixMatcher is the simplest honest matcher: the key begins with the
// family at a word boundary. The bash tool registers a stricter one; this
// package's rules are tested with the simple one so they cannot hide behind it.
func wordPrefixMatcher(key, family string) bool {
	if !strings.HasPrefix(key, family) {
		return false
	}
	return len(key) == len(family) || key[len(family)] == ' '
}

const familyTool = "family-test-tool"

func familyService(t *testing.T) *permissionService {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	RegisterFamilyMatcher(familyTool, wordPrefixMatcher)
	s := NewPermissionService().(*permissionService)
	denyPrompts(t, s) // an uncovered request is published, unanswered, denied in 50ms
	return s
}

func familyReq(key string) CreatePermissionRequest {
	return CreatePermissionRequest{
		SessionID: "conversation", ToolName: familyTool, Action: "execute",
		Path: config.WorkingDirectory(), GrantKey: key,
	}
}

// grantFamily records a family exactly as the dialog would: on a request that
// was published with this key.
func grantFamily(s *permissionService, key, family string) {
	s.GrantFamily(PermissionRequest{
		ID: "p", SessionID: "conversation", ToolName: familyTool, Action: "execute",
		Path: config.WorkingDirectory(), GrantKey: key,
	}, family)
}

func TestAFamilyCoversTheSameProgramWithADifferentTail(t *testing.T) {
	s := familyService(t)
	grantFamily(s, "fieldkit cards list", "fieldkit cards")

	if !s.Request(familyReq("fieldkit cards show office")) {
		t.Error("a command inside the family still prompted — the grant is useless")
	}
	if !s.Request(familyReq("fieldkit cards")) {
		t.Error("the family itself, bare, is not covered")
	}
	if s.Request(familyReq("fieldkit cardsx list")) {
		t.Error("`fieldkit cardsx` was covered by the family `fieldkit cards`: the word boundary is gone")
	}
	if s.Request(familyReq("go test ./...")) {
		t.Error("a different program was covered by the family")
	}
}

func TestAFamilyIsBoundToItsToolActionPathAndTree(t *testing.T) {
	s := familyService(t)
	grantFamily(s, "fieldkit cards list", "fieldkit cards")

	other := familyReq("fieldkit cards show")
	other.ToolName = "edit"
	if s.Request(other) {
		t.Error("a family for one tool covered another tool")
	}
	other = familyReq("fieldkit cards show")
	other.Action = "write"
	if s.Request(other) {
		t.Error("a family for one action covered another action")
	}
	other = familyReq("fieldkit cards show")
	other.Path = "/somewhere/else/entirely"
	if s.Request(other) {
		t.Error("a family approved on one path covered a command naming a different path")
	}
	other = familyReq("fieldkit cards show")
	other.SessionID = "another-conversation"
	if s.Request(other) {
		t.Error("a family granted in one conversation covered another conversation")
	}

	// The tree under the conversation IS the conversation.
	s.RegisterChildSession("helper-1", "conversation")
	child := familyReq("fieldkit cards show")
	child.SessionID = "helper-1"
	if !s.Request(child) {
		t.Error("a helper under the conversation was not covered by its family grant")
	}
}

func TestAFamilyCoversPathsInsideTheFolderItWasApprovedOn(t *testing.T) {
	s := familyService(t)
	dir := t.TempDir()
	s.GrantFamily(PermissionRequest{
		ID: "p", SessionID: "conversation", ToolName: familyTool, Action: "execute",
		Path: dir, GrantKey: "fieldkit privacy scan " + dir,
	}, "fieldkit privacy")

	inside := familyReq("fieldkit privacy scan README")
	inside.Path = dir + "/README.md"
	if !s.Request(inside) {
		t.Error("a command naming a file INSIDE the approved folder was not covered")
	}
	outside := familyReq("fieldkit privacy scan keys")
	outside.Path = t.TempDir()
	if s.Request(outside) {
		t.Error("a command naming a path OUTSIDE the approved folder was covered")
	}
}

// Irreversible, egress, tainted: the three things auto-approve asks about
// anyway, and the three things a family must never wave through.
func TestAFamilyNeverCoversAnIrreversibleRequest(t *testing.T) {
	s := familyService(t)
	grantFamily(s, "rm -rf ./build", "rm -rf")
	r := familyReq("rm -rf /")
	r.Irreversible = "it deletes the whole disk"
	if s.Request(r) {
		t.Fatal("a family covered a request marked irreversible")
	}
}

func TestAFamilyNeverCoversEgress(t *testing.T) {
	s := familyService(t)
	grantFamily(s, "fetch https://a.example/x", "fetch")
	r := familyReq("fetch https://b.example/y")
	r.Egress = true
	if s.Request(r) {
		t.Fatal("a family covered a request that leaves the machine")
	}
}

func TestAFamilyNeverCoversATaintedTurn(t *testing.T) {
	s := familyService(t)
	grantFamily(s, "fieldkit cards list", "fieldkit cards")
	MarkTainted("conversation", "read a web page")
	t.Cleanup(func() { ClearTaint("conversation") })
	if s.Request(familyReq("fieldkit cards show")) {
		t.Fatal("a family covered a command raised after the turn read untrusted content")
	}
	ClearTaint("conversation")
	if !s.Request(familyReq("fieldkit cards show")) {
		t.Fatal("after the taint cleared, the family did not cover the command again")
	}
}

// With auto-approve on, the carve-outs run first and a family must not undo
// them: the same request asks exactly as it did before families existed.
func TestUnderAutoApproveAFamilyStillAsksForTheCarveOuts(t *testing.T) {
	s := familyService(t)
	s.AutoApproveSession("conversation")
	grantFamily(s, "fieldkit cards list", "fieldkit cards")

	r := familyReq("fieldkit cards purge")
	r.Irreversible = "it deletes everything"
	if s.Request(r) {
		t.Error("auto-approve + family covered an irreversible request")
	}
	r = familyReq("fieldkit cards upload")
	r.Egress = true
	if s.Request(r) {
		t.Error("auto-approve + family covered egress")
	}
	MarkTainted("conversation", "read a web page")
	t.Cleanup(func() { ClearTaint("conversation") })
	if s.Request(familyReq("fieldkit cards show")) {
		t.Error("auto-approve + family covered a tainted turn")
	}
}

func TestAFamilyThatDoesNotCoverItsOwnRequestIsNotRecorded(t *testing.T) {
	s := familyService(t)
	grantFamily(s, "python evil.py", "go build")
	if n := len(s.familyGrants); n != 0 {
		t.Fatalf("a family that does not describe the request it answered was recorded (%d grants)", n)
	}
	grantFamily(s, "fieldkit cards list", "")
	if n := len(s.familyGrants); n != 0 {
		t.Fatalf("an empty family was recorded (%d grants)", n)
	}
}

func TestAToolWithoutAMatcherGetsNoFamilyGrants(t *testing.T) {
	s := familyService(t)
	s.GrantFamily(PermissionRequest{
		ID: "p", SessionID: "conversation", ToolName: "edit", Action: "write",
		Path: config.WorkingDirectory(), GrantKey: "/work/a.go",
	}, "/work")
	if n := len(s.familyGrants); n != 0 {
		t.Fatalf("a tool that never opted into families received one (%d grants)", n)
	}
}

func TestIsCoveredHonoursFamiliesWithTheSameRefusals(t *testing.T) {
	s := familyService(t)
	grantFamily(s, "fieldkit cards list", "fieldkit cards")
	queued := PermissionRequest{
		SessionID: "conversation", ToolName: familyTool, Action: "execute",
		Path: config.WorkingDirectory(), GrantKey: "fieldkit cards show",
	}
	if !s.IsCovered(queued) {
		t.Error("a queued request inside the family was not reported as covered")
	}
	queued.Irreversible = "it deletes everything"
	if s.IsCovered(queued) {
		t.Error("a queued irreversible request was reported as covered by a family")
	}
}

func TestAskedKeysRecordsOnlyPromptsThatReachedAPerson(t *testing.T) {
	s := familyService(t)
	s.Request(familyReq("fieldkit host")) // published, unanswered, denied
	s.GrantPersistant(PermissionRequest{
		SessionID: "conversation", ToolName: familyTool, Action: "execute",
		Path: config.WorkingDirectory(), GrantKey: "fieldkit readiness",
	})
	s.Request(familyReq("fieldkit readiness")) // covered: never published

	got := s.AskedKeys("conversation", familyTool)
	if len(got) != 1 || got[0] != "fieldkit host" {
		t.Fatalf("AskedKeys = %q, want only the one prompt that was published", got)
	}
	if other := s.AskedKeys("conversation", "edit"); len(other) != 0 {
		t.Fatalf("AskedKeys for another tool = %q, want none", other)
	}
}

func TestPathWithin(t *testing.T) {
	cases := []struct {
		p, dir string
		want   bool
	}{
		{"/a/b", "/a/b", true},
		{"/a/b/c.txt", "/a/b", true},
		{"/a/bc", "/a/b", false},
		{"/a", "/a/b", false},
		{"", "", true},
		{"/a/b", "", false},
	}
	for _, c := range cases {
		if got := pathWithin(c.p, c.dir); got != c.want {
			t.Errorf("pathWithin(%q, %q) = %v, want %v", c.p, c.dir, got, c.want)
		}
	}
}
