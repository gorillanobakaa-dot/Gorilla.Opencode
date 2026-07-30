package chat

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type editorCmp struct {
	width       int
	height      int
	app         *app.App
	session     session.Session
	textarea    textarea.Model
	attachments []message.Attachment
	deleteMode  bool
	// lastReportedHeight is the desired height we last told the layout about,
	// so we only emit EditorHeightMsg when it actually changes.
	lastReportedHeight int

	// GORILLA OVERRIDE: input history, recalled with Up/Down like any shell.
	//
	// history holds what you have SENT, oldest first. historyPos indexes it
	// while browsing: len(history) means "not browsing". draft keeps whatever
	// was half-typed when browsing started, so arrowing back down returns it
	// instead of throwing it away.
	history    []string
	historyPos int
	draft      string
}

// recallHistory replaces the input with an earlier (or later) message.
//
// Up only recalls when the cursor is on the FIRST line, so it still moves the
// cursor inside a multi-line message — which is what makes it feel native
// rather than hijacked. Down leaves history at the bottom and restores the
// draft, so browsing is never destructive.
//
// Returns false when the key should be handled by the textarea instead.
func (m *editorCmp) recallHistory(back bool) bool {
	if len(m.history) == 0 {
		return false
	}
	browsing := m.historyPos < len(m.history)

	if back {
		// Only take over Up when there is no line above to move to.
		if !browsing && m.textarea.Line() > 0 {
			return false
		}
		if browsing && m.historyPos == 0 {
			return true // already at the oldest; swallow rather than escape
		}
		if !browsing {
			m.draft = m.textarea.Value()
			m.historyPos = len(m.history)
		}
		m.historyPos--
	} else {
		if !browsing {
			return false
		}
		m.historyPos++
		if m.historyPos >= len(m.history) {
			// Past the newest: give the half-typed line back.
			m.historyPos = len(m.history)
			m.textarea.SetValue(m.draft)
			return true
		}
	}
	// SetValue resets then inserts, which leaves the cursor at the end — so the
	// recalled line is ready to edit or send, not to overtype from column 0.
	m.textarea.SetValue(m.history[m.historyPos])
	return true
}

// rememberSent pushes a sent message onto the history and stops browsing.
// Consecutive duplicates are collapsed, as a shell does.
func (m *editorCmp) rememberSent(value string) {
	value = strings.TrimRight(value, "\n")
	if strings.TrimSpace(value) != "" &&
		(len(m.history) == 0 || m.history[len(m.history)-1] != value) {
		m.history = append(m.history, value)
	}
	m.historyPos = len(m.history)
	m.draft = ""
}

// GORILLA OVERRIDE: the input box grows with what you type (like every modern
// chat input) instead of being pinned to a fixed slice of the window.
const (
	minEditorHeight = 1 // one row when empty
	// maxEditorHeight is how tall the input may grow before it starts scrolling
	// internally.
	//
	// GORILLA OVERRIDE: raised from 12. Measured rather than picked: at a typical
	// 96-column input, English prose averages a shade under 6 columns per word
	// including its space, so 20 rows holds roughly 320 words — comfortably past the
	// 300 asked for. The footer's own budget clamps this further on short windows,
	// which is correct: it is a ceiling, not a demand.
	maxEditorHeight = 20
	// editorBufferLines is how many LOGICAL lines the input will accept.
	//
	// bubbles' textarea defaults MaxHeight to 99 and then refuses to add lines past
	// it (textarea.go:1028), so a long pasted prompt was silently truncated at 99
	// newlines regardless of CharLimit being -1. 300 lines holds ~2000 words even if
	// every line is short.
	editorBufferLines = 300
)

// overflowArrow marks lines scrolled out of sight above the input.
//
// U+25B2 rather than a heavier arrow like U+2B06: the latter is rendered as an
// emoji by most fonts, which makes it double-width, and a glyph whose width the
// renderer guesses wrong shifts everything after it.
const overflowArrow = "▲"

// EditorHeightMsg asks the layout to give the editor exactly Height rows.
type EditorHeightMsg struct{ Height int }

// wrappedRows reports how many terminal rows `value` occupies once soft-wrapped
// at `width`. bubbles' textarea has no exported "total visual rows" (LineCount()
// counts LOGICAL lines only, and LineInfo() covers just the cursor's line), so
// measure it with lipgloss — which word-wraps the same way and can count the
// result. This is the "reference dimensions dynamically" rule, not a guess.
func wrappedRows(value string, width int) int {
	if width <= 0 {
		return minEditorHeight
	}
	if value == "" {
		return minEditorHeight
	}
	return lipgloss.Height(lipgloss.NewStyle().Width(width).Render(value))
}

