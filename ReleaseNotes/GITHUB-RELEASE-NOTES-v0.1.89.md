# Gorilla Opencode v0.1.89 — no window draws outside the window

A display-correctness release. One commit, one fault, three screens.

## What was wrong

`/context` drew a **106-column frame whatever your terminal's width was** — its
width calculation floored *up* to a 100-column minimum and then added 6 more
for the border. Both `/osint` screens had the same shape at 80 and 70 columns,
and the `/osint` warning screen asked for **37 rows on a 24-row terminal**.

This is not cosmetic. The screen is redrawn by erasing the previous frame, and
the erase counts **logical lines written**, not rows the terminal used. A line
too wide to fit occupies two physical rows while counting as one, so every
redraw leaves a row behind — and those leftovers strand in your scrollback where
nothing can clear them.

On a maximised terminal you would never have seen it. It bites below about 106
columns: narrow windows, split panes, phone and tablet sessions.

## What changed

- Dialog width is now the **terminal minus the border**, with the preferred
  width as a cap rather than a floor. Lines too long for the space are cut with
  an "…" instead of spilling past the edge.
- The `/osint` warning screen **sheds its prose in stages until it fits**,
  keeping what the screen exists for: the warning, the burn rate, the helper and
  mode controls, and the keys.
- `/context` headings and hints are cut to the available width instead of being
  handed raw to a wrapping layout — which gives a row back on the smallest
  terminals (it now needs 18 where it needed 19).

## Why the tests did not catch it

The existing width test renders through a cell-grid emulator that **clips**
over-wide lines exactly as a real terminal does. Clipping destroys the evidence
before the assertion runs, so that test could never fail for this defect. The
new one measures the frame **before** it reaches a grid, and is proven
non-vacuous: put the old code back and it reports 106 columns at three
different terminal sizes.

## Still open

The stranded-lines report from an Arch/CachyOS tester is **not closed**. This
fix is the same *class* of bug and his screenshots do include the `/context`
screen, so it is now the leading explanation — but it is a plausible mechanism,
not a confirmed diagnosis. Confirming it needs his terminal, font, `$TERM`, and
whether a menu was open at the time.

Everything from v0.1.88 — the `find` tool, `/osint`, the 985-source catalogue —
is unchanged. Binary grows by 8,192 bytes.

Full dual-track notes: [v0.1.89-release-notes.md](Changelogs/v0.1.89-release-notes.md).

**Install (Debian/Ubuntu):** `sudo apt install ./gorilla-opencode_0.1.89_amd64.deb`
— use `apt`, not `dpkg -i`, so the `lynx` dependency resolves.
**Arch/CachyOS:** `sudo pacman -U gorilla-opencode-0.1.89-1-x86_64.pkg.tar.zst`
