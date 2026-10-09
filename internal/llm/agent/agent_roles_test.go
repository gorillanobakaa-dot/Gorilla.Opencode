package agent

// GORILLA OVERRIDE (2026-10-09): the agent tool's explore / plan / coder roles.
//
// The read-only promise of explore and plan is a property of the TOOL LIST, so
// that is what is checked, by name. The coder role's promises — no recursion,
// the coder's model, visible in /tasks, inside the leash — are checked by
// running the real tool end to end with a stand-in model.

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/lsp"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/session"
)

var writeToolNames = []string{tools.BashToolName, tools.EditToolName, tools.WriteToolName, tools.PatchToolName}

func toolNames(ts []tools.BaseTool) map[string]bool {
	out := map[string]bool{}
	for _, t := range ts {
		out[t.Info().Name] = true
	}
	return out
}

func loadRoleConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", "test-key-for-a-loaded-config")
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
}

func TestAgentRoleExploreHasNoWriteTools(t *testing.T) {
	loadRoleConfig(t)
	got := toolNames((&agentTool{}).roleTools(AgentRoleExplore))
	for _, w := range writeToolNames {
		if got[w] {
			t.Errorf("explore helper has %q; it must be read-only", w)
		}
	}
	if !got[tools.FindToolName] || !got[tools.ViewToolName] {
		t.Errorf("explore helper lacks find/view: %v", got)
	}
	if name, _ := roleAgent(AgentRoleExplore); name != config.AgentTask {
		t.Errorf("explore runs as %q, want %q (today's helper)", name, config.AgentTask)
	}
}

func TestAgentRolePlanIsReadOnlyAndAsksForAPlan(t *testing.T) {
	loadRoleConfig(t)
	// A language server present: plan gets diagnostics, still nothing that writes.
	lsps := map[string]*lsp.Client{"go": nil}
	got := toolNames((&agentTool{lspClients: lsps}).roleTools(AgentRolePlan))
	for _, w := range writeToolNames {
		if got[w] {
			t.Errorf("plan helper has %q; it must never write", w)
		}
	}
	if !got[tools.DiagnosticsToolName] {
		t.Errorf("plan helper lacks diagnostics with a language server present: %v", got)
	}
	if toolNames((&agentTool{}).roleTools(AgentRolePlan))[tools.DiagnosticsToolName] {
		t.Error("plan helper offers diagnostics with no language server to answer")
	}

	name, _ := roleAgent(AgentRolePlan)
	if name != config.AgentPlan {
		t.Fatalf("plan runs as %q, want %q", name, config.AgentPlan)
	}
	if err := config.DeriveRoleAgent(name); err != nil {
		t.Fatalf("DeriveRoleAgent: %v", err)
	}
	if got, want := config.Get().Agents[name].Model, config.Get().Agents[config.AgentTask].Model; got != want {
		t.Errorf("unconfigured plan agent is on %q, want the task helper's %q", got, want)
	}
	sys := prompt.GetAgentPrompt(name, models.ProviderGemini)
	for _, want := range []string{"plan", "numbered", "never write"} {
		if !strings.Contains(sys, want) {
			t.Errorf("plan helper's system prompt does not say %q", want)
		}
	}
}

func TestAgentRoleCoderHasWriteToolsButCannotSpawnHelpers(t *testing.T) {
	loadRoleConfig(t)
	config.SetMaxSubAgents(config.DefaultMaxSubAgents)
	got := toolNames((&agentTool{}).roleTools(AgentRoleCoder))
	for _, w := range []string{tools.BashToolName, tools.EditToolName, tools.WriteToolName} {
		if !got[w] {
			t.Errorf("coder helper lacks %q", w)
		}
	}
	for _, spawner := range []string{AgentToolName, ResearchToolName} {
		if got[spawner] {
			t.Errorf("coder helper has %q: helpers could spawn helpers", spawner)
		}
	}
	// Non-vacuous: in the same loadout the MAIN coder does have the agent tool,
	// so its absence above is the filter, not the loadout.
	if !toolNames(CoderAgentTools(nil, nil, nil, nil, nil))[AgentToolName] {
		t.Fatal("the main coder has no agent tool in this loadout; the check above proves nothing")
	}
}

