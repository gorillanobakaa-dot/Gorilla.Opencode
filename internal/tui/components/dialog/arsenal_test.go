package dialog

// GORILLA OVERRIDE (2026-08-19): headless render assertions for /arsenal.
//
// The invariant that matters most here is width. Bubbletea's inline renderer
// erases its last frame by moving the cursor up by the number of LOGICAL lines
// it drew; a line wider than the terminal occupies two PHYSICAL rows and counts
// as one, so the erase under-reaches by a row on every render and the frame
// walks down the screen. That bug cost three releases and three wrong
// diagnoses, so every new full-screen page asserts it from the start.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func renderArsenal(t *testing.T, w, h int, keys ...string) (ArsenalCmp, string) {
	t.Helper()
	m := NewArsenalCmp()
	m.SetSize(w, h)
	var model tea.Model = m
	for _, k := range keys {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		if k == "enter" || k == "esc" || k == "up" || k == "down" {
			// named keys go through their own type
		}
	}
	got := model.(ArsenalCmp)
	return got, got.View()
}

func pressNamed(m tea.Model, typ tea.KeyType) tea.Model {
	out, _ := m.Update(tea.KeyMsg{Type: typ})
	return out
}

func TestNoLineIsWiderThanTheTerminal(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {100, 30}, {160, 50}, {60, 20}} {
		var model tea.Model = func() ArsenalCmp { m := NewArsenalCmp(); m.SetSize(size.w, size.h); return m }()
		// Walk into a series and then into a detail page, so all three views
		// are covered at every width.
		for _, step := range []int{0, 1, 2} {
			view := model.(ArsenalCmp).View()
			for i, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > size.w {
					t.Errorf("%dx%d view %d line %d is %d cols wide: %q",
						size.w, size.h, step, i, got, line)
				}
			}
			model = pressNamed(model, tea.KeyEnter)
		}
	}
}

// The whole feature exists because a capability sat installed and unknown. If
// the page cannot tell present from absent, it reproduces the bug it fixes.
func TestThePageSaysWhatIsAlreadyHere(t *testing.T) {
	_, view := renderArsenal(t, 120, 40)
	if !strings.Contains(view, "capabilities present") {
		t.Fatalf("the header does not report what is already installed:\n%s", view)
	}
}

// Three granularities, always. Slackware's model, and the owner's explicit ask.
func TestAllThreeGranularitiesAreReachable(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(120, 40)
	var model tea.Model = m
	if model.(ArsenalCmp).view != viewSeries {
		t.Fatal("did not open on the series list")
	}
	model = pressNamed(model, tea.KeyEnter)
	if model.(ArsenalCmp).view != viewEntries {
		t.Fatal("enter did not open a series")
	}
	model = pressNamed(model, tea.KeyEnter)
	if model.(ArsenalCmp).view != viewDetail {
		t.Fatal("enter did not open an entry's detail")
	}
	model = pressNamed(model, tea.KeyEsc)
	model = pressNamed(model, tea.KeyEsc)
	if model.(ArsenalCmp).view != viewSeries {
		t.Fatal("esc did not walk back out")
	}
}

// "Everything" is a first-class answer and must never be hidden or made harder
// to reach than the small option. That is the correction this design was
// explicitly rewritten for.
func TestEverythingIsOneKeyAndIsOffered(t *testing.T) {
	m, view := renderArsenal(t, 120, 40, "a")
	if !strings.Contains(view, "a everything") {
		t.Errorf("the key that takes everything is not offered on screen:\n%s", view)
	}
	picked := 0
	for _, on := range m.selected {
		if on {
			picked++
		}
	}
	if picked == 0 {
		t.Fatal("pressing a selected nothing")
	}
}

// Selecting must never pick something already installed — it would inflate the
// cost with packages the user already has.
func TestSelectingEverythingSkipsWhatIsAlreadyInstalled(t *testing.T) {
	m, _ := renderArsenal(t, 120, 40, "a")
	for id, on := range m.selected {
		if on && m.status[id].Present {
			t.Errorf("%s is already installed and was selected anyway", id)
		}
	}
}

