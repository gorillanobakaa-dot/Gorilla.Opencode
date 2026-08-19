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
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/arsenal"
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
	man           arsenal.Manifest
	pm            arsenal.PackageManager

	// status is detection, done once on open: measured, never claimed.
	status map[string]arsenal.Status

	seriesIdx int
	entryIdx  int
	scrollTop int

	// selected is the tagfile in memory — entry ids the user has ticked.
	selected map[string]bool

	// linkSpeed is KB/s used to turn megabytes into minutes.
	linkSpeed float64

	// pricing holds measured costs, filled lazily because apt-get takes a
	// second or two and the screen must open instantly.
	pricing map[string]arsenal.Cost

	notice string
}

func NewArsenalCmp() ArsenalCmp {
	m, _ := arsenal.Load()
	pm := arsenal.DetectPackageManager()
	st := map[string]arsenal.Status{}
	for _, s := range m.Series {
		for _, e := range s.Entries {
			st[e.ID] = arsenal.DetectEntry(e)
		}
	}
	return ArsenalCmp{
		man:      m,
		pm:       pm,
		status:   st,
		selected: map[string]bool{},
		pricing:  map[string]arsenal.Cost{},
		// 8 KB/s is the audience this project is built for (§8). It is a
		// stated assumption, shown on screen, not a silent one.
		linkSpeed: 8,
	}
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
	m.notice += " — press p to measure the cost."
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
			m.view--
			m.scrollTop = 0
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
				m.view--
				m.scrollTop = 0
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
			m.selected = map[string]bool{}
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
			return m, m.emitInstall()
		}
	case arsenalPricedMsg:
		m.pricing[msg.key] = msg.cost
		return m, nil
	}
	return m, nil
}

func (m *ArsenalCmp) move(d int) {
	switch m.view {
	case viewSeries:
		m.seriesIdx = clampInt(m.seriesIdx+d, 0, len(m.man.Series)-1)
	case viewEntries:
		if s, ok := m.currentSeries(); ok {
			m.entryIdx = clampInt(m.entryIdx+d, 0, len(s.Entries)-1)
		}
	case viewDetail:
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
	m.notice = "measuring with " + pmName(m.pm) + "…"
	pkgs, _, _ := m.installable(ids)
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
		{"mute", "esc back · s save this selection as a shareable file"},
		{"", ""},
	}
	if len(chosen) > 0 {
		out = append(out, arsLine{"h2", fmt.Sprintf("%d capabilit%s to add", len(chosen), plural(len(chosen)))})
		for _, e := range chosen {
			out = append(out, arsLine{"", "  • " + e.Title})
		}
	}
	if already > 0 {
		out = append(out, arsLine{"mute", fmt.Sprintf("  (%d already on this machine — not in the command)", already)})
	}
	for _, e := range unavailable {
		out = append(out, arsLine{"warn", "  ! " + e.Title + " — " + arsenal.UnavailableNote(e, m.pm)})
	}

	if len(pkgs) == 0 {
		out = append(out, arsLine{"", ""}, arsLine{"have", "Nothing to install."})
		return out
	}

	out = append(out, arsLine{"", ""}, arsLine{"h2", fmt.Sprintf("%d package(s)", len(pkgs))})
	for _, l := range wrapPlain(strings.Join(pkgs, " "), 96) {
		out = append(out, arsLine{"mute", "  " + l})
	}

	if c, ok := m.pricing[strings.Join(ids, ",")]; ok && c.Measured {
		line := fmt.Sprintf("%s to download, %s on disk", arsenal.HumanBytes(c.DownloadBytes), arsenal.HumanBytes(c.DiskBytes))
		if t := arsenal.DownloadTime(c.DownloadBytes, m.linkSpeed); t != "" {
			line += fmt.Sprintf(" — about %s at %.0f KB/s", t, m.linkSpeed)
		}
		out = append(out, arsLine{"", ""}, arsLine{"sel", line})
		out = append(out, arsLine{"mute", "measured by your own package manager against what is already here, not a table"})
	} else {
		out = append(out, arsLine{"", ""}, arsLine{"mute", "press p on the previous screen to measure what this costs"})
	}

	out = append(out,
		arsLine{"", ""},
		arsLine{"h2", "Run this yourself"},
		arsLine{"", ""})
	for _, l := range wrapPlain(arsenal.InstallCommand(pkgs, m.pm), 96) {
		out = append(out, arsLine{"on", "  " + l})
	}
	out = append(out,
		arsLine{"", ""},
		arsLine{"mute", "This program will not run it and will never ask for your password. Copy it,"},
		arsLine{"mute", "read it, and run it when you want to. apt can be interrupted and resumed, so a"},
		arsLine{"mute", "long download on a bad line is not lost work."})
	return out
}

