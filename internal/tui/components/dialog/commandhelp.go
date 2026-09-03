// GORILLA OVERRIDE: this file did not exist upstream. It is the /help and
// /commands reference — every command, grouped, in plain language.
//
// It exists because the program outgrew its own discoverability: "we have
// introduced so many features that the normal user will get confused. I MYSELF
// knowing what's in here am getting confused."
//
// Two design choices worth defending:
//
// Grouped by what the user is trying to DO ("Which files the AI can see"), not
// alphabetically and not by subsystem. Someone who does not know the command name
// cannot look it up alphabetically, and that is exactly the person reading this.
//
// The selected command's full explanation is shown in place, under the list,
// rather than behind a second keypress. The list alone is a menu; the reason a
// command exists, and what it costs, is the part the user is missing.
package dialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/commands"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

// CloseCommandHelpMsg closes the reference.
type CloseCommandHelpMsg struct{}

// CommandHelpDialog is the /help reference.
type CommandHelpDialog interface {
	tea.Model
	layout.Bindings
	Init() tea.Cmd
	SetSize(width, height int)
	// FocusCommand opens the reference already showing one command, so
	// `/port help` can explain itself instead of leaving the user to find it
	// in a list of thirty. Unknown names fall back to the whole list rather
	// than to an empty screen.
	FocusCommand(name string)
}

// row is either a group heading or a command.
type commandHelpRow struct {
	heading string
	cmd     *commands.Command
}

type commandHelpCmp struct {
	fitter      layout.Fitter
	rows        []commandHelpRow
	selectedIdx int
	scrollTop   int
	width       int
	height      int
	filter      string
	filtering   bool
}

type commandHelpKeyMap struct {
	Up, Down, Filter, Escape, Backspace key.Binding
	// Column jumps between the two columns, the way the old Slackware installer
	// moved between panes. GORILLA OVERRIDE (2026-09-03).
	Column key.Binding
}

var commandHelpKeys = commandHelpKeyMap{
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down", "down")),
	Filter:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Escape:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	Backspace: key.NewBinding(key.WithKeys("backspace")),
	// left/right too: the columns are side by side, so that is the arrow a
	// person reaches for. Binding only tab would be correct and undiscoverable.
	Column: key.NewBinding(key.WithKeys("tab", "left", "right"), key.WithHelp("tab", "other column")),
}

// NewCommandHelpCmp builds the reference dialog.
func NewCommandHelpCmp() CommandHelpDialog { return &commandHelpCmp{} }

func (m *commandHelpCmp) Init() tea.Cmd {
	m.filter = ""
	m.filtering = false
	m.rebuild()
	m.selectedIdx = m.firstCommand()
	m.scrollTop = 0
	return nil
}

// FocusCommand selects one command by name and scrolls to it, so the
// explanation block underneath is that command's.
//
// It matches the NAME exactly rather than reusing the search filter. The
// filter matches substrings across the detail text too, so focusing "port"
// selected /sessions — whose explanation mentions exporting. Close enough for
// a search someone is watching, wrong for a jump that is supposed to land on
// one specific command.
//
// Leaving the full list in place is also the better answer: the surrounding
// commands stay visible, so `/port help` shows what it does AND what else is
// nearby, rather than a single row in an otherwise empty frame.
func (m *commandHelpCmp) FocusCommand(name string) {
	m.Init()
	name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "/")
	for i, r := range m.rows {
		if r.cmd == nil {
			continue
		}
		if strings.EqualFold(r.cmd.Name, name) {
			m.selectedIdx = i
			m.ensureVisible()
			return
		}
		for _, alias := range r.cmd.Aliases {
			if strings.EqualFold(alias, name) {
				m.selectedIdx = i
				m.ensureVisible()
				return
			}
		}
	}
	// No such command: leave the reference as Init left it, showing everything
	// from the top. That is a worse answer than the right command and a much
	// better one than a blank screen.
}

func (m *commandHelpCmp) SetSize(width, height int) {
	m.width, m.height = width, height
	m.ensureVisible()
}

