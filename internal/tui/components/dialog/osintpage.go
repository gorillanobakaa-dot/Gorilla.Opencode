// GORILLA OVERRIDE: this file did not exist upstream. It is the /osint
// capability page — opened by typing /osint with no question, and pointed at
// from /help. The owner's spec: "that command deserves its own page…
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

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
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

func osintContent() []osintLine {
	armed := config.LoadoutEnabled(config.DossierComponentID)
	armLine := osintLine{"red", "STATUS: OFF. Arm it in /context → \"" + config.DossierRowName + "\" → space. Until then /osint <question> refuses."}
	if armed {
		armLine = osintLine{"red", "STATUS: ARMED. /osint <question> will show the burn-rate warning, then run on your say-so."}
	}
	return []osintLine{
		{"h1", "GORILLA OSINT — the professional dossier, explained"},
		{"mute", "↑↓/PgUp/PgDn scroll · esc closes · /osint <question> runs it (after the warning)"},
		{"", ""},
		armLine,
		{"", ""},
		{"h2", "What it is"},
		{"", "A professional intelligence assessment of your question — not a chat answer. The method is a"},
		{"", "civilianized version of the intelligence cycle used by real analysis shops: plan what would"},
		{"", "answer the question, collect from primary sources, vet what came back, grade it, hunt the"},
		{"", "gaps, and only then write the assessment."},
		{"", ""},
		{"h2", "What actually happens when you run it"},
		{"", "1. PLAN    — your question is broken into sub-questions, each with indicators: what evidence"},
		{"", "             would settle it, and where that evidence lives."},
		{"", "2. COLLECT — 4–10 helper agents (you choose) work the sub-questions against a built-in source"},
		{"", "             atlas and a 985-source registry: scholarly APIs (OpenAlex, Crossref, PubMed),"},
		{"", "             SEC corporate filings, World Bank, humanitarian data (HDX, UNHCR), global news"},
		{"", "             (GDELT, refreshed every 15 minutes), sanctions lists, patents, standards, climate"},
		{"", "             data. 866 of the 985 are free; 370 answer with no account at all."},
		{"", "3. VET     — every claim is graded on TWO axes, the way real intelligence is: source"},
		{"", "             reliability A–F (is this outlet/institution trustworthy?) and information"},
		{"", "             credibility 1–6 (is this specific claim confirmed independently?). A1 = official"},
		{"", "             and independently confirmed. F6 = cannot judge. Ten outlets repeating one press"},
		{"", "             release count as ONE source — circular reporting is detected, not multiplied."},
		{"", "4. GAP     — a follow-up round attacks what the first pass missed. Nothing is padded over."},
		{"", "5. PRODUCT — the dossier: the direct answer FIRST (analysts call it BLUF — bottom line up"},
		{"", "             front), then every finding with its grade, SOURCES TRIED (including the ones"},
		{"", "             that failed), NOT ESTABLISHED (what could not be found out, stated plainly),"},
		{"", "             and a recommended action."},
		{"", ""},
		{"h2", "Iron rules it follows"},
		{"", "• It never cites a source it did not actually open."},
		{"", "• It says what it could NOT establish instead of papering over it."},
		{"", "• Every grade travels with its claim — you always know how much weight a sentence bears."},
		{"", ""},
		{"h2", "Where the dossier goes (privacy)"},
		{"", "The finished dossier is written as a markdown file OUTSIDE your working folder:"},
		{"", "  " + config.DossierDir()},
		{"", "Deliberate: working folders are often git repositories, and a private question must never"},
		{"", "end up in a commit pushed to the internet. The folder is yours, on your machine, nowhere else."},
		{"", ""},
		{"h2", "What it costs — the honest part"},
		{"red", "Every helper is a FULL model session. A 10-helper supervised run is ~18 sessions plus a gap"},
		{"red", "round. On paid API pricing that is real money at a real rate per minute; on a free tier it"},
		{"red", "is a large bite of your quota. The warning screen before each run computes the figure for"},
		{"red", "YOUR model at THAT moment — read it. It ships OFF for exactly this reason."},
		{"", ""},
		{"", "Use /research for everyday questions (same discipline, one pass, far cheaper). Use /osint"},
		{"", "when being wrong is more expensive than the run: health decisions, money decisions, claims"},
		{"", "you are about to build on, anything you need to stand behind."},
		{"", ""},
		{"h2", "Who this is for"},
		{"", "A cross between academic research and a government intelligence briefing, written for the"},
		{"", "person neither tradition serves. When someone with nobody to ask brings an honest question,"},
		{"", "this answers it the way an analyst briefs a principal: direct answer first, sources graded,"},
		{"", "limits stated, next step recommended — with the dignity the format itself enforces."},
	}
}

func (m OsintPageCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	// GORILLA FIX (2026-08-17): see dialogWidth — the old max(70, …) floor drew
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
			text = string(r[:w-2]) + "…"
		}
		b = append(b, st.Render(text))
	}
	if end < len(lines) {
		b = append(b, base.Width(w).Foreground(t.TextMuted()).
			Render(fmt.Sprintf("  … %d more line(s) — ↓ to continue", len(lines)-end)))
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
