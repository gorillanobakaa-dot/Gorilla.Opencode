package agent

// GORILLA (2026-10-21): the owner's own rule for running Fieldkit's read-only
// tools without the question.
//
// MEASURED 2026-10-19: every fieldkit run of a read-only tool asked, and
// "Allow for session" covered only that exact call (GrantKey is the whole
// input), so a search through Fieldkit was a stream of approvals. Widening the
// grant itself was refused: a grant the model can steer is a grant the model
// can widen. This is the other road, and it is the OWNER's:
//
//   - The owner writes "run:read-only" into the fieldkit server's readOnlyTools
//     in config.json. Folder settings cannot set mcpServers, so no project can.
//   - Only for a server marked "trust": "local", and only for a call that does
//     not declare "network": true.
//   - Only for a tool the EMBEDDED catalogue (generated from Fieldkit's reviewed
//     public cards, internal/fieldkit/catalogue.json) calls read-only and
//     offline. The server's own word is not taken; a tool not in the catalogue
//     (the owner's local cards, a recipe, a name the model made up) asks.
//
// The answer still goes through runLocalTool's taint check.

import (
	"encoding/json"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/fieldkit"
)

// readOnlyRunRule is the readOnlyTools entry that turns this on.
const readOnlyRunRule = "run:read-only"

// fieldkitReadOnlyRun reports whether this call is a Fieldkit run the owner's
// rule lets through without asking. Fail closed on everything else.
func fieldkitReadOnlyRun(server, tool string, cfg config.MCPServer, input string) bool {
	if server != "fieldkit" || tool != "run" || !cfg.RunsWithoutAsking(readOnlyRunRule, declaresNetwork(input)) {
		return false
	}
	var args struct {
		Tool string `json:"tool"`
	}
	if json.Unmarshal([]byte(input), &args) != nil || args.Tool == "" {
		return false
	}
	c, err := fieldkit.Load()
	if err != nil {
		return false
	}
	for _, t := range c.Tools {
		if t.ID == args.Tool {
			return t.Safety == "read-only" && !t.Online
		}
	}
	return false
}
