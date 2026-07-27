// GORILLA OVERRIDE: this file did not exist upstream. It exposes the live
// Gemini models a second time under a distinct provider — "gemini-oauth" —
// reached through Google's Code Assist free tier via `gorilla-opencode
// login` (see internal/auth + internal/llm/provider/code_assist.go). Same
// models, different transport: no API key, cost 0 (free tier).
package models

import (
	"maps"
	"strings"
)

const ProviderGeminiCA ModelProvider = "gemini-oauth"

// Handy ids for the OAuth models used as agent defaults (ids are the
// canonical Gemini ids prefixed with "gemini-oauth.").
//
// GORILLA OVERRIDE 2026-07-27: GeminiCAPro and GeminiCAFlash used to point at
// Gemini25 / Gemini25Flash, whose APIModels are the rolling aliases
// gemini-pro-latest / gemini-flash-latest. Code Assist cannot resolve rolling
// aliases (see servedByCodeAssist), so both 404'd — and because
// setDefaultModelForAgent falls back to these two, any agent whose real
// provider was unavailable silently landed on a dead model. Both now point at
// pinned ids probed live against cloudcode-pa.
const (
	GeminiCA31FlashLite ModelID = "gemini-oauth.gemini-3.1-flash-lite"
	GeminiCA3Flash      ModelID = "gemini-oauth.gemini-3-flash-preview"
	GeminiCAPro         ModelID = "gemini-oauth.gemini-3-pro-preview"
	GeminiCAFlash       ModelID = "gemini-oauth.gemini-3-flash-preview"
)

// servedByCodeAssist reports whether cloudcode-pa.googleapis.com will resolve
// this Gemini model. Two classes are not served, both returning HTTP 404
// "Requested entity was not found" at call time:
//
//  1. Rolling aliases (gemini-pro-latest, gemini-flash-latest,
//     gemini-flash-lite-latest). The public generativelanguage API resolves
//     these; Code Assist requires a pinned id. Matched on APIModel rather than
//     ID because the ids lie — Gemini25Flash's id is "gemini-2.5-flash" but it
//     serves gemini-flash-latest.
//  2. Model families Code Assist has not been given: the 3.5/3.6 line and the
//     whole 2.0 line.
//
// Verified 2026-07-27 by probing every candidate through `login` auth.
func servedByCodeAssist(m Model) bool {
	if strings.HasSuffix(m.APIModel, "-latest") {
		return false
	}
	switch m.ID {
	case Gemini36Flash, Gemini35Flash, Gemini35FlashLite,
		Gemini20Flash, Gemini20FlashLite:
		return false
	}
	return true
}

// GeminiCAModels mirrors GeminiModels but on the OAuth/Code-Assist provider.
// Populated in init() excluding models unserved by cloudcode-pa.googleapis.com (e.g. 3.6/3.5).
var GeminiCAModels = map[ModelID]Model{}

func init() {
	for id, m := range GeminiModels {
		// Only expose models the Code Assist backend actually serves.
		if !servedByCodeAssist(m) {
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
