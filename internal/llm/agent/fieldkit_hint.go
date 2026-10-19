package agent

// GORILLA (2026-10-19): put the Fieldkit tools that fit a request in front of
// the model, matched in-process (internal/fieldkit, milliseconds, no server
// start). Gemma 4 E2B, measured that day, answered "no such tool" for the TV,
// an Office file and the cooling profile without ever calling discover.

import (
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/fieldkit"
	"github.com/opencode-ai/opencode/internal/logging"
)

// fieldkitHintLimit is how many tools the hint names: enough to cover a family
// (hdmi-status, hdmi-diag...), few enough that a small model reads them all.
const fieldkitHintLimit = 5

// fieldkitHint returns the hint for a request, or "" when no Fieldkit MCP
// server is configured (the model could not call the tools) or nothing fits.
func fieldkitHint(request string) string {
	cfg := config.Get()
	if cfg == nil {
		return ""
	}
	if _, ok := cfg.MCPServers["fieldkit"]; !ok {
		return ""
	}
	c, err := fieldkit.Load()
	if err != nil {
		logging.Warn("fieldkit catalogue unreadable; no hint", "error", err)
		return ""
	}
	return fieldkit.Hint(c, request, fieldkitHintLimit)
}
