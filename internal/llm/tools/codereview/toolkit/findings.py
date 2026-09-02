#!/usr/bin/env python3
"""
findings.py
-----------
One shape for every finding, no matter which tool produced it.

Before this file existed, `report.json` only recorded how MANY things each tool
found and where its log lived. Anything that wanted the actual findings -- an
agent, a de-duplicator, a line-position verifier -- had to open N log files and
learn cppcheck's format, then bandit's, then gitleaks', each one different.

So: every tool gets a `parse_output` function that turns its raw text into a
list of `Finding` records. Nothing here runs anything, and nothing here rewrites
a log -- the logs stay exactly as the tools wrote them. This is a read-only
interpretation layer on top.

Deliberate choices:

  * We parse each tool's NORMAL text output rather than switching everything to
    --json. The raw logs are meant to stay human-readable (that's the whole
    point of keeping them), and text formats don't break between tool versions
    the way JSON schemas do. Semgrep is the one exception -- see PARSER NOTES.
  * A single gcc-style parser covers most of the registry, because most of these
    tools already emit `path:line:col: severity: message`. Bespoke parsers exist
    only for the tools that genuinely don't.
  * `parse_marker_fallback` reproduces the old counting heuristic exactly, so a
    tool with no dedicated parser behaves as it always did -- it just yields
    Findings with `line=None` instead of a bare integer. Nothing regresses.
  * A parser that can't make sense of a line SKIPS it. Never invent a line
    number. A wrong line number is worse than no line number, because the
    position check downstream can only catch drift it can see.
"""

from dataclasses import dataclass, field, asdict
from typing import Dict, List, Optional
import os
import re

# ---------------------------------------------------------------------------
# severity normalisation
#
# Every tool has its own vocabulary ("note", "convention", "MEDIUM", "R"). We
# collapse them onto four levels so that grouping/sorting works across tools.
# Unknown words become "warning" -- the middle option, so an unrecognised
# severity is never silently downgraded to noise or inflated to a crisis.
# ---------------------------------------------------------------------------

SEV_ERROR = "error"
SEV_WARNING = "warning"
SEV_INFO = "info"
SEV_STYLE = "style"

SEVERITY_ALIASES: Dict[str, str] = {
    "error": SEV_ERROR, "fatal": SEV_ERROR, "critical": SEV_ERROR,
    "high": SEV_ERROR, "err": SEV_ERROR, "fatal error": SEV_ERROR,

    "warning": SEV_WARNING, "warn": SEV_WARNING, "medium": SEV_WARNING,
    "refactor": SEV_WARNING, "performance": SEV_WARNING, "portability": SEV_WARNING,

    "note": SEV_INFO, "info": SEV_INFO, "information": SEV_INFO,
    "low": SEV_INFO, "hint": SEV_INFO,

    "style": SEV_STYLE, "convention": SEV_STYLE, "format": SEV_STYLE,
}

# Single letters are deliberately ABSENT from the table above, because they mean
# different things in different tools: pylint's "C" is convention (a style nit),
# while a bare "C" from something else could be anything. Worse, a global letter
# map silently mis-scores any rule id that merely STARTS with one -- golangci's
# "ineffassign" was being read as severity "i" -> info, quietly demoting real
# findings. So letter decoding lives per-parser, keyed to that tool's own scheme.

# flake8 is several tools wearing one coat, and their prefixes are NOT equally
# serious. Getting this wrong is not cosmetic: a real repo yields hundreds of
# E501 "line too long" and a handful of F821 "undefined name". Scoring all of
# them "error" (as a naive letter map does) buries the bug that actually
# crashes at runtime under a pile of whitespace, which is precisely the
# noise-drowns-signal failure this toolkit is otherwise careful to avoid.
#
#   F    pyflakes      -- undefined name, unused import. REAL problems.
#   E9   pycodestyle   -- E901/E902 syntax + IO errors. Real.
#   E/W  pycodestyle   -- PEP8 formatting. Style, however many there are.
#   C9   mccabe        -- complexity. Worth a look, not a bug.
#   B    bugbear       -- likely-bug plugin.
#   N/D  naming/docs   -- style.
def flake8_severity(code: str) -> str:
    if not code:
        return SEV_WARNING
    if code.startswith("F"):
        return SEV_ERROR
    if code.startswith("E9"):
        return SEV_ERROR
    if code[0] in ("E", "W", "N", "D"):
        return SEV_STYLE
    if code[0] in ("C", "B"):
        return SEV_WARNING
    return SEV_WARNING
