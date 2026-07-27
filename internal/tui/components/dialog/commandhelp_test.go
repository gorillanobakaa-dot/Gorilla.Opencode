package dialog

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/opencode-ai/opencode/internal/commands"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

func newHelpAt(t *testing.T, w, h int) *commandHelpCmp {
	t.Helper()
	m := NewCommandHelpCmp().(*commandHelpCmp)
	m.Init()
	m.SetSize(w, h)
	return m
}

// Same discipline as every other dialog: chrome is subtracted from the terminal
// size, never added to the content. Adding it is what shipped an invisible input
// box in v0.1.38 and four 82-column dialogs into an 80-column terminal.
func TestCommandHelpNeverExceedsTerminalWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 100, 120, 200} {
		m := newHelpAt(t, w, 40)
		for i, l := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(l); got > w {
				t.Errorf("width=%d: line %d is %d columns wide:\n%q", w, i, got, l)
			}
		}
	}
}

// A short terminal must not produce a dialog taller than the screen — that
// scrolls the terminal and destroys the layout.
func TestCommandHelpFitsShortTerminals(t *testing.T) {
	for _, h := range []int{10, 16, 24, 40} {
		m := newHelpAt(t, 100, h)
		if got := lipgloss.Height(m.View()); got > h {
			t.Errorf("height=%d: dialog renders %d lines tall", h, got)
		}
	}
}

// Every command must be reachable by scrolling, or the reference is incomplete
// in exactly the way that made it necessary.
func TestEveryCommandIsReachable(t *testing.T) {
	m := newHelpAt(t, 100, 24)

	seen := map[string]bool{}
	collect := func() {
		for _, l := range strings.Split(m.View(), "\n") {
			for _, c := range commands.All {
				if strings.Contains(l, "/"+c.Name) {
					seen[c.Name] = true
				}
			}
		}
	}
	collect()
	for i := 0; i < len(m.rows)+5; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(*commandHelpCmp)
		collect()
	}

	for _, c := range commands.All {
		if !seen[c.Name] {
			t.Errorf("/%s is never visible while scrolling the whole list", c.Name)
		}
	}
}

// Navigation must skip group headings, or ↓ appears to stick.
func TestNavigationSkipsHeadings(t *testing.T) {
	m := newHelpAt(t, 100, 40)

	if !m.selectable(m.selectedIdx) {
		t.Fatal("opens with a heading selected")
	}
	for i := 0; i < 30; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(*commandHelpCmp)
		if !m.selectable(m.selectedIdx) {
			t.Fatalf("after %d presses the selection is on a heading (row %d)", i+1, m.selectedIdx)
		}
	}
}

// The explanation is the point. A list of names the user already half-knows is
// what they had; the reason a command exists is what was missing.
func TestSelectedCommandShowsItsExplanation(t *testing.T) {
	m := newHelpAt(t, 100, 40)
	view := m.View()

	sel := m.rows[m.selectedIdx].cmd
	if sel == nil {
		t.Fatal("nothing selected")
	}
	// The Detail text is wrapped, so check a distinctive run of words from it.
	words := strings.Fields(sel.Detail)
	if len(words) < 4 {
		t.Fatalf("%s has a suspiciously short Detail", sel.Name)
	}
	probe := strings.Join(words[:4], " ")
	if !strings.Contains(view, probe) {
		t.Errorf("the selected command's explanation is not shown; looked for %q in:\n%s", probe, view)
	}
}

// Search has to find a command by what it DOES, not only by its name — the user
// who needs this reference does not know the names.
func TestSearchMatchesDescriptions(t *testing.T) {
	m := newHelpAt(t, 100, 40)

	for _, k := range []rune("quota") {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
		m = next.(*commandHelpCmp)
	}
	m.filtering = true
	m.filter = "quota"
	m.rebuild()

	if len(m.rows) == 0 {
		t.Fatal("searching for \"quota\" matched nothing, though several commands discuss it")
	}
	for _, r := range m.rows {
		if r.cmd == nil {
			continue
		}
		hay := strings.ToLower(r.cmd.Name + r.cmd.Summary + r.cmd.Detail)
		if !strings.Contains(hay, "quota") {
			t.Errorf("/%s matched \"quota\" but does not mention it", r.cmd.Name)
		}
	}
}

