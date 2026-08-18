<!-- Version: 1.0.0 · updated 26-08-18-14-43 -->
# What it costs to run: memory, disk and network

*Measured on 18 August 2026, on the reference machine (Sony VAIO SVE, i7-3632QM,
Debian 13), against gorilla-opencode v0.1.95. Every figure below says which
command produced it. Nothing here is estimated.*

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
