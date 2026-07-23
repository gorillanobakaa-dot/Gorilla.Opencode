package agent

import (
	"context"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/history"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/lsp"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/session"
)

// GORILLA OVERRIDE: every tool is switchable via the context loadout
// (see internal/config/loadout.go and the /context menu). Each tool's
// description rides every turn, so turning a tool off is a real token
// saving — at the cost of that capability.
func loadoutOn(id string) bool { return config.LoadoutEnabled(id) }

func CoderAgentTools(
	permissions permission.Service,
	sessions session.Service,
	messages message.Service,
	history history.Service,
	lspClients map[string]*lsp.Client,
) []tools.BaseTool {
	ctx := context.Background()
	otherTools := GetMcpTools(ctx, permissions)
	if len(lspClients) > 0 && loadoutOn("tool.diagnostics") {
		otherTools = append(otherTools, tools.NewDiagnosticsTool(lspClients))
	}
	var coderTools []tools.BaseTool
	add := func(id string, t tools.BaseTool) {
		if loadoutOn(id) {
			coderTools = append(coderTools, t)
		}
	}
	add("tool.bash", tools.NewBashTool(permissions))
	add("tool.edit", tools.NewEditTool(lspClients, permissions, history))
	add("tool.fetch", tools.NewFetchTool(permissions))
	add("tool.glob", tools.NewGlobTool())
	add("tool.grep", tools.NewGrepTool())
	add("tool.ls", tools.NewLsTool())
	add("tool.view", tools.NewViewTool(lspClients))
	add("tool.patch", tools.NewPatchTool(lspClients, permissions, history))
	add("tool.write", tools.NewWriteTool(lspClients, permissions, history))
	// GORILLA OVERRIDE: on the Nuclear Option (helper-leash = 0) omit the
	// agent tool entirely, so its schema tokens vanish too — not just its
	// spawns (which subagent_guard.go would refuse anyway).
	if config.MaxSubAgents() != config.SubAgentsNuclear {
		add("tool.agent", NewAgentTool(sessions, messages, lspClients, permissions))
	}
	add("tool.sourcegraph", tools.NewSourcegraphTool(permissions))
	return append(coderTools, otherTools...)
}

func TaskAgentTools(lspClients map[string]*lsp.Client, permissions permission.Service) []tools.BaseTool {
	var taskTools []tools.BaseTool
	add := func(id string, t tools.BaseTool) {
		if loadoutOn(id) {
			taskTools = append(taskTools, t)
		}
	}
	add("tool.glob", tools.NewGlobTool())
	add("tool.grep", tools.NewGrepTool())
	add("tool.ls", tools.NewLsTool())
	add("tool.view", tools.NewViewTool(lspClients))
	add("tool.sourcegraph", tools.NewSourcegraphTool(permissions))
	return taskTools
}
