// GORILLA OVERRIDE: this file did not exist upstream. It renders the
// context loadout menu (opened with /context): a transparent, Slackware-
// style view of everything sent to the model every turn, its token cost,
// and switches to strip it down. See internal/config/loadout.go.
package dialog

import (
	"fmt"
	"sort"
	"strings"

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
	loadoutMinWidth     = 100 // fallback when the terminal size is not yet known
	loadoutMaxWidth     = 140 // cap: readable line length on an ultrawide screen
	loadoutHardMinWidth = 20  // below this nothing is legible anyway
	loadoutSidePadding  = 6   // border + breathing room on each side
)

// CloseLoadoutDialogMsg closes the loadout menu.
type CloseLoadoutDialogMsg struct{}

// LoadoutChangedMsg signals the loadout changed (agent should rebuild tools).
type LoadoutChangedMsg struct{}

type LoadoutDialog interface {
	tea.Model
	layout.Bindings
	// SetLastUsage hands the dialog the most recent turn's token usage so it
	// can report cache reuse. reported says whether the provider gave any
	// cache figures at all, which is a different question from whether
	// anything was cached.
	SetLastUsage(input, cacheRead, cacheCreate int64, reported bool)
}

type loadoutDialogCmp struct {
	// Last turn's token usage, for the cache-reuse line. See SetLastUsage.
	usageInput       int64
	usageCacheRead   int64
	usageCacheCreate int64
	usageReported    bool

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
//
// GORILLA FIX (2026-08-17): this used to floor the width UP to loadoutMinWidth
// (100), so on an 80-column terminal it asked for 100 columns of content plus 6
// of chrome and drew a 106-column frame into an 80-column window. That breaks
// the invariant CLAUDE.md puts in capitals: NO LINE IN THE FRAME MAY BE WIDER
// THAN THE TERMINAL. Bubbletea's inline renderer erases its last frame by
// moving the cursor up by the number of LOGICAL lines it drew, so a line that
// wraps to two physical rows makes the erase under-reach by a row per render —
// which is how rows get stranded in the scrollback.
//
// Chrome is SUBTRACTED from the terminal, never added to a content size. The
// 100 is now a PREFERRED width (a cap, so the dialog does not sprawl across an
// ultrawide screen), not a floor. On a terminal too narrow for comfort the rows
// truncate with an ellipsis, which is ugly and correct; drawing outside the
// window is neither.
func (m *loadoutDialogCmp) width() int {
	if m.termWidth <= 0 {
		return loadoutMinWidth
	}
	w := m.termWidth - loadoutSidePadding
	if w > loadoutMaxWidth {
		w = loadoutMaxWidth
	}
	if w < loadoutHardMinWidth {
		w = loadoutHardMinWidth
	}
	return w
}

type loadoutKeyMap struct {
	Up, Down, Left, Right, Toggle, Reset, LowBW, RateDown, RateUp, LeashDown, LeashUp, Escape key.Binding
	// AllLSP is the bulk language-server switch. GORILLA OVERRIDE.
	AllLSP key.Binding
}

var loadoutKeys = loadoutKeyMap{
	Up:   key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up", "up")),
	Down: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down", "down")),
	// GORILLA OVERRIDE: <-/-> adjust the selected dial (speed / helpers). Arrow
	// keys work on every keyboard layout — unlike -/+/[/], which are awkward
	// or hidden on non-US keyboards (this was a real pain on a JP keyboard).
	Left:   key.NewBinding(key.WithKeys("left"), key.WithHelp("<-", "less")),
	Right:  key.NewBinding(key.WithKeys("right"), key.WithHelp("->", "more")),
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

// sortedLoadout is the display order of the feature rows: alphabetical by
// name, case-insensitive.
//
// GORILLA OVERRIDE: registry order was registration order — hand-written tools,
// then prompt blocks, then whatever RegisterLoadoutComponents appended, with
// language servers at the end. Fifteen-plus rows in that order is a mess to
// scan ("right now is a mess" — 2026-08-17). Sorting happens HERE, at display,
// not in the registry: other consumers (calibration, the LSP gate) key on ID
// and do not care, and both Update and renderAt must index the SAME order or
// space would toggle a different row from the one highlighted.
func sortedLoadout() []config.LoadoutComponent {
	rows := make([]config.LoadoutComponent, len(config.LoadoutComponents))
	copy(rows, config.LoadoutComponents)
	sort.SliceStable(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	return rows
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

// SetLastUsage hands the dialog the most recent turn's token usage, so it can
// report how much of the prompt was served from cache.
//
// GORILLA OVERRIDE (2026-09-02): prompt caching was worth 8m14s down to 15s on
// this project's own hardware, measured, and NOTHING in the interface showed it.
// The fault that caused it -- a timestamp inside the cached prefix -- went
// unnoticed for exactly that reason, and a future one would too.
//
// reported says whether the provider gave any cache figures at all. That is a
// different question from whether anything was cached, and conflating them
// would be worse than showing nothing: LM Studio sends no prompt_tokens_details
// whatever (measured 2026-09-02) while demonstrably reusing the prefix, so a
// bare "0 cached" would report working caching as broken.
func (m *loadoutDialogCmp) SetLastUsage(input, cacheRead, cacheCreate int64, reported bool) {
	m.usageInput = input
	m.usageCacheRead = cacheRead
	m.usageCacheCreate = cacheCreate
	m.usageReported = reported
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
			config.ToggleLoadout(sortedLoadout()[m.selectedIdx-numDials].ID)
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

	// Truncation helpers, declared before anything that renders text.
	//
	// GORILLA FIX (2026-08-17): every headline, subtitle and hint now passes
	// through fitLine. They used to be handed to a .Width(w) style raw, and
	// lipgloss WRAPS rather than overflowing — so on a narrow terminal each one
	// silently became two or three rows and the dialog grew taller exactly where
	// there was least room. Truncating with an ellipsis costs a few words; the
	// alternative cost rows.
	fitTo := func(line string, width int) string {
		if r := []rune(line); len(r) > width-1 {
			return string(r[:width-4]) + styles.Ellipsis
		}
		return line
	}
	fitLine := func(line string) string { return fitTo(line, w) }

	total := config.LoadoutActiveTokens()
	header := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render(fitLine("Context loadout — what every turn costs"))
	sub := base.Foreground(t.TextMuted()).Width(w).
		Render(fitLine(fmt.Sprintf("~%s tokens sent on EVERY turn, even to say \"yo\"%s.", commaInt(total), loadoutCostSuffix())))
	fixed := base.Foreground(t.TextMuted()).Width(w).
		Render(fitLine(fmt.Sprintf("(base system prompt ~%s is always on; the rest is yours to cut)", commaInt(config.LoadoutBaseTokens()))))
	// GORILLA FIX (2026-08-19): say how good these numbers are.
	//
	// Every figure on this screen comes from infoTokens(), which serialises a
	// schema and divides by four. Measured against a real tokeniser it
	// OVERSTATES by about 10.1%. And the byte cost in FOOTPRINT.md (6.06
	// bytes/token) was measured against a provider that REFUSES compression,
	// so it is a worst case, not a typical one.
	//
	// Both are good enough for the decision this screen exists to inform —
	// which rows to switch off — and neither is good enough to be quoted as a
	// bill. A screen whose whole purpose is honesty about cost cannot present
	// an estimate in the typography of a measurement.
	// GORILLA OVERRIDE (2026-09-02): say whether the prompt was reused.
	//
	// Everything above this line is what a turn COSTS. This is what the cache
	// GAVE BACK, and until now it appeared nowhere in the interface. On this
	// project's own hardware prefix reuse was worth 8m14s down to 15s, measured;
	// the fault that had been destroying it -- a timestamp inside the cached
	// prefix -- survived unnoticed precisely because no screen showed the
	// number. internal/llm/prompt/prompt_stability_test.go now fails if volatile
	// content returns, and this is the other half: a person can see it too.
	cacheLine := ""
	switch {
	case !m.usageReported:
		// NOT the same as "nothing was cached". LM Studio sends no
		// prompt_tokens_details at all (measured 2026-09-02) while demonstrably
		// reusing the prefix, so reporting 0 here would call working caching
		// broken.
		cacheLine = "cache reuse: this endpoint does not report it (local runtimes usually do not)"
	case m.usageCacheRead == 0 && m.usageCacheCreate == 0:
		cacheLine = "cache reuse: none on the last turn — the whole prompt was processed fresh"
	default:
		total := m.usageInput + m.usageCacheRead + m.usageCacheCreate
		pct := 0
		if total > 0 {
			pct = int(float64(m.usageCacheRead) / float64(total) * 100)
		}
		cacheLine = fmt.Sprintf(
			"cache reuse: %s of %s prompt tokens served from cache (%d%%), %s written",
			commaInt(int(m.usageCacheRead)), commaInt(int(total)), pct,
			commaInt(int(m.usageCacheCreate)))
	}
	cache := base.Foreground(t.TextMuted()).Width(w).Render(fitLine(cacheLine))

	accuracy := base.Foreground(t.TextMuted()).Width(w).
		Render(fitLine("estimates, ~10% high: schema bytes / 4, not a real tokeniser"))

	// GORILLA OVERRIDE (2026-09-01): say what the number MEANS, and name the
	// rows worth cutting.
	//
	// The screen listed eighteen rows and their token costs and left the
	// arithmetic to the reader. Someone who already thinks in token budgets can
	// do it. The person this program is written for reads a long list, has no
	// idea which of those things they actually use, and closes the screen having
	// changed nothing — which is how a 10,000-token loadout survives on a
	// machine where it costs six minutes a turn.
	//
	// Two lines, and only when they are earned: the consequence in the unit the
	// reader pays in, and the three specific rows that would give back the most.
	// Both are omitted entirely when the loadout is not actually crowding
	// anything, because advice that is always on screen stops being read.
	var advice []string
	if _, window, _, share := config.LoadoutContextShare(); window > 0 && share >= 0.25 {
		advice = append(advice, base.Foreground(t.Warning()).Width(w).Render(fitLine(
			"Everything switched ON here is re-read by the AI before it answers — "+
				"every single time you press enter.")))
		if cuts, saved := config.LoadoutBiggestCuts(3); saved > 0 {
			names := make([]string, 0, len(cuts))
			for _, c := range cuts {
				names = append(names, fmt.Sprintf("%s (~%s)", c.Name, commaInt(c.Tokens)))
			}
			advice = append(advice, base.Foreground(t.Warning()).Width(w).Render(fitLine(
				fmt.Sprintf("Turn off what you do not use. Biggest right now: %s — ~%s tokens back.",
					strings.Join(names, ", "), commaInt(saved)))))
		}
	}

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
	// GORILLA OVERRIDE (2026-08-17): state is shown by a WORD that flips, not a
	// checkbox. Real-user feedback on v0.1.87: "Which is off/on, is it x'ed or
	// unx'ed and greyed out, regardless the description still shows off". The
	// [x]/[ ] idiom means nothing to someone who never used a terminal, and the
	// tradeoff text opened with "off:" on rows that were ON. Now the row leads
	// with ON/OFF in reverse-video (ANSI palette colours, so it survives any
	// theme and any terminal background), the word visibly changes when space is
	// pressed, and the description never opens with a bare "off".
	//
	// Composed from fixed-width fragments so the total is exactly w: the badge
	// carries its own colours and is NEVER truncated; only the plain-text
	// remainder passes through fitTo. Truncating a styled string would cut ANSI
	// codes mid-sequence — that is how over-wide-line bugs get written.
	const selW, badgeW = 2, 5
	onBadge := base.Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0")).Bold(true).Render(" ON  ")
	offBadge := base.Background(t.TextMuted()).Foreground(t.Background()).Render(" OFF ")
	toggleRow := func(selected, on bool, rest string, restStyle lipgloss.Style) string {
		mark, markStyle := "  ", base
		if selected {
			mark, markStyle = "> ", base.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		}
		badge := offBadge
		if on {
			badge = onBadge
		}
		restW := w - selW - badgeW
		return lipgloss.JoinHorizontal(lipgloss.Top,
			markStyle.Render(mark),
			badge,
			restStyle.Width(restW).MaxWidth(restW).Render(fitTo(rest, restW)))
	}
	// restStyle mirrors rowStyle minus the fixed width, for the composed rows.
	restStyle := func(selected, muted bool) lipgloss.Style {
		switch {
		case selected:
			return base.Background(t.Primary()).Foreground(t.Background()).Bold(true)
		case muted:
			return base.Foreground(t.TextMuted())
		}
		return base
	}

	// --- Section 1: the two Gorilla control dials (arrow-key adjustable) ---
	dialHeader := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render(fitLine("🦍 GORILLA CONTROLS — tune for your connection / free tier  (up/down pick a line | left/right change it):"))
	// Dial rows carry the same "> " selection pointer as the toggle rows, so
	// "where am I" reads the same way everywhere in the dialog.
	dialRow := func(selected bool, label, desc string) string {
		mark := "  "
		if selected {
			mark = "> "
		}
		return rowStyle(selected, false).Render(fitLine(fmt.Sprintf("%s%-32s ‹ <-/-> ›  %s", mark, label, desc)))
	}

	var rows []string
	rows = append(rows, dialRow(m.selectedIdx == rowPace, "AI SERVER requests — pace-setter", paceDesc()))
	rows = append(rows, dialRow(m.selectedIdx == rowLeash, "GORILLA AGENTS/SUBAGENTS — leash", leashDesc()))

	// --- Section 2: switch features on/off ---
	featHeader := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render(fitLine("Turn features ON/OFF  (> marks where you are | space flips it):"))
	for i, c := range sortedLoadout() {
		on := config.LoadoutEnabled(c.ID)
		mark := ""
		if c.Critical {
			mark = " ⚠"
		}
		// GORILLA OVERRIDE: real measured cost via ComponentTokens.
		//
		// GORILLA FIX (2026-08-17): rows whose real cost is paid when USED carry
		// a +RUN marker. Without it the screen said "EXPENSIVE ~163" next to
		// "~1,007" and looked like nonsense: both numbers were correct per-turn
		// figures, but the expensive thing about these two rows is the run.
		runMark := ""
		if config.RunCostRow(c.ID) {
			runMark = "+RUN"
		}
		rest := fmt.Sprintf("%-30s ~%-7s%-5s %s%s", c.Name, commaInt(config.ComponentTokens(c)), runMark, tradeoffText(on, c.Tradeoff), mark)
		selected := m.selectedIdx == i+numDials
		// GORILLA OVERRIDE: the two money-burners render bright red — several
		// full LLM sessions per use — and the user must never lose sight of
		// them in the list, on any theme. #FF0000 bold survives white, grey,
		// green and black backgrounds alike.
		style := restStyle(selected, !on)
		if (c.ID == "tool.research" || c.ID == config.DossierComponentID) && !selected {
			style = base.Foreground(lipgloss.Color("#FF0000")).Bold(true)
		}
		rows = append(rows, toggleRow(selected, on, rest, style))
	}

	// --- Section 3: show me the working ---
	// GORILLA OVERRIDE: every row states its own cost. Only one of these makes the
	// model generate more; the rest are display-only and free, and saying so
	// matters — a user told "extras cost money" would reasonably switch all of them
	// off and lose the forensic record for no saving at all.
	extrasHeader := base.Foreground(t.Primary()).Bold(true).Width(w).
		Render(fitLine("Show me the working  (space):"))
	var extraRows []string
	for i, e := range config.Extras {
		on := config.ExtraEnabled(e.ID)
		cost := "free"
		if e.Cost == config.CostGeneration {
			cost = "COSTS EXTRA"
		}
		rest := fmt.Sprintf("%-34s %-12s  %s", e.Name, cost, e.What)
		selected := m.selectedIdx == extrasFirstRow()+i
		extraRows = append(extraRows, toggleRow(selected, on, rest, restStyle(selected, !on)))
	}
	extrasNote := base.Foreground(t.TextMuted()).Width(w).
		Render(fitLine("  \"free\" = already generated and paid for; hiding it saves nothing. \"COSTS EXTRA\" = the model writes more."))

	help := base.Foreground(t.TextMuted()).Width(w).
		Render(fitLine("up/down pick | left/right dial | space flips ON/OFF | L all LSPs | l low-bw | r reset | esc close   ⚠ = disabling cripples the agent"))

	// Window the feature rows rather than rendering all of them. featureRows is
	// decided by measurement in View(), not by a guess at how much chrome the rest
	// of the dialog takes.
	feature := rows[numDials:]
	m.clampFeatureTop(featureRows, len(feature))
	end := min(m.featureTop+featureRows, len(feature))
	shown := feature[m.featureTop:end]

	// The number column means ONE thing — tokens per message — and two rows have
	// a second, larger cost that no per-turn figure can carry. Say so on screen
	// rather than leaving the reader to reconcile it.
	runNote := base.Foreground(t.TextMuted()).Width(w).
		Render(fitLine("  +RUN = cheap to carry, costly to USE: one run is 4-10 full AI sessions. The number left of it is per message only."))

	// Say so when rows are off-screen, or a hidden switch looks like a missing one.
	scrollNote := ""
	if len(shown) < len(feature) {
		scrollNote = base.Foreground(t.TextMuted()).Width(w).
			Render(fitLine(fmt.Sprintf("  showing %d-%d of %d — up/down to reach the rest",
				m.featureTop+1, end, len(feature))))
	}

	parts := []string{header}
	if !compact {
		// The subtitle and the context-size line explain; they are not state you
		// act on, so they are the first to go on a short terminal.
		parts = append(parts, sub, fixed, accuracy, cache)
		// GORILLA OVERRIDE (2026-09-01): the advice goes AFTER the estimate
		// caveat and before the blank line, so it reads as the conclusion of the
		// header rather than as another statistic. It is already conditional on
		// the loadout actually crowding the context, so on a large-context model
		// this block is empty and the screen is unchanged.
		parts = append(parts, advice...)
		parts = append(parts, "")
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
		parts = append(parts, runNote, "")
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
// active model's input rate, e.g. " — ~ $0.0033/turn (Grok 4.5 @ $3.00/1M in)".
// Empty string if we can't price it and there is nothing useful to add.
func loadoutCostSuffix() string {
	dollars, per1MIn, name, priced := config.LoadoutCost()

	// GORILLA OVERRIDE (2026-09-01): say what it costs in the unit the reader
	// actually pays in.
	//
	// This screen priced everything in dollars, which is right for a cloud model
	// and useless for a model running on the reader's own machine: there the
	// line said "~ $0.00/turn (free / flat-rate tier)" and gave them no reason
	// to change anything. The loadout is not free there — it is billed in
	// CONTEXT and in TIME. Measured on a laptop running Qwen3 Coder 30B: 11,142
	// tokens of loadout against a 20,224 window is 55% of the conversation gone
	// before the first word, and six to eight minutes of prompt processing on
	// every single turn.
	//
	// So when the model costs nothing in money, the suffix reports the share of
	// the context window instead. Both are the same sentence — "here is what
	// this screen is spending on your behalf" — in the currency that applies.
	if per1MIn <= 0 || !priced {
		if _, window, ctxName, share := config.LoadoutContextShare(); window > 0 {
			if ctxName != "" {
				name = ctxName
			}
			return fmt.Sprintf(" — %.0f%% of %s's %s-token context, before you type anything",
				share*100, name, commaInt(window))
		}
	}

	if !priced {
		if name != "" {
			return fmt.Sprintf(" — unpriced (%s: no price table entry)", name)
		}
		return ""
	}
	if per1MIn <= 0 {
		return fmt.Sprintf(" — ~ $0.00/turn (%s: free / flat-rate tier)", name)
	}
	return fmt.Sprintf(" — ~ %s/turn (%s @ $%.2f/1M in)", formatUSD(dollars), name, per1MIn)
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

// tradeoffText introduces the consequence line so it can never be misread as
// the row's state.
//
// GORILLA OVERRIDE (2026-08-17): it used to open with "off:" on rows that were
// ON (meaning "if you turn this off...") and "OFF —" on rows that were off, so
// every row on the screen led with the word "off" whatever its state. A real
// v0.1.87 user reported exactly that: "regardless the description still shows
// off". The state now lives ONLY in the ON/OFF badge; this text is purely the
// consequence, introduced unambiguously.
func tradeoffText(on bool, tradeoff string) string {
	if on {
		return "turn off and: " + tradeoff
	}
	return "while off: " + tradeoff
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
