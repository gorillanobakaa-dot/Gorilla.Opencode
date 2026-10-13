package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/opencode-ai/opencode/internal/llm/tools"
)

// fakeTool stands in for one of this program's tools.
type fakeTool struct {
	name string
	mu   sync.Mutex
	runs int
	last string
	fail error
}

func (f *fakeTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name:        f.name,
		Description: "a test tool",
		Parameters:  map[string]any{"q": map[string]any{"type": "string"}},
		Required:    []string{"q"},
	}
}

func (f *fakeTool) Run(ctx context.Context, p tools.ToolCall) (tools.ToolResponse, error) {
	f.mu.Lock()
	f.runs++
	f.last = p.Input
	f.mu.Unlock()
	if f.fail != nil {
		return tools.ToolResponse{}, f.fail
	}
	return tools.ToolResponse{Content: "ran " + f.name}, nil
}

func startTestBridge(t *testing.T, ts ...tools.BaseTool) *toolBridge {
	t.Helper()
	b, err := startToolBridge(ts, "sess-1", "msg-1")
	if err != nil {
		t.Fatalf("starting bridge: %v", err)
	}
	t.Cleanup(b.Close)
	return b
}

// The bridge opens a real port. Anything on this machine could connect to it, so
// the token is the only thing standing between another local process and this
// program's tools, which include editing files and running commands. A request
// without it must be refused before the MCP layer sees it at all.
func TestBridgeRefusesAnUnauthenticatedCaller(t *testing.T) {
	b := startTestBridge(t, &fakeTool{name: "view"})

	resp, err := http.Get(b.url)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("a caller with no token got %d; the bridge is open to anything running as this user", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, b.url, nil)
	req.Header.Set("Authorization", "Bearer not-the-token")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET with wrong token: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("a caller with the WRONG token got %d, want 403", resp2.StatusCode)
	}
}

// The bridge must listen on loopback only. On 0.0.0.0 it would be remote code
// execution reachable from the network.
func TestBridgeListensOnLoopbackOnly(t *testing.T) {
	b := startTestBridge(t, &fakeTool{name: "view"})
	if !strings.HasPrefix(b.listener.Addr().String(), "127.0.0.1:") {
		t.Errorf("the bridge is listening on %s, not loopback", b.listener.Addr())
	}
	if !strings.Contains(b.url, "127.0.0.1") {
		t.Errorf("the url handed to the child is %q, not loopback", b.url)
	}
}

// Every token must be unique to its turn. A fixed or reused one would survive in
// a log or a crash dump and still open the bridge afterwards.
func TestBridgeTokenIsFreshEachTurn(t *testing.T) {
	a := startTestBridge(t, &fakeTool{name: "view"})
	b := startTestBridge(t, &fakeTool{name: "view"})
	if a.token == b.token {
		t.Error("two bridges share a token")
	}
	if len(a.token) < 32 {
		t.Errorf("the token is %d characters, which is too few to be worth checking", len(a.token))
	}
}

