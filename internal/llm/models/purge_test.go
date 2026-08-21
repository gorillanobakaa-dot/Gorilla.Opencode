package models

import (
	"os"
	"path/filepath"
	"testing"
)

// The things a purge must NOT destroy. Each was a deliberate decision and
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
	// GORILLA OVERRIDE (2026-08-21): Gemini, not Anthropic. Anthropic's list is
	// fetched now, so an Anthropic model is a DOWNLOADED model and purging it is
	// correct. Gemini is one of the two provider lists still compiled in, which
	// is what this assertion is actually about.
	SupportedModels["test/compiled-gemini"] = Model{ID: "test/compiled-gemini", Provider: ProviderGemini}

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
	if _, ok := SupportedModels["test/compiled-gemini"]; !ok {
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

// GORILLA OVERRIDE (2026-08-21): a fetched provider's models ARE downloaded
// models and must go. Before the catalogues were fetched this was the opposite
// assertion — the models were compiled in, so purging them was the bug (they
// silently came back on the next launch, making the purge count a lie). The
// direction reversed with the mechanism, which is exactly why it is pinned.
func TestPurgeRemovesFetchedProviderCatalogues(t *testing.T) {
	saved := make(map[ModelID]Model, len(SupportedModels))
	for k, v := range SupportedModels {
		saved[k] = v
	}
	t.Cleanup(func() { SupportedModels = saved })

	const groqModel ModelID = "groq.openai/gpt-oss-120b"
	SupportedModels[groqModel] = Model{ID: groqModel, Provider: ProviderGROQ}

	res := PurgeFetchedCatalogues(t.TempDir())

	if _, still := SupportedModels[groqModel]; still {
		t.Error("a fetched Groq model survived the purge")
	}
	if res.RemovedModels < 1 {
		t.Errorf("RemovedModels=%d, the fetched model was not counted", res.RemovedModels)
	}
}

// GORILLA FIX (2026-08-21): a purge must not claim to have removed models that
// ship in the binary and return on the next launch.
//
// Measured before the fix: /purge on a fresh registry reported 284 removed, of
// which 279 were compiled-in OpenRouter entries — true for one session, false
// after a restart, and nothing said so. Splitting the count is the whole fix;
// the models still go, because clearing the picker is what the command is for.
func TestPurgeSaysHowManyModelsComeBack(t *testing.T) {
	saved := make(map[ModelID]Model, len(SupportedModels))
	for k, v := range SupportedModels {
		saved[k] = v
	}
	t.Cleanup(func() { SupportedModels = saved })

	// One genuinely downloaded model and one that ships in the binary.
	SupportedModels["test/downloaded"] = Model{ID: "test/downloaded", Provider: ProviderOpenRouter}
	var shipped ModelID
	for id := range OpenRouterGeneratedModels {
		shipped = id
		break
	}
	if shipped == "" {
		t.Fatal("no compiled-in OpenRouter models to test with")
	}
	SupportedModels[shipped] = OpenRouterGeneratedModels[shipped]

	res := PurgeFetchedCatalogues(t.TempDir())

	if res.RemovedCompiled < 1 {
		t.Errorf("RemovedCompiled=%d — the report claims every cleared model is gone for good", res.RemovedCompiled)
	}
	if res.RemovedCompiled >= res.RemovedModels {
		t.Errorf("RemovedCompiled=%d of RemovedModels=%d — the downloaded one was miscounted as shipped",
			res.RemovedCompiled, res.RemovedModels)
	}
}
