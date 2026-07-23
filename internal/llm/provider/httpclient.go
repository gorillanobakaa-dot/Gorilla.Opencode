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
//     of hanging forever, but NOT a response-header timeout (first byte can be
//     legitimately slow on a big model + slow link).
//   - Proxy from environment: satellite terminals frequently front traffic
//     through a local caching/optimising proxy (HTTP_PROXY/HTTPS_PROXY).
package provider

import (
	"net"
	"net/http"
	"time"
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
		Proxy:       http.ProxyFromEnvironment,
		DialContext: dialer.DialContext,
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
		// Deliberately NO ResponseHeaderTimeout: a big model over a slow link
		// can take a long time to the first byte; we rely on context
		// cancellation and the stream-retry logic instead of a fixed cap.
	}

	return &http.Client{
		Transport: transport,
		// Deliberately NO Timeout — see the file comment.
	}
}
