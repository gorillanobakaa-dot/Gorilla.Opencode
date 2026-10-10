package agent

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/opencode-ai/opencode/internal/config"
)

// A server that never answers the handshake (2026-10-10: calc.exe configured as
// an MCP server) froze start-up and the runtime killed the program with
// "all goroutines are asleep - deadlock!".
type silentMCP struct{ closed bool }

func (s *silentMCP) Initialize(ctx context.Context, _ mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (s *silentMCP) ListTools(ctx context.Context, _ mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (s *silentMCP) CallTool(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return nil, nil
}
func (s *silentMCP) Close() error { s.closed = true; return nil }

func TestAnMCPServerThatNeverAnswersIsDroppedNotWaitedForForever(t *testing.T) {
	prev := mcpHandshakeTimeout
	mcpHandshakeTimeout = 200 * time.Millisecond
	t.Cleanup(func() { mcpHandshakeTimeout = prev })

	c := &silentMCP{}
	done := make(chan int, 1)
	go func() {
		done <- len(getTools(context.Background(), "silent", config.MCPServer{}, nil, c))
	}()
	select {
	case n := <-done:
		if n != 0 {
			t.Errorf("a silent server produced %d tools", n)
		}
		if !c.closed {
			t.Error("the silent server's process was not closed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("getTools is still waiting on a server that never answers")
	}
}
