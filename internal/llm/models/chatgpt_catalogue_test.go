package models

import (
	"os"
	"testing"
)

// liveChatGPTPayload is cut from the real GET /models response taken on
// 2026-08-23 on a free plan (HTTP 200, 5 models, 214 KB). Only the fields this
// package reads are kept: the real payload also carries the entire Codex system
// prompt per model, which is 40 KB of text with no bearing on any of this.
//
// The VALUES are verbatim. In particular the priority numbers (2, 3, 7, 23, 43)
// are the backend's own ordering, and the whole point of the change these tests
// cover is that the order comes from there rather than from a person.
const liveChatGPTPayload = `{"models":[
 {"slug":"gpt-5.6-terra","display_name":"GPT-5.6-Terra","visibility":"list",
  "tool_mode":"code_mode_only","context_window":272000,"max_context_window":872000,
  "priority":2,"available_in_plans":["free","plus","pro"],
  "description":"Balanced agentic coding model for everyday work.",
  "supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"},{"effort":"high"},
   {"effort":"xhigh"},{"effort":"max"},{"effort":"ultra"}]},
 {"slug":"gpt-5.6-luna","display_name":"GPT-5.6-Luna","visibility":"list",
  "tool_mode":"code_mode_only","context_window":272000,"max_context_window":872000,
  "priority":3,"available_in_plans":["free","plus","pro"],
  "description":"Fast and affordable agentic coding model.",
  "supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"},{"effort":"high"},
   {"effort":"xhigh"},{"effort":"max"}]},
 {"slug":"gpt-5.5","display_name":"GPT-5.5","visibility":"list",
  "tool_mode":null,"context_window":272000,"max_context_window":272000,
  "priority":7,"available_in_plans":["free","plus","pro"],
  "description":"Frontier model for complex coding, research, and real-world work.",
  "supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"},{"effort":"high"},
   {"effort":"xhigh"}]},
 {"slug":"gpt-5.4-mini","display_name":"GPT-5.4-Mini","visibility":"list",
  "tool_mode":null,"context_window":272000,"max_context_window":272000,
  "priority":23,"available_in_plans":["free","plus","pro"],
  "description":"Small, fast, and cost-efficient model for simpler coding tasks.",
  "supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"}]},
 {"slug":"codex-auto-review","display_name":"Codex Auto Review","visibility":"hide",
  "tool_mode":"code_mode_only","context_window":272000,"max_context_window":872000,
  "priority":43,"available_in_plans":["plus","pro"],
  "description":"Automatic approval review model for Codex.",
  "supported_reasoning_levels":[{"effort":"low"}]}
]}`

