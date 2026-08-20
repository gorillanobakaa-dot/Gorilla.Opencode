<!-- Version: 1.8.0 · updated 26-08-20-13-54 -->
# What it costs to run: memory, disk and network

*Measured on 18 August 2026, on the reference machine (Sony VAIO SVE, i7-3632QM,
Debian 13), against gorilla-opencode v0.1.95. Every figure below says which
command produced it. Nothing here is estimated.*

*This is the developer track. `SATELLITE.md` tells the same story in plain
language — not a simplification, the same facts and the same numbers written for
someone who does not want the code.*

---

## The short answer

| | |
|---|---|
| **Minimum RAM** | **192 MB** — verified running. 160 MB is killed. |
| Typical peak, full interface | 181 MB |
| Headless (`-p "…"`) | 73 MB |
| **Network while idle** | **zero** — every socket closes |
| One message | 83.5 KB up, 14.6 KB down |
| Download to install | 19.7 MB |

On a 2.4 GB machine it uses **under 8% of RAM**.

---

## Memory

Measured with a hard cgroup ceiling and swap disabled
(`systemd-run --user --scope -p MemoryMax=… -p MemorySwapMax=0`), driving the
real interface into a live chat session, then reading the kernel's own
`memory.peak` and `memory.events`:

| ceiling | result | peak reached | OOM kills |
|---|---|---|---|
| 2.4 GB | runs | 181 MB | 0 |
| 256 MB | runs | 181 MB | 0 |
| **192 MB** | **runs** | **179 MB** | **0** |
| 160 MB | **killed** | — | — |
| 128 MB | **killed** | — | — |

So the working floor is between 160 and 192 MB, and 192 MB is proven.

A headless run — `gorilla-opencode -p "…"`, which is how it would be used from a
script or over a fragile SSH session — peaked at **73 MB**.

**One caveat, stated because it changes the number:** language servers were off
during these runs (`lsp all 9 off`). A language server is a separate process
with its own appetite — `gopls` on a large Go tree can exceed the entire figure
above. On a small machine, leave them off; that is what the `/context` rows for
each server are for.

## Disk

| | |
|---|---|
| Download | 19.7 MB (the `.deb`) |
| Installed | 56 MB |
| Session store, after 65 conversations | 5.4 MB — and `/sessions` can now reclaim it |
| Config | 432 KB |

The binary is built with `-s -w`, which strips the symbol table and DWARF:
66.1 MB → 48.6 MB when that was measured on 2026-08-07. On an 8 KB/s line that
saves about 40 minutes of somebody's afternoon, which is why it is never dropped
from a release build.

## Network

### While you are not using it: nothing

Started the interface, reached a ready chat, then left it alone for 90 seconds
and watched its sockets (`ss -tinp`, filtered to the process).

**Every connection closed. No polling, no keepalive, no heartbeat, no
telemetry.** An idle session on a metered link costs nothing at all.

Startup opens three short-lived connections totalling roughly **45 KB**, then
they close.

### One message

A single short question — *"in one short sentence, what is a mutex?"* — with
13,800 tokens of context:

| | |
|---|---|
| Sent | **83,576 bytes** |
| Received | 14,577 bytes |
| Total | 98,153 bytes |
| **Wire cost per input token** | **6.06 bytes** |

**85% of the traffic is outbound.** That is the opposite of ordinary internet
use, and it matters because satellite and mobile links are usually far weaker
upstream than down.

What that means in time:

| link | upload one message | full exchange |
|---|---|---|
| 50 KB/s | 1.6 s | 1.9 s |
| **8 KB/s** | **10.2 s** | **12.0 s** |
| 2 KB/s, bad day | 40.8 s | 47.9 s |

And to install it in the first place: **42 minutes at 8 KB/s**, 168 minutes at
2 KB/s.

### Why this validates `/context`

The wire cost is **6.06 bytes per input token, on every single message**. The
system prompt, the environment block and every tool schema are re-sent every
time you press enter.

So a tool you never use, carrying 475 tokens of schema, is not an abstraction —
it is **2.9 KB of upload per message**, forever. On an 8 KB/s link that is a
third of a second of your life per message, plus the tokens you pay for.

Switching off 3,000 tokens of loadout you do not need saves roughly **18 KB per
message**: about 2.3 seconds per message at 8 KB/s. Over a working session of a
hundred messages, close to four minutes of waiting and 1.8 MB of transfer.

That is the entire argument for the `/context` screen, in measured bytes rather
than principle.

