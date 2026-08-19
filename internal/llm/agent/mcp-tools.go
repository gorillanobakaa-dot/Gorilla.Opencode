package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/version"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type mcpTool struct {
	mcpName     string
	tool        mcp.Tool
	mcpConfig   config.MCPServer
	permissions permission.Service
}

type MCPClient interface {
	Initialize(
		ctx context.Context,
		request mcp.InitializeRequest,
	) (*mcp.InitializeResult, error)
	ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error)
	CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	Close() error
}

func (b *mcpTool) Info() tools.ToolInfo {
	required := b.tool.InputSchema.Required
	if required == nil {
		required = make([]string, 0)
	}
	return tools.ToolInfo{
		Name:        fmt.Sprintf("%s_%s", b.mcpName, b.tool.Name),
		Description: b.tool.Description,
		Parameters:  b.tool.InputSchema.Properties,
		Required:    required,
	}
}

func runTool(ctx context.Context, c MCPClient, serverName, toolName string, input string) (tools.ToolResponse, error) {
	defer c.Close()
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "OpenCode",
		Version: version.Version,
	}

	_, err := c.Initialize(ctx, initRequest)
	if err != nil {
		return tools.NewTextErrorResponse(err.Error()), nil
	}

	toolRequest := mcp.CallToolRequest{}
	toolRequest.Params.Name = toolName
	var args map[string]any
	if err = json.Unmarshal([]byte(input), &args); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}
	toolRequest.Params.Arguments = args
	result, err := c.CallTool(ctx, toolRequest)
	if err != nil {
		return tools.NewTextErrorResponse(err.Error()), nil
	}

	// GORILLA FIX (2026-08-19): this was `output = v.Text` inside the loop —
	// plain assignment, not append. A server returning several content blocks
	// had every block but the last silently discarded, and the result looked
	// perfectly well-formed, so there was nothing to notice. Multi-block
	// results are normal in MCP: one block of prose and one of structured
	// data is the common shape.
	var out strings.Builder
	for i, v := range result.Content {
		if i > 0 {
			out.WriteString("\n")
		}
		if tc, ok := v.(mcp.TextContent); ok {
			out.WriteString(tc.Text)
		} else {
			fmt.Fprintf(&out, "%v", v)
		}
	}

	// An MCP server is third-party code returning third-party content. It is
	// exactly the shape untrusted.go exists for: the model must be able to
	// tell a server's output from its operator's instructions.
	return tools.NewUntrustedTextResponse("MCP server", serverName, "", out.String()), nil
}

func (b *mcpTool) Run(ctx context.Context, params tools.ToolCall) (tools.ToolResponse, error) {
	sessionID, messageID := tools.GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return tools.ToolResponse{}, fmt.Errorf("session ID and message ID are required for creating a new file")
	}
	permissionDescription := fmt.Sprintf("execute %s with the following parameters: %s", b.Info().Name, params.Input)
	if who := describeMCPServer(b.mcpName); who != "" {
		permissionDescription = who + "\n\n" + permissionDescription
	}
	p := b.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        config.WorkingDirectory(),
			ToolName:    b.Info().Name,
			Action:      "execute",
			Description: permissionDescription,
			Params:      params.Input,
			// GORILLA FIX (2026-08-19): scope the grant to THIS invocation.
			// An MCP server's tools are third-party code; "allow for session" on
			// one call must not authorise every later call to that server with
			// different arguments.
			GrantKey: params.Input,
			// An MCP server may be a remote HTTP endpoint. Stdio servers are
			// local processes, but a local process is free to open a socket,
			// so both count as egress for the auto-approve carve-out.
			Egress: true,
		},
	)
	if !p {
		return tools.NewTextErrorResponse("permission denied"), nil
	}

	switch b.mcpConfig.Type {
	case config.MCPStdio:
		c, err := client.NewStdioMCPClient(
			b.mcpConfig.Command,
			b.mcpConfig.Env,
			b.mcpConfig.Args...,
		)
		if err != nil {
			return tools.NewTextErrorResponse(err.Error()), nil
		}
		tools.MarkMCPTaint(sessionID, b.mcpName)
		return runTool(ctx, c, b.mcpName, b.tool.Name, params.Input)
	case config.MCPSse:
		// GORILLA FIX (2026-08-19): this dialled whatever URL was in config
		// with no check at all, while fetch.go twenty files away has a
		// three-layer SSRF guard. `http://169.254.169.254/latest/meta-data/`
		// was a valid MCP server address until this line existed.
		if reason := tools.BlockedMCPTarget(b.mcpConfig.URL); reason != "" {
			return tools.NewTextErrorResponse(fmt.Sprintf(
				"Refusing to contact MCP server %q at %s: %s",
				b.mcpName, b.mcpConfig.URL, reason)), nil
		}
		c, err := client.NewSSEMCPClient(
			b.mcpConfig.URL,
			client.WithHeaders(b.mcpConfig.Headers),
		)
		if err != nil {
			return tools.NewTextErrorResponse(err.Error()), nil
		}
		tools.MarkMCPTaint(sessionID, b.mcpName)
		return runTool(ctx, c, b.mcpName, b.tool.Name, params.Input)
	}

	return tools.NewTextErrorResponse("invalid mcp type"), nil
}

