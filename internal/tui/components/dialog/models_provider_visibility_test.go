package dialog

import (
	"slices"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// The /model picker's provider tabs are built by getEnabledProviders. Before
// this change it only iterated cfg.Providers, so a provider whose API key was
// present via env var but never saved via /connect was invisible — the user's
// exported GROQ_API_KEY did nothing until they walked through the /connect
// save form. This test pins the union behaviour.
func TestProviderPickerShowsEnvVarProviders(t *testing.T) {
	// Isolate: never touch the real config on disk.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()

	// Start from a clean slate — some providers may be present from the env
	// file loaded by config.Load. Preserve and restore.
	prevProviders := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prevProviders })
	cfg.Providers = map[models.ModelProvider]config.Provider{
		// One provider saved to config, one saved-but-disabled.
		models.ProviderAnthropic: {APIKey: "sk-saved-anthropic", Disabled: false},
		models.ProviderOpenAI:    {APIKey: "sk-saved-openai", Disabled: true},
	}

	// Two providers visible only via env var: Groq is new, OpenAI is disabled
	// in config (should NOT be resurrected).
	t.Setenv("GROQ_API_KEY", "gsk-env-only")
	t.Setenv("OPENAI_API_KEY", "sk-env-would-resurrect")
	// Clear the others so the assertion set is exact.
	for _, k := range []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY",
		"CEREBRAS_API_KEY", "OPENROUTER_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}

	got := getEnabledProviders(cfg)

	if !slices.Contains(got, models.ProviderAnthropic) {
		t.Errorf("saved-and-enabled Anthropic missing from picker: %v", got)
	}
	if !slices.Contains(got, models.ProviderGROQ) {
		t.Errorf("env-var-only Groq missing from picker — the fix regressed: %v", got)
	}
	if slices.Contains(got, models.ProviderOpenAI) {
		t.Errorf("explicitly-disabled OpenAI surfaced via env var — user's disable was overridden: %v", got)
	}
}

// Config-only providers must still work. Regression guard against a naive
// "only look at env" rewrite.
func TestProviderPickerStillShowsConfigOnlyProviders(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prevProviders := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prevProviders })
	cfg.Providers = map[models.ModelProvider]config.Provider{
		models.ProviderCerebras: {APIKey: "sk-saved", Disabled: false},
	}
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"GEMINI_API_KEY", "GROQ_API_KEY", "CEREBRAS_API_KEY",
		"OPENROUTER_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}
	got := getEnabledProviders(cfg)
	if !slices.Contains(got, models.ProviderCerebras) {
		t.Errorf("config-only Cerebras missing: %v", got)
	}
}

// AvailableViaEnv is the primitive the picker leans on. Its contract must be
// tight: only surface a provider whose env var is actually set (non-empty).
func TestAvailableViaEnvBoundaries(t *testing.T) {
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"GEMINI_API_KEY", "GROQ_API_KEY", "CEREBRAS_API_KEY",
		"OPENROUTER_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}
	if got := config.AvailableViaEnv(); len(got) != 0 {
		t.Errorf("no env keys set, got %v, want empty", got)
	}

	t.Setenv("GROQ_API_KEY", "gsk-1")
	t.Setenv("XAI_API_KEY", "xai-1")
	got := config.AvailableViaEnv()
	if !slices.Contains(got, models.ProviderGROQ) {
		t.Errorf("Groq missing: %v", got)
	}
	if !slices.Contains(got, models.ProviderXAI) {
		t.Errorf("xAI missing: %v", got)
	}
	if slices.Contains(got, models.ProviderAnthropic) {
		t.Errorf("Anthropic surfaced despite empty env var: %v", got)
	}
}

// The bug this pins: with an NVIDIA NIM key added through /connect, 102 models
// registered successfully and NONE of them were selectable. ProviderLocal is
// configured as a localEndpoints entry with its own per-endpoint key, so it is
// in neither cfg.Providers nor the *_API_KEY environment scan — the two sources
// getEnabledProviders drew from. From the user's chair that is identical to the
// key being rejected: re-enter it, restart, still only Google models.
func TestProviderPickerShowsLocalEndpointsWithNoProviderEntry(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()

	prevProviders := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prevProviders })
	// Deliberately EMPTY: a local endpoint never writes a cfg.Providers entry.
	cfg.Providers = map[models.ModelProvider]config.Provider{}
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY",
		"GROQ_API_KEY", "CEREBRAS_API_KEY", "OPENROUTER_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}

	const id = models.ModelID("local.test-nim-model")
	models.RegisterLocalRouteForTest(id, "https://integrate.api.nvidia.com/v1", "nvapi-x")
	t.Cleanup(func() { models.ClearLocalRouteForTest(id) })

	got := getEnabledProviders(cfg)
	if !slices.Contains(got, models.ProviderLocal) {
		t.Fatalf("local is absent from the picker (%v) while %d local model(s) are registered — every discovered NVIDIA/Ollama/LM Studio model is unreachable",
			got, 1)
	}
	// And it must lead: a hand-configured endpoint is what the user is hunting for.
	if got[0] != models.ProviderLocal {
		t.Errorf("local is listed at position %d, want first; providers = %v",
			slices.Index(got, models.ProviderLocal), got)
	}
}

// A local endpoint the user explicitly disabled must stay hidden.
func TestDisabledLocalProviderStaysHidden(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prevProviders := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prevProviders })
	cfg.Providers = map[models.ModelProvider]config.Provider{
		models.ProviderLocal: {Disabled: true},
	}

	const id = models.ModelID("local.another-test-model")
	models.RegisterLocalRouteForTest(id, "http://localhost:11434/v1", "")
	t.Cleanup(func() { models.ClearLocalRouteForTest(id) })

	if got := getEnabledProviders(cfg); slices.Contains(got, models.ProviderLocal) {
		t.Errorf("a disabled local provider was resurrected: %v", got)
	}
}
