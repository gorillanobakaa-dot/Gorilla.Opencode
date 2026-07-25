#!/usr/bin/env python3
"""
dual_track.py
=============
Dual-Track Generator: turns source code OR a changelog/diff into two
parallel, complete documents — one for non-technical readers, one for
developers. Neither track is a "dumbed down" summary of the other; both
are full translations of the same material for different audiences.

THIS SCRIPT NEVER MAKES A NETWORK CALL. It does not know or care what model,
company, or backend is being used to fill anything in. There is no --backend,
no --api-key, no --model, no vendor SDK, nothing to authenticate. It has
exactly two jobs:

  PREP:   read your source (code file, or changelog/diff) and write out a
          plain JSON file containing a system prompt, a user prompt, and a
          JSON Schema. That file is the complete spec of what's wanted.
  RENDER: read back a JSON file that matches that schema and turn it into
          polished Markdown, with a completeness/quality check.

Whatever is running this script — a human, Claude, GPT, Llama, Qwen 0.5B,
a shell pipeline, anything that can read a prompt and write JSON — is
responsible for the middle step: read the prep file, produce JSON matching
its "json_schema", save it where the prep file says to. This script doesn't
care how that happens or what generated it.

USAGE:
    # Source code -> two docs (auto-detects project context from git/filesystem)
    python3 dual_track.py code prep app.py --output-dir ./docs
    #   ... whatever's driving this reads docs/app_layman.prep.json and
    #   docs/app_developer.prep.json, writes the two *.filled.json files
    #   the prep files point to ...
    python3 dual_track.py code render app.py --output-dir ./docs

    # Changelog -> release notes (auto-generates from git if no input given)
    python3 dual_track.py release prep --output-dir Changelogs
    #   ... fill in Changelogs/release_layman.filled.json and
    #   Changelogs/release_dev.filled.json as instructed ...
    python3 dual_track.py release render --output-dir Changelogs

    # Or pipe your own diff in manually:
    git log v1..HEAD --stat | python3 dual_track.py release prep --stdin --output-dir Changelogs

Run `python3 dual_track.py code --help` or `release --help` for every flag.
"""

import argparse
import json
import os
import re
import subprocess
import sys
from datetime import datetime
from pathlib import Path


class DualTrackError(Exception):
    """Raised on unrecoverable errors (bad/missing files, malformed JSON that
    survives the lenient parse, etc). The CLI entry point catches this and
    exits with a message; callers importing this module as a library can
    catch it without their whole process dying."""
    pass

# ══════════════════════════════════════════════════════════════════════
#  SHARED LANGUAGE RULES (short, not a manifesto)
# ══════════════════════════════════════════════════════════════════════

BANNED_PHRASES = {
    "significant improvements": "say what improved and by how much",
    "various bug fixes": "name the bugs",
    "enhanced user experience": "describe the actual experience",
    "performance optimizations": "state what got faster and by how much",
    "we are excited to announce": "just announce it",
    "cutting-edge": "describe the actual technology",
    "seamlessly integrates": "does it work or doesn't it",
    "under the hood": "say what changed",
}

CONCRETENESS_RULE = (
    "Be concrete. Give real numbers, names, and before/after comparisons "
    "instead of vague praise. Avoid these phrases: "
    + "; ".join(f'"{p}"' for p in BANNED_PHRASES)
    + ". If information genuinely isn't available, say so explicitly instead "
    "of inventing it."
)

CLAIM_SOURCING_RULE = """You must track where every specific fact came from, so the reader can verify
you instead of just trusting you. For each concrete claim you make anywhere in this document
(a number, a name, a "before/after", a "this changed" statement), add one entry to the
"claim_sources" list with exactly three fields:
  - "claim": a short repeat of the fact, in a few words
  - "basis": literally the string "stated_in_input" if you found this fact in the text you
    were given, or literally the string "model_inference" if you are inferring, summarizing,
    or guessing it because the input didn't say it directly
  - "evidence": if basis is "stated_in_input", copy the exact short phrase from the input that
    proves it (a few words, not a whole paragraph). If basis is "model_inference", write null.

Worked example, so the format is unambiguous:
  Input text contains: "startup time dropped from 4.1s to 1.2s after removing the debug logger"
  You would write two entries:
    {"claim": "startup time went from 4.1s to 1.2s", "basis": "stated_in_input", "evidence": "startup time dropped from 4.1s to 1.2s"}
    {"claim": "the debug logger was the cause of the slowdown", "basis": "stated_in_input", "evidence": "after removing the debug logger"}
  If you then wrote "this will make the tool feel snappier for most users" - that is YOUR
  inference, not something the input said, so:
    {"claim": "most users will notice this as feeling snappier", "basis": "model_inference", "evidence": null}

Do this honestly. If you are not sure whether something was stated or inferred, mark it
"model_inference" - it is always safer to under-claim confidence than over-claim it."""

# ══════════════════════════════════════════════════════════════════════
#  MODE: CODE  (document a source file for two audiences)
# ══════════════════════════════════════════════════════════════════════

CODE_LAYMAN_SYSTEM = f"""You are writing for someone about to run this code on their own computer
who cannot read it themselves. They are trusting a stranger — you, or whoever wrote this — with
their machine, their data, and possibly their money. That is a real, asymmetric risk: a developer
can bail out the moment something looks wrong; this reader can't tell wrong from right in the first
place. Treat that seriously. This is not a "fun facts" explainer, it's the information someone
needs to make an informed decision about a real risk.

- Zero unexplained jargon; any technical word gets an immediate real-world analogy.
- Be honest, including when the honest answer is "don't run this" or "only run this if X."
  Do not manufacture reassurance you don't have grounds for.
- Explicitly cover what data this code touches or sends, what the worst realistic outcome is if
  it's malicious or buggy, and concrete, doable steps this reader can take to sanity-check it
  before trusting it (not "read the source" — they can't).
- Short paragraphs, one idea each. Warm, conversational, never condescending.
- {CONCRETENESS_RULE}
- {CLAIM_SOURCING_RULE}
- Fill in EVERY field listed in the schema below. Do not skip any field and do not leave any
  field empty. If you genuinely do not know what to put in a field, write the literal text
  "Not enough information in the source to answer this" rather than leaving it blank, inventing
  a plausible-sounding answer, or deleting the field.
Respond with ONLY valid JSON matching the schema you're given. Do not write anything before or
after the JSON object - no greeting, no explanation, no markdown code fence, just the JSON itself."""

CODE_DEV_SYSTEM = f"""You are a senior engineer writing internal documentation for people who
will maintain, fork, or audit this code. They want facts and gotchas, not hand-holding.

- Lead with WHY before HOW. Call out dead code, tech debt, and security implications explicitly.
- {CONCRETENESS_RULE}
- {CLAIM_SOURCING_RULE}
- Fill in EVERY field in the schema. If a field genuinely doesn't apply, write "N/A" with a
  one-line reason rather than omitting the field.
Respond with ONLY valid JSON matching the schema you're given. Do not write anything before or
after the JSON object - no greeting, no explanation, no markdown code fence, just the JSON itself."""

