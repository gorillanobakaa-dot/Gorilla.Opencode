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
	"github.com/opencode-ai/opencode/internal/export"
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

// ReopenConnectionPicker opens the connection profile picker from inside a
// session. Set by cmd at startup; nil in builds that do not wire it.
//
// GORILLA OVERRIDE (2026-08-20): the profile only offered itself on first run or
// on a two-rung mismatch, deliberately, so it does not nag. That leaves no way
// to reach it on purpose — and "my connection got worse and turns are failing"
// is exactly when someone wants it and has no reason to know it lives in
// /settings. Same hook shape as ReopenProviderPortal, and for the same reason:
// internal/tui cannot import cmd.
var ReopenConnectionPicker func() error

// ConnectionSwitchSummary reports what a profile change actually altered. Set by
// cmd, like ReopenConnectionPicker, because internal/tui cannot import cmd.
var ConnectionSwitchSummary = func() string { return "" }

// portalExec runs the provider portal while bubbletea has released the
// terminal. The portal is its own tea.Program and needs the screen to itself;
// tea.Exec is the same mechanism the editor already uses for $EDITOR.
type portalExec struct{ run func() error }

func (p portalExec) Run() error        { return p.run() }
func (portalExec) SetStdin(io.Reader)  {}
func (portalExec) SetStdout(io.Writer) {}
func (portalExec) SetStderr(io.Writer) {}

type startCompactSessionMsg struct{}

// helperHeartbeatMsg drives the "still alive" notice while helpers are working.
//
// GORILLA OVERRIDE (2026-08-17): a research lane went quiet for 23 minutes and
// came back with 19,118 tokens — it had been thinking the entire time. On a
// slow model over a slow link, a healthy run is indistinguishable from a hang:
// the screen simply stops. The owner's field experience is the extreme case of
// this — deployed, on a satellite uplink of a few KB/s, where everything looks
// broken and almost nothing is. So the program says so, out loud, on a timer.
//
// It only ticks while helpers are actually running, so it costs nothing at
// rest — the same rule the spinner now follows.
type helperHeartbeatMsg struct{}

// helperHeartbeatEvery is long enough not to nag and short enough that nobody
// concludes the program is dead. Silence on these models routinely runs to
// minutes; five would be too long, thirty seconds would be pestering.
const helperHeartbeatEvery = 90 * time.Second

func helperHeartbeatCmd() tea.Cmd {
	return tea.Tick(helperHeartbeatEvery, func(time.Time) tea.Msg { return helperHeartbeatMsg{} })
}

// helperHeartbeatLines rotate so the notice does not read as a stuck string —
// which would defeat the entire point of a liveness signal.
var helperHeartbeatLines = []string{
	"🦍 Still alive, bitch. %d helper(s) still working, longest %s. Nothing has crashed.",
	"🦍 Still working: %d helper(s) out, longest %s. A slow model looks EXACTLY like a hang. It is not one.",
	"🦍 Still here. %d helper(s), longest %s. /tasks to watch them or X to kill the lot.",
}

// austereHeartbeatLine is shown ONLY on the austere connection profile.
//
// GORILLA FIX (2026-08-23): it used to be a fourth entry in the rotation above,
// so it fired for everybody. The owner, on a fast line by his own explicit
// choice, was told "welcome to austere: slow model, SLOW LINE, quiet screen".
//
// That is the program stating a fact about the user's setup that the user had
// already contradicted in the settings. It is a small line and it is the same
// fault as a full green bar for an allowance that does not exist: confident,
// specific, and not true. A liveness notice exists to stop the user distrusting
// the screen, so a liveness notice that is wrong about them costs double.
//
// The line itself is good and stays, for the people it was written for: the
// satellite uplink where everything looks broken and almost nothing is.
const austereHeartbeatLine = "🦍 %d helper(s) grinding, longest %s. Welcome to austere: slow model, slow line, quiet screen, real work."

// heartbeatLines returns the rotation valid for the ACTIVE profile.
func heartbeatLines() []string {
	if config.CurrentConnProfile().ID == config.ProfileAustere {
		return append(append([]string(nil), helperHeartbeatLines...), austereHeartbeatLine)
	}
	return helperHeartbeatLines
}

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
	// permissionQueue holds requests raised while another prompt is on screen.
	// The dialog shows one at a time; see the pubsub case in Update.
	permissionQueue []permission.PermissionRequest
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

	// GORILLA OVERRIDE: the "still alive" heartbeat while helpers run.
	heartbeatRunning bool
	heartbeatBeat    int

	// GORILLA OVERRIDE: /osint — the serious dossier gate and its capability page.
	osintDialog     dialog.OsintDialogCmp
	showOsintDialog bool
	osintPage       dialog.OsintPageCmp
	showOsintPage   bool
	// GORILLA OVERRIDE (2026-08-19): /arsenal — the capability map. See
	// internal/arsenal.
	arsenalPage dialog.ArsenalCmp
	showArsenal bool
	// GORILLA OVERRIDE (2026-08-18): /sessions — the manager that can reach a
	// conversation you are no longer in: search it, revive it, export it, erase
	// it. Built for machines where the power cut ends the session, not the user.
	sessionsMgr     dialog.SessionsCmp
	showSessionsMgr bool

	// GORILLA OVERRIDE (2026-08-18): /osint --recover — the picker that turns a
	// run whose write-up died into the dossier it should have been.
	osintRecover     dialog.OsintRecoverCmp
	showOsintRecover bool

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

	// GORILLA OVERRIDE (2026-08-19), guard 2 on AGENTS.md: SAY that project
	// instructions were loaded, and how many bytes of them.
	//
	// A file spliced into the model's instructions before the user has typed
	// anything is the one thing in this program that changes its behaviour
	// invisibly. Announcing it costs one line and turns "why is it doing
	// that?" into "because this repository told it to". The refusal case is
	// announced too, and more loudly, because a file that silently did not
	// load is indistinguishable from a file that is not there.
	cmds = append(cmds, func() tea.Msg {
		v := config.AutoLoadProjectInstructions(config.WorkingDirectory())
		switch {
		case v.Bytes == 0:
			return nil
		case v.Loaded:
			return util.InfoMsg{
				Type: util.InfoTypeInfo,
				Msg: fmt.Sprintf("Loaded %s (%s) into the system prompt — %s.",
					config.AgentsFile, humanBytes(int64(v.Bytes)), v.Reason),
			}
		default:
			return util.InfoMsg{
				Type: util.InfoTypeWarn,
				Msg: fmt.Sprintf("NOT loading %s (%s): %s. Read it yourself, and if you trust it, paste what matters.",
					config.AgentsFile, humanBytes(int64(v.Bytes)), v.Reason),
			}
		}
	})

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
	return quotaAlertStyle.Render("  " + at.Format("15:04:05") + "  quota | " + line)
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
	// scrollback (bars, plain-language "left/used", green->red colour). Set only
	// on an explicit /usage: the automatic session-start reading stays one line
	// so it never floods the top of every conversation.
	summary *auth.QuotaSummary
	account string
	// balances are paid-provider readings (DeepSeek, OpenRouter) for providers
	// the user has a key for. Also /usage-only: each is a network call.
	balances []quota.Reading
	// fractions seeds/updates the crossing-alert baseline on every reading.
	fractions map[string]float64
	// meters are the ACTIVE provider's readings, each already carrying the
	// account it belongs to (internal/quota.Meter). Replaced the old
	// (summary, account, chatgpt) trio: those were three arguments nothing
	// forced to agree, which is how one provider's numbers came to be printed
	// under another's sign-in.
	meters []quota.Meter
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

