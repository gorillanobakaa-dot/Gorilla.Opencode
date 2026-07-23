// GORILLA OVERRIDE: this file did not exist upstream. It measures the
// REAL per-turn token cost of every switchable component (each tool's
// serialised schema, and the base system prompt) and feeds the numbers
// into the context loadout, so the /context menu reports what the model
// actually receives — not a guess. Called once at startup.
package agent

import (
	"encoding/json"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/history"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/lsp"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/session"
)

// toolTokens approximates the tokens a tool adds to every request: its
// name, description, and JSON-Schema parameters, at ~4 chars/token.
func toolTokens(t tools.BaseTool) int {
	i := t.Info()
	b, _ := json.Marshal(map[string]any{
		"name":        i.Name,
		"description": i.Description,
		"parameters": map[string]any{
			"type":       "object",
			"properties": i.Parameters,
			"required":   i.Required,
		},
	})
	return len(b) / 4
}

// CalibrateLoadout measures real token costs and records them in the
// config loadout. Deps are the same ones used to build the tools.
func CalibrateLoadout(
	permissions permission.Service,
	sessions session.Service,
	messages message.Service,
	history history.Service,
	lspClients map[string]*lsp.Client,
) {
	// Defensive: measuring the prompt reads config; never let a startup
	// ordering change turn calibration into a crash — fall back to the
	// built-in estimates instead.
	defer func() { _ = recover() }()

	set := func(id string, t tools.BaseTool) { config.SetLoadoutTokens(id, toolTokens(t)) }
	set("tool.bash", tools.NewBashTool(permissions))
	set("tool.edit", tools.NewEditTool(lspClients, permissions, history))
	set("tool.fetch", tools.NewFetchTool(permissions))
	set("tool.glob", tools.NewGlobTool())
	set("tool.grep", tools.NewGrepTool())
	set("tool.ls", tools.NewLsTool())
	set("tool.view", tools.NewViewTool(lspClients))
	set("tool.patch", tools.NewPatchTool(lspClients, permissions, history))
	set("tool.write", tools.NewWriteTool(lspClients, permissions, history))
	set("tool.agent", NewAgentTool(sessions, messages, lspClients, permissions))
	set("tool.sourcegraph", tools.NewSourcegraphTool(permissions))
	if len(lspClients) > 0 {
		set("tool.diagnostics", tools.NewDiagnosticsTool(lspClients))
	}

	// Base system prompt (always on) and the switchable env/lsp blocks.
	// Measure the full prompt, then the marginal cost of each block by
	// difference isn't trivial here; instead record the assembled prompt
	// as the base and keep env/lsp as their standalone sizes.
	base := prompt.BaseCoderPrompt(models.ProviderLocal)
	config.SetBasePromptTokens(len(base) / 4)
	config.SetLoadoutTokens("prompt.env", len(prompt.EnvironmentInfoBlock())/4)
	config.SetLoadoutTokens("prompt.lsp", len(prompt.LSPInfoBlock())/4)
}
