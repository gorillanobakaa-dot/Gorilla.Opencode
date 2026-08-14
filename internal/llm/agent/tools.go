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
	// GORILLA OVERRIDE: without a search tool the agent hand-builds query
	// URLs for publisher sites, gets 403s, and has been observed fabricating
	// citations rather than reporting the failure.
	add("tool.websearch", tools.NewWebSearchTool(permissions))
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
		// GORILLA OVERRIDE: multi-role research. Gated on the same Nuclear
		// Option as the agent tool — it spawns helpers, so with helpers off its
		// schema tokens should go too.
		add("tool.research", NewResearchTool(sessions, messages, lspClients, permissions))
	}
	// GORILLA OVERRIDE: kernel semantic checker, default off in the loadout.
	add("tool.sparse", tools.NewSparseTool(permissions))
	return append(coderTools, otherTools...)
}

// ResearchAgentTools is what a research helper gets. It is TaskAgentTools plus
// the web, and that difference is the point: a helper told to find prior art or
// read a specification cannot do it with glob and grep. Still strictly
// read-only — no bash, edit, write or patch — so a helper can investigate
// anything and change nothing.
//
// GORILLA OVERRIDE: fetch and websearch are NOT loadout-gated here, unlike in
// CoderAgentTools. Their descriptions ride only the helper's own turns, never
// the main conversation, so they cost nothing when research is not running; and
// a research helper without them is a research helper that fabricates. That has
// been observed — see the websearch note in CoderAgentTools.
func ResearchAgentTools(lspClients map[string]*lsp.Client, permissions permission.Service) []tools.BaseTool {
	researchTools := []tools.BaseTool{
		tools.NewFetchTool(permissions),
		tools.NewWebSearchTool(permissions),
		tools.NewGlobTool(),
		tools.NewGrepTool(),
		tools.NewLsTool(),
		tools.NewViewTool(lspClients),
	}
	if len(lspClients) > 0 {
		researchTools = append(researchTools, tools.NewDiagnosticsTool(lspClients))
	}
	return researchTools
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
	return taskTools
}
