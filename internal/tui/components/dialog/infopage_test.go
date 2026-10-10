package dialog

// GORILLA (2026-10-10): the page component behind /editor, /helpers and /hooks.
// The frame-fit and screen-invariant harnesses measure its size; these tests
// measure what a reader can reach: every line of the page by scrolling, the
// status line saying there is more, and esc closing it.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/helppages"
)

// infoPageHooksFixture is /hooks in its tallest and widest form: three hooks
// loaded, one with a command longer than any terminal, and a settings path
// with a long user name.
func infoPageHooksFixture() helppages.Page {
	return helppages.HooksPage([]config.Hook{
		{Event: config.HookBeforeTool, Tools: []string{"bash", "edit", "write"}, Command: strings.Repeat("check-this-input ", 12), TimeoutSeconds: 45},
		{Event: config.HookAfterTool, Command: "Add-Content ~/gorilla-hooks.log $env:GORILLA_HOOK_TOOL"},
		{Event: config.HookTurnEnd, Command: "[console]::beep(880, 300)", TimeoutSeconds: 5},
	}, `C:\Users\someone-with-a-really-long-account-name\.config\gorilla-opencode\config.json`,
		`C:\Users\someone-with-a-really-long-account-name\projects\a-project\.gorilla-opencode.json`, "windows")
}

func TestInfoPageEveryLineIsReachableByScrolling(t *testing.T) {
	for _, sz := range [][2]int{{40, 8}, {80, 24}, {130, 42}} {
		m := NewInfoPageCmp(infoPageHooksFixture())
		m.SetSize(sz[0], sz[1])
		seen := map[string]bool{}
		for i := 0; i < 400; i++ {
			for _, l := range strings.Split(ansi.Strip(m.View()), "\n") {
				seen[strings.TrimSpace(l)] = true
			}
			d, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = d.(InfoPageCmp)
		}
		for _, l := range m.lines() {
			if txt := strings.TrimSpace(l.text); txt != "" && !seen[txt] {
				t.Errorf("%dx%d: line never shown however far you scroll: %q", sz[0], sz[1], txt)
			}
		}
	}
}

func TestInfoPageSaysWhenThereIsMore(t *testing.T) {
	m := NewInfoPageCmp(helppages.Editor())
	m.SetSize(80, 24)
	if v := ansi.Strip(m.View()); !strings.Contains(v, "lines 1-") || !strings.Contains(v, "PgUp/PgDn") {
		t.Errorf("a page longer than the window does not say so:\n%s", v)
	}
	big := NewInfoPageCmp(helppages.Page{Title: "T", Lines: []helppages.Line{{Kind: helppages.Text, Text: "one line"}}})
	big.SetSize(80, 24)
	if v := ansi.Strip(big.View()); strings.Contains(v, "lines ") || !strings.Contains(v, "esc to close") {
		t.Errorf("a page that fits should only say how to close it:\n%s", v)
	}
}

func TestInfoPageEscCloses(t *testing.T) {
	m := NewInfoPageCmp(helppages.Helpers())
	m.SetSize(80, 24)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc returned no command")
	}
	if _, ok := cmd().(CloseInfoPageMsg); !ok {
		t.Error("esc did not close the page")
	}
}

// The page wraps code it must cut; it never clips it. Nothing of a long hook
// command or path may be lost on a narrow terminal.
func TestInfoPageCutsLongCodeWithoutLosingIt(t *testing.T) {
	p := infoPageHooksFixture()
	m := NewInfoPageCmp(p)
	m.SetSize(40, 24)
	var joined strings.Builder
	for _, l := range m.lines() {
		if l.kind == helppages.Code {
			joined.WriteString(l.text)
		}
		if w := ansi.StringWidth(l.text); w > m.contentWidth() {
			t.Errorf("line %q is %d columns, wider than %d", l.text, w, m.contentWidth())
		}
	}
	for _, l := range p.Lines {
		if l.Kind == helppages.Code && !strings.Contains(joined.String(), l.Text) {
			t.Errorf("code line lost on a 40-column terminal: %q", l.Text)
		}
	}
}
