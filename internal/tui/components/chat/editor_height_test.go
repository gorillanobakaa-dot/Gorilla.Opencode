package chat

import "testing"

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
	if got := m.desiredHeight(); got != maxEditorHeight {
		t.Errorf("overlong: got %d, want cap %d", got, maxEditorHeight)
	}
}
