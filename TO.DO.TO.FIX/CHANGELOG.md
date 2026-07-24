## Unreleased — 2026-07-24 — Ultra-dense colon-anchored system prompt optimization (all agents)

- **High-Density Base System Prompt** (`internal/llm/prompt/coder-modern.txt`):
  Replaced the 633-word (~924 token) base prompt with a colon-anchored, high-density bullet structure.
  - Reduces embedded system prompt overhead from ~924 tokens to **304 tokens (−67% net token savings)**.
  - Combines visual bullet anchors (`-`), 2-word concept keys, and simple colon delimiters (`:`) to maximize attention scannability and eliminate BPE sub-word token splits while retaining 100% of anti-loop, build discipline, honesty, safety, and persistence directives.

- **High-Density Task Sub-Agent Prompt** (`internal/llm/prompt/task.go`):
  Applied same ultra-dense format to the read-only task sub-agent that handles exploration/search queries.
  - Reduces task agent overhead from 179 tokens to **48 tokens (−73% reduction)**.
  - Eliminates ALL-CAPS emotional prompting ("IMPORTANT:", "You MUST") that research shows increases hallucinations.
  - Condenses repeated instructions (same rule stated 3 different ways) into single colon-anchored directives.

- **High-Density Summarizer Prompt** (`internal/llm/prompt/summarizer.go`):
  Applied colon-anchored format with anti-sycophancy principles from 2025-2026 research.
  - Reduces summarizer overhead from 87 tokens to **71 tokens (−18% reduction)**.
  - Structured headers (`# include`, `# format`) for better attention scannability.
  - Explicit "factual only: no interpretation or opinion" directive to reduce sycophantic summarization.

- **High-Density Title Prompt** (`internal/llm/prompt/title.go`):
  Applied colon-anchored format with explicit constraint anchors.
  - Reduces title agent overhead from 88 tokens to **64 tokens (−27% reduction)**.
  - Structured `# constraints` section for clear rule retrieval.
  - Explicit anti-meta-text directive ("no meta-text like 'Title:' or 'Summary:'") to prevent output contamination.

- **Updated Research Sources** (`system-prompts/RESEARCH-SOURCES.md`):
  Added 11 new papers from 2024-2026 covering:
  - **Infinite Agentic Loops (IALs)**: arXiv:2607.01641 (2026) — Static analysis detecting loop failures with 91.9% precision
  - **Dual-State Architecture**: arXiv:2512.20660 (2026) — Three-level recovery hierarchy preventing retry explosion
  - **Token Compression**: arXiv:2412.13171 (2024), arXiv:2505.08392 (2025), arXiv:2601.20467 (2026) — 45%+ CoT reduction with preserved accuracy
  - **Anti-Sycophancy**: arXiv:2601.02896 (2025), arXiv:2602.23971 (2026) — Reducing sycophancy from 79% to 49%
  - **Hallucination Reduction**: arXiv:2603.10047 (2026), arXiv:2604.04869 (2026) — 25-45% improvement in factual accuracy

  **Plain-language version:** We compressed all AI agent instructions using the latest 2024-2026 research on reducing hallucinations, sycophancy, and infinite loops. The main agent went from 924 to 304 tokens (67% reduction), helper agents from 179 to 48 tokens (73% reduction), and title/summarizer agents also optimized. Total per-turn savings: **~751 tokens (68% reduction)** while keeping all safety rules intact. Added 11 new research papers documenting the science behind these optimizations.


## v0.1.33 — 2026-07-23 — Satellite-grade networking + a real CI gate


- **Providers now use a satellite-hardened HTTP client** (`httpclient.go`): keeps
  one TLS connection warm and reuses it across the whole tool loop (redialing is
  expensive on a high-latency uplink), prefers HTTP/2 multiplexing, sets finite
  dial/TLS timeouts so a dead link fails fast — but has **no wall-clock timeout**,
  so a slow big-model reply over satellite isn't aborted mid-answer. Respects
  `HTTP(S)_PROXY`. Wired into the OpenAI/NIM and Anthropic paths.
