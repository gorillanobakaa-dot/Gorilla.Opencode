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
	// title + subtitle + scroll indicator + hint + padding + border + margin
	const chrome = 12
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
			if len(m.models) == 0 {
				break
			}
			id := string(m.models[m.selectedIdx].ID)
			on, err := config.ToggleBookmark(id)
			if err != nil {
				return m, util.ReportError(err)
			}
			// Rebuild: the shortlist column may have just appeared or emptied,
			// and leaving the carousel describing a state that no longer exists
			// is how a menu starts lying about itself.
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
			if on {
				util.ReportInfo("bookmarked — press b to see your list")
			} else {
				util.ReportInfo("removed from bookmarks")
			}
			return m, nil
		case key.Matches(msg, modelKeys.Enter):
			if len(m.models) == 0 {
				break
			}
			chosen := m.models[m.selectedIdx]
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
			return m, util.CmdHandler(ModelSelectedMsg{Model: chosen})
		case key.Matches(msg, modelKeys.Escape):
			return m, util.CmdHandler(CloseModelDialogMsg{})
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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

	// Capitalize first letter of provider name (with friendly overrides)
	providerName := providerDisplayName(m.provider)
	title := baseStyle.
		Foreground(t.Primary()).
		Bold(true).
		Width(w).
		Render(fmt.Sprintf("Select %s Model", providerName))

	// GORILLA OVERRIDE: for a curated provider, tell the user the top of the
	// list is the probe-verified coding ranking (1 = best), and that the
	// rest of the provider's catalog follows below — nothing is hidden.
	subtitle := ""
	if len(m.models) > 0 && m.models[0].Rank > 0 {
		ranked := 0
		for _, mm := range m.models {
			if mm.Rank > 0 {
				ranked++
			}
		}
		subtitle = baseStyle.Foreground(t.TextMuted()).Width(w).Padding(0, 0, 1).
			Render(fmt.Sprintf("%d ranked best-first (1=best); %d more below — full catalog, your call", ranked, len(m.models)-ranked))
	} else {
		title = baseStyle.Foreground(t.Primary()).Bold(true).Width(w).Padding(0, 0, 1).
			Render(fmt.Sprintf("Select %s Model", providerName))
	}

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
		} else if m.provider == ProviderBookmarks {
			// GORILLA OVERRIDE: name the provider inside the shortlist.
			//
			// It is the one list that mixes them, so "Gemini 3.6 Flash" can
			// appear twice — once via Antigravity, once via a Gemini API key —
			// and with nothing to tell them apart they read as duplicates. The
			// reported instinct was to delete one, which would silently remove
			// a different route to the same model, on a different quota.
			// Local models already carry their endpoint name from the branch
			// above; everything else needs its provider.
			if home := models.SupportedModels[m.models[i].ID].Provider; home != "" {
				label = fmt.Sprintf("[%s] %s", home, label)
			}
		}
		if r := m.models[i].Rank; r > 0 {
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
	hint := "space ★ bookmark   b YOUR LIST"
	if m.provider == ProviderBookmarks {
		hint = "space remove from bookmarks"
	}
	if m.hScrollPossible {
		hint += "   ←/→ other providers"
	}
	hint += "   enter use   esc close"
	// Never wider than the frame: an over-wide line makes bubbletea's erase
	// under-reach and the footer marches down the screen (see clampToWidth).
	if r := []rune(hint); len(r) > w {
		hint = string(r[:w])
	}
	// GORILLA OVERRIDE: "b" is the only way to reach the shortlist from a
	// distant column, so it must not be the same grey as the rest of the hint.
	// t.Accent() is a lipgloss.AdaptiveColor, so it resolves separately for
	// light and dark terminals rather than being a fixed value that vanishes
	// on one of them.
	key := lipgloss.NewStyle().Foreground(t.Accent()).Bold(true)
	muted := lipgloss.NewStyle().Foreground(t.TextMuted())
	hintLine := baseStyle.Width(w).Render(
		muted.Render(strings.Split(hint, "b YOUR LIST")[0]) +
			func() string {
				if strings.Contains(hint, "b YOUR LIST") {
					return key.Render("b YOUR LIST")
				}
				return ""
			}() +
			muted.Render(func() string {
				parts := strings.SplitN(hint, "b YOUR LIST", 2)
				if len(parts) == 2 {
					return parts[1]
				}
				return ""
			}()),
	)

	parts := []string{title}
	if subtitle != "" {
		parts = append(parts, subtitle)
	}
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
