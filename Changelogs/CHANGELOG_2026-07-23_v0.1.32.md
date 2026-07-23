# Gorilla OpenCode Changelog — July 23, 2026 (night)

## Version v0.1.32 (Build: 2026-07-23)

### Summary

One important bug fix: **streaming connections to the model provider were never
closed**, which on NVIDIA NIM caused the dreaded
`ResourceExhausted: Worker local total request limit reached` after a long agent
session. Fixed in both the OpenAI/NIM and Anthropic streaming paths.

Full root-cause write-up (with the side-by-side comparison to upstream) is in
`../Errors.in.the.code.txt`, entry #001.

---

## Bug Fixes 🐛

### 1. Provider streams were leaked → NVIDIA NIM "ResourceExhausted (N/…)"

**Symptom:**
- On longer agent runs (never a trivial "yo"), the turn died with:
  ```
  failed to process events: received error while streaming:
  {"message":"ResourceExhausted: Worker local total request limit reached (46/3..."}
  ```
- The *same* Nemotron-3-Ultra model and the *same* NVIDIA NIM key work fine in
  official opencode 1.17.20 — so this was ours, not NIM's.

**Root cause:**
- `internal/llm/provider/openai.go` `stream()` opened a stream with
  `client.Chat.Completions.NewStreaming(...)`, drained it with `Next()`, read
  `Err()`, then **returned (success) or `continue`d (retry) without ever calling
  `openaiStream.Close()`**. The openai-go SDK does not auto-close the body on
  drain (verified: `ssestream.Stream.Close()` → `decoder.Close()`).
- NIM's endpoint is HTTP/2, so each un-closed SSE stream stays half-open and is
  counted by NIM as an **active in-flight request** on that worker. An agentic
  session opens one stream per tool-loop round (plus one per helper sub-agent),
  so they accumulate — 1, 2, … 46 — until the worker's in-flight cap is hit and
  the next request is refused with `ResourceExhausted`.
- `internal/llm/provider/anthropic.go` had the same latent leak.

**Why official opencode never hits it:**
- It uses the Vercel AI SDK (`streamText`) with an `AbortController` and tears
  every stream down on exit (`Effect.ensuring(() => abort.abort())`,
  `abort.abort()` throughout `stream.transport.ts`). The connection is always
  released, so nothing accumulates on NIM's side. They centralise stream
  lifecycle; we hand-rolled it per provider and missed the cleanup.

**Fix:**
- Call `stream.Close()` immediately after `stream.Err()`, so it runs on every
  exit path — success, retry, and error — in both providers.

**Files:** `internal/llm/provider/openai.go`, `internal/llm/provider/anthropic.go`.

**Verified:** `go build ./...`, `go vet`, and `go test ./internal/llm/provider/`
all clean.

**Human track:** every question we ask the AI opens a "phone line" to NVIDIA.
The official app hangs up when the answer's done; we left the line open. On a
long task we open a new line every step until NVIDIA's switchboard maxes out and
refuses more — that's the error. Now we hang up after each call, so it stops.
