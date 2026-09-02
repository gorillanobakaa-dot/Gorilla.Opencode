#!/usr/bin/env python3
"""Position check: does a finding really point at the line it claims?

Regression note: an earlier version used a bare containment test, and because
"" is a substring of every string, a BLANK source line "verified" any claim at
all -- which meant the drift search never ran and the check was decorative.
The blank-line cases below exist to keep that from coming back.
"""
import json
import os
import sys
import tempfile
from typing import Any, Dict, List, Tuple

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import findings as f  # noqa: E402  # pylint: disable=import-error,wrong-import-position

SRC = "def a():\n    x = 1\n    return x\n\ndef b():\n    dangerous_call()\n    return 2\n"
#      1         2            3            4(blank) 5         6                   7

CASES: List[Tuple[str, Dict[str, Any], Tuple[str, Any, Any]]] = [
    # label,                      finding,                                              expect
    ("in range, no claim",        dict(line=2),                                          ("keep", 2, "x = 1")),
    ("line past EOF",             dict(line=999),                                        ("drop", None, None)),
    ("file does not exist",       dict(line=1, file="nope.py"),                          ("drop", None, None)),
    ("file-level, line=None",     dict(line=None),                                       ("keep", None, "")),
    ("claim matches line",        dict(line=6, snippet="dangerous_call()"),               ("keep", 6, "dangerous_call()")),
    ("drift over a BLANK line",   dict(line=4, snippet="dangerous_call()"),               ("keep", 6, "dangerous_call()")),
    ("drift by one",              dict(line=7, snippet="dangerous_call()"),               ("keep", 6, "dangerous_call()")),
    ("claim is fiction",          dict(line=2, snippet="launch_missiles()"),              ("drop", None, None)),
    ("whitespace-only claim",     dict(line=3, snippet="   "),                            ("keep", 3, "return x")),
    ("claim differs only by ws",  dict(line=2, snippet="x=1"),                            ("keep", 2, "x = 1")),
    ("drift beyond the window",   dict(line=1, snippet="return 2"),                       ("drop", None, None)),
]


def run() -> int:
    root = tempfile.mkdtemp()
    with open(os.path.join(root, "s.py"), "w") as fh:
        fh.write(SRC)

    failures = 0
    for label, kw, (verdict, exp_line, exp_snip) in CASES:
        kw.setdefault("file", "s.py")
        fi = f.Finding(tool="t", message="m", **kw)
        rep = f.verify_positions(f.finalize([fi]), root=root)
        got = "keep" if rep.kept else "drop"
        if got != verdict:
            failures += 1
            why = rep.reasons.get(fi.fingerprint, "")
            print(f"FAIL {label}: expected {verdict}, got {got} ({why})")
            continue
        if verdict == "keep":
            k = rep.kept[0]
            if k.line != exp_line or k.snippet != exp_snip:
                failures += 1
                print(f"FAIL {label}: expected line={exp_line} snippet={exp_snip!r}, "
                      f"got line={k.line} snippet={k.snippet!r}")
                continue
        print(f"ok   {label}")

    print(f"\n{len(CASES)} checks, {failures} failure(s)")
    return failures





# --- corroboration semantics -------------------------------------------------
# Kept here rather than in test_parsers.py because it's about how findings are
# COMBINED, not how they're extracted.

def run_corroboration() -> int:
    F = f.Finding
    failures = 0

    def check(label, items, expect_groups, expect_style):
        nonlocal failures
        got = f.corroborated(f.finalize(list(items)))
        style = f.style_agreements(f.finalize(list(items)))
        if len(got) != expect_groups or style != expect_style:
            failures += 1
            print(f"FAIL {label}: expected {expect_groups} group(s)/{expect_style} style, "
                  f"got {len(got)}/{style}")
        else:
            print(f"ok   {label}")

    check("two tools, real bug, same line -> 1 group",
          [F(tool="cppcheck", file="a.c", line=10, message="leak", severity="error"),
           F(tool="clang-tidy", file="a.c", line=10, message="leak", severity="warning")],
          1, 0)

    # The change this test exists for: linters agreeing about whitespace is not
    # evidence of a bug, and used to dominate the list.
    check("two linters agree on STYLE -> 0 groups, counted separately",
          [F(tool="flake8", file="a.py", line=5, message="line too long", severity="style"),
           F(tool="pylint", file="a.py", line=5, message="Line too long", severity="style")],
          0, 1)

    check("style + warning on one line -> NOT corroboration (unrelated findings)",
          [F(tool="flake8", file="a.py", line=7, message="line too long", severity="style"),
           F(tool="pylint", file="a.py", line=7, message="too many args", severity="warning")],
          0, 0)

    check("same tool twice -> not corroboration",
          [F(tool="cppcheck", file="b.c", line=3, message="x", severity="warning"),
           F(tool="cppcheck", file="b.c", line=3, message="y", severity="error")],
          0, 0)

    check("different lines -> not corroboration",
          [F(tool="cppcheck", file="c.c", line=1, message="x", severity="error"),
           F(tool="clang-tidy", file="c.c", line=2, message="y", severity="error")],
          0, 0)

    check("file-level findings (line=None) never group",
          [F(tool="black", file="d.py", line=None, message="unformatted", severity="warning"),
           F(tool="isort", file="d.py", line=None, message="unsorted", severity="warning")],
          0, 0)

    check("three tools on one line -> 1 group",
          [F(tool="bandit", file="e.py", line=9, message="shell=True", severity="error"),
           F(tool="flake8", file="e.py", line=9, message="subprocess", severity="warning"),
           F(tool="pylint", file="e.py", line=9, message="subprocess", severity="warning")],
          1, 0)

    print(f"\n7 corroboration checks, {failures} failure(s)")
    return failures




