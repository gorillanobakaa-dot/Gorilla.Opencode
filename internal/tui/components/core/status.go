package core

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/lsp"
	"github.com/opencode-ai/opencode/internal/lsp/protocol"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type StatusCmp interface {
	tea.Model
	// InfoBudget reports how many columns a status message survives here. The
	// app asks before showing one, because a notice that does not fit must also
	// be printed into the transcript rather than silently cut in half.
	InfoBudget() int
}

type statusCmp struct {
	info       util.InfoMsg
	width      int
	messageTTL time.Duration
	lspClients map[string]*lsp.Client
	session    session.Session
	// liveHelperCost is what the helpers of a run IN FLIGHT have spent so far.
	//
	// GORILLA FIX (2026-08-23): the footer reads session.Cost, which is ONE
	// row. Helper turns credit the helper's own session and are only rolled
	// into the parent when the research tool returns, so during a seventeen
	// minute run this footer read "$0.01" against a measured $6.70 sitting in
	// eighteen sibling rows. The money was never lost; it was arriving after
	// the last moment anyone could act on it.
	//
	// Carried separately rather than added into session.Cost, because the
	// rollup DOES land at the end and adding it here too would then count the
	// same run twice.
	liveHelperCost float64
	// GORILLA OVERRIDE: show the "cost is an estimate" note once per run.
	costNoticeShown bool
	// GORILLA FIX (2026-08-18): msgSeq is the generation of the message
	// currently shown. Every message increments it and arms a clear stamped with
	// it; a clear only fires if its stamp still matches. Without this, each
	// message armed an independent 10s timer with no way to invalidate an older
	// one, so message A's timer would wipe message B early — the reported "footer
	// messages vanish quite fast these days", worst in a burst where the last
	// message could be cleared almost immediately by the first message's timer.
	msgSeq int
}

// truncateStatusMsg fits a status/error message into width terminal COLUMNS.
//
// GORILLA FIX: this was inline as `msg[:infoWidth]` — a BYTE offset applied
// against a COLUMN budget. The two agree only for pure ASCII, so any multi-byte
// rune straddling the cut was sawn in half and emitted as invalid UTF-8. The
// provider errors that land here now routinely carry "—" and "⟨⟩" (3 bytes
// each), which took the odds of cutting mid-rune from remote to routine. len()
// was wrong in the same direction: it overstated the length of any non-ASCII
// message and truncated ones that would have fitted.
//
// ansi.Truncate measures display width and never splits a rune — the same tool
// clampToWidth in tui.go already uses for this exact job.
//
// Extracted from View() so the behaviour can actually be tested: asserting that
// ansi.Truncate is rune-safe proves nothing about whether this file calls it.
// errorMessageTTL is how long an error stays in the status bar.
//
// Deliberately longer than messageTTL (10s): errors carry a model name, an HTTP
// status, an explanation and a command to run. Measured at ~150 characters for a
// provider entitlement failure — reading that, deciding, and typing /models does
// not fit in ten seconds.
const errorMessageTTL = 40 * time.Second

func truncateStatusMsg(msg string, width int) string {
	if width <= 0 {
		return msg
	}
	return ansi.Truncate(msg, width, "...")
}

// clearMessageCmd clears the status message after ttl elapses — but only if it
// is still the message shown, via the seq stamp checked in the handler.
func (m statusCmp) clearMessageCmd(ttl time.Duration, seq int) tea.Cmd {
	return tea.Tick(ttl, func(time.Time) tea.Msg {
		return util.ClearStatusMsg{Seq: seq}
	})
}

func (m statusCmp) Init() tea.Cmd {
	return nil
}

