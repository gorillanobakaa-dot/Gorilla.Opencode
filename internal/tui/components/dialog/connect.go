package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// GORILLA OVERRIDE: /connect — add and manage provider connections (API keys,
// OpenAI-compatible local endpoints, Google login) without any one disabling
// the others. Every keyed provider and every local endpoint coexists.

// CloseConnectDialogMsg closes the dialog.
type CloseConnectDialogMsg struct{}

// ConnectionChangedMsg is emitted after a connection is added/toggled so the
// TUI can surface a status line (and any newly available models are already
// live in the registry).
type ConnectionChangedMsg struct{ Info string }

// RunGoogleLoginMsg asks the TUI to run the existing Google OAuth flow.
type RunGoogleLoginMsg struct{}

// UseProviderMsg is emitted when the user presses `u` on a connected provider
// in the /connect list. The TUI closes /connect and opens the model picker
// pre-scrolled to that provider's tab — a one-keypress replacement for the
// old "close, open /model, arrow across to the right tab" flow.
//
// GORILLA OVERRIDE: message did not exist upstream. Paired with
// ModelDialog.SwitchToProvider and the handler in tui.go.
type UseProviderMsg struct {
	Provider models.ModelProvider
}

// ConnectDialog is the /connect dialog.
type ConnectDialog interface {
	tea.Model
	layout.Bindings
}

type connectKind int

const (
	kindKey    connectKind = iota // API-key provider
	kindLocal                     // OpenAI-compatible endpoint (NIM, Ollama, ...)
	kindGoogle                    // Google Code Assist OAuth login
)

type connectEntry struct {
	kind     connectKind
	label    string
	provider models.ModelProvider // kindKey / kindGoogle
	epName   string               // kindLocal preset name
	epURL    string               // kindLocal preset base URL
}

// The catalog of things you can connect. Adding any one never disables another.
var connectEntries = []connectEntry{
	{kind: kindKey, label: "Anthropic", provider: models.ProviderAnthropic},
	{kind: kindKey, label: "OpenAI", provider: models.ProviderOpenAI},
	{kind: kindKey, label: "Google Gemini (API key)", provider: models.ProviderGemini},
	{kind: kindGoogle, label: "Google (Code Assist login)", provider: models.ProviderGeminiCA},
	{kind: kindKey, label: "Groq", provider: models.ProviderGROQ},
	{kind: kindKey, label: "xAI (Grok)", provider: models.ProviderXAI},
	{kind: kindKey, label: "OpenRouter", provider: models.ProviderOpenRouter},
	{kind: kindKey, label: "Cerebras", provider: models.ProviderCerebras},
	{kind: kindLocal, label: "NVIDIA NIM", epName: "nim", epURL: "https://integrate.api.nvidia.com/v1"},
	{kind: kindLocal, label: "Ollama (local)", epName: "ollama", epURL: "http://localhost:11434/v1"},
	{kind: kindLocal, label: "Custom OpenAI-compatible endpoint"},
}

type connectMode int

const (
	modeList connectMode = iota
	modeForm
)

const connectDialogWidth = 54

type connectKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Escape key.Binding
	Toggle key.Binding
	Tab    key.Binding
	// GORILLA OVERRIDE: `u` = "use this provider for the current session".
	// On a connected & enabled row, emits UseProviderMsg which the TUI turns
	// into a jump to /model on the right tab. On a not-connected row we
	// short-circuit to a status message rather than pretending to switch.
	Use key.Binding
}

var connectKeys = connectKeyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "down")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "add/edit")),
	Escape: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close/back")),
	Toggle: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "enable/disable")),
	Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
	Use:    key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "use for session")),
}

type connectDialogCmp struct {
	mode        connectMode
	selectedIdx int
	width       int
	height      int

	formEntry connectEntry
	inputs    []textinput.Model
	focusIdx  int
	formErr   string
}

// NewConnectDialogCmp builds the /connect dialog.
func NewConnectDialogCmp() ConnectDialog {
	return &connectDialogCmp{}
}

