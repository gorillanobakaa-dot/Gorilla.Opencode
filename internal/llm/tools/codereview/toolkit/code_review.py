#!/usr/bin/env python3
"""
code_review.py
---------------
Point this at a directory (or a single file). It will:

  1. Figure out what kind of project it's looking at (a mozilla-central
     checkout, a Linux kernel tree, a Go module, a Rust crate, a Python
     project, or just "generic").
  2. Work out which files are actually in scope (git-tracked files by
     default, or just your uncommitted/recent changes with --diff).
  3. Run every relevant *safe-to-automate* tool in parallel, stage by
     stage, each one's raw output going to its own log file in a results
     folder.
  4. Watch what stage 1/2 found -- if anything looks security-flavored,
     automatically escalate to a stage 3 deep-dive on just those files.
  5. Print a rolling status line as each tool finishes, then write a full
     JSON + Markdown report.
  6. For anything that genuinely needs a project-specific build context
     (Firefox's mach, the kernel's own make targets, valgrind, scan-build)
     it prints the exact command to run by hand instead of guessing.
  7. Optionally, hand the whole aggregate to any LLM (local or remote,
     any vendor) for a prioritized, plain-English synthesis.

Examples:
    python3 code_review.py ~/src/gorilla-opencode
    python3 code_review.py ~/src/chroma --diff origin/main
    python3 code_review.py ~/src/firefox/dom/canvas/WebGLContext.cpp
    python3 code_review.py ~/src/linux --profile linux-kernel --diff HEAD~1
    python3 code_review.py ~/src/myproj --llm-endpoint http://localhost:11434/v1/chat/completions \\
                           --llm-model llama3 --llm-api-style openai

Nothing here calls any cloud API unless you explicitly pass --llm-endpoint.
"""

import argparse
import concurrent.futures
import json
import os
import re
import subprocess
import sys
import time
from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional, Set

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from tools_registry import (  # noqa: E402
    TOOLS, TOOLS_BY_ID, EXTENSION_LANGUAGE, MAKEFILE_NAMES,
    DEFAULT_IGNORE_DIRS, ESCALATION_KEYWORDS, Tool, unparsed_tool_ids,
)
import llm_client  # noqa: E402
import findings as fnd  # noqa: E402
import rules  # noqa: E402
import local_tier  # noqa: E402
import doctor  # noqa: E402
import baseline  # noqa: E402

TOOLKIT_DIR = os.path.dirname(os.path.abspath(__file__))

# Our own log header is "$ cd <cwd> && <argv>" followed by a blank line. Every
# reader below skips exactly this many lines, because the header contains the
# tool's command line -- and matching keywords against that is how a clean
# semgrep run used to escalate itself for mentioning "security".
LOG_HEADER_LINES = 2

KERNEL_FIREFOX_MANUAL_IDS = {
    "firefox-mach-lint", "firefox-mach-static-analysis",
    "kernel-checkpatch", "kernel-clang-tidy", "kernel-clang-analyzer",
    "kernel-sparse", "kernel-coccicheck",
}

# Tools whose checks are redundant with (or actively conflict with) a tree's
# own bundled tooling, so we skip the generic version under that profile.
SKIP_FOR_PROFILE = {
    "firefox": {"eslint", "prettier-check", "cpplint"},
    "linux-kernel": {"cpplint"},
}

SYSTEM_PROMPT = """You are the review-synthesis stage of a local, offline code-review pipeline.
You are NOT running any tools yourself -- everything you need is already in the message below,
which was produced by real static-analysis/lint/security tools (cppcheck, clang-tidy, pylint,
flake8, mypy, bandit, eslint, stylelint, cargo clippy, golangci-lint, gosec, staticcheck,
semgrep, gitleaks, shellcheck, etc.) run directly against the user's source tree.

Follow these rules exactly, in order:

1. Only report findings that are literally present in the text you were given. Do not invent
   file names, line numbers, function names, or issues that aren't in the tool output. If you
   are not sure a finding is real, say so instead of guessing.
2. Group findings by severity, not by tool:
     P0 - security or correctness issues multiple different tools flagged in the same file/area
          (these are the highest-confidence real problems)
     P1 - security findings from a single tool (bandit/gosec/semgrep/gitleaks/cargo-audit)
     P2 - correctness/logic warnings from static analyzers (cppcheck/clang-tidy/mypy/staticcheck)
     P3 - style/formatting only (black/isort/prettier/clang-format/eslint style rules)
3. For each P0/P1 item, state which file, which tool(s) flagged it, and in one plain sentence
   what the risk actually is -- explain it to the developer who wrote the code, don't just
   recite the tool's own jargon back at them.
4. Merge duplicate findings across tools instead of repeating them ("confirmed by clang-tidy
   and cppcheck", once).
5. If a whole category found nothing, say so in one short line instead of omitting it silently.
6. End with a "fix first" list of at most 5 items, ordered by actual risk, not by output volume.
7. Keep the reply under ~600 words unless there are genuinely more than 5 P0 findings, in which
   case list all P0s but keep everything else terse.

You will be given: which project profile was detected, a recon summary, per-tool finding
counts, and raw-output excerpts for anything that found issues. Clean tools are only named,
not quoted in full.
"""

JOB_TIMEOUT_DEFAULT = 600


# ---------------------------------------------------------------------------
# small process helper (mirrors install_tools.py's, kept independent on purpose
# so this file has no import-time dependency on install_tools.py)
# ---------------------------------------------------------------------------

def run(cmd, timeout=30, cwd=None):
    try:
        # check=False is deliberate and explicit: every caller inspects the
        # return code itself (127 = not installed, 124 = timed out, non-zero
        # with no findings = the tool broke). Raising here would collapse
        # those distinctions, which are the ones this toolkit cares most about.
        p = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout,
                           cwd=cwd, check=False)
        return p.returncode, p.stdout, p.stderr
    except FileNotFoundError:
        return 127, "", "not found"
    except subprocess.TimeoutExpired:
        return 124, "", "timed out"
    except Exception as e:
        return 1, "", str(e)


