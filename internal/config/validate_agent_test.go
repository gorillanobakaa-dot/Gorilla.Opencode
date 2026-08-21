package config

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// When validateAgent swaps an agent onto a fallback model, the max-tokens it
// leaves behind must be coherent with the FALLBACK model — never computed from
// the model that was just rejected.
//
// The checks after the swap are written against `model`, i.e. the rejected one.
// Falling through would clamp using the rejected model's context window. With
// today's numbers that is inert (the common rejects and the fallbacks all have
// 1M windows so the clamp never fires), which is exactly why this needs a test
// rather than an eyeball: the fault only appears once the two windows differ.
// fixtureWideModel is registered by the test itself — see the note above.
const fixtureWideModel models.ModelID = "xai.test-fixture-wide-window"

func TestValidateAgentFallbackMaxTokensMatchesFallbackModel(t *testing.T) {
	// The rejected model must have a LARGER context window than the fallback,
	// or the buggy clamp produces the same answer as the correct one and the
	// test cannot fail. The rejected model's window is 1047576 against the
	// Gemini fallback's 1000000, so a clamp computed from the rejected model
	// yields 523788 — above the fallback's 500000 half-window. Verified by
	// re-introducing the fall-through: this test fails, the Gemini-only
	// version passes.
	//
	// UPDATED 2026-08-21: the rejected model is now a FIXTURE this test
	// registers itself, not a real catalogue entry. It was models.GPT41, then
	// briefly an OpenRouter id — both of which are exactly the wrong thing to
	// build a fixture on, because provider catalogues are meant to change under
	// us now. What this test is about is the arithmetic (the clamp must be
	// computed from the FALLBACK model, not the rejected one), so it supplies
	// its own model with a known window and owns its own premise.
	for _, tc := range []struct {
		name      string
		rejected  models.ModelID
		maxTokens int64
	}{
		// UPDATED 2026-08-05: these two rejected a GEMINI model while setting
		// GEMINI_API_KEY so the GEMINI fallback would work — the same provider
		// disabled and available at once. That only passed because validateAgent
		// ignored the environment when a config entry existed, which was itself
		// the bug (a provider shown "(ready)" by the portal was reverted by the
		// validator). With the environment now honoured, the fixture has to pick
		// a rejected provider that is genuinely unusable.
		{"modest max-tokens", fixtureWideModel, 5_000},
		{"the user's configured 50k", fixtureWideModel, 50_000},
		{"wider rejected window, max-tokens above the fallback's half", fixtureWideModel, 600_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if _, err := Load(dir, false); err != nil {
				t.Fatalf("Load: %v", err)
			}
			c := Get()

			// A wider window than the Gemini fallback's 1000000, so a clamp
			// computed from the rejected model (523788) lands ABOVE the
			// fallback's half-window (500000) and the bug is visible.
			models.SupportedModels[fixtureWideModel] = models.Model{
				ID: fixtureWideModel, Name: "fixture", Provider: models.ProviderXAI,
				APIModel: "fixture", ContextWindow: 1_047_576, DefaultMaxTokens: 32_000,
			}
			t.Cleanup(func() { delete(models.SupportedModels, fixtureWideModel) })

			rejected := tc.rejected
			rm, ok := models.SupportedModels[rejected]
			if !ok {
				t.Fatalf("%s missing from SupportedModels", rejected)
			}

			prevProviders := c.Providers[rm.Provider]
			prevAgent := c.Agents[AgentCoder]
			t.Cleanup(func() {
				c.Providers[rm.Provider] = prevProviders
				c.Agents[AgentCoder] = prevAgent
			})

			// Make the configured model's provider unusable, and ensure a
			// fallback is reachable.
			c.Providers[rm.Provider] = Provider{APIKey: "", Disabled: true}
			// Genuinely unusable: no config key AND no environment key. Without
			// this the environment would rescue it, which is now correct
			// behaviour and would leave nothing to fall back FROM.
			t.Setenv("XAI_API_KEY", "")
			t.Setenv("GEMINI_API_KEY", "test-key")
			c.Agents[AgentCoder] = Agent{Model: rejected, MaxTokens: tc.maxTokens}

			if err := validateAgent(c, AgentCoder, c.Agents[AgentCoder]); err != nil {
				t.Fatalf("validateAgent: %v", err)
			}

			got := c.Agents[AgentCoder]
			if got.Model == rejected {
				t.Fatalf("agent kept the unusable model %s", rejected)
			}
			fm, ok := models.SupportedModels[got.Model]
			if !ok {
				t.Fatalf("fallback model %s is not in SupportedModels", got.Model)
			}
			if got.MaxTokens <= 0 {
				t.Errorf("fallback left max-tokens at %d", got.MaxTokens)
			}
			if fm.ContextWindow > 0 && got.MaxTokens > fm.ContextWindow/2 {
				t.Errorf("max-tokens %d exceeds half of the FALLBACK model %s context window (%d) — it was computed from the rejected model",
					got.MaxTokens, got.Model, fm.ContextWindow/2)
			}
		})
	}
}

