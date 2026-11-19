# For developers

## Summary

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

## Security posture

Pure TUI rendering and model display formatting. Adds no network calls,
telemetry, providers, or dependencies.

## Deployment

Use the installer or package for your platform and verify the published
checksums. `gorilla-opencode --version` should report `v0.1.166`.

## Verification

Unit tests cover slider node/label alignment, per-level TrueColor RGB
progression, boundary arrow dimming, Antigravity and ChatGPT italic
descriptions, Antigravity family name disambiguation, narrow-terminal width and
80x24 height invariants, and ChatGPT status bar effort formatting. See the
release evidence for CI outcomes.
