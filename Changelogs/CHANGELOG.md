## v0.1.44 — 2026-07-28 — The copyable mode, now actually reachable

A correction to v0.1.43. We added a plain-text mode that solves a real problem —
the normal interface cannot be selected or copied — and then shipped it as
`--plain` only. The desktop entry is `Exec=gorilla-opencode launch` with no
arguments, and clicking the icon is how nearly everyone starts this program, so the
feature existed and most users did not have it.

**Plain-language version:** if you start the program by clicking its icon, you
previously had no way to reach the copyable mode at all. Now you right-click the
icon and pick it, or set it once in `/settings` and it applies however you launch.
Nothing about the normal interface changed; if you never use plain mode this
release changes nothing for you.

This is the second time this exact class of mistake has landed here. The agent's
working folder used to default to `$HOME` on an icon launch for the same reason —
the icon passes nothing, so anything that needs an argument is invisible to those
users. Worth writing down rather than rediscovering a third time.

### Fixed

- **Plain mode could not be reached without typing a flag** — four routes now,
  none of which need one:
  - **Right-click the desktop icon** → *Plain mode (selectable and copyable)*. A
    `Desktop Action`; `launch` has `DisableFlagParsing` and execs the real binary
    with its arguments verbatim, so `--plain` arrives intact without being typed.
  - **`/settings` → "Which interface to start"** → `plain`. Persisted, so it
    applies however the program starts, icon included. Marked `Restart` because the
    renderer is already running and cannot be swapped mid-session.
  - **`/plain` from inside**, which saves the preference and says plainly that it
    applies at next launch rather than appearing to do nothing.
  - **`--plain`** still works and overrides the setting, for a one-off.

  Precedence: explicit flag → saved setting → full interface. Full stays the
  default; plain mode carries fewer commands, so it must be chosen, never
  inherited. Any unrecognised value in the setting resolves to full, failing safe
  toward the interface with more capability.

  Verified against the icon's *exact* command: `launch` with no flags and
  `interface:plain` saved starts plain mode; set back to `full`, the same command
  starts the TUI.

- **The desktop entry existed in two places and would have drifted**
  (`packaging/gorilla-opencode.desktop` for the `.deb`,
  a string in `cmd/install.go` for `gorilla-opencode install`). Both updated, both
  pass `desktop-file-validate`, and `cmd/desktop_entry_test.go` now holds the
  fields that matter in step — including that the ordinary click stays on the full
  interface, and that the action's name mentions copying, which is the only reason
  it exists.

### Known issues

- The normal interface still cannot be selected or copied — unchanged from v0.1.43,
  and still a large refactor (21 overlay sites across 15 dialog surfaces plus
  ~2,700 lines of chat and layout code assuming a full-screen frame).
- Switching interface needs a restart. Both the command and the settings row say so.
- Desktop Actions may not appear until your environment rescans launchers; log out
  and back in if so.
- Exact reasoning-token counts remain unavailable from NVIDIA NIM.

