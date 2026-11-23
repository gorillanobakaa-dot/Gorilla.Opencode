# For developers

## Summary

- `internal/permission`: per-call refusals (`permission.Refused`) instead of the
  sentinel that cancelled the whole tool batch; `GORILLA_UNATTENDED` switch;
  plan-mode plan-document writes allowed; more read-only git commands classed
  as safe (`internal/llm/tools/commandclass.go`).
- Default sub-agent leash is Economy (0). UI labels say SOLO MODE; helper
  refusal text says "Sub-agents are OFF".
- `internal/tui/tui.go`: one-time warning per session when
  `ResentShare() >= 0.9` and cumulative prompt tokens >= 200000.
- Effort bar parity across providers.
- `hardlines_gate_test.go`: the real-Fieldkit replay no longer `t.Skip`s
  silently; a stand-in hook runs the same refusal list and logs loudly that
  real Fieldkit did not run.

## Security posture

No new network calls, telemetry, providers or dependencies. Fail-closed hard
lines unchanged: unattended mode still refuses what the hardlines gate refuses.

## Not verified

Live TUI run on a real terminal; real-Fieldkit replay in CI.