func (m *commandHelpCmp) rebuild() {
	m.rows = nil
	needle := strings.ToLower(strings.TrimSpace(m.filter))

	match := func(c *commands.Command) bool {
		if needle == "" {
			return true
		}
		hay := strings.ToLower(c.Name + " " + strings.Join(c.Aliases, " ") + " " + c.Summary + " " + c.Detail)
		return strings.Contains(hay, needle)
	}

	for _, g := range commands.GroupOrder {
		var group []commandHelpRow
		for _, c := range commands.InGroup(g) {
			if match(c) {
				group = append(group, commandHelpRow{cmd: c})
			}
		}
		if len(group) == 0 {
			continue
		}
		m.rows = append(m.rows, commandHelpRow{heading: string(g)})
		m.rows = append(m.rows, group...)
	}
}

func (m *commandHelpCmp) selectable(i int) bool {
	return i >= 0 && i < len(m.rows) && m.rows[i].cmd != nil
}

// hasAnyCommand distinguishes "the filter matched nothing" from "the first
// command sits at index 0", which firstCommand cannot: it returns 0 for both.
func (m *commandHelpCmp) hasAnyCommand() bool {
	for i := range m.rows {
		if m.selectable(i) {
			return true
		}
	}
	return false
}

func (m *commandHelpCmp) firstCommand() int {
	for i := range m.rows {
		if m.selectable(i) {
			return i
		}
	}
	return 0
}

// move steps to the next selectable row, skipping headings so navigation never
// lands somewhere inert.
func (m *commandHelpCmp) move(delta int) {
	if len(m.rows) == 0 {
		return
	}
	i := m.selectedIdx
	for {
		i += delta
		if i < 0 || i >= len(m.rows) {
			return // at an end; stay put rather than wrapping
		}
		if m.selectable(i) {
			m.selectedIdx = i
			m.ensureVisible()
			return
		}
	}
}

// columnHeight is how many rows go in EACH column, given the vertical space the
// list has been granted. GORILLA OVERRIDE (2026-09-03).
//
// When everything left to show fits on screen the columns are BALANCED rather
// than filled in order. Filling in order is the obvious reading of a
// newspaper layout and it looks broken here: with 37 rows in a 45-row terminal
// the first column swallows all of them and the second is an empty half of the
// screen, which is the "wastes the window" complaint this change exists to fix,
// merely moved to the right-hand side.
//
// Once there is more than one screenful, columns fill completely and the list
// scrolls, because a balanced split of a scrolling list would reflow every row
// on every keypress.
func (m *commandHelpCmp) columnHeight(listRows int) int {
	cols := m.columns()
	if cols == 1 || listRows <= 0 {
		return max(listRows, 0)
	}
	remaining := len(m.rows) - m.scrollTop
	if remaining <= 0 {
		return listRows
	}
	if remaining <= listRows*cols {
		return (remaining + cols - 1) / cols // ceiling: the last column may be short
	}
	return listRows
}

// jumpColumn moves the selection to the other column, keeping the same vertical
// position. GORILLA OVERRIDE (2026-09-03).
//
// The list is laid out newspaper-style: the left column holds the first
// listCapacity rows of the window and the right column the next listCapacity, so
// the sideways step is exactly listCapacity rows.
//
// It searches OUTWARD from the landing row rather than only forwards. Landing on
// a group heading is common, headings are not selectable, and a forwards-only
// scan would skid past several commands to the next one, or fall off the end and
// do nothing at all. Nothing is worse than the wrong thing here: a key that
// sometimes does nothing reads as broken, which is the lesson the low-bandwidth
// banner was written for.
func (m *commandHelpCmp) jumpColumn() {
	if m.columns() < 2 || len(m.rows) == 0 {
		return
	}
	step := m.columnHeight(m.listCapacity())
	target := m.selectedIdx + step
	if target >= len(m.rows) {
		target = m.selectedIdx - step
	}
	if target < 0 {
		return
	}
	for off := 0; off < step; off++ {
		for _, cand := range [2]int{target + off, target - off} {
			if m.selectable(cand) {
				m.selectedIdx = cand
				m.ensureVisible()
				return
			}
		}
	}
}

