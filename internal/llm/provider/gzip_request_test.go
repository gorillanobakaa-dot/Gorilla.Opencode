package provider

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// bigJSON returns a payload shaped like what we actually upload: a
// conversation of JSON messages with repeated keys but VARYING content.
//
// The first version of this repeated one identical sentence 400 times and
// duly reported 99.3% saved, which is the best case gzip will ever see and
// would have been a lie in any release note.
//
// Even varied, a synthetic payload flatters itself — this one logs ~90%.
// The honest figure for real traffic is ~77%, measured separately against
// this repo's own source rendered as a conversation. DO NOT quote the number
// this test logs. The assertion below is a deliberately loose 50%: a guard
// that compression happened at all, not a benchmark.
func bigJSON() []byte {
	var b bytes.Buffer
	b.WriteString(`{"model":"local","messages":[`)
	for i := 0; i < 400; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b,
			`{"role":"assistant","content":"rebuilt %s_%d.o at offset %d; %s"}`,
			[...]string{"kernel/sched/core", "arch/x86/mm/fault", "gfx/thebes/gfxFont", "dom/base/nsINode"}[i%4],
			i*7919, i*104729,
			[...]string{"linker reported an undefined symbol", "the test suite passed", "mach printed nothing at all", "ld.lld exited 1"}[i%4])
	}
	b.WriteString(`]}`)
	return b.Bytes()
}

func post(t *testing.T, c *http.Client, url string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// The happy path: a server that accepts gzip receives the bytes we meant to
// send, and receives materially fewer of them.
func TestRequestGzipCompressesAndRoundTrips(t *testing.T) {
	payload := bigJSON()
	var gotEncoding string
	var wireBytes int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		raw, _ := io.ReadAll(r.Body)
		atomic.StoreInt64(&wireBytes, int64(len(raw)))

		body := raw
		if gotEncoding == "gzip" {
			zr, err := gzip.NewReader(bytes.NewReader(raw))
			if err != nil {
				t.Errorf("server could not gunzip the body: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if body, err = io.ReadAll(zr); err != nil {
				t.Errorf("gunzip read failed: %v", err)
			}
		}
		if !bytes.Equal(body, payload) {
			t.Errorf("server received %d bytes after decoding, want %d — payload corrupted",
				len(body), len(payload))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &http.Client{Transport: newGzipRequestTransport(http.DefaultTransport)}
	resp := post(t, c, srv.URL, payload)
	defer resp.Body.Close()

	if gotEncoding != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip — the body went out raw", gotEncoding)
	}
	on := atomic.LoadInt64(&wireBytes)
	if on >= int64(len(payload))/2 {
		t.Errorf("compressed to %d bytes from %d; expected well under half", on, len(payload))
	}
	t.Logf("%d -> %d bytes on the wire (%.1f%% saved)",
		len(payload), on, 100*(1-float64(on)/float64(len(payload))))
}

// A server that rejects the encoding must get the request anyway, and must
// only cost us the probe once.
func TestRequestGzipFallsBackAndRemembers(t *testing.T) {
	payload := bigJSON()
	var attempts, gzipAttempts int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&attempts, 1)
		if r.Header.Get("Content-Encoding") == "gzip" {
			atomic.AddInt64(&gzipAttempts, 1)
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		if !bytes.Equal(raw, payload) {
			t.Errorf("fallback body differs from the original payload")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &http.Client{Transport: newGzipRequestTransport(http.DefaultTransport)}

	resp := post(t, c, srv.URL, payload)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first request status = %d, want 200 — fallback did not happen", resp.StatusCode)
	}
	if got := atomic.LoadInt64(&attempts); got != 2 {
		t.Fatalf("first request took %d attempts, want 2 (probe + fallback)", got)
	}

	for i := 0; i < 3; i++ {
		resp = post(t, c, srv.URL, payload)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("follow-up %d status = %d, want 200", i, resp.StatusCode)
		}
	}
	if got := atomic.LoadInt64(&gzipAttempts); got != 1 {
		t.Errorf("sent gzip %d times to a host known to reject it, want 1 — the probe is not being remembered", got)
	}
	if got := atomic.LoadInt64(&attempts); got != 5 {
		t.Errorf("total attempts = %d, want 5 (2 for the probe + 3 clean) — wasted round-trips", got)
	}
}

// A 400 that is NOT about the encoding must not cost us gzip for the rest of
// the session. This is the case that separates "server hates gzip" from
// "request was simply bad", and getting it wrong silently disables the
// feature after any ordinary API error.
func TestRequestGzipKeepsTryingAfterGenuineBadRequest(t *testing.T) {
	payload := bigJSON()
	var gzipAttempts int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			atomic.AddInt64(&gzipAttempts, 1)
		}
		// Always bad, encoded or not — e.g. an invalid model name.
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := &http.Client{Transport: newGzipRequestTransport(http.DefaultTransport)}
	for i := 0; i < 2; i++ {
		resp := post(t, c, srv.URL, payload)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 passed through", resp.StatusCode)
		}
	}
	if got := atomic.LoadInt64(&gzipAttempts); got != 2 {
		t.Errorf("gzip attempts = %d, want 2 — a genuine 400 wrongly disabled compression", got)
	}
}

