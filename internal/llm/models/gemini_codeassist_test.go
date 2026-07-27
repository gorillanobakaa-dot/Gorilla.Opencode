package models

import (
	"strings"
	"testing"
)

// Code Assist (cloudcode-pa.googleapis.com) 404s on rolling aliases. The Gemini
// ids are misleading about this — Gemini25Flash's id is "gemini-2.5-flash" but
// it serves "gemini-flash-latest" — so guard on APIModel, which is what is
// actually put on the wire.
func TestGeminiCAExposesNoRollingAliases(t *testing.T) {
	for id, m := range GeminiCAModels {
		if strings.HasSuffix(m.APIModel, "-latest") {
			t.Errorf("%s exposes rolling alias %q; Code Assist returns 404 for these", id, m.APIModel)
		}
	}
}

// The two constants setDefaultModelForAgent falls back to must exist on the
// Code Assist provider. When they did not, an agent whose real provider was
// unavailable was silently rewritten onto a model that 404'd on first use.
func TestGeminiCAFallbackModelsAreServed(t *testing.T) {
	for _, id := range []ModelID{GeminiCAPro, GeminiCAFlash, GeminiCA3Flash, GeminiCA31FlashLite} {
		m, ok := GeminiCAModels[id]
		if !ok {
			t.Errorf("fallback model %q is not a registered Code Assist model", id)
			continue
		}
		if m.Provider != ProviderGeminiCA {
			t.Errorf("fallback model %q has provider %q, want %q", id, m.Provider, ProviderGeminiCA)
		}
	}
}

// Families Code Assist has not been given. Probed live 2026-07-27: each returns
// HTTP 404 "Requested entity was not found".
func TestGeminiCAExcludesUnservedFamilies(t *testing.T) {
	for _, id := range []ModelID{
		Gemini36Flash, Gemini35Flash, Gemini35FlashLite,
		Gemini20Flash, Gemini20FlashLite,
	} {
		caID := ModelID("gemini-oauth." + string(id))
		if _, ok := GeminiCAModels[caID]; ok {
			t.Errorf("%s is not served by Code Assist but is exposed", caID)
		}
	}
}

// Every Code Assist model must also be reachable through the global registry,
// since that is what agent creation and the model picker resolve against.
func TestGeminiCAModelsRegistered(t *testing.T) {
	for id := range GeminiCAModels {
		if _, ok := SupportedModels[id]; !ok {
			t.Errorf("%s missing from SupportedModels", id)
		}
	}
}
