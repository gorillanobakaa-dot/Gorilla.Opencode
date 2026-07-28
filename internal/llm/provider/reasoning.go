package provider

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/openai/openai-go/option"
	"github.com/opencode-ai/opencode/internal/config"
)

// GORILLA OVERRIDE: extract the model's reasoning from an OpenAI-compatible
// streaming delta.
//
// Only the Anthropic provider ever emitted EventThinkingDelta, so for every
// other backend the reasoning was discarded — while reasoning_effort was still
// being sent on the request. We were paying to make the model think and then
// throwing the thinking away. A real session database confirmed it: 126 finish
// parts, 81 text, 25 tool_call, 25 tool_result, and zero reasoning.
//
// It is discarded rather than merely hidden, and that is the cost: /export can
// then show what the model concluded but never how it got there, which is the
// one thing you need when a conclusion turns out to be wrong.
//
// There is no standard field for this. It is a vendor extension and each one
// spells it differently, so the raw JSON is searched for any of the known names
// rather than relying on a typed field the SDK does not have:
//
//   - reasoning_content — DeepSeek, GLM/Z-AI, most NVIDIA NIM models
//   - reasoning         — OpenRouter, some proxies
//   - thinking          — a few OpenAI-compatible gateways
//
// An unknown backend simply yields nothing, which is the old behaviour; this can
// only add reasoning, never break a stream that has none.
var reasoningFieldNames = []string{"reasoning_content", "reasoning", "thinking"}

// reasoningDelta returns the reasoning token carried by a raw streaming delta,
// or "" if there is none.
//
// Robustness matters more than elegance here: this runs on every chunk of every
// stream, against servers whose payloads we do not control. Anything unexpected
// must yield "" rather than an error or a panic.
func reasoningDelta(rawDeltaJSON string) string {
	if rawDeltaJSON == "" {
		return ""
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rawDeltaJSON), &fields); err != nil {
		return ""
	}

	for _, name := range reasoningFieldNames {
		raw, ok := fields[name]
		if !ok {
			continue
		}
		if s := reasoningString(raw); s != "" {
			return s
		}
	}
	return ""
}

// reasoningString pulls a string out of a value that is usually a plain string
// but is sometimes an object. OpenRouter, for instance, has been known to send
// `"reasoning": {"text": "..."}`, and a structured value must not be rendered as
// Go's map formatting into the user's transcript.
func reasoningString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	// Only the keys that plausibly hold prose. Guessing more widely risks
	// putting an id or a token count into the reasoning stream.
	for _, key := range []string{"text", "content", "thinking"} {
		if inner, ok := obj[key]; ok {
			var s string
			if err := json.Unmarshal(inner, &s); err == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

// ── Asking for the reasoning in the first place ──────────────────────────────
//
// GORILLA OVERRIDE: reading reasoning_content is only half the job. Measured
// against the real NVIDIA NIM endpoint on 2026-07-28 with z-ai/glm-5.2:
//
//	plain request                              delta keys = [content, role]
//	+ reasoning_effort: high                   delta keys = [content, role]
//	+ chat_template_kwargs:{thinking:true}      delta keys = [reasoning_content, role]
//
// So the reasoning_effort parameter this provider already sends does NOTHING for
// GLM — it is an OpenAI-specific field — and the reader added above would have
// been dead code without this. The thinking flag has to be requested explicitly.
//
// It is a vendor extension, so a stricter server may reject it outright. Rather
// than maintain a list of which models accept it, ask once and remember: a
// rejection marks the model and the request is retried without the parameter. A
// server that ignores unknown fields (most do) simply never triggers that path.

// thinkingRejected records models whose server refused the thinking parameter.
// sync.Map because streams for several models can run concurrently.
var thinkingRejected sync.Map // model ID → struct{}

// thinkingRequestOptions returns the extra body fields that ask an
// OpenAI-compatible server to emit its reasoning, unless this model has already
// refused them.
func thinkingRequestOptions(modelID string) []option.RequestOption {
	// GORILLA OVERRIDE: the user's call, and it defaults to OFF. Reasoning makes
	// the model emit substantially more output, so it is not switched on by a
	// default nobody was shown — see config/extras.go and the first-run prompt.
	if !config.ExtraEnabled("extras-reasoning-generate") {
		return nil
	}
	if _, refused := thinkingRejected.Load(modelID); refused {
		return nil
	}
	return []option.RequestOption{
		option.WithJSONSet("chat_template_kwargs", map[string]any{"thinking": true}),
	}
}

// noteThinkingRejected stops us asking this model for reasoning again.
func noteThinkingRejected(modelID string) {
	thinkingRejected.Store(modelID, struct{}{})
}

// thinkingWasRequested reports whether the last attempt carried the parameter,
// which decides whether a rejection is worth retrying without it.
func thinkingWasRequested(modelID string) bool {
	if !config.ExtraEnabled("extras-reasoning-generate") {
		return false
	}
	_, refused := thinkingRejected.Load(modelID)
	return !refused
}

// isParameterRejection identifies "I do not understand that field" as distinct
// from a rate limit, an outage or a bad key. Only the former is fixed by
// dropping the parameter; retrying the others without it would silently disable
// reasoning because of an unrelated blip.
//
// Matched on text because OpenAI-compatible servers are not consistent about
// error codes for unknown parameters — the shape of the message is the only
// signal available across implementations.
func isParameterRejection(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// A 4xx that names our parameter is unambiguous.
	if strings.Contains(msg, "chat_template_kwargs") {
		return true
	}
	// Otherwise require both a client-error signal and unknown-field language,
	// so a generic 400 from a malformed prompt does not disable reasoning.
	clientError := strings.Contains(msg, "400") ||
		strings.Contains(msg, "422") ||
		strings.Contains(msg, "bad request") ||
		strings.Contains(msg, "unprocessable")
	unknownField := strings.Contains(msg, "unknown") ||
		strings.Contains(msg, "unexpected") ||
		strings.Contains(msg, "unrecognized") ||
		strings.Contains(msg, "not permitted") ||
		strings.Contains(msg, "extra fields") ||
		strings.Contains(msg, "additional propert")
	return clientError && unknownField
}
