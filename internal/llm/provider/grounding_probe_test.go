package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/auth"
)

// GORILLA OVERRIDE: a live probe, not a unit test.
//
// It answers ONE question that no amount of reading can settle: does the
// Antigravity backend (daily-cloudcode-pa) accept Google Search grounding —
// tools: [{"googleSearch": {}}] — and return groundingMetadata?
//
// If it does, general web search on a Gemini model needs no SearXNG, no lynx
// and no scraping: the search runs inside Google's infrastructure against
// Google's own index, so none of the bot-detection problems measured on
// 2026-08-08 (CAPTCHAs, anomaly pages, "update your browser") apply.
//
// Opt-in, because it spends real quota:
//
//	GORILLA_LIVE_GROUNDING=1 go test ./internal/llm/provider/ -run LiveGrounding -v
//
// WHY IT ASSERTS ON groundingMetadata AND NOT ON THE ANSWER
//
// There are three outcomes and only one of them is loud:
//
//	400/4xx naming the field  -> rejected. A clear no.
//	200 + groundingMetadata   -> works.
//	200 + NO groundingMetadata -> the endpoint silently dropped a tool type it
//	                              did not recognise, and the model answered
//	                              from memory instead.
//
// The third is the dangerous one, because a fluent answer produced with no
// search whatsoever is indistinguishable from a successful search by reading
// it. That is the same failure this project keeps meeting from new angles -
// silence wearing success's clothes - so the probe checks for the EVIDENCE of
// a search (grounding chunks with URIs), never for a plausible-looking reply.
//
// The query deliberately asks for something a model cannot know from training
// alone, so an ungrounded answer is also visibly wrong. That is a second
// signal, not the assertion.
//
// ---------------------------------------------------------------------------
// ANSWER, measured 2026-08-08: NO. Antigravity does not support grounding.
// ---------------------------------------------------------------------------
//
//	gemini-3.6-flash-medium  HTTP 200 + a confident answer + NO groundingMetadata
//	gemini-3.1-pro-high      HTTP 400 INVALID_ARGUMENT
//
// The envelope is not the problem: gemini-cli places tools in exactly the same
// place (packages/core/src/code_assist/converter.ts, toVertexGenerateContentRequest:
// `tools: req.config?.tools` inside `request`), and that is what this probe sends.
// The daily-cloudcode-pa backend simply does not accept the googleSearch tool.
//
// Note the two failure modes for ONE request shape. The pro model refuses
// honestly; the flash model accepts the request, discards the tool, and answers
// from training data anyway. So even if some future model appeared to work, its
// output could not be trusted without checking groundingMetadata on every call
// - which is why that check is the assertion here and must stay one.
//
// Consequence: web search on this deployment cannot come from Gemini grounding.
// It comes from SearXNG (shipped v0.1.75) or a local text browser. Do not spend
// quota re-testing this without a reason to think the backend changed - and if
// you do, run this file rather than writing a new one.

// probeTool is a local type because the production caTool carries
// `functionDeclarations` without omitempty and would serialise a null field
// alongside googleSearch. A probe must send exactly what it means to send.
type probeTool struct {
	GoogleSearch map[string]any `json:"googleSearch"`
}

type probeInnerRequest struct {
	Contents         []caContent    `json:"contents"`
	Tools            []probeTool    `json:"tools,omitempty"`
	GenerationConfig map[string]any `json:"generationConfig,omitempty"`
}

type probeEnvelope struct {
	Model       string            `json:"model"`
	Project     string            `json:"project,omitempty"`
	Request     probeInnerRequest `json:"request"`
	RequestID   string            `json:"requestId,omitempty"`
	UserAgent   string            `json:"userAgent,omitempty"`
	RequestType string            `json:"requestType,omitempty"`
}

// probeResponse carries the fields the production caResponse does NOT model:
// groundingMetadata is the entire point of this probe.
type probeResponse struct {
	Response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason      string `json:"finishReason"`
			GroundingMetadata *struct {
				GroundingChunks []struct {
					Web *struct {
						URI   string `json:"uri"`
						Title string `json:"title"`
					} `json:"web"`
				} `json:"groundingChunks"`
				WebSearchQueries []string `json:"webSearchQueries"`
			} `json:"groundingMetadata"`
		} `json:"candidates"`
	} `json:"response"`
}

