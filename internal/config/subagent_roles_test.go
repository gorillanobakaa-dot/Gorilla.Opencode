package config

import (
	"os"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/llm/models"
)

// GORILLA OVERRIDE (2026-10-09): the agent tool's role agents.

// The sub-coder edits on the user's behalf, so its fallback is the MAIN model,
// the coder's — never the cheap helper model the task agent gets. The planner
// is read-only and goes with the helpers.
func TestSubCoderDefaultIsTheMainModelNotTheHelperModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-key-for-a-default")
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	saved := map[AgentName]Agent{}
	for k, v := range cfg.Agents {
		saved[k] = v
	}
	t.Cleanup(func() { cfg.Agents = saved })

	for _, name := range []AgentName{AgentCoder, AgentTask, AgentSubCoder, AgentPlan} {
		if !setDefaultModelForAgent(name) {
			t.Fatalf("no default model for %s", name)
		}
	}
	coder, task := cfg.Agents[AgentCoder].Model, cfg.Agents[AgentTask].Model
	if coder == task {
		t.Fatalf("coder and task defaulted to the same model %q; this test cannot tell main from light", coder)
	}
	if got := cfg.Agents[AgentSubCoder].Model; got != coder {
		t.Errorf("sub-coder defaulted to %q, want the main model %q (task gets %q)", got, coder, task)
	}
	if got := cfg.Agents[AgentPlan].Model; got != task {
		t.Errorf("plan defaulted to %q, want the helper model %q", got, task)
	}
}

// Background agents follow the coder (FollowCoderModel). A CONFIGURED
// sub-coder follows like the rest; a DERIVED one is skipped, because moving it
// would write a default into config.json as if the user had chosen it.
func TestFollowCoderMovesAConfiguredSubCoderButNotADerivedOne(t *testing.T) {
	const old = models.ModelID("local.test/role-old")
	const new_ = models.ModelID("local.test/role-new")
	setAgents(t, old, old, old, old)
	registerModel(t, new_)
	t.Cleanup(func() {
		delete(cfg.Agents, AgentSubCoder)
		delete(cfg.Agents, AgentPlan)
	})

	cfg.Agents[AgentSubCoder] = Agent{Model: old, MaxTokens: 1}
	delete(cfg.Agents, AgentPlan)
	if err := DeriveRoleAgent(AgentPlan); err != nil {
		t.Fatalf("DeriveRoleAgent: %v", err)
	}
	if !isDerivedRoleAgent(AgentPlan) || isDerivedRoleAgent(AgentSubCoder) {
		t.Fatal("bookkeeping wrong: plan should be derived, sub-coder configured")
	}

	moves, err := FollowCoderModel(old, new_)
	if err != nil {
		t.Fatalf("FollowCoderModel: %v", err)
	}
	moved := map[AgentName]bool{}
	for _, m := range moves {
		moved[m.Agent] = true
	}
	if !moved[AgentSubCoder] || cfg.Agents[AgentSubCoder].Model != new_ {
		t.Error("a configured sub-coder was left on the old coder model")
	}
	if moved[AgentPlan] {
		t.Error("a derived plan agent was moved (and so written to config.json)")
	}
	// It still follows: re-derived from the task agent, which did move.
	if err := DeriveRoleAgent(AgentPlan); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Agents[AgentPlan].Model; got != new_ {
		t.Errorf("derived plan agent on %q after the move, want %q", got, new_)
	}
}

// Same trap as AgentResearch (research_agent_test.go): an entry written by
// the local-endpoint default loop would pin a role agent to a local model and
// stop it following the coder. With no entry it is derived from its parent,
// which that loop already covers.
func TestRoleAgentsAreNotDefaultedToALocalEndpoint(t *testing.T) {
	src, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i, line := range strings.Split(string(src), "\n") {
		if !strings.Contains(line, "range []AgentName{AgentCoder") {
			continue
		}
		found = true
		for _, name := range []string{"AgentSubCoder", "AgentPlan"} {
			if strings.Contains(line, name) {
				t.Errorf("config.go:%d puts %s in the local-endpoint default list", i+1, name)
			}
		}
	}
	if !found {
		t.Fatal("the local-endpoint default loop was renamed or removed; this test inspects nothing")
	}
}
