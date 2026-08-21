// Version: 1.0.0 · updated 26-08-21-15-05
package auth

import (
	"net/http"
	"time"
)

// GORILLA FIX (2026-08-21): every provider call in this package used
// http.DefaultClient, which has NO TIMEOUT, from a context.Background() with no
// deadline. A stalled connection therefore waits forever.
//
// Observed: the Antigravity portal sign-in completed, printed "Setting up your
// Antigravity free tier..." and sat there. The same account had signed in
// instantly eight minutes earlier, so nothing was wrong with the account — one
// request simply never came back, and the program had no way to notice.
//
// That is the worst shape a failure can take here. The screen says "setting up",
// which is indistinguishable from "working, please wait", so the user waits.
// CLAUDE.md assumes a possibly high-latency link for this audience, and the
// project's own connection profiles exist precisely because a long silence must
// be interpretable.
//
// A bounded client turns an indefinite hang into a sentence. The setup path
// already handles a FAILED SetupProject gracefully — it keeps the saved token
// and retries on first use — so a timeout costs the user nothing and tells them
// where they stand.
//
// authTimeout is a package var, not a const, so tests can shorten it. 45s is
// generous: these are small JSON calls, and the slowest thing here is a token
// exchange on a bad line.
var authTimeout = 45 * time.Second

// authHTTP is the client every call in this package uses. Deliberately a
// function rather than a shared value so a changed authTimeout takes effect.
func authHTTP() *http.Client {
	return &http.Client{Timeout: authTimeout}
}
