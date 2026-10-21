package agent

import (
	"time"

	"github.com/opencode-ai/opencode/internal/llm/cost"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/provider"
	"github.com/opencode-ai/opencode/internal/session"
)

// callCost prices one model call.
//
// GORILLA (2026-10-21): providers with a sourced, dated price table
// (internal/llm/cost/prices.json: Anthropic, the Gemini API, Antigravity's
// Gemini and Claude) are priced from it, with its long-prompt tiers and dated
// promotions; a model the table does not know is UNPRICED, not given the
// catalogue's number. Every other provider keeps the catalogue estimate it had
// before (models.Model.CostPer1M*; CostPer1MInCached holds the cache-write
// rate and CostPer1MOutCached the cache-read rate, whatever their names say).
// OpenCode sends no TTL, so every Anthropic cache write is a 5-minute write.
func callCost(model models.Model, usage provider.TokenUsage, now time.Time) cost.Result {
	if table := cost.TableFor(string(model.Provider), model.APIModel); table != "" {
		if p, err := cost.Load(); err == nil {
			return p.Price(cost.Call{
				Table: table, Model: model.APIModel, Time: now,
				Input: usage.InputTokens, CacheWrite5m: usage.CacheCreationTokens,
				CacheRead: usage.CacheReadTokens, Output: usage.OutputTokens,
			})
		}
	}
	reread := model.CostPer1MOutCached / 1e6 * float64(usage.CacheReadTokens)
	return cost.Result{
		USD: model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) + reread +
			model.CostPer1MIn/1e6*float64(usage.InputTokens) +
			model.CostPer1MOut/1e6*float64(usage.OutputTokens),
		Reread:   reread,
		Priced:   true,
		PricedAs: string(model.ID),
		Context:  usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens,
	}
}

// addCall books one priced or unpriced call on the session's ledger. An
// unpriced call is counted, never added to Cost.
//
// It must run BEFORE the caller overwrites PromptTokens/CompletionTokens with
// this call's figures: those still hold the previous call's prompt and reply,
// which is exactly what this call sent again (capped at this call's prompt,
// because compaction can make the new prompt smaller than the old one).
func addCall(s *session.Session, r cost.Result) {
	s.ResentTokens += min(s.PromptTokens+s.CompletionTokens, r.Context)
	if !r.Priced {
		s.CallsUnpriced++
		return
	}
	s.Cost += r.USD
	s.CostReread += r.Reread
	s.CallsPriced++
	s.LastCallCost = r.USD
}
