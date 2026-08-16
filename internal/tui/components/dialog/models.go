package dialog

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	// GORILLA OVERRIDE: widened from 40 and 10 — 40 cols truncated
	// longer model names and the product name; 10 rows hid most of a
	// large discovered provider (NVIDIA NIM ships ~119 models).
	// numVisibleModels is the FLOOR. The real count comes from visibleRows(),
	// which grows the list to fit the window — a shortlist of thirty on a
	// 1600x900 screen was showing fourteen rows and hiding the rest behind a
	// scroll, which is the wall this list exists to remove.
	numVisibleModels = 14
	maxDialogWidth   = 62
)

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Model models.Model
}

// CloseModelDialogMsg is sent when a model is selected
type CloseModelDialogMsg struct{}

// ModelDialog interface for the model selection dialog
type ModelDialog interface {
	tea.Model
	layout.Bindings
	// SwitchToProvider pre-scrolls the picker to the given provider's column
	// and rebuilds the model list for it. A no-op if the provider is not in
	// the current getEnabledProviders set — the picker opens on its default
	// column rather than an empty screen.
	//
	// GORILLA OVERRIDE: needed for the /connect "u" (use for session) flow so
	// UseProviderMsg can land the user on the right tab in one keypress.
	SwitchToProvider(p models.ModelProvider)
}

type modelDialogCmp struct {
	models             []models.Model
	provider           models.ModelProvider
	availableProviders []models.ModelProvider

	selectedIdx     int
	width           int
	height          int
	scrollOffset    int
	hScrollOffset   int
	hScrollPossible bool

	// GORILLA OVERRIDE: "/" filters ALL enabled providers at once, matching on
	// name AND description. The catalogue is past 270 entries on OpenRouter
	// alone; walking columns row by row to find "the free coding models" is a
	// reading assignment, and the whole point of the descriptions is that they
	// carry the words someone would search for ("coding", "reasoning", "FREE").
	searchActive bool
	query        string
	searchDomain []models.Model // every enabled provider's models, built on open
	savedIdx     int            // selection to restore when search closes
	savedScroll  int

	// GORILLA OVERRIDE: tab opens a full page for the highlighted model. One
	// row cannot hold what an informed choice needs — the row is the headline,
	// this is the article: full description, exact prices, context, and WHICH
	// credential serves it.
	detail *models.Model
}

type modelKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Enter  key.Binding
	Escape key.Binding
	J      key.Binding
	K      key.Binding
	H      key.Binding
	L      key.Binding
	// GORILLA OVERRIDE: space toggles. Deselection is as important as
	// selection — "this one did not do what it claimed" is exactly as useful a
	// judgement as "this one is good", and a list you can only add to becomes
	// another wall.
	Bookmark key.Binding
	// GORILLA OVERRIDE: jump straight to the shortlist from any column.
	// ←/→ move ONE provider at a time, and after browsing to bookmark things
	// you are typically several columns away from it — so the list you just
	// built was reachable only by pressing left repeatedly, or by closing the
	// dialog and reopening it. Neither is discoverable.
	ShowBookmarks key.Binding
	// GORILLA OVERRIDE: search every provider by name AND description.
	Search key.Binding
	// GORILLA OVERRIDE: full detail page for the highlighted model.
	Details key.Binding
}

var modelKeys = modelKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous model"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next model"),
	),
	Left: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "scroll left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "scroll right"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select model"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
	J: key.NewBinding(
		key.WithKeys("j"),
		key.WithHelp("j", "next model"),
	),
	K: key.NewBinding(
		key.WithKeys("k"),
		key.WithHelp("k", "previous model"),
	),
	H: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "scroll left"),
	),
	L: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "scroll right"),
	),
	Bookmark: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "bookmark / unbookmark"),
	),
	ShowBookmarks: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "jump to your bookmarks"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search all providers"),
	),
	Details: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "full model details"),
	),
}

// visibleRows is how many models fit in the current window.
//
// GORILLA OVERRIDE (2026-08-09): this was a hard-coded 14 regardless of screen.
// The shortlist is the one list people are meant to read at a glance, and at
// thirty entries most of it sat below the fold. Sized to the window instead,
// with a floor for tiny terminals and a ceiling so the frame can never grow
// taller than the window — a frame taller than its window is the bug that makes
// the footer march down the screen (see TestFooterMustStaySmallerThanTheWindow).
func (m *modelDialogCmp) visibleRows() int {
	// title + subtitle + connection line (or search query line) + scroll
	// indicator + hint + padding + border + margin
	const chrome = 13
	if m.height <= 0 {
		return numVisibleModels // not sized yet
	}
	n := m.height - chrome
	if n < numVisibleModels {
		n = numVisibleModels
	}
	if n > 30 {
		n = 30
	}
	return n
}

func (m *modelDialogCmp) Init() tea.Cmd {
	m.setupModels()
	return nil
}

