package agent

import (
	"context"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/history"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
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
	// GORILLA FIX (2026-08-19): MCP tools were never registered in the
	// loadout, so /context, the per-turn cost line and the burn-rate warning
	// were all computed over a tool list that excluded them. Someone with
	// three MCP servers was shown a number that was simply wrong — on the one
	// screen built to tell them what their setup costs. Registered here
	// because this is the first point at which the servers have been contacted
	// and their real schemas measured; the registry is idempotent by ID.
	config.RegisterLoadoutComponents(McpLoadoutComponents())
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
	// GORILLA OVERRIDE: one find tool replaces glob + grep + ls. Those three
	// carried ~1,485 tokens of description on EVERY turn, and grep could only
	// return PATHS — so answering any question cost a second turn and a
	// whole-file view (measured on this repo: a 16-token grep answer with an
	// 1,829-token view behind it). find returns matching lines with context,
	// so that second turn usually does not happen, and one tool instead of
	// three removes the tool-choice mistake that smaller models kept making on
	// large trees. The three tools still exist and still compile; restoring
	// them is three lines here. See internal/llm/tools/find.go.
	add("tool.find", tools.NewFindTool())
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
	// GORILLA OVERRIDE (2026-08-18): the code-review toolkit, ~30 analysers
	// embedded in the binary. Loadout-gated because its description is not
	// free and most turns are not reviews — but ON by default, because a
	// review capability nobody knows about is not a capability.
	add("tool.review", tools.NewReviewTool(permissions))
	// Ported patches are the other half of the embedded toolkit. Same
	// loadout gating and the same reason: most turns are not ports, but a
	// capability nobody can reach is not a capability.
	add("tool.patch_port", tools.NewPatchPortTool(permissions))
	add("tool.bio_lookup", tools.NewBioDataTool(permissions))

	full := append(coderTools, otherTools...)

	// tool_search is appended LAST and is never itself deferred: it is the only
	// way back to everything that is. It closes over `full`, so its catalogue is
	// the real toolset rather than a second list that could drift out of step
	// with this one.
	//
	// Deliberately NOT wrapped in add(): a loadout that could switch this off
	// while deferral stayed on would hide tools with no way to find them. The
	// deferral mechanism is gated instead, one row, in config.
	if config.LoadoutEnabled(config.ToolSearchComponentID) {
		snapshot := full
		full = append(full, tools.NewToolSearchTool(func() []tools.BaseTool { return snapshot }))
		// Tell the system prompt what it may advertise. Computed from the real
		// toolset, so a tool added above appears in the index without anyone
		// maintaining a second list — the kind of second list that goes stale
		// and leaves a tool undiscoverable.
		block := tools.DeferredCatalogueBlock(snapshot)
		prompt.SetDeferredCatalogue(block)

		// Tell /context which rows are enabled-but-withheld, so the per-turn
		// figure is what actually goes on the wire. Without this the screen
		// that exists to price a turn quotes the cost of schemas nobody sends.
		//
		// Mapped from tool NAME to loadout ID here, because this is the only
		// place that knows both.
		deferredIDs := map[string]bool{}
		for _, t := range snapshot {
			if !tools.IsDeferrable(t.Info().Name) {
				continue
			}
			if id, ok := loadoutIDForTool(t.Info().Name); ok {
				deferredIDs[id] = true
			}
		}
		config.SetDeferredComponents(deferredIDs, len(block)/4)
	} else {
		// Cleared, not left behind: advertising tools that are all loaded
		// anyway would tell the model to search for things it already has.
		prompt.SetDeferredCatalogue("")
		config.SetDeferredComponents(nil, 0)
	}
	return full
}

// loadoutIDForTool maps a tool's wire name to its /context row.
//
// A table rather than a naming rule, because the two genuinely differ:
// web_fetch is tool.fetch and web_search is tool.websearch. A rule would have
// silently failed on exactly those two and under-reported the saving.
func loadoutIDForTool(name string) (string, bool) {
	switch name {
	case tools.ReviewToolName:
		return "tool.review", true
	case tools.PatchPortToolName:
		return "tool.patch_port", true
	case tools.BioDataToolName:
		return "tool.bio_lookup", true
	case tools.SparseToolName:
		return "tool.sparse", true
	case tools.WebSearchToolName:
		return "tool.websearch", true
	case tools.FetchToolName:
		return "tool.fetch", true
	case AgentToolName:
		return "tool.agent", true
	}
	return "", false
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
		tools.NewFindTool(),
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
	add("tool.find", tools.NewFindTool())
	add("tool.view", tools.NewViewTool(lspClients))
	return taskTools
}

// registerDeferredComponents tells config which loadout rows are enabled but
// withheld, so /context can price a turn by what is actually sent.
//
// Called from both CoderAgentTools and CalibrateLoadout. Either alone would
// leave the figure depending on which ran first.
func registerDeferredComponents() {
	if !config.LoadoutEnabled(config.ToolSearchComponentID) {
		config.SetDeferredComponents(nil, 0)
		return
	}
	ids := map[string]bool{}
	for name := range deferredToolNames() {
		if id, ok := loadoutIDForTool(name); ok {
			ids[id] = true
		}
	}
	config.SetDeferredComponents(ids, len(prompt.DeferredCatalogue())/4)
}

// deferredToolNames is the set of tool names withheld by default.
func deferredToolNames() map[string]bool {
	out := map[string]bool{}
	for _, name := range []string{
		tools.ReviewToolName, tools.PatchPortToolName, tools.BioDataToolName,
		tools.SparseToolName, tools.WebSearchToolName, tools.FetchToolName,
		AgentToolName,
	} {
		if tools.IsDeferrable(name) {
			out[name] = true
		}
	}
	return out
}
