## v0.1.17 — 2026-07-20 — Ranked picker, /clear fix, scrolling

- **Model picker is now a ranked leaderboard.** For NVIDIA NIM it shows
  only the 30 curated, probe-verified best models, numbered 1..30 (1 =
  DeepSeek V4 Pro), dropping the ~88 dead/junk/embedding models. Ranking
  comes from the earlier one-token probe + curation. Providers without a
  curated ranking (Gemini) keep showing everything, best-coder-first.
- **/clear no longer breaks the editor.** It now routes through the same
  new-session flow as Ctrl+N (resets the page session and clears the
  sidebar) instead of only wiping the message list, which left the input
  invisible and unusable.
- **Mouse-wheel scrolling** of the conversation is enabled (you could
  already scroll with PageUp/PageDown; now the wheel works too). Selecting
  terminal text now needs Shift held down.

# Changelog — Gorilla OpenCode

The revived, MIT-licensed original OpenCode (Go), kept working with the
AI providers of 2026. Every source change carries an in-code
`// GORILLA OVERRIDE:` marker — `grep -rn "GORILLA OVERRIDE" .` is the
complete audit trail. Dual-track (plain-language + developer)
explanations live in [DOCUMENTATION.dual-track.md](DOCUMENTATION.dual-track.md).

## v0.1.18 — 2026-07-20 — Unified config dir, honest cost, "alive models" note

- **All config in one clearly-labelled folder**:
  `~/.config/gorilla-opencode/` now holds `config.json` (models/agents/
  theme), `env` (keys), and `loadout.json` — with a one-time migration
  from the old `~/.opencode.json`. No more Gorilla config scattered under
  the generic `opencode` name that other tools share.
- **Cost is marked as an estimate**: the status bar shows `~$0.03 est`
  (or `$0.00`), because the figure is computed from a static, possibly
  stale price table — it is NOT your bill. On a free tier (Gemini) or
  flat-rate key (NIM) your real cost is $0.
- **Model picker says what the list is**: for curated providers it now
  shows "N models — pinged live with 1 token, only responders kept;
  ranked 1=best", so users know the dead models were probed out.

## v0.1.20 — 2026-07-20 — Streaming render throttle (not a network issue)

- Measured: NVIDIA NIM answers in ~0.5-1.4s to first byte — the network
  was never the bottleneck. The slowness was the DISPLAY: the message
  list re-ran the Markdown renderer over the whole growing answer on
  every single streamed token (O(n^2)), so long replies crawled. Now
  intermediate deltas are throttled to ~every 80ms (final token always
  renders), which keeps streaming smooth as answers grow.
- Other latency levers (not code bugs): context size — trim it in
  /context, the env/git block is the big one — and model choice (some
  NIM models reason internally and are just slow).

## v0.1.21 — 2026-07-20 — Fix the rate-limit retry storm ("forever" cure)

- The real cause of "models take forever": on an HTTP 429 (NVIDIA NIM's
  free/eval tier has a low concurrent-request limit) the app retried with
  a runaway backoff — 2,4,8,16,32,64,128,256s over 8 attempts, so a 2s
  blip became 8+ minutes of "Retrying due to rate limit". Now capped at
  6s per retry over 5 attempts (worst case ~20s), and most transient
  429s recover on the first ~0.5s retry.
- The status message is honest now: "Provider busy (rate-limit/5xx),
  retrying 2/5 in 1.0s" — it fires on 429 and 500, not only rate limits.
- (Networking itself was fine all along — NIM answers in ~1s; measured.)

## v0.1.9 — 2026-07-20 — Loadout: real numbers, wider, proof

- The `/context` loadout shows **measured** per-turn token costs (real
  tool schemas + system prompt), not estimates — the total now matches
  reality (~10.4K default) and disabling a tool drops it by its true
  cost. Dialog widened ~2× so tradeoffs aren't truncated.
- Screenshots committed as proof (`docs/screenshots/`, gallery at
  `docs/SCREENSHOTS.md`). Tool toggles apply live; env/LSP prompt blocks
  apply on restart.

## v0.1.8 — 2026-07-20 — Prompt caching (opt-in) + honesty about NIM

