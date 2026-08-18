// GORILLA OVERRIDE (2026-08-18): the half of a dead link that no timeout in the
// HTTP transport can see.
//
// config.FirstByteTimeout bounds the wait for response HEADERS. Once those
// headers arrive the transport considers its job done, and a stream that then
// stops dead — socket open, nothing more delivered — blocks on the next read
// forever. No header timeout can fire, because the headers already came.
//
// That is precisely what a satellite dropout looks like once the answer has
// begun, and it is what the "black hole" mode of the test proxy reproduces:
// the link is not closed and not reset, it simply stops carrying anything.
//
// The guard is a STALL timer, not a wall clock. Every chunk resets it. A stream
// crawling in at one token a second on a 2 KB/s uplink is never touched; only a
// stream delivering NOTHING for the whole window is. That distinction is the
// entire design: the cost of a false positive is destroying an answer the user
// has already paid for, both in money and in upload time.
//
// It arms on the FIRST chunk rather than at stream creation, so it means exactly
// one thing — "the answer started and then stopped" — and never races the
// first-byte timeout for the same failure.
package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrStreamStalled says the answer began and then stopped arriving.
type ErrStreamStalled struct {
	Idle time.Duration
	// Got is how much had already been streamed, because "stalled after 4,000
	// characters" and "stalled immediately" call for different responses.
	Got int
}

func (e *ErrStreamStalled) Error() string {
	partial := "nothing had arrived yet"
	if e.Got > 0 {
		partial = fmt.Sprintf("%d characters had arrived and are kept", e.Got)
	}
	return fmt.Sprintf(
		"the answer stopped arriving: nothing further for %s, though the connection "+
			"stayed open (%s). On a satellite or mobile link a dropout looks exactly "+
			"like this — the link is never closed, it just stops carrying anything. "+
			"Adjust or disable with GORILLA_OPENCODE_STREAM_STALL_TIMEOUT",
		e.Idle.Round(time.Second), partial)
}

// stallGuard cancels a stream context when no progress has been reported for
// the timeout. Progress and Stop are safe to call from any goroutine, and safe
// to call after the guard has already fired or been stopped.
type stallGuard struct {
	timeout time.Duration
	cancel  context.CancelFunc

	mu      sync.Mutex
	timer   *time.Timer
	stopped bool
	fired   bool
}

// newStallGuard returns a derived context and a guard that is NOT yet armed.
// Call Progress on the first chunk to arm it. A timeout of zero disables the
// guard entirely and returns the context unchanged, so someone who has switched
// this off never pays for a timer.
func newStallGuard(ctx context.Context, timeout time.Duration) (context.Context, *stallGuard) {
	if timeout <= 0 {
		return ctx, &stallGuard{}
	}
	sctx, cancel := context.WithCancel(ctx)
	return sctx, &stallGuard{timeout: timeout, cancel: cancel}
}

// Progress reports that a chunk arrived: arm the timer, or push it back.
func (g *stallGuard) Progress() {
	if g == nil || g.timeout <= 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped || g.fired {
		return
	}
	if g.timer == nil {
		g.timer = time.AfterFunc(g.timeout, g.fire)
		return
	}
	g.timer.Reset(g.timeout)
}

func (g *stallGuard) fire() {
	g.mu.Lock()
	if g.stopped || g.fired {
		g.mu.Unlock()
		return
	}
	g.fired = true
	g.mu.Unlock()
	g.cancel()
}

// Fired reports whether the guard, rather than the user or the server, ended
// the stream. The distinction matters: a cancelled turn is the user's choice
// and must stay silent, while a stall is a failure and must be explained.
func (g *stallGuard) Fired() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.fired
}

// Stop releases the timer and the derived context. Always defer it.
func (g *stallGuard) Stop() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.stopped = true
	t := g.timer
	g.timer = nil
	g.mu.Unlock()
	if t != nil {
		t.Stop()
	}
	if g.cancel != nil {
		g.cancel()
	}
}

// isFirstByteTimeout reports the specific case where the transport gave up
// waiting for response headers — the server took the connection and never
// replied.
//
// It is matched on Go's own wording because the transport returns a plain
// wrapped error with no distinct type to assert on. Narrow on purpose: this
// must not catch a dial timeout or a TLS timeout, both of which are ordinary
// link problems that deserve the full retry budget.
func isFirstByteTimeout(err error) bool {
	return err != nil && strings.Contains(err.Error(), "timeout awaiting response headers")
}
