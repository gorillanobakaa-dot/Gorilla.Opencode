// GORILLA OVERRIDE: the first-run consent screen for the optional behaviours
// that show the agent's working.
//
// Only one of them costs anything, and it is off until someone says otherwise.
// The point of asking rather than defaulting: reasoning makes the model generate
// substantially more output, and a program should not start spending more of
// somebody's allowance on the strength of a default they never saw.
//
// It is asked ONCE. A prompt on every launch would be noise, and there is already
// a workspace question before it; the answer is remembered and stays editable in
// /context and /settings.
//
// It renders inline rather than in the alternate screen, like the workspace
// picker, so the choice stays visible above the session as a record of what was
// agreed to.
package startup

import (
	"fmt"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ExtraRow is one switchable behaviour, as presented to the user. The startup
// package cannot import config (config is the lower layer), so the caller passes
// the rows in and applies the answers.
type ExtraRow struct {
	ID   string
	Name string
	What string
	// Costs marks the ones that make the model generate more. Free rows say so,
	// because a user told "these extras cost money" would reasonably assume all of
	// them do, and switching the free ones off would lose information for nothing.
	Costs bool
	// On is the incoming default, and the outgoing answer.
	On bool
}

// ExtrasChoice is what the user decided.
type ExtrasChoice struct {
	Rows []ExtraRow
	Quit bool
}

// Keys are matched on their string form rather than through bubbles/key.
//
// Not stylistic: a sibling test in this package declares a helper function named
// `key`, which collides with that package name at package scope. These startup
// screens are deliberately dependency-light anyway — they run before the theme
// and the TUI exist — so dropping the dependency is the simpler fix than aliasing
// the import and leaving a trap for the next person.

type extrasModel struct {
	rows     []ExtraRow
	sel      int
	width    int
	quit     bool
	accepted bool
}

func (m *extrasModel) Init() tea.Cmd { return nil }

func (m *extrasModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quit = true
			return m, tea.Quit
		case "up", "k":
			if m.sel > 0 {
				m.sel--
			}
		case "down", "j":
			if m.sel < len(m.rows)-1 {
				m.sel++
			}
		case " ", "x":
			m.rows[m.sel].On = !m.rows[m.sel].On
		case "enter":
			m.accepted = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// contentWidth mirrors the workspace picker: never assume a width before the
// terminal has reported one, and never render wider than it.
func (m *extrasModel) contentWidth() int {
	const (
		chrome   = 4
		fallback = 76
		minimum  = 24
	)
	if m.width <= 0 {
		return fallback
	}
	// GORILLA OVERRIDE: removed min(fallback, ...) — fallback is only for the
	// no-width-yet case above. Capping at 76 on a wide terminal clips item
	// names and descriptions; use the full terminal width instead.
	return max(minimum, m.width-chrome)
}

func (m *extrasModel) View() string {
	w := m.contentWidth()
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)
	warn := lipgloss.NewStyle().Bold(true)

	// EVERY line goes through wrapTo or clip. An earlier version wrapped only the
	// prose and left the title, the row labels and the footer at their natural
	// length, which overflowed a 40-column terminal by up to 18 characters — the
	// text is then clipped at the screen edge and this screen is worthless if it
	// cannot be read.
	writeWrapped := func(st lipgloss.Style, text string) {
		for _, l := range wrapTo(text, w) {
			b.WriteString(st.Render(l) + "\n")
		}
	}

	writeWrapped(title, "Before we start — how much do you want to see?")
	b.WriteString("\n")
	writeWrapped(lipgloss.NewStyle(),
		"This agent can show you its working: what it was thinking, which commands "+
			"it ran, and when. Most of that is free. One part is not.")
	b.WriteString("\n")

	for i, r := range m.rows {
		box := "[ ]"
		if r.On {
			box = "[x]"
		}
		cursor := "  "
		if i == m.sel {
			cursor = "> "
		}
		tagText := "free"
		if r.Costs {
			tagText = "COSTS EXTRA"
		}

		// Build the label plain, clip it to the width, and only then style — so
		// the measurement is of visible cells, not escape sequences. The cost tag
		// is preserved and the NAME gives way, because the tag is the part that
		// must never be the thing that falls off the edge.
		label := fmt.Sprintf("%s%s %s", cursor, box, r.Name)
		suffix := "  (" + tagText + ")"
		if room := w - len([]rune(suffix)); room > 0 {
			label = clip(label, room)
		}
		rowStyle := lipgloss.NewStyle()
		if i == m.sel {
			rowStyle = rowStyle.Bold(true)
		}
		tagStyle := dim
		if r.Costs {
			tagStyle = warn
		}
		b.WriteString(rowStyle.Render(label) + tagStyle.Render(suffix) + "\n")

		for _, l := range wrapTo(r.What, w-6) {
			b.WriteString(dim.Render("      "+l) + "\n")
		}
		b.WriteString("\n")
	}

	writeWrapped(warn, "What \"COSTS EXTRA\" means")
	for _, l := range extrasCostLines(w) {
		b.WriteString(l + "\n")
	}

	b.WriteString("\n")
	writeWrapped(dim, "up/down move | space toggle | enter continue | esc quit")
	writeWrapped(dim, "Asked once. Change it any time with /context or /settings.")
	return b.String()
}

// clip shortens a single-line label to w visible columns with an ellipsis. Used
// for labels, where wrapping onto a second line would break the list's alignment.
func clip(s string, w int) string {
	r := []rune(s)
	if w <= 0 {
		return ""
	}
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return "..."
	}
	return string(r[:w-3]) + styles.Ellipsis
}

