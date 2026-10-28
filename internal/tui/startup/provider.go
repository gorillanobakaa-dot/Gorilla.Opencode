// GORILLA OVERRIDE: the every-launch provider portal. People change their mind
// about which provider to run day to day (Ollama offline, NIM when the quota is
// fresh, a paid key when it matters), and the only routes were /connect after
// startup or hand-editing config.json — the second of which the desktop-icon
// majority effectively does not have. So the choice is offered on every launch.
//
// The tax is controlled by making the common case one keystroke: the cursor
// starts on the provider that is already active, and Enter continues with it.
// Esc also keeps the current setup. Only an actual change costs more presses.
//
// Same layering inversion as the workspace picker and the extras screen: this
// package cannot import config, so cmd builds the rows, this file renders them,
// and cmd applies the answer.
package startup

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProviderRow is one selectable provider, as presented to the user.
type ProviderRow struct {
	ID   string // stable id the caller switches on; never shown
	Name string // one-line label
	What string // wrapped description, shown for the selected row only
	// Warning renders bold under the description. Reserved for things that
	// will genuinely surprise someone, not for editorializing.
	Warning string
	// NeedsInput marks rows that require a typed credential when not yet
	// configured. InputPrompt labels the field; Secret masks the echo.
	NeedsInput  bool
	InputPrompt string
	Secret      bool
	// InputPrompt2, when set, asks for a SECOND value after the first.
	//
	// GORILLA OVERRIDE: Cloudflare needs an account id AND an API token.
	// Collecting both from one field meant parsing two values out of an
	// arbitrary paste, and a paste containing a newline submits the field early
	// — the terminal sends that newline as Enter — so everything after the first
	// line was silently discarded. One field per value removes the ambiguity
	// entirely: a stray newline now just advances to the next field.
	InputPrompt2 string
	// Secret2 masks the second field independently: an account id is not a
	// secret and hiding it only makes it harder to check for a typo.
	Secret2 bool
	// Check, when set, judges the first value before it is accepted. It returns
	// "" for a usable value, otherwise the sentence to show under the field.
	// Nothing is saved while it objects. It lives on the row because this
	// package cannot know what an NVIDIA key looks like and cmd can.
	Check func(value string) string
	// Free marks a provider that costs nothing to use. Rendered as a distinct
	// tag because a row that costs money and a row that does not otherwise look
	// identical, and the people this is built for cannot afford to find out by
	// being billed.
	Free bool
	// CostWarning, when set, is shown on a screen of its own before a paid
	// provider can be used, and the person must type "I understand" to go on.
	// CostAccepted means they already did, once, for this provider.
	//
	// GORILLA (2026-10-28, owner): an API key bills a bank card for every
	// message, and the people this is built for are young and on no budget.
	// The row's one line ("Paid API") did not say what a message costs.
	CostWarning  string
	CostAccepted bool
	// Configured means a usable credential is already saved, so Enter selects
	// without asking for anything. `r` replaces the stored credential.
	Configured bool
	// Active marks the provider the session would use if nothing changes.
	// The cursor starts here.
	Active bool
}

// ProviderChoice is what the user decided.
type ProviderChoice struct {
	ID     string // selected row; empty when Keep or Quit
	Input  string // typed credential, empty when none was asked for
	Input2 string // second value, when the row asked for one
	Keep   bool   // esc: continue with the current setup, change nothing
	// CostAccepted: the person typed "I understand" on this row's CostWarning
	// during this visit, so the caller records it and never asks again.
	CostAccepted bool
	Quit         bool // ctrl+c (or esc with nothing configured): abort the launch
}

type providerModel struct {
	rows    []ProviderRow
	canKeep bool // esc continues only if something already works
	sel     int
	width   int
	height  int // 0 until the terminal reports it; then the frame never exceeds it
	// key-entry state
	entering bool
	input    []rune
	// input2 holds the second value, and stage tracks which field is active.
	input2 []rune
	stage  int // 0 = first field, 1 = second
	hint   string
	choice ProviderChoice
	done   bool
	// cost-warning state: acking shows the warning screen, ack is what has been
	// typed, acceptedID the row whose warning was accepted on this visit.
	acking     bool
	ack        []rune
	acceptedID string
	ackThen    string // "enter" or "r": what the key did before the warning
}

// costAckPhrase is what must be typed to accept a CostWarning.
const costAckPhrase = "I understand"

func (m *providerModel) Init() tea.Cmd { return nil }

