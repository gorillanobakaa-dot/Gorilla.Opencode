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

func balanceFixture() []quota.Reading {
	return []quota.Reading{
		{Provider: "DeepSeek", Text: "110.00 CNY available", Fraction: quota.FractionUnknown},
		{Provider: "OpenRouter", Text: "$3.50 of $10.00 credits left", Fraction: 0.35},
	}
}

var quotaNow = time.Date(2026, 8, 3, 14, 34, 46, 0, time.UTC) // 7d before Gemini reset

func TestQuotaPanelSaysLeftAndUsedInWords(t *testing.T) {
	t.Parallel()
	got := renderQuotaPanel(quotaFixture(), "user@example.com", nil, 80, quotaNow)
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
		{0.5, "🍌🍌🍌 Loaded up on bananas... let's go nuts."},
		{0.4, "🍌🍌 Running low on bananas..."},
		{1.0 / 3, "🍌🍌 Running low on bananas..."},
		{0.25, "🍌 Yeah... just a few bananas left."},
		{0.2, "🍌 Yeah... just a few bananas left."},
		{0.1, "🦍 Banana emergency! Scraping the peel..."},
		{0, "🦍 No more bananas for today."},
	}
	for _, c := range cases {
		if got := bananaStatus(c.remaining); got != c.want {
			t.Errorf("bananaStatus(%v) = %q, want %q", c.remaining, got, c.want)
		}
	}
}

// Paid providers below the Antigravity groups: a bar where a denominator
// exists (OpenRouter credits), amount-only where it doesn't (DeepSeek money),
// and a failed fetch reported as a failure — never silently dropped.
func TestQuotaPanelRendersBalances(t *testing.T) {
	t.Parallel()
	balances := append(balanceFixture(),
		quota.Reading{Provider: "OpenRouter", Err: "HTTP 401 Unauthorized"})
	got := renderQuotaPanel(quotaFixture(), "user@example.com", balances, 80, quotaNow)
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
	got := renderQuotaPanel(nil, "", balanceFixture(), 80, quotaNow)
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
		if got := strings.Count(bar, "█"); got != c.filled {
			t.Errorf("remaining %.4f: %d filled cells, want %d\nbar: %s", c.remaining, got, c.filled, bar)
		}
		if got := strings.Count(bar, "░"); got != 50-c.filled {
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
		got := renderQuotaPanel(quotaFixture(), "user@example.com", balanceFixture(), w, quotaNow)
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
		if got := renderQuotaPanel(q, "", nil, 80, quotaNow); !strings.Contains(got, "no quota groups") {
			t.Errorf("empty summary must say so, got %q — silence and success must never look alike", got)
		}
	}
}