// GORILLA OVERRIDE: an OAuth provider (Antigravity, Gemini Code Assist) counts as
// "configured" only once it appears in cfg.Providers. The provider portal signs
// in AFTER config.Load, so within that session the entry must be added
// explicitly — UpsertProviderKey with the oauth-login placeholder — BEFORE the
// agent model is set. Skip it and validateAgent silently reverts every agent
// onto Gemini (revertAgentToDefault returns nil), so the freshly-chosen Claude
// model never takes effect. That is exactly what v0.1.65 shipped: sign in to
// Antigravity, and every agent fell back to gemini-flash-latest.
func TestOAuthProviderKeepsAgentModelWhenRegistered(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := Get()
	want := models.AGClaudeSonnet46
	if _, ok := models.SupportedModels[want]; !ok {
		t.Fatalf("%s not registered", want)
	}
	// A reachable fallback, so an (expected) revert does not hard-error.
	t.Setenv("GEMINI_API_KEY", "test-key")

	// NEGATIVE — reproduces the bug: with no provider entry, the model reverts.
	// If this ever stops reverting, the positive assertion below proves nothing.
	delete(c.Providers, models.ProviderAntigravity)
	c.Agents[AgentCoder] = Agent{Model: want, MaxTokens: 5000}
	if err := validateAgent(c, AgentCoder, c.Agents[AgentCoder]); err != nil {
		t.Fatalf("validateAgent (unregistered): %v", err)
	}
	if c.Agents[AgentCoder].Model == want {
		t.Fatal("unregistered provider unexpectedly kept the model — bug not reproduced; the fix assertion would be vacuous")
	}

	// POSITIVE — the fix: register the provider, then the chosen model sticks.
	if err := UpsertProviderKey(models.ProviderAntigravity, "oauth-login"); err != nil {
		t.Fatalf("UpsertProviderKey: %v", err)
	}
	c.Agents[AgentCoder] = Agent{Model: want, MaxTokens: 5000}
	if err := validateAgent(c, AgentCoder, c.Agents[AgentCoder]); err != nil {
		t.Fatalf("validateAgent (registered): %v", err)
	}
	if got := c.Agents[AgentCoder].Model; got != want {
		t.Fatalf("registered provider still reverted the model: got %s, want %s", got, want)
	}
}

// A retired model id must be migrated forward, not treated as unknown and
// dropped onto an unrelated default.
func TestValidateAgentMigratesLegacyModelIDs(t *testing.T) {
	if len(models.LegacyModelIDs) == 0 {
		t.Skip("no legacy ids registered")
	}
	for legacy, current := range models.LegacyModelIDs {
		t.Run(string(legacy), func(t *testing.T) {
			target, ok := models.SupportedModels[current]
			if !ok {
				t.Fatalf("legacy id %s maps to %s, which is not a registered model", legacy, current)
			}

			dir := t.TempDir()
			if _, err := Load(dir, false); err != nil {
				t.Fatalf("Load: %v", err)
			}
			c := Get()

			prevProviders := c.Providers[target.Provider]
			prevAgent := c.Agents[AgentCoder]
			t.Cleanup(func() {
				c.Providers[target.Provider] = prevProviders
				c.Agents[AgentCoder] = prevAgent
			})

			// Give the target model's provider a key so no fallback kicks in
			// and we observe the migration in isolation.
			c.Providers[target.Provider] = Provider{APIKey: "test-key"}
			c.Agents[AgentCoder] = Agent{Model: legacy, MaxTokens: 5000}

			if err := validateAgent(c, AgentCoder, c.Agents[AgentCoder]); err != nil {
				t.Fatalf("validateAgent: %v", err)
			}
			if got := c.Agents[AgentCoder].Model; got != current {
				t.Errorf("legacy id %s became %s, want %s", legacy, got, current)
			}
		})
	}
}
