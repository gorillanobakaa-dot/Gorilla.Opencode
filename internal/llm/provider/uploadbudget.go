// GORILLA OVERRIDE (2026-08-18): one retry ceiling that is actually the
// ceiling, counted in the unit the user pays in.
//
// WHAT WAS MEASURED. A CONNECT proxy reset the connection eight seconds into
// every attempt — a satellite link dropping, which is the normal case for the
// people this is built for. The client made **14 attempts and uploaded 1.01 MB**
// for one question, produced no answer, and showed no error.
//
// provider.go declares maxRetries = 5.
//
// The extra attempts came from TWO layers retrying independently, neither aware
// of the other:
//
//   - the application loop in openai.go, which counts to maxRetries; and
//   - Go's http.Transport, which silently re-sends a request when the
//     connection dies before any response byte arrives. It is allowed to do
//     that here because gzip_request.go sets GetBody — which is exactly what
//     makes a body replayable.
//
// So the real ceiling was their PRODUCT, not the number written down. That is
// the general trap: a declared retry limit is only real when exactly one thing
// is counting.
//
// WHY THE BUDGET IS IN BYTES. Counting attempts is a proxy, and this project
// has been bitten by proxies before — the grep tool capped MATCHES to protect
// BYTES and returned 2.4 MB. What a retry actually costs is the whole
// conversation re-uploaded. On a metered satellite plan that is money, and on a
// 2 KB/s link it is minutes. So the limit is expressed in bytes, enforced at
// the one place every attempt must pass through whoever initiated it, and the
// error says what it spent.
package provider

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/opencode-ai/opencode/internal/config"
)

// uploadBudgetKey scopes a budget to one turn.
type uploadBudgetKeyT struct{}

var uploadBudgetKey uploadBudgetKeyT

// UploadBudget tracks what a single turn has pushed up the link.
type UploadBudget struct {
	limit    int64
	spent    atomic.Int64
	attempts atomic.Int32
}

// NewUploadBudget returns a budget of limit bytes. limit <= 0 means unlimited,
// which is the right answer on a desk with fibre and the wrong one everywhere
// this program is aimed.
func NewUploadBudget(limit int64) *UploadBudget { return &UploadBudget{limit: limit} }

// Spent and Attempts report what the turn has cost so far.
func (b *UploadBudget) Spent() int64    { return b.spent.Load() }
func (b *UploadBudget) Attempts() int32 { return b.attempts.Load() }

// WithUploadBudget attaches a budget to a turn's context. Every request the
// turn makes — including retries the HTTP transport starts on its own — draws
// from it, because they all carry this context.
func WithUploadBudget(ctx context.Context, b *UploadBudget) context.Context {
	return context.WithValue(ctx, uploadBudgetKey, b)
}

func budgetFrom(ctx context.Context) *UploadBudget {
	b, _ := ctx.Value(uploadBudgetKey).(*UploadBudget)
	return b
}

// ErrUploadBudget is returned when a turn has re-uploaded more than it is
// allowed to. It is deliberately explicit about the cost: on a link where this
// matters, "request failed" is not enough information to decide what to do next.
type ErrUploadBudget struct {
	Spent, Limit int64
	Attempts     int32
}

func (e *ErrUploadBudget) Error() string {
	return fmt.Sprintf(
		"stopped after %d attempts: this turn had already uploaded %s of the %s it is "+
			"allowed, and the connection kept failing. Nothing further was sent. "+
			"On a slow or metered link each retry re-uploads the whole conversation, so "+
			"retrying forever costs real money for no answer. Check the connection and "+
			"try again, or raise the limit if the link is fine",
		e.Attempts, humanBytes(e.Spent), humanBytes(e.Limit))
}

// ErrTurnTooLarge is returned when the FIRST attempt of a turn is already
// bigger than the whole budget. That is a different failure from ErrUploadBudget
// and needs a different remedy, which is why it is a different error.
//
// GORILLA OVERRIDE (2026-08-20): found while adding connection profiles. The
// budget check correctly refused before sending, but reported every refusal as
// "the connection kept failing" — so someone on the Austere profile whose
// conversation had simply grown too long was told to check a link that was
// working perfectly. Nothing failed; the message did not fit. Telling someone to
// debug their satellite dish when the fix is "start a new conversation" is the
// silent-failure class this subsystem exists to remove, wearing a helpful face.
type ErrTurnTooLarge struct {
	Size, Limit int64
	Profile     string
}

func (e *ErrTurnTooLarge) Error() string {
	return fmt.Sprintf(
		"nothing was sent: this message would upload %s, but the %q connection profile "+
			"allows %s per message. The connection is fine - the conversation has simply "+
			"grown too big for this profile. Every message re-uploads the whole "+
			"conversation, so this will not get smaller on its own. Start a new "+
			"conversation, or pick a faster connection profile",
		humanBytes(e.Size), e.Profile, humanBytes(e.Limit))
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

// budgetTransport is the single choke point. Every attempt passes through here
// — the application's retries and the transport's own — so this is the only
// place that can count them all.
//
// It sits INSIDE the gzip wrapper so it measures the bytes that actually go on
// the wire, compressed if compression happened — the user pays for wire bytes,
// not for what the body would have been. A RoundTripper sees the request before
// whatever it wraps, so "measures the wire" means innermost, not outermost.
type budgetTransport struct{ base http.RoundTripper }

func newBudgetTransport(base http.RoundTripper) http.RoundTripper {
	return &budgetTransport{base: base}
}

func (t *budgetTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	b := budgetFrom(req.Context())
	if b == nil || b.limit <= 0 {
		return t.base.RoundTrip(req)
	}

	// ContentLength is set by the time a body reaches the transport; a
	// streaming body of unknown length reports -1 and is not charged, which is
	// the safe direction — never refuse a request because its size is unknown.
	size := req.ContentLength
	if size < 0 {
		size = 0
	}

	// Refuse BEFORE sending. The whole point is not to put the bytes on the
	// link, so the check cannot happen afterwards.
	// A first attempt that cannot fit at all is not a retry problem, and saying
	// so is the difference between "start a new conversation" and a pointless
	// hunt for a network fault.
	if b.attempts.Load() == 0 && size > b.limit {
		return nil, &ErrTurnTooLarge{Size: size, Limit: b.limit, Profile: config.CurrentConnProfile().Name}
	}

	if b.spent.Load()+size > b.limit {
		return nil, &ErrUploadBudget{
			Spent:    b.spent.Load(),
			Limit:    b.limit,
			Attempts: b.attempts.Load(),
		}
	}

	b.spent.Add(size)
	b.attempts.Add(1)
	return t.base.RoundTrip(req)
}
