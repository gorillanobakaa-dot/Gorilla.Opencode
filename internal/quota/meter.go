package quota

// GORILLA OVERRIDE (2026-08-23): one neutral meter type, and every meter carries
// the account it belongs to.
//
// THE BUG THIS KILLS BY CONSTRUCTION. `/usage` on a ChatGPT session printed the
// ANTIGRAVITY weekly quota and a Google email address: the wrong barrel, under
// the wrong account, with nothing in the type system objecting. It was possible
// because the panel took `*auth.QuotaSummary` plus a loose `account string`
// alongside it, so the numbers and the name they belonged to were two separate
// arguments that nothing forced to agree.
//
// v0.1.115 fixed the symptom by gating on the active provider. A gate is a
// runtime check that somebody has to remember to write. This is the structural
// half the roadmap asked for: a Meter cannot exist without naming its own
// account and provider, so a meter can no longer be rendered under someone
// else's name. There is nothing left to remember.
//
// WHY IT LIVES HERE AND NOT IN internal/auth. Drawing a bar should not require
// importing an OAuth stack. auth owns credentials; this package owns "how much
// is left". auth imports quota to adapt its own types (auth -> quota), never the
// reverse, so this package stays free of tokens, refresh flows and browsers.
//
// THE PART THAT IS DESIGN RATHER THAN PLUMBING. Three different things want to
// be the same bar, and only one of them honestly is:
//
//	kind          example                 honest rendering
//	window quota  Antigravity, Codex      a bar. "how much of my allowance is left"
//	balance       DeepSeek                NO bar unless a denominator exists
//	rate limit    Anthropic, xAI, Groq    NO bar. It has a denominator and will
//	                                      happily draw full green, but "92% of
//	                                      your per-minute token budget" says
//	                                      nothing about whether this week is
//	                                      affordable, and it looks identical to
//	                                      a quota bar
//
// That third row is the trap, which is why Kind is in the type from the start
// rather than bolted on later. balances.go already forbids the failure in its
// own header: unknown and plenty-left must never look alike. Kind generalises
// that rule from one provider to all of them.

import "fmt"

// Kind says what a meter is measuring, which decides whether a bar is honest.
type Kind int

const (
	// KindWindowQuota is an allowance inside a rolling window: a real fraction
	// of a real ceiling, and the only kind that earns a bar.
	KindWindowQuota Kind = iota
	// KindBalance is money or credit left. It earns a bar only when the
	// provider also says what you started with, which most do not.
	KindBalance
	// KindRateLimit is a per-minute or per-hour throttle. It never earns a bar:
	// it has a denominator, so it would draw full green and read exactly like a
	// quota, while saying nothing about whether the week is affordable.
	KindRateLimit
)

func (k Kind) String() string {
	switch k {
	case KindWindowQuota:
		return "window quota"
	case KindBalance:
		return "balance"
	case KindRateLimit:
		return "rate limit"
	}
	return "unknown"
}

// DrawsBar reports whether this kind may be rendered as a proportional bar.
// A KindBalance may, but only when it carries a real fraction; see Bar.Drawable.
func (k Kind) DrawsBar() bool { return k == KindWindowQuota || k == KindBalance }

// Bar is one line within a meter: a named limit and how much of it is left.
type Bar struct {
	// Label is the provider's own word for the window ("Weekly Limit",
	// "Monthly limit"), never a constant of ours. Codex says "Monthly" on a
	// free plan and shows a weekly pair on a paid one, so hardcoding either
	// mislabels every tier it was not written against.
	Label string
	// Remaining is the fraction left, 0..1, or FractionUnknown when the
	// provider reports an amount with no ceiling to divide by.
	Remaining float64
	// Reset is a human phrase for when it refills ("resets in 23d"), empty when
	// the provider did not say. Empty means unknown, never "never".
	Reset string
	// Text is the amount for meters with no fraction ("110.00 CNY available").
	// Rendered instead of a bar, not beside one.
	Text string
}

// Drawable reports whether this specific bar has an honest denominator.
func (b Bar) Drawable(k Kind) bool {
	return k.DrawsBar() && b.Remaining != FractionUnknown && b.Remaining >= 0
}

// Meter is one provider's answer to "how much have I got left", carrying the
// account it belongs to so it can never be shown under another one.
type Meter struct {
	// Provider is the display heading: "ANTIGRAVITY", "CHATGPT", "OPENROUTER".
	Provider string
	// Account is whose allowance this is. REQUIRED, and the whole point of the
	// type: the panel prints it beside the numbers, so a reading physically
	// cannot be attributed to the wrong sign-in.
	Account string
	// Plan is shown beside the account when the provider says one ("free").
	Plan string
	// Kind decides whether Bars may be drawn as bars.
	Kind Kind
	// Note is the provider's own explanation of how the limit works, when it
	// offers one. Shown under the bars.
	Note string
	// Bars are the individual limits. A meter with none is a meter that has
	// nothing to say yet, which is a real third state: see Pending.
	Bars []Bar
	// Pending marks a meter that exists but has not been read yet. This backend
	// reports usage ON ITS REPLIES, so before the session's first request there
	// is genuinely nothing to show. "Not known yet" must stay distinguishable
	// from "no meter" and from "meter says 100%", because blank and full are
	// both lies.
	Pending bool
	// Err is non-empty when the fetch failed. Silence and success must never
	// look alike.
	Err string
}

// Validate refuses a meter that cannot be rendered honestly. Called by the
// panel: an unnamed meter is exactly the bug this type exists to prevent, so it
// is reported rather than drawn.
func (m Meter) Validate() error {
	if m.Provider == "" {
		return fmt.Errorf("meter has no provider heading")
	}
	// An errored or pending meter has nothing to attribute yet, so it is
	// allowed to be anonymous: it prints a reason, not a number.
	if m.Err != "" || m.Pending {
		return nil
	}
	if m.Account == "" {
		return fmt.Errorf("meter for %q names no account: a reading with no owner is "+
			"how the Antigravity quota came to be printed under a ChatGPT session", m.Provider)
	}
	for _, b := range m.Bars {
		if b.Label == "" {
			return fmt.Errorf("meter for %q has an unlabelled bar", m.Provider)
		}
		if b.Remaining > 1 {
			return fmt.Errorf("meter for %q reports %v remaining; a fraction cannot exceed 1 "+
				"(is a percentage being passed where a fraction belongs?)", m.Provider, b.Remaining)
		}
	}
	return nil
}

// FromReading adapts a key-based balance into a Meter. Balances are KindBalance:
// they get a bar only when the provider says what the ceiling was, which is why
// DeepSeek renders an amount and OpenRouter a fraction.
func FromReading(r Reading, account string) Meter {
	m := Meter{
		Provider: r.Provider,
		Account:  account,
		Kind:     KindBalance,
		Err:      r.Err,
	}
	if r.Err != "" {
		return m
	}
	m.Bars = []Bar{{
		Label:     "Balance",
		Remaining: r.Fraction,
		Text:      r.Text,
	}}
	if r.FreeTier {
		// Nothing purchased is a different fact from "barrel full" and from
		// "barrel empty", and the renderer words it accordingly.
		m.Bars[0].Remaining = FractionUnknown
	}
	return m
}
