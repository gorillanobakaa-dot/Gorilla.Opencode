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

// GORILLA OVERRIDE (2026-08-19), reported by the owner within minutes of the
// release: "is it me or space does not select anything? p also does not list
// the costs."
//
// It was not him and it was not the key. The page opens with the cursor on
// "The minimum", which on his machine is 8/8 ALREADY INSTALLED — so space
// correctly selected nothing and said nothing, and p then correctly priced an
// empty selection and also said nothing. Two keys doing exactly the right
// thing and looking completely broken.
//
// Directive §3 arriving in a UI: silence and success must never look alike.
// Every test here called toggle() on entries chosen to be missing, which is
// why none of them could see it.
func TestPressingSpaceOnAFullyInstalledGroupSaysSo(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(120, 40)

	var full []string
	for _, s := range m.man.Series {
		all := len(s.Entries) > 0
		for _, e := range s.Entries {
			if !m.status[e.ID].Present {
				all = false
			}
		}
		if all {
			for _, e := range s.Entries {
				full = append(full, e.ID)
			}
			break
		}
	}
	if full == nil {
		t.Skip("no fully-installed series on this machine")
	}

	m.toggle(full)
	if m.notice == "" {
		t.Fatal("selecting a group that is entirely installed did nothing and said nothing — " +
			"indistinguishable from a broken key")
	}
	if !strings.Contains(m.notice, "already installed") {
		t.Errorf("the message does not explain why nothing happened: %q", m.notice)
	}
	if !strings.Contains(m.View(), "already installed") {
		t.Error("the explanation was set but never rendered")
	}
}

// Pressing p with nothing selected is the second half of the same report.
func TestPricingNothingSaysWhichKeySelects(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(120, 40)
	if cmd := m.price(); cmd != nil {
		t.Error("priced an empty selection instead of explaining")
	}
	if !strings.Contains(m.notice, "space") {
		t.Fatalf("the message does not say which key selects: %q", m.notice)
	}
}

// A successful selection must confirm itself too, or the user is guessing.
func TestASuccessfulSelectionConfirmsWhatItDid(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(120, 40)
	var missing string
	for _, s := range m.man.Series {
		for _, e := range s.Entries {
			if !m.status[e.ID].Present {
				missing = e.ID
			}
		}
	}
	if missing == "" {
		t.Skip("everything is installed on this machine")
	}
	m.toggle([]string{missing})
	if !strings.Contains(m.notice, "selected 1") {
		t.Fatalf("a real selection did not confirm itself: %q", m.notice)
	}
	m.toggle([]string{missing})
	if !strings.Contains(m.notice, "un-selected") {
		t.Errorf("un-selecting did not confirm itself: %q", m.notice)
	}
}

// The key dispatch must not throw away what the handler did. `return m,
// m.price()` reads `m` at a moment the spec does not pin down.
func TestKeyHandlersMutationsSurviveTheReturn(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(120, 40)
	var model tea.Model = m
	// p with nothing selected sets a notice and nothing else.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if model.(ArsenalCmp).notice == "" {
		t.Fatal("the notice set inside the p handler did not survive the return")
	}
}

// GORILLA OVERRIDE (2026-08-19), owner's call: "you can always make the window
// either bigger or full screen. That solves the problem."
//
// It does, and it is the better fix. Reserving rows for a trailing notice
// treats the symptom; taking the whole screen removes the question — the
// budget is fixed and known before anything renders, and the content scrolls
// inside it.
func TestThePageFillsTheScreenExactly(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {100, 30}, {160, 50}, {60, 20}, {200, 60}} {
		m := NewArsenalCmp()
		m.SetSize(size.w, size.h)
		// Worst case: a notice AND more content than fits.
		m.notice = "a notice that must not push the frame off the bottom of the terminal"

		for _, view := range []arsenalView{viewSeries, viewEntries, viewDetail, viewPlan} {
			m.view = view
			out := m.View()
			gotH := lipgloss.Height(out)
			if gotH > size.h {
				t.Errorf("%dx%d view %d: frame is %d rows tall, taller than the terminal",
					size.w, size.h, view, gotH)
			}
			if gotH != size.h {
				t.Errorf("%dx%d view %d: frame is %d rows, want exactly %d — a constant "+
					"full-screen frame is the point", size.w, size.h, view, gotH, size.h)
			}
			for i, line := range strings.Split(out, "\n") {
				if got := lipgloss.Width(line); got > size.w {
					t.Errorf("%dx%d view %d line %d is %d cols wide", size.w, size.h, view, i, got)
				}
			}
		}
	}
}

// A notice must never cost the user a row of content silently — it is inside
// the budget, and the overflow marker still appears when content is cut.
func TestTheNoticeAndTheOverflowMarkerLiveInsideTheBudget(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(100, 14) // deliberately short: content cannot fit
	m.notice = "selected 2 — press p to measure the cost."
	out := m.View()
	if lipgloss.Height(out) != 14 {
		t.Fatalf("frame is %d rows, want 14", lipgloss.Height(out))
	}
	if !strings.Contains(out, "more line") {
		t.Error("content was cut with no marker saying so")
	}
	if !strings.Contains(out, "press p") {
		t.Error("the notice was dropped to make room; it must be inside the budget, not extra")
	}
}
