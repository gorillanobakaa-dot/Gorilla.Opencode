// GORILLA OVERRIDE: this file did not exist upstream. It renders the
// context loadout menu (opened with /context): a transparent, Slackware-
// style view of everything sent to the model every turn, its token cost,
// and switches to strip it down. See internal/config/loadout.go.
package dialog

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// GORILLA OVERRIDE: use nearly the full terminal width so no message or
// tradeoff line ever wraps or truncates.
const (
	loadoutMinWidth    = 100
	loadoutSidePadding = 6 // border + breathing room on each side
)

// CloseLoadoutDialogMsg closes the loadout menu.
type CloseLoadoutDialogMsg struct{}

// LoadoutChangedMsg signals the loadout changed (agent should rebuild tools).
type LoadoutChangedMsg struct{}

type LoadoutDialog interface {
	tea.Model
	layout.Bindings
}

type loadoutDialogCmp struct {
	selectedIdx int
	termWidth   int
	// GORILLA OVERRIDE: the terminal HEIGHT, which this dialog previously never
	// recorded. That is why it could not size itself: it rendered every dial, every
	// feature row and every extra unconditionally, asking for 37 rows. A cell-grid
	// test found it cut off on an ordinary 80x24 terminal, and even on 100x30.
	termHeight int
	// fitter caches the row count that last fitted, so an unchanged size costs one
	// render instead of a fresh search on every keystroke.
	fitter layout.Fitter
	// featureTop is the first feature row shown, so a long list scrolls instead of
	// running off the screen.
	featureTop int
}

// width returns the dialog inner width — as wide as the terminal allows,
// so the full messages are always readable.
func (m *loadoutDialogCmp) width() int {
	if m.termWidth <= 0 {
		return loadoutMinWidth
	}
	w := m.termWidth - loadoutSidePadding
	if w < loadoutMinWidth {
		w = loadoutMinWidth
	}
	return w
}

type loadoutKeyMap struct {
	Up, Down, Left, Right, Toggle, Reset, LowBW, RateDown, RateUp, LeashDown, LeashUp, Escape key.Binding
	// AllLSP is the bulk language-server switch. GORILLA OVERRIDE.
	AllLSP key.Binding
}

var loadoutKeys = loadoutKeyMap{
	Up:   key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "up")),
	Down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "down")),
	// GORILLA OVERRIDE: ←/→ adjust the selected dial (speed / helpers). Arrow
	// keys work on every keyboard layout — unlike -/+/[/], which are awkward
	// or hidden on non-US keyboards (this was a real pain on a JP keyboard).
	Left:   key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "less")),
	Right:  key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "more")),
	Toggle: key.NewBinding(key.WithKeys(" ", "enter"), key.WithHelp("space", "toggle/change")),
	Reset:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset defaults")),
	// GORILLA OVERRIDE: one-key low-bandwidth profile (optional tools off).
	LowBW: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "low-bandwidth preset")),
	// GORILLA OVERRIDE: bulk switch for the language servers. Nine configured
	// servers meant nine toggles to get to a quiet session; granular control is
	// only usable with an "all" beside it.
	AllLSP: key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "all LSPs on/off")),
	// Legacy power-user shortcuts (still work regardless of what's selected).
	RateDown:  key.NewBinding(key.WithKeys("-", "_")),
	RateUp:    key.NewBinding(key.WithKeys("+", "=")),
	LeashDown: key.NewBinding(key.WithKeys("[")),
	LeashUp:   key.NewBinding(key.WithKeys("]")),
	Escape:    key.NewBinding(key.WithKeys("esc")),
}

// The two adjustable dials occupy the first rows of the navigable list; the
// switchable tool/prompt components follow. selectedIdx spans both.
const (
	rowPace  = 0 // "AI request speed" dial
	rowLeash = 1 // "Extra AI helpers" dial
	numDials = 2
)

func loadoutRowCount() int {
	return numDials + len(config.LoadoutComponents) + len(config.Extras)
}

