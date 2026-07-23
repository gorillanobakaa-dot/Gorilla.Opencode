# Gorilla OpenCode Changelog — July 23, 2026

## Version v0.1.30 (Build: 2026-07-23)

### Summary

Low-bandwidth context minimization, Phase 1. Replaced the recursive 1000-file
directory dump in the system-prompt environment block with a shallow depth-1
summary, and added a one-key low-bandwidth loadout preset to `/context`. On a
large tree (`/home/gorilla`) the environment block dropped from ~10k–30k tokens
to **~76 tokens** — the dominant per-turn fixed overhead is gone. Rebuilt the
binary.

Full dual-track record (human + developer tracks):
`../TO.DO.GORILLA.OPEN.CODE/CHANGELOG_AND_DECISIONS_2026-07-23.md`.

---

## Performance ⚡

### 1. Shallow environment block (was: recursive 1000-file dump)

**Problem:**
- `getEnvironmentInfo()` in `internal/llm/prompt/coder.go` ran `NewLsTool().Run()`
  with `MaxLSFiles = 1000`, walking sub-directories recursively and inlining the
  whole tree into `<project>` on **every** turn.
- On big trees (home dir, Firefox, kernel) this alone was ~10k–30k tokens of
  fixed per-turn overhead — slow and costly on metered / satellite links.

**Fix:**
- New `projectSummary()` = depth-1 `listTopLevelBrief()` (max 25 entries, hidden
  skipped, dirs marked `/`, `+N more` tail) + `gitStatusBrief()` (branch +
  `git status --short`, max 10 lines, 2s timeout, silent on failure).
- `<project>` block renamed `<project_summary>`; header tells the model it is a
  sketch and to use ls/glob/grep for deeper paths. Removed unused `tools` import.

**Measured:**
- `/home/gorilla`: **~76 tokens** (was ~10k–30k).
- Repo root: **~166 tokens**.

**Files:** `internal/llm/prompt/coder.go`; tests `internal/llm/prompt/env_test.go`.

---

## New Features ✨

### 2. `/context` token counter now shows the money, not just tokens

**Problem:**
- The status-bar footer showed a running session cost, but the `/context`
  loadout counter — the screen whose whole job is "what every turn costs" —
  showed **tokens only, no dollars**. You couldn't see what the fixed per-turn
  overhead actually costs.

**Fix:**
- `config.LoadoutCost()` prices `LoadoutActiveTokens()` at the active coder
  model's input rate using the *same* formula the agent bills a real turn with
  (`CostPer1MIn/1e6 × tokens`), so the numbers line up.
- `/context` header line now reads e.g.
  `~1,100 tokens sent on EVERY turn … — ≈ $0.0033/turn (Grok 4.5 @ $3.00/1M in)`.
- Honest edges: free / flat-rate / OAuth tiers (`CostPer1MIn == 0`) show
  `≈ $0.00/turn (free / flat-rate tier)` — real bill, not missing data; an
  unknown/custom model with no price-table entry shows `unpriced` rather than a
  misleading `$0.00`.
- Sub-cent precision: `$0.0033` under a cent, `$0.11` above.
- The low-bandwidth (`l`) toast now reports the new $/turn too.

**Why it matters here:** at Grok-4.5 input rates the old ~38k-token overhead was
**~$0.11/turn just for context**; the minimized ~1,100 is **~$0.003/turn**. The
counter now makes that saving visible in money.

**Files:** `internal/config/loadout.go` (`LoadoutCost`),
`internal/tui/components/dialog/loadout.go` (`loadoutCostSuffix`, `formatUSD`).

### 3. One-key low-bandwidth loadout preset (`/context` → `l`)

**What's New:**
- Press `l` in the `/context` loadout dialog to switch off optional tools/blocks
  not needed for the core edit/build loop.

**Implementation:**
- `config.ApplyLowBandwidthLoadout()` turns OFF `tool.patch`, `tool.fetch`,
  `tool.diagnostics`, `tool.agent`, `tool.sourcegraph`, `prompt.lsp`; keeps
  critical `bash`/`edit`/`write`/`view` and the (now-cheap) `prompt.env`.
  Persists like any loadout change; returns the recalculated active token count.
- `internal/tui/components/dialog/loadout.go`: `l` keybinding + info toast +
  help-footer update.

**Files:** `internal/config/loadout.go`, `internal/tui/components/dialog/loadout.go`.

---

## New Features ✨ (cont.)

### 6. Pace-setter: user-adjustable request rate limit (`/context` → `−`/`+`)

**Problem:**
- The agent fired requests as fast as work demanded, with only *reactive* 429
  backoff. On free low-RPM tiers (NVIDIA NIM, "up to 40 rpm") an agentic burst
  (tool loop + title + sub-agents) slammed the ceiling → throttle/retry churn.