func (m *providerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.acking {
			return m.updateAcking(msg)
		}
		if m.entering {
			return m.updateEntering(msg)
		}
		switch msg.String() {
		case "ctrl+c":
			m.choice = ProviderChoice{Quit: true}
			m.done = true
			return m, tea.Quit
		case "esc":
			if m.canKeep {
				m.choice = ProviderChoice{Keep: true}
			} else {
				m.choice = ProviderChoice{Quit: true}
			}
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.sel > 0 {
				m.sel--
			}
			m.hint = ""
		case "down", "j":
			if m.sel < len(m.rows)-1 {
				m.sel++
			}
			m.hint = ""
		case "r":
			// Deliberate re-entry of a credential that is already saved.
			if r := m.rows[m.sel]; r.Configured && r.NeedsInput {
				if m.needsAck(r) {
					m.startAck("r")
					return m, nil
				}
				m.entering = true
				m.input, m.input2, m.stage = nil, nil, 0
				m.hint = ""
			}
		case "enter":
			r := m.rows[m.sel]
			if m.needsAck(r) {
				m.startAck("enter")
				return m, nil
			}
			return m.afterEnter()
		}
	}
	return m, nil
}

// needsAck reports whether row r must show its cost warning before anything
// else happens.
func (m *providerModel) needsAck(r ProviderRow) bool {
	return r.CostWarning != "" && !r.CostAccepted && m.acceptedID != r.ID
}

func (m *providerModel) startAck(then string) {
	m.acking, m.ack, m.ackThen, m.hint = true, nil, then, ""
}

// afterEnter is what Enter on the selected row does once any cost warning is
// out of the way: ask for the key, or choose the row.
func (m *providerModel) afterEnter() (tea.Model, tea.Cmd) {
	r := m.rows[m.sel]
	if r.NeedsInput && !r.Configured {
		m.entering = true
		m.input, m.input2, m.stage = nil, nil, 0
		m.hint = ""
		return m, nil
	}
	m.choice = ProviderChoice{ID: r.ID, CostAccepted: m.acceptedID == r.ID}
	m.done = true
	return m, tea.Quit
}

func (m *providerModel) updateAcking(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.choice = ProviderChoice{Quit: true}
		m.done = true
		return m, tea.Quit
	case "esc":
		m.acking, m.ack, m.hint = false, nil, ""
		return m, nil
	case "enter":
		if !strings.EqualFold(strings.TrimSpace(string(m.ack)), costAckPhrase) {
			m.hint = "Type I understand (those two words) and press Enter, or Esc to go back."
			m.ack = nil
			return m, nil
		}
		m.acking, m.ack, m.hint = false, nil, ""
		m.acceptedID = m.rows[m.sel].ID
		if m.ackThen == "r" {
			m.entering = true
			m.input, m.input2, m.stage = nil, nil, 0
			return m, nil
		}
		return m.afterEnter()
	case "backspace":
		if len(m.ack) > 0 {
			m.ack = m.ack[:len(m.ack)-1]
		}
		return m, nil
	}
	if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
		m.ack = append(m.ack, msg.Runes...)
		if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
			m.ack = append(m.ack, ' ')
		}
	}
	return m, nil
}

