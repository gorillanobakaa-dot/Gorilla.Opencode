package chat

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// The input box must grow with its content: verify the row measurement that
// drives it (bubbles' textarea exposes no total-visual-rows API, so we measure
// with lipgloss — this test pins that behaviour down).
func TestWrappedRows(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		width int
		want  int
	}{
		{"empty is one row", "", 20, 1},
		{"short fits one row", "hello", 20, 1},
		{"explicit newlines", "a\nb\nc", 20, 3},
		{"long line soft-wraps", "aaaaaaaaaaaaaaaaaaaaaaaaa", 10, 3}, // 25 chars / 10
		{"zero width guards", "anything", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := wrappedRows(tc.value, tc.width); got != tc.want {
				t.Errorf("wrappedRows(%q, %d) = %d, want %d", tc.value, tc.width, got, tc.want)
			}
		})
	}
}

// desiredHeight must clamp: never collapse below one row, never eat the window.
func TestDesiredHeightClamps(t *testing.T) {
	m := &editorCmp{}
	m.textarea = CreateTextArea(nil)
	m.textarea.SetWidth(10)

	m.textarea.SetValue("")
	if got := m.desiredHeight(); got != minEditorHeight {
		t.Errorf("empty: got %d, want %d", got, minEditorHeight)
	}

	// Far more content than the cap allows.
	long := ""
	for i := 0; i < 200; i++ {
		long += "line\n"
	}
	m.textarea.SetValue(long)
	// UPDATED 2026-08-05: the textarea now yields one row to the overflow notice
	// when content is clipped, so desiredHeight caps at maxEditorHeight-1 and the
	// RENDERED field totals maxEditorHeight. The previous expectation of a flat
	// maxEditorHeight encoded an off-by-one: notice + textarea came to 21 rows
	// against a stated cap of 20, and a frame taller than its budget is what
	// breaks bubbletea's inline erase. The invariant worth asserting is the
	// rendered total, so that is asserted too.
	if got := m.desiredHeight(); got != maxEditorHeight-1 {
		t.Errorf("overlong: got %d, want %d (one row reserved for the overflow notice)",
			got, maxEditorHeight-1)
	}
	if got := lipgloss.Height(m.View()); got != maxEditorHeight {
		t.Errorf("overlong: rendered %d rows, want the field to total exactly %d",
			got, maxEditorHeight)
	}
}
