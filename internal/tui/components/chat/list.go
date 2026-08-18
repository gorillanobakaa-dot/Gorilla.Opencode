package chat

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type cacheItem struct {
	width   int
	content []uiMessage
}
type messagesCmp struct {
	app           *app.App
	width, height int
	viewport      viewport.Model
	session       session.Session
	messages      []message.Message
	uiMessages    []uiMessage
	currentMsgID  string
	cachedContent map[string]cacheItem
	spinner       spinner.Model
	// spinning records whether a spinner tick chain is currently alive.
	//
	// GORILLA FIX (2026-08-17): the chain used to start at Init() and never
	// stop. Bubbles' spinner schedules the next tick from inside Update, so a
	// chain started once ticks for the life of the program — 8 times a second,
	// forever, whether or not anything is working. Every one of those ticks is
	// a tea.Msg, and every message re-runs View() for the whole UI. MEASURED on
	// v0.1.89: a brand-new instance in an empty folder, no conversation, doing
	// absolutely nothing, burned 35% of a CPU core; a session with a real
	// transcript burned 59%, because the render cost scales with what is on
	// screen. It also starves input — with helpers streaming on top, keystrokes
	// queue behind the backlog and the TUI looks frozen, which is exactly what
	// was reported (esc and the arrows "not working" while 8 helpers ran).
	//
	// The spinner is only VISIBLE while the agent is working (see working()),
	// so the chain now lives exactly as long as it is drawn.
	spinning  bool
	rendering bool
	// GORILLA OVERRIDE (2026-08-18): the working indicator now carries an
	// elapsed clock and escalates its wording once the wait crosses the
	// cold-start threshold. Measured that day: on shared endpoints (NVIDIA NIM)
	// an idle model has to warm up, taking 12–19 seconds and sometimes minutes
	// before the first token — during which a mute "Thinking..." spinner is
	// indistinguishable from a hang. That ambiguity was fixed at the wire in the
	// same session (config.FirstByteTimeout); this is the human-facing half of
	// it. taskLabel is the phase the clock is timing, taskSince when that phase
	// began — the clock resets on every phase change so it always reads "how
	// long has THIS step been quiet", not total turn time.
	taskLabel string
	taskSince time.Time
	// coldStartWarned stops the "warming up" toast from repeating within a
	// single phase — it fires once when the wait first crosses the threshold,
	// and resets when the phase changes.
	coldStartWarned bool
	attachments     viewport.Model
	// GORILLA OVERRIDE: throttle streaming re-renders (see below).
	lastStreamRender time.Time
	// GORILLA OVERRIDE: scrollback mode. When the alternate screen is off — the
	// default — settled messages are PRINTED into the terminal's own output
	// instead of being drawn into the viewport, so the terminal owns the history
	// and can scroll, select and copy it. See printer.go.
	scrollback bool
	// printed records which message IDs have already been written out. Printing
	// is irreversible, and every pubsub update carries the WHOLE message, so
	// without this a streaming reply would be emitted again on every token.
	printed map[string]bool
	// reasonedLines is the watermark of how many COMPLETE reasoning lines have
	// already been printed for a message still in flight, and reasoningOpened
	// whether its "thinking" marker has been emitted. Both are keyed by message
	// ID and dropped when the message settles. Reasoning is append-only, so a
	// line before a newline is final and safe to print; the watermark is what
	// stops every pubsub update reprinting the whole block.
	reasonedLines   map[string]int
	reasoningOpened map[string]bool
}
type renderFinishedMsg struct{}

type MessageKeys struct {
	PageDown     key.Binding
	PageUp       key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
}

var messageKeys = MessageKeys{
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("f/pgdn", "page down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("b/pgup", "page up"),
	),
	HalfPageUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "½ page up"),
	),
	HalfPageDown: key.NewBinding(
		key.WithKeys("ctrl+d", "ctrl+d"),
		key.WithHelp("ctrl+d", "½ page down"),
	),
}

