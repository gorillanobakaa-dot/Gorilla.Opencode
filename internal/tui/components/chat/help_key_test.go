// Version: 1.0.0 · updated 26-10-23-03-01
package chat

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/app"
)

func typeRune(t *testing.T, m tea.Model, r rune) (tea.Model, tea.Msg) {
	t.Helper()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	if cmd == nil {
		return next, nil
	}
	return next, cmd()
}

// MacBook Air, 2026-10-23: the status bar said "ctrl+? help", and ctrl+?
// cannot be typed in macOS Terminal.app. "?" on an empty prompt is the help key
// every terminal can send. It must not swallow a "?" inside a question.
func TestQuestionMarkOnAnEmptyPromptAsksForHelp(t *testing.T) {
	ed := NewEditorCmp(&app.App{})
	ed, msg := typeRune(t, ed, '?')
	if _, ok := msg.(ToggleHelpMsg); !ok {
		t.Fatalf("\"?\" on an empty prompt sent %T, want ToggleHelpMsg", msg)
	}
	if v := ed.(*editorCmp).textarea.Value(); v != "" {
		t.Errorf("the \"?\" was also typed into the prompt: %q", v)
	}

	ed, _ = typeRune(t, ed, 'w')
	ed, msg = typeRune(t, ed, '?')
	if _, ok := msg.(ToggleHelpMsg); ok {
		t.Error("\"?\" after text opened help; a question could never contain a \"?\"")
	}
	if v := ed.(*editorCmp).textarea.Value(); v != "w?" {
		t.Errorf("prompt is %q, want \"w?\"", v)
	}
}
