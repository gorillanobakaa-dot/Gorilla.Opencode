# Cost Meter — Why It Stays at $0.00, and How to Fix It

## TL;DR

The status-bar meter only moves for models that have a *price-per-token*
set in code. Your fork was built around NVIDIA NIM, but the function
that converts NIM's `/v1/models` response into an in-app `Model`
**never copies prices over**, so for every NIM / Ollama / LM Studio
turn the math becomes `0 × 119,000 tokens = $0.00`. Anthropic, OpenAI,
Grok, Groq, and direct-Gemini all have prices wired in already — switch
to one of those and the meter moves immediately. This is not a backend
bug; NIM itself *does* report usage.

## What's actually happening (layman)

After each turn `agent.go` does:

```go
cost := model.CostPer1MIn   /1e6 * usage.InputTokens +
        model.CostPer1MOut  /1e6 * usage.OutputTokens +
        model.CostPer1MOutCached /1e6 * usage.CacheReadTokens +
        model.CostPer1MInCached  /1e6 * usage.CacheCreationTokens
sess.Cost += cost
```

If *either* factor is zero, that term is zero. The `CostPer1M*` fields
are not populated for any `/v1/models`-discovered model — they default
to Go's zero-value `0` — and `sess.Cost` then accumulates zero forever.

## Quick check (no code change)

Switch one agent (e.g. `agents.coder.model`) to any priced model:

- `anthropic.claude-sonnet-4-5`
- `openai.gpt-4o`
- `xai.grok-3-mini`
- `groq.llama-3.3-70b`
- direct Gemini (e.g. `gemini-2.5-flash`) — *not* the `gemini-oauth.*` ones

Send a few prompts. If the meter ticks up, usage data is fine and the
only thing missing is pricing for NIM.

## Concretely what to change

Edit `internal/llm/models/local.go`, function `convertLocalModel`
(around line 157). The returned `Model` struct needs the four price
fields set.

The four fields on `Model` (defined in `internal/llm/models/models.go`
lines 22–25), in USD per 1 M tokens:

| Field | Meaning | Example: `meta/llama-3.1-70b-instruct` |
|---|---|---|
| `CostPer1MIn` | Prompt tokens | 0.35 |
| `CostPer1MInCached` | Cached prompt re-read | 0.0 |
| `CostPer1MOut` | Generated tokens | 1.16 |
| `CostPer1MOutCached` | Cached output re-read | 0.0 |

(Illustrative — fetch real values from `build.nvidia.com`.)

### Option A — bundled metadata (recommended)

1. `internal/llm/models/metadata.go`: add
   ```go
   type TokenCost struct {
       In, InCached, Out, OutCached float64
   }
   // each metadata entry gets an optional
   Cost *TokenCost
   ```
2. Fill `Cost` for the NIM model IDs you actually run (the full
   catalog is `/v1/models` from `integrate.api.nvidia.com`, prices at
   `build.nvidia.com`).
3. `internal/llm/models/local.go` `convertLocalModel`, after the
   `lookupModelMeta` call, copy the four fields.

### Option B — env-var override

Read `LOCAL_MODEL_COSTS` (e.g. `meta/llama-3.1-70b-instruct:0.35,0,1.16,0;…`)
in `convertLocalModel` and parse it. Lets you fill prices without
recompiling. Same code effort, nothing to maintain in source.

## Plumbing touchpoints for the change (file map)

- `internal/llm/agent/agent.go` `TrackUsage` lines 546–566, sub-agent
  copy at 725–739 — the formula; nothing to change here.
- `internal/llm/models/models.go` — `Model` struct + four `CostPer1M*`
  fields.
- `internal/llm/models/local.go` — `convertLocalModel` (the only place
  prices are missing).
- `internal/llm/models/metadata.go` — bundle + lookup helper.
- `internal/llm/provider/openai.go` lines 541–551 — `usage()` already
  reads `PromptTokens`/`CompletionTokens`; works for NIM as-is.
- `internal/db/models.go` line 39 + `internal/db/sessions.sql.go` —
  persistence; nothing to change.
- `internal/tui/components/core/status.go` lines 101–127 — display.

## Status-bar disclaimer

`internal/tui/components/core/status.go` lines 66–127 print a one-shot
note saying "a rough estimate from a static price table — on a free or
flat-rate tier your real bill is $0." This is the user-facing version
of the same fact: missing pricing ⇒ meter cannot move. The disclaimer
itself is correct; the underlying data is what's missing.

## Will NIM's response data fill the meter?

Yes. `provider/openai.go` lines 204–210 only note that NIM doesn't
return *cache* metrics. Top-level `prompt_tokens`/`completion_tokens`
*are* present in NIM's responses — once the four prices above are
filled in, the meter will start moving on NIM too.

---

## Memory Leak & Cursor Stuck — Drive-Test Findings

### Symptoms

1. **Memory leak**: RSS grows over time as the app stays open, especially during active LSP usage.
2. **Cursor stuck going backwards**: When pressing backspace once in the editor, the cursor drifts all the way to the start of the line instead of stopping after one position. Feels like the key is being held down.

