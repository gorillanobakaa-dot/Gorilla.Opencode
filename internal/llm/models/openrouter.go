// GORILLA OVERRIDE: the hand-maintained OpenRouter list lived here and is gone.
//
// It carried 22 models. Checked against OpenRouter's live catalogue on
// 2026-08-09: NINE no longer exist there at all (claude-3.5-sonnet,
// claude-3.7-sonnet, claude-3.5-haiku, claude-3-opus, gpt-4.5-preview, o1-mini,
// two gemini-2.5 previews, deepseek-r1-0528:free) and a tenth, o1-pro, cannot
// call tools. Nearly half the list was unselectable-in-practice: pick one and
// the API returns an error.
//
// Worse, two of the dead ones were the DEFAULTS for every agent
// (internal/config/config.go), so configuring OpenRouter produced a setup that
// could not answer at all.
//
// That is what a hand-maintained mirror of someone else's catalogue does: it
// does not fail loudly when upstream moves, it just quietly stops being true.
// The list is now generated - see cmd/openrouter-models and
// openrouter_generated.go - and its prices come from OpenRouter rather than
// being copied from the upstream provider's, which they never matched anyway
// because OpenRouter takes a margin.

package models

// The provider identity stays here; only the model list moved.
const ProviderOpenRouter ModelProvider = "openrouter"