---

## What happens when the link fails — measured, and it is not good

Bandwidth is the easy half. The harder half is what happens when a satellite
link does what satellite links do: drop, or go silent.

`tc netem` was unavailable — **this machine's own kernel has
`CONFIG_NET_SCH_NETEM` unset**, a casualty of the de-bloat. So the test was done
with a local CONNECT proxy instead, which works because the HTTP client honours
`HTTPS_PROXY` and a CONNECT tunnel relays TLS untouched, so certificates still
validate. No root, no kernel module.

### The link drops mid-turn → a retry storm

The proxy accepted the request, relayed it, then reset the connection eight
seconds in — every time.

| | |
|---|---|
| Attempts | **14** |
| Uploaded | **1.01 MB** |
| Elapsed | ~120 s, still going when killed |
| Answer produced | **none** |
| Error shown to the user | **none** |

`internal/llm/provider/provider.go` declares `maxRetries = 5`.

The extra attempts come from **two layers retrying independently**: the
application's own loop, and Go's `http.Transport`, which silently re-sends a
request when the connection dies before any response byte arrives. It can do
that here because the gzip request wrapper sets `GetBody` — which is precisely
what makes a body replayable. Neither layer knows about the other, so the real
ceiling is their product rather than the number written down.

On a metered link this is the expensive failure: **one unanswered question cost
a megabyte**, and at 8 KB/s that is more than two minutes of upload for nothing.

### The link goes silent → it hangs

The proxy held the socket open and simply stopped forwarding — no close, no
reset, which is what a satellite dropout actually looks like.

**It hung for the full 90-second cutoff.** No error, no output, no timeout.

That is deliberate for the interactive case: `httpclient.go` documents having no
`ResponseHeaderTimeout` and no `client.Timeout`, so a large model over a slow
link is never killed mid-answer, and cancellation is left to the user pressing
ESC.

**But the headless path has no user.** `gorilla-opencode -p "…"` in a script, a
cron job or over SSH has nobody to press anything, so it waits forever.

### Both are now fixed — measured against the same proxy that found them

**A per-turn upload budget.** One counter, attached once per turn, at the single
choke point every attempt passes through no matter who started it. It refuses
*before* sending, because the whole point is not to put the bytes on the link,
and the error says what it spent.

It is counted in **bytes, not attempts**, for the reason the grep tool taught
this project: a limit must be expressed in the unit of the resource it protects.
What a retry actually costs is the whole conversation re-uploaded.

| the drop test | before | after |
|---|---|---|
| Attempts | 14+ | **7** |
| Uploaded | 1.01 MB | **252 KB** |
| Outcome | ran until killed, silent | **exits in 50 s with a clear error** |

The error itself confirms the diagnosis — *"stopped after 7 attempts"* against a
declared `maxRetries = 5`. The two layers really were multiplying.

Default 4 MB per turn, which still allows dozens of legitimate retries on a
large conversation. `GORILLA_OPENCODE_TURN_UPLOAD_MB` changes it; `0` disables
it.

**A deadline on the headless path only.** The interactive path keeps its
deliberate absence of a timeout — a slow model must not be killed mid-answer,
and a human can press ESC. Headless has no human.

| the silence test | before | after |
|---|---|---|
| Outcome | hung indefinitely | **exits in 22 s** |
| Exit code | — | **1** |
| Message | none | names the cause and the override |

Default 30 minutes; `GORILLA_OPENCODE_HEADLESS_TIMEOUT` accepts a duration, `0`
waits forever.

**One thing the first attempt got wrong, worth recording.** The deadline
initially surfaced through the same branch as a user cancellation, so it printed
*"No content available"* and exited **0**. A script would have read that as
success with a strange answer — worse than the hang it replaced, because the
hang never claimed to have worked. A deadline is a failure, not a cancellation,
and is now checked first and separately.

**And one in the fix itself.** The budget was first wrapped *outside* the gzip
transport, with a comment confidently stating that this is what measures the
wire. It is the opposite: a `RoundTripper` sees the request before whatever it
wraps, so the outermost one sees the uncompressed body.
`TestTheBudgetMeasuresCompressedWireBytes` caught it — 132,000 bytes charged for
a body that compresses about twentyfold.

---

## The third failure: the server answers the phone and says nothing

The two failures above were *induced* with a test proxy. This one was found by
accident, live, and it is the one an ordinary user actually meets.

