# v0.1.84 — quota warning messages now scream in bright red

Quota tier-crossing alerts and `/usage` summary lines in the scrollback were
displaying in plain terminal white — invisible against normal output. They now
appear **bold and bright red (`#FF0000`)**, matching the warning-colour intent
established in v0.1.83.

**What changed:** One function, `formatQuotaScrollbackLine` in
`internal/tui/tui.go`, now wraps its output in a lipgloss bold+red style before
passing it to `tea.Println`. Both message paths (`quotaLineMsg` and
`quotaAlertMsg`) route through it, so one change covers both.

**Why the v0.1.83 theme change didn't do this:** `WarningColor` in the theme
reaches the footer via `t.Warning()`. `tea.Println` writes raw bytes above the
inline frame and bypasses the theme entirely — the style has to live in the
string itself.

Full release notes: [Changelogs/v0.1.84-release-notes.md](Changelogs/v0.1.84-release-notes.md)
