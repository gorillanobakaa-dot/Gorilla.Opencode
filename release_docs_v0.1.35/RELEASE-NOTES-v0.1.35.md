# Release v0.1.35 — v0.1.35 Text Selection Mode & Status Bar Rendering Fixes

**Date:** 2026-07-25

---
## Why This Release Exists
Users reported text selection being impossible due to mouse trapping, along with visual glitches including duplicated status bars and ANSI escape codes bleeding into standard views.

## What You'll Actually Notice

**Text Selection**
- BEFORE: Mouse drag selection failed or was trapped by TUI cell motion.
- AFTER: Pressing ctrl+y toggles native terminal text selection mode.
- Affects: All interactive TUI users

**Status Bar Display**
- BEFORE: Duplicate status bars appeared at screen bottom with line overflow.
- AFTER: Status bar renders cleanly in a single clamped row.
- Affects: All users

## Decisions & Why

**Added ctrl+y selection toggle** — Allows toggling mouse cell motion dynamically to enable native terminal mouse selection without breaking scroll wheel functionality. (benefits: Users selecting text from chat buffers.; tradeoff: Mouse wheel scroll is disabled while in native selection mode.)

## Deliberately Not Done

- **Full custom internal selection buffer across viewports** — Extremely complex rewrite that would require tracking custom text selections across multiple lipgloss components.

## Privacy & Security
No changes to data handling or permissions.

## Risks, Backup & Recovery

- **Backup needed:** None
- **Install time:** < 1 minute
- **Restart required:** Restart gorilla-opencode TUI
- **Rollback:** Reinstall v0.1.34 .deb package if needed.

## How To Install

**Step 1:** Install updated Debian package
```
sudo dpkg -i gorilla-opencode_0.1.35_amd64.deb
```
✓ Success looks like: Unpacking gorilla-opencode (0.1.35)... Setting up gorilla-opencode (0.1.35)...

## Common Questions

**Q: How do I select text in the TUI now?**
A: Press ctrl+y to enable selection mode, select text natively with your mouse, then press ctrl+y again when done.

## Bottom Line
This maintenance release resolves core UI glitches and unlocks standard mouse text selection via ctrl+y toggle.

## Where This Came From

| Claim | Basis | Evidence from your input |
|-------|-------|---------------------------|
| Text selection fixed via ctrl+y keybinding | 🤖 model inference | *(none — this is the model's own judgment)* |

---
**How to check this document, not just trust it:**
This was written by an AI model reading the input above, not looked up from any external database.
"📄 stated in input" claims are the model's phrasing of something your source text actually said —
verify by finding the matching line in your own file. "🤖 model inference" claims are the model's
own judgment or synthesis, not a fact from your source — treat these as opinions, not measurements.
The model can still mislabel something or make mistakes even when it's trying to be accurate. To
check further: read the raw `.json` file this tool also writes (the unformatted answer, before any
of this rendering); or re-run this tool on the same input and see whether the specific numbers and
claims stay consistent between runs.

---

# v0.1.35 TUI Fixes: Selection Toggle & Dynamic Status Height

## Architecture & Footprint


## Security Posture


## Where This Came From

| Claim | Basis | Evidence from your input |
|-------|-------|---------------------------|
| go test ./... passes | 🤖 model inference | *(none — this is the model's own judgment)* |

---
**How to check this document, not just trust it:**
This was written by an AI model reading the input above, not looked up from any external database.
"📄 stated in input" claims are the model's phrasing of something your source text actually said —
verify by finding the matching line in your own file. "🤖 model inference" claims are the model's
own judgment or synthesis, not a fact from your source — treat these as opinions, not measurements.
The model can still mislabel something or make mistakes even when it's trying to be accurate. To
check further: read the raw `.json` file this tool also writes (the unformatted answer, before any
of this rendering); or re-run this tool on the same input and see whether the specific numbers and
claims stay consistent between runs.