// emitInstall asks the MODEL about a selection. Costs a turn, so it is a
// separate, explicitly-labelled key rather than the default path.
func (m ArsenalCmp) emitInstall() tea.Cmd {
	ids := m.selectedIDs()
	if len(ids) == 0 {
		return nil
	}
	pkgs, chosen, unavailable := m.installable(ids)
	if len(pkgs) == 0 {
		return nil
	}
	cmd := arsenal.InstallCommand(pkgs, m.pm)

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d capabilit%s, %d package(s):\n", len(chosen), plural(len(chosen)), len(pkgs))
	for _, e := range chosen {
		fmt.Fprintf(&sb, "  • %s — %s\n", e.ID, e.Title)
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
		{"mute", fmt.Sprintf("on this machine: %d capabilities present, %d not · package manager: %s",
			haveN, missN, pmName(m.pm))},
	}
	if pickN > 0 {
		line := fmt.Sprintf("selected: %d", pickN)
		if c, ok := m.pricing[strings.Join(m.selectedIDs(), ",")]; ok {
			switch {
			case !c.Measured:
				line += " · could not price: " + c.Note
			case c.DownloadBytes == 0:
				line += " · nothing to download — all of it is already here"
			default:
				line += fmt.Sprintf(" · %s to download, %s on disk",
					arsenal.HumanBytes(c.DownloadBytes), arsenal.HumanBytes(c.DiskBytes))
				if t := arsenal.DownloadTime(c.DownloadBytes, m.linkSpeed); t != "" {
					line += fmt.Sprintf(" · about %s at %.0f KB/s", t, m.linkSpeed)
				}
			}
		} else {
			line += " · press p to measure what it costs"
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

func (m ArsenalCmp) seriesLines() []arsLine {
	out := m.header()
	out = append(out,
		arsLine{"", ""},
		arsLine{"mute", "↑↓ move · enter open · space take this series · a everything · n none"},
		arsLine{"mute", "p measure the real cost · i the install command · s save selection · L load one · esc close"},
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
		out = append(out, arsLine{kind, fmt.Sprintf("%s%-46s %s", cursor, s.Title, tag)})
		if i == m.seriesIdx {
			for _, l := range wrapPlain(s.Why, 96) {
				out = append(out, arsLine{"mute", "    " + l})
			}
		}
	}
	out = append(out,
		arsLine{"", ""},
		arsLine{"mute", "Nothing here is installed by this program. It shows you the exact command; you run it."},
		arsLine{"mute", "Everything listed is free and needs no account. Times assume " + fmt.Sprintf("%.0f KB/s", m.linkSpeed) + "."})
	return out
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
		arsLine{"mute", "↑↓ move · enter full detail · space take this one · p cost · i install command · esc back"},
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
			for _, l := range wrapPlain(e.Teaches, 92) {
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
		{"mute", "esc back · space " + pickVerb(m.selected[e.ID])},
		{"", ""},
	}

	switch {
	case st.Present:
		out = append(out, arsLine{"have", "ALREADY ON THIS MACHINE — found: " + strings.Join(st.Found, ", ")})
	case st.Partial():
		out = append(out, arsLine{"warn", "PARTLY HERE — found " + strings.Join(st.Found, ", ") +
			"; missing " + strings.Join(st.Missing, ", ")})
	default:
		out = append(out, arsLine{"mute", "not installed — looked for: " + strings.Join(e.Detect.Binaries, ", ")})
	}
	out = append(out, arsLine{"", ""}, arsLine{"h2", "What it is"})
	for _, l := range wrapPlain(e.Teaches, 96) {
		out = append(out, arsLine{"", l})
	}

	out = append(out, arsLine{"", ""}, arsLine{"h2", "What the agent gains"})
	for _, u := range e.Unlocks {
		out = append(out, arsLine{"", "• " + u})
	}

	out = append(out, arsLine{"", ""}, arsLine{"h2", "What will disappoint you"})
	for _, l := range wrapPlain(e.Caveats, 96) {
		out = append(out, arsLine{"warn", l})
	}

	out = append(out, arsLine{"", ""}, arsLine{"h2", "How to get it"})
	if pkgs := arsenal.PackagesFor(e, m.pm); len(pkgs) > 0 {
		out = append(out, arsLine{"", arsenal.InstallCommand(pkgs, m.pm)})
	} else {
		out = append(out, arsLine{"warn", arsenal.UnavailableNote(e, m.pm)})
		for name, pkgs := range e.Packages {
			if len(pkgs) > 0 {
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

func (m ArsenalCmp) lines() []arsLine {
	switch m.view {
	case viewEntries:
		return m.entryLines()
	case viewDetail:
		return m.detailLines()
	case viewPlan:
		return m.planLines()
	}
	return m.seriesLines()
}

func (m ArsenalCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := dialogWidth(m.width, 108, 6)

	lines := m.lines()
	visible := len(lines)
	if m.height > 0 {
		// Chrome is border(2) + padding(2) + the two trailing rows this View
		// may append: the "more lines" marker and the notice. Reserved rather
		// than hoped for — a frame taller than the window is the one case
		// bubbletea genuinely cannot recover from.
		visible = max(5, m.height-8)
	}
	top := m.scrollTop
	if top > len(lines)-visible {
		top = max(0, len(lines)-visible)
	}
	end := min(top+visible, len(lines))

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
		text := l.text
		// Truncate rather than let lipgloss wrap: an over-wide line costs a
		// physical row the renderer does not count, which strands debris in
		// the transcript on every frame.
		if r := []rune(text); len(r) > w-1 {
			text = string(r[:w-2]) + "…"
		}
		b = append(b, st.Render(text))
	}
	if end < len(lines) {
		b = append(b, base.Width(w).Foreground(t.TextMuted()).
			Render(fmt.Sprintf("  … %d more line(s) — ↓ to continue", len(lines)-end)))
	}
	if m.notice != "" {
		b = append(b, base.Width(w).Foreground(t.Warning()).Render(m.notice))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, b...)
	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).
		BorderBackground(styles.PanelBackground()).
		Width(lipgloss.Width(content) + 4).
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
	path, err := arsenal.SaveTagfile(ids, m.man, m.pm)
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
func (m *ArsenalCmp) loadTagfile() tea.Cmd {
	path := arsenal.TagfilePath()
	ids, unknown, err := arsenal.LoadTagfile(path, m.man)
	if err != nil {
		m.notice = "No selection to load at " + path
		return nil
	}
	m.selected = map[string]bool{}
	added := 0
	for _, id := range ids {
		if m.status[id].Present {
			continue // already here; selecting it would inflate the cost
		}
		m.selected[id] = true
		added++
	}
	m.notice = fmt.Sprintf("Loaded %d of %d from %s", added, len(ids), path)
	if len(unknown) > 0 {
		m.notice += fmt.Sprintf(" — %d not in this version: %s", len(unknown), strings.Join(unknown, ", "))
	}
	return nil
}
