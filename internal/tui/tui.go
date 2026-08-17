package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/commands"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
	"github.com/opencode-ai/opencode/internal/llm/tools/shell"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/quota"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/components/core"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/page"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type keyMap struct {
	Logs            key.Binding
	Quit            key.Binding
	Help            key.Binding
	SwitchSession   key.Binding
	Commands        key.Binding
	Filepicker      key.Binding
	Models          key.Binding
	SwitchTheme     key.Binding
	ToggleSelection key.Binding
}

// ReopenProviderPortal reopens the every-launch provider picker from inside a
// session. Set by cmd at startup; nil in tests and in any build that does not
// wire it.
//
// GORILLA OVERRIDE: the escape hatch. The portal only ran at launch, so a
// provider that turned out not to work left the user stuck unless they knew to
// quit and relaunch — and the people this is built for are exactly the ones who
// do not know that. /connect, /login and /model between them can do the same
// job, but they are three commands and none of them is the screen the user was
// just shown.
//
// It is a hook rather than a direct call because internal/tui cannot import
// cmd: cmd imports this package.
var ReopenProviderPortal func() error

// portalExec runs the provider portal while bubbletea has released the
// terminal. The portal is its own tea.Program and needs the screen to itself;
// tea.Exec is the same mechanism the editor already uses for $EDITOR.
type portalExec struct{ run func() error }

func (p portalExec) Run() error        { return p.run() }
func (portalExec) SetStdin(io.Reader)  {}
func (portalExec) SetStdout(io.Writer) {}
func (portalExec) SetStderr(io.Writer) {}

type startCompactSessionMsg struct{}

const (
	quitKey = "q"
)

var keys = keyMap{
	Logs: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("ctrl+l", "logs"),
	),

	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("ctrl+_", "ctrl+h"),
		key.WithHelp("ctrl+?", "toggle help"),
	),

	SwitchSession: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "switch session"),
	),

	Commands: key.NewBinding(
		key.WithKeys("ctrl+k"),
		key.WithHelp("ctrl+k", "commands"),
	),
	Filepicker: key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", "select files to upload"),
	),
	Models: key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("ctrl+o", "model selection"),
	),

	SwitchTheme: key.NewBinding(
		key.WithKeys("ctrl+t"),
		key.WithHelp("ctrl+t", "switch theme"),
	),
	ToggleSelection: key.NewBinding(
		key.WithKeys("ctrl+y"),
		key.WithHelp("ctrl+y", "toggle selection mode"),
	),
}

var helpEsc = key.NewBinding(
	key.WithKeys("?"),
	key.WithHelp("?", "toggle help"),
)

var returnKey = key.NewBinding(
	key.WithKeys("esc"),
	key.WithHelp("esc", "close"),
)

var logsKeyReturnKey = key.NewBinding(
	key.WithKeys("esc", "backspace", quitKey),
	key.WithHelp("esc/q", "go back"),
)

type appModel struct {
	width, height int
	// GORILLA OVERRIDE: scrollback means the alternate screen is OFF, so the
	// conversation is printed into the terminal's own output and only the prompt is
	// drawn in place. Dialogs still need a whole screen, so they are shown by
	// entering the alternate screen briefly and leaving it again — see
	// anyOverlayOpen in overlay_state.go.
	scrollback bool
	// bannerShown keeps the identity banner to exactly one printing per session.
	bannerShown bool
	// quotaFractions is the last-seen remaining fraction per Antigravity quota
	// group, seeded by the session-start fetch. After each completed response
	// the tier check compares a fresh reading against these and announces any
	// banana-tier crossing — otherwise a session that burns through 50% in one
	// long tool loop crosses every threshold invisibly between /usage calls.
	quotaFractions  map[string]float64
	lastQuotaCheck  time.Time
	selectionMode   bool
	currentPage     page.PageID
	previousPage    page.PageID
	pages           map[page.PageID]tea.Model
	loadedPages     map[page.PageID]bool
	status          core.StatusCmp
	app             *app.App
	selectedSession session.Session

	showPermissions bool
	permissions     dialog.PermissionDialogCmp

	showHelp bool
	help     dialog.HelpCmp

	showQuit bool
	quit     dialog.QuitDialog

	showSessionDialog bool
	sessionDialog     dialog.SessionDialog

	showCommandDialog bool
	commandDialog     dialog.CommandDialog
	// commandHelp is the /help and /commands plain-language reference.
	// GORILLA OVERRIDE.
	commandHelp dialog.CommandHelpDialog
	commands    []dialog.Command

	showModelDialog bool
	modelDialog     dialog.ModelDialog

	// GORILLA OVERRIDE: context loadout menu (/context)
	showLoadoutDialog bool
	loadoutDialog     dialog.LoadoutDialog

	// GORILLA OVERRIDE: live sub-agent monitor (/tasks)
	showTasksDialog bool
	tasksDialog     dialog.TasksDialog

	// GORILLA OVERRIDE: provider connection manager (/connect)
	showConnectDialog bool
	connectDialog     dialog.ConnectDialog

	// GORILLA OVERRIDE: /add-dir and /cd — workspace roots.
	showAddDirDialog bool
	showCommandHelp  bool
	// GORILLA OVERRIDE: /export now asks where to write and what to call it.
	showExportDialog bool
	// loginURL is the pending sign-in URL, shown as an overlay. GORILLA OVERRIDE.
	loginURL     string
	addDirDialog dialog.AddDirDialog
	exportDialog dialog.ExportDialog

	// GORILLA OVERRIDE: /prompts — view, edit, section-toggle the system prompts.
	showPromptsDialog bool
	promptsDialog     dialog.PromptsDialog

	// GORILLA OVERRIDE: /reset — scoped undo of configuration changes.
	showResetDialog bool
	resetDialog     dialog.ResetDialog

	// GORILLA OVERRIDE: /settings — every tunable option with its range and default.
	showSettingsDialog bool
	settingsDialog     dialog.SettingsDialog

	showInitDialog bool

	// GORILLA OVERRIDE: /research mode chooser.
	researchDialog     dialog.ResearchDialogCmp
	showResearchDialog bool

	// GORILLA OVERRIDE: /osint — the serious dossier gate and its capability page.
	osintDialog     dialog.OsintDialogCmp
	showOsintDialog bool
	osintPage       dialog.OsintPageCmp
	showOsintPage   bool

	// GORILLA OVERRIDE: "your background helpers moved too" — shown after a
	// model switch drags summarizer/task/title/research along, with a revert.
	modelFollowDialog     dialog.ModelFollowDialogCmp
	showModelFollowDialog bool
	initDialog            dialog.InitDialogCmp

	showFilepicker bool
	filepicker     dialog.FilepickerCmp

	showThemeDialog bool
	themeDialog     dialog.ThemeDialog

	showMultiArgumentsDialog bool
	multiArgumentsDialog     dialog.MultiArgumentsDialogCmp

	isCompacting      bool
	compactingMessage string
}

func (a appModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmd := a.pages[a.currentPage].Init()
	a.loadedPages[a.currentPage] = true
	cmds = append(cmds, cmd)
	cmd = a.status.Init()
	cmds = append(cmds, cmd)
	cmd = a.quit.Init()
	cmds = append(cmds, cmd)
	cmd = a.help.Init()
	cmds = append(cmds, cmd)
	cmd = a.sessionDialog.Init()
	cmds = append(cmds, cmd)
	cmd = a.commandDialog.Init()
	cmds = append(cmds, cmd)
	cmd = a.modelDialog.Init()
	cmds = append(cmds, cmd)
	cmd = a.connectDialog.Init()
	cmds = append(cmds, cmd)
	cmd = a.initDialog.Init()
	cmds = append(cmds, cmd)
	cmd = a.filepicker.Init()
	cmds = append(cmds, cmd)
	cmd = a.themeDialog.Init()
	cmds = append(cmds, cmd)

	// Check if we should show the init dialog
	cmds = append(cmds, func() tea.Msg {
		shouldShow, err := config.ShouldShowInitDialog()
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "Failed to check init status: " + err.Error(),
			}
		}
		return dialog.ShowInitDialogMsg{Show: shouldShow}
	})

	// GORILLA OVERRIDE: on session start, if signed in to Antigravity, fetch the
	// weekly quota once and surface it in the status bar — the same one-line view
	// /usage shows, automatically. SILENT (nil msg) when not signed in, so
	// non-Antigravity users never see a thing. Runs in the Cmd goroutine, so a
	// slow fetch on a high-latency link never blocks startup.
	cmds = append(cmds, antigravityUsageCmd(true))

	return tea.Batch(cmds...)
}

// antigravityUsageCmd fetches the Antigravity weekly quota and returns it as an
// InfoMsg for the status bar. It runs in the returned Cmd's goroutine, so the
// network call never blocks the UI, and the result is a tea.Msg — never printed
// onto the screen Bubble Tea owns. quiet=true returns nil when not signed in or
// on a transient error (used at session start, so non-Antigravity users see
// formatQuotaScrollbackLine renders the history entry.
//
// Extracted from the switch so it can be tested: inline formatting inside a
// tea.Msg case is unreachable from a test, and an untested string is one
// refactor away from being silently emptied. The timestamp is mandatory - a
// quota figure without a time is not a measurement, and two dated readings are
// what give you a burn rate.
// GORILLA OVERRIDE: quota alert lines are rendered bold + bright red (#FF0000)
// so they stand out in the scrollback. Plain text is invisible against normal
// terminal output; the intent of the warn-colour change (0787f7b) was that
// these messages scream, not whisper.
var quotaAlertStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF0000"))

func formatQuotaScrollbackLine(at time.Time, line string) string {
	return quotaAlertStyle.Render("  " + at.Format("15:04:05") + "  quota · " + line)
}