// The sub-coder must run on the coder's model, never the light helper model,
// and follow the coder when the coder is moved — unless the user configured it.
func TestSubCoderRunsOnTheCoderModel(t *testing.T) {
	loadRoleConfig(t)
	cfg := config.Get()
	saved := map[config.AgentName]config.Agent{}
	for k, v := range cfg.Agents {
		saved[k] = v
	}
	t.Cleanup(func() { cfg.Agents = saved })

	const main, light = models.ModelID("test.main-model"), models.ModelID("test.light-model")
	cfg.Agents[config.AgentCoder] = config.Agent{Model: main, MaxTokens: 4000, ReasoningEffort: "high"}
	cfg.Agents[config.AgentTask] = config.Agent{Model: light, MaxTokens: 1000}
	delete(cfg.Agents, config.AgentSubCoder)

	if err := config.DeriveRoleAgent(config.AgentSubCoder); err != nil {
		t.Fatalf("DeriveRoleAgent: %v", err)
	}
	if got := cfg.Agents[config.AgentSubCoder]; got != cfg.Agents[config.AgentCoder] {
		t.Errorf("derived sub-coder = %+v, want the coder's %+v", got, cfg.Agents[config.AgentCoder])
	}

	// The coder moves; a derived sub-coder follows on the next spawn.
	const moved = models.ModelID("test.moved-model")
	cfg.Agents[config.AgentCoder] = config.Agent{Model: moved, MaxTokens: 4000}
	if err := config.DeriveRoleAgent(config.AgentSubCoder); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Agents[config.AgentSubCoder].Model; got != moved {
		t.Errorf("derived sub-coder stayed on %q after the coder moved to %q", got, moved)
	}

	// A configured entry is the user's and is left alone.
	cfg2 := config.Agent{Model: "test.users-choice", MaxTokens: 10}
	cfg.Agents[config.AgentPlan] = cfg2
	if err := config.DeriveRoleAgent(config.AgentPlan); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Agents[config.AgentPlan]; got != cfg2 {
		t.Errorf("a configured plan agent was overwritten: %+v", got)
	}

	// No parent, no guess.
	delete(cfg.Agents, config.AgentCoder)
	delete(cfg.Agents, config.AgentSubCoder)
	if err := config.DeriveRoleAgent(config.AgentSubCoder); err == nil {
		t.Error("a sub-coder was derived with no coder configured")
	}
}

// --- whole-tool runs with a stand-in model ---------------------------------

// taskSessions adds CreateTaskSession to the in-memory store.
type taskSessions struct{ *memSessions }

func (s taskSessions) CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (session.Session, error) {
	return s.Save(ctx, session.Session{ID: "helper-" + toolCallID, ParentSessionID: parentSessionID, Title: title})
}

type spawnRecord struct {
	mu         sync.Mutex
	spawned    int
	agentName  config.AgentName
	toolNames  map[string]bool
	registered []SubAgentInfo // the registry, as seen from INSIDE the helper's turn
}

