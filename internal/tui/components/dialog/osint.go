// GORILLA OVERRIDE: this file did not exist upstream. It is the /osint gate —
// the warning screen that stands between the user and the professional dossier.
//
// The voice is deliberate and owner-specified: the tool warns like an adult and
// then respects the decision. "The moment you click on it, maybe there should
// be a fair warning" — this is that warning, with the burn rate computed for
// THIS user's model at THIS moment, not a generic disclaimer. If they are in a
// crunch and need ten agents in full parallel, that is what the tool is for;
// their wallet, their call.
package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// CloseOsintDialogMsg carries the choice back. Chosen false = cancelled, and a
// cancel must never start a run that costs money.
type CloseOsintDialogMsg struct {
	Chosen   bool
	Mode     string
	Agents   int
	Question string
}

// CloseOsintPageMsg closes the capability page.
type CloseOsintPageMsg struct{}

// dialogWidth returns a content width that ALWAYS fits: the terminal minus this
// dialog's chrome, capped at a preferred maximum so it does not sprawl on an
// ultrawide screen, with a small absolute floor.
//
// GORILLA OVERRIDE (2026-08-17): written after a probe found three dialogs
// flooring their width UP — /context asked for 106 columns on an 80-column
// terminal. A frame wider than the window is the documented cause of rows
// stranded in the scrollback (CLAUDE.md: "NO LINE IN THE FRAME MAY BE WIDER
// THAN THE TERMINAL"), because the inline renderer counts logical lines when it
// erases and a wrapped line occupies two physical rows. termWidth <= 0 means
// "size not known yet", where the preferred width is the only sane answer.
func dialogWidth(termWidth, preferred, chrome int) int {
	if termWidth <= 0 {
		return preferred
	}
	w := termWidth - chrome
	if w > preferred {
		w = preferred
	}
	if w < 20 {
		w = 20
	}
	return w
}

// osintModes reuses the run shapes the research engine actually implements.
// Parallel is FIRST and default: the whole point of paying for ten helpers is
// not waiting for them one at a time.
var osintModes = []struct {
	mode, label, what string
}{
	{"parallel", "PARALLEL", "all helpers at once — same cost as sequential, a fraction of the wait"},
	{"sequential", "sequential", "one at a time — slowest; only if your provider rate-limits hard"},
	{"supervised", "supervised", "parallel plus an auditor on every blind lane — the most rigorous and the most expensive"},
}

type OsintDialogCmp struct {
	question string
	selected int // index into osintModes
	agents   int
	width    int
	height   int
}

func NewOsintDialogCmp(question string) OsintDialogCmp {
	return OsintDialogCmp{
		question: question,
		selected: 0,                       // parallel
		agents:   agent.ResearchMinAgents, // start at the cheapest real run
	}
}

func (m *OsintDialogCmp) SetSize(w, h int) { m.width, m.height = w, h }

func (m OsintDialogCmp) Init() tea.Cmd { return nil }

func (m OsintDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.selected < len(osintModes)-1 {
				m.selected++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("left"))):
			if m.agents > agent.ResearchMinAgents {
				m.agents--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("right"))):
			if m.agents < agent.ResearchMaxAgents {
				m.agents++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			return m, util.CmdHandler(CloseOsintDialogMsg{
				Chosen:   true,
				Mode:     osintModes[m.selected].mode,
				Agents:   m.agents,
				Question: m.question,
			})
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			return m, util.CmdHandler(CloseOsintDialogMsg{Chosen: false})
		}
	}
	return m, nil
}

// sessions is the honest total for the chosen shape, from the same role table
// the run reads (see the research dialog's sessionCount for why).
func (m OsintDialogCmp) sessions() int {
	if osintModes[m.selected].mode == "supervised" {
		n, _ := agent.SupervisedSessions(m.agents)
		return n
	}
	return m.agents
}

// inFlight is how many helpers genuinely run at once for the chosen mode —
// sequential is one at a time, so quoting it parallel's peak rate would be a
// lie in the expensive direction, which is still a lie.
func (m OsintDialogCmp) inFlight() int {
	if osintModes[m.selected].mode == "sequential" {
		return 1
	}
	return min(m.sessions(), agent.ResearchMaxInFlight)
}

