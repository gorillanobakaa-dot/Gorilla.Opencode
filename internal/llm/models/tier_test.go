package models

// GORILLA OVERRIDE (2026-11-21): regression tests for Gemini 4 Neon ranking
// and Newer vs Legacy model tier classification.

import (
	"sort"
	"testing"
)

func TestGemini4NeonRanksAboveLegacyModels(t *testing.T) {
	rows := append(rowsOf20261005(),
		AntigravityRow{ID: "gemini-4-neon-high", DisplayName: "Gemini 4 Neon (High)", MaxTokens: 1048576, MaxOutputTokens: 65536, SupportsThinking: true},
		AntigravityRow{ID: "gemini-4-neon-xhigh", DisplayName: "Gemini 4 Neon (XHigh)", MaxTokens: 1048576, MaxOutputTokens: 65536, SupportsThinking: true},
	)
	built := BuildAntigravityModels(rows)
	order := make([]Model, 0, len(built))
	for _, m := range built {
		order = append(order, m)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].Rank < order[j].Rank })
	pos := func(api string) int {
		for i, m := range order {
			if m.APIModel == api {
				return i
			}
		}
		t.Fatalf("%s missing", api)
		return -1
	}

	if pos("gemini-4-neon-xhigh") >= pos("gemini-4-neon-high") {
		t.Fatalf("Neon XHigh should rank ahead of Neon High")
	}
	for _, older := range []string{"gemini-3.1-pro-high", "gemini-2.5-pro", "gpt-oss-120b-medium", "gemini-3.5-flash-lite"} {
		if pos("gemini-4-neon-high") >= pos(older) {
			t.Fatalf("Gemini 4 Neon ranked at %d, behind older %s at %d", pos("gemini-4-neon-high"), older, pos(older))
		}
	}
}

func TestModelTierInfoClassifiesNewerAndLegacyWithDescriptors(t *testing.T) {
	neon := ModelTierInfo(Model{ID: "antigravity.gemini-4-neon-high", APIModel: "gemini-4-neon-high", Name: "Gemini 4 Neon", Provider: ProviderAntigravity})
	if neon.IsLegacy || neon.Vendor != "Gemini" || neon.LaunchYM == "" || neon.Accent != AccentCyan {
		t.Fatalf("unexpected Neon tier: %+v", neon)
	}

	opus := ModelTierInfo(Model{ID: "antigravity.claude-opus-5-5-high", APIModel: "claude-opus-5-5-high", Name: "Claude Opus 5.5", Provider: ProviderAntigravity})
	if opus.IsLegacy || opus.Vendor != "Anthropic" || opus.LaunchYM == "" || opus.Accent != AccentRed {
		t.Fatalf("unexpected Opus 5.5 tier: %+v", opus)
	}

	legacy31 := ModelTierInfo(Model{ID: "antigravity.gemini-3.1-pro-high", APIModel: "gemini-3.1-pro-high", Name: "Gemini 3.1 Pro", Provider: ProviderAntigravity})
	if !legacy31.IsLegacy || legacy31.Descriptor == "" {
		t.Fatalf("Gemini 3.1 Pro should be legacy with descriptor, got %+v", legacy31)
	}

	legacy25 := ModelTierInfo(Model{ID: "antigravity.gemini-2.5-pro", APIModel: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: ProviderAntigravity})
	if !legacy25.IsLegacy || legacy25.Descriptor == "" {
		t.Fatalf("Gemini 2.5 Pro should be legacy with descriptor, got %+v", legacy25)
	}
}
