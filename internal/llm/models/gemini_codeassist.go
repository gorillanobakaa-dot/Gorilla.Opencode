// GORILLA OVERRIDE: this file did not exist upstream. It exposes the live
// Gemini models a second time under a distinct provider — "gemini-oauth" —
// reached through Google's Code Assist free tier via `gorilla-opencode
// login` (see internal/auth + internal/llm/provider/code_assist.go). Same
// models, different transport: no API key, cost 0 (free tier).
package models

import "maps"

const ProviderGeminiCA ModelProvider = "gemini-oauth"

// Handy ids for the two OAuth models used as agent defaults (ids are the
// canonical Gemini ids prefixed with "gemini-oauth.").
const (
	GeminiCAPro   ModelID = "gemini-oauth.gemini-2.5"
	GeminiCAFlash ModelID = "gemini-oauth.gemini-2.5-flash"
)

// GeminiCAModels mirrors GeminiModels but on the OAuth/Code-Assist provider.
// Populated in init() so it always tracks the canonical Gemini list.
var GeminiCAModels = map[ModelID]Model{}

func init() {
	for id, m := range GeminiModels {
		clone := m
		clone.ID = ModelID("gemini-oauth." + string(id))
		clone.Provider = ProviderGeminiCA
		// Free tier via login — no per-token price to estimate.
		clone.CostPer1MIn = 0
		clone.CostPer1MInCached = 0
		clone.CostPer1MOut = 0
		clone.CostPer1MOutCached = 0
		GeminiCAModels[clone.ID] = clone
	}
	// Additive copy into the global registry (order-independent).
	maps.Copy(SupportedModels, GeminiCAModels)
}
