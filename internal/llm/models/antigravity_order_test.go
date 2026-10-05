package models

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// The backend's answer on the owner's account, 2026-10-05, trimmed to the
// fields that decide behaviour. Note what it does NOT contain: claude-sonnet-4-6
// and claude-opus-4-6-thinking, the two models the built-in list ranks highest.
func rowsOf20261005() []AntigravityRow {
	row := func(id, name string) AntigravityRow {
		return AntigravityRow{ID: id, DisplayName: name, MaxTokens: 1048576, MaxOutputTokens: 65536,
			SupportsThinking: true, SupportsImages: true}
	}
	return []AntigravityRow{
		row("claude-opus-5-5-high", "Claude Opus 5.5 (High)"),
		row("claude-opus-5-5-low", "Claude Opus 5.5 (Low)"),
		row("claude-opus-5-5-medium", "Claude Opus 5.5 (Medium)"),
		row("claude-sonnet-5-5-high", "Claude Sonnet 5.5 (High)"),
		row("claude-sonnet-5-5-low", "Claude Sonnet 5.5 (Low)"),
		row("claude-sonnet-5-5-medium", "Claude Sonnet 5.5 (Medium)"),
		row("gpt-oss-120b-medium", "GPT-OSS 120B (Medium)"),
		row("gemini-2.5-pro", "Gemini 2.5 Pro"),
		row("gemini-3.1-pro-high", "Gemini 3.1 Pro (High)"),
		row("gemini-pro-agent", "Gemini 3.1 Pro (High)"),
		row("gemini-3.6-flash-medium", "Gemini 3.6 Flash (Medium)"),
		row("gemini-3.6-flash-high", "Gemini 3.6 Flash (High)"),
		row("gemini-3.8-flash-tiered", ""), // newest: no display name
		// Four ids, one label. This is the row the owner saw four times.
		row("gemini-2.5-flash", "Gemini 3.5 Flash Lite"),
		row("gemini-2.5-flash-lite", "Gemini 3.5 Flash Lite"),
		row("gemini-2.5-flash-thinking", "Gemini 3.5 Flash Lite"),
		row("gemini-3.5-flash-lite", "Gemini 3.5 Flash Lite"),
	}
}

// snapshotAntigravity saves and restores the registry entries and the legacy
// map these tests replace; both are package globals shared with every test.
func snapshotAntigravity() func() {
	saved := map[ModelID]Model{}
	for id, m := range SupportedModels {
		if m.Provider == ProviderAntigravity {
			saved[id] = m
		}
	}
	legacy := map[ModelID]ModelID{}
	for k, v := range LegacyModelIDs {
		legacy[k] = v
	}
	return func() {
		for id, m := range SupportedModels {
			if m.Provider == ProviderAntigravity {
				delete(SupportedModels, id)
			}
		}
		for id, m := range saved {
			SupportedModels[id] = m
		}
		for k := range LegacyModelIDs {
			delete(LegacyModelIDs, k)
		}
		for k, v := range legacy {
			LegacyModelIDs[k] = v
		}
	}
}

// Every row in the picker must be tellable from every other row.
func TestNoTwoAntigravityModelsShareAName(t *testing.T) {
	built := BuildAntigravityModels(rowsOf20261005())
	seen := map[string]ModelID{}
	for id, m := range built {
		if other, dup := seen[m.Name]; dup {
			t.Errorf("%s and %s are both shown as %q: the user cannot choose between rows that read the same", id, other, m.Name)
		}
		seen[m.Name] = id
	}
	// The label is still the backend's; the id is added, not substituted.
	got := built["antigravity.gemini-2.5-flash"].Name
	if got != "Gemini 3.5 Flash Lite [gemini-2.5-flash] (Antigravity free)" {
		t.Errorf("shared label rendered as %q", got)
	}
	// A label nobody shares is left exactly as the backend wrote it.
	if got := built["antigravity.claude-opus-5-5-high"].Name; got != "Claude Opus 5.5 (High) (Antigravity free)" {
		t.Errorf("unique label was altered: %q", got)
	}
}

