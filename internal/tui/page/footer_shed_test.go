package page

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func rows(label string, n int) string {
	out := make([]string, n)
	for i := range out {
		out[i] = label
	}
	return strings.Join(out, "\n")
}

// The footer is redrawn in place, and outside the alternate screen bubbletea erases
// its previous frame by counting logical lines. A footer taller than its budget
// does not merely crowd the conversation — it puts every later erase in the wrong
// place. So it sheds, and it sheds in a deliberate order: the session numbers are
// reference information also available from /context, while the live preview is the
// only sign a reply is arriving at all.
func TestFooterShedsLeastImportantFirst(t *testing.T) {
	live := rows("live", 6)
	prompt := rows("prompt", 3)
	info := rows("info", 2)
	attempts := [][]string{{live, prompt, info}, {live, prompt}, {prompt}}

	cases := []struct {
		budget                         int
		wantLive, wantPrompt, wantInfo bool
		why                            string
	}{
		{budget: 20, wantLive: true, wantPrompt: true, wantInfo: true, why: "everything fits"},
		{budget: 11, wantLive: true, wantPrompt: true, wantInfo: true, why: "exactly fits at 11 rows"},
		{budget: 10, wantLive: true, wantPrompt: true, wantInfo: false, why: "the numbers go first"},
		{budget: 5, wantLive: false, wantPrompt: true, wantInfo: false, why: "only the prompt survives"},
		{budget: 1, wantLive: false, wantPrompt: true, wantInfo: false, why: "nothing fits; the prompt is still shown"},
	}

	for _, c := range cases {
		got := shedToFit(c.budget, attempts)
		has := func(s string) bool { return strings.Contains(got, s) }

		if has("live") != c.wantLive {
			t.Errorf("budget %d: live preview present=%v, want %v (%s)", c.budget, has("live"), c.wantLive, c.why)
		}
		if has("prompt") != c.wantPrompt {
			t.Errorf("budget %d: prompt present=%v, want %v — the prompt is never optional",
				c.budget, has("prompt"), c.wantPrompt)
		}
		if has("info") != c.wantInfo {
			t.Errorf("budget %d: session numbers present=%v, want %v (%s)", c.budget, has("info"), c.wantInfo, c.why)
		}
		// And the result must actually fit, except in the impossible case.
		if c.budget >= 3 && lipgloss.Height(got) > c.budget {
			t.Errorf("budget %d: rendered %d rows, which overflows it", c.budget, lipgloss.Height(got))
		}
	}
}

// An unknown size must render the fullest arrangement rather than nothing: the
// footer is drawn on frames before the first size message arrives, and an empty
// frame reads as a broken program.
func TestUnknownBudgetRendersEverything(t *testing.T) {
	attempts := [][]string{{rows("live", 6), rows("prompt", 3), rows("info", 2)}, {rows("prompt", 3)}}
	for _, budget := range []int{0, -1} {
		got := shedToFit(budget, attempts)
		for _, want := range []string{"live", "prompt", "info"} {
			if !strings.Contains(got, want) {
				t.Errorf("budget %d dropped %q; with no known size there is nothing to fit to",
					budget, want)
			}
		}
	}
}

// Empty parts must not occupy rows. Early in a session there is no reply in flight
// and no modified file, and blank rows in a fixed footer read as a rendering fault.
func TestEmptyPartsDoNotTakeRows(t *testing.T) {
	got := shedToFit(20, [][]string{{"", rows("prompt", 2), ""}})
	if n := lipgloss.Height(got); n != 2 {
		t.Errorf("footer is %d rows for a 2-row prompt with two empty parts; empty parts "+
			"must not reserve space", n)
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("rendered nothing at all")
	}
}

// The ORDER the footer sheds in is a decision, so it is asserted rather than left
// to whoever edits the list next. Without this, swapping the prompt for the session
// numbers passes every other test in this file.
func TestFooterPriorityKeepsThePromptAndShedsTheNumbersFirst(t *testing.T) {
	const live, prompt, info = "LIVE", "PROMPT", "INFO"
	got := footerArrangements(live, prompt, info)

	if len(got) < 2 {
		t.Fatalf("only %d arrangement(s); there is nothing to shed", len(got))
	}

	for i, attempt := range got {
		if !contains(attempt, prompt) {
			t.Errorf("arrangement %d does not include the prompt: %v. The prompt is never "+
				"optional — a footer without it looks like a crash, not a small window", i, attempt)
		}
		if i > 0 && len(attempt) >= len(got[i-1]) {
			t.Errorf("arrangement %d has %d parts, no fewer than the %d before it; the list "+
				"must get smaller or shedding cannot terminate", i, len(attempt), len(got[i-1]))
		}
	}

	// The numbers must go before the preview, not after.
	firstWithout := func(part string) int {
		for i, a := range got {
			if !contains(a, part) {
				return i
			}
		}
		return len(got)
	}
	if firstWithout(info) > firstWithout(live) {
		t.Error("the live preview is shed before the session numbers. The numbers are also " +
			"in /context; the preview is the only indication a reply is arriving.")
	}

	// And the last resort is the prompt alone.
	last := got[len(got)-1]
	if len(last) != 1 || last[0] != prompt {
		t.Errorf("the smallest arrangement is %v, want just the prompt", last)
	}
}

func contains(parts []string, want string) bool {
	for _, p := range parts {
		if p == want {
			return true
		}
	}
	return false
}
