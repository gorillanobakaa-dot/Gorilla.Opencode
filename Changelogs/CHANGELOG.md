## v0.2.0 — 2026-07-29 — New system prompts: breathing life into an old fossil

All four shipped system prompts rewritten against Anthropic's published Claude
Fable 5 prompting guidance. Credited and cited in
[`system-prompts/RESEARCH-SOURCES.md`](../system-prompts/RESEARCH-SOURCES.md);
full detail in [`v0.2.0-release-notes.md`](v0.2.0-release-notes.md).

**Plain-language version:** a "system prompt" is the standing instruction sheet
sent to the AI before it ever sees your question — it decides whether you get an
assistant that checks its work or one that tells you a build succeeded when it
did not. The one this program inherited from upstream OpenCode was written in
2023 and shouts: "IMPORTANT" occurs seven times, and the same brevity instruction
is given three times as if the model would only believe it on the third try. That
is not a dig at its authors — writing good Go is a genuinely different skill from
understanding how a language model behaves, and in 2023 almost everyone wrote
prompts that way. The research since says emphasis of that kind makes things
worse, not better. The new prompts stop the AI reporting progress it has not
verified, stop it taking actions nobody asked for, stop it ending a turn on "I'll
now run the build" without running the build, and stop it offering to hand off
just because the conversation got long. It costs more per request — about 1,058
tokens a turn instead of 464 — and every part of it can be switched off
individually in `/context`, with the cost and the consequence spelled out.

### Changed

- **All four system prompts** (`internal/llm/prompt/*.txt`) — coder, task and
  summarizer rewritten; title reviewed and deliberately left alone. Three new
  switchable coder sections: `# scope` (a question is not a work order; check the
  evidence supports a state-changing command), `# delegation` (use sub-agents,
  and know they are read-only), `# memory` (project context outranks assumptions;
  record lessons without duplicating what git already knows). Expanded `# honesty`
  (every progress claim must point at a tool result from this session),
  `# conduct` (do not end a turn on a promise; a long conversation is not a reason
  to stop) and `# output` (re-ground a reader who was not watching).
- **The cost, stated plainly:** coder prompt 1,855 → 4,233 bytes (measured with a
  probe against the built prompt), which is ~464 → ~1,058 tokens per turn at the
  one-token-per-four-bytes estimate `/context` uses. The byte counts are measured;
  the token counts are an estimate and are labelled as one everywhere they appear.
  Still roughly half the ~2,003-token 2023 prompt it replaced.
- **`summarizer.txt`** now preserves what was already tried and ruled out, so a
  freshly compacted context cannot retry a fix that failed an hour ago.
- **`task.txt`** lost "one word answers" — it was stripping the evidence along
  with the prose, and the parent agent never sees the sub-agent's tool calls.

### Added

- **`internal/llm/prompt/studycopy_test.go`** — byte-compares every embedded
  prompt against its published copy in `system-prompts/current/`. This directory
  exists so anyone can read the instructions the program actually sends, and it
  had already drifted: it was publishing two prompts that are no longer sent and
  none of the ones that are. A stale study copy is worse than none, because it
  reads as authoritative and is false.
- **Five research citations**, all fetched and title-checked against arXiv on
  2026-07-29 — *The Compliance Gap* (2605.01771), *OctoBench* (2601.10343),
  *Natural-Language Agent Harnesses* (2603.25723), *MAS-PromptBench* (2606.23664)
  and *AGENTIF* (2505.16944).

### Fixed

- **`system-prompts/` described a layout that no longer existed.** `current/`
  claimed to hold what ships and held the dead 2023 upstream pair instead; the
  live prompt's study copy was a prose draft that never matched the embedded text.
  Reorganised into `current/` (the four live prompts, now test-enforced) and
  `historical/` (the 2023 originals and the 2026-07 drafts, kept deliberately).
- **Five pre-existing citations** carried a paper's short or informal title rather
  than its full one, found during a full re-audit of every arXiv ID in the file.
  Corrected in place. No ID was wrong and nothing had been fabricated.
- **`coder.go`'s comment block** claimed the prompt was "~924 tokens" of "plain
  declarative prose". Both were stale.

### Known gaps

- No behavioural A/B against the previous prompt on a real Firefox or kernel
  build. The changes rest on published research and Anthropic's reported testing,
  not our own numbers. Tracked in `system-prompts/README.md`.
- The three new `/context` rows were asserted headlessly but not looked at.
- `send_to_user`, recommended by Anthropic's guide, was evaluated and deliberately
  not adopted: the model's text already streams straight to the terminal with
  nothing summarising it in between.

