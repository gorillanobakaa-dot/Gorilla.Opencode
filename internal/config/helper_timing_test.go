package config

// GORILLA OVERRIDE (2026-08-23): ROADMAP item 5.
//
// ResearchSecondsPerStep = 15.0 was invented, and every per-minute and per-hour
// figure on the cost screen rested on it. The 2026-08-14 audit found that
// correctly and the honest response then was to print the assumption beside the
// number. That is better than hiding it and it is not the same as knowing.
//
// These pin the two things that make the replacement worth having: the number is
// real, and the UI can always tell which number it is showing.

import (
	"testing"
	"time"
)

// Below the minimum sample count the answer must be "not measured", not a
// number. One sample is an anecdote, and a forecast that swings between runs
// reads as a broken meter.
func TestTooFewSamplesReportsNotMeasured(t *testing.T) {
	ResetHelperTimingForTest()
	defer ResetHelperTimingForTest()

	for i := 0; i < minHelperTimingSamples-1; i++ {
		RecordHelperDuration(20 * time.Second)
		if _, n, ok := MeasuredSecondsPerHelper(); ok {
			t.Fatalf("reported measured after %d samples, minimum is %d", n, minHelperTimingSamples)
		}
	}
	RecordHelperDuration(20 * time.Second)
	secs, n, ok := MeasuredSecondsPerHelper()
	if !ok {
		t.Fatalf("still not measured at %d samples", n)
	}
	if secs != 20 {
		t.Errorf("median of identical 20s samples is %v, want 20", secs)
	}
}

// The MEDIAN, not the mean. Helper durations are long-tailed: one lane that hit
// a retry storm must not drag the forecast above anything a user will see.
func TestOneOutlierDoesNotMoveTheFigure(t *testing.T) {
	ResetHelperTimingForTest()
	defer ResetHelperTimingForTest()

	for _, d := range []time.Duration{10 * time.Second, 12 * time.Second, 11 * time.Second, 9 * time.Second} {
		RecordHelperDuration(d)
	}
	before, _, _ := MeasuredSecondsPerHelper()

	RecordHelperDuration(20 * time.Minute) // one lane stuck on retries
	after, _, ok := MeasuredSecondsPerHelper()
	if !ok {
		t.Fatal("lost the measurement")
	}
	if after > before*2 {
		t.Errorf("a single outlier moved the figure from %.0fs to %.0fs. A mean would "+
			"do that; the median exists so it does not.", before, after)
	}
}

// Junk is DISCARDED, not clamped. Clamping a helper killed after 200ms into a
// one-second sample would quietly drag the average down and make the forecast
// wrong in the expensive direction.
func TestImplausibleDurationsAreDiscardedNotClamped(t *testing.T) {
	ResetHelperTimingForTest()
	defer ResetHelperTimingForTest()

	for i := 0; i < minHelperTimingSamples; i++ {
		RecordHelperDuration(30 * time.Second)
	}
	base, n, _ := MeasuredSecondsPerHelper()

	RecordHelperDuration(50 * time.Millisecond) // killed before it started
	RecordHelperDuration(2 * time.Hour)         // laptop slept mid-run
	RecordHelperDuration(-5 * time.Second)      // nonsense

	got, gotN, _ := MeasuredSecondsPerHelper()
	if gotN != n {
		t.Errorf("sample count moved %d -> %d; implausible durations were kept", n, gotN)
	}
	if got != base {
		t.Errorf("the figure moved %.0f -> %.0f on junk input", base, got)
	}
}

// The window rolls, so the figure follows a change (new model, worse link)
// instead of being anchored to the first run forever.
func TestTheWindowRollsRatherThanAccumulating(t *testing.T) {
	ResetHelperTimingForTest()
	defer ResetHelperTimingForTest()

	for i := 0; i < helperTimingSamples*2; i++ {
		RecordHelperDuration(5 * time.Second)
	}
	_, n, _ := MeasuredSecondsPerHelper()
	if n != helperTimingSamples {
		t.Fatalf("kept %d samples, the window is %d", n, helperTimingSamples)
	}

	// Everything gets slower. The figure must follow within one window.
	for i := 0; i < helperTimingSamples; i++ {
		RecordHelperDuration(60 * time.Second)
	}
	secs, _, _ := MeasuredSecondsPerHelper()
	if secs != 60 {
		t.Errorf("after a full window of 60s samples the median is %.0fs, want 60: the "+
			"window is not rolling and the forecast is stuck on old behaviour", secs)
	}
}

// THE HONESTY RULE. helperStepsPerMinute must report WHICH number it used, so
// the UI can word itself accordingly. "Not measured yet" and "measured, and it
// agrees with the guess" must never look alike, which is the same rule
// balances.go states for quota bars.
func TestTheRateSaysWhetherItIsMeasured(t *testing.T) {
	ResetHelperTimingForTest()
	defer ResetHelperTimingForTest()

	fallback, measured := helperStepsPerMinute()
	if measured {
		t.Fatal("reported measured with no samples at all")
	}
	if want := 60.0 / ResearchSecondsPerStep; fallback != want {
		t.Errorf("fallback rate %v, want the assumed %v", fallback, want)
	}
	if n, ok := ResearchRateIsMeasured(); ok || n != 0 {
		t.Errorf("ResearchRateIsMeasured() = (%d, %v) with no samples", n, ok)
	}

	// Feed it durations that happen to imply exactly the assumed rate. The flag
	// must still flip: agreeing with the guess is not the same as being one.
	for i := 0; i < minHelperTimingSamples; i++ {
		RecordHelperDuration(time.Duration(ResearchSecondsPerStep*float64(ResearchStepsPerHelper)) * time.Second)
	}
	rate, measured := helperStepsPerMinute()
	if !measured {
		t.Fatal("still reporting unmeasured after enough samples")
	}
	if rate != fallback {
		t.Errorf("rate %v, want %v: these samples were chosen to imply the assumed rate", rate, fallback)
	}
	if n, ok := ResearchRateIsMeasured(); !ok || n != minHelperTimingSamples {
		t.Errorf("ResearchRateIsMeasured() = (%d, %v), want (%d, true)", n, ok, minHelperTimingSamples)
	}
}

// A faster machine must produce a higher burn rate, or the measurement is not
// reaching the arithmetic at all.
func TestAFasterMachineRaisesTheBurnRate(t *testing.T) {
	ResetHelperTimingForTest()
	defer ResetHelperTimingForTest()

	for i := 0; i < minHelperTimingSamples; i++ {
		RecordHelperDuration(120 * time.Second)
	}
	slow, _ := helperStepsPerMinute()

	ResetHelperTimingForTest()
	for i := 0; i < minHelperTimingSamples; i++ {
		RecordHelperDuration(10 * time.Second)
	}
	fast, _ := helperStepsPerMinute()

	if !(fast > slow) {
		t.Errorf("10s helpers give rate %v and 120s helpers give %v; a faster machine "+
			"must burn faster, so the measurement is not feeding the rate", fast, slow)
	}
}