// overlayFooterReserve is the rows this dialog must NOT use.
//
// GORILLA OVERRIDE (2026-09-03): it is drawn by placeOverlay, and in scrollback
// mode that grows the canvas by the overlay's FULL height and then renders the
// prompt and the footer BELOW it. So the real budget is the terminal minus
// whatever the app draws underneath, and a dialog that takes the whole terminal
// height overflows by exactly that much. The view scrolls, and what leaves the
// screen is the TOP: the title and the line saying how many commands exist.
//
// Counted from a real 200x45 screen, where the app draws a blank, the prompt
// line, a blank, two status lines and the key hint under the overlay. Six is
// that count. It is deliberately a whole-number reserve rather than a measured
// handshake with the parent, because being one row too cautious costs one row
// of list and being one row too greedy costs the header.
const overlayFooterReserve = 6

// budgetHeight is the vertical space this dialog may actually use.
func (m *commandHelpCmp) budgetHeight() int {
	if m.height <= 0 {
		return m.height
	}
	if h := m.height - overlayFooterReserve; h > minListRows+commandHelpFixedLines {
		return h
	}
	// On a very short terminal there is nothing sensible to reserve: showing a
	// cramped reference beats showing none.
	return m.height
}

// fixedLines is the chrome that is always present: padding (2), title, subtitle,
// and the blank line under them.
//
// GORILLA OVERRIDE (2026-09-03): the border's 2 rows came out of this when the
// border was removed. Chrome is SUBTRACTED from a fixed budget, so a stale
// constant here does not look like a stale constant: it looks like the list
// losing two rows for no reason, in a place nobody connects to a border.
const commandHelpFixedLines = 2 + 3

// maxDetailLines caps the explanation so a long one cannot crush the list. The
// text is written to fit.
const maxDetailLines = 7

// minListRows is the smallest list worth showing; the explanation yields first.
const minListRows = 3

// detailBlock returns the explanation lines for the current selection: a blank
// spacer, the alias line where there is one, and the wrapped Detail.
func (m *commandHelpCmp) detailBlock(w int) []string {
	if !m.selectable(m.selectedIdx) {
		return nil
	}
	// Allocate height before rendering. The list gets a minimum of
	// minListRows; the explanation gets whatever is left, and is dropped
	// entirely when there is not enough room for it to say anything useful.
	// Without this the block kept its natural size and the dialog rendered 15
	// lines tall in a 10-line terminal, which scrolls the terminal and destroys
	// the layout.
	allowed := maxDetailLines
	if m.height > 0 {
		allowed = m.budgetHeight() - commandHelpFixedLines - minListRows
	}
	if allowed < 2 {
		return nil // no room to explain anything; the list alone is more useful
	}
	if allowed > maxDetailLines {
		allowed = maxDetailLines
	}

	c := m.rows[m.selectedIdx].cmd
	out := []string{""}
	allowed-- // the blank spacer
	if len(c.Aliases) > 0 && allowed > 1 {
		out = append(out, "also: /"+strings.Join(c.Aliases, "  /"))
		allowed--
	}
	wrapped := wrapText(c.Detail, w)
	if len(wrapped) > allowed {
		wrapped = wrapped[:allowed]
	}
	return append(out, wrapped...)
}

// listCapacity is how many list rows fit, computed by SUBTRACTING the chrome and
// the detail block from the terminal height.
//
// The detail block is measured, not estimated. A fixed guess was wrong because
// the explanation wraps to a variable number of lines, so the dialog rendered
// 15 lines tall in a 10-line terminal — and a dialog taller than the screen
// scrolls the terminal and destroys the layout, which is what the equivalent
// width bug did in v0.1.38.
func (m *commandHelpCmp) listCapacity() int {
	if m.height <= 0 {
		return 12
	}
	budget := m.budgetHeight() - commandHelpFixedLines - len(m.detailBlock(m.contentWidth()))
	if budget < minListRows {
		return minListRows
	}
	return budget
}

