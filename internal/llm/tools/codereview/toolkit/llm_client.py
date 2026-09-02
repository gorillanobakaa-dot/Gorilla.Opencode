#!/usr/bin/env python3
"""
llm_client.py
--------------
A vendor-agnostic LLM client using ONLY the Python standard library
(urllib, json). No SDK. No hardcoded provider. No API key required unless
YOUR chosen endpoint demands one, in which case it's read from an
environment variable you choose -- never hardcoded here.

Works with anything that speaks one of two very common wire formats:

  api_style="openai"     -> POST {endpoint}  (a full URL, e.g.
                             http://localhost:11434/v1/chat/completions
                             for Ollama, or http://localhost:8080/v1/chat/completions
                             for llama.cpp's server, or any cloud provider
                             that mimics the OpenAI chat-completions schema)
                             Body: {"model": ..., "messages": [...]}

  api_style="anthropic"  -> POST {endpoint}  (e.g. https://api.anthropic.com/v1/messages)
                             Body: {"model": ..., "max_tokens": ..., "system": ...,
                                    "messages": [...]}
                             Needs header x-api-key + anthropic-version.

  api_style="none"        -> no LLM call is made; code_review.py just skips the
                             synthesis step and hands you the raw tool output.

This file has ZERO knowledge of which company made your model. A 0.2B model
running on a Raspberry Pi and a frontier cloud model look identical to this
client -- same two function calls, same JSON in, same text out.
"""

import json
import os
import urllib.request
import urllib.error
from typing import Optional


class LLMError(RuntimeError):
    pass


def chat(system_prompt: str, user_prompt: str, *, endpoint: str, model: str,
         api_style: str = "openai", api_key_env: Optional[str] = None, timeout: int = 180,
         max_tokens: int = 4096) -> str:
    """Send one request, return the model's text reply. Raises LLMError on failure."""
    if api_style == "none" or not endpoint:
        raise LLMError("no endpoint configured (api_style=none)")

    api_key = os.environ.get(api_key_env) if api_key_env else None

    if api_style == "openai":
        body = {
            "model": model,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            "temperature": 0.2,
            "max_tokens": max_tokens,
            "stream": False,
        }
        headers = {"Content-Type": "application/json"}
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"
        text = _post(endpoint, body, headers, timeout)
        try:
            data = json.loads(text)
            return data["choices"][0]["message"]["content"]
        except (KeyError, IndexError, json.JSONDecodeError) as e:
            raise LLMError(f"unexpected response shape from {endpoint}: {e}\nraw: {text[:500]}")

    if api_style == "anthropic":
        body = {
            "model": model,
            "max_tokens": max_tokens,
            "system": system_prompt,
            "messages": [{"role": "user", "content": user_prompt}],
        }
        headers = {"Content-Type": "application/json", "anthropic-version": "2023-06-01"}
        if api_key:
            headers["x-api-key"] = api_key
        text = _post(endpoint, body, headers, timeout)
        try:
            data = json.loads(text)
            return "".join(b.get("text", "") for b in data.get("content", []) if b.get("type") == "text")
        except (KeyError, json.JSONDecodeError) as e:
            raise LLMError(f"unexpected response shape from {endpoint}: {e}\nraw: {text[:500]}")

    raise LLMError(f"unknown api_style: {api_style!r} (use 'openai', 'anthropic', or 'none')")


def _post(url: str, body: dict, headers: dict, timeout: int) -> str:
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers=headers, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.read().decode("utf-8", errors="replace")
    except urllib.error.HTTPError as e:
        raise LLMError(f"HTTP {e.code} from {url}: {e.read().decode(errors='replace')[:500]}")
    except urllib.error.URLError as e:
        raise LLMError(f"could not reach {url}: {e.reason}")


if __name__ == "__main__":
    # tiny smoke test / manual sanity check, e.g.:
    #   python3 llm_client.py http://localhost:11434/v1/chat/completions llama3 openai
    import sys
    if len(sys.argv) < 4:
        print("usage: llm_client.py <endpoint> <model> <api_style> [api_key_env]")
        sys.exit(1)
    endpoint, model, api_style = sys.argv[1], sys.argv[2], sys.argv[3]
    key_env = sys.argv[4] if len(sys.argv) > 4 else None
    try:
        reply = chat("You are a terse assistant.", "Say OK if you can read this.",
                     endpoint=endpoint, model=model, api_style=api_style, api_key_env=key_env, timeout=30)
        print("REPLY:", reply)
    except LLMError as e:
        print("ERROR:", e)
        sys.exit(1)