// Rank 1 is the best model and every fetched model has a rank. Before this,
// fetched models had none and sorted below two retired built-ins.
func TestFetchedAntigravityModelsAreRankedBestFirst(t *testing.T) {
	built := BuildAntigravityModels(rowsOf20261005())
	order := make([]Model, 0, len(built))
	for _, m := range built {
		if m.Rank <= 0 {
			t.Errorf("%s has no rank, so it sorts below everything that has one", m.ID)
		}
		order = append(order, m)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].Rank < order[j].Rank })
	for i, m := range order {
		if m.Rank != i+1 {
			t.Fatalf("ranks are not 1..N without gaps or ties: position %d holds rank %d", i, m.Rank)
		}
	}
	pos := func(api string) int {
		for i, m := range order {
			if m.APIModel == api {
				return i
			}
		}
		t.Fatalf("%s missing", api)
		return -1
	}
	for _, c := range [][2]string{
		{"claude-opus-5-5-high", "claude-opus-5-5-medium"},     // effort: high before medium
		{"claude-opus-5-5-medium", "claude-opus-5-5-low"},      // ... before low
		{"claude-opus-5-5-low", "claude-sonnet-5-5-high"},      // family: Opus before Sonnet
		{"claude-sonnet-5-5-low", "gemini-3.1-pro-high"},       // Claude before Gemini Pro
		{"gemini-3.1-pro-high", "gemini-2.5-pro"},              // version: newer first
		{"gemini-2.5-pro", "gemini-3.8-flash-tiered"},          // Pro before Flash
		{"gemini-3.8-flash-tiered", "gemini-3.6-flash-high"},   // 3.8 before 3.6, name derived from the id
		{"gemini-3.6-flash-medium", "gpt-oss-120b-medium"},     // Flash before the rest
		{"gpt-oss-120b-medium", "gemini-3.5-flash-lite"},       // Lite last
		{"gemini-3.6-flash-high", "gemini-2.5-flash-thinking"}, // an id saying "flash" labelled Lite is Lite
	} {
		if pos(c[0]) >= pos(c[1]) {
			t.Errorf("%s (position %d) should come before %s (position %d)", c[0], pos(c[0]), c[1], pos(c[1]))
		}
	}
}

// A refresh REPLACES. The two models Google stopped listing must leave the
// picker, and anything still pointing at them must be given a successor of the
// same family rather than be dropped onto an unrelated default.
func TestRefreshRemovesWhatGoogleStoppedListingAndNamesASuccessor(t *testing.T) {
	defer snapshotAntigravity()()
	for id, m := range AntigravityModels { // the built-in, first-run state
		SupportedModels[id] = m
	}

	res, err := RefreshAntigravity(t.TempDir(), rowsOf20261005())
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []ModelID{AGClaudeSonnet46, AGClaudeOpus46} {
		if _, still := SupportedModels[gone]; still {
			t.Errorf("%s is still offered after a refresh that did not list it", gone)
		}
	}
	if !strings.Contains(strings.Join(res.Removed, ","), "claude-sonnet-4-6") {
		t.Errorf("the refresh did not report claude-sonnet-4-6 as removed: %v", res.Removed)
	}
	if got := LegacyModelIDs[AGClaudeSonnet46]; got != "antigravity.claude-sonnet-5-5-high" {
		t.Errorf("successor of Sonnet 4.6 = %q, want the best current Sonnet", got)
	}
	if got := LegacyModelIDs[AGClaudeOpus46]; got != "antigravity.claude-opus-5-5-high" {
		t.Errorf("successor of Opus 4.6 = %q, want the best current Opus", got)
	}
	// A model that is still listed must never be migrated away from.
	if to, mapped := LegacyModelIDs[AGGemini36Flash]; mapped {
		t.Errorf("%s is still offered but is mapped to %s", AGGemini36Flash, to)
	}

	// Second refresh, same answer: nothing added, nothing removed. The old code
	// compared with the built-in map and reported the same additions for ever.
	res, err = RefreshAntigravity(t.TempDir(), rowsOf20261005())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 0 || len(res.Removed) != 0 {
		t.Errorf("an identical refresh reported +%d -%d", len(res.Added), len(res.Removed))
	}
}

// The sign-in default is read from the list, not typed. The typed one was
// claude-sonnet-4-6, which the list no longer holds.
func TestPreferredAntigravityModelsComeFromTheFetchedList(t *testing.T) {
	defer snapshotAntigravity()()
	applyAntigravity(BuildAntigravityModels(rowsOf20261005()))

	coder, background := PreferredAntigravityModels()
	if coder != "antigravity.claude-sonnet-5-5-high" {
		t.Errorf("coder = %s, want the best Claude Sonnet on offer", coder)
	}
	if background != "antigravity.gemini-3.6-flash-medium" {
		t.Errorf("background = %s, want a mid-effort Gemini Flash (the separate weekly allowance)", background)
	}
	for _, id := range []ModelID{coder, background} {
		if _, ok := SupportedModels[id]; !ok {
			t.Errorf("%s was chosen and is not registered", id)
		}
	}

	// No Sonnet and no Flash on offer: still a real model, never "".
	applyAntigravity(BuildAntigravityModels([]AntigravityRow{
		{ID: "gpt-oss-120b-medium", DisplayName: "GPT-OSS 120B (Medium)", MaxTokens: 131072, MaxOutputTokens: 32768},
	}))
	coder, background = PreferredAntigravityModels()
	if coder != "antigravity.gpt-oss-120b-medium" || background != coder {
		t.Errorf("single model gave (%s, %s)", coder, background)
	}
}

