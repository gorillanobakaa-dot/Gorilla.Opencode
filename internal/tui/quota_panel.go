package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
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

// quotaHexColor maps remaining fraction to a hue on the green->red line:
// 1.0 -> #00FF00 (hue 120°), 0.5 -> #FFFF00, 0.0 -> #FF0000 (hue 0°), full
// saturation and value throughout. Blue is always zero on this segment, so the
// two-sextant form below is the whole HSV conversion.
func quotaHexColor(remaining float64) string {
	f := math.Min(1, math.Max(0, remaining))
	h := 120 * f / 60 // position in sextants: [0,2]
	var r, g float64
	if h < 1 { // red -> yellow
		r, g = 1, h
	} else { // yellow -> green
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
// bananaTier is the rung on the ladder, 8 (loaded) down to 0 (empty). The
// alert system compares tiers between readings, so the ladder must live in
// exactly one place — this switch — or a threshold edit desynchronises the
// panel wording from the crossing alerts. Below 20% the band splits into
// sub-tiers so the gorilla's bulletins escalate as the barrel empties — and
// every sub-tier crossing fires its own live alert.
func bananaTier(remaining float64) int {
	switch {
	case remaining >= 2.0/3:
		return 8
	case remaining >= 0.5:
		return 7
	case remaining >= 1.0/3:
		return 6
	case remaining >= 0.2:
		return 5
	case remaining >= 0.15:
		return 4
	case remaining >= 0.10:
		return 3
	case remaining >= 0.05:
		return 2
	case remaining > 0:
		return 1
	default:
		return 0
	}
}

func bananaStatus(remaining float64) string {
	switch bananaTier(remaining) {
	case 8:
		return "🍌🍌🍌 Loaded up on bananas... let's go nuts."
	case 7:
		return "🍌🍌 You're halfway through your bananas..."
	case 6:
		return "🍌🍌 Running low on bananas..."
	case 5:
		return "🍌 Yeah... just a few bananas left."
	case 4:
		return "🦍 Banana emergency! Scraping the peel..."
	case 3:
		return "🦍 This is not a drill. The barrel has a bottom and I can see it."
	case 2:
		return "🦍 Rationing mode: sniff the banana, don't eat it."
	case 1:
		return "🦍 Last banana spotted. Nobody make any sudden prompts."
	default:
		return "🦍 Zero bananas. Even the peel is gone."
	}
}

// bananaAlerts compares a new quota reading against the previous fractions
// and returns one announcement per group whose banana tier DROPPED, plus the
// fractions to store for next time. Rises (the weekly reset) stay silent, and
// a group never seen before only seeds — announcing a tier on first sight
// would fire a spurious "alert" at every session start.
func bananaAlerts(prev map[string]float64, meters []quota.Meter) (alerts []string, next map[string]float64) {
	next = make(map[string]float64)
	for _, m := range meters {
		if len(m.Bars) == 0 {
			continue
		}
		// Key by provider AND account. Two sign-ins can both report a "Weekly
		// Limit"; keying on the label alone would let one account's drop raise
		// an alert naming the other's.
		key := m.Provider + "\x00" + m.Account
		f := math.Min(1, math.Max(0, m.Bars[0].Remaining))
		next[key] = f
		old, seen := prev[key]
		if seen && bananaTier(f) < bananaTier(old) {
			alerts = append(alerts, fmt.Sprintf("%s: %s: %d%% left",
				bananaStatus(f), m.Provider, int(f*100+0.5)))
		}
	}
	return alerts, next
}

// barCellColor is the fixed colour of cell i on the gauge scale: red at the
// left end, green at the right. Split out so the mapping is testable — the
// rendered ANSI is stripped in a non-TTY test run.
func barCellColor(i, cells int) string {
	return quotaHexColor(float64(i+1) / float64(cells))
}

// stripBananaEmoji removes the emoji from a banana line so it can ride the
// FOOTER status bar. The footer is inline-frame real estate: a glyph the
// terminal measures differently than we do under-erases a row per render and
// strands debris (see the trap list in CLAUDE.md). Scrollback keeps the
// gorilla; the frame gets his words only.
func stripBananaEmoji(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r >= 0x1F000 { // supplementary-plane pictographs: 🍌 U+1F34C, 🦍 U+1F98D
			return -1
		}
		return r
	}, s))
}