func (m *modelDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Modal states first: the detail page and the search prompt both
		// repurpose keys the list uses (space, letters), so they must see the
		// keystroke before the list bindings do.
		if m.detail != nil {
			return m.updateDetail(msg)
		}
		if m.searchActive {
			return m.updateSearch(msg)
		}
		switch {
		case key.Matches(msg, modelKeys.Up) || key.Matches(msg, modelKeys.K):
			m.moveSelectionUp()
		case key.Matches(msg, modelKeys.Down) || key.Matches(msg, modelKeys.J):
			m.moveSelectionDown()
		case key.Matches(msg, modelKeys.Left) || key.Matches(msg, modelKeys.H):
			if m.hScrollPossible {
				m.switchProvider(-1)
			}
		case key.Matches(msg, modelKeys.Right) || key.Matches(msg, modelKeys.L):
			if m.hScrollPossible {
				m.switchProvider(1)
			}
		case key.Matches(msg, modelKeys.ShowBookmarks):
			idx := findProviderIndex(m.availableProviders, ProviderBookmarks)
			if idx < 0 {
				// Say what to do instead of silently doing nothing — a key that
				// appears to be broken is worse than one that explains itself.
				return m, util.ReportWarn("you have not bookmarked anything yet — press space on a model to add it")
			}
			m.hScrollOffset = idx
			m.setupModelsForProvider(ProviderBookmarks)
			return m, nil
		case key.Matches(msg, modelKeys.Bookmark):
			return m.toggleBookmarkCurrent()
		case key.Matches(msg, modelKeys.Enter):
			return m.selectCurrent()
		case key.Matches(msg, modelKeys.Search):
			m.openSearch()
			return m, nil
		case key.Matches(msg, modelKeys.Details):
			m.openDetail()
			return m, nil
		case key.Matches(msg, modelKeys.Escape):
			return m, util.CmdHandler(CloseModelDialogMsg{})
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// selectCurrent emits the highlighted (or detailed) model as the session's
// choice. Shared by the column list, the search results and the detail page so
// the three cannot drift on the retired-bookmark refusal below.
func (m *modelDialogCmp) selectCurrent() (tea.Model, tea.Cmd) {
	if m.detail == nil && len(m.models) == 0 {
		return m, nil
	}
	var chosen models.Model
	if m.detail != nil {
		chosen = *m.detail
	} else {
		chosen = m.models[m.selectedIdx]
	}
	// GORILLA OVERRIDE: refuse a bookmark whose model no longer exists.
	// Without this the unresolvable id reaches the agent and comes back
	// as a generic failure, which reads as "this program is broken"
	// rather than "that model was retired". Say which, and say what to
	// press.
	if _, ok := models.SupportedModels[chosen.ID]; !ok {
		return m, util.ReportWarn(fmt.Sprintf(
			"%s is no longer offered by its provider — press space to remove it from your bookmarks",
			chosen.ID))
	}
	util.ReportInfo(fmt.Sprintf("selected model: %s", chosen.Name))
	// Leave the dialog the way the next open should find it: on a provider
	// column, not inside a stale search or detail page — this object is
	// reused between opens without a reset.
	m.detail = nil
	if m.searchActive {
		m.closeSearch()
	}
	return m, util.CmdHandler(ModelSelectedMsg{Model: chosen})
}

// toggleBookmarkCurrent flips the bookmark on the highlighted model and, when
// a provider column is showing, rebuilds the carousel: the shortlist column
// may have just appeared or emptied, and leaving the carousel describing a
// state that no longer exists is how a menu starts lying about itself. In
// search mode the rebuild waits until the search closes — the filtered list on
// screen must not be yanked out from under the cursor.
func (m *modelDialogCmp) toggleBookmarkCurrent() (tea.Model, tea.Cmd) {
	var id string
	switch {
	case m.detail != nil:
		// The detail page holds its own copy: after a rebuild below the list
		// selection can move, and a second space must still mean THIS model.
		id = string(m.detail.ID)
	case len(m.models) > 0:
		id = string(m.models[m.selectedIdx].ID)
	default:
		return m, nil
	}
	on, err := config.ToggleBookmark(id)
	if err != nil {
		return m, util.ReportError(err)
	}
	if !m.searchActive {
		keep := m.provider
		m.availableProviders = getEnabledProviders(config.Get())
		m.hScrollPossible = len(m.availableProviders) > 1
		if idx := findProviderIndex(m.availableProviders, keep); idx >= 0 {
			m.hScrollOffset = idx
		} else {
			m.hScrollOffset = 0
			keep = m.availableProviders[0]
		}
		sel := m.selectedIdx
		m.setupModelsForProvider(keep)
		if sel < len(m.models) {
			m.selectedIdx = sel
		}
	}
	if on {
		util.ReportInfo("bookmarked — press b to see your list")
	} else {
		util.ReportInfo("removed from bookmarks")
	}
	return m, nil
}

// openSearch snapshots every enabled provider's models as the search domain.
// The bookmarks column is skipped: its entries are the same models again, and
// a search that returns the same row twice reads as a bug.
func (m *modelDialogCmp) openSearch() {
	m.searchDomain = nil
	for _, p := range m.availableProviders {
		if p == ProviderBookmarks {
			continue
		}
		m.searchDomain = append(m.searchDomain, getModelsForProvider(p)...)
	}
	m.savedIdx, m.savedScroll = m.selectedIdx, m.scrollOffset
	m.searchActive = true
	m.query = ""
	m.applyFilter()
}

// closeSearch returns to the provider column the search was opened from, with
// the selection where it was. The column set is rebuilt because a bookmark may
// have been toggled from a detail page while the search was open.
func (m *modelDialogCmp) closeSearch() {
	m.searchActive = false
	m.query = ""
	m.searchDomain = nil
	m.availableProviders = getEnabledProviders(config.Get())
	m.hScrollPossible = len(m.availableProviders) > 1
	if idx := findProviderIndex(m.availableProviders, m.provider); idx >= 0 {
		m.hScrollOffset = idx
	} else if len(m.availableProviders) > 0 {
		m.hScrollOffset = 0
		m.provider = m.availableProviders[0]
	}
	m.setupModelsForProvider(m.provider)
	if m.savedIdx < len(m.models) {
		m.selectedIdx = m.savedIdx
		m.scrollOffset = m.savedScroll
	}
}

// applyFilter recomputes the visible list from the query. Every space-separated
// term must appear somewhere in the model's name, description, detail text,
// provider or id — so "free coding" works, and so does "nvidia 1m".
func (m *modelDialogCmp) applyFilter() {
	terms := strings.Fields(strings.ToLower(m.query))
	var out []models.Model
	for _, mod := range m.searchDomain {
		if modelMatches(mod, terms) {
			out = append(out, mod)
		}
	}
	m.models = out
	m.selectedIdx, m.scrollOffset = 0, 0
}

func modelMatches(mod models.Model, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{
		mod.Name, mod.Description, mod.Detail,
		string(mod.Provider), string(mod.ID), models.LocalEndpointFor(mod.ID),
	}, " "))
	for _, t := range terms {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}

// openDetail shows the full page for the highlighted model.
func (m *modelDialogCmp) openDetail() {
	if len(m.models) == 0 {
		return
	}
	mod := m.models[m.selectedIdx]
	m.detail = &mod
}

// updateSearch owns the keys while the search prompt is live. Only the arrows,
// enter, tab and esc keep their list meaning; every printable character —
// including space and the letters j/k/h/l/b that navigate the list — is text.
func (m *modelDialogCmp) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, modelKeys.Up):
		m.moveSelectionUp()
	case key.Matches(msg, modelKeys.Down):
		m.moveSelectionDown()
	case key.Matches(msg, modelKeys.Enter):
		return m.selectCurrent()
	case key.Matches(msg, modelKeys.Details):
		m.openDetail()
	case key.Matches(msg, modelKeys.Escape):
		m.closeSearch()
	default:
		switch msg.Type {
		case tea.KeyRunes:
			m.query += string(msg.Runes)
			m.applyFilter()
		case tea.KeySpace:
			m.query += " "
			m.applyFilter()
		case tea.KeyBackspace:
			if r := []rune(m.query); len(r) > 0 {
				m.query = string(r[:len(r)-1])
				m.applyFilter()
			}
		}
	}
	return m, nil
}

