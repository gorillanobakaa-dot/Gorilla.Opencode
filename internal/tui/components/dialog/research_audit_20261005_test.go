package dialog

// GORILLA (2026-10-05): tests for the /research and /osint audit fixes on the
// screens. The rule under all of them: a number or a promise on these screens
// must come from the code that makes it true. Each test names its audit finding
// (S = /research, O = /osint) and the text that was on screen before.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
)

// seedRunHistory points the cache at a throwaway folder and, when history is
// not empty, writes it as this machine's record of finished research runs. An
// empty history is a machine that has never finished one.
func seedRunHistory(t *testing.T, history string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	if history == "" {
		return
	}
	path := filepath.Join(config.CacheBase(), "research-runs.json")
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("cache redirection failed: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(history), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The August run this project quotes: 8 sessions, 280,744 tokens.
const augustRun = `[{"sessions":8,"tokens_in":248122,"tokens_out":32622,"tool_calls":96,"seconds":780}]`

// flatText undoes wrapping and box drawing, so a sentence can be looked for in a
// rendered frame whatever width it was wrapped at.
func flatText(s string) string {
	s = strings.NewReplacer("║", " ", "│", " ", "╔", " ", "╗", " ", "╚", " ", "╝", " ", "═", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func itoaDialog(n int) string { return strconv.Itoa(n) }

func pageText() string {
	var all []string
	for _, l := range osintContent() {
		all = append(all, l.text)
	}
	return flatText(strings.Join(all, " "))
}

func loadTestConfig(t *testing.T) {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
}

// ─── O1: the page describes lanes, because lanes are what runs ───

// Before: "1. PLAN — your question is broken into sub-questions, each with
// indicators", "helper agents work the sub-questions". No planning code exists.
func TestOsintPageDescribesFixedLanesNotAPlanningStage(t *testing.T) {
	loadTestConfig(t)
	page := pageText()

	for _, gone := range []string{"sub-question", "PLAN ", "indicators", "plan what would answer"} {
		if strings.Contains(page, gone) {
			t.Errorf("the page still describes a planning stage (%q); there is none", gone)
		}
	}
	for _, want := range []string{"there is no planning stage", "fixed lanes", "No lane searches your own machine"} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not say %q", want)
		}
	}
	// Every lane a full run would work is named, from the role table.
	titles := agent.DossierLaneTitles(agent.ResearchMaxAgents)
	if len(titles) != agent.ResearchMaxAgents {
		t.Fatalf("a full dossier has %d lanes, want %d", len(titles), agent.ResearchMaxAgents)
	}
	for _, title := range titles {
		if !strings.Contains(page, flatText(title)) {
			t.Errorf("the page does not list the lane %q", title)
		}
	}
}

// ─── O2: the follow-up is optional, bounded, and in the cost gate ───

// Before: "then a gap round hunts what they missed" and "Dossiers add a gap
// round on top" — a round nothing ran, at a cost no figure contained.
func TestOsintGateStatesTheFollowUpAsItIs(t *testing.T) {
	loadTestConfig(t)
	seedRunHistory(t, "")

	for i, o := range osintModes {
		d := NewOsintDialogCmp("q")
		d.selected = i
		line := d.followUpLine()
		ceiling := agent.DossierFollowUpSessions(o.mode)
		for _, want := range []string{"OPTIONAL", "ONCE", "at most " + strconv.Itoa(ceiling) + " more sessions", "refuses", "NOT IN THE FIGURES ABOVE"} {
			if !strings.Contains(line, want) {
				t.Errorf("%s: the follow-up line does not say %q: %s", o.mode, want, line)
			}
		}
		// It is on the full screen, and it survives into the leanest one.
		d.SetSize(160, 70)
		if v := flatText(d.View()); !strings.Contains(v, "one OPTIONAL follow-up") {
			t.Errorf("%s: the gate does not show the follow-up cost", o.mode)
		}
		for _, sz := range [][2]int{{80, 24}, {60, 10}, {40, 8}} {
			d.SetSize(sz[0], sz[1])
			v := flatText(d.View())
			short := strings.Contains(v, "optional follow-up: max "+strconv.Itoa(ceiling)+" more")
			long := strings.Contains(v, "at most "+strconv.Itoa(ceiling)+" more sessions")
			if !short && !long {
				t.Errorf("%s at %dx%d: the follow-up cost was dropped from the gate:\n%s", o.mode, sz[0], sz[1], d.View())
			}
		}
	}

	d := NewOsintDialogCmp("q")
	d.SetSize(160, 70)
	everything := flatText(d.View()) + " " + assumptionsLine() + " " + pageText()
	for _, gone := range []string{"gap round hunts", "add a gap round", "attacks what the first pass missed", "plus a gap round"} {
		if strings.Contains(everything, gone) {
			t.Errorf("a screen still promises a gap round that nothing runs: %q", gone)
		}
	}
	if !strings.Contains(pageText(), "refuses a third call") {
		t.Error("the page does not state the enforced ceiling")
	}
}

// ─── O3, S15: the size of a run comes from this machine ───

// GORILLA (2026-10-06): until this date the screen priced the assumption and
// printed the measurement beside it with a multiplier ("expect about 1.6x the
// money shown"). The money is priced from the measurement now, so this test
// checks that the per-helper figure moves to the measured basis, that the
// screen names the basis in use, and that the dialog is no taller for it.
func TestResearchScreenPricesTheMeasuredBasis(t *testing.T) {
	pricedConfig(t)
	seedRunHistory(t, "")

	m := dialogAt("supervised", agent.ResearchMaxAgents)
	text := func() string {
		var lines []string
		for _, l := range m.costLines() {
			lines = append(lines, l.text)
		}
		return strings.Join(lines, "\n")
	}
	assumedIn, assumedOut, _, measured := config.ResearchHelperSessionTokens()
	if measured {
		t.Fatal("an empty history reports a measured basis")
	}
	wantAssumed := fmt.Sprintf("BASIS: %s tokens per helper session, assumed; no run measured yet.", humanCount(assumedIn+assumedOut))
	if !strings.Contains(text(), wantAssumed) {
		t.Errorf("with nothing measured the screen must say %q:\n%s", wantAssumed, text())
	}
	if strings.Contains(text(), "expect about") {
		t.Errorf("nothing is measured, yet the screen apologises for the forecast:\n%s", text())
	}
	perHelperAssumed, _, _, _, priced := config.ResearchCost(4)
	if !priced {
		t.Fatal("pricedConfig did not price the helper model")
	}

	// Heights before any measurement exists, to compare against below.
	sizes := []struct{ w, h int }{{176, 48}, {120, 40}, {100, 32}}
	before := map[int]int{}
	for _, sz := range sizes {
		m.SetSize(sz.w, sz.h)
		before[sz.h] = lipgloss.Height(m.View())
	}
	m.SetSize(140, 60)

	seedRunHistory(t, augustRun)
	got := text()
	in, out, runs, measured := config.ResearchHelperSessionTokens()
	if !measured || runs != 1 || in+out != 35093 {
		t.Fatalf("one August run on record gives basis in=%d out=%d runs=%d measured=%v, want 35,093 over 1 run", in, out, runs, measured)
	}
	want := fmt.Sprintf("BASIS: %s tokens per helper session, measured from 1 run on this computer.", humanCount(in+out))
	if !strings.Contains(got, want) {
		t.Errorf("the screen does not name the measured basis (%q):\n%s", want, got)
	}
	// S15, closed: the money itself follows the measurement. The per-helper
	// figure must have moved by the ratio of the two bases, within rounding of
	// the in/out split, and no multiplier may be left on screen to confess a
	// forecast that is no longer low.
	perHelperMeasured, _, _, _, _ := config.ResearchCost(4)
	ratio := float64(in+out) / float64(assumedIn+assumedOut)
	if moved := perHelperMeasured / perHelperAssumed; moved < ratio*0.8 || moved > ratio*1.25 {
		t.Errorf("per-helper money moved %.2fx between the assumed and measured basis; the bases differ %.2fx", moved, ratio)
	}
	if strings.Contains(got, "expect about") || strings.Contains(got, "assumed; no run measured yet") {
		t.Errorf("a measured basis is priced, yet the screen still speaks of the assumed one:\n%s", got)
	}

	// A measurement may not make the dialog taller: every row here is one the
	// key hints can be pushed off the bottom by. The one exception is the short
	// screen, which gives the basis row only when it is measured. That row is
	// the blank one View gave up, so the dialog is no taller than before.
	for _, sz := range sizes {
		m.SetSize(sz.w, sz.h)
		v := m.View()
		allowed := before[sz.h]
		if m.compact {
			allowed++
		}
		if h := lipgloss.Height(v); h > allowed {
			t.Errorf("%dx%d: a measurement on record made the dialog %d rows, up from %d", sz.w, sz.h, h, before[sz.h])
		}
		if !strings.Contains(v, "esc: cancel") {
			t.Errorf("%dx%d: key hints missing", sz.w, sz.h)
		}
		if !strings.Contains(flatText(v), "measured from 1 run on this computer") {
			t.Errorf("%dx%d: the basis row was dropped", sz.w, sz.h)
		}
	}
}

// The two cost screens price one engine from one basis, and must name it in
// the same words: "measured from N runs on this computer" once a run has
// finished here, "assumed; no run measured yet" before.
func TestBothCostScreensNameTheSameForecastBasis(t *testing.T) {
	pricedConfig(t)

	screens := func() map[string]string {
		m := dialogAt("parallel", 4)
		m.SetSize(160, 60)
		d := NewOsintDialogCmp("q")
		d.SetSize(160, 60)
		return map[string]string{"research": flatText(m.View()), "gate": flatText(d.View())}
	}

	seedRunHistory(t, "")
	phrase, measured := forecastBasisPhrase()
	if measured || phrase != "assumed; no run measured yet" {
		t.Fatalf("empty history: phrase %q measured=%v", phrase, measured)
	}
	for name, v := range screens() {
		if !strings.Contains(v, phrase) {
			t.Errorf("%s screen does not say %q with no run on record:\n%s", name, phrase, v)
		}
	}

	// Three runs: the wording counts them, and both screens carry it.
	seedRunHistory(t, `[{"sessions":4,"tokens_in":100000,"tokens_out":20000,"tool_calls":40,"seconds":300},`+
		`{"sessions":8,"tokens_in":248122,"tokens_out":32622,"tool_calls":96,"seconds":780},`+
		`{"sessions":2,"tokens_in":90000,"tokens_out":10000,"tool_calls":10,"seconds":200}]`)
	phrase, measured = forecastBasisPhrase()
	if !measured || phrase != "measured from 3 runs on this computer" {
		t.Fatalf("three runs: phrase %q measured=%v", phrase, measured)
	}
	for name, v := range screens() {
		if !strings.Contains(v, phrase) {
			t.Errorf("%s screen does not say %q with three runs on record:\n%s", name, phrase, v)
		}
		if strings.Contains(v, "assumed; no run measured yet") {
			t.Errorf("%s screen calls a measured basis assumed", name)
		}
	}

	// The gate's SIZE line is the priced basis times the sessions selected.
	d := NewOsintDialogCmp("q")
	d.SetSize(160, 60)
	in, out, _, _ := config.ResearchHelperSessionTokens()
	if want := humanCount((in+out)*d.sessions()) + " tokens (" + itoaDialog(d.sessions()) + " sessions)"; !strings.Contains(strings.Join(d.scaleLines(), " "), want) {
		t.Errorf("the gate's run size is not the priced basis x sessions (want %q):\n%s", want, strings.Join(d.scaleLines(), "\n"))
	}
}

func TestOsintGateFitsWithAMeasurementOnRecord(t *testing.T) {
	loadTestConfig(t)
	seedRunHistory(t, augustRun)
	for i := range osintModes {
		d := NewOsintDialogCmp("q")
		d.selected = i
		d.agents = agent.ResearchMaxAgents
		for _, sz := range [][2]int{{80, 24}, {120, 30}, {200, 50}} {
			d.SetSize(sz[0], sz[1])
			if h := lipgloss.Height(d.View()); h > sz[1] {
				t.Errorf("%s at %dx%d: gate is %d rows tall", osintModes[i].mode, sz[0], sz[1], h)
			}
		}
	}
}

// ─── O4: every number on the page is computed ───

// Before: "a 985-source registry", "866 of the 985 are free; 370 answer with no
// account at all", "refreshed every 15 minutes", "~18 sessions", and on the
// gate "against hundreds of real sources". Helpers are given the atlas only.
func TestOsintTextsClaimOnlyWhatAHelperIsGiven(t *testing.T) {
	loadTestConfig(t)
	seedRunHistory(t, "")
	page := pageText()
	d := NewOsintDialogCmp("q")
	d.SetSize(160, 70)
	gate := flatText(d.View())

	for _, gone := range []string{"985", "866", "370", "every 15 minutes", "hundreds", "~18", "registry"} {
		if strings.Contains(page, gone) {
			t.Errorf("the page still carries the typed claim %q", gone)
		}
		if strings.Contains(gate, gone) {
			t.Errorf("the gate still carries the typed claim %q", gone)
		}
	}

	total, needKey := agent.SourceAtlasCount()
	if total >= 100 {
		t.Fatalf("the atlas has %d entries; this test assumed tens", total)
	}
	if want := fmt.Sprintf("atlas of %d sources", total); !strings.Contains(page, want) || !strings.Contains(gate, want) {
		t.Errorf("page and gate must both state the atlas size as %q", want)
	}
	if want := fmt.Sprintf("%d of them want a free registration key", needKey); !strings.Contains(page, want) {
		t.Errorf("the page does not say %q", want)
	}

	maxSessions, maxAudited := agent.SupervisedSessionsFor(agent.DoctrineDossier, agent.ResearchMaxAgents)
	want := fmt.Sprintf("A %d-helper supervised run is %d sessions (%d of the %d lanes audited), and an optional follow-up can add up to %d more",
		agent.ResearchMaxAgents, maxSessions, maxAudited, agent.ResearchMaxAgents, agent.DossierFollowUpSessions(agent.ModeSupervised))
	if !strings.Contains(page, want) {
		t.Errorf("the page's cost sentence is not the scheduler's arithmetic; want %q", want)
	}
	if want := fmt.Sprintf("(%d to %d)", agent.ResearchMinAgents, agent.ResearchMaxAgents); !strings.Contains(page, want) {
		t.Errorf("the page does not state the helper bounds %s", want)
	}
}

// ─── O5: instructions are called instructions ───

// Before, under "Iron rules it follows": "It never cites a source it did not
// actually open" and "circular reporting is detected, not multiplied". Both are
// sentences in the helper prompt. No code checks either.
func TestOsintPageDoesNotPresentPromptInstructionsAsGuarantees(t *testing.T) {
	loadTestConfig(t)
	page := pageText()
	for _, gone := range []string{"Iron rules", "It never cites", "is detected, not multiplied", "Nothing is padded over", "No invented percentages, ever"} {
		if strings.Contains(page, gone) {
			t.Errorf("the page still states an unverified rule as a guarantee: %q", gone)
		}
	}
	for _, want := range []string{
		"instructions, not guarantees",
		"does not check that a model obeyed",
		"What the program itself does enforce",
		"helpers are told to grade",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not say %q", want)
		}
	}
}

// ─── O6: the page says what leaves the machine ───

// Before: "The folder is yours, on your machine, nowhere else." Nothing on the
// page said the question and the search terms go anywhere.
func TestOsintPageSaysWhatLeavesTheMachine(t *testing.T) {
	loadTestConfig(t)
	page := pageText()
	if strings.Contains(page, "nowhere else") {
		t.Error("the page still says the data goes nowhere else")
	}
	for _, want := range []string{
		"LEAVES:",
		"your question, in full, goes to the provider of the model",
		"search terms the helpers write go to the search and scholarly services",
		"nothing checks that they did",
		"Do not put in the question what must not leave the machine",
		"STAYS:",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the privacy section does not say %q", want)
		}
	}
	// It must be reachable by scrolling, in red, not buried in a muted footnote.
	found := false
	for _, l := range osintContent() {
		if strings.HasPrefix(l.text, "LEAVES:") {
			found = l.kind == "red"
		}
	}
	if !found {
		t.Error("the LEAVES paragraph is not rendered as a warning")
	}
}

