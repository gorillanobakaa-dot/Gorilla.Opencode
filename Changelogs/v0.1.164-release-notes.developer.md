# v0.1.164, for developers

**Previous version:** v0.1.163

## Summary

- usage: the countdown comes from the backend's own reset field through one formatter, `quota.ResetIn` (rounds up; under a day `4h 59m`, from a day `3d 15h`); no reset field means no text; tests cover both (replaces "resets today" and floor-to-days)
- usage: one quota meter per backend bucket (Ultra-only included); missing reading is unknown, not 0%; unmapped models say why (PR #87)
- peers: short socket path for a long state folder; debug line with the effort sent (PR #86)
- tui: peer messages are their own entry, drawn apart; display-only copy of Zero's traffic, never queued for the AI; every theme pair and the status bar reach WCAG AA (PR #88)
- range `v0.1.163..origin/main` (PRs #86-#88)

## Security posture

No new network channel and no telemetry.

## Deployment

```
gorilla-opencode --version
# Expected: v0.1.164
```

## Not verified

- No live-account run of the meters; CI ran the tests on Linux and Windows.

## Known issues

- The Windows exe is unsigned.
