// GORILLA OVERRIDE: this file did not exist upstream. It is the transport
// for "Login with Google" (see internal/auth/gemini_oauth.go): it speaks
// Google's Code Assist envelope (cloudcode-pa.googleapis.com) so a
// logged-in user runs Gemini through the free tier instead of an API key.
//
// It is NOT OpenAI-compatible and it does NOT use the genai SDK — the
// backend wraps the standard Gemini request as {model, project, request:{…}}
// and nests each streamed response under a "response" field. We build that
// JSON by hand, mirroring the conversion the SDK-based gemini.go does.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/opencode-ai/opencode/internal/auth"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

type codeAssistClient struct {
	providerOptions providerClientOptions
	creds           *auth.GeminiCreds
}

// CodeAssistClient satisfies the ProviderClient type parameter.
type CodeAssistClient = *codeAssistClient

func newCodeAssistClient(opts providerClientOptions) CodeAssistClient {
	creds, _ := auth.LoadGeminiCreds()
	return &codeAssistClient{providerOptions: opts, creds: creds}
}

// ---- JSON shapes (Gemini REST, hand-built) ---------------------------------

type caPart struct {
	Text             string          `json:"text,omitempty"`
	FunctionCall     *caFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *caFunctionResp `json:"functionResponse,omitempty"`
	InlineData       *caBlob         `json:"inlineData,omitempty"`
	// REST carries the Gemini-3 thought signature as a base64 string;
	// our message.ToolCall already stores it base64, so it round-trips
	// verbatim (required — replayed calls 400 without it).
	ThoughtSignature string `json:"thoughtSignature,omitempty"`
}

type caFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type caFunctionResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type caBlob struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"`
}

type caContent struct {
	Role  string   `json:"role,omitempty"` // omitted for systemInstruction
	Parts []caPart `json:"parts"`
}

type caFuncDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type caTool struct {
	FunctionDeclarations []caFuncDecl `json:"functionDeclarations"`
}

type caInnerRequest struct {
	Contents          []caContent    `json:"contents"`
	SystemInstruction *caContent     `json:"systemInstruction,omitempty"`
	Tools             []caTool       `json:"tools,omitempty"`
	GenerationConfig  map[string]any `json:"generationConfig,omitempty"`
}

type caEnvelope struct {
	Model   string         `json:"model"`
	Project string         `json:"project,omitempty"`
	Request caInnerRequest `json:"request"`
}

// caResponse is one chunk (SSE) or the whole non-streaming reply; the real
// Gemini payload is nested under "response".
type caResponse struct {
	Response struct {
		Candidates []struct {
			Content struct {
				Parts []caPart `json:"parts"`
				Role  string   `json:"role"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int64 `json:"promptTokenCount"`
			CandidatesTokenCount int64 `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	} `json:"response"`
}

// ---- conversion (mirrors gemini.go convertMessages/convertTools) -----------

func (c *codeAssistClient) convertMessages(messages []message.Message) []caContent {
	var out []caContent
	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			parts := []caPart{{Text: msg.Content().String()}}
			for _, bin := range msg.BinaryContent() {
				mime := bin.MIMEType
				if i := strings.IndexByte(mime, '/'); i >= 0 {
					mime = mime[i+1:]
				}
				parts = append(parts, caPart{InlineData: &caBlob{MIMEType: mime, Data: base64.StdEncoding.EncodeToString(bin.Data)}})
			}
			out = append(out, caContent{Role: "user", Parts: parts})
		case message.Assistant:
			var parts []caPart
			if txt := msg.Content().String(); txt != "" {
				parts = append(parts, caPart{Text: txt})
			}
			for _, call := range msg.ToolCalls() {
				args, _ := parseJsonToMap(call.Input)
				parts = append(parts, caPart{
					FunctionCall:     &caFunctionCall{Name: call.Name, Args: args},
					ThoughtSignature: call.ThoughtSignature,
				})
			}
			if len(parts) > 0 {
				out = append(out, caContent{Role: "model", Parts: parts})
			}
		case message.Tool:
			for _, result := range msg.ToolResults() {
				resp := map[string]any{"result": result.Content}
				if parsed, err := parseJsonToMap(result.Content); err == nil {
					resp = parsed
				}
				name := ""
				for _, m := range messages {
					if m.Role != message.Assistant {
						continue
					}
					for _, call := range m.ToolCalls() {
						if call.ID == result.ToolCallID {
							name = call.Name
						}
					}
				}
				// The 2025 API takes functionResponse in a "user" turn.
				out = append(out, caContent{Role: "user", Parts: []caPart{{
					FunctionResponse: &caFunctionResp{Name: name, Response: resp},
				}}})
			}
		}
	}
	return out
}

func convertToolsCA(ts []tools.BaseTool) []caTool {
	if len(ts) == 0 {
		return nil
	}
	decls := make([]caFuncDecl, 0, len(ts))
	for _, t := range ts {
		info := t.Info()
		decls = append(decls, caFuncDecl{
			Name:        info.Name,
			Description: info.Description,
			Parameters: map[string]any{
				"type":       "object",
				"properties": info.Parameters,
				"required":   info.Required,
			},
		})
	}
	return []caTool{{FunctionDeclarations: decls}}
}

