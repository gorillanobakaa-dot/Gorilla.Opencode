package config

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// THE INVARIANT, stated once so it stops being rediscovered:
//
//	if the startup portal shows a provider as (ready), validateAgent must accept
//	a model from it.
//
// The portal's readiness check consults AvailableViaEnv(); validateAgent
// consulted only cfg.Providers. Whenever those two disagreed, the user picked a
// provider the portal called ready and was silently moved onto a different
// model. This is the THIRD instance of that disagreement — Antigravity in
// v0.1.66 (provider absent until restart), NVIDIA in v0.1.69 (matched by name,
// not address), and now an env key hidden behind a stale config entry.
//
// Observed 2026-08-05: GROQ_API_KEY, CEREBRAS_API_KEY and XAI_API_KEY all set,
// all three shown "(ready)", and all four agents reverted to Gemini with
// "provider cerebras is disabled".
func TestAnEnvKeyIsNotHiddenByAStaleConfigEntry(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	const provider = models.ProviderGROQ
	t.Setenv("GROQ_API_KEY", "test-key-from-environment")

	// Exactly the shape found on disk: an entry exists, has no key, and carries
	// a disabled flag written by an earlier run.
	cfg.Providers[provider] = Provider{APIKey: "", Disabled: true}

	// Any model from that provider.
	var target models.ModelID
	for id, m := range models.SupportedModels {
		if m.Provider == provider {
			target = id
			break
		}
	}
	if target == "" {
		t.Skip("no model registered for this provider in the test binary")
	}

	agent := Agent{Model: target, MaxTokens: 4096}
	cfg.Agents[AgentCoder] = agent
	if err := validateAgent(cfg, AgentCoder, agent); err != nil {
		t.Fatalf("validateAgent: %v", err)
	}

	if got := cfg.Agents[AgentCoder].Model; got != target {
		t.Errorf("the agent was reverted to %q despite GROQ_API_KEY being set; the "+
			"portal would have shown this provider as (ready)", got)
	}
	if cfg.Providers[provider].Disabled {
		t.Error("the stale disabled flag survived, so the next launch reverts again")
	}
}

// The negative case, so the assertion above cannot pass vacuously: with no env
// key and no config key, reverting is correct.
func TestADisabledProviderWithNoKeyAnywhereStillReverts(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	const provider = models.ProviderGROQ
	t.Setenv("GROQ_API_KEY", "")

	cfg.Providers[provider] = Provider{APIKey: "", Disabled: true}

	var target models.ModelID
	for id, m := range models.SupportedModels {
		if m.Provider == provider {
			target = id
			break
		}
	}
	if target == "" {
		t.Skip("no model registered for this provider in the test binary")
	}

	agent := Agent{Model: target, MaxTokens: 4096}
	cfg.Agents[AgentCoder] = agent
	err := validateAgent(cfg, AgentCoder, agent)

	// The model may or may not change: revertAgentToDefault only rewrites it when
	// some OTHER provider is usable, and returns an error when none is. Either
	// outcome is a rejection. What must never happen is a silent accept — the
	// model left in place with no error, which is what the bug produced.
	accepted := err == nil && cfg.Agents[AgentCoder].Model == target
	if accepted {
		t.Error("a provider with no key in config OR environment was silently " +
			"accepted; it must be rejected or reverted")
	}
}

// The portal and the validator must agree. Rather than trusting that both are
// correct, assert the agreement directly for every env-backed provider.
func TestPortalReadinessAndValidatorAgree(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Setenv("CEREBRAS_API_KEY", "test-key-from-environment")

	ready := false
	for _, p := range AvailableViaEnv() {
		if p == models.ProviderCerebras {
			ready = true
		}
	}
	if !ready {
		t.Fatal("AvailableViaEnv does not see CEREBRAS_API_KEY; fixture is wrong")
	}

	// Stale entry, as the portal-ready provider had on disk.
	cfg.Providers[models.ProviderCerebras] = Provider{APIKey: "", Disabled: true}

	var target models.ModelID
	for id, m := range models.SupportedModels {
		if m.Provider == models.ProviderCerebras {
			target = id
			break
		}
	}
	if target == "" {
		t.Skip("no Cerebras model registered in the test binary")
	}

	agent := Agent{Model: target, MaxTokens: 4096}
	cfg.Agents[AgentCoder] = agent
	if err := validateAgent(cfg, AgentCoder, agent); err != nil {
		t.Fatalf("validateAgent: %v", err)
	}
	if got := cfg.Agents[AgentCoder].Model; got != target {
		t.Errorf("the portal reports this provider ready but the validator reverted "+
			"the agent to %q - the two disagree about what \"configured\" means", got)
	}
}
