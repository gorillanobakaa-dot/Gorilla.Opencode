package agent

// GORILLA OVERRIDE (2026-10-05): a stand-in AI, so a WHOLE TURN can be tested.
//
// Until this file the agent package had no way to run a turn without a real
// provider. Two faults closed in v0.1.138 were therefore shipped on reading
// alone: denying one tool call did not stop the others in the same message
// (a `break` that only left a `select`), and a helper agent starting cleared
// the session's untrusted-content flag. Both sit in the loop itself, where a
// test of a helper function cannot reach.
//
// scriptedProvider plays the model: each call to StreamResponse returns the
// next scripted reply. memMessages and memSessions are the two stores, in
// memory. Everything else is the real agent: Run, processGeneration,
// streamAndHandleEvents, the tool loop, finishToolResults.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/provider"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/session"
)

// reply is one scripted answer from the stand-in model.
type reply struct {
	text  string
	calls []message.ToolCall
}

type scriptedProvider struct {
	mu      sync.Mutex
	script  []reply
	asked   int                 // how many times the model was called
	history [][]message.Message // what it was sent each time
}

func (p *scriptedProvider) Model() models.Model {
	return models.Model{ID: "test.stand-in", Name: "stand-in", ContextWindow: 1_000_000, DefaultMaxTokens: 4096}
}
func (p *scriptedProvider) SystemPrompt() string { return "" }
func (p *scriptedProvider) SendMessages(context.Context, []message.Message, []tools.BaseTool) (*provider.ProviderResponse, error) {
	return nil, fmt.Errorf("the stand-in only streams")
}

func (p *scriptedProvider) StreamResponse(_ context.Context, msgs []message.Message, _ []tools.BaseTool) <-chan provider.ProviderEvent {
	p.mu.Lock()
	r := reply{text: "done"}
	if p.asked < len(p.script) {
		r = p.script[p.asked]
	}
	p.asked++
	p.history = append(p.history, msgs)
	p.mu.Unlock()

	ch := make(chan provider.ProviderEvent, 4)
	go func() {
		defer close(ch)
		if r.text != "" {
			ch <- provider.ProviderEvent{Type: provider.EventContentDelta, Content: r.text}
		}
		finish := message.FinishReasonEndTurn
		if len(r.calls) > 0 {
			finish = message.FinishReasonToolUse
		}
		ch <- provider.ProviderEvent{Type: provider.EventComplete, Response: &provider.ProviderResponse{
			Content: r.text, ToolCalls: r.calls, FinishReason: finish,
		}}
	}()
	return ch
}

type memMessages struct {
	message.Service
	mu   sync.Mutex
	next int
	all  []message.Message
}

func (m *memMessages) Create(_ context.Context, sessionID string, p message.CreateMessageParams) (message.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next++
	msg := message.Message{ID: fmt.Sprintf("m%d", m.next), SessionID: sessionID, Role: p.Role, Parts: p.Parts, Model: p.Model}
	m.all = append(m.all, msg)
	return msg, nil
}

func (m *memMessages) Update(_ context.Context, msg message.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.all {
		if m.all[i].ID == msg.ID {
			m.all[i] = msg
		}
	}
	return nil
}

func (m *memMessages) List(_ context.Context, sessionID string) ([]message.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []message.Message
	for _, msg := range m.all {
		if msg.SessionID == sessionID {
			out = append(out, msg)
		}
	}
	return out, nil
}

type memSessions struct {
	session.Service
	mu  sync.Mutex
	all map[string]session.Session
}

func (s *memSessions) Get(_ context.Context, id string) (session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.all == nil {
		s.all = map[string]session.Session{}
	}
	sess, ok := s.all[id]
	if !ok {
		sess = session.Session{ID: id}
		s.all[id] = sess
	}
	return sess, nil
}

func (s *memSessions) Save(_ context.Context, sess session.Session) (session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.all == nil {
		s.all = map[string]session.Session{}
	}
	s.all[sess.ID] = sess
	return sess, nil
}

// scriptedTool is a tool whose behaviour the test decides.
type scriptedTool struct {
	name string
	mu   sync.Mutex
	runs int
	do   func() (tools.ToolResponse, error)
}

func (t *scriptedTool) Info() tools.ToolInfo {
	return tools.ToolInfo{Name: t.name, Description: "test tool", Parameters: map[string]any{}}
}

func (t *scriptedTool) Run(context.Context, tools.ToolCall) (tools.ToolResponse, error) {
	t.mu.Lock()
	t.runs++
	t.mu.Unlock()
	if t.do != nil {
		return t.do()
	}
	return tools.NewTextResponse("ran " + t.name), nil
}

