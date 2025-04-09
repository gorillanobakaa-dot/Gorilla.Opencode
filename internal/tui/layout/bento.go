package layout

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type paneID string

const (
	BentoLeftPane        paneID = "left"
	BentoRightTopPane    paneID = "right-top"
	BentoRightBottomPane paneID = "right-bottom"
)

type BentoPanes map[paneID]tea.Model

const (
	defaultLeftWidthRatio      = 0.2
	defaultRightTopHeightRatio = 0.85

	minLeftWidth         = 10
	minRightBottomHeight = 10
)

type BentoLayout interface {
	tea.Model
	Sizeable
	Bindings
}

type BentoKeyBindings struct {
	SwitchPane      key.Binding
	SwitchPaneBack  key.Binding
	HideCurrentPane key.Binding
	ShowAllPanes    key.Binding
}

var defaultBentoKeyBindings = BentoKeyBindings{
	SwitchPane: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch pane"),
	),
	SwitchPaneBack: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "switch pane back"),
	),
	HideCurrentPane: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "hide current pane"),
	),
	ShowAllPanes: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "show all panes"),
	),
}

type bentoLayout struct {
	width  int
	height int

	leftWidthRatio      float64
	rightTopHeightRatio float64

	currentPane paneID
	panes       map[paneID]SinglePaneLayout
	hiddenPanes map[paneID]bool
}

func (b *bentoLayout) GetSize() (int, int) {
	return b.width, b.height
}

func (b *bentoLayout) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, pane := range b.panes {
		cmd := pane.Init()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (b *bentoLayout) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.SetSize(msg.Width, msg.Height)
		return b, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultBentoKeyBindings.SwitchPane):
			return b, b.SwitchPane(false)
		case key.Matches(msg, defaultBentoKeyBindings.SwitchPaneBack):
			return b, b.SwitchPane(true)
		case key.Matches(msg, defaultBentoKeyBindings.HideCurrentPane):
			return b, b.HidePane(b.currentPane)
		case key.Matches(msg, defaultBentoKeyBindings.ShowAllPanes):
			for id := range b.hiddenPanes {
				delete(b.hiddenPanes, id)
			}
			b.SetSize(b.width, b.height)
			return b, nil
		}
	}

	var cmds []tea.Cmd
	for id, pane := range b.panes {
		u, cmd := pane.Update(msg)
		b.panes[id] = u.(SinglePaneLayout)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return b, tea.Batch(cmds...)
}

func (b *bentoLayout) View() string {
	if b.width <= 0 || b.height <= 0 {
		return ""
	}

	for id, pane := range b.panes {
		if b.currentPane == id {
			pane.Focus()
		} else {
			pane.Blur()
		}
	}

	leftVisible := false
	rightTopVisible := false
	rightBottomVisible := false

	var leftPane, rightTopPane, rightBottomPane string

	if pane, ok := b.panes[BentoLeftPane]; ok && !b.hiddenPanes[BentoLeftPane] {
		leftPane = pane.View()
		leftVisible = true
	}

	if pane, ok := b.panes[BentoRightTopPane]; ok && !b.hiddenPanes[BentoRightTopPane] {
		rightTopPane = pane.View()
		rightTopVisible = true
	}

	if pane, ok := b.panes[BentoRightBottomPane]; ok && !b.hiddenPanes[BentoRightBottomPane] {
		rightBottomPane = pane.View()
		rightBottomVisible = true
	}

	if leftVisible {
		if rightTopVisible || rightBottomVisible {
			rightSection := ""
			if rightTopVisible && rightBottomVisible {
				rightSection = lipgloss.JoinVertical(lipgloss.Top, rightTopPane, rightBottomPane)
			} else if rightTopVisible {
				rightSection = rightTopPane
			} else {
				rightSection = rightBottomPane
			}
			return lipgloss.NewStyle().Width(b.width).Height(b.height).Render(
				lipgloss.JoinHorizontal(lipgloss.Left, leftPane, rightSection),
			)
		} else {
			return lipgloss.NewStyle().Width(b.width).Height(b.height).Render(leftPane)
		}
	} else if rightTopVisible || rightBottomVisible {
		if rightTopVisible && rightBottomVisible {
			return lipgloss.NewStyle().Width(b.width).Height(b.height).Render(
				lipgloss.JoinVertical(lipgloss.Top, rightTopPane, rightBottomPane),
			)
		} else if rightTopVisible {
			return lipgloss.NewStyle().Width(b.width).Height(b.height).Render(rightTopPane)
		} else {
			return lipgloss.NewStyle().Width(b.width).Height(b.height).Render(rightBottomPane)
		}
	}
	return ""
}

