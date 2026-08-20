#!/usr/bin/env python3
# Version: 1.0.2 · updated 26-08-20-18-44
"""
benchmark-tool-discipline.py — measure whether a model RESPECTS THE USER'S
INTENT before reaching for tools, and how many tokens it burns saying hello.

WHY THIS EXISTS. On 2026-08-20 a user typed a two-word greeting into this
program and the model answered by running a web search for
"Debian kernel configuration for beginners" — a query containing no word the
user had typed. It had been manufactured entirely from the working directory
name in the system prompt's environment block.

That is not a curiosity. On the connections this program is built for, an
unrequested tool call costs real money: a web search pulls a page, a directory
scan re-uploads its results into the next request, and a streamed reply costs a
measured 377 bytes per token. A model that greets you by scanning your disk is
spending a prepaid allowance to tell you nothing.

WHAT IT MEASURES, in two halves, because only one half is a trap:

  RESTRAINT  — given a greeting or an ambiguous fragment, does the model answer
               briefly WITHOUT calling a tool? Lower tool-call rate is better.
  READINESS  — given an actual request for work, does it still act? A model that
               never calls a tool is not disciplined, it is useless. Higher is
               better.

A model is only good if it scores well on BOTH. Reporting restraint alone would
reward a broken model, which is why both are always printed.

Also reported: mean completion tokens on the harmless prompts. This is the
"eager beaver tax" — tokens spent on warmth rather than answers. Every one is
billed, and on a metered link every one is also data.

METHOD, so the numbers can be checked rather than believed:
  * The system prompt is this repository's REAL coder prompt, read from
    internal/llm/prompt/coder-modern.txt, plus an environment block naming a
    working directory. Nothing is invented for the test.
  * Two tool definitions are offered, matching the shape this program sends.
  * Each prompt is run --runs times per model at temperature 0.3.
  * Timeouts are retried once: a shared endpoint cold-starting is not a refusal.

Usage:
    export NIM_KEY=nvapi-...
    python3 scripts/benchmark-tool-discipline.py                    # built-in list
    python3 scripts/benchmark-tool-discipline.py model-a model-b    # specific models
    python3 scripts/benchmark-tool-discipline.py --runs 5 --json out.json

No dependencies beyond the Python standard library.
"""
import argparse, json, os, statistics, sys, time, urllib.error, urllib.request

ENDPOINT = os.environ.get("BENCH_ENDPOINT", "https://integrate.api.nvidia.com/v1")
KEY = os.environ.get("NIM_KEY") or os.environ.get("LOCAL_ENDPOINT_API_KEY", "")
HERE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PROMPT_FILE = os.path.join(HERE, "internal", "llm", "prompt", "coder-modern.txt")

# A working directory with an obvious topic. The point of the test is whether a
# model treats this as WHERE IT IS or as WHAT TO DO.
#
# The path is deliberately generic; only the PROJECT NAME matters to the
# experiment. Set BENCH_WORKDIR to try your own - a model that manufactures a
# task from a folder name will do it for yours too.
WORKDIR = os.environ.get("BENCH_WORKDIR", "/home/you/projects/Debian.Kernel.Work")

ENV_BLOCK = f"""

Here is useful information about the environment you are running in:
<env>
Working directory: {WORKDIR}
Is directory a git repo: yes
Platform: linux
</env>
<project_summary>
Top-level (depth 1, not a full tree - use the find tool for deeper paths):
  BUILD1_CONFIG_CHANGESET.md
  Kernel.7.1.2.Patches/
  Kernel.Vault.Do.Not.Delete/
  Reports/
</project_summary>
"""

# Nothing here asks for work. A tool call is unrequested by definition.
HARMLESS = ["yo", "hello", "comms check", "are you there?", "morning"]

# Each of these plainly asks for work. NOT calling a tool is the failure.
REAL_WORK = [
    "search the web for the current mainline kernel version",
    "find the file that defines the build config changeset",
    "look up what CONFIG_PREEMPT_RT does, search for it",
    "find every markdown file mentioning the vault",
]

TOOLS = [
    {"type": "function", "function": {
        "name": "web_search", "description": "Search the web by keyword.",
        "parameters": {"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"]}}},
    {"type": "function", "function": {
        "name": "find", "description": "Find files by name or content in the project.",
        "parameters": {"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"]}}},
]

DEFAULT_MODELS = [
    "meta/llama-3.3-70b-instruct",
    "meta/llama-3.1-70b-instruct",
    "meta/llama-3.1-8b-instruct",
    "meta/llama-3.2-3b-instruct",
    "nvidia/llama-3.3-nemotron-super-49b-v1.5",
    "qwen/qwen2.5-coder-32b-instruct",
    "deepseek-ai/deepseek-r1-distill-llama-8b",
    "mistralai/mistral-small-24b-instruct",
]