- The real limit is undocumented, variable, and load/time-of-day dependent, so no
  hard-coded value is correct.

**Fix (proactive pacing, user-tunable):**
- `internal/llm/provider/ratelimit.go` — dependency-free `paceRequest(ctx)` spaces
  outbound calls to `time.Minute/rpm`; `rpm<=0` = unlimited. Wired into the single
  chokepoint `baseProvider.SendMessages`/`StreamResponse`, so main-agent, title,
  summarize, and sub-agent traffic share one budget. Cap is read live → changes
  apply mid-session.
- `internal/config/ratelimit.go` — persisted `RequestsPerMinute` (default 25),
  preset ladder `40…2` + `UNLIMITED`, `StepRateLimitRPM`.
- `/context` dialog: "Pace-setter" line; `−` slower/safer, `+` faster; live toast.
- Tests: `internal/llm/provider/ratelimit_test.go`.
- Follow-up: adaptive auto-backoff (Phase 2). Sub-agent "helper-leash" dial is
  designed, not yet built.

### 7. Helper-leash: user-adjustable sub-agent spawn cap (`/context` → `[`/`]`)

**Problem:**
- No cap on how many helper agents (sub-agents) the model could spawn, and the
  tool's own description encourages fan-out. Each helper is a full nested request
  loop → on low-RPM/metered tiers an unleashed agent grinds the budget. (Spawns
  can't recurse — depth 1 — and run sequentially, so a single per-turn cap is
  sufficient; see the lesson doc Part 5.)

**Fix (user-tunable leash + Nuclear option):**
- `internal/config/subagents.go` — persisted `MaxSubAgents` (default unlimited),
  sentinels `-1` unlimited / `0` Nuclear (disabled), ladder `100…1`,
  `StepMaxSubAgents`.
- `internal/llm/agent/subagent_guard.go` — per-coder-session spawn counter, reset
  each turn; `agentTool.Run` refuses over-cap spawns as a *normal tool result* so
  the model does the work inline.
- On Nuclear, `CoderAgentTools` omits `tool.agent` entirely (its schema tokens
  vanish too).
- `/context` dialog: "Helper-leash" line; `[` fewer, `]` more.
- Tests: `internal/llm/agent/subagent_guard_test.go`.

### 8. `/context` dials: arrow-key navigation + plain-language labels

**Problem (user-reported on a JP-keyboard Sony VAIO):** the pace/helper dials
were driven by `−`/`+`/`[`/`]` — awkward or unreachable on non-US keyboard
layouts — and labels like "Helper-leash: NUCLEAR" read as insider jargon.

**Fix:**
- Both dials are now **navigable rows** in the menu: **↑↓** to pick a line, **←→**
  to change it. Arrow keys work on every layout. (Old `−/+/[/]` kept as shortcuts.)
- Descriptive relabel that names *what* each dial governs + two titled sections:
  *"🦍 GORILLA CONTROLS — tune for your connection / free tier"* (dials) and
  *"Turn features on/off"* (tools). Dials read `AI SERVER requests — pace-setter`
  and `GORILLA AGENTS/SUBAGENTS — leash`, the latter bottoming out at
  `☢ GORILLA NUCLEAR — ALL … AGENTS/SUBAGENTS DISABLED`.
- `internal/tui/components/dialog/loadout.go`: unified `selectedIdx` over dials +
  components, `adjustSelected`, compact `paceValueShort`/`leashValueShort`.

## Security 🔒

### 4. Fetch tool: SSRF guard (block internal/loopback/link-local targets)

- `blockedFetchTarget()` in `internal/llm/tools/fetch.go` now rejects loopback,
  link-local (incl. cloud metadata `169.254.169.254`), private-LAN, and
  unspecified hosts *before* the permission prompt. Public hosts unaffected.
- Known gap (documented): checks the literal host, not the post-DNS IP
  (DNS-rebinding not yet covered). Test: `internal/llm/tools/fetch_ssrf_test.go`.

### 5. Sourcegraph tool: now permission-gated like every other egress tool

- Added `permission.Service` + `permissions.Request(...)` in
  `internal/llm/tools/sourcegraph.go` `Run` (was silently POSTing queries to
  sourcegraph.com). Threaded through `NewSourcegraphTool`, `TaskAgentTools`,
  `NewAgentTool`, and the `agentTool` struct; call sites in `tools.go` /
  `calibrate.go` updated.

## Build 🔨

- `go build ./...` clean (go 1.26.5). Rebuilt `./gorilla-opencode` → `v0.1.30…`.

---

## Still Open 📋

- **Phase 2** — tool-output payload caps + scratch offload (`internal/llm/tools/`).
- **Phase 4** — history compaction / sliding window (`internal/session/`, `internal/message/`).
- **Phase 3 remainder** — compress tool JSON-schema parameter descriptions.
- Commit the working tree (currently uncommitted).