// A cache written by an older build has no ranks and repeats names. It must be
// right as soon as it is read: the owner's cache was in this state.
func TestAnOldAntigravityCacheIsRepairedWhenRead(t *testing.T) {
	defer snapshotAntigravity()()
	dir := t.TempDir()
	old := map[ModelID]Model{}
	for _, api := range []string{"gemini-2.5-flash", "gemini-3.5-flash-lite"} {
		id := ModelID("antigravity." + api)
		old[id] = Model{ID: id, Name: "Gemini 3.5 Flash Lite (Antigravity free)", Provider: ProviderAntigravity, APIModel: api}
	}
	id := ModelID("antigravity.claude-opus-5-5-high")
	old[id] = Model{ID: id, Name: "Claude Opus 5.5 (High) (Antigravity free)", Provider: ProviderAntigravity, APIModel: "claude-opus-5-5-high"}
	blob, _ := json.Marshal(cachedAntigravity{Schema: antigravityCacheSchema, Refreshed: time.Now(), Models: old})
	if err := os.WriteFile(antigravityCachePath(dir), blob, 0o644); err != nil {
		t.Fatal(err)
	}

	if n, err := LoadRefreshedAntigravity(dir); err != nil || n != 3 {
		t.Fatalf("load: n=%d err=%v", n, err)
	}
	if SupportedModels[id].Rank != 1 {
		t.Errorf("Opus 5.5 rank = %d, want 1", SupportedModels[id].Rank)
	}
	a, b := SupportedModels["antigravity.gemini-2.5-flash"].Name, SupportedModels["antigravity.gemini-3.5-flash-lite"].Name
	if a == b {
		t.Errorf("two cached models still share the name %q", a)
	}
	// Reading it twice must not stack a second "[id]" on the name.
	if _, err := LoadRefreshedAntigravity(dir); err != nil {
		t.Fatal(err)
	}
	if got := SupportedModels["antigravity.gemini-2.5-flash"].Name; got != a {
		t.Errorf("name changed on a second read: %q then %q", a, got)
	}
}

// A ChatGPT cache from before the convention changed holds ranks that count
// DOWN from 9. Read as they stand they would put the coder on the worst model.
func TestAnOldChatGPTCacheHasItsRanksTurnedTheRightWayUp(t *testing.T) {
	restore := snapshotChatGPT()
	defer restore()
	dir := t.TempDir()
	old := map[ModelID]Model{
		ChatGPT56Terra: {ID: ChatGPT56Terra, Provider: ProviderChatGPT, Rank: 9},
		ChatGPT56Luna:  {ID: ChatGPT56Luna, Provider: ProviderChatGPT, Rank: 8},
		ChatGPT55:      {ID: ChatGPT55, Provider: ProviderChatGPT, Rank: 7},
	}
	blob, _ := json.Marshal(cachedChatGPT{Schema: 1, Refreshed: time.Now(), Models: old})
	if err := os.WriteFile(chatgptCachePath(dir), blob, 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := LoadRefreshedChatGPT(dir); err != nil || n != 3 {
		t.Fatalf("load: n=%d err=%v", n, err)
	}
	for id, want := range map[ModelID]int{ChatGPT56Terra: 1, ChatGPT56Luna: 2, ChatGPT55: 3} {
		if got := SupportedModels[id].Rank; got != want {
			t.Errorf("%s rank = %d, want %d", id, got, want)
		}
	}
	if best, cheap := PreferredChatGPTModels(); best != ChatGPT56Terra || cheap != ChatGPT55 {
		t.Errorf("preferred = (%s, %s), want (terra, 5.5)", best, cheap)
	}
}

// One convention for the whole program: where a sign-in provider ranks its
// models, the best one is rank 1. Two providers shipped the reverse and each
// passed its own tests.
func TestRankOneIsTheBestModelForEverySignInProvider(t *testing.T) {
	for name, set := range map[string]map[ModelID]Model{"ChatGPT built-in": ChatGPTModels, "Antigravity built-in": AntigravityModels} {
		lowest := 0
		for _, m := range set {
			if m.Rank > 0 && (lowest == 0 || m.Rank < lowest) {
				lowest = m.Rank
			}
		}
		if lowest != 1 {
			t.Errorf("%s: the best rank is %d, want 1. The picker prints \"1=best\" and sorts ascending", name, lowest)
		}
	}
}
