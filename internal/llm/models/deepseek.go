package models

const (
	ProviderDeepSeek ModelProvider = "deepseek"

	DeepSeekV4Flash ModelID = "deepseek-v4-flash"
	DeepSeekV4Pro   ModelID = "deepseek-v4-pro"
)

var DeepSeekModels = map[ModelID]Model{
	DeepSeekV4Flash: {
		ID:                  DeepSeekV4Flash,
		Name:                "DeepSeek V4 Flash",
		Description:         "DeepSeek V4 Flash — 1M ctx, 384K max output, thinking mode",
		Provider:            ProviderDeepSeek,
		APIModel:            "deepseek-v4-flash",
		CostPer1MIn:         0.14,
		CostPer1MInCached:   0.0028,
		CostPer1MOut:        0.28,
		CostPer1MOutCached:  0, // DeepSeek does not advertise cached-output pricing
		ContextWindow:       1_000_000,
		DefaultMaxTokens:    384_000,
		SupportsAttachments: true,
	},
	DeepSeekV4Pro: {
		ID:                  DeepSeekV4Pro,
		Name:                "DeepSeek V4 Pro",
		Description:         "DeepSeek V4 Pro — 1M ctx, 384K max output, thinking mode",
		Provider:            ProviderDeepSeek,
		APIModel:            "deepseek-v4-pro",
		CostPer1MIn:         0.435,
		CostPer1MInCached:   0.003625,
		CostPer1MOut:        0.87,
		CostPer1MOutCached:  0, // DeepSeek does not advertise cached-output pricing
		ContextWindow:       1_000_000,
		DefaultMaxTokens:    384_000,
		SupportsAttachments: true,
	},
}
