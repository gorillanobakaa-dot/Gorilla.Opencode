#!/usr/bin/env python3
"""
code_review.py — an orchestrator for real static analysers, and a method.

WHAT THIS IS, AND WHAT IT IS NOT
================================

This script does not review code. It cannot: no program that fits in one file
knows whether your pointer arithmetic is sound. What it does is drive the
analysers that DO know — cppcheck, clang-tidy, bandit, semgrep, gitleaks,
clippy, golangci-lint, gosec, shellcheck, ruff, mypy and others — normalise
their very different outputs into one shape, verify that what they reported is
still true of the file on disk, and then report, first and loudest, WHAT DID NOT
RUN.

That last part is the whole reason this exists.

A report full of "not installed" looks exactly like a report that found no
problems. Both are short. Both are green. A reader in a hurry, and every
language model, will read the second meaning into the first unless something
stops them. So the JSON this emits leads with a `trust` block, the caller prints
that block before any finding, and the word MISSING appears before the word
clean.

THE METHOD, WHICH IS THE PART WORTH COPYING
===========================================

Written down because a reviewer — human or model — who follows these five rules
will outperform one who runs more tools.

1. ABSENCE OF EVIDENCE IS NOT EVIDENCE OF ABSENCE.
   If bandit is not installed, this review says nothing whatever about Python
   security. Not "no issues found" — nothing. Every stage records which tools
   ran, which were missing, which errored and which timed out, and the caller
   is expected to say so out loud.

2. VERIFY THE FINDING AGAINST THE FILE.
   Analysers report line numbers against the file as it was when they read it.
   Between the run and the report the file may have moved on, and a stale line
   number points at innocent code. Every finding here is re-read from disk and
   dropped if the line no longer exists. The count of dropped findings is
   reported, because a silent drop is its own kind of lie.

3. AGREEMENT IS EVIDENCE; A SINGLE OPINION IS A LEAD.
   When two independent analysers flag the same line, that is worth acting on.
   When one does, it is worth looking at. These are separated in the output —
   `corroborated` and `findings` — because ranking them together invites acting
   on a lint preference with the same urgency as a buffer overrun.

4. SEVERITY IS A CLAIM, NOT A FACT.
   Every tool grades on its own curve. Normalising them into one scale is
   necessary and slightly dishonest, so the original rule id travels with each
   finding and the reader can go and check.

5. SAY WHAT YOU DID NOT LOOK AT.
   A quick pass that does not announce it skipped the security stage is the same
   lie as an uninstalled analyser. Depth is reported, not implied.

WINDOWS
=======

This runs on Windows first. It assumes no Unix userland: no grep, no find, no
awk, no /bin/sh. Tool discovery goes through shutil.which, which honours PATHEXT
and so finds .exe, .cmd and .bat wrappers — the form most Node- and Python-based
analysers take on Windows. Every subprocess is spawned without a shell, so paths
containing spaces (C:\\Program Files\\...) need no quoting and cannot be
re-interpreted. Nothing here shells out to a POSIX utility.

USAGE
=====

    python code_review.py TARGET --audience agent [--diff REF] [--max-files N]
                                 [--profile NAME] [--deep | --no-stage3]
    python code_review.py TARGET --doctor

The first prints one JSON object on stdout, schema code-review/agent/1.
The second prints a human-readable capability report and exits 3 if this
machine cannot review the target at all.

Exit codes:  0 ran   2 bad usage   3 nothing installed to run
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import time
from collections import defaultdict

SCHEMA = "code-review/agent/1"

# How long any single analyser may take before it is abandoned. A tool that
# hangs must not take the review with it: a partial report that says which tool
# timed out is worth more than no report at all.
TOOL_TIMEOUT_SECONDS = 120

# Upper bound on files handed to any one analyser invocation. Command lines have
# limits — about 32k characters on Windows — and exceeding them fails in a way
# that looks like the tool being broken.
FILES_PER_INVOCATION = 40


# ─────────────────────────────────────────────────────────────────────────────
#  Languages
# ─────────────────────────────────────────────────────────────────────────────

EXT_LANG = {
    ".c": "c", ".h": "c",
    ".cc": "cpp", ".cpp": "cpp", ".cxx": "cpp", ".hpp": "cpp", ".hh": "cpp",
    ".py": "python", ".pyi": "python",
    ".go": "go",
    ".rs": "rust",
    ".js": "javascript", ".jsx": "javascript", ".mjs": "javascript",
    ".ts": "typescript", ".tsx": "typescript",
    ".sh": "shell", ".bash": "shell",
    ".ps1": "powershell", ".psm1": "powershell",
    ".rb": "ruby",
    ".php": "php",
    ".java": "java",
    ".cs": "csharp",
    ".json": "json", ".yaml": "yaml", ".yml": "yaml", ".toml": "toml",
    ".md": "markdown",
}

# Directories never worth reviewing. Reviewing a vendored dependency wastes the
# run and buries your own findings under someone else's.
SKIP_DIRS = {
    ".git", ".svn", ".hg", "node_modules", "vendor", "target", "dist", "build",
    "__pycache__", ".venv", "venv", ".tox", ".mypy_cache", ".pytest_cache",
    ".idea", ".vscode", "bin", "obj", ".next", ".cache", "site-packages",
}


# ─────────────────────────────────────────────────────────────────────────────
#  The analyser registry
# ─────────────────────────────────────────────────────────────────────────────
#
# stage 0  recon        — cheap, always
# stage 1  fast linters — seconds; style and obvious defects
# stage 2  static + security — the ones that find real bugs
# stage 3  deep         — slow, thorough; skipped unless asked
#
# `parser` names the output shape. Adding a tool means adding a row here and, if
# its output is not already one of the known shapes, one parser function. It
# does NOT mean touching the orchestration.

class Tool:
    def __init__(self, name, binary, langs, stage, args, parser,
                 per_file=True, install=None, note=None):
        self.name = name
        self.binary = binary
        self.langs = set(langs)
        self.stage = stage
        self.args = args
        self.parser = parser
        self.per_file = per_file
        self.install = install or ""
        self.note = note or ""


TOOLS = [
    Tool("ruff", "ruff", ["python"], 1,
         ["check", "--output-format", "json"], "ruff",
         install="pip install ruff"),
    Tool("mypy", "mypy", ["python"], 2,
         ["--no-error-summary", "--no-color-output"], "gcc_like",
         install="pip install mypy"),
    Tool("bandit", "bandit", ["python"], 2,
         ["-f", "json", "-q"], "bandit",
         install="pip install bandit",
         note="security; without it this review says NOTHING about Python security"),
    Tool("pyflakes", "pyflakes", ["python"], 1, [], "gcc_like",
         install="pip install pyflakes"),

    Tool("gofmt", "gofmt", ["go"], 1, ["-l"], "filelist"),
    Tool("go vet", "go", ["go"], 2, ["vet"], "gcc_like", per_file=False,
         install="ships with Go"),
    Tool("staticcheck", "staticcheck", ["go"], 2, [], "gcc_like",
         install="go install honnef.co/go/tools/cmd/staticcheck@latest"),
    Tool("golangci-lint", "golangci-lint", ["go"], 3,
         ["run", "--out-format", "json"], "golangci", per_file=False,
         install="https://golangci-lint.run/usage/install/"),
    Tool("gosec", "gosec", ["go"], 2, ["-fmt=json", "-quiet"], "gosec",
         per_file=False, install="go install github.com/securego/gosec/v2/cmd/gosec@latest",
         note="security; without it this review says NOTHING about Go security"),

    Tool("cppcheck", "cppcheck", ["c", "cpp"], 2,
         ["--enable=warning,style,performance,portability",
          "--template={file}:{line}:{severity}:{id}:{message}", "--quiet"],
         "cppcheck", install="https://cppcheck.sourceforge.io/"),
    Tool("clang-tidy", "clang-tidy", ["c", "cpp"], 3, [], "gcc_like",
         install="part of LLVM"),

    Tool("clippy", "cargo", ["rust"], 2,
         ["clippy", "--message-format", "short"], "gcc_like", per_file=False,
         install="rustup component add clippy"),

    Tool("eslint", "eslint", ["javascript", "typescript"], 1,
         ["--format", "json"], "eslint",
         install="npm i -g eslint"),
    Tool("tsc", "tsc", ["typescript"], 2, ["--noEmit", "--pretty", "false"],
         "gcc_like", per_file=False, install="npm i -g typescript"),

    Tool("shellcheck", "shellcheck", ["shell"], 1, ["--format", "json"],
         "shellcheck", install="https://www.shellcheck.net/"),

    Tool("gitleaks", "gitleaks", ["*"], 2,
         ["detect", "--no-banner", "--report-format", "json", "--report-path", "-"],
         "gitleaks", per_file=False,
         install="https://github.com/gitleaks/gitleaks",
         note="secrets; without it nothing here looked for leaked credentials"),
    Tool("semgrep", "semgrep", ["*"], 3,
         ["--json", "--quiet", "--config", "auto"], "semgrep", per_file=False,
         install="pip install semgrep",
         note="security; the deep cross-language pass"),
]


# ─────────────────────────────────────────────────────────────────────────────
#  Discovery
# ─────────────────────────────────────────────────────────────────────────────

def find_binary(name):
    """Locate an executable, honouring Windows PATHEXT.

    shutil.which consults PATHEXT on Windows, so `eslint` finds `eslint.cmd`
    and `ruff` finds `ruff.exe`. Most Node-based analysers exist ONLY as .cmd
    shims there, and a naive existence check for the bare name finds nothing
    and reports a perfectly installed tool as missing.
    """
    return shutil.which(name)


def collect_files(target, max_files, diff_ref):
    """Return (files, languages). Never follows into SKIP_DIRS."""
    target = os.path.abspath(target)
    files = []

    if diff_ref:
        changed = git_changed_files(target, diff_ref)
        if changed is not None:
            files = [f for f in changed if os.path.isfile(f)]

    if not files:
        if os.path.isfile(target):
            files = [target]
        else:
            for root, dirs, names in os.walk(target):
                dirs[:] = [d for d in dirs if d not in SKIP_DIRS and not d.startswith(".")]
                for n in names:
                    ext = os.path.splitext(n)[1].lower()
                    if ext in EXT_LANG:
                        files.append(os.path.join(root, n))

    files.sort()
    if max_files and len(files) > max_files:
        files = files[:max_files]

    langs = sorted({EXT_LANG[os.path.splitext(f)[1].lower()]
                    for f in files if os.path.splitext(f)[1].lower() in EXT_LANG})
    return files, langs


def git_changed_files(target, ref):
    """Files changed against ref, or None if git cannot answer.

    Returns None rather than [] on failure. They mean different things: [] is
    "nothing changed", None is "we could not find out", and treating the second
    as the first would silently review nothing and call it clean.
    """
    git = find_binary("git")
    if not git:
        return None
    root = target if os.path.isdir(target) else os.path.dirname(target)
    try:
        out = subprocess.run(
            [git, "diff", "--name-only", ref],
            cwd=root, capture_output=True, text=True,
            timeout=30, errors="replace",
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if out.returncode != 0:
        return None
    return [os.path.join(root, line.strip())
            for line in out.stdout.splitlines() if line.strip()]


# ─────────────────────────────────────────────────────────────────────────────
#  Output parsers
# ─────────────────────────────────────────────────────────────────────────────
#
# Each returns a list of dicts: tool, file, line, severity, message, rule.
# A parser that cannot understand its input returns [] and the caller records
# the tool under `tools_without_parser` — visibly, because a tool whose output
# we silently failed to read is indistinguishable from one that found nothing.

GCC_LIKE = re.compile(
    r"^(?P<file>(?:[A-Za-z]:)?[^:]+):(?P<line>\d+)(?::(?P<col>\d+))?:\s*"
    r"(?P<sev>error|warning|note|info)?:?\s*(?P<msg>.*)$"
)


def parse_gcc_like(tool, raw):
    out = []
    for line in raw.splitlines():
        m = GCC_LIKE.match(line.strip())
        if not m:
            continue
        out.append({
            "tool": tool, "file": m.group("file"), "line": int(m.group("line")),
            "severity": normalise_severity(m.group("sev") or "warning"),
            "message": m.group("msg").strip(), "rule": "",
        })
    return out


def parse_filelist(tool, raw):
    """gofmt -l prints only the names of files that need reformatting."""
    return [{"tool": tool, "file": ln.strip(), "line": 1, "severity": "low",
             "message": "file is not formatted (gofmt -l listed it)", "rule": "gofmt"}
            for ln in raw.splitlines() if ln.strip()]


def parse_ruff(tool, raw):
    out = []
    for it in _json_or_empty(raw, list):
        out.append({
            "tool": tool, "file": it.get("filename", ""),
            "line": int((it.get("location") or {}).get("row", 1)),
            "severity": "medium", "message": it.get("message", ""),
            "rule": it.get("code") or "",
        })
    return out


def parse_bandit(tool, raw):
    doc = _json_or_empty(raw, dict)
    out = []
    for it in doc.get("results", []):
        out.append({
            "tool": tool, "file": it.get("filename", ""),
            "line": int(it.get("line_number", 1)),
            "severity": normalise_severity(it.get("issue_severity", "medium")),
            "message": it.get("issue_text", ""),
            "rule": it.get("test_id", ""),
        })
    return out


def parse_eslint(tool, raw):
    out = []
    for f in _json_or_empty(raw, list):
        for m in f.get("messages", []):
            out.append({
                "tool": tool, "file": f.get("filePath", ""),
                "line": int(m.get("line", 1)),
                "severity": "high" if m.get("severity") == 2 else "low",
                "message": m.get("message", ""), "rule": m.get("ruleId") or "",
            })
    return out


def parse_shellcheck(tool, raw):
    return [{
        "tool": tool, "file": it.get("file", ""), "line": int(it.get("line", 1)),
        "severity": normalise_severity(it.get("level", "warning")),
        "message": it.get("message", ""), "rule": "SC%s" % it.get("code", ""),
    } for it in _json_or_empty(raw, list)]


def parse_cppcheck(tool, raw):
    out = []
    for line in raw.splitlines():
        parts = line.split(":", 4)
        if len(parts) < 5:
            continue
        # Windows paths start "C:\..." so the drive letter splits first.
        if len(parts[0]) == 1 and parts[0].isalpha():
            parts = [parts[0] + ":" + parts[1]] + parts[2:]
            if len(parts) < 5:
                continue
        try:
            ln = int(parts[1])
        except ValueError:
            continue
        out.append({"tool": tool, "file": parts[0], "line": ln,
                    "severity": normalise_severity(parts[2]),
                    "message": parts[4].strip(), "rule": parts[3]})
    return out


def parse_gosec(tool, raw):
    doc = _json_or_empty(raw, dict)
    return [{
        "tool": tool, "file": it.get("file", ""),
        "line": int(str(it.get("line", "1")).split("-")[0]),
        "severity": normalise_severity(it.get("severity", "medium")),
        "message": it.get("details", ""), "rule": it.get("rule_id", ""),
    } for it in doc.get("Issues", [])]


def parse_golangci(tool, raw):
    doc = _json_or_empty(raw, dict)
    out = []
    for it in (doc.get("Issues") or []):
        pos = it.get("Pos") or {}
        out.append({"tool": tool, "file": pos.get("Filename", ""),
                    "line": int(pos.get("Line", 1)), "severity": "medium",
                    "message": it.get("Text", ""), "rule": it.get("FromLinter", "")})
    return out


def parse_gitleaks(tool, raw):
    return [{
        "tool": tool, "file": it.get("File", ""), "line": int(it.get("StartLine", 1)),
        "severity": "critical",
        "message": "possible secret: %s" % it.get("Description", "credential detected"),
        "rule": it.get("RuleID", ""),
    } for it in _json_or_empty(raw, list)]


def parse_semgrep(tool, raw):
    doc = _json_or_empty(raw, dict)
    out = []
    for it in doc.get("results", []):
        extra = it.get("extra") or {}
        out.append({
            "tool": tool, "file": it.get("path", ""),
            "line": int((it.get("start") or {}).get("line", 1)),
            "severity": normalise_severity(extra.get("severity", "medium")),
            "message": (extra.get("message") or "").strip(),
            "rule": it.get("check_id", ""),
        })
    return out


PARSERS = {
    "gcc_like": parse_gcc_like, "filelist": parse_filelist, "ruff": parse_ruff,
    "bandit": parse_bandit, "eslint": parse_eslint, "shellcheck": parse_shellcheck,
    "cppcheck": parse_cppcheck, "gosec": parse_gosec, "golangci": parse_golangci,
    "gitleaks": parse_gitleaks, "semgrep": parse_semgrep,
}


def _json_or_empty(raw, want):
    """Parse JSON, tolerating the preamble some tools print before it."""
    raw = raw.strip()
    if not raw:
        return want()
    try:
        doc = json.loads(raw)
    except ValueError:
        start = raw.find("[" if want is list else "{")
        if start < 0:
            return want()
        try:
            doc = json.loads(raw[start:])
        except ValueError:
            return want()
    return doc if isinstance(doc, want) else want()


SEVERITY_MAP = {
    "critical": "critical", "error": "high", "high": "high", "danger": "high",
    "warning": "medium", "medium": "medium", "style": "low", "info": "low",
    "note": "low", "low": "low", "performance": "low", "portability": "low",
    "convention": "low", "refactor": "low",
}
SEVERITY_RANK = {"critical": 0, "high": 1, "medium": 2, "low": 3}


def normalise_severity(s):
    return SEVERITY_MAP.get(str(s).strip().lower(), "medium")


# ─────────────────────────────────────────────────────────────────────────────
#  Running
# ─────────────────────────────────────────────────────────────────────────────

def run_tool(tool, binary, files, target):
    """Run one analyser. Returns (findings, status) where status is one of
    ok / errored / timed_out / no_parser."""
    parser = PARSERS.get(tool.parser)
    if parser is None:
        return [], "no_parser"

    batches = [[]] if not tool.per_file else [
        files[i:i + FILES_PER_INVOCATION]
        for i in range(0, len(files), FILES_PER_INVOCATION)
    ]

    findings, status = [], "ok"
    for batch in batches:
        cmd = [binary] + list(tool.args) + (batch if tool.per_file else [target])
        try:
            proc = subprocess.run(
                cmd,
                capture_output=True, text=True, errors="replace",
                timeout=TOOL_TIMEOUT_SECONDS,
                cwd=target if os.path.isdir(target) else os.path.dirname(target),
                # shell=False deliberately: a path containing spaces needs no
                # quoting and cannot be re-interpreted by a shell that is not
                # there. C:\Program Files\... is the common case on Windows.
                shell=False,
            )
        except subprocess.TimeoutExpired:
            return findings, "timed_out"
        except (OSError, subprocess.SubprocessError):
            return findings, "errored"

        raw = (proc.stdout or "") + ("\n" + proc.stderr if proc.stderr else "")
        try:
            got = parser(tool.name, raw)
        except Exception:
            # A parser that throws must not take the review with it. The tool is
            # recorded as unparsed, which is visible in the trust block, rather
            # than silently contributing nothing.
            return findings, "no_parser"
        findings.extend(got)

    return findings, status


def verify_positions(findings, dropped_counter):
    """Drop findings whose line no longer exists, and attach an excerpt.

    RULE 2 of the method. An analyser reports against the file as it read it; by
    the time anyone reads the report the file may have changed, and a stale line
    number points at innocent code. Re-reading is cheap and the alternative is
    confidently wrong.
    """
    cache, kept = {}, []
    for f in findings:
        path = f.get("file") or ""
        if not path:
            continue
        if path not in cache:
            try:
                with open(path, "r", encoding="utf-8", errors="replace") as fh:
                    cache[path] = fh.read().splitlines()
            except OSError:
                cache[path] = None
        lines = cache[path]
        if lines is None:
            # Unreadable is not the same as stale. Keep it, with no excerpt,
            # rather than deleting a finding we merely could not confirm.
            f["excerpt"] = ""
            kept.append(f)
            continue
        n = f.get("line") or 1
        if n < 1 or n > len(lines):
            dropped_counter[0] += 1
            continue
        f["excerpt"] = lines[n - 1].strip()[:200]
        kept.append(f)
    return kept


def flag_suspicious_uniformity(findings, files, notes):
    """Notice when one tool flags nearly everything, which usually means the
    tool is measuring something environmental rather than the code.

    This is judgement a naive runner does not apply, and it is the difference
    between a report a person trusts and one they learn to ignore.

    The case that motivated it: on a Windows checkout with CRLF line endings,
    `gofmt -l` lists EVERY Go file. Twenty-seven findings, all real in the sense
    that gofmt really did print those names, and all worthless -- the files are
    correctly formatted and only the line endings differ. A reviewer who reports
    that as twenty-seven defects has buried whatever else was found.

    So: if a single tool accounts for most of the findings AND touches most of
    the files, say so beside the findings rather than deleting them. Deleting
    would be a judgement this script is not entitled to make; staying silent
    would be worse.
    """
    if not findings or not files:
        return
    by_tool = defaultdict(set)
    for f in findings:
        by_tool[f["tool"]].add(f["file"])
    total_files = len(set(files))
    for tool, touched in by_tool.items():
        share = len(touched) / float(total_files)
        if share >= 0.75 and len(touched) > 3:
            notes.append(
                "%s flagged %d of %d files (%.0f%%). A tool that flags nearly "
                "everything is usually measuring the environment, not the code -- "
                "line endings, a missing config, or the wrong working directory. "
                "Confirm one finding by hand before believing the rest."
                % (tool, len(touched), total_files, share * 100))


def corroborate(findings):
    """Group findings that two or more DIFFERENT tools put on the same line.

    RULE 3. Agreement between independent analysers is the strongest signal this
    script can produce, and it is kept apart from the single-opinion list so a
    reader cannot mistake a style preference for two tools agreeing.
    """
    by_pos = defaultdict(list)
    for f in findings:
        by_pos[(f["file"], f["line"])].append(f)

    out = []
    for (path, line), group in by_pos.items():
        tools = sorted({g["tool"] for g in group})
        if len(tools) < 2:
            continue
        out.append({
            "file": path, "line": line, "tools": tools,
            "messages": [g["message"] for g in group][:6],
        })
    out.sort(key=lambda c: (-len(c["tools"]), c["file"], c["line"]))
    return out


def plan(langs, deep, no_stage3):
    """Which tools apply, and at which stage."""
    max_stage = 1 if no_stage3 else (3 if deep else 2)
    chosen = []
    for t in TOOLS:
        if t.stage > max_stage:
            continue
        if "*" in t.langs or (t.langs & set(langs)):
            chosen.append(t)
    return chosen


# ─────────────────────────────────────────────────────────────────────────────
#  Doctor
# ─────────────────────────────────────────────────────────────────────────────

def doctor(target):
    files, langs = collect_files(target, 0, "")
    lines = ["Code review — what this machine can actually check", ""]
    lines.append("target: %s" % os.path.abspath(target))
    lines.append("%d file(s), languages: %s" % (len(files), ", ".join(langs) or "none detected"))
    lines.append("")

    relevant = [t for t in TOOLS if "*" in t.langs or (t.langs & set(langs))]
    present = [t for t in relevant if find_binary(t.binary)]
    absent = [t for t in relevant if not find_binary(t.binary)]

    if present:
        lines.append("INSTALLED (%d):" % len(present))
        for t in present:
            lines.append("  %-16s stage %d  %s" % (t.name, t.stage, ", ".join(sorted(t.langs))))
    else:
        lines.append("INSTALLED: none")
    lines.append("")

    if absent:
        lines.append("NOT INSTALLED (%d) — the code they cover will be UNREVIEWED:" % len(absent))
        for t in absent:
            extra = ("  [%s]" % t.note) if t.note else ""
            lines.append("  %-16s %s%s" % (t.name, t.install or "see the tool's own docs", extra))
        lines.append("")
        lines.append("A review that runs none of these is not a clean review.")
        lines.append("It is a review that did not happen.")

    print("\n".join(lines))
    # Exit 3 is the documented "nothing to run" signal the caller checks for.
    return 0 if present else 3


# ─────────────────────────────────────────────────────────────────────────────
#  Main
# ─────────────────────────────────────────────────────────────────────────────

def main(argv):
    ap = argparse.ArgumentParser(add_help=True, description=__doc__)
    ap.add_argument("target")
    ap.add_argument("--audience", default="human")
    ap.add_argument("--diff", default="")
    ap.add_argument("--max-files", type=int, default=0)
    ap.add_argument("--profile", default="default")
    ap.add_argument("--deep", action="store_true")
    ap.add_argument("--no-stage3", action="store_true")
    ap.add_argument("--doctor", action="store_true")
    args = ap.parse_args(argv)

    if not os.path.exists(args.target):
        print("target does not exist: %s" % args.target, file=sys.stderr)
        return 2

    if args.doctor:
        return doctor(args.target)

    started = time.time()
    files, langs = collect_files(args.target, args.max_files, args.diff)
    chosen = plan(langs, args.deep, args.no_stage3)

    ran, errored, missing, timed_out, no_parser = [], [], [], [], []
    findings = []

    for tool in sorted(chosen, key=lambda t: t.stage):
        binary = find_binary(tool.binary)
        if not binary:
            missing.append(tool.name)
            continue
        applicable = files if "*" in tool.langs else [
            f for f in files
            if EXT_LANG.get(os.path.splitext(f)[1].lower()) in tool.langs
        ]
        if tool.per_file and not applicable:
            continue
        got, status = run_tool(tool, binary, applicable, args.target)
        if status == "ok":
            ran.append(tool.name)
            findings.extend(got)
        elif status == "timed_out":
            timed_out.append(tool.name)
            findings.extend(got)      # partial output is still evidence
        elif status == "no_parser":
            no_parser.append(tool.name)
        else:
            errored.append(tool.name)

    notes = []
    dropped = [0]
    findings = verify_positions(findings, dropped)
    findings.sort(key=lambda f: (SEVERITY_RANK.get(f["severity"], 9), f["file"], f["line"]))

    flag_suspicious_uniformity(findings, files, notes)
    caveat = build_caveat(ran, missing, errored, timed_out, no_parser, langs)

    report = {
        "schema": SCHEMA,
        "target": os.path.abspath(args.target),
        "profile": args.profile,
        "files_scanned": len(files),
        "languages": langs,
        "results_dir": "",
        "duration_seconds": round(time.time() - started, 1),
        "findings": findings,
        "corroborated": corroborate(findings),
        "trust": {
            "tools_ran": ran,
            "tools_errored": errored,
            "tools_missing": missing,
            "tools_timed_out": timed_out,
            "tools_without_parser": no_parser,
            "position_checked": True,
            "findings_dropped_by_position_check": dropped[0],
            "caveat": (caveat + " " + " ".join(notes)).strip(),
        },
        "manual_steps": notes + manual_steps(langs, missing),
    }
    json.dump(report, sys.stdout, indent=None)
    sys.stdout.write("\n")
    return 0


def build_caveat(ran, missing, errored, timed_out, no_parser, langs):
    """One sentence a reader cannot skip past. RULE 1 and RULE 5."""
    if not ran:
        return ("NOTHING RAN. No analyser for %s is installed on this machine, so "
                "this report is empty for that reason and not because the code is "
                "clean." % (", ".join(langs) or "these files"))
    bits = ["%d analyser(s) ran" % len(ran)]
    if missing:
        bits.append("%d were NOT INSTALLED and their subject areas are unreviewed" % len(missing))
    if errored:
        bits.append("%d failed" % len(errored))
    if timed_out:
        bits.append("%d timed out and reported only partial results" % len(timed_out))
    if no_parser:
        bits.append("%d produced output this script could not read" % len(no_parser))
    return "; ".join(bits) + "."


def manual_steps(langs, missing):
    """What a person should do that no analyser here can do for them."""
    steps = []
    if missing:
        steps.append("Install the missing analysers listed above, then run the review "
                     "again; until then their subject areas are unreviewed.")
    steps.append("Read the corroborated findings first: two independent tools agreeing "
                 "on one line is the strongest signal in this report.")
    steps.append("Treat every severity as the reporting tool's opinion. The rule id "
                 "travels with each finding so it can be checked.")
    if "c" in langs or "cpp" in langs:
        steps.append("No static analyser proves memory safety. Lifetimes, aliasing and "
                     "concurrent access still need a person.")
    if "go" in langs:
        steps.append("Run `go test ./...` and `go test -race ./...`; neither is a "
                     "static check and neither is covered here.")
    steps.append("Nothing here executed your tests. A green review is not a green build.")
    return steps


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv[1:]))
    except KeyboardInterrupt:
        sys.exit(130)