// applyInputTheme paints a textinput with the dialog's theme background so it
// blends into the dialog instead of rendering as a black box.
func applyInputTheme(in *textinput.Model) {
	t := theme.CurrentTheme()
	bg := lipgloss.NewStyle().Background(t.Background())
	in.PromptStyle = bg.Foreground(t.Primary())
	in.TextStyle = bg.Foreground(t.Text())
	in.PlaceholderStyle = bg.Foreground(t.TextMuted())
	in.CompletionStyle = bg.Foreground(t.TextMuted())
	in.Cursor.Style = bg
}

func (m *connectDialogCmp) Init() tea.Cmd {
	m.mode = modeList
	m.inputs = nil
	m.formErr = ""
	return nil
}

func (m *connectDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.mode == modeForm {
			return m.updateForm(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m *connectDialogCmp) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, connectKeys.Up):
		if m.selectedIdx > 0 {
			m.selectedIdx--
		} else {
			m.selectedIdx = len(connectEntries) - 1
		}
	case key.Matches(msg, connectKeys.Down):
		if m.selectedIdx < len(connectEntries)-1 {
			m.selectedIdx++
		} else {
			m.selectedIdx = 0
		}
	case key.Matches(msg, connectKeys.Escape):
		return m, util.CmdHandler(CloseConnectDialogMsg{})
	case key.Matches(msg, connectKeys.Toggle):
		return m, m.toggleSelected()

	// GORILLA OVERRIDE: `u` — jump directly to /model on this provider's tab.
	// Only meaningful for a provider that is actually reachable — for a not-
	// connected or disabled entry we tell the user what's missing rather than
	// silently switching to a broken tab.
	case key.Matches(msg, connectKeys.Use):
		e := connectEntries[m.selectedIdx]
		connected, disabled := m.status(e)
		if !connected {
			return m, util.CmdHandler(ConnectionChangedMsg{
				Info: fmt.Sprintf("%s not connected yet — press enter to add a key or endpoint first", e.label),
			})
		}
		if disabled {
			return m, util.CmdHandler(ConnectionChangedMsg{
				Info: fmt.Sprintf("%s is disabled — press space to enable, then u", e.label),
			})
		}
		// Local endpoints all share ProviderLocal — the model picker
		// distinguishes them by ID prefix, not by provider tab.
		target := e.provider
		if e.kind == kindLocal {
			target = models.ProviderLocal
		}
		return m, tea.Batch(
			util.CmdHandler(CloseConnectDialogMsg{}),
			util.CmdHandler(UseProviderMsg{Provider: target}),
		)

	case key.Matches(msg, connectKeys.Enter):
		e := connectEntries[m.selectedIdx]
		if e.kind == kindGoogle {
			return m, tea.Batch(
				util.CmdHandler(CloseConnectDialogMsg{}),
				util.CmdHandler(RunGoogleLoginMsg{}),
			)
		}
		m.openForm(e)
	}
	return m, nil
}

func (m *connectDialogCmp) openForm(e connectEntry) {
	m.mode = modeForm
	m.formEntry = e
	m.formErr = ""
	m.focusIdx = 0
	cfg := config.Get()

	switch e.kind {
	case kindKey:
		in := textinput.New()
		in.Placeholder = "paste API key"
		in.EchoMode = textinput.EchoPassword
		in.CharLimit = 400
		in.Width = 40
		if p, ok := cfg.Providers[e.provider]; ok && p.APIKey != "" {
			in.SetValue(p.APIKey)
		}
		applyInputTheme(&in)
		in.Focus()
		m.inputs = []textinput.Model{in}
	case kindLocal:
		name := textinput.New()
		name.Placeholder = "name (e.g. ollama)"
		name.CharLimit = 40
		name.Width = 40
		url := textinput.New()
		url.Placeholder = "https://host/v1"
		url.CharLimit = 200
		url.Width = 40
		keyIn := textinput.New()
		keyIn.Placeholder = "API key (optional)"
		keyIn.EchoMode = textinput.EchoPassword
		keyIn.CharLimit = 400
		keyIn.Width = 40
		if e.epName != "" {
			name.SetValue(e.epName)
		}
		if e.epURL != "" {
			url.SetValue(e.epURL)
		}
		for _, ep := range cfg.LocalEndpoints {
			if e.epName != "" && ep.Name == e.epName {
				url.SetValue(ep.BaseURL)
				keyIn.SetValue(ep.APIKey)
			}
		}
		applyInputTheme(&name)
		applyInputTheme(&url)
		applyInputTheme(&keyIn)
		name.Focus()
		m.inputs = []textinput.Model{name, url, keyIn}
	}
}

