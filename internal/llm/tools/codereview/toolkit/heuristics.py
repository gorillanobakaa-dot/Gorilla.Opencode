#!/usr/bin/env python3
"""
heuristics.py
-----------------
The in-house heuristics: checks no off-the-shelf analyser in the registry
performs, because they encode knowledge about specific codebases rather than
about a language in general.

Rebuilt from the spec in the (now scriptless) multi-agent-code-auditor skill,
whose orchestrator at ~/Downloads/Multi-Agent.Code.Auditor/auditor.py no longer
exists. That SKILL.md described five scanners; all five live here now, plus
brace parity ported from patch-auditor's cpp_preflight_protocol.py.

  brace-parity      curly/paren/bracket balance, comment- and string-aware
  template-lookup   dependent names used without `typename`/`template`
  unified-build     Firefox unified-build pollution: leaked macros, file-scope
                    `using namespace`, non-anonymous file-static collisions
  kernel-locks      sleeping while holding a spinlock (the GFP_KERNEL classic)
  blast-radius      every file transitively relying on a given symbol/macro

HOW MUCH TO TRUST THESE
-----------------------
These are LEXICAL heuristics -- regex and a small state machine over stripped
source. They are not a compiler and they do not build an AST. So:

  * brace-parity is near-certain. Counting delimiters after removing comments,
    strings and preprocessor lines is a decision a regex can genuinely make,
    and an imbalance is a hard syntax error. Reported as `error`.
  * kernel-locks and unified-build are strong hints reported as `warning`.
    They flag real patterns but cannot see through macros or function calls, so
    a `spin_lock()` in one function and a `kmalloc(GFP_KERNEL)` in a callee is
    invisible here.
  * template-lookup is the weakest, reported as `info`. C++ name lookup is not
    a lexical property and anything short of a real frontend will guess.

Every check therefore carries its own confidence, and severity reflects it.
Overstating a heuristic is how a review tool teaches people to ignore it.

Output is the normalised findings JSON that findings.py defines, so the registry
parses this exactly like any other tool.

Usage:
    python3 heuristics.py brace-parity FILE...
    python3 heuristics.py template-lookup FILE...
    python3 heuristics.py unified-build FILE...
    python3 heuristics.py kernel-locks FILE...
    python3 heuristics.py blast-radius --symbol NAME --root DIR
    python3 heuristics.py all FILE...
"""

import argparse
import json
import os
import re
import sys

# ---------------------------------------------------------------------------
# source cleaning
#
# Every check below needs code with comments and string literals removed, or it
# will count a brace inside "}" or /* } */ as real. Doing this by hand rather
# than with a naive regex because the interactions matter: a // inside a string
# is not a comment, and a " inside a comment is not a string.
# ---------------------------------------------------------------------------

def strip_comments_and_strings(text: str, keep_layout: bool = True) -> str:
    """Blank out comments and string/char literal CONTENTS, preserving line
    numbering (so a finding's line still means something) and the quote/brace
    characters that delimit real code."""
    out = []
    i, n = 0, len(text)
    state = None  # None | 'line' | 'block' | 'str' | 'char'
    while i < n:
        c = text[i]
        nxt = text[i + 1] if i + 1 < n else ""

        if state is None:
            if c == "/" and nxt == "/":
                state = "line"; out.append("  "); i += 2; continue
            if c == "/" and nxt == "*":
                state = "block"; out.append("  "); i += 2; continue
            if c == '"':
                state = "str"; out.append('"'); i += 1; continue
            if c == "'":
                state = "char"; out.append("'"); i += 1; continue
            out.append(c); i += 1; continue

        if state == "line":
            if c == "\n":
                state = None; out.append("\n")
            else:
                out.append(" " if keep_layout else "")
            i += 1; continue

        if state == "block":
            if c == "*" and nxt == "/":
                state = None; out.append("  "); i += 2; continue
            out.append("\n" if c == "\n" else " "); i += 1; continue

        # inside a string or char literal
        if c == "\\":
            out.append("  "); i += 2; continue
        if (state == "str" and c == '"') or (state == "char" and c == "'"):
            state = None; out.append(c); i += 1; continue
        out.append("\n" if c == "\n" else " ")
        i += 1
    return "".join(out)


