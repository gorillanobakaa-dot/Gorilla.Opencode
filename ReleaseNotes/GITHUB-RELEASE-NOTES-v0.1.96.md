# Gorilla OpenCode v0.1.96 — when the link fails, it fails cheaply and says so

**Everything you need is on this page**, printed in full.

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.96_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.96_amd64.deb` |
| `gorilla-opencode-0.1.96-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer |
| `SHA256SUMS-v0.1.96.txt` | `sha256sum -c` |

Use `apt`, not `dpkg -i`. Restart the program if it is already running.

---

## In one paragraph

Every earlier release measured what this program costs when the connection
works. This one measured what it does when the connection breaks — the normal
case on a satellite link — and fixed it. A dropped link used to retry silently 14
times and upload a megabyte for one unanswered question, because three separate
retry layers multiplied; there is now one budget counted in bytes, and the same
drop costs 252 KB and stops with a clear message. A link that goes quiet used to
hang forever; two timeouts now catch it — one for the answer never starting, one
for it stopping halfway — and the second is a stall timer that never cuts off a
slow-but-flowing stream (proven at 2 KB/s). The waiting indicator counts and says
when a model is warming up. The web tools identify as a browser so public pages
stop refusing the bot. And the input box's CPU cost on old hardware was measured
exactly and deliberately left alone, because the expensive part is a documented
correctness fix and speeding it up needs a real test first.

---

## The full record

Both tracks below are reproduced complete from `Changelogs/v0.1.96-release-notes.md`,
which ships inside the `.deb`.

<!-- BEGIN full notes -->

# Gorilla Opencode v0.1.96 — when the link fails, it fails cheaply and says so

Released 18 August 2026. Replaces v0.1.95.

Two tracks below, both complete. Neither is a summary of the other.

---

## Plain-language track

This program is built for people on satellite and mobile links: slow, expensive,
and — the part nobody tests — **unreliable**. Every earlier release measured what
it costs when the connection *works*. This one measured what it does when the
connection *breaks*, and fixed what that turned up.

The one-line version: **when the network goes wrong, it now fails cheaply and out
loud, instead of expensively and in silence.**

### The connection drops mid-answer

Before: the program tried again, and again — **14 times, a full megabyte
uploaded, for one question that was never answered, and it never said a word.**
On a metered plan that is money gone for nothing.

Three separate things were retrying the same request without knowing about each
other, and their effect multiplied. Now there is **one budget, counted in the
thing you actually pay for — bytes.** When a turn has re-uploaded more than it is
allowed, it stops and tells you what it spent. The same drop now costs 252 KB and
ends in under a minute with a clear message, not a silent megabyte.

### The connection goes quiet

A satellite link doesn't always say goodbye — it just stops carrying anything,
and the connection still *looks* alive. Before, the program waited forever. In
the normal screen you can see the spinner and press Escape, but run from a script
there is nobody to press anything, so it hung all night.

Now there are two limits: one for **the answer never starting** (the server took
the request and went silent), and one for **the answer stopping halfway** (it
began, then the link died). The second is a *stall* timer, not a stopwatch — it
only fires if nothing has arrived for a while, so an answer trickling in slowly on
a bad link is never cut off. Tested at 2 KB/s: a full answer still completes, with
comfortable margin.

### The screen tells you what's happening

The waiting indicator used to just spin. Now it **counts** — `Thinking… (18s)` —
and once a wait crosses the point where a model is probably warming up (measured:
free models on shared servers can take 12–19 seconds, sometimes minutes), it says
so in words, so you can tell "warming up" from "stuck".

### Public pages stop slamming the door

The research and web-reading tools used to introduce themselves as a robot, and a
lot of the web reflexively blocks anything labelled a bot — a search that a person
could run in a browser was refused. They now identify as the browser a person
would use. It reads only public pages a person could read; it evades no login and
no paywall. (You can switch it back with a setting.)

### One thing we measured and deliberately did NOT rush

While looking into why the input box can spike the processor on old hardware, we
found the cause exactly: the box does an expensive full redraw on **every
keystroke** to work out its own height. That is real, and it explains the CPU
spikes and the occasional runaway cursor. But the redraw is also a documented
**correctness** fix from a few releases ago — the cheap shortcut it replaced was
wrong. So we measured it, wrote down exactly what it costs, identified the one
safe way to speed it up, and **left the code alone for this release** rather than
trade correctness for speed on a guess. The fix will come with a proper test
behind it, not before.

