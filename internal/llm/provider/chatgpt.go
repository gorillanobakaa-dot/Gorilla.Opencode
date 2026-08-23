// GORILLA OVERRIDE: this file did not exist upstream. It is the transport for
// OpenAI models reached through a personal ChatGPT sign-in
// (chatgpt.com/backend-api/codex) rather than an API key — see
// internal/auth/chatgpt_oauth.go for the login and
// internal/llm/models/chatgpt.go for which models are registered and why.
//
// # WHY THIS IS NOT A THIN WRAPPER OVER openai.go
//
// The obvious guess is that a ChatGPT token is an API key with a different base
// URL, the way ProviderGROQ / ProviderXAI / ProviderDeepSeek all reuse
// OpenAIClient with WithOpenAIBaseURL. It is not. Two things differ:
//
//  1. The token does not authenticate against api.openai.com at all. It is only
//     spendable at this backend.
//  2. The backend speaks the RESPONSES API, not Chat Completions. Different
//     request field names (input/instructions, not messages/system), a different
//     history item vocabulary (function_call, function_call_output), and a
//     completely different SSE event stream (named response.* events carrying
//     deltas, not choices[].delta chunks).
//
// So there is nothing to share with openai.go, and unlike antigravity.go — which
// is 234 lines only because code_assist.go already had its envelope — this one
// carries its own conversion.
//
// The backend is streaming-only in practice, so send() drains stream() rather
// than duplicating the parse for a non-streaming path that would then be the
// less-tested of the two.
//
// Wire shape verified live on 2026-08-17 against a free ChatGPT plan.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

type chatgptClient struct {
	providerOptions providerClientOptions
	creds           *auth.ChatGPTCreds
}

// ChatGPTClient satisfies the ProviderClient type parameter.
type ChatGPTClient = *chatgptClient

func newChatGPTClient(opts providerClientOptions) ChatGPTClient {
	creds, _ := auth.LoadChatGPTCreds()
	return &chatgptClient{providerOptions: opts, creds: creds}
}

// ---------------------------------------------------------------------------
// Request shape
// ---------------------------------------------------------------------------

// cgRequest is the Responses API request body. Field names are the API's, not
// Chat Completions': the system prompt is "instructions" (a string, not a
// message), and the conversation is "input" (a list of typed items, not a list
// of role/content messages).
type cgRequest struct {
	Model             string      `json:"model"`
	Instructions      string      `json:"instructions,omitempty"`
	Input             []cgItem    `json:"input"`
	Tools             []cgTool    `json:"tools,omitempty"`
	ToolChoice        string      `json:"tool_choice,omitempty"`
	ParallelToolCalls bool        `json:"parallel_tool_calls"`
	Stream            bool        `json:"stream"`
	Store             bool        `json:"store"`
	Reasoning         *cgReason   `json:"reasoning,omitempty"`
	Include           []string    `json:"include,omitempty"`
	Metadata          interface{} `json:"metadata,omitempty"`

	// NO max_output_tokens. The public Responses API takes it; this backend
	// rejects the whole request with
	//   400 Unsupported parameter: max_output_tokens
	// (measured 2026-08-17). The output length is governed by the plan, not by
	// the caller. providerOptions.maxTokens is therefore deliberately unused
	// here — dropping it is the only thing that works, not an oversight.
}

type cgReason struct {
	Effort string `json:"effort,omitempty"`
}

// cgItem is one entry of the conversation. The Responses API models history as
// heterogeneous typed items rather than uniform messages, so a tool call and its
// result are SIBLINGS of the messages around them, not fields hanging off an
// assistant message the way Chat Completions does it.
type cgItem struct {
	Type string `json:"type"`

	// type == "message"
	Role    string      `json:"role,omitempty"`
	Content []cgContent `json:"content,omitempty"`

	// type == "function_call"
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	CallID    string `json:"call_id,omitempty"`

	// type == "function_call_output"
	Output string `json:"output,omitempty"`
}

