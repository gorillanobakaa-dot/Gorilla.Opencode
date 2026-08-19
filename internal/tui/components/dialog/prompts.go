// GORILLA OVERRIDE: this file did not exist upstream. It is the /prompts dialog —
// view, edit and reset the system prompts, and drill into the coder prompt's
// individual sections.
//
// Editing hands off to $EDITOR rather than building a text editor in the TUI. A
// homegrown editor would be worse than vim or nano at every level, and the
// prompts are multi-thousand-character documents.
//
// Honest scoping, stated here and on screen: the whole coder prompt is ~460
// tokens across 8 sections. Turning a section off saves tens of tokens, not
// thousands. The value is BEHAVIOURAL control — drop "# build discipline" on a
// non-build task — not bandwidth.
package dialog

import (
	"fmt"
	"os"
	"os/exec"
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

// ClosePromptsDialogMsg closes the dialog.
type ClosePromptsDialogMsg struct{}

// PromptsChangedMsg is emitted after a prompt is edited or reset, so the TUI can
// rebuild the provider and the model sees the new prompt on the next turn.
type PromptsChangedMsg struct{ Info string }

// promptEditedMsg carries the result of the $EDITOR handoff back into the TUI.
type promptEditedMsg struct {
	id      prompt.PromptID
	tmpPath string
	err     error
}

type PromptsDialog interface {
	tea.Model
	layout.Bindings
}

type promptsView int

const (
	promptsViewList     promptsView = iota // the four prompts
	promptsViewSections                    // sections of the coder prompt
)

const promptsDialogWidth = 92

type promptsKeyMap struct {
	Up, Down, Enter, Edit, Reset, Toggle, Back, Escape key.Binding
}

var promptsKeys = promptsKeyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down", "down")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "sections")),
	Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit in $EDITOR")),
	Reset:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset to default")),
	Toggle: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle section")),
	Back:   key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "back")),
	Escape: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
}

type promptsDialogCmp struct {
	view        promptsView
	selectedIdx int
	sectionIdx  int
	width       int
	height      int
	status      string
}

func NewPromptsDialogCmp() PromptsDialog { return &promptsDialogCmp{} }

func (m *promptsDialogCmp) Init() tea.Cmd {
	m.view = promptsViewList
	m.selectedIdx = 0
	m.status = ""
	return nil
}

func (m *promptsDialogCmp) width_() int {
	// GORILLA OVERRIDE: full terminal width minus this dialog's own chrome, with
	// NO upper cap — matching /context (loadout.go). An earlier version capped the
	// width, which truncated the explanatory text with an ellipsis on a wide
	// terminal and left an unused black margin. The whole point of these rows is
	// that the description is readable.
	//
	// chrome is this dialog's border (1+1) plus padding (2+2). SUBTRACTED from the
	// terminal, never added on top — a wrapper is never free, and adding it made
	// the dialog 82 columns wide in an 80-column terminal.
	//
	// The floor is deliberately small: it only applies on a terminal too narrow to
	// hold the dialog at all, where cramped beats overflowing.
	const (
		chrome   = 6
		minWidth = 30
	)
	if m.width <= 0 {
		return promptsDialogWidth
	}
	if w := m.width - chrome; w > minWidth {
		return w
	}
	return minWidth
}