// extrasFirstRow is the index where the "show me the working" section starts.
//
// GORILLA OVERRIDE: extras are a THIRD section, kept apart from the feature rows
// above them on purpose. A loadout row's number is tokens added to the prompt on
// every turn and feeds the context budget at the top of this dialog; extras do not
// touch prompt size at all — one makes the model generate more, the rest only
// change what is displayed. Listing them together would put figures into that
// budget that do not belong to it.
func extrasFirstRow() int { return numDials + len(config.LoadoutComponents) }

// extraAtRow returns the extra a row index refers to, if it is in that section.
func extraAtRow(idx int) (config.Extra, bool) {
	i := idx - extrasFirstRow()
	if i < 0 || i >= len(config.Extras) {
		return config.Extra{}, false
	}
	return config.Extras[i], true
}

func NewLoadoutDialogCmp() LoadoutDialog { return &loadoutDialogCmp{} }

func (m *loadoutDialogCmp) Init() tea.Cmd { return nil }

func (m *loadoutDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, loadoutKeys.Up):
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}
		case key.Matches(msg, loadoutKeys.Down):
			if m.selectedIdx < loadoutRowCount()-1 {
				m.selectedIdx++
			}
		case key.Matches(msg, loadoutKeys.Left):
			return m, m.adjustSelected(-1)
		case key.Matches(msg, loadoutKeys.Right):
			return m, m.adjustSelected(+1)
		case key.Matches(msg, loadoutKeys.Toggle):
			// On a dial, "change" nudges it up; on a tool row, it toggles.
			if m.selectedIdx < numDials {
				return m, m.adjustSelected(+1)
			}
			if e, ok := extraAtRow(m.selectedIdx); ok {
				now := !config.ExtraEnabled(e.ID)
				if err := config.SetExtra(e.ID, now); err != nil {
					return m, util.ReportError(err)
				}
				// Say what it costs at the moment of the decision, not only on a
				// screen the user saw once at first run.
				if e.Cost == config.CostGeneration && now {
					return m, util.ReportWarn(e.Name + " is ON — this makes the model generate more (see /settings for what that costs)")
				}
				return m, util.CmdHandler(LoadoutChangedMsg{})
			}
			config.ToggleLoadout(config.LoadoutComponents[m.selectedIdx-numDials].ID)
			return m, util.CmdHandler(LoadoutChangedMsg{})
		case key.Matches(msg, loadoutKeys.Reset):
			config.ResetLoadout()
			return m, util.CmdHandler(LoadoutChangedMsg{})
		case key.Matches(msg, loadoutKeys.AllLSP):
			// Turn them all off unless they are already all off, in which case
			// this is the way back on — one key, both directions.
			on, off := config.LSPLoadoutCounts()
			if on+off == 0 {
				return m, util.ReportInfo("No language servers are configured")
			}
			enable := on == 0
			n := config.SetAllLSPs(enable)
			word := "off"
			if enable {
				word = "on"
			}
			return m, tea.Batch(
				util.CmdHandler(LoadoutChangedMsg{}),
				util.ReportInfo(fmt.Sprintf("Switched %d language server(s) %s — applies at next launch", n, word)),
			)
		case key.Matches(msg, loadoutKeys.LowBW):
			n := config.ApplyLowBandwidthLoadout()
			return m, tea.Batch(
				util.CmdHandler(LoadoutChangedMsg{}),
				util.ReportInfo(fmt.Sprintf("Low-bandwidth loadout applied (~%s tokens/turn%s)", commaInt(n), loadoutCostSuffix())),
			)
		// Legacy direct shortcuts (work regardless of selection).
		case key.Matches(msg, loadoutKeys.RateDown):
			config.StepRateLimitRPM(-1)
			return m, util.ReportInfo("AI SERVER requests: " + rateLimitLabel())
		case key.Matches(msg, loadoutKeys.RateUp):
			config.StepRateLimitRPM(+1)
			return m, util.ReportInfo("AI SERVER requests: " + rateLimitLabel())
		case key.Matches(msg, loadoutKeys.LeashDown):
			config.StepMaxSubAgents(-1)
			return m, tea.Batch(util.CmdHandler(LoadoutChangedMsg{}), util.ReportInfo("GORILLA AGENTS/SUBAGENTS: "+subAgentLabel()))
		case key.Matches(msg, loadoutKeys.LeashUp):
			config.StepMaxSubAgents(+1)
			return m, tea.Batch(util.CmdHandler(LoadoutChangedMsg{}), util.ReportInfo("GORILLA AGENTS/SUBAGENTS: "+subAgentLabel()))
		case key.Matches(msg, loadoutKeys.Escape):
			return m, util.CmdHandler(CloseLoadoutDialogMsg{})
		}
	}
	return m, nil
}

