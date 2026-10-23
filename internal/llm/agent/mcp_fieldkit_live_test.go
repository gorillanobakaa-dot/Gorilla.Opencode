package agent

// GORILLA OVERRIDE (2026-10-10): the real Fieldkit server (github.com/
// gorillanobakaa-dot/Gorilla.Fieldkit), spoken to by this program's own MCP
// client and its "trust": "local" policy. Skipped unless FIELDKIT_MCP_COMMAND
// names the program (e.g. "python3 -m fieldkit mcp" run from the Fieldkit
// folder, or "fieldkit mcp"), so CI never depends on a second repository.

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
)

func TestTheRealFieldkitServerAsALocalServer(t *testing.T) {
	cmdline := strings.Fields(os.Getenv("FIELDKIT_MCP_COMMAND"))
	if len(cmdline) == 0 {
		t.Skip("FIELDKIT_MCP_COMMAND not set")
	}
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.MCPServer{Type: config.MCPStdio, Command: cmdline[0], Args: cmdline[1:], Trust: "local"}
	perms := &recordingMCPPermissions{}
	call := func(session, tool, input string) tools.ToolResponse {
		t.Helper()
		ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, session)
		ctx = context.WithValue(ctx, tools.MessageIDContextKey, "m1")
		resp, err := NewMcpTool("fieldkit", mcp.Tool{Name: tool}, perms, cfg).Run(ctx,
			tools.ToolCall{ID: "c1", Name: "fieldkit_" + tool, Input: input})
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		return resp
	}

	// Fieldkit's own words: no egress question, no fence. (The taint mark these
	// lines also checked was removed on 2026-10-23, owner's order.)
	resp := call("fk-own", "describe", `{"tool":"academic-refs"}`)
	if !strings.HasPrefix(resp.Content, "academic-refs:") {
		t.Fatalf("describe answered %q", resp.Content)
	}
	if strings.Contains(resp.Content, "UNTRUSTED CONTENT BEGINS") || perms.calls[len(perms.calls)-1].Egress {
		t.Error("Fieldkit's own answer was treated as untrusted or as egress")
	}

	// A refusal names network=true for a tool that goes online.
	resp = call("fk-net", "run", `{"tool":"release-check","inputs":{"spec":"x"}}`)
	if !strings.Contains(resp.Content, "network=true") {
		t.Errorf("a network tool without network=true: %q", resp.Content)
	}

	// Reading someone else's file: the answer is fenced.
	resp = call("fk-read", "run", `{"tool":"office-read","inputs":{"file":"missing.docx"}}`)
	if !strings.Contains(resp.Content, "UNTRUSTED CONTENT BEGINS") {
		t.Error("office-read (third-party text) was not fenced")
	}
}