# How pylint labels its own messages: C=convention R=refactor W=warning
# E=error F=fatal I=informational.
#
# Careful when editing this comment: pylint treats the word "pylint" followed by
# a colon as an inline configuration pragma ANYWHERE in a comment, not just at
# the start of one. Writing that token here -- even inside a warning about it --
# makes pylint try to parse the rest of the line as file options and emit
# unrecognized-inline-option. Describe the pragma, never spell it.
PYLINT_LETTER = {"C": SEV_STYLE, "R": SEV_WARNING, "W": SEV_WARNING,
                 "E": SEV_ERROR, "F": SEV_ERROR, "I": SEV_INFO}

# A flake8/pylint-shaped code: one to three capitals then two to four digits.
# Only these get letter-decoded; anything else keeps the default severity.
CODE_SHAPE_RE = re.compile(r"^([A-Z]{1,3})(\d{2,4})$")

SEVERITY_RANK = {SEV_ERROR: 0, SEV_WARNING: 1, SEV_INFO: 2, SEV_STYLE: 3}


def norm_severity(raw: Optional[str]) -> str:
    if not raw:
        return SEV_WARNING
    return SEVERITY_ALIASES.get(raw.strip().lower().rstrip(":"), SEV_WARNING)


# ---------------------------------------------------------------------------
# the record
# ---------------------------------------------------------------------------

@dataclass
class Finding:
    """One thing one tool said about one place in the code.

    `file` is stored repo-relative wherever we can work it out, because that's
    what an agent, a git diff and a review comment all speak. `line` is 1-based
    and may be None -- plenty of real findings are file-level or project-level,
    and pretending otherwise is how you end up with comments on line 1.

    `snippet` starts empty. The position check fills it in later by reading the
    source at file:line, which is also how it detects drift.
    """
    tool: str
    file: str
    message: str
    line: Optional[int] = None
    col: Optional[int] = None
    severity: str = SEV_WARNING
    rule_id: str = ""
    snippet: str = ""
    raw: str = ""          # the original line(s), so a human can always audit us
    fingerprint: str = ""  # set by finalize(); used to merge duplicates

    # Optional annotations from the local glue tier (local_tier.py). Declared
    # here rather than attached dynamically because to_dict() uses asdict(),
    # which only sees real fields -- a setattr would silently vanish from
    # report.json, and a field that quietly doesn't serialise is worse than one
    # that isn't there at all.
    plain_message: str = ""  # tool jargon rewritten for a human; message stays authoritative
    local_triage: str = ""   # "noise" if a local model doubted it. ADVISORY. Never a reason to hide it.

    # Set by parsers for tools that detect CREDENTIALS. The position check must
    # not copy this line's source into `snippet`.
    #
    # This exists because of a real leak. parse_gitleaks() carefully withholds
    # the secret from the record -- and there is a test asserting it -- but
    # verify_positions() then read the file at file:line and attached the source
    # as evidence, putting the credential straight into report.json through a
    # different door. The unit test passed the whole time; only a run against a
    # repo with real secrets in it exposed the hole. Withholding a secret in one
    # function and re-reading it in another is not withholding it.
    redact_snippet: bool = False

    # True when file:line refers to a PAST revision rather than the working
    # tree -- a full-history secrets scan reports where a secret used to be.
    # Verifying such a line against the current file is meaningless: it attached
    # `maps.Copy(SupportedModels, AzureModels)` as the evidence for a credential
    # finding, which is fabricated corroboration.
    historical: bool = False

    # Whether this finding is inside the --diff scope the caller asked for.
    # None when no --diff was given. Project-wide tools (go vet ./..., cargo
    # clippy) cannot be restricted to a file list, so they report on the whole
    # module even in diff mode; this flag is how a consumer tells the difference.
    in_diff_scope: Optional[bool] = None

    # Set from the project's .code-review-baseline.json (see baseline.py): a
    # finding this project has already examined and accepted. Suppressed
    # findings are excluded from headline counts and corroboration but are NOT
    # removed from report.json -- the reason travels with them, so the decision
    # stays auditable instead of becoming folklore.
    suppressed: bool = False
    suppression_reason: str = ""

    def to_dict(self) -> dict:
        return asdict(self)


def _rel(path: str, cwd: str) -> str:
    """Repo-relative if we can manage it, otherwise leave it alone."""
    if not path:
        return path
    p = path.strip().strip('"')
    if p.startswith("./"):
        p = p[2:]
    if os.path.isabs(p) and cwd:
        try:
            return os.path.relpath(p, start=cwd)
        except ValueError:  # different drive on Windows
            return p
    return p


def _int_or_none(s) -> Optional[int]:
    try:
        n = int(s)
        return n if n > 0 else None
    except (TypeError, ValueError):
        return None


