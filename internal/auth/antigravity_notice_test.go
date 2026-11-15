package auth

import (
	"encoding/json"
	"testing"
)

// Shape captured from the live fetchAvailableModels on 2026-11-15.
func TestWireNoticeFields(t *testing.T) {
	var resp fetchAvailableModelsResp
	raw := `{"models":{
	 "gpt-oss-120b-medium":{"displayName":"GPT-OSS 120B (Medium)","tagTitle":"Notice","tagDescription":"GPT-OSS will be removed from Antigravity on November 18, 2026."},
	 "claude-opus-5-5-high":{"displayName":"Claude Opus 5.5 (High)","tagTitle":"New"}}}`
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if got := resp.Models["gpt-oss-120b-medium"].Notice(); got != "GPT-OSS will be removed from Antigravity on November 18, 2026." {
		t.Fatalf("notice %q", got)
	}
	if got := resp.Models["claude-opus-5-5-high"].Notice(); got != "" {
		t.Fatalf("a bare 'New' tag is not a notice: %q", got)
	}
}
