package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/session"
)

// A scripted engine: the "model" writes a reply in three steps, asks for a
// tool call, needs permission for it, and finishes. Everything the server
// sends to the editor is read back and checked, in order.

type fakeSessions struct{ n int }

func (f *fakeSessions) Create(ctx context.Context, title string) (session.Session, error) {
	f.n++
	return session.Session{ID: "sess-1", Title: title}, nil
}

type fakePerms struct {
	*pubsub.Broker[permission.PermissionRequest]
	mu      sync.Mutex
	answers []string
}

func (f *fakePerms) Grant(r permission.PermissionRequest) { f.note("grant") }
func (f *fakePerms) GrantPersistant(r permission.PermissionRequest) {
	f.note("grant-session")
}
func (f *fakePerms) Deny(r permission.PermissionRequest) { f.note("deny") }
func (f *fakePerms) note(s string) {
	f.mu.Lock()
	f.answers = append(f.answers, s)
	f.mu.Unlock()
}
func (f *fakePerms) answered() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.answers...)
}

type fakeRunner struct {
	msgs   *pubsub.Broker[message.Message]
	perms  *fakePerms
	cancel chan string
	script func(ctx context.Context, sessionID string, prompt string, out chan<- agent.AgentEvent)
}

func (f *fakeRunner) Run(ctx context.Context, sessionID, content string, _ ...message.Attachment) (<-chan agent.AgentEvent, error) {
	out := make(chan agent.AgentEvent, 1)
	go f.script(ctx, sessionID, content, out)
	return out, nil
}
func (f *fakeRunner) Cancel(sessionID string) { f.cancel <- sessionID }

// rig wires a server to pipes and returns a client side that can send
// requests and read what the agent writes, line by line.
type rig struct {
	t      *testing.T
	toSrv  io.WriteCloser
	from   *bufio.Scanner
	perms  *fakePerms
	runner *fakeRunner
	done   chan error
}

