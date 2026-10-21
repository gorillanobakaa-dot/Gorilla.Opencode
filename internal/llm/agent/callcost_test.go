package agent

import (
	"math"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/provider"
	"github.com/opencode-ai/opencode/internal/session"
)

var oct21 = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// A sourced table wins over the catalogue: the catalogue numbers below are
// deliberately wrong, and the result must not use them.
func TestCallCostUsesTheSourcedTable(t *testing.T) {
	m := models.Model{ID: "x", Provider: "anthropic", APIModel: "claude-opus-5-5",
		CostPer1MIn: 99, CostPer1MOut: 99, CostPer1MInCached: 99, CostPer1MOutCached: 99}
	// 1M in x4 + 1M 5-min write x5 + 10M read x0.20 + 1M out x20 = $31
	r := callCost(m, provider.TokenUsage{InputTokens: 1e6, CacheCreationTokens: 1e6, CacheReadTokens: 10e6,
		OutputTokens: 1e6}, oct21)
	if !r.Priced || !near(r.USD, 31) || !near(r.Reread, 2) || r.Context != 12e6 {
		t.Fatalf("got %+v, want priced $31, reread $2, context 12M", r)
	}
}

func TestCallCostAntigravityPricesWhatAPICreditsWouldCost(t *testing.T) {
	gem := models.Model{Provider: "antigravity", APIModel: "gemini-3.8-flash"}
	// 1M x0.75 + 10M read x0.075 + 1M out x3.75 (promotional price through 2026-12-31)
	if r := callCost(gem, provider.TokenUsage{InputTokens: 1e6, CacheReadTokens: 10e6, OutputTokens: 1e6}, oct21); !near(r.USD, 5.25) {
		t.Errorf("antigravity gemini = %+v, want $5.25", r)
	}
	cl := models.Model{Provider: "antigravity", APIModel: "claude-opus-4-6-thinking"}
	// Google Cloud's Opus 4.6: 1M x5 + 1M out x25
	if r := callCost(cl, provider.TokenUsage{InputTokens: 1e6, OutputTokens: 1e6}, oct21); !near(r.USD, 30) {
		t.Errorf("antigravity claude = %+v, want $30", r)
	}
}

func TestUnknownModelIsCountedNotPriced(t *testing.T) {
	m := models.Model{Provider: "gemini", APIModel: "gemini-pro-default", CostPer1MIn: 5, CostPer1MOut: 5}
	var s session.Session
	addCall(&s, callCost(m, provider.TokenUsage{InputTokens: 1e6, OutputTokens: 1e6}, oct21))
	if s.Cost != 0 || s.CallsUnpriced != 1 || s.CallsPriced != 0 {
		t.Fatalf("an unknown model must be counted as unpriced and add nothing to Cost: %+v", s)
	}
}

// Providers with no sourced table keep the catalogue estimate they had before.
func TestOtherProvidersKeepTheCatalogueEstimate(t *testing.T) {
	m := models.Model{Provider: "openrouter", APIModel: "z-ai/glm", CostPer1MIn: 1, CostPer1MOut: 2,
		CostPer1MInCached: 3, CostPer1MOutCached: 0.5}
	r := callCost(m, provider.TokenUsage{InputTokens: 1e6, OutputTokens: 1e6, CacheCreationTokens: 1e6,
		CacheReadTokens: 2e6}, oct21)
	if !r.Priced || !near(r.USD, 1+2+3+1) || !near(r.Reread, 1) {
		t.Fatalf("catalogue estimate = %+v, want $7 with $1 re-reading", r)
	}
}

func TestLedgerAccumulatesCostRereadAndCalls(t *testing.T) {
	m := models.Model{Provider: "anthropic", APIModel: "claude-sonnet-5-5"}
	var s session.Session
	// 10M read x0.10 = $1; then 1M out x10 = $10
	addCall(&s, callCost(m, provider.TokenUsage{CacheReadTokens: 10e6}, oct21))
	addCall(&s, callCost(m, provider.TokenUsage{OutputTokens: 1e6}, oct21))
	if !near(s.Cost, 11) || !near(s.CostReread, 1) || s.CallsPriced != 2 || !near(s.LastCallCost, 10) {
		t.Fatalf("ledger = cost %v reread %v calls %d last %v, want 11 / 1 / 2 / 10",
			s.Cost, s.CostReread, s.CallsPriced, s.LastCallCost)
	}
}

// Each call's prompt repeats the previous prompt and reply; that part is counted
// as re-sent whether or not the provider's cache discounted it.
func TestLedgerCountsTheConversationSentAgain(t *testing.T) {
	m := models.Model{Provider: "antigravity", APIModel: "gemini-3.8-flash"}
	var s session.Session
	step := func(in, read, out int64) {
		u := provider.TokenUsage{InputTokens: in, CacheReadTokens: read, OutputTokens: out}
		addCall(&s, callCost(m, u, oct21))
		// what TrackUsage does after addCall
		s.PromptTokens, s.CompletionTokens = in+read, out
		s.CumulativePromptTokens += in + read
	}
	step(10_000, 0, 500)      // first call: nothing was sent before
	step(10_000, 2_000, 300)  // 12,000 sent; 10,500 of it was the previous prompt + reply
	step(13_000, 0, 200)      // no cache hit at all: still 12,300 repeated
	if s.ResentTokens != 10_500+12_300 {
		t.Fatalf("re-sent = %d, want %d", s.ResentTokens, 10_500+12_300)
	}
	// compaction: the new prompt is smaller than the old one; capped at the new prompt
	s.PromptTokens, s.CompletionTokens = 50_000, 0
	addCall(&s, callCost(m, provider.TokenUsage{InputTokens: 3_000}, oct21))
	if s.ResentTokens != 10_500+12_300+3_000 {
		t.Fatalf("re-sent after compaction = %d, want capped at the 3,000-token prompt", s.ResentTokens)
	}
}
