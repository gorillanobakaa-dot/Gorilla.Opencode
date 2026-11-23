# Gorilla OpenCode 0.1.169: no more approval floods, /plan can write its plan file, honest SOLO MODE label

## What is this program?

Gorilla OpenCode is a terminal coding assistant. It uses the models and
connections you choose, including models running on your computer.

## Should you download this version?

Yes. Update from `0.1.168` if you leave it running unattended, use `/plan`,
or run long sessions on a paid model.

## Why this matters to you

- **No more approval floods.** A refused action now fails only that one call;
  the rest of the batch keeps going. Set `GORILLA_UNATTENDED=1` and it never
  stops to ask (hard lines stay refused).
- **`/plan` can write its plan file.** Plan mode used to block the plan itself.
- **The default is solo (Economy, 0 helpers) and says so.** It no longer shows
  "NUCLEAR OPTION ACTIVE"; it shows "SOLO MODE (default)".
- **Cost warning.** When 90% or more of a session's input is the old
  conversation re-sent (the Opus credit drain), you get one warning to run
  `/compact` or start a new session.
- More read-only git commands run without asking, and the effort bar behaves
  the same on every model.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### What changed

1. The program no longer stops you over and over to ask permission. If one
   action is refused, only that action is skipped.
2. `/plan` can now save its plan to a file.
3. Out of the box it works alone (no helper agents) and the screen says
   "SOLO MODE" instead of a scary red "NUCLEAR OPTION".
4. If a long session wastes money re-sending the old chat, it tells you once.

### Should you download it?

Yes, if you run it unattended, use `/plan`, or pay per use.

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

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

### Security posture

No new network calls, telemetry, providers or dependencies. Fail-closed hard
lines unchanged: unattended mode still refuses what the hardlines gate refuses.

### Not verified

Live TUI run on a real terminal; real-Fieldkit replay in CI.
