// GORILLA OVERRIDE: this file did not exist upstream.
//
// It hands THIS PROGRAM'S tools to the `claude` child process, so that a Claude
// subscription drives Gorilla OpenCode rather than driving Claude Code.
//
// # WHY A BRIDGE AT ALL
//
// The child arrives with its own Read, Edit, Bash and WebFetch. Left enabled,
// every answer would be produced by Claude Code's tools, in Claude Code's style,
// with none of this program's permission checks, none of its output caps, and
// nothing in its receipt. The user would believe they were testing Gorilla
// OpenCode and would be testing something else. So `--tools ""` strips the lot
// (see claudecli.go) and this file puts ours back.
//
// # WHY SSE AND NOT STDIO
//
// An MCP stdio server is a command the CHILD spawns. The tools would then run in
// a third process with no way back to the terminal, which loses the two things
// worth having: the permission prompt the user answers, and the receipt. Over
// SSE this program is the server, so Run() executes here, in this process, on the
// same permission system every other provider goes through.
//
// Measured 2026-10-12 against claude 2.1.296 while building this:
//
//   - MCP tools are presented to the model as mcp__<server>__<tool>.
//   - `--permission-prompts none` DENIES them. A probe with the flag and without
//     an allowlist came back "the session has no approval surface, so the
//     permission prompt was denied automatically", with the block recorded in
//     the result's permission_denials. The model then explained the failure
//     instead of doing the work, which reads exactly like a broken tool.
//   - Naming each tool in `--allowedTools` unblocks it: same probe, zero
//     denials, the tool ran and returned its value.
//
// The allowlist is therefore built from our own tool list, by name. Claude Code
// may call exactly the tools this program gave it and nothing else, and every
// one of those calls lands in the handler below, where this program's own
// permission check decides. Claude Code's permission layer is not being
// bypassed; it is being told that the gate is somewhere else, and it is.
//
// # THE LISTENER
//
// 127.0.0.1 on an ephemeral port, carrying a random bearer token generated per
// run and never written to disk. It exists only while a turn is in flight. The
// token is the reason the port is not an open door for anything else running as
// this user, and it is checked before mcp-go sees the request at all.
package provider

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/logging"
)

// bridgeServerName is the MCP server name the child sees. It becomes part of
// every tool's visible name (mcp__gorilla__view), so it is short and says whose
// tools these are.
const bridgeServerName = "gorilla"

// toolBridge serves this program's tools to one child process.
type toolBridge struct {
	tools    map[string]tools.BaseTool
	listener net.Listener
	http     *http.Server
	token    string
	url      string

	sessionID string
	messageID string

	mu    sync.Mutex
	calls []BridgeCall
}

// BridgeCall is one tool call the child made, recorded as it happened.
//
// The child executes its own loop, so the agent loop upstream never sees these
// as work to do. Recording them here is what keeps them visible: without it a
// turn that read six files would be reported as "no tool was called", which is
// the receipt lying.
type BridgeCall struct {
	// ID pairs the call with its result in the session record. The child has
	// its own tool_use ids, but they arrive on a different stream from the
	// execution, so the bridge mints its own and uses them for both halves.
	ID       string
	Name     string
	Input    string
	Output   string
	IsError  bool
	Duration time.Duration
}

// startToolBridge brings up the MCP server for one turn. The caller must Close
// it; nothing here outlives the turn.
func startToolBridge(ts []tools.BaseTool, sessionID, messageID string) (*toolBridge, error) {
	if len(ts) == 0 {
		return nil, nil
	}

	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		return nil, fmt.Errorf("could not generate a bridge token: %w", err)
	}

	b := &toolBridge{
		tools:     make(map[string]tools.BaseTool, len(ts)),
		token:     base64.RawURLEncoding.EncodeToString(tok),
		sessionID: sessionID,
		messageID: messageID,
	}

	srv := mcpserver.NewMCPServer("gorilla-opencode", "1.0.0")
	for _, t := range ts {
		info := t.Info()
		if info.Name == "" {
			continue
		}
		b.tools[info.Name] = t
		srv.AddTool(mcp.Tool{
			Name:        info.Name,
			Description: info.Description,
			InputSchema: mcp.ToolInputSchema{
				Type:       "object",
				Properties: info.Parameters,
				Required:   info.Required,
			},
		}, b.handler(info.Name))
	}

	// 127.0.0.1 only. A bridge reachable from the network would be a remote
	// code-execution hole wearing a feature's clothes.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("could not open the tool bridge: %w", err)
	}
	b.listener = ln

	sse := mcpserver.NewSSEServer(srv, mcpserver.WithBaseURL("http://"+ln.Addr().String()))
	b.http = &http.Server{Handler: b.authenticated(sse)}
	b.url = "http://" + ln.Addr().String() + sse.CompleteSsePath()

	go func() {
		if err := b.http.Serve(ln); err != nil && err != http.ErrServerClosed {
			logging.Debug("tool bridge stopped", "err", err)
		}
	}()
	return b, nil
}

