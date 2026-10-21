package tools

import (
	"context"

	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/usage"
)

// UsageToolName is the usage tool's wire name.
const UsageToolName = "usage"

// usageTool hands the AI what /usage and the status bar show the person: this
// session's cost ledger and the allowance left with the provider in use.
//
// GORILLA (2026-10-21): without it, "how many bananas have I got left?" took
// Gemini 3.8 Flash 30 tool calls, a timed-out search of the home folder, an
// unasked read of the Antigravity sign-in file and SQL against the live session
// database; each call re-sent the whole conversation. With it, the question is
// one call. Read-only, no parameters, no files; see internal/usage for the
// doors (the allowance read is the provider already in use, cached a minute).
type usageTool struct {
	sessions session.Service
}

// NewUsageTool builds the usage tool over the session store.
func NewUsageTool(sessions session.Service) BaseTool { return usageTool{sessions: sessions} }

func (usageTool) Info() ToolInfo {
	return ToolInfo{
		Name: UsageToolName,
		Description: "What this session has cost so far and how much allowance or balance is left " +
			"(what the person sees with /usage). Use it for ANY question about usage, quota, tokens, " +
			"cost, spend, credits, limits or \"bananas\". One call answers; never search files, " +
			"settings or databases for this. Read-only.",
		Parameters: map[string]any{},
	}
}

func (t usageTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var s session.Session
	if id, _ := ctx.Value(SessionIDContextKey).(string); id != "" && t.sessions != nil {
		if got, err := t.sessions.Get(ctx, id); err == nil {
			s = got
		}
	}
	return NewTextResponse(usage.Report(ctx, s)), nil
}
