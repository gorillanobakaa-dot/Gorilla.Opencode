#!/usr/bin/env python3
"""
local_tier.py
-------------
The offline tier: a very small local model doing the jobs a very small local
model can actually do.

WHAT THIS IS NOT
----------------
It is not a reviewer. `qwen2.5-coder:1.5b` and friends cannot find bugs -- ask
one to "review this diff" and you get confident invention with plausible line
numbers. Anything at this size that claims otherwise is lying to you.

So the intelligence stays where it already is: the thirty-odd analysers in the
registry, which know their languages far better than a 1.5B model does. The
local model is GLUE. Every job here is extraction or classification over text
that is already in front of it -- never a judgement about code it has to
understand.

THE ONE RULE
------------
**The local model can never remove a finding.** It annotates. A tool said
something concrete about a real line; a 1.5B model's disagreement is not
evidence against that, and "the small model didn't like it" is a terrible reason
to hide a genuine bug. So triage writes a `local_triage` note and nothing else
-- ordering and inclusion stay deterministic.

That restraint is borrowed from OpenCodeReview's own filter prompt, which tells
its model to discard a comment ONLY when it can be proven wrong, and to let
anything uncertain through because the upstream agent saw context the filter
cannot. Same principle, weaker model, so the bar is higher still: we don't even
let it discard.

JOBS IMPLEMENTED
----------------
  phrase   rewrite a tool's jargon into one plain sentence.  <- reliable at 1.5B
  triage   flag findings that look like probable noise.      <- advisory only

JOBS DELIBERATELY NOT IMPLEMENTED
---------------------------------
  route    picking which tools to run. The registry already does this
           deterministically from file extensions; a model here would add
           latency and a chance of being wrong to a problem already solved.
  locate   mapping a finding to a line. Tools emit exact lines, and
           findings.verify_positions() checks them without a model. This job
           only matters for findings produced BY a model (the agent tier), and
           that tier has a far better model available.

Usage:
    python3 code_review.py <target> --local-glue \\
        [--local-endpoint http://localhost:11434/v1/chat/completions] \\
        [--local-model qwen2.5-coder:1.5b]
"""

import re
from typing import List

import llm_client

DEFAULT_ENDPOINT = "http://localhost:11434/v1/chat/completions"
DEFAULT_MODEL = "qwen2.5-coder:1.5b"

# Only bother with findings a person would actually act on. Rephrasing 400
# "line too long" messages wastes minutes and improves nothing.
PHRASE_SEVERITIES = ("error", "warning")
MAX_PHRASE_JOBS = 60
MAX_TRIAGE_JOBS = 40


# ---------------------------------------------------------------------------
# phrase
# ---------------------------------------------------------------------------

PHRASE_SYSTEM = (
    "You rewrite one static-analysis message into one plain sentence a working "
    "developer understands. Explain the consequence, not the tool's jargon. "
    "Do not add advice, severity, file names or line numbers. Do not invent "
    "detail that is not in the input. Reply with the sentence only."
)


def _looks_like_a_sentence(s: str, original: str) -> bool:
    """Guard against the ways a small model fails: empty, runaway, chatty
    preamble, echoing the prompt, or code-fencing its answer."""
    if not s:
        return False
    if len(s) > 400 or len(s) > max(240, len(original) * 6):
        return False
    if "\n" in s.strip():
        return False
    low = s.lower()
    for bad in ("as an ai", "i cannot", "sure,", "here is", "here's",
                "rewritten:", "```", "system:", "user:"):
        if low.startswith(bad) or bad in low[:24]:
            return False
    # Must contain some letters; a model that returns "..." has failed.
    return bool(re.search(r"[a-z]{3}", low))


def phrase_findings(items: List, endpoint: str, model: str, api_style: str = "openai",
                    api_key_env=None, timeout: int = 60, log=print) -> int:
    """Attach `plain_message` to the findings worth explaining.

    Returns how many were rewritten. Never alters `message` itself -- the
    original tool wording stays available, because a rephrasing is a
    convenience and the tool's exact words are the evidence.
    """
    targets = [f for f in items if f.severity in PHRASE_SEVERITIES][:MAX_PHRASE_JOBS]
    if not targets:
        return 0
    done = 0
    for f in targets:
        prompt = (f"Tool: {f.tool}\nRule: {f.rule_id or 'n/a'}\n"
                  f"Message: {f.message}\nCode at that line: {f.snippet or 'n/a'}")
        try:
            reply = llm_client.chat(PHRASE_SYSTEM, prompt, endpoint=endpoint, model=model,
                                    api_style=api_style, api_key_env=api_key_env,
                                    timeout=timeout, max_tokens=160).strip()
        except llm_client.LLMError as e:
            log(f"  local phrase: giving up after {done} rewritten ({e})")
            break
        reply = reply.strip().strip('"').strip()
        if _looks_like_a_sentence(reply, f.message):
            setattr(f, "plain_message", reply)
            done += 1
    return done


# ---------------------------------------------------------------------------
# triage
# ---------------------------------------------------------------------------

TRIAGE_SYSTEM = (
    "You are a conservative reviewer of static-analysis findings. You are shown "
    "one finding and the single line of code it refers to.\n\n"
    "Answer with exactly one word:\n"
    "  NOISE   - only if the line clearly cannot have the described problem\n"
    "  KEEP    - anything else\n\n"
    "You are seeing ONE line with no surrounding context, so you almost never "
    "have enough information to say NOISE. When unsure, answer KEEP. A missed "
    "real bug is far worse than a noisy comment."
)


def triage_findings(items: List, endpoint: str, model: str, api_style: str = "openai",
                    api_key_env=None, timeout: int = 60, log=print) -> int:
    """Annotate probable noise. ADVISORY ONLY -- nothing is dropped or reordered.

    Returns the number marked. Findings gain `local_triage="noise"`; a consumer
    may sort on it, but the finding still ships.
    """
    targets = [f for f in items
               if f.severity in ("warning", "info") and f.snippet][:MAX_TRIAGE_JOBS]
    if not targets:
        return 0
    marked = 0
    for f in targets:
        prompt = (f"Finding: {f.message}\nRule: {f.rule_id or 'n/a'}\n"
                  f"The line it refers to:\n{f.snippet}")
        try:
            reply = llm_client.chat(TRIAGE_SYSTEM, prompt, endpoint=endpoint, model=model,
                                    api_style=api_style, api_key_env=api_key_env,
                                    timeout=timeout, max_tokens=8).strip().upper()
        except llm_client.LLMError as e:
            log(f"  local triage: giving up after {marked} marked ({e})")
            break
        # Anything that isn't an unambiguous NOISE means keep. A model that
        # rambles, hedges, or answers in another language has not made a case.
        if reply.startswith("NOISE") and "KEEP" not in reply:
            setattr(f, "local_triage", "noise")
            marked += 1
    return marked


# ---------------------------------------------------------------------------
# availability
# ---------------------------------------------------------------------------

def probe(endpoint: str, model: str, api_style: str = "openai", api_key_env=None) -> str:
    """Empty string if the model answers, else why it didn't.

    Checked before any real work so a dead ollama produces one clear line
    instead of dozens of identical connection errors.
    """
    try:
        llm_client.chat("Reply with the single word OK.", "ping",
                        endpoint=endpoint, model=model, api_style=api_style,
                        api_key_env=api_key_env, timeout=30, max_tokens=8)
        return ""
    except llm_client.LLMError as e:
        return str(e)
