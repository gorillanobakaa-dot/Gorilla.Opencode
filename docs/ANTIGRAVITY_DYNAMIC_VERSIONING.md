# Antigravity Dynamic Versioning & Zero-Recompile Model Upgrades

This document describes the runtime resolution mechanism for Google Antigravity requests in `gorilla-opencode`, enabling immediate client-version upgrades and newly released model support (such as Gemini 4 Hydrogen, Carbon, and Argon) without recompiling the application.

## Background

Google's Cloud Code / Antigravity backend (`daily-cloudcode-pa.googleapis.com` and `cloudcode-pa.googleapis.com`) inspects the client version header on incoming requests:
```
antigravity/<VERSION> (aidev_client; os_type=windows; arch=amd64; auth_method=consumer)
```
When a new model is announced, requests reporting older client versions are filtered out during model enumeration (`fetchAvailableModels`). In previous releases, the client version was a hardcoded Go constant (`2.24.0`), requiring a full recompile, repackaging, and reinstallation on every upstream bump.

## Dynamic Resolution (No Recompilation)

Requests now resolve their client identity dynamically at runtime on every request:

1. **`ANTIGRAVITY_USER_AGENT`**: Full HTTP header override. If set, this exact string is sent as the `User-Agent`.
2. **`ANTIGRAVITY_VERSION`**: Version number override (e.g. `2.31.2`, `2.32.0`). Formatted automatically into the standard `aidev_client` header.
3. **Default Version**: Bumped to `2.31.2` (`DefaultAntigravityVersion = "2.31.2"`).
4. **`ANTIGRAVITY_REQUEST_UA`**: Overrides the internal `userAgent` field inside the request envelope (defaults to `"antigravity"`).
5. **`ANTIGRAVITY_EXTRA_MODELS`**: Comma-separated list of model IDs to immediately register into the runtime catalogue (e.g. `gemini-4-hydrogen-high,gemini-4-hydrogen-medium`), bypassing delayed backend rollout flags.

## How to Configure

Add any desired overrides to `~/.config/gorilla-opencode/env` (loaded automatically at startup):

```bash
# Example ~/.config/gorilla-opencode/env additions:
ANTIGRAVITY_VERSION=2.31.2
ANTIGRAVITY_EXTRA_MODELS=gemini-4-hydrogen-high,gemini-4-hydrogen-medium,gemini-4-hydrogen-low,gemini-4-hydrogen-xhigh
```

Then launch `gorilla-opencode` or run:
```bash
gorilla-opencode models refresh-antigravity
```
