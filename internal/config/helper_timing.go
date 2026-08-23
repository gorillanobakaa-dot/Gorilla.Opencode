// Version: 1.0.0 · updated 26-08-23-18-10
//
// GORILLA OVERRIDE (2026-08-23): ROADMAP item 5. Measure how long a research
// helper actually takes, instead of resting every per-minute figure on 15.0.
//
// ResearchSecondsPerStep was invented. The author audited the cost dialog on
// 2026-08-14 and found correctly that the whole per-minute burn rate rested on
// it, and the honest response at the time was to print the assumption on screen
// next to the number so a reader could judge it. That is better than hiding it
// and it is not a substitute for knowing.
//
// The roadmap's own instruction: "record each helper's real duration when a run
// finishes and average over past runs." This is that.
//
// WHAT IS MEASURED, AND WHAT IS STILL NOT. A helper's wall-clock duration is
// observable: it is registered when it spawns and set to a terminal state when
// it stops, and the registry already carries StartedAt. So this records SECONDS
// PER HELPER, which is a real quantity.
//
// It deliberately does NOT try to derive seconds-per-STEP by dividing by
// ResearchStepsPerHelper, because that divisor is itself invented. Dividing a
// measurement by a guess produces a guess wearing a measurement's clothes, which
// is worse than the honest guess it replaced. The burn-rate arithmetic is
// reshaped instead to use cost-per-helper and seconds-per-helper, so neither
// invented constant appears in it once real samples exist.
//
// UNTIL THERE ARE SAMPLES the old arithmetic stands and the UI keeps saying
// ASSUMED. A first-run user is told a forecast built on a guess, and told that
// it is. After one research run they are told a forecast built on their own
// machine, their own model and their own connection, which is the only figure
// that was ever going to be right for them: the reference user here is on a 2012
// laptop and a metered link, and a number measured on a developer's machine
// would have been no better than 15.0.
package config

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// helperTimingSamples is how many recent helper durations are kept.
//
// Twenty rather than all of them: a rolling window follows a change (a new
// model, a worse connection) instead of being anchored to the first run
// forever, and twenty is enough for a median to be stable while still fitting
// in a file nobody notices.
const helperTimingSamples = 20

// minHelperTimingSamples is how many are needed before the measurement is used
// at all. Three, because one sample is an anecdote and the failure mode of
// reporting from too few is a confident number that swings wildly between runs,
// which reads as a broken meter.
const minHelperTimingSamples = 3

// helperTimingFloor and helperTimingCeiling bound what is accepted as a sample.
//
// A helper that "finished" in under a second was almost certainly killed before
// it started, or errored on a rate limit, and folding that into an average would
// make the forecast cheerfully wrong in the expensive direction. The ceiling
// catches the opposite: a helper left running while a laptop slept.
const (
	helperTimingFloor   = 1 * time.Second
	helperTimingCeiling = 30 * time.Minute
)

type helperTimingStore struct {
	// Seconds holds the most recent durations, oldest first.
	Seconds []float64 `json:"seconds"`
	// Updated is when a sample was last added, so a stale file can be reported
	// as stale rather than quietly presented as current.
	Updated time.Time `json:"updated"`
}

var (
	helperTimingMu    sync.RWMutex
	helperTiming      helperTimingStore
	helperTimingRead  bool
	helperTimingDirty bool
)

func helperTimingPath() string {
	return filepath.Join(CacheBase(), "helper-timing.json")
}

// loadHelperTiming reads the file once. A missing or unreadable file is the
// normal first-run state and leaves the store empty, which is what makes the UI
// keep saying ASSUMED.
func loadHelperTiming() {
	if helperTimingRead {
		return
	}
	helperTimingRead = true
	data, err := os.ReadFile(helperTimingPath())
	if err != nil {
		return
	}
	var s helperTimingStore
	if json.Unmarshal(data, &s) != nil {
		return
	}
	// Defend against a hand-edited or corrupt file: a negative or absurd sample
	// would poison every figure derived from it.
	kept := s.Seconds[:0]
	for _, v := range s.Seconds {
		if v >= helperTimingFloor.Seconds() && v <= helperTimingCeiling.Seconds() && !math.IsNaN(v) {
			kept = append(kept, v)
		}
	}
	s.Seconds = kept
	helperTiming = s
}