def sanitize(s: str) -> str:
    return re.sub(r"[^A-Za-z0-9_.-]", "_", s)[-140:]


def classify(path: str) -> Optional[str]:
    base = os.path.basename(path)
    if base in MAKEFILE_NAMES:
        return "make"
    ext = os.path.splitext(path)[1].lower()
    return EXTENSION_LANGUAGE.get(ext)


def languages_of(files: List[str]) -> Set[str]:
    """The set of languages present in `files`, unknown extensions excluded.

    classify() returns None for anything we don't recognise, so building the set
    and then discarding None (as four separate call sites used to do) produced a
    Set[Optional[str]] that only *looked* like a Set[str] -- which then made
    `sorted()` on it a latent crash the moment an unrecognised file appeared
    alongside a recognised one. Filtering inside the comprehension is both
    correct and says what it means.
    """
    return {lang for lang in (classify(f) for f in files) if lang is not None}


# ---------------------------------------------------------------------------
# project profile detection
# ---------------------------------------------------------------------------

def detect_profile(target_dir: str) -> str:
    def exists(*parts):
        return os.path.exists(os.path.join(target_dir, *parts))

    if exists("mach") and exists("python", "mozbuild"):
        return "firefox"
    if exists("Kconfig") and exists("MAINTAINERS") and exists("scripts", "checkpatch.pl"):
        return "linux-kernel"
    if exists("go.mod"):
        return "go"
    if exists("Cargo.toml"):
        return "rust"
    if exists("pyproject.toml") or exists("setup.py") or exists("requirements.txt"):
        return "python"
    return "generic"


# ---------------------------------------------------------------------------
# file discovery
# ---------------------------------------------------------------------------

def is_git_repo(path: str) -> bool:
    return run(["git", "-C", path, "rev-parse", "--is-inside-work-tree"], timeout=10)[0] == 0


def discover_files(target: str, target_is_file: bool, args) -> List[str]:
    if target_is_file:
        return [target]

    if args.diff:
        if not is_git_repo(target):
            print(f"ERROR: --diff was given but {target} is not a git repository.")
            sys.exit(1)
        rc, out, err = run(["git", "-C", target, "diff", "--name-only", args.diff], timeout=30)
        if rc != 0:
            print(f"ERROR: `git diff --name-only {args.diff}` failed: {err.strip()}")
            sys.exit(1)
        rels = [l for l in out.splitlines() if l.strip()]
        return [os.path.join(target, r) for r in rels if os.path.isfile(os.path.join(target, r))]

    if not args.all_files and is_git_repo(target):
        rc, out, _ = run(["git", "-C", target, "ls-files"], timeout=60)
        rels = [l for l in out.splitlines() if l.strip()]
        return [os.path.join(target, r) for r in rels if os.path.isfile(os.path.join(target, r))]

    out = []
    for root, dirs, fnames in os.walk(target):
        dirs[:] = [d for d in dirs if d not in DEFAULT_IGNORE_DIRS and not d.startswith(".")]
        for fn in fnames:
            out.append(os.path.join(root, fn))
    return out


# ---------------------------------------------------------------------------
# job construction / execution
# ---------------------------------------------------------------------------

@dataclass
class Job:
    tool_id: str
    target_path: str
    argv: List[str]
    log_path: str
    cwd: str

@dataclass
class JobResult:
    job: Job
    returncode: int
    duration: float
    issue_count: int = 0
    findings: List[fnd.Finding] = field(default_factory=list)


def build_ctx(target_dir: str, results_dir: str, args, rel_path: str = "",
              profile: str = "generic") -> dict:
    def any_exists(names):
        return any(os.path.exists(os.path.join(target_dir, n)) for n in names)

    return {
        "target": target_dir,
        "results_dir": results_dir,
        # Some checks are only meaningful under a specific tree layout (Firefox
        # unified builds, kernel locking). Their build_cmd returns None when the
        # profile doesn't match, which is the registry's skip signal.
        "profile": profile,
        "compile_commands_dir": args.compile_commands_dir,
        "has_own_eslint_config": any_exists(
            [".eslintrc", ".eslintrc.js", ".eslintrc.cjs", ".eslintrc.json",
             ".eslintrc.yml", ".eslintrc.yaml", "eslint.config.js", "eslint.config.mjs"]),
        "eslint_fallback_config": os.path.join(TOOLKIT_DIR, "configs", "eslintrc.fallback.json"),
        "has_own_stylelint_config": any_exists(
            [".stylelintrc", ".stylelintrc.json", ".stylelintrc.js", "stylelint.config.js"]),
        "stylelint_fallback_config": os.path.join(TOOLKIT_DIR, "configs", "stylelintrc.fallback.json"),
        "semgrep_language_packs": [],
        "rel_path": rel_path,
        "kernel_subdir": "",
    }


def make_job(tool: Tool, path: str, argv: List[str], results_dir: str, cwd: str) -> Job:
    rel = os.path.relpath(path, start=cwd) if os.path.isabs(path) else path
    if rel in (".", ""):
        rel = "PROJECT"
    log_name = f"{tool.id}__{sanitize(rel)}.txt"
    return Job(tool_id=tool.id, target_path=path, argv=argv,
               log_path=os.path.join(results_dir, log_name), cwd=cwd)


