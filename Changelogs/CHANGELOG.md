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