CODE_LAYMAN_SCHEMA = """{
  "title": "Plain English title",
  "big_picture": "2-3 paragraphs: what this code does in the real world",
  "key_concepts": [{"name": "TechnicalName", "plain_english": "...", "analogy": "..."}],
  "how_it_works": [{"step": 1, "title": "...", "explanation": "..."}],
  "data_and_privacy": "what data this touches, sends, or stores - or an explicit 'nothing leaves your machine'",
  "worst_case_if_something_goes_wrong": "the realistic worst outcome - financial, privacy, or otherwise - stated plainly, not catastrophized and not minimized",
  "how_to_sanity_check_before_trusting_it": ["concrete, doable step a non-coder can actually take, e.g. 'check if the download page uses https and matches the official project site', not 'read the source code'"],
  "quirks": [{"title": "...", "explanation": "what's weird or worth knowing"}],
  "real_world_impact": {"computer": "...", "browser": "...", "performance": "...", "protection": "..."},
  "risks_or_kill_switches": [{"name": "...", "what_it_does": "...", "without_it": "..."}],
  "should_you_run_this": "an honest, specific recommendation: run it / don't / only if X - not a hedge",
  "why_it_matters": "why a developer would make these choices",
  "glossary": [{"term": "...", "definition": "..."}],
  "claim_sources": [{"claim": "short repeat of a specific fact stated above", "basis": "stated_in_input or model_inference", "evidence": "exact short phrase from the input, or null if model_inference"}]
}"""

CODE_DEV_SCHEMA = """{
  "title": "Technical title",
  "module_summary": "one paragraph: role in the system, trust level",
  "architecture": {"pattern": "...", "dependencies": ["..."], "trust_boundary": "...", "attack_surface": "..."},
  "flags": [{"name": "...", "type": "bool|string|int", "default": "...", "effect": "...", "notes": "..."}],
  "kill_switches": [{"location": "func/line", "condition": "...", "effect": "...", "reversible": true, "notes": "..."}],
  "dead_code": [{"location": "...", "reason": "...", "risk": "..."}],
  "performance": {"cpu": "...", "memory": "...", "io": "...", "notes": "..."},
  "security": {"remote_execution": "...", "data_handling": "...", "attack_surface": "...", "notes": "..."},
  "how_it_works": [{"step": 1, "function": "name()", "description": "...", "side_effects": "..."}],
  "technical_debt": [{"item": "...", "severity": "low|medium|high", "recommendation": "..."}],
  "impact_if_removed": "what breaks / depends on this",
  "testing_notes": "how to test this, including dev-mode flags",
  "claim_sources": [{"claim": "short repeat of a specific fact stated above", "basis": "stated_in_input or model_inference", "evidence": "exact short phrase from the input, or null if model_inference"}]
}"""

# ══════════════════════════════════════════════════════════════════════
#  MODE: RELEASE  (changelog/diff -> release notes for two audiences)
# ══════════════════════════════════════════════════════════════════════

RELEASE_LAYMAN_SYSTEM = f"""You write release notes for an intelligent, non-technical reader who
just wants to know what changed and whether it affects them. This is a translation
of the full technical change, not a trimmed-down summary — nothing significant is omitted.

- Use before/after framing wherever a change is user-perceivable.
- Cover: why this release happened, what the user will notice, what was decided (and
deliberately NOT done) and why, privacy/data implications, install/rollback risk,
known issues, likely questions, and a short honest bottom line.
- Write in first person ("we fixed", "we removed"). Own the decisions.
- {CONCRETENESS_RULE}
- {CLAIM_SOURCING_RULE}
- Fill in EVERY field listed in the schema below. Do not skip any field. If you genuinely do
  not know what to put in a field, write the literal text "Not enough information in the input
  to answer this" rather than leaving it blank or inventing a plausible-sounding answer.
Respond with ONLY valid JSON matching the schema you're given. Do not write anything before or
after the JSON object - no greeting, no explanation, no markdown code fence, just the JSON itself."""

RELEASE_DEV_SYSTEM = f"""You write a technical release note precise enough for an engineer to
audit, reproduce, and build on without reading the diff themselves. Neutral, factual,
like a postmortem. No editorializing.

- Give exact figures (not "faster" — how much faster, measured how).
- Flag anything touching ABI/API compatibility, security, or the hot path.
- {CONCRETENESS_RULE}
- {CLAIM_SOURCING_RULE}
- Fill in EVERY field in the schema. If a field genuinely doesn't apply, write "N/A" with a
  one-line reason rather than omitting the field.
Respond with ONLY valid JSON matching the schema you're given. Do not write anything before or
after the JSON object - no greeting, no explanation, no markdown code fence, just the JSON itself."""

RELEASE_LAYMAN_SCHEMA = """{
  "title": "one-line plain-English theme for this release",
  "story": "why this version exists - what was broken/annoying/prompted it",
  "before_after": [{"area": "e.g. startup time", "before": "...", "after": "...", "who_it_affects": "everyone|some users"}],
  "decisions": [{"change": "...", "why": "...", "who_benefits": "...", "tradeoff": "if any"}],
  "deliberately_not_done": [{"item": "...", "why_not": "..."}],
  "privacy_and_security": "data/telemetry/permission changes, or explicit statement that none occurred",
  "risks_and_recovery": {"backup_needed": "...", "install_time": "...", "restart_required": "...", "rollback_steps": "..."},
  "install_steps": [{"step": 1, "instructions": "...", "command": "exact command or null", "success_looks_like": "..."}],
  "known_issues": [{"issue": "...", "affected": "...", "workaround": "...", "status": "..."}],
  "faq": [{"question": "phrased like a real user would ask", "answer": "..."}],
  "bottom_line": "4-7 sentences, prose, honest about tradeoffs",
  "claim_sources": [{"claim": "short repeat of a specific fact stated above", "basis": "stated_in_input or model_inference", "evidence": "exact short phrase from the input, or null if model_inference"}]
}"""

RELEASE_DEV_SCHEMA = """{
  "title": "technical title",
  "architecture": {"toolchain": "...", "flags": "...", "resource_deltas": "binary size / RSS / cold start, before -> after"},
  "code_changes": [{"file": "...", "change": "added|removed|modified", "old_behavior": "...", "new_behavior": "..."}],
  "subsystem_changes": [{"subsystem": "network|storage|gpu|telemetry|other", "details": "..."}],
  "test_coverage": {"added": "...", "removed": "...", "notes": "..."},
  "security_posture": "CVEs, attack surface, permission changes - or explicit 'no changes'",
  "deployment": [{"step": "fetch|verify|install|verify_active|rollback", "command": "...", "expected_output": "..."}],
  "known_issues": [{"item": "...", "severity": "low|medium|high", "deferred_to": "version or null"}],
  "claim_sources": [{"claim": "short repeat of a specific fact stated above", "basis": "stated_in_input or model_inference", "evidence": "exact short phrase from the input, or null if model_inference"}]
}"""

# ══════════════════════════════════════════════════════════════════════
#  JSON SCHEMAS (real, machine-checkable versions)
# ══════════════════════════════════════════════════════════════════════

def _arr(item_props: dict, required: list = None) -> dict:
    return {"type": "array", "items": {"type": "object", "properties": item_props,
                                        "required": required or list(item_props.keys())}}

_CLAIM_SOURCES_FIELD = _arr({"claim": {"type": "string"},
                             "basis": {"type": "string", "enum": ["stated_in_input", "model_inference"]},
                             "evidence": {"type": ["string", "null"]}},
                            required=["claim", "basis"])

