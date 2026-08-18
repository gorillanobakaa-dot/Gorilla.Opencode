# Gorilla OpenCode v0.1.93 — picking the work back up

**Everything you need to judge this release is on this page.** Not behind a link,
not in a wiki, not "see the docs" — the complete plain-language explanation and
the complete technical one are printed below, in full, because
[the philosophy this project is built on](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.93/PHILOSOPHY.md) holds that
publishing something a reader cannot reach is transparency in theory and a
closed door in practice.

> *"Open source gave the world the recipe. It forgot to teach people how to cook."*

---

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.93_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.93_amd64.deb` |
| `gorilla-opencode-0.1.93-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer — unpack and run |
| `SHA256SUMS-v0.1.93.txt` | Check what you downloaded is what we built: `sha256sum -c` |

Use `apt`, not `dpkg -i` — the package depends on `lynx` and `dpkg` resolves
nothing. **And if the program is already running, quit and restart it:** a
running process keeps the binary it started with.

---

## In one paragraph

`/resume` picks up work that stopped — the power went, the connection dropped,
the model ran out of room — or hands it to a different model. It does not reload
the old conversation: it writes a brief (everything you asked for word for word,
what was done, what failed, where it stopped, and what it does **not** know) and
starts fresh with just that. The brief is built by the program itself, so it
costs nothing and cannot fail the way the original did.

---

<!-- Version: 1.0.0 · updated 26-08-18-12-20 -->
# Gorilla Opencode v0.1.93 — picking the work back up

Released 18 August 2026, one commit after v0.1.92. That release made a past
conversation findable and reopenable. This one adds the operation that reopening
cannot do: handing unfinished work to a model — the same one, or a different one
— without carrying the whole conversation across.

This is the same document in two registers. The plain-language track assumes no
prior knowledge and omits nothing significant. The developer track is precise
enough to audit and rebuild from. Both end with a claim-source table saying which
statements this release measured and which it inferred — see
[PHILOSOPHY.md](../PHILOSOPHY.md) for why.

---

## Plain-language track

### Why reopening is not enough

v0.1.92 shipped `/sessions`, which lists every conversation you have ever had and
lets you reopen one. For a short conversation that is exactly right: every
message goes back in and the model carries on.

For a long job it is the wrong tool, and it fails in a specific way. A real
working session on this machine held 275 messages and 2.6 MB. Putting that back
into a model with a small context window does not resume the work — it
reproduces the failure that stopped it. **The bigger the job, the more certainly
reopening it fails.** That is backwards, because a long job is precisely the kind
that gets interrupted.

There is also a case reopening cannot serve at all: giving the work to a
*different* model. A model that was not there has no way to tell a finished job
from an abandoned one.

### What `/resume` does

Pick a conversation and press Ctrl+R. The program writes a short brief and starts
a **fresh** conversation containing only that brief:

- **Everything you asked for, word for word, in order.** Including your
  corrections. Those are the most valuable lines in any session, because they
  record where the previous attempt went wrong, and the brief says plainly that
  where instructions conflict the later one wins.
- **What the previous attempt did** — which files it wrote to or edited, which
  commands it ran.
- **What went wrong** — every tool failure, named. If the job used research
  helpers, each failure is attributed to the helper that hit it.
- **Where it stopped** — the last thing that was said.
- **What the brief does not know.**

That last section is short and it is the most important part of the whole
feature:

> - Whether any of the work above is **correct**. Nothing here was verified; a
>   file being written is not the file being right.
> - Whether the work is **finished**. A session can stop because it was done,
>   because the power went, or because it hit a wall. The record does not say
>   which.

Without it, a brief reads as a status report, and "someone was working on this"
quietly becomes "this is done". That is how half-finished work gets built on or
committed.

The brief is written by the program itself, in ordinary code. No AI is asked to
work out what happened — which matters, because that is exactly the step that
already failed once.

### Proven on a real run

Driven against a 106-message research conversation with 16 helper sessions, the
brief came out carrying the goal verbatim and seven distinct helper failures:
three searches that found nothing, two timeouts, a path that did not exist, and a
403 from a website — each attributed to the lane that hit it. The whole thing
fits in a fraction of the window the original needed.