// standInHelpers replaces the helper builder for one test: the helper is a
// real agent loop driven by a scripted model that calls one probe tool, and
// the probe snapshots the sub-agent registry while the helper is running.
func standInHelpers(t *testing.T) *spawnRecord {
	t.Helper()
	rec := &spawnRecord{}
	orig := newHelperAgent
	t.Cleanup(func() { newHelperAgent = orig })
	newHelperAgent = func(name config.AgentName, sessions session.Service, messages message.Service, ts []tools.BaseTool) (Service, error) {
		rec.mu.Lock()
		rec.spawned++
		rec.agentName = name
		rec.toolNames = toolNames(ts)
		rec.mu.Unlock()
		probe := &scriptedTool{name: "probe_registry", do: func() (tools.ToolResponse, error) {
			rec.mu.Lock()
			rec.registered = ListSubAgents()
			rec.mu.Unlock()
			return tools.NewTextResponse("seen"), nil
		}}
		p := &scriptedProvider{script: []reply{
			{calls: []message.ToolCall{call("probe-1", "probe_registry")}},
			{text: "changed /tmp/x.go; ran go test: ok"},
		}}
		return &agent{
			Broker:    pubsub.NewBroker[AgentEvent](),
			agentName: name,
			sessions:  sessions,
			messages:  messages,
			tools:     append(ts, probe),
			provider:  p,
		}, nil
	}
	return rec
}

func runAgentTool(t *testing.T, tool tools.BaseTool, parent, callID string, params AgentParams) tools.ToolResponse {
	t.Helper()
	in, _ := json.Marshal(params)
	ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, parent)
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "parent-msg-"+callID)
	resp, err := tool.Run(ctx, tools.ToolCall{ID: callID, Name: AgentToolName, Input: string(in)})
	if err != nil {
		t.Fatalf("agent tool run: %v", err)
	}
	return resp
}

func newRoleTool() tools.BaseTool {
	return NewAgentTool(taskSessions{&memSessions{}}, &memMessages{}, nil, permission.NewPermissionService(), nil)
}

func TestAgentRoleCoderRunsRegisteredWithItsRole(t *testing.T) {
	loadRoleConfig(t)
	config.SetMaxSubAgents(config.SubAgentsUnlimited)
	t.Cleanup(func() { config.SetMaxSubAgents(config.DefaultMaxSubAgents) })
	rec := standInHelpers(t)

	resp := runAgentTool(t, newRoleTool(), "parent-coder", "call-coder", AgentParams{Prompt: "fix the bug", Role: "coder"})
	if resp.IsError {
		t.Fatalf("coder helper refused: %s", resp.Content)
	}
	if !strings.Contains(resp.Content, "changed /tmp/x.go") {
		t.Errorf("the helper's report did not come back: %q", resp.Content)
	}
	if rec.agentName != config.AgentSubCoder {
		t.Errorf("coder helper ran as %q, want %q", rec.agentName, config.AgentSubCoder)
	}
	if !rec.toolNames[tools.EditToolName] || rec.toolNames[AgentToolName] {
		t.Errorf("coder helper tools wrong (want edit, not agent): %v", rec.toolNames)
	}
	if got, want := config.Get().Agents[config.AgentSubCoder].Model, config.Get().Agents[config.AgentCoder].Model; got != want {
		t.Errorf("sub-coder entry is on %q, want the coder's %q", got, want)
	}

	var mine *SubAgentInfo
	for i := range rec.registered {
		if rec.registered[i].ParentSessionID == "parent-coder" {
			mine = &rec.registered[i]
		}
	}
	if mine == nil {
		t.Fatalf("the helper was not in the registry while it ran (/tasks would not show it): %+v", rec.registered)
	}
	if mine.Prompt != "[coder] fix the bug" {
		t.Errorf("registered prompt = %q, want it to lead with the role", mine.Prompt)
	}
	if mine.SessionID != "helper-call-coder" {
		t.Errorf("registered session = %q, want the helper's own session", mine.SessionID)
	}
}

func TestAgentRoleDefaultsToExplore(t *testing.T) {
	loadRoleConfig(t)
	config.SetMaxSubAgents(config.SubAgentsUnlimited)
	t.Cleanup(func() { config.SetMaxSubAgents(config.DefaultMaxSubAgents) })
	rec := standInHelpers(t)

	resp := runAgentTool(t, newRoleTool(), "parent-default", "call-default", AgentParams{Prompt: "where is X"})
	if resp.IsError {
		t.Fatalf("refused: %s", resp.Content)
	}
	if rec.agentName != config.AgentTask {
		t.Errorf("no role ran as %q, want %q", rec.agentName, config.AgentTask)
	}
	for _, w := range writeToolNames {
		if rec.toolNames[w] {
			t.Errorf("default helper has %q", w)
		}
	}
	for _, r := range rec.registered {
		if r.ParentSessionID == "parent-default" && r.Prompt != "[explore] where is X" {
			t.Errorf("registered prompt = %q", r.Prompt)
		}
	}
}

