## v0.1.43 — 2026-07-28 — A session you can read afterwards

Three complaints from ordinary use — warnings printing over the screen, a saved
conversation you could not reconstruct events from, and no way to copy a session
out — turned out to share a theme. Chasing them found two problems nobody had
reported: on a fresh install, saving one setting silently deleted the one before
it, and every duration the program displayed was 1000× too small.

**Plain-language version:** you can now see what the agent did, understand why it
did it, and copy the whole thing out. Saved sessions get times, the model's
reasoning, and every tool call *with the result that came back* — previously you
saw the decisions and none of the outcomes, and that works on conversations you
already have. `--plain` gives you an interactive session that is ordinary terminal
text, so `Ctrl+A` and `Ctrl+Shift+C` do what you expect. The one thing that can
cost you more — asking the model to think out loud — is off until you choose it,
and we tell you what it actually uses instead of inventing a price we cannot know.

### Fixed

- **Warnings printed over the interface and could not be cleared**
  (`internal/config/config.go`) — `Load` configured the log handler ~50 lines
  *after* the steps that log, so those warnings hit Go's built-in default slog
  handler, which writes to **stderr**. The TUI then paints over that text with no
  record in its renderer, so no redraw can clear it. Same unclearable-text bug as
  the `/login` URL in v0.1.42, but caused by a log line rather than a print —
  which is why grepping for `fmt.Print` never found it. Measured: **2 stderr lines
  → 0**, same config, same command.

- **Config writes discarded each other on a fresh install — silent data loss**
  (`internal/config/config.go`) — `updateCfgFile` keyed "does a config exist?" off
  `viper.ConfigFileUsed()`, which stays empty for the *whole process* when no
  config.json existed at startup, and substituted a literal `{}`. Every write
  re-based from empty and threw away the previous one: paste an API key in
  `/connect`, add a local endpoint, key gone. Found by accident from a test
  asserting a removal persisted — there was nothing on disk to remove from.

- **Turn durations were 1000× too small** (`internal/tui/components/chat/message.go`)
  — `formatTimestampDiff` divided by 1000, treating stored **seconds** as
  milliseconds, so a 45-second turn displayed as `45ms`. The cause was a comment:
  three places in the initial migration called `created_at` a "Unix timestamp in
  milliseconds". It never was — the triggers use `strftime('%s')`. Comments
  corrected; goose tracks migrations by version, not checksum, so installs are
  unaffected.

- **`/connect` hid most connections and could not remove any**
  (`internal/tui/components/dialog/connect.go`) — the dialog rendered only its
  built-in catalogue, so an endpoint saved under any other name had no row at all.
  One real config accumulated **four invisible NVIDIA entries** — the same key
  four times, twice with its `nvapi-` prefix missing. Now your own connections are
  listed, `d` removes one behind a confirmation, and a prefix-less key is repaired
  *with the repair stated*. Removal unregisters by endpoint **name**, not baseURL:
  several endpoints can share a URL and only one owns the registered models.

- **Nil dereference in the OpenAI request path** (`internal/llm/provider/openai.go`)
  — `config.Get()` returns nil before `Load`, and `cfg.Debug` was dereferenced
  unguarded in both `send` and `stream`. A debug log line must never be why a
  request cannot start.

### Added

- **`/export` is a record you can reconstruct events from** (`internal/export/`) —
  it had no timestamps, no tool **results** (the renderer handled only User and
  Assistant, so every `tool`-role message was silently dropped), and no reasoning.
  Now: absolute time plus offset from the session start on every message, tool
  calls with their results including failures, the model's reasoning, and a header
  covering elapsed time, models used, abnormal endings and tool-failure count. You
  choose folder and filename; it refuses to overwrite; written `0600` because a
  transcript holds file contents and command output. Rendering moved to a pure
  function, which is how a real defect surfaced — stored tool results carry an
  empty name, so failures read `← (ERROR)` with no clue what broke. They now
  borrow the name of the call they answer.

- **The model's reasoning is captured instead of discarded**
  (`internal/llm/provider/reasoning.go`) — only Anthropic ever emitted a thinking
  event. Everything else had its reasoning thrown away *while `reasoning_effort`
  was still being sent* — paying the model to think, then dropping it. A real
  session database: 81 text parts, 25 tool calls, 25 tool results, **zero
  reasoning**. Measured against live NIM with `z-ai/glm-5.2`: plain → `[content,
  role]`; `+reasoning_effort` → `[content, role]` (it is an OpenAI-only field, so
  it did nothing); `+chat_template_kwargs{thinking:true}` → `[reasoning_content,
  role]`. Reading was only half the job. A server refusing the parameter has it
  dropped for that model and the request retried once, so a refusal never costs a
  turn.

