// GORILLA OVERRIDE: this file did not exist upstream. It registers the models a
// personal ChatGPT sign-in can reach through the Codex backend
// (chatgpt.com/backend-api/codex) — no API key, no credit card. The entitlement
// is the user's own ChatGPT plan, including the FREE plan, which is why this
// path exists at all: it is the only OpenAI route in this program that a user
// with no payment method can take.
//
// Every field below was read from the live model list on 2026-08-17 (a free
// plan, HTTP 200, five models). The list is at GET /models?client_version=…;
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
// KNOWN EXPIRY: OpenAI retires GPT-5.4 and 5.4-mini for ChatGPT-plan sign-ins on
// 31 Aug 2026. ChatGPT54Mini will start failing then and should be dropped, not
// debugged.
package models

const ProviderChatGPT ModelProvider = "chatgpt"

const (
	ChatGPT55     ModelID = "chatgpt.gpt-5.5"
	ChatGPT54Mini ModelID = "chatgpt.gpt-5.4-mini"
)

// ChatGPTModels is the catalogue exposed in the model picker. Cost is 0 in every
// field because nothing here is billed per token — the ChatGPT plan is, and a
// free plan is not billed at all. Do not populate the cost fields with API
// prices; they would be shown to the user as money this path never charges.
var ChatGPTModels = map[ModelID]Model{
	ChatGPT55: {
		ID:          ChatGPT55,
		Name:        "GPT-5.5 (ChatGPT sign-in)",
		Description: "OpenAI GPT-5.5 through your own ChatGPT account — works on the free plan, no API key.",
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
		Rank:                9,
	},
	ChatGPT54Mini: {
		ID:          ChatGPT54Mini,
		Name:        "GPT-5.4 Mini (ChatGPT sign-in)",
		Description: "Smaller, faster OpenAI model on your ChatGPT plan. Retired by OpenAI on 31 Aug 2026.",
		Detail: "The lighter of the two models a ChatGPT login can reach. Same 272K context, " +
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
