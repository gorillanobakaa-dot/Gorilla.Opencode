#!/usr/bin/env python3
"""Tests for the apply ladder in patch_port.py.

The ladder is the whole point of the tool, so these tests build REAL git
repositories and run REAL patches through it. A mock would happily agree with
whatever the code does; git will not.

The three cases below are the three ways a forward-port ends, and telling them
apart is exactly what the tool exists to do:

  1. the hunk still fits, but upstream shifted it   -> FUZZ, and it must land
  2. upstream rewrote the line the patch depends on -> refused, tree untouched
  3. upstream already has the change                -> ALREADY, not re-applied

Case 2 is the one that matters most. Fuzz is the rung that can be silently
wrong, so a test that only proved "fuzz makes things apply" would be actively
harmful -- it would pass just as well for a version that force-applied
everything. Case 2 pins down what fuzz must still REFUSE.
"""

import importlib.util
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
PP_PATH = os.path.join(os.path.dirname(HERE), "patch_port.py")

_spec = importlib.util.spec_from_file_location("patch_port", PP_PATH)
pp = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(pp)

FAILURES = []


def ok(name, detail=""):
    print("ok   %-14s %s" % (name, detail))


def bad(name, detail):
    print("FAIL %-14s %s" % (name, detail))
    FAILURES.append(name)


def git(args, cwd):
    return subprocess.run(["git"] + args, cwd=cwd, capture_output=True,
                          text=True, errors="replace")


def make_repo(path, content, message="base"):
    """A repo with one file, deterministic identity, and CRLF conversion off.

    core.autocrlf is set explicitly because on Windows the global setting is
    routinely 'true', which rewrites line endings on checkout and makes every
    hunk fail for a reason that has nothing to do with the patch.
    """
    os.makedirs(path, exist_ok=True)
    git(["init", "-q"], path)
    git(["config", "user.email", "t@example.invalid"], path)
    git(["config", "user.name", "t"], path)
    git(["config", "core.autocrlf", "false"], path)
    git(["config", "commit.gpgsign", "false"], path)
    write(os.path.join(path, "f.c"), content)
    git(["add", "-A"], path)
    git(["commit", "-qm", message], path)


def write(path, text):
    with open(path, "w", encoding="utf-8", newline="\n") as fh:
        fh.write(text)


def read(path):
    with open(path, encoding="utf-8", newline="") as fh:
        return fh.read()


BASE = "void init(void)\n{\n\tlock();\n\tsetup();\n\tunlock();\n}\n"
FIXED = "void init(void)\n{\n\tlock();\n\tsetup_v2();\n\tunlock();\n}\n"


def build_patch(root):
    """Produce a one-commit patch series in <root>/patches."""
    up = os.path.join(root, "up")
    make_repo(up, BASE)
    write(os.path.join(up, "f.c"), FIXED)
    git(["commit", "-qam", "use setup_v2"], up)
    out = os.path.join(root, "patches")
    git(["format-patch", "-1", "-o", out, "-q"], up)
    return out


def port(tree, series):
    """Run the real forward-port entry point against a real tree."""
    t = pp.Tree(tree)
    patches = pp.load_series(series) if hasattr(pp, "load_series") else None
    if patches is None:
        names = sorted(f for f in os.listdir(series) if f.endswith(".patch"))
        patches = [pp.Patch(os.path.join(series, n)) for n in names]
    return [pp.apply_patch(t, p) for p in patches]


def case_shifted():
    """Upstream inserted lines above the hunk. It must still land."""
    root = tempfile.mkdtemp(prefix="pp_shift_")
    try:
        series = build_patch(root)
        tgt = os.path.join(root, "tgt")
        make_repo(tgt, "#include <linux/module.h>\n\n" + BASE + "\nint tail;\n",
                  "upstream drifted")
        results = port(tgt, series)
        r = results[0]
        landed = "setup_v2" in read(os.path.join(tgt, "f.c"))
        if not landed:
            bad("shifted", "hunk did not land: status=%s" % r.status)
        elif r.status not in (pp.ApplyResult.FUZZ, pp.ApplyResult.THREE_WAY,
                              pp.ApplyResult.CLEAN):
            bad("shifted", "landed but mislabelled: %s" % r.status)
        elif "#include <linux/module.h>" not in read(os.path.join(tgt, "f.c")):
            bad("shifted", "upstream's own lines were destroyed")
        else:
            ok("shifted", "ported onto drifted tree, reported as %s" % r.status)
    finally:
        shutil.rmtree(root, ignore_errors=True)


def case_real_conflict():
    """Upstream rewrote the line. Fuzz must NOT paper over this."""
    root = tempfile.mkdtemp(prefix="pp_conflict_")
    try:
        series = build_patch(root)
        tgt = os.path.join(root, "tgt")
        # every line the hunk depends on is now different
        make_repo(tgt, "void init(void)\n{\n\tmutex_lock(&m);\n"
                       "\tconfigure_hardware();\n\tmutex_unlock(&m);\n}\n",
                  "rewritten upstream")
        before = read(os.path.join(tgt, "f.c"))
        results = port(tgt, series)
        r = results[0]
        after = read(os.path.join(tgt, "f.c"))
        if "setup_v2" in after:
            bad("conflict", "applied a patch whose context no longer exists")
        elif r.status in (pp.ApplyResult.CLEAN, pp.ApplyResult.FUZZ):
            bad("conflict", "claimed success (%s) on a real conflict" % r.status)
        elif before != after:
            bad("conflict", "tree was modified by a failed apply")
        else:
            ok("conflict", "refused (%s), tree left untouched" % r.status)
    finally:
        shutil.rmtree(root, ignore_errors=True)


def case_already():
    """Upstream already carries the change: say so, do not re-apply."""
    root = tempfile.mkdtemp(prefix="pp_already_")
    try:
        series = build_patch(root)
        tgt = os.path.join(root, "tgt")
        make_repo(tgt, FIXED, "already fixed upstream")
        results = port(tgt, series)
        r = results[0]
        if r.status != pp.ApplyResult.ALREADY:
            bad("already", "status=%s, want %s" % (r.status, pp.ApplyResult.ALREADY))
        elif read(os.path.join(tgt, "f.c")) != FIXED:
            bad("already", "tree was modified when nothing needed doing")
        else:
            ok("already", "detected before anything was applied")
    finally:
        shutil.rmtree(root, ignore_errors=True)


def case_fuzz_is_last():
    """Structural: fuzz must sit BELOW three-way, never above it.

    Checked by reading the source, because the ordering is the safety
    property and it is easy to 'optimise' the ladder into the wrong order
    later. Three-way reasons about the real blob; fuzz only pattern-matches.
    """
    src = read(PP_PATH)
    body = src[src.index("def apply_patch("):]
    body = body[:body.index("\ndef ", 1)] if "\ndef " in body[1:] else body
    i3 = body.find("--3way")
    ifz = body.find("apply_with_fuzz")
    if i3 < 0 or ifz < 0:
        bad("ladder-order", "could not find both rungs in apply_patch")
    elif ifz < i3:
        bad("ladder-order", "fuzz is attempted BEFORE three-way")
    else:
        ok("ladder-order", "three-way is tried before fuzz")


def main():
    if not shutil.which("git"):
        print("git not on PATH; skipping")
        return 0
    case_shifted()
    case_real_conflict()
    case_already()
    case_fuzz_is_last()
    print()
    print("%d patch-port checks, %d failure(s)" % (4, len(FAILURES)))
    return 1 if FAILURES else 0


if __name__ == "__main__":
    sys.exit(main())