func (t *scriptedTool) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.runs
}

// newLoopAgent builds a real agent around the stand-ins.
func newLoopAgent(t *testing.T, name config.AgentName, p provider.Provider, ts ...tools.BaseTool) (*agent, *memMessages) {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", "test-key-for-a-loaded-config")
	if _, err := config.Load(t.TempDir(), false); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	msgs := &memMessages{}
	return &agent{
		Broker:    pubsub.NewBroker[AgentEvent](),
		agentName: name,
		sessions:  &memSessions{},
		messages:  msgs,
		tools:     ts,
		provider:  p,
	}, msgs
}

// runTurn sends one user message and waits for the turn to end.
func runTurn(t *testing.T, a *agent, sessionID, text string) AgentEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	done, err := a.Run(ctx, sessionID, text)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	select {
	case ev := <-done:
		return ev
	case <-ctx.Done():
		t.Fatal("the turn did not end within 20 seconds")
		return AgentEvent{}
	}
}

func call(id, name string) message.ToolCall {
	return message.ToolCall{ID: id, Name: name, Input: "{}", Type: "function", Finished: true}
}

// The harness itself must be sound before anything is concluded from it: an
// ordinary turn with one tool call runs the tool, sends the result back to the
// model, and ends with the model's answer.
func TestTheStandInDrivesAnOrdinaryTurn(t *testing.T) {
	tool := &scriptedTool{name: "probe_tool"}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{call("c1", "probe_tool")}},
		{text: "all finished"},
	}}
	a, store := newLoopAgent(t, config.AgentCoder, p, tool)

	ev := runTurn(t, a, "ordinary", "please run the tool")
	if ev.Error != nil {
		t.Fatalf("turn failed: %v", ev.Error)
	}
	if tool.count() != 1 {
		t.Fatalf("the tool ran %d times, want 1", tool.count())
	}
	if p.asked != 2 {
		t.Fatalf("the model was called %d times, want 2 (the call, then the answer)", p.asked)
	}
	// The second request must carry the tool's result.
	sawResult := false
	for _, m := range p.history[1] {
		for _, r := range m.ToolResults() {
			sawResult = sawResult || (r.ToolCallID == "c1" && strings.Contains(r.Content, "ran probe_tool"))
		}
	}
	if !sawResult {
		t.Error("the tool's result was not sent back to the model")
	}
	if got := ev.Message.Content().String(); got != "all finished" {
		t.Errorf("final answer = %q", got)
	}
	if a.IsSessionBusy("ordinary") {
		t.Error("the session is still marked busy after its turn ended")
	}
	all, _ := store.List(context.Background(), "ordinary")
	if len(all) < 4 {
		t.Errorf("stored %d messages, want user, assistant, tool, assistant", len(all))
	}
}

// AUDIT FINDING 3. The model asks for three things in one message. The person
// refuses the first. The other two must NOT run.
//
// Before v0.1.138 they did: the refusal marked them "canceled by user", a
// `break` left only the enclosing `select`, the loop went on, and both ran.
// Put `break` back in place of `goto out` in agent.go and this fails with the
// second and third tools each having run once.
func TestRefusingOneActionStopsTheOthersSentWithIt(t *testing.T) {
	refused := &scriptedTool{name: "needs_permission", do: func() (tools.ToolResponse, error) {
		return tools.ToolResponse{}, permission.ErrorPermissionDenied
	}}
	second := &scriptedTool{name: "delete_things"}
	third := &scriptedTool{name: "send_things"}
	p := &scriptedProvider{script: []reply{
		{calls: []message.ToolCall{call("c1", "needs_permission"), call("c2", "delete_things"), call("c3", "send_things")}},
		{text: "the model must not be asked again after a refusal"},
	}}
	a, store := newLoopAgent(t, config.AgentCoder, p, refused, second, third)

	runTurn(t, a, "denied", "do three things")

	if refused.count() != 1 {
		t.Fatalf("the refused tool was attempted %d times, want 1", refused.count())
	}
	if second.count() != 0 || third.count() != 0 {
		t.Fatalf("after a refusal the remaining tools RAN: second=%d third=%d. The person said no.",
			second.count(), third.count())
	}
	// What is stored must say so: one refusal, two cancellations, and nothing
	// that claims the cancelled calls did anything.
	all, _ := store.List(context.Background(), "denied")
	got := map[string]string{}
	for _, m := range all {
		for _, r := range m.ToolResults() {
			got[r.ToolCallID] = r.Content
		}
	}
	if !strings.HasPrefix(strings.ToLower(got["c1"]), "permission denied") {
		t.Errorf("c1 result = %q, want the refusal", got["c1"])
	}
	for _, id := range []string{"c2", "c3"} {
		if !strings.Contains(got[id], "canceled") {
			t.Errorf("%s result = %q, want a cancellation", id, got[id])
		}
		if strings.Contains(got[id], "ran ") {
			t.Errorf("%s carries the output of a tool that should not have run: %q", id, got[id])
		}
	}
	if p.asked != 1 {
		t.Errorf("the model was called %d times; a refusal ends the turn", p.asked)
	}
}

