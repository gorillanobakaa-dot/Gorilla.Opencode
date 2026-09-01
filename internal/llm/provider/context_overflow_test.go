package provider

import (
	"strings"
	"testing"
)

// GORILLA OVERRIDE (2026-09-01): the real thing, from the session database.
//
// Six of these were recorded in one local session. What reached the status bar
// was two levels of escaped JSON on a single line; what the user needed was
// "18545 against 15104, run /compact".
func TestContextOverflowIsExplainedInPlainEnglish(t *testing.T) {
	realErrors := []string{
		// LM Studio / llama.cpp, verbatim from gorilla-opencode.db.
		`received error while streaming: {"message":"Engine protocol predict request returned 400: {\"error\":{\"code\":400,\"message\":\"request (18545 tokens) exceeds the available context size (15104 tokens), try increasing it\",\"type\":\"exceed_context_size_error\",\"n_prompt_tokens\":18545,\"n_ctx\":15104}}"}`,
		// The 8192 variant from the same database.
		`received error while streaming: {"message":"Engine protocol predict request returned 400: {\"error\":{\"code\":400,\"message\":\"request (10279 tokens) exceeds the available context size (8192 tokens), try increasing it\",\"type\":\"exceed_context_size_error\"}}"}`,
		// OpenAI's wording.
		`This model's maximum context length is 8192 tokens, however you requested 9000 tokens.`,
		// Ollama's wording.
		`context length exceeded`,
	}

	for _, e := range realErrors {
		got := explainContextOverflow(e)
		if got == "" {
			t.Errorf("not recognised as a context overflow:\n%s", e)
			continue
		}
		if !strings.Contains(got, "/compact") {
			t.Errorf("the explanation does not name the fix (/compact):\n%s", got)
		}
		if strings.Contains(got, "{") || strings.Contains(got, `\"`) {
			t.Errorf("raw JSON leaked into the explanation:\n%s", got)
		}
	}
}

// The two numbers are the whole point: "too big" without them gives the reader
// no way to judge how much has to go.
func TestContextOverflowReportsBothSizes(t *testing.T) {
	got := explainContextOverflow(
		`request (18545 tokens) exceeds the available context size (15104 tokens), try increasing it`)
	for _, want := range []string{"18545", "15104"} {
		if !strings.Contains(got, want) {
			t.Errorf("explanation omits %q, so the user cannot tell how much too big it was:\n%s", want, got)
		}
	}
}

// A local model's context length is a setting the user controls, and saying so
// is the difference between "this is broken" and "I can fix this".
func TestContextOverflowMentionsLocalModelSetting(t *testing.T) {
	got := explainContextOverflow("exceeds the available context size")
	if !strings.Contains(got, "Ollama") || !strings.Contains(got, "LM Studio") {
		t.Errorf("explanation does not mention raising the context length on a local server:\n%s", got)
	}
}

// It must not fire on unrelated failures — a false positive here would tell
// someone to compact a conversation when their key was rejected.
func TestUnrelatedErrorsAreNotMistakenForOverflow(t *testing.T) {
	for _, e := range []string{
		"401 Unauthorized: invalid api key",
		"connection reset by peer",
		"model not found",
		"rate limit exceeded, please retry",
		"the server took the request and sent nothing back",
	} {
		if got := explainContextOverflow(e); got != "" {
			t.Errorf("%q was misread as a context overflow:\n%s", e, got)
		}
	}
}
