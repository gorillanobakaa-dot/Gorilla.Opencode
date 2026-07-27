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
}

// row is either a group heading or a command.
type commandHelpRow struct {
	heading string
	cmd     *commands.Command
}

type commandHelpCmp struct {
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
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "down")),
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

func (m *commandHelpCmp) View() string {
	t := theme.CurrentTheme()
	w := m.contentWidth()
	base := lipgloss.NewStyle().Background(t.Background())

	line := func(s string, st lipgloss.Style) string {
		return st.Background(t.Background()).Width(w).MaxWidth(w).Render(s)
	}

	var b []string
	b = append(b, line("Commands — what each one does", base.Bold(true).Foreground(t.Primary())))

	sub := "type / to search · ↑↓ to read · esc to close"
	if m.filtering || m.filter != "" {
		sub = "search: " + m.filter + "_"
	}
	b = append(b, line(sub, base.Foreground(t.TextMuted())))
	b = append(b, line("", base))

	if len(m.rows) == 0 {
		b = append(b, line("Nothing matches "+m.filter, base.Foreground(t.TextMuted())))
	}

	end := min(m.scrollTop+m.listCapacity(), len(m.rows))
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
		row := padRight(name, 18) + r.cmd.Summary
		st := base.Foreground(t.Text())
		if i == m.selectedIdx {
			st = base.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		}
		b = append(b, line("  "+row, st))
	}

	// The selected command's full explanation, in place. This is the part the
	// user is actually missing; a list of names they already half-know is not.
	// Rendered from the same detailBlock that listCapacity measured, so the
	// height budget and the output cannot disagree.
	for _, l := range m.detailBlock(w) {
		st := base.Foreground(t.Text())
		if strings.HasPrefix(l, "also: ") {
			st = base.Foreground(t.TextMuted())
		}
		b = append(b, line(l, st))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).
		Background(t.Background()).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, b...))
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
