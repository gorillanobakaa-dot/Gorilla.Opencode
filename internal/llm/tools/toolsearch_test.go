package tools

import (
	"context"
	"strings"
	"testing"
)

func catalogue() []BaseTool {
	return []BaseTool{
		NewReviewTool(nil),
		NewPatchPortTool(nil),
		NewBioDataTool(nil),
		NewWebSearchTool(nil),
		NewFetchTool(nil),
		NewSparseTool(nil),
	}
}

// The rule that makes the whole thing safe: the way back in is never withheld.
// The Claude API refuses this configuration outright ("At least one tool must
// have defer_loading=false"), and a local model would simply have nothing to
// call.
func TestTheSearchToolIsNeverDeferred(t *testing.T) {
	if IsDeferrable(ToolSearchToolName) {
		t.Fatal("tool_search is deferrable: nothing could ever be discovered")
	}
	for name := range NeverDeferred {
		if IsDeferrable(name) {
			t.Errorf("%s is in NeverDeferred and still reports as deferrable", name)
		}
	}
}

// The tools needed to read, write and run code must always be present. A model
// that has to search before it can edit a file is worse than one that costs
// more.
func TestEverydayToolsAreNeverDeferred(t *testing.T) {
	for _, name := range []string{"bash", "edit", "view", "write", "find", "patch"} {
		if IsDeferrable(name) {
			t.Errorf("%s is deferred; the agent could not work without searching first", name)
		}
	}
}

func TestVisibleToolsWithholdsUntilDiscovered(t *testing.T) {
	const sid = "session-withhold"
	ForgetSession(sid)
	defer ForgetSession(sid)

	all := catalogue()

	// Disabled: nothing is withheld, and the slice is unchanged.
	if got := VisibleTools(all, sid, false); len(got) != len(all) {
		t.Fatalf("disabled: %d tools, want all %d", len(got), len(all))
	}

	// Enabled: every one of these is deferrable, so none should be visible.
	got := VisibleTools(all, sid, true)
	if len(got) != 0 {
		var names []string
		for _, g := range got {
			names = append(names, g.Info().Name)
		}
		t.Fatalf("expected everything withheld, still visible: %v", names)
	}

	// After discovery, exactly that one appears.
	MarkDiscovered(sid, ReviewToolName)
	got = VisibleTools(all, sid, true)
	if len(got) != 1 || got[0].Info().Name != ReviewToolName {
		t.Fatalf("after discovering review, visible = %v", namesOf(got))
	}
}

// Sessions must not inherit each other's discoveries, or a fresh conversation
// would silently start expensive.
func TestDiscoveryIsPerSession(t *testing.T) {
	ForgetSession("s1")
	ForgetSession("s2")
	defer ForgetSession("s1")
	defer ForgetSession("s2")

	MarkDiscovered("s1", ReviewToolName)
	if !IsDiscovered("s1", ReviewToolName) {
		t.Error("s1 did not record its own discovery")
	}
	if IsDiscovered("s2", ReviewToolName) {
		t.Error("s2 inherited a discovery from s1")
	}
	ForgetSession("s1")
	if IsDiscovered("s1", ReviewToolName) {
		t.Error("ForgetSession did not clear the discovery")
	}
}

func TestSearchBySelectLoadsExactlyWhatWasNamed(t *testing.T) {
	got := searchCatalogue(catalogue(), "select:review,patch_port", 5)
	if want := []string{"review", "patch_port"}; !equal(namesOf(got), want) {
		t.Errorf("select: got %v, want %v", namesOf(got), want)
	}
	// Case-insensitive, and unknown names are simply absent rather than fatal.
	got = searchCatalogue(catalogue(), "SELECT:Review,does_not_exist", 5)
	if !equal(namesOf(got), []string{"review"}) {
		t.Errorf("mixed case / unknown: got %v", namesOf(got))
	}
}

// The case that matters in practice: the model describes the JOB, not the tool.
func TestSearchByDescriptionFindsTheRightTool(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"forward-port a patch onto a newer kernel", "patch_port"},
		{"run static analysis and find security bugs", "review"},
		{"look up a protein sequence by accession", "bio_lookup"},
		{"search the web for papers", "web_search"},
		{"download the contents of a URL", "web_fetch"},
	}
	for _, tc := range cases {
		got := searchCatalogue(catalogue(), tc.query, 3)
		if len(got) == 0 {
			t.Errorf("%q matched nothing", tc.query)
			continue
		}
		if got[0].Info().Name != tc.want {
			t.Errorf("%q -> %v, wanted %s first", tc.query, namesOf(got), tc.want)
		}
	}
}

func TestSearchRespectsTheLimit(t *testing.T) {
	got := searchCatalogue(catalogue(), "search", 2)
	if len(got) > 2 {
		t.Errorf("limit 2 returned %d", len(got))
	}
}

// A miss must say so, and list what IS available — otherwise the model
// concludes the capability is absent and tells the user so.
func TestAMissListsWhatCanBeLoaded(t *testing.T) {
	tool := NewToolSearchTool(catalogue)
	resp, err := tool.Run(context.Background(),
		ToolCall{Input: `{"query":"zzzz nothing like this exists"}`})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(resp.Content, "review") {
		t.Errorf("a miss did not list the available tools:\n%s", resp.Content)
	}
	// Assert what the message has to ACHIEVE, not the words it uses. The
	// original pinned the literal "does not exist" and broke the moment the
	// wording improved, which tells you nothing about whether the message
	// still works.
	//
	// A miss has three jobs: show what IS available, say how to load one, and
	// stop the model announcing that something is impossible while a tool for
	// it is on screen. That last one is not hypothetical -- it is what Gemma
	// did before this message was rewritten.
	if !strings.Contains(resp.Content, "select:") {
		t.Error("a miss must say how to load a tool by name")
	}
	if !strings.Contains(resp.Content, "not exist") {
		t.Error("a miss should leave open that the capability may genuinely be absent")
	}
	if !strings.Contains(resp.Content, "impossible") {
		t.Error("a miss must warn against declaring a task impossible while a tool is listed")
	}
}

