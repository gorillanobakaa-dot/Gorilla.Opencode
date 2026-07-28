// GORILLA OVERRIDE: end-to-end cover for reasoning capture on the
// OpenAI-compatible path, driven by a fake server rather than a live endpoint.
//
// The unit tests in reasoning_test.go prove the extractor parses the field. They
// cannot prove the wiring: that the request actually ASKS for reasoning, that a
// streamed reasoning token comes out as an EventThinkingDelta, and that a server
// which refuses the parameter does not cost the user their turn. Those need a
// server, and a fake one is deterministic, offline and free.
//
// The payload shapes below are copied from a real NVIDIA NIM response captured on
// 2026-07-28 with z-ai/glm-5.2.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/message"
)

// sseChunk formats one streaming delta the way an OpenAI-compatible server does.
func sseChunk(delta string) string {
	return fmt.Sprintf(`data: {"id":"c1","object":"chat.completion.chunk","model":"test-model","choices":[{"index":0,"delta":%s,"finish_reason":null}]}`+"\n\n", delta)
}

func sseDone() string {
	return `data: {"id":"c1","object":"chat.completion.chunk","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" + "data: [DONE]\n\n"
}

// newTestOpenAIClient points an openaiClient at a fake server.
func newTestOpenAIClient(baseURL string) *openaiClient {
	return newOpenAIClient(providerClientOptions{
		apiKey:        "test-key",
		maxTokens:     256,
		systemMessage: "sys",
		model: models.Model{
			ID:       "local.test/test-model",
			APIModel: "test-model",
			Provider: models.ProviderLocal,
		},
		openaiOptions: []OpenAIOption{WithOpenAIBaseURL(baseURL)},
	}).(*openaiClient)
}

func drain(t *testing.T, ch <-chan ProviderEvent) (thinking, content string, evErr error) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			switch ev.Type {
			case EventThinkingDelta:
				thinking += ev.Content
			case EventContentDelta:
				content += ev.Content
			case EventError:
				evErr = ev.Error
			}
		case <-timeout:
			t.Fatal("timed out waiting for stream events")
		}
	}
}

// The request must ask for reasoning. Measured against the real endpoint: without
// chat_template_kwargs the server streams only [content, role], so the reader is
// dead code. reasoning_effort — which this provider already sent — does nothing.
func TestStreamRequestsReasoningFromTheServer(t *testing.T) {
	var body atomic.Value

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body.Store(string(raw))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk(`{"role":"assistant","content":"hi"}`))
		fmt.Fprint(w, sseDone())
	}))
	defer srv.Close()

	withReasoningEnabled(t)
	c := newTestOpenAIClient(srv.URL)
	thinkingRejected.Delete("test-model")
	drain(t, c.stream(context.Background(), []message.Message{{
		Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
	}}, nil))

	sent, _ := body.Load().(string)
	if sent == "" {
		t.Fatal("no request body was captured")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(sent), &parsed); err != nil {
		t.Fatalf("request body is not JSON: %v", err)
	}
	kwargs, ok := parsed["chat_template_kwargs"].(map[string]any)
	if !ok {
		t.Fatalf("the request does not ask for reasoning; without chat_template_kwargs the server sends none:\n%s", sent)
	}
	if kwargs["thinking"] != true {
		t.Errorf("chat_template_kwargs = %v, want thinking:true", kwargs)
	}
}

// A streamed reasoning token must surface as EventThinkingDelta, which is what
// agent.go persists as a ReasoningContent part.
func TestStreamedReasoningBecomesAThinkingEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Exactly NIM's shape: reasoning first, then the answer.
		fmt.Fprint(w, sseChunk(`{"role":"assistant","reasoning_content":"17 x 3 "}`))
		fmt.Fprint(w, sseChunk(`{"reasoning_content":"= 51."}`))
		fmt.Fprint(w, sseChunk(`{"content":"51"}`))
		fmt.Fprint(w, sseDone())
	}))
	defer srv.Close()

	withReasoningEnabled(t)
	c := newTestOpenAIClient(srv.URL)
	thinkingRejected.Delete("test-model")
	thinking, content, err := drain(t, c.stream(context.Background(), []message.Message{{
		Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "17*3"}},
	}}, nil))

	if err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if thinking != "17 x 3 = 51." {
		t.Errorf("reasoning = %q, want the concatenated thinking tokens", thinking)
	}
	if content != "51" {
		t.Errorf("content = %q, want just the answer", content)
	}
}

// The reasoning must NOT be folded into the answer. It is sent back as the
// assistant's previous turn, and feeding a model its own private thoughts as
// prior output corrupts the next round.
func TestReasoningIsKeptOutOfTheAnswerContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk(`{"reasoning_content":"SECRET-WORKING"}`))
		fmt.Fprint(w, sseChunk(`{"content":"the answer"}`))
		fmt.Fprint(w, sseDone())
	}))
	defer srv.Close()

	withReasoningEnabled(t)
	c := newTestOpenAIClient(srv.URL)
	thinkingRejected.Delete("test-model")
	_, content, _ := drain(t, c.stream(context.Background(), []message.Message{{
		Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "q"}},
	}}, nil))

	if strings.Contains(content, "SECRET-WORKING") {
		t.Errorf("the reasoning leaked into the answer content: %q", content)
	}
}

// A server that refuses the parameter must not cost the user their turn. NIM is
// documented as rejecting an unknown parameter (prompt_cache_key) with HTTP 400,
// so this path is real, not hypothetical: drop the parameter, retry, succeed.
func TestParameterRejectionFallsBackAndStillAnswers(t *testing.T) {
	var calls atomic.Int32
	var sawParamOnSecond atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"Unsupported parameter: chat_template_kwargs"}}`)
			return
		}
		sawParamOnSecond.Store(strings.Contains(string(raw), "chat_template_kwargs"))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk(`{"content":"answered anyway"}`))
		fmt.Fprint(w, sseDone())
	}))
	defer srv.Close()

	withReasoningEnabled(t)
	c := newTestOpenAIClient(srv.URL)
	thinkingRejected.Delete("test-model")
	_, content, err := drain(t, c.stream(context.Background(), []message.Message{{
		Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "q"}},
	}}, nil))

	if err != nil {
		t.Fatalf("a refused reasoning parameter broke the turn instead of degrading: %v", err)
	}
	if content != "answered anyway" {
		t.Errorf("content = %q, want the answer from the retry", content)
	}
	if calls.Load() < 2 {
		t.Errorf("only %d request(s) made; expected a retry without the parameter", calls.Load())
	}
	if sawParamOnSecond.Load() {
		t.Error("the retry still carried chat_template_kwargs, so it would fail again")
	}
	// And it must remember, so every later turn skips the doomed attempt.
	if thinkingWasRequested("test-model") {
		t.Error("the rejection was not remembered; every turn would pay for a failed request")
	}
}