def finalize(findings: List[Finding], cwd: str = "") -> List[Finding]:
    """Normalise paths, compute fingerprints, sort by severity then location.

    The fingerprint is what makes "two different tools flagged the same thing"
    a set operation instead of something an LLM has to eyeball out of prose.
    It deliberately EXCLUDES the tool name and the message -- two tools
    describing one bug word their messages differently, and that's exactly the
    case we want to collapse.
    """
    for f in findings:
        f.file = _rel(f.file, cwd)
        f.fingerprint = f"{f.file}:{f.line or 0}:{f.rule_id or ''}"
    findings.sort(key=lambda f: (SEVERITY_RANK.get(f.severity, 9), f.file, f.line or 0))
    return findings


# ---------------------------------------------------------------------------
# the workhorse: gcc-style diagnostics
#
#   path:line:col: severity: message [rule]
#   path:line: severity: message
#   path:line:col: message (rule)
#
# Covers cppcheck (--template=gcc), shellcheck (-f gcc), flake8, mypy,
# clang-tidy, go vet, staticcheck, golangci-lint and eslint's compact-ish
# output. One parser, most of the registry.
# ---------------------------------------------------------------------------

GCC_RE = re.compile(
    r"^\s*(?P<file>[^\s:][^:]*?):(?P<line>\d+)(?::(?P<col>\d+))?:\s*"
    r"(?:(?P<sev>error|warning|note|info|style|performance|portability|fatal error|convention|refactor)\s*:\s*)?"
    r"(?P<msg>.*)$",
    re.IGNORECASE,
)

# Trailing rule identifiers: "[SC2086]", "[unusedVariable]", "(staticcheck SA1000)"
TRAILING_RULE_RE = re.compile(r"\s*[\[(]([A-Za-z][A-Za-z0-9_.:-]{1,60})[\])]\s*$")
# flake8/pylint put their code at the START of the message: "E501 line too long"
LEADING_CODE_RE = re.compile(r"^([A-Z]{1,3}\d{2,4})\s+(.*)$")


def parse_gcc_style(text: str, ctx: dict) -> List[Finding]:
    tool = ctx.get("tool_id", "")
    # No path handling here on purpose -- finalize() makes every path relative
    # once, at the end, so each parser doesn't have to get it right separately.
    out: List[Finding] = []
    for line in text.splitlines():
        if not line.strip():
            continue
        m = GCC_RE.match(line)
        if not m:
            continue
        msg = m.group("msg").strip()
        if not msg:
            continue
        rule = ""
        # a trailing [rule] / (rule) is a rule id, not part of the sentence
        rm = TRAILING_RULE_RE.search(msg)
        if rm:
            rule = rm.group(1)
            msg = msg[: rm.start()].strip()
        else:
            lm = LEADING_CODE_RE.match(msg)
            if lm:
                rule, msg = lm.group(1), lm.group(2).strip()
        sev_word = m.group("sev")
        if sev_word:
            severity = norm_severity(sev_word)
        else:
            # No severity word. Decode it only from a genuinely flake8-shaped
            # code (E501, F401); a descriptive rule name like "ineffassign"
            # tells us nothing about severity, so leave it at the default.
            cm = CODE_SHAPE_RE.match(rule) if rule else None
            severity = flake8_severity(rule) if cm else SEV_WARNING
        out.append(Finding(
            tool=tool, file=m.group("file"), line=_int_or_none(m.group("line")),
            col=_int_or_none(m.group("col")), severity=severity,
            rule_id=rule, message=msg, raw=line.rstrip(),
        ))
    return out


# ---------------------------------------------------------------------------
# pylint --output-format=parseable
#   path:line: [C0111(missing-docstring), some.func] message here
# ---------------------------------------------------------------------------

PYLINT_RE = re.compile(
    r"^\s*(?P<file>[^\s:][^:]*?):(?P<line>\d+):\s*\[(?P<code>[A-Z]\d+)"
    r"(?:\((?P<slug>[^)]*)\))?(?:,\s*(?P<ctx>[^\]]*))?\]\s*(?P<msg>.*)$"
)


def parse_pylint(text: str, ctx: dict) -> List[Finding]:
    tool = ctx.get("tool_id", "pylint")
    out: List[Finding] = []
    for line in text.splitlines():
        m = PYLINT_RE.match(line)
        if not m:
            continue
        out.append(Finding(
            tool=tool, file=m.group("file"), line=_int_or_none(m.group("line")),
            severity=PYLINT_LETTER.get(m.group("code")[0], SEV_WARNING),
            rule_id=m.group("slug") or m.group("code"),
            message=m.group("msg").strip(), raw=line.rstrip(),
        ))
    return out or parse_gcc_style(text, ctx)


# ---------------------------------------------------------------------------
# cpplint
#   path:line:  message here  [category/subcategory] [confidence]
# No severity word at all; confidence 1-5 stands in for one.
# ---------------------------------------------------------------------------

