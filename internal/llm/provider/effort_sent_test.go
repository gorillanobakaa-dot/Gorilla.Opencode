package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/message"
)

func effortMsgs() []message.Message {
	return []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}}}
}

// OpenAI-compatible providers: the chosen effort is in the request body the
// server receives (captured by an httptest server, not read back from config).
func TestOpenAISendsChosenEffort(t *testing.T) {
	for _, effort := range []string{"low", "medium", "high"} {
		var body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{}}`)
		}))
		c := newOpenAIClient(providerClientOptions{
			apiKey: "k", maxTokens: 100,
			model:         models.Model{ID: "o", APIModel: "o-test", CanReason: true, Provider: models.ProviderOpenAI},
			openaiOptions: []OpenAIOption{WithOpenAIBaseURL(srv.URL), WithReasoningEffort(effort)},
		})
		if _, err := c.send(context.Background(), effortMsgs(), nil); err != nil {
			srv.Close()
			t.Fatalf("%s: %v", effort, err)
		}
		srv.Close()
		var m map[string]any
		if err := json.Unmarshal([]byte(body), &m); err != nil {
			t.Fatalf("body %q: %v", body, err)
		}
		if m["reasoning_effort"] != effort {
			t.Fatalf("effort %s: body has reasoning_effort=%v\n%s", effort, m["reasoning_effort"], body)
		}
	}
}

// ChatGPT sign-in (Responses API): the effort is reasoning.effort.
func TestChatGPTSendsChosenEffort(t *testing.T) {
	for _, effort := range []string{"low", "medium", "high"} {
		c := &chatgptClient{providerOptions: providerClientOptions{
			model:         models.Model{APIModel: "gpt-test", CanReason: true},
			openaiOptions: []OpenAIOption{WithReasoningEffort(effort)},
		}}
		b, _ := json.Marshal(c.buildRequest(effortMsgs(), nil))
		if !strings.Contains(string(b), `"reasoning":{"effort":"`+effort+`"}`) {
			t.Fatalf("effort %s not in %s", effort, b)
		}
	}
	// nothing chosen -> medium, as before
	c := &chatgptClient{providerOptions: providerClientOptions{model: models.Model{APIModel: "g", CanReason: true}}}
	if b, _ := json.Marshal(c.buildRequest(effortMsgs(), nil)); !strings.Contains(string(b), `"effort":"medium"`) {
		t.Fatalf("default: %s", b)
	}
}

// Antigravity: the effort IS the model id (one backend id per effort), so the
// chosen id must be the envelope's "model".
func TestAntigravitySendsChosenEffortAsModelID(t *testing.T) {
	for _, id := range []string{"claude-opus-5-5-low", "claude-opus-5-5-high", "gemini-4-hydrogen-xhigh"} {
		c := &antigravityClient{
			providerOptions: providerClientOptions{model: models.Model{APIModel: id}, maxTokens: 100},
			creds:           &auth.AntigravityCreds{ProjectID: "p"},
		}
		b, _ := json.Marshal(c.buildEnvelope(effortMsgs(), nil))
		if !strings.Contains(string(b), `"model":"`+id+`"`) {
			t.Fatalf("%s not sent: %s", id, b)
		}
	}
}