CODE_LAYMAN_JSONSCHEMA = {
    "type": "object",
    "properties": {
        "title": {"type": "string"},
        "big_picture": {"type": "string"},
        "key_concepts": _arr({"name": {"type": "string"}, "plain_english": {"type": "string"},
                               "analogy": {"type": "string"}}),
        "how_it_works": _arr({"step": {"type": "integer"}, "title": {"type": "string"},
                               "explanation": {"type": "string"}}),
        "data_and_privacy": {"type": "string"},
        "worst_case_if_something_goes_wrong": {"type": "string"},
        "how_to_sanity_check_before_trusting_it": {"type": "array", "items": {"type": "string"}},
        "quirks": _arr({"title": {"type": "string"}, "explanation": {"type": "string"}}),
        "real_world_impact": {"type": "object", "properties": {
            "computer": {"type": "string"}, "browser": {"type": "string"},
            "performance": {"type": "string"}, "protection": {"type": "string"}}},
        "risks_or_kill_switches": _arr({"name": {"type": "string"}, "what_it_does": {"type": "string"},
                                         "without_it": {"type": "string"}}),
        "should_you_run_this": {"type": "string"},
        "why_it_matters": {"type": "string"},
        "glossary": _arr({"term": {"type": "string"}, "definition": {"type": "string"}}),
        "claim_sources": _CLAIM_SOURCES_FIELD,
    },
    "required": ["title", "big_picture", "data_and_privacy", "worst_case_if_something_goes_wrong",
                 "how_to_sanity_check_before_trusting_it", "should_you_run_this", "claim_sources"],
}

CODE_DEV_JSONSCHEMA = {
    "type": "object",
    "properties": {
        "title": {"type": "string"},
        "module_summary": {"type": "string"},
        "architecture": {"type": "object", "properties": {
            "pattern": {"type": "string"}, "dependencies": {"type": "array", "items": {"type": "string"}},
            "trust_boundary": {"type": "string"}, "attack_surface": {"type": "string"}}},
        "flags": _arr({"name": {"type": "string"}, "type": {"type": "string"},
                        "default": {"type": "string"}, "effect": {"type": "string"}, "notes": {"type": "string"}}),
        "kill_switches": _arr({"location": {"type": "string"}, "condition": {"type": "string"},
                                "effect": {"type": "string"}, "reversible": {"type": "boolean"},
                                "notes": {"type": "string"}}),
        "dead_code": _arr({"location": {"type": "string"}, "reason": {"type": "string"}, "risk": {"type": "string"}}),
        "performance": {"type": "object", "properties": {
            "cpu": {"type": "string"}, "memory": {"type": "string"}, "io": {"type": "string"}, "notes": {"type": "string"}}},
        "security": {"type": "object", "properties": {
            "remote_execution": {"type": "string"}, "data_handling": {"type": "string"},
            "attack_surface": {"type": "string"}, "notes": {"type": "string"}}},
        "how_it_works": _arr({"step": {"type": "integer"}, "function": {"type": "string"},
                               "description": {"type": "string"}, "side_effects": {"type": "string"}}),
        "technical_debt": _arr({"item": {"type": "string"}, "severity": {"type": "string"},
                                 "recommendation": {"type": "string"}}),
        "impact_if_removed": {"type": "string"},
        "testing_notes": {"type": "string"},
        "claim_sources": _CLAIM_SOURCES_FIELD,
    },
    "required": ["title", "module_summary", "impact_if_removed", "claim_sources"],
}

RELEASE_LAYMAN_JSONSCHEMA = {
    "type": "object",
    "properties": {
        "title": {"type": "string"},
        "story": {"type": "string"},
        "before_after": _arr({"area": {"type": "string"}, "before": {"type": "string"},
                               "after": {"type": "string"}, "who_it_affects": {"type": "string"}}),
        "decisions": _arr({"change": {"type": "string"}, "why": {"type": "string"},
                            "who_benefits": {"type": "string"}, "tradeoff": {"type": "string"}}),
        "deliberately_not_done": _arr({"item": {"type": "string"}, "why_not": {"type": "string"}}),
        "privacy_and_security": {"type": "string"},
        "risks_and_recovery": {"type": "object", "properties": {
            "backup_needed": {"type": "string"}, "install_time": {"type": "string"},
            "restart_required": {"type": "string"}, "rollback_steps": {"type": "string"}}},
        "install_steps": _arr({"step": {"type": "integer"}, "instructions": {"type": "string"},
                                "command": {"type": ["string", "null"]}, "success_looks_like": {"type": "string"}}),
        "known_issues": _arr({"issue": {"type": "string"}, "affected": {"type": "string"},
                               "workaround": {"type": "string"}, "status": {"type": "string"}}),
        "faq": _arr({"question": {"type": "string"}, "answer": {"type": "string"}}),
        "bottom_line": {"type": "string"},
        "claim_sources": _CLAIM_SOURCES_FIELD,
    },
    "required": ["title", "story", "privacy_and_security", "bottom_line", "claim_sources"],
}

RELEASE_DEV_JSONSCHEMA = {
    "type": "object",
    "properties": {
        "title": {"type": "string"},
        "architecture": {"type": "object", "properties": {
            "toolchain": {"type": "string"}, "flags": {"type": "string"}, "resource_deltas": {"type": "string"}}},
        "code_changes": _arr({"file": {"type": "string"}, "change": {"type": "string"},
                               "old_behavior": {"type": "string"}, "new_behavior": {"type": "string"}}),
        "subsystem_changes": _arr({"subsystem": {"type": "string"}, "details": {"type": "string"}}),
        "test_coverage": {"type": "object", "properties": {
            "added": {"type": "string"}, "removed": {"type": "string"}, "notes": {"type": "string"}}},
        "security_posture": {"type": "string"},
        "deployment": _arr({"step": {"type": "string"}, "command": {"type": ["string", "null"]},
                             "expected_output": {"type": "string"}}),
        "known_issues": _arr({"item": {"type": "string"}, "severity": {"type": "string"},
                               "deferred_to": {"type": ["string", "null"]}}),
        "claim_sources": _CLAIM_SOURCES_FIELD,
    },
    "required": ["title", "security_posture", "claim_sources"],
}


# ══════════════════════════════════════════════════════════════════════
#  AUTO-CONTEXT GATHERING
#
#  A 0.1B-parameter model cannot infer project structure, version history,
#  or dependencies. It will hallucinate them. These functions do the
#  detective work on the host side and feed hard facts into the prompt.
# ══════════════════════════════════════════════════════════════════════

def _shell(cmd: str, cwd: Path = None, default: str = "") -> str:
    """Run a shell command, swallow errors, return stripped text or default."""
    try:
        return subprocess.check_output(
            cmd, shell=True, cwd=cwd, text=True, stderr=subprocess.DEVNULL, timeout=15
        ).strip()
    except Exception:
        return default


def get_git_context(cwd: Path) -> dict:
    """Extract recent tags, branch, commits since last tag, and remote URL."""
    tags_raw = _shell("git tag --sort=-v:refname | head -10", cwd=cwd)
    tags = [t for t in tags_raw.splitlines() if t]
    last_tag = tags[0] if tags else ""
    branch = _shell("git branch --show-current", cwd=cwd)
    remote = _shell("git remote get-url origin", cwd=cwd)
    commits_since = ""
    if last_tag:
        commits_since = _shell(f"git log {last_tag}..HEAD --oneline | head -30", cwd=cwd)
    total_commits = _shell("git rev-list --count HEAD", cwd=cwd)
    return {
        "tags": tags,
        "last_tag": last_tag,
        "branch": branch,
        "remote_url": remote,
        "commits_since_last_tag": commits_since.splitlines(),
        "total_commits": total_commits,
    }


