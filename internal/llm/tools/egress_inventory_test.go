package tools

// GORILLA OVERRIDE (2026-08-19): an inventory test, not a ban.
//
// The audit proposal wanted one rule — "nothing in internal/ constructs its own
// http.Client" — and grepping showed six that do. Five of them are legitimate
// and a blanket ban would have to be suppressed five times, which is how a
// rule becomes decoration.
//
// The distinction that actually matters is not "who builds a client" but "who
// lets the MODEL choose the address". A provider client talks to the endpoint
// the user configured; the model cannot redirect it. web_fetch takes a URL
// straight from the model, which is why it has three layers of SSRF guard, and
// MCP SSE took one straight from config with none until BlockedMCPTarget.
//
// So this test freezes the inventory with a reason attached to each entry. It
// does not stop anyone adding a client — it stops one being added SILENTLY,
// which is exactly how the MCP hole survived: the guard existed twenty files
// away and nothing connected the two.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var knownHTTPClients = map[string]string{
	"internal/llm/provider/httpclient.go":    "the AI endpoint the user configured; the model cannot choose the address",
	"internal/auth/httpclient.go":            "the shared bounded client for provider auth endpoints; every URL is a constant, the model never supplies one",
	"internal/llm/models/refresh.go":         "the model catalogue, a fixed URL",
	"internal/llm/models/catalogue_fetch.go": "provider /v1/models listings; the URLs are constants in LiveCatalogues, the model cannot choose one",
	"internal/llm/models/verify.go":          "verifies a configured provider's endpoint answers",
	"internal/llm/tools/fetch.go":            "MODEL-CHOSEN: guarded by blockedFetchTarget + dialer Control + CheckRedirect",
	"internal/llm/tools/websearch.go":        "the SearxNG instance from config; the model supplies the query, not the host",
	"internal/llm/models/local.go":           "OpenAI-compatible /v1/models listings for endpoints the USER configured (or the two default local ports); the model never supplies the address",
}

func TestNoUninventoriedHTTPClient(t *testing.T) {
	root := "../../.."
	var found []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if !strings.HasPrefix(rel, "internal/") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !strings.Contains(string(b), "http.Client{") {
			return nil
		}
		if _, ok := knownHTTPClients[rel]; !ok {
			found = append(found, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	for _, f := range found {
		t.Errorf("%s constructs an http.Client and is not in knownHTTPClients.\n"+
			"If the model can influence the address it dials, it needs the guard in egress.go.\n"+
			"If it cannot, add it to the map with the reason — that reason is the point of this test.", f)
	}
}
