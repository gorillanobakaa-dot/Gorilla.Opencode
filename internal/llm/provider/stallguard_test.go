package provider

// GORILLA OVERRIDE (2026-08-18): tests for the stall guard.
//
// The failure these exist for was observed live, not imagined: NVIDIA NIM
// accepted a request for a model it still advertises in /v1/models and then
// returned nothing at all, indefinitely. A bare curl hung the same way, so the
// provider was at fault — but the client's response was to sit there silently.
//
// The property that actually matters is the NEGATIVE one: a slow stream must
// never be killed. Getting that wrong destroys an answer the user has already
// paid for in money and in upload time, which is worse than the hang.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// A stream making no progress is cut, and the context it was given is cancelled.
func TestASilentStreamIsCutAndReported(t *testing.T) {
	ctx, guard := newStallGuard(context.Background(), 60*time.Millisecond)
	defer guard.Stop()

	guard.Progress() // first chunk arrives, arming the guard
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the stream context was never cancelled; a silent link would hang forever")
	}
	if !guard.Fired() {
		t.Error("the guard did not report that IT ended the stream, so a failure would be " +
			"mistaken for the user cancelling and reported silently")
	}
}

// The one that matters most: slow is not stalled. A stream crawling in below the
// timeout must survive indefinitely.
func TestASlowStreamIsNeverCut(t *testing.T) {
	const window = 120 * time.Millisecond
	ctx, guard := newStallGuard(context.Background(), window)
	defer guard.Stop()

	// Twelve chunks at half the window: total elapsed is 5x the timeout, so a
	// wall-clock timer would have killed this four times over.
	for range 12 {
		guard.Progress()
		time.Sleep(window / 2)
		if ctx.Err() != nil {
			t.Fatalf("a stream making steady progress was cut after %v — the guard is "+
				"behaving as a wall clock, not a stall timer", ctx.Err())
		}
	}
	if guard.Fired() {
		t.Error("the guard fired on a healthy slow stream")
	}
}

// Until the first chunk arrives the guard must not be armed: waiting for the
// first byte belongs to the transport's header timeout, and two timers racing
// for one failure produce whichever message ran first.
func TestTheGuardIsNotArmedBeforeTheFirstChunk(t *testing.T) {
	ctx, guard := newStallGuard(context.Background(), 40*time.Millisecond)
	defer guard.Stop()

	time.Sleep(200 * time.Millisecond) // five windows, no Progress ever called
	if ctx.Err() != nil {
		t.Error("the guard fired before any chunk arrived; that case belongs to " +
			"config.FirstByteTimeout")
	}
	if guard.Fired() {
		t.Error("the guard reported firing before it was armed")
	}
}

// A zero timeout means the user switched this off. They must get the original
// context back, unwrapped, and no timer.
func TestZeroDisablesTheGuardEntirely(t *testing.T) {
	base := context.Background()
	ctx, guard := newStallGuard(base, 0)
	if ctx != base {
		t.Error("a disabled guard still wrapped the context")
	}
	guard.Progress()
	time.Sleep(50 * time.Millisecond)
	if guard.Fired() || ctx.Err() != nil {
		t.Error("a disabled guard still fired")
	}
	guard.Stop() // must not panic on a guard that has no cancel func
}

// Stop must be safe after firing, and Progress must be safe after Stop — the
// stream loop calls these from the read path while the timer runs elsewhere.
func TestStopAndProgressAreSafeInAnyOrder(t *testing.T) {
	_, guard := newStallGuard(context.Background(), 20*time.Millisecond)
	guard.Progress()
	time.Sleep(120 * time.Millisecond) // let it fire
	guard.Stop()
	guard.Stop()     // twice
	guard.Progress() // after stop
	if !guard.Fired() {
		t.Error("Stop erased the record that the guard had fired")
	}
}

// The user-facing sentence has to distinguish "stalled with nothing" from
// "stalled having delivered something", because the second case keeps text the
// user can still use and the first does not.
func TestTheStallErrorSaysWhetherAnythingSurvived(t *testing.T) {
	empty := (&ErrStreamStalled{Idle: 90 * time.Second}).Error()
	if !strings.Contains(empty, "nothing had arrived") {
		t.Errorf("an empty stall does not say so: %s", empty)
	}
	partial := (&ErrStreamStalled{Idle: 90 * time.Second, Got: 412}).Error()
	if !strings.Contains(partial, "412 characters had arrived and are kept") {
		t.Errorf("a partial stall does not say what survived: %s", partial)
	}
	// Go renders 90 seconds as "1m30s", which is what the user will read.
	for _, want := range []string{"1m30s", "GORILLA_OPENCODE_STREAM_STALL_TIMEOUT"} {
		if !strings.Contains(partial, want) {
			t.Errorf("the error omits %q: %s", want, partial)
		}
	}
}

// A stall must never be mistaken for a cancellation. agent.go silences
// context.Canceled deliberately, so if the stall error wrapped it the failure
// would vanish — which is the bug the translation in openai.go exists to stop.
func TestAStallIsNotACancellation(t *testing.T) {
	var err error = &ErrStreamStalled{Idle: time.Second}
	if errors.Is(err, context.Canceled) {
		t.Fatal("the stall error is indistinguishable from a user cancellation; " +
			"the failure would be swallowed and reported as nothing at all")
	}
	var stall *ErrStreamStalled
	if !errors.As(err, &stall) {
		t.Fatal("the stall error is not recoverable with errors.As")
	}
}

// A first-byte timeout must be recognised, and NOT confused with the ordinary
// link failures that legitimately deserve the whole retry budget. Getting this
// too wide would cut a real satellite dropout down to one retry.
func TestOnlyAHeaderTimeoutCountsAsAFirstByteTimeout(t *testing.T) {
	yes := []string{
		// The string actually observed on 2026-08-18 against the black-holed
		// model. Note the prefix is "http2:", not "net/http:" — the connection
		// had negotiated HTTP/2, which ForceAttemptHTTP2 makes the normal case
		// here. Matching on the "net/http:" form alone would have missed every
		// real occurrence.
		`Post "https://integrate.api.nvidia.com/v1/chat/completions": http2: timeout awaiting response headers`,
		`Post "https://example.invalid/v1/chat/completions": net/http: timeout awaiting response headers`,
	}
	no := []string{
		`dial tcp 1.2.3.4:443: i/o timeout`,
		`net/http: TLS handshake timeout`,
		`read tcp 1.2.3.4:443: connection reset by peer`,
		`unexpected EOF`,
		``,
	}
	for _, m := range yes {
		if !isFirstByteTimeout(errors.New(m)) {
			t.Errorf("not recognised as a first-byte timeout: %q", m)
		}
	}
	for _, m := range no {
		if isFirstByteTimeout(errors.New(m)) {
			t.Errorf("wrongly treated as a first-byte timeout, which would cut its "+
				"retries from five to one on a genuinely flaky link: %q", m)
		}
	}
	if isFirstByteTimeout(nil) {
		t.Error("a nil error was treated as a first-byte timeout")
	}
}