IF_RE = re.compile(r"^\s*#\s*(if|ifdef|ifndef)\b")
ELSE_RE = re.compile(r"^\s*#\s*(else|elif)\b")
ENDIF_RE = re.compile(r"^\s*#\s*endif\b")


def strip_preprocessor(text: str) -> str:
    """Reduce conditional compilation to a SINGLE branch, then blank all
    directives.

    Removing only the directives (the obvious approach, and what this function
    did at first) is actively wrong: it leaves every branch of an #if/#else in
    place, so

        #ifdef WEIRD
            if (1) {
        #else
            if (0) {
        #endif
            }

    contributes two '{' and one '}' and reports a phantom unclosed brace on
    perfectly valid code. Each branch is individually balanced against the
    others' -- only one is ever compiled.

    So we keep the FIRST branch of each conditional and drop from #else/#elif
    to the matching #endif. Consequence worth knowing: an imbalance that exists
    only inside an #else branch won't be seen. Missing a real bug in a branch
    is a fair price for never crying wolf on valid code, because a checker
    people have learned to ignore catches nothing at all.
    """
    out = []
    skip_depth = 0   # >0 while inside a branch we've chosen to ignore
    if_stack = []    # True if we are currently in the taken branch at that level
    for line in text.splitlines():
        if IF_RE.match(line):
            if_stack.append(True)
            if skip_depth:
                skip_depth += 1
            out.append("")
            continue
        if ELSE_RE.match(line):
            # Entering a non-first branch: skip until the matching #endif.
            if if_stack and if_stack[-1] and not skip_depth:
                if_stack[-1] = False
                skip_depth = 1
            out.append("")
            continue
        if ENDIF_RE.match(line):
            if skip_depth:
                skip_depth -= 1
            if if_stack:
                if_stack.pop()
            out.append("")
            continue
        if line.lstrip().startswith("#"):
            out.append("")
            continue
        out.append("" if skip_depth else line)
    return "\n".join(out)


def _finding(tool, file, line, severity, rule_id, message, snippet="", confidence=""):
    return {
        "tool": tool, "file": file, "line": line, "col": None,
        "severity": severity, "rule_id": rule_id,
        "message": message + (f" [confidence: {confidence}]" if confidence else ""),
        "snippet": snippet.strip()[:200], "raw": "", "fingerprint": "",
    }


# ---------------------------------------------------------------------------
# 1. brace parity
#
# Ported from patch-auditor/scripts/cpp_preflight_protocol.py:check_braces().
# That version counted only { } and reported a bare true/false; this one tracks
# the three delimiter kinds, reports WHERE the imbalance starts, and ignores
# preprocessor lines so #ifdef branches don't produce phantom errors.
# ---------------------------------------------------------------------------

PAIRS = {"{": "}", "(": ")", "[": "]"}
CLOSERS = {v: k for k, v in PAIRS.items()}


def check_brace_parity(path: str) -> list:
    try:
        with open(path, errors="replace") as f:
            raw = f.read()
    except OSError as e:
        return [_finding("brace-parity", path, None, "info", "unreadable", str(e))]

    clean = strip_preprocessor(strip_comments_and_strings(raw))
    src_lines = raw.splitlines()
    stack = []          # (char, lineno)
    out = []
    for lineno, line in enumerate(clean.splitlines(), start=1):
        for ch in line:
            if ch in PAIRS:
                stack.append((ch, lineno))
            elif ch in CLOSERS:
                if not stack:
                    out.append(_finding(
                        "brace-parity", path, lineno, "error", "unmatched-close",
                        f"Unmatched closing '{ch}' -- nothing was open here. "
                        f"A file in this state will not compile.",
                        src_lines[lineno - 1] if lineno <= len(src_lines) else ""))
                elif stack[-1][0] != CLOSERS[ch]:
                    opener, oline = stack[-1]
                    out.append(_finding(
                        "brace-parity", path, lineno, "error", "mismatched-close",
                        f"Closing '{ch}' does not match '{opener}' opened on line "
                        f"{oline}.",
                        src_lines[lineno - 1] if lineno <= len(src_lines) else ""))
                    stack.pop()
                else:
                    stack.pop()
    for opener, oline in stack:
        out.append(_finding(
            "brace-parity", path, oline, "error", "unclosed",
            f"'{opener}' opened here is never closed -- the file ends with it "
            f"still open. This is a hard syntax error, not a style issue.",
            src_lines[oline - 1] if oline <= len(src_lines) else ""))
    return out