func (m *commandHelpCmp) ensureVisible() {
	// GORILLA OVERRIDE (2026-09-03): the window is visibleRows, not
	// listCapacity. With two columns those differ by a factor of two, and
	// scrolling in the smaller unit would drag the view every time the
	// selection entered the right-hand column, which is already on screen.
	cap := m.visibleRows()
	if m.selectedIdx < m.scrollTop {
		m.scrollTop = m.selectedIdx
	}
	if m.selectedIdx >= m.scrollTop+cap {
		m.scrollTop = m.selectedIdx - cap + 1
	}
	// Keep the heading above the selection on screen where it fits: a command
	// with no visible group is harder to place.
	if m.scrollTop > 0 && m.selectedIdx-m.scrollTop == 0 {
		m.scrollTop--
	}
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}
}

func (m *commandHelpCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if m.filtering {
			switch {
			case key.Matches(msg, commandHelpKeys.Escape):
				m.filtering, m.filter = false, ""
				m.rebuild()
				m.selectedIdx = m.firstCommand()
				m.scrollTop = 0
				return m, nil
			case msg.Type == tea.KeyEnter:
				m.filtering = false
				return m, nil
			case key.Matches(msg, commandHelpKeys.Backspace):
				if r := []rune(m.filter); len(r) > 0 {
					m.filter = string(r[:len(r)-1])
				}
			case msg.Type == tea.KeyRunes:
				m.filter += string(msg.Runes)
			default:
				return m, nil
			}
			m.rebuild()
			m.selectedIdx = m.firstCommand()
			m.scrollTop = 0
			return m, nil
		}

		switch {
		case key.Matches(msg, commandHelpKeys.Up):
			m.move(-1)
		case key.Matches(msg, commandHelpKeys.Down):
			m.move(+1)
		case key.Matches(msg, commandHelpKeys.Column):
			m.jumpColumn()
		case key.Matches(msg, commandHelpKeys.Filter):
			m.filtering = true
		case key.Matches(msg, commandHelpKeys.Escape):
			return m, func() tea.Msg { return CloseCommandHelpMsg{} }
		}
		return m, nil
	}
	return m, nil
}

func (m *commandHelpCmp) BindingKeys() []key.Binding {
	return []key.Binding{
		commandHelpKeys.Up, commandHelpKeys.Down,
		commandHelpKeys.Filter, commandHelpKeys.Escape,
	}
}

// GORILLA OVERRIDE (2026-09-03): use the WHOLE window.
//
// This capped itself at 84 columns. On a 200-column terminal that is a narrow
// panel floating in the middle of the screen, with the list showing a dozen of
// forty-odd commands and nothing to suggest the rest exist. Reported from the
// live screen: "normal users would not be aware that there are more commands
// out there or that they can scroll."
//
// A reference nobody can see the extent of is a menu that hides its own menu.
// Full width plus two columns roughly quadruples what is on screen at once,
// which is the actual fix; making the scrollbar more obvious would only have
// made the cage easier to see.
//
// chrome is the horizontal padding only. The border is gone (see renderAt), so
// there is nothing else to subtract, and it is counted here rather than guessed.
func (m *commandHelpCmp) contentWidth() int {
	const (
		chrome   = 4 // Padding(1,2): 2 columns each side, INSIDE the styled width
		minimum  = 24
		fallback = 84 // only before the first WindowSizeMsg
	)
	if m.width <= 0 {
		return fallback
	}
	return max(minimum, m.width-chrome)
}

// twoColumnMinWidth is the content width at which a second column starts
// helping rather than hurting. Below it each column is too narrow to hold a
// command name plus a summary worth reading, and two cramped columns are worse
// than one honest one.
const twoColumnMinWidth = 100

// columnGap separates the two columns. Two spaces, not a drawn rule: a vertical
// line here would be a box-drawing character in every row of the list, and those
// are East Asian Ambiguous, so they measure 1 column normally and 2 on a
// terminal configured for CJK. See CLAUDE.md 4a.
const columnGap = 2

