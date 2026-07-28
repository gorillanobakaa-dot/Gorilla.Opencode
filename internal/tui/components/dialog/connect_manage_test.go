package dialog

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
)

// seedEndpoints replaces the configured endpoints for one test and restores them
// after. Replace, never append: cfg is process-global here and config.Load
// registers real endpoints, so an appending helper lets one test's rows change
// the row indices another test depends on — passing alone, failing in suite.
func seedEndpoints(t *testing.T, eps ...config.LocalEndpoint) {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	c := config.Get()
	prev := c.LocalEndpoints
	t.Cleanup(func() { c.LocalEndpoints = prev })
	c.LocalEndpoints = eps
}

// An endpoint saved under a name no preset covers had no row in /connect at all,
// so it could not be edited, switched off or removed from inside the app. One
// config held four such NVIDIA entries; the only way to see them was to open
// config.json in an editor.
func TestConfiguredEndpointsAppearAsRows(t *testing.T) {
	seedEndpoints(t,
		config.LocalEndpoint{Name: "Gorilla.FREE.NVIDIA.NIM", BaseURL: "https://integrate.api.nvidia.com/v1"},
		config.LocalEndpoint{Name: "ollama", BaseURL: "http://localhost:11434/v1"},
	)

	m := &connectDialogCmp{width: 100, height: 40}
	m.Init()

	labels := make([]string, 0)
	for _, e := range m.entries() {
		labels = append(labels, e.label)
	}

	found := false
	for _, l := range labels {
		if l == "Gorilla.FREE.NVIDIA.NIM" {
			found = true
		}
	}
	if !found {
		t.Errorf("a configured endpoint with a custom name is absent from the list, so it cannot be managed: %v", labels)
	}

	// "ollama" IS covered by a preset row, so it must not be listed twice.
	n := 0
	for _, l := range labels {
		if strings.Contains(strings.ToLower(l), "ollama") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("ollama appears %d times; a preset and a configured entry for the same name must collapse to one row", n)
	}
}

// Removal is destructive and its key sits next to the arrows, so it asks first.
func TestDeleteAsksBeforeRemoving(t *testing.T) {
	seedEndpoints(t, config.LocalEndpoint{Name: "custom-one", BaseURL: "http://127.0.0.1:1/v1"})

	m := &connectDialogCmp{width: 100, height: 40}
	m.Init()
	m.selectedIdx = indexOfLabel(t, m, "custom-one")

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m.mode != modeConfirmDelete {
		t.Fatal("d did not open a confirmation; a single keypress must not delete a stored credential")
	}
	if !strings.Contains(m.View(), "custom-one") {
		t.Error("the confirmation does not name what is about to be removed")
	}

	// Anything that is not "y" cancels, and the endpoint survives.
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeList {
		t.Error("esc did not cancel the confirmation")
	}
	if len(config.Get().LocalEndpoints) != 1 {
		t.Error("the endpoint was removed despite the confirmation being cancelled")
	}
}

// A preset row is an offer to connect, not stored state — there is nothing to
// remove, and pressing d must say so rather than doing nothing at all.
func TestDeleteRefusesPresetRows(t *testing.T) {
	seedEndpoints(t)

	m := &connectDialogCmp{width: 100, height: 40}
	m.Init()
	m.selectedIdx = 0 // Anthropic — a preset

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m.mode == modeConfirmDelete {
		t.Fatal("offered to delete a built-in row")
	}
	if cmd == nil {
		t.Fatal("pressing d on a preset did nothing and said nothing")
	}
	msg, ok := cmd().(ConnectionChangedMsg)
	if !ok || msg.Info == "" {
		t.Fatalf("expected an explanation, got %#v", cmd())
	}
}

// A pending confirmation must not survive closing and reopening the dialog, or
// the next `y` typed in the list would delete an endpoint the user had left.
func TestReopeningClearsAPendingDeletion(t *testing.T) {
	seedEndpoints(t, config.LocalEndpoint{Name: "custom-one", BaseURL: "http://127.0.0.1:1/v1"})

	m := &connectDialogCmp{width: 100, height: 40}
	m.Init()
	m.selectedIdx = indexOfLabel(t, m, "custom-one")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	m.Init() // reopened

	if m.mode != modeList {
		t.Error("reopened straight into the confirmation")
	}
	if m.pendingDelete.epName != "" {
		t.Errorf("a pending deletion of %q survived the reopen", m.pendingDelete.epName)
	}
}

