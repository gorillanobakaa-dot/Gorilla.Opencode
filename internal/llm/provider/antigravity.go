// GORILLA OVERRIDE: this file did not exist upstream. It is the transport for
// Google Antigravity's free tier (Claude, GPT-OSS, Gemini) — the sibling of
// code_assist.go. Same Code Assist envelope and identical streamed response
// shape, so it reuses that file's conversion (caConvertMessages), tool
// conversion (convertToolsCA), part collection (collectParts) and finish
// mapping (mapFinish). It differs only where the live agy capture showed it must:
//   - endpoint daily-cloudcode-pa.googleapis.com (not cloudcode-pa),
//   - a generation User-Agent that identifies as the Antigravity CLI (REQUIRED
//     here; the Gemini path forbids it on generation),
//   - three extra top-level envelope fields (requestId, userAgent, requestType),
//   - the top-level model is a plain name (e.g. "claude-sonnet-4-6"); model
//     selection is per-request, NOT the stateful setUserSettings dance agy's UI
//     narrates into the prompt. Proven live 2026-08-03.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
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

type antigravityClient struct {
	providerOptions providerClientOptions
	creds           *auth.AntigravityCreds
}

// AntigravityClient satisfies the ProviderClient type parameter.
type AntigravityClient = *antigravityClient

func newAntigravityClient(opts providerClientOptions) AntigravityClient {
	creds, _ := auth.LoadAntigravityCreds()
	return &antigravityClient{providerOptions: opts, creds: creds}
}

func (c *antigravityClient) buildEnvelope(messages []message.Message, ts []tools.BaseTool) caEnvelope {
	req := caInnerRequest{
		Contents: caConvertMessages(messages),
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
		Model:       c.providerOptions.model.APIModel,
		Project:     project,
		Request:     req,
		RequestID:   "agent/" + randHex(16),
		UserAgent:   auth.AntigravityRequestUA,
		RequestType: "agent",
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (c *antigravityClient) post(ctx context.Context, method string, env caEnvelope) (*http.Response, error) {
	if c.creds == nil {
		return nil, fmt.Errorf("not signed in to Antigravity — choose it in the provider portal to sign in")
	}
	// Lazily discover the managed free-tier project the envelope needs.
	if c.creds.ProjectID == "" {
		if err := c.creds.SetupProject(ctx); err != nil {
			return nil, fmt.Errorf("setting up Antigravity project: %w", err)
		}
		env.Project = c.creds.ProjectID
	}
	token, err := c.creds.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/%s:%s", auth.AntigravityEndpoint, auth.AntigravityVersion, method)
	if method == "streamGenerateContent" {
		u += "?alt=sse"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	// Unlike the Gemini path, generation DOES carry the Antigravity UA.
	req.Header.Set("User-Agent", auth.AntigravityUserAgent)
	return http.DefaultClient.Do(req)
}

func antigravityErr(resp *http.Response) error {
	// Reuse the same error-body shape as httpErr but with Antigravity wording.
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
			return fmt.Errorf("Antigravity free tier: %s — this model's weekly quota may be spent; try another model", msg)
		}
		return fmt.Errorf("Antigravity: %s (HTTP %d)", msg, resp.StatusCode)
	}
	return fmt.Errorf("Antigravity HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func (c *antigravityClient) send(ctx context.Context, messages []message.Message, ts []tools.BaseTool) (*ProviderResponse, error) {
	resp, err := c.post(ctx, "generateContent", c.buildEnvelope(messages, ts))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, antigravityErr(resp)
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

func (c *antigravityClient) stream(ctx context.Context, messages []message.Message, ts []tools.BaseTool) <-chan ProviderEvent {
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
			eventChan <- ProviderEvent{Type: EventError, Error: antigravityErr(resp)}
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