// Init deliberately does NOT start the spinner. Nothing is working at startup,
// so nothing needs animating; the chain is started on demand in Update.
func (m *messagesCmp) Init() tea.Cmd {
	return m.viewport.Init()
}

func (m *messagesCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case dialog.ThemeChangedMsg:
		m.rerender()
		return m, nil
	case SessionSelectedMsg:
		if msg.ID != m.session.ID {
			cmd := m.SetSession(msg)
			return m, cmd
		}
		return m, nil
	case SessionClearedMsg:
		m.session = session.Session{}
		m.messages = make([]message.Message, 0)
		m.currentMsgID = ""
		m.rendering = false
		// GORILLA OVERRIDE: forget what was printed, without trying to unprint it.
		// The old session's text stays in the terminal's scrollback, which is the
		// point of putting it there.
		m.forgetPrinted()
		return m, nil

	case tea.MouseMsg:
		// GORILLA OVERRIDE: forward mouse-wheel events to the message
		// viewport so users can scroll the conversation with the wheel,
		// not only with PageUp/PageDown.
		u, cmd := m.viewport.Update(msg)
		m.viewport = u
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		if key.Matches(msg, messageKeys.PageUp) || key.Matches(msg, messageKeys.PageDown) ||
			key.Matches(msg, messageKeys.HalfPageUp) || key.Matches(msg, messageKeys.HalfPageDown) {
			u, cmd := m.viewport.Update(msg)
			m.viewport = u
			cmds = append(cmds, cmd)
		}

	case renderFinishedMsg:
		m.rendering = false
		m.viewport.GotoBottom()
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent && msg.Payload.ID == m.session.ID {
			m.session = msg.Payload
			if m.session.SummaryMessageID == m.currentMsgID {
				delete(m.cachedContent, m.currentMsgID)
				m.renderView()
			}
		}
	case pubsub.Event[message.Message]:
		needsRerender := false
		if msg.Type == pubsub.CreatedEvent {
			if msg.Payload.SessionID == m.session.ID {

				messageExists := false
				for _, v := range m.messages {
					if v.ID == msg.Payload.ID {
						messageExists = true
						break
					}
				}

				if !messageExists {
					if len(m.messages) > 0 {
						lastMsgID := m.messages[len(m.messages)-1].ID
						delete(m.cachedContent, lastMsgID)
					}

					m.messages = append(m.messages, msg.Payload)
					delete(m.cachedContent, m.currentMsgID)
					m.currentMsgID = msg.Payload.ID
					needsRerender = true
				}
			}
			// There are tool calls from the child task
			for _, v := range m.messages {
				for _, c := range v.ToolCalls() {
					if c.ID == msg.Payload.SessionID {
						delete(m.cachedContent, v.ID)
						needsRerender = true
					}
				}
			}
		} else if msg.Type == pubsub.UpdatedEvent && msg.Payload.SessionID == m.session.ID {
			for i, v := range m.messages {
				if v.ID == msg.Payload.ID {
					m.messages[i] = msg.Payload
					delete(m.cachedContent, msg.Payload.ID)
					needsRerender = true
					break
				}
			}
		}
		if needsRerender {
			// GORILLA OVERRIDE: in scrollback mode there is no viewport to
			// re-render. Settled messages are printed into the terminal's own
			// output, which also means the O(n^2) re-render below never runs —
			// each message's Markdown is rendered exactly once, when it settles,
			// rather than again on every token.
			if m.scrollback {
				// GORILLA FIX: tea.Batch runs commands CONCURRENTLY with no
				// ordering guarantees (see its doc comment), so batching several
				// prints let them land in any order — the answer beat the
				// "done thinking" marker that was emitted before it. Printed
				// output cannot be reordered afterwards, so the sequence is not
				// optional: tea.Sequence runs them one at a time, in order.
				if prints := m.printPending(); len(prints) > 0 {
					cmds = append(cmds, tea.Sequence(prints...))
				}
				return m, tea.Batch(cmds...)
			}
			// GORILLA OVERRIDE: re-rendering the whole growing message's
			// Markdown on EVERY streamed token is O(n^2) and makes long
			// answers crawl. Throttle intermediate deltas to ~every
			// 80ms; always render the final (finished) token so nothing
			// is lost. This is a display fix — the network was never the
			// bottleneck (NIM answers in ~1s).
			finished := msg.Type == pubsub.CreatedEvent || msg.Payload.IsFinished()
			if !finished && time.Since(m.lastStreamRender) < 80*time.Millisecond {
				return m, tea.Batch(cmds...)
			}
			m.lastStreamRender = time.Now()
			m.renderView()
			if len(m.messages) > 0 {
				if (msg.Type == pubsub.CreatedEvent) ||
					(msg.Type == pubsub.UpdatedEvent && msg.Payload.ID == m.messages[len(m.messages)-1].ID) {
					m.viewport.GotoBottom()
				}
			}
		}
	}

	// GORILLA FIX (2026-08-17): tick only while the spinner is on screen.
	// Forwarding a TickMsg is what schedules the NEXT tick, so declining to
	// forward it lets the chain lapse; starting one when work begins brings it
	// back. See the note on the `spinning` field for the measurement.
	if m.IsAgentWorking() {
		if !m.spinning {
			m.spinning = true
			cmds = append(cmds, m.spinner.Tick)
		}
		// GORILLA OVERRIDE: drive the elapsed clock here, in Update, so the
		// render path stays pure. Reset it whenever the phase changes, so it
		// times the CURRENT step's quiet rather than the whole turn — a fresh
		// "Thinking..." after a tool result should not inherit the previous
		// step's seconds.
		if label := m.taskLabel_(); label != m.taskLabel {
			m.taskLabel = label
			m.taskSince = time.Now()
			m.coldStartWarned = false
		}
		// GORILLA OVERRIDE: once a pre-first-token phase has been quiet past the
		// cold-start threshold, say so — once — in a toast, where a full
		// sentence is safe (the footer is one row and cannot hold it). This is
		// the human-facing twin of config.FirstByteTimeout: that bounds the
		// silent wait, this explains it while it lasts.
		if !m.coldStartWarned && shouldColdStartWarn(m.taskLabel, m.elapsedInPhase()) {
			m.coldStartWarned = true
			// Echo: the sentence is longer than the status bar can show, and the
			// half it drops ("First reply can take a minute…") is the half that
			// tells the user what to do — so it also goes into the transcript in
			// full. See util.ReportInfoEcho.
			cmds = append(cmds, util.ReportInfoEcho(
				"🦍 Still waiting on the model — a quiet endpoint is usually warming up, not "+
					"stuck. First reply can take a minute on a shared/free model. Press esc to cancel."))
		}
		s, cmd := m.spinner.Update(msg)
		m.spinner = s
		cmds = append(cmds, cmd)
	} else if m.spinning {
		if _, isTick := msg.(spinner.TickMsg); isTick {
			// Swallow this one: not forwarding it ends the chain.
			m.spinning = false
		}
		m.taskLabel = ""
		m.taskSince = time.Time{}
		m.coldStartWarned = false
	}
	return m, tea.Batch(cmds...)
}

