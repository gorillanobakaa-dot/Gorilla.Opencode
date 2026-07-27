package dialog

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

// SwitchToProvider is the primitive that turns a UseProviderMsg into "picker
// opens on the right tab". If it silently drops the requested provider, /connect
// u appears to work but you land on the wrong tab. Pin the contract.
func TestSwitchToProviderLandsOnTheRequestedTab(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prev := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prev })
	cfg.Providers = map[models.ModelProvider]config.Provider{
		models.ProviderAnthropic: {APIKey: "sk-anthropic"},
		models.ProviderGROQ:      {APIKey: "sk-groq"},
		models.ProviderCerebras:  {APIKey: "sk-cerebras"},
	}
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"GEMINI_API_KEY", "GROQ_API_KEY", "CEREBRAS_API_KEY",
		"OPENROUTER_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}

	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderGROQ)

	if m.provider != models.ProviderGROQ {
		t.Errorf("provider = %q, want %q — SwitchToProvider did not land on the requested tab", m.provider, models.ProviderGROQ)
	}
	if m.hScrollOffset < 0 || m.hScrollOffset >= len(m.availableProviders) {
		t.Errorf("hScrollOffset %d out of range [0,%d)", m.hScrollOffset, len(m.availableProviders))
	}
	if m.availableProviders[m.hScrollOffset] != models.ProviderGROQ {
		t.Errorf("column at offset %d is %q, want %q",
			m.hScrollOffset, m.availableProviders[m.hScrollOffset], models.ProviderGROQ)
	}
}

// A provider that is not enabled must not crash the picker. It should open on
// the default column and let the user choose from what IS available.
func TestSwitchToProviderFallsBackWhenProviderNotEnabled(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prev := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prev })
	cfg.Providers = map[models.ModelProvider]config.Provider{
		models.ProviderAnthropic: {APIKey: "sk-anthropic"},
	}
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"GEMINI_API_KEY", "GROQ_API_KEY", "CEREBRAS_API_KEY",
		"OPENROUTER_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}

	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderCerebras) // not enabled

	if m.provider == "" {
		t.Error("provider was left blank when requested provider was unavailable")
	}
	// Should land on the sole available provider.
	if len(m.availableProviders) == 1 && m.provider != m.availableProviders[0] {
		t.Errorf("with one available provider, fallback should land there; got %q vs %q",
			m.provider, m.availableProviders[0])
	}
}

// Zero enabled providers must not panic. The picker will be empty but the
// caller (typically UseProviderMsg) should still be able to invoke it.
func TestSwitchToProviderWithNoProvidersDoesNotPanic(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	prev := cfg.Providers
	t.Cleanup(func() { cfg.Providers = prev })
	cfg.Providers = map[models.ModelProvider]config.Provider{}
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"GEMINI_API_KEY", "GROQ_API_KEY", "CEREBRAS_API_KEY",
		"OPENROUTER_API_KEY", "XAI_API_KEY"} {
		t.Setenv(k, "")
	}
	m := &modelDialogCmp{}
	m.SwitchToProvider(models.ProviderGROQ) // must not panic
}
