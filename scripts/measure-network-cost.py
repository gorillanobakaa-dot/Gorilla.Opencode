#!/usr/bin/env python3
# Version: 1.0.0 · updated 26-08-23-12-36
"""
measure-network-cost.py — measure the REAL bytes an operation costs on the
wire, by watching a process's sockets closely enough to catch short-lived
connections before they close.

WHY THIS EXISTS. On 2026-08-23 a user opened this program, signed in to
Antigravity, and typed nothing at all. They ran /usage, which is the first
thing anyone does when they are worried about what they can afford. That
single command opened FOUR outbound HTTPS connections: two to Google, which
are legitimate (OAuth refresh and the Antigravity quota endpoint), and two to
providers the session was not using at all, because the balance check fetches
from every provider with credentials on disk rather than the active one.

It cost zero tokens. That is exactly why it matters. Because no quota is
consumed, this traffic is invisible on every meter the program shows you, and
it is paid BEFORE the first useful byte moves. On the connections this project
targets, single-digit KB/s satellite and metered mobile, bandwidth is the
scarce resource and tokens are the visible one. A cost that is real and
unmeasured is worse than a cost that is large and displayed.

There was no way to put a number on it, so this exists.

HOW IT WORKS. `ss -tnip` reports per-socket byte counters, but a metadata call
completes and closes in well under a second, so a one-shot reading sees
nothing. This samples every 150ms and keeps the high-water mark per peer, so
the connection is counted even though it is gone by the time you read the
output. Peers are reverse-resolved where possible so "which provider was that"
is answerable without a second tool.

It measures ONE process. It does not need root, does not capture packet
contents, and never decrypts anything: it reads counters the kernel already
keeps. It cannot tell you WHAT was sent, only how much and to whom.

USAGE
    ./measure-network-cost.py --seconds 120
    ./measure-network-cost.py --process codex --seconds 60 --json out.json
    ./measure-network-cost.py --pid 12345 --link-kbs 8

Start it, then perform the operation you want to price. It prints a per-peer
table, a total, and what that total costs in seconds on a metered link.

CAVEAT worth stating: byte counters include TLS and TCP overhead, which is what
you actually pay for, so these numbers are higher than the application-layer
payload and that is correct. Retransmissions are counted too. A number from a
lossy link is a real number for that link, not an error.
"""
import argparse
import json
import re
import socket
import subprocess
import sys
import time

SAMPLE_INTERVAL = 0.15  # short enough to catch a metadata call that lives <1s


def find_pid(pattern):
    out = subprocess.run(["pgrep", "-f", pattern], capture_output=True, text=True)
    pids = out.stdout.split()
    if not pids:
        sys.exit(f"No process matching {pattern!r}. Is it running?")
    return pids[0]


def resolve(ip, cache):
    if ip not in cache:
        try:
            cache[ip] = socket.gethostbyaddr(ip)[0]
        except Exception:
            cache[ip] = ""
    return cache[ip]


def sample(tag, peak):
    """One pass over ss output, updating high-water marks."""
    out = subprocess.run(["ss", "-tnip"], capture_output=True, text=True).stdout
    lines = out.splitlines()
    for i, ln in enumerate(lines):
        if not ln.startswith("ESTAB") or tag not in ln:
            continue
        detail = lines[i + 1] if i + 1 < len(lines) else ""
        sent = re.search(r"bytes_sent:(\d+)", detail)
        recv = re.search(r"bytes_received:(\d+)", detail)
        if not (sent or recv):
            continue
        peer = ln.split()[4]
        cur = peak.setdefault(peer, [0, 0])
        cur[0] = max(cur[0], int(sent.group(1)) if sent else 0)
        cur[1] = max(cur[1], int(recv.group(1)) if recv else 0)


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[1],
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--process", default="gorilla-opencode",
                    help="pgrep -f pattern for the process to watch")
    ap.add_argument("--pid", help="watch this exact pid instead of searching")
    ap.add_argument("--seconds", type=int, default=120, help="capture window")
    ap.add_argument("--link-kbs", type=float, default=8.0,
                    help="reference link speed in KB/s for the cost line (default 8, "
                         "the project's reference satellite uplink)")
    ap.add_argument("--json", help="also write the raw table here")
    args = ap.parse_args()

    pid = args.pid or find_pid(args.process)
    tag = f"pid={pid},"
    peak = {}

    print(f"watching pid {pid} for {args.seconds}s. Perform the operation now.",
          flush=True)
    end = time.time() + args.seconds
    while time.time() < end:
        sample(tag, peak)
        time.sleep(SAMPLE_INTERVAL)

    cache = {}
    rows = []
    for peer, (s, r) in sorted(peak.items(), key=lambda kv: -sum(kv[1])):
        rows.append({"peer": peer, "sent": s, "received": r, "total": s + r,
                     "host": resolve(peer.rsplit(":", 1)[0], cache)})

    if not rows:
        print("\nNo connections observed. Either the operation was not performed "
              "during the window, or it used an already-open socket.")
        return

    print(f"\n{'peer':<24}{'sent':>10}{'recv':>10}{'total':>10}  host")
    for row in rows:
        print(f"{row['peer']:<24}{row['sent']:>10}{row['received']:>10}"
              f"{row['total']:>10}  {row['host']}")
    ts = sum(r["sent"] for r in rows)
    tr = sum(r["received"] for r in rows)
    print(f"{'TOTAL':<24}{ts:>10}{tr:>10}{ts + tr:>10}")

    total = ts + tr
    secs = total / (args.link_kbs * 1024)
    print(f"\n{total / 1024:.1f} KiB across {len(rows)} connection(s).")
    print(f"At {args.link_kbs:g} KB/s that is {secs:.1f} seconds of a metered link.")

    if args.json:
        with open(args.json, "w") as f:
            json.dump({"pid": pid, "seconds": args.seconds, "connections": rows,
                       "total_bytes": total}, f, indent=2)
        print(f"wrote {args.json}")


main()
