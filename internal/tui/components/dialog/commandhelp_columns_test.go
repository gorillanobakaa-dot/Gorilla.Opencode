// GORILLA OVERRIDE (2026-09-03): the reference is a full-window, two-column
// screen, and these hold it to that.
//
// Reported from the live screen, on a 200-column terminal: "it has a very very
// bad design, normal users would not be aware that there are more commands out
// there or that they can scroll". The dialog capped itself at 84 columns and
// showed about a dozen of thirty-one commands in a panel floating in the middle
// of the display, with nothing to say the list continued.
//
// The geometry assertions here are the ones CLAUDE.md asks for by name: the
// frame is EXACTLY the terminal size, never merely "not bigger". "Not taller"
// passes against a panel that is silently too small, and too small is the state
// that causes the wrapping this file has been bitten by before.
package dialog

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func helpAt(t *testing.T, w, h int) string {
	t.Helper()
	m := NewCommandHelpCmp()
	m.Init()
	m.SetSize(w, h)
	return m.View()
}

// The panel must use the whole WIDTH, and must never be taller than the window.
//
// GORILLA OVERRIDE (2026-09-03), corrected: the first version of this asserted
// the frame was EXACTLY the terminal height, and drove a real bug. This dialog
// is drawn by placeOverlay, which in scrollback mode grows the canvas by the
// overlay's full height and then puts the prompt and footer below it, so a
// frame as tall as the terminal overflows by exactly the footer and the TOP
// scrolls away, taking the title and the command count with it. Seen on a
// screenshot: the list looked correct and the header had silently gone.
//
// Height therefore has an upper bound, not an equality. Width keeps the
// equality, because using the whole width is the thing being fixed.
func TestTheReferenceFillsTheWholeWindow(t *testing.T) {
	for _, sz := range [][2]int{{200, 45}, {160, 50}, {120, 40}, {100, 30}, {80, 24}} {
		w, h := sz[0], sz[1]
		lines := strings.Split(helpAt(t, w, h), "\n")

		// STRICTLY shorter. This is an overlay: placeOverlay grows the canvas
		// by the overlay's full height and puts the prompt and the footer
		// below it, so a frame that consumes the entire terminal leaves
		// nothing for the thing it is drawn over, the view scrolls, and the
		// rows lost are the ones at the top. Equality here is the bug.
		if len(lines) >= h {
			t.Errorf("%dx%d: frame is %d rows and the terminal is %d. An overlay must "+
				"leave room for the prompt and footer underneath it, or the top scrolls "+
				"away and takes the title with it.", w, h, len(lines), h)
		}
		widest := 0
		for _, l := range lines {
			if x := lipgloss.Width(l); x > widest {
				widest = x
			}
		}
		if widest != w {
			t.Errorf("%dx%d: widest line is %d columns, want exactly %d", w, h, widest, w)
		}
	}
}

// No line may exceed the terminal width. lipgloss WRAPS rather than overflowing,
// so an untruncated line costs a screen row that the height budget never counted
// and the damage appears somewhere else entirely.
func TestNoHelpLineIsWiderThanTheTerminal(t *testing.T) {
	for _, sz := range [][2]int{{200, 45}, {110, 30}, {100, 24}, {90, 20}, {80, 24}, {60, 18}} {
		w, h := sz[0], sz[1]
		for i, l := range strings.Split(helpAt(t, w, h), "\n") {
			if got := lipgloss.Width(l); got > w {
				t.Errorf("%dx%d: line %d is %d columns wide:\n%s", w, h, i, got, l)
			}
		}
	}
}

// A wide terminal must actually use two columns. The whole point is to show
// several times as many commands at once.
func TestAWideTerminalGetsTwoColumns(t *testing.T) {
	m := &commandHelpCmp{}
	m.Init()
	m.SetSize(200, 45)
	if got := m.columns(); got != 2 {
		t.Errorf("a 200-column terminal produced %d column(s), want 2", got)
	}

	// And a narrow one must NOT: two cramped columns are worse than one honest
	// column, and the summaries would be truncated to uselessness.
	m2 := &commandHelpCmp{}
	m2.Init()
	m2.SetSize(80, 24)
	if got := m2.columns(); got != 1 {
		t.Errorf("an 80-column terminal produced %d column(s), want 1", got)
	}
}

