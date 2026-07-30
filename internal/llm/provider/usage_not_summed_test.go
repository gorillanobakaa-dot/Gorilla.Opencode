package provider

import (
	"testing"

	"github.com/openai/openai-go"
)

// The accumulator sums usage across chunks. This proves it, so nobody "fixes"
// the workaround by going back to acc.ChatCompletion.Usage.
func TestAccumulatorSumsUsageAcrossChunksAndMustNotBeUsedForIt(t *testing.T) {
	acc := openai.ChatCompletionAccumulator{}
	const realPromptTokens = 2400
	const chunks = 200

	for i := 0; i < chunks; i++ {
		acc.AddChunk(openai.ChatCompletionChunk{
			Usage: openai.CompletionUsage{PromptTokens: realPromptTokens},
		})
	}

	got := acc.ChatCompletion.Usage.PromptTokens
	if got == realPromptTokens {
		t.Skip("openai-go no longer sums usage; the workaround can be removed")
	}
	if got != realPromptTokens*chunks {
		t.Fatalf("unexpected accumulation shape: got %d", got)
	}
	t.Logf("accumulator reported %d for a %d-token prompt — inflated %dx",
		got, realPromptTokens, got/realPromptTokens)
}

// A prompt-token count is a property of the request: identical no matter how
// many chunks carry it. Overwriting is the only correct treatment.
func TestLastNonZeroUsageWinsRegardlessOfChunkCount(t *testing.T) {
	const realPromptTokens = 2400

	for _, chunks := range []int{1, 10, 500} {
		var streamUsage openai.CompletionUsage
		for i := 0; i < chunks; i++ {
			chunk := openai.ChatCompletionChunk{}
			// Mirror the real loop: most chunks carry no usage at all.
			if i == chunks-1 || i%3 == 0 {
				chunk.Usage = openai.CompletionUsage{
					PromptTokens:     realPromptTokens,
					CompletionTokens: int64(i + 1),
				}
			}
			if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
				streamUsage = chunk.Usage
			}
		}
		if streamUsage.PromptTokens != realPromptTokens {
			t.Errorf("chunks=%d: prompt tokens became %d, want %d — the count "+
				"must not scale with the length of the answer",
				chunks, streamUsage.PromptTokens, realPromptTokens)
		}
	}
}
