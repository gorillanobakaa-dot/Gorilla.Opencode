package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

// recordingMessages is the smallest message.Service that lets us see what the
// agent writes. Only Create is exercised; the rest satisfy the interface.
type recordingMessages struct {
	*pubsub.Broker[message.Message]
	created  []message.Message
	failNext bool
}

func newRecordingMessages() *recordingMessages {
	return &recordingMessages{Broker: pubsub.NewBroker[message.Message]()}
}

func (r *recordingMessages) Create(_ context.Context, sessionID string,
	params message.CreateMessageParams) (message.Message, error) {
	if r.failNext {
		return message.Message{}, context.DeadlineExceeded
	}
	m := message.Message{ID: "m", SessionID: sessionID, Role: params.Role, Parts: params.Parts}
	r.created = append(r.created, m)
	return m, nil
}

func (r *recordingMessages) Update(context.Context, message.Message) error { return nil }
func (r *recordingMessages) Get(context.Context, string) (message.Message, error) {
	return message.Message{}, nil
}
func (r *recordingMessages) List(context.Context, string) ([]message.Message, error) {
	return nil, nil
}
func (r *recordingMessages) Delete(context.Context, string) error                { return nil }
func (r *recordingMessages) DeleteSessionMessages(context.Context, string) error { return nil }

// assistantWithToolCalls builds the state the streaming loop leaves behind when
// the model announced a tool call and the user then pressed Esc.
func assistantWithToolCalls(ids ...string) *message.Message {
	parts := make([]message.ContentPart, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, message.ToolCall{ID: id, Name: "grep", Input: "{}"})
	}
	return &message.Message{ID: "a", SessionID: "s", Role: message.Assistant, Parts: parts}
}

// Cancelling mid-stream used to leave the UI on "Waiting for response..."
// forever: both early returns in the streaming loop skipped the tool loop, which
// was the only code that recorded cancelled tool results. Every announced tool
// call must come back with a result, or message.go renders it as still pending.
func TestCancellingMidStreamRecordsAResultForEveryPendingToolCall(t *testing.T) {
	svc := newRecordingMessages()
	a := &agent{messages: svc}

	assistant := assistantWithToolCalls("call-1", "call-2")
	got := a.cancelPendingToolCalls(assistant)

	if got == nil {
		t.Fatal("no tool message was created, so the UI stays on 'Waiting for response...'")
	}
	if got.Role != message.Tool {
		t.Fatalf("cancellation must be recorded as a Tool message, got %v", got.Role)
	}

	results := got.ToolResults()
	if len(results) != 2 {
		t.Fatalf("want a result for each of the 2 pending calls, got %d", len(results))
	}
	seen := map[string]bool{}
	for _, r := range results {
		seen[r.ToolCallID] = true
		if !r.IsError {
			t.Errorf("%s: a cancelled call is not a success", r.ToolCallID)
		}
		if !strings.Contains(strings.ToLower(r.Content), "cancel") {
			t.Errorf("%s: result does not say it was cancelled: %q", r.ToolCallID, r.Content)
		}
	}
	for _, id := range []string{"call-1", "call-2"} {
		if !seen[id] {
			t.Errorf("no result for %s — that call renders as pending forever", id)
		}
	}
}

// A cancel with nothing in flight must not invent an empty tool message.
func TestCancellingWithNoToolCallsCreatesNothing(t *testing.T) {
	svc := newRecordingMessages()
	a := &agent{messages: svc}

	plain := &message.Message{ID: "a", SessionID: "s", Role: message.Assistant,
		Parts: []message.ContentPart{message.TextContent{Text: "hello"}}}

	if got := a.cancelPendingToolCalls(plain); got != nil {
		t.Fatalf("created a tool message for a reply with no tool calls: %+v", got)
	}
	if len(svc.created) != 0 {
		t.Fatalf("wrote %d message(s) when there was nothing to cancel", len(svc.created))
	}
}

// Recording the cancellation can fail; the cancellation itself must still stand.
// Returning an error here would propagate over the user's actual request, which
// was "stop".
func TestAFailedWriteDoesNotPanicOrBlockTheCancellation(t *testing.T) {
	svc := newRecordingMessages()
	svc.failNext = true
	a := &agent{messages: svc}

	if got := a.cancelPendingToolCalls(assistantWithToolCalls("call-1")); got != nil {
		t.Fatalf("expected nil when the write failed, got %+v", got)
	}
}

// The three tests above exercise cancelPendingToolCalls directly, and all three
// PASS with the bug restored — because the bug was never in the helper, it was
// the two streaming-loop returns not calling it. A test that cannot fail against
// the defect it documents is worse than no test, so this one asserts the call
// sites themselves.
//
// A source assertion rather than a driven loop: streamAndHandleEvents needs a
// provider, a session store and a live event channel, and a fake of all three
// would be testing the fake. What broke was two `return` statements, and that is
// what this checks.
func TestBothStreamingLoopReturnsRecordCancelledToolCalls(t *testing.T) {
	src, err := os.ReadFile("agent.go")
	if err != nil {
		t.Fatalf("cannot read agent.go: %v", err)
	}
	text := string(src)

	// Locate the streaming loop, so we only inspect its returns.
	start := strings.Index(text, "for event := range eventChan {")
	if start < 0 {
		t.Fatal("streaming loop not found — this test needs updating")
	}
	end := strings.Index(text[start:], "\n\ttoolResults :=")
	if end < 0 {
		t.Fatal("end of streaming loop not found — this test needs updating")
	}
	loop := text[start : start+end]

	returns := 0
	for _, line := range strings.Split(loop, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "return assistantMsg,") {
			continue
		}
		returns++
		if strings.Contains(line, "assistantMsg, nil,") {
			t.Errorf("this return abandons pending tool calls, so the UI stays on "+
				"\"Waiting for response...\" forever:\n  %s", line)
		}
		if !strings.Contains(line, "cancelPendingToolCalls") {
			t.Errorf("return does not record cancelled tool calls:\n  %s", line)
		}
	}
	if returns < 2 {
		t.Fatalf("expected the loop's 2 cancellation returns, found %d", returns)
	}
}