def build_jobs(files: List[str], target_dir: str, profile: str, results_dir: str,
               ctx: dict, stages) -> List[Job]:
    jobs = []
    langs_present: Set[str] = languages_of(files)
    skip_ids = SKIP_FOR_PROFILE.get(profile, set())

    for tool in TOOLS:
        if tool.scope not in ("auto-file", "auto-project") or tool.stage not in stages:
            continue
        if tool.id in skip_ids:
            continue

        if "*" in tool.languages:
            if tool.build_cmd is None:
                continue
            argv = tool.build_cmd(target_dir, ctx)
            if argv:
                jobs.append(make_job(tool, target_dir, argv, results_dir, target_dir))
            continue

        if tool.scope == "auto-project":
            if tool.build_cmd is not None and (set(tool.languages) & langs_present):
                argv = tool.build_cmd(target_dir, ctx)
                if argv:
                    jobs.append(make_job(tool, target_dir, argv, results_dir, target_dir))
            continue

        # auto-file
        if tool.build_cmd is None:
            continue
        for f in files:
            if classify(f) not in tool.languages:
                continue
            argv = tool.build_cmd(f, ctx)
            if argv:
                jobs.append(make_job(tool, f, argv, results_dir, target_dir))
    return jobs


def build_stage3_jobs(files: List[str], target_dir: str, results_dir: str, ctx: dict,
                       hit_files: Set[str], deep: bool) -> List[Job]:
    jobs = []
    langs_present: Set[str] = languages_of(files)

    for tool in TOOLS:
        if tool.stage != 3 or tool.scope not in ("auto-file", "auto-project"):
            continue
        if tool.scope == "auto-project":
            if tool.build_cmd is None:
                continue
            if "*" in tool.languages or (set(tool.languages) & langs_present):
                if deep or hit_files:
                    argv = tool.build_cmd(target_dir, ctx)
                    if argv:
                        jobs.append(make_job(tool, target_dir, argv, results_dir, target_dir))
            continue
        if tool.build_cmd is None:
            continue
        candidates = files if deep else [f for f in files if f in hit_files]
        for f in candidates:
            if classify(f) not in tool.languages:
                continue
            argv = tool.build_cmd(f, ctx)
            if argv:
                jobs.append(make_job(tool, f, argv, results_dir, target_dir))
    return jobs


def run_job(job: Job, timeout: int) -> JobResult:
    t0 = time.time()
    header = f"$ cd {job.cwd} && {' '.join(job.argv)}\n\n"
    try:
        with open(job.log_path, "w") as f:
            f.write(header)
            f.flush()
            # check=False: an analyser exiting non-zero usually means it FOUND
            # something, which is success from our side. execute_jobs()
            # interprets the code.
            proc = subprocess.run(job.argv, cwd=job.cwd, stdout=subprocess.PIPE,
                                   stderr=subprocess.STDOUT, text=True,
                                   timeout=timeout, check=False)
            f.write(proc.stdout or "")
        rc = proc.returncode
    except subprocess.TimeoutExpired:
        rc = 124
        with open(job.log_path, "a") as f:
            f.write(f"\n[TIMED OUT after {timeout}s]\n")
    except FileNotFoundError:
        rc = 127
        with open(job.log_path, "a") as f:
            f.write("\n[TOOL BINARY NOT FOUND -- run install_tools.py first]\n")
    except Exception as e:
        rc = 1
        with open(job.log_path, "a") as f:
            f.write(f"\n[UNEXPECTED ERROR: {e}]\n")
    return JobResult(job=job, returncode=rc, duration=time.time() - t0)


def read_log_body(log_path: str) -> str:
    """A tool's output with our own command-line header stripped off."""
    try:
        with open(log_path, errors="replace") as f:
            return "\n".join(f.read().splitlines()[LOG_HEADER_LINES:])
    except OSError:
        return ""


def parse_job_findings(result: JobResult) -> List[fnd.Finding]:
    """Normalised findings for one job.

    Uses the tool's own parser when the registry has one, otherwise the marker
    heuristic that every tool used before findings.py -- so a tool without a
    parser behaves exactly as it did, and adding one is never a regression.
    """
    tool = TOOLS_BY_ID.get(result.job.tool_id)
    if not tool:
        return []
    body = read_log_body(result.job.log_path)
    if not body.strip():
        return []
    cwd = result.job.cwd
    ctx = {
        "tool_id": tool.id,
        "cwd": cwd,
        "target_path": os.path.relpath(result.job.target_path, start=cwd)
                       if os.path.isabs(result.job.target_path) else result.job.target_path,
        "severity_markers": tool.severity_markers,
    }
    parser = tool.parse_output or fnd.parse_marker_fallback
    try:
        out = parser(body, ctx)
    except Exception as e:
        # A parser bug must never take the run down or -- worse -- silently
        # report zero findings, which is indistinguishable from "clean".
        print(f"  ! parser for {tool.id} failed ({e}); falling back to marker heuristic")
        out = fnd.parse_marker_fallback(body, ctx)
    return fnd.finalize(out, cwd=cwd)


def execute_jobs(jobs: List[Job], max_workers: int, timeout: int) -> List[JobResult]:
    if not jobs:
        return []
    results = []
    total = len(jobs)
    done = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as ex:
        futures = {ex.submit(run_job, j, timeout): j for j in jobs}
        for fut in concurrent.futures.as_completed(futures):
            j = futures[fut]
            r = fut.result()
            r.findings = parse_job_findings(r)
            r.issue_count = len(r.findings)
            done += 1
            if r.returncode == 127:
                tag = "MISSING"
            elif r.returncode == 124:
                tag = "TIMEOUT"
            elif r.issue_count > 0:
                tag = "ISSUES"
            elif r.returncode != 0:
                # exited non-zero but we didn't parse any findings from its output --
                # more likely a crash / bad config / network failure than "all clean".
                tag = "ERROR"
            else:
                tag = "CLEAN"
            rel = os.path.relpath(j.target_path, start=j.cwd) if os.path.isabs(j.target_path) else j.target_path
            print(f"  [{done:>4}/{total}] {tag:<8} {j.tool_id:<24} {rel:<48} "
                  f"{r.issue_count:>4} finding(s)  ({r.duration:5.1f}s)")
            results.append(r)
    return results


