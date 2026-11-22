# In plain language

## What this is

A seven-part quality-of-life, safety, and cost-control update covering live
mid-turn commands (`/yolo`, `/clear`, `/compact`), session-wide permission
memory, structured `/plan` mode, fuller terminal layout and readable output,
a tier-sorted `/model` picker, explicit sub-agent/Nuclear Option confirmations,
and a critical fix for Opus 5.5 credit burn plus free-account effort controls.

## Should you download it?

Yes. Every daily user benefits — especially anyone running Claude Opus 5.5 on
Antigravity (which previously burned credits much faster than expected on
multi-turn sessions) or using free Gemini sign-in.

## Why this matters

- **Live `/yolo` and `/clear` while the agent is busy:** You can now type
  `/yolo` (or `/yolo off`) mid-turn without canceling the running reply, and
  press `Shift+Y` on a permission popup to switch straight into YOLO mode for
  safe commands (destructive or out-of-bounds actions still ask). `/clear` and
  `/new` now stop an in-flight turn immediately, discard queued prompts, turn
  off YOLO, and give you a clean screen.
- **No more repetitive permission floods:** Approving a tool family for the
  session (such as `python -u -m ...` or editing files in a folder outside the
  workspace) now actually sticks across the session and immediately releases
  other queued calls waiting on the same approval.
- **Reliable `/compact` and new `/plan` mode:** `/compact` no longer freezes
  the screen when summarization errors out, works mid-turn by queueing
  compaction right after the current reply finishes, and falls back to your
  active model if the summarizer model is unavailable. New `/plan <task>` lets
  the agent map out a step-by-step plan before editing files; run `/plan go` to
  execute or `/plan off` to exit.
- **Better screen use, readable output, and Windows font sizing:** Scrollback
  mode drops wasted side padding, Windows terminals reconcile their real size
  right after launch, classic Windows consoles switch cleanly to Cascadia Mono
  18 (with fallbacks, skipped in Windows Terminal or when
  `GORILLA_NO_FONT_OVERRIDE=1`), and chat replies separate prose from tool
  blocks with crisp colored `[Tool]` badges and left accent rails.
- **Clearer `/model` picker:** Models are now grouped with **Newer models**
  strictly above **Legacy models**, separated by vendor dividers (Anthropic,
  Gemini, OpenAI), launch dates (`[YYYY-MM]`), capability color cues, and
  short capability notes, with `(max)` / `(xhigh)` ranked above `(high)`.
- **Clear confirmation for agent limits & Nuclear Option:** Setting a sub-agent
  limit or enabling Nuclear Option (0 sub-agents) now shows a clear `[SET]` and
  `[APPLIED] SAVED` confirmation banner, keeps a red `NUCLEAR OPTION ACTIVE`
  indicator visible, shows `SOLO (NUCLEAR)` and the session's spawned-agent
  count in the status bar, and lets you inspect every sub-agent spawned during
  the session with `/agents audit` (stored locally in memory only; nothing is
  sent over the network).
- **Opus 5.5 credit-burn fix & free-tier Effort bar:** Claude Opus 5.5 thinking
  chunks were previously saved back into assistant message text and re-sent as
  input on every follow-up turn, multiplying credit consumption. Thinking tokens
  are now separated cleanly, non-streaming calls cap output tokens, Nuclear
  Option skips hidden background title calls, and free Gemini sign-in accounts
  now get the full Low / Medium / High Thinking Effort slider wired directly to
  Gemini's thinking budget.

## What was checked

See the release evidence for GitHub CI results across Linux and Windows and
full multi-platform release builds (Windows, Linux, and macOS).
