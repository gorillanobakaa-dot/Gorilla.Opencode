package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/opencode-ai/opencode/internal/tui/theme"
)

// NO LINE IN THE FRAME MAY BE WIDER THAN THE TERMINAL. An over-wide line
// occupies two physical rows but counts as one, so bubbletea's erase
// under-reaches by a row per wrapped line and the frame marches down the
// screen. This is the bug that took three releases to diagnose; every dialog
// gets this test.
func TestResearchDialogNeverExceedsTheTerminalWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 100, 120, 200} {
		m := NewResearchDialogCmp("does X work?")
		m.SetSize(w, 40)
		for _, line := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Errorf("terminal %d: line is %d wide: %q", w, got, line)
			}
		}
	}
}

func TestBothMoneyAndQuotaAlwaysShown(t *testing.T) {
	m := NewResearchDialogCmp("q")
	m.SetSize(140, 50)
	v := m.View()

	// THE BUG: a zero-rate tier got the quota warning INSTEAD of any money
	// statement, so a user on a PAID monthly subscription read "$0.00" and
	// concluded the run was free. Free entitlement and paid subscription are
	// not mutually exclusive, and most of this app's ~300 models are metered.
	hasMoney := strings.Contains(v, "PER MINUTE") ||
		strings.Contains(v, "metered") ||
		strings.Contains(v, "CANNOT PRICE")
	if !hasMoney {
		t.Error("no money statement at all")
	}
	if !strings.Contains(v, "QUOTA") || !strings.Contains(v, "ORDINARY QUESTIONS in tokens") {
		t.Error("quota warning missing — it applies on every tier, paid or not")
	}
	// A bare $0.00 must never stand alone as if it meant free.
	if strings.Contains(v, "$0.00") && !strings.Contains(v, "NOT FREE") {
		t.Error("$0.00 shown without saying a subscription is already paid for")
	}
	// Rejected wordings stay rejected.
	for _, banned := range []string{"separate conversations with the AI", "of a $2 day"} {
		if strings.Contains(v, banned) {
			t.Errorf("rejected wording is back: %q", banned)
		}
	}
}

func TestPricedModelShowsPerMinuteAndPerHour(t *testing.T) {
	// The author asked for cost per minute as the wake-up call, and per hour
	// as the number that actually frightens.
	m := NewResearchDialogCmp("q")
	m.SetSize(140, 50)
	if v := m.View(); strings.Contains(v, "PER MINUTE") {
		if !strings.Contains(v, "Per hour") {
			t.Error("a priced model must show both per-minute and per-hour")
		}
	}
}

func TestSupervisedCarriesTheWarning(t *testing.T) {
	m := NewResearchDialogCmp("q")
	m.SetSize(100, 40)
	for i, opt := range researchOptions {
		if opt.mode == "supervised" {
			m.selected = i
		}
	}
	v := m.View()
	if !strings.Contains(v, "Feeling lucky") {
		t.Error("the supervised warning is missing — it is the mode that doubles the bill")
	}
	if !strings.Contains(strings.ToUpper(v), "DOUBLE") {
		t.Error("the doubling must still be stated")
	}
}

func TestResearchDialogClampsHelperCount(t *testing.T) {
	m := NewResearchDialogCmp("q")
	m.SetSize(100, 40)

	press := func(mm ResearchDialogCmp, key string, n int) ResearchDialogCmp {
		for i := 0; i < n; i++ {
			next, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			mm = next.(ResearchDialogCmp)
		}
		return mm
	}
	if got := press(m, "l", 50).agents; got != 10 {
		t.Errorf("upper clamp: got %d, want 10 — past ten the synthesis degrades", got)
	}
	if got := press(m, "h", 50).agents; got != 4 {
		t.Errorf("lower clamp: got %d, want 4 — fewer than four lanes does not cover the ground", got)
	}
}