func (m *connectDialogCmp) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, connectKeys.Escape):
		m.mode = modeList
		m.inputs = nil
		return m, nil
	case key.Matches(msg, connectKeys.Tab):
		m.focusField(1)
		return m, nil
	case msg.String() == "shift+tab":
		m.focusField(-1)
		return m, nil
	case key.Matches(msg, connectKeys.Enter):
		return m, m.submitForm()
	}
	var cmd tea.Cmd
	m.inputs[m.focusIdx], cmd = m.inputs[m.focusIdx].Update(msg)
	return m, cmd
}

func (m *connectDialogCmp) focusField(delta int) {
	if len(m.inputs) == 0 {
		return
	}
	m.inputs[m.focusIdx].Blur()
	m.focusIdx = (m.focusIdx + delta + len(m.inputs)) % len(m.inputs)
	m.inputs[m.focusIdx].Focus()
}

func (m *connectDialogCmp) submitForm() tea.Cmd {
	e := m.formEntry
	switch e.kind {
	case kindKey:
		keyv := strings.TrimSpace(m.inputs[0].Value())
		if keyv == "" {
			m.formErr = "key cannot be empty"
			return nil
		}
		if err := config.UpsertProviderKey(e.provider, keyv); err != nil {
			m.formErr = err.Error()
			return nil
		}
		m.mode = modeList
		m.inputs = nil
		return util.CmdHandler(ConnectionChangedMsg{Info: fmt.Sprintf("Connected %s", e.label)})
	case kindLocal:
		name := strings.TrimSpace(m.inputs[0].Value())
		url := strings.TrimSpace(m.inputs[1].Value())
		keyv := strings.TrimSpace(m.inputs[2].Value())
		if name == "" || url == "" {
			m.formErr = "name and base URL are required"
			return nil
		}
		ep := config.LocalEndpoint{Name: name, BaseURL: url, APIKey: keyv}
		if err := config.UpsertLocalEndpoint(ep); err != nil {
			m.formErr = err.Error()
			return nil
		}
		n, _ := models.RegisterLocalEndpoint(name, url, keyv)
		m.mode = modeList
		m.inputs = nil
		if n == 0 {
			return util.CmdHandler(ConnectionChangedMsg{Info: fmt.Sprintf("%s saved — no models found at %s", name, url)})
		}
		return util.CmdHandler(ConnectionChangedMsg{Info: fmt.Sprintf("%s: %d model(s) added", name, n)})
	}
	return nil
}

func (m *connectDialogCmp) toggleSelected() tea.Cmd {
	e := connectEntries[m.selectedIdx]
	cfg := config.Get()
	switch e.kind {
	case kindKey, kindGoogle:
		p, ok := cfg.Providers[e.provider]
		if !ok || p.APIKey == "" {
			return util.CmdHandler(ConnectionChangedMsg{Info: fmt.Sprintf("%s not connected yet", e.label)})
		}
		newDisabled := !p.Disabled
		if err := config.SetProviderDisabled(e.provider, newDisabled); err != nil {
			return util.CmdHandler(ConnectionChangedMsg{Info: err.Error()})
		}
		state := "enabled"
		if newDisabled {
			state = "disabled"
		}
		return util.CmdHandler(ConnectionChangedMsg{Info: fmt.Sprintf("%s %s", e.label, state)})
	case kindLocal:
		var found *config.LocalEndpoint
		for i := range cfg.LocalEndpoints {
			if e.epName != "" && cfg.LocalEndpoints[i].Name == e.epName {
				found = &cfg.LocalEndpoints[i]
				break
			}
		}
		if found == nil {
			return util.CmdHandler(ConnectionChangedMsg{Info: fmt.Sprintf("%s not connected yet", e.label)})
		}
		newDisabled := !found.Disabled
		if err := config.SetLocalEndpointDisabled(found.Name, newDisabled); err != nil {
			return util.CmdHandler(ConnectionChangedMsg{Info: err.Error()})
		}
		if newDisabled {
			models.UnregisterLocalEndpoint(found.BaseURL)
			return util.CmdHandler(ConnectionChangedMsg{Info: fmt.Sprintf("%s disabled", found.Name)})
		}
		n, _ := models.RegisterLocalEndpoint(found.Name, found.BaseURL, found.APIKey)
		return util.CmdHandler(ConnectionChangedMsg{Info: fmt.Sprintf("%s enabled (%d models)", found.Name, n)})
	}
	return nil
}