func (m *providerModel) updateEntering(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.choice = ProviderChoice{Quit: true}
		m.done = true
		return m, tea.Quit
	case "esc":
		// Back a step, not straight out: having typed a long token, being thrown
		// all the way back to the list by one Esc is its own small disaster.
		if m.stage == 1 {
			m.stage = 0
			m.hint = ""
			return m, nil
		}
		m.entering = false
		m.input, m.input2 = nil, nil
		m.hint = ""
		return m, nil
	case "enter":
		row := m.rows[m.sel]
		if m.stage == 0 {
			if len(m.input) == 0 {
				m.hint = "Nothing entered yet — press Ctrl+V to paste the value, or Esc to go back."
				return m, nil
			}
			if row.Check != nil {
				if problem := row.Check(strings.TrimSpace(string(m.input))); problem != "" {
					// Refused, and the field is emptied: the next Ctrl+V must
					// replace the bad value, not be glued onto the end of it.
					m.hint = problem
					m.input = nil
					return m, nil
				}
			}
			if row.InputPrompt2 != "" {
				m.stage = 1
				m.hint = ""
				return m, nil
			}
		} else if len(m.input2) == 0 {
			m.hint = "Nothing entered yet — paste the value, or Esc to go back."
			return m, nil
		}
		m.choice = ProviderChoice{
			ID:           row.ID,
			Input:        strings.TrimSpace(string(m.input)),
			Input2:       strings.TrimSpace(string(m.input2)),
			CostAccepted: m.acceptedID == row.ID,
		}
		m.done = true
		return m, tea.Quit
	case "backspace":
		if m.stage == 1 {
			if len(m.input2) > 0 {
				m.input2 = m.input2[:len(m.input2)-1]
			}
		} else if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	case "ctrl+v":
		// GORILLA OVERRIDE (2026-10-05): Ctrl+V did NOTHING in this field.
		//
		// The comment that stood here said "runes cover typing and paste alike".
		// That holds only when the terminal performs the paste itself and sends
		// the text as keystrokes. When it does not, the program receives the
		// Ctrl+V keypress (bubbletea: KeyCtrlV) and is expected to fetch the
		// clipboard. This handler had no case for it, so the press was dropped.
		//
		// Every other text field in this program is a bubbles textinput, which
		// does fetch the clipboard on Ctrl+V. So paste worked in the chat box
		// and in /connect and failed HERE, on the one screen that exists for
		// pasting keys.
		//
		// MEASURED: the owner's config.json and both of its backups (1 and 30
		// September) hold no NVIDIA entry at all, and his screenshot shows this
		// field at "(1 chars)" after a paste. INFERRED, not measured: that his
		// console window is one that hands the keypress over. Whichever it is,
		// both routes are now handled, and a value that is not a key is refused
		// (ProviderRow.Check) instead of being saved.
		text, err := readClipboard()
		if err != nil || strings.TrimSpace(text) == "" {
			m.hint = "The clipboard is empty or could not be read. Copy the key again, then press Ctrl+V."
			return m, nil
		}
		m.insert(cleanPasted(text))
		return m, nil
	}
	// Typing, and a paste performed by the terminal (right-click, or Ctrl+V in
	// Windows Terminal): the text arrives as runes.
	if msg.Type == tea.KeyRunes {
		m.insert(cleanPasted(string(msg.Runes)))
	}
	return m, nil
}

// readClipboard is a seam: tests must not touch the real clipboard, and the
// real one may hold something private.
var readClipboard = clipboard.ReadAll

// insert appends to whichever field is active.
func (m *providerModel) insert(r []rune) {
	if len(r) == 0 {
		return
	}
	if m.stage == 1 {
		m.input2 = append(m.input2, r...)
	} else {
		m.input = append(m.input, r...)
	}
	m.hint = ""
}

// cleanPasted keeps what can be part of a key or an id and drops the rest:
// line breaks and tabs that ride along with a copied line, and the invisible
// control characters a terminal wraps round a paste. A credential never
// contains any of them, and one stray newline stored inside a key makes every
// request fail with an error that points nowhere near the cause.
func cleanPasted(s string) []rune {
	// The markers a terminal puts round a pasted block, when they arrive as
	// text instead of being consumed.
	for _, marker := range []string{"\x1b[200~", "\x1b[201~", "[200~", "[201~"} {
		s = strings.ReplaceAll(s, marker, "")
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f || r == 0xfeff {
			continue
		}
		out = append(out, r)
	}
	return out
}

// contentWidth mirrors the extras screen: never assume a width before the
// terminal reports one, and never render wider than it.
func (m *providerModel) contentWidth() int {
	const (
		chrome   = 4
		fallback = 76
		minimum  = 24
	)
	if m.width <= 0 {
		return fallback
	}
	// GORILLA OVERRIDE: removed min(fallback, ...) — the fallback is only for
	// the no-width-yet case above. Capping at 76 on a wide terminal clips long
	// provider names mid-word; use the full terminal width instead.
	return max(minimum, m.width-chrome)
}