func TestEscapeClosesTheReference(t *testing.T) {
	m := newHelpAt(t, 100, 40)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc produced no command")
	}
	if _, ok := cmd().(CloseCommandHelpMsg); !ok {
		t.Errorf("esc emitted %T, want CloseCommandHelpMsg", cmd())
	}
}

// The bug the user's screenshots caught: the SELECTED command rendered as a
// blank line, so /help looked like it was missing whichever command you were
// currently reading about — /clear absent with /clear's explanation below it.
//
// Cause: the row style set a highlight background, and the shared line() helper
// then reset the background to the panel colour, leaving foreground equal to
// background. Invisible text.
//
// Note why the earlier tests all passed: a row still CONTAINS its text when
// foreground and background match, so asserting `strings.Contains(view, "/clear")`
// says nothing about whether a human can see it. The colours have to be asserted.
func TestSelectedRowIsVisible(t *testing.T) {
	th := theme.CurrentTheme()
	base := lipgloss.NewStyle().Background(th.Background())

	sel := rowStyle(base, th, true)
	if sel.GetForeground() == sel.GetBackground() {
		t.Errorf("selected row has foreground == background (%v) — it renders as an invisible blank line",
			sel.GetForeground())
	}

	unsel := rowStyle(base, th, false)
	if unsel.GetForeground() == unsel.GetBackground() {
		t.Errorf("unselected row has foreground == background (%v)", unsel.GetForeground())
	}

	// And the highlight must actually distinguish the row, or there is no visible
	// cursor at all.
	if sel.GetBackground() == unsel.GetBackground() {
		t.Errorf("selected and unselected share background %v — nothing marks the cursor", sel.GetBackground())
	}
}

// End-to-end guard on the same fault, at the render level.
//
// Two earlier versions of this test were wrong, which is worth recording because
// each failure mode is easy to repeat:
//  1. Asserting the selected line carried SOME background escape. The buggy code
//     also does — it inherits the panel background — so the test passed against
//     the bug.
//  2. Taking the FIRST background escape on the line. That is the box's leading
//     padding, not the row, so the test failed against correct code.
//
// The background that matters is the one in force where the command name is
// printed: the LAST background set before that text.
func TestSelectedRowCarriesAHighlightInTheRender(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	m := newHelpAt(t, 100, 40)
	view := m.View()

	sel := m.rows[m.selectedIdx].cmd
	if sel == nil {
		t.Fatal("nothing selected")
	}

	bgRe := regexp.MustCompile(`48;2;(\d+;\d+;\d+)`)
	// bgAtText returns the background in force where needle is printed.
	bgAtText := func(line, needle string) (string, bool) {
		i := strings.Index(line, needle)
		if i < 0 {
			return "", false
		}
		all := bgRe.FindAllStringSubmatch(line[:i], -1)
		if len(all) == 0 {
			return "", true
		}
		return all[len(all)-1][1], true
	}

	var selBG, otherBG string
	var haveSel, haveOther bool
	for _, l := range strings.Split(view, "\n") {
		if bg, ok := bgAtText(l, "/"+sel.Name+" "); ok && !haveSel {
			selBG, haveSel = bg, true
			continue
		}
		for _, c := range commands.All {
			if c.Name == sel.Name || haveOther {
				continue
			}
			if bg, ok := bgAtText(l, "/"+c.Name+" "); ok {
				otherBG, haveOther = bg, true
			}
		}
	}

	if !haveSel {
		t.Fatalf("the selected command /%s is not in the view at all", sel.Name)
	}
	if !haveOther {
		t.Fatal("could not find an unselected command row to compare against")
	}
	if selBG == otherBG {
		t.Errorf("the selected row's background (%s) equals an unselected row's — nothing marks the cursor, and if the foreground also matches it the row reads as blank", selBG)
	}
}