def find_escalation_hits(results: List[JobResult]) -> Set[str]:
    """Files whose stage 1/2 output looked security-shaped, for stage 3.

    Reads the log BODY only. Reading the whole file used to include our own
    "$ cd ... && semgrep --config p/security-audit" header, so any tool whose
    command line happened to contain "security" or "secret" escalated its own
    target on a completely clean run.
    """
    hits = set()
    for r in results:
        if r.returncode in (124, 127):
            continue
        content = read_log_body(r.job.log_path).lower()
        if not content:
            continue
        if any(k.lower() in content for k in ESCALATION_KEYWORDS):
            hits.add(r.job.target_path)
    return hits


# ---------------------------------------------------------------------------
# manual-tier tools (things this script deliberately does NOT auto-run)
# ---------------------------------------------------------------------------

def select_manual_tools(profile: str, files: List[str]) -> List[Tool]:
    langs_present: Set[str] = languages_of(files)
    out = []
    for tool in TOOLS:
        if tool.scope != "manual":
            continue
        if tool.id in KERNEL_FIREFOX_MANUAL_IDS:
            if profile in tool.profiles:
                out.append(tool)
        else:
            if "*" in tool.languages or (set(tool.languages) & langs_present):
                out.append(tool)
    return out


def render_manual_block(tool: Tool, ctx: dict) -> str:
    # Every scope="manual" tool defines manual_cmd, but the field is Optional on
    # the dataclass, so say what happens if one ever doesn't rather than raising
    # a TypeError deep inside report rendering.
    cmd_text = tool.manual_cmd(ctx) if tool.manual_cmd else "(no command defined for this tool)"
    lines = [f"### {tool.label}"]
    if tool.notes:
        lines.append(f"_{tool.notes}_")
    lines.append("")
    lines.append("**Step 1 -- run this exactly:**")
    lines.append("```bash")
    lines.append(cmd_text)
    lines.append("```")
    lines.append("**Step 2 -- save the output instead of letting it scroll past:**")
    first_line = cmd_text.strip().splitlines()[0]
    lines.append("```bash")
    lines.append(f"({first_line}) > {os.path.join(ctx['results_dir'], tool.id + '.txt')} 2>&1")
    lines.append("```")
    lines.append(f"**Step 3** -- once it finishes, open `{tool.id}.txt` and search first for "
                  f"the words `error`, `warning`, or `FAIL`.")
    return "\n".join(lines)


def render_checklist_block(tool: Tool) -> str:
    lines = [f"### {tool.label}"]
    if tool.notes:
        lines.append(f"_{tool.notes}_")
    lines.append("")
    for item in (tool.checklist or []):
        lines.append(f"- [ ] {item}")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# report building
# ---------------------------------------------------------------------------

def build_digest(all_results: List[JobResult], profile: str, max_chars: int = 60000) -> str:
    lines = [f"Detected project profile: {profile}", ""]
    clean, dirty, errored, missing = [], [], [], []
    for r in all_results:
        if r.returncode == 127:
            missing.append(r)
        elif r.issue_count > 0:
            dirty.append(r)
        elif r.returncode not in (0, 124):
            errored.append(r)
        else:
            clean.append(r)
    lines.append(f"{len(dirty)} tool run(s) found something; {len(clean)} were genuinely clean; "
                 f"{len(errored)} FAILED TO RUN PROPERLY (do not treat these as clean -- e.g. a "
                 f"network-blocked ruleset download); {len(missing)} tool(s) were not installed.")
    lines.append("")
    if errored:
        lines.append("Tools that errored (exit code nonzero, no findings parsed -- likely a real "
                     "problem, NOT a clean pass): " + ", ".join(
                         sorted({f"{r.job.tool_id} ({os.path.basename(r.job.log_path)})" for r in errored})))
        lines.append("")
    if clean:
        lines.append("Clean (no findings): " + ", ".join(
            sorted({f"{r.job.tool_id}" for r in clean})))
        lines.append("")
    # Corroboration is now a computed fact rather than something the model has
    # to notice in prose, so hand it over pre-chewed. These are the P0s.
    groups = fnd.corroborated([f for r in all_results for f in r.findings])
    if groups:
        lines.append("Places flagged INDEPENDENTLY BY MORE THAN ONE TOOL (treat as the "
                     "highest-confidence findings -- computed by exact file:line "
                     "match, not inferred; style/formatting findings are excluded "
                     "so this is not just two linters agreeing about whitespace):")
        for g in groups[:40]:
            tools = ", ".join(sorted({f.tool for f in g}))
            lines.append(f"  {g[0].file}:{g[0].line}  [{tools}]  {g[0].message[:160]}")
        lines.append("")

    dirty.sort(key=lambda r: -r.issue_count)
    budget = max_chars
    for r in dirty:
        rel = os.path.relpath(r.job.target_path, start=r.job.cwd) if os.path.isabs(r.job.target_path) else r.job.target_path
        header = f"--- {r.job.tool_id} on {rel} ({r.issue_count} findings) ---\n"
        body_lines = read_log_body(r.job.log_path).splitlines()
        body = "\n".join(body_lines[:60])
        block = header + body + "\n\n"
        if len(block) > budget:
            break
        lines.append(block)
        budget -= len(block)
    return "\n".join(lines)