// The page clips an over-long line with an ellipsis, which would cut the end
// off a sentence without saying so. Every line must fit as written.
func TestOsintPageLinesAreNeverClipped(t *testing.T) {
	loadTestConfig(t)
	dossierDir := config.DossierDir()
	for _, l := range osintContent() {
		if strings.Contains(l.text, dossierDir) {
			continue // the user's own path is as long as it is
		}
		if n := utf8.RuneCountInString(l.text); n > osintPageWidth {
			t.Errorf("page line is %d characters, over the %d the page shows: %q", n, osintPageWidth, l.text)
		}
	}
	// A hanging indent is kept on the wrapped lines of a step.
	lines := para("", "1. LANES   — ", strings.Repeat("word ", 60))
	if len(lines) < 2 || !strings.HasPrefix(lines[0].text, "1. LANES   — word") || !strings.HasPrefix(lines[1].text, "             word") {
		t.Errorf("para lost its lead or its hanging indent: %q / %q", lines[0].text, lines[len(lines)-1].text)
	}
}

// S1 on the page: armed but unable to run must not read as ready.
func TestOsintPageStatusKnowsTheEngineCanBeOff(t *testing.T) {
	loadTestConfig(t)
	if config.LoadoutEnabled(config.DossierComponentID) {
		t.Fatal("test premise broken: dossier row ships armed")
	}
	if !strings.Contains(pageText(), "STATUS: OFF") {
		t.Error("unarmed page does not say OFF")
	}
	config.ToggleLoadout(config.DossierComponentID)
	defer config.ToggleLoadout(config.DossierComponentID)
	if !config.LoadoutEnabled(osintResearchComponentID) {
		t.Fatal("test premise broken: research helpers ship switched off")
	}
	if !strings.Contains(pageText(), "STATUS: ARMED. /osint") {
		t.Error("armed page with the engine on does not say ARMED")
	}
	config.ToggleLoadout(osintResearchComponentID)
	defer config.ToggleLoadout(osintResearchComponentID)
	if got := pageText(); !strings.Contains(got, "BUT IT CANNOT RUN") {
		t.Errorf("dossier armed, research helpers off: the page says it is ready. Status: %s", got[:min(len(got), 300)])
	}
}