A baseline run hung. So did the same run with the proxy removed — so the
instrument was innocent. Two bare `curl` calls to the same host with the same
key then split it in one step:

| request | result |
|---|---|
| `GET /v1/models` | **200 in 0.083 s** |
| `POST /v1/chat/completions` (the configured default model) | **0 bytes, hangs** |

Sampling eight models from **NVIDIA NIM's own catalogue**:

| outcome | count | example |
|---|---|---|
| honest 404 in <0.2 s | 4 | `mistralai/mistral-large-2-instruct` |
| **served normally, first byte 0.36 s** | **1** | `nvidia/nemotron-3-nano-30b-a3b` |
| **accepted the connection, returned nothing, ever** | **2** | `meta/llama-3.3-70b-instruct` |

So a provider listing a model is advertising, not a capability. The 404s were
already handled. The silence was not — and the client's entire response to it
was to sit there with a spinner.

**Corrected later the same day, and the correction changes the diagnosis.** The
black hole is a COLD START, not a dead model. Forty minutes on, the same model
answered in 12.07 s and `meta/llama-3.3-70b-instruct` in 19.08 s. A sweep of all
102 listed models (`curl` per model, 25 s cap, 8 in parallel) gave:

| outcome | count |
|---|---|
| 404 — not entitled / not offered to this account | 60 |
| **200 — working** | **32** |
| 000 — accepted and silent | 7 |
| 400 / 500 | 3 |

Working models cluster hard at the fast end: 27 of the 32 delivered a first byte
in under a second, and the three slowest (11.65 s, 12.07 s, 19.08 s) are exactly
the ones that had been black holes earlier — i.e. they were waking up.

The first-byte guard is still correct; only its *message* was wrong. It said the
model "is not actually being served", which would push a user off a model that
merely needed a minute. It now says the endpoint is probably cold.

### Why there was no timeout to catch it

`httpclient.go` set no `ResponseHeaderTimeout`, justified in a comment as *"first
byte can be legitimately slow on a big model + slow link"*.

That was an intuition, and Go's own documentation contradicts it:
ResponseHeaderTimeout *"specifies the amount of time to wait for a server's
response headers **after fully writing the request (including its body, if
any)**"*. A slow uplink therefore never counts against it. Uploading 100 KB at
2 KB/s spends 50 seconds of wall clock and **zero** seconds of that budget.

The comment is preserved in the file, because being wrong in a documented way is
more useful than being quietly corrected.

### Two guards, because there are two failures

**`FirstByteTimeout`** (default 120 s) bounds the wait for headers — the server
that never replies. 120 s is about 330× the measured first byte of the model
that actually worked.

**`StreamStallTimeout`** (default 90 s) covers what a header timeout structurally
cannot see: headers arrived, the answer started, and *then* the link died. No
header timeout can fire, because the headers already came.

It is a **stall timer, not a wall clock** — every chunk resets it, and it only
arms once the first chunk lands. A stream crawling in at one token a second is
never touched; only a stream delivering nothing at all for the whole window is.
That distinction is the whole design: a false positive destroys an answer the
user has already paid for twice, in money and in upload time.

### And a third retry layer, found by arithmetic

With a 20-second first-byte timeout the run should have stopped near 41 s. It
was still going at **123 s**. That ratio is almost exactly 3, and the 3 was
`openai-go`'s own `MaxRetries: 2` default — never configured, so every one of our
attempts was silently three of the SDK's.

That is the **third** independent retry layer in one call path, after the
application loop and Go's transport replay found earlier the same day. Their
effect multiplies. The SDK now gets `WithMaxRetries(0)`: retries belong to the
one layer that knows whether content has already streamed, what the turn has
spent, and what to tell the user.

| the black-hole test | before | after |
|---|---|---|
| Outcome | still running at 110 s+, silent | **exits at 43 s** |
| Exit code | — | **1** |
| Message | none | names the likely cause and points at `/models` |
| Working model, same build | 4 s, exit 0 | **4 s, exit 0 — unchanged** |

The last row is the one that mattered most to check.

---

## The slow-but-working link: the guards must not misfire

The failure cases above are the dramatic ones. The common one is a link that
works and is merely slow, and it is the case most able to produce a FALSE
positive — every timeout added this session watches for silence, and a slow link
is full of silences that are not failures. A stall guard that cannot distinguish
"crawling" from "dead" destroys a working answer, which is strictly worse than
the hang it replaced.

Tested through the same CONNECT proxy in `slow` mode (a fixed byte rate), against
a warm, instant model so the only variable is link speed:

