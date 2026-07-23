# Gorilla OpenCode Changelog — July 23, 2026 (evening)

## Version v0.1.31 (Build: 2026-07-23)

### Summary

A TUI reliability release: two long-standing display/input bugs fixed, and a
new **agent-transparency-and-control** surface so you can always see — and
stop — the helper agents the model spawns on your behalf.

Three things landed:

1. **Tables render correctly again** (they were tall, sparse, and misaligned).
2. **Scrolling back through long output no longer lags, jumps, or types random
   `[<65;…M` gibberish into your input box.**
3. **`/tasks`** — a live monitor of running helper agents, with per-agent kill
   and a "kill 'em all" Nuclear Option, plus a live status-bar counter and a
   toast whenever a helper is spawned.

---

## Bug Fixes 🐛

### 1. Markdown tables were rendered tall, sparse, and out of shape

**Problem:**
- Any table a model printed came out mangled: the **header row was blank**, a
  **blank line was inserted between every row**, cell text wrapped to a narrow
  sliver, and the column separators (`│`) stretched down huge empty columns.
- Root cause was in the app's custom markdown theme, not the model's output.
  `generateMarkdownStyleConfig()` set the table's `BlockPrefix` and
  `BlockSuffix` to `"\n"`. Glamour (the markdown renderer) applies a table
  block's prefix/suffix **to every cell**, so each cell got leading/trailing
  newlines — blanking the header and double-spacing every row.

**Fix:**
- Removed the two `"\n"` affixes from the `Table` style (left empty, matching
  the upstream default). Tables now render tight and aligned, header present,
  single-spaced, columns sized to content.
- Verified empirically: reproduced the breakage by *adding* those two lines to
  glamour's default style, and confirmed the fix against the app's real
  renderer config at terminal width.

**Files:** `internal/tui/styles/markdown.go`.

**Human track:** the model wasn't printing broken tables — the app was breaking
them on the way to your screen. Fixed; tables look like tables now.

---

### 2. Scroll-back lagged, jumped, and leaked mouse escape codes into the editor

**Problem:**
- Scrolling back through a long reply made the UI stutter and jump, the app got
  sluggish, and **random numbers like `[<65;119;22M`** appeared in the input box
  without typing them.
- Those "random numbers" are **SGR mouse-tracking escape codes** (`ESC[<b;x;yM`;
  `64/65` = wheel, `32` = drag). The app runs with `WithMouseCellMotion()`, so a
  mouse **drag** (e.g. trying to select text without holding Shift) emits a flood
  of motion events. Every event was routed through the *entire* status-bar +
  every-dialog + page update chain and forced a full re-render. Under that flood
  the event loop saturated, Bubble Tea's stdin parser fell behind, and
  half-parsed mouse sequences leaked into the focused text area — while the view
  stuttered.

**Fix:**
- Added a top-level `tea.MouseMsg` handler that **drops all non-wheel mouse
  events** and routes only wheel events to the page. Verified nothing consumed
  motion/press/release events (only `list.go` reads mouse; no bubblezone click
  handling), so this is behaviour-preserving. Wheel scrolling still works; the
  flood — and the leak — are gone.

**Files:** `internal/tui/tui.go`.

**Human track:** scrolling up to re-read a long answer used to make the app go
haywire and dribble gibberish into your prompt. That's fixed. (Selecting text to
copy still needs Shift held — that part is unchanged.)

**Related note — `Ctrl+A` only "selects" part of the session:** that's a
different thing and expected. The TUI runs in the terminal's *alternate screen*
(like `vim`/`less`), which has no scrollback, so your terminal's own select-all
only sees what's currently on screen. To capture the **whole** conversation, use
the built-in **`export`** command (`/export`) — it writes the entire session to
`opencode-<name>-<timestamp>.md`.

---

## New Features ✨

### 3. `/tasks` — see and kill the agents working for you

**Problem / policy:**
- The Gorilla policy is that you must always be able to **see** what agents are
  running on your behalf and **stop** them. But the `agent` tool spawned each
  helper synchronously inside a throwaway agent instance with its own private
  request map — so the main agent had **no way to list or cancel** a running
  helper. There was no visibility and no kill switch.

**Fix — three parts:**

1. **A shared sub-agent registry** (`internal/llm/agent/subagent_registry.go`,
   new). Every helper now registers with its own cancel function and a short
   handle (`a1`, `a2`, …). Exposes `ListSubAgents`, `ActiveSubAgentCount`,
   `KillSubAgent(id)`, `KillAllSubAgents()`, and a pubsub broker for live
   spawn/exit events. The `agent` tool runs each helper under a cancelable
   context and registers/unregisters it — so **killing actually cancels the
   running helper** (the cancel unblocks the tool's wait, and the model receives
   a cancellation result instead of hanging).

2. **The `/tasks` monitor** (`internal/tui/components/dialog/tasks.go`, new;
   aliases `/task`, `/agents`, `/kill`). Lists each live helper with its handle,
   elapsed time, and task prompt.
   - `↑ ↓` pick · `enter` / `x` **kill the selected helper** · `esc` close.
   - `X` (or `Ctrl+X`) is the **Nuclear Option**:
     *"☢ Killed 'em all — N helper(s), their tasks, and the horse they rode in
     on."*

3. **Always-on transparency signals.** A live **`🦍 N helper(s) · /tasks`** badge
   appears in the status bar whenever helpers are running, and a toast fires the
   moment one spawns: *"🦍 helper a3 spawned — <task> (/tasks to view or kill)."*
   Spawn/exit events are wired into the TUI's subscription loop so the badge and
   the `/tasks` list stay live.

**Tests:** `internal/llm/agent/subagent_registry_test.go` — kill cancels the
helper's context; double-kill is a safe no-op; the Nuclear Option cancels all
and is idempotent; list ordering is stable. Passes under `-race`.

**Files:** `internal/llm/agent/subagent_registry.go` (+ test),
`internal/llm/agent/agent-tool.go`, `internal/tui/components/dialog/tasks.go`,
`internal/tui/tui.go`, `internal/tui/components/core/status.go`,
`cmd/root.go`.

**Human track:** if the AI ever spins up "helper" agents to go do sub-tasks, you
now see a 🦍 counter light up in the bottom bar and a little pop-up telling you.
Type `/tasks` to see exactly what each helper is doing, and either kill one, or
hit `X` to kill every last one of them at once.

**How this relates to `/context`'s "Nuclear":** `/context` already has a
*Gorilla Nuclear* dial that **prevents** helpers from being spawned at all (a
budget leash). This new `/tasks` Nuclear Option **terminates** helpers that are
already running. Prevention vs. termination — two different switches that work
together.

---

## Notes

- Version stamped `v0.1.31`; official Linux binaries and `.deb`/`.rpm` packages
  are built and published by the `release` CI workflow (goreleaser) on tag push.
- Working folder tidied: changelogs now live in `Changelogs/`.