// authenticated rejects anything without the token before mcp-go sees it.
// subtle.ConstantTimeCompare rather than == so the check does not leak the
// token a byte at a time through its own timing.
func (b *toolBridge) authenticated(next http.Handler) http.Handler {
	want := []byte("Bearer " + b.token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handler runs one of this program's tools for the child.
func (b *toolBridge) handler(name string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		t, ok := b.tools[name]
		if !ok {
			return mcp.NewToolResultText("no such tool: " + name), nil
		}

		args := "{}"
		if req.Params.Arguments != nil {
			if raw, err := json.Marshal(req.Params.Arguments); err == nil {
				args = string(raw)
			}
		}

		// The session and message ids are what the permission system and the
		// receipt key off. Without them a tool runs unattributed and the prompt
		// has nothing to attach itself to.
		ctx = context.WithValue(ctx, tools.SessionIDContextKey, b.sessionID)
		ctx = context.WithValue(ctx, tools.MessageIDContextKey, b.messageID)

		callID := b.nextCallID(name)
		start := time.Now()
		resp, err := t.Run(ctx, tools.ToolCall{ID: callID, Name: name, Input: args})
		took := time.Since(start)

		call := BridgeCall{ID: callID, Name: name, Input: args, Duration: took}
		if err != nil {
			call.IsError, call.Output = true, err.Error()
			b.record(call)
			// Returned as a tool RESULT rather than a transport error: the model
			// should see what went wrong and be able to try something else,
			// which a JSON-RPC level failure does not allow.
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: err.Error()}},
				IsError: true,
			}, nil
		}
		call.IsError, call.Output = resp.IsError, resp.Content
		b.record(call)
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: resp.Content}},
			IsError: resp.IsError,
		}, nil
	}
}

// nextCallID mints an id unique within this turn.
func (b *toolBridge) nextCallID(name string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return fmt.Sprintf("bridge_%s_%d", name, len(b.calls)+1)
}

func (b *toolBridge) record(c BridgeCall) {
	b.mu.Lock()
	b.calls = append(b.calls, c)
	b.mu.Unlock()
	logging.Debug("bridge tool call", "tool", c.Name, "error", c.IsError, "ms", c.Duration.Milliseconds())
}

// Calls returns what the child actually ran, in order.
func (b *toolBridge) Calls() []BridgeCall {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]BridgeCall, len(b.calls))
	copy(out, b.calls)
	return out
}

// mcpConfig is the --mcp-config value: the child is told where to connect and
// what token to present.
func (b *toolBridge) mcpConfig() (string, error) {
	cfg := map[string]any{
		"mcpServers": map[string]any{
			bridgeServerName: map[string]any{
				"type":    "sse",
				"url":     b.url,
				"headers": map[string]string{"Authorization": "Bearer " + b.token},
			},
		},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// allowedTools is the explicit allowlist. Built from our own tool names, so a
// tool this program did not supply cannot be on it.
//
// Sorted for a stable command line: an argument order that changes between runs
// is a diff nobody can read and a test nobody can pin.
func (b *toolBridge) allowedTools() []string {
	out := make([]string, 0, len(b.tools))
	for name := range b.tools {
		out = append(out, BridgeToolName(name))
	}
	sort.Strings(out)
	return out
}

// BridgeToolName is how the child sees one of our tools.
func BridgeToolName(name string) string {
	return "mcp__" + bridgeServerName + "__" + name
}

// LocalToolName is the inverse, for turning a name in the child's output back
// into one of ours. Returns "" for anything that is not one of ours.
func LocalToolName(name string) string {
	prefix := "mcp__" + bridgeServerName + "__"
	if !strings.HasPrefix(name, prefix) {
		return ""
	}
	return strings.TrimPrefix(name, prefix)
}

func (b *toolBridge) Close() {
	if b == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = b.http.Shutdown(ctx)
	_ = b.listener.Close()
}
