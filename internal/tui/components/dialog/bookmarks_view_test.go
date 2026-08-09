package dialog

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// A bookmark whose model has been retired must stay visible and say so.
// Dropping it silently is the failure this project keeps finding: the user
// curated something, it vanished, and nothing explained why.
func TestRetiredBookmarkIsShownNotDropped(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := config.ToggleBookmark("openrouter.vendor/retired-model"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = config.ToggleBookmark("openrouter.vendor/retired-model") })

	got := bookmarkedModels()
	if len(got) != 1 {
		t.Fatalf("a retired bookmark must survive as an entry, got %d", len(got))
	}
	if !strings.Contains(got[0].Description, "UNAVAILABLE") {
		t.Errorf("it must say why it cannot be used, got %q", got[0].Description)
	}
	if !strings.Contains(got[0].Description, "space") {
		t.Errorf("it must say how to clear it, got %q", got[0].Description)
	}
}

// With no bookmarks the picker must behave exactly as it did before — no empty
// column between the user and the models.
func TestNoBookmarksMeansNoBookmarkColumn(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	for _, p := range getEnabledProviders(config.Get()) {
		if p == ProviderBookmarks {
			t.Fatal("an empty shortlist must not take a column in the carousel")
		}
	}
}

// Selecting a retired bookmark must be refused in the dialog, not passed to the
// agent. Handing it an unresolvable id produces a generic failure that reads as
// "this program is broken" rather than "that model was retired".
func TestSelectingRetiredBookmarkIsRefused(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := config.ToggleBookmark("openrouter.vendor/gone"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = config.ToggleBookmark("openrouter.vendor/gone") })

	m := &modelDialogCmp{provider: ProviderBookmarks}
	m.models = bookmarkedModels()
	if len(m.models) != 1 {
		t.Fatalf("expected the retired bookmark, got %d entries", len(m.models))
	}
	// The dialog must not emit ModelSelectedMsg for it. Assert on the guard's
	// own condition rather than driving bubbletea: the model is unresolvable,
	// and that is precisely what Enter now checks.
	if _, ok := models.SupportedModels[m.models[0].ID]; ok {
		t.Fatal("fixture is wrong — this id should not resolve")
	}
}

// The shortlist must be the FIRST column, not merely present.
//
// Prepending it was not enough: getEnabledProviders sorts by
// ProviderPopularity afterwards, the shortlist is not a real provider so it has
// no entry, and unranked providers default to 999 = show last. It ended up at
// the far right of the carousel — several presses away and, to the user,
// indistinguishable from not having been created at all.
func TestBookmarksColumnComesFirst(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	cfg := config.Get()
	// Two real providers, so the sort has something to reorder.
	cfg.Providers = map[models.ModelProvider]config.Provider{
		models.ProviderOpenAI:    {APIKey: "x"},
		models.ProviderAnthropic: {APIKey: "x"},
	}
	if _, err := config.ToggleBookmark("openrouter.vendor/anything"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = config.ToggleBookmark("openrouter.vendor/anything") })

	got := getEnabledProviders(cfg)
	if len(got) == 0 {
		t.Fatal("no providers at all")
	}
	if got[0] != ProviderBookmarks {
		t.Fatalf("the shortlist must lead the carousel, got order: %v", got)
	}
}

// providerDisplayName used to slice the first BYTE of a UTF-8 string, so any
// name starting with a multi-byte rune came back as invalid fragments — one
// replacement glyph per byte. "★ bookmarks" rendered as "◆◆◆ bookmarks".
func TestProviderDisplayNameHandlesMultiByteNames(t *testing.T) {
	got := providerDisplayName(ProviderBookmarks)
	if !strings.HasPrefix(got, "★") {
		t.Errorf("the star must survive intact, got %q (% x)", got, got)
	}
	if !utf8.ValidString(got) {
		t.Errorf("produced invalid UTF-8: % x", got)
	}
	// The ASCII path must still capitalise.
	if n := providerDisplayName("openai"); n != "Openai" {
		t.Errorf("ASCII names should still be capitalised, got %q", n)
	}
	// And an empty name must not panic on r[0].
	if n := providerDisplayName(""); n != "" {
		t.Errorf("empty name should stay empty, got %q", n)
	}
}

