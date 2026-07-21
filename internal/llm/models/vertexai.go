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
		CostPer1MIn:         GeminiModels[Gemini25Flash].CostPer1MIn,
		CostPer1MInCached:   GeminiModels[Gemini25Flash].CostPer1MInCached,
		CostPer1MOut:        GeminiModels[Gemini25Flash].CostPer1MOut,
		CostPer1MOutCached:  GeminiModels[Gemini25Flash].CostPer1MOutCached,
		ContextWindow:       GeminiModels[Gemini25Flash].ContextWindow,
		DefaultMaxTokens:    GeminiModels[Gemini25Flash].DefaultMaxTokens,
		SupportsAttachments: true,
	},
	VertexAIGemini25: {
		ID:                  VertexAIGemini25,
		Name:                "VertexAI: Gemini 2.5 Pro",
		Provider:            ProviderVertexAI,
		APIModel:            "gemini-2.5-pro-preview-03-25",
		CostPer1MIn:         GeminiModels[Gemini25].CostPer1MIn,
		CostPer1MInCached:   GeminiModels[Gemini25].CostPer1MInCached,
		CostPer1MOut:        GeminiModels[Gemini25].CostPer1MOut,
		CostPer1MOutCached:  GeminiModels[Gemini25].CostPer1MOutCached,
		ContextWindow:       GeminiModels[Gemini25].ContextWindow,
		DefaultMaxTokens:    GeminiModels[Gemini25].DefaultMaxTokens,
		SupportsAttachments: true,
	},
}
