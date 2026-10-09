package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE (2026-10-09): the agent tool's plan and coder roles are
// told what they are by GetAgentPrompt, keyed on the agent name. An unmapped
// name falls to "You are a helpful assistant", which for a helper that edits
// files would be a coder with no rules at all — and nothing would fail.
func TestRolePromptsAreMappedAndCarryProjectContext(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	oldDir, oldPaths := cfg.WorkingDir, cfg.ContextPaths
	t.Cleanup(func() {
		cfg.WorkingDir, cfg.ContextPaths = oldDir, oldPaths
		InvalidateContextCache()
	})

	dir := t.TempDir()
	const marker = "ROLE-PROMPT-PROJECT-RULE-7731"
	if err := os.WriteFile(filepath.Join(dir, "RULES.md"), []byte(marker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.WorkingDir = dir
	cfg.ContextPaths = []string{"RULES.md"}
	InvalidateContextCache()

	p := models.ProviderAnthropic

	plan := GetAgentPrompt(config.AgentPlan, p)
	if !strings.Contains(plan, BasePlanPrompt()) {
		t.Error("the plan role does not get the planning prompt")
	}
	for _, want := range []string{"numbered", "never write", "verify"} {
		if !strings.Contains(plan, want) {
			t.Errorf("the planning prompt does not say %q", want)
		}
	}

	sub := GetAgentPrompt(config.AgentSubCoder, p)
	if !strings.Contains(sub, BaseCoderPrompt(p)) {
		t.Error("the coder role does not get the coder prompt")
	}
	if !strings.Contains(sub, SubCoderNote) {
		t.Error("the coder role does not get the sub-agent note")
	}
	for _, want := range []string{"every file you changed", "every command you ran"} {
		if !strings.Contains(SubCoderNote, want) {
			t.Errorf("the sub-agent note does not ask for %q", want)
		}
	}

	for name, got := range map[config.AgentName]string{config.AgentPlan: plan, config.AgentSubCoder: sub} {
		if strings.HasPrefix(got, "You are a helpful assistant") {
			t.Errorf("%s fell through to the default prompt", name)
		}
		if !strings.Contains(got, marker) {
			t.Errorf("%s does not carry the project's own instructions", name)
		}
	}
	// Non-vacuous: the planner must not have been handed the coder prompt.
	if strings.Contains(plan, SubCoderNote) || strings.Contains(plan, BaseCoderPrompt(p)) {
		t.Error("the plan role was given the coder prompt")
	}
}
