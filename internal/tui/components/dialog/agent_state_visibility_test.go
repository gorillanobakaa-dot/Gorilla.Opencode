package dialog

// GORILLA OVERRIDE (2026-11-21): verifies visual confirmation when stepping
// agent count via arrow keys, persistent Nuclear Option confirmation banner,
// and session agent telemetry inside /tasks.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
)

func TestLoadoutDialogConfirmsAgentCountAndNuclearState(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	prev := config.MaxSubAgents()
	t.Cleanup(func() { config.SetMaxSubAgents(prev) })

	config.SetMaxSubAgents(1)
	d := NewLoadoutDialogCmp().(*loadoutDialogCmp)
	d.termWidth = 120
	d.termHeight = 45
	d.selectedIdx = rowLeash

	// Step left from 1 -> Nuclear (0).
	_, _ = d.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if config.MaxSubAgents() != config.SubAgentsNuclear {
		t.Fatalf("MaxSubAgents = %d, want Nuclear (0)", config.MaxSubAgents())
	}

	view := ansi.Strip(d.View())
	if !strings.Contains(view, "[SET]") {
		t.Errorf("agent count row missing [SET] confirmation tag:\n%s", view)
	}
	if !strings.Contains(view, "[APPLIED]") {
		t.Errorf("loadout dialog missing [APPLIED] confirmation banner:\n%s", view)
	}
	if !strings.Contains(view, "NUCLEAR OPTION ACTIVE") {
		t.Errorf("loadout dialog missing persistent NUCLEAR OPTION ACTIVE banner:\n%s", view)
	}
}

func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func TestLoadoutEnterConfirmsAndClosesWithoutUndoingNuclear(t *testing.T) {
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	prev := config.MaxSubAgents()
	t.Cleanup(func() { config.SetMaxSubAgents(prev) })

	config.SetMaxSubAgents(1)
	d := NewLoadoutDialogCmp().(*loadoutDialogCmp)
	d.termWidth = 120
	d.termHeight = 45
	d.selectedIdx = rowLeash

	// Dial left to Nuclear (0), press Space (must hold Nuclear), then Enter (must confirm & close).
	_, _ = d.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if config.MaxSubAgents() != config.SubAgentsNuclear {
		t.Fatalf("after Left: MaxSubAgents = %d, want Nuclear (0)", config.MaxSubAgents())
	}
	_, _ = d.Update(tea.KeyMsg{Type: tea.KeySpace})
	if config.MaxSubAgents() != config.SubAgentsNuclear {
		t.Fatalf("after Space on Agents dial: MaxSubAgents = %d, want Nuclear (0)", config.MaxSubAgents())
	}
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if config.MaxSubAgents() != config.SubAgentsNuclear {
		t.Fatalf("after Enter on Agents dial: MaxSubAgents = %d, want Nuclear (0) — Enter must not step dial +1", config.MaxSubAgents())
	}
	closed := false
	for _, m := range collectMsgs(cmd) {
		if _, ok := m.(CloseLoadoutDialogMsg); ok {
			closed = true
		}
	}
	if !closed {
		t.Fatal("pressing Enter in /context did not emit CloseLoadoutDialogMsg")
	}

	// Pressing Enter on a feature row must also confirm & close without toggling it.
	rows := sortedLoadout()
	targetID := rows[0].ID
	before := config.LoadoutEnabled(targetID)
	d.selectedIdx = numDials
	_, featCmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if config.LoadoutEnabled(targetID) != before {
		t.Fatalf("pressing Enter on feature row %q flipped it instead of confirming & closing", targetID)
	}
	featClosed := false
	for _, m := range collectMsgs(featCmd) {
		if _, ok := m.(CloseLoadoutDialogMsg); ok {
			featClosed = true
		}
	}
	if !featClosed {
		t.Fatal("pressing Enter on a feature row did not emit CloseLoadoutDialogMsg")
	}
}

func TestTasksDialogShowsSessionTelemetryAndHistory(t *testing.T) {
	agent.ResetSessionAgentTelemetry()
	t.Cleanup(agent.ResetSessionAgentTelemetry)

	h := agent.RegisterSubAgentState("sub-1", "sess-telemetry", "tc-1", "audit token burn", agent.SubAgentRunning, func() {})
	agent.SetSubAgentState(h.ID, agent.SubAgentDone)
	agent.UnregisterSubAgent(h.ID)

	td := NewTasksDialogCmp()
	td.SetSessionID("sess-telemetry")
	plain := ansi.Strip(td.View())
	if !strings.Contains(plain, "Session audit: 1 spawned") {
		t.Errorf("tasks dialog missing session spawn count:\n%s", plain)
	}
	if !strings.Contains(plain, "audit token burn") {
		t.Errorf("tasks dialog missing historical spawned agent entry:\n%s", plain)
	}
}
