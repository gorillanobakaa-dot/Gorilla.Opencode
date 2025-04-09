package page

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kujtimiihoxha/termai/internal/tui/components/logs"
	"github.com/kujtimiihoxha/termai/internal/tui/layout"
)

var LogsPage PageID = "logs"

func NewLogsPage() tea.Model {
	return layout.NewBentoLayout(
		layout.BentoPanes{
			layout.BentoRightTopPane:    logs.NewLogsTable(),
			layout.BentoRightBottomPane: logs.NewLogsDetails(),
		},
		layout.WithBentoLayoutCurrentPane(layout.BentoRightTopPane),
		layout.WithBentoLayoutRightTopHeightRatio(0.5),
	)
}