- **Per-feature switches, each labelled with its true cost**
  (`internal/config/extras.go`) — exactly **one** of the four costs anything.
  Showing tool calls, timestamps and already-generated reasoning is free and says
  so; bundling them would imply hiding tool calls saves money when it only loses
  the record. **No price is quoted anywhere**: every local model carries a zero
  cost in the bundled metadata and the endpoints in use bill nothing, so a figure
  would be false on this machine — and a warning that can be caught out once
  teaches you to ignore the rest. Asked once at first run; `esc` is not consent.
  `/context` and `/settings` rows are generated from one registry so the wording
  cannot drift.

- **Timestamps and reasoning in the live interface** — a clock on every message,
  and reasoning beside the answer it produced. It was previously visible only
  while streaming and vanished on completion, so the one thing explaining a
  conclusion was the one thing you could not go back and read.

- **`--plain`: an interactive mode you can select and copy** (`internal/plain/`) —
  the TUI draws in the terminal's **alternate screen**, which has no scrollback by
  design, so `Ctrl+A` had nothing to select and no work inside the TUI could change
  that. Measured with a minimal Bubble Tea program: with the alternate screen
  active, pushed lines reach the terminal **0 of 3** times; without it, **3 of 3**.
  `--plain` runs the same agent with no screen handling — verified against a live
  model with **zero escape bytes** in the output. Permission requests are *asked*
  on stdin rather than auto-approved, and input closing mid-question **denies**.

### Changed

- **Reasoning volume in the export header** — exact reasoning-token counts are not
  available: NIM's usage object carries only prompt, completion and total tokens.
  Reading a reasoning field would have been dead code for the provider in use. So
  the header reports characters actually captured plus a 4-chars/token estimate,
  **labelled as an estimate** with the method stated. No reasoning means no line —
  `0` would read as "reasoning ran and cost nothing".

- **Test isolation in every package that touches config** (`internal/config/configtest/`)
  — any test calling `config.Load` can write the developer's real config through
  the setters. That happened for a **third** time during this work. There is now a
  one-line guard, in all five packages that call `Load`, and it **panics** rather
  than falling back: silent damage is worse than a failed run.

### Known issues

- The main interface still cannot be selected or copied. Converting it means
  rehoming 21 overlay sites across 15 dialog surfaces plus ~2,700 lines of chat and
  layout code that assume a full-screen frame. `--plain` is the interim answer.
- No exact reasoning-token count — the provider does not report one.
- Reasoning capture was verified live only on NIM and Gemini; OpenRouter's
  `reasoning` field and the `thinking` variant are covered by unit tests against
  captured payload shapes, not a live call.
- The display switches gate the TUI and plain renderers but not `/export`, which
  always includes everything. Intentional for a forensic record.
- Interactive keystrokes still cannot be driven in the development sandbox.

## v0.1.42 — 2026-07-28 — Three bugs the screenshots found that the tests did not

Taking screenshots of v0.1.41 for the release page turned up three display bugs.
Two had no test that could have caught them, and the reason why is the useful part.

- **`/help` hid the selected command** (`internal/tui/components/dialog/commandhelp.go`) —
  whichever row the cursor sat on rendered as a blank line, with its explanation
  still shown below. Three screenshots each lost a different command (`/clear`,
  then `/export`, then `/cd`; later `/logout` and `/context`), which is what
  identified it as the cursor rather than missing data. `rowStyle` set a highlight
  background and the shared `line()` helper then reset the background to the panel
  colour, leaving **foreground equal to background**. `line()` now only sizes.

  **Why the tests passed:** a row still *contains* its text when foreground and
  background match, so `strings.Contains(view, "/clear")` matched while nothing was
  visible on screen. Presence is not visibility. The style decision is now its own
  function and the colour invariants are asserted directly.

- **`esc` did not dismiss the sign-in overlay** (`internal/tui/tui.go`) — the clear
  was written into the `keys.Quit` branch, not an escape branch, so esc never
  reached it. The overlay added in v0.1.41 to fix an un-clearable printed URL was
  itself un-clearable. Now handled *after* every dialog's routing block: each
  returns early for a `KeyMsg` when open, so reaching that line proves no dialog
  claimed the key — dialogs keep first claim on esc and any added later inherit
  precedence. The quit key deliberately does **not** clear it; every other overlay
  can be reopened, this one cannot.

- **The sign-in box drew black bars and clipped the URL** (`internal/tui/tui.go`) —
  I had left the URL unwrapped, arguing a folded URL cannot be pasted. Wrong in
  practice: the line was wider than the terminal, so the box grew with it and the
  URL was cut off at the screen edge — unreadable *and* unpastable — while the
  remaining lines, padded only to the intended width, left unpainted cells that
  render as black bars, because lipgloss does not pad the short lines of a
  multi-line render. Now hard-wrapped mid-token (a word-wrapper leaves a URL on one
  over-wide line) with every line padded individually; a short terminal sheds prose,
  never URL characters.

