// GORILLA OVERRIDE: this file did not exist upstream. It is /reset — undo your
// changes, by scope, with the count of what actually differs shown before you
// commit to anything.
//
// Two hard rules, stated on screen as well as here:
//
//   - It NEVER touches credentials. Provider keys, local endpoints and OAuth
//     tokens are /connect's. A reset that silently logged you out of every
//     provider is a destructive surprise, not a restore.
//   - It NEVER touches sessions or message history. /reset restores
//     CONFIGURATION, not data. /clear handles conversations.
//
// Scopes with nothing to undo are greyed out and unselectable — offering to
// reset something already at its default is theatre.
package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// CloseResetDialogMsg closes the dialog.
type CloseResetDialogMsg struct{}

// ResetAppliedMsg is emitted after a reset so the TUI can invalidate the context
// cache and rebuild the provider.
type ResetAppliedMsg struct{ Info string }

type ResetDialog interface {
	tea.Model
	layout.Bindings
}

const resetDialogWidth = 84

type resetScopeID int

const (
	scopeLoadout resetScopeID = iota
	scopePrompts
	scopeCommands
	scopeRoots
	scopeEverything
)

type resetScope struct {
	id       resetScopeID
	label    string
	describe func() (changed int, detail string)
	apply    func() error
}

type resetKeyMap struct {
	Up, Down, Select, Apply, Escape key.Binding
}

var resetKeys = resetKeyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "down")),
	Select: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select")),
	Apply:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "reset selected")),
	Escape: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}

type resetDialogCmp struct {
	scopes      []resetScope
	selected    map[resetScopeID]bool
	selectedIdx int
	width       int
	height      int
	status      string
}

func NewResetDialogCmp() ResetDialog {
	return &resetDialogCmp{selected: map[resetScopeID]bool{}}
}

func (m *resetDialogCmp) Init() tea.Cmd {
	m.selected = map[resetScopeID]bool{}
	m.selectedIdx = 0
	m.status = ""
	m.scopes = buildResetScopes()
	return nil
}

func buildResetScopes() []resetScope {
	return []resetScope{
		{
			id:    scopeLoadout,
			label: "Context loadout (tools, prompt sections, LSP rows, dials)",
			describe: func() (int, string) {
				n := 0
				for _, c := range config.LoadoutComponents {
					if config.LoadoutEnabled(c.ID) != c.Default {
						n++
					}
				}
				extra := []string{}
				if config.RateLimitRPM() != config.DefaultRPM {
					extra = append(extra, "request pace")
					n++
				}
				if config.MaxSubAgents() != config.DefaultMaxSubAgents {
					extra = append(extra, "helper leash")
					n++
				}
				detail := fmt.Sprintf("%d of %d components differ", n, len(config.LoadoutComponents))
				if len(extra) > 0 {
					detail += " (plus " + strings.Join(extra, ", ") + ")"
				}
				return n, detail
			},
			apply: func() error {
				config.ResetLoadout()
				config.SetRateLimitRPM(config.DefaultRPM)
				config.SetMaxSubAgents(config.DefaultMaxSubAgents)
				return nil
			},
		},
		{
			id:    scopePrompts,
			label: "System prompts (restore the copies compiled into the binary)",
			describe: func() (int, string) {
				var edited []string
				for _, id := range prompt.AllPromptIDs {
					if prompt.IsOverridden(id) {
						edited = append(edited, string(id))
					}
				}
				if len(edited) == 0 {
					return 0, "all four are the shipped defaults"
				}
				return len(edited), "EDITED: " + strings.Join(edited, ", ")
			},
			apply: prompt.ResetAllPrompts,
		},
		{
			id:    scopeCommands,
			label: "Commands (re-enable any you switched off)",
			describe: func() (int, string) {
				off := config.DisabledCommands()
				if len(off) == 0 {
					return 0, "none disabled"
				}
				return len(off), "disabled: " + strings.Join(off, ", ")
			},
			apply: func() error { config.ResetCommands(); return nil },
		},
		{
			id:    scopeRoots,
			label: "Workspace roots (remove directories added with /add-dir)",
			describe: func() (int, string) {
				roots := config.Roots()
				extra := len(roots) - 1
				if extra <= 0 {
					return 0, "only the primary root"
				}
				return extra, fmt.Sprintf("%d additional root(s) registered", extra)
			},
			apply: func() error {
				// Roots()[0] is the primary and is never removed here — that is
				// /cd's job, and a workspace with no primary cannot resolve a
				// relative path.
				roots := config.Roots()
				var firstErr error
				for _, r := range roots[1:] {
					if err := config.RemoveDir(r); err != nil && firstErr == nil {
						firstErr = err
					}
				}
				return firstErr
			},
		},
		{
			id:       scopeEverything,
			label:    "EVERYTHING above",
			describe: func() (int, string) { return 0, "applies every scope listed above" },
		},
	}
}