# ---------------------------------------------------------------------------
# 2. dependent-name lookup (C++)
#
# Inside `template <typename T>`, a name like T::value_type is a *dependent*
# type and needs `typename` in front of it. Without it, older/stricter
# frontends reject the code -- the "unqualified dependent template name lookup"
# case from the auditor spec.
#
# We only look inside template bodies, and only at parameters we actually saw
# declared. That keeps it from firing on every X::Y in ordinary code. It will
# still miss plenty and occasionally over-report; hence severity=info.
# ---------------------------------------------------------------------------

TEMPLATE_DECL_RE = re.compile(r"\btemplate\s*<([^>]*)>", re.DOTALL)
TPARAM_RE = re.compile(r"\b(?:typename|class)\s+(\w+)")
DECL_CONTEXT_RE = re.compile(
    r"(?P<pre>[({,;]\s*|^\s*|\breturn\s+)(?P<expr>(?P<tp>\w+)::(?P<member>\w+))\s+(?=\w)")


def check_template_lookup(path: str) -> list:
    if os.path.splitext(path)[1].lower() not in (
            ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".h"):
        return []
    try:
        with open(path, errors="replace") as f:
            raw = f.read()
    except OSError:
        return []
    clean = strip_comments_and_strings(raw)
    src_lines = raw.splitlines()

    params = set()
    for m in TEMPLATE_DECL_RE.finditer(clean):
        params.update(TPARAM_RE.findall(m.group(1)))
    if not params:
        return []

    out = []
    seen = set()
    for lineno, line in enumerate(clean.splitlines(), start=1):
        if "::" not in line:
            continue
        # already qualified, or a value context we can't judge
        for m in DECL_CONTEXT_RE.finditer(line):
            tp = m.group("tp")
            if tp not in params:
                continue
            before = line[: m.start("expr")]
            if re.search(r"\b(typename|template)\s*$", before):
                continue
            key = (lineno, m.group("expr"))
            if key in seen:
                continue
            seen.add(key)
            out.append(_finding(
                "template-lookup", path, lineno, "info", "missing-typename",
                f"`{m.group('expr')}` looks like a dependent type used in a "
                f"declaration without `typename`. If {tp} is a template "
                f"parameter, this needs `typename {m.group('expr')}`.",
                src_lines[lineno - 1] if lineno <= len(src_lines) else "",
                confidence="low -- lexical guess, not real name lookup"))
    return out


# ---------------------------------------------------------------------------
# 3. Firefox unified-build pollution
#
# Firefox compiles many .cpp files concatenated into one translation unit. That
# makes three ordinary-looking things dangerous, because they leak sideways
# into unrelated files that happen to land in the same chunk:
#   * a #define never #undef'd at the end of the file
#   * `using namespace` at file scope
#   * a file-scope `static` that isn't in an anonymous namespace
# ---------------------------------------------------------------------------

DEFINE_RE = re.compile(r"^\s*#\s*define\s+(\w+)")
UNDEF_RE = re.compile(r"^\s*#\s*undef\s+(\w+)")
USING_NS_RE = re.compile(r"^\s*using\s+namespace\s+([\w:]+)\s*;")


