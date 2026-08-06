package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// GORILLA OVERRIDE: a LIVE probe. It talks to real provider endpoints and
// spends real (tiny) quota, so it is skipped unless
// GORILLA_LIVE_GZIP_PROBE=1 is set. Nothing in an ordinary `go test ./...`
// run reaches the network.
//
// Purpose: gzip_request_test.go proves the transport behaves correctly
// against a server *we wrote*. That proves the logic and nothing about the
// real world. This answers the only question that actually matters — does
// the provider accept a gzipped request body — by asking it.
//
//	GORILLA_LIVE_GZIP_PROBE=1 go test ./internal/llm/provider/ \
//	    -run TestLiveGzipProbe -v -count=1
func TestLiveGzipProbe(t *testing.T) {
	if os.Getenv("GORILLA_LIVE_GZIP_PROBE") != "1" {
		t.Skip("live probe: set GORILLA_LIVE_GZIP_PROBE=1 to run (spends real quota)")
	}

	type target struct {
		name    string
		baseURL string
		key     string
		model   string
	}

	var targets []target
	add := func(name, base, key, model string) {
		if base == "" {
			return
		}
		targets = append(targets, target{name, strings.TrimRight(base, "/"), key, model})
	}
	add("nim", os.Getenv("PROBE_NIM_URL"), os.Getenv("PROBE_NIM_KEY"), os.Getenv("PROBE_NIM_MODEL"))
	add("cloudflare", os.Getenv("PROBE_CF_URL"), os.Getenv("PROBE_CF_KEY"), os.Getenv("PROBE_CF_MODEL"))
	add("ollama", os.Getenv("PROBE_OLLAMA_URL"), "", os.Getenv("PROBE_OLLAMA_MODEL"))

	if len(targets) == 0 {
		t.Skip("live probe: no PROBE_*_URL targets configured")
	}

	for _, tg := range targets {
		t.Run(tg.name, func(t *testing.T) {
			// Pad past gzipMinRequestBytes with prose, not filler, so the
			// ratio resembles a real conversation.
			var filler strings.Builder
			for i := 0; filler.Len() < 3000; i++ {
				fmt.Fprintf(&filler,
					"Turn %d: rebuilt arch/x86/mm/fault_%d.o and re-ran the failing test; ld reported nothing. ",
					i, i*13)
			}
			body, err := json.Marshal(map[string]any{
				"model":      tg.model,
				"max_tokens": 1,
				"messages": []map[string]string{
					{"role": "user", "content": filler.String() + "\n\nReply with the single word: ok"},
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			tr := newGzipRequestTransport(http.DefaultTransport)
			gt, ok := tr.(*gzipRequestTransport)
			if !ok {
				t.Fatal("transport is disabled by OPENCODE_NO_REQUEST_GZIP; unset it to probe")
			}
			client := &http.Client{Transport: tr, Timeout: 90 * time.Second}

			url := tg.baseURL + "/chat/completions"
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			if tg.key != "" {
				req.Header.Set("Authorization", "Bearer "+tg.key)
			}

			start := time.Now()
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s: request failed: %v", tg.name, err)
			}
			defer resp.Body.Close()
			reply, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
			elapsed := time.Since(start).Round(time.Millisecond)

			packed, _ := gzipBytes(body)

			// Report the LEARNED state, not merely "we never marked it
			// rejected". The first version of this printed ACCEPTS GZIP for
			// NIM while NIM was in fact answering 500 to the gzipped body.
			verdict := map[gzipState]string{
				gzipAccepts: "ACCEPTS GZIP",
				gzipRejects: "REJECTS GZIP (fell back to raw, host remembered)",
				gzipUnknown: "INDETERMINATE (request failed by both routes)",
			}[gt.get(req.URL.Host)]

			t.Logf("%s [%s]\n  status   : %d in %s\n  body     : %d raw -> %d gzipped (%.1f%% saved)\n  verdict  : %s\n  reply    : %s",
				tg.name, req.URL.Host, resp.StatusCode, elapsed,
				len(body), len(packed), 100*(1-float64(len(packed))/float64(len(body))),
				verdict, strings.TrimSpace(string(reply)))

			// A 2xx after the transport is done — compressed or via
			// fallback — is a pass. The probe reports which; it does not
			// require gzip support, because a provider that refuses is a
			// fact about the provider, not a bug in us.
			if resp.StatusCode >= 400 {
				t.Errorf("%s: status %d — request did not succeed by either route", tg.name, resp.StatusCode)
			}
		})
	}
}