// taskLabel_ names the phase the agent is in right now. It is the single source
// of truth for both the clock reset (Update) and the rendered text (working()),
// so the two can never disagree about which phase is being timed.
func (m *messagesCmp) taskLabel_() string {
	if len(m.messages) == 0 {
		return "Thinking..."
	}
	lastMessage := m.messages[len(m.messages)-1]
	switch {
	case hasToolsWithoutResponse(m.messages):
		return "Waiting for tool response..."
	case hasUnfinishedToolCalls(m.messages):
		return "Building tool call..."
	case !lastMessage.IsFinished():
		return "Generating..."
	default:
		return "Thinking..."
	}
}

func (m *messagesCmp) IsAgentWorking() bool {
	// GORILLA OVERRIDE: tolerate a missing agent instead of dereferencing blindly.
	// This is consulted from the footer, which is rendered on every single frame,
	// so a nil here is not a rare edge — it is a crash on the next keystroke. It
	// also mattered less when the transcript lived in a screen buffer that vanished
	// on exit; now that finished messages are printed into the terminal, a panic
	// leaves a half-written transcript the user cannot un-see.
	if m.app == nil || m.app.CoderAgent == nil {
		return false
	}
	return m.app.CoderAgent.IsSessionBusy(m.session.ID)
}

func formatTimeDifference(unixTime1, unixTime2 int64) string {
	diffSeconds := float64(math.Abs(float64(unixTime2 - unixTime1)))

	if diffSeconds < 60 {
		return fmt.Sprintf("%.1fs", diffSeconds)
	}

	minutes := int(diffSeconds / 60)
	seconds := int(diffSeconds) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

func (m *messagesCmp) renderView() {
	m.uiMessages = make([]uiMessage, 0)
	pos := 0
	baseStyle := styles.BaseStyle()

	if m.width == 0 {
		return
	}
	for inx, msg := range m.messages {
		switch msg.Role {
		case message.User:
			if cache, ok := m.cachedContent[msg.ID]; ok && cache.width == m.width {
				m.uiMessages = append(m.uiMessages, cache.content...)
				continue
			}
			userMsg := renderUserMessage(
				msg,
				msg.ID == m.currentMsgID,
				m.width,
				pos,
			)
			m.uiMessages = append(m.uiMessages, userMsg)
			m.cachedContent[msg.ID] = cacheItem{
				width:   m.width,
				content: []uiMessage{userMsg},
			}
			pos += userMsg.height + 1 // + 1 for spacing
		case message.Assistant:
			if cache, ok := m.cachedContent[msg.ID]; ok && cache.width == m.width {
				m.uiMessages = append(m.uiMessages, cache.content...)
				continue
			}
			isSummary := m.session.SummaryMessageID == msg.ID

			assistantMessages := renderAssistantMessage(
				msg,
				inx,
				m.messages,
				m.app.Messages,
				m.currentMsgID,
				isSummary,
				m.width,
				pos,
				// Alternate-screen path: the viewport owns the whole transcript
				// and nothing was printed, so the reasoning quote belongs here.
				false,
			)
			for _, msg := range assistantMessages {
				m.uiMessages = append(m.uiMessages, msg)
				pos += msg.height + 1 // + 1 for spacing
			}
			m.cachedContent[msg.ID] = cacheItem{
				width:   m.width,
				content: assistantMessages,
			}
		}
	}

	messages := make([]string, 0)
	for _, v := range m.uiMessages {
		messages = append(messages, lipgloss.JoinVertical(lipgloss.Left, v.content),
			baseStyle.
				Width(m.width).
				Render(
					"",
				),
		)
	}

	m.viewport.SetContent(
		baseStyle.
			Width(m.width).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Top,
					messages...,
				),
			),
	)
}

