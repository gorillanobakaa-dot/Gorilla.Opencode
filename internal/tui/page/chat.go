package page

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/completions"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

var ChatPage PageID = "chat"

type chatPage struct {
	app      *app.App
	editor   layout.Container
	messages layout.Container
	// GORILLA OVERRIDE: the transcript component, typed for FooterView. Used only
	// when the alternate screen is off, where the conversation is printed into the
	// terminal and the pane itself is never drawn.
	messagesFooter chat.ScrollbackFooter
	// GORILLA OVERRIDE: scrollback mode has no right-hand panel, so the sidebar is
	// held here for its compact rendering in the footer instead. sidebarModel is
	// kept alongside it because the component is not in the layout in that mode and
	// therefore receives no Update unless this page forwards one — without that its
	// token counts and modified-files list would freeze at their initial values.
	scrollback           bool
	sidebarInfo          chat.FooterInfo
	sidebarModel         tea.Model
	layout               layout.SplitPaneLayout
	session              session.Session
	completionDialog     dialog.CompletionDialog
	showCompletionDialog bool
}

// FooterView is everything this page draws when the conversation lives in the
// terminal's scrollback instead of a pane: the working indicator, and the prompt.
//
// It must stay short. Outside the alternate screen bubbletea erases its previous
// frame by counting logical lines, so a frame taller than the window leaves the
// erase in the wrong place — one stale copy per redraw. Since the rolling preview
// was removed the transcript contributes exactly chat.FooterReservedRows (one row),
// so what remains is bounded by the prompt.
func (p *chatPage) FooterView(maxRows int) string {
	width, _ := p.layout.GetSize()

	// The prompt is not optional; everything else is shed to fit, least important
	// first. Order matters and is deliberate: the session numbers are reference
	// information you can also get from /context, whereas the live preview is the
	// only sign that a reply is arriving at all.
	prompt := p.editor.View()

	var live, info string
	if p.messagesFooter != nil {
		live = p.messagesFooter.FooterView()
	}
	if p.sidebarInfo != nil {
		info = p.sidebarInfo.CompactView(width)
	}

	// GORILLA FIX: the live row must be RESERVED, not merely included.
	//
	// messagesCmp.FooterView returns the working indicator, or "" when the agent
	// is idle. joinNonEmpty drops any part that is whitespace-only, so the whole
	// frame was one row SHORTER while idle than while working — it grew the
	// moment a turn started and shrank the moment it ended.
	//
	// Bubbletea erases its previous frame by walking the cursor UP by that
	// frame's row count. A frame whose height changes between renders therefore
	// erases the wrong number of rows every time the agent starts or stops, which
	// is what made the footer march down the screen and then jump back up instead
	// of settling at the bottom like gemini-cli or codex. Same root cause as the
	// v0.1.50 "text vanishes" bug: a frame that shrinks over-reaches.
	//
	// Reserving the row here rather than making FooterView return padding keeps
	// the fix where the height is decided. It also cannot be undone by a future
	// change to joinNonEmpty, because the row no longer depends on being
	// non-empty to survive.
	return reserveLiveRow(live, shedToFit(maxRows-chat.FooterReservedRows,
		footerArrangements("", prompt, info)))
}

// reserveLiveRow puts the working indicator above the rest of the footer,
// always occupying exactly chat.FooterReservedRows rows whether or not it has
// anything to say.
func reserveLiveRow(live, rest string) string {
	lines := make([]string, 0, chat.FooterReservedRows+1)
	liveLines := strings.Split(live, "\n")
	for i := 0; i < chat.FooterReservedRows; i++ {
		if i < len(liveLines) && strings.TrimSpace(liveLines[i]) != "" {
			lines = append(lines, liveLines[i])
			continue
		}
		lines = append(lines, "")
	}
	if strings.TrimSpace(rest) != "" {
		lines = append(lines, rest)
	}
	// JoinVertical, not joinNonEmpty: the blank reserved rows are the point.
	return lipgloss.JoinVertical(lipgloss.Top, lines...)
}

// footerArrangements lists the footer's possible contents from most to least
// complete. The order encodes what gets given up first when the window is short,
// and it is a named function so a test can assert that priority — an order chosen
// inline is an order nothing checks.
//
// The session's numbers are shed before the live preview: they are reference
// information also reachable from /context, whereas the preview is the only sign
// that a reply is arriving at all. The prompt appears in every arrangement, because
// a program with no visible prompt looks broken rather than cramped.
//
// Within an arrangement the numbers sit BELOW the prompt: they change on every
// token, and a block that reflows above the prompt would shift the line being
// typed on.
func footerArrangements(live, prompt, info string) [][]string {
	return [][]string{
		{live, prompt, info},
		{live, prompt},
		{prompt},
	}
}

