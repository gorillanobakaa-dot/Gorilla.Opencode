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
// money forecast on both gates assumes three steps and 700 output tokens per
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

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// researchRunsKept bounds the history. Enough for a median that one odd run
// cannot move; few enough that a change of model shows up within a week of use.
const researchRunsKept = 20

type researchRunRecord struct {
	When      time.Time `json:"when"`
	Sessions  int       `json:"sessions"`
	TokensIn  int64     `json:"tokens_in"`
	TokensOut int64     `json:"tokens_out"`
	ToolCalls int64     `json:"tool_calls"`
	Seconds   float64   `json:"seconds"`
}

var researchRunsMu sync.Mutex

func researchRunsPath() string {
	return filepath.Join(config.CacheBase(), "research-runs.json")
}

// loadResearchRuns reads the history. A missing or unreadable file is the
// normal first-run state and yields nothing, which is what makes the screens
// say "no measurement yet". Implausible rows are dropped rather than repaired.
func loadResearchRuns() []researchRunRecord {
	data, err := os.ReadFile(researchRunsPath())
	if err != nil {
		return nil
	}
	var runs []researchRunRecord
	if json.Unmarshal(data, &runs) != nil {
		return nil
	}
	kept := runs[:0]
	for _, r := range runs {
		if r.Sessions > 0 && r.TokensIn >= 0 && r.TokensOut >= 0 && r.TokensIn+r.TokensOut > 0 && r.ToolCalls >= 0 {
			kept = append(kept, r)
		}
	}
	return kept
}

// recordResearchRun appends one finished run. Best-effort: failing to remember
// a measurement must never fail the run that produced it.
func recordResearchRun(sessions, reported, helpers int, total helperSpend, elapsed time.Duration) {
	if sessions <= 0 || helpers <= 0 || reported < helpers || total.inTokens+total.outTokens <= 0 {
		return
	}
	researchRunsMu.Lock()
	defer researchRunsMu.Unlock()

	runs := append(loadResearchRuns(), researchRunRecord{
		When:      time.Now().UTC(),
		Sessions:  sessions,
		TokensIn:  total.inTokens,
		TokensOut: total.outTokens,
		ToolCalls: total.toolCalls,
		Seconds:   elapsed.Seconds(),
	})
	if n := len(runs); n > researchRunsKept {
		runs = runs[n-researchRunsKept:]
	}
	data, err := json.MarshalIndent(runs, "", " ")
	if err != nil {
		return
	}
	path := researchRunsPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, data, 0o600)
}

// MeasuredRunSize reports what a helper session has really used on this
// machine: the median, over the remembered runs, of tokens per session and of
// tool calls per session. ok is false until one complete run has been recorded,
// and the caller must then say so rather than substitute a figure.
//
// The MEDIAN for the reason config.MeasuredSecondsPerHelper gives: one run that
// hit a retry storm must not set the number everybody is shown.
func MeasuredRunSize() (tokensPerSession int64, toolCallsPerSession float64, runs int, ok bool) {
	researchRunsMu.Lock()
	history := loadResearchRuns()
	researchRunsMu.Unlock()

	if len(history) == 0 {
		return 0, 0, 0, false
	}
	tokens := make([]float64, len(history))
	calls := make([]float64, len(history))
	for i, r := range history {
		tokens[i] = float64(r.TokensIn+r.TokensOut) / float64(r.Sessions)
		calls[i] = float64(r.ToolCalls) / float64(r.Sessions)
	}
	return int64(median(tokens) + 0.5), median(calls), len(history), true
}

func median(vals []float64) float64 {
	sort.Float64s(vals)
	n := len(vals)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return vals[n/2]
	}
	return (vals[n/2-1] + vals[n/2]) / 2
}