func (m *promptsDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case promptEditedMsg:
		return m, m.finishEdit(msg)

	case tea.KeyMsg:
		if m.view == promptsViewSections {
			return m.updateSections(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m *promptsDialogCmp) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ids := prompt.AllPromptIDs
	switch {
	case key.Matches(msg, promptsKeys.Up):
		if m.selectedIdx > 0 {
			m.selectedIdx--
		} else {
			m.selectedIdx = len(ids) - 1
		}
	case key.Matches(msg, promptsKeys.Down):
		if m.selectedIdx < len(ids)-1 {
			m.selectedIdx++
		} else {
			m.selectedIdx = 0
		}
	case key.Matches(msg, promptsKeys.Escape):
		return m, util.CmdHandler(ClosePromptsDialogMsg{})

	case key.Matches(msg, promptsKeys.Enter):
		// Only the coder prompt is section-split; the others are short enough
		// that per-section control would be noise.
		if ids[m.selectedIdx] == prompt.PromptCoder {
			m.view = promptsViewSections
			m.sectionIdx = 0
		} else {
			m.status = "only the coder prompt is split into sections"
		}

	case key.Matches(msg, promptsKeys.Edit):
		return m, m.startEdit(ids[m.selectedIdx])

	case key.Matches(msg, promptsKeys.Reset):
		id := ids[m.selectedIdx]
		if !prompt.IsOverridden(id) {
			m.status = fmt.Sprintf("%s is already the shipped default", id)
			return m, nil
		}
		if err := prompt.ResetPrompt(id); err != nil {
			m.status = "reset failed: " + err.Error()
			return m, nil
		}
		return m, util.CmdHandler(PromptsChangedMsg{
			Info: fmt.Sprintf("%s prompt reset to the shipped default", id),
		})
	}
	return m, nil
}

func (m *promptsDialogCmp) updateSections(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	secs := prompt.CoderSections()
	switch {
	case key.Matches(msg, promptsKeys.Up):
		if m.sectionIdx > 0 {
			m.sectionIdx--
		} else {
			m.sectionIdx = len(secs) - 1
		}
	case key.Matches(msg, promptsKeys.Down):
		if m.sectionIdx < len(secs)-1 {
			m.sectionIdx++
		} else {
			m.sectionIdx = 0
		}
	case key.Matches(msg, promptsKeys.Back), key.Matches(msg, promptsKeys.Escape):
		m.view = promptsViewList
		m.status = ""
		return m, nil
	case key.Matches(msg, promptsKeys.Toggle):
		if len(secs) == 0 {
			return m, nil
		}
		s := secs[m.sectionIdx]
		config.ToggleLoadout(s.ID)
		state := "off"
		if config.LoadoutEnabled(s.ID) {
			state = "on"
		}
		label := s.Header
		if label == "" {
			label = "preamble"
		}
		return m, util.CmdHandler(PromptsChangedMsg{
			Info: fmt.Sprintf("prompt section %q %s", label, state),
		})
	}
	return m, nil
}

// startEdit writes the ACTIVE prompt text to a temp file and hands the terminal
// to $EDITOR. Seeding with the active text (override if present, else factory)
// means the user edits what is actually being sent, not a blank file.
func (m *promptsDialogCmp) startEdit(id prompt.PromptID) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// nano is present on Debian and forgiving for someone who did not
		// choose an editor; vi would trap a user who does not know it.
		editor = "nano"
	}

	tmp, err := os.CreateTemp("", fmt.Sprintf("gorilla-prompt-%s-*.txt", id))
	if err != nil {
		return util.ReportError(err)
	}
	if _, err := tmp.WriteString(prompt.Text(id)); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return util.ReportError(err)
	}
	tmp.Close()

	c := exec.Command(editor, tmp.Name()) //nolint:gosec
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return promptEditedMsg{id: id, tmpPath: tmp.Name(), err: err}
	})
}

func (m *promptsDialogCmp) finishEdit(msg promptEditedMsg) tea.Cmd {
	defer os.Remove(msg.tmpPath)

	if msg.err != nil {
		return util.ReportError(msg.err)
	}
	content, err := os.ReadFile(msg.tmpPath)
	if err != nil {
		return util.ReportError(err)
	}
	edited := string(content)

	// Unchanged? Say so rather than writing an override identical to the
	// factory copy, which would show as EDITED forever.
	if strings.TrimSpace(edited) == strings.TrimSpace(prompt.Text(msg.id)) {
		return util.ReportInfo("prompt unchanged")
	}
	if err := prompt.SaveOverride(msg.id, edited); err != nil {
		// Covers the blank-prompt refusal, which carries its own explanation.
		return util.ReportWarn(err.Error())
	}
	return util.CmdHandler(PromptsChangedMsg{
		Info: fmt.Sprintf("%s prompt saved — takes effect on the next turn", msg.id),
	})
}

