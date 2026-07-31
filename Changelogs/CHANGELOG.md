## v0.1.48 → v0.1.64 — 2026-07-31 — The conversation no longer stops dead at the first tool call

Sixteen builds made between 28 and 31 July 2026, none of which were ever
published. This entry covers all of them. Full documents:
[layman](v0.1.48-v0.1.64-LAYMAN.md) · [developer](v0.1.48-v0.1.64-DEVELOPER.md).

**Plain-language version:** for three days this program had a bug that made it
close to unusable, and it hid itself well. When the AI used a tool — searching
your files, running a command — the answer arrived, was saved, and was never
shown to you. The screen sat on "Waiting for response…". On 30 July a command
finished in two seconds, the AI wrote its full answer, and the screen showed
nothing for fifteen minutes; that is indistinguishable from a stuck connection,
so you wait, then restart, then blame your provider. Every conversation that
used a tool was cut off at its first tool call. Alongside it, one search could
return 2.4 megabytes in a single result and quietly wreck your token budget, the
context meter read about two hundred times too high, Escape did not stop the AI,
and the bar at the bottom of the screen crawled down the window and jumped back
up. All fixed. Every one was found by measuring, not by reasoning about the code.

### Fixed

- **The transcript no longer halts at the first tool result** (v0.1.63). The
  biggest fix here. `ScrollbackReady` returned false for tool messages to stop
  double-printing, but `printPending` breaks on the first not-ready message — so
  every later message, including the model's finished answer, was generated,
  persisted, and never displayed. **"Ready" means "will not change again", not
  "has something to show".** Duplicate suppression moved to
  `RenderForScrollback` returning `""` for that role.
- **Every tool result is bounded by SIZE at one choke point** (v0.1.62). grep
  capped matches at 100 and returned **2,438,026 bytes**, because it matched
  inside files where a whole source file is one escaped string — 80 lines over
  10 KB, longest 66,438. That one result took a conversation from 15.9K to 675K
  tokens in a single turn, and tool results are re-sent every turn afterwards.
  Now 400 KB in `NewTextResponse`. **A limit must be expressed in the unit of
  the resource it protects.**
- **No frame line may exceed the terminal width** (v0.1.57) — the real cause of
  the marching footer. The inline renderer erases by *logical* line count, so an
  over-wide line occupies two physical rows, counts as one, and under-erases by
  a row per render. Enforced centrally by `clampToWidth`.
- **The context meter was inflated ~200×** (v0.1.55); it displayed 387%. Failed
  turns now say why they failed instead of printing nothing.
- **Escape actually stops the model** (v0.1.54), and streamed reasoning wraps at
  word boundaries.
- **It says why there is no thinking** when you asked to see thinking (v0.1.60).

### Added

- Up and Down recall previous messages (v0.1.64).
- Reasoning streams into scrollback; the preview pane is gone (v0.1.58–v0.1.61).

### Changed

- All four system prompts rewritten (2026-07-29) against Anthropic's published
  Claude Fable 5 guidance, with the research cited in
  [`system-prompts/RESEARCH-SOURCES.md`](../system-prompts/RESEARCH-SOURCES.md).
  Coder prompt 1,855 → 4,233 bytes (~464 → ~1,058 tokens/turn). Every section is
  switchable in `/context` with its cost and what you lose; two are marked
  critical because disabling them increases unverified success claims.

### Known issues

- **The footer is still reported to jump.** Two hypotheses are dead, both with
  permanent tests: height oscillation, and the 20-row editor collapse. Diagnose
  with a real byte capture replayed through `internal/tui/inline/terminal_test.go`
  — not from a screenshot.
- **The v0.1.57 width fix is verified headlessly only**, not across a long
  interactive session. It bites hardest near 80 columns.

### Corrections to the record

- **v0.1.56 was shipped on a wrong diagnosis.** Frame-height oscillation was
  blamed for the marching footer; a headless test shows 3↔4 rows and a 20→1
  collapse both render correctly. The change was kept (constant height is still
  more predictable) but its commit message states a cause that is not real.
  Three independent sources reached that same wrong answer.
- **v0.1.59 shipped with a failing test.** A shell chain did not gate on the
  exit code and printed "all green" while a test was red. A pipe returns the
  last command's status, not the test's.
- **The v0.2.0 / v0.1.49 version numbers were never real.** They were invented
  in error, never approved, and the release carrying them had no downloadable
  assets. Both have been removed. The documents written under them were good
  work and are kept; only the numbers are gone.

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