def write_reports(results_dir: str, target: str, profile: str, files: List[str],
                   all_results: List[JobResult], manual_tools: List[Tool],
                   checklist_tools: List[Tool], ctx: dict, llm_synthesis: Optional[str]) -> str:
    all_findings = fnd.finalize([f for r in all_results for f in r.findings])
    groups = fnd.corroborated(all_findings)
    style_only = fnd.style_agreements(all_findings)
    errored_ids = sorted({r.job.tool_id for r in all_results
                          if r.issue_count == 0 and r.returncode not in (0, 124, 127)})
    missing_ids = sorted({r.job.tool_id for r in all_results if r.returncode == 127})

    json_report = {
        "generated": datetime.now().isoformat(),
        "target": target,
        "profile": profile,
        "files_scanned": len(files),
        # The findings themselves, not just how many there were. This is what an
        # agent consumes; everything else here is provenance for it.
        "findings": [f.to_dict() for f in all_findings],
        # Same place flagged by more than one tool -- computed, not inferred.
        # Style-only agreements are counted separately, not listed: two linters
        # agreeing about line length is not evidence of a bug.
        "corroborated_excludes_style": True,
        "style_only_agreements": style_only,
        "corroborated": [
            {"file": g[0].file, "line": g[0].line,
             "tools": sorted({f.tool for f in g}),
             "messages": [f.message for f in g]}
            for g in groups
        ],
        # Trust metadata. A consumer needs to know that an empty findings list
        # can mean "clean", "the tool crashed", or "the tool isn't installed" --
        # and these are emphatically not the same thing.
        "trust": {
            "tools_errored": errored_ids,
            "tools_missing": missing_ids,
            "tools_without_parser": [t for t in unparsed_tool_ids()
                                     if t in {r.job.tool_id for r in all_results}],
        },
        "jobs": [
            {
                "tool": r.job.tool_id,
                "path": r.job.target_path,
                "returncode": r.returncode,
                "issue_count": r.issue_count,
                "duration_s": round(r.duration, 2),
                "log": r.job.log_path,
            } for r in all_results
        ],
        "manual_tools": [t.id for t in manual_tools],
        "checklist_tools": [t.id for t in checklist_tools],
    }
    json_path = os.path.join(results_dir, "report.json")
    with open(json_path, "w") as f:
        json.dump(json_report, f, indent=2)

    md = ["# Code Review Report", "", f"- Target: `{target}`", f"- Detected profile: **{profile}**",
          f"- Files in scope: {len(files)}", f"- Generated: {datetime.now().isoformat()}", ""]

    md.append("## Findings by tool")
    md.append("")
    md.append("| Tool | Findings | Files touched | Log files |")
    md.append("|---|---|---|---|")
    by_tool: Dict[str, List[JobResult]] = {}
    for r in all_results:
        by_tool.setdefault(r.job.tool_id, []).append(r)
    for tool_id, rs in sorted(by_tool.items(), key=lambda kv: -sum(r.issue_count for r in kv[1])):
        total_issues = sum(r.issue_count for r in rs)
        md.append(f"| {tool_id} | {total_issues} | {len(rs)} | see `.code_review/.../{tool_id}__*.txt` |")
    md.append("")

    errored = [r for r in all_results if r.issue_count == 0 and r.returncode not in (0, 124, 127)]
    if errored:
        md.append("## ⚠ Tool runs that failed (do NOT read these as \"clean\")")
        md.append("")
        for r in errored:
            md.append(f"- `{r.job.tool_id}` exited with code {r.returncode} and produced no parseable "
                      f"findings -- check `{os.path.basename(r.job.log_path)}` (common cause: a "
                      f"registry/ruleset download was blocked, or a real config error).")
        md.append("")

    if manual_tools:
        md.append("## Manual steps (need project-specific build context)")
        md.append("")
        for t in manual_tools:
            md.append(render_manual_block(t, ctx))
            md.append("")

    if checklist_tools:
        md.append("## Manual review checklists")
        md.append("")
        for t in checklist_tools:
            md.append(render_checklist_block(t))
            md.append("")

    if llm_synthesis:
        md.append("## LLM synthesis")
        md.append("")
        md.append(llm_synthesis)
        md.append("")

    md_path = os.path.join(results_dir, "REPORT.md")
    with open(md_path, "w") as f:
        f.write("\n".join(md))

    return md_path


# ---------------------------------------------------------------------------
# agent-facing output
#
# The Markdown report is written for a person: prose, tables, tick-boxes, "run
# this exactly". An agent needs the opposite -- the findings themselves, plus
# enough honesty about what DIDN'T run that it can't mistake silence for a
# clean bill of health. That distinction is the entire reason this exists as a
# separate function rather than a flag on the Markdown writer.
# ---------------------------------------------------------------------------