func (m *promptsDialogCmp) View() string {
	if m.view == promptsViewSections {
		return m.sectionsView()
	}
	return m.listView()
}

func (m *promptsDialogCmp) listView() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := m.width_()

	rows := []string{
		base.Foreground(t.Primary()).Bold(true).Width(w).
			Render("System prompts — what the AI is told before it sees your message"),
		base.Foreground(t.TextMuted()).Width(w).
			Render("EDITED means you changed it. r restores the copy compiled into the binary,"),
		base.Foreground(t.TextMuted()).Width(w).
			Render("which no edit of yours can corrupt — so reset is always trustworthy."),
		base.Width(w).Render(""),
	}

	for i, id := range prompt.AllPromptIDs {
		selected := i == m.selectedIdx
		text := prompt.Text(id)
		tag := ""
		if prompt.IsOverridden(id) {
			tag = "  EDITED"
		}
		extra := ""
		if id == prompt.PromptCoder {
			secs := prompt.CoderSections()
			on := 0
			for _, s := range secs {
				if config.LoadoutEnabled(s.ID) {
					on++
				}
			}
			extra = fmt.Sprintf("  |  %d/%d sections on (enter)", on, len(secs))
		}

		style := base.Width(w)
		if selected {
			style = style.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		}
		rows = append(rows, style.Render(fmt.Sprintf("  %-52s ~%4d tok%s%s",
			prompt.PromptDisplayName[id], len(text)/4, tag, extra)))
	}

	if m.status != "" {
		rows = append(rows, base.Width(w).Render(""),
			base.Width(w).Foreground(t.Warning()).Render("  "+m.status))
	}

	rows = append(rows,
		base.Width(w).Render(""),
		base.Foreground(t.TextMuted()).Width(w).
			Render("enter sections   e edit in $EDITOR   r reset   esc close"),
	)

	return base.Padding(1, 2).Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m *promptsDialogCmp) sectionsView() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := m.width_()
	secs := prompt.CoderSections()

	total, on := 0, 0
	for _, s := range secs {
		total += s.Tokens
		if config.LoadoutEnabled(s.ID) {
			on += s.Tokens
		}
	}

	rows := []string{
		base.Foreground(t.Primary()).Bold(true).Width(w).
			Render("Coder prompt sections — switch any part off"),
		base.Foreground(t.TextMuted()).Width(w).
			Render(fmt.Sprintf("~%d of ~%d tokens active. This is BEHAVIOURAL control, not bandwidth —", on, total)),
		base.Foreground(t.TextMuted()).Width(w).
			Render("the whole prompt is small. Turn a section off to change how the AI acts."),
		base.Width(w).Render(""),
	}

	for i, s := range secs {
		selected := i == m.sectionIdx
		enabled := config.LoadoutEnabled(s.ID)
		box := "[ ]"
		if enabled {
			box = "[x]"
		}
		label := s.Header
		if label == "" {
			label = "preamble (identity)"
		}
		warn := ""
		if s.ID == prompt.SectionID("honesty") || s.ID == prompt.SectionID("preamble") {
			warn = " ^"
		}

		style := base.Width(w)
		switch {
		case selected:
			style = style.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		case !enabled:
			style = style.Foreground(t.TextMuted())
		}
		rows = append(rows, style.Render(fmt.Sprintf("  %s %-22s ~%3d  %s%s",
			box, label, s.Tokens, prompt.SectionTradeoff[s.ID], warn)))
	}

	rows = append(rows,
		base.Width(w).Render(""),
		base.Foreground(t.TextMuted()).Width(w).
			Render("space toggle   backspace back   esc close   ^ = disabling this hurts trustworthiness"),
	)

	return base.Padding(1, 2).Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m *promptsDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(promptsKeys)
}