## v0.1.46 — 2026-07-28 — Undoing a slowdown I caused, and giving the mouse back

Three complaints, three real causes — but only one was what it looked like.

**Plain-language version:** the interface genuinely had got slower, by code I added in
v0.1.45; it is now slightly quicker than before that release. The *models* were never
slower — v0.1.45 stopped under-reporting their time by a factor of a thousand, so an
84-second reply that used to read `84ms` now reads what it always was. And dragging to
select text typed garbage into the input box because the program had quietly taken the
mouse away from your terminal; it no longer does.

### Fixed

- **Dialog redraws were 2–3× more expensive** (`internal/tui/layout/fit.go`) —
  `FitHeight` re-ran its whole render-and-measure search on every `View()`, and Bubble
  Tea calls `View()` on every keystroke **and** every streamed token. Instrumented at
  **3 internal renders per frame** for `/context`. `layout.Fitter` now caches the row
  count that last fitted, verify-then-reuse. Measured like-for-like at 100×30:

  | | v0.1.44 | v0.1.45 | now |
  |---|---|---|---|
  | `/context` | 2.33 ms | **6.65 ms** | **2.05 ms** |
  | `/help` | 1.28 | 2.70 | 1.35 |

  The first version of that cache keyed on terminal size alone and only asked *"does
  the remembered count still fit"*, never *"could more fit now"* — so one cramped
  selection locked in a small list and **two commands became unreachable** while
  scrolling `/help`. An existing reachability test caught it, not me.

- **Dragging to select typed raw escape codes into the editor** (`cmd/root.go`) —
  reported as `[<32;71;41M`. The program requested cell-motion mouse tracking, which
  takes the mouse from the terminal (Shift then needed to select) and reports **one
  event per cell crossed**, so a single drag fires hundreds and stalls the loop until
  the input parser spills raw codes. Dropping non-wheel events in `Update` was too
  late — the cost is upstream of any handler. Mouse reporting is now **opt-in**, with a
  `/settings` row that states the trade. Verified on the real binary under a pty:
  **`?1002h` emitted 0 times off, once on.**

- **One `/context` figure was still a guess** (`internal/llm/agent/calibrate.go`) —
  token costs are measured from real tool schemas at startup, except `diagnostics`,
  which was guarded on having LSP clients. A schema is static, so with every language
  server off — supported, and this developer's setup — that one row showed an estimate.
  Now measured unconditionally, with a test asserting no component still reports its
  declared value.

### Changed

- **One prompt rule relaxed, six kept** (`coder-modern.txt`) — Anthropic's Claude 5
  context-engineering guidance reports removing 80%+ of their coding agent's system
  prompt with no eval loss, and its worked example is a rule we also had. Ours now
  reads `comments: match surrounding density and idiom` instead of `never explain
  WHAT/WHY`. The other six `do not`/`never` lines were reviewed and **kept**: five are
  verification and honesty rules (*never claim unobserved success*, *do not invent
  paths*), and the guidance is about trusting judgement on **style**, not about
  trusting an agent's account of its own work.

- **The release tooling refuses to commit deletions** (`release_pipeline.py`) — it ran
  `git add -A` unguarded. **Nine files of published research under `system-prompts/`
  were sitting deleted in the working tree while this was being written**, unnoticed
  for hours, one release from being written permanently into a tag. Now it stops and
  lists them. It also fast-forwards `main` to the tag, which it never did — the
  omission behind `main` once sitting 43 commits behind.

- **`CLAUDE.md` now documents `release_pipeline.py`**, which has a `go_gorilla` profile
  built for this repo and was undocumented. That cost four consecutive releases driven
  by hand by sessions that never knew it existed.

### Known issues

- **The display corruption reported alongside the mouse leak was never reproduced.**
  Message rendering, the reasoning block, a 4,000-character unbreakable paste and the
  split layout all produce uniform widths headlessly. Attributing it to the mouse flood
  is reasoning, not proof — if it survives this release, that was wrong.
- The size sweep covers **8 of ~15** dialog surfaces; the rest may overflow undetected.
- **Tool descriptions are ~3,680 tokens against the prompt's ~464** and are the real
  cost centre — but there is zero duplication and no prescriptive language left in
  them, so the safe cuts are spent. Trimming further without an eval risks a quietly
  worse agent. Not attempted.
- `layout.Fitter`'s cache key is caller-supplied; nothing enforces completeness.
- The main interface still cannot be selected or copied.

