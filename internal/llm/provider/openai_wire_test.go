package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/openai/openai-go"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/message"
)

// wireJSON marshals the request exactly as it goes on the wire. Both bugs below
// were invisible in Go and obvious in the JSON, which is why these assertions
// are made against bytes rather than against structs.
func wireJSON(t *testing.T, msgs []message.Message, tools []openai.ChatCompletionToolParam) string {
	t.Helper()
	o := &openaiClient{providerOptions: providerClientOptions{
		model:     models.Model{APIModel: "@cf/openai/gpt-oss-120b"},
		maxTokens: 200,
	}}
	params := o.preparedParams(o.convertMessages(msgs), tools)
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshalling request: %v", err)
	}
	return string(b)
}

// THE BUG: an assistant turn that only calls a tool has no text, and leaving
// Content unset serialised as `"content": null`. OpenAI allows it; Cloudflare
// Workers AI rejects the whole request:
//
//	400  Type mismatch of '/messages/1/content', 'string' not in 'null'
//
// The message stays in the history forever, so a conversation survived exactly
// until its first tool call and then failed on every turn after. Observed
// 2026-08-05 reading a file: the view ran, and the follow-up 400'd.
func TestAssistantContentIsNeverNull(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "read the file"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "call_1", Name: "view", Input: `{"file_path":"/tmp/x"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "call_1", Content: "(empty)"},
		}},
	}
	got := wireJSON(t, msgs, []openai.ChatCompletionToolParam{})

	// The SDK OMITS the field when unset rather than writing `"content":null`,
	// so asserting on the literal "null" is vacuous — verified by reverting the
	// fix and watching that assertion still pass. What Cloudflare actually
	// rejects is content being ABSENT ("required properties at '/messages/1' are
	// 'role,content'"), so that is what has to be asserted: every assistant
	// message must carry a content field, empty string included.
	var req struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal([]byte(got), &req); err != nil {
		t.Fatalf("unmarshalling the request: %v", err)
	}
	seen := false
	for _, m := range req.Messages {
		if m["role"] != "assistant" {
			continue
		}
		seen = true
		c, ok := m["content"]
		if !ok {
			t.Errorf("the assistant message has NO content field; Cloudflare requires "+
				"one even when the turn is only a tool call:\n%s", got)
		}
		if c == nil {
			t.Errorf("the assistant message has a null content, which Cloudflare "+
				"rejects outright:\n%s", got)
		}
		if _, isStr := c.(string); !isStr && c != nil {
			t.Errorf("assistant content is %T, want a string", c)
		}
	}
	if !seen {
		t.Fatal("no assistant message in the request; the fixture is wrong")
	}
}

// THE OTHER BUG: generateTitle and the summarizer call SendMessages with
// make([]tools.BaseTool, 0), which serialised as `"tools": []`. Cloudflare:
//
//	400  Value error, `tools` must not be an empty array.
//	     Either provide at least one tool or omit the field
//
// Session titles failed against Cloudflare while ordinary turns worked, and the
// summarizer would have failed the moment a conversation needed compacting.
func TestEmptyToolsIsOmittedNotSentAsAnEmptyArray(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
	}
	got := wireJSON(t, msgs, []openai.ChatCompletionToolParam{})

	if strings.Contains(got, `"tools":[]`) {
		t.Errorf("the request sends an empty tools array, which Cloudflare rejects "+
			"outright:\n%s", got)
	}
	if strings.Contains(got, `"tools"`) {
		t.Errorf("the tools field is present with no tools; it must be omitted:\n%s", got)
	}
}

// And when there ARE tools, they must still be sent — the fix must not have
// silently disabled tool use.
func TestToolsAreStillSentWhenPresent(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
	}
	tools := []openai.ChatCompletionToolParam{{
		Function: openai.FunctionDefinitionParam{Name: "view"},
	}}
	got := wireJSON(t, msgs, tools)

	if !strings.Contains(got, `"tools"`) {
		t.Errorf("tools were dropped from a request that has them:\n%s", got)
	}
	if !strings.Contains(got, `"view"`) {
		t.Errorf("the tool definition is missing:\n%s", got)
	}
}