// updateDetail owns the keys while the detail page is showing.
func (m *modelDialogCmp) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, modelKeys.Enter):
		return m.selectCurrent()
	case key.Matches(msg, modelKeys.Bookmark):
		return m.toggleBookmarkCurrent()
	case key.Matches(msg, modelKeys.Escape), key.Matches(msg, modelKeys.Details):
		m.detail = nil
	}
	return m, nil
}

// moveSelectionUp moves the selection up and STOPS at the top.
//
// GORILLA OVERRIDE (2026-08-09): this used to wrap around - top jumped to
// bottom, bottom jumped to top. With a handful of models that is a convenience.
// NVIDIA NIM alone exposes 128, and at that length wrapping means there is no
// way to tell "I have reached the beginning" from "I am somewhere in the
// middle": you scroll past the end without noticing and lose your place
// entirely. A list with no ends is a list you cannot navigate, only wander.
//
// Both ends are hard now. Holding a key at the boundary does nothing, which is
// exactly the feedback that tells you where you are.
func (m *modelDialogCmp) moveSelectionUp() {
	if m.selectedIdx == 0 {
		return // hard stop at the top
	}
	m.selectedIdx--

	// Keep selection visible
	if m.selectedIdx < m.scrollOffset {
		m.scrollOffset = m.selectedIdx
	}
}

// moveSelectionDown moves the selection down and STOPS at the bottom.
// See moveSelectionUp for why neither end wraps.
func (m *modelDialogCmp) moveSelectionDown() {
	if m.selectedIdx >= len(m.models)-1 {
		return // hard stop at the bottom
	}
	m.selectedIdx++

	// Keep selection visible
	if m.selectedIdx >= m.scrollOffset+m.visibleRows() {
		m.scrollOffset = m.selectedIdx - (m.visibleRows() - 1)
	}
}

func (m *modelDialogCmp) switchProvider(offset int) {
	newOffset := m.hScrollOffset + offset

	// Ensure we stay within bounds
	if newOffset < 0 {
		newOffset = len(m.availableProviders) - 1
	}
	if newOffset >= len(m.availableProviders) {
		newOffset = 0
	}

	m.hScrollOffset = newOffset
	m.provider = m.availableProviders[m.hScrollOffset]
	m.setupModelsForProvider(m.provider)
}

// SwitchToProvider pre-scrolls the picker to `p`'s column. Rebuilds the
// enabled-providers list first, so a provider that was added via /connect a
// moment ago is picked up. If `p` is not enabled, the picker opens on its
// default column — the caller (typically a UseProviderMsg handler) is
// responsible for ensuring the provider is registered first.
//
// GORILLA OVERRIDE: added for the /connect u=use-for-session flow.
func (m *modelDialogCmp) SwitchToProvider(p models.ModelProvider) {
	cfg := config.Get()
	m.availableProviders = getEnabledProviders(cfg)
	m.hScrollPossible = len(m.availableProviders) > 1
	m.searchActive = false
	m.query = ""
	m.searchDomain = nil
	m.detail = nil
	if len(m.availableProviders) == 0 {
		return
	}
	idx := findProviderIndex(m.availableProviders, p)
	if idx < 0 {
		idx = 0
	}
	m.hScrollOffset = idx
	m.provider = m.availableProviders[idx]
	m.selectedIdx = 0
	m.scrollOffset = 0
	m.setupModelsForProvider(m.provider)
}

