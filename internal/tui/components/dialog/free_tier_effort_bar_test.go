package dialog

// GORILLA FIX (2026-11-21): verifies free-tier accounts retain the Thinking
// Effort bar (both multi-stop ReasoningEfforts on free Google Code Assist and
// single-effort Antigravity free-tier variants).

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

func TestFreeTierGeminiCodeAssistShowsInteractiveEffortBar(t *testing.T) {
	m := models.GeminiCAModels[models.GeminiCAPro]
	if len(m.ReasoningEfforts) < 3 {
		t.Fatalf("GeminiCAPro should expose low/medium/high ReasoningEfforts, got %+v", m.ReasoningEfforts)
	}
	dlg := &modelDialogCmp{
		provider: models.ProviderGeminiCA,
		models:   []models.Model{m},
		width:    90,
		height:   30,
	}
	lines := dlg.effortSliderLines(0, 80)
	if len(lines) != 3 {
		t.Fatalf("free-tier Gemini Code Assist missing 3-row effort slider, got %d lines: %v", len(lines), lines)
	}
	row1 := ansi.Strip(lines[0])
	if !strings.HasPrefix(row1, "Effort") || !strings.Contains(row1, "◀") || !strings.Contains(row1, "▶") {
		t.Fatalf("unexpected effort slider row: %q", row1)
	}

	// Arrow keys move effort from default medium -> high -> low.
	dlg.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := dlg.chatGPTEffort(m); got != "high" {
		t.Fatalf("after Right arrow got %q, want high", got)
	}
	dlg.Update(tea.KeyMsg{Type: tea.KeyLeft})
	dlg.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if got := dlg.chatGPTEffort(m); got != "low" {
		t.Fatalf("after two Left arrows got %q, want low", got)
	}
}

func TestSingleEffortAntigravityFreeVariantStillShowsEffortBar(t *testing.T) {
	dlg := &modelDialogCmp{
		provider: models.ProviderAntigravity,
		models: []models.Model{
			fam("gemini-3.6-flash-medium", "Gemini 3.6 Flash (Medium) (Antigravity)"),
		},
	}
	dlg.collapseFamilies("")
	lines := dlg.effortSliderLines(0, 80)
	if len(lines) == 0 {
		t.Fatal("single-effort free-tier variant hid the Effort bar completely")
	}
	if !strings.Contains(ansi.Strip(lines[0]), "medium") {
		t.Fatalf("single-effort bar did not display effort level: %q", ansi.Strip(lines[0]))
	}
}
