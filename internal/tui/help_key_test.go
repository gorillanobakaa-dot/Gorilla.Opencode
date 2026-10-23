// Version: 1.0.0 · updated 26-10-23-03-01
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
)

// MacBook Air, 2026-10-23 (air-2026-10-23-025732-layout-f8acf24.png): the hint
// "ctrl+? help" named a key macOS Terminal.app cannot send. The hint names "?",
// "?" on an empty prompt and F1 both open help, and ctrl+? stays an alias.
func TestHelpOpensWithKeysEveryTerminalCanSend(t *testing.T) {
	base := *New(&app.App{}).(*appModel)

	m, _ := base.Update(chat.ToggleHelpMsg{})
	if !m.(appModel).showHelp {
		t.Error("ToggleHelpMsg (\"?\" on an empty prompt) did not open help")
	}
	m, _ = base.Update(tea.KeyMsg{Type: tea.KeyF1})
	if !m.(appModel).showHelp {
		t.Error("F1 did not open help")
	}
	m, _ = base.Update(tea.KeyMsg{Type: tea.KeyCtrlUnderscore})
	if !m.(appModel).showHelp {
		t.Error("ctrl+? (sent as ctrl+_) no longer opens help where the terminal can send it")
	}

	sized, _ := base.Update(tea.WindowSizeMsg{Width: 169, Height: 37})
	bar := ansi.Strip(sized.(appModel).status.View())
	if strings.Contains(bar, "ctrl+?") || !strings.Contains(bar, "? help") {
		t.Errorf("status bar hint is not the key every terminal can send: %q", strings.TrimSpace(bar))
	}
}