func parseFixture(t *testing.T) []ChatGPTCatalogueEntry {
	t.Helper()
	entries, err := ParseChatGPTCatalogue([]byte(liveChatGPTPayload))
	if err != nil {
		t.Fatalf("recorded live payload did not parse: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("got %d entries, want the 5 the backend really sent", len(entries))
	}
	return entries
}

// The backend's visibility flag is the only filter. codex-auto-review is an
// approval-review endpoint, not a chat model; offering it would be the same
// mistake as putting whisper in a chat picker.
func TestHiddenEntriesAreNotOffered(t *testing.T) {
	built := BuildChatGPTModels(parseFixture(t))
	if len(built) != 4 {
		t.Fatalf("got %d models, want 4 (5 listed, codex-auto-review hidden)", len(built))
	}
	for _, m := range built {
		if m.APIModel == "codex-auto-review" {
			t.Error("codex-auto-review is visibility:hide and must never reach the picker")
		}
	}
}

// THE REGRESSION THIS WHOLE CHANGE EXISTS FOR. The 5.6 pair carries
// tool_mode:code_mode_only, and the previous code excluded them on the untested
// inference that they would "fail on the first tool call". Measured 2026-08-23:
// both answer ordinary function calls with HTTP 200. They must be offered.
func TestCodeModeOnlyModelsAreStillOffered(t *testing.T) {
	built := BuildChatGPTModels(parseFixture(t))
	want := map[string]bool{"gpt-5.6-terra": false, "gpt-5.6-luna": false}
	for _, m := range built {
		if _, ok := want[m.APIModel]; ok {
			want[m.APIModel] = true
		}
	}
	for slug, found := range want {
		if !found {
			t.Errorf("%s is tool_mode:code_mode_only and was dropped. "+
				"That flag is a client-side tool-presentation choice, not a backend "+
				"requirement; both models were verified answering ordinary function "+
				"calls on 2026-08-23.", slug)
		}
	}
}

// Order comes from the backend's priority field, not from a hand-written list.
// terra(2) < luna(3) < 5.5(7) < 5.4-mini(23), and this program's Rank is
// higher-is-better, so the ranks must descend in that same order.
func TestOrderFollowsTheBackendsOwnPriority(t *testing.T) {
	built := BuildChatGPTModels(parseFixture(t))
	wantOrder := []string{"gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4-mini"}
	for i, slug := range wantOrder {
		if built[i].APIModel != slug {
			t.Fatalf("position %d is %s, want %s (backend priority order)", i, built[i].APIModel, slug)
		}
	}
	for i := 1; i < len(built); i++ {
		if built[i].Rank >= built[i-1].Rank {
			t.Errorf("rank did not descend: %s=%d then %s=%d",
				built[i-1].APIModel, built[i-1].Rank, built[i].APIModel, built[i].Rank)
		}
	}
	if built[0].Rank != chatgptTopRank {
		t.Errorf("best model ranked %d, want %d so the picker does not reshuffle "+
			"for users who were on the hand-written list", built[0].Rank, chatgptTopRank)
	}
}

// The operating window is context_window. max_context_window is larger for the
// 5.6 pair (872K) but is not what an ordinary request gets, and a window set too
// high sends a request the backend rejects.
func TestContextWindowUsesTheOperatingValueNotTheCeiling(t *testing.T) {
	for _, m := range BuildChatGPTModels(parseFixture(t)) {
		if m.ContextWindow != 272_000 {
			t.Errorf("%s context window %d, want 272000 (context_window, not max_context_window)",
				m.APIModel, m.ContextWindow)
		}
	}
}

// Nothing on this path is billed per token. Populating cost fields with API
// prices would show the user money this route never charges.
func TestNothingOnThisPathCarriesAPrice(t *testing.T) {
	for _, m := range BuildChatGPTModels(parseFixture(t)) {
		if m.CostPer1MIn != 0 || m.CostPer1MOut != 0 {
			t.Errorf("%s carries a price (%v in, %v out); the ChatGPT plan is billed, not the token",
				m.APIModel, m.CostPer1MIn, m.CostPer1MOut)
		}
	}
}

// CanReason comes from the wire, not a guess: claiming it falsely makes the
// client send a reasoning field the model rejects with a 400.
func TestCanReasonComesFromTheWire(t *testing.T) {
	for _, m := range BuildChatGPTModels(parseFixture(t)) {
		if !m.CanReason {
			t.Errorf("%s: every model in the recorded payload publishes reasoning levels", m.APIModel)
		}
	}
	none := BuildChatGPTModels([]ChatGPTCatalogueEntry{{Slug: "x", Visibility: "list"}})
	if len(none) != 1 || none[0].CanReason {
		t.Error("a model publishing no reasoning levels must not be marked as reasoning")
	}
}

// An empty list is NOT success. Registering it would silently empty the picker
// and read exactly like a working refresh.
func TestAnEmptyListIsRefusedRatherThanRegistered(t *testing.T) {
	dir := t.TempDir()
	if _, err := RefreshChatGPT(dir, []byte(`{"models":[]}`)); err == nil {
		t.Error("an empty model list was accepted; it must be refused")
	}
	if _, err := RefreshChatGPT(dir, []byte(`{"models":[{"slug":"x","visibility":"hide"}]}`)); err == nil {
		t.Error("a list with no CHAT models was accepted; it must be refused")
	}
	if _, err := RefreshChatGPT(dir, []byte(`not json`)); err == nil {
		t.Error("an unreadable body was accepted")
	}
}

// Refresh REPLACES rather than merges, unlike the Antigravity one. A model the
// provider has retired must actually disappear: GPT-5.4 and 5.4-mini stop being
// served to ChatGPT sign-ins on 31 Aug 2026, and a merge would leave the entry
// in the picker looking selectable and returning a 400 to whoever chose it.
func TestRefreshRemovesRetiredModelsRatherThanMergingThem(t *testing.T) {
	dir := t.TempDir()
	restore := snapshotChatGPT()
	defer restore()

	if _, err := RefreshChatGPT(dir, []byte(liveChatGPTPayload)); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if _, ok := SupportedModels[ChatGPT54Mini]; !ok {
		t.Fatal("5.4-mini should be registered before its retirement")
	}

	// What 1 Sep 2026 looks like: the backend stops listing the retired models.
	after := `{"models":[{"slug":"gpt-5.6-terra","display_name":"GPT-5.6-Terra",
	 "visibility":"list","context_window":272000,"priority":2,
	 "supported_reasoning_levels":[{"effort":"low"}]}]}`
	res, err := RefreshChatGPT(dir, []byte(after))
	if err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if _, ok := SupportedModels[ChatGPT54Mini]; ok {
		t.Error("5.4-mini survived a refresh that did not list it; it would 400 when picked")
	}
	if _, ok := SupportedModels[ChatGPT55]; ok {
		t.Error("gpt-5.5 survived a refresh that did not list it")
	}
	if len(res.Removed) != 3 {
		t.Errorf("reported %d retired (%v), want 3", len(res.Removed), res.Removed)
	}
}

// The cache is what makes launch free: disk only, never the network.
func TestCacheRoundTripsAndSurvivesCorruption(t *testing.T) {
	dir := t.TempDir()
	restore := snapshotChatGPT()
	defer restore()

	if _, err := RefreshChatGPT(dir, []byte(liveChatGPTPayload)); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	n, err := LoadRefreshedChatGPT(dir)
	if err != nil || n != 4 {
		t.Fatalf("cache reload gave %d models, err %v; want 4", n, err)
	}
	if age, ok := ChatGPTCatalogueAge(dir); !ok || age < 0 {
		t.Error("a cached list must carry a fetch time; a list with no date is not a measurement")
	}

	// A corrupt cache must not leave the user with no models at all.
	if err := os.WriteFile(chatgptCachePath(dir), []byte("{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRefreshedChatGPT(dir); err == nil {
		t.Error("a corrupt cache should report the problem")
	}
	// A missing cache is the normal first-run state, not an error.
	if n, err := LoadRefreshedChatGPT(t.TempDir()); n != 0 || err != nil {
		t.Errorf("missing cache gave (%d, %v), want (0, nil)", n, err)
	}
}

// The coder gets the best model; the background agents get the cheapest, because
// on a free plan the cooldown is the scarce resource and titles must not be
// generated by the strong model.
func TestPreferredModelsSplitStrongAndCheap(t *testing.T) {
	restore := snapshotChatGPT()
	defer restore()

	applyChatGPT(BuildChatGPTModels(parseFixture(t)))
	best, cheap := PreferredChatGPTModels()
	if best != ChatGPT56Terra {
		t.Errorf("coder model = %s, want %s (backend priority 2)", best, ChatGPT56Terra)
	}
	if cheap != ChatGPT54Mini {
		t.Errorf("background model = %s, want %s (cheapest against the same limit)", cheap, ChatGPT54Mini)
	}

	// One model left is a legitimate state. Both roles land on it rather than
	// one of them landing on an empty id.
	applyChatGPT([]Model{{ID: "chatgpt.only", Provider: ProviderChatGPT, Rank: 5}})
	best, cheap = PreferredChatGPTModels()
	if best != "chatgpt.only" || cheap != "chatgpt.only" {
		t.Errorf("single model gave (%s, %s), want both on chatgpt.only", best, cheap)
	}
}

// snapshotChatGPT saves and restores this provider's registry entries. The
// registry is a package global and these tests replace it; without this the
// first one to run would decide what the rest see.
func snapshotChatGPT() func() {
	saved := map[ModelID]Model{}
	for id, m := range SupportedModels {
		if m.Provider == ProviderChatGPT {
			saved[id] = m
		}
	}
	return func() {
		for id, m := range SupportedModels {
			if m.Provider == ProviderChatGPT {
				delete(SupportedModels, id)
			}
		}
		for id, m := range saved {
			SupportedModels[id] = m
		}
	}
}