// quotaBar draws "[████░░░░]" as a thermometer scale: every cell has a FIXED
// colour (red left end -> green right end, via barCellColor), and the fill
// recedes leftward as quota burns. Two signals in one glance: the tip of the
// fill is always the colour of the current level (green when full, red when
// nearly gone — the same value the old single-colour bar showed), and a
// healthy bar shows the whole green-to-red spectrum.
// A non-zero remainder never rounds down to an empty bar: "a sliver left" and
// "nothing left" must not look identical.
// DO NOT "MODERNISE" THIS TO ASCII. Locked by TestQuotaBarKeepsItsSolidBody.
//
// GORILLA OVERRIDE (2026-08-20, restoring 2026-08-19). This bar was
// [████░░░░] and a codebase-wide ASCII sweep (5e4cd97, 81 files) turned it into
// [####....]. The owner had asked for a fix to misaligned lines in the PROMPT;
// the change was applied by pattern-match across the whole tree and took two
// days of deliberate design with it. The gradient survived - only the glyphs
// were swapped - but a solid block reads as a METER and a row of # reads as
// TEXT, which is the entire point of the thing.
//
// WHY THE ASCII RULE DOES NOT APPLY HERE, stated so the next sweep stops:
// styles/ascii.go exists because ambiguous-width characters make lipgloss
// mis-measure a frame and WRAP it, and the damage shows up as height somewhere
// else. That is a real hazard for chrome that WRAPS CONTENT. This panel is
// printed to SCROLLBACK with tea.Println (see tui.go, the msg.summary branch),
// not into the persistent inline frame, and it sizes its own cells with
// explicit chrome accounting. Worst case on a CJK-configured terminal is one
// wrapped line in scrollback that immediately scrolls away. It cannot corrupt
// the one-row footer, which is what the rule protects.
//
// U+2588 FULL BLOCK is East Asian Ambiguous; U+2591 LIGHT SHADE is Neutral.
// Only the first carries even the theoretical risk, in the one context where it
// cannot bite.
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
			Foreground(lipgloss.Color(barCellColor(i, cells))).Render("\u2588"))
	}
	rest := lipgloss.NewStyle().Foreground(theme.CurrentTheme().TextMuted())
	sb.WriteString(rest.Render(strings.Repeat("\u2591", cells-filled)))
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

// chatGPTMeter carries the active ChatGPT session's meter into the pure
// renderer. A nil pointer means "not the active provider"; a non-nil one with
// an Empty() quota means "active, but no reading yet this session".
// renderMeterSection renders ONE meter: heading, optional note, then a bar per
// limit, using the same banana ladder and colour ramp as everything else. That
// shared ladder is why quotaBar, bananaStatus and bananaTier all take a bare
// fraction.
//
// A meter that fails Validate() is REPORTED, not drawn. An unnamed reading is
// precisely the wrong-barrel bug (see internal/quota/meter.go), and silently
// rendering one would put the type back to being decorative.
func renderMeterSection(m quota.Meter, inlineAccount bool, cells int,
	wrapIndent func(string, int) string, now time.Time,
) string {
	heading := lipgloss.NewStyle().Bold(true)
	var b strings.Builder

	title := strings.ToUpper(m.Provider)
	if inlineAccount && m.Account != "" {
		title += " - " + m.Account
		if m.Plan != "" {
			title += " (" + m.Plan + ")"
		}
	}
	b.WriteString(heading.Render(title) + "\n")

	if err := m.Validate(); err != nil {
		b.WriteString(wrapIndent("cannot show this meter: "+err.Error(), 2) + "\n")
		return b.String()
	}
	if m.Err != "" {
		b.WriteString(wrapIndent("could not read usage: "+m.Err, 2) + "\n")
		return b.String()
	}
	// "Not known yet" is a real third state beside "no meter" and "N% left".
	// Rendering nothing, or 100%, would both be lies.
	if m.Pending {
		b.WriteString(wrapIndent("No usage reading yet this session. This backend "+
			"reports usage on its replies, so the meter appears after the first "+
			"question.", 2) + "\n")
		return b.String()
	}
	if m.Note != "" {
		b.WriteString(wrapIndent(m.Note, 2) + "\n")
	}

	for _, bar := range m.Bars {
		// A limit with no honest denominator gets its amount in words and NO
		// bar. Unknown and plenty-left must never look alike.
		if !bar.Drawable(m.Kind) {
			text := bar.Text
			if text == "" {
				text = "no figure reported"
			}
			b.WriteString("\n  " + bar.Label + "\n")
			b.WriteString(wrapIndent(text, 4) + "\n")
			continue
		}
		f := math.Min(1, math.Max(0, bar.Remaining))
		left := int(f*100 + 0.5)
		// agy 1.1.10 sent "Weekly Limit"; 1.1.11 sends "Weekly Limit
		// Remaining". Append the word only when the wire did not, or it reads
		// "Weekly Limit Remaining Remaining" (seen live).
		name := bar.Label
		if !strings.HasSuffix(name, "Remaining") {
			name += " Remaining"
		}
		b.WriteString("\n  " + name + "\n")
		b.WriteString("    " + quotaBar(f, cells) + fmt.Sprintf(" %.2f%%", f*100) + "\n")
		status := fmt.Sprintf("%s: %d%% left, %d%% used", bananaStatus(f), left, 100-left)
		if phrase := meterResetPhrase(bar.Reset, now); phrase != "" {
			status += " | " + phrase
		}
		b.WriteString(wrapIndent(status, 4) + "\n")
	}
	if m.Footer != "" {
		muted := lipgloss.NewStyle().Foreground(theme.CurrentTheme().TextMuted())
		b.WriteString("\n" + muted.Render(wrapIndent(m.Footer, 0)) + "\n")
	}
	return b.String()
}

