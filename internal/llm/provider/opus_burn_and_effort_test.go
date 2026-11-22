package provider

// GORILLA FIX (2026-11-21): regression tests for the two bugs driving Opus 5.5
// High credit burn and free-tier thinking-effort parity:
//   1. `thought: true` parts in Antigravity / Code Assist responses must NOT be
//      concatenated into visible assistant text (which was saved into history
//      and re-sent as input on every subsequent tool step).
//   2. Free-tier Google Code Assist envelopes must carry thinkingConfig matching
//      the selected reasoning effort.

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/message"
)

func TestCollectPartsDropsThoughtChunksFromVisibleText(t *testing.T) {
	parts := []caPart{
		{Text: "LONG INTERNAL THINKING CHAIN THAT MUST NOT BE RE-SENT", Thought: true},
		{Text: "Final answer."},
	}
	text, calls := collectParts(parts)
	if len(calls) != 0 {
		t.Fatalf("expected 0 calls, got %d", len(calls))
	}
	if text != "Final answer." {
		t.Fatalf("collectParts returned %q, want %q (thought text leaked into visible content)", text, "Final answer.")
	}
}

func TestCodeAssistEnvelopeSetsThinkingBudgetFromEffort(t *testing.T) {
	m := models.Model{
		ID:                     models.GeminiCAPro,
		Provider:               models.ProviderGeminiCA,
		APIModel:               "gemini-3-pro-preview",
		CanReason:              true,
		DefaultReasoningEffort: "medium",
		ReasoningEfforts: []models.ReasoningEffort{
			{Effort: "low"}, {Effort: "medium"}, {Effort: "high"},
		},
	}
	msgs := []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}}}

	cases := map[string]int{"low": 2048, "medium": 8192, "high": 24576}
	for effort, wantBudget := range cases {
		c := &codeAssistClient{
			providerOptions: providerClientOptions{
				model:           m,
				maxTokens:       16384,
				reasoningEffort: effort,
			},
		}
		env := c.buildEnvelope(msgs, nil)
		tc, ok := env.Request.GenerationConfig["thinkingConfig"].(map[string]any)
		if !ok {
			t.Fatalf("effort %q: missing thinkingConfig in envelope", effort)
		}
		if got, _ := tc["thinkingBudget"].(int); got != wantBudget {
			t.Fatalf("effort %q: thinkingBudget = %d, want %d", effort, got, wantBudget)
		}
	}
}
