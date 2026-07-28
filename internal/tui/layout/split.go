package layout

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/tui/styles"
)

type SplitPaneLayout interface {
	tea.Model
	Sizeable
	Bindings
	SetLeftPanel(panel Container) tea.Cmd
	SetRightPanel(panel Container) tea.Cmd
	SetBottomPanel(panel Container) tea.Cmd

	// SetBottomHeight pins the bottom panel to an exact row count instead of
	// deriving it from verticalRatio, so a growing editor can claim rows from
	// the panel above it. Pass 0 to go back to the ratio.
	SetBottomHeight(rows int) tea.Cmd

	ClearLeftPanel() tea.Cmd
	ClearRightPanel() tea.Cmd
	ClearBottomPanel() tea.Cmd
}

type splitPaneLayout struct {
	width         int
	height        int
	ratio         float64
	verticalRatio float64

	// bottomFixedHeight, when > 0, pins the bottom panel to that many rows
	// (see SetBottomHeight) instead of using verticalRatio.
	bottomFixedHeight int

	// Resolved panel widths from the last SetSize, so View() can clamp each
	// column to exactly its allotment (a child that renders wider/taller than
	// allotted would otherwise shift or clip its neighbour).
	leftWidth  int
	rightWidth int

	rightPanel  Container
	leftPanel   Container
	bottomPanel Container
}

type SplitPaneOption func(*splitPaneLayout)

func (s *splitPaneLayout) Init() tea.Cmd {
	var cmds []tea.Cmd

	if s.leftPanel != nil {
		cmds = append(cmds, s.leftPanel.Init())
	}

	if s.rightPanel != nil {
		cmds = append(cmds, s.rightPanel.Init())
	}

	if s.bottomPanel != nil {
		cmds = append(cmds, s.bottomPanel.Init())
	}

	return tea.Batch(cmds...)
}

