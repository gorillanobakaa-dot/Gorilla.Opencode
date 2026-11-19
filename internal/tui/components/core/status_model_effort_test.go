package core

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

func TestStatusBarShowsActiveChatGPTEffort(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	const id = models.ModelID("chatgpt.gpt-6.2-sol-status-test")
	models.SupportedModels[id] = models.Model{
		ID:                     id,
		Name:                   "GPT-6.2-Sol (ChatGPT sign-in)",
		Provider:               models.ProviderChatGPT,
		APIModel:               "gpt-6.2-sol",
		DefaultReasoningEffort: "medium",
		ReasoningEfforts: []models.ReasoningEffort{
			{Effort: "low"},
			{Effort: "medium"},
			{Effort: "high"},
			{Effort: "xhigh"},
		},
	}
	t.Cleanup(func() { delete(models.SupportedModels, id) })

	cfg := config.Get()
	prev := cfg.Agents[config.AgentCoder]
	t.Cleanup(func() { cfg.Agents[config.AgentCoder] = prev })

	cfg.Agents[config.AgentCoder] = config.Agent{Model: id, ReasoningEffort: ""}
	m := statusCmp{width: 140}
	if got := ansi.Strip(m.model()); !strings.Contains(got, "GPT-6.2-Sol (Medium) (ChatGPT sign-in)") {
		t.Errorf("default ChatGPT status chip = %q, want (Medium)", got)
	}

	cfg.Agents[config.AgentCoder] = config.Agent{Model: id, ReasoningEffort: "xhigh"}
	if got := ansi.Strip(m.model()); !strings.Contains(got, "GPT-6.2-Sol (XHigh) (ChatGPT sign-in)") {
		t.Errorf("xhigh ChatGPT status chip = %q, want (XHigh)", got)
	}
}