func (c *codeAssistClient) buildEnvelope(messages []message.Message, ts []tools.BaseTool) caEnvelope {
	req := caInnerRequest{
		Contents: c.convertMessages(messages),
		Tools:    convertToolsCA(ts),
		GenerationConfig: map[string]any{
			"maxOutputTokens": c.providerOptions.maxTokens,
		},
	}
	if sys := c.providerOptions.systemMessage; sys != "" {
		req.SystemInstruction = &caContent{Parts: []caPart{{Text: sys}}}
	}
	project := ""
	if c.creds != nil {
		project = c.creds.ProjectID
	}
	return caEnvelope{
		Model:   c.providerOptions.model.APIModel,
		Project: project,
		Request: req,
	}
}

// ---- HTTP ------------------------------------------------------------------

func (c *codeAssistClient) post(ctx context.Context, method string, env caEnvelope) (*http.Response, error) {
	if c.creds == nil {
		return nil, fmt.Errorf("not signed in — run 'gorilla-opencode login'")
	}
	token, err := c.creds.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/%s:%s", auth.CodeAssistEndpoint, auth.CodeAssistVersion, method)
	if method == "streamGenerateContent" {
		u += "?alt=sse"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func httpErr(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	var e struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		msg := strings.TrimRight(e.Error.Message, ".")
		if resp.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("Gemini free tier: %s — switch to a lighter model (Flash / Flash-Lite) or try again later", msg)
		}
		return fmt.Errorf("Gemini Code Assist: %s (HTTP %d)", msg, resp.StatusCode)
	}
	return fmt.Errorf("Code Assist HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// collectParts turns Gemini parts into (text, toolCalls) and echoes thought
// signatures back into the tool calls so replayed history stays valid.
func collectParts(parts []caPart) (string, []message.ToolCall) {
	var sb strings.Builder
	var calls []message.ToolCall
	for _, p := range parts {
		if p.Text != "" {
			sb.WriteString(p.Text)
		}
		if p.FunctionCall != nil {
			input, _ := json.Marshal(p.FunctionCall.Args)
			calls = append(calls, message.ToolCall{
				ID:               newCallID(),
				Name:             p.FunctionCall.Name,
				Input:            string(input),
				Type:             "function",
				Finished:         true,
				ThoughtSignature: p.ThoughtSignature,
			})
		}
	}
	return sb.String(), calls
}

func newCallID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "call_" + hex.EncodeToString(b)
}

func mapFinish(reason string, hasTools bool) message.FinishReason {
	if hasTools {
		return message.FinishReasonToolUse
	}
	switch reason {
	case "STOP":
		return message.FinishReasonEndTurn
	case "MAX_TOKENS":
		return message.FinishReasonMaxTokens
	case "":
		return message.FinishReasonEndTurn
	default:
		return message.FinishReasonUnknown
	}
}

// ---- ProviderClient impl ---------------------------------------------------

func (c *codeAssistClient) send(ctx context.Context, messages []message.Message, ts []tools.BaseTool) (*ProviderResponse, error) {
	resp, err := c.post(ctx, "generateContent", c.buildEnvelope(messages, ts))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpErr(resp)
	}
	var r caResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	if len(r.Response.Candidates) == 0 {
		return &ProviderResponse{FinishReason: message.FinishReasonEndTurn}, nil
	}
	cand := r.Response.Candidates[0]
	text, calls := collectParts(cand.Content.Parts)
	return &ProviderResponse{
		Content:      text,
		ToolCalls:    calls,
		Usage:        TokenUsage{InputTokens: r.Response.UsageMetadata.PromptTokenCount, OutputTokens: r.Response.UsageMetadata.CandidatesTokenCount},
		FinishReason: mapFinish(cand.FinishReason, len(calls) > 0),
	}, nil
}

func (c *codeAssistClient) stream(ctx context.Context, messages []message.Message, ts []tools.BaseTool) <-chan ProviderEvent {
	eventChan := make(chan ProviderEvent)
	go func() {
		defer close(eventChan)

		resp, err := c.post(ctx, "streamGenerateContent", c.buildEnvelope(messages, ts))
		if err != nil {
			eventChan <- ProviderEvent{Type: EventError, Error: err}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			eventChan <- ProviderEvent{Type: EventError, Error: httpErr(resp)}
			return
		}

		eventChan <- ProviderEvent{Type: EventContentStart}

		var fullText strings.Builder
		var allCalls []message.ToolCall
		var usage TokenUsage
		finish := ""

		scanner := bufio.NewScanner(resp.Body)
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
			var chunk caResponse
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}
			if len(chunk.Response.Candidates) == 0 {
				continue
			}
			cand := chunk.Response.Candidates[0]
			for _, p := range cand.Content.Parts {
				if p.Text != "" {
					fullText.WriteString(p.Text)
					eventChan <- ProviderEvent{Type: EventContentDelta, Content: p.Text}
				}
			}
			_, calls := collectParts(cand.Content.Parts)
			allCalls = append(allCalls, calls...)
			if cand.FinishReason != "" {
				finish = cand.FinishReason
			}
			if chunk.Response.UsageMetadata.CandidatesTokenCount > 0 {
				usage = TokenUsage{
					InputTokens:  chunk.Response.UsageMetadata.PromptTokenCount,
					OutputTokens: chunk.Response.UsageMetadata.CandidatesTokenCount,
				}
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			eventChan <- ProviderEvent{Type: EventError, Error: err}
			return
		}

		eventChan <- ProviderEvent{Type: EventContentStop}
		eventChan <- ProviderEvent{
			Type: EventComplete,
			Response: &ProviderResponse{
				Content:      fullText.String(),
				ToolCalls:    allCalls,
				Usage:        usage,
				FinishReason: mapFinish(finish, len(allCalls) > 0),
			},
		}
	}()
	return eventChan
}