// cgContent is a content part. The text type differs by direction:
// "input_text" on the way in, "output_text" for assistant turns being replayed.
// Sending the wrong one is rejected with a message that names neither.
type cgContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type cgTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
	Strict      bool           `json:"strict"`
}

// convertToolsChatGPT flattens each tool schema. Note the Responses API puts
// name/description/parameters at the TOP level of the tool object; Chat
// Completions nests them under a "function" key. Nesting them here produces a
// 400 that says the tool is missing a name.
func convertToolsChatGPT(ts []tools.BaseTool) []cgTool {
	out := make([]cgTool, 0, len(ts))
	for _, t := range ts {
		info := t.Info()
		params := map[string]any{
			"type":       "object",
			"properties": info.Parameters,
		}
		// An absent "required" and an empty one are different to a strict
		// validator; send a list either way.
		req := info.Required
		if req == nil {
			req = []string{}
		}
		params["required"] = req
		out = append(out, cgTool{
			Type:        "function",
			Name:        info.Name,
			Description: info.Description,
			Parameters:  params,
			// strict:false — this program's tool schemas are hand-written and do
			// not all satisfy strict mode's requirements (every property
			// required, additionalProperties:false). Claiming strict and then
			// failing validation would reject the whole request, not one tool.
			Strict: false,
		})
	}
	return out
}

// cgConvertMessages turns the internal history into Responses API input items.
//
// The ordering rule that matters: a function_call item must appear before its
// matching function_call_output, and both are top-level items. Because our
// Message model stores tool calls on the assistant message and tool results on a
// following tool message, this walks them in order and emits the flattened
// sequence rather than trying to pair them up.
func cgConvertMessages(messages []message.Message) []cgItem {
	items := make([]cgItem, 0, len(messages)+4)
	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			content := []cgContent{}
			if txt := msg.Content().String(); txt != "" {
				content = append(content, cgContent{Type: "input_text", Text: txt})
			}
			for _, img := range msg.ImageURLContent() {
				content = append(content, cgContent{Type: "input_image", ImageURL: img.URL})
			}
			for _, bin := range msg.BinaryContent() {
				// ProviderOpenAI, not ProviderChatGPT: that argument selects the
				// ENCODING, and only the OpenAI branch emits the
				// "data:<mime>;base64,..." URL that image_url requires. Passing our
				// own provider id would send bare base64 and the image would be
				// rejected as a malformed URL.
				content = append(content, cgContent{
					Type:     "input_image",
					ImageURL: bin.String(models.ProviderOpenAI),
				})
			}
			if len(content) == 0 {
				continue
			}
			items = append(items, cgItem{Type: "message", Role: "user", Content: content})

		case message.Assistant:
			if txt := msg.Content().String(); txt != "" {
				items = append(items, cgItem{
					Type: "message", Role: "assistant",
					Content: []cgContent{{Type: "output_text", Text: txt}},
				})
			}
			for _, tc := range msg.ToolCalls() {
				args := tc.Input
				if strings.TrimSpace(args) == "" {
					// A function_call with empty arguments is rejected; the
					// no-argument case must still be a valid JSON object.
					args = "{}"
				}
				items = append(items, cgItem{
					Type:      "function_call",
					Name:      tc.Name,
					Arguments: args,
					CallID:    tc.ID,
				})
			}

		case message.Tool:
			for _, tr := range msg.ToolResults() {
				items = append(items, cgItem{
					Type:   "function_call_output",
					CallID: tr.ToolCallID,
					Output: tr.Content,
				})
			}
		}
	}
	return items
}