### The bug that cost the most time

The edit that added the Ctrl+R handler targeted three tabs of indentation where
the file used two. The replacement matched nothing, changed nothing, and the
script reported success. So the help line, the command list and the release notes
all advertised a key that had no code behind it.

Four rounds of live testing then appeared to rule out Ctrl+R, F5, Shift+Tab and
Insert. None of them had ever been bound. The signal was there and was missed:
when four independent candidates fail in exactly the same way, the fault is in
the path they share, not in the candidates.

There is now a test that asserts every advertised action actually emits its
message. It catches this in a second, and it would have caught it before the
first live test.

---

## Developer track

### `internal/export/handoff.go`

`Handoff(sess, msgs, branches, budgetChars) (string, HandoffStats)` — pure
function over stored data, no model, no I/O.

Section order is deliberate:

1. **Instructions verbatim.** Paraphrase is how a resumed session starts solving
   a different problem. The brief states that later instructions override
   earlier ones.
2. **What was done** — `write`/`edit`/`patch` targets and `bash` commands, pulled
   from the stored tool-call inputs, deduplicated and counted.
3. **What failed** — tool results with `IsError`, plus abnormal finish reasons.
   Placed *before* "where it stopped", so a hopeful last paragraph is not the
   last thing read.
4. **Where it stopped** — the final assistant text, or an explicit statement that
   the session was cut off mid-turn.
5. **What is not known.**

Helper sessions contribute file writes and failures, attributed by lane title,
but never instructions: a helper prompt is generated by this program, and
treating it as a user instruction would put machine-authored text at the top of
the brief. `TestHelperSessionsContributeWorkAndFailuresButNotInstructions` pins
that.

Truncation cuts the middle. The instruction block and the closing guidance are
never dropped, and the brief says when it has been trimmed.
`TestTruncationKeepsTheGoalAndTheGuidance` was proven to fail when truncation
cut from the end instead.

Budget is half the selected model's context window, at the project's four-
characters-per-token convention.

### Keys

