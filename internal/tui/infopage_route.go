// GORILLA (2026-10-10): /editor, /helpers and /hooks open an explanation page.
//
// The pages are built here, at the moment the command is typed, not when the
// program starts: /hooks lists the hooks that are loaded and /helpers states
// the helper limit currently set in /context, and a page built earlier would
// show whatever was true then.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/helppages"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// openInfoPage opens the page a command name (or alias) reaches.
func (a *appModel) openInfoPage(name string) tea.Cmd {
	p, ok := helppages.ByName(name)
	if !ok {
		// Unreachable from the dispatch switch, which lists only names that
		// have a page; said plainly rather than opening an empty frame.
		return util.ReportWarn("/" + name + " has no page to show.")
	}
	a.infoPage = dialog.NewInfoPageCmp(p)
	a.infoPage.SetSize(a.width, a.height)
	a.showInfoPage = true
	return nil
}
