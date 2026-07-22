// GORILLA OVERRIDE: guards the transient-stream-error classifier that lets the
// OpenAI/NIM streaming path retry a dropped SSE connection (before first token)
// instead of failing the whole turn — the "SSE existential crisis" on slow,
// big models.
package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
)

func TestIsTransientStreamError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"clean EOF is success, not transient", io.EOF, false},
		{"unexpected EOF (truncated stream)", io.ErrUnexpectedEOF, true},
		{"context deadline", context.DeadlineExceeded, true},
		{"net.OpError (reset)", &net.OpError{Op: "read", Err: errors.New("connection reset by peer")}, true},
		{"wrapped unexpected EOF", fmt.Errorf("reading stream: %w", io.ErrUnexpectedEOF), true},
		{"connection reset by peer (string)", errors.New("read tcp: connection reset by peer"), true},
		{"broken pipe (string)", errors.New("write: broken pipe"), true},
		{"http2 stream error (string)", errors.New("stream error: stream ID 7; INTERNAL_ERROR"), true},
		{"server closed idle (string)", errors.New("server closed connection"), true},
		{"genuine app error is NOT transient", errors.New("invalid request: bad model id"), false},
		{"auth error is NOT transient", errors.New("401 unauthorized: invalid api key"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isTransientStreamError(c.err); got != c.want {
				t.Fatalf("isTransientStreamError(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