func TestAgentUnknownRoleIsRefusedWithTheValidList(t *testing.T) {
	loadRoleConfig(t)
	config.SetMaxSubAgents(1)
	t.Cleanup(func() { config.SetMaxSubAgents(config.DefaultMaxSubAgents) })
	rec := standInHelpers(t)
	const parent = "parent-unknown-role"
	resetSubAgentSpawns(parent)

	resp := runAgentTool(t, newRoleTool(), parent, "call-unknown", AgentParams{Prompt: "x", Role: "deployer"})
	if !resp.IsError {
		t.Fatal("an unknown role was accepted")
	}
	for _, want := range []string{"deployer", "explore", "plan", "coder"} {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("refusal %q does not name %q", resp.Content, want)
		}
	}
	if rec.spawned != 0 {
		t.Error("a helper was built for an unknown role")
	}
	// The typo must not have spent the user's one helper for this turn.
	if ok, used := reserveSubAgentSpawn(parent, 1); !ok || used != 1 {
		t.Errorf("an unknown role charged the leash (ok=%v used=%d)", ok, used)
	}
}

func TestAgentRolesStayInsideTheLeash(t *testing.T) {
	loadRoleConfig(t)
	t.Cleanup(func() { config.SetMaxSubAgents(config.DefaultMaxSubAgents) })
	rec := standInHelpers(t)
	tool := newRoleTool()

	const parent = "parent-leash"
	resetSubAgentSpawns(parent)
	config.SetMaxSubAgents(1)
	if resp := runAgentTool(t, tool, parent, "call-l1", AgentParams{Prompt: "one", Role: "plan"}); resp.IsError {
		t.Fatalf("first helper refused: %s", resp.Content)
	}
	resp := runAgentTool(t, tool, parent, "call-l2", AgentParams{Prompt: "two", Role: "coder"})
	if !resp.IsError || !strings.Contains(resp.Content, "limit reached") {
		t.Errorf("second helper with a leash of 1 was not refused: %q", resp.Content)
	}
	if rec.spawned != 1 {
		t.Errorf("%d helpers built with a leash of 1", rec.spawned)
	}

	config.SetMaxSubAgents(config.SubAgentsNuclear)
	resp = runAgentTool(t, tool, "parent-nuclear", "call-n1", AgentParams{Prompt: "x", Role: "coder"})
	if !resp.IsError || !strings.Contains(resp.Content, "DISABLED") {
		t.Errorf("coder helper ran under the Nuclear Option: %q", resp.Content)
	}
	if rec.spawned != 1 {
		t.Errorf("a helper was built under the Nuclear Option")
	}
}

func TestAgentToolDescriptionIsTrueForEveryRole(t *testing.T) {
	info := (&agentTool{}).Info()
	for _, r := range AgentRoles {
		if !strings.Contains(info.Description, r) {
			t.Errorf("description does not mention role %q", r)
		}
	}
	// The old line said the agent can not modify files. With role=coder it can.
	if strings.Contains(info.Description, "can not modify files") {
		t.Error("description still says helpers cannot modify files")
	}
	if !strings.Contains(info.Description, "permission") {
		t.Error("description does not say permission questions still reach the user")
	}
	role, ok := info.Parameters["role"].(map[string]any)
	if !ok {
		t.Fatal("no role parameter in the schema")
	}
	if enum, _ := role["enum"].([]string); strings.Join(enum, ",") != "explore,plan,coder" {
		t.Errorf("role enum = %v", role["enum"])
	}
}
