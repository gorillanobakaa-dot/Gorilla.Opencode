package quota

import (
	"strings"
	"testing"
)

// THE INVARIANT THIS TYPE EXISTS FOR. A meter with numbers must name whose
// numbers they are. `/usage` on a ChatGPT session once printed the Antigravity
// weekly quota under a Google email, and it was possible because the figures and
// the account travelled as separate arguments that nothing forced to agree.
func TestAMeterWithNumbersMustNameItsAccount(t *testing.T) {
	orphan := Meter{
		Provider: "ANTIGRAVITY",
		Kind:     KindWindowQuota,
		Bars:     []Bar{{Label: "Weekly Limit", Remaining: 0.91}},
	}
	err := orphan.Validate()
	if err == nil {
		t.Fatal("a meter carrying a reading was accepted with no account. That is the " +
			"exact shape of the bug this type was added to make impossible.")
	}
	if !strings.Contains(err.Error(), "ChatGPT session") {
		t.Errorf("the error should say what went wrong last time, got: %v", err)
	}

	orphan.Account = "someone@example.com"
	if err := orphan.Validate(); err != nil {
		t.Errorf("a named meter was rejected: %v", err)
	}
}

// An errored or not-yet-read meter has nothing to attribute, so it is allowed to
// be anonymous: it prints a reason, not a number.
func TestErroredAndPendingMetersNeedNoAccount(t *testing.T) {
	for _, m := range []Meter{
		{Provider: "CHATGPT", Err: "sign-in expired"},
		{Provider: "CHATGPT", Pending: true},
	} {
		if err := m.Validate(); err != nil {
			t.Errorf("%+v was rejected: %v", m, err)
		}
	}
}

// A percentage passed where a fraction belongs is caught, because 18 and 0.18
// are both plausible-looking and only one is right.
func TestAFractionOverOneIsRefused(t *testing.T) {
	m := Meter{
		Provider: "CHATGPT", Account: "a@b.c", Kind: KindWindowQuota,
		Bars: []Bar{{Label: "Monthly limit", Remaining: 18}},
	}
	if err := m.Validate(); err == nil {
		t.Error("a remaining fraction of 18 was accepted; that is a percentage in a fraction's slot")
	}
}

func TestAnUnlabelledBarIsRefused(t *testing.T) {
	m := Meter{
		Provider: "ANTIGRAVITY", Account: "a@b.c", Kind: KindWindowQuota,
		Bars: []Bar{{Remaining: 0.5}},
	}
	if err := m.Validate(); err == nil {
		t.Error("an unlabelled bar was accepted; the window name comes from the provider")
	}
}

// THE DESIGN DECISION, pinned. A rate limit has a denominator and would happily
// draw full green, looking identical to a quota bar while saying nothing about
// whether the week is affordable. It must never draw one.
func TestARateLimitNeverDrawsABar(t *testing.T) {
	if KindRateLimit.DrawsBar() {
		t.Error("KindRateLimit draws a bar. '92% of your per-minute token budget left' " +
			"reads exactly like a quota and answers a different question.")
	}
	full := Bar{Label: "Per-minute tokens", Remaining: 1.0}
	if full.Drawable(KindRateLimit) {
		t.Error("a full rate-limit bar was drawable; that is the confusing case, not the safe one")
	}
	if !full.Drawable(KindWindowQuota) {
		t.Error("a window quota with a real fraction must draw")
	}
}

// A balance with no ceiling to divide by renders its amount, never an invented
// percentage. DeepSeek reports money left but not what you started with.
func TestABalanceWithNoDenominatorDrawsNoBar(t *testing.T) {
	noCeiling := Bar{Label: "Balance", Remaining: FractionUnknown, Text: "110.00 CNY available"}
	if noCeiling.Drawable(KindBalance) {
		t.Error("a balance with FractionUnknown was drawable; unknown and plenty-left must not look alike")
	}
	withCeiling := Bar{Label: "Balance", Remaining: 0.4}
	if !withCeiling.Drawable(KindBalance) {
		t.Error("a balance that DOES carry a fraction should draw")
	}
}

func TestFromReadingCarriesTheAccountAndTheError(t *testing.T) {
	m := FromReading(Reading{Provider: "OpenRouter", Text: "$4.20", Fraction: 0.42}, "me@example.com")
	if m.Account != "me@example.com" {
		t.Errorf("account lost in adaptation: %q", m.Account)
	}
	if m.Kind != KindBalance {
		t.Errorf("a balance adapted to %v", m.Kind)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("adapted meter is invalid: %v", err)
	}

	// A free-tier account has no barrel to run down. That is a third fact,
	// distinct from full and from empty, so it must not carry a fraction.
	free := FromReading(Reading{Provider: "OpenRouter", Text: "no credits", FreeTier: true}, "me@example.com")
	if free.Bars[0].Drawable(free.Kind) {
		t.Error("a free-tier balance drew a bar; there is no denominator to draw against")
	}

	// A failed fetch says so and carries no invented numbers.
	bad := FromReading(Reading{Provider: "DeepSeek", Err: "timeout"}, "")
	if len(bad.Bars) != 0 {
		t.Error("a failed reading produced bars")
	}
	if err := bad.Validate(); err != nil {
		t.Errorf("an errored meter should validate without an account: %v", err)
	}
}
