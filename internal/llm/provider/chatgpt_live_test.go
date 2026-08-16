// GORILLA OVERRIDE: a LIVE probe of the ChatGPT/Codex transport. Skipped unless
// CHATGPT_LIVE=1 and a signed-in credential exists, so it never runs in a normal
// `go test ./...` and never costs anyone their plan's quota by accident.
//
// It exists because the wire shape of the Responses API through this backend is
// not documented anywhere this project can cite. Every field in chatgpt.go was
// inferred; this is the only thing that turns an inference into a fact. Run it
// after ANY change to the request shape:
//
//	CHATGPT_LIVE=1 go test ./internal/llm/provider/ -run TestChatGPTLive -v -count=1
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
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

func liveChatGPTClient(t *testing.T) ChatGPTClient {
	t.Helper()
	if os.Getenv("CHATGPT_LIVE") == "" {
		t.Skip("set CHATGPT_LIVE=1 to run the live ChatGPT backend probe")
	}
	// TestMain redirects XDG_CONFIG_HOME to a temp dir so nothing here can write
	// the developer's real config. That also hides the real OAuth token, which
	// this test genuinely needs, so point at the real config home for the
	// duration of the READ and put it back immediately. See realConfigHome.
	restore := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", realConfigHome)
	creds, err := auth.LoadChatGPTCreds()
	os.Setenv("XDG_CONFIG_HOME", restore)

	if err != nil || creds == nil || creds.AccessToken == "" {
		t.Skip("not signed in to ChatGPT (run: gorilla-opencode login --chatgpt)")
	}
	// newChatGPTClient would re-load the creds from the isolated path and find
	// nothing, so hand it the ones just read.
	c := newChatGPTClient(providerClientOptions{
		model:         models.SupportedModels[models.ChatGPT55],
		maxTokens:     512,
		systemMessage: "You are a terse assistant. Answer in as few words as possible.",
	})
	c.creds = creds
	return c
}

func userMsg(text string) message.Message {
	return message.Message{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	}
}

// TestChatGPTLiveText proves the plain generation path: a request the backend
// accepts, deltas that arrive, and a completion carrying usage. A 200 on
// /models proved only that the token works; this proves the transport does.
func TestChatGPTLiveText(t *testing.T) {
	c := liveChatGPTClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var deltas int
	var final *ProviderResponse
	for ev := range c.stream(ctx, []message.Message{userMsg("Reply with exactly: PONG")}, nil) {
		switch ev.Type {
		case EventContentDelta:
			deltas++
		case EventError:
			t.Fatalf("stream error: %v", ev.Error)
		case EventComplete:
			final = ev.Response
		}
	}
	if final == nil {
		t.Fatal("stream ended with no completion event")
	}
	t.Logf("content=%q deltas=%d usage=%+v finish=%s",
		final.Content, deltas, final.Usage, final.FinishReason)

	if !strings.Contains(strings.ToUpper(final.Content), "PONG") {
		t.Errorf("model did not answer as asked; got %q", final.Content)
	}
	// Deltas are the whole point of a streaming transport. Content arriving only
	// on the completion event would mean the SSE parse is matching nothing and
	// the answer is being recovered by accident.
	if deltas == 0 {
		t.Error("no content deltas: the response.output_text.delta event is not being parsed")
	}
	if final.Usage.InputTokens == 0 && final.Usage.OutputTokens == 0 {
		t.Error("no usage reported: the response.completed event shape has drifted")
	}
}

// echoTool is a minimal real tool. Its schema is the shape convertToolsChatGPT
// produces, so this exercises the tool wire format rather than a mock of it.
type echoTool struct{}

func (echoTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name:        "get_weather",
		Description: "Get the current weather for a city. Call this whenever a city's weather is requested.",
		Parameters: map[string]any{
			"city": map[string]any{
				"type":        "string",
				"description": "The city name.",
			},
		},
		Required: []string{"city"},
	}
}

func (echoTool) Run(context.Context, tools.ToolCall) (tools.ToolResponse, error) {
	return tools.ToolResponse{}, nil
}

// TestChatGPTLiveToolCall is the one that matters most. Tool calling is where
// the Responses API differs hardest from Chat Completions — tools are flat, not
// nested under "function", and the call comes back as an output ITEM rather
// than a delta on a message. Both were guesses until this ran.
//
// It then feeds the result back as a function_call_output and asks for a second
// turn, because emitting a tool call is only half the protocol: replaying one
// into history is the half that breaks silently, with a 400 that names nothing.
func TestChatGPTLiveToolCall(t *testing.T) {
	c := liveChatGPTClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	ts := []tools.BaseTool{echoTool{}}
	history := []message.Message{userMsg("What is the weather in Lisbon? Use the tool.")}

	first, err := c.send(ctx, history, ts)
	if err != nil {
		t.Fatalf("first turn: %v", err)
	}
	t.Logf("turn 1: finish=%s calls=%d content=%q", first.FinishReason, len(first.ToolCalls), first.Content)

	if len(first.ToolCalls) == 0 {
		t.Fatal("no tool call: either the tool schema was rejected or output_item.done is not being parsed")
	}
	call := first.ToolCalls[0]
	if call.Name != "get_weather" {
		t.Errorf("wrong tool name %q", call.Name)
	}
	if call.ID == "" {
		t.Error("tool call has no id — the function_call_output replay cannot be paired without call_id")
	}
	// Arguments must be JSON the agent loop can unmarshal. A raw string here
	// would be handed straight to a tool and fail at the tool, far from here.
	var args map[string]any
	if err := json.Unmarshal([]byte(call.Input), &args); err != nil {
		t.Errorf("tool arguments are not valid JSON (%v): %q", err, call.Input)
	} else if _, ok := args["city"]; !ok {
		t.Errorf("tool arguments missing the required property: %v", args)
	}
	if first.FinishReason != message.FinishReasonToolUse {
		t.Errorf("finish reason %q, want %q", first.FinishReason, message.FinishReasonToolUse)
	}

	// Second turn: replay the call and its result.
	history = append(history,
		message.Message{
			Role:  message.Assistant,
			Parts: []message.ContentPart{call},
		},
		message.Message{
			Role: message.Tool,
			Parts: []message.ContentPart{message.ToolResult{
				ToolCallID: call.ID,
				Name:       call.Name,
				Content:    `{"temp_c": 19, "conditions": "clear"}`,
			}},
		},
	)

	second, err := c.send(ctx, history, ts)
	if err != nil {
		t.Fatalf("second turn (replaying the tool call into history): %v", err)
	}
	t.Logf("turn 2: finish=%s content=%q", second.FinishReason, second.Content)
	if strings.TrimSpace(second.Content) == "" {
		t.Error("second turn produced no text: the tool result was accepted but not used")
	}
	if !strings.Contains(second.Content, "19") {
		t.Errorf("answer does not reflect the tool result, so the result may not have reached the model: %q", second.Content)
	}
}