// Without the allowlist the child DENIES every bridged tool: measured against
// claude 2.1.296, the call returns "the session has no approval surface" and
// lands in permission_denials. The allowlist must name every tool we supply, in
// the child's own mcp__server__tool form, and nothing else.
func TestBridgeAllowlistNamesExactlyOurTools(t *testing.T) {
	b := startTestBridge(t, &fakeTool{name: "view"}, &fakeTool{name: "edit"})
	got := b.allowedTools()
	want := []string{"mcp__gorilla__edit", "mcp__gorilla__view"}
	if len(got) != len(want) {
		t.Fatalf("allowlist = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("allowlist[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// The config handed to the child must carry the loopback url and the token, and
// must use the sse transport: a stdio server would be spawned by the child, in a
// third process, where this program's permission prompts cannot reach the user.
func TestBridgeConfigCarriesTransportUrlAndToken(t *testing.T) {
	b := startTestBridge(t, &fakeTool{name: "view"})
	raw, err := b.mcpConfig()
	if err != nil {
		t.Fatalf("mcpConfig: %v", err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Type    string            `json:"type"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	srv, ok := cfg.MCPServers[bridgeServerName]
	if !ok {
		t.Fatalf("config names no %q server: %s", bridgeServerName, raw)
	}
	if srv.Type != "sse" {
		t.Errorf("transport is %q, want sse", srv.Type)
	}
	if srv.URL != b.url {
		t.Errorf("url is %q, want %q", srv.URL, b.url)
	}
	if srv.Headers["Authorization"] != "Bearer "+b.token {
		t.Error("the config does not carry the bearer token, so the child would be refused by our own auth")
	}
}

// A call through the bridge must run the real tool, in this process, with the
// arguments intact and the session id the permission system keys off.
func TestBridgeRunsTheToolInThisProcess(t *testing.T) {
	f := &fakeTool{name: "view"}
	b := startTestBridge(t, f)

	var req mcp.CallToolRequest
	req.Params.Name = "view"
	req.Params.Arguments = map[string]any{"q": "hello"}

	res, err := b.handler("view")(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	f.mu.Lock()
	runs, last := f.runs, f.last
	f.mu.Unlock()

	if runs != 1 {
		t.Errorf("the tool ran %d times, want 1", runs)
	}
	if !strings.Contains(last, "hello") {
		t.Errorf("arguments did not cross the bridge: %q", last)
	}
	if res.IsError {
		t.Error("a successful call was reported as an error")
	}
	if len(b.Calls()) != 1 {
		t.Fatalf("the bridge recorded %d calls, want 1; the receipt would be wrong", len(b.Calls()))
	}
	if c := b.Calls()[0]; c.Name != "view" || c.Output != "ran view" || c.ID == "" {
		t.Errorf("recorded call is wrong: %+v", c)
	}
}

// A failing tool comes back as a tool RESULT marked as an error, not as a
// transport failure. The model has to see what went wrong so it can try
// something else; a JSON-RPC error ends the turn instead.
func TestBridgeReportsAFailingToolToTheModel(t *testing.T) {
	f := &fakeTool{name: "edit", fail: errors.New("permission denied by the user")}
	b := startTestBridge(t, f)

	var req mcp.CallToolRequest
	req.Params.Name = "edit"
	req.Params.Arguments = map[string]any{"q": "x"}

	res, err := b.handler("edit")(context.Background(), req)
	if err != nil {
		t.Fatalf("a tool failure was raised as a transport error, which ends the turn: %v", err)
	}
	if !res.IsError {
		t.Error("the failure was reported to the model as success")
	}
	if calls := b.Calls(); len(calls) != 1 || !calls[0].IsError {
		t.Errorf("the failure was not recorded for the receipt: %+v", calls)
	}
}

// What the child calls a tool and what the user calls it are different strings.
// The user never chose the mcp__ prefix and must not be shown it.
func TestBridgeToolNamesRoundTrip(t *testing.T) {
	if got := BridgeToolName("view"); got != "mcp__gorilla__view" {
		t.Errorf("BridgeToolName = %q", got)
	}
	if got := LocalToolName("mcp__gorilla__view"); got != "view" {
		t.Errorf("LocalToolName = %q, want view", got)
	}
	// Anything not ours must be left alone rather than mangled.
	if got := LocalToolName("Bash"); got != "" {
		t.Errorf("LocalToolName(%q) = %q, want empty", "Bash", got)
	}
	if got := LocalToolName("mcp__other__view"); got != "" {
		t.Errorf("another server's tool was claimed as ours: %q", got)
	}
}

// No tools means no bridge, no port and no token. A listener opened for a turn
// that cannot use it is an open door for nothing.
func TestBridgeIsNotStartedWithoutTools(t *testing.T) {
	b, err := startToolBridge(nil, "s", "m")
	if err != nil {
		t.Fatalf("startToolBridge: %v", err)
	}
	if b != nil {
		b.Close()
		t.Error("a bridge was started for a turn with no tools")
	}
}
