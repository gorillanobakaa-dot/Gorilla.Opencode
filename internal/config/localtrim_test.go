package config

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA (P22): deferred tool loading switches itself on for a coder model on
// this machine, and only then; each case below flips one condition.
func withCoderModel(t *testing.T, id models.ModelID, baseURL string) {
	t.Helper()
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	ResetLoadout()
	models.SupportedModels[id] = models.Model{ID: id, Name: "t", Provider: models.ProviderLocal, APIModel: "t",
		ContextWindow: 8192, DefaultMaxTokens: 2048}
	models.RegisterLocalRouteForTest(id, baseURL, "")
	c := Get()
	prev := c.Agents[AgentCoder]
	a := prev
	a.Model = id
	c.Agents[AgentCoder] = a
	t.Cleanup(func() {
		c.Agents[AgentCoder] = prev
		delete(models.SupportedModels, id)
		models.ClearLocalRouteForTest(id)
		ResetLoadout()
	})
}

func TestLocalModelGetsDeferredToolsByDefault(t *testing.T) {
	withCoderModel(t, "local.p22-a", "http://127.0.0.1:1234/v1")
	if !LoadoutEnabled(ToolSearchComponentID) {
		t.Fatal("a model on this machine should get deferred tool loading")
	}
}

func TestRemoteEndpointModelIsNotTrimmed(t *testing.T) {
	withCoderModel(t, "remote.p22-b", "https://integrate.api.nvidia.com/v1")
	if LoadoutEnabled(ToolSearchComponentID) {
		t.Fatal("a remote endpoint must keep the shipped default (off)")
	}
}

func TestLocalTrimEnvironmentOptOut(t *testing.T) {
	withCoderModel(t, "local.p22-c", "http://127.0.0.1:1234/v1")
	t.Setenv(LocalTrimDisabledEnv, "0")
	if LoadoutEnabled(ToolSearchComponentID) {
		t.Fatal("GORILLA_LOCAL_TRIM=0 must switch the trim off")
	}
}

func TestPressingTheKeyOptsOutAndAgainOptsBackIn(t *testing.T) {
	withCoderModel(t, "local.p22-d", "http://127.0.0.1:1234/v1")
	ToggleLoadout(ToolSearchComponentID) // shown on: pressing means off
	if LoadoutEnabled(ToolSearchComponentID) {
		t.Fatal("the key must switch the automatic trim off")
	}
	ToggleLoadout(ToolSearchComponentID)
	if !LoadoutEnabled(ToolSearchComponentID) {
		t.Fatal("pressed again it must hand the row back to the automatic rule")
	}
}

func TestOtherRowsAreUntouchedByTheTrim(t *testing.T) {
	withCoderModel(t, "local.p22-e", "http://127.0.0.1:1234/v1")
	for _, c := range LoadoutComponents {
		if c.ID != ToolSearchComponentID && LoadoutEnabled(c.ID) != c.Default {
			t.Errorf("%s changed from its default", c.ID)
		}
	}
}