func (m statusCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case chat.SessionSelectedMsg:
		m.session = msg
	case chat.SessionClearedMsg:
		m.session = session.Session{}
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent {
			if m.session.ID == msg.Payload.ID {
				m.session = msg.Payload
			}
		}
		// GORILLA OVERRIDE: the first time a session shows a non-zero
		// cost, remind the user (once) that it's only an estimate — on
		// a free or flat-rate tier the real bill is $0.
		if !m.costNoticeShown && m.session.Cost > 0 {
			m.costNoticeShown = true
			return m, util.CmdHandler(util.InfoMsg{
				Type: util.InfoTypeInfo,
				Msg:  "That cost is a rough estimate from a static price table — on a free or flat-rate tier your real bill is $0, genius.",
				TTL:  10 * time.Second,
			})
		}
	case LiveHelperCostMsg:
		m.liveHelperCost = float64(msg)
	case util.InfoMsg:
		m.info = msg
		m.msgSeq++
		ttl := msg.TTL
		if ttl == 0 {
			ttl = m.messageTTL
			// GORILLA FIX: an error is not a toast. It was sharing the 10s
			// default with notices like "copied to clipboard", and a provider
			// failure is a ~150-character diagnosis you have to read, parse and
			// act on — reported 2026-08-05 as "flashes by so fast I barely had
			// time to read it; took two tries to screenshot".
			//
			// The transcript now keeps the full text permanently (see
			// FinishReasonError details), so this is only the notification. It
			// still has to be readable at a glance rather than a reflex test.
			if msg.Type == util.InfoTypeError {
				ttl = errorMessageTTL
			}
		}
		return m, m.clearMessageCmd(ttl, m.msgSeq)
	case util.ClearStatusMsg:
		// Only clear if no newer message has arrived since this clear was armed;
		// otherwise an older message's timer would wipe a newer one early.
		if msg.Seq == m.msgSeq {
			m.info = util.InfoMsg{}
		}
	}
	return m, nil
}

var helpWidget = ""

// getHelpWidget returns the help widget with current theme colors
func getHelpWidget() string {
	t := theme.CurrentTheme()
	helpText := "ctrl+? help"

	return styles.Padded().
		Background(t.TextMuted()).
		Foreground(t.BackgroundDarker()).
		Bold(true).
		Render(helpText)
}

func formatTokensAndCost(tokens, contextWindow int64, cost float64) string {
	// Format tokens in human-readable format (e.g., 110K, 1.2M)
	var formattedTokens string
	switch {
	case tokens >= 1_000_000:
		formattedTokens = fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	case tokens >= 1_000:
		formattedTokens = fmt.Sprintf("%.1fK", float64(tokens)/1_000)
	default:
		formattedTokens = fmt.Sprintf("%d", tokens)
	}

	// Remove .0 suffix if present
	if strings.HasSuffix(formattedTokens, ".0K") {
		formattedTokens = strings.Replace(formattedTokens, ".0K", "K", 1)
	}
	if strings.HasSuffix(formattedTokens, ".0M") {
		formattedTokens = strings.Replace(formattedTokens, ".0M", "M", 1)
	}

	// GORILLA OVERRIDE: the cost is a rough ESTIMATE computed from a
	// static, possibly-stale price table — it is NOT your actual bill.
	// On a free tier (e.g. Gemini's) or a flat-rate key (NVIDIA NIM) the
	// real cost is $0. Mark it so nobody mistakes it for money spent.
	var formattedCost string
	if cost > 0 {
		formattedCost = fmt.Sprintf("~$%.2f est", cost)
	} else {
		formattedCost = "$0.00"
	}

	percentage := (float64(tokens) / float64(contextWindow)) * 100
	if percentage > 80 {
		// add the warning icon and percentage
		formattedTokens = fmt.Sprintf("%s(%d%%)", styles.WarningIcon, int(percentage))
	}

	return fmt.Sprintf("Context: %s, Cost: %s", formattedTokens, formattedCost)
}

// statusChrome is everything on the status line that is NOT the message: the
// help hint, the token/cost box, the helper count, the YOLO warning, the
// diagnostics and the model name. What is left over is the message's budget.
//
// GORILLA OVERRIDE (2026-08-21): extracted from View so the budget has ONE
// definition. The app needs to know, before rendering, whether a notice will be
// truncated here — that is what decides whether it is also printed into the
// transcript. Computing that with a second, parallel expression would be a
// guess that drifts the first time a widget is added to this line.
type statusChrome struct {
	tokens      string
	diagnostics string
	helpers     string
	yolo        string
	model       string
	// avail is the columns left for the message region.
	avail int
}

