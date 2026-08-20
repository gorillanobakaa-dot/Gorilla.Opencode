package provider

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// THE TEST THAT WAS MISSING. The passive measurement shipped dead in v0.1.108
// and v0.1.109: beginLinkSample was defined and never called, because the
// RoundTripper that called it was deleted while moving byte-counting down to the
// socket. Nothing caught it — an unused unexported function compiles, go vet is
// silent, and the existing tests called beginLinkSample DIRECTLY, so they proved
// the unit worked while proving nothing about whether anything reaches it.
//
// So this test drives a REAL request through a REAL server via the transport
// chain and asserts a sample landed. It deliberately does not touch
// beginLinkSample by name.
func TestARealRequestRecordsASample(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	resetLinkStateForTest()

	// Enough bytes and enough elapsed time to clear the sample gate (4 KB /
	// 500ms), written slowly so the duration is real rather than instant.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := strings.Repeat("x", 2048)
		for i := 0; i < 4; i++ {
			_, _ = io.WriteString(w, chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(200 * time.Millisecond)
		}
	}))
	defer srv.Close()

	inner := func() http.RoundTripper {
		return newGzipRequestTransport(newBudgetTransport(&http.Transport{
			DialContext: countingDialContext((&net.Dialer{}).DialContext),
		}))
	}

	// NEGATIVE CONTROL FIRST. Exactly the chain as it shipped in v0.1.108 and
	// v0.1.109 — socket counting present, bracketing absent. If this records a
	// sample, the positive half below proves nothing.
	unwired := &http.Client{Transport: inner()}
	if r, err := unwired.Get(srv.URL); err == nil {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
	}
	if _, ok := config.EstimatedKBps(); ok {
		t.Fatal("negative control recorded a sample without the bracketing " +
			"transport; this test cannot detect the bug it exists for")
	}

	client := &http.Client{Transport: newLinkSampleTransport(inner())}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("draining body: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("closing body: %v", err)
	}

	kbps, ok := config.EstimatedKBps()
	if !ok {
		t.Fatal("a real request through the transport recorded NO sample — " +
			"the passive measurement is not wired in")
	}
	t.Logf("recorded %.1f KB/s from a real request", kbps)
}

func resetLinkStateForTest() {
	config.ResetLinkSamplesForTest()
	wireBytesIn.Store(0)
	inFlight.Store(0)
}
