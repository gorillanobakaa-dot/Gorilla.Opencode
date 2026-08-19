package dialog

// GORILLA OVERRIDE: the "your background agents just moved" screen.
//
// Switching the chat model silently re-points four other agents — the ones that
// name your sessions, compact your context, run sub-agents and do research. For
// a year that happened with no notice at all, then with a one-line status note
// that said how many moved but not where, and finally not at all for
// same-provider switches, which is how a user ended up on Claude Opus 4.6 while
// his research quietly ran on Gemini Flash.
//
// The rule is now simple — background agents always follow the coder — and the
// entire cost of that simplicity is paid here: the user is told exactly what
// moved, what it means for money and for quota, and can put it back with one
// key.
//
// Written for someone who cannot read the source, may be reading in a second
// language, and is paying for every token out of a household budget. Colour
// carries meaning and is layered ON TOP of words, never instead of them, so a
// colour-blind reader loses nothing.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// CloseModelFollowDialogMsg closes the screen; Reverted says the user undid it.
type CloseModelFollowDialogMsg struct {
	Reverted bool
}

// ShowModelFollowDialogMsg opens it with the moves that just happened.
type ShowModelFollowDialogMsg struct {
	Moves []config.AgentModelMove
}

type ModelFollowDialogCmp struct {
	width, height int
	compact       bool
	moves         []config.AgentModelMove
}

func NewModelFollowDialogCmp(moves []config.AgentModelMove) ModelFollowDialogCmp {
	return ModelFollowDialogCmp{moves: moves}
}

func (m ModelFollowDialogCmp) Init() tea.Cmd { return nil }

func (m ModelFollowDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
			if err := config.RevertAgentModels(m.moves); err != nil {
				return m, util.ReportError(err)
			}
			return m, util.CmdHandler(CloseModelFollowDialogMsg{Reverted: true})
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", "esc", "q"))):
			return m, util.CmdHandler(CloseModelFollowDialogMsg{})
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.compact = m.height > 0 && m.height < 34
	}
	return m, nil
}

// whatEachAgentDoes is the plain-language job of each background agent. The
// agent NAMES ("summarizer", "title") are ours, not the user's, and mean
// nothing to someone who has not read the code.
var whatEachAgentDoes = map[config.AgentName]string{
	config.AgentTitle:      "names your chats in the sidebar",
	config.AgentSummarizer: "squashes the conversation when it gets too long",
	config.AgentTask:       "the small helpers the AI sends off to look things up",
	config.AgentResearch:   "the /research helpers — the deep digging",
}

// cheapWork is the set whose job does not benefit from an expensive model.
// Naming a chat on a reasoning model is money burnt for nothing.
var cheapWork = map[config.AgentName]bool{
	config.AgentTitle:      true,
	config.AgentSummarizer: true,
}

