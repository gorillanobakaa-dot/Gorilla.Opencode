#!/usr/bin/env python3
"""
benchmark-models.py — measure how fast an OpenAI-compatible endpoint
serves each model, so you can choose one that actually feels responsive.

It reports two numbers that together explain "why does this feel slow?":

  * first-token latency  — seconds until the FIRST piece of the answer
    arrives. This is the "staring at a blank screen" time. It includes
    the provider queuing your request and (often) cold-starting the model.

  * throughput (tok/s)   — how many streamed chunks per second arrive
    after that. This is the "letters trickling in" speed.

A model can be brilliant and still feel terrible if either number is bad.
Quality and speed are different axes; a bigger model is usually smarter
and slower.

Usage:
    export NIM_KEY=nvapi-...
    python3 benchmark-models.py                 # test the built-in list
    python3 benchmark-models.py model-a model-b # test specific model ids

No dependencies beyond the Python standard library.
"""
import sys, os, json, time, urllib.request

ENDPOINT = os.environ.get("BENCH_ENDPOINT", "https://integrate.api.nvidia.com/v1")
KEY = os.environ.get("NIM_KEY") or os.environ.get("LOCAL_ENDPOINT_API_KEY", "")
PROMPT = "Count from 1 to 20 in words, one number per line."
MAX_TOKENS = 90
TIMEOUT = 45  # seconds; a model slower than this is, for our purposes, unusable

def bench(model):
    body = json.dumps({
        "model": model,
        "messages": [{"role": "user", "content": PROMPT}],
        "max_tokens": MAX_TOKENS, "stream": True,
    }).encode()
    req = urllib.request.Request(ENDPOINT + "/chat/completions", data=body,
        headers={"Authorization": "Bearer " + KEY, "Content-Type": "application/json"})
    t0 = time.time(); first = None; chunks = 0
    try:
        r = urllib.request.urlopen(req, timeout=TIMEOUT)
        for line in r:
            s = line.decode(errors="ignore").strip()
            if s.startswith("data:") and '"content"' in s:
                if first is None:
                    first = time.time()
                chunks += 1
    except Exception as e:
        return {"model": model, "status": f"error: {type(e).__name__}", "ttft": None, "toks": None}
    total = time.time() - t0
    if first is None:
        return {"model": model, "status": "no output", "ttft": None, "toks": None}
    gen = max(total - (first - t0), 0.01)
    return {"model": model, "status": "ok",
            "ttft": round(first - t0, 2), "toks": round(chunks / gen, 1)}

MODELS = sys.argv[1:] or [
    "deepseek-ai/deepseek-v4-pro", "z-ai/glm-5.2", "minimaxai/minimax-m3",
    "deepseek-ai/deepseek-v4-flash", "nvidia/nemotron-3-ultra-550b-a55b",
    "qwen/qwen3.5-397b-a17b", "mistralai/mistral-large-3-675b-instruct-2512",
    "meta/llama-4-maverick-17b-128e-instruct", "nvidia/nemotron-3-super-120b-a12b",
    "qwen/qwen3.5-122b-a10b", "qwen/qwen3-next-80b-a3b-instruct",
    "minimaxai/minimax-m2.7", "nvidia/llama-3.3-nemotron-super-49b-v1.5",
    "openai/gpt-oss-120b", "nvidia/nemotron-3-nano-30b-a3b",
    "meta/llama-3.3-70b-instruct", "mistralai/mixtral-8x7b-instruct-v0.1",
]

if not KEY:
    print("Set NIM_KEY (or LOCAL_ENDPOINT_API_KEY) first.", file=sys.stderr); sys.exit(1)

print(f"{'model':45s} {'first token':>12s} {'tok/s':>7s}  status")
print("-" * 78)
for m in MODELS:
    r = bench(m)
    ttft = f"{r['ttft']:.2f}s" if r["ttft"] is not None else "—"
    toks = f"{r['toks']}" if r["toks"] is not None else "—"
    print(f"{m:45s} {ttft:>12s} {toks:>7s}  {r['status']}", flush=True)
