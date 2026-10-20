package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// 2026-10-20: q and Backspace were meant to leave the logs page and never did
// (key.Matches with no binding is always false). They must work there, and
// nowhere else: Backspace in the chat editor closing a dialog would lose work.
func TestTheLogsPageKeysWorkOnlyOnTheLogsPage(t *testing.T) {
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	back := tea.KeyMsg{Type: tea.KeyBackspace}
	q := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(quitKey)}
	for _, c := range []struct {
		name   string
		msg    tea.KeyMsg
		onLogs bool
		want   bool
	}{
		{"esc anywhere", esc, false, true},
		{"esc on logs", esc, true, true},
		{"backspace on logs", back, true, true},
		{"q on logs", q, true, true},
		{"backspace while typing", back, false, false},
		{"q while typing", q, false, false},
	} {
		if got := isReturnKey(c.msg, c.onLogs); got != c.want {
			t.Errorf("%s: isReturnKey = %v, want %v", c.name, got, c.want)
		}
	}
}
