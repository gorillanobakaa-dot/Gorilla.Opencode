package agent

// GORILLA OVERRIDE (2026-11-21): verifies session-scoped sub-agent spawn
// telemetry survives helper completion and unregistration.

import (
	"testing"
)

func TestSessionAgentTelemetryPersistsAfterUnregister(t *testing.T) {
	ResetSessionAgentTelemetry()
	t.Cleanup(ResetSessionAgentTelemetry)

	h1 := RegisterSubAgentState("child-1", "sess-A", "call-1", "inspect auth", SubAgentRunning, func() {})
	h2 := RegisterSubAgentState("child-2", "sess-A", "call-1", "inspect db", SubAgentQueued, func() {})
	h3 := RegisterSubAgentState("child-3", "sess-B", "call-2", "other session", SubAgentRunning, func() {})

	SetSubAgentState(h1.ID, SubAgentDone)
	KillSubAgent(h2.ID)
	UnregisterSubAgent(h1.ID)
	UnregisterSubAgent(h3.ID)

	stA := SessionAgentTelemetry("sess-A")
	if stA.TotalSpawned != 2 || stA.Done != 1 || stA.Killed != 1 || len(stA.History) != 2 {
		t.Fatalf("unexpected sess-A telemetry: %+v", stA)
	}
	stB := SessionAgentTelemetry("sess-B")
	if stB.TotalSpawned != 1 || stB.Done != 1 {
		t.Fatalf("unexpected sess-B telemetry: %+v", stB)
	}
	all := SessionAgentTelemetry("")
	if all.TotalSpawned != 3 {
		t.Fatalf("process-wide telemetry TotalSpawned = %d, want 3", all.TotalSpawned)
	}
}