func (b *bentoLayout) SetSize(width int, height int) {
	if width < 0 || height < 0 {
		return
	}
	b.width = width
	b.height = height

	leftExists := false
	rightTopExists := false
	rightBottomExists := false

	if _, ok := b.panes[BentoLeftPane]; ok && !b.hiddenPanes[BentoLeftPane] {
		leftExists = true
	}
	if _, ok := b.panes[BentoRightTopPane]; ok && !b.hiddenPanes[BentoRightTopPane] {
		rightTopExists = true
	}
	if _, ok := b.panes[BentoRightBottomPane]; ok && !b.hiddenPanes[BentoRightBottomPane] {
		rightBottomExists = true
	}

	leftWidth := 0
	rightWidth := 0
	rightTopHeight := 0
	rightBottomHeight := 0

	if leftExists && (rightTopExists || rightBottomExists) {
		leftWidth = int(float64(width) * b.leftWidthRatio)
		if leftWidth < minLeftWidth && width >= minLeftWidth {
			leftWidth = minLeftWidth
		}
		rightWidth = width - leftWidth

		if rightTopExists && rightBottomExists {
			rightTopHeight = int(float64(height) * b.rightTopHeightRatio)
			rightBottomHeight = height - rightTopHeight

			if rightBottomHeight < minRightBottomHeight && height >= minRightBottomHeight {
				rightBottomHeight = minRightBottomHeight
				rightTopHeight = height - rightBottomHeight
			}
		} else if rightTopExists {
			rightTopHeight = height
		} else if rightBottomExists {
			rightBottomHeight = height
		}
	} else if leftExists {
		leftWidth = width
	} else if rightTopExists || rightBottomExists {
		rightWidth = width

		if rightTopExists && rightBottomExists {
			rightTopHeight = int(float64(height) * b.rightTopHeightRatio)
			rightBottomHeight = height - rightTopHeight

			if rightBottomHeight < minRightBottomHeight && height >= minRightBottomHeight {
				rightBottomHeight = minRightBottomHeight
				rightTopHeight = height - rightBottomHeight
			}
		} else if rightTopExists {
			rightTopHeight = height
		} else if rightBottomExists {
			rightBottomHeight = height
		}
	}

	if pane, ok := b.panes[BentoLeftPane]; ok && !b.hiddenPanes[BentoLeftPane] {
		pane.SetSize(leftWidth, height)
	}
	if pane, ok := b.panes[BentoRightTopPane]; ok && !b.hiddenPanes[BentoRightTopPane] {
		pane.SetSize(rightWidth, rightTopHeight)
	}
	if pane, ok := b.panes[BentoRightBottomPane]; ok && !b.hiddenPanes[BentoRightBottomPane] {
		pane.SetSize(rightWidth, rightBottomHeight)
	}
}

func (b *bentoLayout) HidePane(pane paneID) tea.Cmd {
	if len(b.panes)-len(b.hiddenPanes) == 1 {
		return nil
	}
	if _, ok := b.panes[pane]; ok {
		b.hiddenPanes[pane] = true
	}
	b.SetSize(b.width, b.height)
	return b.SwitchPane(false)
}

func (b *bentoLayout) SwitchPane(back bool) tea.Cmd {
	orderForward := []paneID{BentoLeftPane, BentoRightTopPane, BentoRightBottomPane}
	orderBackward := []paneID{BentoLeftPane, BentoRightBottomPane, BentoRightTopPane}

	order := orderForward
	if back {
		order = orderBackward
	}

	currentIdx := -1
	for i, id := range order {
		if id == b.currentPane {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		for _, id := range order {
			if _, exists := b.panes[id]; exists {
				if _, hidden := b.hiddenPanes[id]; !hidden {
					b.currentPane = id
					break
				}
			}
		}
	} else {
		startIdx := currentIdx
		for {
			currentIdx = (currentIdx + 1) % len(order)

			nextID := order[currentIdx]
			if _, exists := b.panes[nextID]; exists {
				if _, hidden := b.hiddenPanes[nextID]; !hidden {
					b.currentPane = nextID
					break
				}
			}

			if currentIdx == startIdx {
				break
			}
		}
	}

	var cmds []tea.Cmd
	for id, pane := range b.panes {
		if _, ok := b.hiddenPanes[id]; ok {
			continue
		}
		if id == b.currentPane {
			cmds = append(cmds, pane.Focus())
		} else {
			cmds = append(cmds, pane.Blur())
		}
	}

	return tea.Batch(cmds...)
}

func (s *bentoLayout) BindingKeys() []key.Binding {
	bindings := KeyMapToSlice(defaultBentoKeyBindings)
	if b, ok := s.panes[s.currentPane].(Bindings); ok {
		bindings = append(bindings, b.BindingKeys()...)
	}
	return bindings
}

type BentoLayoutOption func(*bentoLayout)

func NewBentoLayout(panes BentoPanes, opts ...BentoLayoutOption) BentoLayout {
	p := make(map[paneID]SinglePaneLayout, len(panes))
	for id, pane := range panes {
		if sp, ok := pane.(SinglePaneLayout); !ok {
			p[id] = NewSinglePane(
				pane,
				WithSinglePaneFocusable(true),
				WithSinglePaneBordered(true),
			)
		} else {
			p[id] = sp
		}
	}
	if len(p) == 0 {
		panic("no panes provided for BentoLayout")
	}
	layout := &bentoLayout{
		panes:               p,
		hiddenPanes:         make(map[paneID]bool),
		currentPane:         BentoLeftPane,
		leftWidthRatio:      defaultLeftWidthRatio,
		rightTopHeightRatio: defaultRightTopHeightRatio,
	}

	for _, opt := range opts {
		opt(layout)
	}

	return layout
}

func WithBentoLayoutLeftWidthRatio(ratio float64) BentoLayoutOption {
	return func(b *bentoLayout) {
		if ratio > 0 && ratio < 1 {
			b.leftWidthRatio = ratio
		}
	}
}

func WithBentoLayoutRightTopHeightRatio(ratio float64) BentoLayoutOption {
	return func(b *bentoLayout) {
		if ratio > 0 && ratio < 1 {
			b.rightTopHeightRatio = ratio
		}
	}
}

func WithBentoLayoutCurrentPane(pane paneID) BentoLayoutOption {
	return func(b *bentoLayout) {
		b.currentPane = pane
	}
}