// moneyLines states the burn in the only unit that means anything, computed
// for the model helpers will ACTUALLY run on. On a free tier it prices the
// identical run at the same model's paid rate, so the size of what is being
// spent is visible even when the bill is zero.
func (m OsintDialogCmp) moneyLines() []string {
	sessions := m.sessions()
	perHelper, perMinute, per1M, name, priced := config.ResearchCost(m.inFlight())
	var out []string
	switch {
	case !priced:
		// The name is empty when no model is configured yet; a sentence with a
		// hole in it reads like a bug and undermines the warning it carries.
		which := name
		if strings.TrimSpace(which) == "" {
			which = "the model helpers will run on"
		}
		out = append(out, fmt.Sprintf("Cost: UNPRICED — no price-table entry for %s. Assume it is NOT free.", which))
	case per1M <= 0:
		out = append(out, fmt.Sprintf("Your tier (%s) bills flat or free — this run spends QUOTA: roughly %d ordinary questions' worth.",
			name, config.ResearchQuotaMultiple(sessions)))
		if pm, ph, via, ok := config.ResearchPaidEquivalent(helperModel(), m.inFlight()); ok && via != "" {
			out = append(out, fmt.Sprintf("The same run on the paid API (%s): ≈ %s/min at peak, ≈ %s per helper. That is the size of what you are spending.",
				via, formatUSD(pm), formatUSD(ph)))
		}
	default:
		out = append(out, fmt.Sprintf("PEAK BURN: ≈ %s per MINUTE while it runs (%s @ $%.2f/1M in). ≈ %s per helper, %d sessions total.",
			formatUSD(perMinute), name, per1M, formatUSD(perHelper), sessions))
	}
	return out
}

// assumptionsLine names the three figures the forecast rests on, so the number
// above can be argued with rather than trusted.
func assumptionsLine() string {
	return fmt.Sprintf("Assumptions on screen, arguable: %d steps/helper, ~%d tokens out/step, ~%.0fs/step. Dossiers add a gap round on top.",
		config.ResearchStepsPerHelper, config.ResearchOutputPerStep, config.ResearchSecondsPerStep)
}

// scaleLines states the size of the run in TOKENS, which is the only unit that
// stays honest on every tier. Money reads as zero on a free plan however much
// is burned; tokens do not.
//
// The rate is MEASURED, not modelled: a real eight-helper run on 2026-08-17
// consumed 248,122 input and 32,622 output tokens in the thirteen minutes it
// was working — 21,596 tokens a minute. Scaling is linear in the number of
// sessions, which is the same assumption the per-minute figure already makes
// and is printed on screen so it can be argued with.
func (m OsintDialogCmp) scaleLines() []string {
	measuredPerMinutePerSession := 21596.0 / 8.0
	sessions := m.sessions()
	perHour := measuredPerMinutePerSession * float64(sessions) * 60.0
	return []string{
		fmt.Sprintf("SIZE OF THIS RUN: about %s TOKENS PER HOUR while it works (%d sessions x ~%s tokens/min each,",
			commaInt(int(perHour)), sessions, commaInt(int(measuredPerMinutePerSession))),
		"measured from a real run, not modelled). A quarter of a million tokens in the first ten minutes is normal.",
		"If a per-minute figure above looks small to you, this is the number to look at instead.",
	}
}

func helperModel() models.Model {
	hm, _ := config.ResearchHelperModelInfo()
	return hm
}

// View renders at a height MEASURED to fit, shedding prose in priority order.
//
// GORILLA FIX (2026-08-17): this used to render every line unconditionally and
// asked for 37 rows in a 24-row terminal — a frame taller than the window
// scrolls the terminal and destroys the layout. What can never be dropped is
// the decision itself: the warning headline, what it costs, the helper and mode
// controls, and the keys. The explanatory prose goes first, the privacy note
// and the opening line after it. Same approach as /help and /context.
func (m OsintDialogCmp) View() string {
	for lean := 0; lean < 4; lean++ {
		v := m.renderAt(lean)
		if m.height <= 0 || lipgloss.Height(v) <= m.height {
			return v
		}
	}
	return m.renderAt(3)
}

