package dialog

import (
	"fmt"
	"slices"
	"strings"

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
		case key.Matches(msg, modelKeys.Enter):
			util.ReportInfo(fmt.Sprintf("selected model: %s", m.models[m.selectedIdx].Name))
			return m, util.CmdHandler(ModelSelectedMsg{Model: m.models[m.selectedIdx]})
		case key.Matches(msg, modelKeys.Escape):
			return m, util.CmdHandler(CloseModelDialogMsg{})
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// moveSelectionUp moves the selection up or wraps to bottom
func (m *modelDialogCmp) moveSelectionUp() {
	if m.selectedIdx > 0 {
		m.selectedIdx--
	} else {
		m.selectedIdx = len(m.models) - 1
		m.scrollOffset = max(0, len(m.models)-numVisibleModels)
	}

	// Keep selection visible
	if m.selectedIdx < m.scrollOffset {
		m.scrollOffset = m.selectedIdx
	}
}

// moveSelectionDown moves the selection down or wraps to top
func (m *modelDialogCmp) moveSelectionDown() {
	if m.selectedIdx < len(m.models)-1 {
		m.selectedIdx++
	} else {
		m.selectedIdx = 0
		m.scrollOffset = 0
	}

	// Keep selection visible
	if m.selectedIdx >= m.scrollOffset+numVisibleModels {
		m.scrollOffset = m.selectedIdx - (numVisibleModels - 1)
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

func (m *modelDialogCmp) View() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	// Capitalize first letter of provider name
	providerName := strings.ToUpper(string(m.provider)[:1]) + string(m.provider[1:])
	title := baseStyle.
		Foreground(t.Primary()).
		Bold(true).
		Width(maxDialogWidth).
		Padding(0, 0, 1).
		Render(fmt.Sprintf("Select %s Model", providerName))

	// Render visible models
	endIdx := min(m.scrollOffset+numVisibleModels, len(m.models))
	modelItems := make([]string, 0, endIdx-m.scrollOffset)

	for i := m.scrollOffset; i < endIdx; i++ {
		// GORILLA OVERRIDE: show "Name — description" so 100+ discovered
		// models are distinguishable; truncate to the dialog width.
		label := m.models[i].Name
		if d := m.models[i].Description; d != "" {
			label = fmt.Sprintf("%s — %s", m.models[i].Name, d)
		}
		if r := []rune(label); len(r) > maxDialogWidth-1 {
			label = string(r[:maxDialogWidth-2]) + "…"
		}
		itemStyle := baseStyle.Width(maxDialogWidth)
		if i == m.selectedIdx {
			itemStyle = itemStyle.Background(t.Primary()).
				Foreground(t.Background()).Bold(true)
		}
		modelItems = append(modelItems, itemStyle.Render(label))
	}

	scrollIndicator := m.getScrollIndicators(maxDialogWidth)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		baseStyle.Width(maxDialogWidth).Render(lipgloss.JoinVertical(lipgloss.Left, modelItems...)),
		scrollIndicator,
	)

	return baseStyle.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func (m *modelDialogCmp) getScrollIndicators(maxWidth int) string {
	var indicator string

	if len(m.models) > numVisibleModels {
		if m.scrollOffset > 0 {
			indicator += "↑ "
		}
		if m.scrollOffset+numVisibleModels < len(m.models) {
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
	var providers []models.ModelProvider
	for providerId, provider := range cfg.Providers {
		if !provider.Disabled {
			providers = append(providers, providerId)
		}
	}

	// Sort by provider popularity
	slices.SortFunc(providers, func(a, b models.ModelProvider) int {
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
	m.models = getModelsForProvider(provider)
	m.selectedIdx = 0
	m.scrollOffset = 0

	// Try to select the current model if it belongs to this provider
	if provider == models.SupportedModels[selectedModelId].Provider {
		for i, model := range m.models {
			if model.ID == selectedModelId {
				m.selectedIdx = i
				// Adjust scroll position to keep selected model visible
				if m.selectedIdx >= numVisibleModels {
					m.scrollOffset = m.selectedIdx - (numVisibleModels - 1)
				}
				break
			}
		}
	}
}

func getModelsForProvider(provider models.ModelProvider) []models.Model {
	var providerModels []models.Model
	for _, model := range models.SupportedModels {
		if model.Provider == provider {
			providerModels = append(providerModels, model)
		}
	}

	// GORILLA OVERRIDE: rank by coding usefulness so the strongest
	// models sit at the top, instead of the old reverse-alphabetical
	// order that floated junk (Diffusiongemma, Deplot, Cosmos) above
	// DeepSeek V4 Pro. Ties fall back to alphabetical.
	slices.SortFunc(providerModels, func(a, b models.Model) int {
		ra, rb := codingRank(string(a.ID)), codingRank(string(b.ID))
		if ra != rb {
			return ra - rb
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return providerModels
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

func NewModelDialogCmp() ModelDialog {
	return &modelDialogCmp{}
}
