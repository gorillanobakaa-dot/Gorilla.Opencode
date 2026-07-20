// GORILLA OVERRIDE: this file did not exist upstream. Cerebras is a
// native provider now (its own CEREBRAS_API_KEY), mirroring Groq — both
// are OpenAI-compatible and both sell raw inference speed (Cerebras runs
// on wafer-scale chips). Models verified live against api.cerebras.ai on
// 2026-07-20. Costs are 0 because the key this fork targets is the free
// tier; the status bar marks any cost as an estimate anyway.
package models

const (
	ProviderCerebras ModelProvider = "cerebras"

	CerebrasGLM47  ModelID = "cerebras.zai-glm-4.7"
	CerebrasGPTOSS ModelID = "cerebras.gpt-oss-120b"
	CerebrasGemma4 ModelID = "cerebras.gemma-4-31b"
)

var CerebrasModels = map[ModelID]Model{
	CerebrasGLM47: {
		ID:                  CerebrasGLM47,
		Name:                "GLM 4.7 (Cerebras)",
		Description:         "Z.ai GLM 4.7 on wafer-scale silicon — strong coder, very fast",
		Rank:                1,
		Provider:            ProviderCerebras,
		APIModel:            "zai-glm-4.7",
		ContextWindow:       131_072,
		DefaultMaxTokens:    8192,
		SupportsAttachments: false,
	},
	CerebrasGPTOSS: {
		ID:                  CerebrasGPTOSS,
		Name:                "GPT-OSS 120B (Cerebras)",
		Description:         "OpenAI open-weight 120B MoE, 128K ctx, extremely fast",
		Rank:                2,
		Provider:            ProviderCerebras,
		APIModel:            "gpt-oss-120b",
		ContextWindow:       128_000,
		DefaultMaxTokens:    8192,
		SupportsAttachments: false,
	},
	CerebrasGemma4: {
		ID:                  CerebrasGemma4,
		Name:                "Gemma 4 31B (Cerebras)",
		Description:         "Google Gemma 4 31B, general-purpose, fast",
		Rank:                3,
		Provider:            ProviderCerebras,
		APIModel:            "gemma-4-31b",
		ContextWindow:       128_000,
		DefaultMaxTokens:    8192,
		SupportsAttachments: false,
	},
}
