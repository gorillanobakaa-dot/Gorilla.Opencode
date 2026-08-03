package provider

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/message"
)

// The envelope must carry the extra top-level fields the Antigravity backend
// requires (measured from a live agy capture): a plain model name, the project,
// a requestId, userAgent:"antigravity", and requestType:"agent". A missing one
// is exactly what makes daily-cloudcode-pa reject the call, so assert on the
// serialized JSON, not the struct.
func TestAntigravityEnvelopeShape(t *testing.T) {
	c := &antigravityClient{
		providerOptions: providerClientOptions{
			model:         models.Model{APIModel: "claude-sonnet-4-6"},
			maxTokens:     100,
			systemMessage: "You are a test.",
		},
		creds: &auth.AntigravityCreds{ProjectID: "proj-123"},
	}
	env := c.buildEnvelope([]message.Message{{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
	}}, nil)

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		`"model":"claude-sonnet-4-6"`,
		`"project":"proj-123"`,
		`"userAgent":"antigravity"`,
		`"requestType":"agent"`,
		`"requestId":"agent/`,
		`"systemInstruction"`,
		`"contents"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("envelope JSON missing %s\nfull: %s", want, got)
		}
	}
	// The Gemini path must NOT gain these — verify they are absent when unset.
	empty := caEnvelope{Model: "m", Request: caInnerRequest{}}
	eb, _ := json.Marshal(empty)
	for _, absent := range []string{"requestId", "userAgent", "requestType"} {
		if strings.Contains(string(eb), absent) {
			t.Errorf("bare envelope unexpectedly serialized %q (breaks the Gemini path): %s", absent, eb)
		}
	}
}

// Tool calls must carry a correlation id for the Antigravity path (its native
// Anthropic/OpenAI translation 400s without one — "tool_use.id: Field required")
// and must NOT carry one for Gemini Code Assist (which matches by name). This is
// the pure, offline guard for the live toolCallHistory reproduction.
func TestToolCallIDsPresentOnlyForAntigravity(t *testing.T) {
	am := message.Message{Role: message.Assistant}
	am.SetToolCalls([]message.ToolCall{{ID: "call_xyz", Name: "list_files", Input: `{"path":"."}`, Type: "function", Finished: true}})
	tm := message.Message{Role: message.Tool}
	tm.AddToolResult(message.ToolResult{ToolCallID: "call_xyz", Name: "list_files", Content: "x.txt"})
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "go"}}},
		am, tm,
	}

	findIDs := func(contents []caContent) (callID, respID string) {
		for _, c := range contents {
			for _, p := range c.Parts {
				if p.FunctionCall != nil {
					callID = p.FunctionCall.ID
				}
				if p.FunctionResponse != nil {
					respID = p.FunctionResponse.ID
				}
			}
		}
		return
	}

	callID, respID := findIDs(caConvertMessages(msgs, true))
	if callID != "call_xyz" || respID != "call_xyz" {
		t.Fatalf("Antigravity path must carry matching ids; got call=%q resp=%q", callID, respID)
	}

	callID, respID = findIDs(caConvertMessages(msgs, false))
	if callID != "" || respID != "" {
		t.Fatalf("Gemini path must NOT carry ids; got call=%q resp=%q", callID, respID)
	}
}

// TestAntigravityLive drives the REAL transport (buildEnvelope → post → SSE
// parse) against daily-cloudcode-pa, proving the whole gorilla path — not just
// curl — produces a Claude response. Guarded: it needs a live token and never
// runs in normal CI.
//
//	AG_LIVE=1 AG_TOKEN=ya29... AG_PROJECT=probable-tine-zs7sz \
//	  go test ./internal/llm/provider/ -run TestAntigravityLive -v
func TestAntigravityLive(t *testing.T) {
	if os.Getenv("AG_LIVE") != "1" {
		t.Skip("set AG_LIVE=1 (with AG_TOKEN, AG_PROJECT) to run the live check")
	}
	tok := os.Getenv("AG_TOKEN")
	proj := os.Getenv("AG_PROJECT")
	if tok == "" || proj == "" {
		t.Fatal("AG_TOKEN and AG_PROJECT are required")
	}
	c := &antigravityClient{
		providerOptions: providerClientOptions{
			model:         models.AntigravityModels[models.AGClaudeSonnet46],
			maxTokens:     100,
			systemMessage: "You are a terse assistant.",
		},
		creds: &auth.AntigravityCreds{
			AccessToken: tok,
			ProjectID:   proj,
			Expiry:      time.Now().Add(30 * time.Minute), // skip refresh
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := c.send(ctx, []message.Message{{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Reply with only the word PONG"}},
	}}, nil)
	if err != nil {
		t.Fatalf("live send failed: %v", err)
	}
	if strings.TrimSpace(resp.Content) == "" {
		t.Fatalf("empty response; finish=%v usage=%+v", resp.FinishReason, resp.Usage)
	}
	t.Logf("Antigravity Claude replied: %q (in=%d out=%d)", resp.Content, resp.Usage.InputTokens, resp.Usage.OutputTokens)
}
