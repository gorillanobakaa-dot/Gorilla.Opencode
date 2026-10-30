//go:build integration

package tools

import (
	"context"
	"os"
	"strings"
	"testing"
)

// Real fieldkit, real browser, real network. Run by hand:
//
//	fieldkit guard -- go test -tags integration -run TestBrowserFallbackReal -v ./internal/llm/tools/
//
// FETCH_JS_URL overrides the page (default: a page whose quotes are built by JS).
func TestBrowserFallbackReal(t *testing.T) {
	url := os.Getenv("FETCH_JS_URL")
	if url == "" {
		url = "https://quotes.toscrape.com/js/"
	}
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "s1")
	ctx = context.WithValue(ctx, MessageIDContextKey, "m1")
	resp, err := NewFetchTool(&recordingPermissions{grant: true}).Run(ctx, ToolCall{
		Name: FetchToolName, Input: `{"url":"` + url + `"}`})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp.Content)
	if resp.IsError || !strings.Contains(resp.Content, "rendered by a local browser") {
		t.Fatalf("no browser fallback: %s", resp.Content)
	}
}

// The subprocess leg alone: real fieldkit and browser on a JS-built page.
func TestBrowserFetchRealDirect(t *testing.T) {
	text, browser, reason := browserFetch(context.Background(), realURL())
	if reason != "" {
		t.Fatalf("fallback failed: %s", reason)
	}
	t.Logf("browser=%s\n%.300s", browser, text)
	if strings.TrimSpace(text) == "" {
		t.Fatal("no text")
	}
}

func realURL() string {
	if u := os.Getenv("FETCH_JS_URL"); u != "" {
		return u
	}
	return "https://quotes.toscrape.com/js/"
}