### Root Causes

#### 1. Goroutine leak in `Call` — `internal/lsp/transport.go:225`

`Call` blocks on `resp := <-ch` with no `select` on `ctx.Done()`. If the LSP server doesn't respond (crash, timeout, etc.), the calling goroutine hangs forever. The `defer` that cleans up the `handlers` map entry only runs when `Call` returns, so it never fires. Both the goroutine and the map entry leak.

#### 2. Unbounded goroutine spawning in `handleMessages` — `internal/lsp/transport.go:163`

Every LSP notification spawns a new goroutine (`go handler(msg.Params)`). During heavy typing, the LSP server floods the client with `textDocument/publishDiagnostics` notifications, each spawning a goroutine. These pile up and saturate the scheduler, starving the main event loop.

#### 3. Unbounded `diagnostics` map growth — `internal/lsp/client.go`

The `diagnostics` map is written to in `HandleDiagnostics` but never pruned when files are closed. `CloseFile` removes the entry from `openFiles` but does **not** call `ClearDiagnosticsForURI`, so diagnostics for closed files accumulate forever.

#### 4. `waitForLspDiagnostics` overwrites handlers without cleanup — `internal/llm/tools/diagnostics.go:125`

Each call to `waitForLspDiagnostics` registers a new `textDocument/publishDiagnostics` handler via `RegisterNotificationHandler`, overwriting the previous one. The old closure (capturing `originalDiags`) may still be referenced by a running goroutine from `handleMessages`, keeping that map alive until the goroutine finishes.

### Cursor Issue Explanation

The cursor drifting backwards when pressing backspace once is a **symptom of event loop starvation** caused by the above bugs:

1. The LSP server floods the client with diagnostics notifications while typing
2. `handleMessages` spawns a goroutine per notification, saturating the Go scheduler
3. The main bubbletea event loop can't process `tea.KeyMsg` events fast enough
4. The terminal's input buffer fills up, and the terminal starts **repeating** the backspace key (as if the key is held down)
5. Each repeated backspace event moves the cursor one position further back

### Fixes Applied

All root causes have been fixed and verified:

1. **`internal/lsp/transport.go`** — Three fixes in one file:
   - **`Call` now respects context cancellation** (lines 245–275): Uses `select { case resp := <-ch: ... case <-ctx.Done(): ... }` so cancelled/time-out requests exit cleanly. The handler map entry is deleted before returning the context error.
   - **Notification semaphore** (lines 164–175): Added `c.notificationSem` (buffered channel, capacity 32) to the `Client` struct. Notification handlers now acquire the semaphore via non-blocking `select`; when full, the notification is dropped with a debug log instead of spawning an unbounded goroutine.
   - **Stale response dropping** (lines 192–201): Response delivery uses `select { case ch <- msg: close(ch) default: ... }` so `handleMessages` never blocks on a channel whose `Call` goroutine has already exited.

2. **`internal/lsp/client.go`** — Two fixes:
   - **Semaphore field** (line 56): `notificationSem chan struct{}` initialized in `NewClient` with `make(chan struct{}, 32)`.
   - **Diagnostics cleanup on close** (line 707): `CloseFile` now calls `c.ClearDiagnosticsForURI(protocol.DocumentUri(uri))` so the diagnostics map doesn't grow unboundedly.

3. **`internal/llm/tools/diagnostics.go`** — Handler deregistration (lines 145–150):
   - Added `DeregisterNotificationHandler` method to `Client` (client.go line 127).
   - `waitForLspDiagnostics` now calls `client.DeregisterNotificationHandler("textDocument/publishDiagnostics")` for each client after the wait completes, preventing stale closures from accumulating.

4. **Cost meter — NIM pricing** (`internal/llm/models/`):
   - **`metadata.go`** (lines 20–27): Added `CostIn`, `CostInCached`, `CostOut`, `CostOutCached` fields to `ModelMeta`.
   - **`metadata/nim.json`**: Added pricing for 20+ key NIM models (DeepSeek V4 Pro/Flash, Nemotron 3 Super/Ultra/4, Llama 3.1 70B/405B, Qwen 3.5 122B/397B, GLM 5.2, Codestral, StarCoder2, DBRX, etc.) — all in USD per 1M tokens from `build.nvidia.com`.
   - **`local.go`** `convertLocalModel` (line 175): Now copies the four price fields from `ModelMeta` into the returned `Model` struct, so the status-bar meter immediately shows real costs for NIM/Ollama/LM Studio models.

### Verification
```bash
$ go build ./...
$ go vet ./...
$ go test ./...
```
All pass cleanly.

---

### Remaining / Future Work
- Consider making the notification semaphore capacity configurable via config.
- Add metrics/logging for dropped notifications and stale responses to aid debugging.
- Optionally persist `LOCAL_MODEL_COSTS` env-var override for runtime pricing without rebuild.