- **Prompt caching for OpenAI-compatible providers**, opt-in via
  `OPENCODE_PROMPT_CACHE=1`. Sends a stable `prompt_cache_key` per
  (system prompt + model) so a session's turns route to the same cached
  prefix on endpoints that support it (OpenAI, DeepSeek's direct API).
- **Why opt-in, stated plainly:** NVIDIA NIM — the provider this fork
  targets — **rejects** `prompt_cache_key` with HTTP 400 and reports no
  cache metrics, i.e. NIM offers no prompt caching to turn on. Enabling
  it by default would break every NIM request. So it is OFF by default;
  NIM users lose nothing because there was nothing to gain. Anthropic's
  ephemeral caching is separate and always on.

## v0.1.7 — 2026-07-20 — Context loadout (total control)

- **`/context`** menu (aliases `/loadout`, `/tokens`): a transparent,
  Slackware-style view of everything sent to the model every turn and
  its approximate token cost — "~9,850 tokens just to say yo".
- Every tool and the environment/LSP prompt blocks are individually
  switchable; each row states the token cost and what you give up; ⚠
  marks options that cripple the agent. Space toggles, `r` resets to
  defaults, esc closes. Persists to
  `~/.config/gorilla-opencode/loadout.json`; applies live (the agent's
  tool set is rebuilt on the spot, no restart).

## v0.1.6 — 2026-07-20 — /clear + lighter turns

- **`/clear`** (alias `/new`): fresh session, drops accumulated context.
- Sourcegraph tool made opt-in (its ~1,000-token description no longer
  rides every turn by default). Later generalised by the v0.1.7 loadout.

## v0.1.5 — 2026-07-20 — Navigable model picker + slash commands

- **Rich model metadata**: discovered models (NVIDIA NIM's 100+) show a
  curated name plus a capability description — "DeepSeek V4 Pro — 1.6T
  MoE, 1M ctx, 80.6% SWE-bench" — from 115 bundled entries, with real
  context windows.
- **Bounded picker**: a "position/total" counter, wider (62 cols) and
  taller (14 rows).
- **Slash commands**: `/model`, `/models` open the picker; `/export`
  writes the session transcript to Markdown in the working directory.

## v0.1.4 — 2026-07-20 — Branding & model picker

- In-app branding: splash reads "Gorilla OpenCode" and links to this
  repo (Go module path kept as `opencode-ai/opencode` for provenance).
- Models ranked by coding usefulness instead of reverse-alphabetical:
  flagship coders at the top, embeddings/vision/safety at the bottom.

## v0.1.3 — 2026-07-20 — Robust desktop launch

- The `launch` wrapper replaces itself via `execve` (one process owns
  the terminal), fixing the app-grid launch. (The flash-die users hit
  was compounded by GNOME caching the pre-fix `.desktop` entry, cleared
  by reinstalling + refreshing the desktop database.)

## v0.1.2 — 2026-07-20 — Package parity

- The `.deb` desktop entry now uses the `launch` wrapper, and `launch`
  self-heals by creating the key-file template on first run — so users
  who install the package (not the self-installer) also get the fix.

## v0.1.1 — 2026-07-20 — Community-review hardening

Five defects from an independent MiniMax M3 drive-test, all fixed and
guarded by `tests/smoke.sh`:

- Desktop launches read keys from `~/.config/gorilla-opencode/env`
  (GUI apps don't inherit shell env); errors hold the window open.
- Friendly no-provider message instead of "agent coder not found".
- `SilenceUsage`: runtime errors no longer buried under the usage dump.
- `--version` reports the real release (Go ≥1.22 VCS stamping was
  overriding `-ldflags`).
- Consistent `gorilla-opencode` branding in help; FZF warning → debug.

## v0.1.0 — 2026-07-19 — "The fossil breathes"

First revival release. The archived original OpenCode built cleanly on
Go 1.26.5 after ~14 months frozen and, with these patches, ran verified
end-to-end coding tasks (wrote, executed, and reported a file) against
**NVIDIA NIM**, **Google Gemini 3**, and **local Ollama**.

- Local provider: Bearer auth for keyed OpenAI-compatible endpoints
  (new `LOCAL_ENDPOINT_API_KEY`); real key for chat instead of a
  hardcoded `"dummy"`; 32K context floor when the endpoint reports none;
  `CanReason` no longer forced (modern Ollama 400s on it).
- Gemini: `genai` SDK v1.3.0 → v1.64.0; Gemini 3 thought-signature
  round-trip (persisted); thought text filtered from chat; obsolete
  `"function"` role → `"user"`; rolling model aliases; two segfaults
  fixed (nil response in the stream retry path, nil chat from a
  swallowed `Chats.Create` error).
- Config: operator-precedence bug (reasoning effort forced onto every
  local model).
- Embedded application icons + `install`/`uninstall` self-installer;
  `.deb` packaging; checksum-verifying curl/wget installer.

---

### Provenance

Fork of `github.com/opencode-ai/opencode` (Kujtim Hoxha, MIT), archived
in 2025 when it continued as Charm's **Crush** (FSL license). Unrelated
to SST's TypeScript **opencode**, which reuses the name. This revives the
MIT original. Full assessment in the repository docs.