// coderProvider is the provider the coder agent's model belongs to — the one
// the session actually spends against. Returns "" when config/model cannot be
// resolved, which the callers treat as "assume Antigravity" so a user with no
// resolvable model sees no regression.
//
// GORILLA OVERRIDE (2026-08-23): the quota panel keys on this so it never
// presents the Antigravity weekly meter (or its account email) for a session
// signed in to a different provider — the wrong-barrel bug where a ChatGPT
// session showed a Google account at 97%. When Antigravity is not the active
// provider the fetch is skipped entirely, which also drops the wasted balance
// round-trips measured the same day.
func coderProvider() models.ModelProvider {
	c := config.Get()
	if c == nil {
		return ""
	}
	return models.SupportedModels[c.Agents[config.AgentCoder].Model].Provider
}

// antigravityIsActive reports whether the session spends Antigravity quota.
// An unresolvable provider ("") defaults to true: if we cannot tell, behave as
// before rather than hiding a meter the user may rely on.
func antigravityIsActive() bool {
	p := coderProvider()
	return p == "" || p == models.ProviderAntigravity
}

// activeChatGPTMeter returns the ChatGPT meter when ChatGPT is the provider the
// session actually spends against, else nil.
func activeChatGPTMeters() []quota.Meter {
	if coderProvider() != models.ProviderChatGPT {
		return nil
	}
	var account string
	if creds, _ := auth.LoadChatGPTCreds(); creds != nil {
		account = creds.Email
	}
	q, _ := auth.LoadChatGPTQuota()
	if q == nil {
		// Signed in but nothing read yet. The adapter marks this Pending, which
		// is a real third state: blank and 100% are both lies.
		q = &auth.ChatGPTQuota{}
	}
	return []quota.Meter{q.ToMeter(account)}
}