CPPLINT_RE = re.compile(
    r"^\s*(?P<file>[^\s:][^:]*?):(?P<line>\d+):\s+(?P<msg>.*?)\s*"
    r"\[(?P<cat>[a-z0-9_/+-]+)\]\s*\[(?P<conf>\d)\]\s*$", re.IGNORECASE
)


def parse_cpplint(text: str, ctx: dict) -> List[Finding]:
    out: List[Finding] = []
    for line in text.splitlines():
        m = CPPLINT_RE.match(line)
        if not m:
            continue
        conf = _int_or_none(m.group("conf")) or 1
        out.append(Finding(
            tool=ctx.get("tool_id", "cpplint"), file=m.group("file"),
            line=_int_or_none(m.group("line")),
            severity=SEV_WARNING if conf >= 4 else SEV_STYLE,
            rule_id=m.group("cat"), message=m.group("msg").strip(), raw=line.rstrip(),
        ))
    return out


# ---------------------------------------------------------------------------
# bandit -f txt
#   >> Issue: [B101:assert_used] Use of assert detected.
#      Severity: Low   Confidence: High
#      CWE: CWE-703 (https://...)
#      Location: ./foo.py:12:8
# Block-structured: collect fields until Location closes the record.
# ---------------------------------------------------------------------------

BANDIT_ISSUE_RE = re.compile(r">>\s*Issue:\s*\[(?P<code>[^:\]]+):?(?P<slug>[^\]]*)\]\s*(?P<msg>.*)")
BANDIT_SEV_RE = re.compile(r"Severity:\s*(?P<sev>\w+)")
BANDIT_CWE_RE = re.compile(r"(CWE-\d+)")
BANDIT_LOC_RE = re.compile(r"Location:\s*(?P<file>.+?):(?P<line>\d+)(?::(?P<col>\d+))?\s*$")


def parse_bandit(text: str, ctx: dict) -> List[Finding]:
    tool = ctx.get("tool_id", "bandit")
    out: List[Finding] = []
    cur: Optional[dict] = None
    for line in text.splitlines():
        mi = BANDIT_ISSUE_RE.search(line)
        if mi:
            cur = {"code": mi.group("code").strip(), "slug": mi.group("slug").strip(),
                   "msg": mi.group("msg").strip(), "sev": None, "cwe": "", "raw": [line.rstrip()]}
            continue
        if cur is None:
            continue
        cur["raw"].append(line.rstrip())
        ms = BANDIT_SEV_RE.search(line)
        if ms:
            cur["sev"] = ms.group("sev")
        mc = BANDIT_CWE_RE.search(line)
        if mc:
            cur["cwe"] = mc.group(1)
        ml = BANDIT_LOC_RE.search(line)
        if ml:
            rule = cur["slug"] or cur["code"]
            if cur["cwe"]:
                rule = f"{rule} ({cur['cwe']})"
            out.append(Finding(
                tool=tool, file=ml.group("file"), line=_int_or_none(ml.group("line")),
                col=_int_or_none(ml.group("col")), severity=norm_severity(cur["sev"]),
                rule_id=rule, message=cur["msg"], raw="\n".join(cur["raw"]),
            ))
            cur = None
    return out


# ---------------------------------------------------------------------------
# gitleaks -v
#   Finding:  ...
#   Secret:   ...
#   RuleID:   generic-api-key
#   File:     path/to/file
#   Line:     12
#
# NOTE: we record the rule and location but NEVER the secret itself. This file
# ends up in report.json, which is the thing most likely to get pasted into a
# chat window or a CI log. Leaking the credential while reporting the leak
# would be its own security bug.
# ---------------------------------------------------------------------------

GITLEAKS_FIELD_RE = re.compile(r"^\s*(?P<key>Finding|Secret|RuleID|Entropy|File|Line|Commit|Author):\s*(?P<val>.*)$")


def parse_gitleaks(text: str, ctx: dict) -> List[Finding]:
    tool = ctx.get("tool_id", "gitleaks")
    out: List[Finding] = []
    cur: Dict[str, str] = {}

    def flush():
        if cur.get("File"):
            out.append(Finding(
                tool=tool, file=cur["File"], line=_int_or_none(cur.get("Line")),
                severity=SEV_ERROR,  # a live credential is never a style nit
                rule_id=cur.get("RuleID", "secret"),
                message=("Possible secret detected"
                         + (f" (rule: {cur['RuleID']})" if cur.get("RuleID") else "")
                         + (f", commit {cur['Commit'][:8]}" if cur.get("Commit") else "")
                         + " -- value withheld from this report on purpose."),
                raw=f"RuleID={cur.get('RuleID','?')} File={cur['File']} Line={cur.get('Line','?')}",
            ))
        cur.clear()

    for line in text.splitlines():
        m = GITLEAKS_FIELD_RE.match(line)
        if not m:
            continue
        key, val = m.group("key"), m.group("val").strip()
        if key == "Finding" and cur:
            flush()
        if key != "Secret":  # never store the secret
            cur[key] = val
        else:
            cur.setdefault("Secret", "<withheld>")
    flush()
    # Mark every record so the position check can't undo the withholding above,
    # and so a history scan's line numbers aren't checked against the current
    # file. `historical` is decided by tool id: a --log-opts=--all scan reports
    # where a secret WAS, which may be a revision the file no longer resembles.
    historical = "history" in tool
    for f in out:
        f.redact_snippet = True
        f.historical = historical
    return out


