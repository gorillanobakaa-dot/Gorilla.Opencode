package models

import "testing"

// Free and paid accounts of the same provider must offer the same effort bar,
// and a bar must never be offered where no client would honour it.
func TestEveryHonouredReasoningModelHasEffortBar(t *testing.T) {
	for id, m := range SupportedModels {
		if !m.CanReason {
			continue
		}
		switch m.Provider {
		case ProviderGemini, ProviderGeminiCA, ProviderOpenRouter:
			if len(m.EffortOptions()) == 0 {
				t.Errorf("%s (%s) reasons but offers no effort bar", id, m.Provider)
			}
		}
	}
}

func TestEffortOptionsDefaultsAndResolve(t *testing.T) {
	m := Model{Provider: ProviderGemini, CanReason: true}
	if got := m.ResolveReasoningEffort(""); got != "medium" {
		t.Fatalf("default effort = %q, want medium", got)
	}
	if !m.SupportsReasoningEffort("high") {
		t.Fatal("high should be offered")
	}
	adaptive := Model{Provider: ProviderAnthropic, CanReason: true, APIModel: "claude-opus-5-5"}
	if len(adaptive.EffortOptions()) != 0 {
		t.Fatal("adaptive Anthropic models have no budget to scale; no bar")
	}
	legacy := Model{Provider: ProviderAnthropic, CanReason: true, APIModel: "claude-sonnet-4-5"}
	if len(legacy.EffortOptions()) != 3 {
		t.Fatal("budget-thinking Claude should offer three stops")
	}
	plain := Model{Provider: ProviderGemini}
	if len(plain.EffortOptions()) != 0 {
		t.Fatal("non-reasoning model must not get a bar")
	}
}