// adjustSelected applies a −1/+1 step to whichever dial is highlighted. On a
// tool row it does nothing (those toggle with space). dir<0 = less, dir>0 = more.
func (m *loadoutDialogCmp) adjustSelected(dir int) tea.Cmd {
	switch m.selectedIdx {
	case rowPace:
		config.StepRateLimitRPM(dir)
		return util.ReportInfo("AI SERVER requests: " + rateLimitLabel())
	case rowLeash:
		config.StepMaxSubAgents(dir)
		// Nuclear toggles the helper tool's schema tokens in/out of the loadout.
		return tea.Batch(util.CmdHandler(LoadoutChangedMsg{}), util.ReportInfo("GORILLA AGENTS/SUBAGENTS: "+subAgentLabel()))
	}
	return nil
}

// View renders at a height that is MEASURED to fit the terminal.
//
// GORILLA OVERRIDE: it used to render every row unconditionally and never even
// recorded the terminal height, so it asked for 37 rows and was silently cut off on
// an 80x24 screen. layout.FitHeight brings the feature window down until the whole
// dialog genuinely fits.
func (m *loadoutDialogCmp) View() string {
	total := max(1, len(config.LoadoutComponents))
	// Progressively leaner, in priority order: the switches are the point of this
	// dialog, so explanatory prose gives way before any row does. Same approach as
	// /help and the sign-in overlay.
	for i, compact := range []bool{false, true} {
		// The scroll note appears and disappears with the selection, so it counts.
		key := uint64(m.selectedIdx)*1315423911 + uint64(i)
		view := m.fitter.Fit(m.termHeight, total, 1, key, func(rows int) string {
			return m.renderAt(rows, compact)
		})
		if m.termHeight <= 0 || lipgloss.Height(view) <= m.termHeight {
			return view
		}
	}
	return m.fitter.Fit(m.termHeight, total, 1, uint64(m.selectedIdx)*1315423911+99,
		func(rows int) string { return m.renderAt(rows, true) })
}

// clampFeatureTop keeps the selected feature row inside the visible window.
func (m *loadoutDialogCmp) clampFeatureTop(window, total int) {
	if window >= total {
		m.featureTop = 0
		return
	}
	sel := m.selectedIdx - numDials // -1 or less means a dial is selected
	if sel < 0 {
		m.featureTop = 0
		return
	}
	if sel < m.featureTop {
		m.featureTop = sel
	}
	if sel >= m.featureTop+window {
		m.featureTop = sel - window + 1
	}
	m.featureTop = max(0, min(m.featureTop, total-window))
}