// ─── S4: the supervised warning is the scheduler's arithmetic ───

// Before: "Feeling lucky, punk? Double price, every lane checked twice."
// Above four helpers neither half is true.
func TestSupervisedWarningDoesNotClaimEveryLane(t *testing.T) {
	for n := agent.ResearchMinAgents; n <= agent.ResearchMaxAgents; n++ {
		m := dialogAt("supervised", n)
		w := m.theWarning()
		sessions, audited := agent.SupervisedSessions(n)
		if strings.Contains(w, "every lane") || strings.Contains(w, "Double price") {
			t.Errorf("%d helpers: the warning still claims every lane and a flat double: %q", n, w)
		}
		for _, want := range []string{
			"Feeling lucky",
			fmt.Sprintf("%d sessions, not %d", sessions, n),
			fmt.Sprintf("%d lanes checked twice", audited),
		} {
			if !strings.Contains(w, want) {
				t.Errorf("%d helpers: the warning does not say %q: %q", n, want, w)
			}
		}
	}
}

// ─── S5: the «m» key is the instruction, everywhere ───

// Before: a second provider warning ending `Set a "research" agent to change
// it`, and a key line that listed every key but «m».
func TestHelperModelRouteIsTheKeyNotAConfigEdit(t *testing.T) {
	loadTestConfig(t)
	seedRunHistory(t, "")
	m := NewResearchDialogCmp("q")
	m.SetSize(140, 60)
	for _, l := range m.costLines() {
		if strings.Contains(l.text, `Set a "research" agent`) {
			t.Errorf("the screen still tells the user to hand-edit config: %q", l.text)
		}
	}
	if !strings.Contains(researchFooterHint, "m: helper model") {
		t.Errorf("the key line does not list «m»: %q", researchFooterHint)
	}
	if v := m.View(); !strings.Contains(v, "m: helper model") {
		t.Error("the rendered dialog does not show the «m» key in its key line")
	}
}

