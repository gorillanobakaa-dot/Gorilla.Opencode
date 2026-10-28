package startup

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func paidRows(accepted bool) []ProviderRow {
	return []ProviderRow{{
		ID: "anthropic", Name: "Anthropic", What: "Paid API.", NeedsInput: true,
		InputPrompt: "Paste your key.", Secret: true,
		CostWarning:  "API KEY: YOUR CARD PAYS FOR EVERY MESSAGE.\nEvery message costs money.\n\n  model   new chat\n  Sonnet  $0.055",
		CostAccepted: accepted,
	}}
}

func typed(m *providerModel, s string) *providerModel {
	return feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

// A paid row shows the warning first, and nothing goes on until
// "I understand" is typed (owner, 2026-10-28).
func TestPaidRowAsksForIUnderstandBeforeTheKey(t *testing.T) {
	m := &providerModel{rows: paidRows(false), canKeep: true, width: 80, height: 39}
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.acking || !strings.Contains(m.View(), "YOUR CARD PAYS") {
		t.Fatalf("Enter on a paid row did not show the warning:\n%s", m.View())
	}
	m = typed(m, "ok")
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.acking || m.entering {
		t.Fatal("a wrong answer let the person through")
	}
	m = typed(m, "I understand")
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.acking || !m.entering {
		t.Fatal("after I understand, the key field should open")
	}
	m = typed(m, "sk-ant-test")
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.done || !m.choice.CostAccepted || m.choice.Input != "sk-ant-test" {
		t.Errorf("choice = %+v, want the key with CostAccepted", m.choice)
	}
}

// Once accepted, it is not asked again, and Esc on the warning goes back to
// the list without choosing anything.
func TestAcceptedWarningIsNotShownAgainAndEscGoesBack(t *testing.T) {
	m := &providerModel{rows: paidRows(true), canKeep: true}
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.acking {
		t.Error("an accepted warning was shown again")
	}
	m = &providerModel{rows: paidRows(false), canKeep: true}
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = feed(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.acking || m.done || m.entering {
		t.Error("Esc on the warning should return to the list")
	}
}

// The warning screen keeps the frame-width rule at every width.
func TestCostWarningFitsTheTerminal(t *testing.T) {
	for _, w := range []int{24, 40, 80, 167} {
		m := &providerModel{rows: paidRows(false), canKeep: true, width: w, height: 39}
		m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
		m = typed(m, strings.Repeat("x", 300))
		for i, line := range strings.Split(m.View(), "\n") {
			if lw := lipgloss.Width(line); lw > m.contentWidth()+4 || lw > w {
				t.Fatalf("width %d: line %d is %d cells: %q", w, i, lw, line)
			}
		}
	}
}