// status reports whether an entry is currently connected and, if so, disabled.
func (m *connectDialogCmp) status(e connectEntry) (connected, disabled bool) {
	cfg := config.Get()
	switch e.kind {
	case kindKey, kindGoogle:
		p, ok := cfg.Providers[e.provider]
		return ok && p.APIKey != "", ok && p.Disabled
	case kindLocal:
		for _, ep := range cfg.LocalEndpoints {
			if e.epName != "" && ep.Name == e.epName {
				return true, ep.Disabled
			}
		}
	}
	return false, false
}

func (m *connectDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	// Render EACH line at full width (lipgloss .Width() does not pad every line
	// of a multi-line string, so a single Render leaves the short gorilla lines'
	// right side unpainted → terminal black). Per-line render matches the other
	// dialogs and paints the whole width with the theme background.
	gStyle := base.Foreground(t.Primary()).Bold(true).Width(connectDialogWidth)
	gLines := []string{
		"     .-\"-.-\"-.",
		"    (  o   o  )",
		"     )  \\_/  (     gorilla · /connect",
		"    (  =====  )",
		"     '-.___.-'",
	}
	styledGorilla := make([]string, len(gLines))
	for i, l := range gLines {
		styledGorilla[i] = gStyle.Render(l)
	}
	gorilla := lipgloss.JoinVertical(lipgloss.Left, styledGorilla...)

	var body string
	if m.mode == modeForm {
		body = m.formView()
	} else {
		body = m.listView()
	}
	content := lipgloss.JoinVertical(lipgloss.Left, gorilla, base.Width(connectDialogWidth).Render(""), body)
	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func (m *connectDialogCmp) listView() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := connectDialogWidth

	title := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render("Connections — coexist, never disable each other")
	hint := base.Foreground(t.TextMuted()).Width(w).
		Render("enter: add/edit   space: enable/disable   u: use now   esc: close")

	rows := make([]string, 0, len(connectEntries))
	for i, e := range connectEntries {
		connected, disabled := m.status(e)
		badge := "  ·  "
		switch {
		case connected && disabled:
			badge = " off "
		case connected:
			badge = "  ✓  "
		}
		label := e.label
		switch e.kind {
		case kindLocal:
			label += "  (endpoint)"
		case kindGoogle:
			label += "  (login)"
		}
		line := fmt.Sprintf("%s %s", badge, label)
		if r := []rune(line); len(r) > w-1 {
			line = string(r[:w-2]) + "…"
		}
		st := base.Width(w)
		switch {
		case i == m.selectedIdx:
			st = st.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		case connected && !disabled:
			st = st.Foreground(t.Primary())
		case connected && disabled:
			st = st.Foreground(t.TextMuted())
		}
		rows = append(rows, st.Render(line))
	}

	blank := base.Width(w).Render("")
	return lipgloss.JoinVertical(lipgloss.Left,
		title, blank,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		blank, hint,
	)
}

func (m *connectDialogCmp) formView() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := connectDialogWidth

	title := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render("Add / edit: " + m.formEntry.label)

	var labels []string
	switch m.formEntry.kind {
	case kindKey:
		labels = []string{"API key"}
	case kindLocal:
		labels = []string{"Name", "Base URL (…/v1)", "API key (optional)"}
	}

	blank := base.Width(w).Render("")
	rows := []string{title, blank}
	for i, in := range m.inputs {
		lbl := labels[i]
		if i == m.focusIdx {
			lbl = "▸ " + lbl
		} else {
			lbl = "  " + lbl
		}
		rows = append(rows,
			base.Foreground(t.TextMuted()).Width(w).Render(lbl+":"),
			base.Width(w).Render(in.View()),
		)
	}
	if m.formErr != "" {
		rows = append(rows, blank, base.Foreground(t.Error()).Width(w).Render("⚠ "+m.formErr))
	}
	rows = append(rows, blank,
		base.Foreground(t.TextMuted()).Width(w).Render("tab: next field   enter: save   esc: back"),
	)
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m *connectDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(connectKeys)
}
