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
	// Configured means a usable credential is already saved, so Enter selects
	// without asking for anything. `r` replaces the stored credential.
	Configured bool
	// Active marks the provider the session would use if nothing changes.
	// The cursor starts here.
	Active bool
}

// ProviderChoice is what the user decided.
type ProviderChoice struct {
	ID    string // selected row; empty when Keep or Quit
	Input string // typed credential, empty when none was asked for
	Keep  bool   // esc: continue with the current setup, change nothing
	Quit  bool   // ctrl+c (or esc with nothing configured): abort the launch
}

type providerModel struct {
	rows    []ProviderRow
	canKeep bool // esc continues only if something already works
	sel     int
	width   int
	// key-entry state
	entering bool
	input    []rune
	hint     string
	choice   ProviderChoice
	done     bool
}

func (m *providerModel) Init() tea.Cmd { return nil }

func (m *providerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
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
				m.entering = true
				m.input = nil
				m.hint = ""
			}
		case "enter":
			r := m.rows[m.sel]
			if r.NeedsInput && !r.Configured {
				m.entering = true
				m.input = nil
				m.hint = ""
				return m, nil
			}
			m.choice = ProviderChoice{ID: r.ID}
			m.done = true
			return m, tea.Quit
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
		m.entering = false
		m.input = nil
		m.hint = ""
		return m, nil
	case "enter":
		if len(m.input) == 0 {
			m.hint = "Nothing entered yet — paste the value, or Esc to go back."
			return m, nil
		}
		m.choice = ProviderChoice{
			ID:    m.rows[m.sel].ID,
			Input: strings.TrimSpace(string(m.input)),
		}
		m.done = true
		return m, tea.Quit
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	}
	// Runes cover typing and paste alike; bubbletea delivers a paste as one
	// KeyRunes message with every rune in it.
	if msg.Type == tea.KeyRunes {
		m.input = append(m.input, msg.Runes...)
		m.hint = ""
	}
	return m, nil
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
	return max(minimum, min(fallback, m.width-chrome))
}

func (m *providerModel) View() string {
	w := m.contentWidth()
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)
	warn := lipgloss.NewStyle().Bold(true)

	// EVERY line goes through wrapTo or clip — the frame-width invariant.
	writeWrapped := func(st lipgloss.Style, text string) {
		for _, l := range wrapTo(text, w) {
			b.WriteString(st.Render(l) + "\n")
		}
	}

	writeWrapped(title, "Choose a provider for this session")
	writeWrapped(dim, "Enter continues with the highlighted one.")
	b.WriteString("\n")

	for i, r := range m.rows {
		cursor := "  "
		if i == m.sel {
			cursor = "> "
		}
		tag := ""
		switch {
		case r.Active:
			tag = "  (current)"
		case r.Configured:
			tag = "  (ready)"
		}
		label := clip(cursor+r.Name, w-len([]rune(tag)))
		st := lipgloss.NewStyle()
		if i == m.sel {
			st = st.Bold(true)
		}
		b.WriteString(st.Render(label) + dim.Render(tag) + "\n")
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
		writeWrapped(lipgloss.NewStyle(), cur.InputPrompt)
		b.WriteString(m.inputLine(w) + "\n")
		writeWrapped(dim, "Enter to save · Esc to go back")
	}
	if m.hint != "" {
		writeWrapped(warn, m.hint)
	}

	b.WriteString("\n")
	keys := "up/down move · enter select · esc keep current setup · ctrl+c quit"
	if !m.canKeep {
		keys = "up/down move · enter select · esc/ctrl+c quit"
	}
	if cur.Configured && cur.NeedsInput {
		keys = "up/down move · enter select · r replace saved key · esc keep current · ctrl+c quit"
	}
	writeWrapped(dim, keys)
	return b.String()
}

// inputLine renders the typed value. Secrets render as bullets with a length
// counter — a pasted key must NEVER appear on screen (it would sit in the
// terminal transcript; that has happened once already and is a standing rule).
func (m *providerModel) inputLine(w int) string {
	n := len(m.input)
	suffix := fmt.Sprintf(" (%d chars)", n)
	room := w - len([]rune(suffix))
	if room < 1 {
		room = 1
	}
	var shown string
	if m.rows[m.sel].Secret {
		shown = clip(strings.Repeat("•", n), room)
	} else {
		shown = clip(string(m.input), room)
	}
	if shown == "" {
		shown = "_" // an empty field still needs a visible cursor cell
	}
	return shown + suffix
}

// AskProviders shows the portal and returns the answer. canKeep must be true
// only when the session already has a working provider — it is what makes Esc
// mean "keep" rather than "quit".
func AskProviders(rows []ProviderRow, canKeep bool) (ProviderChoice, error) {
	if len(rows) == 0 {
		return ProviderChoice{Keep: true}, nil
	}
	sel := 0
	for i, r := range rows {
		if r.Active {
			sel = i
			break
		}
	}
	m := &providerModel{rows: rows, canKeep: canKeep, sel: sel}
	// Alternate screen for the same reason as the workspace picker and the
	// extras screen: the inline renderer strands a stale half-frame per resize
	// step. See the comment on Ask in workspace.go.
	final, err := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithAltScreen()).Run()
	if err != nil {
		return ProviderChoice{}, err
	}
	return final.(*providerModel).choice, nil
}
