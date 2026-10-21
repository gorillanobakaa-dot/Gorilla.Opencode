package provider

import (
	"encoding/json"
	"testing"

	"google.golang.org/genai"
)

// promptTokenCount includes the cached tokens and thoughtsTokenCount is not in
// candidatesTokenCount (genai v1.64.0 field docs), so a call with a 120k prompt
// of which 100k came from cache, 300 response tokens and 700 thinking tokens is
// 20k input, 100k cache read, 1,000 output.
func TestGeminiUsageSplitsCacheAndCountsThinking(t *testing.T) {
	want := TokenUsage{InputTokens: 20_000, CacheReadTokens: 100_000, OutputTokens: 1_000}

	g := &geminiClient{}
	got := g.usage(&genai.GenerateContentResponse{UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount: 120_000, CachedContentTokenCount: 100_000, CandidatesTokenCount: 300, ThoughtsTokenCount: 700}})
	if got != want {
		t.Errorf("Gemini API usage = %+v, want %+v", got, want)
	}

	var r caResponse
	body := `{"response":{"usageMetadata":{"promptTokenCount":120000,"cachedContentTokenCount":100000,` +
		`"candidatesTokenCount":300,"thoughtsTokenCount":700}}}`
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	if got := r.Response.UsageMetadata.usage(); got != want {
		t.Errorf("Code Assist / Antigravity usage = %+v, want %+v", got, want)
	}

	// No cache field at all (older replies): the whole prompt is input.
	if got := geminiUsage(5_000, 10, 0, 0); got != (TokenUsage{InputTokens: 5_000, OutputTokens: 10}) {
		t.Errorf("uncached usage = %+v", got)
	}
}
