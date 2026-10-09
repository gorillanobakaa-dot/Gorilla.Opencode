package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/history"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/lsp"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/session"
)

type agentTool struct {
	sessions    session.Service
	messages    message.Service
	history     history.Service
	lspClients  map[string]*lsp.Client
	permissions permission.Service
}

const (
	AgentToolName = "agent"
)

// GORILLA OVERRIDE (2026-10-09): role-based helpers, after Kimi Code's
// built-in coder / explore / plan sub-agents. Every role runs in its own
// session with its own context, so a helper's reading never lands in the
// parent's window — only its one final report does.
//
//	explore  read-only (find, view); returns findings. The default, and
//	         exactly what this tool did before roles existed.
//	plan     read-only plus diagnostics; returns a numbered, file-by-file plan.
//	coder    the coder's own tools minus the helper spawners, on the coder's
//	         model; it changes files, and every permission question it raises
//	         still goes to the user through the parent conversation.
const (
	AgentRoleExplore = "explore"
	AgentRolePlan    = "plan"
	AgentRoleCoder   = "coder"
)

// AgentRoles is the valid list, in the order it is shown to the model.
var AgentRoles = []string{AgentRoleExplore, AgentRolePlan, AgentRoleCoder}

type AgentParams struct {
	Prompt string `json:"prompt"`
	Role   string `json:"role,omitempty"`
}

// newHelperAgent builds a helper. A variable only so a test can run the whole
// tool — leash, session, registry, cost — with a stand-in model; production
// never reassigns it.
var newHelperAgent = NewAgent

// roleAgent maps a role to the configured agent it runs as (model, provider
// and system prompt). False for an unknown role.
func roleAgent(role string) (config.AgentName, bool) {
	switch role {
	case AgentRoleExplore:
		return config.AgentTask, true
	case AgentRolePlan:
		return config.AgentPlan, true
	case AgentRoleCoder:
		return config.AgentSubCoder, true
	}
	return "", false
}

// roleTools is the tool set a role's helper gets. Built only after the role is
// known to be valid and the leash has allowed the spawn: the coder set reads
// the MCP tool list, which is not free on first use.
func (b *agentTool) roleTools(role string) []tools.BaseTool {
	switch role {
	case AgentRolePlan:
		return PlanAgentTools(b.lspClients, b.permissions)
	case AgentRoleCoder:
		return SubCoderAgentTools(b.permissions, b.sessions, b.messages, b.history, b.lspClients)
	}
	return TaskAgentTools(b.lspClients, b.permissions)
}

func (b *agentTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name: AgentToolName,
		Description: "Launch a helper agent in its own session. `role` sets what it can do:\n" +
			"- explore (default): read-only search (find, view). Use it for open-ended searches where the first match may be wrong; for a known path or symbol call view or find directly.\n" +
			"- plan: read-only plus diagnostics. Returns a numbered, file-by-file plan with the exact places to change and what to verify. It never writes.\n" +
			"- coder: your coding tools (edit, write, patch, bash and the rest, but not agent or research) on your model. It CAN modify files; every permission question it raises still goes to the user. It reports what it changed and what it ran.\n\n" +
			"Usage notes:\n" +
			"1. Helpers run ONE AT A TIME, in the order requested. Batching calls saves model round-trips, not time.\n" +
			"2. A helper returns one final message, visible only to you. To show the user, summarise it.\n" +
			"3. Each helper is stateless and cannot be asked follow-ups: give a complete task and say exactly what to report back.\n" +
			"4. Explore and plan findings can generally be trusted. Check a coder helper's report before telling the user the work is done.",
		Parameters: map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The task for the agent to perform",
			},
			"role": map[string]any{
				"type":        "string",
				"enum":        AgentRoles,
				"description": "explore (default), plan or coder",
			},
		},
		Required: []string{"prompt"},
	}
}

