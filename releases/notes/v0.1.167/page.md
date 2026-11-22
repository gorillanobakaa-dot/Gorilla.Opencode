# Gorilla OpenCode 0.1.167: live /yolo & /clear, session permissions, /plan mode, cleaner UI, tiered model picker, and Opus credit-burn fix

## What is this program?

Gorilla OpenCode is a terminal coding assistant. It uses the models and
connections you choose, including models running on your computer.

## Should you download this version?

Yes. Update immediately if you use Antigravity Claude Opus 5.5, free Gemini
sign-in, `/yolo`, `/clear`, `/compact`, or run multi-step coding sessions with
tool permissions.

## Why this matters to you

This release stops a runaway Opus 5.5 credit-burn bug caused by re-sending
thinking tokens on every turn, brings the Thinking Effort slider to free Gemini
accounts, makes `/yolo`, `/clear`, and `/compact` work live while an agent is
running, stops repetitive tool approval floods, adds structured `/plan` mode,
organizes `/model` into clear Newer and Legacy tiers, and gives you full
confirmation and local visibility into sub-agent caps and Nuclear Option mode.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### What this is

A seven-part quality-of-life, safety, and cost-control update covering live
mid-turn commands (`/yolo`, `/clear`, `/compact`), session-wide permission
memory, structured `/plan` mode, fuller terminal layout and readable output,
a tier-sorted `/model` picker, explicit sub-agent/Nuclear Option confirmations,
and a critical fix for Opus 5.5 credit burn plus free-account effort controls.

### Should you download it?

Yes. Every daily user benefits — especially anyone running Claude Opus 5.5 on
Antigravity (which previously burned credits much faster than expected on
multi-turn sessions) or using free Gemini sign-in.

### Why this matters

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

### What was checked

See the release evidence for GitHub CI results across Linux and Windows and
full multi-platform release builds (Windows, Linux, and macOS).

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Seven cohesive changes land in `v0.1.167`:

1. **Live `/yolo` and `/clear` mid-turn:** `/yolo` (aliases `auto`, `autopilot`,
   `goal`) and `/yolo off` execute while a turn is active without cancelling
   it; bare `/yolo` is idempotent and releases pending covered permission
   prompts. Fixed `/yolo off` previously falling through as a user prompt.
   Added `Shift+Y` (capital `Y` only, non-button) on the permission dialog while
   preserving irreversible, hard-stop, and outside-root carve-outs. `/clear`
   and `/new` cancel in-flight turns, drop queued messages (`agent.DropQueued`),
   revoke YOLO, reset editor state via `SessionClearedMsg`, and clear visible
   scrollback.
2. **Session permission persistence & approval flood control:**
   `internal/permission/commandfamily.go` parses standalone Python interpreter
   flags (`-I`, `-u`, `-B`, `-s`, `-S`, `-E`, `-O`, `-OO`, `-q`, `-P`) before
   `-m <module>` and preserves dotted module names. `internal/llm/tools/bash.go`
   promotes non-quoted asked-history families to `SessionClass`.
   `internal/llm/tools/patch.go` routes paths through `permissionScope(absPath)`.
   `internal/permission/permission.go` scopes outside-workspace file family
   grants to their parent directory (`grantScopeDir`, refusing `/`, drive roots,
   and `$HOME`), matches edit targets via case-insensitive `samePath`/`pathWithin`,
   extends `extraRoots` synchronously in `grantPending`, releases newly covered
   waiters via `releasePendingCovered`, re-checks `IsCovered` post-Store, and
   uses non-blocking channel delivery.
3. **Reliable `/compact` and structured `/plan` mode:**
   `internal/tui/tui.go` surfaces synchronous `Summarize` errors via
   `compactStartErrMsg` instead of leaving the progress overlay stuck, and bare
   `/compact` during an active turn queues `compactAfterTurn`.
   `internal/llm/agent/agent.go` slices summarization from `SummaryMessageID`,
   falls back to the active coder provider if `summarizeProvider` is nil or
   fails, and fixes a missing `return` on `sessions.Save` failure. Added `/plan`,
   `/plan <task>`, `/plan go` (`approve`/`execute`), and `/plan off`
   (`stop`/`exit`/`cancel`) with previous-mode restoration, wire-only
   `planModeDirective` injection in `streamAndHandleEvents`, registry entries in
   `internal/commands/registry.go`, and updated `docs/COMMANDS.md`.