// ─── S6, O7: bounds and concurrency come from the engine ───

func TestHelperBoundsAndConcurrencyComeFromTheEngine(t *testing.T) {
	loadTestConfig(t)
	seedRunHistory(t, "")
	m := NewResearchDialogCmp("q")
	m.SetSize(140, 60)
	if m.agents != agent.ResearchMinAgents {
		t.Errorf("the dialog opens at %d helpers, the engine's minimum is %d", m.agents, agent.ResearchMinAgents)
	}
	if want := fmt.Sprintf("(%d minimum, %d maximum)", agent.ResearchMinAgents, agent.ResearchMaxAgents); !strings.Contains(m.View(), want) {
		t.Errorf("the dialog does not print the engine's bounds %s", want)
	}

	// The /osint gate asks the same function the /research screen does.
	for i, o := range osintModes {
		for n := agent.ResearchMinAgents; n <= agent.ResearchMaxAgents; n++ {
			d := NewOsintDialogCmp("q")
			d.selected, d.agents = i, n
			if got, want := d.inFlight(), agent.PeakInFlight(agent.DoctrineDossier, o.mode, n); got != want {
				t.Errorf("%s/%d: gate prices %d in flight, the engine runs %d", o.mode, n, got, want)
			}
			if d.inFlight() > n {
				t.Errorf("%s/%d: %d helpers in flight is more than were asked for", o.mode, n, d.inFlight())
			}
		}
	}

	// The literals that were typed must not come back.
	src, err := os.ReadFile("research.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, typed := range []string{"inFlight := 4", "m.agents > 4 {", "m.agents < 10 {", "4 minimum, 10 maximum"} {
		if strings.Contains(string(src), typed) {
			t.Errorf("research.go carries the typed bound %q again", typed)
		}
	}
}
