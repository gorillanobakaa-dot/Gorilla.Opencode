package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/quota"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

// GORILLA OVERRIDE: /usage renders a full panel, not only the one-line summary.
//
// The compact line ("Claude and GPT models: 96%") forced the reader to already
// know that the percentage means REMAINING — a normal user cannot tell 96% left
// from 96% spent, and guessing wrong means either panic or a burned week. The
// panel says both numbers in words ("96% left, 4% used"), draws the bar, and
// colours it by how much is left, green fading to red — the reading a user can
// check at a glance without knowing anything about the wire format.

// quotaPanelMaxWidth caps the panel on wide terminals so the bar stays a
// readable length instead of spanning 200 columns.
const quotaPanelMaxWidth = 80

// quotaBarMaxCells is the bar length at full width, matching the agy CLI's own
// quota view so the two read the same.
const quotaBarMaxCells = 50

// quotaHexColor maps remaining fraction to a hue on the green→red line:
// 1.0 → #00FF00 (hue 120°), 0.5 → #FFFF00, 0.0 → #FF0000 (hue 0°), full
// saturation and value throughout. Blue is always zero on this segment, so the
// two-sextant form below is the whole HSV conversion.
func quotaHexColor(remaining float64) string {
	f := math.Min(1, math.Max(0, remaining))
	h := 120 * f / 60 // position in sextants: [0,2]
	var r, g float64
	if h < 1 { // red → yellow
		r, g = 1, h
	} else { // yellow → green
		r, g = 2-h, 1
	}
	return fmt.Sprintf("#%02X%02X00", int(r*255+0.5), int(g*255+0.5))
}

// bananaStatus is the plain-language verdict under the bar, in project voice —
// this is GORILLA OpenCode, quota is bananas. Thresholds are coarse on
// purpose: the exact number is printed right next to it, so the bananas carry
// mood, not data. Emoji live ONLY in the scrollback panel, never in the footer
// line: the footer is part of the inline frame, where a width the terminal
// disagrees with strands debris (see the trap list in CLAUDE.md).
func bananaStatus(remaining float64) string {
	switch {
	case remaining >= 0.5:
		return "🍌🍌🍌 Loaded up on bananas... let's go nuts."
	case remaining >= 1.0/3:
		return "🍌🍌 Running low on bananas..."
	case remaining >= 0.2:
		return "🍌 Yeah... just a few bananas left."
	case remaining > 0:
		return "🦍 Banana emergency! Scraping the peel..."
	default:
		return "🦍 No more bananas for today."
	}
}

// barCellColor is the fixed colour of cell i on the gauge scale: red at the
// left end, green at the right. Split out so the mapping is testable — the
// rendered ANSI is stripped in a non-TTY test run.
func barCellColor(i, cells int) string {
	return quotaHexColor(float64(i+1) / float64(cells))
}

// quotaBar draws "[████░░░░]" as a thermometer scale: every cell has a FIXED
// colour (red left end → green right end, via barCellColor), and the fill
// recedes leftward as quota burns. Two signals in one glance: the tip of the
// fill is always the colour of the current level (green when full, red when
// nearly gone — the same value the old single-colour bar showed), and a
// healthy bar shows the whole green-to-red spectrum.
// A non-zero remainder never rounds down to an empty bar: "a sliver left" and
// "nothing left" must not look identical.
func quotaBar(remaining float64, cells int) string {
	f := math.Min(1, math.Max(0, remaining))
	filled := int(f*float64(cells) + 0.5)
	if f > 0 && filled == 0 {
		filled = 1
	}
	if filled > cells {
		filled = cells
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < filled; i++ {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(barCellColor(i, cells))).Render("█"))
	}
	rest := lipgloss.NewStyle().Foreground(theme.CurrentTheme().TextMuted())
	sb.WriteString(rest.Render(strings.Repeat("░", cells-filled)))
	sb.WriteString("]")
	return sb.String()
}

// quotaResetPhrase turns an RFC3339 reset time into words, or "" if absent or
// unparseable — same floor-to-days reading as the footer line, so the two views
// never disagree.
func quotaResetPhrase(resetTime string, now time.Time) string {
	if resetTime == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, resetTime)
	if err != nil {
		return ""
	}
	d := int(t.Sub(now).Hours() / 24)
	switch {
	case d < 0:
		return ""
	case d == 0:
		return "resets today"
	default:
		return fmt.Sprintf("resets in %dd", d)
	}
}