func NewMcpTool(name string, tool mcp.Tool, permissions permission.Service, mcpConfig config.MCPServer) tools.BaseTool {
	return &mcpTool{
		mcpName:     name,
		tool:        tool,
		mcpConfig:   mcpConfig,
		permissions: permissions,
	}
}

var mcpTools []tools.BaseTool

func getTools(ctx context.Context, name string, m config.MCPServer, permissions permission.Service, c MCPClient) []tools.BaseTool {
	var stdioTools []tools.BaseTool
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "OpenCode",
		Version: version.Version,
	}

	// GORILLA OVERRIDE (2026-08-19): record what was actually negotiated.
	//
	// The client asks for mcp.LATEST_PROTOCOL_VERSION and the server answers
	// with what IT supports, which may be older. Nothing logged the answer, so
	// a capability missing because of a version mismatch looked identical to a
	// capability the server does not have — and the mcp-go dependency is
	// pinned, so the version this client asks for is a decision, not a
	// default. See the GORILLA OVERRIDE on the pin in go.mod.
	initResult, err := c.Initialize(ctx, initRequest)
	if err != nil {
		logging.Error("error initializing mcp client", "error", err)
		return stdioTools
	}
	if initResult != nil {
		logging.Info("mcp server connected",
			"server", name,
			"requested_protocol", mcp.LATEST_PROTOCOL_VERSION,
			"negotiated_protocol", initResult.ProtocolVersion,
			"server_name", initResult.ServerInfo.Name,
			"server_version", initResult.ServerInfo.Version)
		if initResult.ProtocolVersion != mcp.LATEST_PROTOCOL_VERSION {
			logging.Warn("mcp server speaks an older protocol than this client asked for; "+
				"a missing capability may be a version mismatch rather than an absence",
				"server", name, "requested", mcp.LATEST_PROTOCOL_VERSION,
				"negotiated", initResult.ProtocolVersion)
		}
		mcpServerInfo.Store(name, mcpDescriptor{
			ServerName:   initResult.ServerInfo.Name,
			Version:      initResult.ServerInfo.Version,
			Protocol:     initResult.ProtocolVersion,
			Instructions: strings.TrimSpace(initResult.Instructions),
		})
	}
	toolsRequest := mcp.ListToolsRequest{}
	tools, err := c.ListTools(ctx, toolsRequest)
	if err != nil {
		logging.Error("error listing tools", "error", err)
		return stdioTools
	}
	for _, t := range tools.Tools {
		stdioTools = append(stdioTools, NewMcpTool(name, t, permissions, m))
	}
	defer c.Close()
	return stdioTools
}

// mcpDescriptor is what a server said about itself at Initialize.
type mcpDescriptor struct {
	ServerName   string
	Version      string
	Protocol     string
	Instructions string
}

var mcpServerInfo sync.Map // server name -> mcpDescriptor