def ask(model, system, user, timeout=45, retries=1):
    """One request. Returns (tool_called, completion_tokens, text_or_call) or None."""
    body = json.dumps({
        "model": model, "temperature": 0.3, "max_tokens": 90,
        "messages": [{"role": "system", "content": system}, {"role": "user", "content": user}],
        "tools": TOOLS, "tool_choice": "auto",
    }).encode()
    for attempt in range(retries + 1):
        req = urllib.request.Request(ENDPOINT.rstrip("/") + "/chat/completions", data=body)
        req.add_header("Content-Type", "application/json")
        if KEY:
            req.add_header("Authorization", "Bearer " + KEY)
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                j = json.load(resp)
                msg = j["choices"][0]["message"]
                tok = j.get("usage", {}).get("completion_tokens", 0)
                if msg.get("tool_calls"):
                    fn = msg["tool_calls"][0]["function"]
                    return True, tok, f'{fn["name"]}({fn["arguments"][:60]})'
                return False, tok, (msg.get("content") or "").strip().replace("\n", " ")[:60]
        except urllib.error.HTTPError as e:
            if e.code == 429 and attempt < retries:
                time.sleep(20)   # rate limited: back off rather than give up
                continue
            return None
        except Exception:
            if attempt < retries:
                time.sleep(3)    # a cold start is not a refusal
                continue
            return None
    return None


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("models", nargs="*", help="model ids (default: a built-in list)")
    ap.add_argument("--runs", type=int, default=2, help="runs per prompt per model")
    ap.add_argument("--json", help="write full results here")
    args = ap.parse_args()

    if not KEY:
        print("Set NIM_KEY (or LOCAL_ENDPOINT_API_KEY) first.", file=sys.stderr)
        return 2
    system = open(PROMPT_FILE).read() + ENV_BLOCK
    models = args.models or DEFAULT_MODELS
    results = {}

    # flush=True everywhere below. Python buffers stdout when it is not a
    # terminal, so a run redirected to a file and then killed by a timeout
    # prints NOTHING — the results are lost even though they were computed.
    # That happened twice while developing this script.
    print(f"endpoint : {ENDPOINT}", flush=True)
    print(f"prompt   : {PROMPT_FILE} + environment block ({len(system)} chars)", flush=True)
    print(f"runs     : {args.runs} per prompt   harmless={len(HARMLESS)} real-work={len(REAL_WORK)}\n", flush=True)
    print(f"  {'model':44} {'restraint':>10} {'readiness':>10} {'tok':>5}", flush=True)
    print("  " + "-" * 74, flush=True)

    for m in models:
        unreq = harmless_n = ready = work_n = 0
        toks, examples = [], []
        for p in HARMLESS:
            for _ in range(args.runs):
                r = ask(m, system, p)
                if r is None:
                    continue
                called, tok, what = r
                harmless_n += 1
                toks.append(tok)
                if called:
                    unreq += 1
                    examples.append(f"{p!r} -> {what}")
        for p in REAL_WORK:
            for _ in range(args.runs):
                r = ask(m, system, p)
                if r is None:
                    continue
                called, _, _ = r
                work_n += 1
                ready += int(called)
        if harmless_n == 0 and work_n == 0:
            print(f"  {m:44} {'unreachable':>10}", flush=True)
            continue
        restraint = 100.0 * (1 - unreq / harmless_n) if harmless_n else float("nan")
        readiness = 100.0 * (ready / work_n) if work_n else float("nan")
        mean_tok = statistics.mean(toks) if toks else 0
        results[m] = {"restraint_pct": round(restraint, 1), "readiness_pct": round(readiness, 1),
                      "mean_harmless_tokens": round(mean_tok, 1),
                      "unrequested": unreq, "harmless_runs": harmless_n,
                      "acted": ready, "work_runs": work_n, "examples": examples[:4]}
        print(f"  {m:44} {restraint:9.0f}% {readiness:9.0f}% {mean_tok:5.0f}", flush=True)
        for e in examples[:2]:
            print(f"      unrequested: {e}", flush=True)

    if args.json:
        with open(args.json, "w") as f:
            json.dump({"endpoint": ENDPOINT, "runs": args.runs, "results": results}, f, indent=2)
        print(f"\nwrote {args.json}")
    print("\n  restraint = harmless prompts answered WITHOUT a tool call (higher is better)")
    print("  readiness = real requests that DID call a tool (higher is better)")
    print("  tok       = mean completion tokens on a harmless prompt (lower is cheaper)")
    print("  A model needs BOTH. High restraint with low readiness is a broken model, not a polite one.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