func (m statusCmp) chrome() statusChrome {
	t := theme.CurrentTheme()
	// A nil config is startup order, not corruption: the status bar can be
	// asked for its widths before Load() has run. It used to dereference
	// straight through and panic. There is nothing to name yet — render the
	// line without a model rather than take the app down over a label.
	var model models.Model
	if cfg := config.Get(); cfg != nil {
		model = models.SupportedModels[cfg.Agents[config.AgentCoder].Model]
	}

	c := statusChrome{model: m.model()}

	tokenInfoWidth := 0
	if m.session.ID != "" {
		// GORILLA FIX (2026-08-17): count the helpers too. This read
		// m.session.PromptTokens + CompletionTokens, which is the CONVERSATION
		// alone — a run that burned 507,935 tokens across 17 helper sessions
		// displayed 44,688 and the owner had to total the database by hand to
		// discover it. TotalTokens() falls back to the same two fields when no
		// helpers exist, so an ordinary chat is unaffected.
		totalTokens := m.session.TotalTokens()
		tokens := formatTokensAndCost(totalTokens, model.ContextWindow, m.session.Cost+m.liveHelperCost)
		tokensStyle := styles.Padded().
			Background(t.Text()).
			Foreground(t.BackgroundSecondary())
		percentage := (float64(totalTokens) / float64(model.ContextWindow)) * 100
		if percentage > 80 {
			tokensStyle = tokensStyle.Background(t.Warning())
		}
		tokenInfoWidth = lipgloss.Width(tokens) + 2
		c.tokens = tokensStyle.Render(tokens)
	}

	c.diagnostics = styles.Padded().
		Background(t.BackgroundDarker()).
		Render(m.projectDiagnostics())

	// GORILLA OVERRIDE: live helper-agent count. Transparency — the user
	// always sees how many sub-agents are running on their behalf, and that
	// /tasks can stop them.
	helpersWidth := 0
	if n := agent.ActiveSubAgentCount(); n > 0 {
		c.helpers = styles.Padded().
			Background(t.Warning()).
			Foreground(t.Background()).
			Render(fmt.Sprintf("🦍 %d helper(s) | /tasks", n))
		helpersWidth = lipgloss.Width(c.helpers)
	}

	// GORILLA OVERRIDE: while YOLO is on, say so on every single frame. It
	// switches off the prompt that stands between an agent and the user's
	// files, so it must never be something you can forget you enabled — the
	// same reasoning as the helper count beside it, one step louder.
	yoloWidth := 0
	if permission.SessionAutoApproved(m.session.ID) {
		c.yolo = styles.Padded().
			Background(t.Error()).
			Foreground(t.Background()).
			Bold(true).
			Render("☢ YOLO — auto-approving")
		yoloWidth = lipgloss.Width(c.yolo)
	}

	c.avail = max(0, m.width-lipgloss.Width(helpWidget)-lipgloss.Width(c.model)-lipgloss.Width(c.diagnostics)-tokenInfoWidth-helpersWidth-yoloWidth)
	return c
}

// InfoBudget is how many COLUMNS a status message actually gets before
// truncateStatusMsg cuts it — the same number View passes to that function, not
// an estimate of it. Zero or less means "not measurable yet" (no WindowSizeMsg
// has arrived), which is also the case where View does not truncate at all.
func (m statusCmp) InfoBudget() int {
	return m.chrome().avail - infoPadding
}

// infoPadding is the margin View subtracts from the message region before
// truncating: the padded style's own two columns plus room for the "..."
// marker and the widths this line cannot predict (emoji that measure one column
// and draw two).
const infoPadding = 10

func (m statusCmp) View() string {
	t := theme.CurrentTheme()
	c := m.chrome()

	// Initialize the help widget
	status := getHelpWidget()
	status += c.tokens

	if m.info.Msg != "" {
		infoStyle := styles.Padded().
			Foreground(t.Background()).
			Width(c.avail)

		switch m.info.Type {
		case util.InfoTypeInfo:
			infoStyle = infoStyle.Background(t.Info())
		case util.InfoTypeWarn:
			infoStyle = infoStyle.Background(t.Warning())
		case util.InfoTypeError:
			infoStyle = infoStyle.Background(t.Error())
		}

		msg := truncateStatusMsg(m.info.Msg, c.avail-infoPadding)
		status += infoStyle.Render(msg)
	} else {
		status += styles.Padded().
			Foreground(t.Text()).
			Background(t.BackgroundSecondary()).
			Width(c.avail).
			Render("")
	}

	status += c.yolo
	status += c.helpers
	status += c.diagnostics
	status += c.model

	return lipgloss.NewStyle().
		MaxWidth(m.width).
		MaxHeight(1).
		Render(status)
}

