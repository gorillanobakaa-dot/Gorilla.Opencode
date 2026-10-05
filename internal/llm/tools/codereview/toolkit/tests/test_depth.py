#!/usr/bin/env python3
"""Depth, command resolution and network notes: what a run says must be what it does.

Regression note (2026-10-05). Four statements made about a review were not
connected to the code that would have made them true:

  1. "quick = linters and formatters only; the security stages were skipped
     entirely". Quick was --no-stage3, and stages 1-2 ran in full: bandit,
     gosec, cargo audit, semgrep and gitleaks all ran on a "quick" pass.
  2. "<tool> is not installed". On Windows a bare name that is a .cmd shim
     raised FileNotFoundError, so everything npm installs was reported missing
     while installed.
  3. "nothing is downloaded when you run it". semgrep fetched rule packs and
     reported metrics; nothing in the registry recorded that any tool touched
     the network.
  4. The raw gitleaks log kept every secret it found, in full, on disk.

Each case below fails if the corresponding fix is taken out.
"""
import argparse
import os
import stat
import sys
import tempfile

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import code_review as cr  # noqa: E402  # pylint: disable=import-error,wrong-import-position
import doctor  # noqa: E402  # pylint: disable=import-error,wrong-import-position
import tools_registry as reg  # noqa: E402  # pylint: disable=import-error,wrong-import-position


def check(label, ok, detail=""):
    print(f"  {'ok  ' if ok else 'FAIL'} {label}" + ("" if ok or not detail else f": {detail}"))
    return bool(ok)


def every_language_files(root):
    """One file per language the registry knows, so every tool is 'relevant'."""
    files = []
    seen = set()
    for ext, lang in sorted(reg.EXTENSION_LANGUAGE.items()):
        if lang in seen:
            continue
        seen.add(lang)
        p = os.path.join(root, "f" + ext)
        with open(p, "w", encoding="utf-8") as f:
            f.write("\n")
        files.append(p)
    return files