- **`ResourceExhausted` / server-busy now retries with back-off** instead of
  failing the turn. A new classifier catches NIM's in-band
  "request limit reached", plus rate-limit/overloaded/503/529, and backs off
  longer (2→20s) than transport blips — self-healing on a flaky link without
  hammering a congested endpoint. Retries only before content streams.
- **New `ci` workflow**: `go build` + `go vet` + `go test ./...` on every push
  and PR — no secrets needed. This is the gate that was missing; the last two
  inherited bugs (stream leak, test panic) both hid because nothing ran the
  tests. (Existing goreleaser workflows have never run — Actions needs enabling
  in the repo settings.)

  **Plain-language version:** the app is now much tougher on a bad satellite
  connection — it keeps the line warm, never hangs up on a slow answer, and waits
  politely when the server says "too busy" instead of giving up. And a robot now
  runs all the tests on every change so these bugs can't sneak back.

## v0.1.32 — 2026-07-23 — Stop leaking streams (the NIM "ResourceExhausted" fix)

- **Provider streams are now closed after every request.** On longer agent runs,
  NVIDIA NIM would eventually reject the turn with
  `ResourceExhausted: Worker local total request limit reached (46/…)` — even
  though the same model + key work fine in official opencode. Cause: our
  streaming code (`internal/llm/provider/openai.go`, `anthropic.go`) opened an
  SSE stream each turn, drained it, and never called `Close()`. The openai-go
  SDK doesn't auto-close on drain, so over an HTTP/2 connection each stream
  stayed half-open and NIM counted it as an active request — they piled up until
  the worker's in-flight cap was hit. Official opencode routes everything through
  the AI SDK with an AbortController that always tears the stream down; we
  hand-rolled the loop and missed the cleanup. Fixed by calling `stream.Close()`
  on every exit path (success/retry/error) in both providers.

  **Plain-language version:** every AI call opens a phone line to NVIDIA. The
  official app hangs up when done; we left the line open, so on a long task the
  lines stacked up until NVIDIA's switchboard refused new calls — that was the
  error. Now we hang up after each call. (Full write-up: `Errors.in.the.code.txt`.)

## v0.1.31 — 2026-07-23 — Tidy tables, calm scrolling, and a kill switch for helper agents

- **Markdown tables render correctly again.** They were coming out tall and
  sparse — blank header row, a blank line between every row, columns stretched
  into huge empty gaps. The cause was the app's own markdown theme: the table
  style set a `"\n"` block prefix/suffix, which glamour applies to *every cell*.
  Removed it; tables are now tight, aligned, and single-spaced.
  (`internal/tui/styles/markdown.go`.)

  **Plain-language version:** the AI wasn't printing broken tables — the app was
  mangling them on the way to your screen. Tables look like tables now.