---

## Developer track

### Scope

Nine commits since v0.1.95, in four groups: the retry/timeout guards, the
headless deadline, the on-screen liveness, and the fetch User-Agent. Plus one
measurement (editor keystroke cost) recorded without a code change, and two
brain lessons.

### The retry storm — a limit counted in the wrong unit, multiplied by hidden layers

Measured through a local CONNECT proxy (the client honours `HTTPS_PROXY`; the
machine's own kernel has `CONFIG_NET_SCH_NETEM` unset, so `tc netem` was
unavailable). A link that reset 8 s into every attempt produced **14 attempts and
1.01 MB uploaded** for one unanswered question, against a declared
`maxRetries = 5`, with no user-visible error.

The excess came from **three independent retry layers**, found over the day:

1. the application loop in `openai.go`;
2. Go's `http.Transport`, which silently re-sends when the connection dies before
   any response byte arrives — possible here because `gzip_request.go` sets
   `GetBody`, which is what makes a body replayable;
3. `openai-go`'s own `MaxRetries: 2` default (`requestconfig.go`), never
   configured, so every application attempt was silently three of the SDK's.

Their effect is the **product**, not the maximum. The arithmetic gave it away: a
20 s first-byte bound that should have stopped near 41 s ran to 123 s — almost
exactly 3×.

Fixes:
- `internal/llm/provider/uploadbudget.go` — a per-turn budget in **bytes**, at the
  single choke point (`budgetTransport`, innermost so it measures the compressed
  wire), refusing *before* sending, with an error that names the cost. Default
  4 MB, `GORILLA_OPENCODE_TURN_UPLOAD_MB` overrides, `0` disables.
- `option.WithMaxRetries(0)` on the SDK client — retries belong to the one layer
  that knows whether content already streamed, what the turn spent, and what to
  tell the user.
- A recurring first-byte timeout gets exactly one retry, then an honest error.

Measured drop test, before → after: **14 attempts / 1.01 MB / silent → 7 / 252 KB
/ exits in 50 s with the cause named.**

### The two ways a server goes quiet

`httpclient.go` deliberately had **no** `ResponseHeaderTimeout`, justified as
"first byte can be slow on a big model + slow link." Go's own docs contradict
that: the timer starts only *after* the request body is fully written, so a slow
uplink never counts against it. Measured first byte of the one working NIM model
that day: **0.36 s**.

- `config.FirstByteTimeout()` (default 120 s, ~330× that measured first byte) →
  `Transport.ResponseHeaderTimeout`. Bounds "the server never replied."
- `config.StreamStallTimeout()` (default 90 s) → `internal/llm/provider/stallguard.go`.
  Covers what a header timeout structurally cannot: headers arrived, the answer
  started, then the stream stopped. A **stall** timer — armed by the first chunk,
  reset by every chunk after — so a slow-but-flowing stream is never killed. It
  cancels a derived context, and `openai.go` translates the resulting
  `context.Canceled` into `ErrStreamStalled` so `agent.go` cannot swallow it as a
  user cancellation.

The original wrong comment is kept next to the correction, because being wrong in
a documented way is more useful than being quietly fixed.

### The headless deadline

Interactive keeps its deliberate absence of a wall-clock timeout — a slow model
must not be killed mid-answer, and a human can press ESC. Headless (`-p`) has no
human. `config.NonInteractiveDeadline()` (default 30 m,
`GORILLA_OPENCODE_HEADLESS_TIMEOUT`) is checked *before* the cancellation branch
so it exits non-zero with a named cause, not `0` with "No content available" (the
first attempt got that wrong; a script would read exit 0 as success).

### The cold-start correction

The first-byte error first said the model "is not actually being served." Forty
minutes later the same "black-holed" model answered in 12 s. A sweep of all 102
listed NIM models: **32 working, 60 honest 404s, 7 silent, 3 erroring** — and the
silent ones were the slowest to wake, i.e. **cold-starting**, not dead. The
message now says the endpoint is probably warming up and suggests `/models`.

### On-screen liveness

