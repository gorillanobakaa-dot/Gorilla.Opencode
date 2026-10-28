package tui

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// On the MacBook Air (2026-10-28) /usage in an Anthropic session said only
// "Antigravity is not the active provider". It must name Anthropic instead.
func TestUsageNamesAnthropicWhenItIsActive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	cfg, err := config.Load(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	id := models.ModelID("test-anthropic-usage")
	models.SupportedModels[id] = models.Model{ID: id, Provider: models.ProviderAnthropic, APIModel: "claude-opus-5-5"}
	t.Cleanup(func() { delete(models.SupportedModels, id) })
	coder := cfg.Agents[config.AgentCoder]
	coder.Model = id
	cfg.Agents[config.AgentCoder] = coder

	m := antigravityUsageCmd(false)()
	q, ok := m.(quotaLineMsg)
	if !ok {
		t.Fatalf("got %T, want quotaLineMsg", m)
	}
	if q.line != anthropicUsageLine {
		t.Errorf("headline = %q, want the Anthropic line", q.line)
	}
}
