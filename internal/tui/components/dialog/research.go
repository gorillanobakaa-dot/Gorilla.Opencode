package dialog

// GORILLA OVERRIDE: the /research mode chooser.
//
// The research tool can run three ways and they differ by a LOT of money. The
// model picks a mode from a schema description the user never sees, which is
// exactly the wrong place for a decision that multiplies someone's bill. So the
// user picks, and is told the multiplier in the same breath.
//
// The warning is blunt on purpose. "This may use additional tokens" is the
// sentence of someone who does not pay the bill. This audience does: single
// digit KB/s, no credit card, quota that runs out. If a choice costs four times
// as much, it says four times, before the choice is made — not in a summary
// afterwards.

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// ResearchMode is what the user chose.
type ResearchMode struct {
	Mode   string
	Agents int
}

type researchOption struct {
	mode string
	name string
	// what it does, plainly
	what string
	// the cost sentence, stated as a multiple of the cheapest option
	cost string
	// one-line summary, shown for the modes you are NOT on
	short string
}

var researchOptions = []researchOption{
	{
		mode:  "sequential",
		name:  "Sequential",
		short: "one at a time, slowest, same price",
		what:  "One helper at a time. Slowest by far — you wait for each in turn.",
		// GORILLA FIX 2026-08-14: this said "Cheapest per answer", beside a run
		// total identical to parallel's. It is not cheaper. It is the same money
		// spread thinner, which is why its per-minute rate looks lower.
		cost: "NOT cheaper — the same total, spread over a longer wait. Same sessions, lower rate.",
	},
	{
		mode:  "parallel",
		name:  "Parallel",
		short: "up to 4 at a time, same price, much faster",
		// GORILLA FIX: was "All helpers at once (4 in flight)" — self-contradictory
		// above 4, and the selector goes to 10. Ten lanes are three batches, so
		// "the time of the slowest one" was wrong too.
		what: "Up to 4 helpers at a time, in batches. Same work as sequential, far less waiting.",
		cost: "SAME token cost as sequential. You are buying time, not answers.",
	},
	{
		mode:  "supervised",
		name:  "Supervised",
		short: "parallel + an auditor on each blind lane, ~DOUBLE price",
		// GORILLA FIX: was "audits every lane" / "Every helper gets a checker" /
		// "every lane checked twice". Supervision covers the BLIND lanes only —
		// the verifier and completeness lanes are never audited. Above 4 helpers
		// those claims are simply false, and the unaudited one is the verifier:
		// the lane whose whole job is to catch the others being confidently wrong.
		what: "Parallel, then a second agent audits each blind lane and returns APPROVED / WEAK / REJECTED before you see it.",
		cost: "Nearly double: same rate, roughly twice as long. Worth it when being wrong is expensive.",
	},
}

type ResearchDialogCmp struct {
	width, height int
	// compact drops blank lines and section headers when the terminal is too
	// short to show the dialog AND its key hints.
	compact  bool
	selected int
	agents   int
	question string
	keys     researchDialogKeyMap
}

func NewResearchDialogCmp(question string) ResearchDialogCmp {
	return ResearchDialogCmp{
		selected: 1, // parallel: same price as sequential, less waiting
		agents:   4, // the mandatory four lanes; the cheapest real investigation
		question: question,
	}
}

type researchDialogKeyMap struct{}

func (k researchDialogKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "mode")),
		key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←/→", "helpers")),
		key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "helper model")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "go")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

func (k researchDialogKeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }

func (m ResearchDialogCmp) Init() tea.Cmd { return nil }

func (m ResearchDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q"))):
			return m, util.CmdHandler(CloseResearchDialogMsg{})
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			m.selected = (m.selected - 1 + len(researchOptions)) % len(researchOptions)
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j", "tab"))):
			m.selected = (m.selected + 1) % len(researchOptions)
		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "h"))):
			if m.agents > 4 {
				m.agents--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("right", "l"))):
			if m.agents < 10 {
				m.agents++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("m"))):
			// GORILLA FIX: the old screen told the user to "set a research agent
			// to change it" and gave them no way to do it — the only route was
			// hand-editing config.json. This is that route.
			//
			// It also CREATES the "research" agent entry, which most configs
			// lack; that absence is why helpers silently inherited the task
			// agent's cheap background model in the first place.
			if err := config.UseChatModelForResearch(); err != nil {
				return m, util.ReportError(err)
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			return m, util.CmdHandler(CloseResearchDialogMsg{
				Chosen:   true,
				Mode:     researchOptions[m.selected].mode,
				Agents:   m.agents,
				Question: m.question,
			})
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.compact = m.height > 0 && m.height < 40
	}
	return m, nil
}