// columns is how many command columns fit.
func (m *commandHelpCmp) columns() int {
	if m.contentWidth() >= twoColumnMinWidth {
		return 2
	}
	return 1
}

// columnWidth is the width of ONE column, gap already subtracted.
func (m *commandHelpCmp) columnWidth() int {
	cols := m.columns()
	if cols == 1 {
		return m.contentWidth()
	}
	return (m.contentWidth() - columnGap) / cols
}

// visibleRows is how many commands are on screen at once: the height of a
// column times the number of columns. Scrolling and selection both work in
// this unit, or the second column becomes a place the cursor can never reach.
func (m *commandHelpCmp) visibleRows() int {
	return m.listCapacity() * m.columns()
}

// View renders the dialog at a size that is MEASURED to fit, not predicted.
//
// GORILLA OVERRIDE: this used to compute its capacity from
// commandHelpFixedLines — a constant standing in for border, padding, title,
// subtitle and a blank line. That constant was wrong, and a minimum-rows floor
// silently overrode the height limit on top of it, so the dialog asked for 11 rows
// in a 10-row terminal and 13 in an 8-row one. Both were found by rendering into a
// terminal cell grid (internal/tui/screentest), not by reading the arithmetic.
//
// Now the frame is rendered and measured. layout.FitHeight brings the row count
// down until the result genuinely fits, so whatever the chrome happens to be it is
// counted because it exists rather than because someone predicted it. If even the
// smallest list still does not fit, the explanation is dropped — the list of
// commands is the point of the dialog, the explanation is the extra.
func (m *commandHelpCmp) View() string {
	// Progressively leaner variants, in priority order. The list of commands is the
	// reason the dialog exists, so it is the last thing to give ground: first the
	// explanation goes, then the search hint and the blank line under the title.
	// Same approach as the sign-in overlay, which sheds its prose before it sheds a
	// single character of the URL.
	for i, v := range []struct{ detail, subtitle bool }{
		{true, true},
		{false, true},
		{false, false},
	} {
		// The explanation block's length depends on the selection, and the search
		// hint on the filter, so both belong in the key.
		key := uint64(m.selectedIdx)*1315423911 + uint64(len(m.filter))*31 + uint64(i)
		view := m.fitter.Fit(m.budgetHeight(), len(m.rows), 1, key, func(rows int) string {
			return m.renderAt(rows, v.detail, v.subtitle)
		})
		if m.height <= 0 || lipgloss.Height(view) <= m.budgetHeight() {
			return view
		}
	}
	// Nothing fits: return the leanest form rather than nothing at all. A clipped
	// dialog is still usable; an empty one is not.
	return m.fitter.Fit(m.budgetHeight(), len(m.rows), 1,
		uint64(m.selectedIdx)*1315423911+uint64(len(m.filter))*31+99,
		func(rows int) string { return m.renderAt(rows, false, false) })
}

