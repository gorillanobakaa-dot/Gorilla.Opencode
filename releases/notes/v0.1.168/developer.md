# For developers

## Summary

Fixes the `/context` loadout dialog key-binding regression discovered on live
`v0.1.167` usage (`internal/tui/components/dialog/loadout.go`):

- Split `loadoutKeys.Confirm` (`"enter"`) out of `loadoutKeys.Toggle` (`" "`).
  Previously `Toggle` matched both Space and Enter and called
  `m.adjustSelected(+1)` on dial rows (`rowPace`, `rowLeash`), so dialing left
  to Nuclear Option (`config.SubAgentsNuclear == 0`) and pressing `Enter` to
  save/close silently stepped `MaxSubAgents` right to `1`.
- Pressing `Enter` anywhere in `/context` now preserves every active dial and
  toggle value untouched, emits `LoadoutChangedMsg`, a confirmation info toast
  (`Saved: NUCLEAR OPTION ACTIVE — main agent works solo` when Nuclear is set),
  and `CloseLoadoutDialogMsg`.
- Pressing `Space` on dial rows (`rowPace`, `rowLeash`) records `[SET]` /
  `[APPLIED]` confirmation without altering the dial value; `Space` on feature
  and display rows continues to toggle `ON`/`OFF`.
- Updated the `/context` footer help line and Nuclear Option banner copy to
  advertise `enter confirm & close`.
- Added 4 privacy-scrubbed screenshots under `docs/screenshots/gallery/v0168-*.png`
  illustrating the tiered `/model` picker, 3-agent and 12-agent caps, and
  Nuclear Option state.

## Security posture

Pure TUI key-handling fix and static documentation images (verified with
`fieldkit privacy scan --images`: 0 findings). Adds no network calls,
telemetry, providers, or third-party dependencies.

## Deployment

Install using the release binary, installer, or package for Windows, Linux, or
macOS and verify against `SHA256SUMS-v0.1.168.txt`. `gorilla-opencode --version`
reports `v0.1.168`.

## Verification

Added `TestLoadoutEnterConfirmsAndClosesWithoutUndoingNuclear` in
`internal/tui/components/dialog/agent_state_visibility_test.go`, verifying that
stepping `←` to Nuclear Option (`0`), pressing `Space`, and pressing `Enter`
keeps `config.MaxSubAgents() == 0` and emits `CloseLoadoutDialogMsg`, and that
pressing `Enter` on a feature row closes `/context` without flipping the toggle.
See the release evidence for Linux and Windows CI and reproducible release build
results.
