package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/quota"
)

// Fixture mirroring the captured retrieveUserQuotaSummary body in
// internal/auth/antigravity_quota_test.go — same numbers, so the two views can
// be checked against each other.
func quotaFixture() *auth.QuotaSummary {
	return &auth.QuotaSummary{
		Groups: []auth.QuotaGroup{
			{
				DisplayName: "Gemini Models",
				Description: "Models within this group: Gemini Flash, Gemini Pro",
				Buckets: []auth.QuotaBucket{{
					DisplayName: "Weekly Limit", Window: "weekly",
					ResetTime: "2026-08-10T14:34:46Z", RemainingFraction: 0.3094944,
				}},
			},
			{
				DisplayName: "Claude and GPT models",
				Description: "Models within this group: Claude Opus, Claude Sonnet, GPT-OSS",
				// agy 1.1.11 live shape: displayName already ends "Remaining"
				// (1.1.10 sent bare "Weekly Limit" — first group above). The
				// renderer must not double the word; caught live 2026-08-11 as
				// "Weekly Limit Remaining Remaining".
				Buckets: []auth.QuotaBucket{{
					DisplayName: "Weekly Limit Remaining", Window: "weekly",
					ResetTime: "2026-08-10T18:14:14Z", RemainingFraction: 1,
				}},
			},
		},
		Description: "Within each group, models share a weekly limit.",
	}
}

// meterFixture adapts the wire fixture through the real adapter, so these tests
// exercise the same path production does rather than hand-building meters.
func meterFixture(account string) []quota.Meter {
	return quotaFixture().ToMeter(account)
}

func balanceFixture() []quota.Reading {
	return []quota.Reading{
		{Provider: "DeepSeek", Text: "110.00 CNY available", Fraction: quota.FractionUnknown},
		{Provider: "OpenRouter", Text: "$3.50 of $10.00 credits left", Fraction: 0.35},
	}
}

var quotaNow = time.Date(2026, 8, 3, 14, 34, 46, 0, time.UTC) // 7d before Gemini reset

func TestQuotaPanelSaysLeftAndUsedInWords(t *testing.T) {
	t.Parallel()
	got := renderQuotaPanel(meterFixture("user@example.com"), nil, 80, quotaNow)
	if strings.Contains(got, "Remaining Remaining") {
		t.Error("bucket label doubled: the wire's displayName already says Remaining")
	}
	for _, want := range []string{
		"Account: user@example.com",
		"GEMINI MODELS",
		"CLAUDE AND GPT MODELS",
		"Models within this group: Gemini Flash, Gemini Pro",
		"Weekly Limit Remaining",
		"30.95%",             // exact remaining, two decimals like the agy CLI
		"31% left, 69% used", // the plain-language reading — the point of the panel
		"100% left, 0% used", // untouched group
		"resets in 7d",
		"🍌 Yeah... just a few bananas left.", // 31% is in the 20–33% band
		"🍌🍌🍌 Loaded up on bananas",           // untouched group
		"share a weekly limit",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("panel missing %q\n--- got ---\n%s", want, got)
		}
	}
}

// The banana ladder: more alarming as the meter drops, gorilla when it's gone.
// Boundary cases pinned so a threshold edit is a deliberate act.
func TestBananaLadder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		remaining float64
		want      string
	}{
		{1, "🍌🍌🍌 Loaded up on bananas... let's go nuts."},
		{2.0 / 3, "🍌🍌🍌 Loaded up on bananas... let's go nuts."},
		{0.59, "🍌🍌 You're halfway through your bananas..."},
		{0.5, "🍌🍌 You're halfway through your bananas..."},
		{0.49, "🍌🍌 Running low on bananas..."},
		{0.4, "🍌🍌 Running low on bananas..."},
		{1.0 / 3, "🍌🍌 Running low on bananas..."},
		{0.25, "🍌 Yeah... just a few bananas left."},
		{0.2, "🍌 Yeah... just a few bananas left."},
		// The emergency band escalates in 5% steps — each crossing fires its
		// own live alert.
		{0.19, "🦍 Banana emergency! Scraping the peel..."},
		{0.15, "🦍 Banana emergency! Scraping the peel..."},
		{0.12, "🦍 This is not a drill. The barrel has a bottom and I can see it."},
		{0.10, "🦍 This is not a drill. The barrel has a bottom and I can see it."},
		{0.07, "🦍 Rationing mode: sniff the banana, don't eat it."},
		{0.05, "🦍 Rationing mode: sniff the banana, don't eat it."},
		{0.03, "🦍 Last banana spotted. Nobody make any sudden prompts."},
		{0.001, "🦍 Last banana spotted. Nobody make any sudden prompts."},
		{0, "🦍 Zero bananas. Even the peel is gone."},
	}
	for _, c := range cases {
		if got := bananaStatus(c.remaining); got != c.want {
			t.Errorf("bananaStatus(%v) = %q, want %q", c.remaining, got, c.want)
		}
	}
}

