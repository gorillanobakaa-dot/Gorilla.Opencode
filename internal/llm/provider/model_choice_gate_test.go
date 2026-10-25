package provider

// GORILLA OVERRIDE (2026-10-25): while an agent's configured model is unusable
// and nothing has been chosen in its place, no request leaves, through either
// way a provider sends. See config/model_choice.go.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
)

func TestNothingIsSentWhileAModelChoiceIsPending(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk(`{"role":"assistant","content":"hi"}`))
		fmt.Fprint(w, sseDone())
	}))
	defer srv.Close()

	p := &baseProvider[OpenAIClient]{client: newTestOpenAIClient(srv.URL)}
	msgs := []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}}}

	release := config.HoldModelChoiceForTest(config.AgentCoder, "local.deepseek/deepseek-r1-0528-qwen3-8b", "not in lmstudio any more")
	if _, err := p.SendMessages(context.Background(), msgs, nil); !errors.Is(err, config.ErrModelChoicePending) {
		t.Errorf("SendMessages while held: err = %v, want ErrModelChoicePending", err)
	}
	_, _, evErr := drain(t, p.StreamResponse(context.Background(), msgs, nil))
	if !errors.Is(evErr, config.ErrModelChoicePending) {
		t.Errorf("StreamResponse while held: err = %v, want ErrModelChoicePending", evErr)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("%d request(s) reached the server while a model choice was pending", n)
	}

	// The gate is the hold and nothing else: once chosen, requests go.
	release()
	_, _, evErr = drain(t, p.StreamResponse(context.Background(), msgs, nil))
	if evErr != nil {
		t.Fatalf("after the choice: %v", evErr)
	}
	if hits.Load() == 0 {
		t.Fatal("no request reached the server after the choice; the test would prove nothing")
	}
}
