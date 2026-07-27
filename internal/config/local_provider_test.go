package config

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// A model served by a configured local endpoint is reachable by definition:
// models.RegisterLocalEndpoint only registers it after successfully listing it
// from that endpoint, and LocalRouteFor carries its baseURL and key.
//
// But ProviderLocal has no cfg.Providers entry and no *_API_KEY, so
// getProviderAPIKey returned "" and validateAgent judged every local model
// "not configured" — silently swapping the agent onto a cloud model. With
// Ollama running and gemma3:270m registered, three agents were reverted on
// EVERY startup and the user saw three "is unusable" notes for a working setup.
func TestLocalModelWithARouteIsNotReverted(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := Get()

	// Register a fake local model with a route, the way a live endpoint would.
	const id models.ModelID = "local.test-model"
	prevSupported, hadSupported := models.SupportedModels[id]
	models.SupportedModels[id] = models.Model{
		ID:               id,
		Name:             "test local model",
		Provider:         models.ProviderLocal,
		APIModel:         "test-model",
		ContextWindow:    8192,
		DefaultMaxTokens: 2048,
	}
	models.RegisterLocalRouteForTest(id, "http://localhost:11434/v1", "")
	t.Cleanup(func() {
		if hadSupported {
			models.SupportedModels[id] = prevSupported
		} else {
			delete(models.SupportedModels, id)
		}
		models.ClearLocalRouteForTest(id)
	})

	prevAgent := c.Agents[AgentSummarizer]
	prevProviders := c.Providers
	t.Cleanup(func() { c.Agents[AgentSummarizer] = prevAgent; c.Providers = prevProviders })

	// No cfg.Providers["local"] entry — exactly the real situation.
	c.Providers = map[models.ModelProvider]Provider{}
	c.Agents[AgentSummarizer] = Agent{Model: id, MaxTokens: 2048}

	if err := validateAgent(c, AgentSummarizer, c.Agents[AgentSummarizer]); err != nil {
		t.Fatalf("validateAgent: %v", err)
	}

	if got := c.Agents[AgentSummarizer].Model; got != id {
		t.Errorf("agent was reverted from %q to %q — a routed local model must be treated as configured", id, got)
	}
}

// A local model with NO route is genuinely unreachable (endpoint down at startup,
// model pulled from Ollama since) and must still fall back, with the note the
// user sees. Otherwise we would trade a false alarm for a silent failure.
func TestLocalModelWithoutARouteStillReverts(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := Get()

	const id models.ModelID = "local.no-route-model"
	models.SupportedModels[id] = models.Model{
		ID: id, Name: "unrouted", Provider: models.ProviderLocal,
		ContextWindow: 8192, DefaultMaxTokens: 2048,
	}
	t.Cleanup(func() { delete(models.SupportedModels, id) })

	prevAgent := c.Agents[AgentSummarizer]
	prevProviders := c.Providers
	t.Cleanup(func() { c.Agents[AgentSummarizer] = prevAgent; c.Providers = prevProviders })

	c.Providers = map[models.ModelProvider]Provider{}
	c.Agents[AgentSummarizer] = Agent{Model: id, MaxTokens: 2048}
	t.Setenv("GEMINI_API_KEY", "test-key") // give the fallback somewhere to land

	if err := validateAgent(c, AgentSummarizer, c.Agents[AgentSummarizer]); err != nil {
		t.Fatalf("validateAgent: %v", err)
	}
	if got := c.Agents[AgentSummarizer].Model; got == id {
		t.Error("an unrouted local model was kept — a genuinely unreachable endpoint must still fall back")
	}
}

// The max-tokens clamp must still apply on the local path. Extracting
// validateAgentMaxTokens so the local branch could reuse it would be pointless
// if the branch skipped it.
func TestLocalPathStillClampsMaxTokens(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := Get()

	const id models.ModelID = "local.clamp-model"
	models.SupportedModels[id] = models.Model{
		ID: id, Name: "clamp", Provider: models.ProviderLocal,
		ContextWindow: 8192, DefaultMaxTokens: 2048,
	}
	models.RegisterLocalRouteForTest(id, "http://localhost:11434/v1", "")
	t.Cleanup(func() {
		delete(models.SupportedModels, id)
		models.ClearLocalRouteForTest(id)
	})

	prevAgent := c.Agents[AgentSummarizer]
	t.Cleanup(func() { c.Agents[AgentSummarizer] = prevAgent })

	// Absurd max-tokens: more than half the 8192 context window.
	c.Agents[AgentSummarizer] = Agent{Model: id, MaxTokens: 99999}
	if err := validateAgent(c, AgentSummarizer, c.Agents[AgentSummarizer]); err != nil {
		t.Fatalf("validateAgent: %v", err)
	}
	got := c.Agents[AgentSummarizer].MaxTokens
	if got > 8192/2 {
		t.Errorf("max-tokens = %d, want <= %d — the clamp was skipped on the local path", got, 8192/2)
	}
}
