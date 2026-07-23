# Gorilla OpenCode Changelog — July 23, 2026 (late night)

## Version v0.1.33 (Build: 2026-07-23)

### Summary

Network hardening for the mission profile — laptops and phones in remote/hostile
environments whose only uplink is a **satellite phone link** (high latency,
frequent drops, single-digit-KB/s bandwidth). Plus the CI gate that would have
caught the last two inherited bugs.

Three changes:
1. A shared, satellite-tuned HTTP client for the model providers.
2. `ResourceExhausted` / server-busy errors now retry with back-off instead of
   failing the turn.
3. A `ci` GitHub Actions workflow: build + vet + **test** on every push/PR.

---

## Network Resilience 🛰️

### 1. Satellite-hardened HTTP client (`internal/llm/provider/httpclient.go`, new)

The providers used the SDK default HTTP client (no connection tuning), and one
path hard-coded a 30s wall-clock timeout that would kill a slow big-model stream.
Both are wrong for a satellite uplink. The new shared client, wired into the
OpenAI/NIM and Anthropic providers:

- **No wall-clock `Timeout`** — a streaming answer is long-lived; a fixed
  deadline would abort a legitimate slow reply (a 550B model's first token can
  take tens of seconds over satellite). Cancellation stays per-request via
  context (ESC / turn cancel).
- **Keep-alive + 120s idle reuse** — the expensive TLS handshake is paid once
  and the warm connection is reused across the whole agent tool loop. (Only
  possible now that streams are `Close()`d — v0.1.32.)
- **HTTP/2 multiplexing** — all traffic to a provider shares one connection;
  frugal with the tiny bandwidth budget.
- **Finite dial/TLS timeouts (30s)** so a dead link fails fast instead of
  hanging forever — but **no** response-header timeout (slow first byte is
  normal on a big model + slow link).
- **Proxy from environment** — satellite terminals often front traffic through a
  local optimising proxy (`HTTP(S)_PROXY`).

### 2. `ResourceExhausted` / server-busy → retry with back-off

Previously a NIM `ResourceExhausted: Worker local total request limit reached`
(or any in-band rate-limit/overloaded error) surfaced immediately and failed the
turn. On a link where every round-trip is precious, a transient "server busy"
should self-heal. Now:

- A new classifier (`isServerBusyError`) recognises `ResourceExhausted`,
  "request limit reached", "too many requests", "rate limit", "overloaded",
  "service unavailable", "try again later", etc. — including the in-band SSE
  form that never surfaces as an HTTP status.
- These retry with a **longer** back-off (2→4→8→16→20s) than transport blips
  (0.5→…→6s), because hammering a congested endpoint makes it worse and wastes
  bandwidth. Only before any content has streamed (a mid-answer retry would
  duplicate output), capped at `maxRetries`.
- The typed-error path also now retries HTTP **503** and **529** (overloaded),
  not just 429/500.

**Files:** `internal/llm/provider/openai.go`, `anthropic.go`, `httpclient.go`;
tests in `busyerror_test.go`.

---

## CI ⚙️

### 3. `ci` workflow — build + vet + test on push/PR

`.github/workflows/ci.yml` runs `go build ./...`, `go vet ./...`, and
`go test ./... -count=1`. No secrets, so it runs anywhere Actions is enabled
(unlike the goreleaser workflows). This is the gate that was missing —
Errors.in.the.code.txt #001 (stream leak) and #002 (test panic) both hid
precisely because nothing ran the tests.

> Note: the repo's existing `build.yml`/`release.yml` have never executed —
> GitHub Actions appears to be disabled at the repo level. Enable it once in the
> repo's **Actions** tab for `ci.yml` (and the release automation) to start
> running.

---

**Human track:** we made the app much tougher on a bad satellite connection. It
now keeps one phone line warm and reuses it instead of redialing (expensive when
the line is slow), never hangs up on a slow reply just because a timer expired,
and when NVIDIA says "too busy, try later" it politely waits and retries instead
of giving up. And we added an automatic check that runs the test suite on every
change, so this class of bug can't sneak back in unnoticed.