func (m *messagesCmp) View() string {
	baseStyle := styles.BaseStyle()

	if m.rendering {
		return baseStyle.
			Width(m.width).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Top,
					"Loading...",
					m.working(),
					m.help(),
				),
			)
	}
	if len(m.messages) == 0 {
		content := baseStyle.
			Width(m.width).
			Height(m.height - 1).
			Render(
				m.initialScreen(),
			)

		return baseStyle.
			Width(m.width).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Top,
					content,
					"",
					m.help(),
				),
			)
	}

	return baseStyle.
		Width(m.width).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Top,
				m.viewport.View(),
				m.working(),
				m.help(),
			),
		)
}

func hasToolsWithoutResponse(messages []message.Message) bool {
	toolCalls := make([]message.ToolCall, 0)
	toolResults := make([]message.ToolResult, 0)
	for _, m := range messages {
		toolCalls = append(toolCalls, m.ToolCalls()...)
		toolResults = append(toolResults, m.ToolResults()...)
	}

	for _, v := range toolCalls {
		found := false
		for _, r := range toolResults {
			if v.ID == r.ToolCallID {
				found = true
				break
			}
		}
		if !found && v.Finished {
			return true
		}
	}
	return false
}

func hasUnfinishedToolCalls(messages []message.Message) bool {
	toolCalls := make([]message.ToolCall, 0)
	for _, m := range messages {
		toolCalls = append(toolCalls, m.ToolCalls()...)
	}
	for _, v := range toolCalls {
		if !v.Finished {
			return true
		}
	}
	return false
}

