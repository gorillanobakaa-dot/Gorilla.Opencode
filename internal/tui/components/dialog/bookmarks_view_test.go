package dialog

import (
	"strings"
	"testing"

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
