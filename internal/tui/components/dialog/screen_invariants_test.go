// GORILLA OVERRIDE: screen-level invariants for every dialog, asserted on a real
// terminal cell grid rather than on the string a component happens to emit.
//
// The motivation is a specific failure. Three display bugs reached the user through
// screenshots of a release, and the tests that existed could not have caught two of
// them, because both were about what the SCREEN showed rather than what the code
// produced: a selected row whose foreground equalled its background (present in the
// output, invisible to the eye), and a dialog rendering more rows than the terminal
// had. See internal/tui/screentest for why a cell grid answers those and string
// matching does not.
package dialog

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/screentest"
)

// terminalSizes covers the range worth caring about: a cramped SSH window, a
// laptop, a maximised terminal, and a deliberately tiny one because "it will never
// be that small" is how the /help overflow shipped.
var terminalSizes = []struct{ w, h int }{
	{60, 10}, {80, 24}, {100, 30}, {120, 40}, {160, 50}, {40, 8},
}

// sizedDialog is any dialog that can be told a size and rendered. Most take it as
// a tea.WindowSizeMsg rather than a SetSize call, so the adapters below normalise
// the two.
type sizedDialog interface {
	SetSize(width, height int)
	View() string
}

// viaWindowSize adapts a dialog that learns its size from a WindowSizeMsg.
type viaWindowSize struct {
	m tea.Model
}

func (v viaWindowSize) SetSize(width, height int) {
	v.m.Update(tea.WindowSizeMsg{Width: width, Height: height})
}
func (v viaWindowSize) View() string { return v.m.View() }

// standardHeight is the classic minimum terminal, and the line this project holds:
// no dialog may overflow a terminal this tall or taller. 80x24 is not an edge case,
// it is the default size of an xterm.
const standardHeight = 24

// knownSmallOverflow records the dialogs that still overflow terminals SHORTER than
// standardHeight, keyed by "name WxH" with the minimum they currently need.
//
// Keyed by SIZE as well as name, because the requirement is not one number: a
// narrower terminal wraps more text and therefore needs MORE rows. The first
// version of this ratchet keyed on name alone and immediately reported four false
// improvements — which is the ratchet working, since a figure that does not match
// reality is a figure nobody can trust.
//
// It is a ratchet, not an excuse. Asserted from both directions: a dialog that gets
// worse fails, and one that gets BETTER also fails, so the entry must be lowered
// rather than quietly rotting. A dialog not listed may not overflow at any size,
// which is what stops new ones joining the list.
//
// All of these were found by rendering into a terminal cell grid. Before that they
// were invisible, because the labels are present in the output either way.
var knownSmallOverflow = map[string]int{
	"/add-dir 60x10": 18,
	"/add-dir 40x8":  21,
	"/export 60x10":  13,
	"/export 40x8":   13,
	"/prompts 60x10": 22,
	"/prompts 40x8":  24,
	"/reset 60x10":   26,
	"/reset 40x8":    32,
	"/settings 40x8": 11,
	"/connect 60x10": 17,
	"/connect 40x8":  17,
	"/context 60x10": 19,
	"/context 40x8":  19,
}

// No dialog may ask for more rows than the terminal has, at any terminal 24 rows or
// taller. Overflow is invisible to the person using it — the box is simply cut off,
// with nothing to say that anything is missing.
//
// Two of these were real and live when this test was written: /context wanted 37
// rows and /connect 26, so both were cut off on an ordinary 80x24 screen.
func TestNoDialogOverflowsAStandardTerminal(t *testing.T) {
	for _, size := range terminalSizes {
		if size.h < standardHeight {
			continue
		}
		for name, build := range sizedDialogs(t) {
			t.Run(fmt.Sprintf("%s/%dx%d", name, size.w, size.h), func(t *testing.T) {
				d := build()
				d.SetSize(size.w, size.h)

				s := screentest.Render(d.View(), size.w, size.h)
				if s.Overflows() {
					t.Errorf("%s wants %d rows in a %d-row terminal — the bottom %d rows are cut off with no sign to the user:\n%s",
						name, s.RowsWanted(), size.h, s.RowsWanted()-size.h, s)
				}
			})
		}
	}
}

