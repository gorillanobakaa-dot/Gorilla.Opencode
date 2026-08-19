// GORILLA OVERRIDE: this file did not exist upstream.
//
// ASCII-safe drawing characters, and rules that are SIZED rather than typed.
//
// WHY, measured 2026-08-19 at the owner's suggestion. Three separate problems
// with box-drawing characters, all real:
//
//  1. WIDTH IS AMBIGUOUS. U+2500 and friends — and also "...", "·", "*", "up",
//     "down", "<-", "->" — are East Asian *Ambiguous*: one column normally, TWO in
//     a terminal configured for CJK. lipgloss computes width with
//     runewidth's default (ambiguous = 1) while the terminal may draw 2, so
//     every width calculation silently goes wrong by one column per
//     character, and lipgloss responds by WRAPPING. The frame then grows in
//     height, somewhere else, and nothing points at the cause.
//
//  2. BYTE SLICING BREAKS THEM. `s[:5]` on "──────" yields "─\xe2\x94" — half
//     a rune, invalid UTF-8, measured as width 2. For ASCII, byte offset ==
//     rune offset == column, so the three things code habitually assumes are
//     equal actually are.
//
//  3. THEY ARE 30x SLOWER TO MEASURE. lipgloss.Width on a 100-character line:
//     12,361 ns box-drawing against 404 ns ASCII, because runewidth has an
//     ASCII fast path and otherwise falls back to table lookups. Small per
//     line; ~0.5 ms per full redraw on the 2012 laptop this is built for, and
//     free to avoid.
//
// The owner's point, and he is right about the history too: `+----+----+` was
// the portable way to draw tables on DOS and Unix precisely BECAUSE the pretty
// version was not portable — CP437's box glyphs turned to mojibake the moment
// you left that code page. Same failure, different decade.
//
// This is NOT a ban on borders or tables. lipgloss.ASCIIBorder() exists and is
// the right choice for both. What is banned is the decorative continuous line,
// and especially the HARD-CODED one: a rule typed as a fixed run of characters
// is wrong on every terminal that is not the width the author had.
package styles

import "strings"

// ASCII replacements for the ambiguous-width characters that carry layout.
//
// Deliberately not applied to prose punctuation — em-dashes, curly quotes and
// the like are also ambiguous, but they sit inside sentences that get wrapped,
// where a one-column drift is absorbed rather than accumulated into frame
// corruption. Replacing 1,071 em-dashes would be enormous churn for no
// structural gain. The rule is: ambiguous characters may appear in PROSE, never
// in anything that draws or separates.
const (
	// Ellipsis is the truncation marker. THREE columns, not one — any code
	// reserving room for it must reserve 3.
	Ellipsis = "..."
	// Bullet starts a list item.
	Bullet = "*"
	// Sep divides items on one line ("a | b | c").
	Sep = "|"
	// VLine is a vertical rule.
	VLine = "|"
	// HChar is the single character a horizontal rule is built from.
	HChar = "-"
)

// Rule returns a horizontal rule exactly w columns wide.
//
// SIZED, never typed. Every rule this replaced was a hard-coded run of "─"
// chosen to look right in the author's terminal, which means it was wrong in
// every narrower one — too long, so it wrapped, so the frame grew by a row per
// rule. That is the "stacked and overlapping lines" symptom: not one bug, one
// bug per rule, all at once.
func Rule(w int) string {
	if w <= 0 {
		return ""
	}
	return strings.Repeat(HChar, w)
}

// RuleLabel returns a labelled rule exactly w columns wide:
//
//	-- WHAT THIS COSTS ------------------------------
//
// If w is too narrow for the label, the label wins and the rule disappears —
// the words are the content, the dashes are decoration, and decoration is what
// gets dropped when space runs out.
func RuleLabel(label string, w int) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return Rule(w)
	}
	head := HChar + HChar + " " + label + " "
	if len(head) >= w {
		// No room for both. The words win — but they are still CUT to w,
		// because a heading that overflows its container is the very failure
		// this file exists to stop, and "the label is important" is not a
		// reason to let it wrap.
		r := []rune(label)
		if len(r) > w {
			r = r[:w]
		}
		out := string(r)
		if len(head) < w {
			out = head[:w]
		}
		return out
	}
	return head + Rule(w-len(head))
}