def get_project_metadata(cwd: Path) -> dict:
    """Sniff pyproject.toml, package.json, Cargo.toml for name/version/deps."""
    pyproject = cwd / "pyproject.toml"
    if pyproject.exists():
        text = pyproject.read_text(errors="replace")
        name = re.search(r'^name\s*=\s*"([^"]+)"', text, re.MULTILINE)
        version = re.search(r'^version\s*=\s*"([^"]+)"', text, re.MULTILINE)
        deps = re.findall(r'^\s*"?([a-zA-Z0-9_\-\[\]]+)[>=<~!;"\s]', text, re.MULTILINE)
        deps = [d for d in deps if d not in ("name", "version", "description", "authors", "license")][:15]
        return {"build_system": "python", "name": name.group(1) if name else None,
                "version": version.group(1) if version else None, "dependencies": deps}

    pkg = cwd / "package.json"
    if pkg.exists():
        try:
            data = json.loads(pkg.read_text())
            return {"build_system": "node", "name": data.get("name"),
                    "version": data.get("version"),
                    "dependencies": list(data.get("dependencies", {}).keys())[:15]}
        except Exception:
            pass

    cargo = cwd / "Cargo.toml"
    if cargo.exists():
        text = cargo.read_text(errors="replace")
        name = re.search(r'^name\s*=\s*"([^"]+)"', text, re.MULTILINE)
        version = re.search(r'^version\s*=\s*"([^"]+)"', text, re.MULTILINE)
        return {"build_system": "rust", "name": name.group(1) if name else None,
                "version": version.group(1) if version else None, "dependencies": []}

    return {"build_system": "unknown", "name": None, "version": None, "dependencies": []}


def get_code_context(source: Path) -> dict:
    """Gather file-level context: size, imports, siblings, tests."""
    code = source.read_text(errors="replace")
    lines = code.splitlines()
    imports = []
    for line in lines[:150]:
        stripped = line.strip()
        if stripped.startswith(("import ", "from ", "require(", "using ", "#include", "include ", "use ", "extern ")):
            imports.append(stripped)
    siblings = []
    if source.parent.exists():
        siblings = [f.name for f in source.parent.iterdir() if f.is_file()][:20]
    tests = []
    test_patterns = [
        source.parent / f"test_{source.name}",
        source.parent / f"{source.stem}_test{source.suffix}",
        source.parent / f"{source.stem}.test{source.suffix}",
        source.parent / f"tests/test_{source.name}",
        source.parent / f"tests/{source.stem}_test{source.suffix}",
        source.parent / f"__tests__/{source.stem}.test{source.suffix}",
    ]
    for tp in test_patterns:
        if tp.exists():
            tests.append(str(tp.relative_to(source.parent)))
    tests_dir = source.parent / "tests"
    if tests_dir.exists():
        for f in tests_dir.iterdir():
            if source.stem in f.name and f.is_file():
                tests.append(f"tests/{f.name}")
                if len(tests) >= 5:
                    break
    return {
        "line_count": len(lines),
        "language": source.suffix.lstrip(".") or "unknown",
        "imports": imports[:20],
        "sibling_files": siblings,
        "test_files": tests[:5],
    }


def build_context_block(source: Path = None, cwd: Path = None) -> str:
    """Assemble a plain-text context block for injection into prompts.
    This is the 'cheat sheet' a tiny model gets so it doesn't hallucinate."""
    parts = []
    cwd = cwd or (source.parent if source else Path.cwd())

    git = get_git_context(cwd)
    meta = get_project_metadata(cwd)

    parts.append("=" * 60)
    parts.append("PROJECT CONTEXT (auto-detected from filesystem/git)")
    parts.append("=" * 60)

    if meta.get("name"):
        parts.append(f"Project name:    {meta['name']}")
    if meta.get("version"):
        parts.append(f"Project version: {meta['version']}")
    if meta.get("build_system") != "unknown":
        parts.append(f"Build system:    {meta['build_system']}")
    if meta.get("dependencies"):
        parts.append(f"Dependencies:    {', '.join(meta['dependencies'])}")

    if git.get("last_tag"):
        parts.append(f"Last git tag:    {git['last_tag']}")
    if git.get("branch"):
        parts.append(f"Current branch:  {git['branch']}")
    if git.get("remote_url"):
        parts.append(f"Git remote:      {git['remote_url']}")
    if git.get("total_commits"):
        parts.append(f"Total commits:   {git['total_commits']}")

    if git.get("commits_since_last_tag"):
        parts.append(f"Commits since {git['last_tag']}:")
        for c in git["commits_since_last_tag"][:15]:
            parts.append(f"  - {c}")
        if len(git["commits_since_last_tag"]) > 15:
            parts.append(f"  ... and {len(git['commits_since_last_tag']) - 15} more")

    if source:
        ctx = get_code_context(source)
        parts.append("")
        parts.append("-" * 40)
        parts.append(f"FILE CONTEXT: {source.name}")
        parts.append("-" * 40)
        parts.append(f"Language:        {ctx['language']}")
        parts.append(f"Line count:      {ctx['line_count']}")
        if ctx["imports"]:
            parts.append(f"Imports (top):   {', '.join(i.split()[1].split(';')[0].split(',')[0] for i in ctx['imports'][:8])}")
        if ctx["sibling_files"]:
            parts.append(f"Sibling files:   {', '.join(ctx['sibling_files'][:10])}")
        if ctx["test_files"]:
            parts.append(f"Test files:      {', '.join(ctx['test_files'])}")

    parts.append("=" * 60)
    parts.append("END CONTEXT — the source material follows below")
    parts.append("=" * 60)
    return "\n".join(parts)

# ══════════════════════════════════════════════════════════════════════
#  READING BACK A FILLED-IN ANSWER
# ══════════════════════════════════════════════════════════════════════

def _extract_json(raw: str) -> dict:
    """Pull a JSON object out of arbitrary text, tolerating code fences
    or stray preamble/postamble text around it."""
    text = raw.strip()
    if text.startswith("```"):
        text = text.split("\n", 1)[1] if "\n" in text else text[3:]
        if text.endswith("```"):
            text = text.rsplit("```", 1)[0]
    text = text.strip()
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        start, end = text.find("{"), text.rfind("}")
        if start != -1 and end != -1 and end > start:
            return json.loads(text[start:end + 1])
        raise


def load_filled_json(path: Path) -> dict:
    if not path.exists():
        raise DualTrackError(
            f"expected a filled-in JSON file at {path} but it doesn't exist yet. "
            f"Read the matching *.prep.json file, write JSON matching its "
            f"\"json_schema\" field, and save it at this exact path first.")
    raw = path.read_text(encoding="utf-8", errors="replace")
    try:
        return _extract_json(raw)
    except json.JSONDecodeError as e:
        raise DualTrackError(
            f"{path} isn't valid JSON ({e}). It should contain nothing but a single "
            f"JSON object matching the json_schema from the matching *.prep.json file - "
            f"no markdown fence, no explanation before or after it.")


# ══════════════════════════════════════════════════════════════════════
#  MARKDOWN RENDERERS — CODE MODE
# ══════════════════════════════════════════════════════════════════════

def render_claim_sources(d: dict) -> list:
    sources = d.get("claim_sources", [])
    if not sources:
        return []
    L = ["## Where This Came From\n",
         "| Claim | Basis | Evidence from your input |",
         "|-------|-------|---------------------------|"]
    for s in sources:
        claim = str(s.get("claim", "")).replace("|", "\\|")
        basis = s.get("basis", "")
        basis_label = "📄 stated in input" if basis == "stated_in_input" else "🤖 model inference"
        evidence = s.get("evidence")
        evidence_cell = str(evidence).replace("|", "\\|") if evidence else "*(none — this is the model's own judgment)*"
        L.append(f"| {claim} | {basis_label} | {evidence_cell} |")
    L.append("")
    return L


