package auth

import (
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/quota"
)

// Each adapter must stamp the account it was given onto every meter it emits.
// This is the structural half of the /usage fix: v0.1.115 gated the wrong
// reading out at runtime, and a gate is something somebody has to remember to
// write. A meter that cannot exist without its owner needs no remembering.
func TestAntigravityMetersCarryTheGoogleAccount(t *testing.T) {
	q := &QuotaSummary{Groups: []QuotaGroup{
		{DisplayName: "Gemini Models", Description: "shared weekly limit",
			Buckets: []QuotaBucket{{DisplayName: "Weekly Limit", RemainingFraction: 0.9058, ResetTime: "6d"}}},
		{DisplayName: "Claude and GPT Models",
			Buckets: []QuotaBucket{{DisplayName: "Weekly Limit", RemainingFraction: 1}}},
	}}

	meters := q.ToMeter("gorilla@example.com")
	if len(meters) != 2 {
		t.Fatalf("got %d meters, want one per group", len(meters))
	}
	for _, m := range meters {
		if m.Account != "gorilla@example.com" {
			t.Errorf("meter %q lost its account: %q", m.Provider, m.Account)
		}
		// The heading is the GROUP name: one Google sign-in reports several
		// families, each its own barrel, so they must not share one title.
		if m.Provider == AntigravityProviderName {
			t.Errorf("meter kept the generic provider heading instead of its group name")
		}
		if m.Kind != quota.KindWindowQuota {
			t.Errorf("an allowance inside a rolling window adapted to %v", m.Kind)
		}
		if err := m.Validate(); err != nil {
			t.Errorf("adapted meter is invalid: %v", err)
		}
	}
	if meters[0].Provider != "Gemini Models" || meters[1].Provider != "Claude and GPT Models" {
		t.Errorf("headings are %q and %q, want the group names",
			meters[0].Provider, meters[1].Provider)
	}
}

// The ChatGPT adapter must take Remaining from the single conversion point, not
// re-derive it. The wire says used_percent; the panel reports what is LEFT, and
// the obvious arithmetic is off by one ULP at every tier boundary.
func TestChatGPTMeterUsesTheSingleConversionPoint(t *testing.T) {
	q := &ChatGPTQuota{
		Primary:   &ChatGPTWindow{UsedPercent: 82, WindowMinutes: 43200, ResetAt: time.Now().Add(23 * 24 * time.Hour).Unix()},
		Secondary: &ChatGPTWindow{UsedPercent: 0},
		PlanType:  "free",
	}
	m := q.ToMeter("gorilla@example.com")

	if m.Account != "gorilla@example.com" || m.Provider != ChatGPTProviderName {
		t.Fatalf("meter is %q / %q", m.Provider, m.Account)
	}
	if m.Plan != "free" {
		t.Errorf("plan lost: %q", m.Plan)
	}
	if len(m.Bars) != 2 {
		t.Fatalf("got %d bars, want primary and secondary", len(m.Bars))
	}
	// 82% used => exactly 0.18 left. Exact equality on purpose: the (100-x)/100
	// form is exact at round percentages and the 1-x/100 form is not.
	if m.Bars[0].Remaining != 0.18 {
		t.Errorf("primary remaining = %v, want exactly 0.18", m.Bars[0].Remaining)
	}
	// The label comes from the wire. 43,200 minutes is 30 days: Monthly.
	if m.Bars[0].Label != "Monthly limit" {
		t.Errorf("label = %q, want Monthly limit (from the wire, never a constant)", m.Bars[0].Label)
	}
	if m.Bars[1].Label != "Secondary usage limit" {
		t.Errorf("secondary label = %q", m.Bars[1].Label)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("adapted meter is invalid: %v", err)
	}
}

// Nothing read yet is a REAL third state. This backend reports usage on its
// replies, so before the first request there is genuinely nothing to show, and
// reporting 100% there would be a lie in the expensive direction.
func TestAnUnreadChatGPTMeterIsPendingNotFull(t *testing.T) {
	m := (&ChatGPTQuota{}).ToMeter("gorilla@example.com")
	if !m.Pending {
		t.Error("an unread meter was not marked Pending")
	}
	if len(m.Bars) != 0 {
		t.Errorf("an unread meter produced %d bars; blank and 100%% are both lies", len(m.Bars))
	}
	if err := m.Validate(); err != nil {
		t.Errorf("a pending meter should validate: %v", err)
	}
}

// A missing reset time must stay missing. Unknown dressed as a number is the
// failure this whole subsystem is built to avoid.
func TestAnAbsentResetTimeStaysAbsent(t *testing.T) {
	if got := chatGPTResetPhrase(0); got != "" {
		t.Errorf("no reset time produced %q; unknown must not be given a number", got)
	}
	if got := chatGPTResetPhrase(time.Now().Add(-time.Hour).Unix()); got != "resets now" {
		t.Errorf("an elapsed window produced %q", got)
	}
	for _, tc := range []struct {
		in   time.Duration
		want string
	}{
		// Rounded UP, so the number is an upper bound: the allowance is back
		// by then or sooner. Truncating would say 29m for a window 29m59s out,
		// which sends the user back into a limit that has not lifted.
		{30 * time.Minute, "resets in 30m"},
		{5 * time.Hour, "resets in 5h"},
		{23 * 24 * time.Hour, "resets in 23d"},
	} {
		got := chatGPTResetPhrase(time.Now().Add(tc.in).Unix())
		if got != tc.want {
			t.Errorf("%v => %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A nil summary must not panic: /usage runs before any fetch has completed.
func TestNilSummaryAdaptsToNothing(t *testing.T) {
	var q *QuotaSummary
	if m := q.ToMeter("a@b.c"); m != nil {
		t.Errorf("a nil summary produced %d meters", len(m))
	}
}
