package dialog

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
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
