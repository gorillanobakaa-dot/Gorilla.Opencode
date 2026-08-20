// GORILLA OVERRIDE: this file did not exist upstream. It is the connection
// profile picker shown after the provider portal.
//
// WHY AFTER AND NOT BEFORE. The screen wants to tell you how fast your link is.
// Measuring that BEFORE any traffic would mean buying a measurement — and on a
// 2 KB/s satellite link a probe costs real money and real minutes to report
// something the user already knows. After the provider portal there has already
// been a real transfer (a model list), so the estimate is free. That single
// constraint fixes the ordering; see internal/config/linkspeed.go.
//
// NO DECORATIVE RULES HERE. Standing instruction, and the measured reasons are
// in internal/tui/styles/ascii.go: box-drawing characters are ambiguous-width,
// break under byte slicing, and cost 30x to measure. Text and blank lines only.
//
// Same layering inversion as the other pickers: this package cannot import
// config, so cmd builds the rows and applies the answer.
package startup

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConnRow is one selectable profile, already rendered into plain strings by cmd.
type ConnRow struct {
	ID    string
	Name  string
	Rate  string // "1-9 KB/s"
	Links string // real-world examples so someone recognises their own
	What  string // plain-language effect, shown for the selected row

	// Effect is the concrete "what changes" line: waits, retries, upload cap.
	Effect string
	// Advice is the honest suggestion for this speed — which tools cost the most
	// and what is safe to switch off. Empty on fast profiles, where nothing needs
	// saying. Advice ONLY: this screen never changes the loadout, because a
	// profile that silently altered what the agent can do would make switching
	// unpredictable (owner's decision, 2026-08-20).
	Advice string

	Recommended bool // matches the measurement
	Active      bool // currently in force; the cursor starts here
}

// ConnChoice is what the user decided.
type ConnChoice struct {
	ID   string
	Keep bool // esc: leave the profile alone
	Quit bool // ctrl+c
}

type connModel struct {
	rows     []ConnRow
	measured string // "about 3 KB/s", or "" when nothing has been measured
	cursor   int
	choice   ConnChoice
	done     bool
	width    int
}

// NewConnModel builds the picker. The cursor starts on the ACTIVE profile, not
// the recommended one: Enter must always mean "keep what I have" for someone who
// opened this by accident.
func NewConnModel(rows []ConnRow, measured string) *connModel {
	m := &connModel{rows: rows, measured: measured, width: 80}
	for i, r := range rows {
		if r.Active {
			m.cursor = i
		}
	}
	return m
}

func (m *connModel) Init() tea.Cmd { return nil }

func (m *connModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.choice = ConnChoice{Quit: true}
			m.done = true
			return m, tea.Quit
		case "esc":
			m.choice = ConnChoice{Keep: true}
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "enter":
			m.choice = ConnChoice{ID: m.rows[m.cursor].ID}
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *connModel) contentWidth() int {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	if w > 100 {
		w = 100
	}
	return w
}

func (m *connModel) View() string {
	if m.done {
		return ""
	}
	w := m.contentWidth()
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(bold.Render("How fast is your connection?") + "\n")

	// The measurement, hedged on purpose. One sample on a high-latency link
	// under-reports, so this is a floor and must read like one. A confident
	// wrong number is worse than an honest vague one.
	if m.measured != "" {
		b.WriteString(dim.Render("Measured "+m.measured+
			" from traffic already sent - nothing extra was downloaded to find out. "+
			"It is a lower bound; your link may be faster.") + "\n")
	} else {
		b.WriteString(dim.Render("Nothing measured yet, so there is no suggestion below. "+
			"Pick whichever line sounds like your connection.") + "\n")
	}
	b.WriteString("\n")

	for i, r := range m.rows {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		label := fmt.Sprintf("%-22s %s", r.Name, r.Rate)
		line := marker + label
		if i == m.cursor {
			line = bold.Render(line)
		}
		var tags []string
		if r.Recommended {
			tags = append(tags, "suggested for your speed")
		}
		if r.Active {
			tags = append(tags, "current")
		}
		if len(tags) > 0 {
			line += dim.Render("  (" + strings.Join(tags, ", ") + ")")
		}
		b.WriteString(line + "\n")
	}

	sel := m.rows[m.cursor]
	b.WriteString("\n")
	b.WriteString(wrapPlain(sel.Links, w) + "\n\n")
	b.WriteString(wrapPlain(sel.What, w) + "\n\n")
	b.WriteString(dim.Render(wrapPlain(sel.Effect, w)) + "\n")
	if sel.Advice != "" {
		b.WriteString("\n" + wrapPlain(sel.Advice, w) + "\n")
	}
	b.WriteString("\n" + dim.Render("up/down move | enter choose | esc keep current | ctrl+c quit") + "\n")
	return b.String()
}

// wrapPlain wraps on spaces at display width. lipgloss.Width is used rather than
// len because a byte count is not a column count.
func wrapPlain(s string, w int) string {
	if s == "" {
		return ""
	}
	words := strings.Fields(s)
	var out, line strings.Builder
	for _, word := range words {
		probe := word
		if line.Len() > 0 {
			probe = " " + word
		}
		if lipgloss.Width(line.String()+probe) > w && line.Len() > 0 {
			out.WriteString(line.String() + "\n")
			line.Reset()
			line.WriteString(word)
			continue
		}
		line.WriteString(probe)
	}
	if line.Len() > 0 {
		out.WriteString(line.String())
	}
	return out.String()
}

// AskConnection runs the picker. A failure to run must never block the launch —
// same contract as the provider portal.
func AskConnection(rows []ConnRow, measured string) (ConnChoice, error) {
	if len(rows) == 0 {
		return ConnChoice{Keep: true}, nil
	}
	m := NewConnModel(rows, measured)
	// WithAltScreen is not optional here: an inline prompt leaves one stale
	// frame behind on every terminal resize. Enforced by
	// startup/altscreen_test.go, which caught this exact omission.
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return ConnChoice{Keep: true}, err
	}
	if fm, ok := final.(*connModel); ok {
		return fm.choice, nil
	}
	return ConnChoice{Keep: true}, nil
}