// sessionCount is the honest total, including supervisors.
//
// GORILLA FIX 2026-08-14: this returned m.agents*2 for supervised, with a
// comment conceding it was "an upper bound rather than a promise" — a
// concession the user never sees, attached to a number printed as fact and
// multiplied into the money.
//
// It is only right at exactly 4 helpers. Supervision covers the BLIND lanes;
// the verifier and completeness lanes peek at the others and are never audited.
// Measured against the real selectRoles: 5→9 not 10, 6→11 not 12, 10→18 not 20.
// Asking agent.SupervisedSessions means the forecast reads the same role table
// the run does, so the two cannot drift.
func (m ResearchDialogCmp) sessionCount() int {
	if researchOptions[m.selected].mode == "supervised" {
		sessions, _ := agent.SupervisedSessions(m.agents)
		return sessions
	}
	return m.agents
}

// costSentence says what this costs in the only unit that means anything:
// MONEY. Falls back to quota when the tier is free.
//
// GORILLA OVERRIDE, twice over.
//
// v1 said "That is 10 model sessions. Not one answer — 10." The author asked
// what it was supposed to mean, which is the right response to it.
//
// v2 said "10 helpers + 10 checkers = 20 separate conversations with the AI."
// Technically true, and his verdict was that it "tells a non-technical user
// sweet fuck all". Also right. This ships to kids whose FAMILY lives on $50-60
// a month and for whom English is a second language. "Conversations" is our
// word. Money is theirs, and they will want it precise.
//
// So: the real rate, the real arithmetic, and his warning — which lands harder
// than anything corporate, and was his instruction, not my idea to overrule.
// costKind drives the colour. Colour carries MEANING here, consistently:
// red is money leaving, amber is a limit you can hit, green is measured,
// blue is published by the vendor.
//
// GORILLA OVERRIDE: every emphasised line used to be the same warning colour,
// so nothing stood out and the block read as a wall. Reported 2026-08-14 as an
// accessibility problem — for ADHD and dyslexic readers a uniform block is
// noise they get lost in.
//
// The CAPS labels stay. Colour is layered ON TOP of them, never instead: a
// colour-blind reader must lose nothing, so every distinction is carried by
// the words as well as the hue.
type costKind int

const (
	kindHeader    costKind = iota
	kindMoney              // red, bold — what leaves your account
	kindQuota              // amber — a limit you can hit
	kindMeasured           // green — measured on this machine
	kindPublished          // blue — the vendor's own number
	kindAssumed            // amber — invented, argue with it
	kindDanger             // red, bold — act on this
	kindMuted
)

type costLine struct {
	text string
	kind costKind
}

// GORILLA FIX 2026-08-14: THE MONEY ON THIS SCREEN DID NOT ADD UP.
//
// The per-minute figure printed at %.2f and the per-hour figure beside it at
// %.0f — whole dollars. Measured, the real rate was $0.006560/min = $0.3936/hr,
// and the dialog rendered it as:
//
//	$0.01 PER MINUTE.    PER HOUR: $0
//
// An hour shown as costing NOTHING, next to a per-minute price. And at the
// parallel rate, "$0.02 PER MINUTE. PER HOUR: $1" — 0.02 x 60 is 1.20, not 1.
// Reported as "your math is absolutely shit", which is the correct reading:
// a user cannot multiply the two numbers in front of them and arrive at the
// third. Every figure was computed correctly and then destroyed on the way to
// the screen.
//
// Two verbs, two separate faults:
//   - %.0f on the hour rounded 0.39 to "0" — a 100% error, and the one that
//     says an hour is free
//   - %.2f on a SUB-CENT per-minute rate cannot be multiplied by 60 at all:
//     0.00656 shown as "0.01" is already 52% high before you start
//
// So a rate needs enough decimals to survive being multiplied by 60, and an
// amount must never round down to $0.00 while the real value is above zero.
// This screen exists to be checked with a calculator by someone who cannot read
// the source. If the arithmetic on screen does not close, nothing else on it is
// worth anything.

