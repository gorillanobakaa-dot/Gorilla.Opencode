// GORILLA (2026-10-10): this file did not exist upstream. It draws the long
// explanation pages behind /editor, /helpers and /hooks (internal/helppages).
//
// One scrollable page component for all three, rather than a copy of the /osint
// page per command, because those pages are text and nothing else: the content
// is the part that differs, and it lives in internal/helppages where plain mode
// prints the same lines.
//
// Two differences from the /osint page, both deliberate:
//
//   - Paragraphs are wrapped to the window, not to a fixed 96 columns and then
//     clipped with an ellipsis. A clipped sentence on a narrow terminal is a
//     sentence the reader never sees the end of.
//   - No border. The owner's standing instruction is no lines, and box-drawing
//     characters change width on a terminal set up for East Asian text.
//
// The frame is sized to the budget commandhelp.go uses, so in scrollback mode
// the prompt and footer drawn under it stay on screen, and it fits every size
// in the screen-invariant harness, including 40x8, without a ratchet entry.
package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/helppages"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// CloseInfoPageMsg closes an explanation page.
type CloseInfoPageMsg struct{}

// InfoPageCmp is one scrollable explanation page.
type InfoPageCmp struct {
	page          helppages.Page
	width, height int
	scrollTop     int
}

// NewInfoPageCmp builds the dialog for one page.
func NewInfoPageCmp(p helppages.Page) InfoPageCmp { return InfoPageCmp{page: p} }

// Page is the page being shown.
func (m InfoPageCmp) Page() helppages.Page { return m.page }

func (m *InfoPageCmp) SetSize(w, h int) {
	m.width, m.height = w, h
	m.clamp()
}

func (m InfoPageCmp) Init() tea.Cmd { return nil }

var infoPageKeys = struct {
	Up, Down, PageUp, PageDown, Top, Bottom, Close key.Binding
}{
	Up:       key.NewBinding(key.WithKeys("up", "k")),
	Down:     key.NewBinding(key.WithKeys("down", "j")),
	PageUp:   key.NewBinding(key.WithKeys("pgup")),
	PageDown: key.NewBinding(key.WithKeys("pgdown", " ")),
	Top:      key.NewBinding(key.WithKeys("home", "g")),
	Bottom:   key.NewBinding(key.WithKeys("end", "G")),
	Close:    key.NewBinding(key.WithKeys("esc", "q", "enter")),
}

func (m InfoPageCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		page := max(1, m.contentRows()-1)
		switch {
		case key.Matches(msg, infoPageKeys.Up):
			m.scrollTop--
		case key.Matches(msg, infoPageKeys.Down):
			m.scrollTop++
		case key.Matches(msg, infoPageKeys.PageUp):
			m.scrollTop -= page
		case key.Matches(msg, infoPageKeys.PageDown):
			m.scrollTop += page
		case key.Matches(msg, infoPageKeys.Top):
			m.scrollTop = 0
		case key.Matches(msg, infoPageKeys.Bottom):
			m.scrollTop = len(m.lines())
		case key.Matches(msg, infoPageKeys.Close):
			return m, util.CmdHandler(CloseInfoPageMsg{})
		}
		m.clamp()
	}
	return m, nil
}

func (m InfoPageCmp) Bindings() []key.Binding { return nil }

// infoPageChrome is the rows that are not page text: padding above and below
// (2), the title, and the status line under the text.
const infoPageChrome = 4

// infoPageMaxWidth keeps a paragraph readable on a very wide terminal.
const infoPageMaxWidth = 100

// budgetHeight is the height the frame may use: the terminal less the rows the
// app draws under a dialog in scrollback mode, as in commandhelp.go, except on
// a terminal too short to spare them, where the whole height is used.
func (m InfoPageCmp) budgetHeight() int {
	if m.height <= 0 {
		return 0
	}
	if h := m.height - overlayFooterReserve; h >= infoPageChrome+6 {
		return h
	}
	return m.height
}

// contentRows is how many page lines fit at once.
func (m InfoPageCmp) contentRows() int {
	if m.height <= 0 {
		return 20 // before the first WindowSizeMsg
	}
	return max(1, m.budgetHeight()-infoPageChrome)
}

// contentWidth is the width of a page line: the terminal less Padding(1,2).
func (m InfoPageCmp) contentWidth() int {
	if m.width <= 0 {
		return 96
	}
	return min(infoPageMaxWidth, max(20, m.width-4))
}

type infoLine struct {
	kind helppages.Kind
	text string
}

// lines is the page wrapped to the current width. Every line is at most
// contentWidth display columns: paragraphs wrap at spaces, and a Code line
// wider than the window (a long path on this machine) is cut, not clipped,
// so nothing is hidden.
func (m InfoPageCmp) lines() []infoLine {
	w := m.contentWidth()
	var out []infoLine
	for _, l := range m.page.Lines {
		switch l.Kind {
		case helppages.Code:
			for _, piece := range helppages.HardWrap(l.Text, w) {
				out = append(out, infoLine{l.Kind, piece})
			}
		case helppages.Heading:
			for _, piece := range helppages.Wrap(l.Text, w) {
				out = append(out, infoLine{l.Kind, piece})
			}
		default:
			if strings.TrimSpace(l.Text) == "" {
				out = append(out, infoLine{l.Kind, ""})
				continue
			}
			for _, piece := range helppages.Wrap(l.Text, w) {
				out = append(out, infoLine{l.Kind, piece})
			}
		}
	}
	return out
}

func (m *InfoPageCmp) clamp() {
	maxTop := max(0, len(m.lines())-m.contentRows())
	m.scrollTop = min(max(0, m.scrollTop), maxTop)
}

func (m InfoPageCmp) View() string {
	t := theme.CurrentTheme()
	w := m.contentWidth()
	base := lipgloss.NewStyle().Background(styles.PanelBackground())
	line := func(s string, st lipgloss.Style) string {
		return st.Width(w).MaxWidth(w).Render(truncateToWidth(s, w))
	}

	all := m.lines()
	rows := m.contentRows()
	top := min(max(0, m.scrollTop), max(0, len(all)-rows))
	end := min(top+rows, len(all))

	var b []string
	b = append(b, line(m.page.Title, base.Bold(true).Foreground(t.Primary())))
	for _, l := range all[top:end] {
		st := base.Foreground(t.Text())
		switch l.kind {
		case helppages.Heading:
			st = base.Bold(true).Foreground(t.Primary())
		case helppages.Code:
			st = base.Foreground(t.Accent())
		case helppages.Note:
			st = base.Foreground(t.TextMuted())
		}
		b = append(b, line(l.text, st))
	}

	status := "esc to close"
	if len(all) > rows {
		status = fmt.Sprintf("lines %d-%d of %d | up/down, PgUp/PgDn to read | esc to close", top+1, end, len(all))
	}
	b = append(b, line(status, base.Foreground(t.TextMuted())))

	return base.Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, b...))
}