def emit_agent_json(results_dir, target, profile, files, all_results,
                    manual_tools, checklist_tools, ctx, pos_report,
                    diff_summary=None, suppressed_n=0,
                    baseline_problems=None) -> None:
    baseline_problems = baseline_problems or []
    all_findings = fnd.finalize([f for r in all_results for f in r.findings])
    groups = fnd.corroborated(all_findings)
    style_only = fnd.style_agreements(all_findings)

    errored = sorted({r.job.tool_id for r in all_results
                      if r.issue_count == 0 and r.returncode not in (0, 124, 127)})
    missing = sorted({r.job.tool_id for r in all_results if r.returncode == 127})
    timed_out = sorted({r.job.tool_id for r in all_results if r.returncode == 124})
    ran_ids = {r.job.tool_id for r in all_results}

    langs = sorted(languages_of(files))

    doc = {
        "schema": "code-review/agent/1",
        "generated": datetime.now().isoformat(),
        "target": target,
        "profile": profile,
        "files_scanned": len(files),
        "languages": langs,

        "findings": [f.to_dict() for f in all_findings],

        # Highest-confidence findings: same file:line, two or more DIFFERENT
        # tools, style severity excluded. Start here.
        "corroborated": [
            {"file": g[0].file, "line": g[0].line,
             "tools": sorted({f.tool for f in g}),
             "severities": sorted({f.severity for f in g}),
             "messages": [f.message for f in g]}
            for g in groups
        ],
        "corroborated_excludes_style": True,
        "style_only_agreements": style_only,

        # Present only when --diff was used. Read `note` before assuming the
        # findings are limited to the change under review -- they are not.
        "diff": diff_summary,

        # Findings this project has examined and accepted (.code-review-baseline
        # .json). They remain in `findings` with suppressed=true and a written
        # reason; they are excluded from corroboration and should not be
        # re-reported as new problems.
        "suppressed_by_baseline": suppressed_n,
        "baseline_problems": baseline_problems,

        # Read this before concluding anything from an empty findings list.
        "trust": {
            "tools_ran": sorted(ran_ids),
            "tools_errored": errored,
            "tools_missing": missing,
            "tools_timed_out": timed_out,
            "tools_without_parser": [t for t in unparsed_tool_ids() if t in ran_ids],
            "position_checked": pos_report is not None,
            "findings_dropped_by_position_check":
                len(pos_report.dropped) if pos_report else 0,
            "caveat": (
                "An empty findings list is NOT the same as clean code. Tools in "
                "tools_missing never ran; tools in tools_errored failed to run and "
                "produced nothing parseable. Only treat a language as reviewed if "
                "its tools appear in tools_ran and not in the other lists. Findings "
                "from tools_without_parser may lack precise line numbers. Static "
                "analysers do not find semantic bugs -- wrong logic, broken "
                "invariants, swallowed errors -- so read the changed code yourself "
                "as well as this list."
            ),
            # A tool objecting to nearly every file is usually measuring the
            # environment, not the code. Reported here so it sits beside the
            # other reasons a reader should not take the list at face value.
            "uniformity_warnings": fnd.uniformity_warnings(
                all_findings, len(files) if files else 0),
        },

        # Review know-how for the languages present, so the reader knows what to
        # look for in the code that no tool here can check.
        "review_rules": rules.rules_for_languages(langs),

        # Things this toolkit deliberately refuses to auto-run, as literal
        # commands. An agent CAN run these -- it has a shell -- so they are
        # offered rather than merely described.
        "manual_steps": [
            {"id": t.id, "label": t.label, "why": t.notes,
             "command": t.manual_cmd(ctx) if t.manual_cmd else ""}
            for t in manual_tools
        ],
        "checklists": [
            {"id": t.id, "label": t.label, "items": t.checklist or []}
            for t in checklist_tools
        ],

        "logs_dir": results_dir,
        "human_report": os.path.join(results_dir, "REPORT.md"),
    }
    json.dump(doc, sys.stdout, indent=2)
    sys.stdout.write("\n")


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def parse_args():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("target", nargs="?", default=".",
                     help="directory or single file to review (default: current directory)")
    ap.add_argument("--doctor", action="store_true",
                     help="check whether this machine can actually review anything: which "
                          "analysers are present per language, what's missing, how much needs "
                          "downloading, how much disk that takes, and how to fix it. "
                          "Reviews nothing; exits 3 if no analyser is installed.")
    ap.add_argument("--profile", default="auto",
                     choices=["auto", "firefox", "linux-kernel", "go", "rust", "python", "generic"])
    ap.add_argument("--diff", default=None, metavar="REF",
                     help="only review files changed vs this git ref (e.g. HEAD~1, origin/main)")
    ap.add_argument("--all-files", action="store_true",
                     help="walk the whole tree instead of using `git ls-files` (includes untracked/build dirs)")
    ap.add_argument("--max-files", type=int, default=2000,
                     help="safety cap on files scanned in one run when not using --diff (default 2000)")
    ap.add_argument("--jobs", type=int, default=os.cpu_count() or 4, help="parallel workers")
    ap.add_argument("--timeout", type=int, default=JOB_TIMEOUT_DEFAULT, help="per-job timeout in seconds")
    ap.add_argument("--deep", action="store_true", help="force stage 3 deep-dive tools on everything")
    ap.add_argument("--no-stage3", action="store_true", help="never auto-escalate to stage 3")
    ap.add_argument("--skip-preflight", action="store_true",
                    help="run even if the preflight finds NO analysis tools installed (not recommended)")
    ap.add_argument("--compile-commands-dir", default=None,
                     help="directory containing compile_commands.json, enables clang-tidy")
    ap.add_argument("--results-dir", default=None, help="override where logs/reports are written")
    ap.add_argument("--audience", default="human", choices=["human", "agent"],
                     help="'agent' prints the normalised findings as JSON on stdout and keeps "
                          "all progress chatter on stderr, so a coding agent can consume this "
                          "tool directly instead of parsing per-tool log files")
    ap.add_argument("--no-position-check", action="store_true",
                     help="keep findings whose file/line can't be verified (not recommended)")
    ap.add_argument("--local-glue", action="store_true",
                     help="use a small LOCAL model (ollama etc) for the two jobs such a model "
                          "can actually do: rephrasing tool jargon into plain English, and "
                          "flagging probable noise. It never finds, drops or reorders findings.")
    ap.add_argument("--local-endpoint", default=local_tier.DEFAULT_ENDPOINT,
                     help=f"local OpenAI-compatible endpoint (default {local_tier.DEFAULT_ENDPOINT})")
    ap.add_argument("--local-model", default=local_tier.DEFAULT_MODEL,
                     help=f"local model name (default {local_tier.DEFAULT_MODEL})")
    ap.add_argument("--llm-endpoint", default=None, help="full URL, e.g. http://localhost:11434/v1/chat/completions")
    ap.add_argument("--llm-model", default=None, help="model name as your endpoint expects it")
    ap.add_argument("--llm-api-style", default="none", choices=["openai", "anthropic", "none"])
    ap.add_argument("--llm-api-key-env", default=None,
                     help="name of an env var holding your API key, if your endpoint needs one")
    return ap.parse_args()


# ---------------------------------------------------------------------------
# preflight: verify the tools that WOULD run are actually installed BEFORE we
# do any work. Without this, running against a machine with no tools installed
# produces an all-"MISSING" report that a low-reasoning caller can mistake for
# "no problems found" -- the most dangerous possible false negative.
# ---------------------------------------------------------------------------

