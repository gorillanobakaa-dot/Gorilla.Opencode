package agent

import (
	"context"
	"os"
	"strconv"

	"github.com/opencode-ai/opencode/internal/history"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/lsp"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/session"
)

// GORILLA OVERRIDE: the Sourcegraph tool (public-code web search) carries
// a large description that is sent to the model on every single turn, but
// most local development never uses it. It is opt-in via
// OPENCODE_SOURCEGRAPH=1 to keep the per-turn token overhead down.
func sourcegraphEnabled() bool {
	on, _ := strconv.ParseBool(os.Getenv("OPENCODE_SOURCEGRAPH"))
	return on
}

func CoderAgentTools(
	permissions permission.Service,
	sessions session.Service,
	messages message.Service,
	history history.Service,
	lspClients map[string]*lsp.Client,
) []tools.BaseTool {
	ctx := context.Background()
	otherTools := GetMcpTools(ctx, permissions)
	if len(lspClients) > 0 {
		otherTools = append(otherTools, tools.NewDiagnosticsTool(lspClients))
	}
	coderTools := []tools.BaseTool{
		tools.NewBashTool(permissions),
		tools.NewEditTool(lspClients, permissions, history),
		tools.NewFetchTool(permissions),
		tools.NewGlobTool(),
		tools.NewGrepTool(),
		tools.NewLsTool(),
		tools.NewViewTool(lspClients),
		tools.NewPatchTool(lspClients, permissions, history),
		tools.NewWriteTool(lspClients, permissions, history),
		NewAgentTool(sessions, messages, lspClients),
	}
	if sourcegraphEnabled() {
		coderTools = append(coderTools, tools.NewSourcegraphTool())
	}
	return append(coderTools, otherTools...)
}

func TaskAgentTools(lspClients map[string]*lsp.Client) []tools.BaseTool {
	taskTools := []tools.BaseTool{
		tools.NewGlobTool(),
		tools.NewGrepTool(),
		tools.NewLsTool(),
		tools.NewViewTool(lspClients),
	}
	if sourcegraphEnabled() {
		taskTools = append(taskTools, tools.NewSourcegraphTool())
	}
	return taskTools
}