func (s *splitPaneLayout) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s, s.SetSize(msg.Width, msg.Height)
	}

	if s.rightPanel != nil {
		u, cmd := s.rightPanel.Update(msg)
		s.rightPanel = u.(Container)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if s.leftPanel != nil {
		u, cmd := s.leftPanel.Update(msg)
		s.leftPanel = u.(Container)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if s.bottomPanel != nil {
		u, cmd := s.bottomPanel.Update(msg)
		s.bottomPanel = u.(Container)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return s, tea.Batch(cmds...)
}

func (s *splitPaneLayout) View() string {
	// GORILLA OVERRIDE: compose as [ messages-over-editor | sidebar ] so the
	// sidebar (right panel) spans the FULL height down to the status bar,
	// instead of stopping above a full-width editor band.
	leftColumn := ""
	if s.leftPanel != nil {
		leftColumn = s.leftPanel.View()
	}
	if s.bottomPanel != nil {
		bottomView := s.bottomPanel.View()
		if leftColumn != "" {
			leftColumn = lipgloss.JoinVertical(lipgloss.Left, leftColumn, bottomView)
		} else {
			leftColumn = bottomView
		}
	}

	// Clamp each column to EXACTLY its allotted box before joining. A child that
	// renders wider or taller than allotted (e.g. a growing textarea) would
	// otherwise shift its neighbour sideways and get clipped by the outer
	// Width(), leaving unpainted gaps. MaxWidth/MaxHeight make that impossible.
	clamp := func(v string, w, h int) string {
		if w <= 0 || h <= 0 {
			return v
		}
		return lipgloss.NewStyle().
			Width(w).Height(h).
			MaxWidth(w).MaxHeight(h).
			Render(v)
	}

	var finalView string
	if s.rightPanel != nil {
		rightView := s.rightPanel.View()
		if leftColumn != "" {
			finalView = lipgloss.JoinHorizontal(lipgloss.Top,
				clamp(leftColumn, s.leftWidth, s.height),
				clamp(rightView, s.rightWidth, s.height),
			)
		} else {
			finalView = rightView
		}
	} else {
		finalView = leftColumn
	}

	if finalView != "" {
		style := lipgloss.NewStyle().
			Width(s.width).
			Height(s.height).
			Background(styles.PanelBackground())

		return style.Render(finalView)
	}

	return finalView
}

func (s *splitPaneLayout) SetSize(width, height int) tea.Cmd {
	s.width = width
	s.height = height

	var topHeight, bottomHeight int
	if s.bottomPanel != nil {
		if s.bottomFixedHeight > 0 {
			// Pinned: the bottom panel takes exactly the rows it asked for,
			// leaving at least one row for the panel above it.
			bottomHeight = s.bottomFixedHeight
			if bottomHeight > height-1 {
				bottomHeight = max(1, height-1)
			}
			topHeight = height - bottomHeight
		} else {
			topHeight = int(float64(height) * s.verticalRatio)
			bottomHeight = height - topHeight
		}
	} else {
		topHeight = height
		bottomHeight = 0
	}

	var leftWidth, rightWidth int
	if s.leftPanel != nil && s.rightPanel != nil {
		leftWidth = int(float64(width) * s.ratio)
		rightWidth = width - leftWidth
		// GORILLA OVERRIDE: the sidebar (cwd/session/LSP/modified files) needs
		// only a modest width; a fixed 30% share wastes a lot of columns on a
		// wide terminal. Cap it so the extra width goes to the chat instead.
		const maxRightPanelWidth = 35
		if rightWidth > maxRightPanelWidth {
			rightWidth = maxRightPanelWidth
			leftWidth = width - rightWidth
		}
	} else if s.leftPanel != nil {
		leftWidth = width
		rightWidth = 0
	} else if s.rightPanel != nil {
		leftWidth = 0
		rightWidth = width
	}

	// GORILLA OVERRIDE: the sidebar spans the full height beside the messages+
	// editor column; the editor (bottom) is only as wide as the messages.
	bottomWidth := width
	if s.rightPanel != nil {
		bottomWidth = leftWidth
	}
	s.leftWidth, s.rightWidth = leftWidth, rightWidth

	var cmds []tea.Cmd
	if s.leftPanel != nil {
		cmd := s.leftPanel.SetSize(leftWidth, topHeight)
		cmds = append(cmds, cmd)
	}

	if s.rightPanel != nil {
		cmd := s.rightPanel.SetSize(rightWidth, height)
		cmds = append(cmds, cmd)
	}

	if s.bottomPanel != nil {
		cmd := s.bottomPanel.SetSize(bottomWidth, bottomHeight)
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (s *splitPaneLayout) GetSize() (int, int) {
	return s.width, s.height
}

func (s *splitPaneLayout) SetLeftPanel(panel Container) tea.Cmd {
	s.leftPanel = panel
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) SetRightPanel(panel Container) tea.Cmd {
	s.rightPanel = panel
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) SetBottomHeight(rows int) tea.Cmd {
	if rows < 0 {
		rows = 0
	}
	if s.bottomFixedHeight == rows {
		return nil // no-op: avoid a needless resize storm on every keystroke
	}
	s.bottomFixedHeight = rows
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) SetBottomPanel(panel Container) tea.Cmd {
	s.bottomPanel = panel
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) ClearLeftPanel() tea.Cmd {
	s.leftPanel = nil
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) ClearRightPanel() tea.Cmd {
	s.rightPanel = nil
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) ClearBottomPanel() tea.Cmd {
	s.bottomPanel = nil
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) BindingKeys() []key.Binding {
	keys := []key.Binding{}
	if s.leftPanel != nil {
		if b, ok := s.leftPanel.(Bindings); ok {
			keys = append(keys, b.BindingKeys()...)
		}
	}
	if s.rightPanel != nil {
		if b, ok := s.rightPanel.(Bindings); ok {
			keys = append(keys, b.BindingKeys()...)
		}
	}
	if s.bottomPanel != nil {
		if b, ok := s.bottomPanel.(Bindings); ok {
			keys = append(keys, b.BindingKeys()...)
		}
	}
	return keys
}

func NewSplitPane(options ...SplitPaneOption) SplitPaneLayout {

	layout := &splitPaneLayout{
		ratio:         0.7,
		verticalRatio: 0.9, // Default 90% for top section, 10% for bottom
	}
	for _, option := range options {
		option(layout)
	}
	return layout
}

func WithLeftPanel(panel Container) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.leftPanel = panel
	}
}

func WithRightPanel(panel Container) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.rightPanel = panel
	}
}

func WithRatio(ratio float64) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.ratio = ratio
	}
}

func WithBottomPanel(panel Container) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.bottomPanel = panel
	}
}

func WithVerticalRatio(ratio float64) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.verticalRatio = ratio
	}
}