func newRig(t *testing.T, script func(ctx context.Context, sessionID, prompt string, out chan<- agent.AgentEvent)) *rig {
	t.Helper()
	msgs := pubsub.NewBroker[message.Message]()
	perms := &fakePerms{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	runner := &fakeRunner{msgs: msgs, perms: perms, cancel: make(chan string, 4), script: script}
	eng := Engine{
		Sessions:    &fakeSessions{},
		Messages:    msgs,
		Permissions: perms,
		Agent:       runner,
		Receipt: func(string) (string, bool) {
			return "--- what actually ran ---\n1 tool call:\n  view a.go -> ok\n", true
		},
	}
	clientIn, srvOut := io.Pipe()
	srvIn, clientOut := io.Pipe()
	srv := New(eng, "v-test", srvIn, srvOut)
	srv.AdoptFolder = func(string) error { return nil }
	done := make(chan error, 1)
	go func() { done <- srv.Serve(context.Background()) }()
	sc := bufio.NewScanner(clientIn)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	r := &rig{t: t, toSrv: clientOut, from: sc, perms: perms, runner: runner, done: done}
	t.Cleanup(func() { clientOut.Close() })
	return r
}

func (r *rig) send(v any) {
	b, _ := json.Marshal(v)
	if _, err := r.toSrv.Write(append(b, '\n')); err != nil {
		r.t.Fatal(err)
	}
}

func (r *rig) next() map[string]any {
	r.t.Helper()
	if !r.from.Scan() {
		r.t.Fatal("the agent closed the channel")
	}
	var m map[string]any
	if err := json.Unmarshal(r.from.Bytes(), &m); err != nil {
		r.t.Fatalf("not JSON: %s", r.from.Text())
	}
	return m
}

func assistant(id, sessionID, text, thinking string, calls ...message.ToolCall) message.Message {
	m := message.Message{ID: id, SessionID: sessionID, Role: message.Assistant}
	if thinking != "" {
		m.Parts = append(m.Parts, message.ReasoningContent{Thinking: thinking})
	}
	if text != "" {
		m.Parts = append(m.Parts, message.TextContent{Text: text})
	}
	for _, c := range calls {
		m.Parts = append(m.Parts, c)
	}
	return m
}

func TestAPromptTurnIsStreamedToTheEditorWithToolCallsPermissionAndReceipt(t *testing.T) {
	var sawPrompt string
	var r *rig
	r = newRig(t, func(ctx context.Context, sessionID, prompt string, out chan<- agent.AgentEvent) {
		sawPrompt = prompt
		// Thinking, then text in two steps.
		m := assistant("m1", sessionID, "", "let me look")
		runnerOf(t, r).msgs.Publish(pubsub.UpdatedEvent, m)
		m = assistant("m1", sessionID, "Reading", "let me look")
		runnerOf(t, r).msgs.Publish(pubsub.UpdatedEvent, m)
		call := message.ToolCall{ID: "call-1", Name: "view", Input: `{"file_path":"C:\\proj\\a.go"}`, Finished: true}
		m = assistant("m1", sessionID, "Reading a.go now.", "let me look", call)
		runnerOf(t, r).msgs.Publish(pubsub.UpdatedEvent, m)
		// The tool asks for permission and waits for the editor.
		req := permission.PermissionRequest{ID: "p1", SessionID: sessionID, ToolName: "view", Action: "read", Path: "C:\\proj\\a.go", Description: "Read a.go"}
		runnerOf(t, r).perms.Publish(pubsub.CreatedEvent, req)
		waitFor(t, func() bool { return len(r.perms.answered()) == 1 })
		// Tool result, then the final message.
		tr := message.Message{ID: "t1", SessionID: sessionID, Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "call-1", Name: "view", Content: "package main"}}}
		runnerOf(t, r).msgs.Publish(pubsub.CreatedEvent, tr)
		final := assistant("m2", sessionID, "It is a Go file.", "")
		final.AddFinish(message.FinishReasonEndTurn)
		runnerOf(t, r).msgs.Publish(pubsub.UpdatedEvent, final)
		out <- agent.AgentEvent{Type: agent.AgentEventTypeResponse, Message: final}
	})

	r.send(map[string]any{"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": map[string]any{"protocolVersion": 1, "clientInfo": map[string]any{"name": "test-editor", "version": "0"}}})
	init := r.next()
	res := init["result"].(map[string]any)
	if res["protocolVersion"].(float64) != 1 || res["agentInfo"].(map[string]any)["name"] != "gorilla-opencode" {
		t.Fatalf("initialize: %v", init)
	}
	if caps := res["agentCapabilities"].(map[string]any)["promptCapabilities"].(map[string]any); caps["embeddedContext"] != true {
		t.Fatalf("embedded context must be accepted: %v", caps)
	}

	r.send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "session/new", "params": map[string]any{"cwd": t.TempDir(), "mcpServers": []any{}}})
	ns := r.next()
	sid := ns["result"].(map[string]any)["sessionId"].(string)
	if sid != "sess-1" {
		t.Fatalf("session/new: %v", ns)
	}

	r.send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "session/prompt", "params": map[string]any{
		"sessionId": sid,
		"prompt": []map[string]any{
			{"type": "text", "text": "what is this file?"},
			{"type": "resource", "resource": map[string]any{"uri": "file:///C:/proj/a.go", "text": "package main"}},
		}}})

	var seen []string
	var promptResult map[string]any
	for promptResult == nil {
		m := r.next()
		switch m["method"] {
		case "session/update":
			u := m["params"].(map[string]any)["update"].(map[string]any)
			kind := u["sessionUpdate"].(string)
			line := kind
			if c, ok := u["content"].(map[string]any); ok {
				line += ":" + c["text"].(string)
			}
			if kind == "tool_call" {
				line += ":" + u["name"].(string) + ":" + u["kind"].(string) + ":" + u["status"].(string)
			}
			if kind == "tool_call_update" {
				line += ":" + u["status"].(string)
			}
			seen = append(seen, line)
		case "session/request_permission":
			p := m["params"].(map[string]any)
			opts := p["options"].([]any)
			if len(opts) != 3 {
				t.Fatalf("three options expected: %v", opts)
			}
			tc := p["toolCall"].(map[string]any)
			if tc["toolCallId"] != "call-1" {
				t.Errorf("the question must point at the announced call, got %v", tc["toolCallId"])
			}
			seen = append(seen, "permission:"+tc["title"].(string))
			r.send(map[string]any{"jsonrpc": "2.0", "id": m["id"], "result": map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow"}}})
		default:
			if m["id"] != nil && m["id"].(float64) == 2 {
				promptResult = m
			}
		}
	}

	want := []string{
		"agent_thought_chunk:let me look",
		"agent_message_chunk:Reading",
		"agent_message_chunk: a.go now.",
		"tool_call:view:read:in_progress",
		"permission:Read a.go",
		"tool_call_update:completed",
		"agent_message_chunk:It is a Go file.",
		"agent_message_chunk:\n\n--- what actually ran ---\n1 tool call:\n  view a.go -> ok\n",
	}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Errorf("the editor saw:\n%s\nwanted:\n%s", strings.Join(seen, "\n"), strings.Join(want, "\n"))
	}
	if stop := promptResult["result"].(map[string]any)["stopReason"]; stop != "end_turn" {
		t.Errorf("stopReason %v", stop)
	}
	if r.perms.answered()[0] != "grant" {
		t.Errorf("the editor's Allow became %q", r.perms.answered()[0])
	}
	if !strings.Contains(sawPrompt, "what is this file?") || !strings.Contains(sawPrompt, `<resource uri="file:///C:/proj/a.go">`) {
		t.Errorf("the agent got the prompt %q", sawPrompt)
	}
}

