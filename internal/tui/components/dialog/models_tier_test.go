package dialog

// GORILLA OVERRIDE (2026-11-21): verifies the model picker renders "Newer
// models", vendor dividers (Anthropic vs Gemini), launch year-month tags,
// and the "Legacy models" divider with capability descriptors.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

func TestModelPickerRendersNewerAndLegacySectionsWithVendorDividers(t *testing.T) {
	m := &modelDialogCmp{
		provider: models.ProviderAntigravity,
		width:    100,
		height:   40,
		models: []models.Model{
			{ID: "antigravity.claude-opus-5-5-high", APIModel: "claude-opus-5-5-high", Name: "Claude Opus 5.5", Provider: models.ProviderAntigravity, Rank: 1},
			{ID: "antigravity.gemini-4-neon-high", APIModel: "gemini-4-neon-high", Name: "Gemini 4 Neon", Provider: models.ProviderAntigravity, Rank: 2},
			{ID: "antigravity.gemini-3.1-pro-high", APIModel: "gemini-3.1-pro-high", Name: "Gemini 3.1 Pro", Provider: models.ProviderAntigravity, Rank: 3},
			{ID: "antigravity.gemini-2.5-pro", APIModel: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: models.ProviderAntigravity, Rank: 4},
		},
	}

	plain := ansi.Strip(m.View())
	for _, want := range []string{"Newer models", "[2026-11]", "Gemini", "Legacy models", "legacy 3.1 pro tier", "legacy 2.5 pro tier"} {
		if !strings.Contains(plain, want) {
			t.Errorf("model picker View() missing %q:\n%s", want, plain)
		}
	}
	if strings.Index(plain, "Gemini 4 Neon") > strings.Index(plain, "Legacy models") {
		t.Errorf("Gemini 4 Neon rendered below Legacy models divider:\n%s", plain)
	}
}