func (m ModelFollowDialogCmp) lines() []costLine {
	var out []costLine
	add := func(k costKind, f string, a ...any) {
		text := fmt.Sprintf(f, a...)
		if m.compact && text == "" {
			return
		}
		out = append(out, costLine{text, k})
	}
	if len(m.moves) == 0 {
		add(kindMuted, "Nothing moved.")
		return out
	}

	to := m.moves[0].To
	toModel, toKnown := models.SupportedModels[to]
	toName := string(to)
	if toKnown {
		toName = config.ModelLabel(toModel)
	}

	add(kindDanger, "YOUR BACKGROUND HELPERS MOVED TOO.")
	add(kindMuted, "You changed the model you chat with. These other jobs run on a")
	add(kindMuted, "model as well, and they have followed you to «%s».", toName)
	add(kindMuted, "")

	add(kindHeader, "WHAT MOVED")
	for _, mv := range m.moves {
		fromName := string(mv.From)
		if fm, ok := models.SupportedModels[mv.From]; ok {
			fromName = config.ModelLabel(fm)
		}
		job := whatEachAgentDoes[mv.Agent]
		if job == "" {
			job = string(mv.Agent)
		}
		kind := kindMeasured
		if cheapWork[mv.Agent] {
			kind = kindAssumed // this one is probably waste; say so below
		}
		add(kind, "%s", job)
		add(kindMuted, "   was %s  ->  now %s", fromName, toName)
	}
	add(kindMuted, "")

	add(kindHeader, "WHAT IT COSTS YOU")
	fromModel, fromKnown := models.SupportedModels[m.moves[0].From]
	switch {
	case !toKnown || !fromKnown:
		add(kindMuted, "No published price for one of these models, so there is no")
		add(kindMuted, "before-and-after figure to show you.")
	case toModel.CostPer1MIn == 0 && fromModel.CostPer1MIn == 0:
		add(kindMuted, "Neither model bills per token — both are on a flat or free tier.")
		add(kindQuota, "You are not spending more MONEY. You are spending your «QUOTA» faster,")
		add(kindQuota, "because the new model is the one doing all of it now.")
	case fromModel.CostPer1MIn == 0:
		add(kindMoney, "These jobs used to cost nothing per token. Now they cost %s per 1M in.",
			amount(toModel.CostPer1MIn))
		add(kindDanger, "That is a NEW bill that did not exist five seconds ago.")
	case toModel.CostPer1MIn == 0:
		add(kindMeasured, "These jobs got CHEAPER: %s per 1M in -> no per-token rate.",
			amount(fromModel.CostPer1MIn))
	default:
		ratio := toModel.CostPer1MIn / fromModel.CostPer1MIn
		add(kindMoney, "Per million words in: %s  ->  %s",
			amount(fromModel.CostPer1MIn), amount(toModel.CostPer1MIn))
		switch {
		case ratio >= 1.1:
			add(kindDanger, "Background jobs now cost about «%.0f times» what they did.", ratio)
		case ratio <= 0.9:
			add(kindMeasured, "Background jobs now cost about %.0f%% of what they did.", ratio*100)
		default:
			add(kindMuted, "Roughly the same price as before.")
		}
	}

	// The separate-pool warning: a real, specific loss, not a generality.
	if fromKnown && toKnown {
		if bg, ok := config.BackgroundModelForProvider(fromModel.Provider); ok &&
			m.moves[0].From == bg && toModel.Provider == fromModel.Provider {
			add(kindQuota, "")
			add(kindQuota, "You have also LEFT A SEPARATE FREE ALLOWANCE.")
			add(kindMuted, "   %s is billed from its own pool, apart from the one", config.ModelLabel(fromModel))
			add(kindMuted, "   %s draws on. Running background jobs there used", toName)
			add(kindMuted, "   two allowances instead of one. Now everything shares one.")
		}
	}

	// The waste that always applies when a naming job lands on a big model.
	var wasteful []string
	for _, mv := range m.moves {
		if cheapWork[mv.Agent] {
			wasteful = append(wasteful, whatEachAgentDoes[mv.Agent])
		}
	}
	if len(wasteful) > 0 && toKnown && toModel.CostPer1MIn > 0 {
		add(kindAssumed, "")
		add(kindAssumed, "PROBABLY NOT WORTH IT for: %s.", strings.Join(wasteful, "; "))
		add(kindMuted, "   Those jobs do not get better on a stronger model. They are")
		add(kindMuted, "   the amber rows above.")
	}

	add(kindMuted, "")
	add(kindHeader, "WHAT YOU GET")
	add(kindMeasured, "Your «/research» now runs on the model you actually picked.")
	add(kindMuted, "   That is the reason this happens automatically: research is the")
	add(kindMuted, "   deep work, and it was silently being done by a weaker model.")

	add(kindMuted, "")
	add(kindQuota, "You can put ALL of it back exactly as it was. Nothing is lost.")
	return out
}

func (m ModelFollowDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()

	maxWidth := 76
	if m.width > 0 && m.width-8 < maxWidth {
		maxWidth = m.width - 8
	}
	if maxWidth < 40 {
		maxWidth = 40
	}

	var rows []string
	for _, l := range m.lines() {
		st := base.Width(maxWidth).Padding(0, 1)
		text := l.text
		switch l.kind {
		case kindHeader:
			st = st.Foreground(t.TextMuted())
			// Sized, not typed — see the note in research.go's renderCost and
			// internal/tui/styles/ascii.go.
			text = styles.RuleLabel(text, maxInt(8, maxWidth-2))
		case kindMoney, kindDanger:
			st = st.Foreground(t.Error()).Bold(true)
		case kindQuota, kindAssumed:
			st = st.Foreground(t.Warning())
		case kindMeasured:
			st = st.Foreground(t.Success())
		case kindPublished:
			st = st.Foreground(t.Info())
		default:
			st = st.Foreground(t.TextMuted())
		}
		rows = append(rows, st.Render(emphasise(text, t)))
	}

	hints := base.Width(maxWidth).Padding(1, 1, 0, 1).Foreground(t.TextMuted()).
		Render("enter: keep it   r: put it back   esc: keep it")
	rows = append(rows, hints)

	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(styles.PanelBackground()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(rows[0]) + 4).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m ModelFollowDialogCmp) BindingKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "keep it")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "put it back")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "keep it")),
	}
}
