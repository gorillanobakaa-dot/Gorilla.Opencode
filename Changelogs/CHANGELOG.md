## v0.1.45 — 2026-07-28 — Two dialogs were cut off and nothing was checking

The screen-level test harness built after v0.1.44 was pointed at the dialogs and
immediately reported overflow in **seven of eight**, two of them on an ordinary
80×24 terminal: **`/context` wanted 37 rows** (cut off even at 100×30) and
**`/connect` 26**. Both were being truncated on screen with nothing to indicate
anything was missing.

**Plain-language version:** two menus were having their bottoms cut off on a
normal-sized window, and nobody had noticed because nothing was checking what the
screen actually showed. There is now a test that checks exactly that — it found
these itself — and the menus size themselves by measuring rather than guessing. If
you never opened `/context` or `/connect`, nothing else changes for you.

### Fixed

- **`/context` and `/connect` truncated on a standard terminal.** The cause was
  arithmetic instead of measurement: capacity was derived by subtracting a constant
  standing in for border, padding and title. `/help`'s constant
  (`commandHelpFixedLines = 2+2+3`) was simply wrong, with a `minListRows` floor
  that silently overrode the height limit on top of it. `layout.FitHeight` now
  renders, measures, and reduces rows until the result genuinely fits.

  `/context` additionally **never recorded the terminal height at all**, which is
  why it could not size itself. It now stores it, windows its switch list around the
  selection, and sheds explanatory prose before it sheds a switch. `37 → 19` rows;
  `/connect` `26 → 17`. Anything scrolled out of view is announced, because a hidden
  row is indistinguishable from a missing feature.

- **A clamp in `PlaceOverlay` that did nothing.** Added so an oversized dialog could
  not push the composed frame past the terminal and scroll it. It trimmed `fgLines`
  but the early return (`if fgWidth >= bgWidth && fgHeight >= bgHeight { return fg }`
  — upstream's own `FIXME`) handed back the untouched original, so it had no effect
  on the one path it existed for. Fixed, and it now has tests, which it did not
  before.

### Added

- **`internal/tui/screentest` — assertions on a terminal cell grid.** Renders a
  component into a fixed-size buffer and reports what each cell holds, including
  whether its text is *legible* (foreground differs from background). This is the
  tool that found the overflows.

  It exists because string matching cannot catch two of the three shapes of display
  bug this project has shipped. Text present but invisible is a question about a
  cell's two colours, not the byte stream — that was the v0.1.42 `/help` bug, and it
  took three attempts to write a string test for it, the first of which *passed
  against the bug*. And overrunning text is easy to miss because lipgloss **wraps
  rather than overflows**, so an over-long line appears as extra height.

  **Zero new dependencies:** `x/cellbuf` was already in the module graph.

  The harness had two defects of its own, found before trusting it. lipgloss strips
  styling when the output is not a terminal, so every styled string arrived plain
  and every colour assertion passed vacuously — the package was *worse than useless*
  in that state. And a double-width grapheme occupies two cells, so reading a space
  for the second split `日本` into `日 本`.

### Known issues

- Dialogs still overflow terminals below 24 rows, held by a **ratchet** keyed by
  dialog *and* size, asserted in both directions: worse fails, and **better also
  fails** until the entry is lowered, so the record cannot rot. Keying on name alone
  reported four false improvements immediately — a narrower terminal wraps more and
  needs *more* rows.
- The sweep covers 8 of ~15 dialog surfaces. The model picker, sessions, permissions,
  tasks, filepicker, theme and arguments dialogs are not yet in it and may overflow
  undetected.
- `Legible()` treats a nil foreground or background as readable, since the terminal's
  defaults are unknown — a one-sided colour matching the user's theme would slip past.
- The main interface still cannot be selected or copied; `--plain` remains the answer.

### On verification

Reported plainly because it bears on how much the above should be trusted:
per-call-site reverts were **inconclusive**, since each site kept a fallback that
still measured. Only neutralising `FitHeight` itself was decisive — **10 failing
subtests** across `/connect`, `/context` and `/help`. Forcing `compact=false` failed
3 more including 80×24. One test I had written was **deleted**: it claimed `/context`
keeps switches rather than prose, but measurement showed 10 switch rows visible with
*and* without the shedding, so it could not fail for its stated reason.

