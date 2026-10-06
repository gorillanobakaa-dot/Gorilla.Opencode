package config

// GORILLA OVERRIDE (2026-10-06): the record of finished research runs, and the
// per-helper token basis every research forecast is priced from.
//
// WHY IT MOVED HERE. On 2026-10-05 (audit findings O3 and S15) the agent package
// began remembering what finished runs really used, and the two cost screens
// printed that measurement beside the forecast with a multiplier: "expect about
// 1.6x the money shown". The forecast itself still priced the assumption:
// ResearchStepsPerHelper steps of (helper context + ResearchOutputPerStep),
// about 21,800 tokens a helper in the test configuration, where the one real run
// this project quotes works out at about 35,000. A screen that prints a number
// and then says the number is wrong by a known factor should print the right
// number. The forecast is computed in this package, and this package cannot
// import internal/llm/agent (agent imports config), so the record lives here
// now: one reader, and the money is worked out from the same figure the screen
// calls measured. The agent package keeps the completeness rule (every helper
// reported) and pushes finished runs in, the way RecordHelperDuration is fed.
//
// Only runs in which EVERY helper reported are kept (the caller enforces it). A
// run that was killed or lost lanes to a rate limit says little about what a run
// costs, and folding it in would drag the figure down exactly when runs are
// going badly. Where no run has finished yet the forecast keeps the assumed
// basis and the screens say so; there is no fallback constant here on purpose,
// because a fallback is how the August number became permanent.

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ResearchRunsKept bounds the history. Enough for a median that one odd run
// cannot move; few enough that a change of model shows up within a week of use.
const ResearchRunsKept = 20

type researchRunRecord struct {
	When      time.Time `json:"when"`
	Sessions  int       `json:"sessions"`
	TokensIn  int64     `json:"tokens_in"`
	TokensOut int64     `json:"tokens_out"`
	ToolCalls int64     `json:"tool_calls"`
	Seconds   float64   `json:"seconds"`
}

var researchRunsMu sync.Mutex

// ResearchRunsPath is where this machine's finished runs are remembered.
func ResearchRunsPath() string {
	return filepath.Join(CacheBase(), "research-runs.json")
}

// loadResearchRuns reads the history. A missing or unreadable file is the
// normal first-run state and yields nothing, which is what makes the screens
// say the basis is assumed. Implausible rows are dropped rather than repaired.
// Read on every call rather than cached: the file is small, and a cache keyed
// to a process would show a stale basis for the rest of a session after a run.
func loadResearchRuns() []researchRunRecord {
	data, err := os.ReadFile(ResearchRunsPath())
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

// RecordResearchRun appends one finished, complete run. Best-effort: failing to
// remember a measurement must never fail the run that produced it.
func RecordResearchRun(sessions int, tokensIn, tokensOut, toolCalls int64, elapsed time.Duration) {
	if sessions <= 0 || tokensIn < 0 || tokensOut < 0 || tokensIn+tokensOut <= 0 || toolCalls < 0 {
		return
	}
	researchRunsMu.Lock()
	defer researchRunsMu.Unlock()

	runs := append(loadResearchRuns(), researchRunRecord{
		When:      time.Now().UTC(),
		Sessions:  sessions,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		ToolCalls: toolCalls,
		Seconds:   elapsed.Seconds(),
	})
	if n := len(runs); n > ResearchRunsKept {
		runs = runs[n-ResearchRunsKept:]
	}
	data, err := json.MarshalIndent(runs, "", " ")
	if err != nil {
		return
	}
	path := ResearchRunsPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, data, 0o600)
}

// MeasuredRunSize reports what a helper session has really used on this
// machine: the median, over the remembered runs, of tokens per session and of
// tool calls per session. ok is false until one complete run has been recorded,
// and the caller must then say so rather than substitute a figure.
//
// The MEDIAN for the reason MeasuredSecondsPerHelper gives: one run that hit a
// retry storm must not set the number everybody is shown.
func MeasuredRunSize() (tokensPerSession int64, toolCallsPerSession float64, runs int, ok bool) {
	tokens, _, calls, n := measuredSessionMedians()
	if n == 0 {
		return 0, 0, 0, false
	}
	return int64(tokens + 0.5), calls, n, true
}

// measuredSessionMedians is the one place the history is reduced to figures:
// median tokens per session, the input share of all tokens over the history,
// median tool calls per session, and how many runs back them. n is 0 when
// nothing is recorded.
func measuredSessionMedians() (tokens, inShare, calls float64, n int) {
	researchRunsMu.Lock()
	history := loadResearchRuns()
	researchRunsMu.Unlock()

	if len(history) == 0 {
		return 0, 0, 0, 0
	}
	perSession := make([]float64, len(history))
	perSessionCalls := make([]float64, len(history))
	var in, all int64
	for i, r := range history {
		perSession[i] = float64(r.TokensIn+r.TokensOut) / float64(r.Sessions)
		perSessionCalls[i] = float64(r.ToolCalls) / float64(r.Sessions)
		in += r.TokensIn
		all += r.TokensIn + r.TokensOut
	}
	return median(perSession), float64(in) / float64(all), median(perSessionCalls), len(history)
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

// ResearchHelperSessionTokens is the token basis one helper session is priced
// at, split into input and output because the two carry different prices.
//
// When at least one complete run is recorded on this machine, in+out is the
// recorded median tokens per session (the figure MeasuredRunSize reports and
// the screens print), split by the history's own input share so the money
// follows from exactly the number shown. When none is recorded the basis is
// the assumption: ResearchStepsPerHelper steps of the helper's measured
// context plus ResearchOutputPerStep out, and measured is false so the screens
// say so. runs is how many recorded runs back a measured basis, 0 otherwise.
func ResearchHelperSessionTokens() (in, out, runs int, measured bool) {
	if tokens, inShare, _, n := measuredSessionMedians(); n > 0 {
		total := int(tokens + 0.5)
		in = int(math.Round(float64(total) * inShare))
		return in, total - in, n, true
	}
	return ResearchStepsPerHelper * ResearchHelperBasisTokens(), ResearchStepsPerHelper * ResearchOutputPerStep, 0, false
}

// ResearchOrdinaryQuestionTokens is what one of the user's own turns costs in
// tokens: the coder's context and a reply. It is the unit the QUOTA block
// converts a run into, and it is defined once so the screen's explanatory line
// and ResearchQuotaMultiple cannot drift apart.
func ResearchOrdinaryQuestionTokens() int {
	return LoadoutActiveTokens() + ResearchOutputPerStep
}