TRANSPARENCY_FOOTER = """---
**How to check this document, not just trust it:**
This was written by an AI model reading the input above, not looked up from any external database.
"📄 stated in input" claims are the model's phrasing of something your source text actually said —
verify by finding the matching line in your own file. "🤖 model inference" claims are the model's
own judgment or synthesis, not a fact from your source — treat these as opinions, not measurements.
The model can still mislabel something or make mistakes even when it's trying to be accurate. To
check further: read the raw `.json` file this tool also writes (the unformatted answer, before any
of this rendering); or re-run this tool on the same input and see whether the specific numbers and
claims stay consistent between runs."""

def render_code_layman(d: dict, filename: str) -> str:
    now = datetime.now().strftime("%Y-%m-%d")
    L = [f"# {d.get('title', filename)} — Plain English Guide",
         f"\n> Generated {now} from `{filename}`\n", "---\n"]

    if d.get("should_you_run_this"):
        L.append("## Should You Run This?\n")
        L.append(d["should_you_run_this"] + "\n")

    if d.get("worst_case_if_something_goes_wrong"):
        L.append("## Worst Case, Honestly\n")
        L.append(d["worst_case_if_something_goes_wrong"] + "\n")

    if d.get("data_and_privacy"):
        L.append("## What Data This Touches\n")
        L.append(d["data_and_privacy"] + "\n")

    checks = d.get("how_to_sanity_check_before_trusting_it", [])
    if checks:
        L.append("## Before You Trust It, Check This\n")
        for c in checks:
            L.append(f"- {c}")
        L.append("")

    L.append("## The Big Picture\n")
    L.append(d.get("big_picture", "") + "\n")

    concepts = d.get("key_concepts", [])
    if concepts:
        L.append("## Key Concepts\n")
        L.append("| Name | What it is | Real-world comparison |")
        L.append("|------|-----------|------------------------|")
        for c in concepts:
            L.append(f"| `{c.get('name','')}` | {c.get('plain_english','')} | {c.get('analogy','')} |")
        L.append("")

    if d.get("how_it_works"):
        L.append("## How It Works\n")
        for s in d["how_it_works"]:
            L.append(f"### Step {s.get('step','')}: {s.get('title','')}\n")
            L.append(s.get("explanation", "") + "\n")

    if d.get("quirks"):
        L.append("## Worth Knowing\n")
        for q in d["quirks"]:
            L.append(f"**{q.get('title','')}** — {q.get('explanation','')}\n")

    impact = d.get("real_world_impact", {})
    if any(impact.values()):
        L.append("## What This Means For You\n")
        for label, key in [("Your computer", "computer"), ("Your browser", "browser"),
                            ("Speed & performance", "performance"), ("Your protection", "protection")]:
            if impact.get(key):
                L.append(f"**{label}:** {impact[key]}\n")

    if d.get("risks_or_kill_switches"):
        L.append("## Risks & Switches\n")
        for r in d["risks_or_kill_switches"]:
            L.append(f"**{r.get('name','')}** — {r.get('what_it_does','')} "
                      f"(without it: {r.get('without_it','')})\n")

    if d.get("why_it_matters"):
        L.append("## Why A Developer Would Do This\n")
        L.append(d["why_it_matters"] + "\n")

    if d.get("glossary"):
        L.append("## Glossary\n")
        for g in d["glossary"]:
            L.append(f"**{g.get('term','')}** — {g.get('definition','')}\n")

    L.extend(render_claim_sources(d))
    L.append(TRANSPARENCY_FOOTER)
    L.append("\n*Auto-generated for non-technical readers.*")
    return "\n".join(L)


def render_code_dev(d: dict, filename: str) -> str:
    now = datetime.now().strftime("%Y-%m-%d")
    L = [f"# {d.get('title', filename)}", f"\n> Generated {now} | Source: `{filename}`\n", "---\n",
         "## Module Summary\n", d.get("module_summary", "") + "\n"]

    arch = d.get("architecture", {})
    if arch:
        L.append("## Architecture\n")
        L.append(f"- **Pattern:** {arch.get('pattern','')}")
        L.append(f"- **Trust boundary:** {arch.get('trust_boundary','')}")
        L.append(f"- **Attack surface:** {arch.get('attack_surface','')}")
        if arch.get("dependencies"):
            L.append(f"- **Dependencies:** {', '.join(f'`{x}`' for x in arch['dependencies'])}")
        L.append("")

    if d.get("flags"):
        L.append("## Flags & Preferences\n")
        L.append("| Name | Type | Default | Effect | Notes |")
        L.append("|------|------|---------|--------|-------|")
        for f in d["flags"]:
            L.append(f"| `{f.get('name','')}` | `{f.get('type','')}` | `{f.get('default','')}` "
                      f"| {f.get('effect','')} | {f.get('notes','')} |")
        L.append("")

    if d.get("kill_switches"):
        L.append("## Kill Switches\n")
        for k in d["kill_switches"]:
            rev = "reversible" if k.get("reversible") else "**not reversible**"
            L.append(f"### `{k.get('location','')}`\n- Condition: {k.get('condition','')}\n"
                      f"- Effect: {k.get('effect','')}\n- {rev}\n- {k.get('notes','')}\n")

    if d.get("dead_code"):
        L.append("## Dead Code\n")
        for c in d["dead_code"]:
            L.append(f"- **`{c.get('location','')}`** — {c.get('reason','')} (risk: {c.get('risk','')})")
        L.append("")

    perf = d.get("performance", {})
    if any(perf.values()):
        L.append("## Performance\n")
        for k in ("cpu", "memory", "io", "notes"):
            if perf.get(k):
                L.append(f"- **{k.upper()}:** {perf[k]}")
        L.append("")

    sec = d.get("security", {})
    if any(sec.values()):
        L.append("## Security\n")
        for label, key in [("Remote execution", "remote_execution"), ("Data handling", "data_handling"),
                            ("Attack surface", "attack_surface"), ("Notes", "notes")]:
            if sec.get(key):
                L.append(f"- **{label}:** {sec[key]}")
        L.append("")

    if d.get("how_it_works"):
        L.append("## Implementation Flow\n")
        for s in d["how_it_works"]:
            L.append(f"### {s.get('step','')}. `{s.get('function','')}`\n{s.get('description','')}\n"
                      f"Side effects: {s.get('side_effects','')}\n")

    if d.get("technical_debt"):
        icon = {"low": "🟡", "medium": "🟠", "high": "🔴"}
        L.append("## Technical Debt\n")
        for t in d["technical_debt"]:
            L.append(f"{icon.get(t.get('severity','low'),'🟡')} **{t.get('severity','').upper()}** "
                      f"— {t.get('item','')} → {t.get('recommendation','')}")
        L.append("")

    L.append("## Impact If Removed\n" + d.get("impact_if_removed", "") + "\n")
    L.append("## Testing Notes\n" + d.get("testing_notes", "") + "\n")
    L.extend(render_claim_sources(d))
    L.append(TRANSPARENCY_FOOTER)
    L.append("\n*Auto-generated developer documentation.*")
    return "\n".join(L)

# ══════════════════════════════════════════════════════════════════════
#  MARKDOWN RENDERERS — RELEASE MODE
# ══════════════════════════════════════════════════════════════════════