def tool_available(tool) -> bool:
    """True if the tool's binary exists. Reuses each tool's own check_cmd; a
    return code of 127 (or FileNotFound, which run() maps to 127) means the
    command isn't on PATH. Any other code means it's installed."""
    if not tool.check_cmd:
        return True  # recon/manual placeholders have no binary to check
    rc, _, _ = run(tool.check_cmd, timeout=15)
    return rc != 127


def relevant_tools(profile: str, files: List[str], stages=(0, 1, 2)) -> List[Tool]:
    """The auto-run tools that WOULD fire for this profile + the languages
    actually present in scope -- i.e. exactly what build_jobs would select."""
    langs = languages_of(files)
    skip = SKIP_FOR_PROFILE.get(profile, set())
    out = []
    for t in TOOLS:
        if t.scope not in ("auto-file", "auto-project") or t.stage not in stages:
            continue
        if t.id in skip:
            continue
        if "*" in t.languages or (set(t.languages) & langs):
            out.append(t)
    return out


def preflight(profile: str, files: List[str], skip_gate: bool, target_dir: str = "") -> None:
    """Report which relevant tools are installed. If NONE of the actual
    analysis tools (everything except pure recon) are present, refuse to run
    unless the caller explicitly passed --skip-preflight."""
    rel = relevant_tools(profile, files)
    present = [t for t in rel if tool_available(t)]
    missing = [t for t in rel if t not in present]
    # Only EXTERNAL analysers count towards "this machine can review something".
    # Our own heuristics.py entries are always available, so counting them
    # would let a machine with no third-party tools installed pass the gate --
    # exactly the false negative this gate exists to prevent.
    analysis_present = [t for t in present if t.category != "recon" and t.check_cmd]
    langs_for_doctor = languages_of(files)

    print(f"Preflight: {len(present)}/{len(rel)} relevant tools installed"
          f" ({len(analysis_present)} analysis tool(s) ready).")
    if missing:
        print("  Not installed: " + ", ".join(sorted(t.id for t in missing)))

    if not analysis_present:
        # Hand off to the doctor rather than repeating a shorter, worse version
        # of what it says: it knows the per-language breakdown, the download
        # size and whether there's even disk space for the fix.
        print("\n" + "!" * 70)
        print("PREFLIGHT FAILED: no analyser for this project is installed.")
        print("Running now would inspect NOTHING, and an empty result must NOT be")
        print("read as 'no problems found'. Full breakdown follows.")
        print("!" * 70)
        doctor.report(target_dir, langs_for_doctor)
        print("  Override (NOT recommended): pass --skip-preflight.\n")
        if not skip_gate:
            sys.exit(3)