| rate | exit | wall | full answer | largest inter-chunk gap |
|---|---|---|---|---|
| 4 KB/s | 0 | 70 s | yes | 18 s |
| 2 KB/s | 0 | 130 s | yes (150 words) | **32.8 s** |

`config.StreamStallTimeout` is 90 s. The worst real gap on the slower link was
32.8 s, a ~2.7x margin, and it holds at the 2 KB/s bad-day floor. So the guard
has comfortable headroom before a slow link could read as a dead one. Below
~1 KB/s the margin would narrow, but 2 KB/s is already the floor at which the
tool is usable at all.

The dominant cost on the slow link was not latency but VOLUME: one throwaway
question pushed **~140 KB upstream** — the conversation and every tool schema,
re-sent per step of the agent turn — which at 2 KB/s is where most of the 130 s
went. This is the same asymmetry the per-message section measures (85% outbound),
observed from the failure-handling side: on a constrained uplink the guards were
never the risk, the upload was.

---

## How to reproduce any of this

```bash
# Memory floor — raise or lower the ceiling until it dies
systemd-run --user --scope -p MemoryMax=192M -p MemorySwapMax=0 \
    --unit=octest gorilla-opencode
cat /sys/fs/cgroup/user.slice/user-*.slice/user@*.service/app.slice/octest.scope/memory.peak
cat /sys/fs/cgroup/.../octest.scope/memory.events    # oom_kill count

# Network, per process, no extra tooling
ss -tinp | grep -A1 "pid=$(pgrep -f 'bin/gorilla-opencode$')"
# read bytes_sent / bytes_received, before and after a turn
```

## Claim sources

Every figure above is **stated in input** — produced by the command named, on
2026-08-18, on the reference machine. The only inference is the arithmetic
converting bytes to seconds at a given link speed, which assumes the link
achieves its nominal rate and no retransmission; real satellite links do worse.

```bash
# Split "is the provider broken" from "is this model broken" in two calls
curl -s -o /dev/null -w "models:  %{http_code} %{time_total}s\n" \
  -H "Authorization: Bearer $KEY" https://integrate.api.nvidia.com/v1/models
curl -s -o /dev/null -w "chat:    %{http_code} ttfb=%{time_starttransfer}s\n" --max-time 30 \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"model":"MODEL","messages":[{"role":"user","content":"hi"}],"max_tokens":10}' \
  https://integrate.api.nvidia.com/v1/chat/completions
# http_code 000 with 0 bytes = black hole. 404 in <0.2s = not entitled/not served.

# Slow-but-working link: proxy at a fixed byte rate, then a real turn through it
python3 scripts/satellite-proxy.py slow 999 8873 2000   # 2 KB/s
HTTPS_PROXY=http://127.0.0.1:8873 gorilla-opencode -q -p "your prompt"
# watch the proxy log: inter-chunk gaps must stay under StreamStallTimeout (90s).

# Where is a hung build actually blocked? Go dumps every goroutine on SIGQUIT
GOTRACEBACK=all gorilla-opencode -p "..." & sleep 25; kill -QUIT $!
```


## Response compression: asked for, refused, quantified

Measured 2026-08-20. `DisableCompression` is unset and nothing sets
`Accept-Encoding` manually — which matters, because setting that header by hand
silently disables Go's automatic inflate. So the transport does the right thing:
it advertises gzip and would transparently decompress.

Providers decline.

| endpoint | `Accept-Encoding: gzip` sent | `content-encoding` returned | bytes |
|---|---|---|---|
| NIM `/v1/models` | yes | **absent** | 9,848 either way |
| NIM streaming completion | yes | **absent** | 22,256 |
| local llama.cpp | yes | **absent** | 602 either way |
| `api.github.com/meta` (control) | yes | `gzip` | — |

The control matters: it proves the request is well formed and the mechanism
works, so "absent" is a refusal rather than a bug on our side.

**The streaming case is the expensive one.** SSE sends one JSON envelope per
token — `id`, `model`, `object`, `choices[0].index`, `delta`, `finish_reason` —
around a payload of a few characters. Measured on a 59-token completion:

```
raw stream               22,256 bytes    377 bytes per token
whole-blob gzip             665 bytes    97.0%  <- FLATTERS ITSELF
per-chunk Z_SYNC_FLUSH    1,663 bytes    92.5%  <- the honest number
```

