package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
)

// withReasoningDisabled is withReasoningEnabled's mirror: the switch OFF for
// one test, restored after.
func withReasoningDisabled(t *testing.T) {
	t.Helper()
	before := config.ExtraEnabled("extras-reasoning-generate")
	if err := config.SetExtra("extras-reasoning-generate", false); err != nil {
		t.Fatalf("disabling reasoning: %v", err)
	}
	t.Cleanup(func() {
		if err := config.SetExtra("extras-reasoning-generate", before); err != nil {
			t.Errorf("restoring reasoning setting: %v", err)
		}
	})
}

func askOnce(t *testing.T, c *openaiClient) (string, error) {
	t.Helper()
	_, content, err := drain(t, c.stream(context.Background(), []message.Message{{
		Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "q"}},
	}}, nil))
	return content, err
}

// Measured 2026-10-24 on LM Studio 0.4.26 (gemma-4-12b-qat): with no field the
// server thinks anyway, 117 of 142 tokens. OFF must be sent, not implied.
func TestReasoningOffIsSentToALocalEndpoint(t *testing.T) {
	var body atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body.Store(string(raw))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk(`{"content":"hi"}`))
		fmt.Fprint(w, sseDone())
	}))
	defer srv.Close()

	withReasoningDisabled(t)
	thinkingRejected.Delete("test-model")
	if _, err := askOnce(t, newTestOpenAIClient(srv.URL)); err != nil {
		t.Fatal(err)
	}
	got, _ := body.Load().(string)
	if !strings.Contains(got, `"reasoning_effort":"none"`) {
		t.Errorf("reasoning switched off but the request did not say so: %s", got)
	}
	if strings.Contains(got, "chat_template_kwargs") {
		t.Errorf("reasoning switched off but the request asked for thinking: %s", got)
	}
}

// ON keeps asking for thinking and never sends "none".
func TestReasoningOnDoesNotSendNone(t *testing.T) {
	var body atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body.Store(string(raw))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk(`{"content":"hi"}`))
		fmt.Fprint(w, sseDone())
	}))
	defer srv.Close()

	withReasoningEnabled(t)
	thinkingRejected.Delete("test-model")
	if _, err := askOnce(t, newTestOpenAIClient(srv.URL)); err != nil {
		t.Fatal(err)
	}
	got, _ := body.Load().(string)
	if strings.Contains(got, `"reasoning_effort":"none"`) {
		t.Errorf("reasoning switched on but the request said none: %s", got)
	}
}

// A server that does not know the field must still answer, once, and never be
// sent it again.
func TestReasoningOffRejectedFallsBackAndIsRemembered(t *testing.T) {
	var calls atomic.Int32
	var secondCarried atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"Unsupported parameter: reasoning_effort"}}`)
			return
		}
		secondCarried.Store(strings.Contains(string(raw), "reasoning_effort"))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk(`{"content":"answered anyway"}`))
		fmt.Fprint(w, sseDone())
	}))
	defer srv.Close()

	withReasoningDisabled(t)
	thinkingRejected.Delete("test-model")
	t.Cleanup(func() { thinkingRejected.Delete("test-model") })
	content, err := askOnce(t, newTestOpenAIClient(srv.URL))
	if err != nil {
		t.Fatalf("a refused reasoning_effort broke the turn: %v", err)
	}
	if content != "answered anyway" {
		t.Errorf("content = %q, want the retry's answer", content)
	}
	if secondCarried.Load() {
		t.Error("the retry still carried reasoning_effort")
	}
	if thinkingWasRequested("test-model", true) {
		t.Error("the rejection was not remembered")
	}
}

// A cloud model with the switch OFF is left as before: nothing added.
func TestReasoningOffAddsNothingForACloudModel(t *testing.T) {
	withReasoningDisabled(t)
	thinkingRejected.Delete("cloud-model")
	if opts := thinkingRequestOptions("cloud-model", false); len(opts) != 0 {
		t.Errorf("cloud model got %d extra field(s) with reasoning off", len(opts))
	}
}

// The server's reasoning count reaches the usage, so the speed line can say
// how much of the output was thinking.
func TestReasoningTokensAreReadFromTheStreamUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// In LM Studio's order: content, finish, usage, [DONE].
		fmt.Fprint(w, sseChunk(`{"content":"hi"}`))
		fmt.Fprint(w, `data: {"id":"c1","object":"chat.completion.chunk","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		fmt.Fprint(w, `data: {"id":"c1","object":"chat.completion.chunk","model":"test-model","choices":[],"usage":{"prompt_tokens":22,"completion_tokens":142,"total_tokens":164,"completion_tokens_details":{"reasoning_tokens":117}}}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	withReasoningEnabled(t)
	thinkingRejected.Delete("test-model")
	c := newTestOpenAIClient(srv.URL)
	var usage TokenUsage
	for ev := range c.stream(context.Background(), []message.Message{{
		Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "q"}},
	}}, nil) {
		if ev.Type == EventComplete {
			usage = ev.Response.Usage
		}
	}
	if usage.ReasoningTokens != 117 || usage.OutputTokens != 142 {
		t.Errorf("usage = %+v, want 142 out of which 117 reasoning", usage)
	}
}
