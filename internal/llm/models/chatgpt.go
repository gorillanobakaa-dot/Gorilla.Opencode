// GORILLA OVERRIDE: this file did not exist upstream. It registers the models a
// personal ChatGPT sign-in can reach through the Codex backend
// (chatgpt.com/backend-api/codex): no API key, no credit card. The entitlement
// is the user's own ChatGPT plan, including the FREE plan, which is why this
// path exists at all: it is the only OpenAI route in this program that a user
// with no payment method can take.
//
// Every field below was read from the live model list on 2026-08-17 (a free
// plan, HTTP 200, five models). The list is at GET /models?client_version=...;
// see auth.ChatGPTCreds.ProbeBackend.
//
// # WHY ONLY TWO OF THE FIVE ARE REGISTERED
//
// The backend reports a per-model "tool_mode". Two values appear:
//
//	gpt-5.5        tool_mode: null            → ordinary function calling
//	gpt-5.4-mini   tool_mode: null            → ordinary function calling
//	gpt-5.6-terra  tool_mode: code_mode_only  → NOT ordinary function calling
//	gpt-5.6-luna   tool_mode: code_mode_only  → NOT ordinary function calling
//	codex-auto-review  visibility: hide       → not a chat model
//
// "code_mode_only" means the model expects its tools presented as a code
// sandbox rather than as a function schema list; this program's tools are
// function schemas. Registering the 5.6 models would put two rows in the picker
// that sign in fine and then fail on the first tool call, which is worse than
// not offering them. They are left out until that shape is implemented, and the
// omission is deliberate rather than an oversight.
//
// ---------------------------------------------------------------------------
// CORRECTION (2026-08-23): THE PARAGRAPH ABOVE IS WRONG. MEASURED, NOT ARGUED.
//
// It is left standing because it shows what changed, and because the mistake in
// it is worth more than the conclusion: it reasoned from the NAME of a flag to
// what a server would do, and then wrote the guess down in the confident voice
// of a finding. There is no record of anyone sending a request to check.
//
// The owner caught it from the outside on 2026-08-23: Codex was running
// gpt-5.6-luna on the same free Google account this program was telling him he
// could only reach GPT-5.5 and GPT-5.4-Mini on.
//
// What code_mode_only actually is, from the Codex source at
// ~/Downloads/codex-rust-v0.147.0 (codex-rs/features/src/lib.rs): "Restrict
// model-visible tools to code mode entrypoints (exec, wait)". A CLIENT-SIDE
// choice about which tools go in the tools array. Not a wire format, not a
// backend requirement, and nothing the server enforces.
//
// Sending each model one ordinary Responses request with one ordinary function
// schema, on the free plan, 2026-08-23:
//
//	gpt-5.5        HTTP 200  get_weather({"city":"Bucharest"})
//	gpt-5.6-luna   HTTP 200  get_weather({"city":"Bucharest"})
//	gpt-5.6-terra  HTTP 200  get_weather({"city":"Bucharest"})
//
// All four listed chat models are now registered, and the list is FETCHED rather
// than typed: see chatgpt_catalogue.go. The map below is only the offline
// fallback and the first-run list, and it is replaced by the live one as soon as
// a signed-in session asks.
// ---------------------------------------------------------------------------
//
// KNOWN EXPIRY: OpenAI retires GPT-5.4 and 5.4-mini for ChatGPT-plan sign-ins on
// 31 Aug 2026. ChatGPT54Mini will start failing then. It no longer needs to be
// dropped by hand: the fetched list stops carrying it and RefreshChatGPT
// replaces rather than merges, so it disappears on the first refresh after the
// retirement.
package models

const ProviderChatGPT ModelProvider = "chatgpt"

const (
	ChatGPT56Terra ModelID = "chatgpt.gpt-5.6-terra"
	ChatGPT56Luna  ModelID = "chatgpt.gpt-5.6-luna"
	ChatGPT55      ModelID = "chatgpt.gpt-5.5"
	ChatGPT54Mini  ModelID = "chatgpt.gpt-5.4-mini"
)