# ---------------------------------------------------------------------------
# cargo clippy / rustc
#   warning: unused variable: `x`
#     --> src/main.rs:4:9
# Severity comes one line BEFORE the location, so we remember it and attach it
# when the arrow shows up.
# ---------------------------------------------------------------------------

RUST_HEAD_RE = re.compile(r"^(?P<sev>error|warning)(?:\[(?P<code>[^\]]+)\])?:\s*(?P<msg>.*)$")
RUST_LOC_RE = re.compile(r"^\s*-->\s*(?P<file>[^\s:]+):(?P<line>\d+):(?P<col>\d+)\s*$")


def parse_rust(text: str, ctx: dict) -> List[Finding]:
    tool = ctx.get("tool_id", "cargo-clippy")
    out: List[Finding] = []
    pending: Optional[dict] = None
    for line in text.splitlines():
        mh = RUST_HEAD_RE.match(line)
        if mh:
            pending = {"sev": mh.group("sev"), "code": mh.group("code") or "",
                       "msg": mh.group("msg").strip(), "raw": line.rstrip()}
            continue
        ml = RUST_LOC_RE.match(line)
        if ml and pending:
            out.append(Finding(
                tool=tool, file=ml.group("file"), line=_int_or_none(ml.group("line")),
                col=_int_or_none(ml.group("col")), severity=norm_severity(pending["sev"]),
                rule_id=pending["code"], message=pending["msg"],
                raw=pending["raw"] + "\n" + line.rstrip(),
            ))
            pending = None
    return out


# ---------------------------------------------------------------------------
# gosec text
#   [/abs/path/file.go:12] - G401 (CWE-326): Use of weak cryptographic primitive
# ---------------------------------------------------------------------------

GOSEC_RE = re.compile(
    r"^\s*\[(?P<file>[^\]:]+):(?P<line>\d+)\]\s*-\s*(?P<code>G\d+)\s*"
    r"(?:\((?P<cwe>CWE-\d+)\))?\s*:?\s*(?P<msg>.*?)\s*(?:\(Confidence:\s*(?P<conf>\w+),\s*Severity:\s*(?P<sev>\w+)\))?\s*$"
)


def parse_gosec(text: str, ctx: dict) -> List[Finding]:
    out: List[Finding] = []
    for line in text.splitlines():
        m = GOSEC_RE.match(line)
        if not m:
            continue
        rule = m.group("code")
        if m.group("cwe"):
            rule = f"{rule} ({m.group('cwe')})"
        out.append(Finding(
            tool=ctx.get("tool_id", "gosec"), file=m.group("file"),
            line=_int_or_none(m.group("line")),
            severity=norm_severity(m.group("sev") or "warning"),
            rule_id=rule, message=m.group("msg").strip(), raw=line.rstrip(),
        ))
    return out


# ---------------------------------------------------------------------------
# semgrep
#
# PARSER NOTES -- the one place we ask a tool to change its output format.
# Semgrep's human-readable output is a nested, colour-aligned tree whose shape
# shifts between versions; parsing it reliably is not realistic. So the semgrep
# entries in the registry pass --json, and the log holds that JSON. It's still
# the tool's complete, unedited output -- just less pleasant to read by eye.
# If a log turns out NOT to be JSON (older semgrep, an error page), we fall
# back to the gcc parser rather than returning nothing.
# ---------------------------------------------------------------------------

def parse_semgrep(text: str, ctx: dict) -> List[Finding]:
    import json
    tool = ctx.get("tool_id", "semgrep")
    start = text.find("{")
    if start == -1:
        return parse_gcc_style(text, ctx)
    try:
        data = json.loads(text[start:])
    except (ValueError, TypeError):
        return parse_gcc_style(text, ctx)
    out: List[Finding] = []
    for r in data.get("results", []) or []:
        extra = r.get("extra", {}) or {}
        meta = extra.get("metadata", {}) or {}
        rule = r.get("check_id", "") or ""
        cwe = meta.get("cwe")
        if isinstance(cwe, list):
            cwe = cwe[0] if cwe else None
        if cwe:
            rule = f"{rule} ({str(cwe).split(':')[0]})"
        out.append(Finding(
            tool=tool, file=r.get("path", "") or "",
            line=_int_or_none((r.get("start") or {}).get("line")),
            col=_int_or_none((r.get("start") or {}).get("col")),
            severity=norm_severity(extra.get("severity") or meta.get("impact")),
            rule_id=rule,
            message=(extra.get("message") or "").strip().replace("\n", " "),
            raw=f"semgrep {r.get('check_id','')} at {r.get('path','')}",
        ))
    return out


