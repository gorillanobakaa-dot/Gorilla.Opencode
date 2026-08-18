<!-- Version: 1.3.0 · updated 26-08-18-16-35 -->
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

# Where is a hung build actually blocked? Go dumps every goroutine on SIGQUIT
GOTRACEBACK=all gorilla-opencode -p "..." & sleep 25; kill -QUIT $!
```
