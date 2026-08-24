// GORILLA OVERRIDE: cover for a crash, not for a feature.
//
// The OpenAI client read Choices[0] without checking the list had anything in
// it. A backend that answers 200 with nothing to read therefore did not cost
// the user a turn, it killed the process:
//
//	panic: runtime error: index out of range [0] with length 0
//	  provider.(*openaiClient).stream.func1() openai.go:478
//
// Found 2026-08-24 while driving the Windows cross-build under Wine against a
// stub server that answered a STREAMING request with an ordinary JSON body.
// That mistake was the test harness's, but the same empty list arrives from a
// real backend three other ways: an explicit "choices": [], a stream that ends
// before any chunk, and a 200 whose body is actually an error page. All three
// are exercised below.
//
// These are end-to-end through a fake server rather than unit tests on the
// guard, because the bug was never in the arithmetic. It was in what the wire
// is allowed to contain.
package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opencode-ai/opencode/internal/message"
)

// oneUserTurn is the smallest conversation that will produce a request.
func oneUserTurn() []message.Message {
	return []message.Message{{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
	}}
}

// THE ORIGINAL REPRODUCER: a streaming request answered with a plain JSON
// body. No "data:" lines, so the stream ends cleanly with nothing accumulated.
func TestStreamSurvivesANonStreamingReply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"c1","object":"chat.completion","model":"test-model",`+
			`"choices":[{"index":0,"finish_reason":"stop",`+
			`"message":{"role":"assistant","content":"hi"}}]}`)
	}))
	defer srv.Close()

	_, content, evErr := drain(t, newTestOpenAIClient(srv.URL).
		stream(context.Background(), oneUserTurn(), nil))

	if !errors.Is(evErr, ErrEmptyCompletion) {
		t.Fatalf("want ErrEmptyCompletion, got err=%v content=%q", evErr, content)
	}
}

// An empty choices array inside an otherwise well-formed SSE stream.
func TestStreamSurvivesAnEmptyChoicesArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"id":"c1","object":"chat.completion.chunk",`+
			`"model":"test-model","choices":[]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	_, content, evErr := drain(t, newTestOpenAIClient(srv.URL).
		stream(context.Background(), oneUserTurn(), nil))

	if !errors.Is(evErr, ErrEmptyCompletion) {
		t.Fatalf("want ErrEmptyCompletion, got err=%v content=%q", evErr, content)
	}
}

// The non-streaming path had the same unguarded read.
func TestSendSurvivesAnEmptyChoicesArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"c1","object":"chat.completion",`+
			`"model":"test-model","choices":[]}`)
	}))
	defer srv.Close()

	resp, err := newTestOpenAIClient(srv.URL).
		send(context.Background(), oneUserTurn(), nil)

	if !errors.Is(err, ErrEmptyCompletion) {
		t.Fatalf("want ErrEmptyCompletion, got err=%v resp=%+v", err, resp)
	}
}
