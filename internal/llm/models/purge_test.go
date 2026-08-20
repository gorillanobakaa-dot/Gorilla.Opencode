package models

import (
	"os"
	"path/filepath"
	"testing"
)

// The three things a purge must NOT destroy. Each was a deliberate decision and
// each would be a silent data loss if it regressed.
func TestPurgeSparesTheModelsThatWork(t *testing.T) {
	dir := t.TempDir()

	// Snapshot a compiled-in model and a fetched one.
	saved := make(map[ModelID]Model, len(SupportedModels))
	for k, v := range SupportedModels {
		saved[k] = v
	}
	t.Cleanup(func() {
		SupportedModels = saved
	})

	SupportedModels["test/fetched-openrouter"] = Model{ID: "test/fetched-openrouter", Provider: ProviderOpenRouter}
	SupportedModels["test/fetched-antigravity"] = Model{ID: "test/fetched-antigravity", Provider: ProviderAntigravity}
	SupportedModels["test/compiled-anthropic"] = Model{ID: "test/compiled-anthropic", Provider: ProviderAnthropic}

	for _, name := range []string{cacheFileName, "antigravity-models.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	res := PurgeFetchedCatalogues(dir)

	if _, still := SupportedModels["test/fetched-openrouter"]; still {
		t.Error("a fetched OpenRouter model survived the purge")
	}
	if _, still := SupportedModels["test/fetched-antigravity"]; still {
		t.Error("a fetched Antigravity model survived the purge")
	}
	// The point of the whole design: compiled-in providers are the ones that
	// work, several free with a Gmail sign-in. Deleting them would throw away
	// access someone completed an OAuth flow for, AND would silently undo
	// itself on the next launch because they ship with the binary.
	if _, ok := SupportedModels["test/compiled-anthropic"]; !ok {
		t.Error("a compiled-in model was purged; those must survive")
	}
	if res.RemovedModels < 2 {
		t.Errorf("RemovedModels=%d, expected at least the two fetched ones", res.RemovedModels)
	}
	if len(res.FilesDeleted) != 2 {
		t.Errorf("FilesDeleted=%v, expected both cache files", res.FilesDeleted)
	}
	for _, name := range res.FilesDeleted {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s was reported deleted but is still there", name)
		}
	}
}

// Purging with nothing to purge must be a no-op that says so, not an error.
func TestPurgeOnACleanSlateIsHarmless(t *testing.T) {
	res := PurgeFetchedCatalogues(t.TempDir())
	if len(res.FilesDeleted) != 0 {
		t.Errorf("deleted %v from an empty dir", res.FilesDeleted)
	}
}