// And on a terminal shorter than standard, a dialog may only need what the ratchet
// says it needs — no more, and no less without the entry being updated.
func TestSmallTerminalOverflowsMatchTheRatchet(t *testing.T) {
	for _, size := range terminalSizes {
		if size.h >= standardHeight {
			continue
		}
		for name, build := range sizedDialogs(t) {
			t.Run(fmt.Sprintf("%s/%dx%d", name, size.w, size.h), func(t *testing.T) {
				d := build()
				d.SetSize(size.w, size.h)
				s := screentest.Render(d.View(), size.w, size.h)

				key := fmt.Sprintf("%s %dx%d", name, size.w, size.h)
				allowed, listed := knownSmallOverflow[key]
				if !s.Overflows() {
					if listed {
						// It fits now. Good — but the entry must come down or it
						// stops meaning anything.
						t.Errorf("%s no longer overflows at %dx%d; remove its knownSmallOverflow entry (currently %d)",
							name, size.w, size.h, allowed)
					}
					return
				}
				if !listed {
					t.Errorf("%s overflows at %dx%d (wants %d) and is not in knownSmallOverflow — a new dialog must not join that list silently:\n%s",
						name, size.w, size.h, s.RowsWanted(), s)
					return
				}
				if s.RowsWanted() > allowed {
					t.Errorf("%s now wants %d rows at %dx%d, worse than the recorded %d",
						name, s.RowsWanted(), size.w, size.h, allowed)
				}
				if s.RowsWanted() < allowed {
					t.Errorf("%s now wants only %d rows (recorded %d) — an improvement; lower the knownSmallOverflow entry so the ratchet keeps its grip",
						name, s.RowsWanted(), allowed)
				}
			})
		}
	}
}

// Nor wider than the terminal, or the right-hand side is clipped at the screen edge.
func TestNoDialogOverflowsTheWidth(t *testing.T) {
	for _, size := range terminalSizes {
		for name, build := range sizedDialogs(t) {
			t.Run(fmt.Sprintf("%s/%dx%d", name, size.w, size.h), func(t *testing.T) {
				d := build()
				d.SetSize(size.w, size.h)

				s := screentest.Render(d.View(), size.w, size.h)
				// A row filling the full width on a bordered box means the border
				// was clipped rather than fitted inside.
				if cols, row := s.WidestRow(); cols > size.w {
					t.Errorf("%s row %d is %d columns in a %d-column terminal", name, row, cols, size.w)
				}
			})
		}
	}
}

// The reported /help bug, now asserted on the screen: whichever row is selected
// must be readable. It rendered as a blank line because rowStyle set a highlight
// background and a shared helper then reset the background to the panel colour,
// leaving foreground equal to background.
func TestTheSelectedHelpRowIsLegibleAtEverySize(t *testing.T) {
	for _, size := range terminalSizes {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			m := NewCommandHelpCmp()
			m.Init()
			m.SetSize(size.w, size.h)

			s := screentest.Render(m.View(), size.w, size.h)

			// Find the cursor marker rather than assuming an index, so the test
			// survives the command list changing.
			row := s.FindRow("▸")
			if row < 0 {
				row = s.FindRow(">")
			}
			if row < 0 {
				t.Skipf("no selection marker visible at %dx%d; nothing to assert", size.w, size.h)
			}
			if !s.RowLegible(row) {
				t.Errorf("the selected row (%d) has no readable character — foreground equals background, which is the /help bug:\n%s", row, s)
			}
		})
	}
}

// Every dialog must put SOMETHING readable on screen. A dialog that renders only
// blanks and borders is a bug that no string assertion would notice, because the
// labels are all present in the output.
func TestEveryDialogShowsSomethingReadable(t *testing.T) {
	for name, build := range sizedDialogs(t) {
		t.Run(name, func(t *testing.T) {
			d := build()
			d.SetSize(80, 24)

			s := screentest.Render(d.View(), 80, 24)

			legible := 0
			for y := 0; y < 24; y++ {
				if s.RowLegible(y) {
					legible++
				}
			}
			if legible == 0 {
				t.Errorf("%s renders nothing a user could read:\n%s", name, s)
			}
		})
	}
}

// sizedDialogs builds the dialogs that take a size. Constructed fresh per subtest,
// because these hold selection state and a shared instance would let one case
// affect the next.
func sizedDialogs(t *testing.T) map[string]func() sizedDialog {
	t.Helper()
	// config.Load is needed by the dialogs that read settings; the package's
	// TestMain isolates it from the real config.
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	return map[string]func() sizedDialog{
		"/help": func() sizedDialog {
			m := NewCommandHelpCmp()
			m.Init()
			return m
		},
		"/export": func() sizedDialog {
			m := NewExportDialogCmp()
			m.SetDefaults(t.TempDir(), "session.md")
			m.Init()
			return viaWindowSize{m}
		},
		"/connect": func() sizedDialog {
			m := NewConnectDialogCmp()
			m.Init()
			return viaWindowSize{m}
		},
		"/context": func() sizedDialog {
			m := NewLoadoutDialogCmp()
			m.Init()
			return viaWindowSize{m}
		},
		"/settings": func() sizedDialog {
			m := NewSettingsDialogCmp()
			m.Init()
			return viaWindowSize{m}
		},
		"/add-dir": func() sizedDialog {
			m := NewAddDirDialogCmp()
			m.Init()
			return viaWindowSize{m}
		},
		"/reset": func() sizedDialog {
			m := NewResetDialogCmp()
			m.Init()
			return viaWindowSize{m}
		},
		"/prompts": func() sizedDialog {
			m := NewPromptsDialogCmp()
			m.Init()
			return viaWindowSize{m}
		},
	}
}