`enter` reopen · `ctrl+r` resume · `ctrl+e` export · `DEL` erase · `tab` sort ·
`esc` close. `ctrl+r` is absent from this app's global keymap (`ctrl+l c _ h s k
f o t y`) and is bound only in the chat editor, which a modal shields — the same
position `ctrl+e` occupies, and `ctrl+e` arrives. Verified live rather than
assumed, after v0.1.92's key hunt.

### Tests added

- `TestEveryAdvertisedActionIsActuallyWired` — every action key emits its message
  and appears in the help line.
- Seven handoff tests covering instruction fidelity and order, work and failure
  reporting, the honesty clauses, the cut-off-mid-turn case, helper attribution,
  and truncation. Two proven non-vacuous by reverting the behaviour they assert.

---

## Claim sources

| Claim | Source |
|---|---|
| Brief from a 106-message run with 16 helper sessions; seven failures attributed by lane | **stated in input** — driven through the real TUI against a copied store, 2026-08-18 |
| A real session held 275 messages and 2.6 MB | **stated in input** — measured 2026-08-18 |
| `ctrl+r` reaches the dialog; `enter`, `tab`, `DEL`, `ctrl+e`, arrows also do | **stated in input** — driven live |
| F5, Shift+Tab and Insert reach the dialog | **not established** — they were tested against a build in which no key was bound, so nothing was learned about them either way |
| Half the context window is the right budget for a brief | **model inference** — not measured; chosen to leave room for the work itself |


---

# Appendix A — sessions, resuming and storage, in full


*Dual-track. The first half is the complete explanation in plain language — not a
simplification. The second half is the developer record, precise enough to
rebuild from. Every number here was measured on 18 August 2026 and says where it
came from.*

---

## Part one: in plain language

### The problem this solves

The program keeps every conversation you have ever had. Until now it could
barely show them to you, and it could not delete them at all.

That is a bad combination anywhere. It is a serious one on the machines this
project is built for.

**The power goes.** A fifteen-year-old laptop has a battery measured in minutes,
sometimes seconds, so it lives plugged in. When the electricity drops the session
stops mid-sentence — and sometimes the electricity does not come back for days.
Everything the program offered before assumed you were still sitting in the
conversation you cared about: the session switcher showed titles and nothing
else, and "save this conversation" could only ever save the one currently open.
Neither is any use the following week.

**The disk is small.** Not a 500 GB disk — that is the lucky kid. Early
Chromebooks, and devices from before Chromebooks, running on raw SLC NAND,
CompactFlash, PATA flash or eMMC, where 1 to 2 GB free is the whole budget. The
program accumulated conversations forever with no way to see what they cost and
no way to remove them.

### What `/sessions` does

It lists every conversation you have ever had, newest first, with the date, how
many messages it holds, and **how much space it is taking up**. The line at the
top tells you the total.

**Type to search.** It searches inside the messages, not just the titles. This
matters more than it sounds: titles are generated automatically, and "New
Session" is a real title this program writes. Weeks later you do not remember
what the summary called that day — you remember the error you were chasing.
Searching for that error finds the conversation.

**Enter reopens one**, exactly where it stopped. The power cut ends the session;
it does not end the conversation.

**Ctrl+E saves one to a file** — the whole thing. Every message with its time,
the model's reasoning, and every tool it ran with the input it was given and the
result that came back, including the failures. That is deliberate, and it is the
reason the feature exists in this shape: when something goes wrong you need to be
able to work out *how*. How did it end up publishing something private. How did
it end up deleting something. A record of only the visible chat cannot answer
either question.

If the conversation was a research run, the helper sessions come too. A run on
this machine listed 275 messages and, before this was fixed, exported 14 of them
— the other 261 were in twenty-three helper sessions, which is where all the
reasoning and every tool call lived. The export now carries all of it.

**Ctrl+R picks the work back up.** This is the one to reach for when the job was
not finished — the power went, the connection dropped, the model ran out of
room, or you want a *different* model to take over.

It is not the same as reopening. Reopening loads every message back in, which is
right for a short chat and wrong for a long job — and a long job is exactly what
gets interrupted. A real session here held 275 messages; putting that back into
a small model reproduces the failure that stopped the work in the first place.

Instead the program writes a short brief and starts a **fresh** conversation
with just that:

- **everything you asked for, word for word, in order** — including your
  corrections, which are the most valuable lines in the session, because they
  record where the previous attempt went wrong;
- which files were changed and which commands were run;
- what went wrong, attributed to the helper that hit it;
- where it stopped;
- and **what the brief does not know**: whether any of the work was correct, and
  whether it was finished.

That last part is what makes it safe to hand to a different model. A brief that
reads as settled fact turns "someone was working on this" into "this is done",
and that is how half-finished work gets built on.

The brief is built by the program itself, in ordinary code. No AI is asked to
work out what happened — which matters, because that is precisely the step that
already failed once. Driven live against a real 106-message research run with 16
helper sessions, it produced a brief naming the goal verbatim and seven distinct
lane failures (timeouts, a 403, three empty searches).

**Delete erases one for good** — the conversation, the helper sessions it
spawned, and everything in them — and then **returns the space to your disk**,
and tells you how much came back.

That last part is the whole point, and it is where most software quietly lies.

### Why "delete" usually frees nothing

The conversations live in a SQLite database — a single file. When you delete rows
from it, SQLite marks that space as free *for its own later use*. The file on
your disk does not get smaller. Not by one byte.

Measured, in the project's own test:

| | Size of the database file |
|---|---|
| With 40 messages of about 25 KB each | 1,073,152 bytes |
| **After deleting every one of them** | **1,073,152 bytes** — unchanged |
| After rebuilding the file | 65,536 bytes |

On a machine with 1 GB free, an erase that returns nothing is not an erase. So
this rebuilds the file every time, and reports the difference the *filesystem*
shows rather than what the database claims.

Measured live on a real store, erasing one research run:

- Before: **5,575,048 bytes**
- After: **2,351,104 bytes**
- Reported on screen: **"Erased 24 sessions · 3.1 MB returned to the disk"**

Twenty-four sessions, because one conversation plus the twenty-three helpers it
had spawned. Those helpers were invisible before, and they held most of the
bytes.

### The number at the top is the honest one

The header shows what the database **occupies on disk**, not what the messages add
up to. On this machine those were 9.4 MB and 4.6 MB — because a write-ahead log
had grown to 4.3 MB and nothing had ever truncated it. Nearly half the space was
in a file no one was counting. The number you are shown is the one your disk
sees.

### Two safety rules

- Erasing takes **two deliberate keypresses**, and anything that is not `y`
  cancels — including an arrow key you were already holding down.
- Typing goes to the search box and can **never** trigger an action. Someone
  searching for the word "delete" spells d-e-l-e-t-e, and none of those letters
  may erase anything.

---

## Part two: the developer record

### Surfaces

| Command | Scope |
|---|---|
| `/export` | the conversation currently open; you choose folder and filename |
| `/sessions` (`/history`) | any conversation ever held; search, reopen, resume, export, erase |
| `/resume` (`/continue`, `/handoff`) | the same list, opened to hand stalled work over |

Keys inside `/sessions`: `↑↓` move, `enter` reopen, `ctrl+r` resume, `ctrl+e`
export, `DEL` erase, `tab` toggle date/size sort, `esc` close. Printable
characters go to the search field.

### The handoff brief — `internal/export/handoff.go`

`Handoff(sess, msgs, branches, budgetChars)` distils a session in pure Go and
returns the brief plus stats. No model is involved: a resume that needed an LLM
to work out what had happened would be a second chance to fail at the step that
already failed.

Sections, in this order and for these reasons:

1. **Instructions, verbatim.** Paraphrasing is how a resumed session quietly
   starts solving a different problem. Later instructions are flagged as
   overriding earlier ones.
2. **What was done** — files written or edited, commands run, deduplicated.
3. **What failed** — every tool error and abnormal ending, placed *before* the
   last thing said, so it is not read as already resolved.
4. **Where it stopped.**
5. **What is not known** — correctness, completeness, and anything decided but
   never acted on.

Helper sessions contribute their file writes and failures, attributed by lane
name, but never their prompts: a helper's prompt is generated by this program,
and treating it as an instruction would put machine-authored text at the top of
the brief as though the user had written it.

Truncation cuts the middle. The instructions at the top and the guidance at the
bottom are never dropped — they are the two parts that decide whether the next
model does the right thing — and the brief says when it has been trimmed.

### Why those keys, and not the obvious ones

The first implementation used `ctrl+s` (sort), `ctrl+d` (erase) and `ctrl+e`
(export). Two of the three never arrived, and both failures were found by driving
the real binary rather than by any test:

- **`ctrl+s`** is XOFF. On a terminal that has not disabled software flow control
  it freezes the display. This program *also* binds it globally to the old
  session switcher, and handles it before any dialog sees a key.
- **`ctrl+d`** is EOF.

`internal/tui/provider_escape_test.go` had recorded this rule before the screen
existed — ctrl+z suspend, ctrl+d EOF, ctrl+q/s flow control, ctrl+b tmux prefix —
and the app already binds `ctrl+a c d e f h k l n o r s t u x y`. There was no
free control key. `DEL` and `tab` are unbound and unreserved.
`TestManagerActionsAvoidReservedKeys` pins it.

### Storage layer — `internal/db/session_storage.go`

Hand-written, deliberately not sqlc-generated, so a regeneration cannot drop it.

- `SessionStorageFor` / `AllSessionStorage` — bytes and message counts per
  conversation, **including its helper sessions**, since that is what answers
  "what do I get back if I delete this?". One query for the whole list rather
  than one per row: on eMMC a query per session is felt when opening a screen.
- `StoreTotals` — conversations, helpers, messages, live content bytes.
- `DeleteSessionTree` — helper sessions **first**, then the parent. Sessions have
  no foreign key on `parent_session_id`, so SQLite will not cascade to them and a
  plain delete strands seventeen orphans per supervised research run. The order
  matters: interrupted between the two statements leaves helpers gone and the
  parent still listed — visible and deletable again. The reverse order would
  leave a deleted parent with orphans nothing can name, which hides the bytes
  forever.
- `Reclaim` — `VACUUM` then `PRAGMA wal_checkpoint(TRUNCATE)`. VACUUM alone does
  not touch the write-ahead log, and the log was 44% of the total on this
  machine. VACUUM needs temporary room roughly the size of the database, which is
  not a given on a full device, so the error is returned rather than swallowed
  and the UI says the space was *not* returned.
- `SearchSessions` — `LIKE` against title and `messages.parts`, lowercased. Not
  FTS: a full-text index is a second copy of every message on a device with 1 GB
  free. Measured on 578 messages / 4.77 MB the scan is imperceptible.

### Export — `internal/export/`

`WriteSessionTree(dir, sess, msgs, branches, now)` renders the conversation and
appends every helper session in full, reusing the same message renderer, and
returns the total message count so the UI cannot claim more than it wrote. Files
are `0o600`: a transcript holds whatever was discussed, and these machines are
shared. Existing files are never overwritten — a taken name gets a numbered
suffix rather than an error, because telling someone whose power just came back
that their export "already exists" is the wrong answer.

Destination is `~/Documents/Gorilla-Session-Exports/`, derived from the user's
home and never hardcoded, falling back to the home directory and then to the
config base. Never the working folder: it is often a git repository.

### Measurements

| What | Value | Where from |
|---|---|---|
| Store on this machine | 65 sessions, 578 messages | `sqlite3` count, 2026-08-18 |
| Message parts | 4,774,160 bytes | `SUM(LENGTH(parts))` |
| Database file | 5,480,448 bytes | `ls -la` |
| Write-ahead log | 4,313,672 bytes | `ls -la` |
| Delete without VACUUM | 1,073,152 → 1,073,152 bytes | `TestReclaimActuallyShrinksTheFileOnDisk` |
| After VACUUM | 65,536 bytes | same test |
| Live erase of one research run | 5,575,048 → 2,351,104 bytes | `du -sb`, driven through the real TUI |
| Research run exported | 275 messages, 23 helper sessions, 2,675,779 bytes | live export, `ls -la` |
| — containing | 238 tool calls, 119 reasoning blocks | `grep -c` on the artifact |

Every figure above is **stated in input** — measured on this machine on
2026-08-18 by the command named. Nothing here is estimated.

### Bugs found while building it, all by driving the real binary

1. **Export dropped 95% of a research run.** 14 messages written where the list
   said 275; the rest were in helper sessions. Fixed by `WriteSessionTree`.
2. **The size column vanished** whenever a title was long — and titles are
   auto-generated, so almost always. The title is now truncated, never the size.
3. **A row with double-width characters wrapped.** `中文` is two runes and four
   columns; a real title in this store contains `U+FFFC`. Truncation was counting
   runes. It now measures display columns. The symptom is *height*, not width —
   lipgloss wraps, so every resulting line measures under the frame and a width
   assertion sails straight past it.
4. **The search box did nothing when typed into at speed.** bubbletea coalesces
   fast input into one `KeyMsg` carrying several runes; only single-rune messages
   were accepted. Pasting always arrives that way.
5. **Two of three action keys never arrived** — see the key section above.
6. **The dialog drew 32 rows in a 24-row window.** Caught by
   `TestDialogFramesNeverExceedTheTerminal` before it reached a screen.
7. **A key was advertised that was never wired.** The scripted edit adding the
   resume `case` targeted three tabs of indentation where the file used two.
   `str.replace` found nothing, changed nothing, and the script exited 0 — so
   the help line, the command registry and the changelog all named a key with no
   handler behind it. Four live tests then appeared to "disprove" `ctrl+r`, F5,
   shift+Tab and Insert, none of which had ever been bound. The tell was that
   four independent candidates failed identically while `ctrl+e`, in the same
   switch, worked: when several candidates fail the same way, the fault is in
   the shared path. `TestEveryAdvertisedActionIsActuallyWired` now asserts each
   action emits its message, and catches it in a second.


---

# Appendix B — every command in the program


Generated from `internal/commands/registry.go` by `go run ./cmd/commands-doc`.
Do not edit by hand — a test compares this file against the registry, so a
command cannot be added without appearing here.

You can read the same list inside the program with **`/help`**, where the
explanation for whichever command you have highlighted appears underneath it.
Type a command by starting a message with `/`.

**Nothing here costs money except where it says so.** Two commands spend real
tokens on your behalf — `/research` and `/osint` — and both say what they will
cost before they start. Everything else is local.

## At a glance

| Type this | What happens |
|---|---|
| `/clear` · `/new` | Start a fresh conversation. |
| `/plain` · `/copy` `/copyable` | Switch to the interface you can select and copy. |
| `/resume` · `/continue` `/handoff` | Pick up work that stopped, or work another model started. |
| `/sessions` · `/history` | Every past conversation: search, reopen, save, erase. |
| `/export` | Save this conversation to a file. |
| `/compact` · `/summarize` `/summarise` | Squeeze the conversation down so it keeps working. |
| `/cd [folder]` | Switch to working in one folder. |
| `/add-dir` · `/adddir` `/dirs` `/roots` | Work in more than one folder at once. |
| `/init` | Write the project notes file the AI reads first. |
| `/model` · `/models` | Choose which AI answers you. |
| `/connect` · `/connections` | Add or manage your AI accounts and keys. |
| `/providers` · `/provider` `/switch` | Switch to a different AI provider. |
| `/login` | Sign in with your Google account. |
| `/logout` | Sign out of your Google account. |
| `/usage` | Show your quota and balances — how many bananas are left. |
| `/context` · `/loadout` `/tokens` | Turn features off to spend less. |
| `/settings` · `/config` `/prefs` | Every option, what it accepts, and its default. |
| `/prompts` · `/prompt` | Read or change the AI's standing instructions. |
| `/reset` · `/defaults` | Put things back the way they shipped. |
| `/research <question>` | Send helper agents to investigate, each on one angle. |
| `/osint <question>` · `/dossier` | The serious one. Professional dossier. Burns real money. |
| `/yolo` · `/auto` `/autopilot` `/goal` | Approve everything for this conversation. No more prompts. |
| `/tasks` · `/task` `/agents` `/kill` | See and stop background helpers. |
| `/help` · `/commands` `/?` | This list. |

## Your conversation

### `/clear`

*Also: `/new`*

**Start a fresh conversation.**

The AI forgets everything said so far. Use this when you move to a different task — a long conversation costs more on every message, because the whole history is sent each time.

### `/plain`

*Also: `/copy`, `/copyable`*

**Switch to the interface you can select and copy.**

This interface draws on a screen your terminal keeps no history of, which is why Ctrl+A selects nothing here. Plain mode writes ordinary text instead, so you can select, copy and search the whole conversation with your terminal's own keys. It has fewer commands. This takes effect next time you start the program — the current screen is already running. Switch back in /settings, or right-click the desktop icon for a one-off.

### `/resume`

*Also: `/continue`, `/handoff`*

**Pick up work that stopped, or work another model started.**

For when the job is not finished: the power went, the connection dropped, the model ran out of room, or you want a different model to take over. It opens the same list as /sessions — press Ctrl+R on the one you want.

This is NOT the same as reopening the conversation. Reopening loads every message back in, which is right for a short chat and wrong for a long job — and a long job is exactly what gets interrupted. Putting a thousand messages back into a small model is what stopped the work in the first place.

Instead it writes a short brief and starts a FRESH conversation with just that: everything you asked for, word for word, in order; which files were changed and which commands were run; what went wrong; and where it stopped. The brief is built by the program itself, so it costs nothing and cannot fail the way the original did.

It also says plainly what it does NOT know — whether any of the work was correct, and whether it was finished. That matters most when you hand it to a different model, which has no way to tell a finished job from an abandoned one and would otherwise assume the best.

### `/sessions`

*Also: `/history`*

**Every past conversation: search, reopen, save, erase.**

The one to reach for when a session ended without you — the power went, the connection dropped, the machine was closed. It lists every conversation you have ever had, newest first, with the date, how many messages, and how much space it is taking up.

Type to search. It looks inside the messages as well as the titles, because titles are generated and are often useless weeks later — you remember the error you were chasing, not what the summary called that day.

Enter reopens a conversation exactly where it stopped. Ctrl+E saves it to a file — the whole thing: every message with its time, the model's reasoning, and every tool it ran with the result that came back, including the failures. That is what lets you work out afterwards how something ended up published, or deleted.

Ctrl+D erases one for good, along with the helper sessions it spawned, and returns the space to your disk — really returns it, and tells you how much came back. Deleting alone frees nothing on this kind of database; the file only shrinks when it is rebuilt, which is why this reports the actual before-and-after. Ctrl+S sorts by size, so the conversations worth deleting are the ones at the top.

### `/export`

**Save this conversation to a file.**

Asks you which folder and what to call it, then writes the whole session out as text: every message with its date and time, how far into the session it happened, which model answered, the model's reasoning, and every tool it ran with the result that came back — including the ones that failed. Use it when you need to know exactly what happened and when.

This one saves the conversation you are in, and lets you name the file. To save a conversation you have LEFT — after a power cut, or from last week — use /sessions instead, which can also reach the helper sessions a research run spawned.

### `/compact`

*Also: `/summarize`, `/summarise`*

**Squeeze the conversation down so it keeps working.**

Every message you send carries the whole conversation with it, and each model has a limit on how much it can hold. Approach that limit and answers get worse, then stop. This writes a summary of everything so far and continues from that instead, so the thread survives while the bulk goes.

Use it when a long session starts to drift, or before starting a big job in an old conversation. It costs one model call to write the summary. Models with small windows need this often — the status bar shows how full you are. It also runs by itself at 95% full if you leave that setting on in /settings.

## Which files the AI can see

### `/cd [folder]`

**Switch to working in one folder.**

This is the important one. The AI searches and reads inside your working folder, so pointing it at one project instead of your whole home folder is the difference between a handful of files and a million. Fewer files means faster answers and far less of your quota spent. Typing it with no folder opens a chooser.

### `/add-dir`

*Also: `/adddir`, `/dirs`, `/roots`*

**Work in more than one folder at once.**

Adds a second (or third) folder alongside your main one — useful when a change spans two projects. Each folder you add is more for the AI to search, so add only what you need. To move to a single folder instead of adding one, use /cd.

### `/init`

**Write the project notes file the AI reads first.**

Looks through the project and writes a short file describing how to build, test and work in it, in the house style of this codebase. Every future conversation in this folder reads that file before anything else, so the AI starts knowing your conventions instead of guessing at them. Run it once per project, and again after big changes.

## Models and accounts

### `/model`

*Also: `/models`*

**Choose which AI answers you.**

Bigger models are better at hard problems and cost more; small ones are cheap and fast. Models running on your own machine cost nothing to use. Each is listed with the connection it comes from.

### `/connect`

*Also: `/connections`*

**Add or manage your AI accounts and keys.**

Where you paste an API key, add a local server such as Ollama or NVIDIA, or turn a connection off without deleting it. Adding a connection makes its models appear in /model. The list shows the servers you have added as well as the ones on offer; press d to remove one of yours for good, or space to just switch it off.

### `/providers`

*Also: `/provider`, `/switch`*

**Switch to a different AI provider.**

Reopens the same picker you saw when the app started, with the free options marked. Use it when the provider you chose does not work — a key refused, a model not included in your plan — instead of quitting and starting again. Esc leaves everything as it is.

### `/login`

**Sign in with your Google account.**

Opens your browser. Lets you use Google's models through your account instead of pasting an API key.

### `/logout`

**Sign out of your Google account.**

Removes the stored sign-in. Any API keys you typed are untouched.

### `/usage`

**Show your quota and balances — how many bananas are left.**

Shows what you have left to spend, in plain words. If you signed in with the Antigravity free tier: how much of your weekly allowance remains — Gemini has a separate pool from Claude and GPT-OSS — and when each resets. If you have a DeepSeek or OpenRouter key: your remaining balance there too. A one-line summary also appears on its own at the start of each session.

## Cost, speed and behaviour

### `/context`

*Also: `/loadout`, `/tokens`*

**Turn features off to spend less.**

Everything the AI can do is described to it on every single message, and you pay for that description each time. Switching off what you are not using makes every message cheaper. This is also where you turn language servers off — press L for all of them at once, or pick them one by one.

### `/settings`

*Also: `/config`, `/prefs`*

**Every option, what it accepts, and its default.**

One list of every setting with a plain-language description, the range it accepts and what it shipped as, so you can always get back to a known state.

### `/prompts`

*Also: `/prompt`*

**Read or change the AI's standing instructions.**

The instructions the AI is given before it sees your message — how careful to be, how to report what it did. You can switch sections off or rewrite them. Advanced: changing these changes how the AI behaves everywhere.

### `/reset`

*Also: `/defaults`*

**Put things back the way they shipped.**

Undoes your changes, in whichever area you pick — settings, instructions, or feature switches. Use this when something is behaving oddly and you no longer remember what you changed.

## Background helpers

### `/research <question>`

**Send helper agents to investigate, each on one angle.**

The everyday investigation tool: four to ten helpers, each given ONE angle, collecting with the same intelligence-cycle discipline as /osint but in a single pass. A verifier attacks their conclusions. Each helper is a full model session, so the dialog shows the cost before anything starts. Worth it when being wrong is expensive; waste when a single search would answer. For the full professional dossier — rounds, graded sources, the works — see /osint.

### `/osint <question>`

*Also: `/dossier`*

**The serious one. Professional dossier. Burns real money.**

A professional intelligence assessment, not a chat answer: plans your question into sub-questions, collects from hundreds of free primary sources (scholarly APIs, SEC filings, World Bank, humanitarian data, global news), grades every claim on two axes like a real intelligence shop, hunts its own gaps, and tells you plainly what it could NOT establish. OFF by default — arm it in /context. Every run starts with a warning showing the burn rate in money, because 4-10 helpers is 4-10 full model sessions. Type /osint alone for the full explanation page.

/osint --recover writes up a run that collected its findings but never produced the dossier — the usual outcome when a connection drops or the model runs out of room at the very last step. It costs nothing to look: the findings are already on disk and in the local store, and it lists every past run so you can pick one. The write-up happens in a fresh conversation carrying only those findings, which is exactly why it succeeds where the original run ran out of room. Nothing is collected again and no helpers are sent out.

### `/yolo`

*Also: `/auto`, `/autopilot`, `/goal`*

**Approve everything for this conversation. No more prompts.**

Normally the program stops and asks before it edits a file, runs a command, or reaches the internet. This turns that off for the conversation you are in: every tool call is approved automatically, including every research helper — which is the point, because a ten-helper run otherwise asks you the same question ten times.

What you are handing over: file edits, shell commands and web access, unattended. Use it when you have told the agent to get on with a job and you do not want to babysit it. Do not use it in a folder you cannot afford to have changed.

It lasts only as long as this conversation and is never written to disk, so it cannot silently follow you into tomorrow. Type /yolo again to turn it off. /tasks still stops helpers at any time.

### `/tasks`

*Also: `/task`, `/agents`, `/kill`*

**See and stop background helpers.**

The AI can start helpers to work on parts of a job. Each one costs quota of its own, so this is where you check what is running and stop anything you do not want.

## Help

### `/help`

*Also: `/commands`, `/?`*

**This list.**

Every command, what it does, and what it costs or changes.

---

*This page is generated. If a command is missing from it, that is a bug in
the program rather than in the documentation — the registry and this file are
held together by a test.*