// meterResetPhrase accepts either an already-worded phrase (the ChatGPT adapter
// produces "resets in 23d") or a raw timestamp string from the Antigravity wire,
// which quotaResetPhrase knows how to word. Empty stays empty: a window with no
// stated reset must not be given an invented one.
func meterResetPhrase(reset string, now time.Time) string {
	if reset == "" {
		return ""
	}
	if strings.HasPrefix(reset, "resets") {
		return reset
	}
	return quotaResetPhrase(reset, now)
}

// renderQuotaPanel renders the full Models & Quota view for the scrollback.
// Pure: no network, no wall clock, no terminal, so it is testable headlessly.
//
// GORILLA OVERRIDE (2026-08-23): this took (*auth.QuotaSummary, account string,
// *chatGPTMeter) - numbers and the name they belonged to as separate arguments
// that nothing forced to agree. That is how the ANTIGRAVITY weekly quota came to
// be printed under a ChatGPT session. It now takes []quota.Meter, where every
// reading carries its own account, so the mismatch is unrepresentable rather
// than merely guarded against.
func renderQuotaPanel(meters []quota.Meter, balances []quota.Reading, width int, now time.Time) string {
	if len(meters) == 0 && len(balances) == 0 {
		return "  No quota or balance information available"
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

	// wordwrap, not lipgloss .Width(): .Width pads every line to w with trailing
	// spaces, which the user then drags along when copying from the scrollback.
	wrapIndent := func(s string, indent int) string {
		pad := strings.Repeat(" ", indent)
		lines := strings.Split(wordwrap.String(s, w-indent), "\n")
		for i := range lines {
			lines[i] = pad + lines[i]
		}
		return strings.Join(lines, "\n")
	}

	var b strings.Builder
	first := true
	for i := 0; i < len(meters); {
		// Consecutive meters sharing one account are one block. Antigravity
		// reports several model groups under a single Google sign-in, so the
		// account is stated once above them; a provider reporting a single
		// meter carries its account inline in the heading instead. Either way
		// the account appears, which is the invariant that matters.
		run := 1
		for i+run < len(meters) && meters[i+run].Account == meters[i].Account {
			run++
		}
		acct := meters[i].Account
		if run > 1 && acct != "" {
			if !first {
				b.WriteString("\n")
			}
			first = false
			line := "Account: " + acct
			if p := meters[i].Plan; p != "" {
				line += " (" + p + ")"
			}
			b.WriteString(wrapIndent(line, 2) + "\n")
			// The account line and the first heading under it are one block;
			// the per-meter separator below supplies the single blank between.
		}
		for _, m := range meters[i : i+run] {
			if !first {
				b.WriteString("\n")
			}
			first = false
			b.WriteString(renderMeterSection(m, run == 1, cells, wrapIndent, now))
		}
		i += run
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
// OpenRouter, ...) in the same layout and voice as the Antigravity groups: a bar
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
			b.WriteString(wrapIndent(fmt.Sprintf("%s: %s", bananaStatus(f), r.Text), 4) + "\n\n")
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