// desiredHeight is the row count the editor wants for its current content,
// clamped so it never collapses or eats the whole window.
func (m *editorCmp) desiredHeight() int {
	rows := wrappedRows(m.textarea.Value(), m.textarea.Width())
	if len(m.attachments) > 0 {
		rows++ // the attachments line sits above the input
	}
	return max(minEditorHeight, min(maxEditorHeight, rows))
}

type EditorKeyMaps struct {
	Send       key.Binding
	OpenEditor key.Binding
}

type bluredEditorKeyMaps struct {
	Send       key.Binding
	Focus      key.Binding
	OpenEditor key.Binding
}
type DeleteAttachmentKeyMaps struct {
	AttachmentDeleteMode key.Binding
	Escape               key.Binding
	DeleteAllAttachments key.Binding
}

var editorMaps = EditorKeyMaps{
	Send: key.NewBinding(
		key.WithKeys("enter", "ctrl+s"),
		key.WithHelp("enter", "send message"),
	),
	OpenEditor: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "open editor"),
	),
}

var DeleteKeyMaps = DeleteAttachmentKeyMaps{
	AttachmentDeleteMode: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r+{i}", "delete attachment at index i"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel delete mode"),
	),
	DeleteAllAttachments: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("ctrl+r+r", "delete all attchments"),
	),
}

const (
	maxAttachments = 5
)

func (m *editorCmp) openEditor() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	tmpfile, err := os.CreateTemp("", "msg_*.md")
	if err != nil {
		return util.ReportError(err)
	}
	tmpfile.Close()
	c := exec.Command(editor, tmpfile.Name()) //nolint:gosec
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return util.ReportError(err)
		}
		content, err := os.ReadFile(tmpfile.Name())
		if err != nil {
			return util.ReportError(err)
		}
		if len(content) == 0 {
			return util.ReportWarn("Message is empty")
		}
		os.Remove(tmpfile.Name())
		attachments := m.attachments
		m.attachments = nil
		return SendMsg{
			Text:        string(content),
			Attachments: attachments,
		}
	})
}

func (m *editorCmp) Init() tea.Cmd {
	return textarea.Blink
}

func (m *editorCmp) send() tea.Cmd {
	if m.app.CoderAgent.IsSessionBusy(m.session.ID) {
		return util.ReportWarn("Agent is working, please wait...")
	}

	value := m.textarea.Value()
	m.textarea.Reset()
	attachments := m.attachments

	m.attachments = nil
	if value == "" {
		return nil
	}
	m.rememberSent(value)
	// GORILLA OVERRIDE: slash commands. A message that is exactly a
	// known slash command (e.g. "/model", "/models", "/export") is
	// intercepted and dispatched instead of being sent to the model —
	// the ergonomics users expect from the current OpenCode.
	if cmd := strings.TrimSpace(value); strings.HasPrefix(cmd, "/") {
		// GORILLA OVERRIDE: split the command word from its arguments, so
		// `/cd ~/src/linux` reaches the handler with the path intact. Previously
		// the entire line became the command Name, so anything with an argument
		// fell through to "Unknown command".
		word, args, _ := strings.Cut(strings.TrimPrefix(cmd, "/"), " ")
		return util.CmdHandler(SlashCommandMsg{
			Name: strings.ToLower(strings.TrimSpace(word)),
			Args: strings.TrimSpace(args),
		})
	}
	return tea.Batch(
		util.CmdHandler(SendMsg{
			Text:        value,
			Attachments: attachments,
		}),
	)
}

