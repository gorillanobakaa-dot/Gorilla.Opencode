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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/arsenal"
	"github.com/opencode-ai/opencode/internal/config"

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
//
// AMENDED 2026-10-05 (audit A3): this asserted "press p to measure" on whatever
// package manager the test machine had. On Scoop that sentence is an offer the
// program cannot keep, so the test was demanding the defect on Windows. It now
// states the package manager and asserts the offer only where it is true; the
// Scoop side is TestScoopIsNeverOfferedAMeasurementItCannotMake.
func TestCostIsOfferedInMinutesNotOnlyMegabytes(t *testing.T) {
	m := bareArsenal(t, arsenal.APT)
	m.toggle(m.everyID())
	view := m.View()
	if !strings.Contains(view, "press p to measure") {
		t.Errorf("no route to the real cost:\n%s", view)
	}
	if !strings.Contains(view, "p measure the real cost") {
		t.Errorf("the p key is not in the hints on a package manager that can measure:\n%s", view)
	}
}

// bareArsenal is the page on an imagined machine with nothing installed, a
// stated package manager and the assumed link speed, so a test asserts the
// page's logic rather than whatever happens to be on the machine running it.
func bareArsenal(t *testing.T, pm arsenal.PackageManager) ArsenalCmp {
	t.Helper()
	m := NewArsenalCmp()
	m.SetSize(120, 40)
	m.pm = pm
	m.linkSpeed, m.linkMeasured = assumedLinkKBps, false
	for id := range m.status {
		m.status[id] = arsenal.Status{}
	}
	return m
}

// arsFirstEntry returns a listed entry that pm can (or cannot) fetch.
func arsFirstEntry(t *testing.T, m ArsenalCmp, fetchable bool) arsenal.Entry {
	t.Helper()
	for _, s := range m.man.Series {
		for _, e := range s.Entries {
			if arsenal.Available(e, m.pm) == fetchable {
				return e
			}
		}
	}
	t.Fatalf("no listed entry with fetchable=%v on %q", fetchable, m.pm)
	return arsenal.Entry{}
}

func arsKey(m tea.Model, k string) ArsenalCmp {
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	return out.(ArsenalCmp)
}

// arsAllViews is the unstyled text of every view, by name.
func arsAllViews(m ArsenalCmp) map[string]string {
	out := map[string]string{}
	for name, v := range map[string]arsenalView{"series": viewSeries, "entries": viewEntries, "detail": viewDetail, "plan": viewPlan} {
		m.view = v
		var b strings.Builder
		for _, l := range m.lines() {
			b.WriteString(l.text + "\n")
		}
		out[name] = b.String()
	}
	return out
}

