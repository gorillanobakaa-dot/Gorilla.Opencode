package dialog

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
)

// Headless render assertions for the three dialogs added today. The brain's TUI
// method is explicit: never fix or trust a dialog by looking at a screenshot —
// construct it in-package, call View(), and assert on the geometry. These catch
// the ragged-row and unpainted-background class of bug (ROOT CAUSE #1/#2) without
// a terminal.
//
// They also catch the container-chrome trap indirectly: a dialog whose rows are
// not uniform width will show it here.

// renderAt sizes a dialog like a real terminal would and returns its lines.
func renderAt(t *testing.T, m tea.Model, w, h int) []string {
	t.Helper()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	view := updated.(interface{ View() string }).View()
	if strings.TrimSpace(view) == "" {
		t.Fatal("dialog rendered an empty view")
	}
	return strings.Split(view, "\n")
}

// assertUniformWidth is the core invariant: every line the same visual width.
// A short line leaves the terminal background showing through, which reads as a
// black gap — lipgloss .Width() does NOT pad every line of a multi-line string.
func assertUniformWidth(t *testing.T, name string, lines []string) {
	t.Helper()
	if len(lines) == 0 {
		t.Fatalf("%s: no lines", name)
	}
	want := lipgloss.Width(lines[0])
	for i, ln := range lines {
		if got := lipgloss.Width(ln); got != want {
			t.Errorf("%s: row %d width = %d, want %d (ragged row leaves an unpainted gap)\nrow: %q",
				name, i, got, want, ln)
			return // one report is enough; they cascade
		}
	}
}

func TestAddDirDialogRenders(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	m := NewAddDirDialogCmp()
	m.Init()

	for _, tc := range []struct{ w, h int }{{100, 30}, {80, 24}, {200, 60}} {
		lines := renderAt(t, m, tc.w, tc.h)
		assertUniformWidth(t, "adddir", lines)

		joined := strings.Join(lines, "\n")
		// The honesty requirement: the dialog must state that adding a root does
		// not grant access, because "add a directory" reads like an unlock.
		if !strings.Contains(joined, "does NOT grant new access") {
			t.Errorf("adddir at %dx%d does not state that adding a root grants no access:\n%s", tc.w, tc.h, joined)
		}
		if !strings.Contains(joined, "PRIMARY") {
			t.Errorf("adddir does not label the primary root:\n%s", joined)
		}
		// Never wider than the terminal.
		if got := lipgloss.Width(lines[0]); got > tc.w {
			t.Errorf("adddir is %d columns wide in a %d-column terminal", got, tc.w)
		}
	}
}

func TestPromptsDialogRendersBothViews(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	prompt.RegisterSectionComponents()

	m := NewPromptsDialogCmp()
	m.Init()

	// List view.
	lines := renderAt(t, m, 120, 40)
	assertUniformWidth(t, "prompts/list", lines)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"System prompts", "coder", "summarizer", "task", "title"} {
		if !strings.Contains(joined, want) {
			t.Errorf("prompts list view missing %q:\n%s", want, joined)
		}
	}

	// Drill into sections with enter.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(PromptsDialog)
	lines = renderAt(t, m, 120, 40)
	assertUniformWidth(t, "prompts/sections", lines)
	joined = strings.Join(lines, "\n")
	for _, want := range []string{"sections", "honesty", "build discipline", "BEHAVIOURAL control"} {
		if !strings.Contains(joined, want) {
			t.Errorf("prompts sections view missing %q:\n%s", want, joined)
		}
	}
	// The critical-section warning marker must be present.
	if !strings.Contains(joined, "▲") {
		t.Errorf("sections view does not mark the critical sections:\n%s", joined)
	}
}

func TestResetDialogRenders(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	m := NewResetDialogCmp()
	m.Init()

	lines := renderAt(t, m, 110, 40)
	assertUniformWidth(t, "reset", lines)
	joined := strings.Join(lines, "\n")

	for _, want := range []string{
		"Reset to defaults",
		"Context loadout",
		"System prompts",
		"Commands",
		"Workspace roots",
		"EVERYTHING",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("reset dialog missing scope %q:\n%s", want, joined)
		}
	}

	// The two hard promises must be on screen, not just in a doc comment. A
	// reset that silently dropped credentials or history would be a destructive
	// surprise, and the user has to be able to see that it will not.
	if !strings.Contains(joined, "NEVER touched") {
		t.Errorf("reset dialog does not promise that keys and sessions are untouched:\n%s", joined)
	}
	if !strings.Contains(joined, "/connect") || !strings.Contains(joined, "/clear") {
		t.Errorf("reset dialog does not point at /connect and /clear for the things it will not do:\n%s", joined)
	}
}

// A dialog must survive a small terminal without producing rows wider than the
// screen — that is what pushed the sidebar off-screen in v0.1.38.
func TestNewDialogsFitNarrowTerminals(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	prompt.RegisterSectionComponents()

	for name, m := range map[string]tea.Model{
		"adddir":  NewAddDirDialogCmp(),
		"prompts": NewPromptsDialogCmp(),
		"reset":   NewResetDialogCmp(),
	} {
		if init, ok := m.(interface{ Init() tea.Cmd }); ok {
			init.Init()
		}
		lines := renderAt(t, m, 60, 20)
		assertUniformWidth(t, name+"/narrow", lines)
	}
}
