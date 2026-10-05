// GORILLA OVERRIDE: this file did not exist upstream. It is /arsenal.
//
// The pitch, in the owner's words: "this gorilla should be able to ask the
// user: listen up, I can do lots of things — I need to install about 3000
// packages and I will be the best coding agent on the planet. FEELING LUCKY,
// PUNK?"
//
// And the model it is built on, also his: the Slackware installer. "The text
// one that lets you download, create, tweak and customise your install down to
// a freaking library. That level of control. In bulk, in sections, or
// INDIVIDUALLY." Three granularities, always, and a one-line description on
// every single item at the point of choosing — that is the part that TEACHES.
//
// Two rules this screen must never break:
//
//  1. COST IS INFORMATION, NOT A GATE. Show the megabytes and the hours, then
//     let them pick everything anyway. "Poor kids are usually patient. They are
//     used to slow downloads. You don't know what you don't know — they would
//     not even know where to begin. We provide exactly that." A number shown so
//     someone can choose is respect; the same number used to steer them to a
//     smaller option is condescension wearing a helpful face. "Everything" is
//     always on the menu.
//  2. NOTHING IS EVER INSTALLED FROM HERE. The command is displayed and copied;
//     the user runs it. An installer is the highest-stakes prompt in the
//     program, and the August audit established that a prompt describing less
//     than what happens is worse than none at all.
package dialog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/arsenal"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// CloseArsenalMsg closes the page.
type CloseArsenalMsg struct{}

// ArsenalInstallMsg is emitted only when the user explicitly asks for the
// selection to be discussed. It is NOT how the command is shown — see viewPlan.
type ArsenalInstallMsg struct {
	Command string
	Summary string
}

// arsenalView is which of the three granularities is on screen.
type arsenalView int

const (
	viewSeries  arsenalView = iota // the series list — bulk
	viewEntries                    // one series, item by item
	viewDetail                     // one entry, everything known about it
	viewPlan                       // the install plan: the exact command, in full
)

type ArsenalCmp struct {
	width, height int
	view          arsenalView
	// planFrom is the view the install plan was opened from, so esc goes back
	// THERE.
	//
	// GORILLA FIX (2026-10-05), from the /arsenal audit (A5): leaving the plan
	// did `m.view--`, which is the detail page whatever came before. Pressing i
	// on the series list and then esc showed the full detail of an entry the
	// user had never opened, under a hint that said "esc back".
	planFrom arsenalView
	// man is the manifest AS IT APPLIES TO THIS OPERATING SYSTEM — what is
	// listed and counted. full is everything, kept so a tagfile written on
	// another system can be read and explained rather than called unknown.
	man  arsenal.Manifest
	full arsenal.Manifest
	pm   arsenal.PackageManager

	// status is detection, done once on open: measured, never claimed.
	status map[string]arsenal.Status

	seriesIdx int
	entryIdx  int
	scrollTop int

	// selected is the tagfile in memory — entry ids the user has ticked.
	selected map[string]bool

	// linkSpeed is KB/s used to turn megabytes into minutes. linkMeasured says
	// where it came from: true when it is this machine's own observed
	// throughput, false when it is the assumed figure.
	linkSpeed    float64
	linkMeasured bool

	// tagIdx is which selection file L loads next. See loadTagfile.
	tagIdx int

	// pricing holds measured costs, filled lazily because apt-get takes a
	// second or two and the screen must open instantly.
	pricing map[string]arsenal.Cost

	notice string
}

// assumedLinkKBps is used only when no transfer has been timed on this machine
// yet. 8 KB/s is the audience this project is built for (§8). It is an
// ASSUMPTION, and every line that uses it says so in that word.
const assumedLinkKBps = 8

func NewArsenalCmp() ArsenalCmp {
	full, _ := arsenal.Load()
	// GORILLA FIX (2026-10-05), from the /arsenal audit (A10): list and count
	// only what can exist on this operating system. See arsenal.ForPlatform.
	m := arsenal.ForPlatform(full)
	pm := arsenal.DetectPackageManager()
	st := map[string]arsenal.Status{}
	for _, s := range m.Series {
		for _, e := range s.Entries {
			st[e.ID] = arsenal.DetectEntry(e)
		}
	}
	c := ArsenalCmp{
		man:       m,
		full:      full,
		pm:        pm,
		status:    st,
		selected:  map[string]bool{},
		pricing:   map[string]arsenal.Cost{},
		linkSpeed: assumedLinkKBps,
	}
	// GORILLA FIX (2026-10-05), from the /arsenal audit (A9): every time figure
	// used a typed 8 KB/s while the program already keeps a measurement of
	// this machine's link (config.EstimatedKBps, timed from transfers that
	// were happening anyway — zero extra bytes). The measurement is used when
	// there is one. It is in KiB/s; DownloadTime counts in units of 1000.
	if kibps, ok := config.EstimatedKBps(); ok {
		c.linkSpeed, c.linkMeasured = kibps*1.024, true
	}
	return c
}

// speedLabel is how a time figure names the speed it was worked out at. The
// measured figure is a floor (see config/linkspeed.go), so the real wait is at
// most what is shown; the assumed figure is labelled as assumed.
func (m ArsenalCmp) speedLabel() string {
	if m.linkMeasured {
		return fmt.Sprintf("at %.0f KB/s, measured on this link", m.linkSpeed)
	}
	return fmt.Sprintf("at an assumed %.0f KB/s", m.linkSpeed)
}

func (m *ArsenalCmp) SetSize(w, h int) { m.width, m.height = w, h }

func (m ArsenalCmp) Init() tea.Cmd { return nil }

// currentSeries and currentEntry are the cursor, guarded against an empty
// manifest so a bad edit cannot panic the TUI.
func (m ArsenalCmp) currentSeries() (arsenal.Series, bool) {
	if m.seriesIdx < 0 || m.seriesIdx >= len(m.man.Series) {
		return arsenal.Series{}, false
	}
	return m.man.Series[m.seriesIdx], true
}

func (m ArsenalCmp) currentEntry() (arsenal.Entry, bool) {
	s, ok := m.currentSeries()
	if !ok || m.entryIdx < 0 || m.entryIdx >= len(s.Entries) {
		return arsenal.Entry{}, false
	}
	return s.Entries[m.entryIdx], true
}