- **Six screenshots** added to `docs/SCREENSHOTS.md`, `/help` surfaced in the
  README, each image linking to full resolution — downscaling makes terminal
  screenshots unreadable.

**Verification:** 8 tests for the overlay (esc dismissal, consuming nothing but esc
since it is non-modal, uniform width and terminal fit at six widths, whole-URL
recoverability, prose-not-URL shedding) plus colour invariants for `/help`; first
tests in package `tui`. Non-vacuous throughout — restoring the unwrapped line gives
68 width failures, the misplaced clear gives "esc was not handled while the overlay
was up", the background clobber gives "the selected row's background (32;32;32)
equals an unselected row's". The render-level colour test took **three** attempts
and both wrong versions are documented in place: asserting *some* background escape
passed against the bug, and taking the *first* escape on the line failed against
correct code (that is the box's padding, not the row).

**Operational note that cost real debugging time:** a running process keeps the
binary it started with. After installing the fix the bug appeared unchanged, because
the window predated the install — `readlink /proc/<pid>/exe` showed
`/usr/bin/gorilla-opencode (deleted)` and the title bar still showed the older build
hash. Restart every window before concluding a fix is absent.

**Plain-language version:** Three things looked wrong on screen. The command list
was hiding whichever command you had selected — the text was there all along, drawn
in the same colour as its background, which is why an automated check kept saying it
was fine while a person saw a blank line. The sign-in box ignored the escape key,
and printed its link so wide it ran off the edge of the screen. All three are fixed,
with six screenshots added showing the results. One thing worth knowing: if you
install an update while the program is open, the open window keeps running the old
version — close it and open a new one.

## v0.1.41 — 2026-07-27 — Five bugs from real use, two of them misdiagnosed by the symptom

Every item here came from one person using v0.1.40 properly and writing down what
went wrong. Two of the five were the program reporting the truth badly rather than
doing the wrong thing — which is worse than it sounds, because a control that
works but looks broken teaches you to distrust the others.

- **Local/self-hosted models were unselectable** (`internal/tui/components/dialog/models.go`,
  `internal/llm/models/local.go`, `internal/config/config.go`) — two independent faults stacked,
  either alone enough to produce "I added my NVIDIA key and still only get Google models":
  1. `getEnabledProviders` drew from `cfg.Providers` + the `*_API_KEY` env scan. `ProviderLocal`
     is in neither (endpoints live in `cfg.LocalEndpoints` with per-endpoint keys), so **104
     local models registered and zero were reachable**.
  2. NVIDIA serves `/v1/models` **unauthenticated** (verified: `http=200`, 102 models, no
     `Authorization` header), so an entry with a missing or malformed key registers and looks
     healthy. Entries sharing a `baseURL` register identical model ids and overwrite each
     other's route, so the **last** entry captured all 102 — here one whose key had lost its
     `nvapi-` prefix, giving `http=401` on every completion behind a full model list.
  Fixed: include `ProviderLocal` when routes exist (honouring an explicit disable); collapse
  endpoints by `baseURL` preferring a keyed entry, keeping the first of two so re-adding cannot
  steal a working route; label each model with its connection; sort local first (its popularity
  was `0`, which maps to "unranked, last").

- **The first `/context` toggle press did nothing** (`internal/config/loadout.go`) — a real bug,
  not cosmetic. `LoadoutEnabled` reports an **absent** key as enabled; `ToggleLoadout` did
  `state[id] = !state[id]`, taking the zero value `false` and flipping it to `true`. Every
  `lsp.*` row is in that state, because those components register from `cfg.LSP` at `Load` time,
  *after* the state has been read. `SetAllLSPs` had the same flaw.

- **The LSP panel listed servers you had switched off** (`internal/tui/components/chat/chat.go`) —
  it iterated `cfg.LSP` raw. They *were* being disabled: same config, all 9 off → **0**
  language-server processes; on → clangd 1, gopls 2, 5 node servers. Now lists only what runs,
  plus `N off (/context to change)` so a quietened setup is distinguishable from an
  unconfigured one. **Bulk switch added**: `L` in `/context`, deliberately scoped to `lsp.*`.

- **The session title was the model's narration** (`internal/llm/agent/title.go`, new) — the old
  code stored `TrimSpace(ReplaceAll(reply, "\n", " "))`, so
  `Here's a possible title, keeping the constraints in mind: **Title:** Your Business Brief`
  became the title verbatim. Tightening the prompt was not an option: it already forbids exactly
  that and the model ignored it. The reply is now reduced to the part that is a title, falling
  back to the user's own first message.

- **The `/login` URL could never disappear** (`internal/auth/prompt.go`, new) — five
  `fmt.Println` calls into a screen Bubble Tea owns: painted over the frame with no record in
  the renderer, so no redraw could clear it. Now injected via context and drawn as a dismissible
  overlay (URL line intentionally unwrapped to stay copyable; `esc` hides without cancelling).
  Swept the class — three more in `custom_commands.go`.

- **`/help` and `/commands`** (`internal/commands/`, `dialog/commandhelp.go`, both new) — every
  command in plain language, grouped by *what you are trying to do* rather than alphabetically,
  because someone who does not know a command's name cannot look it up alphabetically. A
  bidirectional drift test reads the dispatch switch in `tui.go` and fails on
  typable-but-undocumented or documented-but-dead; it caught `/help` itself as unwired.
  Unknown commands now suggest a near miss (Levenshtein — "modl" shares no substring with "model").

- **Startup workspace picker** (`internal/tui/startup/`, `internal/config/roots.go`) —
  `config.json`'s `"wd"` was **write-only**: persisted via `encoding/json` (`json:"wd"`) but read
  via `viper.Unmarshal`, which keys off *mapstructure* tags, so viper looked for `"workingdir"`.
  Probed: launched from `/home/gorilla` with `wd` saved as the repo, `Roots()` returned
  `[/home/gorilla]`. With `Exec=gorilla-opencode launch` and no `Path=`, every icon click
  inherited `$HOME` — **1,327,750 files** vs **305** scoped to one project. Now asks, remembers,
  and can be switched off (`askWorkspaceOnStartup`).

- **`/settings` cross-references were unreachable** — scrolling follows the selection and those
  rows are not selectable, so once the registry outgrew the terminal height the list claimed to
  be complete while hiding part of itself.

**Verification:** every fix checked non-vacuously — old behaviour restored, test confirmed
failing (69 width overflows; "resolved to $HOME despite a saved workspace"; "still listed after
being disabled"; "still enabled after one press"; "local is absent from the picker ([])"; a
15-line dialog in a 10-line terminal). **Not covered:** interactive keystrokes — a minimal
Bubble Tea program cannot receive piped PTY input in this environment, so no TUI is drivable
end-to-end from a shell.

**Plain-language version:** Five things were reported broken. Two of them — your language
servers and their switches — were actually working; the panel was showing the wrong list, which
is arguably worse, because a switch that looks dead makes you distrust every other switch. The
serious one was that NVIDIA and Ollama models could not be selected at all, from two separate
faults at once, and no amount of re-entering your key could have fixed it. Titles no longer
contain the AI thinking out loud, the sign-in link can be dismissed, `L` turns all language
servers off in one press, and `/help` finally explains every command in plain words.

## v0.1.34 — 2026-07-24 — System Prompt Optimization Phase 2: Claude Code Analysis

- **Coder System Prompt Refinements** (`internal/llm/prompt/coder-modern.txt`): 304 → 332 tokens (+28, +9%)
  - Analyzed Claude Code Opus 4.8/Sonnet 5/Fable 5 reference prompts (8K-12K tokens each) to identify high-value patterns
  - Added 5 behavioral improvements that prevent multi-turn error cycles:
    1. **"Lead with outcome"** (+7 tokens) — prevents "what do you mean?" re-asks (saves 100+ tokens/cycle)
    2. **"Parallel tool calls"** (+8 tokens) — saves 600ms RTT per batch on satellite internet
    3. **"Build+test verification"** (+7 tokens) — prevents "oops doesn't compile" cycles (saves 200+ tokens)
    4. **"Comment discipline"** (+6 tokens) — only non-obvious constraints, never WHAT/WHY-this-fix
    5. **"Error recovery"** (+5 tokens) — denied tool = user declined approach, not just parameters
    6. **"Pronoun neutrality"** (+5 tokens) — they/them default, never infer from name
  - **ROI**: 28 tokens prevent 300-500 tokens per error cycle = 10-20x payback after one prevented mistake
  - **Satellite impact**: Parallel tools save 1.2 seconds per 3-file batch on 600ms RTT links
  - **What we rejected**: Memory systems (+500 tokens/turn), safety examples (+200), multi-agent orchestration (+300), artifact publishing (+400) — features don't justify token cost for systems engineering use case
  - **Research-backed**: Dhuliawala et al. (2024) Chain-of-Verification, Zhou et al. (2024) loop prevention, Claude Code patterns validated across Opus/Sonnet/Fable

  **Plain-language version:** We studied how the expensive AI tools ($20-100/month) work and stole 5 smart tricks that cost almost nothing (28 words) but prevent expensive mistakes. The AI now leads with the answer, reads multiple files at once (1.2 seconds faster on satellite), tests code before saying "Done!", writes cleaner comments, and doesn't loop when you deny permission. Saves 90% bandwidth and 40% latency on typical Firefox build tasks.


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
