package provider

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

// GORILLA FIX (2026-10-27, job 07): one dropped connection ended the turn.
// Measured in the acceptance run over the owner's phone hotspot: the first
// request of a scenario failed with "net/http: TLS handshake timeout" and the
// chat stopped with an error, before the model had seen the prompt. A request
// that never got a connection was never received, so sending it again cannot
// do anything twice or be billed twice. Only those errors are retried (a
// refused or failed dial, a failed name lookup, a TLS handshake that timed
// out), twice, 2 s then 4 s apart; anything after the connection is made is
// left to the caller, as before.

// chatgptDo sends one request; a variable so the tests need no network.
var chatgptDo = http.DefaultClient.Do

// chatgptConnectBackoff is the wait before each retry.
var chatgptConnectBackoff = []time.Duration{2 * time.Second, 4 * time.Second}

// neverConnected reports an error that happened before the request could
// reach the server.
func neverConnected(err error) bool {
	if err == nil {
		return false
	}
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return true
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return true
	}
	return strings.Contains(err.Error(), "TLS handshake timeout")
}

// doWithConnectRetry sends the request newReq builds, again after a failure
// to connect, until it connects, the waits run out or ctx ends.
func doWithConnectRetry(ctx context.Context, newReq func() (*http.Request, error)) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		req, err := newReq()
		if err != nil {
			return nil, err
		}
		resp, err := chatgptDo(req)
		if err == nil || !neverConnected(err) || attempt >= len(chatgptConnectBackoff) {
			return resp, err
		}
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(chatgptConnectBackoff[attempt]):
		}
	}
}
