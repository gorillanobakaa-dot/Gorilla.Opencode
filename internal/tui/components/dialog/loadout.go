// GORILLA OVERRIDE: this file did not exist upstream. It renders the
// context loadout menu (opened with /context): a transparent, Slackware-
// style view of everything sent to the model every turn, its token cost,
// and switches to strip it down. See internal/config/loadout.go.
package dialog

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// GORILLA OVERRIDE: use nearly the full terminal width so no message or
// tradeoff line ever wraps or truncates.
const (
	loadoutMinWidth    = 100
	loadoutSidePadding = 6 // border + breathing room on each side
)

// CloseLoadoutDialogMsg closes the loadout menu.
type CloseLoadoutDialogMsg struct{}

// LoadoutChangedMsg signals the loadout changed (agent should rebuild tools).
type LoadoutChangedMsg struct{}

type LoadoutDialog interface {
	tea.Model
	layout.Bindings
}

type loadoutDialogCmp struct {
	selectedIdx int
	termWidth   int
}

// width returns the dialog inner width — as wide as the terminal allows,
// so the full messages are always readable.
func (m *loadoutDialogCmp) width() int {
	if m.termWidth <= 0 {
		return loadoutMinWidth
	}
	w := m.termWidth - loadoutSidePadding
	if w < loadoutMinWidth {
		w = loadoutMinWidth
	}
	return w
}

type loadoutKeyMap struct {
	Up, Down, Toggle, Reset, Escape key.Binding
}

var loadoutKeys = loadoutKeyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "down")),
	Toggle: key.NewBinding(key.WithKeys(" ", "enter"), key.WithHelp("space", "toggle")),
	Reset:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset defaults")),
	Escape: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
}

func NewLoadoutDialogCmp() LoadoutDialog { return &loadoutDialogCmp{} }

func (m *loadoutDialogCmp) Init() tea.Cmd { return nil }

func (m *loadoutDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, loadoutKeys.Up):
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}
		case key.Matches(msg, loadoutKeys.Down):
			if m.selectedIdx < len(config.LoadoutComponents)-1 {
				m.selectedIdx++
			}
		case key.Matches(msg, loadoutKeys.Toggle):
			config.ToggleLoadout(config.LoadoutComponents[m.selectedIdx].ID)
			return m, util.CmdHandler(LoadoutChangedMsg{})
		case key.Matches(msg, loadoutKeys.Reset):
			config.ResetLoadout()
			return m, util.CmdHandler(LoadoutChangedMsg{})
		case key.Matches(msg, loadoutKeys.Escape):
			return m, util.CmdHandler(CloseLoadoutDialogMsg{})
		}
	}
	return m, nil
}

func (m *loadoutDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := m.width()

	total := config.LoadoutActiveTokens()
	header := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render("Context loadout — what every turn costs")
	sub := base.Foreground(t.TextMuted()).Width(w).
		Render(fmt.Sprintf("~%s tokens are sent to the model on EVERY turn, even to say \"yo\".", commaInt(total)))
	fixed := base.Foreground(t.TextMuted()).Width(w).
		Render(fmt.Sprintf("(base system prompt ~%s is always on; the rest is yours to cut)", commaInt(config.LoadoutBaseTokens())))

	var rows []string
	for i, c := range config.LoadoutComponents {
		on := config.LoadoutEnabled(c.ID)
		box := "[ ]"
		if on {
			box = "[x]"
		}
		mark := ""
		if c.Critical {
			mark = " ⚠"
		}
		// GORILLA OVERRIDE: real measured cost via ComponentTokens.
		line := fmt.Sprintf("%s %-18s ~%-6s  %s%s", box, c.Name, commaInt(config.ComponentTokens(c)), tradeoffText(on, c.Tradeoff), mark)
		if r := []rune(line); len(r) > w-1 {
			line = string(r[:w-2]) + "…"
		}
		style := base.Width(w)
		switch {
		case i == m.selectedIdx:
			style = style.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		case !on:
			style = style.Foreground(t.TextMuted())
		}
		rows = append(rows, style.Render(line))
	}

	help := base.Foreground(t.TextMuted()).Width(w).
		Render("space toggle · r reset defaults · esc close   ⚠ = disabling cripples the agent · prompt.* apply on restart")

	content := lipgloss.JoinVertical(lipgloss.Left,
		header, sub, fixed, "",
		lipgloss.JoinVertical(lipgloss.Left, rows...), "",
		help,
	)
	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func tradeoffText(on bool, tradeoff string) string {
	if on {
		return "off: " + tradeoff
	}
	return "OFF — " + tradeoff
}

// commaInt formats an int with thousands separators.
func commaInt(n int) string {
	s := fmt.Sprintf("%d", n)
	out := ""
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += ","
		}
		out += string(r)
	}
	return out
}

func (m *loadoutDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(loadoutKeys)
}
