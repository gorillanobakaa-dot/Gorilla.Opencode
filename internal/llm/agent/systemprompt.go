package agent

import (
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/prompt"
)

// CoderSystemPrompt is the system prompt the coder model is sent, built the way
// a session builds it: the coder's tool set first (which writes the "Your
// tools" table and the deferred catalogue into the prompt), then the prompt for
// the configured coder model. `gorilla-opencode system-prompt` prints it.
//
// GORILLA (2026-10-27, job 06): so anyone, the verifier included, can read the
// exact prompt on their own machine instead of trusting a description of it.
// With skipMCP the MCP servers are not started and their tools are left out;
// everything else is the real tool set. The services are nil: building a tool
// only stores them, and nothing here runs a tool.
func CoderSystemPrompt(skipMCP bool) string {
	if skipMCP {
		mcpLoadMu.Lock()
		mcpLoaded = true
		mcpLoadMu.Unlock()
	}
	CoderAgentTools(nil, nil, nil, nil, nil)
	return prompt.GetAgentPrompt(config.AgentCoder, "")
}