// Running the tool must actually make the tool callable afterwards. This is the
// end-to-end behaviour; everything else is detail.
func TestRunningTheSearchMakesTheToolVisible(t *testing.T) {
	const sid = "session-e2e"
	ForgetSession(sid)
	defer ForgetSession(sid)

	all := catalogue()
	if len(VisibleTools(all, sid, true)) != 0 {
		t.Fatal("precondition: something was visible before searching")
	}

	ctx := context.WithValue(context.Background(), SessionIDContextKey, sid)
	ctx = context.WithValue(ctx, MessageIDContextKey, "m1")
	resp, err := NewToolSearchTool(catalogue).Run(ctx,
		ToolCall{Input: `{"query":"port a patch to a new kernel version"}`})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(resp.Content, "patch_port") {
		t.Fatalf("search did not report loading patch_port:\n%s", resp.Content)
	}
	if !IsDiscovered(sid, PatchPortToolName) {
		t.Fatal("the tool was reported as loaded but not recorded as discovered")
	}
	if !containsName(namesOf(VisibleTools(all, sid, true)), PatchPortToolName) {
		t.Fatal("patch_port is still withheld after being discovered")
	}
}

// The index in the system prompt is the only way the model knows what exists.
// Without it, deferral is just missing tools.
func TestCatalogueBlockNamesEveryDeferredTool(t *testing.T) {
	block := DeferredCatalogueBlock(catalogue())
	for _, tl := range catalogue() {
		if !strings.Contains(block, tl.Info().Name) {
			t.Errorf("%s is deferred but absent from the catalogue block", tl.Info().Name)
		}
	}
	if !strings.Contains(block, ToolSearchToolName) {
		t.Error("the block does not say how to load anything")
	}
	// It must stay an index, not a second copy of every description.
	if len(block) > 3000 {
		t.Errorf("catalogue block is %d chars; it is supposed to be cheap", len(block))
	}
}

func namesOf(ts []BaseTool) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Info().Name)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsName(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

// The observed failure, now guarded: search for the task SUBJECT, miss, repeat
// the identical query, and give up while the right tool is one word away.
//
// Gemma 4 E2B did exactly this with "official Go release notes for generics",
// twice, then told the user it could not search the web. web_search was listed
// in both misses.
func TestARepeatedFailingSearchRescuesRatherThanStrands(t *testing.T) {
	const sid = "stuck-session"
	ForgetSession(sid)
	defer ForgetSession(sid)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, sid)
	ctx = context.WithValue(ctx, MessageIDContextKey, "m1")
	tool := NewToolSearchTool(catalogue)
	q := `{"query":"official Go release notes for generics"}`

	first, err := tool.Run(ctx, ToolCall{Input: q})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if IsDiscovered(sid, WebSearchToolName) {
		t.Fatal("the first miss loaded something; the relevance floor is not holding")
	}
	if !strings.Contains(first.Content, "web_search") {
		t.Error("the miss did not list web_search, so the model had nothing to act on")
	}
	if !strings.Contains(first.Content, "select:") {
		t.Error("the miss did not tell the model how to load a tool by name")
	}

	second, err := tool.Run(ctx, ToolCall{Input: q})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !IsDiscovered(sid, WebSearchToolName) {
		t.Errorf("the repeated search still loaded nothing; the model would give up here.\n%s",
			second.Content)
	}
	if !strings.Contains(second.Content, "second time") {
		t.Error("the rescue did not say why it loaded something")
	}
}

// The rescue must not become a licence to spend: one tool, not five.
func TestTheRescueLoadsExactlyOneTool(t *testing.T) {
	const sid = "stuck-once"
	ForgetSession(sid)
	defer ForgetSession(sid)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, sid)
	ctx = context.WithValue(ctx, MessageIDContextKey, "m1")
	tool := NewToolSearchTool(catalogue)
	q := `{"query":"something vague and unmatched entirely"}`
	tool.Run(ctx, ToolCall{Input: q})
	tool.Run(ctx, ToolCall{Input: q})
	if n := DiscoveredCount(sid); n > 1 {
		t.Errorf("the rescue loaded %d tools; it must load at most one", n)
	}
}

// A different query is not a repeat, or the rescue would fire on ordinary
// browsing of the catalogue.
func TestADifferentQueryIsNotARepeat(t *testing.T) {
	const sid = "not-a-repeat"
	ForgetSession(sid)
	defer ForgetSession(sid)
	if repeatedQuery(sid, "alpha") {
		t.Error("the first query counted as a repeat")
	}
	if repeatedQuery(sid, "beta") {
		t.Error("a different query counted as a repeat")
	}
	if !repeatedQuery(sid, "beta") {
		t.Error("the same query twice was not detected")
	}
	if !repeatedQuery(sid, "  BETA  ") {
		t.Error("case and spacing should not defeat repeat detection")
	}
}
