package tui

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/util"
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

// A session on a paid key opens with a red reminder that names the model and
// what the first message can cost (owner, 2026-10-28).
func TestPaidKeySessionOpensWithARedLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.Load(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	id := models.ModelID("test-anthropic-start")
	models.SupportedModels[id] = models.Model{ID: id, Name: "Claude Sonnet 5", Provider: models.ProviderAnthropic, APIModel: "claude-sonnet-5"}
	t.Cleanup(func() { delete(models.SupportedModels, id) })
	coder := cfg.Agents[config.AgentCoder]
	coder.Model = id
	cfg.Agents[config.AgentCoder] = coder

	q, ok := antigravityUsageCmd(true)().(quotaLineMsg)
	if !ok {
		t.Fatal("no start line on a paid key")
	}
	if q.kind != util.InfoTypeError || !strings.Contains(q.line, "your card pays") || !strings.Contains(q.line, "Claude Sonnet 5: up to $0.055") {
		t.Errorf("start line = %q (kind %v)", q.line, q.kind)
	}
}
