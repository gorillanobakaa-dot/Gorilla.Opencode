package dialog

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
)

// THE BUG, reported 2026-08-14 with three screenshots and the arithmetic worked
// out by hand:
//
//	$0.01 PER MINUTE.    PER HOUR: $0        (true rate $0.006560/min = $0.3936/hr)
//	$0.02 PER MINUTE.    PER HOUR: $1        (0.02 x 60 = 1.20, not 1)
//
// The per-hour figure printed at %.0f — whole dollars — so an hour of a real,
// billed rate rendered as costing NOTHING. Every number was computed correctly
// and then thrown away by the format verb.
//
// This screen's whole purpose is to be checked with a calculator by someone who
// cannot read the source. These tests assert the printed numbers close.

// parseMoney reads back a figure exactly as a user would off the screen.
func parseMoney(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(strings.TrimPrefix(s, "$"), 64)
	if err != nil {
		t.Fatalf("cannot read %q as money: %v", s, err)
	}
	return v
}

// realisticRates spans free-tier equivalents through expensive metered models.
// 0.006560 and 0.026240 are the MEASURED Gemini-2.0-Flash-equivalent rates at
// 1 and 4 helpers in flight — the two figures in the screenshots.
var realisticRates = []float64{
	0.000041, 0.000600, 0.006560, 0.026240, 0.0625, 0.1, 0.155, 0.5, 1.0, 2.5, 9.11, 36.43,
}

func TestPrintedPerMinuteTimesSixtyIsThePrintedPerHour(t *testing.T) {
	for _, perMin := range realisticRates {
		gotMin := parseMoney(t, rate(perMin))
		gotHour := parseMoney(t, amount(perMin*60))

		want := gotMin * 60
		// Tolerance is half a cent on the hour, which is the resolution the
		// hourly figure is printed at. Anything worse means a user multiplying
		// what they can see gets a different answer from what they can see.
		if math.Abs(want-gotHour) > 0.005 {
			t.Errorf("rate %.6f/min prints as %s/min and %s/hr; %s x 60 = $%.4f, which is not %s",
				perMin, rate(perMin), amount(perMin*60), rate(perMin), want, amount(perMin*60))
		}
	}
}

// The specific line from the screenshot. An hour of a billed rate must never
// print as zero.
func TestAnHourOfARealRateIsNeverPrintedAsZero(t *testing.T) {
	for _, perMin := range realisticRates {
		if perMin <= 0 {
			continue
		}
		got := amount(perMin * 60)
		if parseMoney(t, got) == 0 {
			t.Errorf("rate %.6f/min prints an hourly cost of %s — that tells the user an hour is free",
				perMin, got)
		}
	}
}

// A run total below a cent must not read as free either.
func TestSubCentTotalsAreNotPrintedAsZero(t *testing.T) {
	for _, v := range []float64{0.0001, 0.0009, 0.004, 0.0099} {
		if got := amount(v); parseMoney(t, got) == 0 {
			t.Errorf("a real cost of $%.4f prints as %s", v, got)
		}
	}
	if got := amount(0); got != "$0.00" {
		t.Errorf("genuinely zero should print $0.00, got %s", got)
	}
	if got := rate(0); got != "$0.00" {
		t.Errorf("genuinely zero rate should print $0.00, got %s", got)
	}
}

// NON-VACUOUS GUARD. The old verbs must fail the arithmetic test above. If this
// ever passes, the assertions are not testing anything.
func TestTheOldFormatVerbsFailTheArithmetic(t *testing.T) {
	oldRate := func(v float64) string { return fmt.Sprintf("$%.2f", v) }
	oldAmount := func(v float64) string { return fmt.Sprintf("$%.0f", v) }

	broken := 0
	var worst string
	for _, perMin := range realisticRates {
		gotMin := parseMoney(t, oldRate(perMin))
		gotHour := parseMoney(t, oldAmount(perMin*60))
		if math.Abs(gotMin*60-gotHour) > 0.005 {
			broken++
			if worst == "" {
				worst = fmt.Sprintf("%s/min vs %s/hr (true %.6f/min)", oldRate(perMin), oldAmount(perMin*60), perMin)
			}
		}
	}
	if broken == 0 {
		t.Fatal("the old per-minute/per-hour verbs pass the arithmetic check — the check is vacuous")
	}
	t.Logf("old verbs broke %d of %d rates, e.g. %s", broken, len(realisticRates), worst)
}

// The exact screenshot case, spelled out so a regression names itself.
func TestTheScreenshotCaseNowCloses(t *testing.T) {
	const measured = 0.006560 // Gemini 2.0 Flash equivalent, 1 helper in flight

	gotMin, gotHour := rate(measured), amount(measured*60)
	if gotHour == "$0" || gotHour == "$0.00" {
		t.Fatalf("still printing an hour as free: %s per minute, %s per hour", gotMin, gotHour)
	}
	if v := parseMoney(t, gotMin) * 60; math.Abs(v-parseMoney(t, gotHour)) > 0.005 {
		t.Fatalf("%s x 60 = $%.4f, screen says %s", gotMin, v, gotHour)
	}
	t.Logf("was: $0.01/min $0/hr    now: %s/min %s/hr", gotMin, gotHour)
}
