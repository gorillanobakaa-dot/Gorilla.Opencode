// GORILLA OVERRIDE: this file did not exist upstream. It registers the models
// Google Antigravity's free tier serves to a personal Google account — Claude
// Sonnet/Opus, GPT-OSS, and Gemini — reached through daily-cloudcode-pa via
// "Login with Google (Antigravity)" (see internal/auth/antigravity_oauth.go and
// internal/llm/provider/antigravity.go). No API key; the entitlement is the
// user's own free tier, so cost is 0.
//
// The model id strings are exactly what `agy models` (1.1.10) reports and what
// the backend accepts as the envelope's top-level "model" field. claude-sonnet-4-6
// was additionally proven live (200, modelVersion "claude-sonnet-4-6") on
// 2026-08-03; the others are taken verbatim from the same client's catalogue.
package models

const ProviderAntigravity ModelProvider = "antigravity"

const (
	AGClaudeSonnet46 ModelID = "antigravity.claude-sonnet-4-6"
	AGClaudeOpus46   ModelID = "antigravity.claude-opus-4-6-thinking"
	AGGPTOSS120B     ModelID = "antigravity.gpt-oss-120b-medium"
	AGGemini36Flash  ModelID = "antigravity.gemini-3.6-flash-medium"
	AGGemini31Pro    ModelID = "antigravity.gemini-3.1-pro-high"
)

// AntigravityModels is the curated catalogue exposed in the model picker.
var AntigravityModels = map[ModelID]Model{
	AGClaudeSonnet46: {
		ID:                  AGClaudeSonnet46,
		Name:                "Claude Sonnet 4.6 (Antigravity free)",
		Description:         "Anthropic Claude Sonnet 4.6 via your free Google Antigravity tier. Strong general coding model.",
		Provider:            ProviderAntigravity,
		APIModel:            "claude-sonnet-4-6",
		ContextWindow:       200_000,
		DefaultMaxTokens:    16_384,
		CanReason:           false,
		SupportsAttachments: true,
		Rank:                9,
	},
	AGClaudeOpus46: {
		ID:                  AGClaudeOpus46,
		Name:                "Claude Opus 4.6 Thinking (Antigravity free)",
		Description:         "Anthropic Claude Opus 4.6 with extended thinking, via your free Google Antigravity tier. Strongest reasoning; shared weekly quota.",
		Provider:            ProviderAntigravity,
		APIModel:            "claude-opus-4-6-thinking",
		ContextWindow:       200_000,
		DefaultMaxTokens:    32_000,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                10,
	},
	AGGPTOSS120B: {
		ID:                  AGGPTOSS120B,
		Name:                "GPT-OSS 120B (Antigravity free)",
		Description:         "OpenAI GPT-OSS 120B via your free Google Antigravity tier.",
		Provider:            ProviderAntigravity,
		APIModel:            "gpt-oss-120b-medium",
		ContextWindow:       128_000,
		DefaultMaxTokens:    16_384,
		CanReason:           true,
		SupportsAttachments: false,
		Rank:                6,
	},
	AGGemini36Flash: {
		ID:                  AGGemini36Flash,
		Name:                "Gemini 3.6 Flash (Antigravity free)",
		Description:         "Google Gemini 3.6 Flash via the Antigravity tier. Fast, large context; shares the Gemini weekly quota.",
		Provider:            ProviderAntigravity,
		APIModel:            "gemini-3.6-flash-medium",
		ContextWindow:       1_000_000,
		DefaultMaxTokens:    16_384,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                7,
	},
	AGGemini31Pro: {
		ID:                  AGGemini31Pro,
		Name:                "Gemini 3.1 Pro (Antigravity free)",
		Description:         "Google Gemini 3.1 Pro via the Antigravity tier. Deep reasoning, large context; shares the Gemini weekly quota.",
		Provider:            ProviderAntigravity,
		APIModel:            "gemini-3.1-pro-high",
		ContextWindow:       1_000_000,
		DefaultMaxTokens:    16_384,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                8,
	},
}

func init() {
	// Additive copy into the global registry (order-independent), mirroring the
	// Gemini Code Assist registration.
	for id, m := range AntigravityModels {
		SupportedModels[id] = m
	}
	// Rank the provider so its models sort sensibly in the picker rather than
	// landing last (0 => "unranked"). Claude/GPT here are a headline capability.
	ProviderPopularity[ProviderAntigravity] = 2
}