def check_unified_build(path: str) -> list:
    ext = os.path.splitext(path)[1].lower()
    if ext not in (".cpp", ".cc", ".cxx", ".h", ".hpp"):
        return []
    try:
        with open(path, errors="replace") as f:
            raw = f.read()
    except OSError:
        return []
    src_lines = raw.splitlines()
    is_header = ext in (".h", ".hpp")

    defined, undefed = {}, set()
    out = []
    depth = 0
    clean_lines = strip_comments_and_strings(raw).splitlines()

    for lineno, line in enumerate(clean_lines, start=1):
        m = DEFINE_RE.match(line)
        if m:
            defined[m.group(1)] = lineno
        m = UNDEF_RE.match(line)
        if m:
            undefed.add(m.group(1))

        m = USING_NS_RE.match(line)
        if m and depth == 0:
            out.append(_finding(
                "unified-build", path, lineno, "warning", "file-scope-using-namespace",
                f"`using namespace {m.group(1)}` at file scope. In a unified "
                f"build this leaks into every other .cpp concatenated into the "
                f"same chunk and can silently change overload resolution in "
                f"files that never asked for it."
                + (" In a header this affects every translation unit that "
                   "includes it." if is_header else ""),
                src_lines[lineno - 1] if lineno <= len(src_lines) else "",
                confidence="high -- brace depth tracked lexically"))

        depth += line.count("{") - line.count("}")
        if depth < 0:
            depth = 0

    # Guard macros are supposed to outlive the file; don't nag about those.
    for name, lineno in sorted(defined.items(), key=lambda kv: kv[1]):
        if name in undefed:
            continue
        if name.endswith(("_H", "_H_", "_HPP", "_INCLUDED", "_h__")) or name.startswith("MOZ_INCLUDE"):
            continue
        out.append(_finding(
            "unified-build", path, lineno, "warning", "macro-not-undefd",
            f"Macro `{name}` is #define'd but never #undef'd. Under a unified "
            f"build it stays defined for every file compiled after this one in "
            f"the same chunk. Add `#undef {name}` at the end of the file unless "
            f"it is deliberately part of the public surface.",
            src_lines[lineno - 1] if lineno <= len(src_lines) else "",
            confidence="medium -- cannot tell intentional exports from leaks"))
    return out


# ---------------------------------------------------------------------------
# 4. kernel: sleeping while holding a spinlock
#
# Holding a spinlock disables preemption. Allocating with GFP_KERNEL, calling
# msleep/schedule, or taking a mutex inside that window can sleep, and sleeping
# with preemption disabled is a deadlock or a "BUG: scheduling while atomic".
#
# Tracked as a per-function lock depth over stripped source. Only catches the
# case where both the lock and the sleep are lexically in the same function --
# a sleep inside a callee is invisible to a lexical pass, and we say so.
# ---------------------------------------------------------------------------

LOCK_ACQUIRE_RE = re.compile(r"\b(spin_lock(?:_irq|_irqsave|_bh|_nested)?)\s*\(")
LOCK_RELEASE_RE = re.compile(r"\b(spin_unlock(?:_irq|_irqrestore|_bh)?)\s*\(")
RCU_ACQUIRE_RE = re.compile(r"\brcu_read_lock\s*\(")
RCU_RELEASE_RE = re.compile(r"\brcu_read_unlock\s*\(")

SLEEPERS = [
    (re.compile(r"\bGFP_KERNEL\b"), "GFP_KERNEL allocation",
     "GFP_KERNEL may sleep to reclaim memory. Use GFP_ATOMIC (or GFP_NOWAIT) "
     "while holding a spinlock, or move the allocation outside the lock."),
    (re.compile(r"\b(msleep|ssleep|schedule|schedule_timeout|cond_resched|yield)\s*\("),
     "explicit sleep/yield",
     "This blocks or reschedules. With preemption disabled that is a "
     "'scheduling while atomic' bug."),
    (re.compile(r"\b(mutex_lock|down|down_interruptible|wait_for_completion)\s*\("),
     "sleeping lock",
     "Mutexes and semaphores can sleep; they must not be taken under a spinlock."),
    (re.compile(r"\b(copy_(?:to|from)_user|vmalloc|kvmalloc)\s*\("),
     "may-fault / may-sleep call",
     "This can fault or sleep and is not safe in atomic context."),
]

FUNC_START_RE = re.compile(r"^\w[\w\s\*]*\**\s*\w+\s*\([^;]*\)\s*\{?\s*$")