def main() -> int:
    good = True

    with tempfile.TemporaryDirectory() as root, tempfile.TemporaryDirectory() as results:
        files = every_language_files(root)
        args = argparse.Namespace(compile_commands_dir=None)
        ctx = cr.build_ctx(root, results, args, profile="generic")

        # 1. A quick pass schedules linters and formatters and NOTHING else.
        quick = cr.build_jobs(files, root, "generic", results, ctx, stages=(0, 1, 2),
                              categories=reg.QUICK_CATEGORIES)
        cats = {reg.TOOLS_BY_ID[j.tool_id].category for j in quick}
        good &= check("quick: only quick categories are scheduled",
                      cats <= set(reg.QUICK_CATEGORIES), f"got {sorted(cats)}")
        good &= check("quick: something IS scheduled", len(quick) > 0)
        for banned in ("security", "secrets", "static-analysis"):
            ids = sorted({j.tool_id for j in quick
                          if reg.TOOLS_BY_ID[j.tool_id].category == banned})
            good &= check(f"quick: no {banned} tool is scheduled", not ids, f"scheduled {ids}")

        # The standard pass, by contrast, does schedule them -- otherwise the
        # cases above would pass on a registry that had simply lost its tools.
        std = cr.build_jobs(files, root, "generic", results, ctx, stages=(0, 1, 2))
        std_cats = {reg.TOOLS_BY_ID[j.tool_id].category for j in std}
        good &= check("standard: security tools are scheduled", "security" in std_cats)
        good &= check("standard: secret scan is scheduled", "secrets" in std_cats)

        # 2. The run reports its own depth, and names what the depth left out.
        d = cr.depth_of(argparse.Namespace(quick=True, deep=False), "generic", files)
        good &= check("depth: quick is reported as quick", d["mode"] == "quick")
        skipped = set(d["tools_skipped_by_depth"])
        scheduled = {j.tool_id for j in quick}
        good &= check("depth: skipped and scheduled are disjoint", not (skipped & scheduled),
                      f"both: {sorted(skipped & scheduled)}")
        every_auto = {t.id for t in reg.TOOLS if t.scope in ("auto-file", "auto-project")
                      and t.build_cmd is not None}
        # Tools whose build_cmd declines this tree (profile-specific heuristics)
        # are neither scheduled nor skipped-by-depth in a way that matters, so
        # only the direction that would hide a tool is asserted.
        unaccounted = {i for i in every_auto - scheduled - skipped
                       if reg.TOOLS_BY_ID[i].category in reg.QUICK_CATEGORIES
                       and reg.TOOLS_BY_ID[i].stage < 3
                       and reg.TOOLS_BY_ID[i].build_cmd(root, ctx)}
        good &= check("depth: no quick-category tool silently unaccounted for",
                      not unaccounted, f"{sorted(unaccounted)}")
        good &= check("depth: security is listed as skipped entirely",
                      "security" in d["categories_skipped"])
        good &= check("depth: a category that ran is never listed as skipped",
                      not (set(d["categories_skipped"]) & set(reg.QUICK_CATEGORIES)),
                      f"{d['categories_skipped']}")
        s = cr.depth_of(argparse.Namespace(quick=False, deep=False), "generic", files)
        good &= check("depth: standard skips nothing by depth",
                      s["mode"] == "standard" and not s["tools_skipped_by_depth"])

        # 3. Network notes: the report carries every relevant tool that has one.
        rep = cr.network_report(root)
        got = {t["id"] for t in rep["tools"]}
        want = {t.id for t in reg.TOOLS if t.network and t.scope in ("auto-file", "auto-project")}
        good &= check("network: every tool with a network note is reported", got == want,
                      f"missing {sorted(want - got)}, extra {sorted(got - want)}")
        good &= check("network: each entry says whether the tool is installed",
                      all(isinstance(t["installed"], bool) for t in rep["tools"]))

    # A tool that reaches the network without saying so is the defect itself.
    for t in reg.TOOLS:
        if t.build_cmd is None or t.scope not in ("auto-file", "auto-project"):
            continue
        try:
            argv = t.build_cmd("x", {}) or []
        except Exception:  # pylint: disable=broad-except
            argv = []
        if argv[:1] == ["semgrep"]:
            good &= check(f"{t.id}: semgrep runs with --metrics=off", "--metrics=off" in argv)
            good &= check(f"{t.id}: semgrep carries a network note", bool(t.network))
        if argv[:1] == ["gitleaks"]:
            good &= check(f"{t.id}: gitleaks redacts secrets in its own log", "--redact" in argv)
        if argv[:2] == ["cargo", "audit"]:
            good &= check(f"{t.id}: cargo audit carries a network note", bool(t.network))

    # 4. A program found through PATHEXT (Windows) or PATH (everywhere) resolves.
    with tempfile.TemporaryDirectory() as bindir:
        if os.name == "nt":
            shim = os.path.join(bindir, "gorilla-shim-probe.cmd")
            with open(shim, "w", encoding="ascii", newline="\r\n") as f:
                f.write("@echo off\necho shim-ran\n")
        else:
            shim = os.path.join(bindir, "gorilla-shim-probe")
            with open(shim, "w", encoding="ascii") as f:
                f.write("#!/bin/sh\necho shim-ran\n")
            os.chmod(shim, os.stat(shim).st_mode | stat.S_IXUSR)
        old = os.environ.get("PATH", "")
        os.environ["PATH"] = bindir + os.pathsep + old
        try:
            rc, out, _ = cr.run(["gorilla-shim-probe"])
            good &= check("resolve: a shim on PATH runs by its bare name",
                          rc == 0 and "shim-ran" in out, f"rc={rc} out={out!r}")
            good &= check("resolve: the doctor sees the same shim",
                          doctor._run_check(["gorilla-shim-probe"]) == 0)  # pylint: disable=protected-access
        finally:
            os.environ["PATH"] = old
    rc, _, _ = cr.run(["gorilla-definitely-not-installed-xyz"])
    good &= check("resolve: a missing program is still 127", rc == 127, f"rc={rc}")
    good &= check("resolve: the doctor agrees it is missing",
                  doctor._run_check(["gorilla-definitely-not-installed-xyz"]) == 127)  # pylint: disable=protected-access

    print("PASS" if good else "FAILED")
    return 0 if good else 1


if __name__ == "__main__":
    sys.exit(main())