// describeMCPServer renders what is known about a server for the permission
// dialog.
//
// GORILLA FIX (2026-08-19): the approval prompt said "execute
// <server>_<tool> with the following parameters" and nothing else. A user
// approving a third-party tool call could not see WHOSE code it was, what
// version, or what the server says it is for. The audit's finding was that a
// prompt describing less than what happens is worse than no prompt; this is
// the same defect, in the one place the program hands work to somebody else's
// software.
//
// The server's own Instructions string is UNTRUSTED — it is written by the
// server author — so it is quoted as a claim, not stated as fact, and capped.
func describeMCPServer(name string) string {
	v, ok := mcpServerInfo.Load(name)
	if !ok {
		return ""
	}
	d, ok := v.(mcpDescriptor)
	if !ok {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "MCP server %q", name)
	if d.ServerName != "" && d.ServerName != name {
		fmt.Fprintf(&b, " (identifies itself as %s", d.ServerName)
		if d.Version != "" {
			fmt.Fprintf(&b, " %s", d.Version)
		}
		b.WriteString(")")
	} else if d.Version != "" {
		fmt.Fprintf(&b, " (version %s)", d.Version)
	}
	if d.Protocol != "" {
		fmt.Fprintf(&b, ", MCP protocol %s", d.Protocol)
	}
	if d.Instructions != "" {
		desc := d.Instructions
		if len(desc) > 300 {
			desc = desc[:300] + "..."
		}
		fmt.Fprintf(&b, "\nThe server describes itself as: %q (its own words, not verified)", desc)
	}
	return b.String()
}

// McpLoadoutComponents is one /context row per configured MCP server, with the
// measured token cost of every tool that server contributes.
//
// GORILLA FIX (2026-08-19): MCP tools were never registered, so /context, the
// per-turn cost line and the burn-rate warning were all computed over a tool
// list that excluded them. A user with three MCP servers was shown a number
// that was simply wrong, and the one screen built to tell them what their
// setup costs was the screen not counting the most expensive part of it.
//
// The rows are informational: the count is real whether or not the row can be
// switched off, and a wrong number on the honesty screen is the worst possible
// place for one.
func McpLoadoutComponents() []config.LoadoutComponent {
	byServer := map[string]int{}
	for _, t := range mcpTools {
		mt, ok := t.(*mcpTool)
		if !ok {
			continue
		}
		byServer[mt.mcpName] += toolTokens(t)
	}
	out := make([]config.LoadoutComponent, 0, len(byServer))
	names := make([]string, 0, len(byServer))
	for n := range byServer {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		count := 0
		for _, t := range mcpTools {
			if mt, ok := t.(*mcpTool); ok && mt.mcpName == n {
				count++
			}
		}
		out = append(out, config.LoadoutComponent{
			ID:   "mcp." + n,
			Name: fmt.Sprintf("MCP: %s (%d tool(s))", n, count),
			Tradeoff: fmt.Sprintf("agent loses every tool from the %q MCP server. "+
				"These are third-party: their cost rides every request, and what "+
				"they do is decided by whoever wrote that server", n),
			Tokens:  byServer[n],
			Default: true,
		})
	}
	return out
}

func GetMcpTools(ctx context.Context, permissions permission.Service) []tools.BaseTool {
	if len(mcpTools) > 0 {
		return mcpTools
	}
	for name, m := range config.Get().MCPServers {
		switch m.Type {
		case config.MCPStdio:
			c, err := client.NewStdioMCPClient(
				m.Command,
				m.Env,
				m.Args...,
			)
			if err != nil {
				logging.Error("error creating mcp client", "error", err)
				continue
			}

			mcpTools = append(mcpTools, getTools(ctx, name, m, permissions, c)...)
		case config.MCPSse:
			c, err := client.NewSSEMCPClient(
				m.URL,
				client.WithHeaders(m.Headers),
			)
			if err != nil {
				logging.Error("error creating mcp client", "error", err)
				continue
			}
			mcpTools = append(mcpTools, getTools(ctx, name, m, permissions, c)...)
		}
	}

	return mcpTools
}
