package core

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kujtimiihoxha/termai/internal/config"
	"github.com/kujtimiihoxha/termai/internal/llm/models"
	"github.com/kujtimiihoxha/termai/internal/tui/styles"
	"github.com/kujtimiihoxha/termai/internal/tui/util"
	"github.com/kujtimiihoxha/termai/internal/version"
)

type statusCmp struct {
	info       *util.InfoMsg
	width      int
	messageTTL time.Duration
}

// clearMessageCmd is a command that clears status messages after a timeout
func (m statusCmp) clearMessageCmd(ttl time.Duration) tea.Cmd {
	return tea.Tick(ttl, func(time.Time) tea.Msg {
		return util.ClearStatusMsg{}
	})
}

func (m statusCmp) Init() tea.Cmd {
	return nil
}

func (m statusCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case util.InfoMsg:
		m.info = &msg
		ttl := msg.TTL
		if ttl == 0 {
			ttl = m.messageTTL
		}
		return m, m.clearMessageCmd(ttl)
	case util.ClearStatusMsg:
		m.info = nil
	}
	return m, nil
}

var (
	versionWidget = styles.Padded.Background(styles.DarkGrey).Foreground(styles.Text).Render(version.Version)
	helpWidget    = styles.Padded.Background(styles.Grey).Foreground(styles.Text).Render("? help")
)

func (m statusCmp) View() string {
	status := styles.Padded.Background(styles.Grey).Foreground(styles.Text).Render("? help")
	if m.info != nil {
		infoStyle := styles.Padded.
			Foreground(styles.Base).
			Width(m.availableFooterMsgWidth())
		switch m.info.Type {
		case util.InfoTypeInfo:
			infoStyle = infoStyle.Background(styles.Blue)
		case util.InfoTypeWarn:
			infoStyle = infoStyle.Background(styles.Peach)
		case util.InfoTypeError:
			infoStyle = infoStyle.Background(styles.Red)
		}
		// Truncate message if it's longer than available width
		msg := m.info.Msg
		availWidth := m.availableFooterMsgWidth() - 10
		if len(msg) > availWidth && availWidth > 0 {
			msg = msg[:availWidth] + "..."
		}
		status += infoStyle.Render(msg)
	} else {
		status += styles.Padded.
			Foreground(styles.Base).
			Background(styles.LightGrey).
			Width(m.availableFooterMsgWidth()).
			Render("")
	}
	status += m.model()
	status += versionWidget
	return status
}

func (m statusCmp) availableFooterMsgWidth() int {
	// -2 to accommodate padding
	return max(0, m.width-lipgloss.Width(helpWidget)-lipgloss.Width(versionWidget)-lipgloss.Width(m.model()))
}

func (m statusCmp) model() string {
	model := models.SupportedModels[config.Get().Model.Coder]
	return styles.Padded.Background(styles.Grey).Foreground(styles.Text).Render(model.Name)
}

func NewStatusCmp() tea.Model {
	return &statusCmp{
		messageTTL: 10 * time.Second,
	}
}
