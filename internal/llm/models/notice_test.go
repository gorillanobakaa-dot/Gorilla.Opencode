package models

import (
	"strings"
	"testing"
)

// The backend's own notice (wire field tagDescription, measured 2026-11-15)
// reaches the catalogue entry; a row without one has none.
func TestBackendNoticeReachesCatalogue(t *testing.T) {
	rows := []AntigravityRow{
		{ID: "gpt-oss-120b-medium", DisplayName: "GPT-OSS 120B (Medium)", APIProvider: "API_PROVIDER_OPENAI_VERTEX", MaxTokens: 1000, MaxOutputTokens: 100,
			Notice: "GPT-OSS will be removed from Antigravity on November 18, 2026."},
		{ID: "claude-opus-5-5-high", DisplayName: "Claude Opus 5.5 (High)", APIProvider: "API_PROVIDER_ANTHROPIC_VERTEX", MaxTokens: 1000, MaxOutputTokens: 100},
	}
	got := BuildAntigravityModels(rows)
	var withNotice, without int
	for _, m := range got {
		if strings.Contains(m.Notice, "November 18, 2026") {
			withNotice++
		} else if m.Notice == "" {
			without++
		}
	}
	if withNotice != 1 || without != 1 {
		t.Fatalf("notices: with=%d without=%d (%v)", withNotice, without, got)
	}
}