# ---------------------------------------------------------------------------
# our own checks (heuristics.py)
#
# These already emit this module's record shape, so parsing is validation rather
# than extraction. We still go field by field instead of trusting the JSON
# wholesale, because a malformed record that reaches report.json is a record
# some downstream consumer will crash on.
# ---------------------------------------------------------------------------

def parse_heuristics(text: str, ctx: dict) -> List[Finding]:
    import json
    start = text.find("{")
    if start == -1:
        return []
    try:
        data = json.loads(text[start:])
    except (ValueError, TypeError):
        return []
    out: List[Finding] = []
    for r in data.get("findings", []) or []:
        if not isinstance(r, dict) or not r.get("file"):
            continue
        sev = r.get("severity", SEV_WARNING)
        out.append(Finding(
            tool=r.get("tool") or ctx.get("tool_id", ""),
            file=r["file"],
            line=_int_or_none(r.get("line")),
            col=_int_or_none(r.get("col")),
            severity=sev if sev in SEVERITY_RANK else norm_severity(sev),
            rule_id=str(r.get("rule_id") or ""),
            message=str(r.get("message") or "").strip(),
            snippet=str(r.get("snippet") or "").strip(),
            raw=str(r.get("raw") or ""),
        ))
    return out


# ---------------------------------------------------------------------------
# formatters (black/isort/prettier/clang-format/cargo fmt)
#
# These don't report findings, they report "this file is not formatted". One
# Finding per file is the honest representation -- emitting one per changed
# line would drown every real bug in whitespace.
# ---------------------------------------------------------------------------

def parse_format_check(text: str, ctx: dict) -> List[Finding]:
    tool = ctx.get("tool_id", "format")
    target = ctx.get("target_path", "")
    reformat = re.search(r"^(would reformat|--- )", text, re.MULTILINE)
    err = re.search(r"^\s*(Code style issues|error:|Error:)", text, re.MULTILINE)
    if not (reformat or err):
        return []
    return [Finding(
        tool=tool, file=target, line=None, severity=SEV_STYLE, rule_id="unformatted",
        message="File is not formatted according to the project's formatter.",
        raw=text.strip().splitlines()[0] if text.strip() else "",
    )]


# ---------------------------------------------------------------------------
# fallback: the original marker heuristic, preserved exactly
#
# This is what `count_issues()` in code_review.py used to do for everything.
# Keeping it means an unparsed tool behaves as it always did rather than
# silently reporting zero -- which would look identical to "clean", the one
# failure mode this toolkit is most careful about everywhere else.
# ---------------------------------------------------------------------------

GENERIC_DIAG_LINE_RE = re.compile(r"^\s*[^\s:]+:\d+(:\d+)?:\s")


def parse_marker_fallback(text: str, ctx: dict) -> List[Finding]:
    tool = ctx.get("tool_id", "")
    target = ctx.get("target_path", "")
    # Markers under 3 chars (bare "E"/"W") match ordinary English words and
    # produced bogus counts in testing -- same reasoning as the original.
    markers = [m.lower() for m in (ctx.get("severity_markers") or []) if len(m) >= 3]
    out: List[Finding] = []
    for line in text.splitlines():
        hit_diag = bool(GENERIC_DIAG_LINE_RE.match(line))
        hit_marker = bool(markers) and any(m in line.lower() for m in markers)
        if not (hit_diag or hit_marker):
            continue
        if hit_diag:
            g = parse_gcc_style(line, ctx)
            if g:
                out.extend(g)
                continue
        out.append(Finding(
            tool=tool, file=target, line=None, severity=SEV_WARNING, rule_id="",
            message=line.strip()[:400], raw=line.rstrip(),
        ))
    return out


# ---------------------------------------------------------------------------
# position verification
#
# The single cheapest way to lose a developer's trust is a comment pinned to
# the wrong line. This pass is pure Python -- no model, no network -- and it
# does three things:
#
#   1. drops findings whose file no longer exists
#   2. drops findings whose line is past the end of the file
#   3. fills in `snippet` with the actual source at file:line
#
# (3) is what makes the whole thing worth doing twice: once a finding carries
# the line it claims to describe, ANY later consumer -- an agent, a diff
# viewer, a human -- can check the claim itself. And when a finding arrives
# WITH a claimed snippet (which is how an LLM reviewer reports), we compare it
# against the real source and drop it if they disagree. That comparison is the
# actual drift detector; tool findings can't drift because tools read the file
# we're reading, but model findings drift constantly.
# ---------------------------------------------------------------------------

