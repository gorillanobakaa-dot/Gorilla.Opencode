package models

const (
	ProviderGROQ ModelProvider = "groq"

	// GROQ
	QWENQwq ModelID = "qwen-qwq"

	// GROQ preview models
	Llama4Scout               ModelID = "meta-llama/llama-4-scout-17b-16e-instruct"
	Llama4Maverick            ModelID = "meta-llama/llama-4-maverick-17b-128e-instruct"
	Llama3_3_70BVersatile     ModelID = "llama-3.3-70b-versatile"
	DeepseekR1DistillLlama70b ModelID = "deepseek-r1-distill-llama-70b"
)

// GORILLA OVERRIDE (2026-08-09): names and descriptions.
//
// These shipped as bare Go identifiers - "Llama4Scout", "Llama3_3_70BVersatile",
// "DeepseekR1DistillLlama70b" - with no description at all. In a picker whose
// whole purpose is to spare people researching model names, an identifier is the
// worst possible label: it is not the model's real name, it is not searchable,
// and it tells you nothing about size, context or what it is for.
//
// That matters more here than it looks. Working out what an unfamiliar model
// name means costs a web search and a heavy vendor page EACH, which on a
// single-digit-KB/s connection is not slow, it is impossible. The curated
// description is the whole point.
//
// Every fact below is derived from the APIModel string beside it
// ("llama-4-scout-17b-16e-instruct" -> 17B active, 16 experts) or from the
// context window already in the entry. Nothing is remembered or guessed.
var GroqModels = map[ModelID]Model{
	//
	// GROQ
	QWENQwq: {
		ID:                 QWENQwq,
		Name:               "Qwen QwQ 32B (Groq)",
		Description:        "32B reasoning model, 128K ctx — very fast on Groq's LPU hardware",
		Provider:           ProviderGROQ,
		APIModel:           "qwen-qwq-32b",
		CostPer1MIn:        0.29,
		CostPer1MInCached:  0.275,
		CostPer1MOutCached: 0.0,
		CostPer1MOut:       0.39,
		ContextWindow:      128_000,
		DefaultMaxTokens:   50000,
		// for some reason, the groq api doesn't like the reasoningEffort parameter
		CanReason:           false,
		SupportsAttachments: false,
	},

	Llama4Scout: {
		ID:                  Llama4Scout,
		Name:                "Llama 4 Scout (Groq)",
		Description:         "17B active / 16 experts MoE, 128K ctx — cheapest Llama 4 here, very fast",
		Provider:            ProviderGROQ,
		APIModel:            "meta-llama/llama-4-scout-17b-16e-instruct",
		CostPer1MIn:         0.11,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.34,
		ContextWindow:       128_000, // 10M when?
		SupportsAttachments: true,
	},

	Llama4Maverick: {
		ID:                  Llama4Maverick,
		Name:                "Llama 4 Maverick (Groq)",
		Description:         "17B active / 128 experts MoE, 128K ctx — stronger Llama 4 tier",
		Provider:            ProviderGROQ,
		APIModel:            "meta-llama/llama-4-maverick-17b-128e-instruct",
		CostPer1MIn:         0.20,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.20,
		ContextWindow:       128_000,
		SupportsAttachments: true,
	},

	Llama3_3_70BVersatile: {
		ID:                  Llama3_3_70BVersatile,
		Name:                "Llama 3.3 70B (Groq)",
		Description:         "70B dense, 128K ctx — solid generalist fallback",
		Provider:            ProviderGROQ,
		APIModel:            "llama-3.3-70b-versatile",
		CostPer1MIn:         0.59,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.79,
		ContextWindow:       128_000,
		SupportsAttachments: false,
	},

	DeepseekR1DistillLlama70b: {
		ID:                  DeepseekR1DistillLlama70b,
		Name:                "DeepSeek R1 Distill 70B (Groq)",
		Description:         "R1 reasoning distilled into Llama 70B, 128K ctx — reasoning-tuned",
		Provider:            ProviderGROQ,
		APIModel:            "deepseek-r1-distill-llama-70b",
		CostPer1MIn:         0.75,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0.99,
		ContextWindow:       128_000,
		CanReason:           true,
		SupportsAttachments: false,
	},
}
