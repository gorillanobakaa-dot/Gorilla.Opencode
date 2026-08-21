// Version: 1.0.0 · updated 26-08-21-15-10
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// GORILLA FIX (2026-08-21): a stalled provider call must end in a sentence, not
// a hang.
//
// Observed on the real binary: the Antigravity sign-in completed, printed
// "Setting up your Antigravity free tier..." and never returned. Every call in
// this package used http.DefaultClient — no timeout — from a context with no
// deadline, so one request that never came back froze the whole login behind a
// message indistinguishable from "working, please wait".
//
// This test stands up a server that accepts the connection and then says
// nothing, which is exactly that failure, and asserts the call gives up.
func TestAStalledProviderCallTimesOutInsteadOfHanging(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never respond
	}))
	t.Cleanup(func() { close(block); srv.Close() })

	restore := authTimeout
	authTimeout = 200 * time.Millisecond
	t.Cleanup(func() { authTimeout = restore })

	req, err := http.NewRequestWithContext(context.Background(), "POST", srv.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		resp, err := authHTTP().Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a server that never answered was treated as a success")
		}
		if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline") {
			t.Errorf("gave up, but not for a timeout reason: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the call did not return — this is the hang the fix exists to prevent")
	}
}

// The shipped timeout must be bounded and sane. A zero timeout is the bug this
// replaced; an hour is the bug wearing a number.
func TestShippedAuthTimeoutIsBoundedAndReasonable(t *testing.T) {
	if authTimeout <= 0 {
		t.Fatal("authTimeout is zero — that is http.DefaultClient again, with extra steps")
	}
	if authTimeout > 2*time.Minute {
		t.Errorf("authTimeout is %v; nobody watches a message that long without concluding it is broken", authTimeout)
	}
}
