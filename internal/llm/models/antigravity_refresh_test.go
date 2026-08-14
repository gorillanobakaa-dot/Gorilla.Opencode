package models

import (
	"os"
	"path/filepath"
	"testing"
)

// The live response shape, measured 2026-08-14 against daily-cloudcode-pa.
// Trimmed to the rows that decide behaviour.
func liveRows() []AntigravityRow {
	return []AntigravityRow{
		// The newest model has NO displayName. Filtering on displayName is the
		// obvious rule and would drop exactly this row.
		{ID: "gemini-3.7-flash-tiered", MaxTokens: 1048576, MaxOutputTokens: 65536, SupportsThinking: true, SupportsImages: true, APIProvider: "API_PROVIDER_GOOGLE_GEMINI"},
		{ID: "gemini-3.6-flash-high", DisplayName: "Gemini 3.6 Flash (High)", MaxTokens: 1048576, MaxOutputTokens: 65536, SupportsThinking: true, SupportsImages: true, APIProvider: "API_PROVIDER_GOOGLE_GEMINI"},
		{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6 (Thinking)", MaxTokens: 250000, MaxOutputTokens: 64000, SupportsThinking: true, SupportsImages: true, APIProvider: "API_PROVIDER_ANTHROPIC_VERTEX"},
		// Internal scaffolding.
		{ID: "chat_20706", MaxTokens: 16384, IsInternal: true, APIProvider: "API_PROVIDER_INTERNAL"},
		// Editor tab-completion, not a chat model.
		{ID: "tab_flash_lite_preview", MaxTokens: 16384, MaxOutputTokens: 4096, APIProvider: "API_PROVIDER_GOOGLE_GEMINI"},
		// Image generation: no output token budget.
		{ID: "gemini-3.1-flash-image", MaxTokens: 32768, APIProvider: "API_PROVIDER_GOOGLE_GEMINI"},
	}
}

func TestBuildKeepsTheNewestModelEvenWithoutADisplayName(t *testing.T) {
	built := BuildAntigravityModels(liveRows())

	// The bug this guards: gemini-3.7-flash-tiered is the ONLY id the backend
	// accepts for Gemini 3.7 (gemini-3.7-flash-high/medium/low all return
	// NOT_FOUND), and it carries no displayName. A displayName filter silently
	// removes the one model the user came for.
	got, ok := built["antigravity.gemini-3.7-flash-tiered"]
	if !ok {
		t.Fatal("gemini-3.7-flash-tiered was dropped — a displayName filter has crept back in")
	}
	if got.APIModel != "gemini-3.7-flash-tiered" {
		t.Errorf("APIModel = %q; the backend only honours the id it returned", got.APIModel)
	}
	if got.Name == "" || got.Name == " (Antigravity free)" {
		t.Errorf("derived name is empty: %q", got.Name)
	}
	if !got.CanReason || !got.SupportsAttachments {
		t.Errorf("capabilities lost: reason=%v attach=%v", got.CanReason, got.SupportsAttachments)
	}
	if got.ContextWindow != 1048576 || got.DefaultMaxTokens != 65536 {
		t.Errorf("token limits wrong: ctx=%d out=%d", got.ContextWindow, got.DefaultMaxTokens)
	}
}

func TestBuildDropsNonChatRows(t *testing.T) {
	built := BuildAntigravityModels(liveRows())
	for _, id := range []ModelID{
		"antigravity.chat_20706",
		"antigravity.tab_flash_lite_preview",
		"antigravity.gemini-3.1-flash-image",
	} {
		if _, ok := built[id]; ok {
			t.Errorf("%s should not be offered in a chat picker", id)
		}
	}
	if len(built) != 3 {
		t.Errorf("expected 3 usable of 6 rows, got %d", len(built))
	}
}

// Non-vacuous: assert the filter would FAIL if it required a displayName.
func TestDisplayNameFilterWouldBeWrong(t *testing.T) {
	var kept int
	for _, r := range liveRows() {
		if r.usable() && r.DisplayName != "" {
			kept++
		}
	}
	if kept != 2 {
		t.Fatalf("guard test drifted: expected 2, got %d", kept)
	}
	// 3 usable vs 2 with a displayName — the difference is Gemini 3.7.
	if got := len(BuildAntigravityModels(liveRows())); got == kept {
		t.Fatal("filter no longer distinguishes; the 3.7 regression would pass unnoticed")
	}
}

func TestRefreshRoundTripsThroughTheCache(t *testing.T) {
	dir := t.TempDir()
	if _, err := RefreshAntigravity(dir, liveRows()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "antigravity-models.json")); err != nil {
		t.Fatalf("cache not written: %v", err)
	}

	// Simulate a cold start: drop the model from the registries, then load.
	const id ModelID = "antigravity.gemini-3.7-flash-tiered"
	delete(AntigravityModels, id)
	delete(SupportedModels, id)

	n, err := LoadRefreshedAntigravity(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if n == 0 {
		t.Fatal("cache loaded nothing")
	}
	if _, ok := SupportedModels[id]; !ok {
		t.Error("cold start did not register the model — /models would not show it")
	}
}

func TestCorruptCacheLeavesTheBuiltInListWorking(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "antigravity-models.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRefreshedAntigravity(dir); err == nil {
		t.Error("a corrupt cache should be reported, not swallowed")
	}
	// The built-in catalogue must survive it.
	if len(AntigravityModels) == 0 {
		t.Fatal("built-in Antigravity list was lost")
	}
}

func TestMissingCacheIsNotAnError(t *testing.T) {
	n, err := LoadRefreshedAntigravity(t.TempDir())
	if err != nil || n != 0 {
		t.Errorf("absent cache should be silent: n=%d err=%v", n, err)
	}
}
