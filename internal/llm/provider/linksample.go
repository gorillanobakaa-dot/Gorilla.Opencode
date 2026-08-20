// GORILLA OVERRIDE: this file did not exist upstream. It feeds the passive
// link-speed estimate the connection profile picker reads.
//
// Nothing here initiates a transfer. It times transfers that were going to
// happen anyway, so the picker can suggest a profile without spending a byte to
// measure the link. Why there is no speed test: internal/config/linkspeed.go.
//
// WHY THE COUNT HAPPENS AT THE SOCKET, NOT IN A RoundTripper.
//
// The first version wrapped the response body in a RoundTripper and counted
// what it read. That is wrong in the same way FOOTPRINT.md already records the
// upload budget being wrong: http.Transport negotiates gzip for responses and
// decompresses them transparently, so ANY RoundTripper sits ABOVE the
// decompression and sees the inflated body. Provider responses compress heavily
// — the same 75-85% that gzip_request.go measures on the way up — so counting
// there would have reported the link as several times faster than it is.
//
// That error fails in the dangerous direction. Over-reporting speed recommends
// a FASTER profile, which means shorter timeouts and a smaller upload budget on
// a link that cannot take them: turns start failing, and the screen that was
// supposed to prevent that caused it.
//
// Counting the socket is immune. Bytes on the wire are bytes on the wire —
// after compression, after TLS, after HTTP/2 framing — which is also exactly
// what the user pays their provider for.
package provider

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

// wireBytesIn counts every byte read off every socket this client owns.
var wireBytesIn atomic.Int64

// inFlight is how many response bodies are open right now. A sample is only
// taken when it is exactly one: with two overlapping requests the wire delta
// contains both, which would over-report throughput — again the dangerous
// direction. Skipping the ambiguous case costs nothing, because an agent turn
// makes many sequential requests and only needs one clean sample.
var inFlight atomic.Int32

// countingConn counts bytes read from the socket.
type countingConn struct{ net.Conn }

func (c *countingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		wireBytesIn.Add(int64(n))
	}
	return n, err
}

// countingDialContext wraps a dialer so every connection counts itself.
func countingDialContext(base func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := base(ctx, network, addr)
		if err != nil {
			return c, err
		}
		return &countingConn{Conn: c}, nil
	}
}

// linkSampleSpan measures one response: wall time from the moment the body is
// available to the moment it is closed, against the wire bytes that arrived in
// the same window.
type linkSampleSpan struct {
	start     time.Time
	startWire int64
	alone     bool
	done      bool
}

func beginLinkSample() *linkSampleSpan {
	s := &linkSampleSpan{start: time.Now(), startWire: wireBytesIn.Load()}
	s.alone = inFlight.Add(1) == 1
	return s
}

// end reports the sample. A body abandoned early — a cancelled turn, the stall
// guard firing — still reports what arrived: those bytes really did take that
// long, which is a genuine observation of the link.
func (s *linkSampleSpan) end() {
	if s == nil || s.done {
		return
	}
	s.done = true
	remaining := inFlight.Add(-1)
	if !s.alone || remaining != 0 {
		// Another request overlapped this one at some point; the delta is not
		// attributable to a single response.
		return
	}
	config.RecordTransfer(wireBytesIn.Load()-s.startWire, time.Since(s.start))
}
