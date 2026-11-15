# v0.1.163, for developers

**Previous version:** v0.1.160

## Summary

- picker: one row per model family with an effort chooser; usage: live model list; doctor features (YAML rules)
- permissions.web, git -C / --no-pager reads, read-only diagnosis commands, outside-root places (PRs #74-#82)
- tui: Esc 'Continue or change?'; steer queue; peers notice; status bar working folder
- antigravity: real app list body, UA and ideType; probe hides models the backend does not serve; v0.1.161 and v0.1.162 were never tagged, their changes ship here
- picker: backend notice (tagTitle, tagDescription); ChatGPT sends the chosen effort; recorded-transport tests (PR #83)
- range `v0.1.160..origin/main` (PRs #64-#83)

## Security posture

No new network channel and no telemetry.

## Deployment

```
gorilla-opencode --version
# Expected: v0.1.163
```

## Not verified

- macOS and Windows runs of the new picker and permission rules were not done by hand; CI ran the tests on Linux and Windows.

## Known issues

- The Windows exe is unsigned.
- `doctor features` reports 14 stale doc references that were not fixed.