// quotaTierCheckCmd fetches a fresh quota reading and compares it against the
// previous fractions. Silent on every failure path: an alert system that nags
// about its own plumbing is worse than none.
func quotaTierCheckCmd(prev map[string]float64) tea.Cmd {
	return func() tea.Msg {
		// Not spending Antigravity quota: do not poll it, and do not raise
		// banana alerts about a barrel the session is not drawing down.
		if !antigravityIsActive() {
			return nil
		}
		creds, _ := auth.LoadAntigravityCreds()
		if creds == nil || creds.AccessToken == "" {
			return nil
		}
		q, err := creds.RetrieveQuota(context.Background())
		if err != nil {
			return nil
		}
		alerts, next := bananaAlerts(prev, q.ToMeter(creds.Email))
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
		// Antigravity is not the active provider: do not fetch its weekly quota
		// (that is the wrong barrel, and the fetch costs bandwidth on a metered
		// link for a number the session is not spending). Say the login exists
		// without pulling it, and still show any paid-provider balances the user
		// holds. quiet=true is the session-start line; stay silent there.
		if !antigravityIsActive() {
			if quiet {
				return nil
			}
			line := "Antigravity is not the active provider, so its weekly quota is not shown."
			if creds, _ := auth.LoadAntigravityCreds(); creds != nil && creds.Email != "" {
				line = fmt.Sprintf("Antigravity is not the active provider: signed in as %s, weekly quota not fetched.", creds.Email)
			}
			msg := quotaLineMsg{line: line, kind: util.InfoTypeInfo}
			// Show the ACTIVE provider's own meter instead of a blank. Removing
			// the wrong number without supplying the right one left a user at
			// 20% of a monthly limit with nothing to look at.
			if cg := activeChatGPTMeters(); len(cg) > 0 {
				msg.meters = cg
				switch m := cg[0]; {
				case m.Pending:
					line = "No usage reading yet this session: this backend reports usage on its replies."
				case len(m.Bars) > 0:
					line = fmt.Sprintf("%s: %d%% left", m.Bars[0].Label,
						int(m.Bars[0].Remaining*100+0.5))
				}
				msg.line = line
			}
			msg.balances = configuredBalances(context.Background())
			return msg
		}
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
		meters := q.ToMeter(creds.Email)
		_, msg.fractions = bananaAlerts(nil, meters) // seed/update the alert baseline
		if !quiet {
			msg.meters = meters
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
			if len(msg.meters) > 0 || len(msg.balances) > 0 {
				out += "\n\n" + renderQuotaPanel(msg.meters, msg.balances, a.width, time.Now()) + "\n"
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
				Msg:  stripBananaEmoji(strings.Join(msg.alerts, " | ")),
				TTL:  15 * time.Second,
			}
			st, cmd := a.status.Update(info)
			a.status = st.(core.StatusCmp)
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	// Status
	case util.InfoMsg:
		// GORILLA OVERRIDE (2026-08-18): an Echo notice is too long for the
		// status bar, which truncates to one line — so also print it in full into
		// the transcript, where it wraps freely and stays. tea.Println is the
		// only safe way to write above the inline frame, and scrollback is where
		// the permanent copy belongs; the footer keeps its short flash. Same
		// dual-channel pattern the quota reading uses.
		if a.shouldEchoNotice(msg) {
			t := theme.CurrentTheme()
			// Bookend the transcript line with util.NoticeDeco (🦍⚠️ ⚠️ 🦍). Drop
			// the message's own leading gorilla so the bookend supplies them all.
			// The footer banner is untouched — this is transcript-only.
			body := strings.TrimSpace(strings.TrimPrefix(msg.Msg, "🦍"))
			plain := "  " + time.Now().Format("15:04:05") + "  " +
				util.NoticeDeco + " " + body + " " + util.NoticeDeco

			// WRAP to the terminal. Composed, this notice is ~188 columns; on a
			// narrower terminal the tail wrapped and dumped the closing bookend
			// alone on the next line at column 0, which read as broken output.
			// Word-wrapping keeps the bookend at the end of the last line where
			// it belongs. Width 0 means no WindowSizeMsg has arrived yet — leave
			// it unwrapped rather than render into a zero-width style.
			// Colour by severity. Every echoed notice used to be Warning
			// amber, which was right when only the cold-start warning echoed;
			// now that any oversized notice does, an ordinary /update report
			// printed in the same amber as a provider failure reads as a
			// problem. The footer already distinguishes the three — the
			// transcript copy must agree with it.
			style := lipgloss.NewStyle().Bold(true).Foreground(t.Warning())
			switch msg.Type {
			case util.InfoTypeInfo:
				style = style.Foreground(t.Info())
			case util.InfoTypeError:
				style = style.Foreground(t.Error())
			}
			if a.width > 20 {
				style = style.Width(a.width)
			}
			// One line, wrapped to the terminal. No blank lines, no rule: extra
			// lines are just more that can render wrong, and the bookends already
			// mark where the notice starts and ends.
			cmds = append(cmds, tea.Println(style.Render(plain)))
		}
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
		// GORILLA FIX (2026-08-23): QUEUE, do not overwrite.
		//
		// This used to be an unconditional SetPermissions, and the dialog holds
		// exactly one request in a plain field. So a fan-out that raised ten
		// requests at once painted each over the last, and only the TENTH was
		// ever seen. The other nine goroutines stayed parked on their channels
		// with nothing left to republish them, waited out PermissionWait (ten
		// minutes) and were then DENIED.
		//
		// It fails closed, so it was never a security hole. It is worse than
		// that in one specific way: a request the user was never shown was
		// recorded as one the user refused, and the visible symptom was a run
		// that hung for ten minutes and came back blaming the network.
		if a.showPermissions {
			a.permissionQueue = append(a.permissionQueue, msg.Payload)
			return a, nil
		}
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
		// Answering one question can settle others already in the queue: an
		// "allow for session", or a fleet grant, covers them outright. Grant
		// those silently rather than asking again for something just approved.
		next, rest := drainPermissionQueue(a.permissionQueue,
			a.app.Permissions.IsCovered, a.app.Permissions.Grant)
		a.permissionQueue = rest
		if next != nil {
			return a, a.permissions.SetPermissions(*next)
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

	case helperHeartbeatMsg:
		running, longest, _ := agent.HeartbeatState()
		if running == 0 {
			// Nothing running: let the chain lapse. It restarts on the next spawn.
			//
			// The footer's live figure MUST be cleared here. The research tool
			// rolls the whole run into the parent session when it returns, so
			// leaving the live total in place would add the same run twice and
			// turn an under-report into an over-report, which is not an
			// improvement.
			a.heartbeatRunning = false
			return a, util.CmdHandler(core.LiveHelperCostMsg(0))
		}
		a.heartbeatBeat++
		lines := heartbeatLines()
		line := lines[a.heartbeatBeat%len(lines)]
		notice := fmt.Sprintf(line, running, longest.Round(time.Second))

		// GORILLA FIX (2026-08-23): say what it has cost SO FAR.
		//
		// Measured on a live run: the footer read "spent $0.01" for seventeen
		// minutes while the database held $6.70 across eighteen helper sessions.
		// Not a lost figure. Helper turns credit the helper's OWN session, and
		// the parent is only credited when the research tool returns, so the
		// true number arrives after the last moment anyone could act on it.
		//
		// The heartbeat is the right home for the running total: it already
		// ticks only while helpers are alive, so this costs one query every 90
		// seconds during a run and nothing at all at rest.
		spend := a.liveHelperSpend()
		if spend > 0 {
			notice += fmt.Sprintf(" Burned $%.2f so far.", spend)
		}
		return a, tea.Batch(
			util.ReportInfo(notice),
			// The footer carries the same figure, so it is visible without
			// waiting for the next beat.
			util.CmdHandler(core.LiveHelperCostMsg(spend)),
			helperHeartbeatCmd(),
		)

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

	case dialog.SessionsCloseMsg:
		a.showSessionsMgr = false
		return a, nil

	case dialog.SessionsSearchMsg:
		// Titles are filtered inside the dialog; message CONTENT needs the
		// database. Titles are frequently useless for finding a session weeks
		// later — this program writes "New Session" as a real title.
		hits, err := a.app.Sessions.Search(context.Background(), msg.Needle)
		if err != nil {
			return a, util.ReportError(err)
		}
		a.sessionsMgr.SetMatches(hits)
		return a, nil

	case dialog.SessionsReviveMsg:
		a.showSessionsMgr = false
		a.selectedSession = msg.Session
		return a, util.CmdHandler(chat.SessionSelectedMsg(msg.Session))

	case dialog.SessionsExportMsg:
		// Export whichever session was chosen, not whichever one is open. That
		// distinction is the point: after a power cut the session you want is
		// never the one you are in.
		ctx := context.Background()
		msgs, err := a.app.Messages.List(ctx, msg.Session.ID)
		if err != nil {
			return a, util.ReportError(err)
		}
		// Helper sessions travel with the conversation. Without them a research
		// run exported as 14 messages while the list said 275 — the other 261,
		// and every lane's reasoning and tool call, were silently absent.
		var branches []export.Branch
		if kids, err := a.app.Sessions.Children(ctx, msg.Session.ID); err == nil {
			for _, kid := range kids {
				km, err := a.app.Messages.List(ctx, kid.ID)
				if err != nil {
					continue
				}
				branches = append(branches, export.Branch{Session: kid, Messages: km})
			}
		}
		path, total, err := export.WriteSessionTree(config.SessionExportDir(), msg.Session, msgs, branches, time.Now())
		if err != nil {
			return a, util.ReportError(err)
		}
		a.sessionsMgr.SetNotice(fmt.Sprintf("Exported %d messages (%d helper sessions) -> %s",
			total, len(branches), path))
		return a, nil

	case dialog.SessionsResumeMsg:
		a.showSessionsMgr = false
		return a, a.resumeSession(msg.Session)

	case dialog.SessionsDeleteMsg:
		return a, a.eraseSession(msg.Session)

	case dialog.CloseOsintRecoverMsg:
		a.showOsintRecover = false
		if !msg.Chosen {
			return a, nil
		}
		path, findings, err := agent.RecoverFindings(context.Background(), msg.Run, a.app.Sessions, a.app.Messages)
		if err != nil {
			return a, util.ReportError(err)
		}
		// The write-up happens in a FRESH conversation. That is the entire
		// mechanism: the original run did not fail for lack of findings, it
		// failed because the orchestrator was carrying raw tool results, its own
		// reasoning and the whole conversation at 145% of its window. The
		// findings themselves measured ~15,045 tokens.
		prompt := agent.AssemblyPrompt(msg.Run.Question, findings, path)
		if est, window := agent.EstimateTokens(prompt), a.assemblyWindow(); window > 0 && est > window {
			return a, util.ReportWarn(agent.ChunkedAssemblyNote(est, window))
		}
		a.selectedSession = session.Session{}
		return a, tea.Sequence(
			util.CmdHandler(chat.NewSessionMsg{}),
			util.CmdHandler(chat.SendMsg{Text: prompt}),
		)

	case dialog.CloseOsintPageMsg:
		a.showOsintPage = false
		return a, nil

	case dialog.CloseArsenalMsg:
		a.showArsenal = false
		return a, nil

	// GORILLA OVERRIDE (2026-08-19): /arsenal NEVER installs anything. It
	// prints the exact command into the conversation, where it can be read,
	// selected and copied — and where the user decides. An installer is the
	// highest-stakes prompt in this program, and the August audit's finding
	// was that a prompt describing less than what happens is worse than none.
	case dialog.ArsenalInstallMsg:
		a.showArsenal = false
		return a, util.CmdHandler(chat.SendMsg{Text: "I chose these from /arsenal:\n\n" +
			msg.Summary + "\nThe command to install them is:\n\n    " + msg.Command +
			"\n\nDo NOT run it. Show it to me, tell me in one line what each one will let you do " +
			"that you cannot do now, and say plainly if any of it is a bad idea on this machine."})

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
		// GORILLA OVERRIDE (2026-08-19): /arsenal is the capability map — what
		// this agent can do on THIS machine, what it could do, and what that
		// would cost. It exists because a model reported it could not read a
		// screenshot while tesseract sat installed three inches away: the
		// barrier was never bandwidth, it was knowing the thing exists.
		case "arsenal", "tools":
			a.arsenalPage = dialog.NewArsenalCmp()
			a.arsenalPage.SetSize(a.width, a.height)
			a.showArsenal = true
			return a, nil
		case "osint", "dossier":
			q := strings.TrimSpace(msg.Args)
			// GORILLA OVERRIDE (2026-08-18): --recover is a FLAG, not a question.
			//
			// It was documented before it was built: the salvage path told users
			// to run it, and typing it sent the literal string "--recover" into a
			// ten-helper supervised dossier as the subject under investigation.
			// The model refused to fabricate a dossier about a flag, which was the
			// right call and cost the user a run's worth of setup to discover.
			// Recognised here, before anything can be spent.
			if isRecoverFlag(q) {
				runs := agent.ListRecoverableRuns(context.Background(), a.app.Sessions, a.app.Messages)
				a.osintRecover = dialog.NewOsintRecoverCmp(runs)
				a.osintRecover.SetSize(a.width, a.height)
				a.showOsintRecover = true
				return a, nil
			}
			if q == "" {
				a.osintPage = dialog.NewOsintPageCmp()
				a.osintPage.SetSize(a.width, a.height)
				a.showOsintPage = true
				return a, nil
			}
			if !config.LoadoutEnabled(config.DossierComponentID) {
				return a, util.ReportWarn("The serious OSINT dossier is switched OFF (it burns real money, so it ships that way). Arm it: /context -> \"" + config.DossierRowName + "\" -> space. Or type /osint alone to read what it does first.")
			}
			a.osintDialog = dialog.NewOsintDialogCmp(q)
			a.osintDialog.SetSize(a.width, a.height)
			a.showOsintDialog = true
			return a, nil
		// GORILLA OVERRIDE (2026-08-18): /sessions reaches a conversation you
		// are no longer in. The old switcher (ctrl+s) showed titles and nothing
		// else — no date, no size, no search — and could not export or erase.
		// GORILLA OVERRIDE (2026-08-18): /review runs the embedded code-review
		// toolkit. Routed through the AGENT rather than called directly: the
		// analysers are half a review, and the model has to read the changed
		// code and say so. A command that printed findings and stopped would be
		// the "looks complete, is half" failure the tool's own description
		// warns about.
		case "review", "audit", "codereview":
			// GORILLA FIX (2026-08-18): parse the arguments. The first version
			// used the whole string as a path, so `/review --deep` asked for a
			// review of a folder called "--deep" — the same flag-read-as-content
			// defect as `/osint --recover` earlier the same day. Parsing and
			// prompt building live in reviewargs.go with their own tests; they
			// are also OUT of this switch on purpose, because a nested switch
			// here reads to TestEveryDispatchedCommandIsDocumented as three new
			// slash commands called /quick, /security and /full.
			req := parseReviewArgs(msg.Args)
			if len(req.Unknown) > 0 {
				return a, util.ReportWarn(unknownReviewOptionMessage(req.Unknown))
			}
			return a, util.CmdHandler(chat.SendMsg{Text: reviewPrompt(req)})

		case "sessions", "history":
			if cmd := a.openSessionsManager(); cmd != nil {
				return a, cmd
			}
			return a, nil
		// GORILLA OVERRIDE (2026-08-18): /resume opens the same list, because
		// picking up stalled work always starts with "which one" — and once you
		// are there, reopening, exporting and erasing are the same three things
		// you might want. The hint line tells you which key hands it over.
		case "resume", "continue", "handoff":
			if cmd := a.openSessionsManager(); cmd != nil {
				return a, cmd
			}
			a.sessionsMgr.SetNotice("ctrl+r hands the highlighted work to this model in a fresh conversation.")
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
		// GORILLA OVERRIDE: /yolo — grant once, then let the helpers run.
		//
		// The permission prompt exists for a good reason and this switches it
		// off, so the message says exactly what is being handed over and the
		// footer keeps saying it for as long as it lasts. Scoped to THIS
		// conversation and gone when it ends: nothing is written to disk, so a
		// moment of impatience cannot become a permanent standing grant.
		//
		// Because grants resolve through the session tree, one toggle covers
		// every research helper and sub-agent too — which is the entire point,
		// and was the reported problem: a 10-helper run asking the same
		// question ten times.
		// /goal is the same switch with a job attached: arm it and go, which is
		// the shape people know from other agents. Bare /yolo stays a toggle.
		case "yolo", "auto", "autopilot", "goal":
			if a.selectedSession.ID == "" && strings.TrimSpace(msg.Args) == "" {
				return a, util.ReportWarn("Start a conversation first — YOLO applies to the session you are in.")
			}
			task := strings.TrimSpace(msg.Args)
			// With no task, this toggles. Toggling OFF must never be mistaken
			// for arming, so the off-branch comes first and only when bare.
			if task == "" && a.app.Permissions.IsAutoApproved(a.selectedSession.ID) {
				a.app.Permissions.RevokeAutoApprove(a.selectedSession.ID)
				return a, util.ReportInfo("YOLO OFF — you will be asked again before tools touch anything.")
			}
			if a.selectedSession.ID != "" {
				a.app.Permissions.AutoApproveSession(a.selectedSession.ID)
			}
			warning := "☢ YOLO ON for this conversation. Every tool call is approved automatically — " +
				"file edits, shell commands, web access, and every research helper — with no further prompts. " +
				"It ends when this conversation does, or type /yolo again. /tasks still kills helpers."
			if task == "" {
				return a, util.ReportWarn(warning)
			}
			// Armed AND tasked in one command. The warning still fires, because
			// the point of the warning is that it is never skipped.
			return a, tea.Batch(
				util.ReportWarn(warning),
				util.CmdHandler(chat.SendMsg{Text: task}),
			)

		// GORILLA OVERRIDE: /compact and /init existed ONLY in the ctrl+k
		// palette, so typing them — which is how this program teaches every
		// other command — answered "Unknown command". This is the same trap
		// CLAUDE.md records for /usage: a palette RegisterCommand is not a
		// typed command. /compact matters most on small-context models, where
		// the window fills fast and the alternative is losing the thread.
		case "compact", "summarize", "summarise":
			if a.selectedSession.ID == "" {
				return a, util.ReportWarn("Nothing to compact yet — this starts once a conversation exists.")
			}
			return a, util.CmdHandler(startCompactSessionMsg{})
		case "init":
			a.showInitDialog = true
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
				return a.adoptPortalModel()
			})
		case "purge", "purgemodels", "purge-models":
			// GORILLA OVERRIDE (2026-08-20): clears the FETCHED catalogues only.
			// Compiled-in providers, bookmarks and the hidden list all survive -
			// see internal/llm/models/purge.go for why each is spared.
			if a.app.CoderAgent != nil && a.app.CoderAgent.IsBusy() {
				return a, util.ReportWarn("finish or cancel the current turn before purging the model lists")
			}
			// The models the agents are on survive the purge — see the keep
			// parameter in models/purge.go.
			var inUse []models.ModelID
			for _, ag := range config.Get().Agents {
				inUse = append(inUse, ag.Model)
			}
			res := models.PurgeFetchedCatalogues(config.CacheBase(), inUse...)
			if res.RemovedModels == 0 && len(res.FilesDeleted) == 0 {
				return a, util.ReportInfo("nothing to purge - no downloaded model lists were present")
			}
			// GORILLA FIX (2026-08-21): say what came from a local endpoint,
			// separately. Those return by themselves on the next launch because
			// the connection is still configured, and a user who purged them to
			// clear the picker needs to know that BEFORE they restart and
			// conclude the purge did nothing.
			msgText := fmt.Sprintf(
				"cleared %d models, %d left. Your bookmarks, hidden list and the models your agents are on are untouched. Run /update to fetch fresh lists.",
				res.RemovedModels, res.Kept)
			// GORILLA FIX (2026-08-21): say which ones come BACK. Some of what a
			// purge clears ships inside the binary and is re-registered by an
			// init() on the next launch, so "purged" was the wrong word for it —
			// the picker shrinks now and is full again after a restart, with
			// nothing having said so. Reported here rather than quietly, because
			// someone who purged to shorten the list needs to know that BEFORE
			// they restart and conclude the command did nothing.
			if res.RemovedCompiled > 0 {
				msgText += fmt.Sprintf(
					" %d of them ship with the app and come back when you restart — /connection disables a provider for good, and H hides models permanently.",
					res.RemovedCompiled)
			}
			if res.RemovedLocal > 0 {
				msgText += fmt.Sprintf(
					" %d of them came from your configured endpoint(s) — those re-register on the next launch or /update; use /connection to disable an endpoint for good.",
					res.RemovedLocal)
			}
			return a, util.ReportInfo(msgText)

		case "update", "updatemodels", "update-models", "refresh":
			// Refresh is a network round trip per provider, so it must not run
			// while a turn is in flight competing for the same link.
			if a.app.CoderAgent != nil && a.app.CoderAgent.IsBusy() {
				return a, util.ReportWarn("finish or cancel the current turn before refreshing the model lists")
			}
			return a, a.refreshModelCatalogues()

		case "connection", "conn", "link":
			// GORILLA OVERRIDE (2026-08-20): /connection — the profile picker.
			// Dispatch happens through this switch, not the palette: a palette
			// registration alone leaves a typed `/connection` reported as an
			// unknown command (learned from /usage).
			if ReopenConnectionPicker == nil {
				return a, util.ReportWarn("the connection picker is not available in this build")
			}
			if a.app.CoderAgent != nil && a.app.CoderAgent.IsBusy() {
				return a, util.ReportWarn("finish or cancel the current turn before changing the connection profile")
			}
			return a, tea.Exec(portalExec{run: ReopenConnectionPicker}, func(err error) tea.Msg {
				if err != nil {
					return util.InfoMsg{Type: util.InfoTypeError, Msg: err.Error()}
				}
				msgText := ConnectionSwitchSummary()
				if msgText == "" {
					msgText = "Connection profile unchanged."
				}
				return util.InfoMsg{Type: util.InfoTypeInfo, Msg: msgText}
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
			// GORILLA FIX (2026-08-23): the label budget is the SCREEN, not 40.
			//
			// This was truncatePrompt(..., 40), a constant with no relationship
			// to the terminal. Reported from a 1519px window where the labels
			// still read "ADVERSARY - what breaks, l...": the number was never
			// about the screen, so a wide terminal bought nothing. Worse, the
			// status component truncates AGAIN to fit real columns, so the inner
			// 40 destroyed text that would have fitted and then handed the
			// result to something that would have cut it correctly anyway.
			//
			// The hint is reserved rather than the label, because "(/tasks to
			// view or kill)" is the actionable half. A label that runs out of
			// room loses words; a hint that runs out of room loses the only
			// instruction telling the user they can kill a runaway helper.
			//
			// The em-dashes went too. Directive 1, and this line renders once
			// per helper, so a ten-helper run put twenty of them on screen.
			cmds := []tea.Cmd{util.ReportInfo(fmt.Sprintf("🦍 helper %s spawned: %s  (/tasks to view or kill)",
				msg.Payload.ID, truncatePrompt(msg.Payload.Prompt, spawnLabelBudget(a.width, msg.Payload.ID))))}
			// Start the "still alive" heartbeat if it is not already ticking.
			if !a.heartbeatRunning {
				a.heartbeatRunning = true
				cmds = append(cmds, helperHeartbeatCmd())
			}
			return a, tea.Batch(cmds...)
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
			// Not while the manager is open: it owns the keyboard, and this
			// binding would otherwise open the old switcher behind it. Found by
			// driving the real binary — tab in the manager appeared to do
			// nothing because this branch consumed the key first.
			if a.currentPage == page.ChatPage && !a.showQuit && !a.showPermissions && !a.showCommandDialog && !a.showSessionsMgr {
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
				if a.showArsenal {
					a.showArsenal = false
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

	if a.showSessionsMgr {
		d, sCmd := a.sessionsMgr.Update(msg)
		a.sessionsMgr = d.(dialog.SessionsCmp)
		cmds = append(cmds, sCmd)
		if _, ok := msg.(tea.KeyMsg); ok {
			return a, tea.Batch(cmds...)
		}
	}

	if a.showOsintRecover {
		d, rCmd := a.osintRecover.Update(msg)
		a.osintRecover = d.(dialog.OsintRecoverCmp)
		cmds = append(cmds, rCmd)
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

	if a.showArsenal {
		d, aCmd := a.arsenalPage.Update(msg)
		a.arsenalPage = d.(dialog.ArsenalCmp)
		cmds = append(cmds, aCmd)
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
		return "..."
	}
	return string(r[:max-3]) + styles.Ellipsis
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

	// GORILLA FIX (2026-08-19), from a screenshot of a real run: pad the
	// background to the terminal height BEFORE compositing any overlay.
	//
	// PlaceOverlay clamps an overlay to the background's height — deliberately,
	// so a dialog can never be taller than what it sits on. In scrollback mode
	// the background is just the chat page plus the status bar, which is short,
	// so a FULL-SCREEN page (/arsenal, /osint) had its bottom rows and its
	// bottom border silently cut off. The page was correct; the canvas it was
	// painted on was smaller than the screen.
	//
	// This is the mirror image of the bug the clamp exists to prevent, and it
	// only appears in scrollback mode — which is the mode this project steers
	// older machines toward, so it is the mode most users are in.
	if a.height > 0 {
		if h := lipgloss.Height(appView); h < a.height {
			appView += strings.Repeat("\n", a.height-h)
		}
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

	if a.showSessionsMgr {
		overlay := a.sessionsMgr.View()
		appView = layout.PlaceOverlay(
			a.width/2-lipgloss.Width(overlay)/2,
			a.height/2-lipgloss.Height(overlay)/2,
			overlay,
			appView,
			true,
		)
	}

	if a.showOsintRecover {
		overlay := a.osintRecover.View()
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

	if a.showArsenal {
		overlay := a.arsenalPage.View()
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

	// GORILLA OVERRIDE (2026-08-23): THE LAST THREE OVERLAYS ARE DRAWN IN REVERSE
	// KEY-PRIORITY ORDER, and that is the whole point of them being here.
	//
	// The bug, reported twice by the owner: with `/tasks` open, tab and esc did
	// nothing. His words: "the tab button will not allow you to switch anything as
	// long as your /tasks list is wide enough to cover the buttons of the prompt
	// underneath. You have to wait for some of the tasks to finish so the tasks
	// window gets narrower and narrower and it exposes the buttons underneath, and
	// it is only then when TAB begins to work."
	//
	// The cause was an inversion between two orders in this file. In Update, a
	// visible permission dialog swallows every KeyMsg and returns, so it owns the
	// keyboard ahead of `/tasks`. In View, it used to be drawn FIRST of sixteen
	// overlays, so every one of them painted over it. The dialog eating the
	// keystrokes was underneath the dialog the user could see.
	//
	// Permissions is the only overlay that ARRIVES UNBIDDEN. Every other one is
	// opened by a keystroke, and cannot be opened while permissions is blocking
	// the keyboard. So a permission prompt lands on top of whatever was already
	// open, and it must be drawn there too.
	//
	// The order below is the exact reverse of the key-handling order at the top of
	// Update (filepicker, quit, permissions): last drawn is topmost, so the
	// highest key priority ends up on top. Keeping quit and filepicker ABOVE
	// permissions is deliberate rather than incidental: they outrank it for keys,
	// and ctrl+c must keep working while a permission is pending.
	//
	// Pinned by TestBlockingOverlaysAreDrawnInReverseKeyOrder.
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

// adoptPortalModel moves the RUNNING agent onto whatever the provider portal
// just saved, and says which model is now answering.
//
// GORILLA FIX (2026-08-21): without this the portal changed the config and
// nothing else. The footer reads the config, so it immediately said "Claude
// Sonnet 4.6 (Antigravity free)" — while app.CoderAgent still held the provider
// it was built with at startup and kept sending to NVIDIA NIM's Llama 3.3 70B.
//
// Reported by the owner, who spotted it from the model's BEHAVIOUR before any
// label gave it away: he typed "hm" and got an unrequested web_fetch of
// debian.org, and said "I have a feeling this is not Claude". The transcript's
// per-message label agreed with him — it read "Llama 3.3 70B" under a footer
// that read Claude. He was spending NIM quota while believing he was on the free
// Antigravity tier.
//
// /model has done this correctly since 1fecb4c: agent.Update builds the new
// provider FIRST, and only swaps it in if that succeeds, so a failure leaves the
// interface telling the truth. The portal simply never called it. This routes
// the portal through the same atomic path rather than adding a second one.
func (a appModel) adoptPortalModel() tea.Msg {
	cfg := config.Get()
	if cfg == nil || a.app == nil || a.app.CoderAgent == nil {
		return util.InfoMsg{Type: util.InfoTypeInfo, Msg: "Provider updated."}
	}
	want := cfg.Agents[config.AgentCoder].Model
	if want == "" {
		return util.InfoMsg{Type: util.InfoTypeInfo, Msg: "Provider updated."}
	}
	if a.app.CoderAgent.Model().ID == want {
		return util.InfoMsg{Type: util.InfoTypeInfo, Msg: "Provider updated — still on " + modelLabel(want) + "."}
	}
	if _, err := a.app.CoderAgent.Update(config.AgentCoder, want); err != nil {
		// Say exactly what is true: the credential is saved, and the session is
		// NOT on the new model. Silence here is what produced the original bug.
		return util.InfoMsg{
			Type: util.InfoTypeError,
			Msg: fmt.Sprintf("Provider saved, but this session is STILL answering as %s — could not switch to %s: %v",
				modelLabel(a.app.CoderAgent.Model().ID), modelLabel(want), err),
		}
	}
	return util.InfoMsg{
		Type: util.InfoTypeInfo,
		Msg:  "Now answering as " + modelLabel(want) + ". Use /model for a different one from this provider.",
	}
}

// shouldEchoNotice decides whether a status notice ALSO belongs in the
// transcript.
//
// GORILLA OVERRIDE (2026-08-21): Echo used to be opt-in, and almost nothing
// opted in — so /update, /purge, the AGENTS.md verdict and every provider error
// existed only as whatever fraction of themselves fitted the footer. The rule is
// now the actual reason those notices were lost: if it does not FIT, it is
// echoed. The footer keeps its flash either way.
//
// InfoBudget is the status bar's own truncation budget, not an estimate of it,
// so the two can never disagree about whether the text was cut. A budget of
// zero means no WindowSizeMsg has arrived yet — the footer does not truncate in
// that state either, so neither do we echo. An explicit ReportInfoEcho still
// echoes at any length, because "important enough to keep" is a separate
// judgement from "too long to show".
func (a appModel) shouldEchoNotice(msg util.InfoMsg) bool {
	if !a.scrollback || strings.TrimSpace(msg.Msg) == "" {
		return false
	}
	if msg.Echo {
		return true
	}
	budget := a.status.InfoBudget()
	return budget > 0 && ansi.StringWidth(msg.Msg) > budget
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

// isRecoverFlag recognises the recovery flag in the forms someone will actually
// type. Written permissively on purpose: the alternative to matching "-recover"
// is silently treating it as a research question and spending money on it.
func isRecoverFlag(arg string) bool {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "--recover", "-recover", "recover", "--resume", "-resume", "resume":
		return true
	}
	return false
}

// assemblyWindow is the context window of the model that will do the write-up —
// the one currently selected, because /osint --recover is normally run after
// /model precisely to pick a bigger one. Zero means unknown, and unknown must
// not block the attempt: the findings are already on disk, so trying and failing
// costs a turn rather than the run.
func (a appModel) assemblyWindow() int {
	if a.app == nil || a.app.CoderAgent == nil {
		return 0
	}
	// Leave a third of the window for the dossier the model has to WRITE. A
	// prompt that exactly fills the window leaves no room for the answer, which
	// is the same mistake the original run made from the other direction.
	return int(float64(a.app.CoderAgent.Model().ContextWindow) * 0.66)
}

// refreshModelCatalogues re-fetches every provider catalogue that can be
// fetched, and reports what changed in the units a user cares about.
//
// GORILLA OVERRIDE (2026-08-20): the pair to /purge. Refresh already existed as
// two separate CLI subcommands (`models refresh`, `models refresh-antigravity`)
// which meant quitting the session to run them — so in practice nobody did, and
// the picker showed whatever was cached the day it was first populated,
// including models the provider had since retired.
//
// Hidden models stay hidden across a refresh: the picker filters on
// config.IsModelHidden at list time, so re-adding an entry to the registry does
// not put it back in front of someone who rejected it. That is the whole reason
// hiding is a persisted list rather than a deletion.
func (a *appModel) refreshModelCatalogues() tea.Cmd {
	return func() tea.Msg {
		dir := config.CacheBase()
		var notes []string

		// 1. OpenRouter - a plain HTTP fetch, no credential needed.
		if res, err := models.RefreshOpenRouter(dir); err != nil {
			notes = append(notes, fmt.Sprintf("OpenRouter failed: %v", err))
		} else {
			notes = append(notes, fmt.Sprintf("OpenRouter %d usable (+%d, -%d)",
				res.Usable, len(res.Added), len(res.Removed)))
		}

		// 2. Antigravity - needs the stored Gmail login. Absent credentials is
		// not a failure, it is "you are not signed in", and saying so is more
		// use than an error nobody can act on.
		if creds, err := auth.LoadAntigravityCreds(); err != nil || creds == nil {
			notes = append(notes, "Antigravity skipped (not signed in)")
		} else if fetched, ferr := creds.FetchAvailableModels(context.Background()); ferr != nil {
			notes = append(notes, fmt.Sprintf("Antigravity failed: %v", ferr))
		} else {
			rows := make([]models.AntigravityRow, 0, len(fetched))
			for id, m := range fetched {
				rows = append(rows, models.AntigravityRow{
					ID: id, DisplayName: m.DisplayName, APIProvider: m.APIProvider,
					MaxTokens: m.MaxTokens, MaxOutputTokens: m.MaxOutputTokens,
					SupportsImages: m.SupportsImages, SupportsThinking: m.SupportsThinking,
					IsInternal: m.IsInternal,
				})
			}
			if res, rerr := models.RefreshAntigravity(dir, rows); rerr != nil {
				notes = append(notes, fmt.Sprintf("Antigravity failed: %v", rerr))
			} else {
				notes = append(notes, fmt.Sprintf("Antigravity %d usable", res.Usable))
			}
		}

		// 2b. ChatGPT sign-in. GORILLA OVERRIDE (2026-08-23): this provider used
		// to be in the "ships with the app" list below, and that is exactly how
		// it rotted. The owner had Codex running gpt-5.6-luna on the same free
		// account while this program offered him a model OpenAI retires on 31
		// Aug 2026. The list is now fetched like everyone else's.
		if creds, err := auth.LoadChatGPTCreds(); err != nil || creds == nil {
			notes = append(notes, "ChatGPT skipped (not signed in)")
		} else if status, body, ferr := creds.ProbeBackend(context.Background()); ferr != nil {
			notes = append(notes, fmt.Sprintf("ChatGPT failed: %v", ferr))
		} else if status != 200 {
			notes = append(notes, fmt.Sprintf("ChatGPT failed (HTTP %d)", status))
		} else if res, rerr := models.RefreshChatGPT(dir, []byte(body)); rerr != nil {
			notes = append(notes, fmt.Sprintf("ChatGPT failed: %v", rerr))
		} else {
			note := fmt.Sprintf("ChatGPT %d usable", res.Usable)
			if len(res.Added) > 0 || len(res.Removed) > 0 {
				note += fmt.Sprintf(" (+%d, -%d)", len(res.Added), len(res.Removed))
			}
			if len(res.Removed) > 0 {
				note += ", retired: " + strings.Join(res.Removed, ", ")
			}
			notes = append(notes, note)
		}

		// 3. Every provider whose list is FETCHED rather than compiled in —
		// Groq, Cerebras, Anthropic, OpenAI, xAI, DeepSeek. Skipped silently
		// when no key is on file: "you are not signed in" is not a failure, and
		// listing eight providers as failed because the user has six of them
		// unconfigured buries the two that matter.
		for p, cat := range models.LiveCatalogues {
			key := config.ProviderAPIKey(p)
			if strings.TrimSpace(key) == "" {
				continue
			}
			res, err := models.FetchProviderCatalogue(p, key, dir)
			if err != nil {
				notes = append(notes, fmt.Sprintf("%s failed: %v", cat.Label, err))
				continue
			}
			note := catalogueNote(res)
			notes = append(notes, note)
		}

		// 4. Every OpenAI-compatible endpoint the user configured - NVIDIA NIM,
		// a local llama.cpp, LM Studio, anything added with /connect. Each is
		// re-asked what it serves right now, which is the only way a model
		// retired since the endpoint was added stops being offered.
		if n := config.ReregisterLocalEndpoints(); n > 0 {
			notes = append(notes, fmt.Sprintf("%d configured endpoint(s) re-asked", n))
		}

		// 5. Say what CANNOT be refreshed, so silence is not read as success.
		//
		// GORILLA OVERRIDE (2026-08-21): this list used to name eleven providers.
		// It is now two — Gemini (API key) and the sign-in providers — because
		// everything else fetches its own list. Azure, Copilot, Bedrock and
		// VertexAI are not here at all: they were removed, since none of them is
		// reachable without an enterprise account or a card.
		notes = append(notes, "Gemini ships with the app and updates with it")

		if n := config.HiddenCount(); n > 0 {
			notes = append(notes, fmt.Sprintf("%d hidden stayed hidden (H to review)", n))
		}
		return refreshSummaryMsg(notes)
	}
}

// catalogueNote is one provider's line in the /update report.
//
// GORILLA OVERRIDE (2026-08-21): the SKIPPED count is reported, not just the
// usable one.
//
// A fetched catalogue is filtered before it is registered — speech, image,
// embedding and safety-classifier models are not chat models, and selecting one
// produces a bare HTTP 400 that reads as a broken key. That filter is a list of
// substrings, so it can be too greedy, and a too-greedy filter is invisible from
// the usable count alone: "OpenAI 5 usable" looks like a small catalogue rather
// than like 73 models thrown away by mistake. Printing both makes the ratio
// visible at a glance to the first person who runs this with a key.
//
// This exists because the release it shipped in could not be tested against a
// paid provider — nobody here has an Anthropic, OpenAI or DeepSeek key. The
// honest response to "I cannot verify this" is to make the failure legible to
// whoever can, rather than to assert it works.
func catalogueNote(res models.CatalogueResult) string {
	note := fmt.Sprintf("%s %d usable", res.Label, res.Usable)
	if len(res.Added) > 0 || len(res.Removed) > 0 {
		note += fmt.Sprintf(" (+%d, -%d)", len(res.Added), len(res.Removed))
	}
	if res.Skipped > 0 {
		note += fmt.Sprintf(", %d skipped", res.Skipped)
	}
	// Name the retired ones. A model that vanished from a provider is the single
	// most useful thing this command can tell you — it is the difference between
	// "your bookmark is gone" and a 400 the next time you pick it.
	if len(res.Removed) > 0 {
		note += " — retired: " + strings.Join(res.Removed, ", ")
	}
	return note
}

// refreshSummaryMsg packs the per-provider notes into one notice.
//
// GORILLA OVERRIDE (2026-08-21): Echo is set. The joined summary is far wider
// than any terminal — the screenshot that prompted this showed it cut at
// "2 configured endpoint(s) re-asked | bu...", losing the line whose whole job
// is to say the built-in providers were NOT refreshed. That is exactly the
// failure ReportInfoEcho was built for: the footer flashes the head of it, the
// transcript keeps all of it, scrollable and copyable. A refresh the user has
// to re-run to find out what it did is not a report.
func refreshSummaryMsg(notes []string) util.InfoMsg {
	return util.InfoMsg{
		Type: util.InfoTypeInfo,
		Msg:  strings.Join(notes, " | "),
		Echo: true,
	}
}

// drainPermissionQueue picks the next question actually worth asking.
//
// GORILLA FIX (2026-08-23): extracted from Update so it can be tested. The bug
// it exists to prevent does not show up in a unit test of the service or of the
// dialog, because it lived in the seam between them: the dialog holds one
// request, the service publishes many, and nothing counted.
//
// Entries already settled by the answer just given are granted through `grant`
// and skipped. Returns the first unsettled request, or nil if the queue is
// exhausted, along with what remains after it.
func drainPermissionQueue(
	queue []permission.PermissionRequest,
	covered func(permission.PermissionRequest) bool,
	grant func(permission.PermissionRequest),
) (*permission.PermissionRequest, []permission.PermissionRequest) {
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if covered(next) {
			grant(next)
			continue
		}
		return &next, queue
	}
	return nil, queue
}

// spawnLabelBudget is how many columns the helper label may use in the spawn
// notice, given the terminal width.
//
// The fixed chrome is measured from the format string rather than counted by
// hand, so editing the wording cannot silently invalidate the arithmetic. That
// is the same trap as the constant it replaces: a number that was true once.
func spawnLabelBudget(width int, id string) int {
	// Everything in the notice except the label itself. The gorilla is two
	// columns wide, which is why this is measured and not len()'d.
	chrome := lipgloss.Width("🦍 helper "+id+" spawned:   (/tasks to view or kill)") +
		spawnNoticeDecoration

	budget := width - chrome
	// A floor, because a very narrow terminal should still show SOMETHING of
	// the role rather than collapsing to an ellipsis. The status line does the
	// final fit to real columns, so overshooting here is safe and undershooting
	// is not.
	if budget < spawnLabelFloor {
		return spawnLabelFloor
	}
	return budget
}

const (
	// spawnNoticeDecoration is the width the status component wraps around a
	// message (the warning gorillas either side). Reserved so the label does
	// not get cut a second time by something this function cannot see.
	spawnNoticeDecoration = 24
	// spawnLabelFloor keeps a recognisable amount of the role visible on a
	// narrow pane. Roles are distinguished by their FIRST word (ADVERSARY,
	// REQUIREMENT, PRIOR ART), so this needs to survive that plus a little.
	spawnLabelFloor = 24
)

// liveHelperSpend is what the helpers of the RUN IN FLIGHT have spent so far.
//
// Summed from the registry's session ids rather than from "every child of this
// conversation", and the difference is not cosmetic. When a research tool
// returns it adds the whole run onto the parent session, so a query that summed
// every child would count a FINISHED run twice: once inside the parent's own
// total and again in the children that fed it. The registry holds only the
// helpers of the run currently in flight and is purged per tool call, so this
// is correct at every moment rather than only at the end.
//
// Best effort. A figure that cannot be read is reported as zero and the notice
// simply omits it, because a wrong number here is worse than no number: this is
// the line somebody would use to decide whether to kill a run.
func (a appModel) liveHelperSpend() float64 {
	if a.selectedSession.ID == "" {
		return 0
	}
	live := agent.LiveSubAgentSessions(a.selectedSession.ID)
	if len(live) == 0 {
		return 0
	}
	inFlight := make(map[string]bool, len(live))
	for _, id := range live {
		inFlight[id] = true
	}

	children, err := a.app.Sessions.Children(context.Background(), a.selectedSession.ID)
	if err != nil {
		return 0
	}
	var total float64
	for _, c := range children {
		if inFlight[c.ID] {
			total += c.Cost
		}
	}
	return total
}
