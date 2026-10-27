package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
)

// GORILLA FIX (2026-10-27, job 09, gap #12): general web search without lynx.
//
// The built-in reader fetches the same engines' results pages the lynx route
// renders (lynxEngines, in the same order) and turns them into lynx's dump
// layout with htmlToText, so parseLynxDump and every rule in it apply
// unchanged: success is still "at least one external result extracted".
//
// It is OFF unless the user turns it on ("webSearchBuiltin": true in
// config.json, or GORILLA_OPENCODE_WEB_READER=builtin). It sends the search
// terms to those engines, which is a channel the user opens, not one this
// program opens for them. The User-Agent says what it is (see the lynx file's
// header for why an honest one fails more legibly).

// webReaderEnvVar turns the built-in reader on without editing a file.
const webReaderEnvVar = "GORILLA_OPENCODE_WEB_READER"

// builtinReaderBodyLimit caps one results page. A results page is tens of KB;
// anything near this is not one.
const builtinReaderBodyLimit = 3 << 20

// builtinReaderEnabled reports whether the user turned the built-in reader on.
func builtinReaderEnabled() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(webReaderEnvVar)), "builtin") {
		return true
	}
	cfg := config.Get()
	return cfg != nil && cfg.WebSearchBuiltin
}

// searchBuiltin is the lynx route with this program as the browser.
func (t *webSearchTool) searchBuiltin(ctx context.Context, q string, n int) ([]searchHit, error) {
	return t.searchTextEngines(ctx, q, n, "reader", t.fetchAsText)
}

// fetchAsText GETs one page through the SSRF-safe client and renders it.
func (t *webSearchTool) fetchAsText(ctx context.Context, target string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", politeContact)
	req.Header.Set("Accept", "text/html")
	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, builtinReaderBodyLimit))
	if err != nil {
		return "", err
	}
	text := htmlToText(string(body), resp.Request.URL)
	if resp.StatusCode != http.StatusOK {
		// The text is still returned: a block page that names itself is
		// worth the bytes count in the failure line.
		return text, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return text, nil
}

// WebSearchStatus is one line per web route for `gorilla-opencode doctor`:
// what this machine can search the open web with, and what is missing.
func WebSearchStatus() []string {
	var lines []string
	routes := 0
	if ep := searxngEndpoint(); ep != "" {
		routes++
		lines = append(lines, "[ok] web search: SearXNG at "+redactURL(ep))
	} else {
		lines = append(lines, "[i] web search: no SearXNG configured (\"searxngURL\" in config.json or "+searxngEnvVar+")")
	}
	if p := lynxPath(); p != "" {
		routes++
		lines = append(lines, "[ok] web search: lynx at "+p)
	} else {
		lines = append(lines, "[i] web search: lynx not installed")
	}
	if builtinReaderEnabled() {
		routes++
		lines = append(lines, "[ok] web search: built-in reader on (sends search terms to Brave, Ecosia, Mojeek, Marginalia)")
	} else {
		lines = append(lines, "[i] web search: built-in reader off (\"webSearchBuiltin\": true in config.json, or "+webReaderEnvVar+"=builtin)")
	}
	if routes == 0 {
		lines = append(lines, "[!] web search: source=web is OFF (none of the three above); the scholarly sources still work")
	}
	return lines
}

// redactURL drops any user:password from an address before it is printed.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}
