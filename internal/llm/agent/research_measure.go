package agent

// GORILLA OVERRIDE (2026-10-05): this file did not exist upstream. It remembers
// what finished research runs really used, so the cost screens can print a
// measurement instead of a typed number.
//
// WHY IT EXISTS — audit findings O3 and S15. The /osint gate printed "about N
// TOKENS PER HOUR ... measured from a real run, not modelled", and the figure
// behind it was the constant 21596/8: one run, on one model, on 2026-08-17,
// typed into the source and shown to every user on every model ever since. The
// sentence under it ("a quarter of a million tokens in the first ten minutes is
// normal") contradicted the line above it at the default four helpers. And the
// money forecast on both gates assumed three steps and 700 output tokens per
// helper, where that same August run works out at about 35,000 tokens per
// helper — a forecast low by a multiple, with nothing on screen to say so.
//
// Every run already ends with a receipt (ResearchReceipt): sessions, tokens in
// and out, tool calls. It was printed and then forgotten. This keeps the last
// few, on this machine only, and reports their median per session. Where no run
// has finished yet the callers say "no measurement yet" — there is no fallback
// constant here on purpose, because a fallback is how the August number became
// permanent.
//
// Only runs in which EVERY helper reported are kept. A run that was killed or
// lost lanes to a rate limit says little about what a run costs, and folding it
// in would drag the figure down exactly when runs are going badly.
//
// GORILLA OVERRIDE (2026-10-06): the store itself moved to
// config/research_runs.go, so the money forecast (computed in config, which
// cannot import this package) is priced from the measurement rather than
// printed beside it with a multiplier. This file keeps the completeness rule
// above, which only the run knows, and pushes complete runs in.

import (
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// researchRunsKept bounds the history; see config.ResearchRunsKept.
const researchRunsKept = config.ResearchRunsKept

func researchRunsPath() string { return config.ResearchRunsPath() }

// recordResearchRun appends one finished run, provided every helper reported.
// Best-effort: failing to remember a measurement must never fail the run that
// produced it.
func recordResearchRun(sessions, reported, helpers int, total helperSpend, elapsed time.Duration) {
	if sessions <= 0 || helpers <= 0 || reported < helpers || total.inTokens+total.outTokens <= 0 {
		return
	}
	config.RecordResearchRun(sessions, total.inTokens, total.outTokens, total.toolCalls, elapsed)
}

// MeasuredRunSize reports what a helper session has really used on this
// machine: the median, over the remembered runs, of tokens per session and of
// tool calls per session. ok is false until one complete run has been recorded,
// and the caller must then say so rather than substitute a figure.
func MeasuredRunSize() (tokensPerSession int64, toolCallsPerSession float64, runs int, ok bool) {
	return config.MeasuredRunSize()
}