// Small bodies must go out untouched: gzip's 18-byte header makes them
// bigger, and they are not the problem this exists to solve.
func TestRequestGzipSkipsSmallBodies(t *testing.T) {
	var gotEncoding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &http.Client{Transport: newGzipRequestTransport(http.DefaultTransport)}
	resp := post(t, c, srv.URL, []byte(`{"model":"local","messages":[]}`))
	resp.Body.Close()

	if gotEncoding != "" {
		t.Errorf("Content-Encoding = %q on a tiny body, want none", gotEncoding)
	}
}

// The opt-out must actually opt out.
func TestRequestGzipRespectsOptOut(t *testing.T) {
	t.Setenv("OPENCODE_NO_REQUEST_GZIP", "1")

	var gotEncoding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &http.Client{Transport: newGzipRequestTransport(http.DefaultTransport)}
	resp := post(t, c, srv.URL, bigJSON())
	resp.Body.Close()

	if gotEncoding != "" {
		t.Errorf("Content-Encoding = %q with OPENCODE_NO_REQUEST_GZIP=1, want none", gotEncoding)
	}
}

// GET and other bodyless requests must be left completely alone.
func TestRequestGzipIgnoresBodylessRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if enc := r.Header.Get("Content-Encoding"); enc != "" {
			t.Errorf("Content-Encoding = %q on a GET, want none", enc)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &http.Client{Transport: newGzipRequestTransport(http.DefaultTransport)}
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// NVIDIA NIM answers a gzipped body with 500 and "failed to decode json
// body: invalid character '\x1f'" — it feeds the compressed bytes to a JSON
// parser and reports the parser's complaint as a server fault. Found by the
// live probe, not by any test we had written. Without 500 in
// encodingRejected the fallback never fires and every NIM request dies.
func TestRequestGzipFallsBackOnNIMStyle500(t *testing.T) {
	payload := bigJSON()
	var gzipAttempts int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			atomic.AddInt64(&gzipAttempts, 1)
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"error":"failed to decode json body: invalid character '\x1f'"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &http.Client{Transport: newGzipRequestTransport(http.DefaultTransport)}
	for i := 0; i < 3; i++ {
		resp := post(t, c, srv.URL, payload)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 — no fallback on a NIM-style 500", i, resp.StatusCode)
		}
	}
	if got := atomic.LoadInt64(&gzipAttempts); got != 1 {
		t.Errorf("gzip attempts = %d, want 1 — the 500 rejection was not remembered", got)
	}
}

// Once a host has proven it accepts gzip, a later 500 is a real server error.
// Re-probing it would double the round-trip on every transient failure, on
// the link least able to afford it.
func TestRequestGzipDoesNotRetryOnceHostIsProven(t *testing.T) {
	payload := bigJSON()
	var total, gzipAttempts int64
	var failNow atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&total, 1)
		if r.Header.Get("Content-Encoding") == "gzip" {
			atomic.AddInt64(&gzipAttempts, 1)
		}
		if failNow.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &http.Client{Transport: newGzipRequestTransport(http.DefaultTransport)}

	resp := post(t, c, srv.URL, payload) // teaches the transport gzip works
	resp.Body.Close()

	failNow.Store(true)
	before := atomic.LoadInt64(&total)
	resp = post(t, c, srv.URL, payload)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want the 500 passed through", resp.StatusCode)
	}
	if got := atomic.LoadInt64(&total) - before; got != 1 {
		t.Errorf("a genuine 500 cost %d attempts on a proven host, want 1", got)
	}
	if got := atomic.LoadInt64(&gzipAttempts); got != 2 {
		t.Errorf("gzip attempts = %d, want 2 — compression was wrongly abandoned", got)
	}
}