func (m *resetDialogCmp) width_() int {
	// GORILLA OVERRIDE: the terminal width MINUS this dialog's own chrome, not a
	// content width the chrome is then added to. The border (1+1) and padding
	// (2+2) cost 6 columns; ignoring them made the dialog 82 columns wide in an
	// 80-column terminal, which clips or wraps. Same trap as the v0.1.38
	// invisible input box — a wrapper is never free.
	const chrome = 6
	if m.width > 0 {
		w := m.width - chrome
		if w < 46 {
			return 46
		}
		if w > resetDialogWidth {
			return resetDialogWidth
		}
		return w
	}
	return resetDialogWidth
}

func (m *resetDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, resetKeys.Up):
			if m.selectedIdx > 0 {
				m.selectedIdx--
			} else {
				m.selectedIdx = len(m.scopes) - 1
			}
		case key.Matches(msg, resetKeys.Down):
			if m.selectedIdx < len(m.scopes)-1 {
				m.selectedIdx++
			} else {
				m.selectedIdx = 0
			}
		case key.Matches(msg, resetKeys.Escape):
			return m, util.CmdHandler(CloseResetDialogMsg{})

		case key.Matches(msg, resetKeys.Select):
			s := m.scopes[m.selectedIdx]
			if s.id == scopeEverything {
				// Selecting EVERYTHING selects every scope that has something
				// to undo, so the user sees exactly what it will touch.
				all := !m.selected[scopeEverything]
				for _, sc := range m.scopes {
					if sc.id == scopeEverything {
						continue
					}
					if n, _ := sc.describe(); n > 0 {
						m.selected[sc.id] = all
					}
				}
				m.selected[scopeEverything] = all
				return m, nil
			}
			if n, _ := s.describe(); n == 0 {
				m.status = "nothing to reset in that scope"
				return m, nil
			}
			m.selected[s.id] = !m.selected[s.id]
			m.selected[scopeEverything] = false

		case key.Matches(msg, resetKeys.Apply):
			return m, m.apply()
		}
	}
	return m, nil
}

func (m *resetDialogCmp) apply() tea.Cmd {
	var done []string
	var firstErr error
	for _, s := range m.scopes {
		if s.id == scopeEverything || !m.selected[s.id] || s.apply == nil {
			continue
		}
		if err := s.apply(); err != nil && firstErr == nil {
			firstErr = err
		}
		done = append(done, scopeShortName(s.id))
	}

	if len(done) == 0 {
		m.status = "nothing selected — press space to choose a scope"
		return nil
	}
	if firstErr != nil {
		return util.ReportError(firstErr)
	}
	// Rebuild the scope list so the counts reflect the reset that just happened.
	m.scopes = buildResetScopes()
	m.selected = map[resetScopeID]bool{}
	return util.CmdHandler(ResetAppliedMsg{
		Info: "reset: " + strings.Join(done, ", "),
	})
}

func scopeShortName(id resetScopeID) string {
	switch id {
	case scopeLoadout:
		return "loadout"
	case scopePrompts:
		return "prompts"
	case scopeCommands:
		return "commands"
	case scopeRoots:
		return "roots"
	}
	return "?"
}

func (m *resetDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := m.width_()

	rows := []string{
		base.Foreground(t.Primary()).Bold(true).Width(w).
			Render("Reset to defaults — undo your changes"),
		base.Width(w).Render(""),
	}

	for i, s := range m.scopes {
		changed, detail := s.describe()
		selected := i == m.selectedIdx
		nothingToDo := changed == 0 && s.id != scopeEverything

		box := "[ ]"
		if m.selected[s.id] {
			box = "[x]"
		}
		if nothingToDo {
			box = "   "
		}

		style := base.Width(w)
		switch {
		case selected:
			style = style.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		case nothingToDo:
			style = style.Foreground(t.TextMuted())
		}
		rows = append(rows, style.Render(fmt.Sprintf("  %s %s", box, s.label)))

		detailStyle := base.Width(w).Foreground(t.TextMuted())
		if selected {
			detailStyle = detailStyle.Background(t.Primary()).Foreground(t.Background())
		}
		rows = append(rows, detailStyle.Render("        "+detail))
	}

	if m.status != "" {
		rows = append(rows, base.Width(w).Render(""),
			base.Width(w).Foreground(t.Warning()).Render("  "+m.status))
	}

	rows = append(rows,
		base.Width(w).Render(""),
		base.Foreground(t.Success()).Width(w).
			Render("  API keys, providers and local endpoints are NEVER touched — that is /connect."),
		base.Foreground(t.Success()).Width(w).
			Render("  Sessions and chat history are NEVER touched — that is /clear."),
		base.Width(w).Render(""),
		base.Foreground(t.TextMuted()).Width(w).
			Render("space select   enter reset selected   esc cancel"),
	)

	return base.Padding(1, 2).Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m *resetDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(resetKeys)
}