// Crossing detection: a tier drop announces, same tier stays silent, a rise
// (the weekly reset) stays silent, and a never-seen group only seeds.
func TestBananaAlerts(t *testing.T) {
	t.Parallel()
	const acct = "user@example.com"
	meters := func(f float64) []quota.Meter {
		return (&auth.QuotaSummary{Groups: []auth.QuotaGroup{{
			DisplayName: "Claude and GPT models",
			Buckets:     []auth.QuotaBucket{{DisplayName: "Weekly Limit", RemainingFraction: f}},
		}}}).ToMeter(acct)
	}
	// The baseline is keyed by provider AND account, so two sign-ins reporting
	// the same window name cannot alert about each other. Built through the
	// same helper the code uses rather than typed out, or the test would pass
	// against a keying change that breaks production.
	seed := func(f float64) map[string]float64 {
		_, next := bananaAlerts(nil, meters(f))
		return next
	}

	// Observed live 2026-08-11: 59% -> 30% in five minutes crossed two tiers
	// invisibly. The alert must fire and describe the CURRENT tier.
	alerts, next := bananaAlerts(seed(0.59), meters(0.2971))
	if len(alerts) != 1 || !strings.Contains(alerts[0], "just a few bananas") ||
		!strings.Contains(alerts[0], "Claude and GPT models: 30% left") {
		t.Errorf("59%%->30%% must announce the current tier with the number, got %v", alerts)
	}
	if len(next) != 1 {
		t.Fatalf("expected one baseline entry, got %v", next)
	}
	for k, v := range next {
		if v != 0.2971 {
			t.Errorf("baseline not updated: %v", next)
		}
		if !strings.Contains(k, acct) {
			t.Errorf("baseline key %q does not name the account; two sign-ins "+
				"reporting the same window name would alert about each other", k)
		}
	}
	// Same tier: silent.
	if alerts, _ := bananaAlerts(seed(0.59), meters(0.55)); len(alerts) != 0 {
		t.Errorf("no crossing must mean no alert, got %v", alerts)
	}
	// Weekly reset (rise): silent.
	if alerts, _ := bananaAlerts(seed(0.05), meters(1)); len(alerts) != 0 {
		t.Errorf("a rise must stay silent, got %v", alerts)
	}
	// First sighting: seeds, no spurious alert.
	alerts, next = bananaAlerts(nil, meters(0.1))
	if len(alerts) != 0 || len(next) != 1 {
		t.Errorf("a never-seen group must seed silently, got %v / %v", alerts, next)
	}
	// A DIFFERENT account at the same tier must not inherit the first one's
	// baseline. This is the wrong-barrel bug in its alert form.
	other := (&auth.QuotaSummary{Groups: []auth.QuotaGroup{{
		DisplayName: "Claude and GPT models",
		Buckets:     []auth.QuotaBucket{{DisplayName: "Weekly Limit", RemainingFraction: 0.2}},
	}}}).ToMeter("someone-else@example.com")
	if alerts, _ := bananaAlerts(seed(0.9), other); len(alerts) != 0 {
		t.Errorf("one account's drop alerted against another account's baseline: %v", alerts)
	}
}

func TestStripBananaEmoji(t *testing.T) {
	t.Parallel()
	in := "🦍 Last banana spotted. Nobody make any sudden prompts. — Claude and GPT models: 3% left"
	got := stripBananaEmoji(in)
	if strings.ContainsRune(got, '🦍') || strings.ContainsRune(got, '🍌') {
		t.Errorf("emoji survived into the footer copy: %q", got)
	}
	if !strings.HasPrefix(got, "Last banana spotted") || !strings.Contains(got, "3% left") {
		t.Errorf("words must survive intact: %q", got)
	}
}

