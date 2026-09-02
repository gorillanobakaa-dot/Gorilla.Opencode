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
}

var commandHelpKeys = commandHelpKeyMap{
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down", "down")),
	Filter:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Escape:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	Backspace: key.NewBinding(key.WithKeys("backspace")),
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

// fixedLines is the chrome that is always present: border (2), padding (2),
// title, subtitle, and the blank line under them.
const commandHelpFixedLines = 2 + 2 + 3

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
		allowed = m.height - commandHelpFixedLines - minListRows
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
	budget := m.height - commandHelpFixedLines - len(m.detailBlock(m.contentWidth()))
	if budget < minListRows {
		return minListRows
	}
	return budget
}

func (m *commandHelpCmp) ensureVisible() {
	cap := m.listCapacity()
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

// contentWidth is the usable width inside the border.
func (m *commandHelpCmp) contentWidth() int {
	const (
		chrome    = 6
		preferred = 84
		minimum   = 24
	)
	if m.width <= 0 {
		return preferred
	}
	return max(minimum, min(preferred, m.width-chrome))
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
		view := m.fitter.Fit(m.height, len(m.rows), 1, key, func(rows int) string {
			return m.renderAt(rows, v.detail, v.subtitle)
		})
		if m.height <= 0 || lipgloss.Height(view) <= m.height {
			return view
		}
	}
	// Nothing fits: return the leanest form rather than nothing at all. A clipped
	// dialog is still usable; an empty one is not.
	return m.fitter.Fit(m.height, len(m.rows), 1,
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
		sub := "type / to search | up/down to read | esc to close"
		if searching {
			sub = "search: " + m.filter + "_"
		}
		b = append(b, line(sub, base.Foreground(t.TextMuted())))
		if withSubtitle {
			b = append(b, line("", base))
		}
	}

	if len(m.rows) == 0 {
		b = append(b, line("Nothing matches "+m.filter, base.Foreground(t.TextMuted())))
	}

	end := min(m.scrollTop+listRows, len(m.rows))
	for i := m.scrollTop; i < end; i++ {
		r := m.rows[i]
		if r.heading != "" {
			b = append(b, line(r.heading, base.Bold(true).Foreground(t.Text())))
			continue
		}
		name := "/" + r.cmd.Name
		if r.cmd.Args != "" {
			name += " " + r.cmd.Args
		}
		// Two columns, so the eye can run down the names.
		//
		// TRUNCATED, not wrapped. lipgloss's Width() wraps anything longer, so a
		// command with a long Args used more than one SCREEN line while
		// listCapacity still counted it as one ROW. The two disagreed, the list
		// overflowed its budget, and the rows at the bottom were pushed out of
		// the frame entirely — /help became unreachable by scrolling the moment
		// /port was added, and /review had already been quietly costing two
		// lines before that. One row is now one line no matter what anyone puts
		// in Args, so the height budget cannot be wrong again.
		row := padRight(name, 18) + r.cmd.Summary
		b = append(b, line(truncateToWidth("  "+row, w), rowStyle(base, t, i == m.selectedIdx)))
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

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).
		Background(styles.PanelBackground()).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, b...))
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
