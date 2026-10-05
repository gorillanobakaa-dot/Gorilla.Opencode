// GORILLA OVERRIDE: this file did not exist upstream. It is the /osint
// capability page — opened by typing /osint with no question, and pointed at
// from /help. The owner's spec: "that command deserves its own page...
// maximized to the max in which the whole capabilities will have to be
// explained to the user."
//
// It is a PAGE, not a paragraph, because this is the one feature whose misuse
// costs real money and whose proper use is genuinely different from chatting.
// Scrollable, plain language, honest about cost, and it says where the product
// lands and why (outside the working folder — privacy).
package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type OsintPageCmp struct {
	width, height int
	scrollTop     int
}

func NewOsintPageCmp() OsintPageCmp { return OsintPageCmp{} }

func (m *OsintPageCmp) SetSize(w, h int) { m.width, m.height = w, h }

func (m OsintPageCmp) Init() tea.Cmd { return nil }

func (m OsintPageCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.scrollTop > 0 {
				m.scrollTop--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			m.scrollTop++ // clamped in View against the real line count
		case key.Matches(msg, key.NewBinding(key.WithKeys("pgup"))):
			m.scrollTop = max(0, m.scrollTop-10)
		case key.Matches(msg, key.NewBinding(key.WithKeys("pgdown"))):
			m.scrollTop += 10
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q", "enter"))):
			return m, util.CmdHandler(CloseOsintPageMsg{})
		}
	}
	return m, nil
}

// pageLines is the content. Each entry is (kind, text); kinds pick the style.
type osintLine struct {
	kind string // "h1", "h2", "red", "mute", ""
	text string
}

// osintPageWidth is the widest a line of page text may be. The page clips a
// longer line with an ellipsis, so every paragraph below is wrapped to this
// before it is shown; no sentence is typed to a guessed width.
const osintPageWidth = 96

// para wraps one paragraph of page text. lead opens the first line ("1. LANES
// — ", "  * ") and every later line is indented to the same depth, so a
// numbered step or a bullet keeps its hanging indent.
func para(kind, lead, text string) []osintLine {
	indent := strings.Repeat(" ", len([]rune(lead)))
	var out []osintLine
	line, empty := lead, true
	for _, word := range strings.Fields(text) {
		switch {
		case empty:
			line += word
			empty = false
		case len([]rune(line))+1+len([]rune(word)) > osintPageWidth:
			out = append(out, osintLine{kind, line})
			line = indent + word
		default:
			line += " " + word
		}
	}
	if !empty {
		out = append(out, osintLine{kind, line})
	}
	return out
}

