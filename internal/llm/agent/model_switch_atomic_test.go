package agent

// GORILLA OVERRIDE (2026-08-20): a model switch must be ATOMIC.
//
// Update() used to call config.UpdateAgentModel FIRST and build the provider
// SECOND. When the build failed the error was returned - so it looked like a
// clean refusal - but the config had already moved. Everything that reads the
// config (the footer, the status bar, /usage) then showed the model the user
// had picked, while the live agent still held its previous provider and
// answered every message with the old model.
//
// Nothing in the interface disagreed with itself, so there was nothing to see.
// It was found by reading the SESSION DATABASE: the picker had been set to
// antigravity.claude-sonnet-4-6 sixteen seconds before the session began, and
// every assistant row said local.meta/llama-3.3-70b-instruct.
//
// The user's report was "I selected antigravity" and the program's answer was a
// different model's behaviour entirely - including tool calls on a two-word
// greeting, which is that model's documented habit (docs/TOOL-DISCIPLINE.md)
// and not something the configured model does.

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// A switch that cannot be honoured must leave the config exactly as it was.
//
// THE FAILURE MODE MATTERS, and my first draft of this test got it wrong: it
// used a model id that does not exist, which config.UpdateAgentModel rejects
// outright, so the config never moved and the test PASSED AGAINST THE BUG.
// Recorded here because that is the whole trap - a guard aimed at the wrong
// failure is indistinguishable from a guard that works.
//
// The real window is narrow: a model that config accepts (it exists, its
// provider is configured and has a key) but whose PROVIDER CANNOT BE
// CONSTRUCTED. That is what a fake provider id reproduces below, and it is the
// same shape as a live OAuth provider whose token will not refresh.
func TestFailedModelSwitchDoesNotMoveTheConfig(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	// A model config will happily accept, served by a provider no constructor
	// knows how to build.
	const fakeProvider = models.ModelProvider("provider-with-no-constructor")
	const fakeModel = models.ModelID("provider-with-no-constructor.some-model")
	models.SupportedModels[fakeModel] = models.Model{
		ID: fakeModel, Provider: fakeProvider, DefaultMaxTokens: 4096,
	}
	t.Cleanup(func() { delete(models.SupportedModels, fakeModel) })

	cfg := config.Get()
	if cfg.Agents == nil {
		cfg.Agents = map[config.AgentName]config.Agent{}
	}
	if cfg.Providers == nil {
		cfg.Providers = map[models.ModelProvider]config.Provider{}
	}
	// Configured and keyed, so validateAgent lets it through.
	//
	// cfg is a package global and SupportedModels is a package global, so both
	// are removed afterwards. This repository has lost time three times in one
	// day to exactly this: a test that seeds a global and leaves it there passes
	// alone and changes the answer for whatever runs next.
	cfg.Providers[fakeProvider] = config.Provider{APIKey: "present"}
	t.Cleanup(func() { delete(cfg.Providers, fakeProvider) })

	const startingModel = models.ModelID("starting.model.under.test")
	cfg.Agents[config.AgentCoder] = config.Agent{Model: startingModel, MaxTokens: 4096}

	a := &agent{agentName: config.AgentCoder}
	if _, err := a.Update(config.AgentCoder, fakeModel); err == nil {
		t.Fatal("Update reported success for a model whose provider cannot be built")
	}

	after := config.Get().Agents[config.AgentCoder].Model
	if after != startingModel {
		t.Errorf("a FAILED switch moved the configured model from %q to %q.\n"+
			"  This is the bug. The config advances, everything that reads it - footer,\n"+
			"  status bar, /usage - shows the NEW model, and the agent keeps its old\n"+
			"  provider. A different model then answers every message and nothing on\n"+
			"  screen disagrees with anything else on screen.\n"+
			"  Build the provider FIRST; commit the config only once it succeeds.",
			startingModel, after)
	}
}

// The provider builder must honour the model it is GIVEN, not the one in the
// config. Without this, "build first" is impossible: the only way to construct
// the new provider would be to write the config first - the exact ordering that
// caused the bug.
func TestProviderBuilderUsesTheModelItIsGivenNotTheConfiguredOne(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	if cfg.Agents == nil {
		cfg.Agents = map[config.AgentName]config.Agent{}
	}
	// The agent must EXIST, or the builder fails on "agent not found" and never
	// reaches the model lookup, which would make this assertion vacuous.
	cfg.Agents[config.AgentCoder] = config.Agent{Model: models.ModelID("some.other.model")}

	_, err := createAgentProviderFor(config.AgentCoder, models.ModelID("definitely.not.a.model"))
	if err == nil {
		t.Fatal("createAgentProviderFor ignored the model it was given")
	}
	if !strings.Contains(err.Error(), "definitely.not.a.model") {
		t.Errorf("error %q does not name the model that was requested;\n"+
			"  it appears to have reported on the CONFIGURED model instead", err)
	}
}
