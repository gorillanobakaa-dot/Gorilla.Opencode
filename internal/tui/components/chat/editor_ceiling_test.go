package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
)

// THE BUG: the editor ignored the rows it was given.
//
// Measured 2026-08-05: handed 1, 2, 3 or 5 rows by the layout it rendered 16
// every time, because desiredHeight() consulted only its own content and
// maxEditorHeight. The comment on maxEditorHeight claimed "the footer's own
// budget clamps this further on short windows" — no such clamp existed.
//
// Consequence: on a short terminal the frame grew taller than the window, which
// breaks bubbletea's inline erase (it walks the cursor up by LOGICAL lines). A
// long prompt then appeared pinned to a single line, scrolling from the last
// word, while the identical build wrapped correctly in a taller window.
//
// Note the shape of this test: it varies the CEILING, not the width. Every
// earlier probe varied width, passed, and missed the bug entirely.
func TestTheFieldNeverExceedsItsCeiling(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	long := strings.Repeat("word ", 300) // far more than any ceiling here

	for _, ceiling := range []int{1, 2, 3, 5, 8} {
		m := &editorCmp{}
		m.textarea = CreateTextArea(nil)
		m.SetSize(100, ceiling)
		m.SetHeightCeiling(ceiling)
		m.textarea.SetValue(long)

		got := lipgloss.Height(m.View())
		if got > ceiling {
			t.Errorf("ceiling %d: rendered %d rows — a frame taller than the window "+
				"breaks the inline erase and the field appears stuck on one line",
				ceiling, got)
		}
	}
}

// Hidden rows must be ANNOUNCED. A field that silently drops what you typed is
// indistinguishable from data loss — which is exactly how this was reported.
func TestAClampedFieldSaysWhatItIsHiding(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := &editorCmp{}
	m.textarea = CreateTextArea(nil)
	m.SetSize(100, 3)
	m.SetHeightCeiling(3)
	m.textarea.SetValue(strings.Repeat("word ", 300))

	view := m.View()
	if !strings.Contains(view, overflowArrow) {
		t.Errorf("a clamped field gives no sign it is holding text back:\n%s", view)
	}
	if n := m.hiddenLines(); n == 0 {
		t.Error("hiddenLines() reports 0 while rows are being withheld")
	}
}

// With no ceiling set (0), behaviour is unchanged — the cap must not fire before
// the frame owner has had a chance to set it.
func TestNoCeilingMeansNoExtraLimit(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := &editorCmp{}
	m.textarea = CreateTextArea(nil)
	m.SetSize(100, 1)
	// Enough content to exceed maxEditorHeight at this width — 300 words only
	// needed 16 rows, so the original fixture could never have reached the cap.
	m.textarea.SetValue(strings.Repeat("word ", 900))

	// Assert the WHOLE view, not the textarea: the overflow notice occupies a row
	// too, so the meaningful invariant is that the rendered field stays within
	// maxEditorHeight overall. (Checking desiredHeight alone reads 19 here — the
	// textarea gives up one row to the notice — which is correct and would have
	// made this test fail for the wrong reason.)
	if got := lipgloss.Height(m.View()); got != maxEditorHeight {
		t.Errorf("with no ceiling the field rendered %d rows, want the usual cap of %d",
			got, maxEditorHeight)
	}
}

// A ceiling wider than the content must not pad the field out to it.
func TestCeilingIsACeilingNotADemand(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := &editorCmp{}
	m.textarea = CreateTextArea(nil)
	m.SetSize(100, 10)
	m.SetHeightCeiling(10)
	m.textarea.SetValue("short")

	if got := m.desiredHeight(); got != minEditorHeight {
		t.Errorf("a one-line prompt claimed %d rows under a ceiling of 10; the cap "+
			"is a ceiling, not a demand", got)
	}
}
