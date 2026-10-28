package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// requestJSON builds the Messages request for one user turn and returns it as
// the JSON that goes on the wire.
func requestJSON(t *testing.T, apiModel, text string) string {
	t.Helper()
	c := &anthropicClient{
		providerOptions: providerClientOptions{model: models.Model{APIModel: apiModel}, maxTokens: 8000},
		options:         anthropicOptions{shouldThink: DefaultShouldThinkFn},
	}
	msgs := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(text))}
	b, err := json.Marshal(c.preparedMessages(msgs, nil))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The Air's v0.1.156 failed every turn with "`temperature` is deprecated for
// this model" (2026-10-28). No request may carry it.
func TestAnthropicRequestHasNoTemperature(t *testing.T) {
	for _, m := range []string{"claude-opus-5-5", "claude-fable-5-1", "claude-sonnet-5", "claude-haiku-4-5"} {
		for _, text := range []string{"comms check", "think about it"} {
			if j := requestJSON(t, m, text); strings.Contains(j, "temperature") {
				t.Errorf("%s %q: request carries temperature: %s", m, text, j)
			}
		}
	}
}

func TestAnthropicThinkingShapePerModel(t *testing.T) {
	if j := requestJSON(t, "claude-opus-5-5", "think about it"); !strings.Contains(j, `"thinking":{"type":"adaptive"}`) || strings.Contains(j, "budget_tokens") {
		t.Errorf("current model must get adaptive thinking, got %s", j)
	}
	if j := requestJSON(t, "claude-haiku-4-5", "think about it"); !strings.Contains(j, "budget_tokens") {
		t.Errorf("Haiku 4.5 must keep budget thinking, got %s", j)
	}
	if j := requestJSON(t, "claude-opus-5-5", "comms check"); strings.Contains(j, "thinking") {
		t.Errorf("no thinking asked, none must be sent, got %s", j)
	}
}