# --- secret redaction, end to end ---------------------------------------------
# The bug this guards: parse_gitleaks() withheld the secret AND a unit test
# asserted it, but verify_positions() then read the source at file:line and
# attached the credential as `snippet`. Both halves passed their own tests; the
# leak lived in the seam. So this test runs the PIPELINE, not a function.

def run_redaction() -> int:
    import tempfile as _tmp
    failures = 0
    root = _tmp.mkdtemp()
    SECRET = "GOCSPX-" + "n0tAr3alSecretButShaped1ikeOne"
    with open(os.path.join(root, "auth.go"), "w") as fh:
        fh.write("package auth\n\nconst clientSecret = \"%s\"\n" % SECRET)

    gitleaks_out = (
        'Finding:     clientSecret = "%s"\n'
        'Secret:      %s\n'
        'RuleID:      generic-api-key\n'
        'File:        auth.go\n'
        'Line:        3\n'
    ) % (SECRET, SECRET)

    for tool in ("gitleaks-worktree", "gitleaks-history"):
        ctx: dict = {"tool_id": tool, "cwd": root, "target_path": "auth.go",
                     "severity_markers": []}
        parsed = f.finalize(f.parse_gitleaks(gitleaks_out, ctx), cwd=root)
        rep = f.verify_positions(parsed, root=root)
        everything = json.dumps([x.to_dict() for x in rep.kept + rep.dropped])
        if SECRET in everything:
            failures += 1
            print(f"FAIL {tool}: SECRET LEAKED into the finding record")
        else:
            print(f"ok   {tool}: secret absent from the whole record after position check")

    # A history finding must survive even though its line may not match the
    # current file, and must not carry current source as evidence.
    ctx = {"tool_id": "gitleaks-history", "cwd": root, "target_path": "auth.go",
           "severity_markers": []}
    hist = f.finalize(f.parse_gitleaks(
        'Finding:     x\nSecret:      %s\nRuleID:      generic-api-key\n'
        'File:        auth.go\nLine:        9999\n' % SECRET, ctx), cwd=root)
    rep = f.verify_positions(hist, root=root)
    if len(rep.kept) == 1 and rep.kept[0].snippet == "":
        print("ok   history finding past EOF kept, no fabricated snippet")
    else:
        failures += 1
        print(f"FAIL history finding: kept={len(rep.kept)} "
              f"snippet={rep.kept[0].snippet if rep.kept else 'n/a'!r}")

    # A NON-secret finding must still get its snippet -- redaction must not
    # become a blanket that disables the position check for everything.
    normal = f.finalize([f.Finding(tool="cppcheck", file="auth.go", line=3,
                                   message="something", severity="warning")], cwd=root)
    rep = f.verify_positions(normal, root=root)
    if rep.kept and rep.kept[0].snippet:
        print("ok   non-secret finding still gets its snippet attached")
    else:
        failures += 1
        print("FAIL non-secret finding lost its snippet")


    # The tool-agnostic half. The first redaction fix keyed off the tool name
    # (gitleaks) and a real repo immediately leaked the same secret via gosec's
    # G101 "hardcoded credentials". Any tool can report a credential, so these
    # cases assert on the finding's NATURE.
    for tool, rule, msg in [
        ("gosec",    "G101 (CWE-798)",  "Potential hardcoded credentials"),
        ("bandit",   "B105",            "Possible hardcoded password"),
        ("semgrep",  "generic.secrets.detected-generic-api-key", "API key detected"),
        ("mystery",  "",                "Detected secret in source"),
    ]:
        fi = f.Finding(tool=tool, file="auth.go", line=3, message=msg,
                       severity="error", rule_id=rule,
                       raw='const clientSecret = "%s"' % SECRET)
        rep = f.verify_positions(f.finalize([fi], cwd=root), root=root)
        blob = json.dumps([x.to_dict() for x in rep.kept + rep.dropped])
        if SECRET in blob:
            failures += 1
            print(f"FAIL {tool}/{rule or 'no-rule'}: SECRET LEAKED via a non-gitleaks tool")
        else:
            print(f"ok   {tool}/{rule or 'message-only'}: redacted by category, not tool name")

    print(f"\n8 redaction checks, {failures} failure(s)")
    return failures


if __name__ == "__main__":
    sys.exit(1 if (run() + run_corroboration() + run_redaction()) else 0)