func (m *modelDialogCmp) View() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()
	// GORILLA OVERRIDE: responsive width — as wide as the terminal
	// allows so long model descriptions ("DeepSeek V4 Pro — 1.6T MoE,
	// 1M ctx, 80.6% SWE-bench") are readable, not truncated at 62.
	w := maxDialogWidth
	if m.width > 0 && m.width-8 > w {
		w = m.width - 8
	}
	// GORILLA OVERRIDE: … and never wider than the terminal. maxDialogWidth
	// acted as a floor, so on anything narrower than 68 columns the frame was
	// clipped at the screen edge. Chrome is subtracted from the terminal
	// (border 2 + padding 4), never added to content. 24 is the last-resort
	// floor at which anything is still legible.
	if m.width > 0 && w > m.width-6 {
		w = m.width - 6
		if w < 24 {
			w = 24
		}
	}

	// GORILLA OVERRIDE: the detail page replaces the list entirely — one
	// model, everything known about it, because one row cannot hold an
	// informed decision.
	if m.detail != nil {
		return m.renderDetail(w)
	}

	// Capitalize first letter of provider name (with friendly overrides)
	titleText := fmt.Sprintf("Select %s Model", providerDisplayName(m.provider))
	if m.searchActive {
		titleText = "Search Models — every provider at once"
	}
	title := baseStyle.
		Foreground(t.Primary()).
		Bold(true).
		Width(w).
		Render(titleText)

	// The lines between the title and the list: what this column is (ranked
	// or not), and WHICH credential serves it — then one blank spacer.
	var infoLines []string
	if m.searchActive {
		// The query line doubles as the match counter, so narrowing the words
		// visibly narrows the number.
		qline := fmt.Sprintf("/ %s▌  — %d of %d models match name or description",
			m.query, len(m.models), len(m.searchDomain))
		if r := []rune(qline); len(r) > w {
			qline = string(r[:w-1]) + "…"
		}
		infoLines = append(infoLines, baseStyle.Foreground(t.Accent()).Width(w).Render(qline))
	} else {
		// GORILLA OVERRIDE: for a curated provider, tell the user the top of
		// the list is the probe-verified coding ranking (1 = best), and that
		// the rest of the provider's catalog follows below — nothing is hidden.
		if len(m.models) > 0 && m.models[0].Rank > 0 {
			ranked := 0
			for _, mm := range m.models {
				if mm.Rank > 0 {
					ranked++
				}
			}
			infoLines = append(infoLines, baseStyle.Foreground(t.TextMuted()).Width(w).
				Render(fmt.Sprintf("%d ranked best-first (1=best); %d more below — full catalog, your call", ranked, len(m.models)-ranked)))
		}
		// GORILLA OVERRIDE: name the credential behind the column. A row
		// reading "MoonshotAI: Kimi K2" LOOKS like it comes from Moonshot,
		// but the request is billed to whichever key this provider column
		// holds — and someone rotating free keys to spread quota cannot make
		// an informed pick without knowing which key is live right now.
		if cl := m.connectionLine(); cl != "" {
			if r := []rune(cl); len(r) > w {
				cl = string(r[:w-1]) + "…"
			}
			infoLines = append(infoLines, baseStyle.Foreground(t.TextMuted()).Width(w).Render(cl))
		}
	}
	infoLines = append(infoLines, baseStyle.Width(w).Render(""))

	// Render visible models
	endIdx := min(m.scrollOffset+m.visibleRows(), len(m.models))
	modelItems := make([]string, 0, endIdx-m.scrollOffset)

	for i := m.scrollOffset; i < endIdx; i++ {
		// GORILLA OVERRIDE: show "N. Name — description" for curated
		// ranked models (N = quality rank, 1 = best), so the picker
		// reads as a leaderboard; plain "Name — description" otherwise.
		label := m.models[i].Name
		if d := m.models[i].Description; d != "" {
			label = fmt.Sprintf("%s — %s", m.models[i].Name, d)
		}
		// GORILLA OVERRIDE: every OpenAI-compatible endpoint lands under the one
		// "local" provider, so name the connection each model comes from.
		// Otherwise a list mixing 102 NVIDIA NIM models with the two served by a
		// local Ollama gives no way to tell which is the cloud quota and which is
		// the machine in the room.
		if ep := models.LocalEndpointFor(m.models[i].ID); ep != "" {
			label = fmt.Sprintf("[%s] %s", ep, label)
		} else if m.provider == ProviderBookmarks || m.searchActive {
			// GORILLA OVERRIDE: name the provider inside every list that mixes
			// providers — the shortlist and the search results. "Gemini 3.6
			// Flash" can appear twice, once via Antigravity and once via a
			// Gemini API key, and with nothing to tell them apart they read as
			// duplicates. The reported instinct was to delete one, which would
			// silently remove a different route to the same model, on a
			// different quota. Local models already carry their endpoint name
			// from the branch above; everything else needs its provider.
			if home := models.SupportedModels[m.models[i].ID].Provider; home != "" {
				label = fmt.Sprintf("[%s] %s", home, label)
			}
		}
		// Ranks are per-provider judgements; in a cross-provider search they
		// would collide ("1." three times) and imply an ordering nobody made.
		if r := m.models[i].Rank; r > 0 && !m.searchActive {
			label = fmt.Sprintf("%2d. %s", r, label)
		}
		// GORILLA OVERRIDE: mark what is already on the shortlist, everywhere it
		// appears. Without it, someone browsing a provider has no way to tell
		// whether they already bookmarked something and toggles it back off by
		// accident — and the whole point of the list is that you decide once.
		if m.provider != ProviderBookmarks && config.IsBookmarked(string(m.models[i].ID)) {
			label = "★ " + label
		}
		if r := []rune(label); len(r) > w-1 {
			label = string(r[:w-2]) + "…"
		}
		itemStyle := baseStyle.Width(w)
		if i == m.selectedIdx {
			itemStyle = itemStyle.Background(t.Primary()).
				Foreground(t.Background()).Bold(true)
		}
		modelItems = append(modelItems, itemStyle.Render(label))
	}
	if m.searchActive && len(m.models) == 0 {
		modelItems = append(modelItems, baseStyle.Foreground(t.TextMuted()).Width(w).
			Render("nothing matches — try fewer or different words; esc goes back"))
	}

	scrollIndicator := m.getScrollIndicators(w)

	// GORILLA OVERRIDE: say what the keys do, on every provider column.
	//
	// Bookmarking was unfindable: nothing anywhere mentioned space, and the only
	// label naming the shortlist appeared AFTER you had already used it —
	// a feature you cannot discover without already knowing it. Reported from a
	// live run: "there's no indication that YOU HAVE THE OPTION to even select
	// your models and bookmark them".
	//
	// One line, on every column, in the frame the user is already reading.
	var segs []hintSeg
	switch {
	case m.searchActive:
		segs = []hintSeg{
			{"type to filter   ↑/↓ move   ", false},
			{"tab details", true},
			{"   enter use   esc back to columns", false},
		}
	case m.provider == ProviderBookmarks:
		segs = []hintSeg{
			{"space remove from bookmarks   ", false},
			{"/ search", true},
			{"   ", false},
			{"tab details", true},
		}
	default:
		segs = []hintSeg{
			{"space ★ bookmark   ", false},
			{"b YOUR LIST", true},
			{"   ", false},
			{"/ search", true},
			{"   ", false},
			{"tab details", true},
		}
	}
	if !m.searchActive {
		if m.hScrollPossible {
			segs = append(segs, hintSeg{"   ←/→ other providers", false})
		}
		segs = append(segs, hintSeg{"   enter use   esc close", false})
	}
	hintLine := renderHint(w, segs)

	parts := []string{title}
	parts = append(parts, infoLines...)
	parts = append(parts,
		baseStyle.Width(w).Render(lipgloss.JoinVertical(lipgloss.Left, modelItems...)),
		scrollIndicator,
		hintLine,
	)
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return baseStyle.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(styles.PanelBackground()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

// connectionLine says which credential serves the CURRENT column, in the
// frame, above the rows it applies to.
func (m *modelDialogCmp) connectionLine() string {
	switch m.provider {
	case ProviderBookmarks:
		return "each row is tagged [connection] — the same model can arrive via two keys, on two quotas"
	case models.ProviderLocal:
		return "served by your configured endpoints — each row names the one that owns it"
	case models.ProviderGeminiCA:
		return "served through your Google login (Code Assist quota)"
	case models.ProviderAntigravity:
		return "served through your Google login (Antigravity quota)"
	case models.ProviderChatGPT:
		return "served through your ChatGPT login (your plan's limits, not a bill)"
	case models.ProviderCopilot:
		return "served through your GitHub Copilot login"
	}
	if fp := config.ProviderKeyFingerprint(m.provider); fp != "" {
		line := fmt.Sprintf("every request below is billed to your %s key %s", m.provider, fp)
		if m.provider == models.ProviderOpenRouter {
			line += " — the vendor name says who MADE the model, this key is who serves it"
		}
		return line
	}
	return ""
}

// connectionFor is the per-model version of connectionLine, for lists that mix
// providers (search, bookmarks) and for the detail page.
func connectionFor(mod models.Model) string {
	if ep := models.LocalEndpointFor(mod.ID); ep != "" {
		return fmt.Sprintf("your local endpoint %q", ep)
	}
	switch mod.Provider {
	case models.ProviderGeminiCA:
		return "your Google login (Code Assist quota)"
	case models.ProviderAntigravity:
		return "your Google login (Antigravity quota)"
	case models.ProviderChatGPT:
		return "your ChatGPT login (your plan's limits, not a bill)"
	case models.ProviderCopilot:
		return "your GitHub Copilot login"
	}
	if fp := config.ProviderKeyFingerprint(mod.Provider); fp != "" {
		return fmt.Sprintf("your %s key %s", mod.Provider, fp)
	}
	return fmt.Sprintf("%s (no credential on file)", mod.Provider)
}

// renderDetail is the full page for one model: everything a row could not
// hold, so the decision can be made here instead of on a vendor website that
// this audience may not be able to afford to load.
func (m *modelDialogCmp) renderDetail(w int) string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()
	mod := *m.detail

	clamp := func(s string) string {
		if r := []rune(s); len(r) > w {
			return string(r[:w-1]) + "…"
		}
		return s
	}
	norm := baseStyle.Width(w)
	muted := baseStyle.Foreground(t.TextMuted()).Width(w)

	var rows []string
	rows = append(rows, baseStyle.Foreground(t.Primary()).Bold(true).Width(w).Render(clamp(mod.Name)))
	rows = append(rows, muted.Render(clamp("id: "+string(mod.ID))))
	rows = append(rows, norm.Render(""))

	fact := func(label, value string) {
		rows = append(rows, norm.Render(clamp(fmt.Sprintf("%-13s %s", label, value))))
	}
	// The connection leads: it is the fact the list could not show, and the
	// one that decides whose quota the next million tokens land on.
	fact("served via", connectionFor(mod))
	if i := strings.Index(mod.APIModel, "/"); i > 0 {
		fact("made by", mod.APIModel[:i]+" (the vendor — they do not bill you here)")
	}
	price := "FREE"
	if mod.CostPer1MIn != 0 || mod.CostPer1MOut != 0 {
		price = fmt.Sprintf("$%.2f in / $%.2f out per 1M tokens", mod.CostPer1MIn, mod.CostPer1MOut)
		if mod.CostPer1MInCached > 0 {
			price += fmt.Sprintf(", cached reads $%.2f", mod.CostPer1MInCached)
		}
	}
	fact("price", price)
	if mod.ContextWindow > 0 {
		fact("context", fmt.Sprintf("%dK tokens, max output %dK", mod.ContextWindow/1000, mod.DefaultMaxTokens/1000))
	}
	caps := []string{"tool calls"}
	if mod.CanReason {
		caps = append(caps, "extended reasoning")
	}
	if mod.SupportsAttachments {
		caps = append(caps, "image input")
	}
	fact("capabilities", strings.Join(caps, ", "))
	if mod.Rank > 0 {
		fact("rank", fmt.Sprintf("#%d on this provider's ranked list (1 = best)", mod.Rank))
	}
	if config.IsBookmarked(string(mod.ID)) {
		fact("bookmarked", "★ yes — space removes it")
	} else {
		fact("bookmarked", "no — space adds it")
	}

	// The long text, wrapped. Cut to the window with an honest marker — a
	// silently missing tail reads as the whole text.
	body := mod.Description
	if mod.Detail != "" {
		body += "\n\n" + mod.Detail
	}
	// Models truncated at the source carry their own apology inside Detail
	// (see DetailForPicker) — the data says whose cut it is, so nothing needs
	// guessing here at render time.
	if strings.TrimSpace(body) != "" {
		rows = append(rows, norm.Render(""))
		// frame chrome: padding+border+hint ≈ 7 rows; never render past the
		// window (a frame taller than its window is the marching-footer bug).
		budget := m.height - len(rows) - 8
		if budget < 4 {
			budget = 4
		}
		wrapped := strings.Split(lipgloss.NewStyle().Width(w).Render(body), "\n")
		if len(wrapped) > budget {
			wrapped = append(wrapped[:budget], clamp("… (window too small for the rest — resize to read it all)"))
		}
		for _, ln := range wrapped {
			rows = append(rows, norm.Render(ln))
		}
	}

	// A frame taller than the window is the marching-footer bug. Even the
	// facts block alone can exceed a tiny terminal, so the whole page is
	// cut to fit — with a marker, never silently.
	if maxRows := m.height - 7; m.height > 0 && len(rows) > maxRows {
		if maxRows < 3 {
			maxRows = 3
		}
		rows = append(rows[:maxRows], clamp("… (terminal too small — resize to see the rest)"))
	}

	rows = append(rows, norm.Render(""))
	rows = append(rows, renderHint(w, []hintSeg{
		{"space ★ bookmark   enter use this model   ", false},
		{"tab/esc back", true},
	}))

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return baseStyle.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(styles.PanelBackground()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

// hintSeg is one run of the hint line; hot segments render in the accent
// colour. Replaces a string-split hack that could only highlight one literal.
type hintSeg struct {
	text string
	hot  bool
}

// renderHint joins the segments, dropping whole trailing segments that would
// not fit. Never wider than the frame: an over-wide line makes bubbletea's
// erase under-reach and the footer marches down the screen (see clampToWidth).
// GORILLA OVERRIDE: the hot colour is t.Accent(), a lipgloss.AdaptiveColor, so
// it resolves separately for light and dark terminals rather than being a
// fixed value that vanishes on one of them.
func renderHint(w int, segs []hintSeg) string {
	t := theme.CurrentTheme()
	hot := lipgloss.NewStyle().Foreground(t.Accent()).Bold(true)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted())
	var parts []string
	used := 0
	for _, s := range segs {
		n := len([]rune(s.text))
		if used+n > w {
			break
		}
		used += n
		if s.hot {
			parts = append(parts, hot.Render(s.text))
		} else {
			parts = append(parts, muted.Render(s.text))
		}
	}
	return styles.BaseStyle().Width(w).Render(strings.Join(parts, ""))
}

func (m *modelDialogCmp) getScrollIndicators(maxWidth int) string {
	var indicator string

	if len(m.models) > m.visibleRows() {
		if m.scrollOffset > 0 {
			indicator += "↑ "
		}
		if m.scrollOffset+m.visibleRows() < len(m.models) {
			indicator += "↓ "
		}
	}

	if m.hScrollPossible {
		if m.hScrollOffset > 0 {
			indicator = "← " + indicator
		}
		if m.hScrollOffset < len(m.availableProviders)-1 {
			indicator += "→"
		}
	}

	// GORILLA OVERRIDE: always show "position/total" so the user knows
	// where they are in a long list and when they've reached the end,
	// instead of an unbounded scroll with no reference point.
	pos := fmt.Sprintf("%d/%d", m.selectedIdx+1, len(m.models))
	if len(m.models) == 0 {
		pos = "0/0" // an empty search result is position nowhere, not 1 of 0
	}
	if indicator != "" {
		indicator = pos + "  " + indicator
	} else {
		indicator = pos
	}

	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	return baseStyle.
		Foreground(t.Primary()).
		Width(maxWidth).
		Align(lipgloss.Right).
		Bold(true).
		Render(indicator)
}

func (m *modelDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(modelKeys)
}

func (m *modelDialogCmp) setupModels() {
	cfg := config.Get()
	modelInfo := GetSelectedModel(cfg)
	m.availableProviders = getEnabledProviders(cfg)
	m.hScrollPossible = len(m.availableProviders) > 1
	// This object is reused between opens: a search or detail page left on
	// screen at the last close must not be what the next open shows.
	m.searchActive = false
	m.query = ""
	m.searchDomain = nil
	m.detail = nil

	m.provider = modelInfo.Provider
	m.hScrollOffset = findProviderIndex(m.availableProviders, m.provider)

	m.setupModelsForProvider(m.provider)
}

func GetSelectedModel(cfg *config.Config) models.Model {

	agentCfg := cfg.Agents[config.AgentCoder]
	selectedModelId := agentCfg.Model
	return models.SupportedModels[selectedModelId]
}

func getEnabledProviders(cfg *config.Config) []models.ModelProvider {
	seen := make(map[models.ModelProvider]bool)
	var providers []models.ModelProvider

	// GORILLA OVERRIDE: the shortlist leads, and only when it has something in
	// it. Showing an empty column to someone who has never bookmarked anything
	// would put a blank screen between them and the models - the opposite of
	// the problem it is here to solve. With no bookmarks the picker behaves
	// exactly as before, opening on the curated ranked list.
	if len(config.BookmarkedModels()) > 0 {
		providers = append(providers, ProviderBookmarks)
		seen[ProviderBookmarks] = true
	}

	// Providers saved in config (added via /connect, or backfilled from env
	// at Load time).
	for providerId, provider := range cfg.Providers {
		if !provider.Disabled {
			providers = append(providers, providerId)
			seen[providerId] = true
		}
	}

	// GORILLA OVERRIDE: also include providers whose API key is present in
	// the environment but never persisted to cfg.Providers. Without this,
	// exporting GROQ_API_KEY (or any other *_API_KEY) leaves the provider
	// invisible in /model until the user goes through /connect → save — a
	// step that only exists to write a file the user did not ask to write.
	//
	// A provider EXPLICITLY disabled in cfg wins over its env var — the seen
	// map has already recorded the disabled entry above, so a matching env
	// var below is skipped rather than overriding the user's choice.
	for _, p := range config.AvailableViaEnv() {
		if seen[p] {
			continue
		}
		if entry, ok := cfg.Providers[p]; ok && entry.Disabled {
			continue // user explicitly disabled it; do not resurrect via env
		}
		providers = append(providers, p)
		seen[p] = true
	}

	// GORILLA OVERRIDE: local (OpenAI-compatible) endpoints are configured as
	// localEndpoints entries with their own per-endpoint key, so ProviderLocal
	// appears in neither cfg.Providers nor the *_API_KEY environment scan above.
	// Without this it was never listed, and every model discovered from NVIDIA
	// NIM, Ollama or LM Studio was unselectable — 102 working NVIDIA models
	// registered and none reachable, which is indistinguishable from the key
	// having been refused.
	if !seen[models.ProviderLocal] && models.HasLocalModels() {
		if entry, ok := cfg.Providers[models.ProviderLocal]; !ok || !entry.Disabled {
			providers = append(providers, models.ProviderLocal)
			seen[models.ProviderLocal] = true
		}
	}

	// Sort by provider popularity
	slices.SortFunc(providers, func(a, b models.ModelProvider) int {
		// GORILLA FIX (2026-08-09): the shortlist column must lead, and this
		// sort was quietly sending it to the back. It is not a real provider so
		// it has no ProviderPopularity entry, and unranked providers default to
		// 999 = "show last" — so bookmarks landed at the far right of the
		// carousel, several presses away, and looked like it had not been
		// created at all. Prepending it above was not enough; the sort runs
		// afterwards and does not care where a thing started.
		if a == ProviderBookmarks && b != ProviderBookmarks {
			return -1
		}
		if b == ProviderBookmarks && a != ProviderBookmarks {
			return 1
		}

		rA := models.ProviderPopularity[a]
		rB := models.ProviderPopularity[b]

		// models not included in popularity ranking default to last
		if rA == 0 {
			rA = 999
		}
		if rB == 0 {
			rB = 999
		}
		return rA - rB
	})
	return providers
}

// findProviderIndex returns the index of the provider in the list, or -1 if not found
func findProviderIndex(providers []models.ModelProvider, provider models.ModelProvider) int {
	for i, p := range providers {
		if p == provider {
			return i
		}
	}
	return -1
}

func (m *modelDialogCmp) setupModelsForProvider(provider models.ModelProvider) {
	cfg := config.Get()
	agentCfg := cfg.Agents[config.AgentCoder]
	selectedModelId := agentCfg.Model

	m.provider = provider
	if provider == ProviderBookmarks {
		m.models = bookmarkedModels()
	} else {
		m.models = getModelsForProvider(provider)
	}
	m.selectedIdx = 0
	m.scrollOffset = 0

	// Try to select the current model if it belongs to this provider
	if provider == models.SupportedModels[selectedModelId].Provider {
		for i, model := range m.models {
			if model.ID == selectedModelId {
				m.selectedIdx = i
				// Adjust scroll position to keep selected model visible
				if m.selectedIdx >= m.visibleRows() {
					m.scrollOffset = m.selectedIdx - (m.visibleRows() - 1)
				}
				break
			}
		}
	}
}

// GORILLA OVERRIDE: a virtual provider holding the user's personal shortlist.
//
// It is not a real backend - it is a view across every provider, pinned to the
// front of the carousel so it is the first thing anyone sees. That placement is
// the whole point: the catalogue is now hundreds of models with names like
// "inclusionai/ling-3.0-tiny:free", and for someone who just wants to learn to
// code, that is not a choice, it is a wall. Worse, working out what those names
// mean would take a web search and a heavy vendor page PER MODEL - which on a
// single-digit-KB/s line is not slow, it is impossible.
//
// So: decide once, from the curated descriptions already here, and never scroll
// the catalogue again.
const ProviderBookmarks models.ModelProvider = "★ bookmarks"

// bookmarkedModels resolves the saved ids. Entries that no longer resolve are
// kept and marked unavailable rather than dropped: a bookmark disappearing with
// no explanation is exactly the silent failure this project keeps finding.
func bookmarkedModels() []models.Model {
	var out []models.Model
	for _, id := range config.BookmarkedModels() {
		if m, ok := models.SupportedModels[models.ModelID(id)]; ok {
			// The rank belongs to the model's home provider and means nothing
			// here: the shortlist showed "10, 2, 3, 5, 6, 10 …" with two entries
			// numbered 10, which reads as a broken sort. Cleared, so the list
			// shows the only order the user authored — the one they added them
			// in — and the "N ranked best-first" subtitle stops claiming a
			// ranking that is not there.
			m.Rank = 0
			out = append(out, m)
			continue
		}
		out = append(out, models.Model{
			ID:          models.ModelID(id),
			Name:        string(id),
			Description: "UNAVAILABLE — this model is no longer offered; press space to remove it",
			Provider:    ProviderBookmarks,
		})
	}
	return out
}

func getModelsForProvider(provider models.ModelProvider) []models.Model {
	var providerModels []models.Model
	for _, model := range models.SupportedModels {
		if model.Provider == provider {
			providerModels = append(providerModels, model)
		}
	}

	// Coding-usefulness heuristic order — used for unranked models and for
	// providers that have no curated ranking at all.
	byCoding := func(a, b models.Model) int {
		ra, rb := codingRank(string(a.ID)), codingRank(string(b.ID))
		if ra != rb {
			return ra - rb
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	}

	// GORILLA OVERRIDE: show EVERY model the provider offers — the curated,
	// probe-verified best ones first (Rank 1..N), then everything else below.
	// The ranking is guidance, not a gate: someone else may legitimately want
	// a smaller/older/"shit tier" model, and it's not our place to hide it.
	// Providers without ranks (e.g. Gemini) fall through to coding order.
	var ranked, rest []models.Model
	for _, m := range providerModels {
		if m.Rank > 0 {
			ranked = append(ranked, m)
		} else {
			rest = append(rest, m)
		}
	}
	slices.SortFunc(ranked, func(a, b models.Model) int { return a.Rank - b.Rank })
	slices.SortFunc(rest, byCoding)
	if len(ranked) > 0 {
		return append(ranked, rest...)
	}
	return rest
}

// codingRank scores a model id by coding usefulness (lower = better).
// It matches on substrings of the raw model id so it works for any
// provider's discovered models, not a hardcoded list.
func codingRank(id string) int {
	s := strings.ToLower(id)
	has := func(subs ...string) bool {
		for _, sub := range subs {
			if strings.Contains(s, sub) {
				return true
			}
		}
		return false
	}
	// Bottom: not generative coding models at all.
	if has("embed", "rerank", "guard", "safety", "content-safety", "moderation",
		"deplot", "cosmos", "gliner", "parse", "video", "vision", "-vl-", "vlm",
		"diffusion", "tts", "-image", "ocr", "riva", "nvclip", "neva", "fuyu", "kosmos") {
		return 90
	}
	// Tier 1: current flagship coders.
	if has("deepseek-v4-pro", "deepseek-v4.1", "glm-5", "kimi-k2", "minimax-m3",
		"qwen3.5", "nemotron-3-ultra", "nemotron-3-super", "mistral-large-3") {
		return 10
	}
	// Tier 2: strong / fast current models.
	if has("deepseek-v4", "deepseek", "glm", "qwen3", "qwen", "minimax",
		"nemotron-3", "llama-4", "mistral-large", "codestral", "starcoder", "codellama") {
		return 20
	}
	// Tier 3: older but capable general models.
	if has("llama-3", "mixtral", "mistral", "nemotron", "granite", "gemma-4", "gpt-oss") {
		return 40
	}
	// Everything else in the middle-bottom.
	return 60
}

// providerDisplayName renders a friendly provider title for the picker.
// GORILLA OVERRIDE: the raw id "gemini-oauth" is capitalized to an ugly
// "Gemini-oauth"; show what it actually is instead.
func providerDisplayName(p models.ModelProvider) string {
	switch p {
	case models.ProviderGeminiCA:
		return "Gemini (Google login)"
	case models.ProviderAntigravity:
		return "Antigravity (Google login — Claude/GPT/Gemini)"
	case models.ProviderChatGPT:
		return "ChatGPT (OpenAI login — free plan works)"
	case ProviderBookmarks:
		// "★ bookmarks" was accurate and told nobody anything. Someone landing
		// here has to work out that this is THEIR list, assembled by them —
		// which is exactly the reading-comprehension tax this list exists to
		// remove.
		return "★ YOUR BOOKMARKS — the models you picked"
	}
	s := string(p)
	if s == "" {
		return s
	}
	// GORILLA FIX (2026-08-09): this was strings.ToUpper(s[:1]) + s[1:], which
	// slices the first BYTE, not the first rune. Any provider name starting
	// with a multi-byte character was cut into invalid UTF-8 fragments and
	// rendered as one replacement glyph per byte - "★ bookmarks" came out as
	// "◆◆◆ bookmarks", three diamonds for one star. Latent since the function
	// was written; the bookmarks column was simply the first name to trigger it.
	r := []rune(s)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}

func NewModelDialogCmp() ModelDialog {
	return &modelDialogCmp{}
}
