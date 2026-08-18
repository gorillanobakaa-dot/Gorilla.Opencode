// GORILLA OVERRIDE (2026-08-18): the /osint --recover picker.
//
// It exists because the flag was documented before it was built. The salvage
// path told the user "run /osint --recover" in the findings file it wrote and in
// the tool's own report — and typing it sent the literal string "--recover" to a
// ten-helper supervised dossier as the question under investigation. The model
// caught it and refused ("I cannot fabricate a dossier about '--recover' — that's
// a flag, not a question"), which is the right behaviour and is not a substitute
// for the command existing.
//
// The list is deliberately dull: what was asked, when, how many lanes reported,
// what it cost. Someone reaching for this has already lost two hours of work
// once; the screen's job is to show them it is still there.
package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// CloseOsintRecoverMsg carries the chosen run back. Chosen false = cancelled.
type CloseOsintRecoverMsg struct {
	Chosen bool
	Run    agent.RecoverableRun
}

// osintRecoverMaxRows is the CEILING on how many runs are shown at once. The
// real number is measured against the terminal — see visibleRows. The list
// scrolls either way; this is only how much of it is on screen.
const osintRecoverMaxRows = 8

// visibleRows is how many runs fit, in a frame that must never be taller than
// the window.
//
// GORILLA FIX (2026-08-18): the first version drew eight runs unconditionally
// at two lines each and came out 32 rows tall on an 80x24 terminal — caught by
// TestDialogFramesNeverExceedTheTerminal before it was ever seen on a screen. A
// frame taller than the window scrolls the terminal and destroys the layout,
// the same defect the /osint gate was fixed for the day before.
func (m OsintRecoverCmp) visibleRows() int {
	if m.height <= 0 {
		return osintRecoverMaxRows
	}
	// Chrome: border and padding (4), title and its blank (2), the intro and
	// its blank (3), the closing note, its blanks and the key line (5), and one
	// row of slack so the frame never sits flush against the window edge.
	rows := (m.height - 15) / 2
	if rows > osintRecoverMaxRows {
		rows = osintRecoverMaxRows
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

// showIntro reports whether there is room for the explanatory prose. On a
// cramped window the LIST is what cannot be dropped — someone who opened this
// screen has already been told what it does.
func (m OsintRecoverCmp) showIntro() bool { return m.height <= 0 || m.height >= 22 }

type OsintRecoverCmp struct {
	runs     []agent.RecoverableRun
	selected int
	offset   int
	width    int
	height   int
}

func NewOsintRecoverCmp(runs []agent.RecoverableRun) OsintRecoverCmp {
	return OsintRecoverCmp{runs: runs}
}

func (m *OsintRecoverCmp) SetSize(w, h int) { m.width, m.height = w, h }

func (m OsintRecoverCmp) Init() tea.Cmd { return nil }

func (m OsintRecoverCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.selected < len(m.runs)-1 {
				m.selected++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if len(m.runs) == 0 {
				return m, util.CmdHandler(CloseOsintRecoverMsg{Chosen: false})
			}
			return m, util.CmdHandler(CloseOsintRecoverMsg{Chosen: true, Run: m.runs[m.selected]})
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			return m, util.CmdHandler(CloseOsintRecoverMsg{Chosen: false})
		}
		// Keep the highlighted row on screen.
		if m.selected < m.offset {
			m.offset = m.selected
		}
		if rows := m.visibleRows(); m.selected >= m.offset+rows {
			m.offset = m.selected - rows + 1
		}
	}
	return m, nil
}

func (m OsintRecoverCmp) View() string {
	t := theme.CurrentTheme()
	// Chrome: the rounded border (2) plus the padding this dialog sets (2 each
	// side). Capped, never floored — a frame wider than the terminal is what
	// strands rows in the scrollback.
	width := dialogWidth(m.width, 84, 6)

	// PanelBackground, not the theme background directly: outside the alternate
	// screen it emits no escape at all, so the dialog does not stay painted over
	// the terminal when nothing else is. Enforced by
	// TestNoComponentFillsWithTheThemeBackgroundDirectly.
	base := styles.BaseStyle().Background(styles.PanelBackground())
	title := base.Foreground(t.Primary()).Bold(true).Width(width).Render("Recover a research run")

	var lines []string
	lines = append(lines, title, "")

	if len(m.runs) == 0 {
		lines = append(lines,
			base.Foreground(t.Text()).Width(width).Render(fitLine("Nothing to recover — no past run was found.", width)),
			"",
			base.Foreground(t.TextMuted()).Width(width).Render(
				fitLine("Findings are saved automatically when a run's lanes report, and every", width)),
			base.Foreground(t.TextMuted()).Width(width).Render(
				fitLine("helper session is kept in the local store. If this list is empty, no", width)),
			base.Foreground(t.TextMuted()).Width(width).Render(
				fitLine("research has been run on this machine yet.", width)),
			"",
			base.Foreground(t.TextMuted()).Width(width).Render("esc  close"),
		)
		return base.Padding(1, 2).Border(lipgloss.RoundedBorder()).
			BorderBackground(styles.PanelBackground()).BorderForeground(t.TextMuted()).
			Width(width + 4).
			Render(strings.Join(lines, "\n"))
	}

	if m.showIntro() {
		lines = append(lines,
			base.Foreground(t.TextMuted()).Width(width).Render(
				fitLine("These runs collected findings. Choose one and it will be written up as a", width)),
			base.Foreground(t.TextMuted()).Width(width).Render(
				fitLine("dossier — no searching, no helpers, nothing collected again.", width)),
			"",
		)
	}

	end := m.offset + m.visibleRows()
	if end > len(m.runs) {
		end = len(m.runs)
	}
	for i := m.offset; i < end; i++ {
		run := m.runs[i]
		label := fitLine(run.Label(), width-4)
		detail := fitLine(run.Detail(), width-4)
		if i == m.selected {
			lines = append(lines,
				base.Foreground(t.Background()).Background(t.Primary()).Bold(true).
					Width(width).Render("  "+label+"  "),
				base.Foreground(t.Text()).Width(width).Render("    "+detail),
			)
		} else {
			lines = append(lines,
				base.Foreground(t.Text()).Width(width).Render("  "+label),
				base.Foreground(t.TextMuted()).Width(width).Render("    "+detail),
			)
		}
	}
	if len(m.runs) > m.visibleRows() {
		lines = append(lines, "", base.Foreground(t.TextMuted()).Width(width).
			Render(fmt.Sprintf("showing %d–%d of %d", m.offset+1, end, len(m.runs))))
	}

	if m.showIntro() {
		lines = append(lines,
			"",
			base.Foreground(t.TextMuted()).Width(width).Render(
				fitLine("The write-up starts a FRESH conversation carrying only these findings —", width)),
			base.Foreground(t.TextMuted()).Width(width).Render(
				fitLine("that is the whole reason it works where the original run ran out of room.", width)),
		)
	}
	lines = append(lines,
		"",
		base.Foreground(t.TextMuted()).Width(width).Render(
			fitLine("↑↓ choose   enter  write it up   esc  cancel", width)),
	)

	return base.Padding(1, 2).Border(lipgloss.RoundedBorder()).
		BorderBackground(styles.PanelBackground()).BorderForeground(t.TextMuted()).
		Width(width + 4).
		Render(strings.Join(lines, "\n"))
}

func (m OsintRecoverCmp) Bindings() []key.Binding { return nil }

// fitLine truncates rather than wraps. lipgloss WRAPS a string wider than its
// Width, so an untruncated line silently becomes two rows — the same defect
// that made /context grow taller exactly where there was least room.
//
// GORILLA FIX (2026-08-18): measured in DISPLAY COLUMNS, not runes. The first
// version counted runes, and a real session title in the store contains U+FFFC
// (object replacement character), which occupies two columns and counts as one
// rune — so that row rendered two columns wider than the frame and wrapped,
// stranding half of it. Rune count is not width for anything a user can paste
// into a prompt: CJK, emoji and replacement characters are all double-width.
// ansi.Truncate does the same arithmetic the terminal does.
func fitLine(line string, width int) string {
	if width < 4 {
		return ""
	}
	if lipgloss.Width(line) <= width-1 {
		return line
	}
	return ansi.Truncate(line, width-2, "…")
}