func (m *commandHelpCmp) renderAt(listRows int, withDetail, withSubtitle bool) string {
	t := theme.CurrentTheme()
	w := m.contentWidth()
	base := lipgloss.NewStyle().Background(styles.PanelBackground())

	// Only sizes. It must NOT set the background: doing so clobbered the
	// selected row's highlight, leaving foreground equal to background — the
	// selected command rendered as an invisible blank line, so /help appeared to
	// be missing whichever command you were reading about. Every caller derives
	// its style from base, which already carries the panel background.
	line := func(s string, st lipgloss.Style) string {
		return st.Width(w).MaxWidth(w).Render(s)
	}

	var b []string
	b = append(b, line("Commands — what each one does", base.Bold(true).Foreground(t.Primary())))

	// The hint and the blank under it are the first prose to go on a very short
	// terminal — except while searching, when the query being typed IS the state
	// the user needs to see and dropping it would look like the search broke.
	searching := m.filtering || m.filter != ""
	if withSubtitle || searching {
		// GORILLA OVERRIDE (2026-09-03): say how many there are, and name the
		// key that reaches the rest.
		//
		// The old hint offered search, up/down and close, and never said the
		// list continued past the visible rows. Someone who does not already
		// know a reference is scrollable reads the bottom row as the end of the
		// program's abilities. The count is the honest fix: "of 43" tells you
		// there is more without needing a scrollbar to be noticed.
		sub := "type / to search | up/down to read | esc to close"
		if n := m.commandCount(); n > 0 {
			shown := min(m.visibleRows(), len(m.rows)-m.scrollTop)
			pos := "showing " + itoa(shown) + " of " + itoa(len(m.rows)) + " lines, " + itoa(n) + " commands"
			nav := "up/down to read"
			if m.columns() > 1 {
				nav = "up/down to read | tab for the other column"
			}
			sub = pos + " | " + nav + " | / to search | esc to close"
		}
		if searching {
			sub = "search: " + m.filter + "_"
		}
		// TRUNCATED. This line grew when the count and the tab hint were added
		// to it, and at 80 columns lipgloss WRAPPED it to two screen rows while
		// commandHelpFixedLines still counted one. That is the documented trap:
		// the symptom is height, in a different place from the cause.
		b = append(b, line(truncateToWidth(sub, w), base.Foreground(t.TextMuted())))
		if withSubtitle {
			b = append(b, line("", base))
		}
	}

	if len(m.rows) == 0 {
		b = append(b, line("Nothing matches "+m.filter, base.Foreground(t.TextMuted())))
	}

	// GORILLA OVERRIDE (2026-09-03): lay the list out in columns.
	//
	// Newspaper order, not row-major: the left column is the first listRows of
	// the window and the right column the next listRows. Reading straight down a
	// column is how a grouped list is read, and it keeps a group's commands
	// together instead of interleaving two groups line by line.
	cols := m.columns()
	cw := m.columnWidth()
	colH := m.columnHeight(listRows)
	// Only the rows actually used. Padding out to listRows would push the
	// explanation block down the screen behind a wall of blanks, and on a tall
	// terminal off it entirely.
	for r := 0; r < colH; r++ {
		cells := make([]string, 0, cols)
		for c := 0; c < cols; c++ {
			// Past the bottom of a balanced column this is out of range and
			// renderCell returns blank padding of the right width, so the row
			// below still lines up.
			i := -1
			if r < colH {
				i = m.scrollTop + c*colH + r
			}
			cells = append(cells, m.renderCell(i, cw, base, t))
		}
		b = append(b, line(strings.Join(cells, strings.Repeat(" ", columnGap)), base))
	}

	// The selected command's full explanation, in place. This is the part the
	// user is actually missing; a list of names they already half-know is not.
	// Rendered from the same detailBlock that listCapacity measured, so the
	// height budget and the output cannot disagree.
	detail := m.detailBlock(w)
	if !withDetail {
		detail = nil
	}
	for _, l := range detail {
		st := base.Foreground(t.Text())
		if strings.HasPrefix(l, "also: ") {
			st = base.Foreground(t.TextMuted())
		}
		b = append(b, line(l, st))
	}

	// GORILLA OVERRIDE (2026-09-03): no border.
	//
	// This is a full-window reference now, so a rounded box drawn around the
	// whole screen frames nothing: it spends 2 columns and 2 rows of a fixed
	// budget to draw a line just inside the edge of the terminal, which already
	// has an edge. It is also box-drawing, and those characters are East Asian
	// Ambiguous, so they measure 1 column normally and 2 on a terminal set up
	// for CJK: the one decoration here that can change width on somebody else's
	// machine. CLAUDE.md 4a, and the owner's standing instruction is NO LINES.
	// GORILLA OVERRIDE (2026-09-03), second pass: do NOT pad this to the full
	// terminal height.
	//
	// The first version did, on the reasoning that a full-window reference
	// should BE the window. That is wrong for this dialog specifically. It is
	// drawn by placeOverlay, and in scrollback mode that grows the canvas by the
	// overlay's FULL height and then puts the prompt and the footer BELOW it. A
	// frame exactly as tall as the terminal therefore produces terminal height
	// plus footer, the whole thing scrolls, and the rows that leave the screen
	// are the ones at the TOP: the title and the line saying how many commands
	// there are. Caught on a real screenshot, where the list looked right and
	// the header was simply gone.
	//
	// So height is left to the fitter, which already sizes to fit, and the
	// widening is horizontal only. That was the actual complaint.

	return lipgloss.NewStyle().
		Background(styles.PanelBackground()).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, b...))
}

