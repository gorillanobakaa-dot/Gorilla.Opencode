package models

const (
	ProviderGemini ModelProvider = "gemini"

	// Rolling aliases (Google keeps these pointed at whatever it currently
	// serves, so they never go stale). Kept with their historical constant
	// names + ID strings because config.go, openrouter.go and vertexai.go
	// reference them — renaming would break those.
	Gemini25Flash ModelID = "gemini-2.5-flash" // → gemini-flash-latest
	Gemini25      ModelID = "gemini-2.5"       // → gemini-pro-latest

	Gemini20Flash     ModelID = "gemini-2.0-flash"
	Gemini20FlashLite ModelID = "gemini-2.0-flash-lite"

	// GORILLA OVERRIDE: models below added 2026-07-20 after verifying
	// liveness directly against the Gemini API (ListModels + a real
	// generateContent probe on each). Only models that (a) list on a live
	// key and (b) advertise generateContent are included; image/TTS/audio/
	// embedding/robotics/computer-use variants were deliberately excluded,
	// and the retired Gemini 1.5 line (gemini-1.5-flash / -pro, both HTTP
	// 404 "not found") was left out. Some of these return 429 "limit: 0"
	// on a free-tier key — that is a quota/billing gate, not a dead model;
	// the model itself is alive and works once the key has quota.
	Gemini3Pro        ModelID = "gemini-3-pro-preview"
	Gemini35Flash     ModelID = "gemini-3.5-flash"
	Gemini3Flash      ModelID = "gemini-3-flash-preview"
	Gemini31Pro       ModelID = "gemini-3.1-pro-preview"
	Gemini31FlashLite ModelID = "gemini-3.1-flash-lite"
	Gemini25Pro       ModelID = "gemini-2.5-pro"
	Gemini25FlashLite ModelID = "gemini-2.5-flash-lite"
	GeminiFlashLite   ModelID = "gemini-flash-lite-latest"
)

