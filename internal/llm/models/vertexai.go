package models

const (
	ProviderVertexAI ModelProvider = "vertexai"

	// Models
	VertexAIGemini36Flash ModelID = "vertexai.gemini-3.6-flash"
	VertexAIGemini35Flash ModelID = "vertexai.gemini-3.5-flash"
	VertexAIGemini25Flash ModelID = "vertexai.gemini-2.5-flash"
	VertexAIGemini25      ModelID = "vertexai.gemini-2.5"
)

var VertexAIGeminiModels = map[ModelID]Model{
	VertexAIGemini36Flash: {
		ID:                  VertexAIGemini36Flash,
		Name:                "VertexAI: Gemini 3.6 Flash",
		Provider:            ProviderVertexAI,
		APIModel:            "gemini-3.6-flash",
		ContextWindow:       1000000,
		DefaultMaxTokens:    50000,
		SupportsAttachments: true,
	},
	VertexAIGemini35Flash: {
		ID:                  VertexAIGemini35Flash,
		Name:                "VertexAI: Gemini 3.5 Flash",
		Provider:            ProviderVertexAI,
		APIModel:            "gemini-3.5-flash",
		ContextWindow:       1000000,
		DefaultMaxTokens:    50000,
		SupportsAttachments: true,
	},
	VertexAIGemini25Flash: {
		ID:                  VertexAIGemini25Flash,
		Name:                "VertexAI: Gemini 2.5 Flash",
		Provider:            ProviderVertexAI,
		APIModel:            "gemini-2.5-flash-preview-04-17",
		CostPer1MIn:         GeminiModels[GeminiFlashLatest].CostPer1MIn,
		CostPer1MInCached:   GeminiModels[GeminiFlashLatest].CostPer1MInCached,
		CostPer1MOut:        GeminiModels[GeminiFlashLatest].CostPer1MOut,
		CostPer1MOutCached:  GeminiModels[GeminiFlashLatest].CostPer1MOutCached,
		ContextWindow:       GeminiModels[GeminiFlashLatest].ContextWindow,
		DefaultMaxTokens:    GeminiModels[GeminiFlashLatest].DefaultMaxTokens,
		SupportsAttachments: true,
	},
	VertexAIGemini25: {
		ID:                  VertexAIGemini25,
		Name:                "VertexAI: Gemini 2.5 Pro",
		Provider:            ProviderVertexAI,
		APIModel:            "gemini-2.5-pro-preview-03-25",
		CostPer1MIn:         GeminiModels[GeminiProLatest].CostPer1MIn,
		CostPer1MInCached:   GeminiModels[GeminiProLatest].CostPer1MInCached,
		CostPer1MOut:        GeminiModels[GeminiProLatest].CostPer1MOut,
		CostPer1MOutCached:  GeminiModels[GeminiProLatest].CostPer1MOutCached,
		ContextWindow:       GeminiModels[GeminiProLatest].ContextWindow,
		DefaultMaxTokens:    GeminiModels[GeminiProLatest].DefaultMaxTokens,
		SupportsAttachments: true,
	},
}