4. **Terminal real estate, console font override, and output readability:**
   `internal/tui/resize_windows.go` reconciles live terminal size once 120 ms
   after startup; `internal/tui/page/chat.go` removes side padding around
   `messagesContainer` in scrollback mode; `internal/tui/styles/markdown.go`
   sets `defaultMargin = 0`. Added `cmd/consolefont_windows.go` (Win32
   `SetCurrentConsoleFontEx` to `Cascadia Mono 18` with verification and
   fallback chain `Cascadia Code` -> `Consolas` -> `Lucida Console`, skipped
   under `WT_SESSION`, redirected pipes, or `GORILLA_NO_FONT_OVERRIDE=1`),
   `cmd/consolefont_linux.go` (OSC 50 on `xterm`/`rxvt` via `fc-match`), and
   `cmd/consolefont_other.go`. `scrollback.go`, `printer.go`, and `message.go`
   add blank-line separation between prose and tool blocks plus category-colored
   bold `[Tool]` badges and left accent rails with no opaque background fill.
5. **Tiered `/model` picker sorting and dividers:**
   `internal/llm/provider/antigravity_order.go` classifies `"neon"` and Gemini
   `>= 4.0` into `agFamilyGemini4` and ranks `(max)`/`(xhigh)` above `(high)`.
   Added `internal/llm/models/tier.go` (`ModelTierInfo`) and updated
   `internal/tui/components/dialog/models.go` to sort Newer models strictly
   above Legacy models with `"Newer models"` and `"Legacy models"` headers,
   vendor dividers, launch `[YYYY-MM]` tags, capability color accents, and
   concise descriptors.
6. **Agent-count confirmation, Nuclear Option indicators, and local session
   agent audit:** Added in-memory per-session spawn audit log
   `SessionAgentTelemetry` in `internal/llm/agent/subagent_registry.go`
   (surviving `UnregisterSubAgent`), `[SET]` + `[APPLIED] SAVED` banners and
   persistent red `NUCLEAR OPTION ACTIVE` indicator in
   `internal/tui/components/dialog/loadout.go`, `SOLO (NUCLEAR)` and spawned
   agent count chips in `internal/tui/components/core/status.go`, session audit
   rows in `internal/tui/components/dialog/tasks.go`, and `/agents audit|stats|count|status`
   in `internal/tui/tui.go`.
7. **Opus 5.5 thinking-token credit-burn fix & free-tier effort bar parity:**
   Added `Thought bool` (`json:"thought,omitempty"`) to `caPart` in
   `internal/llm/provider/code_assist.go` and routed `thought: true` chunks in
   `antigravity.go` and `code_assist.go` to `EventThinkingDelta` rather than
   content text (eliminating quadratic re-sending of thinking tokens across
   turns). Capped non-streaming `antigravityClient.send()` `maxOutputTokens` to
   `4096`. Skipped background `generateTitle` calls in `agent.go` when Nuclear
   Option is active. Retried `FetchAvailableModels` with `c.ProjectID` / `{}`
   in `antigravity_models.go`. Added `low`/`medium`/`high` `ReasoningEfforts` to
   free `gemini-oauth` (`gemini_codeassist.go`), wired `thinkingConfig.thinkingBudget`
   in `code_assist.go` via `provider.WithEffort`, and generalized
   `models_family.go`/`models.go`/`family.go` so free Code Assist and
   single-effort free Antigravity models render the Effort slider.

### Security posture

No new third-party dependencies, network endpoints, crash reporters, or
telemetry exporters are introduced. Outside-workspace session permission grants
fail closed on filesystem roots (`/`, drive roots) and `$HOME`, and live `/yolo`
keeps irreversible, hard-stop, and outside-root carve-outs gated behind explicit
confirmation. The session sub-agent audit log is strictly local and in-memory.

### Deployment

Install using the release binary, installer, or package for Windows, Linux, or
macOS and verify against `SHA256SUMS-v0.1.167.txt`. `gorilla-opencode --version`
reports `v0.1.167`.

### Verification

Unit tests cover live `/yolo` and `/clear` mid-turn flows, Python/dotted-module
command-family extraction, outside-workspace directory scoping and queued
waiter release, `/compact` fallback/queueing and `/plan` transitions, console
font fallback policy and scrollback layout, model tier classification and
ordering, sub-agent spawn audit persistence and Nuclear Option indicators, and
`caPart.Thought` routing plus free-tier Gemini thinking budget and Effort bar
rendering. See the release evidence for Linux and Windows CI results and
multi-platform release build verification.