func (m *editorCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case dialog.ThemeChangedMsg:
		m.textarea = CreateTextArea(&m.textarea)
	case dialog.CompletionSelectedMsg:
		existingValue := m.textarea.Value()
		modifiedValue := strings.Replace(existingValue, msg.SearchString, msg.CompletionValue, 1)

		m.textarea.SetValue(modifiedValue)
		return m, nil
	case SessionSelectedMsg:
		if msg.ID != m.session.ID {
			m.session = msg
		}
		return m, nil
	case dialog.AttachmentAddedMsg:
		if len(m.attachments) >= maxAttachments {
			logging.ErrorPersist(fmt.Sprintf("cannot add more than %d images", maxAttachments))
			return m, cmd
		}
		m.attachments = append(m.attachments, msg.Attachment)
	case tea.KeyMsg:
		if key.Matches(msg, DeleteKeyMaps.AttachmentDeleteMode) {
			m.deleteMode = true
			return m, nil
		}
		if key.Matches(msg, DeleteKeyMaps.DeleteAllAttachments) && m.deleteMode {
			m.deleteMode = false
			m.attachments = nil
			return m, nil
		}
		if m.deleteMode && len(msg.Runes) > 0 && unicode.IsDigit(msg.Runes[0]) {
			num := int(msg.Runes[0] - '0')
			m.deleteMode = false
			if num < 10 && len(m.attachments) > num {
				if num == 0 {
					m.attachments = m.attachments[num+1:]
				} else {
					m.attachments = slices.Delete(m.attachments, num, num+1)
				}
				return m, nil
			}
		}
		if key.Matches(msg, messageKeys.PageUp) || key.Matches(msg, messageKeys.PageDown) ||
			key.Matches(msg, messageKeys.HalfPageUp) || key.Matches(msg, messageKeys.HalfPageDown) {
			return m, nil
		}
		if key.Matches(msg, editorMaps.OpenEditor) {
			if m.app.CoderAgent.IsSessionBusy(m.session.ID) {
				return m, util.ReportWarn("Agent is working, please wait...")
			}
			return m, m.openEditor()
		}
		if key.Matches(msg, DeleteKeyMaps.Escape) {
			m.deleteMode = false
			// Leaving history browsing restores what you were typing.
			if m.historyPos < len(m.history) {
				m.historyPos = len(m.history)
				m.textarea.SetValue(m.draft)
			}
			return m, nil
		}
		// History recall. Checked before the textarea so Up/Down can be taken
		// over, but only when there is no line to move to — see recallHistory.
		if m.textarea.Focused() && msg.String() == "up" {
			if m.recallHistory(true) {
				if h := m.desiredHeight(); h != m.lastReportedHeight {
					m.lastReportedHeight = h
					return m, util.CmdHandler(EditorHeightMsg{Height: h})
				}
				return m, nil
			}
		}
		if m.textarea.Focused() && msg.String() == "down" {
			if m.recallHistory(false) {
				if h := m.desiredHeight(); h != m.lastReportedHeight {
					m.lastReportedHeight = h
					return m, util.CmdHandler(EditorHeightMsg{Height: h})
				}
				return m, nil
			}
		}
		// Hanlde Enter key
		if m.textarea.Focused() && key.Matches(msg, editorMaps.Send) {
			value := m.textarea.Value()
			if len(value) > 0 && value[len(value)-1] == '\\' {
				// If the last character is a backslash, remove it and add a newline
				m.textarea.SetValue(value[:len(value)-1] + "\n")
				return m, nil
			} else {
				// Otherwise, send the message. send() resets the textarea, so
				// re-measure afterwards and let the layout shrink the box back
				// down. Recomputing (rather than assuming the minimum) stays
				// correct when send() bails out early, e.g. agent busy.
				sendCmd := m.send()
				if h := m.desiredHeight(); h != m.lastReportedHeight {
					m.lastReportedHeight = h
					return m, tea.Batch(sendCmd, util.CmdHandler(EditorHeightMsg{Height: h}))
				}
				return m, sendCmd
			}
		}

	}
	m.textarea, cmd = m.textarea.Update(msg)

	// GORILLA OVERRIDE: tell the layout when the content's row count changes so
	// the input box grows/shrinks with what's typed. Only fires on an actual
	// change, so ordinary keystrokes don't trigger a resize every time.
	if h := m.desiredHeight(); h != m.lastReportedHeight {
		m.lastReportedHeight = h
		return m, tea.Batch(cmd, util.CmdHandler(EditorHeightMsg{Height: h}))
	}
	return m, cmd
}

// hiddenLines is how many wrapped rows of the current input are scrolled out of
// sight above the visible field, or 0 if all of it fits.
func (m *editorCmp) hiddenLines() int {
	wanted := wrappedRows(m.textarea.Value(), m.textarea.Width())
	if shown := m.textarea.Height(); wanted > shown {
		return wanted - shown
	}
	return 0
}

// overflowNotice is the line shown above the input when part of what you typed has
// scrolled out of view.
//
// It exists because the failure it replaces was silent: with the field one row
// tall, typing past the width scrolled the earlier rows away with no indication
// that anything was there, so a long prompt appeared to be losing words. Saying how
// many lines are hidden turns an apparent bug into a fact about the window.
func (m *editorCmp) overflowNotice() string {
	n := m.hiddenLines()
	if n == 0 {
		return ""
	}
	word := "lines"
	if n == 1 {
		word = "line"
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.CurrentTheme().Primary()).
		Render(fmt.Sprintf(" %s %d more %s", overflowArrow, n, word))
}