// AUDIT FINDING 4. A turn that has read something a stranger wrote is marked,
// and the mark makes the next risky action ask first. Only the person typing a
// new message may clear it. A helper agent starting is the model acting, not
// the person.
//
// Before v0.1.138 every agent cleared it. Remove the `agentName ==
// config.AgentCoder` guard in processGeneration and the first half fails.
func TestOnlyAMessageFromThePersonClearsTheUntrustedContentMark(t *testing.T) {
	const sess = "tainted-session"
	t.Cleanup(func() { permission.ClearTaint(sess) })

	helper, _ := newLoopAgent(t, config.AgentTask, &scriptedProvider{script: []reply{{text: "helper done"}}})
	permission.MarkTainted(sess, "a web page was read")
	if ev := runTurn(t, helper, sess, "look into it"); ev.Error != nil {
		t.Fatalf("helper turn failed: %v", ev.Error)
	}
	if !permission.IsTainted(sess) {
		t.Fatal("a helper agent starting cleared the untrusted-content mark; the next action would be auto-approved")
	}

	coder, _ := newLoopAgent(t, config.AgentCoder, &scriptedProvider{script: []reply{{text: "ok"}}})
	if ev := runTurn(t, coder, sess, "a new message typed by the person"); ev.Error != nil {
		t.Fatalf("coder turn failed: %v", ev.Error)
	}
	if permission.IsTainted(sess) {
		t.Error("a new message from the person did not clear the mark; every later action would ask for ever")
	}
}

// AUDIT FINDING 14, in the loop. A connection that drops closes the stream with
// neither a completion nor an error. That must end the turn as a failure, not
// be stored as an answer.
type droppingProvider struct{ scriptedProvider }

func (p *droppingProvider) StreamResponse(context.Context, []message.Message, []tools.BaseTool) <-chan provider.ProviderEvent {
	ch := make(chan provider.ProviderEvent, 1)
	ch <- provider.ProviderEvent{Type: provider.EventContentDelta, Content: "half an ans"}
	close(ch)
	return ch
}

func TestAReplyCutOffMidWayIsAFailureNotAnAnswer(t *testing.T) {
	a, store := newLoopAgent(t, config.AgentCoder, &droppingProvider{})
	ev := runTurn(t, a, "dropped", "say something long")
	if ev.Error == nil {
		t.Fatal("a stream that stopped without finishing was accepted as a complete answer")
	}
	if !strings.Contains(ev.Error.Error(), "before the reply was complete") {
		t.Errorf("the error does not say what happened: %v", ev.Error)
	}
	all, _ := store.List(context.Background(), "dropped")
	last := all[len(all)-1]
	if last.FinishReason() != message.FinishReasonError {
		t.Errorf("the half reply is stored with finish %q, want an error", last.FinishReason())
	}
}

// AUDIT FINDING 19, in the loop. A session left with an unanswered tool call
// (the program was killed mid-tool) must be usable again, and the model must be
// told the call never came back.
func TestAnInterruptedSessionCanBeContinued(t *testing.T) {
	p := &scriptedProvider{script: []reply{{text: "carrying on"}}}
	a, store := newLoopAgent(t, config.AgentCoder, p)
	ctx := context.Background()
	_, _ = store.Create(ctx, "interrupted", message.CreateMessageParams{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "go"}}})
	_, _ = store.Create(ctx, "interrupted", message.CreateMessageParams{Role: message.Assistant, Parts: []message.ContentPart{call("lost", "bash")}})

	if ev := runTurn(t, a, "interrupted", "continue"); ev.Error != nil {
		t.Fatalf("the interrupted session could not be continued: %v", ev.Error)
	}
	answered := false
	for _, m := range p.history[0] {
		for _, r := range m.ToolResults() {
			answered = answered || (r.ToolCallID == "lost" && strings.Contains(r.Content, "never returned"))
		}
	}
	if !answered {
		t.Error("the model was sent a tool call with no result; real providers refuse that history")
	}
}
