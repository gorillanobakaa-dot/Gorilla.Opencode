package dialog

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

func fam(id, name string) models.Model {
	return models.Model{ID: models.ModelID(id), Provider: "antigravity", APIModel: id, Name: name}
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