// perMinuteAndHour formats a rate and its hourly equivalent FROM A SINGLE
// ROUNDING, so the two printed figures multiply by 60 exactly as they appear.
//
// Rounding them independently is what broke the first attempt at this fix, and
// the test caught it: $0.006560/min printed as "$0.0066" while the hour was
// computed from the unrounded value and printed "$0.39". Both were faithful to
// the true number and they still contradicted each other on screen, because the
// user multiplies what they can SEE. Round once, derive the rest.
func perMinuteAndHour(perMin float64) (perMinuteStr, perHourStr string) {
	if perMin <= 0 {
		return "$0.00", "$0.00"
	}
	shown := roundSignificant(perMin, 3)
	return rate(shown), amount(shown * 60)
}

// roundSignificant keeps sig significant figures. Fixed decimal places cannot
// do this job across the range in play here: the same %.4f that is right for
// $0.0262/min flattens a $0.000041/min free-tier equivalent to $0.0000.
func roundSignificant(v float64, sig int) float64 {
	if v == 0 {
		return 0
	}
	mag := math.Pow(10, float64(sig)-math.Ceil(math.Log10(math.Abs(v))))
	return math.Round(v*mag) / mag
}

// rate formats a per-minute price, showing every digit it actually carries so
// the figure can be multiplied by 60, with a two-decimal floor so it still
// reads as money.
func rate(v float64) string {
	if v <= 0 {
		return "$0.00"
	}
	return fmt.Sprintf("$%.*f", max(2, decimalsOf(v)), v)
}

// decimalsOf reports how many decimal places v needs to print exactly, using
// the shortest representation that round-trips.
func decimalsOf(v float64) int {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return 0
	}
	return len(s) - i - 1
}

// amount formats a total.
//
// GORILLA FIX 2026-08-14, second pass: two decimals is not enough resolution
// below a dollar. A supervised run really is about twice a parallel one —
// $0.0161 against $0.0323 — but printed at %.2f those became "$0.02" and
// "$0.03", which reads as 1.5x and makes the screen contradict its own
// "nearly double" label AGAIN, one layer down from the fault just fixed.
//
// Same rule as rate(): significant figures below a dollar, cents above, so
// ratios between two printed totals survive the printing. "$0.00" against a
// real cost reads as free and must never appear.
func amount(v float64) string {
	switch {
	case v <= 0:
		return "$0.00"
	case v < 1:
		return rate(roundSignificant(v, 3))
	default:
		return fmt.Sprintf("$%.2f", v)
	}
}

// runMoney renders the run total and its rate FROM ONE ANCHOR, so every figure
// on screen multiplies out.
//
// GORILLA FIX 2026-08-14: the per-session price and the run total used to be
// rounded independently — the exact fault this file already claims to have
// fixed for the per-minute/per-hour pair, left in place one line below it. At
// $0.0164 a session and 20 sessions the screen read "20 x ~$0.02 each" and
// "THIS RUN: about $0.33", and 20 x $0.02 is $0.40.
//
// The anchor is the RUN TOTAL, because that is the number the user is actually
// deciding about. The rate is derived from it, not the other way round, so
// rate x duration = total by construction rather than by luck.
func runMoney(perSession float64, sessions int, minutes float64) (total, perMinute, perHour string) {
	t := roundSignificant(perSession*float64(sessions), 3)
	if minutes <= 0 {
		return amount(t), "$0.00", "$0.00"
	}
	pm, ph := perMinuteAndHour(t / minutes)
	return amount(t), pm, ph
}

// parseBack reads a printed figure back, so a derived line is computed from the
// number the user can SEE rather than from a float they cannot.
func parseBack(printed string) float64 {
	v, err := strconv.ParseFloat(strings.TrimPrefix(printed, "$"), 64)
	if err != nil {
		return 0
	}
	return v
}

// humanDuration prints a run length the way someone waiting for it would say it.
func humanDuration(seconds float64) string {
	s := int(seconds + 0.5)
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	if s%60 == 0 {
		return fmt.Sprintf("%dm", s/60)
	}
	return fmt.Sprintf("%dm %ds", s/60, s%60)
}

