package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/commands"
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