// A cancel must never start a run. This costs money, so "escaped" and
// "confirmed" must not be the same message.
func TestResearchDialogCancelDoesNotStartARun(t *testing.T) {
	m := NewResearchDialogCmp("q")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc produced no message")
	}
	msg, ok := cmd().(CloseResearchDialogMsg)
	if !ok {
		t.Fatalf("esc sent %T, expected CloseResearchDialogMsg", cmd())
	}
	if msg.Chosen {
		t.Error("cancel reported Chosen=true — this would spend the user's quota after they said no")
	}
}

func TestResearchDialogConfirmCarriesTheChoice(t *testing.T) {
	m := NewResearchDialogCmp("investigate the thing")
	for i, opt := range researchOptions {
		if opt.mode == "sequential" {
			m.selected = i
		}
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = next.(ResearchDialogCmp)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd().(CloseResearchDialogMsg)
	if !msg.Chosen {
		t.Fatal("enter did not confirm")
	}
	if msg.Mode != "sequential" {
		t.Errorf("mode = %q, want sequential", msg.Mode)
	}
	if msg.Agents != 5 {
		t.Errorf("agents = %d, want 5", msg.Agents)
	}
	if msg.Question != "investigate the thing" {
		t.Errorf("question lost: %q", msg.Question)
	}
}

// Non-vacuous: the mode strings must match what the research tool accepts, or
// the dialog cheerfully sends a mode the tool rejects.
func TestDialogModesMatchTheToolsModes(t *testing.T) {
	want := map[string]bool{"sequential": true, "parallel": true, "supervised": true}
	for _, opt := range researchOptions {
		if !want[opt.mode] {
			t.Errorf("dialog offers %q, which the research tool does not accept", opt.mode)
		}
		delete(want, opt.mode)
	}
	if len(want) != 0 {
		t.Errorf("dialog does not offer these tool modes: %v", want)
	}
}

// On a flat-rate tier every figure is $0.00 and the user learns nothing about
// the scale of what they are setting off. A real metered model must be priced
// alongside, so a number is always on screen.
func TestFlatRateTierStillShowsARealNumber(t *testing.T) {
	m := NewResearchDialogCmp("q")
	m.SetSize(140, 50)
	v := m.View()
	if !strings.Contains(v, "$0.00 metered") {
		t.Skip("this install's helper model is metered; nothing to check")
	}
	if !strings.Contains(v, "IF YOU RAN THIS ON A METERED MODEL") {
		t.Error("flat tier shows no comparison — the user sees only zeros and learns nothing")
	}
	// A RANGE, not one model: quoting only the priciest (o1 pro) was alarmist
	// and told the user nothing about models they might actually use.
	if !strings.Contains(v, "cheapest") || !strings.Contains(v, "dearest") {
		t.Error("must show a price RANGE across providers, not a single extreme")
	}
	if !strings.Contains(v, "PER MINUTE") {
		t.Error("no per-minute figure anywhere; that was the whole point")
	}
}

// The author audited the dialog and found the per-minute figure rested on an
// undocumented constant, and that the quota multiple was simply the helper
// count asserting "1 helper = 1 question". Both were presented as fact. The
// dialog must now show which inputs are measured, which are published, and
// which are guesses.
func TestDialogShowsItsWorking(t *testing.T) {
	m := NewResearchDialogCmp("q")
	m.SetSize(140, 60)
	v := m.View()

	for _, want := range []string{"MEASURED", "PUBLISHED", "ASSUMED"} {
		if !strings.Contains(v, want) {
			t.Errorf("the derivation must label its inputs; %q missing", want)
		}
	}
	// The load-bearing guess must be named as load-bearing.
	if !strings.Contains(v, "rests on") {
		t.Error("the per-minute figure depends on one invented constant; say so")
	}
	// The quota multiple must be derived, not the bare helper count.
	if !strings.Contains(v, "ORDINARY QUESTIONS in tokens") {
		t.Error("quota must be expressed in tokens, not by asserting 1 helper = 1 question")
	}
}

// Colour must carry MEANING, and must not be the only signal.
//
// Every emphasised line used to be one warning colour, so nothing stood out —
// reported as an accessibility problem for ADHD and dyslexic readers, who get
// lost in a uniform block. Money, quota, measured and assumed must be visually
// distinct, AND still distinguishable with colour stripped out.
func TestCostBlockUsesDistinctColoursByMeaning(t *testing.T) {
	m := NewResearchDialogCmp("q")
	m.SetSize(140, 60)

	kinds := map[costKind]bool{}
	for _, l := range m.costLines() {
		kinds[l.kind] = true
	}
	for _, want := range []costKind{kindHeader, kindQuota, kindMeasured, kindPublished, kindAssumed} {
		if !kinds[want] {
			t.Errorf("kind %d never used; the block is visually flat", want)
		}
	}
	if len(kinds) < 5 {
		t.Errorf("only %d distinct kinds — not enough contrast to guide the eye", len(kinds))
	}

	// Colour-blind readers must lose nothing: the words alone must still
	// separate the categories.
	plain := m.View()
	for _, label := range []string{"MEASURED", "PUBLISHED", "ASSUMED", "QUOTA"} {
		if !strings.Contains(plain, label) {
			t.Errorf("%q missing — colour would be the only cue", label)
		}
	}
}

// Inline emphasis: whole-line colour cannot highlight one word mid-sentence,
// and "you have ALREADY paid" carries its weight on that one word.
//
// Tested against emphasise() directly, NOT through View(): with no config
// loaded the dialog takes the "cannot price" branch and those lines never
// render, so a View()-based assertion would be checking a path that does not
// execute. That mistake was made once already here.
func TestEmphasiseHighlightsInlineWithoutEatingText(t *testing.T) {
	th := theme.CurrentTheme()

	got := emphasise("you have «ALREADY» paid", th)
	if strings.Contains(got, "«") || strings.Contains(got, "»") {
		t.Error("markers leaked into the output")
	}
	if !strings.Contains(got, "ALREADY") {
		t.Fatal("the emphasised word was eaten")
	}
	if !strings.Contains(got, "you have ") || !strings.Contains(got, " paid") {
		t.Error("surrounding text was lost")
	}
	// It must actually be styled, or this is a no-op that looks like a feature.
	if !strings.Contains(got, "\x1b[") {
		t.Error("no styling applied — the word is not emphasised at all")
	}

	// Multiple marks in one line.
	multi := emphasise("«A» and «B»", th)
	for _, w := range []string{"A", "and", "B"} {
		if !strings.Contains(multi, w) {
			t.Errorf("lost %q with two marks", w)
		}
	}
	// Unbalanced marker must not panic or truncate.
	if got := emphasise("dangling « marker", th); !strings.Contains(got, "marker") {
		t.Error("unbalanced marker ate the rest of the line")
	}
	// No markers: unchanged.
	if got := emphasise("plain text", th); got != "plain text" {
		t.Errorf("unmarked text altered: %q", got)
	}
}

// The dialog must FIT. Extra wrapped lines pushed the key hints off the bottom
// of the screen, so the user could not see how to confirm or cancel.
func TestDialogFitsOnScreenWithItsKeyHints(t *testing.T) {
	for _, sz := range []struct{ w, h int }{{176, 48}, {120, 40}, {100, 32}} {
		m := NewResearchDialogCmp("does X work on this machine?")
		m.SetSize(sz.w, sz.h)
		for i, opt := range researchOptions {
			if opt.mode == "supervised" { // the tallest variant
				m.selected = i
			}
		}
		m.agents = 10
		v := m.View()

		if got := lipgloss.Height(v); got > sz.h {
			t.Errorf("%dx%d: dialog is %d rows — taller than the screen, the footer is cut off", sz.w, sz.h, got)
		}
		if !strings.Contains(v, "enter: go") || !strings.Contains(v, "esc: cancel") {
			t.Errorf("%dx%d: key hints missing from the rendered dialog", sz.w, sz.h)
		}
	}
}

// "dearest" is archaic; nobody says it. Plain words only.
func TestNoArchaicWording(t *testing.T) {
	m := NewResearchDialogCmp("q")
	m.SetSize(160, 50)
	v := m.View()
	for _, w := range []string{"dearest", "cheapest"} {
		if strings.Contains(v, w) {
			t.Errorf("%q is left over from the rejected cheapest/dearest range", w)
		}
	}
}