func (m *providerModel) View() string {
	w := m.contentWidth()
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)
	warn := lipgloss.NewStyle().Bold(true)

	if m.acking {
		return m.ackView(w)
	}

	// EVERY line goes through wrapTo or clip — the frame-width invariant.
	writeWrapped := func(st lipgloss.Style, text string) {
		for _, l := range wrapTo(text, w) {
			b.WriteString(st.Render(l) + "\n")
		}
	}

	writeWrapped(title, "Choose a provider for this session")
	writeWrapped(dim, "Enter continues with the highlighted one.")
	b.WriteString("\n")

	// GORILLA FIX (2026-10-19): on an 80x24 terminal (the default on the owner's
	// 11-inch MacBook Air) the frame was taller than the window and the title
	// scrolled off the top. The list now shows only the rows that fit, around the
	// cursor, and says how many are above and below.
	first, last := 0, len(m.rows)
	if m.height > 0 {
		// everything that is not a provider row: the title block, the
		// description block and the key line, as this function renders them
		fixed := strings.Count(b.String(), "\n") + 1 + len(wrapTo(m.rows[m.sel].What, w)) + 1 +
			len(wrapTo(dimKeys(m), w))
		if m.rows[m.sel].Warning != "" {
			fixed += len(wrapTo(m.rows[m.sel].Warning, w))
		}
		if m.entering {
			fixed += 6
		}
		if m.hint != "" {
			fixed += len(wrapTo(m.hint, w))
		}
		// -1: the frame ends with a newline, and the terminal gives that a row too
		first, last = visibleWindow(len(m.rows), m.sel, m.height-fixed-1)
	}
	if first > 0 {
		b.WriteString(dim.Render(fmt.Sprintf("  ^ %d more above", first)) + "\n")
	}
	for i := first; i < last; i++ {
		r := m.rows[i]
		cursor := "  "
		if i == m.sel {
			cursor = "> "
		}
		tag := ""
		if r.Free {
			tag = "  free"
		}
		switch {
		case r.Active:
			tag += "  (current)"
		case r.Configured:
			tag += "  (ready)"
		}
		label := clip(cursor+r.Name, w-len([]rune(tag)))
		st := lipgloss.NewStyle()
		if i == m.sel {
			st = st.Bold(true)
		}
		b.WriteString(st.Render(label) + dim.Render(tag) + "\n")
	}
	if last < len(m.rows) {
		b.WriteString(dim.Render(fmt.Sprintf("  v %d more below", len(m.rows)-last)) + "\n")
	}

	// Description of the SELECTED row only. Rendering all eleven would make
	// the frame taller than a small terminal, and a frame taller than the
	// window breaks the renderer.
	cur := m.rows[m.sel]
	b.WriteString("\n")
	for _, l := range wrapTo(cur.What, w) {
		b.WriteString(dim.Render(l) + "\n")
	}
	if cur.Warning != "" {
		for _, l := range wrapTo(cur.Warning, w) {
			b.WriteString(warn.Render(l) + "\n")
		}
	}

	if m.entering {
		b.WriteString("\n")
		// With two fields, show the completed one above so the value just typed
		// is visibly banked rather than seeming to have vanished.
		if cur.InputPrompt2 != "" && m.stage == 1 {
			writeWrapped(dim, "1. "+cur.InputPrompt)
			b.WriteString(dim.Render(m.echo(m.input, cur.Secret, w)) + "\n\n")
			writeWrapped(lipgloss.NewStyle(), "2. "+cur.InputPrompt2)
			b.WriteString(m.echo(m.input2, cur.Secret2, w) + "\n")
			writeWrapped(dim, "Ctrl+V to paste | Enter to save | Esc to go back to the first value")
		} else {
			prompt := cur.InputPrompt
			if cur.InputPrompt2 != "" {
				prompt = "1. " + prompt
			}
			writeWrapped(lipgloss.NewStyle(), prompt)
			b.WriteString(m.echo(m.input, cur.Secret, w) + "\n")
			next := "Ctrl+V to paste | Enter to save | Esc to go back"
			if cur.InputPrompt2 != "" {
				next = "Ctrl+V to paste | Enter for the next value | Esc to go back"
			}
			writeWrapped(dim, next)
		}
	}
	if m.hint != "" {
		writeWrapped(warn, m.hint)
	}

	b.WriteString("\n")
	writeWrapped(dim, dimKeys(m))
	return b.String()
}

// ackView is the cost warning on a screen of its own: nothing else competes
// with it, and on the Air's 39 rows it fits without the list around it.
func (m *providerModel) ackView(w int) string {
	var b strings.Builder
	red := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#D70000"))
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)
	for i, para := range strings.Split(m.rows[m.sel].CostWarning, "\n") {
		if para == "" {
			b.WriteString("\n")
			continue
		}
		st := lipgloss.NewStyle()
		if i == 0 {
			st = red
		}
		// Table rows keep their columns; prose wraps.
		if strings.HasPrefix(para, "  ") {
			b.WriteString(st.Render(clip(para, w)) + "\n")
			continue
		}
		for _, l := range wrapTo(para, w) {
			b.WriteString(st.Render(l) + "\n")
		}
	}
	b.WriteString("\n")
	for _, l := range wrapTo("Type "+costAckPhrase+" and press Enter to continue:", w) {
		b.WriteString(bold.Render(l) + "\n")
	}
	b.WriteString("> " + clip(string(m.ack), w-2) + "\n")
	if m.hint != "" {
		for _, l := range wrapTo(m.hint, w) {
			b.WriteString(bold.Render(l) + "\n")
		}
	}
	b.WriteString("\n")
	for _, l := range wrapTo("esc go back to the list | ctrl+c quit", w) {
		b.WriteString(dim.Render(l) + "\n")
	}
	return b.String()
}

