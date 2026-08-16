// GORILLA OVERRIDE: this file did not exist upstream. It gzips OUTGOING
// request bodies.
//
// Go's http.Transport negotiates gzip for responses automatically and
// transparently decompresses them. It never compresses what you send. So on
// every turn the entire conversation — system prompt, every prior message,
// every tool result — is uploaded as raw JSON, and JSON is extremely
// redundant (repeated keys, whitespace, ASCII). Measured on real payloads
// this compresses by roughly 75-85%.
//
// On the mission profile this fork targets (satellite uplink, single-digit
// KB/s) the UPLINK is usually the scarcer direction, and it is the only bulk
// traffic still going out uncompressed — HTTP/2 already compresses headers
// via HPACK, since httpclient.go forces it.
//
// Not every provider accepts a gzipped request body, and there is no way to
// ask in advance. So this probes: it compresses, and if the server rejects
// the encoding it retries raw and remembers that host, at a cost of exactly
// one wasted round-trip per host per process. A host is only marked
// unsupported when the uncompressed retry SUCCEEDS — otherwise the failure
// was a genuine bad request and has nothing to do with the encoding, and
// giving up on gzip for the rest of the session would be the wrong lesson.
//
// Opt out entirely with GORILLA_OPENCODE_NO_REQUEST_GZIP=1.
package provider

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
)

// Bodies below this compress to something larger than they started, because
// the gzip header alone is 18 bytes. A short body is also not the problem
// this file exists to solve.
const gzipMinRequestBytes = 1024

// What we know about one host. Three states, not two, because the cost of
// probing has to be bounded: once a host has demonstrably accepted a gzipped
// body, a later error is a real error and must NOT trigger another
// retry-uncompressed. Otherwise every transient 500 would cost a doubled
// round-trip forever, on the link least able to afford it.
type gzipState int

const (
	gzipUnknown gzipState = iota // not yet probed — compress, retry on doubt
	gzipAccepts                  // proven to work — compress, never retry
	gzipRejects                  // proven to fail — never compress
)

type gzipRequestTransport struct {
	base http.RoundTripper

	mu    sync.RWMutex
	state map[string]gzipState
}

func newGzipRequestTransport(base http.RoundTripper) http.RoundTripper {
	if off, _ := strconv.ParseBool(os.Getenv("GORILLA_OPENCODE_NO_REQUEST_GZIP")); off {
		return base
	}
	return &gzipRequestTransport{base: base, state: map[string]gzipState{}}
}

func (t *gzipRequestTransport) get(host string) gzipState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state[host]
}

func (t *gzipRequestTransport) set(host string, s gzipState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state[host] = s
}

func (t *gzipRequestTransport) rejects(host string) bool {
	return t.get(host) == gzipRejects
}

// withBody returns a copy of req carrying b. The RoundTripper contract
// forbids mutating the request we were handed, and we may need to send it
// twice.
func withBody(req *http.Request, b []byte, encoded bool) *http.Request {
	r := req.Clone(req.Context())
	r.Body = io.NopCloser(bytes.NewReader(b))
	r.ContentLength = int64(len(b))
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(b)), nil
	}
	if encoded {
		r.Header.Set("Content-Encoding", "gzip")
	} else {
		r.Header.Del("Content-Encoding")
	}
	return r
}

func gzipBytes(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodingRejected reports whether a status plausibly means "I do not accept
// a gzipped body".
//
// 415 is the correct answer and almost nobody gives it. 400 is what several
// APIs return when they fail to parse what arrived. And 500 is in this list
// because of a measured case, not a hypothetical: NVIDIA NIM answers a
// gzipped body with
//
//	500  failed to decode json body: invalid character '\x1f' looking for
//	     beginning of value
//
// — 0x1f being the first byte of the gzip magic number. It is handing the
// compressed bytes straight to a JSON parser and reporting the parser's
// distress as a server fault. Without 500 here, the fallback never fires and
// every NIM request dies. This is exactly what the live probe was for; no
// amount of testing against our own httptest server would have found it.
func encodingRejected(code int) bool {
	return code == http.StatusUnsupportedMediaType ||
		code == http.StatusBadRequest ||
		code == http.StatusUnprocessableEntity ||
		code == http.StatusInternalServerError
}

func (t *gzipRequestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil || req.Body == http.NoBody ||
		req.Header.Get("Content-Encoding") != "" ||
		t.rejects(req.URL.Host) {
		return t.base.RoundTrip(req)
	}

	raw, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}

	if len(raw) < gzipMinRequestBytes {
		return t.base.RoundTrip(withBody(req, raw, false))
	}

	packed, err := gzipBytes(raw)
	if err != nil || len(packed) >= len(raw) {
		return t.base.RoundTrip(withBody(req, raw, false))
	}

	host := req.URL.Host
	known := t.get(host)

	resp, err := t.base.RoundTrip(withBody(req, packed, true))
	if err != nil {
		// A transport failure says nothing about whether the server
		// understands gzip. Report it and keep the optimistic default.
		return nil, err
	}
	if resp.StatusCode < 400 {
		if known == gzipUnknown {
			t.set(host, gzipAccepts)
		}
		return resp, nil
	}
	// This host has already proven it understands gzip, so an error now is a
	// real error. Do not re-probe; the doubled round-trip would be pure loss.
	if known == gzipAccepts || !encodingRejected(resp.StatusCode) {
		return resp, nil
	}

	// Might be the encoding, might be a genuinely bad request. Find out by
	// sending the identical payload uncompressed.
	resp.Body.Close()
	plain, err := t.base.RoundTrip(withBody(req, raw, false))
	if err != nil {
		return nil, err
	}
	if plain.StatusCode < 400 {
		// The only difference was the encoding. Stop paying for the probe.
		t.set(host, gzipRejects)
	}
	return plain, nil
}
