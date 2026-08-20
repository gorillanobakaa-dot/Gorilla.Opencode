package provider

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

type fakeConn struct {
	net.Conn
	payload []byte
	off     int
}

func (c *fakeConn) Read(p []byte) (int, error) {
	if c.off >= len(c.payload) {
		return 0, nil
	}
	n := copy(p, c.payload[c.off:])
	c.off += n
	return n, nil
}
func (c *fakeConn) Close() error { return nil }

// The count must happen at the socket, so it sees WIRE bytes rather than the
// inflated body a RoundTripper would see above Go's response decompression.
func TestSocketCountingSeesWireBytes(t *testing.T) {
	before := wireBytesIn.Load() // delta, so no reset needed here
	dial := countingDialContext(func(context.Context, string, string) (net.Conn, error) {
		return &fakeConn{payload: make([]byte, 8*1024)}, nil
	})
	c, err := dial(context.Background(), "tcp", "example.invalid:443")
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	for i := 0; i < 2; i++ {
		if _, err := c.Read(buf); err != nil {
			t.Fatal(err)
		}
	}
	if got := wireBytesIn.Load() - before; got != 8*1024 {
		t.Errorf("counted %d wire bytes, want 8192", got)
	}
}

// Overlapping requests make the wire delta unattributable; sampling then would
// over-report speed, which is the direction that causes failed turns.
func TestOverlappingRequestsAreNotSampled(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Reset FIRST. The samples live in a package variable, so without this the
	// assertion below reads a sample left by whichever test ran before — which
	// is exactly how this failed once the reachability test was added. Third
	// instance of shared-state pollution in this session; the state is global,
	// so every test that reads it must start from a known point.
	resetLinkStateForTest()
	a := beginLinkSample()
	b := beginLinkSample() // overlaps a
	wireBytesIn.Add(64 * 1024)
	time.Sleep(600 * time.Millisecond)
	a.end()
	b.end()
	if _, ok := config.EstimatedKBps(); ok {
		t.Error("sampled an overlapping pair; the delta is not attributable to one response")
	}
}