var GeminiModels = map[ModelID]Model{
	// ---- rolling aliases -------------------------------------------------
	Gemini25Flash: {
		ID:          Gemini25Flash,
		Name:        "Gemini Flash (latest)",
		Description: "Rolling Flash alias — Google keeps it current, 1M ctx",
		Provider:    ProviderGemini,
		// GORILLA OVERRIDE: the 04-17 preview alias died in 2025, and
		// versioned 2.5 aliases are gated off for new accounts ("no
		// longer available to new users", verified 2026-07-19). The
		// rolling alias tracks whatever Google currently serves.
		APIModel:            "gemini-flash-latest",
		CostPer1MIn:         0.15,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.60,
		ContextWindow:       1000000,
		DefaultMaxTokens:    50000,
		SupportsAttachments: true,
	},
	Gemini25: {
		ID:          Gemini25,
		Name:        "Gemini Pro (latest)",
		Description: "Rolling Pro alias — Google keeps it current, 1M ctx",
		Provider:    ProviderGemini,
		// GORILLA OVERRIDE: same as above — rolling alias, never stale.
		APIModel:            "gemini-pro-latest",
		CostPer1MIn:         1.25,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        10,
		ContextWindow:       1000000,
		DefaultMaxTokens:    50000,
		SupportsAttachments: true,
	},
	GeminiFlashLite: {
		ID:                  GeminiFlashLite,
		Name:                "Gemini Flash Lite (latest)",
		Description:         "Rolling Flash-Lite alias — cheapest/fastest, 1M ctx",
		Provider:            ProviderGemini,
		APIModel:            "gemini-flash-lite-latest",
		ContextWindow:       1000000,
		DefaultMaxTokens:    16000,
		SupportsAttachments: true,
	},

	// ---- Gemini 3.x (current generation) ---------------------------------
	Gemini3Pro: {
		ID:                  Gemini3Pro,
		Name:                "Gemini 3 Pro (preview)",
		Description:         "Google flagship — strongest reasoning, 1M ctx",
		Provider:            ProviderGemini,
		APIModel:            "gemini-3-pro-preview",
		ContextWindow:       1000000,
		DefaultMaxTokens:    50000,
		SupportsAttachments: true,
	},
	Gemini31Pro: {
		ID:                  Gemini31Pro,
		Name:                "Gemini 3.1 Pro (preview)",
		Description:         "Gen-3.1 Pro — deep reasoning, 1M ctx",
		Provider:            ProviderGemini,
		APIModel:            "gemini-3.1-pro-preview",
		ContextWindow:       1000000,
		DefaultMaxTokens:    50000,
		SupportsAttachments: true,
	},
	Gemini35Flash: {
		ID:                  Gemini35Flash,
		Name:                "Gemini 3.5 Flash",
		Description:         "Newest Flash — fast + strong general/coding, 1M ctx",
		Provider:            ProviderGemini,
		APIModel:            "gemini-3.5-flash",
		ContextWindow:       1000000,
		DefaultMaxTokens:    50000,
		SupportsAttachments: true,
	},
	Gemini3Flash: {
		ID:                  Gemini3Flash,
		Name:                "Gemini 3 Flash (preview)",
		Description:         "Gen-3 Flash — fast, 1M ctx",
		Provider:            ProviderGemini,
		APIModel:            "gemini-3-flash-preview",
		ContextWindow:       1000000,
		DefaultMaxTokens:    50000,
		SupportsAttachments: true,
	},
	Gemini31FlashLite: {
		ID:                  Gemini31FlashLite,
		Name:                "Gemini 3.1 Flash Lite",
		Description:         "Gen-3.1 Flash-Lite — cheapest/fastest gen-3, 1M ctx",
		Provider:            ProviderGemini,
		APIModel:            "gemini-3.1-flash-lite",
		ContextWindow:       1000000,
		DefaultMaxTokens:    16000,
		SupportsAttachments: true,
	},

	// ---- Gemini 2.5 ------------------------------------------------------
	Gemini25Pro: {
		ID:                  Gemini25Pro,
		Name:                "Gemini 2.5 Pro",
		Description:         "Gemini 2.5 Pro — strong reasoning, 1M ctx",
		Provider:            ProviderGemini,
		APIModel:            "gemini-2.5-pro",
		CostPer1MIn:         1.25,
		CostPer1MOut:        10,
		ContextWindow:       1000000,
		DefaultMaxTokens:    50000,
		SupportsAttachments: true,
	},
	Gemini25FlashLite: {
		ID:                  Gemini25FlashLite,
		Name:                "Gemini 2.5 Flash Lite",
		Description:         "Gemini 2.5 Flash-Lite — cheap + fast, 1M ctx",
		Provider:            ProviderGemini,
		APIModel:            "gemini-2.5-flash-lite",
		ContextWindow:       1000000,
		DefaultMaxTokens:    16000,
		SupportsAttachments: true,
	},

	// ---- Gemini 2.0 ------------------------------------------------------
	Gemini20Flash: {
		ID:                  Gemini20Flash,
		Name:                "Gemini 2.0 Flash",
		Description:         "Gemini 2.0 Flash — fast, 1M ctx",
		Provider:            ProviderGemini,
		APIModel:            "gemini-2.0-flash",
		CostPer1MIn:         0.10,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.40,
		ContextWindow:       1000000,
		DefaultMaxTokens:    6000,
		SupportsAttachments: true,
	},
	Gemini20FlashLite: {
		ID:                  Gemini20FlashLite,
		Name:                "Gemini 2.0 Flash Lite",
		Description:         "Gemini 2.0 Flash-Lite — cheapest 2.0, 1M ctx",
		Provider:            ProviderGemini,
		APIModel:            "gemini-2.0-flash-lite",
		CostPer1MIn:         0.05,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.30,
		ContextWindow:       1000000,
		DefaultMaxTokens:    6000,
		SupportsAttachments: true,
	},
}
