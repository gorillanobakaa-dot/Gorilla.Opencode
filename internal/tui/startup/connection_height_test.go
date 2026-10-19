package startup

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// 2026-10-19: at 80x24 (Terminal's default on the owner's 11-inch MacBook Air)
// the connection picker was taller than the window and its title scrolled off.
// The owner drags the window, so the frame is checked after every resize.
func TestTheConnectionPickerFollowsTheWindow(t *testing.T) {
	long := strings.Repeat("A sentence of ordinary length that wraps on a narrow window. ", 5)
	rows := make([]ConnRow, 5)
	for i := range rows {
		rows[i] = ConnRow{ID: string(rune('a' + i)), Name: "Profile", Rate: "10-60 KB/s",
			Links: "Typical of: " + long, What: long, Effect: long, Advice: long}
	}
	rows[2].Active = true
	m := NewConnModel(rows, "under 1 KB/s")
	for _, size := range [][2]int{{169, 43}, {80, 24}, {138, 37}, {100, 30}, {144, 33}, {80, 24}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		// not trimmed: the frame's trailing newline is a row on screen too
		lines := strings.Split(m.View(), "\n")
		if len(lines) > size[1] {
			t.Errorf("%dx%d: %d lines, taller than the window", size[0], size[1], len(lines))
		}
		if !strings.Contains(lines[0], "How fast is your connection") {
			t.Errorf("%dx%d: the title is not the first line: %q", size[0], size[1], lines[0])
		}
		if !strings.Contains(m.View(), "enter choose") {
			t.Errorf("%dx%d: the key line is gone", size[0], size[1])
		}
	}
}