func (c *chatgptClient) buildRequest(messages []message.Message, ts []tools.BaseTool) cgRequest {
	req := cgRequest{
		Model:        c.providerOptions.model.APIModel,
		Instructions: c.providerOptions.systemMessage,
		Input:        cgConvertMessages(messages),
		Tools:        convertToolsChatGPT(ts),
		Stream:       true,
		// store:false — do not leave a copy of the user's code in OpenAI's
		// conversation storage. Codex sends the same.
		Store: false,
		// Serial tool calls. The backend advertises parallel support, but this
		// program's agent loop executes one call per turn, so promising parallel
		// invites a response it cannot act on in one pass.
		ParallelToolCalls: false,
	}
	if len(req.Tools) > 0 {
		req.ToolChoice = "auto"
	}
	if c.providerOptions.model.CanReason {
		req.Reasoning = &cgReason{Effort: "medium"}
	}
	return req
}

// ---------------------------------------------------------------------------
// Transport
// ---------------------------------------------------------------------------

func (c *chatgptClient) post(ctx context.Context, body cgRequest) (*http.Response, error) {
	if c.creds == nil {
		return nil, fmt.Errorf("not signed in to ChatGPT — run `gorilla-opencode login --chatgpt`, or choose ChatGPT in the provider portal")
	}
	token, err := c.creds.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	// client_version is a REQUIRED query parameter here exactly as it is on
	// /models; omitting it is a 400 that names ('query', 'client_version') and
	// says nothing about the request body.
	u := fmt.Sprintf("%s/responses?client_version=%s",
		auth.ChatGPTBackend, url.QueryEscape(auth.ChatGPTClientVersion))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	for k, v := range c.creds.AuthHeaders(token) {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	// GORILLA OVERRIDE (2026-08-23): the usage meter for this backend rides the
	// RESPONSE, not a queryable endpoint — there is none. Recording it here, at
	// the one point every ChatGPT request passes through, costs zero extra
	// requests, which is what §8 demands on a metered link. Best-effort and
	// silent: failing to note a usage number must never break the request that
	// carried it. See internal/auth/chatgpt_quota.go.
	if err == nil && resp != nil {
		auth.RecordChatGPTQuota(resp.Header)
	}
	return resp, err
}

// chatgptErr turns an error response into something a user can act on. The
// backend's 429 is a plan cooldown, not a spend limit — saying "quota exceeded"
// to someone on a free plan reads as "you owe money", which is wrong and is the
// kind of message that makes people go looking for a credit card.
func chatgptErr(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var e struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
		Detail json.RawMessage `json:"detail"`
	}
	msg := ""
	if json.Unmarshal(body, &e) == nil {
		msg = strings.TrimSpace(e.Error.Message)
		if msg == "" && len(e.Detail) > 0 {
			msg = strings.Trim(string(e.Detail), `"`)
		}
	}
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return fmt.Errorf("ChatGPT plan limit reached: %s — this is your plan's usage cooldown, not a bill; wait, or switch model", msg)
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("ChatGPT sign-in rejected (HTTP %d): %s — try `gorilla-opencode login --chatgpt --relogin`", resp.StatusCode, msg)
	}
	return fmt.Errorf("ChatGPT backend HTTP %d: %s", resp.StatusCode, msg)
}

// ---------------------------------------------------------------------------
// Response stream
// ---------------------------------------------------------------------------