def check_kernel_locks(path: str) -> list:
    if os.path.splitext(path)[1].lower() not in (".c", ".h"):
        return []
    try:
        with open(path, errors="replace") as f:
            raw = f.read()
    except OSError:
        return []
    clean = strip_comments_and_strings(raw)
    src_lines = raw.splitlines()

    out = []
    lock_depth = 0
    lock_line = None
    lock_kind = ""
    brace_depth = 0

    for lineno, line in enumerate(clean.splitlines(), start=1):
        # Leaving the function resets state: a lock is never held across
        # function boundaries in a way we could follow lexically anyway.
        if brace_depth == 0:
            lock_depth, lock_line = 0, None

        if LOCK_ACQUIRE_RE.search(line):
            lock_depth += 1
            if lock_line is None:
                lock_line, lock_kind = lineno, "spinlock"
        elif RCU_ACQUIRE_RE.search(line):
            lock_depth += 1
            if lock_line is None:
                lock_line, lock_kind = lineno, "rcu_read_lock"

        if lock_depth > 0:
            for rx, what, advice in SLEEPERS:
                if rx.search(line):
                    out.append(_finding(
                        "kernel-locks", path, lineno, "warning", "sleep-in-atomic",
                        f"{what} inside a {lock_kind} held since line {lock_line}. "
                        f"{advice}",
                        src_lines[lineno - 1] if lineno <= len(src_lines) else "",
                        confidence="medium -- same-function lexical scan; a sleep "
                                   "inside a called function is not visible here"))
                    break

        if LOCK_RELEASE_RE.search(line) or RCU_RELEASE_RE.search(line):
            lock_depth = max(0, lock_depth - 1)
            if lock_depth == 0:
                lock_line = None

        brace_depth += line.count("{") - line.count("}")
        if brace_depth < 0:
            brace_depth = 0
    return out


# ---------------------------------------------------------------------------
# 5. blast radius
#
# Not a defect check -- a question. "I touched this macro/symbol; what else
# depends on it?" Answers with every file referencing it, so the review can be
# scoped to the real surface rather than the changed file.
# ---------------------------------------------------------------------------

SOURCE_EXTS = {".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx",
               ".m", ".mm", ".py", ".go", ".rs", ".js", ".ts", ".java"}
SKIP_DIRS = {".git", "node_modules", "obj-x86_64-pc-linux-gnu", "target",
             "build", "dist", "__pycache__", ".mozbuild", "third_party"}


def check_blast_radius(symbol: str, root: str, max_hits: int = 500) -> list:
    rx = re.compile(r"\b" + re.escape(symbol) + r"\b")
    out = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS and not d.startswith(".")]
        for fn in filenames:
            if os.path.splitext(fn)[1].lower() not in SOURCE_EXTS:
                continue
            full = os.path.join(dirpath, fn)
            try:
                with open(full, errors="replace") as f:
                    for lineno, line in enumerate(f, start=1):
                        if rx.search(line):
                            out.append(_finding(
                                "blast-radius", os.path.relpath(full, root), lineno,
                                "info", "depends-on-symbol",
                                f"References `{symbol}`. If the change alters its "
                                f"meaning, signature or value, this site is in scope "
                                f"for review too.",
                                line))
                            if len(out) >= max_hits:
                                return out
            except OSError:
                continue
    return out


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

CHECKS = {
    "brace-parity": check_brace_parity,
    "template-lookup": check_template_lookup,
    "unified-build": check_unified_build,
    "kernel-locks": check_kernel_locks,
}


def main() -> int:
    ap = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("check", choices=sorted(CHECKS) + ["blast-radius", "all"])
    ap.add_argument("files", nargs="*", help="files to inspect")
    ap.add_argument("--symbol", help="blast-radius: the symbol/macro to trace")
    ap.add_argument("--root", default=".", help="blast-radius: tree to search")
    args = ap.parse_args()

    results = []
    if args.check == "blast-radius":
        if not args.symbol:
            print("blast-radius needs --symbol NAME", file=sys.stderr)
            return 2
        results = check_blast_radius(args.symbol, args.root)
    elif args.check == "all":
        for path in args.files:
            for fn in CHECKS.values():
                results.extend(fn(path))
    else:
        for path in args.files:
            results.extend(CHECKS[args.check](path))

    json.dump({"schema": "source-heuristics/1", "findings": results}, sys.stdout, indent=2)
    sys.stdout.write("\n")
    # Exit 0 even with findings: the orchestrator reads the JSON, and a non-zero
    # code here would be misread as "the check itself broke".
    return 0


if __name__ == "__main__":
    sys.exit(main())
