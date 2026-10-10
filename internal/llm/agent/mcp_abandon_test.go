package agent

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/opencode-ai/opencode/internal/config"
)

// A program that is not an MCP server and ignores its stdin (measured
// 2026-10-10: ping configured as a server) was left running after the
// handshake failed, because Close only shuts stdin and waits.
func TestAServerThatFailsTheHandshakeIsKilledNotLeftRunning(t *testing.T) {
	prev := mcpHandshakeTimeout
	mcpHandshakeTimeout = 300 * time.Millisecond
	t.Cleanup(func() { mcpHandshakeTimeout = prev })

	cmd, args := "sleep", []string{"30"}
	if runtime.GOOS == "windows" {
		cmd, args = "ping", []string{"-n", "30", "127.0.0.1"}
	}
	c, err := client.NewStdioMCPClient(cmd, nil, args...)
	if err != nil {
		t.Skipf("cannot start %s: %v", cmd, err)
	}
	proc := stdioCmd(c)
	if proc == nil || proc.Process == nil {
		t.Fatal("the mcp-go stdio client no longer exposes its process as field \"cmd\"; abandonMCPClient cannot kill a failed server")
	}

	start := time.Now()
	if n := len(getTools(context.Background(), "not-a-server", config.MCPServer{}, nil, c)); n != 0 {
		t.Fatalf("%d tools from a program that is not a server", n)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("giving up on the server took %s", took)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if proc.ProcessState != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = proc.Process.Kill()
	t.Fatal("the server process is still running after the handshake failed")
}

func TestMCPServersAreContactedOncePerRunEvenWhenTheyFail(t *testing.T) {
	mcpLoadMu.Lock()
	prevLoaded, prevTools := mcpLoaded, mcpTools
	mcpLoaded, mcpTools = true, nil
	mcpLoadMu.Unlock()
	t.Cleanup(func() {
		mcpLoadMu.Lock()
		mcpLoaded, mcpTools = prevLoaded, prevTools
		mcpLoadMu.Unlock()
	})
	// With the load already done and nothing found, a second call must return
	// at once without contacting anything; config.Get() is not even needed.
	start := time.Now()
	if got := GetMcpTools(context.Background(), nil); len(got) != 0 {
		t.Fatalf("got %d tools", len(got))
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("a second call contacted the servers again")
	}
}
