package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/session"
)

// ProtocolVersion is the ACP major version this agent speaks.
const ProtocolVersion = 1

// Engine is what the server needs from the program: the same four services
// the plain front end drives. An interface so the server can be tested
// without a model, a database or a terminal.
type Engine struct {
	Sessions    SessionCreator
	Messages    pubsub.Suscriber[message.Message]
	Permissions PermissionDesk
	Agent       Runner
	// Receipt returns the program's own record of what ran in a session
	// (internal/app/receipt.go). Optional.
	Receipt func(sessionID string) (string, bool)
}

// SessionCreator is the one thing the server needs from the session store.
type SessionCreator interface {
	Create(ctx context.Context, title string) (session.Session, error)
}

// PermissionDesk is where the editor's answers to permission questions go.
type PermissionDesk interface {
	pubsub.Suscriber[permission.PermissionRequest]
	Grant(permission.PermissionRequest)
	GrantPersistant(permission.PermissionRequest)
	Deny(permission.PermissionRequest)
}

// Runner runs one prompt turn and can cancel it.
type Runner interface {
	Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan agent.AgentEvent, error)
	Cancel(sessionID string)
}

// Server is one ACP connection.
type Server struct {
	eng     Engine
	conn    *conn
	version string

	// AdoptFolder makes the editor's session folder the working folder, as
	// `-c` does at launch. A seam so the server can be tested without a
	// loaded configuration.
	AdoptFolder func(cwd string) error

	mu       sync.Mutex
	sessions map[string]*acpSession
	inited   bool
}

// adoptWorkingDir is the real AdoptFolder: chdir, then record the folder the
// way a launch with -c records it. Roots the user added on purpose are kept.
func adoptWorkingDir(cwd string) error {
	want := filepath.Clean(cwd)
	if want == filepath.Clean(config.WorkingDirectory()) {
		return nil
	}
	if err := os.Chdir(want); err != nil {
		return err
	}
	if _, err := config.SetWorkingDir(want, false); err != nil {
		logging.Warn("acp: could not record the editor's folder", "cwd", want, "err", err)
	}
	return nil
}

type acpSession struct {
	id     string
	cwd    string
	cancel context.CancelFunc // cancels the running prompt turn, if any
}

// New builds a server over r (client to agent) and w (agent to client).
func New(eng Engine, version string, r io.Reader, w io.Writer) *Server {
	return &Server{eng: eng, conn: newConn(r, w), version: version, sessions: map[string]*acpSession{}, AdoptFolder: adoptWorkingDir}
}

// Serve reads messages until the client closes its side or ctx ends.
// Requests are handled concurrently: session/cancel must get through while a
// session/prompt is still running.
func (s *Server) Serve(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	for s.conn.in.Scan() {
		line := s.conn.in.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var m rpcMessage
		if err := json.Unmarshal(line, &m); err != nil {
			_ = s.conn.respondError(nil, codeParse, "the line was not JSON: "+err.Error())
			continue
		}
		if m.Method == "" {
			// An answer to something we asked.
			if !s.conn.deliverResponse(m) {
				logging.Warn("acp: answer for an unknown request", "id", string(m.ID))
			}
			continue
		}
		msg := m
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.dispatch(ctx, msg)
		}()
	}
	return s.conn.in.Err()
}

func (s *Server) dispatch(ctx context.Context, m rpcMessage) {
	isNotification := len(m.ID) == 0
	reply := func(result any, err error) {
		if isNotification {
			return
		}
		if err != nil {
			var re *rpcError
			if errors.As(err, &re) {
				_ = s.conn.respondError(m.ID, re.Code, re.Message)
				return
			}
			_ = s.conn.respondError(m.ID, codeInternal, err.Error())
			return
		}
		_ = s.conn.respond(m.ID, result)
	}
	switch m.Method {
	case "initialize":
		reply(s.initialize(m.Params))
	case "authenticate":
		// No account, no key of its own: there is nothing to authenticate.
		reply(map[string]any{}, nil)
	case "session/new":
		reply(s.newSession(ctx, m.Params))
	case "session/prompt":
		reply(s.prompt(ctx, m.Params))
	case "session/cancel":
		s.cancelSession(m.Params)
		reply(map[string]any{}, nil)
	default:
		reply(nil, &rpcError{Code: codeMethodNotFound, Message: "method not supported: " + m.Method})
	}
}

// ---------------------------------------------------------------------------
// initialize
// ---------------------------------------------------------------------------