// shedToFit returns the first arrangement that fits maxRows, or the last one if
// none do.
//
// Separated from FooterView so it can be tested without an app: the arithmetic is
// the part that matters, and a test that drove a whole chatPage would need a
// database, a history service and an agent to assert something about row counts.
// maxRows <= 0 means "unknown size", where refusing to render is worse than
// rendering the fullest arrangement.
func shedToFit(maxRows int, attempts [][]string) string {
	var view string
	for _, attempt := range attempts {
		view = joinNonEmpty(attempt)
		if maxRows <= 0 || lipgloss.Height(view) <= maxRows {
			return view
		}
	}
	// Nothing fit, not even the last and smallest arrangement. Return it anyway: a
	// window this short cannot really be used, but a program showing no prompt at
	// all looks broken rather than cramped.
	return view
}

func joinNonEmpty(parts []string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Top, kept...)
}

type ChatKeyMap struct {
	ShowCompletionDialog key.Binding
	NewSession           key.Binding
	Cancel               key.Binding
}

var keyMap = ChatKeyMap{
	ShowCompletionDialog: key.NewBinding(
		key.WithKeys("@"),
		key.WithHelp("@", "Complete"),
	),
	NewSession: key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "new session"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
}

func (p *chatPage) Init() tea.Cmd {
	cmds := []tea.Cmd{
		p.layout.Init(),
		p.completionDialog.Init(),
	}
	return tea.Batch(cmds...)
}

func (p *chatPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cmd := p.layout.SetSize(msg.Width, msg.Height)
		cmds = append(cmds, cmd)
	case chat.EditorHeightMsg:
		// GORILLA OVERRIDE: the editor grew/shrank with its content — give it
		// exactly the rows it asked for and let the message list take the rest.
		// msg.Height is CONTENT rows, so add the container's own border/padding
		// or the content gets squeezed to zero rows and the input box vanishes.
		cmds = append(cmds, p.layout.SetBottomHeight(msg.Height+p.editor.VerticalChrome()))
	case dialog.CompletionDialogCloseMsg:
		p.showCompletionDialog = false
	case chat.SendMsg:
		cmd := p.sendMessage(msg.Text, msg.Attachments)
		if cmd != nil {
			return p, cmd
		}
	case dialog.CommandRunCustomMsg:
		// Check if the agent is busy before executing custom commands
		if p.app.CoderAgent.IsBusy() {
			return p, util.ReportWarn("Agent is busy, please wait before executing a command...")
		}

		// Process the command content with arguments if any
		content := msg.Content
		if msg.Args != nil {
			// Replace all named arguments with their values
			for name, value := range msg.Args {
				placeholder := "$" + name
				content = strings.ReplaceAll(content, placeholder, value)
			}
		}

		// Handle custom command execution
		cmd := p.sendMessage(content, nil)
		if cmd != nil {
			return p, cmd
		}
	case chat.SessionSelectedMsg:
		if p.session.ID == "" {
			cmd := p.setSidebar()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		p.session = msg
	case chat.NewSessionMsg:
		// GORILLA OVERRIDE: /clear routes here so it does exactly what
		// the built-in new-session keybinding does — reset the page's
		// session and clear the sidebar, not just wipe the message list.
		p.session = session.Session{}
		return p, tea.Batch(
			p.clearSidebar(),
			util.CmdHandler(chat.SessionClearedMsg{}),
		)
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keyMap.ShowCompletionDialog):
			p.showCompletionDialog = true
			// Continue sending keys to layout->chat
		case key.Matches(msg, keyMap.NewSession):
			p.session = session.Session{}
			return p, tea.Batch(
				p.clearSidebar(),
				util.CmdHandler(chat.SessionClearedMsg{}),
			)
		case key.Matches(msg, keyMap.Cancel):
			if p.session.ID != "" {
				// Cancel the current session's generation process
				// This allows users to interrupt long-running operations
				p.app.CoderAgent.Cancel(p.session.ID)
				return p, nil
			}
		}
	}
	if p.showCompletionDialog {
		context, contextCmd := p.completionDialog.Update(msg)
		p.completionDialog = context.(dialog.CompletionDialog)
		cmds = append(cmds, contextCmd)

		// Doesn't forward event if enter key is pressed
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "enter" {
				return p, tea.Batch(cmds...)
			}
		}
	}

	u, cmd := p.layout.Update(msg)
	cmds = append(cmds, cmd)
	p.layout = u.(layout.SplitPaneLayout)

	// GORILLA OVERRIDE: the sidebar is not in the layout in scrollback mode, so it
	// would never see a message. Forward one, or its token counts and modified-files
	// list stay frozen at whatever they were when the session opened.
	if p.scrollback && p.sidebarModel != nil {
		sm, sideCmd := p.sidebarModel.Update(msg)
		p.sidebarModel = sm
		if info, ok := sm.(chat.FooterInfo); ok {
			p.sidebarInfo = info
		}
		cmds = append(cmds, sideCmd)
	}

	return p, tea.Batch(cmds...)
}

