package provider

// GORILLA FIX (2026-10-23): retry a Google server error instead of ending the
// turn on it.
//
// Live on the MacBook Air at 16:37:36 (Gemini 3.8 Flash through Antigravity):
// "The model returned an error and produced no answer: Antigravity: Internal
// error encountered (HTTP 500)". The turn ended and the stranger install sat
// idle for 40 minutes. The Antigravity and Code Assist clients had no retry at
// all, and the Gemini API client retried only rate limits. A 500, 502, 503 or
// 504 from Google is usually gone a few seconds later.
//
// Only the REQUEST is retried, before any text has been streamed: once a reply
// has started, repeating it would show the person the same words twice.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/logging"
)

// googleServerRetries is how many times a transient server error is retried
// after the first attempt. Small on purpose: a server that is down for longer
// than about half a minute should be reported, not waited on.
const googleServerRetries = 4

// serverRetryWait is the pause before retry n (1-based): 2 s doubling, plus 20%,
// the same backoff the Gemini client uses for rate limits. A variable so a test
// does not wait half a minute.
var serverRetryWait = func(n int) time.Duration {
	ms := 2000 * (1 << (n - 1))
	return time.Duration(ms+ms/5) * time.Millisecond
}

// notifyServerRetry puts the retry on screen, through the same channel the
// rate-limit notice uses. A variable so a test can read what was said.
var notifyServerRetry = func(msg string, wait time.Duration) {
	logging.WarnPersist(msg, logging.PersistTimeArg, wait+100*time.Millisecond)
}

// isTransientServerStatus reports the HTTP statuses worth retrying.
func isTransientServerStatus(code int) bool {
	switch code {
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// isTransientServerText reports an error message that names a transient
// server fault, for clients whose errors arrive as text.
func isTransientServerText(msg string) bool {
	low := strings.ToLower(msg)
	for _, s := range []string{"internal error", "unavailable", "backend error",
		"error 500", "error 502", "error 503", "error 504",
		"http 500", "http 502", "http 503", "http 504"} {
		if strings.Contains(low, s) {
			return true
		}
	}
	return false
}

// serverRetryNotice is the line shown on screen before retry n.
func serverRetryNotice(code, n int) string {
	if code > 0 {
		return fmt.Sprintf("Google server error (HTTP %d), retry %d of %d", code, n, googleServerRetries)
	}
	return fmt.Sprintf("Google server error, retry %d of %d", n, googleServerRetries)
}

// postRetryingServerErrors calls post until the answer is not a transient
// server error, at most 1 + googleServerRetries times. The last answer is
// returned as it is, so the caller reports it in its own words.
func postRetryingServerErrors(ctx context.Context, post func() (*http.Response, error)) (*http.Response, error) {
	for n := 1; ; n++ {
		resp, err := post()
		if err != nil || !isTransientServerStatus(resp.StatusCode) || n > googleServerRetries {
			return resp, err
		}
		code := resp.StatusCode
		resp.Body.Close()
		wait := serverRetryWait(n)
		notifyServerRetry(serverRetryNotice(code, n), wait)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}