// dimKeys is the key line under the portal. One definition, because View both
// renders it and counts its wrapped height.
func dimKeys(m *providerModel) string {
	cur := m.rows[m.sel]
	switch {
	case cur.Configured && cur.NeedsInput:
		return "up/down move | enter select | r replace saved key | esc keep current | ctrl+c quit"
	case !m.canKeep:
		return "up/down move | enter select | esc/ctrl+c quit"
	}
	return "up/down move | enter select | esc keep current setup | ctrl+c quit"
}

// visibleWindow is the slice [first, last) of n list rows to draw when only
// room rows fit, keeping sel inside it. One of the room rows on each side that
// is cut off is taken by its "N more" line. room below 3 still shows the
// selected row: a list that shows nothing cannot be used.
func visibleWindow(n, sel, room int) (first, last int) {
	if room >= n {
		return 0, n
	}
	if room < 3 {
		room = 3
	}
	show := room - 2 // both "more" lines, the worst case
	first = sel - show/2
	if first < 0 {
		first = 0
	}
	last = first + show
	if last > n {
		last = n
		first = max(0, n-show)
	}
	// a side with nothing cut off does not need its "more" line: give it back
	if first == 0 {
		last = min(n, room-1)
	} else if last == n {
		first = max(0, n-(room-1))
	}
	return first, last
}

// inputLine renders the typed value. Secrets render as bullets with a length
// counter — a pasted key must NEVER appear on screen (it would sit in the
// terminal transcript; that has happened once already and is a standing rule).
// echo renders one field's value. Secrets render as bullets with a length
// counter — a pasted key must NEVER appear on screen (it would sit in the
// terminal transcript; that has happened once already and is a standing rule).
// Non-secret values ARE shown, because an account id is not a credential and
// hiding it only makes a typo impossible to spot.
func (m *providerModel) echo(val []rune, secret bool, w int) string {
	n := len(val)
	suffix := fmt.Sprintf(" (%d chars)", n)
	room := w - len([]rune(suffix))
	if room < 1 {
		room = 1
	}
	var shown string
	if secret {
		shown = clip(strings.Repeat("*", n), room)
	} else {
		shown = clip(string(val), room)
	}
	if shown == "" {
		shown = "_" // an empty field still needs a visible cursor cell
	}
	return shown + suffix
}

// ProviderModel is the portal's bubbletea model.
//
// GORILLA OVERRIDE: exported so callers outside this package can render the
// portal headlessly and assert what is actually on the screen. The row list is
// assembled in cmd/, so a test that a given provider is VISIBLE can only live
// there — and "the row exists in a slice" is not the same claim as "the user can
// see it". Behaviour is unchanged; AskProviders builds this same value.
type ProviderModel = providerModel

// NewProviderModel builds the portal model with the cursor on the active row.
func NewProviderModel(rows []ProviderRow, canKeep bool) *ProviderModel {
	sel := 0
	for i, r := range rows {
		if r.Active {
			sel = i
			break
		}
	}
	return &providerModel{rows: rows, canKeep: canKeep, sel: sel}
}

// AskProviders shows the portal and returns the answer. canKeep must be true
// only when the session already has a working provider — it is what makes Esc
// mean "keep" rather than "quit".
func AskProviders(rows []ProviderRow, canKeep bool) (ProviderChoice, error) {
	if len(rows) == 0 {
		return ProviderChoice{Keep: true}, nil
	}
	m := NewProviderModel(rows, canKeep)
	// Alternate screen for the same reason as the workspace picker and the
	// extras screen: the inline renderer strands a stale half-frame per resize
	// step. See the comment on Ask in workspace.go.
	final, err := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithAltScreen()).Run()
	if err != nil {
		return ProviderChoice{}, err
	}
	return final.(*providerModel).choice, nil
}