// Every line of a dialog must be the same width. lipgloss does not pad the short
// lines of a multi-line render, so any line narrower than the widest leaves
// unpainted cells that show as terminal black — the black bars reported before.
func TestConnectViewsHaveUniformLineWidth(t *testing.T) {
	seedEndpoints(t,
		config.LocalEndpoint{Name: "a-very-long-endpoint-name-that-would-overflow-the-dialog-box", BaseURL: "https://an-extremely-long-hostname.example.com/some/deep/path/v1"},
	)

	m := &connectDialogCmp{width: 100, height: 40}
	m.Init()

	views := map[string]func(){
		"list": func() { m.mode = modeList },
		"confirm-delete": func() {
			m.mode = modeConfirmDelete
			m.pendingDelete = m.entries()[len(m.entries())-1]
		},
	}
	for name, setup := range views {
		setup()
		lines := strings.Split(m.View(), "\n")
		if len(lines) < 3 {
			t.Fatalf("%s: rendered only %d lines", name, len(lines))
		}
		want := lipgloss.Width(lines[0])
		for i, l := range lines {
			if got := lipgloss.Width(l); got != want {
				t.Errorf("%s: line %d is %d columns, first line is %d — the difference renders as a black bar:\n%q",
					name, i, got, want, l)
			}
		}
	}
}

// A long name or URL must not make the confirmation box taller.
//
// Note on what this does NOT test: I first wrote this as a WIDTH assertion, on
// the assumption that an over-long name would push the box past the dialog
// width. It would not — lipgloss .Width(w) WRAPS text that exceeds w rather than
// overflowing, so every line came back at exactly 60 columns with or without the
// truncation, and the test passed against the bug. Wrapping converts the problem
// from width into HEIGHT: a 120-character name becomes three lines, a long URL
// four more, and the box outgrows a short terminal. That is the real invariant,
// so that is what is asserted here.
func TestConfirmDeleteBoxHeightDoesNotGrowWithLongNames(t *testing.T) {
	height := func(t *testing.T, ep config.LocalEndpoint) int {
		t.Helper()
		seedEndpoints(t, ep)
		m := &connectDialogCmp{width: 100, height: 40}
		m.Init()
		m.mode = modeConfirmDelete
		m.pendingDelete = m.entries()[len(m.entries())-1]
		return lipgloss.Height(m.View())
	}

	short := height(t, config.LocalEndpoint{Name: "ep", BaseURL: "http://127.0.0.1:1/v1"})
	long := height(t, config.LocalEndpoint{
		Name:    strings.Repeat("long-name-", 12),
		BaseURL: "https://" + strings.Repeat("host.", 30) + "example.com/v1",
	})

	if long != short {
		t.Errorf("the box grew from %d to %d lines for a long name and URL — lipgloss wraps rather than overflows, so an untruncated name silently makes the dialog taller and it overruns a short terminal", short, long)
	}
}

// `d` must not be swallowed while a form's text input has focus, or the user
// cannot type a "d" into an endpoint name or paste a key containing one.
func TestDeleteKeyDoesNotFireWhileTypingInAForm(t *testing.T) {
	seedEndpoints(t)

	m := &connectDialogCmp{width: 100, height: 40}
	m.Init()
	m.openForm(connectEntry{kind: kindLocal, label: "Custom OpenAI-compatible endpoint"})

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	if m.mode != modeForm {
		t.Fatalf("typing d in a form left form mode (mode=%v)", m.mode)
	}
	if got := m.inputs[m.focusIdx].Value(); !strings.Contains(got, "d") {
		t.Errorf("the character was consumed as a shortcut instead of typed; field holds %q", got)
	}
}

// Sanity check on the keymap itself, so the help footer and the handler cannot
// drift apart.
func TestDeleteBindingIsAdvertised(t *testing.T) {
	m := &connectDialogCmp{}
	var found bool
	for _, b := range m.BindingKeys() {
		if key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}, b) {
			found = true
		}
	}
	if !found {
		t.Error("d is handled but not advertised in BindingKeys, so it stays invisible to the help footer")
	}
}

func indexOfLabel(t *testing.T, m *connectDialogCmp, label string) int {
	t.Helper()
	for i, e := range m.entries() {
		if e.label == label {
			return i
		}
	}
	t.Fatalf("no row labelled %q", label)
	return 0
}