func (m *editorCmp) View() string {
	t := theme.CurrentTheme()

	// GORILLA OVERRIDE: size ourselves rather than waiting to be sized.
	//
	// The height used to arrive only via the layout: the editor emitted
	// EditorHeightMsg, the page forwarded it to the split pane, and the pane resized
	// this container on a later frame. Outside the alternate screen the pane is not
	// even rendered, so that round trip is pure indirection — and while it was in
	// flight the field stayed one row tall, showing only the wrapped row under the
	// cursor. That is the reported "it only shows one word".
	//
	// The message is still emitted, because the alternate-screen layout needs it to
	// give the transcript the remaining rows. But the field no longer waits for it.
	if !config.AlternateScreenEnabled() {
		if h := m.desiredHeight(); m.textarea.Height() != h {
			m.textarea.SetHeight(h)
		}
	}

	// Style the prompt with theme colors
	style := lipgloss.NewStyle().
		Padding(0, 0, 0, 1).
		Bold(true).
		Foreground(t.Primary())

	if len(m.attachments) == 0 {
		field := lipgloss.JoinHorizontal(lipgloss.Top, style.Render(">"), m.textarea.View())
		if notice := m.overflowNotice(); notice != "" {
			return lipgloss.JoinVertical(lipgloss.Left, notice, field)
		}
		return field
	}
	m.textarea.SetHeight(m.height - 1)
	return lipgloss.JoinVertical(lipgloss.Top,
		m.attachmentsContent(),
		lipgloss.JoinHorizontal(lipgloss.Top, style.Render(">"),
			m.textarea.View()),
	)
}

func (m *editorCmp) SetSize(width, height int) tea.Cmd {
	m.width = width
	m.height = height
	// GORILLA OVERRIDE: the textarea must leave room for the "> " prompt that
	// View() prepends via JoinHorizontal, otherwise the editor renders WIDER
	// than its allotment — which shoves the sidebar's bottom rows out of the
	// layout (they then get clipped, showing an unpainted gap). A stray
	// SetWidth(width) here used to clobber this width-3 and cause exactly that.
	m.textarea.SetWidth(width - 3) // account for the prompt and padding right
	m.textarea.SetHeight(height)
	return nil
}

func (m *editorCmp) GetSize() (int, int) {
	return m.textarea.Width(), m.textarea.Height()
}

func (m *editorCmp) attachmentsContent() string {
	var styledAttachments []string
	t := theme.CurrentTheme()
	attachmentStyles := styles.BaseStyle().
		MarginLeft(1).
		Background(t.TextMuted()).
		Foreground(t.Text())
	for i, attachment := range m.attachments {
		var filename string
		if len(attachment.FileName) > 10 {
			filename = fmt.Sprintf(" %s %s...", styles.DocumentIcon, attachment.FileName[0:7])
		} else {
			filename = fmt.Sprintf(" %s %s", styles.DocumentIcon, attachment.FileName)
		}
		if m.deleteMode {
			filename = fmt.Sprintf("%d%s", i, filename)
		}
		styledAttachments = append(styledAttachments, attachmentStyles.Render(filename))
	}
	content := lipgloss.JoinHorizontal(lipgloss.Left, styledAttachments...)
	return content
}

func (m *editorCmp) BindingKeys() []key.Binding {
	bindings := []key.Binding{}
	bindings = append(bindings, layout.KeyMapToSlice(editorMaps)...)
	bindings = append(bindings, layout.KeyMapToSlice(DeleteKeyMaps)...)
	return bindings
}

func CreateTextArea(existing *textarea.Model) textarea.Model {
	t := theme.CurrentTheme()
	textColor := t.Text()
	textMutedColor := t.TextMuted()

	ta := textarea.New()
	// GORILLA OVERRIDE: the fill comes from styles.PanelBackground(), not the theme
	// background directly, so the input is unpainted outside the alternate screen
	// like everything else. bgColor was a local variable, which is why the sweep that
	// routed every other fill did not reach these — a component that copies a colour
	// into a variable first is invisible to a search for the colour.
	bgColor := styles.PanelBackground()
	ta.BlurredStyle.Base = styles.BaseStyle().Background(bgColor).Foreground(textColor)
	ta.BlurredStyle.CursorLine = styles.BaseStyle().Background(bgColor)
	ta.BlurredStyle.Placeholder = styles.BaseStyle().Background(bgColor).Foreground(textMutedColor)
	ta.BlurredStyle.Text = styles.BaseStyle().Background(bgColor).Foreground(textColor)
	ta.FocusedStyle.Base = styles.BaseStyle().Background(bgColor).Foreground(textColor)
	ta.FocusedStyle.CursorLine = styles.BaseStyle().Background(bgColor)
	ta.FocusedStyle.Placeholder = styles.BaseStyle().Background(bgColor).Foreground(textMutedColor)
	ta.FocusedStyle.Text = styles.BaseStyle().Background(bgColor).Foreground(textColor)

	ta.Prompt = " "
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	// Without this, bubbles refuses to add a logical line past its default of 99, so
	// a long pasted prompt is silently truncated no matter what CharLimit says.
	ta.MaxHeight = editorBufferLines

	if existing != nil {
		ta.SetValue(existing.Value())
		ta.SetWidth(existing.Width())
		ta.SetHeight(existing.Height())
	}

	ta.Focus()
	return ta
}

func NewEditorCmp(app *app.App) tea.Model {
	ta := CreateTextArea(nil)
	return &editorCmp{
		app:      app,
		textarea: ta,
	}
}