// renderAt draws the gate at a leanness level: 0 is everything, 3 is the
// decision and nothing else.
func (m OsintDialogCmp) renderAt(lean int) string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	// GORILLA (2026-08-17): this screen takes the WHOLE terminal. It was a
	// 110-column box and the owner's verdict was that the warning "gets
	// truncated" and reads "deceivingly small and reassuring". A screen whose
	// job is to stop someone spending money they do not have cannot be the
	// screen that runs out of room. dialogWidth still guarantees it never
	// exceeds the window — see loadoutDialogCmp.width().
	w := dialogWidth(m.width, 200, 8)

	red := base.Foreground(lipgloss.Color("#FF0000")).Bold(true).Width(w)
	head := base.Foreground(t.Primary()).Bold(true).Width(w)
	body := base.Width(w)
	mute := base.Foreground(t.TextMuted()).Width(w)

	var b []string
	b = append(b, red.Render("⚠ GORILLA OSINT — THE SERIOUS ONE"))
	if lean < 3 {
		// The opening line is the owner-specified voice. It survives everything
		// except the leanest form, where only the decision remains.
		b = append(b,
			body.Render(""),
			body.Render("Well done, bitch — you found the professional tool. Read this once, because it is not bluffing:"),
		)
	}
	if lean < 2 {
		b = append(b,
			body.Render(""),
			body.Render(fmt.Sprintf("It will spin up %d helper agents. Every one is a FULL model session working a lane of your", m.agents)),
			body.Render("question against hundreds of real sources, then a gap round hunts what they missed. This is"),
			body.Render("the most expensive thing this program can do, and it does it on purpose."),
		)
	}
	b = append(b, body.Render(""))
	// The money NEVER goes. It is the entire reason this screen exists.
	for _, line := range m.moneyLines() {
		b = append(b, red.Render(line))
	}
	// GORILLA (2026-08-17): the SCALE, in tokens, measured from a real run.
	//
	// The per-minute figure above is correct and it is not enough: "$0.03/min"
	// and "12 questions' worth" read as small, and the owner — who can do this
	// arithmetic — only caught the true size by totalling the database by hand
	// afterwards. Most people cannot and will not. A measured run of EIGHT
	// helpers burned 280,744 tokens in thirteen minutes: about 1.3 million
	// tokens an hour, and a ten-helper supervised run projects to roughly 2.9
	// million an hour. Those are the numbers that make the decision real.
	if lean < 3 {
		for _, line := range m.scaleLines() {
			b = append(b, red.Render(line))
		}
	}
	if lean < 3 {
		// The styles carry .Width(w), which WRAPS rather than overflowing — so a
		// long line costs rows, never columns, and View() measures those rows.
		b = append(b, red.Render(assumptionsLine()))
	}
	if lean < 1 {
		b = append(b,
			body.Render(""),
			body.Render("It is your wallet and it is your funeral. If you are in a crunch and need ten agents in full"),
			body.Render("parallel to get an answer you can stand behind — that is exactly what this exists for. Your call."),
		)
	}
	if lean < 2 {
		b = append(b,
			body.Render(""),
			mute.Render(fmt.Sprintf("The finished dossier is saved OUTSIDE your working folder (%s),", config.DossierDir())),
			mute.Render("so a private question can never end up in a git repository."),
		)
	}
	b = append(b,
		body.Render(""),
		head.Render(fmt.Sprintf("Helpers: ‹ %d ›  (←/→, %d–%d)", m.agents, agent.ResearchMinAgents, agent.ResearchMaxAgents)),
	)
	for i, o := range osintModes {
		line := fmt.Sprintf("  %-11s %s", o.label, o.what)
		if i == m.selected {
			b = append(b, base.Width(w).Background(t.Primary()).Foreground(t.Background()).Bold(true).Render("> "+line[2:]))
		} else {
			b = append(b, body.Render(line))
		}
	}
	b = append(b,
		body.Render(""),
		mute.Render("enter: burn it   ↑↓: mode   ←→: helpers   esc: walk away (costs nothing)"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left, b...)
	return base.Padding(1, 2).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#FF0000")).
		BorderBackground(styles.PanelBackground()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func (m OsintDialogCmp) Bindings() []key.Binding { return nil }
