package agent

// GORILLA OVERRIDE (2026-10-10): "trust": "local" (config.MCPServer.Trust).
//
// What these tests hold: a server marked local skips the egress question and
// the taint ONLY on what its answer says, every unclear case is treated as
// today, a server that lies about going online loses the benefit, and a
// server NOT marked local is exactly as strict as before.

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/permission"
)

func TestDeclaresNetworkOnlyForATopLevelTrue(t *testing.T) {
	cases := map[string]bool{
		`{"tool":"x","network":true}`:       true,
		`{"tool":"x","network":false}`:      false,
		`{"tool":"x","network":"true"}`:     false, // a string is not a declaration
		`{"tool":"x"}`:                      false,
		`{"inputs":{"network":true}}`:       false, // only the top level counts
		`not json`:                          false,
		``:                                  false,
		`{"tool":"x","network":true,"a":1}`: true,
	}
	for in, want := range cases {
		if got := declaresNetwork(in); got != want {
			t.Errorf("declaresNetwork(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestLocalAnswersAreTrustedOnlyWhenTheyExplicitlySaySo(t *testing.T) {
	cases := []struct {
		name     string
		meta     map[string]any
		declared bool
		want     bool
	}{
		{"no labels at all", nil, false, false},
		{"untrusted missing", map[string]any{"egress": false}, false, false},
		{"untrusted true", map[string]any{"untrusted": true, "egress": false}, false, false},
		{"untrusted as a string", map[string]any{"untrusted": "false"}, false, false},
		{"own answer", map[string]any{"untrusted": false, "egress": false}, false, true},
		{"own answer, declared network, went online", map[string]any{"untrusted": false, "egress": true}, true, true},
		{"went online without declaring it", map[string]any{"untrusted": false, "egress": true}, false, false},
	}
	for _, c := range cases {
		got, why := localAnswerTrusted(c.meta, c.declared)
		if got != c.want {
			t.Errorf("%s: trusted=%v (%s), want %v", c.name, got, why, c.want)
		}
		if !got && why == "" {
			t.Errorf("%s: untrusted without a reason", c.name)
		}
	}
}

func TestTrustLocalIsIgnoredForSSEAndForOtherValues(t *testing.T) {
	if (config.MCPServer{Type: config.MCPSse, Trust: "local"}).LocalTrust() {
		t.Error("an sse server must never count as local")
	}
	if (config.MCPServer{Type: config.MCPStdio, Trust: "yes"}).LocalTrust() {
		t.Error("only the exact value \"local\" counts")
	}
	if !(config.MCPServer{Type: config.MCPStdio, Trust: "local"}).LocalTrust() {
		t.Error("a stdio server marked local must count")
	}
}

// TestFakeLocalMCPServer is not a test: it is the MCP server the tests below
// start as a child process (this test binary, re-run with an environment
// variable). It answers one call with the _meta given in GORILLA_FAKE_MCP_META
// ("none" = no _meta at all).
func TestFakeLocalMCPServer(t *testing.T) {
	metaJSON := os.Getenv("GORILLA_FAKE_MCP_META")
	if metaJSON == "" {
		t.Skip("helper process only")
	}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := json.NewEncoder(os.Stdout)
	for in.Scan() {
		var msg map[string]any
		if json.Unmarshal(in.Bytes(), &msg) != nil {
			continue
		}
		id, hasID := msg["id"]
		if !hasID {
			continue // a notification
		}
		var result any
		switch msg["method"] {
		case "initialize":
			result = map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}},
				"serverInfo": map[string]any{"name": "fake", "version": "1"}}
		case "tools/call":
			r := map[string]any{"content": []any{map[string]any{"type": "text", "text": "answer from the fake server"}},
				"isError": false}
			if metaJSON != "none" {
				var meta map[string]any
				_ = json.Unmarshal([]byte(metaJSON), &meta)
				r["_meta"] = meta
			}
			result = r
		default:
			result = map[string]any{}
		}
		_ = out.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	os.Exit(0)
}

type recordingMCPPermissions struct {
	permission.Service
	calls []permission.CreatePermissionRequest
}

func (r *recordingMCPPermissions) Request(opts permission.CreatePermissionRequest) bool {
	r.calls = append(r.calls, opts)
	return true
}

func runFakeServer(t *testing.T, session, trust, meta, input string) (tools.ToolResponse, *recordingMCPPermissions) {
	t.Helper()
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	permission.ClearTaint(session)
	t.Cleanup(func() { permission.ClearTaint(session) })
	perms := &recordingMCPPermissions{}
	cfg := config.MCPServer{Type: config.MCPStdio, Command: os.Args[0], Trust: trust,
		Args: []string{"-test.run=^TestFakeLocalMCPServer$"}, Env: []string{"GORILLA_FAKE_MCP_META=" + meta}}
	tool := NewMcpTool("fake", mcp.Tool{Name: "run"}, perms, cfg)
	ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, session)
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "m1")
	resp, err := tool.Run(ctx, tools.ToolCall{ID: "c1", Name: "fake_run", Input: input})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(perms.calls) != 1 {
		t.Fatalf("%d permission requests, want 1", len(perms.calls))
	}
	return resp, perms
}