// ChatGPTModels is the catalogue exposed in the model picker. Cost is 0 in every
// field because nothing here is billed per token. The ChatGPT plan is, and a
// free plan is not billed at all. Do not populate the cost fields with API
// prices; they would be shown to the user as money this path never charges.
// Ranks match the order the backend itself publishes (its "priority" field,
// lower being more prominent: terra 2, luna 3, gpt-5.5 7, gpt-5.4-mini 23), so
// the picker does not reshuffle when the fallback is replaced by the fetched
// list. chatgpt_catalogue.go derives the same numbers from the wire.
var ChatGPTModels = map[ModelID]Model{
	ChatGPT56Terra: {
		ID:          ChatGPT56Terra,
		Name:        "GPT-5.6-Terra (ChatGPT sign-in)",
		Description: "Balanced agentic coding model for everyday work.",
		Detail: "Reached through the Codex backend using your ChatGPT login rather than an API key, " +
			"so it costs nothing beyond the plan you already have (including the free one). " +
			"272K context. OpenAI's own client runs this model with a code sandbox instead of a " +
			"tool list; ordinary tool calls were verified working here on 2026-08-23, but it is " +
			"tuned for that other shape.",
		Provider:            ProviderChatGPT,
		APIModel:            "gpt-5.6-terra",
		ContextWindow:       272_000,
		DefaultMaxTokens:    32_000,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                9,
	},
	ChatGPT56Luna: {
		ID:          ChatGPT56Luna,
		Name:        "GPT-5.6-Luna (ChatGPT sign-in)",
		Description: "Fast and affordable agentic coding model.",
		Detail: "Reached through the Codex backend using your ChatGPT login rather than an API key, " +
			"so it costs nothing beyond the plan you already have (including the free one). " +
			"272K context. OpenAI's own client runs this model with a code sandbox instead of a " +
			"tool list; ordinary tool calls were verified working here on 2026-08-23, but it is " +
			"tuned for that other shape.",
		Provider:            ProviderChatGPT,
		APIModel:            "gpt-5.6-luna",
		ContextWindow:       272_000,
		DefaultMaxTokens:    32_000,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                8,
	},
	ChatGPT55: {
		ID:          ChatGPT55,
		Name:        "GPT-5.5 (ChatGPT sign-in)",
		Description: "Frontier model for complex coding, research, and real-world work.",
		Detail: "Reached through the Codex backend using a ChatGPT login rather than an API key, " +
			"so it costs nothing beyond the plan you already have (including the free one). " +
			"272K context. Supports tools and image input. Usage counts against your ChatGPT " +
			"plan's limits, so heavy use on a free plan will hit a cooldown rather than a bill.",
		Provider:            ProviderChatGPT,
		APIModel:            "gpt-5.5",
		ContextWindow:       272_000,
		DefaultMaxTokens:    32_000,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                7,
	},
	ChatGPT54Mini: {
		ID:          ChatGPT54Mini,
		Name:        "GPT-5.4 Mini (ChatGPT sign-in)",
		Description: "Small, fast, and cost-efficient model for simpler coding tasks. Retired by OpenAI on 31 Aug 2026.",
		Detail: "The lightest model a ChatGPT login can reach. Same 272K context, " +
			"quicker and cheaper against your plan's limits, weaker on hard reasoning. " +
			"OpenAI has announced it stops serving GPT-5.4 to ChatGPT sign-ins on 31 August 2026; " +
			"after that date this entry will stop working and GPT-5.5 is the one to use.",
		Provider:            ProviderChatGPT,
		APIModel:            "gpt-5.4-mini",
		ContextWindow:       272_000,
		DefaultMaxTokens:    32_000,
		CanReason:           true,
		SupportsAttachments: true,
		Rank:                6,
	},
}

func init() {
	// Additive copy into the global registry, mirroring the Antigravity and
	// Gemini Code Assist registrations.
	for id, m := range ChatGPTModels {
		SupportedModels[id] = m
	}
	// Rank alongside the other free-tier sign-ins so these sort near the top of
	// the picker instead of landing last as "unranked".
	ProviderPopularity[ProviderChatGPT] = 2
}