// Paid providers below the Antigravity groups: a bar where a denominator
// exists (OpenRouter credits), amount-only where it doesn't (DeepSeek money),
// and a failed fetch reported as a failure — never silently dropped.
func TestQuotaPanelRendersBalances(t *testing.T) {
	t.Parallel()
	balances := append(balanceFixture(),
		quota.Reading{Provider: "OpenRouter", Err: "HTTP 401 Unauthorized"})
	got := renderQuotaPanel(meterFixture("user@example.com"), balances, 80, quotaNow)
	for _, want := range []string{
		"DEEPSEEK",
		"110.00 CNY available",
		"🍌 Bananas in stock (no weekly total to draw a bar from).",
		"OPENROUTER",
		"35.00%",
		"$3.50 of $10.00 credits left",
		"🍌🍌 Running low on bananas...",
		"Couldn't check the balance: HTTP 401 Unauthorized",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("panel missing %q\n--- got ---\n%s", want, got)
		}
	}
}

// A DeepSeek-only user (no Antigravity sign-in) still gets a panel.
func TestQuotaPanelBalancesWithoutAntigravity(t *testing.T) {
	t.Parallel()
	got := renderQuotaPanel(nil, balanceFixture(), 80, quotaNow)
	if strings.Contains(got, "no quota groups") {
		t.Fatalf("balances present but panel claims no quota groups:\n%s", got)
	}
	if !strings.Contains(got, "DEEPSEEK") || !strings.Contains(got, "OPENROUTER") {
		t.Errorf("balance sections missing:\n%s", got)
	}
}

// The bar must scale with the remaining fraction: 0.3094944 of a 50-cell bar
// rounds to 15 filled cells, a full group fills all 50, and a sliver never
// rounds down to an empty bar (empty must mean zero).
func TestQuotaBarScalesWithRemaining(t *testing.T) {
	t.Parallel()
	cases := []struct {
		remaining float64
		filled    int
	}{
		{1, 50},
		{0.3094944, 15},
		{0.001, 1}, // sliver ≠ nothing
		{0, 0},
	}
	for _, c := range cases {
		bar := quotaBar(c.remaining, 50)
		// U+2588 / U+2591, not '#' / '.' — the meter has a solid body again
		// (restored 2026-08-20; see quota_locked_test.go for why it is locked).
		if got := strings.Count(bar, "\u2588"); got != c.filled {
			t.Errorf("remaining %.4f: %d filled cells, want %d\nbar: %s", c.remaining, got, c.filled, bar)
		}
		if got := strings.Count(bar, "\u2591"); got != 50-c.filled {
			t.Errorf("remaining %.4f: %d empty cells, want %d", c.remaining, got, 50-c.filled)
		}
	}
}

// Green when full, yellow at half, red when empty — the user-facing promise.
// Reference values computed with Python colorsys (hue 120*f, s=1, v=1).
func TestQuotaColorGradientGreenToRed(t *testing.T) {
	t.Parallel()
	cases := map[float64]string{
		1:    "#00FF00",
		0.75: "#80FF00",
		0.5:  "#FFFF00",
		0.25: "#FF8000",
		0:    "#FF0000",
		1.7:  "#00FF00", // out-of-range input clamps, never a garbage colour
		-3:   "#FF0000",
	}
	for f, want := range cases {
		if got := quotaHexColor(f); got != want {
			t.Errorf("quotaHexColor(%v) = %s, want %s", f, got, want)
		}
	}
}

// The gauge scale is fixed: leftmost cell red, rightmost green, and the cell
// at the fill tip carries the same colour the level itself maps to — the tip
// IS the old single-colour signal.
func TestBarCellScaleRedToGreen(t *testing.T) {
	t.Parallel()
	if got := barCellColor(49, 50); got != "#00FF00" {
		t.Errorf("rightmost cell must be pure green, got %s", got)
	}
	if got := barCellColor(0, 50); got != "#FF0A00" {
		t.Errorf("leftmost cell must be near-pure red, got %s", got)
	}
	// Tip of a half-full bar = colour of 50% = yellow.
	if got := barCellColor(24, 50); got != "#FFFF00" {
		t.Errorf("tip cell at half fill must be yellow, got %s", got)
	}
}