- **Scrolling back through long output no longer lags, jumps, or types gibberish
  into your prompt.** Those random `[<65;119;22M` characters were mouse
  escape-codes leaking in: a mouse *drag* (e.g. selecting text without Shift)
  flooded the app with motion events, saturating the render loop until the input
  parser fell behind and spilled half-parsed sequences into the editor. The app
  now ignores non-wheel mouse events entirely; wheel scrolling still works.
  (`internal/tui/tui.go`.)

  **Plain-language version:** scrolling up to re-read a long answer used to make
  the app stutter and dribble weird numbers into your input box. Fixed.
  (To copy the whole session, use `/export` — `Ctrl+A` only ever sees the
  current screen because the app runs on the terminal's alternate screen.)

- **New: `/tasks` — see and kill the helper agents working for you.** If the
  model spawns "helper" sub-agents, a `🦍 N helper(s) · /tasks` badge now lights
  up in the status bar and a toast tells you the moment one starts. `/tasks`
  opens a live monitor: pick a helper and kill it (`enter`/`x`), or hit `X` for
  the Nuclear Option — *"kill 'em all, their tasks, and the horse they rode in
  on."* A shared registry gives each helper a cancelable context so a kill
  actually stops it. Tested under `-race`.
  (`internal/llm/agent/subagent_registry.go`, `agent-tool.go`,
  `internal/tui/components/dialog/tasks.go`, `tui.go`, `core/status.go`,
  `cmd/root.go`.)

  **Plain-language version:** you can now always see when the AI puts other
  agents to work for you — and stop them, one at a time or all at once. This is
  different from `/context`'s Nuclear dial, which *prevents* helpers from
  starting; `/tasks` *terminates* ones already running.

## v0.1.29 — 2026-07-22 — Streaming survives slow big models (the "SSE existential crisis")

- **A dropped token stream is now retried instead of killing the whole turn.**
  The streaming path only retried HTTP 429/500 (rate-limit / server) errors —
  any transport-level failure (dropped SSE connection, unexpected EOF, reset,
  read timeout) was fatal and surfaced as `failed to process events`. Big, slow
  models (Nemotron 550B, Yi Large) are slow to their *first* token, so an idle
  proxy or a flaky mobile/4G link drops the stream before it even starts — which
  isn't a 429/500, so it was never retried. Now such drops are retried with
  backoff, but **only before any content has streamed** (a mid-answer retry
  would duplicate output). Reported by users running big NIM models on a phone.
  (`openai.go`: `shouldRetry` gains transport-error handling + a tested
  `isTransientStreamError` classifier; the non-streaming path retries too.)

  **Plain-language version:** on the huge, slow models the reply sometimes just
  died with an error — worse on a phone or a patchy connection. The app now
  quietly re-tries the connection a few times *before* the answer starts, so the
  big models get a chance to wake up instead of the app giving up on them.

## v0.1.28 — 2026-07-22 — Model picker shows the whole catalog again

- **The picker no longer hides unranked models.** The curated, probe-verified
  best models still sit at the top, numbered 1..N (1 = best for coding) — but
  the rest of the provider's catalog now follows below them instead of being
  dropped. The ranking is guidance, not a gate: if you want a smaller, older,
  or frankly worse model, it's your key and your call, and the tool won't
  decide for you. (`getModelsForProvider` now appends the unranked models,
  sorted by the coding heuristic, after the ranked ones.)
- The picker subtitle now reads honestly — "N ranked best-first; M more below
  — full catalog" — instead of implying the junk was removed.

  **Plain-language version:** the model list used to show only the 30 hand-picked
  best; the couple hundred other models your NVIDIA NIM key can reach were
  hidden. Now they all show — the good ones on top, everything else underneath.

## v0.1.27 — 2026-07-22 — The .deb now gives you an app-grid launcher

- **Installing the package now creates a desktop entry + icons**, so
  Gorilla OpenCode shows up in your application grid without the extra
  `gorilla-opencode install` step. The `.deb`/`.rpm` ship the `.desktop`
  file into `/usr/share/applications/` and the icons (128/256/512/1024 +
  scalable SVG) into the hicolor theme; dpkg's own triggers refresh the
  icon and desktop caches automatically. Packaging is now committed in
  `.goreleaser.yml` (`nfpms.contents`) + `packaging/gorilla-opencode.desktop`
  so it's reproducible, not a one-off.

  **Plain-language version:** before, if you installed the `.deb` you got a
  working command but no clickable icon in your apps menu — you had to run a
  second command to get one. Now the icon appears the moment you install the
  package. (The app is otherwise unchanged from v0.1.26.)

## v0.1.26 — 2026-07-22 — Model picker: no more half-finished descriptions

- **Every model blurb in the NIM picker now reads as a complete sentence.**
  24 of the curated descriptions were stored cut off mid-sentence with a
  trailing "…" — e.g. "Solid generalist fallback, better…" and "fine for
  quick chat-style Q&A…". It *looked* like the dialog was clipping text to
  the window width, but the ellipsis was baked into the **data**, not added
  by the renderer — so resizing the terminal or rebuilding never helped. All
  24 are now finished. Pure data fix (`internal/llm/models/metadata/nim.json`);
  no code changed, and the deliberately blunt "shit tier" model ratings are
  kept intact.

  **Plain-language version:** the little grey descriptions next to each model
  used to trail off with "…" like an unfinished thought. That missing text
  wasn't hidden off the edge of the screen — it was never in the file to
  begin with. Now every description ends properly. (Takes effect after a
  rebuild, since the model list is baked into the program when it's built.)

## v0.1.25 — 2026-07-21 — Gemini 3.6 Flash, and making it actually reachable

- **Newest Google models, verified live**: `gemini-3.6-flash` and
  `gemini-3.5-flash-lite` (both 1M context) added and probed on
  2026-07-21 via `ListModels` + a real `generateContent` call. Registered
  for both Google AI Studio (`GEMINI_API_KEY`) and Vertex AI.
- **Fixed "the new Gemini models don't work" — even with a valid key.**
  The cause was *routing*, not the key. A saved config of
  `gemini-oauth.gemini-2.5-flash` sent every request through Google's
  **Code Assist** backend (OAuth), which ignores `GEMINI_API_KEY` and
  hasn't shipped 3.6 yet (HTTP 404). The Gemini default is now
  `gemini-3.6-flash` on the standard API-key endpoint when a key is set.
- **The picker no longer offers models that will 404.** Signed in with
  Google (Code Assist), it now hides the models that backend doesn't
  serve (3.6 Flash, 3.5 Flash/Lite, 2.0 Flash) instead of listing them
  and failing on selection.
- **Removed a broken pre-call**: the experimental `Caches.Create` step
  fired before every Gemini request, adding a roundtrip and throwing
  HTTP 400 ("cached content too small") on short prompts. Gone.
- **Large files open again**: the `view` tool's max read size went from
  250 KB → 5 MB, so big JSON metadata catalogues (the ~1.6 MB model
  price/context table) can be read. The 2,000-lines-per-read cap is
  unchanged, so context stays lean.

Backend reality, probed 2026-07-21 (why routing matters): the API-key
endpoint serves 3.6 / 3.5 / 3.5-lite / 3.1-lite / 3-flash-preview; the
OAuth Code Assist endpoint serves only 3.1-lite and 3-flash-preview of the
Flash line. Pro models are billing-gated (429) on both. Verified with zero
hardcoded keys in source or binary (`git diff`); `GEMINI_API_KEY` is read
at runtime from the environment or `~/.config/gorilla-opencode/env`.

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

## v0.1.22 — 2026-07-20 — Stop the concurrent title request (root-cause of the 429s)

- Proven with a request-counting proxy: one "yo" fired TWO simultaneous
  chat requests — your message and a concurrent session-title request.
  NVIDIA NIM's free tier caps CONCURRENT requests (separate from the 40
  rpm), so the second was 429'd, triggering the retry storm. Now the
  title request waits for your message to finish first — peak concurrency
  1, not 2. Combined with v0.1.21's backoff cap, a plain "yo" no longer
  rate-limits.

## v0.1.24 — 2026-07-20 — Slim the bash tool description (~1,600 tokens saved)

- The bash tool's description carried ~1,400 tokens of git-commit and
  pull-request *ritual* — <commit_analysis> XML tags, HEREDOC templates,
  a "Generated with opencode" footer, tool-use choreography — sent on
  EVERY turn. Replaced with a compact "Git and GitHub" paragraph that
  keeps the real safety rules (never touch git config, no interactive
  -i flags, no empty commits, don't auto-push, check before committing)
  and drops the boilerplate. Bash tool: ~2,442 -> ~845 tokens.
- The agent still commits fine — git know-how is in the model's weights,
  not the prompt. A default-trimmed loadout now runs well under 5k/turn.

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
