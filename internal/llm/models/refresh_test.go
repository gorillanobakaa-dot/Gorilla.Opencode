package models

import (
	"os"
	"path/filepath"
	"testing"
)

// GORILLA OVERRIDE: the safety property that matters most here is NOT that a
// refresh works — it is that a failed or corrupt one leaves a working program.
// Someone on a bad connection will interrupt this, and a half-written cache read
// at next launch must not leave them with an empty model picker and no way back.

func TestLoadRefreshedCatalogueMissingFileIsNotAnError(t *testing.T) {
	n, err := LoadRefreshedCatalogue(t.TempDir())
	if err != nil || n != 0 {
		t.Fatalf("a never-refreshed install is normal, got n=%d err=%v", n, err)
	}
}

func TestLoadRefreshedCatalogueSurvivesCorruption(t *testing.T) {
	for name, body := range map[string]string{
		"truncated json": `{"models": {"openrouter.x": {"id":"openrouter.x"`,
		"not json":       "openrouter was here",
		"empty object":   `{}`,
		"empty models":   `{"models":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, cacheFileName), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			before := len(SupportedModels)
			n, err := LoadRefreshedCatalogue(dir)
			if err == nil {
				t.Errorf("corrupt cache must be reported, not swallowed")
			}
			if n != 0 {
				t.Errorf("nothing should be merged from a corrupt cache, got %d", n)
			}
			if len(SupportedModels) != before {
				t.Errorf("the built-in list was damaged: %d -> %d", before, len(SupportedModels))
			}
		})
	}
}

// Entries that could not possibly work must not be merged even from a
// well-formed cache — an empty APIModel would send a request with no model.
func TestLoadRefreshedCatalogueRejectsUnusableEntries(t *testing.T) {
	dir := t.TempDir()
	body := `{"models":{
	  "openrouter.good": {"id":"openrouter.good","provider":"openrouter","api_model":"vendor/good","context_window":1000,"default_max_tokens":100},
	  "openrouter.nomodel": {"id":"openrouter.nomodel","provider":"openrouter","api_model":"","context_window":1000},
	  "openrouter.wrongprovider": {"id":"openrouter.wrongprovider","provider":"openai","api_model":"x","context_window":1000}
	}}`
	if err := os.WriteFile(filepath.Join(dir, cacheFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := LoadRefreshedCatalogue(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("only the usable entry should merge, got %d", n)
	}
	if _, ok := SupportedModels["openrouter.nomodel"]; ok {
		t.Error("an entry with no APIModel was registered — requests would have no model")
	}
	delete(SupportedModels, "openrouter.good")
}

func TestCatalogueAgeReportsNeverRefreshed(t *testing.T) {
	if _, ok := CatalogueAge(t.TempDir()); ok {
		t.Error("a directory with no cache must report never-refreshed")
	}
}
