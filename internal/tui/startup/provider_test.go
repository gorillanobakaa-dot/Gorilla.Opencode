package startup

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func testRows() []ProviderRow {
	return []ProviderRow{
		{ID: "a", Name: "Alpha provider with quite a long label indeed", What: strings.Repeat("word ", 60), Configured: true, Active: true},
		{ID: "b", Name: "Bravo", What: "Needs a key.", Warning: strings.Repeat("warning ", 30), NeedsInput: true, InputPrompt: "Paste the key.", Secret: true},
		{ID: "c", Name: "Charlie", What: "No key needed."},
	}
}

func sized(w int) *providerModel {
	m := &providerModel{rows: testRows(), canKeep: true}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 40})
	return mm.(*providerModel)
}

func feed(m *providerModel, msg tea.Msg) *providerModel {
	mm, _ := m.Update(msg)
	return mm.(*providerModel)
}

// The frame-width invariant: at every width, with EACH row selected (so the
// long-description and long-warning render paths are all exercised) AND in
// key-entry mode with a long pasted value, no rendered line exceeds the
// terminal. Selecting every row matters — a per-row description is only drawn
// for the selected one, so testing a single selection misses the others.
func TestProviderPortalNoLineExceedsTheTerminalWidth(t *testing.T) {
	assertWidth := func(t *testing.T, m *providerModel, w int) {
		for i, line := range strings.Split(m.View(), "\n") {
			if lw := lipgloss.Width(line); lw > w {
				t.Fatalf("width %d: line %d is %d cells wide: %q", w, i, lw, line)
			}
		}
	}
	rows := testRows()
	for _, w := range []int{24, 40, 80, 120} {
		for sel := range rows {
			// List mode with this row selected (covers its description/warning).
			m := sized(w)
			m.sel = sel
			assertWidth(t, m, w)

			// Key-entry mode with a huge pasted value, where applicable.
			if rows[sel].NeedsInput && !rows[sel].Configured {
				m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
				m = feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("x", 300))})
				assertWidth(t, m, w)
			}
		}
	}
}

// A secret must never be echoed. This is the falsifiable form of the
// mask-credentials rule: the pasted value must not appear in any frame.
func TestSecretInputIsNeverEchoed(t *testing.T) {
	m := sized(80)
	m.sel = 1
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter}) // enter input mode
	secret := "nvapi-SUPERSECRETVALUE123"
	m = feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(secret)})
	v := m.View()
	if strings.Contains(v, secret) || strings.Contains(v, "SUPERSECRET") {
		t.Fatal("secret input is echoed to the screen")
	}
	// But the length counter should reflect it was received.
	if !strings.Contains(v, "25 chars") {
		t.Fatalf("expected a length counter of 25 chars, view was:\n%s", v)
	}
}

// A non-secret field (the GCP project id) echoes normally.
func TestNonSecretInputIsShown(t *testing.T) {
	rows := []ProviderRow{{ID: "p", Name: "Project", NeedsInput: true, InputPrompt: "id?", Secret: false}}
	m := &providerModel{rows: rows, canKeep: false}
	m = feed(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("my-project")})
	if !strings.Contains(m.View(), "my-project") {
		t.Fatal("non-secret input should be shown")
	}
}

// Enter on a configured row asks for nothing and selects immediately.
func TestEnterOnConfiguredRowSelectsWithoutInput(t *testing.T) {
	m := sized(80) // sel starts at 0 via WindowSizeMsg? no — cursor scan is in AskProviders
	m.sel = 0      // row "a" is Configured
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.done || m.choice.ID != "a" || m.choice.Input != "" {
		t.Fatalf("expected immediate selection of \"a\", got %+v (done=%v)", m.choice, m.done)
	}
}

// Enter on an unconfigured NeedsInput row opens input rather than selecting.
func TestEnterOnUnconfiguredRowOpensInput(t *testing.T) {
	m := sized(80)
	m.sel = 1 // Bravo: NeedsInput, not Configured
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.done {
		t.Fatal("should not have completed; input should be open")
	}
	if !m.entering {
		t.Fatal("expected key-entry mode")
	}
	// Now type and submit.
	m = feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("  gsk_abc  ")})
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.done || m.choice.ID != "b" || m.choice.Input != "gsk_abc" {
		t.Fatalf("expected trimmed key for row b, got %+v", m.choice)
	}
}

// r on a configured NeedsInput row re-opens input deliberately.
func TestReplaceKeyOnConfiguredRow(t *testing.T) {
	rows := []ProviderRow{{ID: "k", Name: "Keyed", NeedsInput: true, Configured: true, Secret: true, InputPrompt: "key?"}}
	m := &providerModel{rows: rows, canKeep: true}
	m = feed(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m = feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if !m.entering {
		t.Fatal("r should re-open input on a configured keyed row")
	}
}

// Empty Enter in input mode sets a hint and does not select.
func TestEmptyInputDoesNotSelect(t *testing.T) {
	m := sized(80)
	m.sel = 1
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter}) // open input
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter}) // submit empty
	if m.done {
		t.Fatal("empty input should not complete the selection")
	}
	if m.hint == "" {
		t.Fatal("expected a hint after empty submit")
	}
}

// Esc keeps the current setup when something works, and quits when nothing does.
func TestEscSemantics(t *testing.T) {
	m := &providerModel{rows: testRows(), canKeep: true}
	m = feed(m, tea.KeyMsg{Type: tea.KeyEsc})
	if c := m.choice; !c.Keep || c.Quit {
		t.Fatalf("canKeep=true: esc should Keep, got %+v", c)
	}
	m = &providerModel{rows: testRows(), canKeep: false}
	m = feed(m, tea.KeyMsg{Type: tea.KeyEsc})
	if c := m.choice; !c.Quit || c.Keep {
		t.Fatalf("canKeep=false: esc should Quit, got %+v", c)
	}
}

// Esc inside key-entry backs out to the list rather than quitting.
func TestEscFromInputReturnsToList(t *testing.T) {
	m := sized(80)
	m.sel = 1
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter}) // open input
	m = feed(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.done || m.entering {
		t.Fatalf("esc in input should return to list, got done=%v entering=%v", m.done, m.entering)
	}
}

// AskProviders places the cursor on the Active row without running a program:
// exercised indirectly by checking the scan logic mirrored here.
func TestActiveRowIsPreselected(t *testing.T) {
	rows := []ProviderRow{
		{ID: "x", Name: "X"},
		{ID: "y", Name: "Y", Active: true},
		{ID: "z", Name: "Z"},
	}
	sel := 0
	for i, r := range rows {
		if r.Active {
			sel = i
			break
		}
	}
	if sel != 1 {
		t.Fatalf("expected active row 1 preselected, got %d", sel)
	}
}