// A genuine error must NOT be mistaken for a parameter rejection — otherwise one
// rate-limit blip would silently disable reasoning for the rest of the session.
func TestUnrelatedErrorsDoNotDisableReasoning(t *testing.T) {
	thinkingRejected.Delete("test-model")

	for _, err := range []error{
		fmt.Errorf("429 Too Many Requests: rate limit exceeded"),
		fmt.Errorf("401 Unauthorized: invalid api key"),
		fmt.Errorf("500 Internal Server Error"),
		fmt.Errorf("400 Bad Request: messages must not be empty"),
	} {
		if isParameterRejection(err) {
			t.Errorf("%v was treated as a parameter rejection — reasoning would be switched off by an unrelated failure", err)
		}
	}

	// And the ones that genuinely are.
	for _, err := range []error{
		fmt.Errorf("Unsupported parameter: chat_template_kwargs"),
		fmt.Errorf("400 Bad Request: unknown field \"foo\""),
		fmt.Errorf("422 Unprocessable Entity: additional properties are not allowed"),
	} {
		if !isParameterRejection(err) {
			t.Errorf("%v was not recognised as a parameter rejection", err)
		}
	}
}

// The default must spend nothing. Reasoning is the one setting here that makes the
// model produce more output, so it has to be chosen rather than inherited: a user
// who never saw the explanation must not be paying for it.
func TestReasoningIsNotRequestedUnlessTheUserOptedIn(t *testing.T) {
	var body atomic.Value

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body.Store(string(raw))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk(`{"content":"hi"}`))
		fmt.Fprint(w, sseDone())
	}))
	defer srv.Close()

	// Explicitly OFF — no withReasoningEnabled here.
	before := config.ExtraEnabled("extras-reasoning-generate")
	if err := config.SetExtra("extras-reasoning-generate", false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.SetExtra("extras-reasoning-generate", before) })

	c := newTestOpenAIClient(srv.URL)
	thinkingRejected.Delete("test-model")
	thinking, content, err := drain(t, c.stream(context.Background(), []message.Message{{
		Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
	}}, nil))
	if err != nil {
		t.Fatalf("stream error: %v", err)
	}

	sent, _ := body.Load().(string)
	if strings.Contains(sent, "chat_template_kwargs") {
		t.Errorf("reasoning was requested with the setting off — the user would be billed for output they never asked for:\n%s", sent)
	}
	if thinking != "" {
		t.Errorf("thinking events emitted while disabled: %q", thinking)
	}
	if content != "hi" {
		t.Errorf("the ordinary answer broke when reasoning was off: %q", content)
	}
}

// And the registry default itself must be off, so a fresh install spends nothing
// before anyone has been asked.
func TestReasoningDefaultsToOffInTheRegistry(t *testing.T) {
	e, ok := config.ExtraByID("extras-reasoning-generate")
	if !ok {
		t.Fatal("the reasoning extra is not registered")
	}
	if e.Default {
		t.Error("reasoning generation defaults to ON — a fresh install would spend extra tokens before the user was ever asked")
	}
	if e.Cost != config.CostGeneration {
		t.Error("reasoning is not marked as costing anything, so no warning would be shown next to it")
	}

	// The free ones must default ON and be marked free: hiding them saves nothing,
	// so defaulting them off would lose information for no benefit.
	for _, id := range []string{"extras-reasoning-show", "extras-toolcalls-show", "extras-timestamps-show"} {
		e, ok := config.ExtraByID(id)
		if !ok {
			t.Errorf("%s is not registered", id)
			continue
		}
		if e.Cost != config.CostFree {
			t.Errorf("%s is marked as costing something; displaying already-generated data is free", id)
		}
		if !e.Default {
			t.Errorf("%s defaults to off, hiding information at no saving", id)
		}
	}
}