// quotaLineMsg carries a fetched quota reading.
//
// GORILLA OVERRIDE: /usage used to render only as a footer toast, which is
// ephemeral - to see the number again you had to type /usage again, and that
// costs another request against the very quota you are trying to conserve.
// A reading you can scroll back to is strictly better: "what was left twenty
// minutes ago" is answerable for free.
//
// So the line is ALSO printed into the scrollback, timestamped, because a quota
// figure without a time is not a measurement. The toast stays for immediacy.
type quotaLineMsg struct {
	line string
	kind util.InfoType
	// summary, when non-nil, renders the full Models & Quota panel into the
	// scrollback (bars, plain-language "left/used", green→red colour). Set only
	// on an explicit /usage: the automatic session-start reading stays one line
	// so it never floods the top of every conversation.
	summary *auth.QuotaSummary
	account string
	// balances are paid-provider readings (DeepSeek, OpenRouter) for providers
	// the user has a key for. Also /usage-only: each is a network call.
	balances []quota.Reading
	// fractions seeds/updates the crossing-alert baseline on every reading.
	fractions map[string]float64
}

// quotaAlertMsg carries banana-tier crossings detected by the post-response
// check, plus the fresh fractions to store as the new baseline.
type quotaAlertMsg struct {
	alerts    []string
	fractions map[string]float64
}

// quotaCheckMinInterval throttles the post-response quota check. A tool-heavy
// turn can complete several responses a minute; one small request every half
// minute is enough to catch a crossing while it is happening.
const quotaCheckMinInterval = 30 * time.Second

// quotaTierCheckCmd fetches a fresh quota reading and compares it against the
// previous fractions. Silent on every failure path: an alert system that nags
// about its own plumbing is worse than none.
func quotaTierCheckCmd(prev map[string]float64) tea.Cmd {
	return func() tea.Msg {
		creds, _ := auth.LoadAntigravityCreds()
		if creds == nil || creds.AccessToken == "" {
			return nil
		}
		q, err := creds.RetrieveQuota(context.Background())
		if err != nil {
			return nil
		}
		alerts, next := bananaAlerts(prev, q)
		return quotaAlertMsg{alerts: alerts, fractions: next}
	}
}

// configuredBalances fetches a balance reading for every supported provider
// the user actually has a key for — config first, then the environment (the
// same two places the agent's own requests resolve a key from).
func configuredBalances(ctx context.Context) []quota.Reading {
	cfg := config.Get()
	envKeys := map[string]string{
		"deepseek":   os.Getenv("DEEPSEEK_API_KEY"),
		"openrouter": os.Getenv("OPENROUTER_API_KEY"),
	}
	var out []quota.Reading
	for _, id := range quota.Supported() {
		key := ""
		if cfg != nil {
			if p, ok := cfg.Providers[models.ModelProvider(id)]; ok && !p.Disabled {
				key = p.APIKey
			}
		}
		if key == "" {
			key = envKeys[id]
		}
		if key == "" {
			continue
		}
		out = append(out, quota.Fetch(ctx, id, key))
	}
	return out
}

// nothing); quiet=false reports the reason (used by /usage on demand).
func antigravityUsageCmd(quiet bool) tea.Cmd {
	return func() tea.Msg {
		creds, _ := auth.LoadAntigravityCreds()
		if creds == nil || creds.AccessToken == "" {
			if quiet {
				return nil
			}
			// No Antigravity, but a paid provider's balance is still worth a
			// panel — a DeepSeek-only user asked /usage about THEIR meter.
			msg := quotaLineMsg{line: "Not signed in to Antigravity — pick it in the provider portal to sign in.", kind: util.InfoTypeWarn}
			msg.balances = configuredBalances(context.Background())
			return msg
		}
		q, err := creds.RetrieveQuota(context.Background())
		if err != nil {
			if quiet {
				return nil
			}
			return quotaLineMsg{line: "Antigravity usage: " + err.Error(), kind: util.InfoTypeError}
		}
		msg := quotaLineMsg{line: auth.FormatQuotaLine(q, time.Now()), kind: util.InfoTypeInfo}
		_, msg.fractions = bananaAlerts(nil, q) // seed/update the alert baseline
		if !quiet {
			msg.summary = q
			msg.account = creds.Email
			msg.balances = configuredBalances(context.Background())
		}
		return msg
	}
}

// Update wraps the real update so the terminal buffer can follow the dialogs.
//
// GORILLA OVERRIDE: dialogs are written to paint a whole screen, and outside the
// alternate screen there is no whole screen to paint — only a short footer above
// the printed conversation. So opening any dialog enters the alternate screen and
// closing it leaves again, which is why this is a wrapper: the decision is made
// once, from the overlay state before and after, rather than at each of the ~18
// places a dialog is opened. Doing it at the call sites is the version that gets
// one path right and forgets another.
func (a appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	wasOpen := a.anyOverlayOpen()

	updated, cmd := a.update(msg)

	next, ok := updated.(appModel)
	if !ok {
		return updated, cmd
	}
	if buffer := next.bufferCmd(wasOpen); buffer != nil {
		cmd = tea.Batch(cmd, buffer)
	}
	return next, cmd
}