// RecordHelperDuration adds one finished helper's wall-clock time.
//
// Called from the sub-agent registry when a helper reaches a terminal state.
// internal/config cannot import internal/llm/agent (agent imports config), so
// the value is pushed in from the other side, the same way SetLoadoutTokens
// feeds the loadout its measured schema sizes.
//
// Out-of-range durations are DISCARDED rather than clamped. Clamping would turn
// a helper killed after 200ms into a one-second sample and quietly drag the
// average down; discarding says "this told us nothing", which is true.
func RecordHelperDuration(d time.Duration) {
	if d < helperTimingFloor || d > helperTimingCeiling {
		return
	}
	helperTimingMu.Lock()
	loadHelperTiming()
	helperTiming.Seconds = append(helperTiming.Seconds, d.Seconds())
	if n := len(helperTiming.Seconds); n > helperTimingSamples {
		helperTiming.Seconds = helperTiming.Seconds[n-helperTimingSamples:]
	}
	helperTiming.Updated = time.Now().UTC()
	helperTimingDirty = true
	helperTimingMu.Unlock()
}

// FlushHelperTiming persists the samples. Called when a research run finishes
// rather than on every helper, so ten helpers cost one write instead of ten.
// Best-effort: failing to remember a timing must never break a run.
func FlushHelperTiming() {
	helperTimingMu.Lock()
	dirty := helperTimingDirty
	snapshot := helperTimingStore{
		Seconds: append([]float64(nil), helperTiming.Seconds...),
		Updated: helperTiming.Updated,
	}
	helperTimingDirty = false
	helperTimingMu.Unlock()

	if !dirty || len(snapshot.Seconds) == 0 {
		return
	}
	data, err := json.MarshalIndent(snapshot, "", " ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(helperTimingPath()), 0o755)
	_ = writeSecretFile(helperTimingPath(), data)
}

// MeasuredSecondsPerHelper reports the typical helper duration from real runs.
//
// ok is false until there are at least minHelperTimingSamples, and the caller
// must then fall back to the assumed arithmetic AND say so. "Not measured yet"
// and "measured, and it happens to equal the guess" must not look alike.
//
// The MEDIAN, not the mean. Helper durations are long-tailed: one helper that
// hit a retry storm and took four minutes would drag a mean far above anything
// the user will actually experience, and the number is being used to forecast a
// typical run rather than a worst case.
func MeasuredSecondsPerHelper() (seconds float64, samples int, ok bool) {
	helperTimingMu.RLock()
	// The read path may be the first toucher on a fresh process.
	if !helperTimingRead {
		helperTimingMu.RUnlock()
		helperTimingMu.Lock()
		loadHelperTiming()
		helperTimingMu.Unlock()
		helperTimingMu.RLock()
	}
	vals := append([]float64(nil), helperTiming.Seconds...)
	helperTimingMu.RUnlock()

	if len(vals) < minHelperTimingSamples {
		return 0, len(vals), false
	}
	sort.Float64s(vals)
	n := len(vals)
	if n%2 == 1 {
		return vals[n/2], n, true
	}
	return (vals[n/2-1] + vals[n/2]) / 2, n, true
}

// ResetHelperTimingForTest clears the store. Tests only: the samples are a cache
// keyed to one machine's real behaviour and there is no user-facing reason to
// discard them.
func ResetHelperTimingForTest() {
	helperTimingMu.Lock()
	helperTiming = helperTimingStore{}
	helperTimingRead = true // treat as loaded-and-empty, do not re-read the disk
	helperTimingDirty = false
	helperTimingMu.Unlock()
}
