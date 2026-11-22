# For developers

## Summary

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

## Security posture

No new third-party dependencies, network endpoints, crash reporters, or
telemetry exporters are introduced. Outside-workspace session permission grants
fail closed on filesystem roots (`/`, drive roots) and `$HOME`, and live `/yolo`
keeps irreversible, hard-stop, and outside-root carve-outs gated behind explicit
confirmation. The session sub-agent audit log is strictly local and in-memory.

## Deployment

Install using the release binary, installer, or package for Windows, Linux, or
macOS and verify against `SHA256SUMS-v0.1.167.txt`. `gorilla-opencode --version`
reports `v0.1.167`.

## Verification

Unit tests cover live `/yolo` and `/clear` mid-turn flows, Python/dotted-module
command-family extraction, outside-workspace directory scoping and queued
waiter release, `/compact` fallback/queueing and `/plan` transitions, console
font fallback policy and scrollback layout, model tier classification and
ordering, sub-agent spawn audit persistence and Nuclear Option indicators, and
`caPart.Thought` routing plus free-tier Gemini thinking budget and Effort bar
rendering. See the release evidence for Linux and Windows CI results and
multi-platform release build verification.