func TestALocalServersOwnAnswerNeitherAsksAsEgressNorTaints(t *testing.T) {
	resp, perms := runFakeServer(t, "local-own", "local", `{"untrusted":false,"egress":false}`, `{"tool":"academic-refs"}`)
	if perms.calls[0].Egress {
		t.Error("a local call that did not declare the network was treated as egress")
	}
	if permission.IsTainted("local-own") {
		t.Error("an answer labelled as the server's own tainted the session")
	}
	if resp.Content != "answer from the fake server" {
		t.Errorf("own answer was wrapped or changed: %q", resp.Content)
	}
}

func TestALocalCallThatDeclaresTheNetworkIsEgress(t *testing.T) {
	_, perms := runFakeServer(t, "local-net", "local", `{"untrusted":false,"egress":true}`,
		`{"tool":"jobs-search","network":true}`)
	if !perms.calls[0].Egress {
		t.Error("a call declaring network=true must ask as egress")
	}
	if permission.IsTainted("local-net") {
		t.Error("an honest, own answer after a declared network call tainted the session")
	}
}

func TestALocalAnswerWithOtherPeoplesTextTaints(t *testing.T) {
	resp, _ := runFakeServer(t, "local-third", "local", `{"untrusted":true,"egress":false}`, `{"tool":"office-read"}`)
	if !permission.IsTainted("local-third") {
		t.Error("an answer labelled untrusted did not taint")
	}
	if resp.Content == "answer from the fake server" {
		t.Error("an untrusted answer was not fenced")
	}
}

func TestALocalServerThatSendsNoLabelsIsTreatedAsToday(t *testing.T) {
	runFakeServer(t, "local-none", "local", "none", `{"tool":"x"}`)
	if !permission.IsTainted("local-none") {
		t.Error("an answer with no _meta skipped the taint")
	}
}

func TestALocalServerThatLiesAboutGoingOnlineLosesTheBenefit(t *testing.T) {
	runFakeServer(t, "local-liar", "local", `{"untrusted":false,"egress":true}`, `{"tool":"x"}`)
	if !permission.IsTainted("local-liar") {
		t.Error("undeclared egress did not taint")
	}
}

func TestAServerNotMarkedLocalIsAsStrictAsBefore(t *testing.T) {
	resp, perms := runFakeServer(t, "plain", "", `{"untrusted":false,"egress":false}`, `{"tool":"x"}`)
	if !perms.calls[0].Egress {
		t.Error("an unmarked server lost its egress question")
	}
	if !permission.IsTainted("plain") {
		t.Error("an unmarked server's answer did not taint")
	}
	if resp.Content == "answer from the fake server" {
		t.Error("an unmarked server's answer was not fenced")
	}
}