func TestCancelStopsTheTurnAndADeniedQuestionIsADenial(t *testing.T) {
	var r *rig
	r = newRig(t, func(ctx context.Context, sessionID, prompt string, out chan<- agent.AgentEvent) {
		req := permission.PermissionRequest{ID: "p1", SessionID: sessionID, ToolName: "bash", Action: "run", Egress: true}
		runnerOf(t, r).perms.Publish(pubsub.CreatedEvent, req)
		waitFor(t, func() bool { return len(r.perms.answered()) == 1 })
		<-ctx.Done()
		out <- agent.AgentEvent{Type: agent.AgentEventTypeError, Error: agent.ErrRequestCancelled}
	})
	r.send(map[string]any{"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": map[string]any{"protocolVersion": 1}})
	r.next()
	r.send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "session/new", "params": map[string]any{"cwd": t.TempDir(), "mcpServers": []any{}}})
	sid := r.next()["result"].(map[string]any)["sessionId"].(string)
	r.send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "session/prompt", "params": map[string]any{"sessionId": sid, "prompt": []map[string]any{{"type": "text", "text": "rm things"}}}})

	q := r.next()
	if q["method"] != "session/request_permission" {
		t.Fatalf("expected the permission question, got %v", q)
	}
	if title := q["params"].(map[string]any)["toolCall"].(map[string]any)["title"].(string); !strings.Contains(title, "sends data off this machine") {
		t.Errorf("an egress question must say so: %q", title)
	}
	r.send(map[string]any{"jsonrpc": "2.0", "id": q["id"], "result": map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "deny"}}})
	waitFor(t, func() bool { return len(r.perms.answered()) == 1 })
	if r.perms.answered()[0] != "deny" {
		t.Fatalf("Deny became %q", r.perms.answered()[0])
	}

	r.send(map[string]any{"jsonrpc": "2.0", "method": "session/cancel", "params": map[string]any{"sessionId": sid}})
	select {
	case got := <-r.runner.cancel:
		if got != sid {
			t.Errorf("cancelled %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session/cancel never reached the agent")
	}
	for {
		m := r.next()
		if m["id"] != nil && m["id"].(float64) == 2 {
			if stop := m["result"].(map[string]any)["stopReason"]; stop != "cancelled" {
				t.Errorf("stopReason %v", stop)
			}
			return
		}
	}
}

func TestUnknownMethodAndBadSessionAreErrorsNotSilence(t *testing.T) {
	r := newRig(t, func(ctx context.Context, sessionID, prompt string, out chan<- agent.AgentEvent) {})
	r.send(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "session/load", "params": map[string]any{}})
	m := r.next()
	if e := m["error"].(map[string]any); e["code"].(float64) != -32601 {
		t.Errorf("session/load must be method-not-found, got %v", m)
	}
	r.send(map[string]any{"jsonrpc": "2.0", "id": 8, "method": "session/prompt", "params": map[string]any{"sessionId": "nope", "prompt": []map[string]any{{"type": "text", "text": "x"}}}})
	m = r.next()
	if e := m["error"].(map[string]any); e["code"].(float64) != -32602 {
		t.Errorf("an unknown session must be invalid params, got %v", m)
	}
	r.send(map[string]any{"jsonrpc": "2.0", "id": 9, "method": "session/new", "params": map[string]any{"cwd": "relative/path", "mcpServers": []any{}}})
	m = r.next()
	if e := m["error"].(map[string]any); e["code"].(float64) != -32602 {
		t.Errorf("a relative cwd must be refused, got %v", m)
	}
}

func TestToolKindsAndTitles(t *testing.T) {
	for name, want := range map[string]string{"view": "read", "edit": "edit", "find": "search", "bash": "execute", "fetch": "fetch", "research": "think", "diagnostics": "other"} {
		if got := toolKind(name); got != want {
			t.Errorf("%s -> %s, want %s", name, got, want)
		}
	}
	tc := message.ToolCall{Name: "find", Input: `{"path":"C:\\p","query":"needle"}`}
	if got := toolTitle(tc); got != "find needle in C:\\p" {
		t.Errorf("title %q", got)
	}
	if loc := toolLocations(message.ToolCall{Name: "view", Input: `{"file_path":"C:\\p\\a.go"}`}); len(loc) != 1 || loc[0]["path"] != "C:\\p\\a.go" {
		t.Errorf("locations %v", loc)
	}
}

func runnerOf(t *testing.T, r *rig) *fakeRunner { t.Helper(); return r.runner }

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}
