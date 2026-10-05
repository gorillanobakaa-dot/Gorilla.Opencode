package models

import (
	"strings"
	"testing"
)

// The shape Google returned on 2026-10-05, cut down to the rows that decide
// behaviour.
const geminiListing = `{"models":[
 {"name":"models/gemini-flash-latest","displayName":"Gemini Flash Latest","inputTokenLimit":1048576,"outputTokenLimit":65536,"supportedGenerationMethods":["generateContent","countTokens"],"thinking":true},
 {"name":"models/gemini-3.8-flash","displayName":"Gemini 3.8 Flash","description":"Fast and versatile. Second sentence that must not be shown.","inputTokenLimit":1048576,"outputTokenLimit":65536,"supportedGenerationMethods":["generateContent"],"thinking":true},
 {"name":"models/gemini-3.8-flash-tts","displayName":"TTS","supportedGenerationMethods":["generateContent"]},
 {"name":"models/gemini-3.1-flash-image","displayName":"Image","supportedGenerationMethods":["generateContent"]},
 {"name":"models/gemini-embedding-001","displayName":"Embedding","supportedGenerationMethods":["embedContent"]},
 {"name":"models/gemma-4-27b-it","displayName":"Gemma","supportedGenerationMethods":["generateContent"]}
]}`

func snapshotGemini() func() {
	saved := map[ModelID]Model{}
	for id, m := range SupportedModels {
		if m.Provider == ProviderGemini {
			saved[id] = m
		}
	}
	legacy := map[ModelID]ModelID{}
	for k, v := range LegacyModelIDs {
		legacy[k] = v
	}
	return func() {
		s, _, commit := beginRegistryEdit()
		for id, m := range s {
			if m.Provider == ProviderGemini {
				delete(s, id)
			}
		}
		for id, m := range saved {
			s[id] = m
		}
		commit()
		for k := range LegacyModelIDs {
			delete(LegacyModelIDs, k)
		}
		for k, v := range legacy {
			LegacyModelIDs[k] = v
		}
	}
}

func TestTheGeminiListComesFromGoogleNotFromThisProgram(t *testing.T) {
	defer snapshotGemini()()
	typedBefore := SupportedModels[GeminiFlashLatest]
	if typedBefore.ID == "" {
		t.Fatal("the built-in rolling alias is missing")
	}
	dir := t.TempDir()
	res, err := RefreshGemini(dir, []byte(geminiListing))
	if err != nil {
		t.Fatal(err)
	}
	if res.Fetched != 6 || res.Usable != 2 {
		t.Fatalf("fetched %d usable %d, want 6 and 2 (speech, image, embedding and non-Gemini rows are not chat models)", res.Fetched, res.Usable)
	}

	// A model nobody typed is offered the day Google lists it.
	m, ok := SupportedModels["gemini-3.8-flash"]
	if !ok {
		t.Fatal("gemini-3.8-flash is listed by Google and not offered here")
	}
	if m.Provider != ProviderGemini || m.APIModel != "gemini-3.8-flash" || m.ContextWindow != 1048576 || !m.CanReason {
		t.Errorf("fetched entry is wrong: %+v", m)
	}
	if m.Description != "Fast and versatile." {
		t.Errorf("description = %q, want the first sentence only", m.Description)
	}
	if m.DefaultMaxTokens <= 0 || m.DefaultMaxTokens > m.ContextWindow/2 {
		t.Errorf("per-request cap %d is not a sane fraction of the window", m.DefaultMaxTokens)
	}

	// A typed entry Google still lists keeps its curated text and price, and
	// takes its limits from the wire.
	kept := SupportedModels[GeminiFlashLatest]
	if kept.Name != typedBefore.Name || kept.CostPer1MIn != typedBefore.CostPer1MIn {
		t.Errorf("the curated entry was replaced: %+v", kept)
	}
	if kept.ContextWindow != 1048576 {
		t.Errorf("the curated entry kept a typed context window: %d", kept.ContextWindow)
	}

	// A typed entry Google no longer lists leaves, and is mapped forward.
	for _, id := range res.Removed {
		if _, still := SupportedModels[ModelID(id)]; still {
			t.Errorf("%s was reported removed and is still offered", id)
		}
		if LegacyModelIDs[ModelID(id)] != GeminiFlashLatest {
			t.Errorf("%s was removed with no successor", id)
		}
	}
	if len(res.Removed) == 0 {
		t.Error("the fixture lists two models; every other typed Gemini entry should have been removed")
	}
	for id, m := range SupportedModels {
		if m.Provider == ProviderGemini && (strings.Contains(string(id), "tts") || strings.Contains(string(id), "image") || strings.Contains(string(id), "embedding")) {
			t.Errorf("%s is not a chat model and is offered", id)
		}
	}

	// Survives a restart, from disk alone.
	snapshotGemini()() // put the typed list back, as a fresh process would have it
	if n := LoadRefreshedGemini(dir); n != 2 {
		t.Fatalf("loaded %d from the cache, want 2", n)
	}
	if _, ok := SupportedModels["gemini-3.8-flash"]; !ok {
		t.Error("the fetched model is gone after a restart")
	}
	if _, ok := GeminiCatalogueAge(dir); !ok {
		t.Error("the list has no date")
	}
}

func TestAnEmptyOrBrokenGeminiListChangesNothing(t *testing.T) {
	defer snapshotGemini()()
	before := len(SupportedModels)
	for _, raw := range []string{`{"models":[]}`, `not json`, `{"models":[{"name":"models/gemini-x-tts","supportedGenerationMethods":["generateContent"]}]}`} {
		if _, err := RefreshGemini(t.TempDir(), []byte(raw)); err == nil {
			t.Errorf("%q was accepted as a model list", raw)
		}
	}
	if len(SupportedModels) != before {
		t.Error("a failed refresh changed the registry")
	}
}