// osintContent is the page.
//
// GORILLA FIX (2026-10-05): audit findings O1, O2, O4, O5 and O6. The page this
// replaces described a different program from the one that ran:
//
//   - "1. PLAN — your question is broken into sub-questions, each with
//     indicators." No planning code exists. The engine works the question in
//     fixed lanes. (O1; the lanes themselves were also the wrong ones, which is
//     fixed in the research tool. They are listed here from the role table.)
//   - "4. GAP — a follow-up round attacks what the first pass missed." Nothing
//     ran one; the model may ask for one follow-up, and now cannot ask for two.
//     (O2)
//   - "a 985-source registry ... 866 of the 985 are free; 370 answer with no
//     account ... refreshed every 15 minutes ... ~18 sessions." Typed. The
//     registry is not shipped and helpers are given the atlas only. Every
//     number on the page is now computed from the thing it counts. (O4)
//   - "Iron rules it follows: it never cites a source it did not actually
//     open." That is an instruction in the helper prompt. No code checks it.
//     A rule nobody enforces, printed as a guarantee, is the confident wrong
//     answer this feature is sold as the cure for. They are worded as what
//     they are. (O5)
//   - "The folder is yours, on your machine, nowhere else." True of the file
//     and silent about everything else: the question and every search term go
//     to the model provider and to the search and scholarly services. On the
//     page whose whole job is explaining the feature, that omission was the
//     misleading part. (O6)
func osintContent() []osintLine {
	armed := config.LoadoutEnabled(config.DossierComponentID)
	researchOn := config.LoadoutEnabled(osintResearchComponentID)

	var status []osintLine
	switch {
	case armed && researchOn:
		status = para("red", "", "STATUS: ARMED. /osint <question> will show the burn-rate warning, then run on your say-so.")
	case armed:
		status = para("red", "", "STATUS: ARMED, BUT IT CANNOT RUN. The research helpers are switched off, and a dossier is made of them. "+
			"Turn them on in /context (the row that mentions /research) and press space.")
	default:
		status = para("red", "", "STATUS: OFF. Arm it in /context -> \""+config.DossierRowName+"\" -> space. Until then /osint <question> refuses.")
	}

	minHelpers, maxHelpers := agent.ResearchMinAgents, agent.ResearchMaxAgents
	atlas, atlasNeedKey := agent.SourceAtlasCount()
	maxSessions, maxAudited := agent.SupervisedSessionsFor(agent.DoctrineDossier, maxHelpers)
	followUp := agent.DossierFollowUpSessions(agent.ModeSupervised)
	mandatory := agent.DossierLaneTitles(minHelpers)
	all := agent.DossierLaneTitles(maxHelpers)

	var c []osintLine
	add := func(lines ...osintLine) { c = append(c, lines...) }
	blank := osintLine{"", ""}

	add(osintLine{"h1", "GORILLA " + config.AllSourceProductName + " — explained"})
	add(osintLine{"mute", "up/down, PgUp/PgDn scroll | esc closes | /osint <question> runs it (after the warning)"})
	add(blank)
	add(status...)
	add(blank)

	add(osintLine{"h2", "What it is"})
	add(para("", "", "An ALL-SOURCE assessment of your question — not a chat answer. \"All-source\" is the trade term for "+
		"fusing different KINDS of source — the official record, the research literature, the reporting, and "+
		"the case against — and weighing them against each other, which is what makes it assessment rather "+
		"than searching. The method is a civilianised version of the cycle real analysis shops run: collect by "+
		"kind of source, vet, grade, say what is missing, then write.")...)
	add(blank)

	add(osintLine{"h2", "What actually happens when you run it"})
	add(para("", "1. LANES   — ", fmt.Sprintf("there is no planning stage. The program works the question in fixed lanes, one "+
		"helper agent per lane, and you choose how many (%d to %d). The first %d always run:", minHelpers, maxHelpers, minHelpers))...)
	for _, title := range mandatory {
		add(para("", "               * ", title)...)
	}
	add(para("", "             ", "More helpers add, in this order:")...)
	for _, title := range all[len(mandatory):] {
		add(para("", "               * ", title)...)
	}
	add(para("", "             ", "No lane searches your own machine, and dossier helpers are not given the tools to.")...)
	add(para("", "2. COLLECT — ", fmt.Sprintf("each helper searches the web and fetches pages, starting from a built-in atlas of "+
		"%d sources (scholarly indexes, official statistics, filings, humanitarian data, news; %d of them "+
		"want a free registration key). The atlas is a starting map; helpers may go beyond it. Nothing here "+
		"promises a source was reached: the dossier's SOURCES TRIED section lists what actually was.",
		atlas, atlasNeedKey))...)
	add(para("", "3. GRADE   — ", "helpers are told to grade every claim on TWO axes: source reliability A-F (is this "+
		"outlet or institution trustworthy?) and information credibility 1-6 (is this specific claim confirmed "+
		"independently?). A1 = official and independently confirmed. F6 = cannot judge.")...)
	add(para("", "4. GAP     — ", fmt.Sprintf("an optional follow-up. If the first pass leaves a gap the answer depends on, the model MAY "+
		"ask once more. The program holds that to %d helpers in the mode you chose and refuses a third call. "+
		"Often there is no follow-up at all.", minHelpers))...)
	add(para("", "5. PRODUCT — ", "the dossier: the direct answer FIRST (analysts call it BLUF — bottom line up front), "+
		"then every finding with its grade, SOURCES TRIED (including the ones that failed), NOT ESTABLISHED "+
		"(what could not be found out, stated plainly), and a recommended action.")...)
	add(blank)

	add(osintLine{"h2", "What the helpers are instructed to do — instructions, not guarantees"})
	add(para("", "", "These are written into every helper's orders. The program does not check that a model obeyed "+
		"them, and models do disobey. They are listed so you know what to hold the dossier to:")...)
	add(para("", "  * ", "Cite only a source it actually opened; a link built from memory counts as an invention.")...)
	add(para("", "  * ", "Trace a claim to its origin before counting a second source as confirmation: ten outlets "+
		"repeating one press release are one source.")...)
	add(para("", "  * ", "Say what it could NOT establish instead of papering over it.")...)
	add(para("", "  * ", "Keep every grade attached to its claim.")...)
	add(para("", "", "What the program itself does enforce: the lanes, the helper count, the one follow-up, no "+
		"local-file tools for dossier helpers, and saving every lane's report to disk before the write-up.")...)
	add(blank)

	add(osintLine{"h2", "How it states what it thinks (and why that is the anti-nonsense part)"})
	add(para("", "", "The weakness of an AI answer is uniform confidence: everything sounds equally certain. The helpers "+
		"and the write-up are told to follow the UK government's standard for intelligence assessment instead, "+
		"which separates two things most writing muddles together:")...)
	add(para("", "  LIKELIHOOD — ", "how likely the judgement is, in seven fixed words (remote chance, highly unlikely, "+
		"unlikely, realistic possibility, likely, highly likely, almost certain). No invented percentages.")...)
	add(para("", "  CONFIDENCE — ", "how solid the basis is: HIGH, MODERATE or LOW, and which of information base, "+
		"analytical rigour, or complexity set that level.")...)
	add(para("", "", "So \"highly likely, LOW confidence — one source, uncorroborated\" is a legitimate answer, and a far more "+
		"useful one than a confident paragraph hiding the same thin evidence. Each line is also to be marked "+
		"FACT, INFERENCE or ASSUMPTION, with rival explanations considered.")...)
	add(blank)

	add(osintLine{"h2", "Privacy — what stays here and what leaves"})
	add(para("", "", "STAYS: the finished dossier, and every lane's raw findings, are written as files OUTSIDE your "+
		"working folder, in a folder only your account can open:")...)
	add(osintLine{"", "  " + config.DossierDir()})
	add(para("", "", "Deliberate: working folders are often git repositories, and a private question must never end up "+
		"in a commit pushed to the internet.")...)
	add(para("red", "", "LEAVES: your question, in full, goes to the provider of the model you are using — to every helper "+
		"session, not one. The search terms the helpers write go to the search and scholarly services they query "+
		"(the scholarly indexes, the statistics and filings sites, and your SearXNG or the sites lynx fetches), "+
		"and the pages they open see the request. Those requests carry this program's name, not yours, but the "+
		"words of the query are the query. Helpers are told to keep your name, email and exact location out of "+
		"search terms; nothing checks that they did. Do not put in the question what must not leave the machine.")...)
	add(para("", "", "The program asks your permission before helpers search, unless you have already allowed it for the session.")...)
	add(blank)

	add(osintLine{"h2", "What it costs — the honest part"})
	add(para("red", "", fmt.Sprintf("Every helper is a FULL model session. A %d-helper supervised run is %d sessions (%d of the %d "+
		"lanes audited), and an optional follow-up can add up to %d more. On paid API pricing that is real "+
		"money at a real rate per minute; on a free tier it is a large bite of your quota. The warning screen "+
		"before each run computes the figure for YOUR model at THAT moment — read it. It ships OFF for exactly "+
		"this reason.", maxHelpers, maxSessions, maxAudited, maxHelpers, followUp))...)
	add(blank)
	add(para("", "", "Use /research for everyday questions about code and this machine (fixed engineering lanes, one "+
		"pass, cheaper). Use /osint when the question is about the world and being wrong is more expensive than "+
		"the run: health decisions, money decisions, claims you are about to build on.")...)
	add(blank)

	add(osintLine{"h2", "Who this is for"})
	add(para("", "", "A cross between academic research and a government intelligence briefing, written for the person "+
		"neither tradition serves. When someone with nobody to ask brings an honest question, this answers it "+
		"the way an analyst briefs a principal: direct answer first, sources graded, limits stated, next step "+
		"recommended — with the dignity the format itself enforces.")...)
	return c
}

