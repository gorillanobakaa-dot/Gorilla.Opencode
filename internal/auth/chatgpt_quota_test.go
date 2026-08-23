package auth

import (
	"net/http"
	"testing"
)

// Window labels are ported from the Codex reference client, not invented. The
// first hand-written version had no 5h bucket (the most common ChatGPT window)
// and used threshold ranges instead of a 5% tolerance, so it would have called
// a 5-hour limit "Hourly" and a 200-minute window "Daily".
func TestWindowLabelMatchesCodexBuckets(t *testing.T) {
	for _, tc := range []struct {
		minutes int64
		want    string
	}{
		{300, "5h limit"},        // exactly 5h
		{295, "5h limit"},        // within 5% tolerance
		{1440, "Daily limit"},    // exactly a day
		{10080, "Weekly limit"},  // exactly a week
		{43200, "Monthly limit"}, // 30 days, what a free plan reports
		{525600, "Annual limit"},
		{200, "Usage limit"}, // matches no bucket: fallback word, NOT a guess
		{0, "Usage limit"},
	} {
		if got := (ChatGPTWindow{WindowMinutes: tc.minutes}).Label(false); got != tc.want {
			t.Errorf("%d minutes => %q, want %q", tc.minutes, got, tc.want)
		}
	}
	if got := (ChatGPTWindow{WindowMinutes: 200}).Label(true); got != "Secondary usage limit" {
		t.Errorf("secondary fallback = %q", got)
	}
}

// The wire sends used_percent; this panel reports REMAINING. Inverting it is
// the panic-or-burned-week error the quota panel override exists to prevent, so
// the conversion is pinned with an asymmetric value (never 50).
func TestRemainingInvertsUsedPercent(t *testing.T) {
	// EXACT equality on purpose. (100-used)/100 is exact at round percentages;
	// the 1-used/100 form is not, and that one ULP crosses the banana ladder's
	// 0.20 boundary and mislabels a fifth-full meter as an emergency.
	if got := (ChatGPTWindow{UsedPercent: 80}).Remaining(); got != 0.2 {
		t.Errorf("80%% used => %v remaining, want exactly 0.2", got)
	}
	// The other alert boundaries must land exactly too.
	for used, want := range map[float64]float64{85: 0.15, 90: 0.10, 95: 0.05} {
		if got := (ChatGPTWindow{UsedPercent: used}).Remaining(); got != want {
			t.Errorf("%v%% used => %v, want exactly %v (tier boundary)", used, got, want)
		}
	}
	if got := (ChatGPTWindow{UsedPercent: 0}).Remaining(); got != 1 {
		t.Errorf("nothing used => %v, want 1", got)
	}
	// Out-of-range values from the wire must clamp, not produce a negative bar.
	if got := (ChatGPTWindow{UsedPercent: 130}).Remaining(); got != 0 {
		t.Errorf("over-100%% used => %v, want 0", got)
	}
}

func TestParseChatGPTQuotaReadsHeaderFamily(t *testing.T) {
	h := http.Header{}
	h.Set("x-codex-primary-used-percent", "80")
	h.Set("x-codex-primary-window-minutes", "43200")
	h.Set("x-codex-primary-reset-at", "1789000000")
	h.Set("x-codex-limit-name", "codex")

	q := ParseChatGPTQuota(h)
	if q == nil {
		t.Fatal("headers present but nothing parsed")
	}
	if q.Primary == nil || q.Primary.UsedPercent != 80 {
		t.Fatalf("primary not parsed: %+v", q.Primary)
	}
	if got := q.Primary.Label(false); got != "Monthly limit" {
		t.Errorf("label = %q, want Monthly limit", got)
	}
	if q.Secondary != nil {
		t.Error("no secondary headers were sent; one must not be invented")
	}
}

// No usage headers at all must yield nil, so "no meter" stays distinguishable
// from "meter reads zero".
func TestParseChatGPTQuotaNilWhenAbsent(t *testing.T) {
	if q := ParseChatGPTQuota(http.Header{}); q != nil {
		t.Errorf("absent headers must parse to nil, got %+v", q)
	}
}

// A window carrying nothing but a zero is not a window.
//
// The owner asked why our panel showed a "Secondary usage limit" at 100% when
// Codex, on the same free account, shows only the monthly one. The backend does
// send x-codex-secondary-used-percent: 0, with no window length and no reset
// time. Accepting it made Remaining() return 1.0 and the panel drew a FULL GREEN
// BAR for an allowance that does not exist on this plan: unknown rendered as
// plenty-left, which balances.go forbids in its own header.
//
// The guard is Codex's own (codex-api/src/rate_limits.rs, parse_rate_limit_window),
// ported rather than invented.
func TestAWindowOfNothingButZeroIsNotAWindow(t *testing.T) {
	// Exactly what a free plan sends: a real monthly primary, and a secondary
	// that is a bare zero.
	h := http.Header{}
	h.Set("x-codex-primary-used-percent", "84")
	h.Set("x-codex-primary-window-minutes", "43200")
	h.Set("x-codex-primary-reset-at", "1789495295")
	h.Set("x-codex-secondary-used-percent", "0")

	q := ParseChatGPTQuota(h)
	if q == nil || q.Primary == nil {
		t.Fatal("the real monthly window was lost")
	}
	if q.Secondary != nil {
		t.Errorf("a secondary window with used=0, no window length and no reset was "+
			"kept: %+v.\n  It renders as a full green bar for an allowance that does "+
			"not exist. Codex discards it; so must we.", q.Secondary)
	}
}

// The guard must not throw away a genuine untouched allowance. Zero used WITH a
// declared window or a reset time is a real limit nobody has spent yet, and
// hiding it would be the opposite error.
func TestAnUntouchedButRealWindowSurvives(t *testing.T) {
	for name, set := range map[string]func(http.Header){
		"zero used, but a declared window": func(h http.Header) {
			h.Set("x-codex-secondary-window-minutes", "10080")
		},
		"zero used, but a reset time": func(h http.Header) {
			h.Set("x-codex-secondary-reset-at", "1789495295")
		},
	} {
		h := http.Header{}
		h.Set("x-codex-secondary-used-percent", "0")
		set(h)
		q := ParseChatGPTQuota(h)
		if q == nil || q.Secondary == nil {
			t.Errorf("%s: a real untouched allowance was discarded", name)
			continue
		}
		if got := q.Secondary.Remaining(); got != 1 {
			t.Errorf("%s: untouched window reports %v remaining, want 1", name, got)
		}
	}
}

// And a response carrying only the empty secondary must parse to nothing at all,
// so "no meter" stays distinguishable from "meter reads full".
func TestOnlyAnEmptySecondaryMeansNoReading(t *testing.T) {
	h := http.Header{}
	h.Set("x-codex-secondary-used-percent", "0")
	if q := ParseChatGPTQuota(h); q != nil {
		t.Errorf("a bare zero secondary produced a reading: %+v", q)
	}
}