func TestLiveGroundingProbeAntigravity(t *testing.T) {
	if os.Getenv("GORILLA_LIVE_GROUNDING") != "1" {
		t.Skip("set GORILLA_LIVE_GROUNDING=1 to run (spends real Antigravity quota)")
	}

	model := os.Getenv("GORILLA_GROUNDING_MODEL")
	if model == "" {
		model = "gemini-3.6-flash-medium"
	}

	// Read the REAL creds file directly — this package's TestMain overrides
	// XDG_CONFIG_HOME to a temp dir, so auth.LoadAntigravityCreds() resolves
	// into that temp dir and returns (nil, nil). Same workaround, same reason,
	// as TestAntigravityReproMatrix in antigravity_repro_test.go.
	home, _ := os.UserHomeDir()
	data, rerr := os.ReadFile(filepath.Join(home, ".config", "gorilla-opencode", "antigravity-oauth.json"))
	if rerr != nil {
		t.Skip("no Antigravity creds on disk; sign in via the portal first")
	}
	creds := &auth.AntigravityCreds{}
	if err := json.Unmarshal(data, creds); err != nil {
		t.Fatalf("parse creds: %v", err)
	}
	if creds.AccessToken == "" {
		t.Skip("empty creds")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if creds.ProjectID == "" {
		if err := creds.SetupProject(ctx); err != nil {
			t.Fatalf("setting up project: %v", err)
		}
	}
	token, err := creds.Ensure(ctx)
	if err != nil {
		t.Fatalf("could not obtain token: %v", err)
	}
	// Never print the credential. Length and prefix only - enough to tell
	// "empty/garbage" from "a real token", nothing more. (House rule §7.)
	if len(token) < 20 {
		t.Fatalf("token looks wrong: len=%d prefix=%.4q", len(token), token)
	}
	t.Logf("token acquired: len=%d prefix=%.4q…", len(token), token)

	const question = "Using web search, what is the newest stable Linux kernel " +
		"version released as of today, and on what date was it released? " +
		"Answer with the version and date."

	env := probeEnvelope{
		Model:   model,
		Project: creds.ProjectID,
		Request: probeInnerRequest{
			Contents: []caContent{{
				Role:  "user",
				Parts: []caPart{{Text: question}},
			}},
			// THE PROBE. Everything else here is known-good.
			Tools:            []probeTool{{GoogleSearch: map[string]any{}}},
			GenerationConfig: map[string]any{"maxOutputTokens": 512},
		},
		RequestID:   "agent/groundingprobe",
		UserAgent:   auth.AntigravityRequestUA,
		RequestType: "agent",
	}

	body, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("request tools field: %s", `[{"googleSearch":{}}]`)

	u := fmt.Sprintf("%s/%s:%s", auth.AntigravityEndpoint, auth.AntigravityVersion, "generateContent")
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", auth.AntigravityUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	t.Logf("HTTP %d, %d bytes", resp.StatusCode, len(raw))

	// OUTCOME 1: rejected outright. A clear, loud no.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("REJECTED — googleSearch is not accepted on this endpoint.\nHTTP %d: %s",
			resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out probeResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("could not parse response: %v\nbody: %.600s", err, raw)
	}
	if len(out.Response.Candidates) == 0 {
		t.Fatalf("no candidates returned. body: %.600s", raw)
	}
	cand := out.Response.Candidates[0]

	var answer strings.Builder
	for _, p := range cand.Content.Parts {
		answer.WriteString(p.Text)
	}
	t.Logf("answer: %s", strings.TrimSpace(answer.String()))

	// OUTCOME 3: the quiet failure. 200, fluent prose, no evidence a search
	// ever happened. This MUST fail the test - it is the case that would
	// otherwise ship as a working feature.
	if cand.GroundingMetadata == nil || len(cand.GroundingMetadata.GroundingChunks) == 0 {
		t.Fatalf("SILENTLY IGNORED — HTTP 200 with an answer but NO groundingMetadata.\n" +
			"The endpoint accepted the request and dropped the googleSearch tool; the\n" +
			"model answered from training data. Do NOT build on this: an ungrounded\n" +
			"answer is indistinguishable from a searched one by reading it.")
	}

	// OUTCOME 2: it works.
	gm := cand.GroundingMetadata
	t.Logf("GROUNDED — %d chunk(s), queries: %v", len(gm.GroundingChunks), gm.WebSearchQueries)
	withURI := 0
	for i, c := range gm.GroundingChunks {
		if c.Web == nil || c.Web.URI == "" {
			continue
		}
		withURI++
		if i < 8 {
			t.Logf("  [%d] %s — %s", i+1, c.Web.Title, c.Web.URI)
		}
	}
	if withURI == 0 {
		t.Fatal("groundingMetadata present but no chunk carries a web URI — " +
			"citations would be empty, which is not a usable result")
	}
}