def render_release_layman(d: dict, meta: dict) -> str:
    L = [f"# {meta.get('project','Release')} {meta.get('version','')} — {d.get('title','')}",
         f"\n**Date:** {meta.get('date', datetime.now().strftime('%Y-%m-%d'))}"]
    if meta.get("target"):
        L.append(f"**Target:** {meta['target']}")
    if meta.get("previous"):
        L.append(f"**Previous version:** {meta['previous']}")
    L.append("\n---\n## Why This Release Exists\n" + d.get("story", "") + "\n")

    if d.get("before_after"):
        L.append("## What You'll Actually Notice\n")
        for b in d["before_after"]:
            L.append(f"**{b.get('area','')}**")
            L.append(f"- BEFORE: {b.get('before','')}")
            L.append(f"- AFTER: {b.get('after','')}")
            if b.get("who_it_affects"):
                L.append(f"- Affects: {b['who_it_affects']}")
            L.append("")

    if d.get("decisions"):
        L.append("## Decisions & Why\n")
        for x in d["decisions"]:
            L.append(f"**{x.get('change','')}** — {x.get('why','')} "
                      f"(benefits: {x.get('who_benefits','')}"
                      + (f"; tradeoff: {x['tradeoff']}" if x.get("tradeoff") else "") + ")\n")

    if d.get("deliberately_not_done"):
        L.append("## Deliberately Not Done\n")
        for x in d["deliberately_not_done"]:
            L.append(f"- **{x.get('item','')}** — {x.get('why_not','')}")
        L.append("")

    L.append("## Privacy & Security\n" + d.get("privacy_and_security", "") + "\n")

    risk = d.get("risks_and_recovery", {})
    if any(risk.values()):
        L.append("## Risks, Backup & Recovery\n")
        for label, key in [("Backup needed", "backup_needed"), ("Install time", "install_time"),
                            ("Restart required", "restart_required"), ("Rollback", "rollback_steps")]:
            if risk.get(key):
                L.append(f"- **{label}:** {risk[key]}")
        L.append("")

    if d.get("install_steps"):
        L.append("## How To Install\n")
        for s in d["install_steps"]:
            L.append(f"**Step {s.get('step','')}:** {s.get('instructions','')}")
            if s.get("command"):
                L.append(f"```\n{s['command']}\n```")
            if s.get("success_looks_like"):
                L.append(f"✓ Success looks like: {s['success_looks_like']}")
            L.append("")

    if d.get("known_issues"):
        L.append("## What's Still Broken\n")
        for i in d["known_issues"]:
            L.append(f"- **{i.get('issue','')}** ({i.get('affected','')}) — "
                      f"workaround: {i.get('workaround','none')} — status: {i.get('status','')}")
        L.append("")

    if d.get("faq"):
        L.append("## Common Questions\n")
        for qa in d["faq"]:
            L.append(f"**Q: {qa.get('question','')}**\nA: {qa.get('answer','')}\n")

    L.append("## Bottom Line\n" + d.get("bottom_line", "") + "\n")
    L.extend(render_claim_sources(d))
    L.append(TRANSPARENCY_FOOTER)
    return "\n".join(L)


def render_release_dev(d: dict) -> str:
    L = [f"# {d.get('title','Technical Delta')}\n", "## Architecture & Footprint\n"]
    arch = d.get("architecture", {})
    for label, key in [("Toolchain", "toolchain"), ("Flags", "flags"), ("Resource deltas", "resource_deltas")]:
        if arch.get(key):
            L.append(f"- **{label}:** {arch[key]}")
    L.append("")

    if d.get("code_changes"):
        L.append("## Code Changes\n")
        L.append("| File | Change | Old behavior | New behavior |")
        L.append("|------|--------|--------------|--------------|")
        for c in d["code_changes"]:
            L.append(f"| `{c.get('file','')}` | {c.get('change','')} | "
                      f"{c.get('old_behavior','')} | {c.get('new_behavior','')} |")
        L.append("")

    if d.get("subsystem_changes"):
        L.append("## Subsystem Changes\n")
        for s in d["subsystem_changes"]:
            L.append(f"**{s.get('subsystem','').upper()}:** {s.get('details','')}\n")

    tc = d.get("test_coverage", {})
    if any(tc.values()):
        L.append("## Test Coverage\n")
        for k in ("added", "removed", "notes"):
            if tc.get(k):
                L.append(f"- **{k.title()}:** {tc[k]}")
        L.append("")

    L.append("## Security Posture\n" + d.get("security_posture", "") + "\n")

    if d.get("deployment"):
        L.append("## Deployment & Verification\n```bash")
        for s in d["deployment"]:
            L.append(f"# {s.get('step','')}")
            if s.get("command"):
                L.append(s["command"])
            if s.get("expected_output"):
                L.append(f"# Expected: {s['expected_output']}")
        L.append("```\n")

    if d.get("known_issues"):
        L.append("## Known Issues & Deferred Work\n")
        for i in d["known_issues"]:
            L.append(f"- [{i.get('severity','?')}] {i.get('item','')}"
                      + (f" (deferred to {i['deferred_to']})" if i.get("deferred_to") else ""))
        L.append("")

    L.extend(render_claim_sources(d))
    L.append(TRANSPARENCY_FOOTER)
    return "\n".join(L)


# ══════════════════════════════════════════════════════════════════════
#  VALIDATION
# ══════════════════════════════════════════════════════════════════════

def validate_json(data: dict, required_nonempty: list) -> dict:
    checks = {}
    for key in required_nonempty:
        val = data.get(key)
        present = bool(val) if not isinstance(val, (list, dict)) else len(val) > 0
        checks[f"has_{key}"] = (present, f"'{key}' is empty or missing")

    all_text = json.dumps(data).lower()
    found = [p for p in BANNED_PHRASES if p in all_text]
    checks["avoids_banned_phrases"] = (
        len(found) == 0,
        "found: " + ", ".join(found) if found else ""
    )
    digit_count = sum(c.isdigit() for c in all_text)
    checks["has_concrete_numbers"] = (
        digit_count > 8,
        f"only {digit_count} digits found across the document — check for vague, unquantified claims"
    )
    return checks


def print_validation(checks: dict) -> bool:
    print("\nQUALITY VALIDATION", file=sys.stderr)
    print("-" * 40, file=sys.stderr)
    all_pass = True
    for name, (passed, detail) in checks.items():
        icon = "✓" if passed else "✗"
        print(f"  {icon}  {name.replace('_', ' ')}", file=sys.stderr)
        if not passed:
            all_pass = False
            if detail:
                print(f"       → {detail}", file=sys.stderr)
    print("-" * 40, file=sys.stderr)
    print("All checks passed." if all_pass else "Some checks failed — review before publishing.",
          file=sys.stderr)
    return all_pass


# ══════════════════════════════════════════════════════════════════════
#  PREP FILE FORMAT
# ══════════════════════════════════════════════════════════════════════

def _prep_envelope(system_prompt: str, user_prompt: str, schema_hint: str,
                    jsonschema: dict, write_to: Path, then_run: str) -> dict:
    return {
        "instructions": (
            "Produce ONE JSON object matching \"json_schema\" below (\"schema_with_hints\" explains "
            "what each field means in plain language). Save ONLY that JSON object - no markdown "
            "code fence, no explanation before or after it - to the exact path in "
            "\"write_completion_to\". \"system_prompt\" sets the rules and voice to write in; "
            "\"user_prompt\" has the actual input material and a repeat of the schema."
        ),
        "system_prompt": system_prompt,
        "user_prompt": user_prompt,
        "json_schema": jsonschema,
        "schema_with_hints": schema_hint,
        "write_completion_to": str(write_to),
        "then_run": then_run,
    }


def _write_prep_file(path: Path, envelope: dict):
    path.write_text(json.dumps(envelope, indent=2), encoding="utf-8")