// extrasCostLines is the honest account, wrapped.
//
// It quotes NO price, deliberately. There is no published rate for the models
// configured here — every local model carries a zero cost in the bundled
// metadata — and an NVIDIA NIM free tier or a local Ollama bills no money at all.
// "This will cost you $X" would be false on this machine, and a warning that is
// provably wrong is worse than no warning: it teaches people to ignore the ones
// that are true.
func extrasCostLines(w int) []string {
	paras := []string{
		"Thinking out loud means the model writes a lot more than just the answer — " +
			"often several times as much. What that costs depends on where it runs:",
		"  | a provider that bills per token — real money, more of it",
		"  | a free tier such as NVIDIA NIM — no money, but your allowance runs " +
			"down faster and you may start hitting request limits",
		"  | a model on this machine such as Ollama — no money, but more CPU, " +
			"more heat, more battery",
		"Either way, every reply takes longer.",
		"No figure is shown because none is published for these models. A made-up " +
			"number would be worse than none.",
	}
	out := []string{}
	for _, p := range paras {
		out = append(out, wrapTo(p, w)...)
	}
	return out
}

// wrapTo is a plain word wrapper that preserves a leading bullet indent.
//
// The startup screens are deliberately dependency-light — they run before the
// theme and the TUI exist — so this does not reach for a wrapping library.
//
// The indent handling is not decoration: strings.Fields discards leading
// whitespace, so an earlier version turned "  | a provider that bills" into
// "· a provider that bills" and the cost list stopped reading as a list.
func wrapTo(s string, w int) []string {
	if w < 8 {
		w = 8
	}

	// Split any leading whitespace/bullet marker from the prose so it survives.
	//
	// ASCII marker since 2026-08-19: the middle dot is East-Asian-Ambiguous,
	// and these lines are wrapped to a measured width. The producer is
	// internal/config/extras.go — the two must change together or the bullet
	// stops being recognised and the cost list silently loses its indent.
	prefix := ""
	rest := s
	if i := strings.Index(s, styles.Bullet+" "); i >= 0 && strings.TrimSpace(s[:i]) == "" {
		prefix = s[:i+2]
		rest = s[i+2:]
	}
	// Continuation lines align under the text, not under the bullet.
	cont := strings.Repeat(" ", len([]rune(prefix)))

	words := strings.Fields(rest)
	if len(words) == 0 {
		return []string{strings.TrimRight(prefix, " ")}
	}

	lines := []string{}
	cur := prefix
	indent := prefix
	for _, word := range words {
		candidate := word
		if strings.TrimSpace(cur) != "" {
			candidate = cur + " " + word
		} else {
			candidate = indent + word
		}
		if len([]rune(candidate)) > w && strings.TrimSpace(cur) != "" {
			lines = append(lines, cur)
			cur = cont + word
			indent = cont
			continue
		}
		cur = candidate
	}
	if strings.TrimSpace(cur) != "" {
		lines = append(lines, cur)
	}
	return lines
}

// AskExtras shows the consent screen and returns the answers.
func AskExtras(rows []ExtraRow) (ExtrasChoice, error) {
	if len(rows) == 0 {
		return ExtrasChoice{}, nil
	}
	m := &extrasModel{rows: rows}
	// GORILLA OVERRIDE: alternate screen, for the same reason as the workspace
	// picker — bubbletea's inline renderer leaves one stale half-drawn frame per
	// resize step, because it erases by logical line count and never repaints on
	// a resize. See the comment on Ask in workspace.go.
	final, err := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithAltScreen()).Run()
	if err != nil {
		return ExtrasChoice{}, err
	}
	fm := final.(*extrasModel)
	if fm.quit {
		return ExtrasChoice{Quit: true}, nil
	}
	return ExtrasChoice{Rows: fm.rows}, nil
}
