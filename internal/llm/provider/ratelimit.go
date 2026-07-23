// GORILLA OVERRIDE: this file did not exist upstream. It is the proactive
// pace-setter — the missing half of rate-limit handling.
//
// Upstream (and this fork until now) only handled rate limits REACTIVELY: fire
// as fast as the work demands, and back off after a 429. On a low-RPM free tier
// (NVIDIA NIM, "up to 40 rpm") an agent's natural burst of tool-loop + sub-agent
// requests slams straight into the ceiling and triggers retry churn.
//
// This limiter spaces outbound requests so the client glides UNDER the user's
// configured cap instead of hitting it. It is deliberately a tiny, dependency-
// free token-spacer (no golang.org/x/time/rate) so the whole mechanism is
// readable in one screen — in keeping with the project's transparency ethos.
//
// The cap is read live from config.RateLimitRPM() on every request, so a change
// made in the /context dial takes effect immediately, mid-session.
package provider

import (
	"context"
	"sync"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

var (
	rlMu          sync.Mutex
	rlNextAllowed time.Time
)

// paceRequest blocks until this process is allowed to send its next request
// under the current requests-per-minute cap. A cap of 0 means unlimited and
// returns immediately. It honours ctx cancellation while waiting.
//
// All providers funnel through baseProvider.SendMessages / StreamResponse, so a
// single shared limiter here covers main-agent, title, summarize, and sub-agent
// traffic on one budget.
func paceRequest(ctx context.Context) error {
	rpm := config.RateLimitRPM()
	if rpm <= 0 {
		return nil // unlimited
	}
	spacing := time.Minute / time.Duration(rpm)

	rlMu.Lock()
	now := time.Now()
	// Earliest this request may go: at least `spacing` after the previous one.
	start := now
	if rlNextAllowed.After(now) {
		start = rlNextAllowed
	}
	rlNextAllowed = start.Add(spacing)
	rlMu.Unlock()

	wait := time.Until(start)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