// coldStartHint is when a quiet wait stops reading as "just a moment" and
// starts reading as "is this broken?". Measured 2026-08-18: warm models on
// NVIDIA NIM answer in well under a second, while a cold one took 12–19s. So a
// wait past ~12s is very likely a warm-up, and that is exactly when the user
// most needs telling.
const coldStartHint = 12 * time.Second

// elapsedInPhase is how long the current step has been running, or 0 if the
// clock is not set (the first frame of a phase, before Update has stamped it).
func (m *messagesCmp) elapsedInPhase() time.Duration {
	if m.taskSince.IsZero() {
		return 0
	}
	return time.Since(m.taskSince)
}

// isPreTokenPhase reports whether a phase is a wait for the model's FIRST
// output, as opposed to a tool round-trip. Only these get the cold-start
// explanation, because only these are the "is it warming up or hung?" ambiguity
// — calling a tool wait a model warm-up would be a plain lie.
func isPreTokenPhase(task string) bool {
	return task == "Thinking..." || task == "Generating..."
}

// shouldColdStartWarn is the toast decision, pulled out of Update so it can be
// tested without driving the whole event loop. It fires once a pre-token phase
// has been quiet past the measured threshold.
func shouldColdStartWarn(task string, elapsed time.Duration) bool {
	return isPreTokenPhase(task) && elapsed >= coldStartHint
}

// workingLabel turns a phase and its elapsed time into the footer text.
//
// It is deliberately SHORT — the footer is exactly FooterReservedRows (1) tall,
// and lipgloss WRAPS rather than truncates, so a long string here would become a
// second row and break the cursor-erase invariant printer.go depends on. So the
// footer only ever counts: "Thinking... (20s)". The sentence that EXPLAINS a
// long wait is emitted once as an info toast from Update (see coldStartToast),
// where length is free. This split is the same one the helper heartbeat uses.
func (m *messagesCmp) workingLabel(task string, elapsed time.Duration) string {
	if task == "" {
		return ""
	}
	if secs := int(elapsed.Seconds()); secs > 0 {
		return fmt.Sprintf("%s (%ds)", task, secs)
	}
	return task
}

func (m *messagesCmp) working() string {
	text := ""
	if m.IsAgentWorking() && len(m.messages) > 0 {
		t := theme.CurrentTheme()
		baseStyle := styles.BaseStyle()

		task := m.taskLabel_()
		// GORILLA OVERRIDE: show how long this step has been quiet, and once it
		// crosses the measured cold-start threshold, say so in words. Silence on
		// a shared endpoint is usually a model warming up, not a crash — the
		// program should say which. Only the pre-first-token phases carry the
		// reassurance: "Waiting for tool response" is a different wait and has
		// its own honest label.
		label := m.workingLabel(task, m.elapsedInPhase())
		if label != "" {
			text += baseStyle.
				Width(m.width).
				Foreground(t.Primary()).
				Bold(true).
				Render(fmt.Sprintf("%s %s ", m.spinner.View(), label))
		}
	}
	return text
}

