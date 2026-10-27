#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Speed table: pfind.py (+ ripgrep) against pfind.exe, 5 runs each, median.

    python -I tools/pfind-rs/tests/bench.py [ROOT] [--runs 5] [--markdown]

ROOT defaults to the Gorilla.Opencode clone. Each query is what find.go sends
(internal/llm/tools/find.go): --no-color --count --limit 15 --max 3
--max-columns 500 --max-columns-preview, plus the mode flags. One warm-up run
per implementation and query is not counted (it fills the disk cache, so both
sides are measured warm). The two sides alternate run by run. Read-only: it
only searches ROOT.
"""
import argparse
import os
import statistics
import subprocess
import sys
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[2]
PFIND_PY = REPO / "internal" / "llm" / "tools" / "pfind.py"
EXE = REPO / "tools" / "pfind-rs" / "target" / "release" / ("pfind.exe" if os.name == "nt" else "pfind")
FINDGO = ["--no-color", "--count", "--limit", "15", "--max", "3", "--max-columns", "500", "--max-columns-preview"]


def queries(root):
    return [
        ("content + name search, -C 2 (find query=)", FINDGO + ["-C", "2", "--", "permission", root]),
        ("content, type filter (-t go)", FINDGO + ["--type", "go", "-C", "2", "--", "exec.CommandContext", root]),
        ("regex content search", FINDGO + ["--regex", "-C", "2", "--", r"func \w+Windows\(", root]),
        ("files only", FINDGO + ["--files-only", "--", "TODO", root]),
        ("listing (find with no query)", FINDGO + ["--files", "--max-depth", "2", "--", root]),
        ("--json ranked results", ["--json", "--limit", "15", "--max", "3", "--", "pfind", root]),
        ("--code language summary", FINDGO + ["--code", "--max-filesize", "2M", "--", root]),
    ]


def one(cmd):
    t = time.perf_counter()
    p = subprocess.run(cmd, capture_output=True)
    return time.perf_counter() - t, p.returncode


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("root", nargs="?", default=str(Path.home() / "Documents" / "Gorilla.Opencode"))
    ap.add_argument("--runs", type=int, default=5)
    ap.add_argument("--markdown", action="store_true")
    o = ap.parse_args()
    py = [sys.executable, "-I", str(PFIND_PY)]
    exe = [str(EXE)]
    rows = []
    for name, args in queries(o.root):
        one(py + args)
        one(exe + args)
        tp, te, codes = [], [], set()
        for _ in range(o.runs):
            d, rc = one(py + args)
            tp.append(d)
            codes.add(("py", rc))
            d, rc = one(exe + args)
            te.append(d)
            codes.add(("exe", rc))
        mp, me = statistics.median(tp), statistics.median(te)
        rows.append((name, mp, me, mp / me if me else float("inf"), sorted(codes)))
        print(f"{name}: pfind.py {mp*1000:.0f} ms, pfind.exe {me*1000:.0f} ms, x{mp/me:.1f}  exit codes {sorted(codes)}",
              file=sys.stderr)
    if o.markdown:
        print(f"| query (on {o.root}) | pfind.py + rg, median of {o.runs} | pfind.exe, median of {o.runs} | speed-up |")
        print("|---|---:|---:|---:|")
        for name, mp, me, x, _ in rows:
            print(f"| {name} | {mp*1000:.0f} ms | {me*1000:.0f} ms | {x:.1f}x |")
    return 0


if __name__ == "__main__":
    sys.exit(main())
