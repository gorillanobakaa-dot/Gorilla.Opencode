// GORILLA OVERRIDE: this file did not exist upstream. It provides one shared,
// hardened *http.Client tuned for the mission profile: laptops and phones in
// remote/hostile environments whose only uplink is a satellite phone link —
// high round-trip latency, frequent drops, and single-digit-KB/s bandwidth.
//
// The upstream providers used the SDK's default client (no connection tuning),
// and one path (copilot) even hard-coded a 30s wall-clock Timeout that would
// abort a slow big-model stream outright. On a satellite link both are wrong.
//
// Design choices, and why each matters on a satellite uplink:
//
//   - NO client.Timeout. A streaming answer is long-lived; a wall-clock deadline
//     would kill a legitimate slow reply (a 550B model's first token can take
//     tens of seconds over satellite). Cancellation is handled per-request via
//     context (user ESC / turn cancel), not a blunt timer.
//   - Keep-Alive + generous IdleConnTimeout so the EXPENSIVE TLS handshake is
//     paid once and the warm connection is reused across the whole agent tool
//     loop. (This only works because we now Close() streams — see openai.go.)
//   - ForceAttemptHTTP2: multiplex over a single connection instead of opening
//     new ones — far more frugal with the tiny bandwidth budget.
//   - Finite dial / TLS-handshake timeouts so a dead link fails in ~30s instead
//     of hanging forever.
//   - A response-header timeout AFTER ALL. The original line here read "but NOT
//     a response-header timeout (first byte can be legitimately slow on a big
//     model + slow link)". It is kept in this comment because it was wrong in an
//     instructive way: Go starts that timer only after the request body is fully
//     written, so a slow uplink never counts against it. See
//     config.FirstByteTimeout for what was measured.
//   - Proxy from environment: satellite terminals frequently front traffic
//     through a local caching/optimising proxy (HTTP_PROXY/HTTPS_PROXY).
package provider

import (
	"net"
	"net/http"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// resilientHTTPClient returns an *http.Client tuned for high-latency, lossy,
// low-bandwidth links. Safe to share across requests and goroutines.
func resilientHTTPClient() *http.Client {
	dialer := &net.Dialer{
		// TCP connect budget — generous for satellite round-trip times.
		Timeout: 30 * time.Second,
		// Keep the socket (and thus the pricey TLS session) warm between the
		// many sequential requests an agent turn makes.
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		// GORILLA OVERRIDE: count bytes at the socket for the passive link-speed
		// estimate. This is the only layer that sees WIRE bytes — a RoundTripper
		// sits above Go's transparent response decompression and would report a
		// heavily-compressed stream as a much faster link. See linksample.go.
		DialContext: countingDialContext(dialer.DialContext),
		// Prefer HTTP/2 so all traffic to one provider multiplexes over a
		// single connection — cheaper on a constrained uplink.
		ForceAttemptHTTP2: true,
		// Reuse connections aggressively; re-handshaking on every request is
		// prohibitively expensive when RTT is high.
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
		// Bound the wait for response HEADERS. This does NOT bound the answer:
		// Go starts the clock only once the request body is fully uploaded, and
		// stops it as soon as headers arrive, so neither a slow uplink nor a
		// long stream is affected. It bounds exactly one thing — a server that
		// accepted the connection and will never reply. Measured live: NVIDIA
		// NIM does that for models it still lists in /v1/models.
		ResponseHeaderTimeout: config.FirstByteTimeout(),
	}

	return &http.Client{
		// GORILLA OVERRIDE: two wrappers, and the ORDER is load-bearing.
		//
		// gzip is OUTER, budget is INNER. A RoundTripper sees the request
		// before the one it wraps, so the outer wrapper gets the original body
		// and the inner one gets whatever the outer produced. The budget must
		// therefore be innermost, or it charges for the uncompressed body
		// rather than the bytes that actually travel — and the user pays for
		// wire bytes.
		//
		// The first version had these the other way round, with a comment
		// confidently claiming the opposite. TestTheBudgetMeasuresCompressed-
		// WireBytes caught it: 132,000 bytes charged for a body that
		// compresses about twenty times.
		//
		// gzip_request.go: Go compresses responses for free but never
		// requests, and the uploaded conversation is the largest thing we send.
		// uploadbudget.go: one retry ceiling that is actually the ceiling.
		Transport: newGzipRequestTransport(newBudgetTransport(transport)),
		// Deliberately NO Timeout — see the file comment.
	}
}