// An entry that cannot be installed here must not read as free.
//
// AMENDED 2026-10-05 (audit A13): this test could pass or skip and could not
// fail — its only assertion was the condition for skipping. It now states its
// machine (apt, where ast-grep has no package) and asserts the label.
func TestUnavailableEntriesAreLabelled(t *testing.T) {
	m := bareArsenal(t, arsenal.APT)
	m.SetSize(120, 60)
	// Walk to the coding series, which contains ast-grep (no Debian package).
	for i, s := range m.man.Series {
		if s.ID == "coding" {
			m.seriesIdx = i
			for j, e := range s.Entries {
				if e.ID == "astgrep" {
					m.entryIdx = j
				}
			}
		}
	}
	m.view = viewEntries
	view := m.View()
	if !strings.Contains(view, "[N/A ]") {
		t.Errorf("an entry apt cannot fetch carries no N/A badge:\n%s", view)
	}
	if !strings.Contains(view, "not packaged for apt") {
		t.Errorf("the entry under the cursor does not say why it is unavailable:\n%s", view)
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
	id, _ := installableEntry(t, m)
	m.toggle([]string{id})
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

	// GORILLA OVERRIDE (2026-09-01): assert against THIS machine's package
	// manager. The test hardcoded the zbar entry, its Debian package name
	// ("zbar-tools"), and an apt-or-pacman command — so on Windows it asserted
	// the presence of a package with no Scoop equivalent and a command that does
	// not exist there. It could only ever fail, while saying nothing about
	// whether the plan screen actually worked.
	id, pkgs := installableEntry(t, m)
	m.toggle([]string{id})
	m.showPlan()
	view := m.View()

	for _, want := range pkgs {
		if !strings.Contains(view, want) {
			t.Errorf("package %q is not on the plan:\n%s", want, view)
		}
	}
	// AMENDED 2026-10-05 (audit A11): the command can be more than one line,
	// each of which must be on the plan whole.
	for _, cmd := range arsenal.InstallCommands(pkgs, m.pm) {
		if !strings.Contains(view, cmd) {
			t.Errorf("the exact install command %q is not on the plan:\n%s", cmd, view)
		}
	}
}

// installableEntry returns an entry this machine's package manager can actually
// install, with its package names. Skips when there is nothing to show.
func installableEntry(t *testing.T, m ArsenalCmp) (string, []string) {
	t.Helper()
	if m.pm == arsenal.Unknown {
		t.Skip("no supported package manager here; the plan has nothing to offer")
	}
	man, err := arsenal.Load()
	if err != nil {
		t.Fatalf("arsenal.Load: %v", err)
	}
	// GORILLA OVERRIDE (2026-09-03), first Linux run: also require the entry to
	// be ABSENT. toggle() deliberately skips anything already installed — that
	// is the 2026-08-19 fix in arsenal.go — so on a machine that HAS the first
	// available package (this Linux box has poppler-utils; the Windows box had
	// nothing) the helper handed back an entry that selects nothing, showPlan()
	// correctly refused to open a plan for an empty selection, and both plan
	// tests failed while describing the symptom rather than the cause.
	//
	// A bare-machine assumption, not a platform one: it would have failed the
	// same way on any Windows box with the tools already on it.
	for _, s := range man.Series {
		for _, e := range s.Entries {
			if arsenal.Available(e, m.pm) && !m.status[e.ID].Present {
				return e.ID, arsenal.PackagesFor(e, m.pm)
			}
		}
	}
	t.Skipf("nothing installable and not already present with %q", m.pm)
	return "", nil
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
	// AMENDED 2026-10-05: the first series is marked installed here, so the
	// test runs on every machine. It used to skip wherever no series happened
	// to be complete — which, before audit A10, was every Windows machine.
	m := bareArsenal(t, arsenal.Unknown)
	for _, e := range m.man.Series[0].Entries {
		m.status[e.ID] = arsenal.Status{Present: true}
	}

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
			// AMENDED 2026-08-19 from a screenshot: the invariant is "never
			// TALLER than the terminal", not "always exactly it". Padding a
			// short page out to full height put thirty blank rows under eight
			// rows of content, covering the sidebar and the conversation to
			// show nothing, and reading as a program that had stopped.
			if gotH > size.h {
				t.Errorf("%dx%d view %d: frame is %d rows tall, taller than the terminal",
					size.w, size.h, view, gotH)
			}
			if gotH < 3 {
				t.Errorf("%dx%d view %d: frame collapsed to %d rows", size.w, size.h, view, gotH)
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
	// Content cannot fit at this height, so the frame DOES fill it — that is
	// the case the budget arithmetic exists for.
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

// GORILLA OVERRIDE (2026-08-19), reported from a real run with a screenshot:
// the header showed "97.9 MB to download, 331.0 MB on disk, about 3.4 hours"
// while the line underneath still read "measuring with apt...". The screen
// displayed the answer and simultaneously denied having it, and the reasonable
// conclusion for anyone reading that is that the program has hung.
//
// A progress message must be REPLACED BY ITS OUTCOME. Same rule as everywhere
// else here: a state that has finished must not still look like one that is
// running.
//
// AMENDED 2026-10-05: states its machine (apt, nothing installed, assumed link
// speed). It used whatever the test machine had, which on Scoop cannot measure
// at all, and on a machine with a timed link turns "hours" into minutes.
func TestTheMeasuringNoticeIsReplacedByTheResult(t *testing.T) {
	m := bareArsenal(t, arsenal.APT)
	m.toggle([]string{"zbar"})

	cmd := m.price()
	if cmd == nil {
		t.Fatal("pricing a real selection produced no work")
	}
	if !strings.Contains(m.notice, "measuring") {
		t.Fatalf("no progress message while measuring: %q", m.notice)
	}

	// The measurement comes back.
	var model tea.Model = m
	model, _ = model.Update(arsenalPricedMsg{
		key:  strings.Join(m.selectedIDs(), ","),
		cost: arsenal.Cost{DownloadBytes: 97_900_000, DiskBytes: 331_000_000, Measured: true, DiskMeasured: true},
	})
	got := model.(ArsenalCmp)

	if strings.Contains(got.notice, "measuring") {
		t.Fatalf("the progress message survived the result: %q", got.notice)
	}
	for _, want := range []string{"97.9 MB", "331.0 MB", "hours"} {
		if !strings.Contains(got.notice, want) {
			t.Errorf("the outcome does not report %q: %q", want, got.notice)
		}
	}
	if !strings.Contains(got.View(), "97.9 MB") {
		t.Error("the outcome was set but never rendered")
	}
}

// A failed measurement must say so rather than sit on "measuring" forever.
func TestAFailedMeasurementReplacesTheNoticeToo(t *testing.T) {
	m := bareArsenal(t, arsenal.APT)
	m.toggle([]string{"zbar"})
	m.price()

	var model tea.Model = m
	model, _ = model.Update(arsenalPricedMsg{
		key:  strings.Join(m.selectedIDs(), ","),
		cost: arsenal.Cost{Note: "E: Unable to locate package zbar-tools"},
	})
	got := model.(ArsenalCmp)
	if strings.Contains(got.notice, "measuring") {
		t.Fatalf("a failed measurement left the progress message up: %q", got.notice)
	}
	if !strings.Contains(got.notice, "could not measure") {
		t.Errorf("the failure was not reported: %q", got.notice)
	}
}

// GORILLA OVERRIDE (2026-08-19), from a screenshot: a short page must not
// reserve the whole screen to display nothing.
//
// The series list is eight rows. Padded to a 52-row terminal it covered the
// sidebar and the conversation with thirty blank rows, which reads as a program
// that stopped halfway. "Never taller than the terminal" is the invariant;
// "always exactly the terminal" was my addition and it was wrong.
func TestAShortPageDoesNotReserveTheWholeScreen(t *testing.T) {
	m := NewArsenalCmp()
	m.SetSize(150, 52)
	out := m.View()
	h := lipgloss.Height(out)
	if h > 52 {
		t.Fatalf("frame is %d rows, taller than the terminal", h)
	}
	if h >= 45 {
		t.Errorf("the series list rendered %d rows on a 52-row terminal; it has %d series and "+
			"the rest is blank padding covering the conversation for nothing",
			h, len(m.man.Series))
	}
	t.Logf("series list on a 52-row terminal: %d rows", h)
}

// GORILLA OVERRIDE (2026-08-19): this page must be a WINDOW, not a takeover.
//
// It was the only page taking the full terminal width. Two consequences, both
// visible in screenshots and both mistaken for bugs: it covered the sidebar,
// and its border landed on the very edge of the screen where it reads as no
// border at all.
func TestThePageIsAWindowNotATakeover(t *testing.T) {
	for _, w := range []int{150, 200, 120} {
		m := NewArsenalCmp()
		m.SetSize(w, 52)
		got := lipgloss.Width(m.View())
		if got >= w {
			t.Errorf("at %d columns the page rendered %d wide — it covers the whole terminal, "+
				"so its border sits on the screen edge and reads as no border", w, got)
		}
	}
	// On a genuinely narrow terminal it may use nearly everything; it must
	// still never exceed it.
	m := NewArsenalCmp()
	m.SetSize(70, 24)
	if got := lipgloss.Width(m.View()); got > 70 {
		t.Errorf("at 70 columns the page rendered %d wide", got)
	}
}

// ── 2026-10-05: tests for the /arsenal audit, defects A1 to A14 ────────────

// GORILLA FIX (2026-10-05), audit A2: on a Windows machine without Scoop, `a`
// then `p` answered "measured: nothing to download - all of it is already
// here" for every missing tool. Zero packages is "cannot fetch", not "have".
func TestASelectionNothingCanFetchIsNeverReportedAsNothingToDownload(t *testing.T) {
	for _, pm := range []arsenal.PackageManager{arsenal.Unknown, arsenal.APT, arsenal.Pacman, arsenal.Scoop} {
		m := bareArsenal(t, pm)
		// Take every listed entry this package manager has no package for.
		var unfetchable []string
		for _, s := range m.man.Series {
			for _, e := range s.Entries {
				if !arsenal.Available(e, pm) {
					unfetchable = append(unfetchable, e.ID)
				}
			}
		}
		if len(unfetchable) == 0 {
			continue // this package manager can fetch everything listed here
		}
		m.toggle(unfetchable)

		got := arsKey(m, "p")
		if strings.Contains(got.notice, "nothing to download") || strings.Contains(got.notice, "already here") {
			t.Errorf("%q: an unfetchable selection was priced as free: %q", pm, got.notice)
		}
		if got.notice == "" {
			t.Errorf("%q: p on an unfetchable selection said nothing", pm)
		}
		if len(got.pricing) != 0 {
			t.Errorf("%q: a price was recorded for a selection with no packages: %+v", pm, got.pricing)
		}
		for name, text := range arsAllViews(got) {
			if strings.Contains(text, "nothing to download") || strings.Contains(text, "already here") {
				t.Errorf("%q, %s view: says there is nothing to download:\n%s", pm, name, text)
			}
		}
		if plan := arsAllViews(got)["plan"]; strings.Contains(plan, "Nothing to install.") {
			t.Errorf("%q: the plan says \"Nothing to install.\" for tools that are not installed and cannot be fetched:\n%s", pm, plan)
		}
		if head := arsAllViews(got)["series"]; !strings.Contains(head, "none of it can be fetched") {
			t.Errorf("%q: the header does not say the selection cannot be fetched:\n%s", pm, head)
		}
	}
}

// Audit A2/A4, the other route to the same lie: an answer that came back
// unmeasured must never be rendered as a figure.
func TestAnUnmeasuredAnswerIsNeverRenderedAsZero(t *testing.T) {
	m := bareArsenal(t, arsenal.APT)
	e := arsFirstEntry(t, m, true)
	m.toggle([]string{e.ID})
	var model tea.Model = m
	model, _ = model.Update(arsenalPricedMsg{
		key:  strings.Join(m.selectedIDs(), ","),
		cost: arsenal.Cost{Note: "apt answered, but not in a form this program can read — not measured"},
	})
	got := model.(ArsenalCmp)
	for name, text := range arsAllViews(got) {
		if strings.Contains(text, "0 B") || strings.Contains(text, "nothing left to download") {
			t.Errorf("%s view shows a zero for an unmeasured cost:\n%s", name, text)
		}
	}
	if !strings.Contains(got.notice, "could not measure") {
		t.Errorf("notice = %q", got.notice)
	}
	if plan := arsAllViews(got)["plan"]; !strings.Contains(plan, "cost not measured") {
		t.Errorf("the plan does not say the cost is unmeasured:\n%s", plan)
	}

	// A download figure with no disk figure (pacman) must not print "0 B".
	model, _ = model.Update(arsenalPricedMsg{
		key:  strings.Join(m.selectedIDs(), ","),
		cost: arsenal.Cost{DownloadBytes: 5_000_000, Measured: true, Note: "download only"},
	})
	got = model.(ArsenalCmp)
	for name, text := range arsAllViews(got) {
		if strings.Contains(text, "0 B") {
			t.Errorf("%s view prints an unreported disk size as 0 B:\n%s", name, text)
		}
	}
	if !strings.Contains(got.notice, "unreported") {
		t.Errorf("the notice does not say the disk size is unreported: %q", got.notice)
	}
}

// GORILLA FIX (2026-10-05), audit A3: on Scoop the screen offered "p measure
// the real cost" in three places and p could only answer that it cannot.
func TestScoopIsNeverOfferedAMeasurementItCannotMake(t *testing.T) {
	for _, pm := range []arsenal.PackageManager{arsenal.Scoop, arsenal.Unknown} {
		m := bareArsenal(t, pm)
		m.toggle(m.everyID())
		if strings.Contains(m.notice, "press p") {
			t.Errorf("%q: selecting offered p: %q", pm, m.notice)
		}
		for name, text := range arsAllViews(m) {
			for _, offer := range []string{"p measure", "press p", "p cost"} {
				if strings.Contains(text, offer) {
					t.Errorf("%q, %s view offers %q:\n%s", pm, name, offer, text)
				}
			}
		}
		if pm == arsenal.Scoop {
			if plan := arsAllViews(m)["plan"]; !strings.Contains(plan, "scoop does not report download size") {
				t.Errorf("the plan does not say why there is no cost:\n%s", plan)
			}
		}
		// The key still answers if pressed.
		got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
		if cmd != nil {
			t.Errorf("%q: p started a measurement that cannot be made", pm)
		}
		if n := got.(ArsenalCmp).notice; !strings.Contains(n, "Cannot measure") && !strings.Contains(n, "No supported") {
			t.Errorf("%q: p said %q", pm, n)
		}
	}
}

// GORILLA FIX (2026-10-05), audit A5: esc from the install plan went to the
// detail page whatever came before it.
func TestLeavingThePlanReturnsToWhereItWasOpened(t *testing.T) {
	for _, from := range []arsenalView{viewSeries, viewEntries, viewDetail} {
		m := bareArsenal(t, arsenal.APT)
		m.toggle(m.everyID())
		m.view = from
		var model tea.Model = arsKey(m, "i")
		if model.(ArsenalCmp).view != viewPlan {
			t.Fatalf("from view %d: i did not open the plan", from)
		}
		// i again on the plan must not make the plan its own way back.
		model = arsKey(model, "i")
		if got := pressNamed(model, tea.KeyEsc).(ArsenalCmp).view; got != from {
			t.Errorf("opened the plan from view %d, esc went to view %d", from, got)
		}
		if got := pressNamed(model, tea.KeyLeft).(ArsenalCmp).view; got != from {
			t.Errorf("opened the plan from view %d, left went to view %d", from, got)
		}
	}
}

// GORILLA FIX (2026-10-05), audit A7: the one key that spends money was the
// one key printed nowhere, and it was silent when it had nothing to send.
func TestTheKeyThatCostsATurnIsLabelledAndNeverSilent(t *testing.T) {
	m := bareArsenal(t, arsenal.APT)
	for _, name := range []string{"series", "plan"} {
		text := arsAllViews(m)[name]
		if name == "plan" {
			m.toggle(m.everyID())
			text = arsAllViews(m)[name]
			m.selected = map[string]bool{}
		}
		if !strings.Contains(text, "? send the selection to the model") || !strings.Contains(text, "costs one model turn") {
			t.Errorf("%s view does not label the ? key with what it costs:\n%s", name, text)
		}
	}

	// Nothing selected.
	got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if cmd != nil || got.(ArsenalCmp).notice == "" {
		t.Errorf("? with nothing selected: cmd=%v notice=%q", cmd != nil, got.(ArsenalCmp).notice)
	}

	// Selected, and none of it fetchable.
	m.toggle([]string{arsFirstEntry(t, m, false).ID})
	got, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if cmd != nil {
		t.Error("? sent an unfetchable selection to the model")
	}
	if n := got.(ArsenalCmp).notice; !strings.Contains(n, "Nothing was sent") {
		t.Errorf("? with nothing fetchable said %q", n)
	}

	// Selected and fetchable: the message goes, with every command line.
	s := bareArsenal(t, arsenal.Scoop)
	s.toggle(s.everyID())
	pkgs, _, _ := s.selectionSummary()
	_, cmd = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if cmd == nil {
		t.Fatal("? with a fetchable selection sent nothing")
	}
	msg, ok := cmd().(ArsenalInstallMsg)
	if !ok {
		t.Fatalf("? produced %T", cmd())
	}
	if strings.Contains(msg.Command, ";") {
		t.Errorf("the command sent to the model joins commands with ';': %q", msg.Command)
	}
	for _, line := range arsenal.InstallCommands(pkgs, arsenal.Scoop) {
		if !strings.Contains(msg.Command, line) {
			t.Errorf("the command sent to the model lacks %q: %q", line, msg.Command)
		}
	}
}

// GORILLA FIX (2026-10-05), audit A9: the 8 KB/s was typed. It is now this
// machine's own measured link speed when there is one, and otherwise an
// assumption that is called an assumption on every line that uses it.
func TestTheLinkSpeedIsMeasuredOrLabelledAsAssumed(t *testing.T) {
	m := NewArsenalCmp()
	kib, ok := config.EstimatedKBps()
	if m.linkMeasured != ok {
		t.Fatalf("linkMeasured = %v while the program's own link estimate says ok=%v", m.linkMeasured, ok)
	}
	if ok && (m.linkSpeed < kib || m.linkSpeed > kib*1.03) {
		t.Errorf("link speed %.2f does not come from the measured %.2f KiB/s", m.linkSpeed, kib)
	}
	if !ok && m.linkSpeed != assumedLinkKBps {
		t.Errorf("with no measurement the speed is %.2f, want the assumed %d", m.linkSpeed, assumedLinkKBps)
	}

	priced := func(m ArsenalCmp) ArsenalCmp {
		m.toggle([]string{arsFirstEntry(t, m, true).ID})
		var model tea.Model = m
		model, _ = model.Update(arsenalPricedMsg{
			key:  strings.Join(m.selectedIDs(), ","),
			cost: arsenal.Cost{DownloadBytes: 18_000_000, DiskBytes: 40_000_000, Measured: true, DiskMeasured: true},
		})
		return model.(ArsenalCmp)
	}

	assumed := priced(bareArsenal(t, arsenal.APT))
	views := arsAllViews(assumed)
	if !strings.Contains(views["series"], "ASSUME 8 KB/s") {
		t.Errorf("the series page does not label the speed as an assumption:\n%s", views["series"])
	}
	for _, name := range []string{"series", "plan"} {
		if !strings.Contains(views[name], "at an assumed 8 KB/s") {
			t.Errorf("%s: a time figure does not say its speed is assumed:\n%s", name, views[name])
		}
		if strings.Contains(views[name], "measured on this link") {
			t.Errorf("%s: claims a link measurement that was not made:\n%s", name, views[name])
		}
	}

	fast := bareArsenal(t, arsenal.APT)
	fast.linkSpeed, fast.linkMeasured = 500, true
	views = arsAllViews(priced(fast))
	if strings.Contains(views["series"], "ASSUME") || strings.Contains(views["plan"], "assumed") {
		t.Errorf("a measured speed is still labelled as assumed:\n%s\n%s", views["series"], views["plan"])
	}
	if !strings.Contains(views["plan"], "at 500 KB/s, measured on this link") {
		t.Errorf("the plan does not use the measured speed:\n%s", views["plan"])
	}
	if !strings.Contains(views["plan"], "36 seconds") {
		t.Errorf("18 MB at 500 KB/s is 36 seconds; the plan says:\n%s", views["plan"])
	}
}

// GORILLA FIX (2026-10-05), audit A10: what cannot exist on this operating
// system is neither listed nor counted, and no series is listed empty.
func TestThePageListsOnlyWhatCanExistOnThisSystem(t *testing.T) {
	m := NewArsenalCmp()
	listed := 0
	for _, s := range m.man.Series {
		if len(s.Entries) == 0 {
			t.Errorf("series %q is listed with nothing in it", s.ID)
		}
		for _, e := range s.Entries {
			listed++
			if !arsenal.Applicable(e) {
				t.Errorf("%s is listed and cannot exist on this system", e.ID)
			}
		}
	}
	if len(m.status) != listed {
		t.Errorf("%d entries probed, %d listed — the header would count the difference as missing", len(m.status), listed)
	}
	full := 0
	for _, s := range m.full.Series {
		for _, e := range s.Entries {
			full++
			if !arsenal.Applicable(e) {
				if _, probed := m.status[e.ID]; probed {
					t.Errorf("%s cannot exist here and was probed and counted", e.ID)
				}
			}
		}
	}
	if runtime.GOOS == "windows" {
		if listed >= full {
			t.Errorf("Windows lists all %d entries, including the Linux-only ones", full)
		}
		if _, ok := m.status["bubblewrap"]; ok {
			t.Error("bubblewrap is counted on Windows; \"The minimum\" can never be complete")
		}
		for _, s := range m.man.Series {
			if s.ID == "windows" {
				t.Errorf("%q is listed on Windows", s.Title)
			}
		}
	}

	// A fully installed series reads N/N.
	b := bareArsenal(t, arsenal.Unknown)
	s := b.man.Series[0]
	for _, e := range s.Entries {
		b.status[e.ID] = arsenal.Status{Present: true}
	}
	want := fmt.Sprintf("%d/%d here", len(s.Entries), len(s.Entries))
	if text := arsAllViews(b)["series"]; !strings.Contains(text, want) {
		t.Errorf("a series with everything installed does not read %q:\n%s", want, text)
	}
}

// GORILLA FIX (2026-10-05), audit A11: two commands, two lines, on every page
// that shows them — and never a line break hidden inside one row.
func TestScoopCommandsAreShownOnePerLine(t *testing.T) {
	m := bareArsenal(t, arsenal.Scoop)
	for i, s := range m.man.Series {
		for j, e := range s.Entries {
			if e.ID == "libreoffice" {
				m.seriesIdx, m.entryIdx = i, j
			}
		}
	}
	m.toggle([]string{"libreoffice", "testdisk"})
	for name, v := range map[string]arsenalView{"detail": viewDetail, "plan": viewPlan} {
		m.view = v
		install := 0
		for _, l := range m.lines() {
			if strings.Contains(l.text, "\n") {
				t.Errorf("%s: a row contains a line break and would grow the frame: %q", name, l.text)
			}
			if strings.Contains(l.text, "; scoop") || strings.Contains(l.text, "&&") {
				t.Errorf("%s: two commands joined on one row: %q", name, l.text)
			}
			if strings.HasPrefix(strings.TrimSpace(l.text), "scoop install ") {
				install++
				if strings.Contains(l.text, "bucket add") {
					t.Errorf("%s: the bucket command and the install command share a row: %q", name, l.text)
				}
			}
		}
		if install != 1 {
			t.Errorf("%s: %d rows carry the install command, want 1", name, install)
		}
	}
}

// GORILLA FIX (2026-10-05), audit A12, first part: n cleared the selection and
// said nothing.
func TestClearingTheSelectionSaysSo(t *testing.T) {
	m := bareArsenal(t, arsenal.APT)
	if got := arsKey(m, "n"); got.notice == "" {
		t.Error("n with nothing selected did nothing and said nothing")
	}
	m.toggle(m.everyID())
	n := len(m.selectedIDs())
	got := arsKey(m, "n")
	if len(got.selectedIDs()) != 0 {
		t.Fatal("n did not clear the selection")
	}
	if !strings.Contains(got.notice, fmt.Sprintf("un-selected %d", n)) {
		t.Errorf("n cleared %d and said %q", n, got.notice)
	}
}

// Audit A12, second part: a selection file from somebody else is loadable
// without overwriting your own, and a real read error is not reported as "no
// file".
func TestLoadWalksEverySelectionFileAndReportsRealErrors(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := bareArsenal(t, arsenal.APT)

	m.loadTagfile()
	if !strings.Contains(m.notice, ".tagfile") || !strings.Contains(m.notice, arsenal.TagfileDir()) {
		t.Fatalf("with no file, the notice does not say what to put where: %q", m.notice)
	}

	mine, theirs := arsFirstEntry(t, m, true).ID, ""
	for _, id := range m.everyID() {
		if id != mine {
			theirs = id
		}
	}
	m.toggle([]string{mine})
	m.saveTagfile()
	own, err := os.ReadFile(arsenal.TagfilePath())
	if err != nil {
		t.Fatal(err)
	}
	friend := filepath.Join(arsenal.TagfileDir(), "from-a-friend.tagfile")
	if err := os.WriteFile(friend, []byte("# theirs\n"+theirs+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m.selected = map[string]bool{}
	m.loadTagfile()
	if !m.selected[mine] || !strings.Contains(m.notice, "selection.tagfile") || !strings.Contains(m.notice, "file 1 of 2") {
		t.Errorf("first L: selected=%v notice=%q", m.selectedIDs(), m.notice)
	}
	m.loadTagfile()
	if !m.selected[theirs] || m.selected[mine] || !strings.Contains(m.notice, "from-a-friend.tagfile") {
		t.Errorf("second L did not load the friend's file: selected=%v notice=%q", m.selectedIDs(), m.notice)
	}
	m.loadTagfile()
	if !m.selected[mine] {
		t.Errorf("third L did not come back round to the first file: %q", m.notice)
	}
	if after, _ := os.ReadFile(arsenal.TagfilePath()); string(after) != string(own) {
		t.Error("loading somebody else's selection changed the user's own file")
	}

	// A thing that is there and cannot be read is not "no selection".
	if err := os.Remove(friend); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(arsenal.TagfilePath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(arsenal.TagfileDir(), "broken.tagfile"), 0o700); err != nil {
		t.Fatal(err)
	}
	m.tagIdx = 0
	m.loadTagfile()
	if !strings.Contains(m.notice, "Could not read") || strings.Contains(m.notice, "No selection") {
		t.Errorf("an unreadable selection was reported as %q", m.notice)
	}
}

// Audit A10 meeting A12: an id that is real but cannot exist on this system is
// not "not in this version".
func TestATagfileFromAnotherSystemIsExplainedNotCalledUnknown(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m := bareArsenal(t, arsenal.APT)
	// Imagine a system where the last series cannot exist.
	gone := m.man.Series[len(m.man.Series)-1]
	m.man.Series = m.man.Series[:len(m.man.Series)-1]
	kept := m.man.Series[0].Entries[0].ID

	if err := os.MkdirAll(arsenal.TagfileDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	body := kept + "\n" + gone.Entries[0].ID + "\nquantum-decompiler\n"
	if err := os.WriteFile(arsenal.TagfilePath(), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	m.loadTagfile()
	if !m.selected[kept] || m.selected[gone.Entries[0].ID] {
		t.Errorf("selected = %v", m.selectedIDs())
	}
	if !strings.Contains(m.notice, "1 cannot exist on this system") {
		t.Errorf("the entry from another system was not explained: %q", m.notice)
	}
	if !strings.Contains(m.notice, "1 not in this version: quantum-decompiler") {
		t.Errorf("the truly unknown id was not reported, or the other one was lumped in with it: %q", m.notice)
	}
}

// Audit A1 and A6 on screen: a rejected impostor and an off-PATH find are both
// explained where the user is looking.
func TestTheDetailPageExplainsImpostorsAndOffPathFinds(t *testing.T) {
	m := bareArsenal(t, arsenal.Scoop)
	e := m.man.Series[0].Entries[0]
	m.view = viewDetail

	m.status[e.ID] = arsenal.Status{
		Missing: []string{"convert"},
		Ignored: []arsenal.IgnoredHit{{Binary: "convert", Path: `C:\Windows\System32\convert.exe`, Is: "Windows' own disk converter"}},
	}
	text := arsAllViews(m)["detail"]
	if strings.Contains(text, "ALREADY ON THIS MACHINE") {
		t.Errorf("an impostor produced a HAVE:\n%s", text)
	}
	if !strings.Contains(text, "not counted: convert") || !strings.Contains(text, "disk converter") {
		t.Errorf("the page does not say why convert.exe was not counted:\n%s", text)
	}
	if badge, _ := m.stateBadge(e); badge == "HAVE" {
		t.Error("an impostor produced a HAVE badge")
	}

	m.status[e.ID] = arsenal.Status{Present: true, OffPath: true, Found: []string{`C:\x\soffice.exe`}}
	text = arsAllViews(m)["detail"]
	if !strings.Contains(text, `C:\x\soffice.exe`) || !strings.Contains(text, "NOT on PATH") {
		t.Errorf("an off-PATH find does not show its full path and say it is not on PATH:\n%s", text)
	}
}

// Audit A14: the page says which PATH it asked.
func TestThePageSaysPresenceIsAboutThisSessionsPath(t *testing.T) {
	if text := arsAllViews(bareArsenal(t, arsenal.APT))["series"]; !strings.Contains(text, "PATH this program was started with") {
		t.Errorf("the page does not say that presence depends on the PATH it was started with:\n%s", text)
	}
}

// GORILLA FIX (2026-10-05): wraps were sized from the terminal while the box is
// capped at 120 columns, so on a wide terminal the install command was wrapped
// wider than the box and then cut off with "...".
func TestNothingIsWrappedWiderThanTheBox(t *testing.T) {
	for _, w := range []int{60, 80, 120, 140, 200, 300} {
		m := bareArsenal(t, arsenal.APT)
		m.SetSize(w, 400)
		m.toggle(m.everyID())
		pkgs, _, _ := m.selectionSummary()
		for _, v := range []arsenalView{viewSeries, viewEntries, viewDetail, viewPlan} {
			m.view = v
			for _, l := range m.lines() {
				if got := lipgloss.Width(l.text); got > m.textWidth() && w >= 80 {
					t.Errorf("%d columns, view %d: a %d-column row in a %d-column box will be cut: %q",
						w, v, got, m.textWidth(), l.text)
				}
			}
		}
		// Every package name survives into the rendered plan.
		m.view = viewPlan
		out := m.View()
		for _, p := range pkgs {
			if strings.Count(out, p) < 2 { // once in the package list, once in the command
				t.Errorf("%d columns: package %q is not in both the list and the command", w, p)
			}
		}
	}
}

// GORILLA FIX (2026-10-05): the list views never scrolled, and the plan did not
// answer up/down, while both printed "down to continue".
func TestTheCursorAndThePlanStayReachableOnAShortTerminal(t *testing.T) {
	m := bareArsenal(t, arsenal.APT)
	m.SetSize(100, 14)
	var model tea.Model = m
	for range m.man.Series {
		model = pressNamed(model, tea.KeyDown)
	}
	got := model.(ArsenalCmp)
	last := got.man.Series[len(got.man.Series)-1]
	if got.seriesIdx != len(got.man.Series)-1 {
		t.Fatalf("down did not reach the last series: %d", got.seriesIdx)
	}
	out := got.View()
	if !strings.Contains(out, "> "+last.Title[:10]) {
		t.Errorf("the cursor is on %q and that row is not on screen:\n%s", last.Title, out)
	}
	if h := lipgloss.Height(out); h > 14 {
		t.Errorf("frame is %d rows in a 14-row terminal", h)
	}

	got.toggle(got.everyID())
	got.showPlan()
	before := got.View()
	if !strings.Contains(before, "more line") {
		t.Skip("the plan fits in 14 rows on this manifest; nothing to scroll")
	}
	model = got
	for i := 0; i < 400; i++ {
		model = pressNamed(model, tea.KeyDown)
	}
	after := model.(ArsenalCmp).View()
	if after == before {
		t.Fatal("down on the install plan did nothing while the page said \"down to continue\"")
	}
	if !strings.Contains(after, "will not run it") {
		t.Errorf("the end of the plan cannot be reached by scrolling:\n%s", after)
	}
}