# ══════════════════════════════════════════════════════════════════════
#  CODE-MODE VALIDATION FIELDS
# ══════════════════════════════════════════════════════════════════════

_CODE_VALIDATION_FIELDS = {
    "layman": ["big_picture", "data_and_privacy", "worst_case_if_something_goes_wrong", "should_you_run_this"],
    "developer": ["module_summary", "architecture", "impact_if_removed"],
}


# ══════════════════════════════════════════════════════════════════════
#  MODE RUNNERS — CODE
# ══════════════════════════════════════════════════════════════════════

_CODE_TRACKS = {
    "layman": (CODE_LAYMAN_SYSTEM, CODE_LAYMAN_SCHEMA, CODE_LAYMAN_JSONSCHEMA, render_code_layman),
    "developer": (CODE_DEV_SYSTEM, CODE_DEV_SCHEMA, CODE_DEV_JSONSCHEMA, render_code_dev),
}


def _code_track_list(fmt: str) -> list:
    return list(_CODE_TRACKS) if fmt == "both" else [fmt]


def run_code_prep(args):
    source = Path(args.source_file)
    if not source.exists():
        print(f"Error: file not found: {args.source_file}", file=sys.stderr)
        sys.exit(1)
    code = source.read_text(encoding="utf-8", errors="replace")
    out_dir = Path(args.output_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    stem = source.stem

    # Auto-detect context unless disabled
    context_block = ""
    if not args.no_auto_context:
        context_block = build_context_block(source=source) + "\n\n"

    written = []
    for track in _code_track_list(args.format):
        system, schema, jsonschema, _ = _CODE_TRACKS[track]
        prep_path = out_dir / f"{stem}_{track}.prep.json"
        filled_path = out_dir / f"{stem}_{track}.filled.json"
        user_prompt = (
            f"{context_block}"
            f"Analyze this source code and return JSON matching:\n{schema}\n\n"
            f"SOURCE ({source.name}, {len(code.splitlines())} lines):\n{code}"
        )
        next_cmd = f"python3 dual_track.py code render {args.source_file} --output-dir {args.output_dir}"
        if args.no_auto_context:
            next_cmd += " --no-auto-context"
        _write_prep_file(prep_path, _prep_envelope(system, user_prompt, schema, jsonschema, filled_path, next_cmd))
        written.append((track, prep_path, filled_path))

    print(f"Wrote {len(written)} prep file(s) for `{source.name}`:\n")
    for track, prep_path, filled_path in written:
        print(f"  [{track}] {prep_path}")
    print("\nFor each one:")
    print('  1. Read the prep file\'s "system_prompt" and "user_prompt".')
    print('  2. Write a JSON object matching its "json_schema" (see "schema_with_hints" for field meanings).')
    print('  3. Save ONLY that JSON object - to the exact path in the prep file\'s "write_completion_to".')
    print(f"\nThen run:\n  python3 dual_track.py code render {args.source_file} --output-dir {args.output_dir}"
          + (f" --format {args.format}" if args.format != "both" else ""))


def run_code_render(args):
    source = Path(args.source_file)
    out_dir = Path(args.output_dir)
    stem = source.stem

    results = {}
    for track in _code_track_list(args.format):
        _, _, _, renderer = _CODE_TRACKS[track]
        filled_path = out_dir / f"{stem}_{track}.filled.json"
        data = load_filled_json(filled_path)

        if args.validate:
            print_validation(validate_json(data, _CODE_VALIDATION_FIELDS.get(track, [])))

        md = renderer(data, source.name)
        results[track] = md

        (out_dir / f"{stem}_{track}.json").write_text(json.dumps(data, indent=2), encoding="utf-8")
        (out_dir / f"{stem}_{track}.md").write_text(md, encoding="utf-8")
        print(f"  wrote {out_dir / f'{stem}_{track}.md'}", file=sys.stderr)

    wrote_a_file = False
    if args.output_layman and "layman" in results:
        Path(args.output_layman).write_text(results["layman"], encoding="utf-8")
        print(f"Written to {args.output_layman}", file=sys.stderr)
        wrote_a_file = True
    if args.output_dev and "developer" in results:
        Path(args.output_dev).write_text(results["developer"], encoding="utf-8")
        print(f"Written to {args.output_dev}", file=sys.stderr)
        wrote_a_file = True
    if args.output:
        full_md = "\n\n---\n\n".join(results[t] for t in _code_track_list(args.format) if t in results)
        Path(args.output).write_text(full_md, encoding="utf-8")
        print(f"Written to {args.output}", file=sys.stderr)
        wrote_a_file = True

    if args.json:
        print(json.dumps(results))
    elif not wrote_a_file and args.format != "both":
        track = _code_track_list(args.format)[0]
        print(results[track])


# ══════════════════════════════════════════════════════════════════════
#  MODE RUNNERS — RELEASE
# ══════════════════════════════════════════════════════════════════════

def _release_meta(args) -> dict:
    return {
        "project": args.project or "Release",
        "version": args.version or "",
        "previous": args.previous,
        "target": args.target,
        "date": args.date or datetime.now().strftime("%Y-%m-%d"),
    }


def run_release_prep(args):
    if args.stdin:
        print("Reading from stdin (Ctrl+D when done)...", file=sys.stderr)
        technical_input = sys.stdin.read().strip()
    elif args.input:
        p = Path(args.input)
        if not p.exists():
            print(f"Error: file not found: {args.input}", file=sys.stderr)
            sys.exit(1)
        technical_input = p.read_text(encoding="utf-8", errors="replace")
    else:
        # Auto-generate changelog from git if no input provided
        cwd = Path(args.output_dir).parent if args.output_dir != "." else Path.cwd()
        git_ctx = get_git_context(cwd)
        if not git_ctx.get("last_tag"):
            print("Error: no --input or --stdin given, and no git tags found to auto-generate changelog.", file=sys.stderr)
            sys.exit(1)
        tag = git_ctx["last_tag"]
        print(f"Auto-generating changelog from git: commits since {tag}...", file=sys.stderr)
        technical_input = _shell(f"git log {tag}..HEAD --stat", cwd=cwd)
        if not technical_input.strip():
            technical_input = f"No commits since {tag}. This appears to be a maintenance or documentation release."
        # Also infer version from git if not provided
        if not args.version and git_ctx.get("tags"):
            # Suggest next patch version
            parts = tag.lstrip("v").split(".")
            if len(parts) >= 2 and parts[-1].isdigit():
                parts[-1] = str(int(parts[-1]) + 1)
                args.version = "v" + ".".join(parts)

    if not technical_input.strip():
        print("Error: input is empty.", file=sys.stderr)
        sys.exit(1)

    out_dir = Path(args.output_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    meta = _release_meta(args)
    meta_block = "\n".join(f"{k.upper()}: {v}" for k, v in meta.items() if v)
    (out_dir / "release.meta.json").write_text(json.dumps(meta, indent=2), encoding="utf-8")

    # Auto-context for release mode too
    context_block = ""
    if not args.no_auto_context:
        context_block = build_context_block(cwd=out_dir if out_dir.exists() else Path.cwd()) + "\n\n"

    next_cmd = f"python3 dual_track.py release render --output-dir {args.output_dir}"

    layman_prep = out_dir / "release_layman.prep.json"
    layman_filled = out_dir / "release_layman.filled.json"
    layman_user_prompt = f"{context_block}{meta_block}\n\nCHANGE INPUT:\n{technical_input}\n\nReturn JSON matching:\n{RELEASE_LAYMAN_SCHEMA}"
    _write_prep_file(layman_prep, _prep_envelope(
        RELEASE_LAYMAN_SYSTEM, layman_user_prompt, RELEASE_LAYMAN_SCHEMA,
        RELEASE_LAYMAN_JSONSCHEMA, layman_filled, next_cmd))

    dev_prep = out_dir / "release_dev.prep.json"
    dev_filled = out_dir / "release_dev.filled.json"
    dev_user_prompt = f"{context_block}{meta_block}\n\nCHANGE INPUT:\n{technical_input}\n\nReturn JSON matching:\n{RELEASE_DEV_SCHEMA}"
    _write_prep_file(dev_prep, _prep_envelope(
        RELEASE_DEV_SYSTEM, dev_user_prompt, RELEASE_DEV_SCHEMA,
        RELEASE_DEV_JSONSCHEMA, dev_filled, next_cmd))

    print("Wrote 2 prep files:\n")
    print(f"  [layman]    {layman_prep}")
    print(f"  [developer] {dev_prep}")
    print("\nFor each one:")
    print('  1. Read the prep file\'s "system_prompt" and "user_prompt".')
    print('  2. Write a JSON object matching its "json_schema" (see "schema_with_hints" for field meanings).')
    print('  3. Save ONLY that JSON object - to the exact path in the prep file\'s "write_completion_to".')
    print(f"\nThen run:\n  {next_cmd}")


def run_release_render(args):
    out_dir = Path(args.output_dir)
    meta_path = Path(args.meta) if args.meta else out_dir / "release.meta.json"
    if meta_path.exists():
        meta = json.loads(meta_path.read_text(encoding="utf-8"))
    else:
        meta = _release_meta(args)

    layman_path = Path(args.layman_json) if args.layman_json else out_dir / "release_layman.filled.json"
    dev_path = Path(args.dev_json) if args.dev_json else out_dir / "release_dev.filled.json"
    layman_data = load_filled_json(layman_path)
    dev_data = load_filled_json(dev_path)

    layman_md = render_release_layman(layman_data, meta)
    dev_md = render_release_dev(dev_data)
    full_md = layman_md + "\n\n---\n\n" + dev_md

    if args.validate:
        print_validation(validate_json(layman_data, ["story", "before_after", "bottom_line"]))
        print_validation(validate_json(dev_data, ["code_changes", "deployment"]))

    wrote_a_file = False
    if args.output_layman:
        Path(args.output_layman).write_text(layman_md, encoding="utf-8")
        print(f"Written to {args.output_layman}", file=sys.stderr)
        wrote_a_file = True
    if args.output_dev:
        Path(args.output_dev).write_text(dev_md, encoding="utf-8")
        print(f"Written to {args.output_dev}", file=sys.stderr)
        wrote_a_file = True
    if args.output:
        Path(args.output).write_text(full_md, encoding="utf-8")
        print(f"Written to {args.output}", file=sys.stderr)
        wrote_a_file = True

    # Auto-cleanup intermediate json files unless --keep-json is passed
    if not getattr(args, "keep_json", False):
        for p in [layman_path, dev_path, meta_path]:
            if p.exists() and p.suffix == ".json":
                try:
                    p.unlink()
                except OSError:
                    pass
        # also cleanup prep files if present
        for prep_name in ["release_layman.prep.json", "release_dev.prep.json"]:
            prep_p = out_dir / prep_name
            if prep_p.exists():
                try:
                    prep_p.unlink()
                except OSError:
                    pass

    if args.json:
        print(json.dumps({"layman": layman_md, "developer": dev_md}))
    elif not wrote_a_file:
        print(full_md)


# ══════════════════════════════════════════════════════════════════════
#  CLI
# ══════════════════════════════════════════════════════════════════════

def main():
    parser = argparse.ArgumentParser(
        description="Dual-track documentation generator (layman + developer tracks). "
                     "Never makes a network call - see the module docstring (python3 "
                     "dual_track.py -h, or just open the file) for the prep/render workflow.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    mode_sub = parser.add_subparsers(dest="mode", required=True)

    code_p = mode_sub.add_parser("code", help="Document a source code file")
    code_action = code_p.add_subparsers(dest="action", required=True)

    code_prep = code_action.add_parser("prep", help="Write the prep file(s) - do this first")
    code_prep.add_argument("source_file")
    code_prep.add_argument("--output-dir", default="./docs")
    code_prep.add_argument("--format", choices=["both", "layman", "developer"], default="both")
    code_prep.add_argument("--no-auto-context", action="store_true",
                            help="Skip auto-detecting project context (git tags, dependencies, etc.)")

    code_render = code_action.add_parser("render", help="Render the filled-in JSON into Markdown")
    code_render.add_argument("source_file")
    code_render.add_argument("--output-dir", default="./docs")
    code_render.add_argument("--format", choices=["both", "layman", "developer"], default="both")
    code_render.add_argument("--validate", action="store_true",
                              help="Run quality checks on the filled JSON before rendering")
    code_render.add_argument("--output", "-o", help="Write combined markdown to this file instead of stdout")
    code_render.add_argument("--output-layman", metavar="FILE", help="Write just the layman track here")
    code_render.add_argument("--output-dev", metavar="FILE", help="Write just the developer track here")
    code_render.add_argument("--json", action="store_true",
                              help='Emit {"layman": "...", "developer": "..."} as the only thing on '
                                   "stdout - for scripts/agents to consume, instead of markdown meant "
                                   "for a human terminal.")
    code_render.add_argument("--no-auto-context", action="store_true",
                              help="(no-op in render, kept for CLI consistency)")

    rel_p = mode_sub.add_parser("release", help="Generate release notes from a changelog/diff")
    rel_action = rel_p.add_subparsers(dest="action", required=True)

    rel_prep = rel_action.add_parser("prep", help="Write the prep file(s) - do this first")
    src = rel_prep.add_mutually_exclusive_group(required=False)
    src.add_argument("--input", "-i", metavar="FILE")
    src.add_argument("--stdin", action="store_true")
    rel_prep.add_argument("--output-dir", default=".")
    rel_prep.add_argument("--project", "-p")
    rel_prep.add_argument("--version", "-v")
    rel_prep.add_argument("--previous")
    rel_prep.add_argument("--target", "-t")
    rel_prep.add_argument("--date")
    rel_prep.add_argument("--no-auto-context", action="store_true",
                           help="Skip auto-detecting project context (git tags, dependencies, etc.)")

    rel_render = rel_action.add_parser("render", help="Render the filled-in JSON into Markdown")
    rel_render.add_argument("--output-dir", default=".")
    rel_render.add_argument("--layman-json", metavar="FILE")
    rel_render.add_argument("--dev-json", metavar="FILE")
    rel_render.add_argument("--meta", metavar="FILE")
    rel_render.add_argument("--project", "-p")
    rel_render.add_argument("--version", "-v")
    rel_render.add_argument("--previous")
    rel_render.add_argument("--target", "-t")
    rel_render.add_argument("--date")
    rel_render.add_argument("--output", "-o")
    rel_render.add_argument("--output-layman", metavar="FILE")
    rel_render.add_argument("--output-dev", metavar="FILE")
    rel_render.add_argument("--json", action="store_true")
    rel_render.add_argument("--validate", action="store_true")
    rel_render.add_argument("--keep-json", action="store_true", help="Keep temporary prep/filled JSON files after rendering markdown")

    args = parser.parse_args()

    try:
        if args.mode == "code":
            (run_code_prep if args.action == "prep" else run_code_render)(args)
        else:
            (run_release_prep if args.action == "prep" else run_release_render)(args)
    except DualTrackError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