func (a appModel) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.MouseMsg:
		// GORILLA FIX: only wheel events drive scrolling. Motion/press/
		// release events are unused by any component (verified: nothing
		// reads them, no bubblezone click handling). Forwarding every
		// motion event — e.g. a mouse drag while trying to select text —
		// through the full status+dialog+page update chain saturated the
		// event loop. That lag let bubbletea's stdin parser fall behind and
		// leak raw SGR mouse codes (ESC[<..M) into the editor, and made the
		// view stutter/jump. Dropping non-wheel events keeps wheel scroll
		// working while eliminating the flood.
		if !tea.MouseEvent(msg).IsWheel() {
			return a, nil
		}
		a.pages[a.currentPage], cmd = a.pages[a.currentPage].Update(msg)
		return a, cmd
	case tea.WindowSizeMsg:
		statusHeight := lipgloss.Height(a.status.View())
		if statusHeight < 1 {
			statusHeight = 1
		}
		msg.Height -= statusHeight
		a.width, a.height = msg.Width, msg.Height

		// GORILLA OVERRIDE: nothing is scrolled or padded here. The frame is drawn
		// at the cursor, immediately after the last printed line, exactly as a
		// shell prompt is — see session_banner.go for why the old bottom-pinning
		// was removed rather than repaired.
		if cmd := a.bannerCmd(); cmd != nil {
			cmds = append(cmds, cmd)
		}

		s, _ := a.status.Update(msg)
		a.status = s.(core.StatusCmp)
		a.pages[a.currentPage], cmd = a.pages[a.currentPage].Update(msg)
		cmds = append(cmds, cmd)

		prm, permCmd := a.permissions.Update(msg)
		a.permissions = prm.(dialog.PermissionDialogCmp)
		cmds = append(cmds, permCmd)

		help, helpCmd := a.help.Update(msg)
		a.help = help.(dialog.HelpCmp)
		cmds = append(cmds, helpCmd)

		session, sessionCmd := a.sessionDialog.Update(msg)
		a.sessionDialog = session.(dialog.SessionDialog)
		cmds = append(cmds, sessionCmd)

		command, commandCmd := a.commandDialog.Update(msg)
		a.commandDialog = command.(dialog.CommandDialog)
		cmds = append(cmds, commandCmd)

		filepicker, filepickerCmd := a.filepicker.Update(msg)
		a.filepicker = filepicker.(dialog.FilepickerCmp)
		cmds = append(cmds, filepickerCmd)

		// GORILLA OVERRIDE: the loadout and model dialogs need the
		// terminal width to render full-width; feed them the size even
		// while hidden so it's set before they open.
		loadoutModel, loadoutCmd := a.loadoutDialog.Update(msg)
		a.loadoutDialog = loadoutModel.(dialog.LoadoutDialog)
		cmds = append(cmds, loadoutCmd)

		tasksModel, tasksSizeCmd := a.tasksDialog.Update(msg)
		a.tasksDialog = tasksModel.(dialog.TasksDialog)
		cmds = append(cmds, tasksSizeCmd)

		modelModel, modelSizeCmd := a.modelDialog.Update(msg)
		a.modelDialog = modelModel.(dialog.ModelDialog)
		cmds = append(cmds, modelSizeCmd)

		addDirModel, addDirSizeCmd := a.addDirDialog.Update(msg)
		a.addDirDialog = addDirModel.(dialog.AddDirDialog)
		cmds = append(cmds, addDirSizeCmd)

		exportModel, exportSizeCmd := a.exportDialog.Update(msg)
		a.exportDialog = exportModel.(dialog.ExportDialog)
		cmds = append(cmds, exportSizeCmd)

		promptsModel, promptsSizeCmd := a.promptsDialog.Update(msg)
		a.promptsDialog = promptsModel.(dialog.PromptsDialog)
		cmds = append(cmds, promptsSizeCmd)

		resetModel, resetSizeCmd := a.resetDialog.Update(msg)
		a.resetDialog = resetModel.(dialog.ResetDialog)
		cmds = append(cmds, resetSizeCmd)

		settingsModel, settingsSizeCmd := a.settingsDialog.Update(msg)
		a.settingsDialog = settingsModel.(dialog.SettingsDialog)
		cmds = append(cmds, settingsSizeCmd)

		connectModel, connectSizeCmd := a.connectDialog.Update(msg)
		a.connectDialog = connectModel.(dialog.ConnectDialog)
		cmds = append(cmds, connectSizeCmd)

		a.initDialog.SetSize(msg.Width, msg.Height)

		if a.showMultiArgumentsDialog {
			a.multiArgumentsDialog.SetSize(msg.Width, msg.Height)
			args, argsCmd := a.multiArgumentsDialog.Update(msg)
			a.multiArgumentsDialog = args.(dialog.MultiArgumentsDialogCmp)
			cmds = append(cmds, argsCmd, a.multiArgumentsDialog.Init())
		}

		return a, tea.Batch(cmds...)
	// GORILLA OVERRIDE: a quota reading goes into the scrollback as well as the
	// footer. The footer answers "what is it now"; the scrollback answers "what
	// was it before", which the footer cannot do at any price - re-asking spends
	// a request against the quota being measured.
	//
	// tea.Println is the only way to write above the inline frame. Printing to
	// stdout directly is painted over by the next render with no record in the
	// renderer, and no redraw can ever clear it (see the trap list in CLAUDE.md).
	case quotaLineMsg:
		if len(msg.fractions) > 0 {
			a.quotaFractions = msg.fractions
		}
		if a.scrollback {
			out := formatQuotaScrollbackLine(time.Now(), msg.line)
			if msg.summary != nil || len(msg.balances) > 0 {
				out += "\n\n" + renderQuotaPanel(msg.summary, msg.account, msg.balances, a.width, time.Now()) + "\n"
			}
			cmds = append(cmds, tea.Println(out))
		}
		info := util.InfoMsg{Type: msg.kind, Msg: msg.line}
		st, cmd := a.status.Update(info)
		a.status = st.(core.StatusCmp)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)

	// GORILLA OVERRIDE: banana-tier crossings, announced as they happen — in
	// the scrollback with the gorilla, and on the footer status bar with the
	// emoji stripped (the frame is where emoji width mismatches strand
	// debris; see the trap list).
	case quotaAlertMsg:
		if len(msg.fractions) > 0 {
			a.quotaFractions = msg.fractions
		}
		if a.scrollback {
			for _, al := range msg.alerts {
				cmds = append(cmds, tea.Println(formatQuotaScrollbackLine(time.Now(), al)))
			}
		}
		if len(msg.alerts) > 0 {
			info := util.InfoMsg{
				Type: util.InfoTypeWarn,
				Msg:  stripBananaEmoji(strings.Join(msg.alerts, " · ")),
				TTL:  15 * time.Second,
			}
			st, cmd := a.status.Update(info)
			a.status = st.(core.StatusCmp)
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	// Status
	case util.InfoMsg:
		s, cmd := a.status.Update(msg)
		a.status = s.(core.StatusCmp)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)
	case pubsub.Event[logging.LogMessage]:
		if msg.Payload.Persist {
			switch msg.Payload.Level {
			case "error":
				s, cmd := a.status.Update(util.InfoMsg{
					Type: util.InfoTypeError,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				})
				a.status = s.(core.StatusCmp)
				cmds = append(cmds, cmd)
			case "info":
				s, cmd := a.status.Update(util.InfoMsg{
					Type: util.InfoTypeInfo,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				})
				a.status = s.(core.StatusCmp)
				cmds = append(cmds, cmd)

			case "warn":
				s, cmd := a.status.Update(util.InfoMsg{
					Type: util.InfoTypeWarn,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				})

				a.status = s.(core.StatusCmp)
				cmds = append(cmds, cmd)
			default:
				s, cmd := a.status.Update(util.InfoMsg{
					Type: util.InfoTypeInfo,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				})
				a.status = s.(core.StatusCmp)
				cmds = append(cmds, cmd)
			}
		}
	case util.ClearStatusMsg:
		s, _ := a.status.Update(msg)
		a.status = s.(core.StatusCmp)

	// Permission
	case pubsub.Event[permission.PermissionRequest]:
		a.showPermissions = true
		return a, a.permissions.SetPermissions(msg.Payload)
	case dialog.PermissionResponseMsg:
		var cmd tea.Cmd
		switch msg.Action {
		case dialog.PermissionAllow:
			a.app.Permissions.Grant(msg.Permission)
		case dialog.PermissionAllowForSession:
			a.app.Permissions.GrantPersistant(msg.Permission)
		case dialog.PermissionDeny:
			a.app.Permissions.Deny(msg.Permission)
		}
		a.showPermissions = false
		return a, cmd

	case page.PageChangeMsg:
		return a, a.moveToPage(msg.ID)

	case dialog.CloseQuitMsg:
		a.showQuit = false
		return a, nil

	case dialog.CloseSessionDialogMsg:
		a.showSessionDialog = false
		return a, nil

	case dialog.CloseCommandDialogMsg:
		a.showCommandDialog = false
		return a, nil

	case startCompactSessionMsg:
		// Start compacting the current session
		a.isCompacting = true
		a.compactingMessage = "Starting summarization..."

		if a.selectedSession.ID == "" {
			a.isCompacting = false
			return a, util.ReportWarn("No active session to summarize")
		}

		// Start the summarization process
		return a, func() tea.Msg {
			ctx := context.Background()
			a.app.CoderAgent.Summarize(ctx, a.selectedSession.ID)
			return nil
		}

	case pubsub.Event[agent.AgentEvent]:
		payload := msg.Payload
		if payload.Error != nil {
			a.isCompacting = false
			return a, util.ReportError(payload.Error)
		}

		a.compactingMessage = payload.Progress

		if payload.Done && payload.Type == agent.AgentEventTypeSummarize {
			a.isCompacting = false
			return a, util.ReportInfo("Session summarization complete")
		} else if payload.Done && payload.Type == agent.AgentEventTypeResponse && a.selectedSession.ID != "" {
			// GORILLA OVERRIDE: after each completed response, re-read the
			// quota (throttled) and announce any banana-tier crossing. Without
			// this, a tool-heavy session can fall from "loaded up" to "just a
			// few" entirely between two /usage calls — observed live: 59% to
			// 30% in five minutes, every threshold crossed invisibly. Gated on
			// quotaFractions so non-Antigravity users never pay the check.
			if len(a.quotaFractions) > 0 && time.Since(a.lastQuotaCheck) > quotaCheckMinInterval {
				a.lastQuotaCheck = time.Now()
				prev := make(map[string]float64, len(a.quotaFractions))
				for k, v := range a.quotaFractions {
					prev[k] = v
				}
				cmds = append(cmds, quotaTierCheckCmd(prev))
			}
			model := a.app.CoderAgent.Model()
			contextWindow := model.ContextWindow
			tokens := a.selectedSession.CompletionTokens + a.selectedSession.PromptTokens
			if (tokens >= int64(float64(contextWindow)*0.95)) && config.Get().AutoCompact {
				cmds = append(cmds, util.CmdHandler(startCompactSessionMsg{}))
			}
		}
		// Continue listening for events
		return a, tea.Batch(cmds...)

	case dialog.CloseThemeDialogMsg:
		a.showThemeDialog = false
		return a, nil

	case dialog.ThemeChangedMsg:
		a.pages[a.currentPage], cmd = a.pages[a.currentPage].Update(msg)
		a.showThemeDialog = false
		return a, tea.Batch(cmd, util.ReportInfo("Theme changed to: "+msg.ThemeName))

	case dialog.CloseModelDialogMsg:
		a.showModelDialog = false
		return a, nil

	case dialog.CloseConnectDialogMsg:
		a.showConnectDialog = false
		return a, nil

	case dialog.ConnectionChangedMsg:
		// Connection was added/toggled; models are already live in the
		// registry. Keep the dialog open so the user can add more.
		return a, util.ReportInfo(msg.Info)

	case dialog.CloseAddDirDialogMsg:
		a.showAddDirDialog = false
		return a, nil

	case dialog.CloseExportDialogMsg:
		a.showExportDialog = false
		return a, nil

	case dialog.ExportConfirmedMsg:
		a.showExportDialog = false
		return a, a.writeExport(msg.Dir, msg.Name)

	case dialog.ClosePromptsDialogMsg:
		a.showPromptsDialog = false
		return a, nil

	case dialog.CloseResetDialogMsg:
		a.showResetDialog = false
		return a, nil

	case dialog.CloseSettingsDialogMsg:
		a.showSettingsDialog = false
		return a, nil

	// A setting change may alter the prompt (contextPaths) or the tool set, so it
	// goes through the same rebuild path as everything else and reports honestly
	// when a turn is in flight.
	case dialog.SettingsChangedMsg:
		if msg.InvalidateCtx {
			prompt.InvalidateContextCache()
		}
		info := msg.Info
		if a.app.ReloadCoderTools() {
			info += " — takes effect after the current turn finishes"
		}
		return a, util.ReportInfo(info)

	// A prompt edit or section toggle changes the system prompt, so the provider
	// must be rebuilt for it to reach the model. ReloadCoderTools reports whether
	// it had to defer because a turn is in flight (P4).
	case dialog.PromptsChangedMsg:
		info := msg.Info
		if a.app.ReloadCoderTools() {
			info += " — takes effect after the current turn finishes"
		}
		return a, util.ReportInfo(info)

	// A reset can touch roots (context files) and prompts, so invalidate the
	// context cache as well as rebuilding the provider.
	case dialog.ResetAppliedMsg:
		prompt.InvalidateContextCache()
		info := msg.Info
		if a.app.ReloadCoderTools() {
			info += " — takes effect after the current turn finishes"
		}
		return a, util.ReportInfo(info)

	// GORILLA OVERRIDE: a root change must reach the MODEL, not just the config
	// file. Three things have to happen or the control is a silent no-op:
	//   1. the project-context cache is invalidated, so the new root's CLAUDE.md
	//      is actually read (P1 exists precisely for this)
	//   2. the provider is rebuilt so the env block re-renders with the new root
	//      list (ReloadCoderTools, deferred if a turn is in flight — P4)
	//   3. on a PRIMARY change, the persistent bash shell is torn down; it holds
	//      its own cwd from spawn time and would otherwise keep running commands
	//      in the previous directory while everything else believed it had moved
	case dialog.RootsChangedMsg:
		prompt.InvalidateContextCache()
		if msg.PrimaryChanged {
			shell.ResetPersistentShell(config.WorkingDirectory())
		}
		info := msg.Info
		if a.app.ReloadCoderTools() {
			info += " — takes effect after the current turn finishes"
		}
		return a, util.ReportInfo(info)

	case dialog.RunGoogleLoginMsg:
		return a, a.runLogin()

	// GORILLA OVERRIDE: /connect `u` — close the connect dialog, open the
	// model picker pre-scrolled to the requested provider's tab. Replaces
	// the "close, open /model, arrow across three tabs" ritual with one keypress.
	case dialog.UseProviderMsg:
		a.showConnectDialog = false
		a.modelDialog.Init()
		a.modelDialog.SwitchToProvider(msg.Provider)
		a.showModelDialog = true
		return a, util.ReportInfo(fmt.Sprintf("Switched to %s — pick a model", msg.Provider))

	case dialog.ModelSelectedMsg:
		a.showModelDialog = false

		// Capture the outgoing coder model BEFORE the update, so the helper
		// agents that were shadowing it can be identified afterwards.
		prevCoder := config.Get().Agents[config.AgentCoder].Model

		model, err := a.app.CoderAgent.Update(config.AgentCoder, msg.Model.ID)
		if err != nil {
			return a, util.ReportError(err)
		}

		// GORILLA FIX: bring the helper agents along. Changing the coder used to
		// leave summarizer/task/title on the old model — invisible until a title
		// failed, or worse, until summarisation was needed mid-session.
		note := fmt.Sprintf("Model changed to %s", model.Name)
		moves, ferr := config.FollowCoderModel(prevCoder, msg.Model.ID)
		if ferr != nil {
			// Not fatal: the coder switch already succeeded and is what was asked
			// for. Say so rather than failing the whole action silently.
			return a, util.ReportInfo(note + fmt.Sprintf(" — but the background agents could not be moved: %v", ferr))
		}
		if len(moves) == 0 {
			return a, util.ReportInfo(note)
		}
		// GORILLA FIX: a status note cannot carry this. Four agents just changed
		// model, which changes what they cost, which quota they draw and how
		// good the research will be. Show it, itemised, with a way back.
		d, _ := dialog.NewModelFollowDialogCmp(moves).
			Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
		a.modelFollowDialog = d.(dialog.ModelFollowDialogCmp)
		a.showModelFollowDialog = true
		return a, util.ReportInfo(note)

	case dialog.CloseModelFollowDialogMsg:
		a.showModelFollowDialog = false
		if msg.Reverted {
			return a, util.ReportInfo("Background agents put back exactly as they were.")
		}
		return a, nil

	case dialog.ShowInitDialogMsg:
		a.showInitDialog = msg.Show
		return a, nil

	case dialog.CloseResearchDialogMsg:
		a.showResearchDialog = false
		if !msg.Chosen {
			return a, nil
		}
		// Instruct rather than call directly: the tool belongs to the agent, and
		// routing through it keeps the findings in the conversation where the
		// model can act on them.
		prompt := fmt.Sprintf(
			"Use the research tool to investigate the following. "+
				"Set mode=%q and agents=%d exactly as given — the user chose these and they decide what this costs. "+
				"Pass everything already established in this conversation as `context` so no helper pays to re-derive it. "+
				"When the helpers report, check at least one load-bearing claim yourself, carry the evidence tiers through, "+
				"and say plainly what nobody established.\n\nQUESTION: %s",
			msg.Mode, msg.Agents, msg.Question)
		return a, util.CmdHandler(chat.SendMsg{Text: prompt})

	case dialog.CloseOsintDialogMsg:
		a.showOsintDialog = false
		if !msg.Chosen {
			return a, nil
		}
		// GORILLA OVERRIDE: the dossier product is WRITTEN OUTSIDE the working
		// folder, always. A working folder is often a git repo; a personal
		// question swept into a commit and pushed is the worst failure this
		// feature could have. ~/Documents/gorilla-dossiers is nobody's repo.
		prompt := fmt.Sprintf(
			"Use the research tool with doctrine=%q, mode=%q and agents=%d exactly as given — "+
				"the user chose these on the warning screen and they decide what this costs. "+
				"Pass everything already established in this conversation as `context` so no helper pays to re-derive it. "+
				"When the helpers report: run the gap check the tool's report demands, verify at least one load-bearing claim "+
				"yourself, carry every two-axis grade through unchanged, and assemble the dossier product "+
				"(BLUF first, graded claims, SOURCES TRIED, NOT ESTABLISHED, recommended action). "+
				"Then write the complete dossier as markdown to a NEW timestamped file under %q using the write tool "+
				"(create the folder if it is missing), tell the user the exact path, and give them the BLUF and key "+
				"findings in the conversation. Never write the dossier into the working folder: it may be a git "+
				"repository, and a private question must never end up in a commit.\n\nQUESTION: %s",
			"dossier", msg.Mode, msg.Agents, config.DossierDir(), msg.Question)
		return a, util.CmdHandler(chat.SendMsg{Text: prompt})

	case dialog.CloseOsintPageMsg:
		a.showOsintPage = false
		return a, nil

	case dialog.CloseInitDialogMsg:
		a.showInitDialog = false
		if msg.Initialize {
			// Run the initialization command
			for _, cmd := range a.commands {
				if cmd.ID == "init" {
					// Mark the project as initialized
					if err := config.MarkProjectInitialized(); err != nil {
						return a, util.ReportError(err)
					}
					return a, cmd.Handler(cmd)
				}
			}
		} else {
			// Mark the project as initialized without running the command
			if err := config.MarkProjectInitialized(); err != nil {
				return a, util.ReportError(err)
			}
		}
		return a, nil

	case chat.SlashCommandMsg:
		// GORILLA OVERRIDE: dispatch editor slash commands.
		switch msg.Name {
		// GORILLA OVERRIDE: /research asks HOW to run before spending anything.
		// The mode multiplies the bill (supervised is double), and the model
		// picking it from a schema the user never sees is the wrong place for
		// that decision.
		case "research":
			q := strings.TrimSpace(msg.Args)
			if q == "" {
				return a, util.ReportWarn("Give it something to investigate: /research does X actually work on this machine?")
			}
			a.researchDialog = dialog.NewResearchDialogCmp(q)
			a.researchDialog.SetSize(a.width, a.height)
			a.showResearchDialog = true
			return a, nil
		// GORILLA OVERRIDE: /osint is the SERIOUS research command — the full
		// dossier. Deliberately not an alias of /research: it is armed manually
		// in /context (ships off), warns with the computed burn rate before every
		// run, and writes its product OUTSIDE the working folder so a personal
		// question can never be swept into someone's git repo and pushed to the
		// internet. Bare /osint opens the capability page instead of a warning
		// toast — this command is the one that earns a full explanation.
		case "osint", "dossier":
			q := strings.TrimSpace(msg.Args)
			if q == "" {
				a.osintPage = dialog.NewOsintPageCmp()
				a.osintPage.SetSize(a.width, a.height)
				a.showOsintPage = true
				return a, nil
			}
			if !config.LoadoutEnabled(config.DossierComponentID) {
				return a, util.ReportWarn("The serious OSINT dossier is switched OFF (it burns real money, so it ships that way). Arm it: /context → \"" + config.DossierRowName + "\" → space. Or type /osint alone to read what it does first.")
			}
			a.osintDialog = dialog.NewOsintDialogCmp(q)
			a.osintDialog.SetSize(a.width, a.height)
			a.showOsintDialog = true
			return a, nil
		case "model", "models":
			a.modelDialog.Init()
			a.showModelDialog = true
			return a, nil
		// GORILLA OVERRIDE: /provider and /switch are natural aliases —
		// UPDATED 2026-08-05: /providers, /provider and /switch now open the
		// launch-time PICKER instead of this dialog (see the "providers" case
		// below). This dialog remains the detailed manager — add a local server,
		// toggle a connection off, remove one for good — which is a different job
		// from "the provider I picked does not work, show me the others".
		case "connect", "connections":
			a.connectDialog.Init()
			a.showConnectDialog = true
			return a, nil
		// GORILLA OVERRIDE: /add-dir manages workspace roots; /cd opens the same
		// dialog positioned to promote one. Adding a root does not grant access
		// (there is no sandbox) — it loads that root's context files, scopes
		// permissions to it, tells the model it exists, and watches it for
		// diagnostics.
		case "add-dir", "adddir", "dirs", "roots":
			a.addDirDialog.Init()
			a.showAddDirDialog = true
			return a, nil

		// GORILLA OVERRIDE: /cd narrows the workspace to one directory, in one
		// step, with the path on the command line. This is the operation the
		// whole roots feature exists for: an agent started in a home directory
		// that holds a kernel tree and a browser tree will walk millions of
		// files, and the fix is to point it at ONE project.
		//
		// keepOld is false and SetWorkingDir additionally drops any root that
		// CONTAINS the new one, so this genuinely narrows instead of leaving the
		// wide tree in scope while reporting success. Bare /cd opens the dialog.
		case "cd":
			if msg.Args == "" {
				a.addDirDialog.Init()
				a.showAddDirDialog = true
				return a, nil
			}
			target, err := config.SetWorkingDir(msg.Args, false)
			if err != nil {
				return a, util.ReportError(err)
			}
			prompt.InvalidateContextCache()
			shell.ResetPersistentShell(config.WorkingDirectory())
			info := fmt.Sprintf("workspace is now %s", target)
			if a.app.ReloadCoderTools() {
				info += " — takes effect after the current turn finishes"
			}
			return a, util.ReportInfo(info)
		// GORILLA OVERRIDE: /prompts — see exactly what the AI is told, edit it
		// in $EDITOR, or switch individual sections of the coder prompt off.
		case "prompts", "prompt":
			a.promptsDialog.Init()
			a.showPromptsDialog = true
			return a, nil
		// GORILLA OVERRIDE: /reset — undo config changes by scope. Never touches
		// credentials (that is /connect) or sessions (that is /clear).
		case "reset", "defaults":
			a.resetDialog.Init()
			a.showResetDialog = true
			return a, nil
		// GORILLA OVERRIDE: /settings — every option, its plain-language
		// description, what it accepts, and its default, all on screen.
		case "settings", "config", "prefs":
			a.settingsDialog.Init()
			a.showSettingsDialog = true
			return a, nil
		case "export":
			return a, a.openExportDialog()
		// GORILLA OVERRIDE: /plain switches to the copyable interface. It cannot
		// take effect now — the renderer is already running and owns the screen —
		// so it records the preference and says plainly that it applies next
		// launch, rather than appearing to do nothing. The preference is what makes
		// the mode reachable from the desktop icon, which passes no flags.
		case "plain", "copy", "copyable":
			if err := config.SetInterfaceMode(config.InterfacePlain); err != nil {
				return a, util.ReportError(err)
			}
			return a, util.ReportInfo("Plain mode is set — quit and start again to use it. Everything will be ordinary terminal text you can select and copy. Use /settings to switch back.")
		case "clear", "new":
			// GORILLA OVERRIDE: /clear starts a fresh session, dropping
			// the accumulated context. Routed through the chat page's
			// full new-session flow so the editor/sidebar reset cleanly.
			a.selectedSession = session.Session{}
			return a, util.CmdHandler(chat.NewSessionMsg{})
		case "context", "loadout", "tokens":
			a.showLoadoutDialog = true
			return a, nil
		case "task", "tasks", "agents", "kill":
			// GORILLA OVERRIDE: /tasks — live monitor of running helper
			// agents; kill one, or the Nuclear Option (kill 'em all).
			a.showTasksDialog = true
			return a, nil
		// GORILLA OVERRIDE: /help and /commands open the plain-language command
		// reference. The program grew past the point where anyone could hold its
		// command list in their head, including the person who asked for them.
		case "help", "commands", "?":
			a.commandHelp.Init()
			a.commandHelp.SetSize(a.width, a.height)
			a.showCommandHelp = true
			return a, nil
		case "providers", "provider", "switch":
			// GORILLA OVERRIDE: the escape hatch, as a COMMAND rather than a key.
			//
			// A key binding was tried first and abandoned: ctrl+p is Print in most
			// GUI contexts and "previous line" in readline; ctrl+c is SIGINT (and
			// copy, for anyone who has rebound it); esc already cancels a running
			// turn here, so taking it would mean esc sometimes stops your request
			// and sometimes opens a menu. Nearly every remaining control key is
			// either bound in this app or reserved by the terminal (ctrl+z
			// suspend, ctrl+d EOF, ctrl+q/s flow control, ctrl+b tmux prefix).
			//
			// A slash command collides with nothing, appears in /help, and is the
			// idiom this app already teaches.
			if ReopenProviderPortal == nil {
				return a, util.ReportWarn("switching providers is not available in this build")
			}
			if a.app.CoderAgent != nil && a.app.CoderAgent.IsBusy() {
				return a, util.ReportWarn("finish or cancel the current turn before switching provider")
			}
			return a, tea.Exec(portalExec{run: ReopenProviderPortal}, func(err error) tea.Msg {
				if err != nil {
					return util.InfoMsg{Type: util.InfoTypeError, Msg: err.Error()}
				}
				return util.InfoMsg{
					Type: util.InfoTypeInfo,
					Msg:  "Provider updated — use /models if you want a different model from it.",
				}
			})
		case "usage":
			// GORILLA OVERRIDE: /usage — Antigravity weekly quota. Typed commands
			// dispatch through this switch, NOT the command palette, so a palette
			// RegisterCommand alone left `/usage` reported as "Unknown command".
			return a, antigravityUsageCmd(false)
		case "login":
			// GORILLA OVERRIDE: /login — run the browser OAuth flow
			// to sign in with Google (Code Assist free tier).
			return a, a.runLogin()
		case "logout":
			// GORILLA OVERRIDE: /logout — drop stored OAuth creds
			// and clear the gemini-oauth provider from config.
			return a, a.runLogout()
		default:
			// GORILLA OVERRIDE: suggest, then point at the reference. The old
			// message recited a dozen command names regardless of what was typed,
			// which is the least useful moment to hand someone a full list.
			if s := commands.Suggest(msg.Name, 3); len(s) > 0 {
				return a, util.ReportWarn(fmt.Sprintf("Unknown command /%s — did you mean /%s?  (/help lists them all)",
					msg.Name, strings.Join(s, ", /")))
			}
			return a, util.ReportWarn(fmt.Sprintf("Unknown command /%s — type /help to see every command and what it does", msg.Name))
		}

	case dialog.CloseCommandHelpMsg:
		a.showCommandHelp = false
		return a, nil

	case dialog.CloseLoadoutDialogMsg:
		a.showLoadoutDialog = false
		return a, nil

	case dialog.CloseTasksDialogMsg:
		a.showTasksDialog = false
		return a, nil

	case pubsub.Event[agent.SubAgentInfo]:
		// GORILLA OVERRIDE: transparency — surface helper spawn/exit. The
		// event itself also triggers a re-render, keeping the /tasks list and
		// status-bar count live while they're on screen.
		if msg.Type == pubsub.CreatedEvent {
			return a, util.ReportInfo(fmt.Sprintf("🦍 helper %s spawned — %s  (/tasks to view or kill)", msg.Payload.ID, truncatePrompt(msg.Payload.Prompt, 40)))
		}
		return a, nil

	case loginURLMsg:
		// Render it, do not print it. A dialog is part of the frame, so it can be
		// dismissed and redrawn; a stdout write cannot be taken back.
		a.loginURL = msg.URL
		return a, nil

	case loginResultMsg:
		// The sign-in finished (either way), so retire the URL overlay.
		a.loginURL = ""
		if msg.err != nil {
			return a, util.ReportError(fmt.Errorf("login failed: %w", msg.err))
		}
		// Re-run config validation so the new OAuth creds are picked up
		cfg := config.Get()
		if cfg != nil {
			if _, ok := cfg.Providers[models.ProviderGeminiCA]; !ok {
				cfg.Providers[models.ProviderGeminiCA] = config.Provider{APIKey: "oauth-login"}
			}
		}
		return a, util.ReportInfo(fmt.Sprintf("Signed in as %s. Select a Gemini Code Assist model to begin.", msg.Email))

	case logoutDoneMsg:
		if msg.err != nil {
			return a, util.ReportError(fmt.Errorf("logout failed: %w", msg.err))
		}
		// Remove the gemini-oauth provider so it won't be used
		cfg := config.Get()
		if cfg != nil {
			delete(cfg.Providers, models.ProviderGeminiCA)
		}
		return a, util.ReportInfo("Signed out. Use /login to sign in again, or type /model to pick a different provider.")

	case dialog.LoadoutChangedMsg:
		// Rebuild the coder agent's tools so toggles take effect now.
		// GORILLA OVERRIDE: this used to report the new token count
		// unconditionally. While a turn is in flight the system prompt cannot be
		// re-rendered, so the reported figure was not what the model would
		// actually receive on the current turn. Say so instead.
		deferred := a.app.ReloadCoderTools()
		return a, util.ReportInfo(withDeferredNote(
			fmt.Sprintf("Loadout: ~%d tokens/turn", config.LoadoutActiveTokens()), deferred))

	case dialog.ConfigChangedMsg:
		// GORILLA OVERRIDE: the single hot-apply path for every settings, roots
		// and prompt change. InvalidateCtx is set by anything that changes WHICH
		// files are in scope (workspace roots, contextPaths) — without it the
		// context cache holds and the change is invisible to the model.
		if msg.InvalidateCtx {
			prompt.InvalidateContextCache()
		}
		deferred := a.app.ReloadCoderTools()
		return a, util.ReportInfo(withDeferredNote(msg.Info, deferred))

	case chat.SessionSelectedMsg:
		a.selectedSession = msg
		a.sessionDialog.SetSelectedSession(msg.ID)

	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent && msg.Payload.ID == a.selectedSession.ID {
			a.selectedSession = msg.Payload
		}
	case dialog.SessionSelectedMsg:
		a.showSessionDialog = false
		if a.currentPage == page.ChatPage {
			return a, util.CmdHandler(chat.SessionSelectedMsg(msg.Session))
		}
		return a, nil

	case dialog.CommandSelectedMsg:
		a.showCommandDialog = false
		// Execute the command handler if available
		if msg.Command.Handler != nil {
			return a, msg.Command.Handler(msg.Command)
		}
		return a, util.ReportInfo("Command selected: " + msg.Command.Title)

	case dialog.ShowMultiArgumentsDialogMsg:
		// Show multi-arguments dialog
		a.multiArgumentsDialog = dialog.NewMultiArgumentsDialogCmp(msg.CommandID, msg.Content, msg.ArgNames)
		a.showMultiArgumentsDialog = true
		return a, a.multiArgumentsDialog.Init()

	case dialog.CloseMultiArgumentsDialogMsg:
		// Close multi-arguments dialog
		a.showMultiArgumentsDialog = false

		// If submitted, replace all named arguments and run the command
		if msg.Submit {
			content := msg.Content

			// Replace each named argument with its value
			for name, value := range msg.Args {
				placeholder := "$" + name
				content = strings.ReplaceAll(content, placeholder, value)
			}

			// Execute the command with arguments
			return a, util.CmdHandler(dialog.CommandRunCustomMsg{
				Content: content,
				Args:    msg.Args,
			})
		}
		return a, nil

	case tea.KeyMsg:
		// If multi-arguments dialog is open, let it handle the key press first
		if a.showMultiArgumentsDialog {
			args, cmd := a.multiArgumentsDialog.Update(msg)
			a.multiArgumentsDialog = args.(dialog.MultiArgumentsDialogCmp)
			return a, cmd
		}

		switch {

		case key.Matches(msg, keys.Quit):
			a.showQuit = !a.showQuit
			if a.showHelp {
				a.showHelp = false
			}
			if a.showSessionDialog {
				a.showSessionDialog = false
			}
			if a.showCommandDialog {
				a.showCommandDialog = false
			}
			if a.showFilepicker {
				a.showFilepicker = false
				a.filepicker.ToggleFilepicker(a.showFilepicker)
			}
			if a.showModelDialog {
				a.showModelDialog = false
			}
			if a.showLoadoutDialog {
				a.showLoadoutDialog = false
			}
			if a.showTasksDialog {
				a.showTasksDialog = false
			}
			if a.showConnectDialog {
				a.showConnectDialog = false
			}
			if a.showAddDirDialog {
				a.showAddDirDialog = false
			}
			if a.showExportDialog {
				a.showExportDialog = false
			}
			// NOT the sign-in overlay. Every other overlay here can simply be
			// reopened; the sign-in URL cannot — it would mean restarting /login.
			// So the quit key does not discard it. esc does, handled after the
			// dialog routing below.
			if a.showPromptsDialog {
				a.showPromptsDialog = false
			}
			if a.showResetDialog {
				a.showResetDialog = false
			}
			if a.showSettingsDialog {
				a.showSettingsDialog = false
			}
			if a.showMultiArgumentsDialog {
				a.showMultiArgumentsDialog = false
			}
			return a, nil
		case key.Matches(msg, keys.SwitchSession):
			if a.currentPage == page.ChatPage && !a.showQuit && !a.showPermissions && !a.showCommandDialog {
				// Load sessions and show the dialog
				// Load both views: the sessions started in this folder, and
				// every session. The dialog opens scoped to the current folder
				// and toggles locally, so one keypress switches without a
				// second query.
				ctx := context.Background()
				cwd := config.WorkingDirectory()
				all, err := a.app.Sessions.List(ctx)
				if err != nil {
					return a, util.ReportError(err)
				}
				scoped, err := a.app.Sessions.ListByDir(ctx, cwd)
				if err != nil {
					return a, util.ReportError(err)
				}
				if len(all) == 0 {
					return a, util.ReportWarn("No sessions available")
				}
				a.sessionDialog.SetSessions(scoped, all, cwd)
				a.showSessionDialog = true
				return a, nil
			}
			return a, nil
		case key.Matches(msg, keys.Commands):
			if a.currentPage == page.ChatPage && !a.showQuit && !a.showPermissions && !a.showSessionDialog && !a.showThemeDialog && !a.showFilepicker {
				// Show commands dialog
				if len(a.commands) == 0 {
					return a, util.ReportWarn("No commands available")
				}
				a.commandDialog.SetCommands(a.commands)
				a.showCommandDialog = true
				return a, nil
			}
			return a, nil
		case key.Matches(msg, keys.Models):
			if a.showModelDialog {
				a.showModelDialog = false
				return a, nil
			}
			if a.currentPage == page.ChatPage && !a.showQuit && !a.showPermissions && !a.showSessionDialog && !a.showCommandDialog {
				a.showModelDialog = true
				return a, nil
			}
			return a, nil
		case key.Matches(msg, keys.SwitchTheme):
			if !a.showQuit && !a.showPermissions && !a.showSessionDialog && !a.showCommandDialog {
				// Show theme switcher dialog
				a.showThemeDialog = true
				// Theme list is dynamically loaded by the dialog component
				return a, a.themeDialog.Init()
			}
			return a, nil
		case key.Matches(msg, keys.ToggleSelection):
			a.selectionMode = !a.selectionMode
			if a.selectionMode {
				fmt.Print("\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l")
				return a, util.ReportInfo("Selection mode ENABLED: Native mouse drag selection active")
			}
			fmt.Print("\x1b[?1000h\x1b[?1002h\x1b[?1006h")
			return a, util.ReportInfo("Selection mode DISABLED: TUI mouse scrolling active")
		case key.Matches(msg, returnKey) || key.Matches(msg):
			if msg.String() == quitKey {
				if a.currentPage == page.LogsPage {
					return a, a.moveToPage(page.ChatPage)
				}
			} else if !a.filepicker.IsCWDFocused() {
				if a.showQuit {
					a.showQuit = !a.showQuit
					return a, nil
				}
				if a.showHelp {
					a.showHelp = !a.showHelp
					return a, nil
				}
				if a.showResearchDialog {
					a.showResearchDialog = false
					return a, nil
				}
				if a.showOsintDialog {
					a.showOsintDialog = false
					return a, nil
				}
				if a.showOsintPage {
					a.showOsintPage = false
					return a, nil
				}
				if a.showInitDialog {
					a.showInitDialog = false
					// Mark the project as initialized without running the command
					if err := config.MarkProjectInitialized(); err != nil {
						return a, util.ReportError(err)
					}
					return a, nil
				}
				if a.showFilepicker {
					a.showFilepicker = false
					a.filepicker.ToggleFilepicker(a.showFilepicker)
					return a, nil
				}
				if a.currentPage == page.LogsPage {
					return a, a.moveToPage(page.ChatPage)
				}
			}
		case key.Matches(msg, keys.Logs):
			return a, a.moveToPage(page.LogsPage)

		case key.Matches(msg, keys.Help):
			if a.showQuit {
				return a, nil
			}
			a.showHelp = !a.showHelp
			return a, nil
		case key.Matches(msg, helpEsc):
			if a.app.CoderAgent.IsBusy() {
				if a.showQuit {
					return a, nil
				}
				a.showHelp = !a.showHelp
				return a, nil
			}
		case key.Matches(msg, keys.Filepicker):
			a.showFilepicker = !a.showFilepicker
			a.filepicker.ToggleFilepicker(a.showFilepicker)
			return a, nil
		}
	default:
		f, filepickerCmd := a.filepicker.Update(msg)
		a.filepicker = f.(dialog.FilepickerCmp)
		cmds = append(cmds, filepickerCmd)

	}

	if a.showFilepicker {
		f, filepickerCmd := a.filepicker.Update(msg)
		a.filepicker = f.(dialog.FilepickerCmp)
		cmds = append(cmds, filepickerCmd)
		// Only block key messages send all other messages down
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showQuit {
		q, quitCmd := a.quit.Update(msg)
		a.quit = q.(dialog.QuitDialog)
		cmds = append(cmds, quitCmd)
		// Only block key messages send all other messages down
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}
	if a.showPermissions {
		d, permissionsCmd := a.permissions.Update(msg)
		a.permissions = d.(dialog.PermissionDialogCmp)
		cmds = append(cmds, permissionsCmd)
		// Only block key messages send all other messages down
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showSessionDialog {
		d, sessionCmd := a.sessionDialog.Update(msg)
		a.sessionDialog = d.(dialog.SessionDialog)
		cmds = append(cmds, sessionCmd)
		// Only block key messages send all other messages down
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showCommandDialog {
		d, commandCmd := a.commandDialog.Update(msg)
		a.commandDialog = d.(dialog.CommandDialog)
		cmds = append(cmds, commandCmd)
		// Only block key messages send all other messages down
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showModelDialog {
		d, modelCmd := a.modelDialog.Update(msg)
		a.modelDialog = d.(dialog.ModelDialog)
		cmds = append(cmds, modelCmd)
		// Only block key messages send all other messages down
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showLoadoutDialog {
		d, loadoutCmd := a.loadoutDialog.Update(msg)
		a.loadoutDialog = d.(dialog.LoadoutDialog)
		cmds = append(cmds, loadoutCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showTasksDialog {
		d, tasksCmd := a.tasksDialog.Update(msg)
		a.tasksDialog = d.(dialog.TasksDialog)
		cmds = append(cmds, tasksCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showCommandHelp {
		d, helpCmd := a.commandHelp.Update(msg)
		a.commandHelp = d.(dialog.CommandHelpDialog)
		return a, helpCmd
	}

	if a.showExportDialog {
		d, exportCmd := a.exportDialog.Update(msg)
		a.exportDialog = d.(dialog.ExportDialog)
		cmds = append(cmds, exportCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showAddDirDialog {
		d, addDirCmd := a.addDirDialog.Update(msg)
		a.addDirDialog = d.(dialog.AddDirDialog)
		cmds = append(cmds, addDirCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showPromptsDialog {
		d, promptsCmd := a.promptsDialog.Update(msg)
		a.promptsDialog = d.(dialog.PromptsDialog)
		cmds = append(cmds, promptsCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showResetDialog {
		d, resetCmd := a.resetDialog.Update(msg)
		a.resetDialog = d.(dialog.ResetDialog)
		cmds = append(cmds, resetCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showSettingsDialog {
		d, settingsCmd := a.settingsDialog.Update(msg)
		a.settingsDialog = d.(dialog.SettingsDialog)
		cmds = append(cmds, settingsCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showConnectDialog {
		d, connectCmd := a.connectDialog.Update(msg)
		a.connectDialog = d.(dialog.ConnectDialog)
		cmds = append(cmds, connectCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showResearchDialog {
		d, rCmd := a.researchDialog.Update(msg)
		a.researchDialog = d.(dialog.ResearchDialogCmp)
		cmds = append(cmds, rCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showOsintDialog {
		d, oCmd := a.osintDialog.Update(msg)
		a.osintDialog = d.(dialog.OsintDialogCmp)
		cmds = append(cmds, oCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showOsintPage {
		d, pCmd := a.osintPage.Update(msg)
		a.osintPage = d.(dialog.OsintPageCmp)
		cmds = append(cmds, pCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showModelFollowDialog {
		d, fCmd := a.modelFollowDialog.Update(msg)
		a.modelFollowDialog = d.(dialog.ModelFollowDialogCmp)
		cmds = append(cmds, fCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showInitDialog {
		d, initCmd := a.initDialog.Update(msg)
		a.initDialog = d.(dialog.InitDialogCmp)
		cmds = append(cmds, initCmd)
		// Only block key messages send all other messages down
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showThemeDialog {
		d, themeCmd := a.themeDialog.Update(msg)
		a.themeDialog = d.(dialog.ThemeDialog)
		cmds = append(cmds, themeCmd)
		// Only block key messages send all other messages down
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	// GORILLA OVERRIDE: dismiss the sign-in overlay.
	//
	// Placed HERE, after every dialog's routing block, deliberately. Each of those
	// blocks returns early for a key message when its dialog is open, so reaching
	// this line proves no dialog wanted the key — which gives a deliberately
	// opened dialog first claim on esc, and makes any dialog added later take
	// precedence automatically without touching this code.
	//
	// The first attempt at this put the clear inside the `keys.Quit` branch, so
	// esc never reached it and the overlay could not be dismissed at all.
	if a.tryDismissLoginOverlay(msg) {
		return a, tea.Batch(cmds...)
	}

	s, _ := a.status.Update(msg)
	a.status = s.(core.StatusCmp)
	a.pages[a.currentPage], cmd = a.pages[a.currentPage].Update(msg)
	cmds = append(cmds, cmd)
	return a, tea.Batch(cmds...)
}

// truncatePrompt shortens a helper's prompt for one-line toasts/labels.
func truncatePrompt(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// RegisterCommand adds a command to the command dialog
func (a *appModel) RegisterCommand(cmd dialog.Command) {
	a.commands = append(a.commands, cmd)
}

func (a *appModel) findCommand(id string) (dialog.Command, bool) {
	for _, cmd := range a.commands {
		if cmd.ID == id {
			return cmd, true
		}
	}
	return dialog.Command{}, false
}

func (a *appModel) moveToPage(pageID page.PageID) tea.Cmd {
	if a.app.CoderAgent.IsBusy() {
		// For now we don't move to any page if the agent is busy
		return util.ReportWarn("Agent is busy, please wait...")
	}

	var cmds []tea.Cmd
	if _, ok := a.loadedPages[pageID]; !ok {
		cmd := a.pages[pageID].Init()
		cmds = append(cmds, cmd)
		a.loadedPages[pageID] = true
	}
	a.previousPage = a.currentPage
	a.currentPage = pageID
	if sizable, ok := a.pages[a.currentPage].(layout.Sizeable); ok {
		cmd := sizable.SetSize(a.width, a.height)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

// footerView is the whole frame when the conversation lives in the terminal's
// scrollback: the prompt, a capped preview of any reply in flight, and the status
// line. Everything settled has already been printed and is the terminal's to
// scroll, select and copy.
//
// It must stay shorter than the window. Outside the alternate screen bubbletea
// erases its previous frame by walking the cursor up by the number of LOGICAL
// lines it last drew; if the frame is taller than the window, the lines that
// scrolled off are not where that count believes they are and every later erase
// lands wrong — one stale copy per redraw, which is the failure the startup
// picker demonstrated before it was moved.
func (a appModel) footerView() string {
	type footerer interface{ FooterView(maxRows int) string }

	status := a.status.View()
	page, ok := a.pages[a.currentPage].(footerer)
	if !ok {
		// A page with nothing to contribute (the log viewer) still needs its status
		// line, and drawing its full body inline would be the very overflow this
		// mode exists to avoid.
		return status
	}

	// Give the prompt every row available above the status bar. The old policy
	// used half the window as a fixed budget, which made a long message scroll
	// out of sight even when the terminal had plenty of unused rows. The editor
	// already clamps itself to this budget and the final frame is still bounded
	// by the terminal height, so an arbitrary half-window ceiling is unnecessary.
	statusRows := lipgloss.Height(status)
	if statusRows < 1 {
		statusRows = 1
	}
	budget := a.height - statusRows
	if budget < chat.FooterReservedRows+1 {
		budget = chat.FooterReservedRows + 1
	}
	return clampToWidth(
		lipgloss.JoinVertical(lipgloss.Top, page.FooterView(budget), status),
		a.width)
}

// clampToWidth truncates every line of the frame to the terminal width.
//
// GORILLA FIX — this is the root cause of the footer "marching down the screen
// and jumping back up", found 2026-07-30 by driving a real bubbletea program
// headlessly and replaying its output through a terminal emulator
// (internal/tui/inline/scroll_boundary_test.go).
//
// Bubbletea's inline renderer erases its previous frame by moving the cursor UP
// by the number of LOGICAL lines it last drew. A line wider than the terminal
// occupies TWO physical rows but counts as one logical line, so the erase
// under-reaches by one row per wrapped line, every render. The un-erased rows
// are left stranded in the output as footer debris and the frame drifts.
//
// Measured: a single over-wide footer line strands an orphaned fragment in the
// middle of the printed transcript.
//
// It has to be fixed HERE rather than in each component. The footer is built
// from the working indicator, the editor, the session info line and the status
// bar, and every one of them uses lipgloss Width(), which WRAPS rather than
// truncates (already a documented trap in CLAUDE.md — the symptom of an
// over-long string is extra HEIGHT, not extra width). Fixing them one at a time
// leaves the invariant one careless Render() away from breaking again. One choke
// point, one rule: nothing in the frame is ever wider than the terminal.
func clampToWidth(view string, width int) string {
	if width <= 0 {
		return view
	}
	lines := strings.Split(view, "\n")
	for i, l := range lines {
		if ansi.StringWidth(l) > width {
			lines[i] = ansi.Truncate(l, width, "")
		}
	}
	return strings.Join(lines, "\n")
}

func (a appModel) View() string {
	// GORILLA OVERRIDE: with the alternate screen off and no dialog open, the frame
	// is only the footer. Rendering the full-screen layout here would paint a whole
	// window's worth of panels on top of text already printed to the terminal, and
	// none of it could be erased cleanly.
	if a.scrollback && !a.anyOverlayOpen() {
		return a.footerView()
	}

	components := []string{
		a.pages[a.currentPage].View(),
	}

	components = append(components, a.status.View())

	appView := lipgloss.JoinVertical(lipgloss.Top, components...)

	if a.showPermissions {
		overlay := a.permissions.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showFilepicker {
		overlay := a.filepicker.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)

	}

	// Show compacting status overlay
	if a.isCompacting {
		t := theme.CurrentTheme()
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocused()).
			BorderBackground(styles.PanelBackground()).
			Padding(1, 2).
			Background(styles.PanelBackground()).
			Foreground(t.Text())

		overlay := style.Render("Summarizing\n" + a.compactingMessage)
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showHelp {
		bindings := layout.KeyMapToSlice(keys)
		if p, ok := a.pages[a.currentPage].(layout.Bindings); ok {
			bindings = append(bindings, p.BindingKeys()...)
		}
		if a.showPermissions {
			bindings = append(bindings, a.permissions.BindingKeys()...)
		}
		if a.currentPage == page.LogsPage {
			bindings = append(bindings, logsKeyReturnKey)
		}
		if !a.app.CoderAgent.IsBusy() {
			bindings = append(bindings, helpEsc)
		}
		a.help.SetBindings(bindings)

		overlay := a.help.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showQuit {
		overlay := a.quit.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showSessionDialog {
		overlay := a.sessionDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showModelDialog {
		overlay := a.modelDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showLoadoutDialog {
		overlay := a.loadoutDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showTasksDialog {
		overlay := a.tasksDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showCommandHelp {
		overlay := a.commandHelp.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(col, row, overlay, appView, true)
	}

	if a.loginURL != "" {
		overlay := a.loginURLOverlay()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(col, row, overlay, appView, true)
	}

	if a.showExportDialog {
		overlay := a.exportDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(col, row, overlay, appView, true)
	}

	if a.showAddDirDialog {
		overlay := a.addDirDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(col, row, overlay, appView, true)
	}

	if a.showPromptsDialog {
		overlay := a.promptsDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(col, row, overlay, appView, true)
	}

	if a.showResetDialog {
		overlay := a.resetDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(col, row, overlay, appView, true)
	}

	if a.showSettingsDialog {
		overlay := a.settingsDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(col, row, overlay, appView, true)
	}

	if a.showConnectDialog {
		overlay := a.connectDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showCommandDialog {
		overlay := a.commandDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showModelFollowDialog {
		overlay := a.modelFollowDialog.View()
		appView = layout.PlaceOverlay(
			a.width/2-lipgloss.Width(overlay)/2,
			a.height/2-lipgloss.Height(overlay)/2,
			overlay,
			appView,
			true,
		)
	}

	if a.showResearchDialog {
		overlay := a.researchDialog.View()
		appView = layout.PlaceOverlay(
			a.width/2-lipgloss.Width(overlay)/2,
			a.height/2-lipgloss.Height(overlay)/2,
			overlay,
			appView,
			true,
		)
	}

	if a.showOsintDialog {
		overlay := a.osintDialog.View()
		appView = layout.PlaceOverlay(
			a.width/2-lipgloss.Width(overlay)/2,
			a.height/2-lipgloss.Height(overlay)/2,
			overlay,
			appView,
			true,
		)
	}

	if a.showOsintPage {
		overlay := a.osintPage.View()
		appView = layout.PlaceOverlay(
			a.width/2-lipgloss.Width(overlay)/2,
			a.height/2-lipgloss.Height(overlay)/2,
			overlay,
			appView,
			true,
		)
	}

	if a.showInitDialog {
		overlay := a.initDialog.View()
		appView = layout.PlaceOverlay(
			a.width/2-lipgloss.Width(overlay)/2,
			a.height/2-lipgloss.Height(overlay)/2,
			overlay,
			appView,
			true,
		)
	}

	if a.showThemeDialog {
		overlay := a.themeDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	if a.showMultiArgumentsDialog {
		overlay := a.multiArgumentsDialog.View()
		row := lipgloss.Height(appView) / 2
		row -= lipgloss.Height(overlay) / 2
		col := lipgloss.Width(appView) / 2
		col -= lipgloss.Width(overlay) / 2
		appView = layout.PlaceOverlay(
			col,
			row,
			overlay,
			appView,
			true,
		)
	}

	return appView
}

// runningProgram is the live tea.Program, recorded by SetProgram so a
// background goroutine can push a message into the event loop.
//
// GORILLA OVERRIDE: needed because the OAuth flow has to report its URL WHILE it
// blocks waiting for the browser callback — there is no return value to carry it.
// It used to fmt.Println the URL onto the screen Bubble Tea owns, which no redraw
// could clear, so the sign-in URL stayed burned across the interface for the rest
// of the session.
var runningProgram atomic.Pointer[tea.Program]

// SetProgram records the running program. Call it once, immediately after
// tea.NewProgram and before Run.
func SetProgram(p *tea.Program) { runningProgram.Store(p) }

func New(app *app.App) tea.Model {
	startPage := page.ChatPage
	model := &appModel{
		currentPage:    startPage,
		loadedPages:    make(map[page.PageID]bool),
		status:         core.NewStatusCmp(app.LSPClients),
		help:           dialog.NewHelpCmp(),
		quit:           dialog.NewQuitCmp(),
		sessionDialog:  dialog.NewSessionDialogCmp(),
		commandDialog:  dialog.NewCommandDialogCmp(),
		commandHelp:    dialog.NewCommandHelpCmp(),
		modelDialog:    dialog.NewModelDialogCmp(),
		connectDialog:  dialog.NewConnectDialogCmp(),
		addDirDialog:   dialog.NewAddDirDialogCmp(),
		exportDialog:   dialog.NewExportDialogCmp(),
		promptsDialog:  dialog.NewPromptsDialogCmp(),
		resetDialog:    dialog.NewResetDialogCmp(),
		settingsDialog: dialog.NewSettingsDialogCmp(),
		loadoutDialog:  dialog.NewLoadoutDialogCmp(),
		tasksDialog:    dialog.NewTasksDialogCmp(),
		permissions:    dialog.NewPermissionDialogCmp(),
		initDialog:     dialog.NewInitDialogCmp(),
		themeDialog:    dialog.NewThemeDialogCmp(),
		app:            app,
		// GORILLA OVERRIDE: read once. The buffer is chosen when the program starts,
		// so this cannot change mid-session, and a View() that re-read it every frame
		// could start drawing a full screen over text it had already printed.
		scrollback: !config.AlternateScreenEnabled(),
		commands:   []dialog.Command{},
		pages: map[page.PageID]tea.Model{
			page.ChatPage: page.NewChatPage(app),
			page.LogsPage: page.NewLogsPage(),
		},
		filepicker: dialog.NewFilepickerCmp(app),
	}

	model.RegisterCommand(dialog.Command{
		ID:          "init",
		Title:       "Initialize Project",
		Description: "Create/Update the OpenCode.md memory file",
		Handler: func(cmd dialog.Command) tea.Cmd {
			prompt := `Please analyze this codebase and create a OpenCode.md file containing:
1. Build/lint/test commands - especially for running a single test
2. Code style guidelines including imports, formatting, types, naming conventions, error handling, etc.

The file you create will be given to agentic coding agents (such as yourself) that operate in this repository. Make it about 20 lines long.
If there's already a opencode.md, improve it.
If there are Cursor rules (in .cursor/rules/ or .cursorrules) or Copilot rules (in .github/copilot-instructions.md), make sure to include them.`
			return tea.Batch(
				util.CmdHandler(chat.SendMsg{
					Text: prompt,
				}),
			)
		},
	})

	model.RegisterCommand(dialog.Command{
		ID:          "compact",
		Title:       "Compact Session",
		Description: "Summarize the current session and create a new one with the summary",
		Handler: func(cmd dialog.Command) tea.Cmd {
			return func() tea.Msg {
				return startCompactSessionMsg{}
			}
		},
	})
	// GORILLA OVERRIDE: /usage shows the Antigravity free-tier weekly quota
	// (agy's /usage screen, condensed to one status line). The fetch runs inside
	// the returned tea.Cmd's goroutine, so the network call never blocks the UI,
	// and the result comes back as an InfoMsg — never printed onto the screen
	// Bubble Tea owns.
	model.RegisterCommand(dialog.Command{
		ID:          "usage",
		Title:       "Antigravity Usage",
		Description: "Show your quota and provider balances (Antigravity, DeepSeek, OpenRouter)",
		Handler: func(cmd dialog.Command) tea.Cmd {
			return antigravityUsageCmd(false)
		},
	})

	// Load custom commands
	customCommands, err := dialog.LoadCustomCommands()
	if err != nil {
		logging.Warn("Failed to load custom commands", "error", err)
	} else {
		for _, cmd := range customCommands {
			model.RegisterCommand(cmd)
		}
	}

	return model
}

// ---- login / logout --------------------------------------------------------

type loginResultMsg struct {
	Email string
	err   error
}

// loginURLMsg carries the sign-in URL out of the OAuth flow so the TUI can
// render it. GORILLA OVERRIDE: the flow used to fmt.Println it straight onto the
// screen Bubble Tea owns, which no redraw could ever clear.
type loginURLMsg struct {
	URL string
}

type logoutDoneMsg struct {
	err error
}

func (a *appModel) runLogin() tea.Cmd {
	// The URL is reported through the context and pushed into the TUI as a
	// message. Send() is safe from this goroutine and is how a background task
	// hands work back to the event loop; writing to stdout instead is what
	// burned the URL permanently into the interface.
	prog := runningProgram.Load()
	return func() tea.Msg {
		ctx := auth.WithAuthPrompt(context.Background(), func(url string) {
			if prog != nil {
				prog.Send(loginURLMsg{URL: url})
			}
		})
		creds, err := auth.Login(ctx)
		if err != nil {
			return loginResultMsg{err: err}
		}
		if err := creds.Save(); err != nil {
			return loginResultMsg{err: fmt.Errorf("saving credentials: %w", err)}
		}
		if err := creds.SetupCodeAssist(ctx, ""); err != nil {
			// Login still succeeded; surface the warning on next launch.
			logging.Warn("Code Assist onboarding failed", "error", err)
		}
		return loginResultMsg{Email: creds.Email}
	}
}

func (a *appModel) runLogout() tea.Cmd {
	return func() tea.Msg {
		if err := auth.Logout(); err != nil {
			return logoutDoneMsg{err: err}
		}
		return logoutDoneMsg{}
	}
}

// withDeferredNote appends an explanation when a change could not be applied
// immediately because a turn is in flight.
//
// GORILLA OVERRIDE: the provider (and so the system prompt) cannot be swapped
// mid-request, so such a change is queued and applied when the turn ends. Saying
// nothing would leave the user believing a setting took effect when it did not —
// and the previous code did exactly that.
func withDeferredNote(info string, deferred bool) string {
	if !deferred {
		return info
	}
	return info + " — takes effect after the current turn finishes"
}

// tryDismissLoginOverlay clears the sign-in overlay if msg is esc and the
// overlay is up. Returns true if it handled the key.
//
// Split out as its own method so the behaviour is unit-testable: reaching it
// through appModel.Update needs a fully constructed app, and the bug it fixes
// (esc doing nothing) is exactly the kind that a test should have caught.
func (a *appModel) tryDismissLoginOverlay(msg tea.Msg) bool {
	if a.loginURL == "" {
		return false
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok || !key.Matches(km, returnKey) {
		return false
	}
	// Only hides it. The flow keeps waiting for the browser callback, so
	// dismissing by accident does not force the user to start over.
	a.loginURL = ""
	return true
}

// loginURLOverlay renders the pending sign-in URL.
//
// GORILLA OVERRIDE: this exists because the OAuth flow used to fmt.Println the
// URL. Under the TUI that writes into a screen Bubble Tea owns: the text lands on
// top of the interface, Bubble Tea has no record of drawing it, and so no redraw
// can remove it.
//
// The URL is WRAPPED across lines, and every line is padded to the same width.
//
// The first version left the URL on one unwrapped line, on the argument that a
// URL folded across lines cannot be pasted. That argument was wrong in practice
// and the screenshot showed why: the line was far wider than the terminal, so the
// box grew with it and the URL was simply cut off at the screen edge — unreadable
// AND unpastable. It also meant every other line was shorter than the box, and
// lipgloss does not pad the short lines of a multi-line render, so the unpainted
// remainder showed as black bars beside the text.
func (a *appModel) loginURLOverlay() string {
	t := theme.CurrentTheme()

	// Chrome is subtracted from the terminal width, never added to the content.
	const (
		chrome    = 6 // border 2 + padding 4
		preferred = 92
		minimum   = 24
	)
	w := preferred
	if a.width > 0 {
		w = max(minimum, min(preferred, a.width-chrome))
	}

	body := lipgloss.NewStyle().Background(styles.PanelBackground()).Foreground(t.Text())
	line := func(s string, st lipgloss.Style) string {
		// Width pads, MaxWidth clips: together they guarantee every line is
		// exactly w columns, which is what keeps the box rectangular.
		return st.Background(styles.PanelBackground()).Width(w).MaxWidth(w).Render(s)
	}

	urlLines := hardWrap(a.loginURL, w)

	head := []string{
		line("Sign in with Google", body.Bold(true).Foreground(t.Primary())),
		line("", body),
		line("Your browser should have opened. If it did not, visit:", body),
		line("", body),
	}
	foot := []string{
		line("", body),
		line("esc hides this — signing in keeps running", body.Foreground(t.TextMuted())),
	}

	// Fit the height by shedding explanation, never the URL: a partial URL is
	// worthless, whereas the prose is only helpful.
	if a.height > 0 {
		const vChrome = 4 // border 2 + padding 2
		budget := a.height - vChrome
		for len(head)+len(urlLines)+len(foot) > budget {
			switch {
			case len(head) > 1:
				head = head[:len(head)-1] // drop from the bottom, keep the title
			case len(foot) > 1:
				foot = foot[:1]
			case len(foot) > 0:
				foot = nil
			default:
				// Nothing left to shed; the URL stays whole and the overlay is
				// taller than the terminal. Better a scrolled URL than half a URL.
				budget = 0
			}
			if budget == 0 {
				break
			}
		}
	}

	parts := append(append(head, renderEach(urlLines, body.Foreground(t.Primary()), w)...), foot...)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).
		Background(styles.PanelBackground()).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

// renderEach pads each line individually. A single Render of a multi-line string
// leaves the short lines unpadded, which is what produced black bars beside the
// text — the trap this codebase has hit more than once.
func renderEach(lines []string, st lipgloss.Style, w int) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, st.Width(w).MaxWidth(w).Render(l))
	}
	return out
}

// hardWrap splits s into chunks of at most w columns, breaking mid-token.
// Word-wrapping is wrong here: a URL is one token, so a word-wrapper would leave
// it on a single over-wide line — the original bug.
func hardWrap(s string, w int) []string {
	if w < 1 {
		return []string{s}
	}
	r := []rune(s)
	if len(r) == 0 {
		return nil
	}
	var out []string
	for len(r) > w {
		out = append(out, string(r[:w]))
		r = r[w:]
	}
	return append(out, string(r))
}

// modelLabel is the human name of a model id, falling back to the id itself for
// anything the catalogue does not know. Used in status notes, where a raw id
// like "antigravity.gemini-3.6-flash-medium" is not what the user was shown in
// the picker and reads as a different thing entirely.
func modelLabel(id models.ModelID) string {
	if m, ok := models.SupportedModels[id]; ok && m.Name != "" {
		return m.Name
	}
	return string(id)
}