// installable is what a selection actually resolves to: the entries that are
// missing AND obtainable here. An entry already present adds nothing; an entry
// with no package for this system is unavailable, which is NOT the same as free.
func (m ArsenalCmp) installable(ids []string) (pkgs []string, chosen []arsenal.Entry, unavailable []arsenal.Entry) {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	for _, s := range m.man.Series {
		for _, e := range s.Entries {
			if !want[e.ID] || m.status[e.ID].Present {
				continue
			}
			if !arsenal.Available(e, m.pm) {
				unavailable = append(unavailable, e)
				continue
			}
			chosen = append(chosen, e)
			pkgs = append(pkgs, arsenal.PackagesFor(e, m.pm)...)
		}
	}
	return dedupe(pkgs), chosen, unavailable
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func (m ArsenalCmp) selectedIDs() []string {
	ids := make([]string, 0, len(m.selected))
	for id, on := range m.selected {
		if on {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// everyID is the "everything" answer, which is always on the menu.
func (m ArsenalCmp) everyID() []string {
	var ids []string
	for _, s := range m.man.Series {
		for _, e := range s.Entries {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

func (m ArsenalCmp) seriesIDs(s arsenal.Series) []string {
	ids := make([]string, 0, len(s.Entries))
	for _, e := range s.Entries {
		ids = append(ids, e.ID)
	}
	return ids
}

// toggle flips a group, and REPORTS what it did.
//
// GORILLA FIX (2026-08-19), found by the owner within minutes of the release:
// "is it me or space does not select anything?"
//
// It was not him, and it was not the key. The page opens with the cursor on
// the first series — "The minimum" — which on his machine is 8/8 ALREADY
// INSTALLED. So space correctly selected nothing, and then said nothing, and
// the next key (p, price it) correctly priced an empty selection and also said
// nothing. Two keys in a row doing exactly the right thing and looking
// completely broken.
//
// This is directive §3 arriving in a UI: silence and success must never look
// alike. The behaviour was right; the absence of feedback was the bug, and it
// was invisible to every test because the tests called toggle() on entries
// chosen to be missing.
func (m *ArsenalCmp) toggle(ids []string) {
	selectable := 0
	present := 0
	for _, id := range ids {
		if m.status[id].Present {
			present++
			continue
		}
		selectable++
	}
	if selectable == 0 {
		switch {
		case present == 0:
			m.notice = "Nothing here to select."
		case present == 1:
			m.notice = "That one is already installed — nothing to select."
		default:
			m.notice = fmt.Sprintf("All %d of these are already installed — nothing to select.", present)
		}
		return
	}

	// If everything selectable in the group is already on, the key turns it
	// off. That is what makes one key both "take this series" and "drop it".
	allOn := true
	for _, id := range ids {
		if m.status[id].Present {
			continue
		}
		if !m.selected[id] {
			allOn = false
			break
		}
	}
	changed := 0
	for _, id := range ids {
		if m.status[id].Present {
			continue
		}
		m.selected[id] = !allOn
		changed++
	}
	verb := "selected"
	if allOn {
		verb = "un-selected"
	}
	m.notice = fmt.Sprintf("%s %d", verb, changed)
	if present > 0 {
		m.notice += fmt.Sprintf(" (%d already installed, skipped)", present)
	}
	// GORILLA FIX (2026-10-05), audit A3: the next step is only "measure" where
	// this package manager can measure.
	switch {
	case allOn:
	case arsenal.CanMeasure(m.pm):
		m.notice += " — press p to measure the cost."
	default:
		m.notice += " — press i for the install plan."
	}
}

// selectionSummary splits the current selection into what the package manager
// can fetch and what it cannot.
func (m ArsenalCmp) selectionSummary() (pkgs []string, fetchable, unfetchable int) {
	pkgs, chosen, unavailable := m.installable(m.selectedIDs())
	return pkgs, len(chosen), len(unavailable)
}

// nothingFetchable is the sentence for a selection that resolves to no packages
// at all.
//
// GORILLA FIX (2026-10-05), from the /arsenal audit (A2): on a Windows machine
// without Scoop, `a` then `p` answered "measured: nothing to download - all of
// it is already here" for twenty-four tools, none of which were there. Zero
// packages meant "cannot fetch any of this", and it was reported as "has all
// of this". Opposite facts, again.
func (m ArsenalCmp) nothingFetchable(n int) string {
	if m.pm == arsenal.Unknown {
		return fmt.Sprintf("No supported package manager was found, so none of the %d selected can be fetched from here — nothing to measure, nothing to install.", n)
	}
	return fmt.Sprintf("None of the %d selected can be fetched by %s — nothing to measure, nothing to install.", n, pmName(m.pm))
}

func (m ArsenalCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		// Cleared here so a message never outlives the key that caused it —
		// but every branch below that does nothing visible MUST set it again,
		// or the screen goes back to looking broken.
		m.notice = ""
		switch msg.String() {
		case "esc", "q":
			if m.view == viewSeries {
				return m, util.CmdHandler(CloseArsenalMsg{})
			}
			m.back()
			return m, nil
		case "up", "k":
			m.move(-1)
			return m, nil
		case "down", "j":
			m.move(1)
			return m, nil
		case "enter", "right", "l":
			switch m.view {
			case viewSeries:
				m.view, m.entryIdx, m.scrollTop = viewEntries, 0, 0
			case viewEntries:
				m.view, m.scrollTop = viewDetail, 0
			}
			return m, nil
		case "left", "h":
			if m.view > viewSeries {
				m.back()
			}
			return m, nil
		case " ":
			// Space selects at whatever granularity you are looking at. That
			// IS the Slackware model: bulk, series, individual, one key.
			switch m.view {
			case viewSeries:
				if s, ok := m.currentSeries(); ok {
					m.toggle(m.seriesIDs(s))
				}
			case viewEntries, viewDetail:
				if e, ok := m.currentEntry(); ok {
					m.toggle([]string{e.ID})
				}
			}
			return m, nil
		case "a":
			// "Everything" is a first-class answer and must never be hidden.
			m.toggle(m.everyID())
			return m, nil
		case "n":
			// GORILLA FIX (2026-10-05), from the /arsenal audit (A12): this
			// cleared the selection and said nothing, against the rule four
			// lines above it. With nothing selected it looked like a dead key.
			had := len(m.selectedIDs())
			m.selected = map[string]bool{}
			if had == 0 {
				m.notice = "Nothing was selected."
			} else {
				m.notice = fmt.Sprintf("un-selected %d — the selection is now empty.", had)
			}
			return m, nil
		// GORILLA FIX (2026-08-19): these were `return m, m.price()`.
		//
		// The Go spec orders function CALLS left to right within a return
		// statement, but says nothing about when a plain operand like `m` is
		// read — so `m` could be copied BEFORE the method mutates it, and
		// every notice and view change set inside would be silently thrown
		// away. It happened to work with this compiler, which is the worst
		// kind of working: correct by accident, on one toolchain, with no
		// test that could tell.
		case "p":
			cmd := m.price()
			return m, cmd
		case "i":
			cmd := m.showPlan()
			return m, cmd
		case "s":
			cmd := m.saveTagfile()
			return m, cmd
		case "L":
			cmd := m.loadTagfile()
			return m, cmd
		case "?":
			// Costs a model turn, so it is never the default path.
			cmd := m.emitInstall()
			return m, cmd
		}
	case arsenalPricedMsg:
		m.pricing[msg.key] = msg.cost
		// GORILLA FIX (2026-08-19), reported from a real run: the "measuring
		// with apt..." notice was set when the measurement STARTED and nothing
		// ever replaced it. The header updated with the answer — 97.9 MB,
		// 331.0 MB, about 3.4 hours — while the line underneath still said it
		// was working. So the screen showed the result and denied having it,
		// and the reasonable conclusion for anyone reading it is that the
		// program has hung.
		//
		// A progress message MUST be replaced by its outcome. The general
		// version of this is the same rule as everywhere else in the program:
		// a state that has finished must not still look like a state that is
		// running.
		switch {
		case !msg.cost.Measured:
			m.notice = "could not measure: " + msg.cost.Note
		case msg.cost.DownloadBytes == 0:
			// Only reachable when the package manager's own answer was read
			// and said zero (audit A2/A4): no packages, or an answer that
			// could not be parsed, both arrive as !Measured above.
			m.notice = "measured: " + pmName(m.pm) + " reports nothing left to download for this."
		default:
			// Short enough to survive an 80-column terminal, because the
			// figure is the whole point of the line and a truncated number is
			// no number at all. The full breakdown is on the install plan.
			m.notice = "measured: " + arsenal.HumanBytes(msg.cost.DownloadBytes) + " down / " +
				diskText(msg.cost) + " disk"
			if t := arsenal.DownloadTime(msg.cost.DownloadBytes, m.linkSpeed); t != "" {
				m.notice += " / ~" + t
			}
			m.notice += " -- press i"
		}
		return m, nil
	}
	return m, nil
}

// diskText is the disk figure, or the plain statement that there is none. A
// figure the package manager did not report must never be printed as "0 B".
func diskText(c arsenal.Cost) string {
	if !c.DiskMeasured {
		return "unreported"
	}
	return arsenal.HumanBytes(c.DiskBytes)
}

// back leaves the current view for the one it was reached from.
func (m *ArsenalCmp) back() {
	if m.view == viewPlan {
		m.view = m.planFrom
	} else {
		m.view--
	}
	m.scrollTop = 0
}

func (m *ArsenalCmp) move(d int) {
	switch m.view {
	case viewSeries:
		m.seriesIdx = clampInt(m.seriesIdx+d, 0, len(m.man.Series)-1)
	case viewEntries:
		if s, ok := m.currentSeries(); ok {
			m.entryIdx = clampInt(m.entryIdx+d, 0, len(s.Entries)-1)
		}
	case viewDetail, viewPlan:
		// GORILLA FIX (2026-10-05): the plan was missing here. A plan longer
		// than the screen showed "... more line(s) — down to continue" and
		// down did nothing, so the install command at the bottom — the one
		// thing the page is for — could not be reached on a short terminal.
		m.scrollTop = maxInt(0, m.scrollTop+d)
	}
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type arsenalPricedMsg struct {
	key  string
	cost arsenal.Cost
}

// price measures the current selection with the real package manager. It runs
// as a command rather than inline because apt-get takes a second or two and a
// screen that freezes on open is a screen people stop opening.
func (m *ArsenalCmp) price() tea.Cmd {
	ids := m.selectedIDs()
	if len(ids) == 0 {
		// Same failure as toggle's: pricing nothing is correct and looks
		// broken. Say which key selects.
		m.notice = "Nothing selected yet — space takes what the cursor is on, a takes everything."
		return nil
	}
	// GORILLA FIX (2026-10-05), audit A3: where the package manager cannot
	// price anything, say so at once instead of pretending to start.
	if !arsenal.CanMeasure(m.pm) {
		m.notice = "Cannot measure here: " + arsenal.CannotMeasureNote(m.pm) + "."
		return nil
	}
	pkgs, _, unfetchable := m.selectionSummary()
	if len(pkgs) == 0 {
		// Audit A2. See nothingFetchable.
		m.notice = m.nothingFetchable(unfetchable)
		return nil
	}
	m.notice = "measuring with " + pmName(m.pm) + "..."
	key := strings.Join(ids, ",")
	pm := m.pm
	return func() tea.Msg {
		return arsenalPricedMsg{key: key, cost: arsenal.MeasureCost(pkgs, pm)}
	}
}

// showPlan switches to the install plan.
//
// GORILLA FIX (2026-08-19): the first version of this SENT the command to the
// model and asked it to explain the selection. Caught in the live run, where
// it is obvious what that costs: a full model turn, billed, on every press,
// to restate information the manifest already carries in plain words.
//
// For this audience that is exactly the wrong trade — tokens are a recurring
// bill they cannot afford (§8), and a screen that quietly spends money when
// you press a key is a screen you learn not to press. The plan is rendered
// locally, from data already in the binary, for nothing.
func (m *ArsenalCmp) showPlan() tea.Cmd {
	if len(m.selectedIDs()) == 0 {
		m.notice = "Nothing selected. Space picks the thing under the cursor; a picks everything."
		return nil
	}
	if m.view != viewPlan {
		m.planFrom = m.view
	}
	m.view, m.scrollTop = viewPlan, 0
	return nil
}

// planLines is the install plan: what was chosen, what it costs, and the exact
// command — never run from here.
func (m ArsenalCmp) planLines() []arsLine {
	ids := m.selectedIDs()
	pkgs, chosen, unavailable := m.installable(ids)
	already := len(ids) - len(chosen) - len(unavailable)

	out := []arsLine{
		{"h1", "Install plan"},
		{"mute", "esc back | up/down scroll | s save this selection as a shareable file"},
		{"mute", askHint},
		{"", ""},
	}
	if len(chosen) > 0 {
		out = append(out, arsLine{"h2", fmt.Sprintf("%d capabilit%s to add", len(chosen), plural(len(chosen)))})
		for _, e := range chosen {
			out = append(out, arsLine{"", "  * " + e.Title})
		}
	}
	if already > 0 {
		out = append(out, arsLine{"mute", fmt.Sprintf("  (%d already on this machine — not in the command)", already)})
	}
	for _, e := range unavailable {
		out = append(out, arsLine{"warn", "  ! " + e.Title + " — " + arsenal.UnavailableNote(e, m.pm)})
	}

	if len(pkgs) == 0 {
		// Audit A2: "Nothing to install." in green read as "all done". When
		// the reason is that nothing selected can be fetched, say that.
		out = append(out, arsLine{"", ""})
		if len(unavailable) > 0 {
			for _, l := range wrapPlain(m.nothingFetchable(len(unavailable)), m.wrapWidth(0)) {
				out = append(out, arsLine{"warn", l})
			}
		} else {
			out = append(out, arsLine{"have", "Nothing to install."})
		}
		return out
	}

	out = append(out, arsLine{"", ""}, arsLine{"h2", fmt.Sprintf("%d package(s)", len(pkgs))})
	for _, l := range wrapPlain(strings.Join(pkgs, " "), m.wrapWidth(2)) {
		out = append(out, arsLine{"mute", "  " + l})
	}

	c, priced := m.pricing[strings.Join(ids, ",")]
	switch {
	case priced && c.Measured:
		line := fmt.Sprintf("%s to download, disk %s", arsenal.HumanBytes(c.DownloadBytes), diskText(c))
		if t := arsenal.DownloadTime(c.DownloadBytes, m.linkSpeed); t != "" {
			line += fmt.Sprintf(" — about %s %s", t, m.speedLabel())
		}
		out = append(out, arsLine{"", ""}, arsLine{"sel", line})
		out = append(out, arsLine{"mute", "measured by your own package manager against what is already here, not a table"})
		if c.Note != "" {
			out = append(out, arsLine{"mute", c.Note})
		}
	case priced:
		out = append(out, arsLine{"", ""}, arsLine{"warn", "cost not measured: " + c.Note})
	case arsenal.CanMeasure(m.pm):
		out = append(out, arsLine{"", ""}, arsLine{"mute", "cost not measured yet — press p to ask " + pmName(m.pm) + " what this costs"})
	default:
		// Audit A3: no offer of a measurement that cannot be made.
		out = append(out, arsLine{"", ""}, arsLine{"mute", "cost not measured: " + arsenal.CannotMeasureNote(m.pm)})
	}

	out = append(out,
		arsLine{"", ""},
		arsLine{"h2", "Run this yourself"},
		arsLine{"", ""})
	// One command per line, in order (audit A11): each is wrapped on its own
	// so two commands can never be joined into one unrunnable line.
	for _, command := range arsenal.InstallCommands(pkgs, m.pm) {
		for i, l := range wrapPlain(command, m.wrapWidth(4)) {
			if i > 0 {
				l = "  " + l
			}
			out = append(out, arsLine{"on", "  " + l})
		}
	}
	// GORILLA OVERRIDE (2026-09-01): the closing note names the package manager
	// in play. It said "apt can be interrupted and resumed" unconditionally,
	// which is a statement about a program that is not installed on Windows and
	// was never true for pacman either.
	tail := "read it, and run it when you want to. apt can be interrupted and resumed, so a long download on a bad line is not lost work."
	switch m.pm {
	case arsenal.Pacman:
		tail = "read it, and run it when you want to. pacman can be interrupted and resumed, so a long download on a bad line is not lost work."
	case arsenal.Scoop:
		tail = "read it, and run it when you want to. scoop installs into your own user folder, so it needs no administrator rights."
	}
	out = append(out,
		arsLine{"", ""},
		arsLine{"mute", "This program will not run it and will never ask for your password. Copy it,"})
	for _, l := range wrapPlain(tail, m.wrapWidth(0)) {
		out = append(out, arsLine{"mute", l})
	}
	return out
}

// emitInstall asks the MODEL about a selection. Costs a turn, so it is a
// separate, explicitly-labelled key rather than the default path.
//
// GORILLA FIX (2026-10-05), from the /arsenal audit (A7): the key was printed
// nowhere on the screen, so "explicitly-labelled" above was false — the one key
// on this page that spends money was the one key with no label. And with
// nothing selected, or nothing fetchable, it returned nil and said nothing.
// It is now in the hints with its cost (askHint), and both empty cases answer.
func (m *ArsenalCmp) emitInstall() tea.Cmd {
	ids := m.selectedIDs()
	if len(ids) == 0 {
		m.notice = "Nothing selected, so there is nothing to ask the model about. Space picks; a picks everything."
		return nil
	}
	pkgs, chosen, unavailable := m.installable(ids)
	if len(pkgs) == 0 {
		m.notice = m.nothingFetchable(len(unavailable)) + " Nothing was sent to the model."
		return nil
	}
	// tui.go indents the command by four spaces when it writes the message;
	// the continuation lines are given the same indent here so a two-command
	// answer stays one block.
	cmd := strings.Join(arsenal.InstallCommands(pkgs, m.pm), "\n    ")

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d capabilit%s, %d package(s):\n", len(chosen), plural(len(chosen)), len(pkgs))
	for _, e := range chosen {
		fmt.Fprintf(&sb, "  * %s — %s\n", e.ID, e.Title)
	}
	for _, e := range unavailable {
		fmt.Fprintf(&sb, "  ! %s — %s\n", e.ID, arsenal.UnavailableNote(e, m.pm))
	}
	return util.CmdHandler(ArsenalInstallMsg{Command: cmd, Summary: sb.String()})
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// ── rendering ────────────────────────────────────────────────────────────

type arsLine struct {
	kind string // "h1", "h2", "sel", "on", "have", "warn", "mute", ""
	text string
}

// stateBadge is the single most important column on the screen: it separates
// what this machine CAN do right now from what it COULD do. The bug that
// created this whole feature was a capability nobody knew was installed, so
// "HAVE" is measured on open and never assumed.
func (m ArsenalCmp) stateBadge(e arsenal.Entry) (string, string) {
	st := m.status[e.ID]
	switch {
	case st.Present:
		return "HAVE", "have"
	case st.Partial():
		return "PART", "warn"
	case !arsenal.Available(e, m.pm):
		return "N/A ", "mute"
	case m.selected[e.ID]:
		return "PICK", "on"
	}
	return "  - ", "mute"
}

func (m ArsenalCmp) header() []arsLine {
	haveN, missN, pickN := 0, 0, 0
	for _, s := range m.man.Series {
		for _, e := range s.Entries {
			switch {
			case m.status[e.ID].Present:
				haveN++
			default:
				missN++
			}
			if m.selected[e.ID] {
				pickN++
			}
		}
	}
	head := []arsLine{
		{"h1", "ARSENAL — what this agent can do, and what it could do"},
		{"mute", fmt.Sprintf("on this machine: %d capabilities present, %d not | package manager: %s",
			haveN, missN, pmName(m.pm))},
	}
	if pickN > 0 {
		line := fmt.Sprintf("selected: %d", pickN)
		if c, ok := m.pricing[strings.Join(m.selectedIDs(), ",")]; ok {
			switch {
			case !c.Measured:
				line += " | could not price: " + c.Note
			case c.DownloadBytes == 0:
				line += " | " + pmName(m.pm) + " reports nothing left to download"
			default:
				line += fmt.Sprintf(" | %s to download, disk %s",
					arsenal.HumanBytes(c.DownloadBytes), diskText(c))
				if t := arsenal.DownloadTime(c.DownloadBytes, m.linkSpeed); t != "" {
					line += fmt.Sprintf(" | about %s %s", t, m.speedLabel())
				}
			}
		} else if pkgs, _, unfetchable := m.selectionSummary(); len(pkgs) == 0 {
			// Audit A2: a selection the package manager cannot fetch any of.
			line += fmt.Sprintf(" | none of it can be fetched by %s (%d)", pmWord(m.pm), unfetchable)
		} else if arsenal.CanMeasure(m.pm) {
			line += " | press p to measure what it costs"
		} else {
			// Audit A3.
			line += " | cost not measured: " + arsenal.CannotMeasureNote(m.pm)
		}
		head = append(head, arsLine{"sel", line})
	}
	return head
}

func pmName(pm arsenal.PackageManager) string {
	if pm == arsenal.Unknown {
		return "none found"
	}
	return string(pm)
}

// pmWord is the package manager inside a sentence, where "none found" would
// not read.
func pmWord(pm arsenal.PackageManager) string {
	if pm == arsenal.Unknown {
		return "any package manager here"
	}
	return string(pm)
}

// askHint labels the one key on this page that costs money. See emitInstall.
const askHint = "? send the selection to the model to discuss — costs one model turn"

// actionHint is the second line of keys. The p key is listed only where it can
// do what it says (audit A3).
func (m ArsenalCmp) actionHint(last string) string {
	keys := "i the install plan | s save selection | L load one | " + last
	if arsenal.CanMeasure(m.pm) {
		return "p measure the real cost | " + keys
	}
	return keys
}

func (m ArsenalCmp) seriesLines() []arsLine {
	out := m.header()
	out = append(out,
		arsLine{"", ""},
		arsLine{"mute", "up/down move | enter open | space take this series | a everything | n none"},
		arsLine{"mute", m.actionHint("esc close")},
		arsLine{"mute", askHint},
		arsLine{"", ""})

	for i, s := range m.man.Series {
		have, tot, pick := 0, len(s.Entries), 0
		for _, e := range s.Entries {
			if m.status[e.ID].Present {
				have++
			}
			if m.selected[e.ID] {
				pick++
			}
		}
		cursor := "  "
		kind := ""
		if i == m.seriesIdx {
			cursor, kind = "> ", "h2"
		}
		tag := fmt.Sprintf("%d/%d here", have, tot)
		if pick > 0 {
			tag += fmt.Sprintf(", %d picked", pick)
		}
		// The count is the point of the row, so the TITLE gives way when the
		// two do not fit: at 80 columns the longest title used to push
		// "0/4 here" off the right-hand edge, where it was cut.
		titleW := maxInt(10, m.textWidth()-len(cursor)-1-len(tag))
		out = append(out, arsLine{kind, fmt.Sprintf("%s%-*s %s", cursor, min(46, titleW),
			truncateToWidth(s.Title, titleW), tag)})
		if i == m.seriesIdx {
			for _, l := range wrapPlain(s.Why, m.wrapWidth(4)) {
				out = append(out, arsLine{"mute", "    " + l})
			}
		}
	}
	out = append(out,
		arsLine{"", ""},
		arsLine{"mute", "Nothing here is installed by this program. It shows you the exact command; you run it."})
	// Counted from the manifest, not asserted: the day an entry needing an
	// account is added, this line changes by itself.
	if n := m.needingAccount(); n == 0 {
		out = append(out, arsLine{"mute", "Everything listed is free and needs no account."})
	} else {
		out = append(out, arsLine{"warn", fmt.Sprintf("%d of these need an account or a card — each one says so on its own page.", n)})
	}
	// GORILLA FIX (2026-10-05), audit A9: say where the speed came from.
	if m.linkMeasured {
		out = append(out, arsLine{"mute", fmt.Sprintf("Times are worked out at %.0f KB/s, the best speed measured on this link so far.", m.linkSpeed)})
	} else {
		out = append(out, arsLine{"mute", fmt.Sprintf("Times ASSUME %.0f KB/s: no transfer has been timed on this machine yet, so this is a guess.", m.linkSpeed)})
	}
	// GORILLA FIX (2026-10-05), audit A14: "here" means on the PATH of the
	// shell this program was started from. The same machine gives different
	// counts from Git Bash and from PowerShell, and the screen did not say
	// which question it had answered.
	out = append(out, arsLine{"mute", "\"Here\" means found on the PATH this program was started with; another shell may differ."})
	return out
}

// needingAccount counts listed entries that ask for an account or a card.
func (m ArsenalCmp) needingAccount() int {
	n := 0
	for _, s := range m.man.Series {
		for _, e := range s.Entries {
			if e.Needs.Account || e.Needs.Card {
				n++
			}
		}
	}
	return n
}

func (m ArsenalCmp) entryLines() []arsLine {
	s, ok := m.currentSeries()
	if !ok {
		return []arsLine{{"warn", "empty manifest"}}
	}
	out := m.header()
	out = append(out,
		arsLine{"", ""},
		arsLine{"h2", s.Title},
		arsLine{"mute", "up/down move | enter full detail | space take this one"},
		arsLine{"mute", m.actionHint("esc back")},
		arsLine{"", ""})

	for i, e := range s.Entries {
		badge, kind := m.stateBadge(e)
		cursor := "  "
		if i == m.entryIdx {
			cursor = "> "
			if kind == "mute" {
				kind = ""
			}
		}
		out = append(out, arsLine{kind, fmt.Sprintf("%s[%s] %s", cursor, badge, e.Title)})
		if i == m.entryIdx {
			// The one-line description AT THE POINT OF CHOOSING is the part of
			// the Slackware installer that actually taught people what exists.
			for _, l := range wrapPlain(e.Teaches, m.wrapWidth(7)) {
				out = append(out, arsLine{"mute", "       " + l})
			}
			if n := arsenal.UnavailableNote(e, m.pm); n != "" {
				out = append(out, arsLine{"warn", "       " + n})
			}
		}
	}
	return out
}

func (m ArsenalCmp) detailLines() []arsLine {
	e, ok := m.currentEntry()
	if !ok {
		return []arsLine{{"warn", "nothing selected"}}
	}
	st := m.status[e.ID]
	out := []arsLine{
		{"h1", e.Title},
		{"mute", "esc back | space " + pickVerb(m.selected[e.ID])},
		{"", ""},
	}

	// Wrapped, not left to truncation: these lines can now carry a full path,
	// and a path cut off with "..." cannot be typed.
	state := func(kind, text string) {
		for _, l := range wrapPlain(text, m.wrapWidth(0)) {
			out = append(out, arsLine{kind, l})
		}
	}
	switch {
	case st.Present && st.OffPath:
		// Audit A6: present on the disk, absent from PATH. Both halves are
		// stated, because "HAVE" alone would promise a command that fails.
		state("have", "ALREADY ON THIS MACHINE — found: "+strings.Join(st.Found, ", "))
		state("warn", "It is NOT on PATH, so its short name will not run. Call it by the full path above.")
	case st.Present:
		state("have", "ALREADY ON THIS MACHINE — found: "+strings.Join(st.Found, ", "))
	case st.Partial():
		state("warn", "PARTLY HERE — found "+strings.Join(st.Found, ", ")+
			"; missing "+strings.Join(st.Missing, ", "))
	default:
		state("mute", "not found on PATH — looked for: "+strings.Join(e.Detect.Binaries, ", "))
	}
	// Audit A1: a file of the right name that is the wrong program. Said out
	// loud, so nobody who can see convert.exe on their own disk has to wonder
	// why it was not counted.
	for _, ig := range st.Ignored {
		state("mute", fmt.Sprintf("not counted: %s at %s is %s.", ig.Binary, ig.Path, ig.Is))
	}
	out = append(out, arsLine{"", ""}, arsLine{"h2", "What it is"})
	for _, l := range wrapPlain(e.Teaches, m.wrapWidth(0)) {
		out = append(out, arsLine{"", l})
	}

	out = append(out, arsLine{"", ""}, arsLine{"h2", "What the agent gains"})
	for _, u := range e.Unlocks {
		out = append(out, arsLine{"", "* " + u})
	}

	out = append(out, arsLine{"", ""}, arsLine{"h2", "What will disappoint you"})
	for _, l := range wrapPlain(e.Caveats, m.wrapWidth(0)) {
		out = append(out, arsLine{"warn", l})
	}

	out = append(out, arsLine{"", ""}, arsLine{"h2", "How to get it"})
	if pkgs := arsenal.PackagesFor(e, m.pm); len(pkgs) > 0 {
		// One command per line (audit A11).
		for _, command := range arsenal.InstallCommands(pkgs, m.pm) {
			for _, l := range wrapPlain(command, m.wrapWidth(0)) {
				out = append(out, arsLine{"", l})
			}
		}
	} else {
		out = append(out, arsLine{"warn", arsenal.UnavailableNote(e, m.pm)})
		// Sorted: a Go map has no order, so these lines changed places from
		// one opening of the page to the next.
		names := make([]string, 0, len(e.Packages))
		for name := range e.Packages {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if pkgs := e.Packages[name]; len(pkgs) > 0 {
				out = append(out, arsLine{"mute", fmt.Sprintf("on %s: %s", name, strings.Join(pkgs, " "))})
			}
		}
	}
	needs := "no account, no card, works offline"
	if e.Needs.NetworkAtRuntime {
		needs = "no account, no card — but it needs the internet each time you use it"
	}
	if e.Needs.Account || e.Needs.Card {
		needs = "NEEDS AN ACCOUNT OR A CARD"
	}
	out = append(out, arsLine{"", ""}, arsLine{"mute", needs})
	return out
}

func pickVerb(on bool) string {
	if on {
		return "un-pick it"
	}
	return "pick it"
}

// truncateToWidth cuts a line to w DISPLAY COLUMNS, not runes.
//
// Rune counting is wrong here: an em-dash is one rune and one column, but CJK
// and emoji are one rune and TWO, so a rune-counted line can still overflow.
// lipgloss.Width measures what the terminal will actually draw.
func truncateToWidth(s string, w int) string {
	if w < 2 || lipgloss.Width(s) <= w {
		return s
	}
	// Reserve the marker's REAL width. It was "…" at one column; it is now
	// "..." at three, and the reservation did not follow it — so every
	// truncated line came out two columns too long, wrapped, and grew the
	// frame. Caught immediately by TestThePageFillsTheScreenExactly, which is
	// exactly why that test asserts an exact height rather than "not taller".
	marker := styles.Ellipsis
	mw := lipgloss.Width(marker)
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+mw > w {
		r = r[:len(r)-1]
	}
	return string(r) + marker
}

// wrapPlain wraps unstyled text at w columns on word boundaries. Done here
// rather than by lipgloss .Width() because lipgloss WRAPS silently, so an
// over-long line becomes extra HEIGHT with no warning and the frame drifts.
func wrapPlain(s string, w int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var out []string
	cur := words[0]
	for _, word := range words[1:] {
		if len(cur)+1+len(word) > w {
			out = append(out, cur)
			cur = word
			continue
		}
		cur += " " + word
	}
	return append(out, cur)
}

// cursorTop is the first line to draw so that the cursor's row, and the
// description under it, are on screen.
//
// GORILLA FIX (2026-10-05): the two list views never scrolled. On a terminal
// too short for the list the page printed "... more line(s) — down to
// continue", and down moved the cursor off the bottom of the window where it
// could no longer be seen. The marker promised scrolling that did not exist.
func cursorTop(lines []arsLine, visible int) int {
	cur := -1
	for i, l := range lines {
		if strings.HasPrefix(l.text, "> ") {
			cur = i
			break
		}
	}
	if cur < 0 || len(lines) <= visible {
		return 0
	}
	// The description lines are the indented ones directly under the cursor.
	last := cur
	for last+1 < len(lines) && strings.HasPrefix(lines[last+1].text, "    ") {
		last++
	}
	// One row is kept back for the overflow marker.
	room := max(1, visible-1)
	top := 0
	if last >= room {
		top = last - room + 1
	}
	if top > cur {
		top = cur
	}
	return top
}

// boxWidth and textWidth are the frame's styled width and the width of the text
// inside it. View explains the arithmetic.
func (m ArsenalCmp) boxWidth() int  { return max(24, dialogWidth(m.width, 120, 6)) }
func (m ArsenalCmp) textWidth() int { return max(20, m.boxWidth()-4) }

// wrapWidth is how wide wrapped text may be when the caller will put `indent`
// columns in front of it.
//
// GORILLA FIX (2026-10-05): every wrap was sized from the TERMINAL width
// (m.width minus a constant) while the box is capped at 120 columns. On any
// terminal wider than about 134 the text was wrapped wider than the box and
// then truncated to fit it — including the install command, the one line on
// this page that must be exact. At 200 columns that is a 188-column wrap
// inside a 116-column box. Wraps are now sized from the box.
func (m ArsenalCmp) wrapWidth(indent int) int { return maxInt(30, m.textWidth()-indent) }

func (m ArsenalCmp) lines() []arsLine {
	switch m.view {
	case viewEntries:
		return m.fit(m.entryLines())
	case viewDetail:
		return m.fit(m.detailLines())
	case viewPlan:
		return m.fit(m.planLines())
	}
	return m.fit(m.seriesLines())
}

// fit makes every row fit the box by FOLDING it, so that View's truncation is
// a last resort and not the normal case.
//
// GORILLA FIX (2026-10-05): at 80 columns the box holds 70, and the key hints,
// the footer and the header were each longer than that. They were cut with
// "..." — which removed "esc close" from the hints and would have removed the
// new line that labels the ? key. A row of " | " items is re-packed item by
// item; a sentence is wrapped on words, keeping its indent. The cursor rows of
// the two lists are left alone: they are one item each and are sized where
// they are built.
func (m ArsenalCmp) fit(in []arsLine) []arsLine {
	w := m.textWidth()
	out := make([]arsLine, 0, len(in))
	for _, l := range in {
		if lipgloss.Width(l.text) <= w ||
			strings.HasPrefix(l.text, "> ") || strings.HasPrefix(l.text, "  [") {
			out = append(out, l)
			continue
		}
		if strings.Contains(l.text, " | ") {
			cur := ""
			for _, part := range strings.Split(l.text, " | ") {
				switch {
				case cur == "":
					cur = part
				case lipgloss.Width(cur)+3+lipgloss.Width(part) > w:
					out = append(out, arsLine{l.kind, cur})
					cur = part
				default:
					cur += " | " + part
				}
			}
			out = append(out, arsLine{l.kind, cur})
			continue
		}
		body := strings.TrimLeft(l.text, " ")
		indent := l.text[:len(l.text)-len(body)]
		for _, piece := range wrapPlain(body, maxInt(10, w-len(indent))) {
			out = append(out, arsLine{l.kind, indent + piece})
		}
	}
	return out
}

// View renders the page FULL SCREEN.
//
// GORILLA FIX (2026-08-19), owner's call: "you can always make the window
// either bigger or full screen. That solves the problem."
//
// It does, and it is the better fix. The previous version sized the box to its
// content and then had to reserve rows for the trailing notice and the "more
// lines" marker, because those are appended AFTER the content is windowed —
// so a full box plus a notice could render a frame taller than the terminal,
// which is the one case bubbletea genuinely cannot recover from.
//
// Reserving rows treats the symptom. Taking the whole screen removes the
// question: the budget is fixed, known before anything is rendered, and the
// content scrolls inside it. It also just reads better — this page is a map
// with long descriptions, and a centred box two thirds the width of the
// terminal was throwing away the space that makes it legible.
func (m ArsenalCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()

	// Chrome, counted exactly rather than approximately.
	//
	// The border adds 2 columns and 2 rows OUTSIDE the styled width; the
	// Padding(1,2) below adds 4 columns and 2 rows INSIDE it. So the box's
	// styled width is m.width-2, and the usable text width is 4 narrower
	// again. Getting this wrong is not a cosmetic error: every line then
	// exceeds its own container, lipgloss WRAPS rather than overflowing, and
	// the frame silently grows in HEIGHT — which is exactly the failure this
	// whole change was meant to remove. Measured at 60x20 before the fix: a
	// 27-row frame in a 20-row terminal.
	// GORILLA FIX (2026-08-19), the third thing the screenshots taught: use the
	// house width helper like every other page does.
	//
	// This page was the only one taking the FULL terminal width. Two
	// consequences, both visible and both mistaken for bugs: it covered the
	// sidebar, and its border landed on the very edge of the screen where it
	// reads as no border at all. /osint caps at 104 and looks like a window;
	// this looked like the program had taken over the terminal.
	//
	// 120 rather than 104 because the entries carry long plain-language
	// descriptions and the whole point of them is being readable — but capped,
	// so the frame is a window on any screen wider than that.
	boxW := m.boxWidth()
	w := m.textWidth()
	budget := max(5, m.height-4)

	lines := m.lines()
	// The notice and the overflow marker are part of the budget, not extra.
	reserve := 0
	if m.notice != "" {
		reserve++
	}
	visible := max(1, budget-reserve)
	top := m.scrollTop
	if m.view == viewSeries || m.view == viewEntries {
		top = cursorTop(lines, visible)
	}
	if top > len(lines)-visible {
		top = max(0, len(lines)-visible)
	}
	end := min(top+visible, len(lines))
	if end < len(lines) {
		// The marker takes one of the content rows rather than being added on
		// top of them.
		visible = max(1, visible-1)
		end = min(top+visible, len(lines))
	}

	var b []string
	for _, l := range lines[top:end] {
		st := base.Width(w).MaxWidth(w)
		switch l.kind {
		case "h1", "h2":
			st = st.Foreground(t.Primary()).Bold(true)
		case "have":
			st = st.Foreground(lipgloss.Color("2")).Bold(true)
		case "on", "sel":
			st = st.Foreground(t.Primary()).Bold(true)
		case "warn":
			st = st.Foreground(t.Warning())
		case "mute":
			st = st.Foreground(t.TextMuted())
		}
		// Truncate rather than let lipgloss wrap: an over-wide line costs a
		// physical row the renderer does not count, which strands debris in
		// the transcript on every frame.
		b = append(b, st.Render(truncateToWidth(l.text, w)))
	}
	if end < len(lines) {
		b = append(b, base.Width(w).Foreground(t.TextMuted()).
			Render(fmt.Sprintf("  ... %d more line(s) — down to continue", len(lines)-end)))
	}
	if m.notice != "" {
		// Truncated like every other line. It was not, and a long notice
		// wrapped to two rows — putting the frame one row over the terminal
		// at 60x20, which is the exact bug the surrounding arithmetic exists
		// to prevent, one line further down.
		b = append(b, base.Width(w).Foreground(t.Warning()).Render(truncateToWidth(m.notice, w)))
	}

	// GORILLA FIX (2026-08-19), from a screenshot: do NOT pad to the full
	// height.
	//
	// The previous version padded every view out to the terminal height so the
	// frame would be a constant size. On screen that means the series list —
	// eight rows of content — sits in a box with thirty blank rows under it,
	// covering the sidebar and the conversation to display nothing. It reads
	// as a program that stopped halfway.
	//
	// The invariant that actually matters is "never TALLER than the terminal",
	// not "always exactly the terminal". Constant height was my reasoning, and
	// the picture beat it: v0.1.56's note already records that bubbletea
	// handles a frame that shrinks. The budget still comes from the terminal
	// height, which is what keeps the notice and the overflow marker inside a
	// known bound; the box simply stops when the content does.

	content := lipgloss.JoinVertical(lipgloss.Left, b...)
	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).
		BorderBackground(styles.PanelBackground()).
		// boxW, not w: lipgloss counts padding INSIDE Width, so the box is
		// told its full styled width and the lines are told the narrower text
		// width. Passing w here made the box 4 columns too narrow for its own
		// content, and lipgloss wraps rather than overflowing.
		Width(boxW).
		Render(content)
}

func (m ArsenalCmp) Bindings() []key.Binding { return nil }

// ── tagfiles ─────────────────────────────────────────────────────────────

// saveTagfile writes the selection as plain text, one id per line.
//
// This is the part of the Slackware installer that mattered most and is least
// obvious. A tagfile is a SELECTION AS A FILE: editable in any editor,
// re-runnable, and — the point — SHAREABLE. Someone who works out a good
// forensics selection can post that file, and the next person gets the map for
// free. That is knowledge distribution at zero marginal cost, which is the
// whole project thesis pointed at tooling.
//
// Plain text with comments, not JSON, because a person has to be able to open
// it and understand it without any of this software.
func (m *ArsenalCmp) saveTagfile() tea.Cmd {
	ids := m.selectedIDs()
	if len(ids) == 0 {
		m.notice = "Nothing selected to save."
		return nil
	}
	path, err := arsenal.SaveTagfile(ids, m.full, m.pm)
	if err != nil {
		m.notice = "Could not save: " + err.Error()
		return nil
	}
	m.notice = "Saved to " + path + " — plain text, editable, shareable."
	return nil
}

// loadTagfile reads a selection back in.
//
// Saving without loading would be half the feature: the POINT of a tagfile is
// that somebody else can send you theirs. A selection you can only ever write
// teaches nobody anything.
//
// Ids this build does not know about are REPORTED, never silently dropped — a
// tagfile from a newer version naming a capability that does not exist here is
// a fact the user should hear.
//
// GORILLA FIX (2026-10-05), from the /arsenal audit (A12), three faults:
//
//   - It could read one path only, the one `s` writes, so a file from somebody
//     else could be loaded only by overwriting your own. L now walks every
//     *.tagfile in the folder, one per press, and names the file it loaded.
//   - Every failure was reported as "No selection to load" — including a file
//     that exists and cannot be read. The real error is now shown.
//   - An id that exists but not on this operating system was going to be
//     called "not in this version" once the list became per-platform. It is
//     reported as what it is.
func (m *ArsenalCmp) loadTagfile() tea.Cmd {
	files := arsenal.Tagfiles()
	if len(files) == 0 {
		m.notice = "No selection file to load. Put a file ending .tagfile in " + arsenal.TagfileDir()
		return nil
	}
	if m.tagIdx >= len(files) {
		m.tagIdx = 0
	}
	at := m.tagIdx
	path := files[at]
	m.tagIdx++

	ids, unknown, err := arsenal.LoadTagfile(path, m.full)
	if err != nil {
		if os.IsNotExist(err) {
			m.notice = "No selection to load at " + path
		} else {
			m.notice = "Could not read " + path + ": " + err.Error()
		}
		return nil
	}
	listed := map[string]bool{}
	for _, id := range m.everyID() {
		listed[id] = true
	}
	m.selected = map[string]bool{}
	added, elsewhere := 0, 0
	for _, id := range ids {
		if !listed[id] {
			elsewhere++ // real, but cannot exist on this operating system
			continue
		}
		if m.status[id].Present {
			continue // already here; selecting it would inflate the cost
		}
		m.selected[id] = true
		added++
	}
	// The file NAME leads and the folder is left out: the notice is one row,
	// and the part that tells two files apart must survive truncation.
	m.notice = fmt.Sprintf("Loaded %d of %d from %s", added, len(ids), filepath.Base(path))
	if len(files) > 1 {
		m.notice += fmt.Sprintf(" (file %d of %d, L for the next)", at+1, len(files))
	}
	if elsewhere > 0 {
		m.notice += fmt.Sprintf(" — %d cannot exist on this system", elsewhere)
	}
	if len(unknown) > 0 {
		m.notice += fmt.Sprintf(" — %d not in this version: %s", len(unknown), strings.Join(unknown, ", "))
	}
	return nil
}
