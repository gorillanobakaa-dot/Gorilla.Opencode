package models

import "maps"

type (
	ModelID       string
	ModelProvider string
)

type Model struct {
	ID   ModelID `json:"id"`
	Name string  `json:"name"`
	// GORILLA OVERRIDE: short human description of capability
	// (params, context window, coding strength) shown in the model
	// picker. Populated for discovered models from bundled metadata.
	Description string `json:"description,omitempty"`
	// GORILLA OVERRIDE: the full, untruncated description for the picker's
	// detail page (tab on a model). Description is one row and must fit one;
	// this is everything that was cut to make it fit. Empty means "no longer
	// text exists" and the detail page falls back to the structured fields.
	Detail string `json:"detail,omitempty"`
	// GORILLA OVERRIDE: curated rank (1 = best). 0 = not on the
	// verified best-models list; the picker can hide these.
	Rank                int           `json:"rank,omitempty"`
	Provider            ModelProvider `json:"provider"`
	APIModel            string        `json:"api_model"`
	CostPer1MIn         float64       `json:"cost_per_1m_in"`
	CostPer1MOut        float64       `json:"cost_per_1m_out"`
	CostPer1MInCached   float64       `json:"cost_per_1m_in_cached"`
	CostPer1MOutCached  float64       `json:"cost_per_1m_out_cached"`
	ContextWindow       int64         `json:"context_window"`
	DefaultMaxTokens    int64         `json:"default_max_tokens"`
	CanReason           bool          `json:"can_reason"`
	SupportsAttachments bool          `json:"supports_attachments"`
}

// GORILLA CULL (2026-08-21): Bedrock, Azure, Copilot and VertexAI were removed
// — model lists, provider clients and picker rows. Every one of them requires an
// enterprise account, a paid subscription or a billing-enabled cloud project, so
// none of them is reachable by the audience this program is built for (CLAUDE.md:
// "which option works for someone on a 2012 laptop, a metered connection, and no
// credit card?"). Between them they contributed 30 entries to a picker whose whole
// problem is that it is too long to read.
//
// They were not deleted: the files are in
// /home/gorilla/Agents.Work.Trash/gorilla-opencode-provider-cull-26-08-21-11-58/
// should anyone ever want them back.

const (
	// ForTests
	ProviderMock ModelProvider = "__mock"
)

// Providers in order of popularity. Lower sorts first in the picker.
var ProviderPopularity = map[ModelProvider]int{
	ProviderAnthropic:  2,
	ProviderOpenAI:     3,
	ProviderGemini:     4,
	ProviderGROQ:       5,
	ProviderCerebras:   5,
	ProviderOpenRouter: 6,
	ProviderDeepSeek:   6,
	ProviderXAI:        7,
}

var SupportedModels = map[ModelID]Model{}

func init() {
	// What is left compiled in: Gemini (a curated list behind a free, no-card
	// AI Studio key) and the generated OpenRouter catalogue. Everything else is
	// either fetched from its provider (catalogue_fetch.go) or registered by an
	// OAuth sign-in the app itself controls (Antigravity, ChatGPT, Code Assist).
	maps.Copy(SupportedModels, GeminiModels)
	maps.Copy(SupportedModels, OpenRouterGeneratedModels)
}
