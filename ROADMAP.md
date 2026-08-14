# Roadmap — where this is, and what is left

**Version: 1.0.0 · updated 26-08-14-16-05**

Written for the next instance. Read this before starting work, then read
`CLAUDE.md` (traps and release checklist) and `~/.agents/SETTLED.md` (decisions
already made — apply them, do not re-derive them).

---

## Where we are

`v0.1.82`. The session that produced it started as "get WhatsApp video calling
working" and ended up building the research tool, because the WhatsApp failure
was itself a research failure: two days and two multi-hour engine rebuilds went
into patching a video encoder for a gate that never checked it. The answer —
`SharedArrayBuffer`, one environment variable — was findable in 38 minutes by a
structured multi-role investigation using the same model on the same day.

So the through-line of this release is: **an agent that investigates instead of
guessing, and tells the user what that costs before spending their money.**

---

## What was built

### The research tool — `internal/llm/agent/research-tool.go`

Ten fixed investigator lanes with non-overlapping briefs. The first four are
mandatory (`local`, `prior_art`, `primary_source`, `requirement`) because
overlap is money spent twice and a gap is a wrong answer. `requirement` is the
lane that would have ended the WhatsApp investigation on day one.

Every helper must satisfy an output contract — `## ANSWER / ## FINDINGS /
## CONFIDENCE / ## NOT ESTABLISHED` — which is **checked**, not hoped for, and
every claim carries an evidence tier. A claim that entered as one forum comment
must not leave as a fact.

Three modes, chosen by the user, never by the model:

| mode | what it does |
|---|---|
| `sequential` | one at a time. Same total cost, longer wait |
| `parallel` | all at once (default) |
| `supervised` | parallel, then a second agent audits each **blind** lane |

`ResearchMaxInFlight = ResearchMaxAgents + 1` (11), so a full run never queues.
Raised from 4 on 2026-08-14: a cap of 4 meant a 10-helper run showed 4 in
`/tasks` and looked broken. The 429 risk is real and has not gone away — it is
now *visible* as a red FAILED marker rather than silently halving an
investigation the user deliberately asked for. Research is manually triggered,
so the user is present when it fires.

### Sub-agent states — `internal/llm/agent/subagent_registry.go`

`QUEUED / RUNNING / DONE / FAILED / KILLED`, each with a gorilla + signal marker
**and the state word**. The word is not decoration: the reference machine is a
2012 laptop whose terminal font may render every emoji as an identical box, and
`/tasks` is how a user decides what to kill.

Helpers register **before** they queue for a slot. Previously registration
happened inside `runHelper`, which a goroutine only reaches after *winning* a
slot — so queued helpers were invisible **and unkillable**, and the Nuclear
Option cancelled the running four, released their slots and started the next
four.

### Model following — `config.FollowCoderModel` + `dialog/modelfollow.go`

Background agents always follow the coder model now. This went through three
revisions and the reasoning matters:

1. **Upstream**: move only helpers still sitting on the previous coder model.
2. **Provider-based** (2026-08-14, morning): also move helpers on a *different
   provider*, so a switched-away account stops drawing quota.
3. **Always follow** (2026-08-14, afternoon): both earlier rules reasoned about
   MONEY and were blind to **capability**. A user on Claude Opus 4.6 had
   research silently running on Gemini Flash — same provider, same bill, much
   worse answer. Nobody picks Opus to save money.

Dragging a user's deliberate choice is only acceptable because it is
**perfectly reversible**: every move is itemised on a screen with its cost and
quota consequences, and `r` restores each agent to its own previous model.

### Tool-name repair — `internal/llm/agent/toolname.go`

**Read the header of that file before touching it.** Measured: 30 of 44 tool
calls in a research run failed with `Tool not found: ls<|message|>` — the model
appended a chat-template control token to the function name.

Dispatch strips **one** trailing `<|…|>`, requires a plain identifier, then
demands an **exact** match against that agent's own tool list. No prefix
matching, no fuzzy fallback, no guessing. The dead `strings.HasPrefix` monkey
patch is deleted rather than left as a temptation.

This is guarded by attack-shaped tests and two source-level guards, because
making it fuzzy would turn a typo-fix into a privilege-escalation primitive.

### The cost screen — `internal/tui/components/dialog/research.go`

Rewritten four times against user screenshots. Every figure on it now multiplies
out with a calculator. Bugs fixed:

- the base system prompt was counted **twice**, inflating every dollar ~28%
- supervised session counts were `agents*2`; real counts are 8, 9, 11, 13, 15,
  17, 18 for 4..10, because supervision skips the peeking lanes
- `$0.01 PER MINUTE / PER HOUR: $0` — `%.0f` on the hour
- rate and total rounded independently, so they disagreed on screen
- `8 helpers` printed under a selector reading `Helpers: 4`

---

## What is left

### 1. `esc` does not close `/tasks` — CONFIRMED, NOT FIXED

**Highest priority. Reproduced by the user twice.**

`showPermissions` is checked at `tui.go:1430`; `showTasksDialog` at `tui.go:1479`.
Both swallow every `KeyMsg` and return early, so **permissions wins every
keystroke**. But rendering is the other way round — permissions at ~1745,
`/tasks` at ~1891, and later means on top. **The dialog that owns the keyboard
is drawn underneath the one that does not.**

The user's own description, which matches exactly: *"the tab button will not
allow you to switch anything as long as your /tasks list is wide enough to cover
the buttons of the prompt underneath. You have to wait for some of the tasks to
finish so the tasks window gets narrower and narrower and it exposes the buttons
underneath — and it is only then when TAB begins to work."*

**Fix**: render the permission dialog LAST so z-order matches focus. It is a
blocking question and should own the screen; `esc` then returns to `/tasks`.
Verify with a headless render assertion, not by eye.

### 2. DONE rows vanish from `/tasks`

`runWave` still has `defer UnregisterSubAgent(entry.ID)`, which fires the
instant the goroutine returns — right after the state is set to DONE. The row
exists as DONE for microseconds and is then deleted.

**Fix**: drop the unregister-on-completion, keep terminal rows, purge the whole
call's entries when the research tool returns. `Live()` already keeps them out
of the status count.

**Note the test trap**: `TestFinishedHelpersStopCountingButStayVisible` passes
today because it exercises the registry API directly and never the wave. Fix the
test at the same time or it will keep lying.

### 3. Stale `4` in the mode descriptions

`research.go` still says *"Parallel — up to 4 at a time"*. Hardcoded; should read
`agent.ResearchMaxInFlight` or it goes stale the next time the cap moves.

### 4. Permission prompts are per-helper

Ten helpers wanting `web_search` means up to ten identical blocking prompts.
"Allow for session" fixes it for an experienced user; it is a poor first-run
experience. Undecided — needs the owner's call.

### 5. `ResearchSecondsPerStep = 15.0` is invented

Every per-minute and per-hour figure rests on it. It is labelled ASSUMED on
screen, which is honest but not a substitute. **Fix properly**: record each
helper's real duration when a run finishes and average over past runs.

### 6. Two claims on the cost screen still overstate

- *"WORTH ABOUT N ORDINARY QUESTIONS"* has no token in its derivation — it is
  the step count rescaled.
- *"THIS RUN"* counts helper sessions only. The orchestrator turns on the
  **coder** model are excluded; on a cheap-helper/expensive-coder setup the
  synthesis turn alone can exceed the whole helper cost.

### 7. Unverified

- `Cancel()` on the main agent may not kill sub-agents (flagged by the user's
  own model, never confirmed).
- `sessionCount()` in `research.go` is now unreachable — `costLines` uses
  `RunShape`. Left in place deliberately (it delegates to the same source of
  truth, so it cannot drift). Not deleted, per the never-delete house rule.

---

## How to work on this

- **Verify the artifact, never the exit code.** `${PIPESTATUS[0]}`, and check
  the binary's mtime and `--version` after every build.
- **Tests must be non-vacuous.** Restore the bug and confirm the test fails.
  Several tests in this session passed against the very bug they described —
  one asserted through an unexecuted branch, one ran against an unpriced config
  and checked nothing, one `continue`d past a missing line.
- **TUI work needs headless render assertions.** Never fix a visual bug by
  guessing at a screenshot. `lipgloss.Width()` per line, assert the row count.
- **No line in the frame may be wider than the terminal.** This is the root
  cause of the footer marching down the screen, and emoji are two cells each.
- **Never delete — quarantine to `/home/gorilla/Agents.Work.Trash/`.** A wrong
  fact gets a correction written NEXT TO IT, never substituted for it.
