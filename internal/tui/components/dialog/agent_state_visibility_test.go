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
