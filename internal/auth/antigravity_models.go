package auth

// GORILLA OVERRIDE: ask the Antigravity backend what it actually serves.
//
// THE BUG THIS EXISTS TO PREVENT
//   internal/llm/models/antigravity.go was five models typed out by hand from
//   `agy models` (client version 1.1.10). Measured 2026-08-14, the backend was
//   serving twenty usable ones. Gemini 3.7 had shipped and was unreachable: not
//   because anything was broken, but because nobody had retyped the list.
//
//   This is the same decay refresh.go was written for — a hand-maintained
//   mirror of someone else's catalogue does not break when upstream moves, it
//   quietly stops being true.
//
// WHY WE DO NOT COPY `agy models`
//   Because it is wrong, and wrong in the way that costs a debugging session.
//   Measured 2026-08-14 against daily-cloudcode-pa:
//
//     gemini-3.7-flash-high     agy prints it   -> backend: NOT_FOUND
//     gemini-3.7-flash-medium   agy prints it   -> backend: NOT_FOUND
//     gemini-3.7-flash-low      agy prints it   -> backend: NOT_FOUND
//     gemini-3.7-flash-tiered   agy hides it    -> backend: ACCEPTED
//
//   agy expands one "-tiered" entry into three tier names for its own display.
//   Those names are not model ids. Shipping them would have put three Gemini
//   3.7 entries in the picker that 404 the moment they are selected. The ids in
//   the fetchAvailableModels RESPONSE are the only ones the backend honours.
//
// ENDPOINT
//   POST https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels
//   with an EMPTY JSON body. It rejects "metadata" and "cloudaicompanionProject"
//   with 400 Unknown name — unlike loadCodeAssist, which requires metadata.

import (
	"context"
	"fmt"
)

// AntigravityModelInfo is one entry of the fetchAvailableModels response.
// Field names are the measured wire shape, not a guess.
type AntigravityModelInfo struct {
	DisplayName       string `json:"displayName"`
	APIProvider       string `json:"apiProvider"`
	ModelProvider     string `json:"modelProvider"`
	MaxTokens         int64  `json:"maxTokens"`
	MaxOutputTokens   int64  `json:"maxOutputTokens"`
	SupportsImages    bool   `json:"supportsImages"`
	SupportsThinking  bool   `json:"supportsThinking"`
	SupportsVideo     bool   `json:"supportsVideo"`
	ThinkingBudget    int64  `json:"thinkingBudget"`
	MinThinkingBudget int64  `json:"minThinkingBudget"`
	IsInternal        bool   `json:"isInternal"`
	Recommended       bool   `json:"recommended"`
}

type fetchAvailableModelsResp struct {
	Models map[string]AntigravityModelInfo `json:"models"`
}

// FetchAvailableModels asks the backend for the live catalogue. The returned
// map is keyed by the id the backend accepts as the envelope's "model" field —
// which is the whole point of asking it rather than a client.
func (c *AntigravityCreds) FetchAvailableModels(ctx context.Context) (map[string]AntigravityModelInfo, error) {
	token, err := c.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	var resp fetchAvailableModelsResp
	// Empty body, deliberately: see the endpoint note above.
	if err := c.callAntigravity(ctx, token, "fetchAvailableModels", map[string]any{}, &resp); err != nil {
		return nil, fmt.Errorf("fetchAvailableModels: %w", err)
	}
	if len(resp.Models) == 0 {
		return nil, fmt.Errorf("backend returned no models")
	}
	return resp.Models, nil
}
