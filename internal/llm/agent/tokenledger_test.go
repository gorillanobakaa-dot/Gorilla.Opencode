package agent

// GORILLA OVERRIDE (2026-08-19): the ledger has to answer two different
// questions and used to have one pair of numbers for both.
//
// "How full is the context?" is assigned and reset by compaction.
// "What has this conversation cost?" accumulates and survives compaction.
// Conflating them meant the export and the sidebar reported a single turn as
// though it were the whole run, while Cost — three lines away in the same
// function — had always accumulated.

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/provider"
	"github.com/opencode-ai/opencode/internal/session"
)

// applyUsage mirrors exactly what TrackUsage does to a session, so the ledger
// arithmetic is testable without a database or a live provider.
func applyUsage(sess *session.Session, u provider.TokenUsage) {
	sess.CompletionTokens = u.OutputTokens
	sess.PromptTokens = u.InputTokens + u.CacheCreationTokens + u.CacheReadTokens
	sess.CumulativeCompletionTokens += u.OutputTokens
	sess.CumulativePromptTokens += u.InputTokens + u.CacheCreationTokens + u.CacheReadTokens
}

func TestContextGaugeShowsTheCurrentTurnNotARunningTotal(t *testing.T) {
	var s session.Session
	applyUsage(&s, provider.TokenUsage{InputTokens: 1000, OutputTokens: 200})
	applyUsage(&s, provider.TokenUsage{InputTokens: 1500, OutputTokens: 300})

	if s.PromptTokens != 1500 || s.CompletionTokens != 300 {
		t.Fatalf("context gauge accumulated: got %d in / %d out, want 1500 / 300.\n"+
			"A running total here climbs past the context window and sits there showing a false warning.",
			s.PromptTokens, s.CompletionTokens)
	}
}

func TestTheLedgerAccumulates(t *testing.T) {
	var s session.Session
	applyUsage(&s, provider.TokenUsage{InputTokens: 1000, OutputTokens: 200})
	applyUsage(&s, provider.TokenUsage{InputTokens: 1500, OutputTokens: 300})

	if s.CumulativePromptTokens != 2500 || s.CumulativeCompletionTokens != 500 {
		t.Fatalf("ledger did not accumulate: got %d in / %d out, want 2500 / 500",
			s.CumulativePromptTokens, s.CumulativeCompletionTokens)
	}
}

// A cache read is INPUT. Tool schemas live in the cached prefix, so the number
// a user consults to answer "what are my schemas costing me" must not be
// attributing them to the output column.
func TestCacheReadsCountAsInput(t *testing.T) {
	var s session.Session
	applyUsage(&s, provider.TokenUsage{
		InputTokens:         100,
		OutputTokens:        50,
		CacheCreationTokens: 400,
		CacheReadTokens:     3000,
	})
	if s.PromptTokens != 3500 {
		t.Errorf("input = %d, want 3500 (100 fresh + 400 written to cache + 3000 read from it)", s.PromptTokens)
	}
	if s.CompletionTokens != 50 {
		t.Errorf("output = %d, want 50 — cache reads are not output", s.CompletionTokens)
	}
}

// Summarising costs real tokens. A ledger that forgot them at every compaction
// would understate a long session by exactly the amount the user is most
// likely to be asking about.
func TestCompactionResetsTheGaugeButNotTheLedger(t *testing.T) {
	var s session.Session
	applyUsage(&s, provider.TokenUsage{InputTokens: 40000, OutputTokens: 2000})

	// What the summarise path does:
	summary := provider.TokenUsage{InputTokens: 40000, OutputTokens: 900}
	s.CompletionTokens = summary.OutputTokens
	s.PromptTokens = 0
	s.CumulativeCompletionTokens += summary.OutputTokens
	s.CumulativePromptTokens += summary.InputTokens + summary.CacheCreationTokens + summary.CacheReadTokens

	if s.PromptTokens != 0 {
		t.Errorf("the context gauge did not reset after compaction: %d", s.PromptTokens)
	}
	if s.CumulativePromptTokens != 80000 {
		t.Errorf("cumulative input = %d, want 80000 — the summarise call's own cost was dropped", s.CumulativePromptTokens)
	}
	if s.CumulativeCompletionTokens != 2900 {
		t.Errorf("cumulative output = %d, want 2900", s.CumulativeCompletionTokens)
	}
}