def main():
    args = parse_args()
    agent_mode = args.audience == "agent"

    # In agent mode stdout must carry NOTHING but the final JSON document, so
    # every progress line goes to stderr. Swapping the stream once here beats
    # threading a `file=` argument through every print in the file, and it also
    # catches prints from helpers we don't own.
    real_stdout = sys.stdout
    if agent_mode:
        sys.stdout = sys.stderr

    target = os.path.abspath(args.target)
    if not os.path.exists(target):
        print(f"ERROR: {target} does not exist.")
        sys.exit(1)

    # --doctor answers "can this machine review anything, and what would it
    # cost me" and then stops. It must come before any file discovery so it
    # still works on a machine where nothing is installed.
    if args.doctor:
        langs = doctor.languages_in(target)
        summary = doctor.report(target, langs)
        sys.exit(0 if summary["ready"] else 3)
    target_is_file = os.path.isfile(target)
    target_dir = os.path.dirname(target) if target_is_file else target

    profile = args.profile if args.profile != "auto" else detect_profile(target_dir)

    print(f"\n{'='*70}")
    print(f" Code review: {target}")
    print(f" Detected profile: {profile}")
    print(f"{'='*70}\n")

    files = discover_files(target, target_is_file, args)
    if not files:
        print("No files found in scope. Nothing to do.")
        sys.exit(0)

    if len(files) > args.max_files and not args.diff and not target_is_file:
        print(f"NOTE: {len(files)} files in scope -- capping at --max-files {args.max_files} "
              f"(most recently modified first). Use --diff <ref> to review just your changes, "
              f"or pass --max-files to raise this.")
        files.sort(key=lambda f: -os.path.getmtime(f))
        files = files[: args.max_files]

    print(f"Files in scope: {len(files)}")

    # Gate: make sure the tools we're about to rely on actually exist, so an
    # empty run can never be mistaken for a clean one.
    preflight(profile, files, args.skip_preflight, target_dir)

    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    results_dir = args.results_dir or os.path.join(target_dir, ".code_review", ts)
    os.makedirs(results_dir, exist_ok=True)
    print(f"Results directory: {results_dir}\n")

    ctx = build_ctx(target_dir, results_dir, args, profile=profile)

    print("-- Stage 0: recon --")
    jobs0 = build_jobs(files, target_dir, profile, results_dir, ctx, stages=(0,))
    results0 = execute_jobs(jobs0, args.jobs, args.timeout)

    print("\n-- Stage 1-2: fast lint + standard static analysis / security --")
    jobs12 = build_jobs(files, target_dir, profile, results_dir, ctx, stages=(1, 2))
    print(f"Launching {len(jobs12)} job(s) across {args.jobs} worker(s)...")
    results12 = execute_jobs(jobs12, args.jobs, args.timeout)

    all_results = results0 + results12

    hit_files = find_escalation_hits(results12) if not args.no_stage3 else set()
    results3 = []
    if not args.no_stage3 and (args.deep or hit_files):
        print(f"\n-- Stage 3: deep-dive ({'forced by --deep' if args.deep else f'{len(hit_files)} file(s) flagged security-shaped hints'}) --")
        jobs3 = build_stage3_jobs(files, target_dir, results_dir, ctx, hit_files, args.deep)
        print(f"Launching {len(jobs3)} job(s)...")
        results3 = execute_jobs(jobs3, args.jobs, args.timeout)
    all_results += results3

    # ---- position check -------------------------------------------------
    # Pure Python, no model. Every finding either points at a line that really
    # exists (and now carries that line's source) or it doesn't ship.
    pos_report = None
    if not args.no_position_check:
        collected = [f for r in all_results for f in r.findings]
        pos_report = fnd.verify_positions(collected, root=target_dir)
        print(f"\n-- {pos_report.summary()} --")
        for f in pos_report.dropped[:10]:
            print(f"   dropped {f.tool} {f.file}:{f.line} -- "
                  f"{pos_report.reasons.get(f.fingerprint, 'unverifiable')}")
        kept = {id(f) for f in pos_report.kept}
        for r in all_results:
            r.findings = [f for f in r.findings if id(f) in kept]
            r.issue_count = len(r.findings)

    # ---- accepted findings (project baseline) ----------------------------
    accepted, baseline_problems = baseline.load(target_dir)
    suppressed_n = 0
    if accepted or baseline_problems:
        live = [f for r in all_results for f in r.findings]
        suppressed_n = baseline.apply(live, accepted)
        print(f"\n-- baseline: {len(accepted)} accepted rule(s), "
              f"{suppressed_n} finding(s) suppressed --")
        for prob in baseline_problems:
            print(f"   ! {prob}")

    # ---- diff scoping ----------------------------------------------------
    # Project-wide tools cannot be pointed at a file list: `go vet ./...`,
    # `cargo clippy`, `golangci-lint run` and `staticcheck ./...` analyse the
    # whole module by design, and for Go that is EVERY tool in the registry.
    # So `--diff` narrowed file discovery to 13 files and then reported 319
    # findings across 80 files -- 92% of them in code the caller never touched.
    # Nothing is discarded (a real bug outside the diff is still a real bug),
    # but each finding is now labelled so a consumer can lead with the changes
    # under review instead of the whole repository.
    diff_summary = None
    if args.diff:
        in_scope = {os.path.relpath(p, start=target_dir) if os.path.isabs(p) else p
                    for p in files}
        inside = outside = 0
        for r in all_results:
            for fi in r.findings:
                fi.in_diff_scope = fi.file in in_scope
                inside += fi.in_diff_scope
                outside += not fi.in_diff_scope
        diff_summary = {
            "ref": args.diff,
            "files_changed": len(files),
            "findings_in_changed_files": inside,
            "findings_elsewhere": outside,
            "note": (
                "Tools that analyse a whole module (go vet, cargo clippy, "
                "golangci-lint, staticcheck, semgrep on a directory) cannot be "
                "restricted to a file list, so they report on the entire "
                "project even in --diff mode. Findings with in_diff_scope=false "
                "are real but are NOT part of the change under review -- lead "
                "with in_diff_scope=true."
            ),
        }
        print(f"\n-- diff scope: {inside} finding(s) in the {len(files)} changed file(s), "
              f"{outside} elsewhere in the project --")

    # ---- local glue tier (optional, offline) -----------------------------
    if args.local_glue:
        print(f"\n-- local glue: {args.local_model} @ {args.local_endpoint} --")
        why = local_tier.probe(args.local_endpoint, args.local_model)
        if why:
            print(f"   unavailable, continuing without it: {why}")
        else:
            live = [f for r in all_results for f in r.findings]
            n = local_tier.phrase_findings(live, args.local_endpoint, args.local_model)
            m = local_tier.triage_findings(live, args.local_endpoint, args.local_model)
            print(f"   {n} message(s) rephrased, {m} flagged as probable noise "
                  f"(advisory only -- nothing dropped)")

    # select_manual_tools() already returns only scope="manual" entries, so the
    # old re-filter here was a no-op and the first checklist_tools comprehension
    # could never match anything -- it was immediately overwritten anyway.
    langs_in_scope = languages_of(files)
    manual_tools = select_manual_tools(profile, files)
    checklist_tools = [t for t in TOOLS if t.scope == "checklist"
                       and (set(t.languages) & langs_in_scope)]

    llm_synthesis = None
    if args.llm_api_style != "none" and args.llm_endpoint and args.llm_model:
        print(f"\n-- Sending aggregate findings to LLM ({args.llm_api_style} @ {args.llm_endpoint}) --")
        digest = build_digest(all_results, profile)
        try:
            llm_synthesis = llm_client.chat(
                SYSTEM_PROMPT, digest, endpoint=args.llm_endpoint, model=args.llm_model,
                api_style=args.llm_api_style, api_key_env=args.llm_api_key_env, timeout=180,
            )
            print("LLM synthesis received.")
        except llm_client.LLMError as e:
            print(f"LLM synthesis failed (continuing without it): {e}")

    md_path = write_reports(results_dir, target, profile, files, all_results,
                             manual_tools, checklist_tools, ctx, llm_synthesis)

    if agent_mode:
        sys.stdout = real_stdout
        emit_agent_json(results_dir, target, profile, files, all_results,
                        manual_tools, checklist_tools, ctx, pos_report,
                        diff_summary, suppressed_n, baseline_problems)
        return

    total_issues = sum(r.issue_count for r in all_results)
    print(f"\n{'='*70}")
    print(f" Done. {len(all_results)} job(s) run, {total_issues} total finding(s).")
    if manual_tools:
        print(f" {len(manual_tools)} manual step(s) need your attention -- see {md_path}")
    if checklist_tools:
        print(f" {len(checklist_tools)} manual checklist(s) included in the report.")
    print(f" Full report: {md_path}")
    print(f" Raw logs:    {results_dir}/")
    print(f"{'='*70}\n")


if __name__ == "__main__":
    main()
