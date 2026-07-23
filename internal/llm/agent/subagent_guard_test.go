package agent

import (
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
)

// GORILLA OVERRIDE: guards the helper-leash spawn counter (Dial 2).
func TestHelperLeashCounting(t *testing.T) {
	sid := "leash-sess-A"
	resetSubAgentSpawns(sid)

	for i := 1; i <= 3; i++ { // limit 3: three succeed
		if ok, used := reserveSubAgentSpawn(sid, 3); !ok || used != i {
			t.Fatalf("spawn %d: ok=%v used=%d", i, ok, used)
		}
	}
	if ok, used := reserveSubAgentSpawn(sid, 3); ok || used != 3 { // fourth refused
		t.Fatalf("4th spawn should be refused, got ok=%v used=%d", ok, used)
	}

	resetSubAgentSpawns(sid) // per-turn reset re-opens budget
	if ok, _ := reserveSubAgentSpawn(sid, 3); !ok {
		t.Fatal("after reset, spawn should succeed")
	}

	for i := 0; i < 50; i++ { // unlimited never refuses
		if ok, _ := reserveSubAgentSpawn("leash-sess-unl", -1); !ok {
			t.Fatal("unlimited should never refuse")
		}
	}

	resetSubAgentSpawns("s1")
	resetSubAgentSpawns("s2")
	reserveSubAgentSpawn("s1", 1)
	if ok, _ := reserveSubAgentSpawn("s2", 1); !ok { // independent budgets
		t.Fatal("s2 budget should be independent of s1")
	}
}

func TestLeashLadder(t *testing.T) {
	config.SetMaxSubAgents(25)
	if got := config.StepMaxSubAgents(-1); got != 12 {
		t.Errorf("down from 25 = %d, want 12", got)
	}
	if got := config.StepMaxSubAgents(+1); got != 25 {
		t.Errorf("up from 12 = %d, want 25", got)
	}
	config.SetMaxSubAgents(1)
	if got := config.StepMaxSubAgents(-1); got != config.SubAgentsNuclear {
		t.Errorf("down from 1 = %d, want Nuclear(0)", got)
	}
	config.SetMaxSubAgents(config.SubAgentsUnlimited)
	if got := config.StepMaxSubAgents(+1); got != config.SubAgentsUnlimited {
		t.Errorf("up from unlimited = %d, want unlimited(-1)", got)
	}
	config.SetMaxSubAgents(config.DefaultMaxSubAgents)
}
