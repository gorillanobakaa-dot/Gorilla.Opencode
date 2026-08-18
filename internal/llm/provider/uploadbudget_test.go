package provider

// GORILLA OVERRIDE (2026-08-18): tests for the one-ceiling-that-is-really-the-
// ceiling.
//
// The bug these exist for was measured, not imagined: a link that reset every
// eight seconds produced 14 attempts and 1.01 MB of upload for one question,
// against a declared maxRetries of 5, because two layers were counting
// separately. So the test that matters simulates exactly that — a transport
// that always fails, retried by something that does not know about the budget —
// and asserts the bytes stop.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// alwaysFails is the dropping link: every attempt dies before a response.
type alwaysFails struct{ calls int }

func (t *alwaysFails) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls++
	return nil, errors.New("connection reset by peer")
}

func req(t *testing.T, body string) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodPost, "https://example.invalid/v1/chat", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.ContentLength = int64(len(body))
	return r
}

// The headline: retries stop once the turn has spent its bytes, no matter who
// is doing the retrying.
func TestRetriesStopWhenTheTurnHasSpentItsBytes(t *testing.T) {
	base := &alwaysFails{}
	rt := newBudgetTransport(base)

	const body = 100 * 1024 // 100 KB, about one real conversation
	budget := NewUploadBudget(1024 * 1024)
	ctx := WithUploadBudget(context.Background(), budget)

	var lastErr error
	// Retry far more times than the budget allows — this stands in for the two
	// layers that between them managed 14 attempts.
	for range 100 {
		r := req(t, strings.Repeat("x", body)).WithContext(ctx)
		if _, err := rt.RoundTrip(r); err != nil {
			lastErr = err
			var over *ErrUploadBudget
			if errors.As(err, &over) {
				break
			}
		}
	}

	var over *ErrUploadBudget
	if !errors.As(lastErr, &over) {
		t.Fatalf("retries were never stopped; last error was %v", lastErr)
	}
	if base.calls > 11 {
		t.Errorf("%d attempts reached the network for a 1 MB budget at 100 KB each; "+
			"the budget is not bounding what goes on the wire", base.calls)
	}
	if budget.Spent() > 1024*1024 {
		t.Errorf("spent %d bytes against a 1,048,576 byte budget — it overshot", budget.Spent())
	}
	// The error has to be worth reading on a link where this matters.
	for _, want := range []string{"attempts", "uploaded", "re-uploads the whole conversation"} {
		if !strings.Contains(over.Error(), want) {
			t.Errorf("the error does not mention %q: %s", want, over)
		}
	}
}

// The budget refuses BEFORE sending. Checking afterwards would mean the bytes
// were already on the link, which is the entire thing being prevented.
func TestTheLastAttemptIsRefusedBeforeItIsSent(t *testing.T) {
	base := &alwaysFails{}
	rt := newBudgetTransport(base)
	ctx := WithUploadBudget(context.Background(), NewUploadBudget(150*1024))

	// First 100 KB fits.
	if _, err := rt.RoundTrip(req(t, strings.Repeat("x", 100*1024)).WithContext(ctx)); err == nil {
		t.Fatal("expected the transport error through")
	}
	if base.calls != 1 {
		t.Fatalf("first request did not reach the network (calls=%d)", base.calls)
	}

	// Second would take it to 200 KB against a 150 KB budget.
	_, err := rt.RoundTrip(req(t, strings.Repeat("x", 100*1024)).WithContext(ctx))
	var over *ErrUploadBudget
	if !errors.As(err, &over) {
		t.Fatalf("second request was not refused: %v", err)
	}
	if base.calls != 1 {
		t.Errorf("the over-budget request was still sent (calls=%d) — the bytes went on the link", base.calls)
	}
}

// No budget in the context, or a zero limit, must change nothing. Someone on
// fibre should never meet this code.
func TestWithoutABudgetNothingIsRefused(t *testing.T) {
	for _, c := range []struct {
		name string
		ctx  context.Context
	}{
		{"no budget attached", context.Background()},
		{"unlimited budget", WithUploadBudget(context.Background(), NewUploadBudget(0))},
	} {
		base := &alwaysFails{}
		rt := newBudgetTransport(base)
		for range 20 {
			r := req(t, strings.Repeat("x", 200*1024)).WithContext(c.ctx)
			if _, err := rt.RoundTrip(r); err != nil {
				var over *ErrUploadBudget
				if errors.As(err, &over) {
					t.Fatalf("%s: refused a request when it should not have", c.name)
				}
			}
		}
		if base.calls != 20 {
			t.Errorf("%s: %d of 20 attempts reached the network", c.name, base.calls)
		}
	}
}

// A body of unknown length must never be refused: not knowing the size is not a
// reason to block the request, and guessing high would break streaming uploads.
func TestUnknownLengthBodiesAreNotCharged(t *testing.T) {
	base := &alwaysFails{}
	rt := newBudgetTransport(base)
	budget := NewUploadBudget(1024)
	ctx := WithUploadBudget(context.Background(), budget)

	r, _ := http.NewRequest(http.MethodPost, "https://example.invalid/x",
		io.NopCloser(bytes.NewReader(make([]byte, 4096))))
	r.ContentLength = -1 // unknown
	r = r.WithContext(ctx)

	if _, err := rt.RoundTrip(r); err != nil {
		var over *ErrUploadBudget
		if errors.As(err, &over) {
			t.Fatal("a body of unknown length was refused")
		}
	}
	if base.calls != 1 {
		t.Error("the request never reached the network")
	}
	if budget.Spent() != 0 {
		t.Errorf("an unknown-length body was charged %d bytes", budget.Spent())
	}
}

// The budget measures the WIRE, so it must sit INSIDE the gzip wrapper — a
// RoundTripper sees the request before whatever it wraps. The first version had
// this backwards, with a comment confidently asserting the opposite.
func TestTheBudgetMeasuresCompressedWireBytes(t *testing.T) {
	base := &alwaysFails{}
	// Same nesting as resilientHTTPClient: gzip outer, budget inner, so the
	// budget sees the compressed body.
	rt := newGzipRequestTransport(newBudgetTransport(base))

	budget := NewUploadBudget(10 * 1024 * 1024)
	ctx := WithUploadBudget(context.Background(), budget)

	// Highly compressible, and well over gzipMinRequestBytes.
	body := strings.Repeat("the same sentence over and over. ", 4000)
	r := req(t, body).WithContext(ctx)
	_, _ = rt.RoundTrip(r)

	if budget.Spent() == 0 {
		t.Fatal("nothing was charged at all")
	}
	if budget.Spent() >= int64(len(body)) {
		t.Errorf("charged %d bytes for a %d byte body that compresses ~20x — "+
			"the budget is measuring the uncompressed body, not the wire",
			budget.Spent(), len(body))
	}
	fmt.Printf("  (charged %d wire bytes for a %d byte body)\n", budget.Spent(), len(body))
}