func (p *chatPage) setSidebar() tea.Cmd {
	model := chat.NewSidebarCmp(p.session, p.app.History)

	// GORILLA OVERRIDE: sidebar is never added to the layout — no right panel, ever.
	// The component is still initialised (Init starts the modified-files scan)
	// and kept for its compact footer rendering in scrollback mode.
	if info, ok := model.(chat.FooterInfo); ok {
		p.sidebarInfo = info
	}
	p.sidebarModel = model
	return model.Init()
}

func (p *chatPage) clearSidebar() tea.Cmd {
	p.sidebarInfo, p.sidebarModel = nil, nil
	// GORILLA OVERRIDE: sidebar is never in the layout, nothing to clear there.
	return nil
}

func (p *chatPage) sendMessage(text string, attachments []message.Attachment) tea.Cmd {
	var cmds []tea.Cmd
	if p.session.ID == "" {
		session, err := p.app.Sessions.Create(context.Background(), "New Session")
		if err != nil {
			return util.ReportError(err)
		}

		p.session = session
		cmd := p.setSidebar()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, util.CmdHandler(chat.SessionSelectedMsg(session)))
	}

	_, err := p.app.CoderAgent.Run(context.Background(), p.session.ID, text, attachments...)
	if err != nil {
		return util.ReportError(err)
	}
	return tea.Batch(cmds...)
}

func (p *chatPage) SetSize(width, height int) tea.Cmd {
	return p.layout.SetSize(width, height)
}

func (p *chatPage) GetSize() (int, int) {
	return p.layout.GetSize()
}

func (p *chatPage) View() string {
	layoutView := p.layout.View()

	if p.showCompletionDialog {
		_, layoutHeight := p.layout.GetSize()
		editorWidth, editorHeight := p.editor.GetSize()

		p.completionDialog.SetWidth(editorWidth)
		overlay := p.completionDialog.View()

		layoutView = layout.PlaceOverlay(
			0,
			layoutHeight-editorHeight-lipgloss.Height(overlay),
			overlay,
			layoutView,
			false,
		)
	}

	return layoutView
}

func (p *chatPage) BindingKeys() []key.Binding {
	bindings := layout.KeyMapToSlice(keyMap)
	bindings = append(bindings, p.messages.BindingKeys()...)
	bindings = append(bindings, p.editor.BindingKeys()...)
	return bindings
}

func NewChatPage(app *app.App) tea.Model {
	cg := completions.NewFileAndFolderContextGroup()
	completionDialog := dialog.NewCompletionDialogCmp(cg)

	messagesModel := chat.NewMessagesCmp(app)
	messagesContainer := layout.NewContainer(
		messagesModel,
		layout.WithPadding(1, 1, 0, 1),
	)
	editorContainer := layout.NewContainer(
		chat.NewEditorCmp(app),
		layout.WithBorder(true, false, false, false),
	)
	// GORILLA OVERRIDE: keep a typed handle on the transcript component so the
	// footer can be rendered without the messages pane. The container deliberately
	// hides its content, and the handle stays valid because the component's Update
	// returns the same pointer it was called on.
	footer, _ := messagesModel.(chat.ScrollbackFooter)
	return &chatPage{
		app:      app,
		editor:   editorContainer,
		messages: messagesContainer,
		// GORILLA OVERRIDE: read once, as the appModel does. The buffer is chosen
		// when the program starts, so this cannot change mid-session.
		scrollback:       !config.AlternateScreenEnabled(),
		messagesFooter:   footer,
		completionDialog: completionDialog,
		layout: layout.NewSplitPane(
			layout.WithLeftPanel(messagesContainer),
			layout.WithBottomPanel(editorContainer),
		),
	}
}