// osintResearchComponentID is the loadout row the dossier's engine lives under.
const osintResearchComponentID = "tool.research"

func (m OsintPageCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	// GORILLA FIX (2026-08-17): see dialogWidth — the old max(70, ...) floor drew
	// a frame wider than a narrow terminal, which strands rows in scrollback.
	w := dialogWidth(m.width, 104, 6)

	lines := osintContent()
	// Window the content to the terminal: chrome is border(2)+padding(2).
	visible := len(lines)
	if m.height > 0 {
		visible = max(5, m.height-6)
	}
	top := m.scrollTop
	if top > len(lines)-visible {
		top = max(0, len(lines)-visible)
	}
	end := min(top+visible, len(lines))

	var b []string
	for _, l := range lines[top:end] {
		st := base.Width(w).MaxWidth(w)
		switch l.kind {
		case "h1":
			st = st.Foreground(t.Primary()).Bold(true)
		case "h2":
			st = st.Foreground(t.Primary()).Bold(true)
		case "red":
			st = st.Foreground(lipgloss.Color("#FF0000")).Bold(true)
		case "mute":
			st = st.Foreground(t.TextMuted())
		}
		text := l.text
		if r := []rune(text); len(r) > w-1 {
			text = string(r[:w-4]) + styles.Ellipsis
		}
		b = append(b, st.Render(text))
	}
	if end < len(lines) {
		b = append(b, base.Width(w).Foreground(t.TextMuted()).
			Render(fmt.Sprintf("  ... %d more line(s) — down to continue", len(lines)-end)))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, b...)
	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary()).
		BorderBackground(styles.PanelBackground()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func (m OsintPageCmp) Bindings() []key.Binding { return nil }