def _norm_code(s: str) -> str:
    """Whitespace-insensitive comparison. A reformat shouldn't count as drift,
    but a genuinely different statement should."""
    return re.sub(r"\s+", "", s or "")


# Rule ids and phrasings that mean "this finding is ABOUT a credential".
#
# The first version of the redaction fix keyed off the TOOL (gitleaks), and a
# real run proved that wrong within minutes: gosec's G101 "potential hardcoded
# credentials" carried the same secret into report.json through the identical
# code path. Any tool can report a credential, so the test has to be about what
# the finding IS, not who produced it. When in doubt this errs towards
# redacting -- losing a snippet costs a reader one file open, while printing a
# live key into a file destined for chat windows and CI logs cannot be undone.
CREDENTIAL_RULE_RE = re.compile(
    r"(?:^|[^A-Za-z])(?:"
    r"G101|G102|"                       # gosec hardcoded credentials / bind-all
    r"B10[567]|"                         # bandit hardcoded password/funcarg/default
    r"CWE-798|CWE-259|CWE-321|CWE-547|"  # hardcoded credentials family
    r"generic-api-key|api[-_]?key|secret|password|passwd|credential|"
    r"private[-_]?key|access[-_]?token|auth[-_]?token"
    r")", re.IGNORECASE)

CREDENTIAL_MSG_RE = re.compile(
    r"hardcoded\s+(?:credential|password|secret|key)|"
    r"possible secret|detected secret|api key|private key|"
    r"credential.{0,20}(?:in source|hardcoded)", re.IGNORECASE)

# A quoted literal long enough to be a real token. Used to scrub verbatim tool
# output for credential findings, where the tool may have echoed the value.
QUOTED_LITERAL_RE = re.compile(r"""(['"])([^'"\n]{12,})(\1)""")


def is_credential_finding(f: "Finding") -> bool:
    """Would attaching this line's source expose a secret?"""
    if f.redact_snippet:
        return True
    if f.rule_id and CREDENTIAL_RULE_RE.search(f.rule_id):
        return True
    return bool(f.message and CREDENTIAL_MSG_RE.search(f.message))


def scrub_literals(text: str) -> str:
    """Replace long quoted literals with a placeholder, keeping the shape."""
    return QUOTED_LITERAL_RE.sub(
        lambda m: f"{m.group(1)}<redacted {len(m.group(2))} chars>{m.group(3)}", text or "")


@dataclass
class PositionReport:
    kept: List[Finding] = field(default_factory=list)
    dropped: List[Finding] = field(default_factory=list)
    reasons: Dict[str, str] = field(default_factory=dict)  # fingerprint -> why

    def summary(self) -> str:
        if not self.dropped:
            return f"position check: all {len(self.kept)} finding(s) verified"
        return (f"position check: {len(self.kept)} verified, "
                f"{len(self.dropped)} dropped as unlocatable")


