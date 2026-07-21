// GORILLA OVERRIDE: this file did not exist upstream. It exposes the live
// Gemini models a second time under a distinct provider — "gemini-oauth" —
// reached through Google's Code Assist free tier via `gorilla-opencode
// login` (see internal/auth + internal/llm/provider/code_assist.go). Same
// models, different transport: no API key, cost 0 (free tier).
package models

import "maps"

const ProviderGeminiCA ModelProvider = "gemini-oauth"

// Handy ids for the OAuth models used as agent defaults (ids are the
// canonical Gemini ids prefixed with "gemini-oauth.").
const (
	GeminiCA31FlashLite ModelID = "gemini-oauth.gemini-3.1-flash-lite"
	GeminiCA3Flash      ModelID = "gemini-oauth.gemini-3-flash-preview"
	GeminiCAPro         ModelID = "gemini-oauth.gemini-2.5"
	GeminiCAFlash       ModelID = "gemini-oauth.gemini-2.5-flash"
)

// GeminiCAModels mirrors GeminiModels but on the OAuth/Code-Assist provider.
// Populated in init() excluding models unserved by cloudcode-pa.googleapis.com (e.g. 3.6/3.5).
var GeminiCAModels = map[ModelID]Model{}

func init() {
	for id, m := range GeminiModels {
		// GORILLA OVERRIDE: Google Code Assist (cloudcode-pa.googleapis.com) returns HTTP 404
		// "Requested entity was not found" for 3.6-flash, 3.5-flash, 3.5-flash-lite, and 2.0-flash.
		// Only expose models live on the Code Assist backend.
		if id == Gemini36Flash || id == Gemini35Flash || id == Gemini35FlashLite || id == Gemini20Flash {
			continue
		}
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
