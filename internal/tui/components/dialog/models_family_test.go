package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/opencode-ai/opencode/internal/llm/models"
)

func fam(id, name string) models.Model {
	return models.Model{ID: models.ModelID(id), Provider: "antigravity", APIModel: id, Name: name}
}

func TestPickerChatGPTEffortUsesArrowsAndKeepsModelID(t *testing.T) {
	model := models.Model{
		ID: "chatgpt.fixture", APIModel: "fixture", Provider: models.ProviderChatGPT,
		DefaultReasoningEffort: "high",
		ReasoningEfforts:       []models.ReasoningEffort{{Effort: "high"}, {Effort: "xhigh"}},
	}
	m := &modelDialogCmp{models: []models.Model{model}, searchActive: true}
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := m.chatGPTEffort(model); got != "xhigh" || m.models[0].ID != model.ID {
		t.Fatalf("picked %q on %q", got, m.models[0].ID)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if got := m.chatGPTEffort(model); got != "high" {
		t.Fatalf("left arrow picked %q", got)
	}
}

func TestPickerOneRowPerFamilyAndEffortIsSentID(t *testing.T) {
	m := &modelDialogCmp{models: []models.Model{
		fam("claude-opus-5-5-low", "Claude Opus 5.5 (Low) (Antigravity)"),
		fam("claude-opus-5-5-high", "Claude Opus 5.5 (High) (Antigravity)"),
		fam("claude-opus-5-5-max", "Claude Opus 5.5 (Max) (Antigravity)"),
		fam("gemini-4-argon", "Gemini 4 Argon (Antigravity)"),
	}}
	m.collapseFamilies("")
	if len(m.models) != 2 {
		t.Fatalf("want 2 family rows, got %d", len(m.models))
	}
	if m.models[0].Name != "Claude Opus 5.5" || m.models[0].ID != "claude-opus-5-5-high" {
		t.Fatalf("default row = %q %q", m.models[0].Name, m.models[0].ID)
	}
	m.selectedIdx = 0
	m.cycleEffort(1)
	if m.models[0].ID != "claude-opus-5-5-max" {
		t.Fatalf("after ] got %q", m.models[0].ID)
	}
	m.cycleEffort(1) // at the top: stays
	m.cycleEffort(-1)
	m.cycleEffort(-1)
	if m.models[0].ID != "claude-opus-5-5-low" {
		t.Fatalf("after [[ got %q", m.models[0].ID)
	}
	if s := m.effortStrip(0); !strings.Contains(s, "[LOW]") {
		t.Fatalf("strip %q", s)
	}
	if s := m.effortStrip(1); s != "" {
		t.Fatalf("no-effort model shows a strip: %q", s)
	}
	if lines := m.effortSliderLines(1, 80); len(lines) != 0 {
		t.Fatalf("no-effort model shows a slider: %v", lines)
	}
}

func TestPickerStartsOnSavedEffort(t *testing.T) {
	m := &modelDialogCmp{models: []models.Model{
		fam("claude-opus-5-5-low", "Claude Opus 5.5 (Low)"),
		fam("claude-opus-5-5-high", "Claude Opus 5.5 (High)"),
	}}
	m.collapseFamilies("claude-opus-5-5-low")
	if m.models[0].ID != "claude-opus-5-5-low" {
		t.Fatalf("got %q", m.models[0].ID)
	}
}

func TestPickerShowsBackendNoticeWhicheverEffortIsHighlighted(t *testing.T) {
	a := fam("gpt-oss-120b-low", "GPT-OSS 120B (Low)")
	b := fam("gpt-oss-120b-medium", "GPT-OSS 120B (Medium)")
	b.Notice = "GPT-OSS will be removed from Antigravity on November 18, 2026."
	m := &modelDialogCmp{models: []models.Model{a, b}}
	m.collapseFamilies("")
	if n := m.noticeFor(0); !strings.Contains(n, "November 18") {
		t.Fatalf("notice %q", n)
	}
}

func TestEffortSliderLayoutAlignmentAndLiveAntigravityDescriptions(t *testing.T) {
	m := &modelDialogCmp{models: []models.Model{
		fam("gemini-4-neon-low", "Gemini 4 Neon (Low) (Antigravity)"),
		fam("gemini-4-neon-medium", "Gemini 4 Neon (Medium) (Antigravity)"),
		fam("gemini-4-neon-high", "Gemini 4 Neon (High) (Antigravity)"),
		fam("gemini-4-neon-xhigh", "Gemini 4 Neon (XHigh) (Antigravity)"),
	}}
	m.collapseFamilies("")
	m.selectedIdx = 0

	// Default is "high" (index 2 of 4).
	lines := m.effortSliderLines(0, 80)
	if len(lines) != 3 {
		t.Fatalf("want 3 slider rows (track, labels, explanation), got %d: %v", len(lines), lines)
	}
	row1 := ansi.Strip(lines[0])
	row2 := ansi.Strip(lines[1])
	row3 := ansi.Strip(lines[2])

	if !strings.HasPrefix(row1, "Effort") || !strings.Contains(row1, "◀") || !strings.Contains(row1, "▶") {
		t.Fatalf("row 1 missing Effort or directional arrows: %q", row1)
	}
	if strings.Count(row1, "●") != 2 || strings.Count(row1, "◎") != 1 || strings.Count(row1, "○") != 1 {
		t.Fatalf("high stop should show 2 filled, 1 active ringed, 1 hollow dot: %q", row1)
	}

	// Verify each stop label is centered directly beneath its circular dot.
	var dotCols []int
	for col, r := range []rune(row1) {
		if r == '●' || r == '◎' || r == '○' {
			dotCols = append(dotCols, col)
		}
	}
	wantLabels := []string{"low", "medium", "high", "xhigh"}
	if len(dotCols) != len(wantLabels) {
		t.Fatalf("dot count = %d, want %d in %q", len(dotCols), len(wantLabels), row1)
	}
	for idx, lbl := range wantLabels {
		pos := strings.Index(row2, lbl)
		if pos < 0 {
			t.Fatalf("label %q missing from row 2: %q", lbl, row2)
		}
		labelCenter := pos + len(lbl)/2
		diff := labelCenter - dotCols[idx]
		if diff < -1 || diff > 1 {
			t.Errorf("label %q centered at col %d, dot at col %d\nrow1: %q\nrow2: %q",
				lbl, labelCenter, dotCols[idx], row1, row2)
		}
	}
	if !strings.Contains(row3, "Deeper reasoning for complex problems") {
		t.Errorf("high description = %q", row3)
	}

	// Moving right to xhigh updates nodes and explanation live.
	m.cycleEffort(1)
	lines = m.effortSliderLines(0, 80)
	if !strings.Contains(ansi.Strip(lines[2]), "Extra-deep reasoning for demanding problems") {
		t.Errorf("xhigh description = %q", ansi.Strip(lines[2]))
	}
	// Moving down to medium and low updates explanation live.
	m.cycleEffort(-2)
	lines = m.effortSliderLines(0, 80)
	if !strings.Contains(ansi.Strip(lines[2]), "Balanced speed and reasoning quality") {
		t.Errorf("medium description = %q", ansi.Strip(lines[2]))
	}
	m.cycleEffort(-1)
	lines = m.effortSliderLines(0, 80)
	if !strings.Contains(ansi.Strip(lines[2]), "Faster responses, lighter reasoning") {
		t.Errorf("low description = %q", ansi.Strip(lines[2]))
	}
}

func TestEffortSliderDynamicColorsPerLevel(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
	t.Cleanup(func() { lipgloss.SetColorProfile(prevProfile) })

	model := models.Model{
		ID:                     "chatgpt.gpt-6.2-sol",
		Name:                   "GPT-6.2-Sol (ChatGPT sign-in)",
		Provider:               models.ProviderChatGPT,
		APIModel:               "gpt-6.2-sol",
		DefaultReasoningEffort: "medium",
		ReasoningEfforts: []models.ReasoningEffort{
			{Effort: "minimal"},
			{Effort: "low"},
			{Effort: "medium", Description: "Provider medium description"},
			{Effort: "high"},
			{Effort: "xhigh"},
			{Effort: "max"},
			{Effort: "ultra"},
		},
	}

	seenColors := map[string]string{}
	key, _, _ := models.FamilyOf(model)
	for _, opt := range model.ReasoningEfforts {
		m := &modelDialogCmp{
			models: []models.Model{model},
			effort: map[string]string{key: opt.Effort},
		}
		lines := m.effortSliderLines(0, 100)
		if len(lines) != 3 {
			t.Fatalf("%s: want 3 slider lines, got %d", opt.Effort, len(lines))
		}
		// Extract the exact TrueColor RGB sequence lipgloss emits for this level's AdaptiveColor.
		sample := lipgloss.NewStyle().Foreground(effortColor(opt.Effort)).Render("X")
		rgbIdx := strings.Index(sample, "38;2;")
		if rgbIdx < 0 {
			t.Fatalf("%s: expected TrueColor sequence in %q", opt.Effort, sample)
		}
		wantSeq, _, _ := strings.Cut(sample[rgbIdx:], "m")
		if prev, dup := seenColors[wantSeq]; dup {
			t.Errorf("%s shares color %s with %s", opt.Effort, wantSeq, prev)
		}
		seenColors[wantSeq] = opt.Effort

		if !strings.Contains(lines[0], wantSeq) {
			t.Errorf("%s: track row missing color %s: %q", opt.Effort, wantSeq, lines[0])
		}
		if !strings.Contains(lines[1], wantSeq) {
			t.Errorf("%s: label row missing color %s: %q", opt.Effort, wantSeq, lines[1])
		}
		if !hasSGR(lines[2], "3") {
			t.Errorf("%s: description row should be italic: %q", opt.Effort, lines[2])
		}
		if opt.Effort == "medium" && !strings.Contains(ansi.Strip(lines[2]), "Provider medium description") {
			t.Errorf("medium should prefer provider description, got %q", ansi.Strip(lines[2]))
		}
		if opt.Effort == "xhigh" && !strings.Contains(ansi.Strip(lines[2]), "Extra-deep reasoning") {
			t.Errorf("xhigh with empty provider description should fall back to Agy wording, got %q", ansi.Strip(lines[2]))
		}
		// Directional arrows highlight blue only when movable in that direction.
		const blueArrow = "38;2;96;165;250"
		leftChunk, rightChunk, _ := strings.Cut(lines[0], "◎")
		leftBlue := strings.Contains(leftChunk, blueArrow)
		rightBlue := strings.Contains(rightChunk, blueArrow)
		if opt.Effort == "minimal" && (leftBlue || !rightBlue) {
			t.Errorf("minimal boundary arrows wrong: leftBlue=%v rightBlue=%v", leftBlue, rightBlue)
		}
		if opt.Effort == "ultra" && (!leftBlue || rightBlue) {
			t.Errorf("ultra boundary arrows wrong: leftBlue=%v rightBlue=%v", leftBlue, rightBlue)
		}
		if opt.Effort == "high" && (!leftBlue || !rightBlue) {
			t.Errorf("high middle arrows should both be active: leftBlue=%v rightBlue=%v", leftBlue, rightBlue)
		}
	}
}

func TestCleanAntigravityFamilyNamesDropRedundantRawIDs(t *testing.T) {
	m := &modelDialogCmp{models: []models.Model{
		fam("gemini-4-hydrogen-high", "Gemini 4 Hydrogen (High) [gemini-4-hydrogen-high] (Antigravity)"),
		fam("gemini-4-hydrogen-low", "Gemini 4 Hydrogen (Low) [gemini-4-hydrogen-low] (Antigravity)"),
		fam("gemini-4-hydrogen-tiered-high", "Gemini 4 Hydrogen (High) [gemini-4-hydrogen-tiered-high] (Antigravity)"),
		fam("gemini-4-hydrogen-sonar", "Gemini 4 Hydrogen (Sonar) (Antigravity)"),
		fam("gemini-4-hydrogen-orchestrator", "Gemini 4 Hydrogen (Orchestrator) (Antigravity)"),
		fam("gemini-2.5-flash", "Gemini 3.5 Flash Lite [gemini-2.5-flash] (Antigravity)"),
		fam("gemini-2.5-flash-lite", "Gemini 3.5 Flash Lite [gemini-2.5-flash-lite] (Antigravity)"),
		fam("gemini-2.5-flash-thinking", "Gemini 3.5 Flash Lite [gemini-2.5-flash-thinking] (Antigravity)"),
		fam("gemini-3.5-flash-lite", "Gemini 3.5 Flash Lite [gemini-3.5-flash-lite] (Antigravity)"),
	}}
	m.collapseFamilies("")

	got := make([]string, 0, len(m.models))
	seen := map[string]bool{}
	for _, row := range m.models {
		if strings.Contains(row.Name, "[") {
			t.Errorf("row %q still carries a raw bracketed ID", row.Name)
		}
		if seen[row.Name] {
			t.Errorf("duplicate picker row name %q", row.Name)
		}
		seen[row.Name] = true
		got = append(got, row.Name)
	}
	want := []string{
		"Gemini 4 Hydrogen",
		"Gemini 4 Hydrogen (Tiered)",
		"Gemini 4 Hydrogen (Sonar)",
		"Gemini 4 Hydrogen (Orchestrator)",
		"Gemini 3.5 Flash Lite (2.5 Flash)",
		"Gemini 3.5 Flash Lite (2.5 Flash Lite)",
		"Gemini 3.5 Flash Lite (2.5 Flash Thinking)",
		"Gemini 3.5 Flash Lite",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEffortSliderAndPickerFitNarrowAndStandardTerminals(t *testing.T) {
	prevProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prevProfile) })

	chatGPTModel := models.Model{
		ID:                     "chatgpt.gpt-6.2-sol",
		Name:                   "GPT-6.2-Sol (ChatGPT sign-in)",
		Provider:               models.ProviderChatGPT,
		APIModel:               "gpt-6.2-sol",
		DefaultReasoningEffort: "medium",
		ReasoningEfforts: []models.ReasoningEffort{
			{Effort: "low"},
			{Effort: "medium", Description: "Balances speed and reasoning depth for everyday tasks"},
			{Effort: "high"},
			{Effort: "xhigh"},
			{Effort: "max"},
			{Effort: "ultra"},
		},
	}

	for _, sz := range []struct{ w, h int }{{40, 8}, {60, 20}, {80, 24}, {120, 40}} {
		m := &modelDialogCmp{
			provider: models.ProviderChatGPT,
			models:   []models.Model{chatGPTModel},
			width:    sz.w,
			height:   sz.h,
		}
		view := m.View()
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > sz.w {
				t.Errorf("%dx%d line %d is %d cells wide: %q", sz.w, sz.h, i+1, got, ansi.Strip(line))
			}
		}
		if sz.h >= standardHeight {
			if got := lipgloss.Height(view); got > sz.h {
				t.Errorf("%dx%d view is %d rows tall, overflows terminal", sz.w, sz.h, got)
			}
		}
		plain := ansi.Strip(view)
		if !strings.Contains(plain, "Search:") {
			t.Errorf("%dx%d view missing Search: input bar", sz.w, sz.h)
		}
		if !strings.Contains(plain, "Keyboard:") {
			t.Errorf("%dx%d view missing Keyboard: legend", sz.w, sz.h)
		}
		if !hasSGR(view, "4") {
			t.Errorf("%dx%d view missing underlined Search box", sz.w, sz.h)
		}
	}
}

func hasSGR(s, code string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			end := strings.IndexByte(s[i+2:], 'm')
			if end < 0 {
				break
			}
			parts := strings.Split(s[i+2:i+2+end], ";")
			for j := 0; j < len(parts); j++ {
				if parts[j] == "38" || parts[j] == "48" {
					if j+1 < len(parts) && parts[j+1] == "2" {
						j += 4
						continue
					}
					if j+1 < len(parts) && parts[j+1] == "5" {
						j += 2
						continue
					}
				}
				if parts[j] == code {
					return true
				}
			}
			i += 2 + end
		}
	}
	return false
}