def verify_positions(items: List[Finding], root: str, drift_window: int = 3) -> PositionReport:
    """Confirm each finding points at a real line, and attach that line.

    `drift_window` only applies to findings that arrived WITH a claimed
    snippet: if the claim doesn't match the stated line, we look a few lines
    either side before giving up, because a small offset is the common case and
    silently discarding a real bug over an off-by-two would be worse than
    correcting it. If it's found nearby, the line is corrected in place.
    """
    rep = PositionReport()
    cache: Dict[str, Optional[List[str]]] = {}

    def lines_of(rel: str) -> Optional[List[str]]:
        if rel not in cache:
            path = rel if os.path.isabs(rel) else os.path.join(root, rel)
            try:
                with open(path, errors="replace") as fh:
                    cache[rel] = fh.read().splitlines()
            except (OSError, IsADirectoryError):
                cache[rel] = None
        return cache[rel]

    for f in items:
        # File-level findings have nothing to verify; a formatter complaining
        # about a whole file is legitimate and must not be thrown away.
        if f.line is None:
            rep.kept.append(f)
            continue

        # A historical finding's line refers to a revision that is not on disk.
        # Checking it against the current file would either drop it (if the file
        # shrank) or attach an unrelated line as evidence. Keep it, say why it
        # wasn't verified, and attach nothing.
        if f.historical:
            f.snippet = ""
            rep.kept.append(f)
            rep.reasons[f.fingerprint] = (
                "not position-checked: line refers to a past revision, not the "
                "working tree")
            continue

        src = lines_of(f.file)
        if src is None:
            rep.dropped.append(f)
            rep.reasons[f.fingerprint] = f"file not readable: {f.file}"
            continue
        if f.line > len(src):
            rep.dropped.append(f)
            rep.reasons[f.fingerprint] = (
                f"line {f.line} is past end of {f.file} ({len(src)} lines)")
            continue

        # Credential findings: confirm the line EXISTS (done above), then stop.
        # Copying the source here is what leaked secrets into report.json --
        # first via gitleaks, then, after a tool-specific fix, via gosec G101.
        # Hence the category test rather than a tool list.
        if is_credential_finding(f):
            f.snippet = "<redacted: this line holds a detected secret>"
            # The tool's own output may have echoed the value too.
            f.raw = scrub_literals(f.raw)
            rep.kept.append(f)
            continue

        actual = src[f.line - 1]
        n_actual = _norm_code(actual)
        n_claim = _norm_code(f.snippet)

        # A claim of nothing verifies nothing -- just attach the real line.
        # (Whitespace-only counts as nothing, which is why we test the
        # NORMALISED claim rather than the raw string.)
        if not n_claim:
            f.snippet = actual.strip()
            rep.kept.append(f)
            continue

        # Both sides must be non-empty before a containment test means anything:
        # "" is a substring of every string, so a blank source line would
        # otherwise "confirm" any claim at all and the drift search below would
        # never run -- silently defeating the entire check.
        if n_actual and (n_claim in n_actual or n_actual in n_claim):
            f.snippet = actual.strip()
            rep.kept.append(f)
            continue

        # Claimed snippet doesn't match. Search the neighbourhood before dropping.
        found_at = None
        for delta in range(1, drift_window + 1):
            for cand in (f.line - delta, f.line + delta):
                if not (1 <= cand <= len(src)):
                    continue
                n_cand = _norm_code(src[cand - 1])
                if n_cand and n_claim in n_cand:
                    found_at = cand
                    break
            if found_at:
                break
        if found_at:
            rep.reasons[f.fingerprint] = f"line corrected {f.line} -> {found_at}"
            f.line = found_at
            f.snippet = src[found_at - 1].strip()
            rep.kept.append(f)
        else:
            rep.dropped.append(f)
            rep.reasons[f.fingerprint] = (
                f"claimed code not found at or near {f.file}:{f.line}")
    return rep


# ---------------------------------------------------------------------------
# cross-tool agreement
# ---------------------------------------------------------------------------

# Severities that count as evidence of a real problem. `style` is excluded, and
# that exclusion is the whole point -- see corroborated().
CORROBORATION_SEVERITIES = (SEV_ERROR, SEV_WARNING, SEV_INFO)


def corroborated(findings: List[Finding],
                 severities=CORROBORATION_SEVERITIES) -> List[List[Finding]]:
    """Groups of findings at the same place from DIFFERENT tools.

    This is the P0 tier -- "multiple tools flagged the same thing" -- computed
    instead of inferred. Previously an LLM had to spot it by reading prose,
    which it could get wrong in both directions.

    Matching is file + line, ignoring rule ids: two tools rarely agree on a rule
    name for the same underlying bug.

    STYLE FINDINGS ARE EXCLUDED, and without that this list lies. pylint and
    flake8 both report line-length and missing-docstring, so on a real repo the
    great majority of "two tools agree" pairs are two linters agreeing about
    whitespace. That drowns the handful of places where two static analysers
    independently found the same actual bug -- which is the only reason anyone
    reads this list. On this toolkit's own source it cut 125 groups to a
    genuinely interesting few.

    Note the consequence, which is deliberate: if flake8 says E501 (style) and
    pylint says too-many-arguments (warning) on one line, that is NOT
    corroboration. They found two unrelated things that happen to share a line,
    and the style finding is dropped before grouping, leaving one tool.
    """
    buckets: Dict[str, List[Finding]] = {}
    for f in findings:
        if f.line is None:
            continue  # file-level findings would over-match
        if f.severity not in severities:
            continue
        if f.suppressed:
            continue  # an accepted finding is not evidence of a live problem
        buckets.setdefault(f"{f.file}:{f.line}", []).append(f)
    groups = []
    for _, group in sorted(buckets.items()):
        if len({f.tool for f in group}) > 1:
            groups.append(group)
    return groups


def style_agreements(findings: List[Finding]) -> int:
    """How many same-line multi-tool agreements corroborated() left out.

    Reported rather than silently discarded: this codebase's rule is that a
    number which quietly vanished is worse than one you chose to ignore. If this
    is large it means your linters overlap heavily on formatting, which is worth
    knowing and is not a bug.
    """
    buckets: Dict[str, List[Finding]] = {}
    for f in findings:
        if f.line is None or f.severity != SEV_STYLE:
            continue
        buckets.setdefault(f"{f.file}:{f.line}", []).append(f)
    return sum(1 for g in buckets.values() if len({f.tool for f in g}) > 1)