`internal/tui/components/chat/list.go`: the working indicator now carries an
elapsed clock, reset per phase, and — for pre-first-token phases only — fires a
one-shot info toast past `coldStartHint` (12 s). The footer stays exactly
`FooterReservedRows` (1) tall; the explanation rides a toast, not the footer,
because lipgloss wraps and a second row would break the scrollback-erase
invariant (the width→height trap). A tool-wait counts but is never labelled a
model warm-up.

### The fetch User-Agent

`config.BrowserUserAgent()` (default a current desktop Firefox-on-Linux token,
`GORILLA_OPENCODE_USER_AGENT` overrides, `honest` restores the bot token), wired
into the web-fetch tool that every research/OSINT lane uses (`tools.go:92`).
Measured: `google.com/search` 302→200 from the same client the same second on the
UA change alone; lynx did not help (Reuters 401'd it too), so the lever is the
UA, not a browser subprocess. Deliberately **not** applied to the provider auth
handshakes or the JSON-API contact in `websearch.go` (SEC EDGAR 403s anything but
an email-form contact — measured 2026-08-17).

### The slow-but-working link

The guards must not misfire on a link that merely crawls — the common satellite
case, and the one most able to produce a false positive. Tested through the proxy
in `slow` mode against a warm model: **4 KB/s → 70 s, 2 KB/s → 130 s, both exit 0
with the full answer.** Largest inter-chunk gap at 2 KB/s was **32.8 s** against a
90 s stall guard — a ~2.7× margin that holds at the bad-day floor. The dominant
cost was upload volume (~140 KB for one question), not latency: the guards were
never the risk on a constrained uplink, the upload was.

### Measured, deliberately not shipped: the editor keystroke cost

`measuredRows()` renders the whole input through a styled 300-row probe to count
its rows, on every keystroke. Measured with production setup (`CreateTextArea` +
`config.Load`; a bare `textarea.New()` saturates the probe to 300 and makes the
number meaningless): **10.3–12.4 ms and 1.6–2.2 MB and ~6,300 allocations per
keystroke**, near-constant with input length. On weak hardware that exceeds a
held key's ~33 ms repeat budget, so events queue and drain after release — the
reported cursor runaway — and the allocation churn is the reported 70–80% CPU.

This cost was never previously documented. But the probe is the **correctness**
fix from v0.1.85: it exists because the cheap `wrappedRows` estimate undercounts
bubbles' visual rows. A cache gated on that same estimate was prototyped and
**reverted** — it reintroduced the divergence (a test caught `cached=5` vs
`fresh=4` on deletion). The only safe optimisation is a cheaper probe (plain
styling; row count is width/text-driven, not colour), and it needs the
non-vacuous test that has been deferred since v0.1.85 before it touches release
code. Recorded in `LESSONS/02.TUI.RENDER` and the brain; not shipped here.

### Verification

`go build ./...`, `go vet ./...`, `go test ./...` all clean. New tests:
`uploadbudget_test.go` (the budget bounds wire bytes, refuses before sending,
noop without a budget), `stallguard_test.go` (slow is never cut, silence is,
armed only after the first chunk, a stall is not a cancellation, only a header
timeout counts as first-byte). Satellite behaviour measured before and after
against the same proxy that found each fault.

### Measurements

| what | figure | how |
|---|---|---|
| retry storm, before | 14 attempts, 1.01 MB, silent | CONNECT proxy `cut 8` |
| retry storm, after | 7 attempts, 252 KB, 50 s, named error | same proxy |
| headless hang, after | exits 22 s, exit 1 | proxy `hole 6` |
| first byte, working NIM model | 0.36 s | streaming curl |
| NIM catalogue liveness | 32 work / 60 404 / 7 silent / 3 err (of 102) | per-model probe |
| slow link 4 KB/s | 70 s, exit 0, full answer | proxy `slow … 4000` |
| slow link 2 KB/s | 130 s, exit 0, gap 32.8 s vs 90 s guard | proxy `slow … 2000` |
| UA unblock | google 302→200 | same client, UA only |
| editor keystroke cost | 12 ms / 2.1 MB / 6,300 allocs | benchmark, production setup |

### Claim sourcing

Every figure above is **stated in input** — produced by the named command on
2026-08-18 on the reference machine (Sony VAIO SVE, i7-3632QM, Debian 13). The
only inferences are the byte→second conversions at a given link speed, which
assume the link achieves its nominal rate with no retransmission — real satellite
links do worse.

<!-- END full notes -->
