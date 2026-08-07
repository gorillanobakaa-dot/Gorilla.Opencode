package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// GORILLA OVERRIDE: a repeatable measurement of the token sieve, run against
// real pages rather than a synthetic fixture.
//
// PHILOSOPHY.md Part Seven claims the ladder cuts a page by roughly 83% before
// anything reaches a paid API. That number came from one arXiv page. This test
// re-measures it across heavy, real-world pages so the claim can be checked
// rather than trusted - and so a regression in the stripping logic shows up as
// a number instead of a slightly larger bill.
//
// Opt-in, because it needs the network and the sizes move as sites change:
//
//	GORILLA_TOKEN_BENCH=1 go test ./internal/llm/tools/ -run TokenCost -v
//
// Nothing is stored and nothing is interpreted; this reads Content-Length and
// counts bytes.
func TestTokenCostOfRealPages(t *testing.T) {
	if os.Getenv("GORILLA_TOKEN_BENCH") != "1" {
		t.Skip("set GORILLA_TOKEN_BENCH=1 to run (needs network)")
	}

	pages := []struct{ label, url string }{
		{"arXiv abstract page", "https://arxiv.org/abs/2509.03518"},
		{"arXiv export API", "https://export.arxiv.org/api/query?id_list=2509.03518"},
		{"Anna's Archive search", "https://annas-archive.is/search?q=machine+learning"},
		{"Wikipedia article", "https://en.wikipedia.org/wiki/Reverse_proxy"},
		{"Wikipedia REST summary", "https://en.wikipedia.org/api/rest_v1/page/summary/Reverse_proxy"},
		{"MDN docs page", "https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Accept"},
		{"GitHub blob page", "https://github.com/golang/go/blob/master/README.md"},
		{"raw.githubusercontent", "https://raw.githubusercontent.com/golang/go/master/README.md"},
	}

	client := &http.Client{Timeout: 45 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Printf("\n%-26s %10s %10s %9s %9s\n", "page", "raw B", "sent B", "raw tok", "sent tok")
	fmt.Println(strings.Repeat("-", 68))

	var totalRaw, totalSent int
	for _, p := range pages {
		req, err := http.NewRequestWithContext(ctx, "GET", p.url, nil)
		if err != nil {
			t.Errorf("%s: %v", p.label, err)
			continue
		}
		req.Header.Set("User-Agent", "gorilla-opencode/1.0 (token-cost benchmark)")
		req.Header.Set("Accept", "text/markdown, text/plain;q=0.9, text/html;q=0.8, */*;q=0.1")
		resp, err := client.Do(req)
		if err != nil {
			t.Logf("%-26s unreachable: %v", p.label, err)
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Logf("%-26s HTTP %d", p.label, resp.StatusCode)
			continue
		}

		raw := len(body)
		ct := resp.Header.Get("Content-Type")

		// Exactly what the tool would hand the model.
		var sent string
		if looksLikeHTML(ct, body) {
			md, err := convertHTMLToMarkdown(decodeBody(body, ct))
			if err != nil {
				t.Errorf("%s: conversion failed: %v", p.label, err)
				continue
			}
			sent = md
		} else {
			sent = decodeBody(body, ct)
		}

		totalRaw += raw
		totalSent += len(sent)
		fmt.Printf("%-26s %10d %10d %9d %9d   %.0f%% cut\n",
			p.label, raw, len(sent), raw/4, len(sent)/4,
			100*(1-float64(len(sent))/float64(raw)))

		time.Sleep(1500 * time.Millisecond) // be polite to every host here
	}

	if totalRaw == 0 {
		t.Skip("no pages reachable")
	}
	cut := 100 * (1 - float64(totalSent)/float64(totalRaw))
	fmt.Println(strings.Repeat("-", 68))
	fmt.Printf("%-26s %10d %10d %9d %9d   %.0f%% cut\n",
		"TOTAL", totalRaw, totalSent, totalRaw/4, totalSent/4, cut)

	// The claim in PHILOSOPHY.md Part Seven. If stripping regresses, this fails
	// rather than quietly costing users money.
	if cut < 50 {
		t.Errorf("token sieve cut only %.0f%% overall; PHILOSOPHY.md Part Seven "+
			"claims the ladder removes most of a page. Either the stripping "+
			"regressed or the claim needs revising - do not leave them disagreeing.", cut)
	}
}
