// Version: 1.0.0 · updated 26-08-21-12-35
package dialog

import (
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

func mdl(id, api string, p models.ModelProvider) models.Model {
	return models.Model{ID: models.ModelID(id), Name: id, APIModel: api, Provider: p}
}

// The free route with the most usable allowance wins the row. Measured
// 2026-08-21: OpenRouter's free tier is 50 requests A DAY without a card, while
// NIM's limit is per MINUTE — so a row served by both must land on NIM.
func TestBestFreeRouteWinsTheRow(t *testing.T) {
	in := []models.Model{
		mdl("openrouter.openai/gpt-oss-120b:free", "openai/gpt-oss-120b", models.ProviderOpenRouter),
		mdl("local.openai/gpt-oss-120b", "openai/gpt-oss-120b", models.ProviderLocal),
		mdl("groq.openai/gpt-oss-120b", "openai/gpt-oss-120b", models.ProviderGROQ),
	}
	out := collapseDuplicates(in)
	if len(out) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(out), out)
	}
	if out[0].Provider != models.ProviderLocal {
		t.Errorf("row served by %s; the per-minute route should win over a 50-a-day one", out[0].Provider)
	}
	if !strings.Contains(out[0].Description, "also on") {
		t.Error("the alternatives vanished silently; a collapsed route the user cannot see is a model they cannot find")
	}
	for _, want := range []string{"groq", "openrouter"} {
		if !strings.Contains(out[0].Description, want) {
			t.Errorf("%s not named as an alternative: %q", want, out[0].Description)
		}
	}
}

// Different models must never collapse. A wrong merge HIDES a model, which is
// worse than showing a duplicate row.
func TestDifferentModelsAreNotCollapsed(t *testing.T) {
	in := []models.Model{
		mdl("groq.openai/gpt-oss-120b", "openai/gpt-oss-120b", models.ProviderGROQ),
		mdl("groq.openai/gpt-oss-20b", "openai/gpt-oss-20b", models.ProviderGROQ),
		mdl("cerebras.zai-glm-4.7", "zai-glm-4.7", models.ProviderCerebras),
	}
	if got := len(collapseDuplicates(in)); got != 3 {
		t.Errorf("got %d rows, want 3 — distinct models were merged", got)
	}
}

// Vendor prefixes and OpenRouter's routing suffixes are packaging, not identity:
// NVIDIA calls it meta/llama-3.3-70b-instruct, OpenRouter meta-llama/…:free.
func TestVendorPrefixAndFreeSuffixAreTheSameModel(t *testing.T) {
	in := []models.Model{
		mdl("openrouter.meta-llama/llama-3.3-70b-instruct:free", "meta-llama/llama-3.3-70b-instruct", models.ProviderOpenRouter),
		mdl("local.meta/llama-3.3-70b-instruct", "meta/llama-3.3-70b-instruct", models.ProviderLocal),
	}
	if got := len(collapseDuplicates(in)); got != 1 {
		t.Errorf("got %d rows, want 1 — the same model under two spellings", got)
	}
}

// A paid route never displaces a free one, whatever order they arrive in.
func TestPaidNeverWinsOverFree(t *testing.T) {
	in := []models.Model{
		mdl("anthropic.claude-sonnet-5", "claude-sonnet-5", models.ProviderAnthropic),
		mdl("antigravity.claude-sonnet-5", "claude-sonnet-5", models.ProviderAntigravity),
	}
	out := collapseDuplicates(in)
	if len(out) != 1 {
		t.Fatalf("got %d rows, want 1", len(out))
	}
	if out[0].Provider != models.ProviderAntigravity {
		t.Errorf("row served by %s; the free sign-in must win over a paid key", out[0].Provider)
	}
}

// The ranking must stay coarse and ordered the way the measurement says. If
// someone reorders this table, the reason it is ordered that way should stop them.
func TestQuotaRankKeepsPerMinuteAbovePerDay(t *testing.T) {
	if quotaRank(models.ProviderLocal) >= quotaRank(models.ProviderOpenRouter) {
		t.Error("OpenRouter's 50-requests-a-day free tier is ranked at or above a per-minute endpoint")
	}
	if quotaRank(models.ProviderAntigravity) >= quotaRank(models.ProviderAnthropic) {
		t.Error("a paid key outranks a free sign-in")
	}
	if quotaRank("some-provider-that-does-not-exist") <= quotaRank(models.ProviderOpenAI) {
		t.Error("an unknown provider outranks a known one")
	}
}