Quote **92.5%**. The 97% figure requires compressing the completed stream, which
cannot be done without destroying incrementality — the same self-flattering
measurement trap recorded for request gzip, where a synthetic corpus reported
99.3% against a real-world 77%.

At the Austere profile's 2 KB/s that is **10.9s of pure receive time per short
answer, against 0.8s if it were compressed**. A 500-token reply costs ~188 KB
and roughly 94 seconds of downlink.

**Nothing to implement.** Response encoding is the server's choice; the client
already asks. Recorded so the saving is not mistaken for an available one, and
so the next person measuring bytes per turn counts the downlink too — it is
larger than the uplink for any answer longer than a couple of hundred tokens.

Also observed in the same headers: `deprecation: 2026-08-26T09:00:00Z` on
`meta/llama-3.1-8b-instruct`.


## Non-streaming on slow profiles: 27x less downlink, identical tokens

Measured 2026-08-20 against NIM, same prompt, same model, same 60-token answer:

```
stream:true    22,256 bytes    377 bytes per output token
stream:false      834 bytes
                  27x
```

Token accounting is IDENTICAL — `total_tokens: 106` on both, read from each
response's own usage block. The provider bill does not move; only the user's
metered allowance does. Those are the same money on a satellite or prepaid
mobile plan and different money on a flat link, which is exactly why this is
per-profile rather than global.

Cause is the transport, not the model. SSE wraps every token in its own JSON
envelope (`id`, `model`, `object`, `choices[0].index`, `delta`,
`finish_reason`) around a payload of a few characters. A non-streamed reply
sends one envelope for the whole answer.

**Where it is implemented.** `baseProvider.StreamResponse` in
`internal/llm/provider/provider.go` checks `config.StreamRepliesEnabled()` and,
when false, routes to `sendAsSingleEvent` — which runs the existing
non-streaming `send()` and reports it on the same channel: one
`EventContentDelta` carrying the whole text, then `EventComplete` with tool
calls and usage. Callers keep consuming a channel exactly as before, so the TUI
never learns which mode it is in, and every provider inherits it from one seam
rather than each client implementing it.

`Stream` is false for Austere and Constrained, true for Modest and above.
`GORILLA_OPENCODE_STREAM=0/1` overrules the profile.

**The trade-off, stated because it is real.** Non-streaming removes the stall
guard's progress signal: `stallGuard` resets on every chunk, and with one chunk
there is nothing to reset. A stalled link stops being distinguishable from a
slow answer, leaving `FirstByteTimeout` to carry that job alone — which is why
the slow profiles set it to 8 and 15 minutes.

**What it does not change:** capability. Same tools, same answer, same
quality — consistent with the rule that a connection profile alters waiting and
spending, never what the agent can do.


## Connection profile ladder (as shipped)

| profile | band | `Stream` | `FirstByte` | `StreamStall` | `UploadMB` | `MaxRetries` |
|---|---|---|---|---|---|---|
| `austere` | 1-9 KB/s | **false** | 15m | 5m | 0.5 | 2 |
| `constrained` | 10-60 KB/s | **false** | 8m | 3m | 1.5 | 3 |
| `modest` *(default)* | 60-250 KB/s | true | 4m | 2m | 4 | 4 |
| `broadband` | 250 KB/s-5 MB/s | true | 2m | 90s | 8 | 5 |
| `unconstrained` | 5 MB/s+ | true | 60s | 45s | 16 | 5 |

Defined in `internal/config/connprofile.go`. The ladder is asserted monotonic by
`TestConnProfileLadderIsMonotonic` — a rung that is less patient or less frugal
than a slower one is a bug, because the user picks "slower" and would silently
get "less careful".

`modest` is the default deliberately: its numbers sit close to the pre-profile
shipped values (120s / 90s / 4 MB), so upgrading changes almost nothing for
anyone who never opens the picker. The default is not `unconstrained`, because
this program is built for bad connections and must not assume a good one.

**Precedence:** an explicit environment variable always beats the profile —
`GORILLA_OPENCODE_FIRST_BYTE_TIMEOUT`, `GORILLA_OPENCODE_STREAM_STALL_TIMEOUT`,
`GORILLA_OPENCODE_TURN_UPLOAD_MB`, `GORILLA_OPENCODE_STREAM`. Someone who set one
meant it, and a preset silently overriding a deliberate choice is the same
silent-failure class this subsystem exists to remove.

**Scope, fixed by design:** a profile sets waiting and spending only. It never
touches the loadout, the tool set or the model, so switching profiles is
predictable and reversible.
