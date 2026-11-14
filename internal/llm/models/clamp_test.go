package models

import "testing"

func TestClampMaxOutputGPTOSS(t *testing.T) {
	m := Model{ID: "antigravity.gpt-oss-120b-medium", APIModel: "gpt-oss-120b-medium", DefaultMaxTokens: 262144}
	if got := ClampMaxOutput(m, 262144); got != 131072 {
		t.Fatalf("got %d, want 131072 (provider limit)", got)
	}
	if got := ClampMaxOutput(Model{APIModel: "other", DefaultMaxTokens: 8000}, 64000); got != 8000 {
		t.Fatalf("got %d, want the model's own 8000", got)
	}
	if got := ClampMaxOutput(m, 4096); got != 4096 {
		t.Fatalf("a lower request must pass: %d", got)
	}
}

func TestAntigravityBuildClampsGPTOSS(t *testing.T) {
	out := BuildAntigravityModels([]AntigravityRow{{ID: "gpt-oss-120b-medium", DisplayName: "GPT-OSS 120B", MaxTokens: 262144, MaxOutputTokens: 262144}})
	for _, m := range out {
		if m.DefaultMaxTokens > 131072 {
			t.Fatalf("DefaultMaxTokens %d above provider limit", m.DefaultMaxTokens)
		}
		return
	}
	t.Fatal("row not built (not Usable?)")
}