// The real payoff: on a wide screen, essentially the whole reference is visible
// without scrolling at all.
func TestAWideTerminalShowsEveryCommandAtOnce(t *testing.T) {
	view := helpAt(t, 200, 45)
	m := &commandHelpCmp{}
	m.Init()
	m.SetSize(200, 45)

	missing := []string{}
	for i := range m.rows {
		if !m.selectable(i) {
			continue
		}
		if !strings.Contains(view, "/"+m.rows[i].cmd.Name) {
			missing = append(missing, m.rows[i].cmd.Name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d command(s) are not on a 200x45 screen: %s\n"+
			"  The complaint this layout answers is that people cannot tell there are "+
			"more commands. Showing them is better than announcing them.",
			len(missing), strings.Join(missing, ", "))
	}
}

// Tab must reach the other column, and must land on a command rather than a
// group heading. A key that silently does nothing reads as broken.
func TestTabMovesBetweenColumnsAndLandsOnACommand(t *testing.T) {
	m := &commandHelpCmp{}
	m.Init()
	m.SetSize(200, 45)

	before := m.selectedIdx
	m.jumpColumn()
	after := m.selectedIdx

	if after == before {
		t.Fatal("tab did not move the selection")
	}
	if !m.selectable(after) {
		t.Errorf("tab landed on row %d, which is a heading, not a command", after)
	}

	// And back again, so it is a way across rather than a one-way trip.
	m.jumpColumn()
	if !m.selectable(m.selectedIdx) {
		t.Errorf("tab back landed on a non-command row %d", m.selectedIdx)
	}
}

// On a narrow terminal there is no second column, so tab must be a no-op rather
// than teleporting the selection somewhere unrelated.
func TestTabDoesNothingInOneColumnMode(t *testing.T) {
	m := &commandHelpCmp{}
	m.Init()
	m.SetSize(80, 24)

	before := m.selectedIdx
	m.jumpColumn()
	if m.selectedIdx != before {
		t.Errorf("tab moved the selection from %d to %d in single-column mode",
			before, m.selectedIdx)
	}
}

// The subtitle has to say the list is longer than the screen, because the
// original complaint was that nobody could tell.
func TestTheSubtitleSaysHowManyCommandsThereAre(t *testing.T) {
	view := helpAt(t, 200, 45)
	if !strings.Contains(view, "commands") {
		t.Error("the header does not say how many commands exist")
	}
	if !strings.Contains(view, "tab") {
		t.Error("the header does not mention tab, so the second column is undiscoverable")
	}
}

// The selected command's explanation must survive the new layout. It is the
// part of this screen a user cannot get anywhere else.
func TestTheExplanationIsStillShown(t *testing.T) {
	m := &commandHelpCmp{}
	m.Init()
	m.SetSize(200, 45)
	m.FocusCommand("clear")
	view := m.View()

	detail := m.rows[m.selectedIdx].cmd.Detail
	first := strings.Fields(detail)
	if len(first) < 3 {
		t.Skip("the focused command has no detail text to look for")
	}
	if !strings.Contains(view, strings.Join(first[:3], " ")) {
		t.Errorf("the explanation for the selected command is missing from the frame")
	}
}

// The header must be a FIXED number of rows at every width.
//
// GORILLA OVERRIDE (2026-09-03): written after the width test above failed to
// catch the bug it was supposed to. Removing the subtitle's truncation left
// TestNoHelpLineIsWiderThanTheTerminal green, because lipgloss does not overflow
// when a line is too long, it WRAPS: the result is two lines that are each
// narrow enough, and one extra screen row that commandHelpFixedLines never
// counted. The symptom is height, in a different place from the cause, and a
// width assertion cannot see it. CLAUDE.md says this in as many words and I
// wrote the width-only test anyway.
//
// So this pins the thing that actually moves: where the list starts. If any
// header line wraps, everything below it shifts down by a row.
func TestTheHeaderIsTheSameHeightAtEveryWidth(t *testing.T) {
	firstHeadingRow := func(w, h int) int {
		for i, l := range strings.Split(helpAt(t, w, h), "\n") {
			if strings.Contains(l, "Your conversation") {
				return i
			}
		}
		return -1
	}

	want := firstHeadingRow(200, 45)
	if want < 0 {
		t.Fatal("could not find the first group heading at 200x45")
	}
	for _, w := range []int{160, 120, 110, 100, 96, 90, 80, 70, 60} {
		if got := firstHeadingRow(w, 30); got != want {
			t.Errorf("at width %d the list starts on row %d, but on row %d at width 200.\n"+
				"  A header line wrapped. It costs a row the height budget did not allow for, "+
				"and every row below it moves.", w, got, want)
		}
	}
}

// The title and the command count must be ON the frame, not scrolled off it.
//
// GORILLA OVERRIDE (2026-09-03): this is the assertion that would have caught
// the padding bug immediately. The list rendered perfectly while the header was
// pushed off the top of the terminal, and every other test here passed, because
// they all looked at the list.
func TestTheTitleAndCountSurviveOnScreen(t *testing.T) {
	for _, sz := range [][2]int{{200, 45}, {160, 50}, {120, 40}, {100, 30}} {
		w, h := sz[0], sz[1]
		view := helpAt(t, w, h)
		if !strings.Contains(view, "Commands") {
			t.Errorf("%dx%d: the title is not in the frame", w, h)
		}
		if !strings.Contains(view, "commands") {
			t.Errorf("%dx%d: the command count is not in the frame", w, h)
		}
	}
}
