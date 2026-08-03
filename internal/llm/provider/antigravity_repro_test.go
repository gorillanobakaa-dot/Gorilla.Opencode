package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

type reproTool struct{}

func (reproTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name:        "list_files",
		Description: "List files in a directory.",
		Parameters:  map[string]any{"path": map[string]any{"type": "string", "description": "dir"}},
		Required:    []string{"path"},
	}
}
func (reproTool) Run(context.Context, tools.ToolCall) (tools.ToolResponse, error) {
	return tools.ToolResponse{}, nil
}

// Reproduces the reported failure: Gemini works but Claude/GPT-OSS error on
// real (multi-turn) use. Isolates single-vs-multi-turn and with-vs-without
// tools using the user's real signed-in creds. AG_LIVE=1 to run.
func TestAntigravityReproMatrix(t *testing.T) {
	if os.Getenv("AG_LIVE") != "1" {
		t.Skip("set AG_LIVE=1 to run against the live backend with your creds")
	}
	// Read the REAL creds file directly — this package's TestMain overrides
	// XDG_CONFIG_HOME to a temp dir, so auth.LoadAntigravityCreds would miss it.
	home, _ := os.UserHomeDir()
	data, rerr := os.ReadFile(filepath.Join(home, ".config", "gorilla-opencode", "antigravity-oauth.json"))
	if rerr != nil {
		t.Skip("no Antigravity creds on disk; sign in via the portal first")
	}
	creds := &auth.AntigravityCreds{}
	if err := json.Unmarshal(data, creds); err != nil {
		t.Fatalf("parse creds: %v", err)
	}
	if creds.AccessToken == "" {
		t.Skip("empty creds")
	}

	client := func() *antigravityClient {
		return &antigravityClient{
			providerOptions: providerClientOptions{
				model:         models.AntigravityModels[models.AGClaudeSonnet46],
				maxTokens:     200,
				systemMessage: "You are a terse assistant. Answer in one word.",
			},
			creds: creds,
		}
	}
	u := func(s string) message.Message {
		return message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: s}}}
	}
	a := func(s string) message.Message {
		return message.Message{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: s}}}
	}

	single := []message.Message{u("say only hi")}
	multi := []message.Message{u("hi"), a("Hello! How can I help?"), u("say only bye")}

	// History containing an actual tool call + result — the case the reported
	// error ("assistant tool_calls array element" missing 'id') points at.
	am := message.Message{Role: message.Assistant}
	am.SetToolCalls([]message.ToolCall{{ID: "call_abc123", Name: "list_files", Input: `{"path":"."}`, Type: "function", Finished: true}})
	tm := message.Message{Role: message.Tool}
	tm.AddToolResult(message.ToolResult{ToolCallID: "call_abc123", Name: "list_files", Content: "a.txt\nb.txt"})
	toolHist := []message.Message{u("list the files here"), am, tm, u("now say only done")}

	cases := []struct {
		name  string
		msgs  []message.Message
		tools []tools.BaseTool
	}{
		{"single_noTools", single, nil},
		{"single_withTools", single, []tools.BaseTool{reproTool{}}},
		{"multi_noTools", multi, nil},
		{"multi_withTools", multi, []tools.BaseTool{reproTool{}}},
		{"toolCallHistory", toolHist, []tools.BaseTool{reproTool{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			resp, err := client().send(ctx, tc.msgs, tc.tools)
			if err != nil {
				t.Fatalf("FAILED: %v", err)
			}
			t.Logf("OK: %q (in=%d out=%d)", resp.Content, resp.Usage.InputTokens, resp.Usage.OutputTokens)
		})
	}
}
