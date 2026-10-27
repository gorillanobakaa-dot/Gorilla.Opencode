package provider

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

// tlsTimeout stands for net/http's unexported "net/http: TLS handshake timeout".
type tlsTimeout struct{}

func (tlsTimeout) Error() string   { return "net/http: TLS handshake timeout" }
func (tlsTimeout) Timeout() bool   { return true }
func (tlsTimeout) Temporary() bool { return true }

func withFakeDo(t *testing.T, do func(*http.Request) (*http.Response, error)) {
	prevDo, prevB := chatgptDo, chatgptConnectBackoff
	chatgptDo, chatgptConnectBackoff = do, []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { chatgptDo, chatgptConnectBackoff = prevDo, prevB })
}

func newTestReq() (*http.Request, error) {
	return http.NewRequest(http.MethodPost, "https://chatgpt.invalid/responses", nil)
}

// Job 07: one TLS handshake timeout on the phone hotspot ended the turn.
func TestAChatGPTRequestThatNeverConnectedIsSentAgain(t *testing.T) {
	calls := 0
	withFakeDo(t, func(*http.Request) (*http.Response, error) {
		calls++
		if calls < 3 {
			return nil, &net.OpError{Op: "dial", Err: errors.New("connectex: refused")}
		}
		return &http.Response{StatusCode: 200}, nil
	})
	resp, err := doWithConnectRetry(context.Background(), newTestReq)
	if err != nil || resp == nil || calls != 3 {
		t.Fatalf("calls=%d err=%v resp=%v, want a response on the third try", calls, err, resp)
	}
}

func TestATLSHandshakeTimeoutIsRetriedThenGivenUp(t *testing.T) {
	calls := 0
	withFakeDo(t, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, &urlErr{tlsTimeout{}}
	})
	_, err := doWithConnectRetry(context.Background(), newTestReq)
	if err == nil || calls != 3 {
		t.Fatalf("calls=%d err=%v, want 3 tries then the error", calls, err)
	}
}

// An error after the connection was made may mean the server got the request:
// it is never sent twice.
func TestAnErrorAfterConnectingIsNotRetried(t *testing.T) {
	calls := 0
	withFakeDo(t, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("read tcp: connection reset by peer")
	})
	if _, err := doWithConnectRetry(context.Background(), newTestReq); err == nil || calls != 1 {
		t.Fatalf("calls=%d err=%v, want one try", calls, err)
	}
}

// urlErr wraps like *url.Error does, so errors.As and the message both pass through.
type urlErr struct{ err error }

func (u *urlErr) Error() string { return `Post "https://chatgpt.invalid": ` + u.err.Error() }
func (u *urlErr) Unwrap() error { return u.err }