func (m *messagesCmp) help() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	text := ""

	if m.app.CoderAgent.IsBusy() {
		text += lipgloss.JoinHorizontal(
			lipgloss.Left,
			baseStyle.Foreground(t.TextMuted()).Bold(true).Render("press "),
			baseStyle.Foreground(t.Text()).Bold(true).Render("esc"),
			baseStyle.Foreground(t.TextMuted()).Bold(true).Render(" to exit cancel"),
		)
	} else {
		text += lipgloss.JoinHorizontal(
			lipgloss.Left,
			baseStyle.Foreground(t.TextMuted()).Bold(true).Render("press "),
			baseStyle.Foreground(t.Text()).Bold(true).Render("enter"),
			baseStyle.Foreground(t.TextMuted()).Bold(true).Render(" to send the message,"),
			baseStyle.Foreground(t.TextMuted()).Bold(true).Render(" write"),
			baseStyle.Foreground(t.Text()).Bold(true).Render(" \\"),
			baseStyle.Foreground(t.TextMuted()).Bold(true).Render(" and enter to add a new line"),
		)
	}
	return baseStyle.
		Width(m.width).
		Render(text)
}

func (m *messagesCmp) initialScreen() string {
	baseStyle := styles.BaseStyle()

	return baseStyle.Width(m.width).Render(
		lipgloss.JoinVertical(
			lipgloss.Top,
			header(m.width, baseStyle),
			"",
			lspsConfigured(m.width, baseStyle),
		),
	)
}

func (m *messagesCmp) rerender() {
	for _, msg := range m.messages {
		delete(m.cachedContent, msg.ID)
	}
	m.renderView()
}

func (m *messagesCmp) SetSize(width, height int) tea.Cmd {
	if m.width == width && m.height == height {
		return nil
	}
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height - 2
	m.attachments.Width = width + 40
	m.attachments.Height = 3
	m.rerender()
	return nil
}

func (m *messagesCmp) GetSize() (int, int) {
	return m.width, m.height
}

func (m *messagesCmp) SetSession(session session.Session) tea.Cmd {
	if m.session.ID == session.ID {
		return nil
	}
	m.session = session
	messages, err := m.app.Messages.List(context.Background(), session.ID)
	if err != nil {
		return util.ReportError(err)
	}
	m.messages = messages
	if len(m.messages) > 0 {
		m.currentMsgID = m.messages[len(m.messages)-1].ID
	}
	delete(m.cachedContent, m.currentMsgID)
	// GORILLA OVERRIDE: in scrollback mode, loading a session prints its history
	// into the terminal rather than filling a viewport. printed is reset first so
	// the newly loaded messages are emitted; anything already in the terminal from
	// a previous session stays there, as history should.
	if m.scrollback {
		m.forgetPrinted()
		m.rendering = false
		// Ordered, not batched — see the note in Update. A session's history
		// printed out of order cannot be repaired.
		return tea.Sequence(m.printPending()...)
	}
	m.rendering = true
	return func() tea.Msg {
		m.renderView()
		return renderFinishedMsg{}
	}
}

func (m *messagesCmp) BindingKeys() []key.Binding {
	return []key.Binding{
		m.viewport.KeyMap.PageDown,
		m.viewport.KeyMap.PageUp,
		m.viewport.KeyMap.HalfPageUp,
		m.viewport.KeyMap.HalfPageDown,
	}
}

func NewMessagesCmp(app *app.App) tea.Model {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	vp := viewport.New(0, 0)
	attachmets := viewport.New(0, 0)
	vp.KeyMap.PageUp = messageKeys.PageUp
	vp.KeyMap.PageDown = messageKeys.PageDown
	vp.KeyMap.HalfPageUp = messageKeys.HalfPageUp
	vp.KeyMap.HalfPageDown = messageKeys.HalfPageDown
	return &messagesCmp{
		app:           app,
		cachedContent: make(map[string]cacheItem),
		viewport:      vp,
		spinner:       s,
		attachments:   attachmets,
		// GORILLA OVERRIDE: read once, at construction. The buffer the program
		// draws on is chosen when the program starts, so this cannot change
		// mid-session — and a component that re-read it every frame could end up
		// printing into a screen that keeps no history of the print.
		scrollback: !config.AlternateScreenEnabled(),
		printed:    make(map[string]bool),

		reasonedLines:   make(map[string]int),
		reasoningOpened: make(map[string]bool),
	}
}