// cgEvent is the union of the response.* SSE events this client acts on.
// Unknown events are ignored rather than treated as errors: the backend adds
// event types without notice, and a client that fails on an unrecognised one
// breaks the day they ship a new feature.
type cgEvent struct {
	Type string `json:"type"`

	// deltas
	Delta string `json:"delta"`

	// response.output_item.added / .done
	Item cgOutputItem `json:"item"`

	// response.completed / .failed
	Response struct {
		Usage struct {
			InputTokens        int64 `json:"input_tokens"`
			OutputTokens       int64 `json:"output_tokens"`
			InputTokensDetails struct {
				CachedTokens int64 `json:"cached_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
		Status string `json:"status"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"response"`

	// top-level error event
	Message string `json:"message"`
}

type cgOutputItem struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	CallID    string `json:"call_id"`
	ID        string `json:"id"`
}

func (c *chatgptClient) stream(ctx context.Context, messages []message.Message, ts []tools.BaseTool) <-chan ProviderEvent {
	eventChan := make(chan ProviderEvent)
	go func() {
		defer close(eventChan)

		resp, err := c.post(ctx, c.buildRequest(messages, ts))
		if err != nil {
			eventChan <- ProviderEvent{Type: EventError, Error: err}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			eventChan <- ProviderEvent{Type: EventError, Error: chatgptErr(resp)}
			return
		}

		eventChan <- ProviderEvent{Type: EventContentStart}

		var fullText strings.Builder
		var calls []message.ToolCall
		var usage TokenUsage
		finish := message.FinishReasonEndTurn
		var streamErr error

		scanner := bufio.NewScanner(resp.Body)
		// Tool arguments arrive whole on output_item.done, and a large file edit
		// makes that one very long line. 8 MB matches the Antigravity reader.
		scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var ev cgEvent
			if json.Unmarshal([]byte(payload), &ev) != nil {
				continue
			}

			switch ev.Type {
			case "response.output_text.delta":
				if ev.Delta != "" {
					fullText.WriteString(ev.Delta)
					eventChan <- ProviderEvent{Type: EventContentDelta, Content: ev.Delta}
				}

			case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
				// Reasoning summaries, when the account is allowed them. Routed
				// to the thinking channel so they are visibly not the answer.
				if ev.Delta != "" {
					eventChan <- ProviderEvent{Type: EventThinkingDelta, Thinking: ev.Delta}
				}

			case "response.output_item.done":
				if ev.Item.Type != "function_call" {
					continue
				}
				// call_id is what function_call_output must echo back. The item's
				// own "id" is a different identifier and pairing on it silently
				// orphans every tool result.
				id := ev.Item.CallID
				if id == "" {
					id = ev.Item.ID
				}
				args := ev.Item.Arguments
				if strings.TrimSpace(args) == "" {
					args = "{}"
				}
				tc := message.ToolCall{
					ID:       id,
					Name:     ev.Item.Name,
					Input:    args,
					Type:     "function",
					Finished: true,
				}
				calls = append(calls, tc)
				eventChan <- ProviderEvent{Type: EventToolUseStop, ToolCall: &tc}

			case "response.completed":
				usage = TokenUsage{
					InputTokens:     ev.Response.Usage.InputTokens,
					OutputTokens:    ev.Response.Usage.OutputTokens,
					CacheReadTokens: ev.Response.Usage.InputTokensDetails.CachedTokens,
				}

			case "response.incomplete":
				finish = message.FinishReasonMaxTokens

			case "response.failed", "error":
				msg := ev.Response.Error.Message
				if msg == "" {
					msg = ev.Message
				}
				if msg == "" {
					msg = "the ChatGPT backend ended the response without saying why"
				}
				streamErr = fmt.Errorf("ChatGPT: %s", msg)
			}
		}

		if streamErr != nil {
			eventChan <- ProviderEvent{Type: EventError, Error: streamErr}
			return
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			eventChan <- ProviderEvent{Type: EventError, Error: err}
			return
		}
		if len(calls) > 0 {
			finish = message.FinishReasonToolUse
		}

		eventChan <- ProviderEvent{Type: EventContentStop}
		eventChan <- ProviderEvent{
			Type: EventComplete,
			Response: &ProviderResponse{
				Content:      fullText.String(),
				ToolCalls:    calls,
				Usage:        usage,
				FinishReason: finish,
			},
		}
	}()
	return eventChan
}

// send drains stream(). The backend is streaming-only in practice, so this
// deliberately does not carry a second parse of the same wire format — a
// duplicate implementation would be the one nobody exercises and therefore the
// one that rots.
func (c *chatgptClient) send(ctx context.Context, messages []message.Message, ts []tools.BaseTool) (*ProviderResponse, error) {
	for ev := range c.stream(ctx, messages, ts) {
		switch ev.Type {
		case EventError:
			return nil, ev.Error
		case EventComplete:
			return ev.Response, nil
		}
	}
	return nil, fmt.Errorf("ChatGPT backend closed the stream before completing the response")
}
