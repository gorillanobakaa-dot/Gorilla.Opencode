package config

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// setAgents forces a known starting state without going through the setters,
// so a test failure points at FollowCoderModel rather than at the fixture.
func setAgents(t *testing.T, coder, summarizer, task, title models.ModelID) {
	t.Helper()
	if cfg == nil {
		if _, err := Load(t.TempDir(), false); err != nil {
			t.Fatalf("Load: %v", err)
		}
	}
	// Local models are registered at runtime from the provider's catalogue, so a
	// test binary has none. Register the ids this test uses, or UpdateAgentModel
	// rejects them as "not supported".
	for _, id := range []models.ModelID{coder, summarizer, task, title} {
		registerModel(t, id)
	}
	for name, id := range map[AgentName]models.ModelID{
		AgentCoder: coder, AgentSummarizer: summarizer, AgentTask: task, AgentTitle: title,
	} {
		a := cfg.Agents[name]
		a.Model = id
		cfg.Agents[name] = a
	}
}

// registerModel makes an id acceptable to UpdateAgentModel's validation.
func registerModel(t *testing.T, id models.ModelID) {
	t.Helper()
	if _, ok := models.SupportedModels[id]; ok || id == "" {
		return
	}
	models.SupportedModels[id] = models.Model{
		ID: id, Provider: models.ProviderLocal,
		ContextWindow: 8192, DefaultMaxTokens: 4096,
	}
	// A local model also needs a registered ROUTE, or UpdateAgentModel rejects
	// it with "no valid provider available".
	models.RegisterLocalRouteForTestNamed(id, "http://127.0.0.1:1/v1", "k", "test-endpoint")
	t.Cleanup(func() {
		delete(models.SupportedModels, id)
		models.ClearLocalRouteForTest(id)
	})
}

func modelOf(t *testing.T, name AgentName) models.ModelID {
	t.Helper()
	return cfg.Agents[name].Model
}

// THE BUG: /models changed only the coder, so the helpers stayed on the old
// model. Observed 2026-08-05 with coder on diffusiongemma while summarizer,
// task and title were all still on 01-ai/yi-large — a model the account cannot
// run. The only visible symptom was a recurring "failed to generate title";
// summarisation and sub-agents were primed to fail silently later.
func TestHelpersFollowTheCoderWhenTheyWereShadowingIt(t *testing.T) {
	const old = models.ModelID("local.01-ai/yi-large")
	const new_ = models.ModelID("local.meta/llama-3.1-8b-instruct")
	setAgents(t, old, old, old, old)
	registerModel(t, new_)

	moved, err := FollowCoderModel(old, new_)
	if err != nil {
		t.Fatalf("FollowCoderModel: %v", err)
	}
	if moved != 3 {
		t.Errorf("moved %d helpers, want 3", moved)
	}
	for _, name := range []AgentName{AgentSummarizer, AgentTask, AgentTitle} {
		if got := modelOf(t, name); got != new_ {
			t.Errorf("%s stayed on %q; it was shadowing the coder and is now stranded", name, got)
		}
	}
}

// A helper deliberately set to something else must NOT be dragged along — a
// cheap fast model for titles is a legitimate, common choice.
func TestADeliberatelyDifferentHelperIsLeftAlone(t *testing.T) {
	const old = models.ModelID("local.01-ai/yi-large")
	const new_ = models.ModelID("local.meta/llama-3.3-70b-instruct")
	const cheapTitle = models.ModelID("local.meta/llama-3.1-8b-instruct")
	setAgents(t, old, old, old, cheapTitle)
	registerModel(t, new_)

	moved, err := FollowCoderModel(old, new_)
	if err != nil {
		t.Fatalf("FollowCoderModel: %v", err)
	}
	if moved != 2 {
		t.Errorf("moved %d helpers, want 2 (title was set deliberately)", moved)
	}
	if got := modelOf(t, AgentTitle); got != cheapTitle {
		t.Errorf("title was overwritten with %q, discarding a deliberate choice", got)
	}
}

// No previous model, or no actual change, must be a no-op rather than a
// blanket overwrite.
func TestNoOpCases(t *testing.T) {
	const same = models.ModelID("local.meta/llama-3.3-70b-instruct")
	const other = models.ModelID("local.01-ai/yi-large")
	setAgents(t, same, other, other, other)

	if moved, err := FollowCoderModel(same, same); err != nil || moved != 0 {
		t.Errorf("same model in and out: moved=%d err=%v, want 0/nil", moved, err)
	}
	if moved, err := FollowCoderModel("", same); err != nil || moved != 0 {
		t.Errorf("empty previous model: moved=%d err=%v, want 0/nil", moved, err)
	}
	if got := modelOf(t, AgentSummarizer); got != other {
		t.Errorf("a no-op case still modified the summarizer: %q", got)
	}
}