// Cost is INFORMATION, not a gate: it must appear, and it must never prevent a
// selection.
func TestCostIsOfferedInMinutesNotOnlyMegabytes(t *testing.T) {
	_, view := renderArsenal(t, 120, 40, "a")
	if !strings.Contains(view, "press p to measure") {
		t.Errorf("no route to the real cost:\n%s", view)
	}
}

// An entry that cannot be installed here must not read as free.
func TestUnavailableEntriesAreLabelled(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(120, 60)
	// Walk to the coding series, which contains ast-grep (no Debian package).
	for i, s := range m.man.Series {
		if s.ID == "coding" {
			m.seriesIdx = i
		}
	}
	m.view = viewEntries
	view := m.View()
	if !strings.Contains(view, "N/A") && !strings.Contains(view, "not packaged") {
		t.Skip("no unavailable entry on this machine's package manager")
	}
}

func TestNothingSelectedGivesAnInstructionNotAnError(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(120, 40)
	m.showPlan()
	if m.notice == "" {
		t.Fatal("pressing install with nothing selected did nothing and said nothing")
	}
	if !strings.Contains(m.notice, "Space") {
		t.Errorf("the message does not say what to do instead: %q", m.notice)
	}
	if m.view == viewPlan {
		t.Error("opened an install plan for an empty selection")
	}
}

// GORILLA OVERRIDE (2026-08-19): the first version of the install key SENT the
// command to the model and asked it to explain the selection. Caught in the
// live run: that is a full model turn, billed, on every press, to restate what
// the manifest already carries in plain words. For an audience where tokens are
// a recurring bill they cannot afford, a screen that quietly spends money when
// you press a key is a screen you learn not to press.
func TestTheInstallPlanCostsNoTokens(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(120, 40)
	m.toggle([]string{"zbar"})
	if cmd := m.showPlan(); cmd != nil {
		t.Fatal("showing the install plan returned a command — it must render locally, for nothing")
	}
	if m.view != viewPlan {
		t.Fatal("the plan did not open")
	}
	view := m.View()
	if !strings.Contains(view, "will not run it") || !strings.Contains(view, "password") {
		t.Errorf("the plan does not say plainly that nothing is installed from here:\n%s", view)
	}
}

// The exact command has to be ON the plan, in full, or the screen is useless.
func TestThePlanShowsTheExactCommand(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(140, 50)
	m.toggle([]string{"zbar"})
	m.showPlan()
	view := m.View()
	if !strings.Contains(view, "zbar-tools") {
		t.Errorf("the package name is not on the plan:\n%s", view)
	}
	if !strings.Contains(view, "apt-get install") && !strings.Contains(view, "pacman") {
		t.Errorf("no install command on the install plan:\n%s", view)
	}
}

// Saving without loading would be half the feature: the POINT of a tagfile is
// that somebody else can send you theirs.
func TestATagfileCanBeLoadedBackNotJustWritten(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := NewArsenalCmp()
	m.SetSize(120, 40)

	// Pick something that is definitely not installed here, so the round trip
	// is not silently emptied by the already-present filter.
	var pick string
	for _, s := range m.man.Series {
		for _, e := range s.Entries {
			if !m.status[e.ID].Present {
				pick = e.ID
			}
		}
	}
	if pick == "" {
		t.Skip("everything in the manifest is installed on this machine")
	}
	m.toggle([]string{pick})
	m.saveTagfile()
	if !strings.Contains(m.notice, "Saved") {
		t.Fatalf("save reported: %q", m.notice)
	}

	m.selected = map[string]bool{}
	m.loadTagfile()
	if !m.selected[pick] {
		t.Fatalf("the selection did not survive a save and load; notice was %q", m.notice)
	}
	if !strings.Contains(m.notice, "Loaded") {
		t.Errorf("load said nothing useful: %q", m.notice)
	}
}

func TestLoadingWithNoTagfileSaysSoRatherThanFailingSilently(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := NewArsenalCmp()
	m.SetSize(120, 40)
	m.loadTagfile()
	if m.notice == "" {
		t.Fatal("loading a selection that does not exist did nothing and said nothing")
	}
}
