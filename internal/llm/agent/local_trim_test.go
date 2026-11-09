package agent

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
)

// GORILLA (P22): what the local trim saves on the first request, counted the
// way /context counts (bytes/4 for the prompt, toolTokens for each schema sent).
func TestLocalTrimShrinksTheFirstRequest(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	config.ResetLoadout()
	const id models.ModelID = "local.p22-measure"
	models.SupportedModels[id] = models.Model{ID: id, Name: "t", Provider: models.ProviderLocal, APIModel: "t",
		ContextWindow: 32768, DefaultMaxTokens: 2048}
	models.RegisterLocalRouteForTest(id, "http://127.0.0.1:1234/v1", "")
	c := config.Get()
	prev := c.Agents[config.AgentCoder]
	a := prev
	a.Model = id
	c.Agents[config.AgentCoder] = a
	t.Cleanup(func() {
		c.Agents[config.AgentCoder] = prev
		delete(models.SupportedModels, id)
		models.ClearLocalRouteForTest(id)
		config.ResetLoadout()
	})

	measure := func() (tools, promptTokens int) {
		all := CoderAgentTools(nil, nil, nil, nil, nil)
		ag := &agent{tools: all}
		for _, tl := range ag.visibleTools("p22-session") {
			tools += toolTokens(tl)
		}
		return tools, len(prompt.GetAgentPrompt(config.AgentCoder, models.ProviderLocal)) / 4
	}

	t.Setenv(config.LocalTrimDisabledEnv, "0")
	beforeTools, beforePrompt := measure()
	t.Setenv(config.LocalTrimDisabledEnv, "")
	afterTools, afterPrompt := measure()

	t.Logf("BEFORE (GORILLA_LOCAL_TRIM=0): tool schemas %d + prompt %d = %d tokens on the first request",
		beforeTools, beforePrompt, beforeTools+beforePrompt)
	t.Logf("AFTER  (local trim):           tool schemas %d + prompt %d = %d tokens on the first request",
		afterTools, afterPrompt, afterTools+afterPrompt)
	if afterTools+afterPrompt >= beforeTools+beforePrompt {
		t.Fatalf("the trim must shrink the first request: %d -> %d", beforeTools+beforePrompt, afterTools+afterPrompt)
	}
}