// renderQuotaPanel renders the full Models & Quota view for the scrollback:
// the Antigravity weekly groups (when signed in), then any configured paid
// providers with a balance endpoint. Pure: no network, no wall clock, no
// terminal — testable headlessly.
func renderQuotaPanel(q *auth.QuotaSummary, account string, balances []quota.Reading, width int, now time.Time) string {
	if (q == nil || len(q.Groups) == 0) && len(balances) == 0 {
		return "  Antigravity: no quota groups reported"
	}
	w := width
	if w <= 0 || w > quotaPanelMaxWidth {
		w = quotaPanelMaxWidth
	}
	cells := quotaBarMaxCells
	// 4 indent + 2 brackets + 1 space + len("100.00%") = 14 columns of chrome.
	if max := w - 14; cells > max {
		cells = max
	}
	if cells < 10 {
		cells = 10
	}

	t := theme.CurrentTheme()
	heading := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted())
	// wordwrap, not lipgloss .Width(): .Width pads every line to w with trailing
	// spaces, which the user then drags along when copying from the scrollback.
	wrap := func(s string) string { return wordwrap.String(s, w) }
	// wrapIndent keeps continuation lines under the same indent as the first —
	// without it a wrapped status line dumps its tail ("in 2d") at column 0,
	// seen in the first live screenshot.
	wrapIndent := func(s string, indent int) string {
		pad := strings.Repeat(" ", indent)
		lines := strings.Split(wordwrap.String(s, w-indent), "\n")
		for i := range lines {
			lines[i] = pad + lines[i]
		}
		return strings.Join(lines, "\n")
	}

	var b strings.Builder
	if account != "" {
		b.WriteString(wrapIndent("Account: "+account, 2) + "\n\n")
	}
	first := true
	var groups []auth.QuotaGroup
	if q != nil {
		groups = q.Groups
	}
	for _, g := range groups {
		if len(g.Buckets) == 0 {
			continue
		}
		bk := g.Buckets[0]
		f := math.Min(1, math.Max(0, bk.RemainingFraction))
		left := int(f*100 + 0.5)
		if !first {
			b.WriteString("\n")
		}
		first = false
		b.WriteString(heading.Render(strings.ToUpper(g.DisplayName)) + "\n")
		if g.Description != "" {
			b.WriteString(wrapIndent(g.Description, 2) + "\n")
		}
		// agy 1.1.10 sent displayName "Weekly Limit"; 1.1.11 sends "Weekly
		// Limit Remaining". Append the word only when the wire didn't —
		// caught live as "Weekly Limit Remaining Remaining".
		name := bk.DisplayName
		if name == "" {
			name = "Limit"
		}
		if !strings.HasSuffix(name, "Remaining") {
			name += " Remaining"
		}
		b.WriteString("\n  " + name + "\n")
		b.WriteString("    " + quotaBar(f, cells) + fmt.Sprintf(" %.2f%%", f*100) + "\n")
		status := fmt.Sprintf("%s — %d%% left, %d%% used", bananaStatus(f), left, 100-left)
		if reset := quotaResetPhrase(bk.ResetTime, now); reset != "" {
			status += " · " + reset
		}
		b.WriteString(wrapIndent(status, 4) + "\n")
	}
	if q != nil && q.Description != "" {
		b.WriteString("\n" + muted.Render(wrap(q.Description)) + "\n")
	}
	if len(balances) > 0 {
		if !first {
			b.WriteString("\n")
		}
		b.WriteString(renderBalanceSections(balances, cells, wrapIndent))
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderBalanceSections renders the paid-provider balances (DeepSeek,
// OpenRouter, …) in the same layout and voice as the Antigravity groups: a bar
// only where the provider gives a denominator, the amount in words always, and
// a failed fetch shown as a failed fetch — a provider silently missing from
// the panel reads as "no key configured", which is a lie when the truth is
// "the request failed".
func renderBalanceSections(balances []quota.Reading, cells int, wrapIndent func(string, int) string) string {
	heading := lipgloss.NewStyle().Bold(true)
	var b strings.Builder
	for _, r := range balances {
		b.WriteString(heading.Render(strings.ToUpper(r.Provider)) + "\n")
		if r.Err != "" {
			b.WriteString(wrapIndent("Couldn't check the balance: "+r.Err, 2) + "\n\n")
			continue
		}
		b.WriteString("\n  Balance\n")
		if r.Fraction >= 0 {
			f := math.Min(1, math.Max(0, r.Fraction))
			b.WriteString("    " + quotaBar(f, cells) + fmt.Sprintf(" %.2f%%", f*100) + "\n")
			b.WriteString(wrapIndent(fmt.Sprintf("%s — %s", bananaStatus(f), r.Text), 4) + "\n\n")
			continue
		}
		// No denominator (DeepSeek money, OpenRouter free tier): amount only.
		b.WriteString(wrapIndent(r.Text, 4) + "\n")
		verdict := "🍌 Bananas in stock (no weekly total to draw a bar from)."
		if r.FreeTier {
			verdict = "🍌 Free bananas only — nothing to run out of."
		}
		b.WriteString(wrapIndent(verdict, 4) + "\n\n")
	}
	return b.String()
}