// commandCount is how many of the rows are commands rather than group headings.
// The list length overstates what a user is looking for by the number of groups.
func (m *commandHelpCmp) commandCount() int {
	n := 0
	for i := range m.rows {
		if m.selectable(i) {
			n++
		}
	}
	return n
}

// itoa keeps the subtitle free of a fmt import for one integer.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		return "-" + string(d)
	}
	return string(d)
}

// renderCell renders one list slot: a heading, a command, or blank padding when
// the column has run past the end of the rows. GORILLA OVERRIDE (2026-09-03).
//
// It always returns EXACTLY cw display columns. A short cell would let the row
// to its right slide leftwards and the second column would stop being a column,
// and a long one would make lipgloss wrap the joined line into two SCREEN rows
// while the height budget still counted it as one. That disagreement is the
// documented cause of the list overflowing its frame, so the width is padded and
// truncated here rather than trusted.
func (m *commandHelpCmp) renderCell(i, cw int, base lipgloss.Style, t theme.Theme) string {
	if i < 0 || i >= len(m.rows) {
		return base.Width(cw).MaxWidth(cw).Render("")
	}
	r := m.rows[i]
	if r.heading != "" {
		return base.Bold(true).Foreground(t.Text()).
			Width(cw).MaxWidth(cw).Render(truncateToWidth(r.heading, cw))
	}
	name := "/" + r.cmd.Name
	if r.cmd.Args != "" {
		name += " " + r.cmd.Args
	}
	// TRUNCATED, not wrapped. lipgloss's Width() wraps anything longer, so a
	// command with a long Args used more than one SCREEN line while listCapacity
	// still counted it as one ROW. The two disagreed, the list overflowed its
	// budget, and the rows at the bottom were pushed out of the frame entirely:
	// /help became unreachable by scrolling the moment /port was added. One row
	// is one line no matter what anyone puts in Args.
	//
	// The name column is narrower in two-column mode because there is less room,
	// but never so narrow that "/port [operation]" loses its operation.
	nameW := 18
	if m.columns() > 1 {
		nameW = 17
	}
	row := "  " + padRight(name, nameW) + r.cmd.Summary
	return rowStyle(base, t, i == m.selectedIdx).
		Width(cw).MaxWidth(cw).Render(truncateToWidth(row, cw))
}

// rowStyle is the style for one command row. Extracted so the invariant that
// actually matters can be asserted directly: a rendered row still CONTAINS its
// text even when foreground and background are identical, so a test that greps
// the view for "/clear" passes while the row is invisible on screen. That is
// exactly how the invisible-selection bug shipped.
func rowStyle(base lipgloss.Style, t theme.Theme, selected bool) lipgloss.Style {
	if selected {
		return base.Background(t.Primary()).Foreground(t.Background()).Bold(true)
	}
	return base.Foreground(t.Text())
}

func padRight(s string, n int) string {
	if len([]rune(s)) >= n {
		return s + " "
	}
	return s + strings.Repeat(" ", n-len([]rune(s)))
}

// wrapText breaks text at word boundaries to at most width columns.
func wrapText(s string, width int) []string {
	if width < 8 {
		width = 8
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var out []string
	cur := words[0]
	for _, word := range words[1:] {
		if len([]rune(cur))+1+len([]rune(word)) <= width {
			cur += " " + word
			continue
		}
		out = append(out, cur)
		cur = word
	}
	return append(out, cur)
}
