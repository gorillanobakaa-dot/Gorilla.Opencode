package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
)

// wordsFor builds prose of roughly n words.
func wordsFor(n int) string {
	w := make([]string, n)
	for i := range w {
		w[i] = "word"
	}
	return strings.Join(w, " ")
}

// The reported bug: typing a long prompt showed one word. The field was one row
// tall, so soft-wrapping scrolled everything above the cursor's row out of sight,
// and the height only arrived later via the layout. Outside the alternate screen the
// field must size ITSELF, on the frame it is drawn.
func TestInputSizesItselfWithoutWaitingForTheLayout(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	m := &editorCmp{}
	m.textarea = CreateTextArea(nil)
	m.SetSize(100, 1) // the one row the layout starts it at
	m.textarea.SetValue(wordsFor(120))

	rows := lipgloss.Height(m.View())
	if rows <= 1 {
		t.Fatalf("the field rendered %d row(s) for a 120-word prompt; all but the "+
			"cursor's line is invisible, which is what made a long prompt look like it "+
			"was losing words", rows)
	}
	if want := m.desiredHeight(); rows < want {
		t.Errorf("rendered %d rows but wanted %d", rows, want)
	}
}

func TestExactLongPromptWrapsAtTerminalWidth(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	const value = "lalala lalalal lalala lalala lalala lalalalalalal lalalalala f f f f fhjakal  alallalal ghhghhhf dhshshshshe ehekdkdn  askdjd djnjnjdn iwidnidisn hhshshshhs hs here we gon"
	m := &editorCmp{}
	m.textarea = CreateTextArea(nil)
	// The terminal is 176 columns, but the prompt and border leave 172 columns
	// for text. Use 170 to exercise the first actual soft-wrap boundary.
	m.SetSize(174, 1)
	m.textarea.SetValue(value)

	if got := wrappedRows(value, m.textarea.Width()); got < 2 {
		t.Fatalf("wrappedRows measured %d rows at textarea width %d for %d characters", got, m.textarea.Width(), len(value))
	}
	if got := lipgloss.Height(m.View()); got < 2 {
		t.Fatalf("exact prompt rendered as %d row at 176 columns; earlier text would scroll out of sight", got)
	}
}

// 300 words must be visible at once, which is what was asked for. Asserted against
// the measurement rather than the constant, so raising the cap without checking the
// arithmetic cannot quietly fail it.
func TestThreeHundredWordsFitWithoutScrolling(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	m := &editorCmp{}
	m.textarea = CreateTextArea(nil)
	m.SetSize(100, 1)
	m.textarea.SetValue(wordsFor(300))

	if hidden := func() int { m.View(); return m.hiddenLines() }(); hidden != 0 {
		t.Errorf("%d row(s) of a 300-word prompt are scrolled out of sight; the visible "+
			"cap is %d rows and this needs %d", hidden, maxEditorHeight,
			wrappedRows(m.textarea.Value(), m.textarea.Width()))
	}
}

// Past the cap, the field must SAY how much is hidden rather than silently drop it.
// Silence is what made this look like data loss.
func TestHiddenLinesAreAnnounced(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	m := &editorCmp{}
	m.textarea = CreateTextArea(nil)
	m.SetSize(100, 1)
	m.textarea.SetValue(wordsFor(1200)) // far past 20 rows

	view := m.View()
	if !strings.Contains(view, overflowArrow) {
		t.Errorf("no overflow arrow for a 1200-word prompt; the hidden rows are "+
			"indistinguishable from lost text:\n%s", view)
	}
	if !strings.Contains(view, "more line") {
		t.Errorf("the notice does not say how many lines are hidden:\n%s", view)
	}
	if n := m.hiddenLines(); n == 0 {
		t.Error("hiddenLines() reports 0 while the notice is being shown")
	}

	// The arrow must be single-width, or every column after it is misplaced.
	if w := lipgloss.Width(overflowArrow); w != 1 {
		t.Errorf("the overflow arrow is %d columns wide; a double-width glyph shifts "+
			"the rest of the line", w)
	}
}

// A short prompt must NOT carry the notice: a permanent indicator says nothing.
func TestNoOverflowNoticeWhenEverythingFits(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}

	m := &editorCmp{}
	m.textarea = CreateTextArea(nil)
	m.SetSize(100, 1)
	m.textarea.SetValue("a short question")

	if view := m.View(); strings.Contains(view, overflowArrow) {
		t.Errorf("a one-line prompt claims lines are hidden:\n%s", view)
	}
}

// bubbles refuses to add logical lines past MaxHeight, whose default is 99. A long
// pasted prompt was therefore truncated regardless of CharLimit being unlimited.
func TestTheBufferAcceptsAVeryLongPastedPrompt(t *testing.T) {
	m := &editorCmp{}
	m.textarea = CreateTextArea(nil)
	m.SetSize(100, 1)

	if m.textarea.MaxHeight <= 99 {
		t.Errorf("MaxHeight is %d, still at or below bubbles' default of 99; a pasted "+
			"prompt with more newlines than that is silently truncated",
			m.textarea.MaxHeight)
	}

	// 250 short lines: well past the old default, inside the new one.
	const lines = 250
	m.textarea.SetValue(strings.TrimRight(strings.Repeat("line of text\n", lines), "\n"))
	if got := m.textarea.LineCount(); got < lines {
		t.Errorf("the buffer kept %d of %d pasted lines", got, lines)
	}
}
