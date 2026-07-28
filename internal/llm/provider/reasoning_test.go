package provider

import (
	"strings"
	"testing"
)

// The bug: reasoning was captured only from Anthropic. Every OpenAI-compatible
// backend — NIM, GLM, Ollama, DeepSeek — streams it under a vendor-specific name
// the SDK has no typed field for, so it was dropped. A real session database held
// zero reasoning parts against 81 text parts.
//
// These are the delta shapes those backends actually send.
func TestReasoningIsExtractedFromEveryKnownVendorField(t *testing.T) {
	cases := map[string]struct {
		delta string
		want  string
	}{
		"DeepSeek / GLM / NIM": {`{"role":"assistant","reasoning_content":"Let me check the branch."}`, "Let me check the branch."},
		"OpenRouter":           {`{"role":"assistant","reasoning":"Checking the handler."}`, "Checking the handler."},
		"gateway variant":      {`{"thinking":"Considering the width."}`, "Considering the width."},
		"structured reasoning": {`{"reasoning":{"text":"Nested prose."}}`, "Nested prose."},
		"reasoning alongside content": {
			`{"content":"the answer","reasoning_content":"the working"}`,
			// The reasoning channel must not swallow the answer, nor vice versa.
			"the working",
		},
	}
	for name, c := range cases {
		if got := reasoningDelta(c.delta); got != c.want {
			t.Errorf("%s: reasoningDelta(%s) = %q, want %q", name, c.delta, got, c.want)
		}
	}
}

// A plain content-only delta — the overwhelming majority — must yield nothing, or
// every ordinary token would be duplicated into the reasoning stream.
func TestOrdinaryContentIsNotMistakenForReasoning(t *testing.T) {
	for _, delta := range []string{
		`{"content":"hello"}`,
		`{"role":"assistant","content":""}`,
		`{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"bash"}}]}`,
		`{}`,
	} {
		if got := reasoningDelta(delta); got != "" {
			t.Errorf("reasoningDelta(%s) = %q, expected nothing", delta, got)
		}
	}
}

// This runs on every chunk of every stream against servers whose payloads we do
// not control. Anything malformed must yield "" rather than panic or error — a
// crash mid-stream would lose the whole turn.
func TestMalformedDeltasAreSurvivedNotFatal(t *testing.T) {
	for _, delta := range []string{
		"",
		"not json at all",
		`{"reasoning_content":`,
		`{"reasoning_content":null}`,
		`{"reasoning_content":12345}`,
		`{"reasoning_content":["a","b"]}`,
		`{"reasoning_content":{}}`,
		`{"reasoning":{"id":"abc","tokens":40}}`, // structured, but no prose
		`[]`,
		`null`,
	} {
		if got := reasoningDelta(delta); got != "" {
			t.Errorf("reasoningDelta(%q) = %q, expected nothing from a payload we cannot read", delta, got)
		}
	}
}

// Field precedence has to be deterministic when a proxy sends more than one.
func TestFieldPrecedenceIsDeterministic(t *testing.T) {
	const both = `{"reasoning_content":"specific","reasoning":"generic"}`
	first := reasoningDelta(both)
	for i := 0; i < 20; i++ {
		if got := reasoningDelta(both); got != first {
			t.Fatalf("unstable result across calls: %q then %q — map iteration order is leaking into the output", first, got)
		}
	}
	if first != "specific" {
		t.Errorf("got %q, want the more specific reasoning_content field", first)
	}
}

// A structured value must never be rendered via Go's default formatting. If that
// happened, a user's transcript would fill with map[...] noise.
func TestStructuredReasoningNeverLeaksGoFormatting(t *testing.T) {
	for _, delta := range []string{
		`{"reasoning":{"text":"fine"}}`,
		`{"reasoning":{"id":"x","tokens":3}}`,
		`{"reasoning":[1,2,3]}`,
	} {
		got := reasoningDelta(delta)
		if strings.Contains(got, "map[") || strings.Contains(got, "0x") {
			t.Errorf("reasoningDelta(%s) leaked Go formatting: %q", delta, got)
		}
	}
}

// Whitespace-only reasoning is real: models stream leading spaces and newlines
// between thoughts, and swallowing them would run words together.
func TestWhitespaceReasoningIsPreserved(t *testing.T) {
	if got := reasoningDelta(`{"reasoning_content":" and then"}`); got != " and then" {
		t.Errorf("leading space lost: %q", got)
	}
	if got := reasoningDelta(`{"reasoning_content":"\n\n"}`); got != "\n\n" {
		t.Errorf("newlines lost: %q", got)
	}
}
