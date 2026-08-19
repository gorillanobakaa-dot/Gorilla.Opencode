// GORILLA OVERRIDE: this file did not exist upstream. It renders the live
// sub-agent (helper) monitor, opened with /tasks. This is the Gorilla
// transparency-and-control surface: the user can SEE every helper agent
// running on their behalf and KILL it — one at a time, or all at once with
// the Nuclear Option. See internal/llm/agent/subagent_registry.go.
package dialog

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	tasksMinWidth    = 80
	tasksSidePadding = 6
)

// CloseTasksDialogMsg closes the tasks monitor.
type CloseTasksDialogMsg struct{}

type TasksDialog interface {
	tea.Model
	layout.Bindings
}

type tasksDialogCmp struct {
	selectedIdx int
	termWidth   int
}

func NewTasksDialogCmp() TasksDialog { return &tasksDialogCmp{} }

type tasksKeyMap struct {
	Up, Down, Kill, Nuke, Escape key.Binding
}

var tasksKeys = tasksKeyMap{
	Up:   key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up", "up")),
	Down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down", "down")),
	Kill: key.NewBinding(key.WithKeys("enter", "x", "d"), key.WithHelp("enter", "kill selected")),
	// GORILLA NUCLEAR OPTION — "Kill 'em all, and the horse they rode in on."
	Nuke:   key.NewBinding(key.WithKeys("X", "ctrl+x"), key.WithHelp("X", "kill 'em all")),
	Escape: key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "close")),
}

func (m *tasksDialogCmp) Init() tea.Cmd { return nil }

func (m *tasksDialogCmp) width() int {
	if m.termWidth <= 0 {
		return tasksMinWidth
	}
	w := m.termWidth - tasksSidePadding
	if w < tasksMinWidth {
		w = tasksMinWidth
	}
	return w
}

func (m *tasksDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
	case tea.KeyMsg:
		tasks := agent.ListSubAgents()
		switch {
		case key.Matches(msg, tasksKeys.Up):
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}
		case key.Matches(msg, tasksKeys.Down):
			if m.selectedIdx < len(tasks)-1 {
				m.selectedIdx++
			}
		case key.Matches(msg, tasksKeys.Kill):
			if m.selectedIdx < len(tasks) {
				info, ok := agent.KillSubAgent(tasks[m.selectedIdx].ID)
				if m.selectedIdx > 0 {
					m.selectedIdx--
				}
				if ok {
					return m, util.ReportWarn(fmt.Sprintf("Killed helper %s — %s", info.ID, truncate(info.Prompt, 48)))
				}
			}
		case key.Matches(msg, tasksKeys.Nuke):
			n := agent.KillAllSubAgents()
			m.selectedIdx = 0
			if n == 0 {
				return m, util.ReportInfo("No helpers running — nothing to nuke.")
			}
			// GORILLA NUCLEAR OPTION.
			return m, util.ReportWarn(fmt.Sprintf("☢ Killed 'em all — %d helper(s), their tasks, and the horse they rode in on.", n))
		case key.Matches(msg, tasksKeys.Escape):
			return m, util.CmdHandler(CloseTasksDialogMsg{})
		}
	}
	return m, nil
}

func (m *tasksDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := m.width()

	// Pulled fresh every render, so the list is always current (the app
	// re-renders on every sub-agent spawn/exit event).
	tasks := agent.ListSubAgents()
	if m.selectedIdx >= len(tasks) {
		m.selectedIdx = max(0, len(tasks)-1)
	}

	// GORILLA FIX 2026-08-14: the header used to say "Running helper agents"
	// while the list could only ever contain running ones — a queued helper was
	// not registered at all. A user who asked for 10 saw 4 and concluded the
	// feature was broken. The count is now itemised so the list can never imply
	// a number the run does not have.
	header := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render("🦍 Helper agents — " + taskCountSummary(tasks))
	sub := base.Foreground(t.TextMuted()).Width(w).
		Render("Every sub-agent the main agent spawned on your behalf. You are in control.")

	var rows []string
	if len(tasks) == 0 {
		rows = append(rows, base.Foreground(t.TextMuted()).Width(w).
			Render("  (no helper agents running right now)"))
	} else {
		for i, tk := range tasks {
			elapsed := time.Since(tk.StartedAt).Truncate(time.Second)
			// GORILLA OVERRIDE: state marker + state WORD. The gorilla is the
			// constant; the coloured signal beside it carries the state. The
			// word is not decoration and must never be dropped to save room —
			// the reference machine is a 2012 laptop whose terminal font may
			// render every emoji as an identical box, and this list is how the
			// user decides what to kill.
			line := fmt.Sprintf(" %-4s %s %-7s %6s  %s",
				tk.ID, tk.State.Marker(), tk.State.Label(), elapsed.String(),
				truncate(tk.Prompt, w-taskRowPrefixCells))
			style := base.Width(w)
			if i == m.selectedIdx {
				style = style.Background(t.Primary()).Foreground(t.Background()).Bold(true)
			} else {
				switch tk.State {
				case agent.SubAgentRunning:
					style = style.Foreground(t.Success())
				case agent.SubAgentQueued:
					style = style.Foreground(t.Warning())
				case agent.SubAgentFailed, agent.SubAgentKilled:
					style = style.Foreground(t.Error())
				case agent.SubAgentDone:
					style = style.Foreground(t.TextMuted())
				}
			}
			rows = append(rows, style.Render(line))
		}
	}

	help := base.Foreground(t.TextMuted()).Width(w).
		Render("up/down pick | enter/x kill one | X kill 'em all (Nuclear) | esc close")

	content := lipgloss.JoinVertical(lipgloss.Left,
		header, sub, "",
		lipgloss.JoinVertical(lipgloss.Left, rows...), "",
		help,
	)
	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(styles.PanelBackground()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func (m *tasksDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(tasksKeys)
}

func truncate(s string, max int) string {
	if max < 1 {
		max = 1
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "..."
	}
	return string(r[:max-3]) + styles.Ellipsis
}

// taskRowPrefixCells is the display width of everything a row prints before the
// prompt: leading space, 4-cell id, space, 4-cell marker, space, 7-cell label,
// space, 6-cell elapsed, two spaces. Kept as a named constant because the
// marker is EMOJI — two cells each, four for the pair — and getting this wrong
// puts the line over the terminal width, which is the documented root cause of
// the footer-drift bug (CLAUDE.md). TestTaskRowsNeverExceedTheWidth holds it
// honest.
const taskRowPrefixCells = 1 + 4 + 1 + 4 + 1 + 7 + 1 + 6 + 2

// taskCountSummary describes the run in its own terms: "4 running, 6 queued"
// rather than a single number that cannot distinguish a small run from a
// throttled one.
func taskCountSummary(tasks []agent.SubAgentInfo) string {
	if len(tasks) == 0 {
		return "none right now"
	}
	var running, queued, done, failed, killed int
	for _, tk := range tasks {
		switch tk.State {
		case agent.SubAgentRunning:
			running++
		case agent.SubAgentQueued:
			queued++
		case agent.SubAgentDone:
			done++
		case agent.SubAgentFailed:
			failed++
		case agent.SubAgentKilled:
			killed++
		}
	}
	parts := []string{}
	for _, p := range []struct {
		n    int
		word string
	}{
		{running, "running"}, {queued, "queued"}, {done, "done"},
		{failed, "failed"}, {killed, "killed"},
	} {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", p.n, p.word))
		}
	}
	return strings.Join(parts, ", ")
}
