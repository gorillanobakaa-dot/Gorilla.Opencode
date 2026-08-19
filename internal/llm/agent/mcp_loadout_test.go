package agent

// GORILLA OVERRIDE (2026-08-19): /context is the honesty screen. A wrong
// number on it is worse than no number, and MCP tools were missing from it
// entirely — so someone running three MCP servers was shown a per-turn cost
// computed over a tool list that excluded the most expensive part of their
// setup.

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/opencode-ai/opencode/internal/config"
)

func withFakeMcpTools(t *testing.T, tools []*mcpTool) {
	t.Helper()
	prev := mcpTools
	mcpTools = nil
	for _, mt := range tools {
		mcpTools = append(mcpTools, mt)
	}
	t.Cleanup(func() { mcpTools = prev })
}

func fakeMcpTool(server, name, desc string) *mcpTool {
	return &mcpTool{
		mcpName: server,
		tool: mcp.Tool{
			Name:        name,
			Description: desc,
		},
	}
}

func TestEveryMcpServerGetsALoadoutRowWithARealCost(t *testing.T) {
	withFakeMcpTools(t, []*mcpTool{
		fakeMcpTool("filesystem", "read_file", strings.Repeat("describe this tool. ", 20)),
		fakeMcpTool("filesystem", "write_file", strings.Repeat("describe this tool. ", 20)),
		fakeMcpTool("weather", "forecast", strings.Repeat("describe this tool. ", 10)),
	})

	rows := McpLoadoutComponents()
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want one per server (2): %+v", len(rows), rows)
	}
	// Sorted, so the order is stable in the /context menu rather than map order.
	if rows[0].ID != "mcp.filesystem" || rows[1].ID != "mcp.weather" {
		t.Errorf("rows are not in a stable sorted order: %s, %s", rows[0].ID, rows[1].ID)
	}
	for _, r := range rows {
		if r.Tokens <= 0 {
			t.Errorf("%s reports %d tokens — a row that claims to cost nothing is the bug this fixes", r.ID, r.Tokens)
		}
		if !strings.Contains(r.Tradeoff, "third-party") {
			t.Errorf("%s does not say the tools are third-party: %q", r.ID, r.Tradeoff)
		}
	}
	// Two tools of the same size must cost more than one of them.
	if rows[0].Tokens <= rows[1].Tokens {
		t.Errorf("filesystem (2 tools) reports %d tokens, weather (1 tool) reports %d — the count is not being summed",
			rows[0].Tokens, rows[1].Tokens)
	}
	if !strings.Contains(rows[0].Name, "2 tool") {
		t.Errorf("the row does not say how many tools it covers: %q", rows[0].Name)
	}
}

func TestNoMcpServersMeansNoRows(t *testing.T) {
	withFakeMcpTools(t, nil)
	if rows := McpLoadoutComponents(); len(rows) != 0 {
		t.Fatalf("invented %d rows with no MCP servers configured", len(rows))
	}
}

// Registration is idempotent by ID, so a config reload cannot double-count a
// server's cost on the one screen built to report it accurately.
func TestRegisteringTwiceDoesNotDoubleTheCost(t *testing.T) {
	withFakeMcpTools(t, []*mcpTool{fakeMcpTool("weather", "forecast", "gets the weather")})
	before := len(config.LoadoutComponents)
	config.RegisterLoadoutComponents(McpLoadoutComponents())
	config.RegisterLoadoutComponents(McpLoadoutComponents())
	added := len(config.LoadoutComponents) - before
	if added != 1 {
		t.Fatalf("registering twice added %d rows, want 1", added)
	}
}

// The approval prompt is the one boundary this program has. Handing work to
// somebody else's software without saying whose is the audit's exact finding.
func TestTheApprovalPromptNamesTheServer(t *testing.T) {
	mcpServerInfo.Store("weather", mcpDescriptor{
		ServerName:   "weather-mcp",
		Version:      "1.2.3",
		Protocol:     "2024-11-05",
		Instructions: "Provides forecasts.",
	})
	t.Cleanup(func() { mcpServerInfo.Delete("weather") })

	got := describeMCPServer("weather")
	for _, want := range []string{"weather", "weather-mcp", "1.2.3", "2024-11-05", "Provides forecasts."} {
		if !strings.Contains(got, want) {
			t.Errorf("the description omits %q:\n%s", want, got)
		}
	}
	// The server author wrote that sentence. It is quoted as a claim.
	if !strings.Contains(got, "not verified") {
		t.Errorf("the server's self-description is presented as fact:\n%s", got)
	}
}

func TestAnUnknownServerDescribesNothingRatherThanGuessing(t *testing.T) {
	if got := describeMCPServer("never-connected"); got != "" {
		t.Fatalf("invented a description for a server that never initialised: %q", got)
	}
}