func (m ResearchDialogCmp) costLines() []costLine {
	mode := researchOptions[m.selected].mode
	// The forecast reads the SAME role table and the SAME concurrency cap the
	// scheduler uses, so the screen and the run cannot disagree.
	n, audited, _, seconds := agent.RunShape(mode, m.agents)
	minutes := seconds / 60

	inFlight := 4
	if mode == "sequential" {
		inFlight = 1
	}
	perHelper, _, per1MIn, modelName, priced := config.ResearchCost(inFlight)

	var lines []costLine
	add := func(k costKind, f string, a ...any) {
		text := fmt.Sprintf(f, a...)
		// On a short screen every blank line is a line of content lost off the
		// bottom, and the footer goes with it.
		if m.compact && text == "" {
			return
		}
		lines = append(lines, costLine{text, k})
	}

	// GORILLA FIX 2026-08-14: the helper model is now stated FIRST, always, as
	// the caption for every figure below it — not gated on a provider mismatch
	// and not a footnote after the money.
	//
	// The user switched to Claude Opus 4.6 (Thinking) and this screen carried on
	// pricing Gemini 2.0 Flash in silence, because both are Antigravity and the
	// old warning only fired across providers. His verdict — "the research
	// function is NOT MODEL AWARE" — was right, and the provider gate was the
	// reason. Nobody picks Opus to save money; they pick it because the work is
	// hard, and research is the hard part.
	if helper, chat, ok := config.ResearchModelChoice(); ok {
		hName, cName := config.ModelLabel(helper), config.ModelLabel(chat)
		if helper.ID != chat.ID {
			add(kindDanger, "HELPERS RUN ON: %s", hName)
			add(kindMuted, "   NOT %s, which you are chatting with.", cName)
			add(kindMuted, "   Every figure below is %s's. Your research will be as", hName)
			add(kindMuted, "   good as that model — not as good as the one you picked.")
			if helper.Provider != chat.Provider {
				add(kindQuota, "   It is also a DIFFERENT PROVIDER: separate account, separate quota.")
			}
			add(kindQuota, "   Press «m» to run helpers on %s instead.", cName)
			add(kindMuted, "")
		} else {
			add(kindMeasured, "HELPERS RUN ON: %s — the model you are chatting with.", hName)
			add(kindMuted, "")
		}
	}

	add(kindHeader, "── WHAT THIS COSTS ─────────────────────────────────────")
	switch {
	case !priced:
		add(kindDanger, "CANNOT PRICE %s. No rate on record.", modelName)
		add(kindMuted, "It could be anything. Check what that model charges FIRST.")
	case per1MIn > 0:
		total, pmStr, phStr := runMoney(perHelper, n, minutes)
		add(kindMoney, "%s PER MINUTE.    PER HOUR: %s", pmStr, phStr)
		add(kindMoney, "THIS RUN: about %s  —  %s of running.", total, humanDuration(seconds))
		add(kindMuted, "   %s x %s per minute = %s", humanDuration(seconds), pmStr, total)
		add(kindMuted, "   %d sessions, %s each, on %s.", n, amount(parseBack(total)/float64(n)), modelName)
	default:
		add(kindMuted, "$0.00 metered — %s bills no per-token rate.", modelName)
		add(kindDanger, "THAT IS NOT FREE.")
		add(kindMuted, "On a paid subscription you have «ALREADY» paid; this decides")
		add(kindMuted, "how fast you spend what you bought.")
		// The model THEY are on, not a random one from the catalogue.
		if hm, ok := config.ResearchHelperModelInfo(); ok {
			if _, ph, via, ok2 := config.ResearchPaidEquivalent(hm, inFlight); ok2 && via != "" {
				add(kindMuted, "")
				add(kindMuted, "IF YOU WERE PAYING FOR THIS MODEL INSTEAD OF A FREE TIER:")
				total, pmStr, phStr := runMoney(ph, n, minutes)
				add(kindMoney, "   %s PER MINUTE.    PER HOUR: %s", pmStr, phStr)
				add(kindMoney, "   THIS RUN: about %s  —  %s of running.", total, humanDuration(seconds))
				add(kindMuted, "   %s x %s per minute = %s", humanDuration(seconds), pmStr, total)
				add(kindMuted, "   priced at %s, the closest paid listing for this model.", via)
			} else {
				add(kindMuted, "")
				add(kindMuted, "No paid listing on record for this model, so there is no")
				add(kindMuted, "money figure to show you. It is not free — it is unpriced.")
			}
		}
	}

	// GORILLA FIX 2026-08-14: this said "%d helpers x %d steps each" using the
	// SESSION count, so at 4 helpers supervised it read "8 helpers" three lines
	// under a selector reading "Helpers: 4", and at 10 it read "20 helpers" on a
	// screen stating the maximum is 10. One word carrying two different meanings
	// is most of why the arithmetic looked broken.
	add(kindMuted, "")
	add(kindHeader, "── QUOTA ───────────────────────────────────────────────")
	add(kindQuota, "WORTH ABOUT %d ORDINARY QUESTIONS in tokens.", config.ResearchQuotaMultiple(n))
	if mode == "supervised" {
		add(kindMuted, "   %d helpers + %d auditors = %d sessions, %d steps each.",
			m.agents, audited, n, config.ResearchStepsPerHelper)
		// The DOUBLE claim, made checkable. It is double TIME at one rate, not
		// a faster burn — and above 4 helpers it is not every lane either.
		// Only say so when a lane is actually being skipped: "4 of 4 audited"
		// followed by an explanation of an exclusion that is not happening is
		// its own small piece of nonsense.
		if audited < m.agents {
			add(kindQuota, "   Only %d of %d lanes are audited. The verifier and completeness", audited, m.agents)
			add(kindMuted, "   lanes read the others' work and are never checked themselves.")
		}
	} else {
		add(kindMuted, "   %d helpers x %d steps each.", n, config.ResearchStepsPerHelper)
	}
	add(kindQuota, "Run out and «NOTHING WORKS» until it resets. Paid or free.")

	if helper, chat, otherProvider := config.ResearchHelperModel(); otherProvider {
		add(kindMuted, "")
		add(kindDanger, "WARNING: HELPERS RUN ON A DIFFERENT PROVIDER.")
		add(kindMuted, "   %s, not %s that you are chatting with.", helper, chat)
		add(kindMuted, "   Different account, different bill. Set a \"research\" agent to change it.")
	}

	add(kindMuted, "")
	if !m.compact {
		add(kindHeader, "── HOW THIS IS WORKED OUT ──────────────────────────────")
	}
	add(kindMeasured, "MEASURED: %s tokens of context per step (this machine).", humanCount(config.ResearchBasisTokens()))
	add(kindPublished, "PUBLISHED: the model's own per-1M price.")
	add(kindAssumed, "ASSUMED («not measured»): %d steps · %d out · %.0fs per step — the per-minute figure rests on that %.0fs.",
		config.ResearchStepsPerHelper, config.ResearchOutputPerStep, config.ResearchSecondsPerStep, config.ResearchSecondsPerStep)
	return lines
}