func (m *loadoutDialogCmp) renderAt(featureRows int, compact bool) string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := m.width()

	total := config.LoadoutActiveTokens()
	header := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render("Context loadout — what every turn costs")
	sub := base.Foreground(t.TextMuted()).Width(w).
		Render(fmt.Sprintf("~%s tokens sent on EVERY turn, even to say \"yo\"%s.", commaInt(total), loadoutCostSuffix()))
	fixed := base.Foreground(t.TextMuted()).Width(w).
		Render(fmt.Sprintf("(base system prompt ~%s is always on; the rest is yours to cut)", commaInt(config.LoadoutBaseTokens())))
	// rowStyle applies the shared selected / disabled styling to any row.
	rowStyle := func(selected, muted bool) lipgloss.Style {
		s := base.Width(w)
		switch {
		case selected:
			return s.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		case muted:
			return s.Foreground(t.TextMuted())
		}
		return s
	}
	fitLine := func(line string) string {
		if r := []rune(line); len(r) > w-1 {
			return string(r[:w-2]) + "…"
		}
		return line
	}

	// --- Section 1: the two Gorilla control dials (arrow-key adjustable) ---
	dialHeader := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render("🦍 GORILLA CONTROLS — tune for your connection / free tier  (↑↓ pick a line · ←→ change it):")
	paceLine := fitLine(fmt.Sprintf(" %-32s ‹ ←/→ ›  %s", "AI SERVER requests — pace-setter", paceDesc()))
	leashLine := fitLine(fmt.Sprintf(" %-32s ‹ ←/→ ›  %s", "GORILLA AGENTS/SUBAGENTS — leash", leashDesc()))

	var rows []string
	rows = append(rows, rowStyle(m.selectedIdx == rowPace, false).Render(paceLine))
	rows = append(rows, rowStyle(m.selectedIdx == rowLeash, false).Render(leashLine))

	// --- Section 2: switch features on/off ---
	featHeader := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render("Turn features on/off  (space):")
	for i, c := range config.LoadoutComponents {
		on := config.LoadoutEnabled(c.ID)
		box := "[ ]"
		if on {
			box = "[x]"
		}
		mark := ""
		if c.Critical {
			mark = " ⚠"
		}
		// GORILLA OVERRIDE: real measured cost via ComponentTokens.
		line := fitLine(fmt.Sprintf("%s %-18s ~%-6s  %s%s", box, c.Name, commaInt(config.ComponentTokens(c)), tradeoffText(on, c.Tradeoff), mark))
		rows = append(rows, rowStyle(m.selectedIdx == i+numDials, !on).Render(line))
	}

	// --- Section 3: show me the working ---
	// GORILLA OVERRIDE: every row states its own cost. Only one of these makes the
	// model generate more; the rest are display-only and free, and saying so
	// matters — a user told "extras cost money" would reasonably switch all of them
	// off and lose the forensic record for no saving at all.
	extrasHeader := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render("Show me the working  (space):")
	var extraRows []string
	for i, e := range config.Extras {
		on := config.ExtraEnabled(e.ID)
		box := "[ ]"
		if on {
			box = "[x]"
		}
		cost := "free"
		if e.Cost == config.CostGeneration {
			cost = "COSTS EXTRA"
		}
		line := fitLine(fmt.Sprintf("%s %-30s %-12s  %s", box, e.Name, cost, e.What))
		extraRows = append(extraRows, rowStyle(m.selectedIdx == extrasFirstRow()+i, !on).Render(line))
	}
	extrasNote := base.Foreground(t.TextMuted()).Width(w).
		Render(fitLine("  \"free\" = already generated and paid for; hiding it saves nothing. \"COSTS EXTRA\" = the model writes more."))

	help := base.Foreground(t.TextMuted()).Width(w).
		Render("↑↓ pick · ←→ dial · space toggle · L all LSPs · l low-bw · r reset · esc close   ⚠ = disabling cripples the agent")

	// Window the feature rows rather than rendering all of them. featureRows is
	// decided by measurement in View(), not by a guess at how much chrome the rest
	// of the dialog takes.
	feature := rows[numDials:]
	m.clampFeatureTop(featureRows, len(feature))
	end := min(m.featureTop+featureRows, len(feature))
	shown := feature[m.featureTop:end]

	// Say so when rows are off-screen, or a hidden switch looks like a missing one.
	scrollNote := ""
	if len(shown) < len(feature) {
		scrollNote = base.Foreground(t.TextMuted()).Width(w).
			Render(fitLine(fmt.Sprintf("  showing %d-%d of %d — ↑↓ to reach the rest",
				m.featureTop+1, end, len(feature))))
	}

	parts := []string{header}
	if !compact {
		// The subtitle and the context-size line explain; they are not state you
		// act on, so they are the first to go on a short terminal.
		parts = append(parts, sub, fixed, "")
	}
	parts = append(parts,
		dialHeader,
		rows[rowPace], rows[rowLeash], "",
		featHeader,
		lipgloss.JoinVertical(lipgloss.Left, shown...),
	)
	if scrollNote != "" {
		parts = append(parts, scrollNote)
	}
	if !compact {
		parts = append(parts, "")
	}
	parts = append(parts,
		extrasHeader,
		lipgloss.JoinVertical(lipgloss.Left, extraRows...),
	)
	if !compact {
		parts = append(parts, extrasNote, "")
	}
	parts = append(parts, help)
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(styles.PanelBackground()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

// loadoutCostSuffix turns the per-turn token overhead into money at the
// active model's input rate, e.g. " — ≈ $0.0033/turn (Grok 4.5 @ $3.00/1M in)".
// Empty string if we can't price it and there is nothing useful to add.
func loadoutCostSuffix() string {
	dollars, per1MIn, name, priced := config.LoadoutCost()
	if !priced {
		if name != "" {
			return fmt.Sprintf(" — unpriced (%s: no price table entry)", name)
		}
		return ""
	}
	if per1MIn <= 0 {
		return fmt.Sprintf(" — ≈ $0.00/turn (%s: free / flat-rate tier)", name)
	}
	return fmt.Sprintf(" — ≈ %s/turn (%s @ $%.2f/1M in)", formatUSD(dollars), name, per1MIn)
}

// paceDesc / leashDesc are the full descriptive strings shown in the dial rows —
// they spell out WHAT the setting controls (requests to the AI server; the
// agents/subagents the main agent may spawn) so a newcomer isn't left guessing.

func paceDesc() string {
	rpm := config.RateLimitRPM()
	if rpm <= 0 {
		return "UNLIMITED — no pacing (floors it; paid/high tiers only)"
	}
	return fmt.Sprintf("%d/min (spaces calls ~%.1fs apart) — lower if you get \"rate limited\"", rpm, 60.0/float64(rpm))
}

func leashDesc() string {
	switch n := config.MaxSubAgents(); {
	case n == config.SubAgentsNuclear:
		return "☢ GORILLA NUCLEAR — ALL FUCKING AGENTS/SUBAGENTS DISABLED (fewest calls; main agent works solo)"
	case n == config.SubAgentsUnlimited:
		return "UNLIMITED — no leash (more agents = faster but more server requests; paid/high tiers)"
	default:
		return fmt.Sprintf("up to %d agent(s)/subagent(s) per turn — each one adds AI-server requests", n)
	}
}

// subAgentLabel / rateLimitLabel are the shorter status-bar toast versions shown
// when a dial changes (and by the legacy -/+/[/] shortcuts).
func subAgentLabel() string {
	switch n := config.MaxSubAgents(); {
	case n == config.SubAgentsNuclear:
		return "☢ GORILLA NUCLEAR — ALL FUCKING AGENTS/SUBAGENTS DISABLED (main agent works solo)"
	case n == config.SubAgentsUnlimited:
		return "UNLIMITED (no leash — paid/high tiers only)"
	default:
		return fmt.Sprintf("up to %d agent(s)/subagent(s) per turn", n)
	}
}

func rateLimitLabel() string {
	rpm := config.RateLimitRPM()
	if rpm <= 0 {
		return "UNLIMITED (no pacing — floors it; paid/high tiers only)"
	}
	return fmt.Sprintf("%d requests/min (spaces calls ~%.1fs apart)", rpm, 60.0/float64(rpm))
}

// formatUSD prints a dollar amount with enough precision for sub-cent
// per-turn figures: 4 decimals under a cent, 2 otherwise.
func formatUSD(d float64) string {
	if d > 0 && d < 0.01 {
		return fmt.Sprintf("$%.4f", d)
	}
	return fmt.Sprintf("$%.2f", d)
}

func tradeoffText(on bool, tradeoff string) string {
	if on {
		return "off: " + tradeoff
	}
	return "OFF — " + tradeoff
}

// commaInt formats an int with thousands separators.
func commaInt(n int) string {
	s := fmt.Sprintf("%d", n)
	out := ""
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += ","
		}
		out += string(r)
	}
	return out
}

func (m *loadoutDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(loadoutKeys)
}
