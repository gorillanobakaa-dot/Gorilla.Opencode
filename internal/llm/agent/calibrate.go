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
func toolTokens(t tools.BaseTool) int { return infoTokens(t.Info()) }

// infoTokens is the same measurement against an explicit schema, so a tool
// whose Info() varies with configuration can be measured in a known state.
func infoTokens(i tools.ToolInfo) int {
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
	set("tool.websearch", tools.NewWebSearchTool(permissions))
	set("tool.find", tools.NewFindTool())
	set("tool.view", tools.NewViewTool(lspClients))
	set("tool.patch", tools.NewPatchTool(lspClients, permissions, history))
	set("tool.write", tools.NewWriteTool(lspClients, permissions, history))
	set("tool.agent", NewAgentTool(sessions, messages, lspClients, permissions))
	// GORILLA FIX (2026-08-17): measure the research tool WITHOUT the dossier
	// addition, and the dossier row as that addition alone. Measuring Info()
	// here counted the dossier's tokens in BOTH rows whenever it was armed.
	if rt, ok := NewResearchTool(sessions, messages, lspClients, permissions).(*researchTool); ok {
		config.SetLoadoutTokens("tool.research", infoTokens(rt.infoBase()))
	}
	// The dossier row's cost is the MARGINAL schema the research tool gains
	// when it is armed — measured from the actual strings, not guessed.
	config.SetLoadoutTokens(config.DossierComponentID, DossierSchemaTokens())
	set("tool.sparse", tools.NewSparseTool(permissions))
	set("tool.review", tools.NewReviewTool(permissions))
	set("tool.patch_port", tools.NewPatchPortTool(permissions))
	set("tool.bio_lookup", tools.NewBioDataTool(permissions))
	// GORILLA OVERRIDE: measure diagnostics unconditionally. This was guarded on
	// having LSP clients, but the tool's SCHEMA is static — the clients only affect
	// what it returns at call time, not what it costs to declare. With every
	// language server switched off (a supported and common setup) the guard left
	// /context showing the hand-written estimate for this one row while every other
	// row showed a measured value, which is the worst of both.
	set("tool.diagnostics", tools.NewDiagnosticsTool(lspClients))

	// Base system prompt (always on) and the switchable env/lsp blocks.
	// Measure the full prompt, then the marginal cost of each block by
	// difference isn't trivial here; instead record the assembled prompt
	// as the base and keep env/lsp as their standalone sizes.
	base := prompt.BaseCoderPrompt(models.ProviderLocal)
	config.SetBasePromptTokens(len(base) / 4)
	config.SetLoadoutTokens("prompt.env", len(prompt.EnvironmentInfoBlock())/4)
	// GORILLA FIX (2026-08-19): line-gated rows need measuring too. Rows gated
	// per SECTION are measured by section; prompt.localtools is gated per LINE
	// and had nothing measuring it, so it would have shipped displaying a
	// hand-typed guess on the one screen built not to do that. The sentinel in
	// calibrate_test.go caught it on the first run.
	config.SetLoadoutTokens("prompt.localtools", prompt.GatedLineTokens("prompt.localtools"))
	config.SetLoadoutTokens("prompt.lsp", len(prompt.LSPInfoBlock())/4)
}
