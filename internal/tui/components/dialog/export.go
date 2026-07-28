package dialog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// GORILLA OVERRIDE: /export used to write to a hardcoded path under the working
// directory with a generated filename, and told you where it went afterwards. You
// could not choose the folder and you could not name the file, so saving a
// session somewhere deliberate meant exporting and then moving it by hand.

// CloseExportDialogMsg closes the dialog.
type CloseExportDialogMsg struct{}

// ExportConfirmedMsg carries the chosen destination back to the TUI, which owns
// the session data and does the writing.
type ExportConfirmedMsg struct {
	Dir  string
	Name string
}

type ExportDialog interface {
	tea.Model
	layout.Bindings
	// SetDefaults seeds the fields before the dialog opens: the directory to
	// offer and a suggested filename derived from the session.
	SetDefaults(dir, name string)
}

const exportDialogWidth = 72

type exportKeyMap struct {
	Submit, Cancel, NextField, PrevField key.Binding
}

var exportKeys = exportKeyMap{
	Submit:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "export")),
	Cancel:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	NextField: key.NewBinding(key.WithKeys("tab", "down"), key.WithHelp("tab", "next field")),
	PrevField: key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("shift+tab", "previous")),
}

type exportDialogCmp struct {
	width, height int
	inputs        []textinput.Model
	focusIdx      int
	err           string
}

func NewExportDialogCmp() ExportDialog {
	m := &exportDialogCmp{}
	dir := textinput.New()
	dir.CharLimit = 400
	dir.Width = exportDialogWidth - 4
	name := textinput.New()
	name.CharLimit = 200
	name.Width = exportDialogWidth - 4
	applyInputTheme(&dir)
	applyInputTheme(&name)
	m.inputs = []textinput.Model{dir, name}
	return m
}

func (m *exportDialogCmp) SetDefaults(dir, name string) {
	m.inputs[0].SetValue(dir)
	m.inputs[1].SetValue(name)
}

func (m *exportDialogCmp) Init() tea.Cmd {
	m.err = ""
	m.focusIdx = 0
	m.inputs[1].Blur()
	m.inputs[0].Focus()
	// Put the cursor at the end so a long path is not scrolled to its start.
	m.inputs[0].CursorEnd()
	m.inputs[1].CursorEnd()
	return nil
}

func (m *exportDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, exportKeys.Cancel):
			return m, util.CmdHandler(CloseExportDialogMsg{})
		case key.Matches(msg, exportKeys.NextField):
			m.focus(1)
			return m, nil
		case key.Matches(msg, exportKeys.PrevField):
			m.focus(-1)
			return m, nil
		case key.Matches(msg, exportKeys.Submit):
			return m, m.submit()
		}
		var cmd tea.Cmd
		m.inputs[m.focusIdx], cmd = m.inputs[m.focusIdx].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *exportDialogCmp) focus(delta int) {
	m.inputs[m.focusIdx].Blur()
	m.focusIdx = (m.focusIdx + delta + len(m.inputs)) % len(m.inputs)
	m.inputs[m.focusIdx].Focus()
	m.inputs[m.focusIdx].CursorEnd()
}

// submit validates the destination and hands it back. Validation happens here
// rather than after the dialog closes so a bad path can be corrected in place
// instead of losing the export and the typing with it.
func (m *exportDialogCmp) submit() tea.Cmd {
	dir, err := ResolveExportDir(m.inputs[0].Value())
	if err != nil {
		m.err = err.Error()
		m.focusIdx = 0
		m.inputs[1].Blur()
		m.inputs[0].Focus()
		return nil
	}
	name, err := SanitiseExportName(m.inputs[1].Value())
	if err != nil {
		m.err = err.Error()
		m.focusIdx = 1
		m.inputs[0].Blur()
		m.inputs[1].Focus()
		return nil
	}
	return tea.Batch(
		util.CmdHandler(CloseExportDialogMsg{}),
		util.CmdHandler(ExportConfirmedMsg{Dir: dir, Name: name}),
	)
}

// ResolveExportDir expands and checks a typed or pasted destination folder.
// Nothing here went through a shell, so `~` is still literal.
func ResolveExportDir(raw string) (string, error) {
	dir := strings.TrimSpace(raw)
	dir = strings.Trim(dir, "\"'") // pasted paths often arrive quoted
	if dir == "" {
		return "", fmt.Errorf("choose a folder to export into")
	}
	if dir == "~" || strings.HasPrefix(dir, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %v", err)
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("not a usable path: %v", err)
	}
	info, err := os.Stat(abs)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("%s does not exist", abs)
	}
	if err != nil {
		return "", fmt.Errorf("cannot use %s: %v", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is a file, not a folder", abs)
	}
	return abs, nil
}

// SanitiseExportName keeps a typed filename usable without silently rewriting
// what was asked for. It rejects path separators rather than stripping them: a
// name containing a slash means the folder field was misunderstood, and quietly
// flattening it would put the file somewhere the user did not choose.
func SanitiseExportName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	name = strings.Trim(name, "\"'")
	if name == "" {
		return "", fmt.Errorf("give the file a name")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("name cannot contain / or \\ — put the folder in the field above")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("%q is not a filename", name)
	}
	// Default the extension, but never override a deliberate one.
	if filepath.Ext(name) == "" {
		name += ".md"
	}
	return name, nil
}

func (m *exportDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := exportDialogWidth

	// Every line rendered at full width: lipgloss does not pad the short lines of
	// a multi-line render, and unpainted cells show as black bars.
	lines := []string{
		base.Foreground(t.Primary()).Bold(true).Width(w).Render("Export session"),
		base.Foreground(t.TextMuted()).Width(w).Render("Full transcript: timestamps, reasoning, tool calls and results."),
		base.Width(w).Render(""),
	}

	labels := []string{"Folder", "Filename"}
	for i, in := range m.inputs {
		lbl := "  " + labels[i]
		if i == m.focusIdx {
			lbl = "▸ " + labels[i]
		}
		lines = append(lines,
			base.Foreground(t.TextMuted()).Width(w).Render(lbl+":"),
			base.Width(w).Render(in.View()),
		)
	}

	if m.err != "" {
		lines = append(lines,
			base.Width(w).Render(""),
			base.Foreground(t.Error()).Width(w).Render("⚠ "+truncateTo(m.err, w-2)),
		)
	}
	lines = append(lines,
		base.Width(w).Render(""),
		base.Foreground(t.TextMuted()).Width(w).Render("tab: next field   enter: export   esc: cancel"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func (m *exportDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(exportKeys)
}