// NO LINE MAY BE WIDER THAN THE TERMINAL (see the trap list in CLAUDE.md).
// lipgloss wraps rather than overflows, so the failure mode of an over-wide
// string is extra height — which is why this asserts width per line, ANSI- and
// emoji-aware (a banana is two columns).
func TestQuotaPanelNoLineWiderThanWidth(t *testing.T) {
	t.Parallel()
	for _, w := range []int{80, 60, 40, 25} {
		got := renderQuotaPanel(meterFixture("user@example.com"), balanceFixture(), w, quotaNow)
		for i, line := range strings.Split(got, "\n") {
			if lw := lipgloss.Width(line); lw > w {
				t.Errorf("width %d: line %d is %d columns wide: %q", w, i, lw, line)
			}
		}
		// A wrapped status line must keep its indent: the first live run at
		// ~78 columns dumped the tail ("in 2d") at column 0.
		if strings.Contains(got, "\nin ") {
			t.Errorf("width %d: wrapped continuation lost its indent:\n%s", w, got)
		}
	}
}

func TestQuotaPanelEmptySaysSo(t *testing.T) {
	t.Parallel()
	for _, q := range []*auth.QuotaSummary{nil, {}} {
		got := renderQuotaPanel(q.ToMeter("user@example.com"), nil, 80, quotaNow)
		if !strings.Contains(got, "No quota or balance information available") {
			t.Errorf("nothing to show must SAY so, got %q: silence and success must never look alike", got)
		}
	}
}

// The wrong-barrel regression: when there is no Antigravity summary to show
// (the session spends a different provider), the Antigravity account email must
// NOT be printed. It belongs to the quota, not to the session. Floating it at
// the top over another provider's balances is exactly the bug this guards —
// a signed-in Google address appearing above a ChatGPT session at 97%.
func TestAccountHiddenWhenNoQuotaSummary(t *testing.T) {
	t.Parallel()
	got := renderQuotaPanel(nil, balanceFixture(), 80, quotaNow)
	if strings.Contains(got, "user@example.com") {
		t.Errorf("account email leaked with no quota summary — the wrong-barrel bug:\n%s", got)
	}
	// The paid-provider balances the user genuinely holds must still render.
	// Headings are upper-cased by the renderer, so match that.
	if !strings.Contains(got, "DEEPSEEK") {
		t.Errorf("balances must still show when Antigravity is absent, got %q", got)
	}
}

// The completion of the wrong-barrel fix. Gating the Antigravity meter off a
// ChatGPT session removed a misleading number; this proves the RIGHT number
// takes its place. Before this, /usage on ChatGPT showed nothing at all while
// the user sat at 20% of a monthly limit (observed against Codex, 2026-08-23).
func TestChatGPTMeterRendersWithBananaLadder(t *testing.T) {
	t.Parallel()
	cg := (&auth.ChatGPTQuota{
		Primary:  &auth.ChatGPTWindow{UsedPercent: 80, WindowMinutes: 43200},
		PlanType: "Free",
	}).ToMeter("user@example.com")
	got := renderQuotaPanel([]quota.Meter{cg}, nil, 80, quotaNow)
	for _, want := range []string{
		"CHATGPT - user@example.com (Free)",
		"Monthly limit Remaining",            // label from the WIRE, not hardcoded
		"20.00%",                             // 80% used inverted to remaining
		"20% left, 80% used",                 // both numbers in words
		"🍌 Yeah... just a few bananas left.", // same ladder as every provider
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ChatGPT panel missing %q\n--- got ---\n%s", want, got)
		}
	}
	// The Antigravity account must not appear on a ChatGPT panel.
	if strings.Contains(got, "Account:") {
		t.Errorf("Antigravity account line leaked onto a ChatGPT panel:\n%s", got)
	}
}

// Traffic-driven meter: before the session's first request there is no reading,
// and "not known yet" must be distinguishable from "no meter exists" and from
// a meter reading zero. A blank or a full bar would both be lies.
func TestChatGPTMeterSaysWhenNoReadingYet(t *testing.T) {
	t.Parallel()
	cg := (&auth.ChatGPTQuota{}).ToMeter("user@example.com")
	got := renderQuotaPanel([]quota.Meter{cg}, nil, 80, quotaNow)
	if !strings.Contains(got, "No usage reading yet this session") {
		t.Errorf("an empty meter must say so, not render blank:\n%s", got)
	}
	if strings.Contains(got, "100.00%") || strings.Contains(got, "0.00%") {
		t.Errorf("no reading must not be drawn as a bar:\n%s", got)
	}
}