// renderCost paints each line by meaning.
func (m ResearchDialogCmp) renderCost(width int) string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	var out []string
	for _, l := range m.costLines() {
		st := base.Width(width).Padding(0, 1)
		switch l.kind {
		case kindHeader:
			st = st.Foreground(t.Primary()).Bold(true)
		case kindMoney:
			st = st.Foreground(t.Error()).Bold(true)
		case kindDanger:
			st = st.Foreground(t.Error()).Bold(true)
		case kindQuota:
			st = st.Foreground(t.Warning()).Bold(true)
		case kindMeasured:
			st = st.Foreground(t.Success())
		case kindPublished:
			st = st.Foreground(t.Info())
		case kindAssumed:
			st = st.Foreground(t.Warning())
		default:
			st = st.Foreground(t.TextMuted())
		}
		out = append(out, st.Render(emphasise(l.text, t)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// emphasise paints «marked» words in a hot colour INSIDE a line.
//
// GORILLA OVERRIDE: colouring whole lines was not enough — "you have ALREADY
// paid" carried its weight on one word, and that word rendered the same as the
// rest of the sentence. Per-line colour cannot emphasise mid-sentence, which is
// exactly where the meaning sat.
func emphasise(text string, t theme.Theme) string {
	if !strings.Contains(text, "«") {
		return text
	}
	hot := styles.BaseStyle().Foreground(t.Error()).Bold(true)
	var b strings.Builder
	rest := text
	for {
		i := strings.Index(rest, "«")
		if i < 0 {
			b.WriteString(rest)
			break
		}
		j := strings.Index(rest[i:], "»")
		if j < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:i])
		b.WriteString(hot.Render(rest[i+len("«") : i+j]))
		rest = rest[i+j+len("»"):]
	}
	return b.String()
}

// humanCount renders a token count readably: 24800 -> "24.8K".
func humanCount(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%.1fK", float64(n)/1000)
}

// theWarning is the author's, verbatim in spirit. It is here because the
// corporate version told people nothing and this one does not.
func (m ResearchDialogCmp) theWarning() string {
	switch researchOptions[m.selected].mode {
	case "supervised":
		return "Feeling lucky, punk? Double price, every lane checked twice. Well — do ya?"
	case "sequential":
		return "The slow way. Same money, more waiting. Your funeral."
	default:
		return "Same money as slow, done sooner. This one is free lunch — take it."
	}
}

func (m ResearchDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()

	// Never wider than the terminal: an over-wide line makes bubbletea's erase
	// under-reach by a row per wrapped line and the frame marches down the
	// screen. See clampToWidth in tui.go.
	// GORILLA OVERRIDE: use the width the terminal actually has.
	//
	// A fixed 92 wrapped sentences mid-phrase on a wide screen while leaving
	// half the window empty, and the extra wrapped lines pushed the key hints
	// ("enter: go   esc: cancel") off the bottom — so the dialog lost its own
	// instructions. Reported 2026-08-14 with the footer missing.
	//
	// Cap at 120 because prose past that is hard to track back to the next
	// line, but otherwise take what is on offer.
	maxWidth := 120
	if m.width > 0 {
		maxWidth = min(maxWidth, m.width-8)
	}
	if maxWidth < 30 {
		maxWidth = 30
	}

	title := base.Foreground(t.Primary()).Bold(true).Width(maxWidth).Padding(0, 1).
		Render("Research — how do you want this run?")

	rows := []string{title}
	if !m.compact {
		rows = append(rows,
			base.Width(maxWidth).Render(""),
			base.Foreground(t.Text()).Width(maxWidth).Padding(0, 1).
				Render("Helpers investigate in fixed lanes. Each is a full session — this spends your quota."),
			base.Width(maxWidth).Render(""))
	}

	// GORILLA OVERRIDE: full detail for the SELECTED mode only.
	//
	// Describing all three in full made the dialog 45 rows on a 32-row screen,
	// so the key hints fell off the bottom and the user could not see how to
	// confirm or cancel. You only need the detail for the one you are choosing;
	// the others need enough to know they exist.
	for i, opt := range researchOptions {
		if i == m.selected {
			rows = append(rows,
				base.Width(maxWidth).Padding(0, 1).Render("▸ "+
					base.Foreground(t.Background()).Background(t.Primary()).Bold(true).Render(" "+opt.name+" ")),
				base.Foreground(t.TextMuted()).Width(maxWidth).Padding(0, 4).Render(opt.what),
				base.Foreground(t.Warning()).Width(maxWidth).Padding(0, 4).Render(opt.cost),
			)
			continue
		}
		rows = append(rows, base.Foreground(t.TextMuted()).Width(maxWidth).Padding(0, 1).
			Render("   "+opt.name+" — "+opt.short))
	}
	rows = append(rows, base.Width(maxWidth).Render(""))

	rows = append(rows,
		base.Foreground(t.Text()).Width(maxWidth).Padding(0, 1).
			Render(fmt.Sprintf("Helpers: ← %d →   (4 minimum, 10 maximum)", m.agents)),
		m.renderCost(maxWidth),
		base.Foreground(t.Primary()).Bold(true).Width(maxWidth).Padding(1, 1).
			Render(m.theWarning()),
		base.Width(maxWidth).Render(""),
		base.Foreground(t.TextMuted()).Width(maxWidth).Padding(0, 1).
			Render("enter: go   ↑↓: mode   ←→: helpers   esc: cancel"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(styles.PanelBackground()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func (m *ResearchDialogCmp) SetSize(width, height int) {
	m.width, m.height = width, height
	m.compact = height > 0 && height < 40
}

func (m ResearchDialogCmp) Bindings() []key.Binding { return m.keys.ShortHelp() }

// CloseResearchDialogMsg carries the choice back. Chosen is false when the user
// cancelled — a cancel must not silently start a run that costs money.
type CloseResearchDialogMsg struct {
	Chosen   bool
	Mode     string
	Agents   int
	Question string
}

// ShowResearchDialogMsg opens the chooser for a question.
type ShowResearchDialogMsg struct {
	Question string
}
