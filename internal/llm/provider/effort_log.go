package provider

import (
	"strings"

	"github.com/opencode-ai/opencode/internal/logging"
)

// GORILLA (2026-11-15): one debug line per outgoing request with the model id and the effort that goes with it, so a
// live call shows what was really sent (the model id alone did not). Only the provider, the model id and the effort
// word are logged; never a header, a token, a prompt or any other part of the body.

// effortInModelID is the effort word a backend model id ends with (Antigravity encodes it in the id:
// gemini-4-hydrogen-low), or "" when the id carries none.
func effortInModelID(id string) string {
	for _, e := range []string{"minimal", "low", "medium", "high", "xhigh", "max"} {
		if strings.HasSuffix(id, "-"+e) {
			return e
		}
	}
	return ""
}

// effortLine is the redacted description of an outgoing request: its key/value pairs for the debug log.
func effortLine(provider, model, effort, where string) []any {
	if effort == "" {
		effort = "none"
	}
	return []any{"provider", provider, "model", model, "effort", effort, "sent_as", where}
}

func logEffort(provider, model, effort, where string) {
	logging.Debug("request effort", effortLine(provider, model, effort, where)...)
}