func (b *agentTool) Run(ctx context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
	var params AgentParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}
	if params.Prompt == "" {
		return tools.NewTextErrorResponse("prompt is required"), nil
	}
	role := strings.ToLower(strings.TrimSpace(params.Role))
	if role == "" {
		role = AgentRoleExplore
	}
	// Refused BEFORE the leash is charged: a mistyped role must not spend one
	// of the user's helper slots for this turn.
	agentName, ok := roleAgent(role)
	if !ok {
		return tools.NewTextErrorResponse(fmt.Sprintf("unknown role %q. Valid roles: %s.", params.Role, strings.Join(AgentRoles, ", "))), nil
	}

	sessionID, messageID := tools.GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return tools.ToolResponse{}, fmt.Errorf("session_id and message_id are required")
	}

	// GORILLA OVERRIDE: enforce the user's helper-leash (config.MaxSubAgents,
	// "Dial 2" in /context). A refusal is returned as a normal tool result so
	// the model adapts (does the work inline) rather than erroring the turn.
	// Every role counts against the same leash.
	switch limit := config.MaxSubAgents(); {
	case limit == config.SubAgentsNuclear:
		return tools.NewTextErrorResponse("Sub-agents are DISABLED (Gorilla Nuclear Option). Do this task yourself with the direct tools, or the user can re-enable helpers in /context."), nil
	case limit != config.SubAgentsUnlimited:
		if ok, used := reserveSubAgentSpawn(sessionID, limit); !ok {
			return tools.NewTextErrorResponse(fmt.Sprintf("Helper-agent limit reached for this turn (%d used of %d allowed). Continue this task with the direct tools instead of spawning another helper.", used, limit)), nil
		}
	}

	// plan and coder have no entry in a config.json written before roles
	// existed; derive it from the parent agent now (no-op for explore).
	if err := config.DeriveRoleAgent(agentName); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("cannot start a %s helper: %s", role, err)), nil
	}

	agent, err := newHelperAgent(agentName, b.sessions, b.messages, b.roleTools(role))
	if err != nil {
		return tools.ToolResponse{}, fmt.Errorf("error creating agent: %s", err)
	}

	session, err := b.sessions.CreateTaskSession(ctx, call.ID, sessionID, "New Agent Session")
	// Same reason as the research helpers: a sub-agent's approvals belong to
	// the conversation that spawned it, not to a session the user cannot see.
	// For role=coder this is what keeps every edit and command in front of the
	// user.
	if err == nil {
		b.permissions.RegisterChildSession(session.ID, sessionID)
	}
	if err != nil {
		return tools.ToolResponse{}, fmt.Errorf("error creating session: %s", err)
	}

	// GORILLA OVERRIDE: register this helper so the user can SEE it (/tasks,
	// status bar) and KILL it — one by one or via the Nuclear Option. The
	// helper runs under its own cancelable context; killing it cancels that
	// context, which unblocks the <-done wait below with a cancellation error.
	// The role leads the registered prompt, so /tasks says which kind of
	// helper is running: one that edits must not look like a search.
	taskCtx, taskCancel := context.WithCancel(ctx)
	defer taskCancel()
	entry := RegisterSubAgent(session.ID, sessionID, call.ID, "["+role+"] "+params.Prompt, taskCancel)
	defer UnregisterSubAgent(entry.ID)

	done, err := agent.Run(taskCtx, session.ID, params.Prompt)
	if err != nil {
		return tools.ToolResponse{}, fmt.Errorf("error generating agent: %s", err)
	}
	result := <-done
	if result.Error != nil {
		return tools.ToolResponse{}, fmt.Errorf("error generating agent: %s", result.Error)
	}

	response := result.Message
	if response.Role != message.Assistant {
		return tools.NewTextErrorResponse("no response"), nil
	}

	updatedSession, err := b.sessions.Get(ctx, session.ID)
	if err != nil {
		return tools.ToolResponse{}, fmt.Errorf("error getting session: %s", err)
	}
	parentSession, err := b.sessions.Get(ctx, sessionID)
	if err != nil {
		return tools.ToolResponse{}, fmt.Errorf("error getting parent session: %s", err)
	}

	parentSession.Cost += updatedSession.Cost

	_, err = b.sessions.Save(ctx, parentSession)
	if err != nil {
		return tools.ToolResponse{}, fmt.Errorf("error saving parent session: %s", err)
	}
	return tools.NewTextResponse(response.Content().String()), nil
}

func NewAgentTool(
	Sessions session.Service,
	Messages message.Service,
	LspClients map[string]*lsp.Client,
	Permissions permission.Service,
	History history.Service,
) tools.BaseTool {
	return &agentTool{
		sessions:    Sessions,
		messages:    Messages,
		history:     History,
		lspClients:  LspClients,
		permissions: Permissions,
	}
}
