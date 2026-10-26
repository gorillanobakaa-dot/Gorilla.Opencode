# MacBook Air (macOS 10.13, 1366x768): window placement and layout fixes

Version: 1.0.0 · updated 26-10-23-02-45

Owner, 2026-10-23 (translated from Romanian): "when the window opens, it must be calculated to open almost at the
maximum, 167x39, so that it sits above the Dock and is CENTRED properly, not going outside the screen at the top,
the bottom, the left or the right. Then all the screenshots of layout errors I documented on the MacBook Air's
desktop must be analysed and fixed."

The Air runs v0.1.149-dev (old source); Linux and Windows run the latest. The screenshots are in
`docs/screenshots/gallery/macos-air-2026-10/` (30 files, full size, from the Air's Desktop).
Published 2026-10-26: the pictures that show no account name, e-mail address, sign-in link, computer name or
Romanian text; three of them with the title bar cropped off. Eleven pictures and one text capture of the screen
stay on the maintainer's machine because the identifying text sits inside what they show.

## 1. The window: 167x39, centred, above the Dock, never off screen

Measured 2026-10-23: the window opened at 169x33 with its right part past the screen's right edge (x from 275 to
beyond 1366); the owner dragged it back by hand. The source launcher
`packaging/macos/Gorilla OpenCode.command` only GROWS the window (xterm `ESC[8;rows;cols t`, default 138x37) and
never POSITIONS it. The launcher installed on the Air is older still: it does not size the window at all.

Target behaviour of the launcher:
1. Read the usable screen: `NSScreen.mainScreen.visibleFrame` (AppleScriptObjC, `use framework "AppKit"`), which
   already excludes the menu bar and the Dock, whatever side the Dock is on and whether it hides. (Finder's
   `bounds of window of desktop` failed over ssh with -1728; system_profiler only gives the panel size and takes
   about a second.)
2. Ask for 167x39 (owner's number for this Air), then CAP it to what fits the visible frame for the current font:
   cell width/height from the window's pixel size divided by its columns/rows after the resize (`bounds of front
   window` in AppleScript), so a bigger font gets fewer columns, never an off-screen window.
3. Centre it: x = vf.x + (vf.w - win.w)/2, y = vf.y + (vf.h - win.h)/2, and set `bounds of front window`
   (Terminal honours it). Clamp so all four edges stay inside the visible frame.
4. Keep `GORILLA_WINDOW` as the person's override, still capped to the screen.
5. Do it BEFORE the folder dialog, so the program's first frame is drawn at the final size (a resize after start
   is what leaves duplicated prompt rows, see 2c below).

Done when: on the Air, a double-click opens one Terminal window, 167x39 or the largest that fits, centred, its four
edges inside the visible frame, nothing under the Dock (screenshot proof, full size). A test asserts the arithmetic
(visible frame and cell size in, bounds out) including a Dock on the left and a hidden Dock.

Also seen: Terminal opens an extra empty window when it starts (Fieldkit's `fieldkit mac open-app` closes it); the
launcher should not leave one behind either.

## 2. Layout defects seen on the Air (analysis of the screenshots below)

Known from the harness's own captures and text reads (Terminal tab `history`), to confirm against the gallery:
- a. window past the right edge (section 1).
- b. the header loses its first letter: "orilla OpenCode", with a lone "G" at the far right column. The text read
  shows the row padded with spaces to the full width, so the next character wraps (the "no line wider than the
  terminal" rule in CLAUDE.md, at exactly width, not width+1: Terminal.app's autowrap moves to the next row when a
  character lands in the last column). The quota line does the same.
- c. the input area shows 2-4 ">" prompt rows and two cursor blocks while empty (owner's screenshot 01:14).
- d. permission dialogs render an EMPTY Command/URL field for short single-line values; a long wrapped command
  showed (being fixed in the permissions rework, PERMISSIONS-REWORK-PLAN.md).
- e. rows behind a dialog cut to "Waiting for to".
- f. the status bar's right end clipped ("v0.1.149-de").
- g. the OAuth URL printed at start runs past the right edge.

### Confirmed against the gallery (all 30 read, 2026-10-23; 8 show Gorilla OpenCode, 22 the browser)

| # | Defect | Files (air-2026-10-23-*) | Likely cause |
|---|---|---|---|
| 1 | 2-4 ">" input rows, 1-2 cursor blocks, stray blue block | ALL 8: 011418, 013154, 013256, 015247, 015258, 015446, 015712, 021438 | input frame redrawn without erasing the previous one |
| 2 | window past the right screen edge at 169x33 | 011418, 013154 (x=12-16), at the edge in the rest; fits at 164x35 (013256) | launcher never positions the window (section 1) |
| 3 | header "orilla OpenCode", lone "G" in the last column of the row above | 015247, 015258 (G visible), 011418 (G off screen) | a row padded to exactly the width wraps in Terminal.app |
| 4 | permission dialog: empty URL / Command value | 015446, 015712 (URL), 021438 (Command) | value not rendered for short single-line values (permissions rework) |
| 5 | rows behind a dialog cut to "Waiting for to", a grey block beside and below the dialog | 015446, 015712, 021438 | overlay drawn over existing rows without clearing |
| 6 | dialog border misaligned: top border offset, stray vertical bar, doubled corner | 015446, 015712, 021438 | border width counted differently from content width |
| 7 | status bar right end clipped "v0.1.149-de" | 013154 | footer wider than the terminal by one or more columns |
| 8 | stale rows left behind: orphan empty dialog top border, stacked "Generating..." x4 and "Waiting..." x2, a duplicated status bar with the info line missing, spinner fragments in the last column | 013154, 021438; fragments also in 015247/015258, 015446, 015712 | frames not erased; something drawn in the last column |
| 9 | early or broken wrapping: "Still waiting... Press esc to / cancel." on 2 lines; the pasted prompt hard-wrapped mid-word; /usage lines wrapped early, last line cut ("| resets") | 021438, 015446, 013256 | fixed wrap widths and frame height in those views |
| 10 | no status bar visible under the input | 011418 | frame taller than the space |

Everything here shares one cause the project already knows (CLAUDE.md, "NO LINE IN THE FRAME MAY BE WIDER THAN
THE TERMINAL" and "DECORATION IS A LAYOUT LIABILITY"): on Terminal.app a character in the LAST column also wraps,
so the safe width is terminal width minus one, and every line, border, spinner and footer must stay inside it.
Assert it headlessly at 167x39 and 169x33 (the Air's sizes) with the repo's terminal emulator test, not by eye.