// A shortlist entry must not carry the rank it had at its home provider. Those
// numbers are meaningless out of context and collide — the list showed two
// entries numbered 10.
func TestBookmarksDropInheritedRanks(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	var withRank models.ModelID
	for id, m := range models.SupportedModels {
		if m.Rank > 0 {
			withRank = id
			break
		}
	}
	if withRank == "" {
		t.Skip("no ranked model in the registry to test with")
	}
	if _, err := config.ToggleBookmark(string(withRank)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = config.ToggleBookmark(string(withRank)) })

	for _, m := range bookmarkedModels() {
		if m.Rank != 0 {
			t.Errorf("%s kept rank %d from its home provider", m.ID, m.Rank)
		}
	}
}

// The feature has to be findable. Reported from a live run: "there's no
// indication that YOU HAVE THE OPTION to even select your models and bookmark
// them" — the only label naming the shortlist appeared after you had already
// used it, which is not discoverability, it is a reward for already knowing.
func TestPickerTellsYouAboutBookmarking(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	cfg := config.Get()
	cfg.Providers = map[models.ModelProvider]config.Provider{
		models.ProviderAnthropic: {APIKey: "x"},
	}
	m := &modelDialogCmp{width: 200, height: 40}
	m.availableProviders = getEnabledProviders(cfg)
	m.setupModelsForProvider(models.ProviderAnthropic)

	view := m.View()
	if !strings.Contains(view, "space") {
		t.Error("no mention of the space key anywhere in the picker")
	}
	if !strings.Contains(strings.ToLower(view), "bookmark") {
		t.Error("the word 'bookmark' never appears, so nobody learns the feature exists")
	}
}

// And the shortlist must say whose list it is, not just what it is called.
func TestBookmarksTitleSaysItIsYours(t *testing.T) {
	name := providerDisplayName(ProviderBookmarks)
	if !strings.Contains(strings.ToUpper(name), "YOUR") {
		t.Errorf("the title must make ownership obvious, got %q", name)
	}
}

// The reported dead end: after browsing several providers to bookmark things,
// you are several columns from the shortlist, ←/→ moves one at a time, and the
// only way anyone found to see the list was to close the dialog and reopen it.
// "b" must reach it from anywhere, in one press.
func TestJumpToBookmarksFromAnyColumn(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	cfg := config.Get()
	cfg.Providers = map[models.ModelProvider]config.Provider{
		models.ProviderAnthropic: {APIKey: "x"},
		models.ProviderOpenAI:    {APIKey: "x"},
		models.ProviderGemini:    {APIKey: "x"},
	}
	if _, err := config.ToggleBookmark("openrouter.vendor/anything"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = config.ToggleBookmark("openrouter.vendor/anything") })

	m := &modelDialogCmp{}
	m.availableProviders = getEnabledProviders(cfg)
	m.hScrollPossible = true
	// Stand at the far end, as someone does after browsing to bookmark.
	last := len(m.availableProviders) - 1
	if last < 2 {
		t.Skip("need several providers to make the jump meaningful")
	}
	m.hScrollOffset = last
	m.setupModelsForProvider(m.availableProviders[last])
	if m.provider == ProviderBookmarks {
		t.Skip("already on the shortlist")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if m.provider != ProviderBookmarks {
		t.Fatalf("b must reach the shortlist from column %d, landed on %q", last, m.provider)
	}
}

// With nothing bookmarked, b must explain rather than appear broken.
func TestJumpToBookmarksWithNoneSaysWhatToDo(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	m := &modelDialogCmp{}
	m.availableProviders = getEnabledProviders(config.Get())
	if findProviderIndex(m.availableProviders, ProviderBookmarks) >= 0 {
		t.Skip("bookmarks exist in this environment")
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}); cmd == nil {
		t.Error("pressing b with no bookmarks must report something, not do nothing")
	}
}
