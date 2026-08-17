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
		out = append(out, fmt.Sprintf("Cost: UNPRICED — no price table entry for %s. Assume it is not free.", name))
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
	out = append(out, fmt.Sprintf("Assumptions on screen, arguable: %d steps/helper, ~%d tokens out/step, ~%.0fs/step. Dossiers add a gap round on top.",
		config.ResearchStepsPerHelper, config.ResearchOutputPerStep, config.ResearchSecondsPerStep))
	return out
}

func helperModel() models.Model {
	hm, _ := config.ResearchHelperModelInfo()
	return hm
}

func (m OsintDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	w := min(110, max(80, m.width-8))

	red := base.Foreground(lipgloss.Color("#FF0000")).Bold(true).Width(w)
	head := base.Foreground(t.Primary()).Bold(true).Width(w)
	body := base.Width(w)
	mute := base.Foreground(t.TextMuted()).Width(w)

	var b []string
	b = append(b,
		red.Render("⚠ GORILLA OSINT — THE SERIOUS ONE"),
		body.Render(""),
		body.Render("Well done, bitch — you found the professional tool. Read this once, because it is not bluffing:"),
		body.Render(""),
		body.Render(fmt.Sprintf("It will spin up %d helper agents. Every one is a FULL model session working a lane of your", m.agents)),
		body.Render("question against hundreds of real sources, then a gap round hunts what they missed. This is"),
		body.Render("the most expensive thing this program can do, and it does it on purpose."),
		body.Render(""),
	)
	for _, line := range m.moneyLines() {
		b = append(b, red.Render(line))
	}
	b = append(b,
		body.Render(""),
		body.Render("It is your wallet and it is your funeral. If you are in a crunch and need ten agents in full"),
		body.Render("parallel to get an answer you can stand behind — that is exactly what this exists for. Your call."),
		body.Render(""),
		mute.Render(fmt.Sprintf("The finished dossier is saved OUTSIDE your working folder (%s),", config.DossierDir())),
		mute.Render("so a private question can never end up in a git repository."),
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