func (m *statusCmp) projectDiagnostics() string {
	t := theme.CurrentTheme()

	// Check if any LSP server is still initializing
	initializing := false
	for _, client := range m.lspClients {
		if client.GetServerState() == lsp.StateStarting {
			initializing = true
			break
		}
	}

	// If any server is initializing, show that status
	if initializing {
		return lipgloss.NewStyle().
			Background(t.BackgroundDarker()).
			Foreground(t.Warning()).
			Render(fmt.Sprintf("%s Initializing LSP...", styles.SpinnerIcon))
	}

	errorDiagnostics := []protocol.Diagnostic{}
	warnDiagnostics := []protocol.Diagnostic{}
	hintDiagnostics := []protocol.Diagnostic{}
	infoDiagnostics := []protocol.Diagnostic{}
	for _, client := range m.lspClients {
		for _, d := range client.GetDiagnostics() {
			for _, diag := range d {
				switch diag.Severity {
				case protocol.SeverityError:
					errorDiagnostics = append(errorDiagnostics, diag)
				case protocol.SeverityWarning:
					warnDiagnostics = append(warnDiagnostics, diag)
				case protocol.SeverityHint:
					hintDiagnostics = append(hintDiagnostics, diag)
				case protocol.SeverityInformation:
					infoDiagnostics = append(infoDiagnostics, diag)
				}
			}
		}
	}

	if len(errorDiagnostics) == 0 && len(warnDiagnostics) == 0 && len(hintDiagnostics) == 0 && len(infoDiagnostics) == 0 {
		return "No diagnostics"
	}

	diagnostics := []string{}

	if len(errorDiagnostics) > 0 {
		errStr := lipgloss.NewStyle().
			Background(t.BackgroundDarker()).
			Foreground(t.Error()).
			Render(fmt.Sprintf("%s %d", styles.ErrorIcon, len(errorDiagnostics)))
		diagnostics = append(diagnostics, errStr)
	}
	if len(warnDiagnostics) > 0 {
		warnStr := lipgloss.NewStyle().
			Background(t.BackgroundDarker()).
			Foreground(t.Warning()).
			Render(fmt.Sprintf("%s %d", styles.WarningIcon, len(warnDiagnostics)))
		diagnostics = append(diagnostics, warnStr)
	}
	if len(hintDiagnostics) > 0 {
		hintStr := lipgloss.NewStyle().
			Background(t.BackgroundDarker()).
			Foreground(t.Text()).
			Render(fmt.Sprintf("%s %d", styles.HintIcon, len(hintDiagnostics)))
		diagnostics = append(diagnostics, hintStr)
	}
	if len(infoDiagnostics) > 0 {
		infoStr := lipgloss.NewStyle().
			Background(t.BackgroundDarker()).
			Foreground(t.Info()).
			Render(fmt.Sprintf("%s %d", styles.InfoIcon, len(infoDiagnostics)))
		diagnostics = append(diagnostics, infoStr)
	}

	return strings.Join(diagnostics, " ")
}

func (m statusCmp) availableFooterMsgWidth(diagnostics, tokenInfo string) int {
	tokensWidth := 0
	if m.session.ID != "" {
		tokensWidth = lipgloss.Width(tokenInfo) + 2
	}
	return max(0, m.width-lipgloss.Width(helpWidget)-lipgloss.Width(m.model())-lipgloss.Width(diagnostics)-tokensWidth)
}

func (m statusCmp) model() string {
	t := theme.CurrentTheme()

	cfg := config.Get()
	if cfg == nil {
		return "Unknown"
	}

	coder, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return "Unknown"
	}
	model := models.SupportedModels[coder.Model]

	return styles.Padded().
		Background(t.Secondary()).
		Foreground(t.Background()).
		Render(model.Name)
}

func NewStatusCmp(lspClients map[string]*lsp.Client) StatusCmp {
	helpWidget = getHelpWidget()

	return &statusCmp{
		messageTTL: 10 * time.Second,
		lspClients: lspClients,
	}
}

// LiveHelperCostMsg carries the in-flight helper spend to the footer. Zero when
// no run is live, which is why the footer falls straight back to the session's
// own total the moment a run finishes and the rollup lands.
type LiveHelperCostMsg float64