type initializeParams struct {
	ProtocolVersion int `json:"protocolVersion"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

func (s *Server) initialize(raw json.RawMessage) (any, error) {
	var p initializeParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
		}
	}
	s.mu.Lock()
	s.inited = true
	s.mu.Unlock()
	logging.Info("acp: initialised", "client", p.ClientInfo.Name, "client_version", p.ClientInfo.Version, "requested", p.ProtocolVersion)
	return map[string]any{
		"protocolVersion": ProtocolVersion,
		"agentCapabilities": map[string]any{
			"loadSession": false,
			"promptCapabilities": map[string]any{
				"image":           false,
				"audio":           false,
				"embeddedContext": true,
			},
			"mcpCapabilities": map[string]any{"http": false, "sse": false},
		},
		"agentInfo": map[string]any{
			"name":    "gorilla-opencode",
			"title":   "Gorilla OpenCode",
			"version": s.version,
		},
		"authMethods": []any{},
	}, nil
}

// ---------------------------------------------------------------------------
// session/new
// ---------------------------------------------------------------------------

type newSessionParams struct {
	Cwd        string            `json:"cwd"`
	MCPServers []json.RawMessage `json:"mcpServers"`
}

func (s *Server) newSession(ctx context.Context, raw json.RawMessage) (any, error) {
	var p newSessionParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}
	if p.Cwd == "" || !filepath.IsAbs(p.Cwd) {
		return nil, &rpcError{Code: codeInvalidParams, Message: "cwd must be an absolute path"}
	}
	// The program was started in one folder; an editor may open a session in
	// another. The folder is adopted for the session the same way `-c` adopts
	// it at launch.
	if err := s.AdoptFolder(p.Cwd); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "cwd: " + err.Error()}
	}
	if len(p.MCPServers) > 0 {
		logging.Warn("acp: the editor offered MCP servers; this version does not take them", "count", len(p.MCPServers))
	}
	sess, err := s.eng.Sessions.Create(ctx, "Editor session: "+filepath.Base(p.Cwd))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.sessions[sess.ID] = &acpSession{id: sess.ID, cwd: p.Cwd}
	s.mu.Unlock()
	return map[string]any{"sessionId": sess.ID}, nil
}

// ---------------------------------------------------------------------------
// session/prompt
// ---------------------------------------------------------------------------

type contentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	URI      string `json:"uri,omitempty"`
	Name     string `json:"name,omitempty"`
	Resource *struct {
		URI      string `json:"uri"`
		MimeType string `json:"mimeType,omitempty"`
		Text     string `json:"text,omitempty"`
	} `json:"resource,omitempty"`
}

type promptParams struct {
	SessionID string         `json:"sessionId"`
	Prompt    []contentBlock `json:"prompt"`
}

// promptText flattens the client's content blocks into the one string the
// agent takes. Embedded resources are quoted with their uri, as Kimi and the
// spec's own examples do; links become a line naming the file.
func promptText(blocks []contentBlock) string {
	var b strings.Builder
	for _, c := range blocks {
		switch c.Type {
		case "text":
			b.WriteString(c.Text)
			b.WriteString("\n")
		case "resource_link":
			fmt.Fprintf(&b, "[file: %s]\n", strings.TrimPrefix(c.URI, "file://"))
		case "resource":
			if c.Resource != nil && c.Resource.Text != "" {
				fmt.Fprintf(&b, "<resource uri=%q>\n%s\n</resource>\n", c.Resource.URI, c.Resource.Text)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func (s *Server) prompt(ctx context.Context, raw json.RawMessage) (any, error) {
	var p promptParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}
	s.mu.Lock()
	as, ok := s.sessions[p.SessionID]
	s.mu.Unlock()
	if !ok {
		return nil, &rpcError{Code: codeInvalidParams, Message: "unknown sessionId"}
	}
	text := promptText(p.Prompt)
	if text == "" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "the prompt has no text"}
	}

	turnCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.mu.Lock()
	as.cancel = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		as.cancel = nil
		s.mu.Unlock()
	}()

	// Subscribe BEFORE starting the agent, or the first tokens are missed.
	msgs := s.eng.Messages.Subscribe(turnCtx)
	perms := s.eng.Permissions.Subscribe(turnCtx)

	done, err := s.eng.Agent.Run(turnCtx, as.id, text)
	if err != nil {
		return nil, err
	}
	st := newTurnState(s, as.id)
	for {
		select {
		case <-turnCtx.Done():
			return map[string]any{"stopReason": "cancelled"}, nil
		case ev := <-msgs:
			if ev.Payload.SessionID == as.id {
				st.render(ev.Payload)
			}
		case req := <-perms:
			if req.Payload.SessionID == as.id {
				// The tool call that asks is announced before its question:
				// both arrive at once and Go's select has no preference.
				st.drain(msgs)
				s.askPermission(turnCtx, st, req.Payload)
			}
		case result := <-done:
			st.drain(msgs)
			return st.finish(result), nil
		}
	}
}

func (s *Server) cancelSession(raw json.RawMessage) {
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	s.mu.Lock()
	as, ok := s.sessions[p.SessionID]
	var cancel context.CancelFunc
	if ok {
		cancel = as.cancel
	}
	s.mu.Unlock()
	if !ok {
		return
	}
	s.eng.Agent.Cancel(as.id)
	if cancel != nil {
		cancel()
	}
}

// ---------------------------------------------------------------------------
// Streaming one turn
// ---------------------------------------------------------------------------

// turnState remembers what has already been sent for each message, because
// every update carries the whole message and only the new suffix is a chunk.
type turnState struct {
	s         *Server
	sessionID string
	sentText  map[string]int    // message id -> runes of content already sent
	sentThink map[string]int    // message id -> runes of reasoning already sent
	calls     map[string]string // tool call id -> tool name, once announced
	finished  map[string]bool   // tool call id -> result sent
}

func newTurnState(s *Server, sessionID string) *turnState {
	return &turnState{s: s, sessionID: sessionID, sentText: map[string]int{}, sentThink: map[string]int{},
		calls: map[string]string{}, finished: map[string]bool{}}
}

func (st *turnState) update(u map[string]any) {
	_ = st.s.conn.notify("session/update", map[string]any{"sessionId": st.sessionID, "update": u})
}

func textBlock(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func (st *turnState) render(m message.Message) {
	switch m.Role {
	case message.Assistant:
		st.streamChunk("agent_thought_chunk", m.ID, m.ReasoningContent().Thinking, st.sentThink)
		st.streamChunk("agent_message_chunk", m.ID, m.Content().String(), st.sentText)
		for _, tc := range m.ToolCalls() {
			if _, seen := st.calls[tc.ID]; seen || tc.ID == "" {
				continue
			}
			st.calls[tc.ID] = tc.Name
			u := map[string]any{
				"sessionUpdate": "tool_call",
				"toolCallId":    tc.ID,
				"name":          tc.Name,
				"title":         toolTitle(tc),
				"kind":          toolKind(tc.Name),
				"status":        "in_progress",
			}
			if loc := toolLocations(tc); len(loc) > 0 {
				u["locations"] = loc
			}
			if tc.Input != "" && json.Valid([]byte(tc.Input)) {
				u["rawInput"] = json.RawMessage(tc.Input)
			}
			st.update(u)
		}
	case message.Tool:
		for _, tr := range m.ToolResults() {
			if st.finished[tr.ToolCallID] || tr.ToolCallID == "" {
				continue
			}
			st.finished[tr.ToolCallID] = true
			status := "completed"
			if tr.IsError {
				status = "failed"
			}
			u := map[string]any{
				"sessionUpdate": "tool_call_update",
				"toolCallId":    tr.ToolCallID,
				"status":        status,
			}
			if c := strings.TrimSpace(tr.Content); c != "" {
				u["content"] = []map[string]any{{"type": "content", "content": textBlock(clipResult(c))}}
				u["rawOutput"] = c
			}
			st.update(u)
		}
	}
}

func (st *turnState) streamChunk(kind, id, full string, sent map[string]int) {
	r := []rune(full)
	n := sent[id]
	if len(r) <= n {
		return
	}
	sent[id] = len(r)
	st.update(map[string]any{"sessionUpdate": kind, "content": textBlock(string(r[n:]))})
}

func (st *turnState) drain(msgs <-chan pubsub.Event[message.Message]) {
	for {
		select {
		case ev := <-msgs:
			if ev.Payload.SessionID == st.sessionID {
				st.render(ev.Payload)
			}
		default:
			return
		}
	}
}

// finish maps the agent's result to a stop reason, and sends the receipt,
// which is the program's own record of what ran and is not written by the AI.
func (st *turnState) finish(result agent.AgentEvent) map[string]any {
	stop := "end_turn"
	if result.Error != nil {
		switch {
		case errors.Is(result.Error, context.Canceled), errors.Is(result.Error, agent.ErrRequestCancelled):
			stop = "cancelled"
		default:
			st.update(map[string]any{"sessionUpdate": "agent_message_chunk",
				"content": textBlock("\n\n" + result.Error.Error() + "\n")})
		}
	} else {
		switch result.Message.FinishReason() {
		case message.FinishReasonMaxTokens:
			stop = "max_tokens"
		case message.FinishReasonCanceled:
			stop = "cancelled"
		case message.FinishReasonPermissionDenied:
			stop = "refusal"
		}
	}
	if st.s.eng.Receipt != nil && os.Getenv("GORILLA_OPENCODE_NO_RECEIPT") != "1" {
		if r, ok := st.s.eng.Receipt(st.sessionID); ok && strings.TrimSpace(r) != "" {
			st.update(map[string]any{"sessionUpdate": "agent_message_chunk", "content": textBlock("\n\n" + r)})
		}
	}
	return map[string]any{"stopReason": stop}
}

// ---------------------------------------------------------------------------
// Permission
// ---------------------------------------------------------------------------

// askPermission turns the program's permission question into the editor's.
// The editor shows it; the answer is applied exactly as the TUI applies a key
// press. A cancelled turn, or a closed editor, is a refusal: never consent.
func (s *Server) askPermission(ctx context.Context, st *turnState, req permission.PermissionRequest) {
	title := req.ToolName + ": " + req.Action
	if req.Description != "" {
		title = req.Description
	}
	if req.Egress {
		title += " (sends data off this machine)"
	}
	tc := map[string]any{
		"toolCallId": st.toolCallIDFor(req),
		"title":      title,
		"kind":       toolKind(req.ToolName),
		"status":     "pending",
	}
	if req.Path != "" {
		tc["locations"] = []map[string]any{{"path": req.Path}}
	}
	if b, err := json.Marshal(req.Params); err == nil && string(b) != "null" {
		tc["rawInput"] = json.RawMessage(b)
	}
	params := map[string]any{
		"sessionId": st.sessionID,
		"toolCall":  tc,
		"options": []map[string]any{
			{"optionId": "allow", "name": "Allow", "kind": "allow_once"},
			{"optionId": "allow-session", "name": "Allow for this session", "kind": "allow_always"},
			{"optionId": "deny", "name": "Deny", "kind": "reject_once"},
		},
	}
	raw, err := s.conn.call(ctx, "session/request_permission", params)
	if err != nil {
		logging.Warn("acp: permission question not answered; denied", "tool", req.ToolName, "err", err)
		s.eng.Permissions.Deny(req)
		return
	}
	var out struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	if json.Unmarshal(raw, &out) != nil || out.Outcome.Outcome != "selected" {
		s.eng.Permissions.Deny(req)
		return
	}
	switch out.Outcome.OptionID {
	case "allow":
		s.eng.Permissions.Grant(req)
	case "allow-session":
		s.eng.Permissions.GrantPersistant(req)
	default:
		s.eng.Permissions.Deny(req)
	}
}

// toolCallIDFor finds the announced tool call a permission question belongs
// to: the most recent unfinished call of that tool. The permission request
// itself does not carry the call id.
func (st *turnState) toolCallIDFor(req permission.PermissionRequest) string {
	for id, name := range st.calls {
		if name == req.ToolName && !st.finished[id] {
			return id
		}
	}
	return "permission-" + req.ID
}

// ---------------------------------------------------------------------------
// Presentation helpers
// ---------------------------------------------------------------------------

// toolKind maps this program's tools to ACP's categories, which editors use
// for icons. Unknown tools are "other".
func toolKind(name string) string {
	switch strings.ToLower(name) {
	case "view", "read", "ls":
		return "read"
	case "edit", "write", "patch", "patch_port":
		return "edit"
	case "find", "glob", "grep", "sparse":
		return "search"
	case "bash", "shell":
		return "execute"
	case "fetch", "web_search", "websearch", "bio_lookup":
		return "fetch"
	case "agent", "research", "review":
		return "think"
	}
	return "other"
}

// toolTitle is one line for the editor's tool row: the tool and what it is about.
func toolTitle(tc message.ToolCall) string {
	var args map[string]any
	if json.Unmarshal([]byte(tc.Input), &args) == nil {
		for _, key := range []string{"command", "file_path", "path", "query", "url", "pattern", "prompt", "question"} {
			if v, ok := args[key].(string); ok && v != "" {
				if q, ok := args["query"].(string); key == "path" && ok && q != "" {
					v = q + " in " + v
				}
				return tc.Name + " " + clipResult(firstLine(v))
			}
		}
	}
	return tc.Name
}

func toolLocations(tc message.ToolCall) []map[string]any {
	var args map[string]any
	if json.Unmarshal([]byte(tc.Input), &args) != nil {
		return nil
	}
	for _, key := range []string{"file_path", "path"} {
		if v, ok := args[key].(string); ok && v != "" && filepath.IsAbs(v) {
			return []map[string]any{{"path": v}}
		}
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// clipResult bounds what goes into the editor's tool row. Editors keep every
// update in memory; a 32 KB search result does not belong there whole.
func clipResult(s string) string {
	const max = 4000
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + fmt.Sprintf("\n[... %d more characters not shown]", len(r)-max)
}
