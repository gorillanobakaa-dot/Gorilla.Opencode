# Gorilla OpenCode 0.1.166: visual effort slider and cleaner model picker

## What is this program?

Gorilla OpenCode is a terminal coding assistant. It uses the models and
connections you choose, including models running on your computer.

## Should you download this version?

Update if you use `/model` to switch models or reasoning effort on Antigravity
or ChatGPT sign-in.

## Why this matters to you

Reasoning effort now renders as a live color-shifting horizontal slider with
centered stop labels and plain-English descriptions for both Antigravity and
ChatGPT, plus cleaner model names and the active ChatGPT effort right in the
status bar.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### What this is

A visual Effort slider in the `/model` picker, cleaner Antigravity model names,
and your active ChatGPT reasoning level shown in the bottom status bar.

### Should you download it?

Yes if you switch reasoning effort or use Antigravity or ChatGPT sign-in.

### Why this matters

- The `/model` picker now draws a horizontal Effort slider with directional
  arrows, circular stops, and color cues that shift with effort level
  (cyan for minimal, green for low, amber for medium, red for high,
  purple/magenta for xhigh and above).
- Centered labels sit directly under each stop, with a live italic explanation
  underneath for both Antigravity and ChatGPT models.
- The search field is underlined, keyboard shortcuts use highlighted keycaps,
  and Antigravity model names drop redundant raw IDs in square brackets while
  keeping distinct routes clearly labeled.
- The bottom status bar now shows your active ChatGPT effort (for example,
  `GPT-6.2-Sol (Medium) (ChatGPT sign-in)`).

### What was checked

See the release evidence for GitHub CI results across Linux and Windows and
local build verification.

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Replaced the plain-text `Reasoning` row in `internal/tui/components/dialog`
with a multi-row `Effort` slider (`effortSliderLines`) that adapts stop spacing
to terminal width and falls back to a compact single-line bar on very narrow
terminals. Active tracks, dots (`●`, `◎`, `○`), and active labels shift color
by effort tier via `lipgloss.AdaptiveColor`, while directional arrows (`◀`/`▶`)
dim at boundaries.

Both Antigravity family variants and ChatGPT `ReasoningEfforts` share the same
slider and live italic description renderer, preferring provider-supplied
descriptions and falling back to built-in Agy-matching explanations.
`cleanFamilyNames` removes redundant bracketed raw IDs from collapsed
Antigravity families while retaining readable route suffixes (`(Tiered)`,
`(2.5 Flash)`, etc.) whenever routes differ. `models.DisplayNameWithEffort`
formats active ChatGPT effort across the status bar, inline footer, banner, and
selection confirmation.

### Security posture

Pure TUI rendering and model display formatting. Adds no network calls,
telemetry, providers, or dependencies.

### Deployment

Use the installer or package for your platform and verify the published
checksums. `gorilla-opencode --version` should report `v0.1.166`.

### Verification

Unit tests cover slider node/label alignment, per-level TrueColor RGB
progression, boundary arrow dimming, Antigravity and ChatGPT italic
descriptions, Antigravity family name disambiguation, narrow-terminal width and
80x24 height invariants, and ChatGPT status bar effort formatting. See the
release evidence for CI outcomes.
