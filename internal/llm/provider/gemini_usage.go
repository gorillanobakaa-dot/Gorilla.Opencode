package provider

// geminiUsage turns Gemini's usageMetadata into a TokenUsage.
//
// GORILLA FIX (2026-10-21): the three Gemini-shaped clients (Gemini API, Code
// Assist, Antigravity) passed promptTokenCount as InputTokens and
// candidatesTokenCount as OutputTokens. The genai SDK documents both fields
// (google.golang.org/genai v1.64.0 types.go):
//
//   - promptTokenCount: "When cached_content is set, this is still the total
//     effective prompt size meaning this includes the number of tokens in the
//     cached content." The Gemini API client ALSO reported the cached count as
//     CacheReadTokens, so every cached token was counted twice: once at the
//     full input price and once at the cache price, and the context gauge
//     (input + cache reads) read high by the cached amount.
//   - thoughtsTokenCount is separate from candidatesTokenCount, and the price
//     page bills output "including thinking tokens". Leaving it out made a
//     thinking model's output, and its cost, look smaller than it was.
//
// Input is therefore the prompt minus the cached part, and output is the
// response plus the thinking. TokenUsage keeps OpenCode's (and Anthropic's)
// meaning: InputTokens never includes cache reads.
func geminiUsage(prompt, candidates, cached, thoughts int64) TokenUsage {
	input := prompt - cached
	if input < 0 {
		input = 0
	}
	return TokenUsage{
		InputTokens:     input,
		OutputTokens:    candidates + thoughts,
		CacheReadTokens: cached,
	}
}
