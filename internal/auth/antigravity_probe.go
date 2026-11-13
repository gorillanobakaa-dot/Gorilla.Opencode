package auth

// GORILLA OVERRIDE: a model list is not evidence that a model answers. This
// sends ONE tiny real prompt to a model id and reports whether the backend
// accepted it, so a model that 404s is never shown as usable.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProbeResult is the outcome of one real prompt.
type ProbeResult struct {
	OK       bool
	NotFound bool // HTTP 404 / "Requested entity was not found": hide the model
	Status   int
	Note     string
}

// ProbeAntigravityModel sends "Reply with the single word OK." to model id and
// reads the answer. Never logs the token.
func (c *AntigravityCreds) ProbeAntigravityModel(ctx context.Context, id string) ProbeResult {
	token, err := c.Ensure(ctx)
	if err != nil {
		return ProbeResult{Note: err.Error()}
	}
	env := map[string]any{
		"model":       id,
		"project":     c.ProjectID,
		"requestId":   fmt.Sprintf("agent/probe-%d", time.Now().UnixNano()),
		"userAgent":   EffectiveAntigravityRequestUA(),
		"requestType": "agent",
		"request": map[string]any{
			"contents":         []map[string]any{{"role": "user", "parts": []map[string]string{{"text": "Reply with the single word OK."}}}},
			"generationConfig": map[string]any{"maxOutputTokens": 64},
		},
	}
	data, _ := json.Marshal(env)
	u := fmt.Sprintf("%s/%s:generateContent", AntigravityEndpoint, AntigravityVersion)
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(data))
	if err != nil {
		return ProbeResult{Note: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", EffectiveAntigravityUserAgent())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ProbeResult{Note: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	return ClassifyProbe(resp.StatusCode, string(body))
}

// ClassifyProbe turns a status and body into a ProbeResult (pure, tested).
func ClassifyProbe(status int, body string) ProbeResult {
	r := ProbeResult{Status: status}
	switch {
	case status == 200:
		r.OK = true
		r.Note = "answered"
	case status == 404 || strings.Contains(body, "Requested entity was not found"):
		r.NotFound = true
		r.Note = "not found: the backend lists it but does not serve it"
	case status == 429:
		// Quota spent means the model exists; keep it.
		r.OK = true
		r.Note = "quota spent (model exists)"
	default:
		r.Note = fmt.Sprintf("HTTP %d", status)
	}
	return r
}
