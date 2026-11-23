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
