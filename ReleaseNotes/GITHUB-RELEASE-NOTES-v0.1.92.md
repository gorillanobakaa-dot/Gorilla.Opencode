# Gorilla OpenCode v0.1.92 — the conversation you lost, and the disk space you never got back

**Everything you need to judge this release is on this page.** Not behind a link,
not in a wiki, not "see the docs" — the complete plain-language explanation and
the complete technical one are printed below, in full, because
[the philosophy this project is built on](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.92/PHILOSOPHY.md) holds that
publishing something a reader cannot reach is transparency in theory and a
closed door in practice.

> *"Open source gave the world the recipe. It forgot to teach people how to cook."*

---

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.92_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.92_amd64.deb` |
| `gorilla-opencode-0.1.92-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer — unpack and run |
| `SHA256SUMS-v0.1.92.txt` | Check what you downloaded is what we built: `sha256sum -c` |

Use `apt`, not `dpkg -i` — the package depends on `lynx` and `dpkg` resolves
nothing. **And if the program is already running, quit and restart it:** a
running process keeps the binary it started with.

---

## In one paragraph

Two commands. `/sessions` reaches any conversation you have ever had — search it
by what was said in it, reopen it, save the whole thing to a file, or erase it
and **actually get the disk space back**. `/osint --recover` writes up a research
run that collected its findings and then died before producing the dossier,
which is the normal outcome of an interrupted run on a bad connection. Both exist
because of the same fact about the machines this is built for: the power cut ends
the session, and the disk was full before you started.

---

<!-- Version: 1.0.0 · updated 26-08-18-10-05 -->
# Gorilla Opencode v0.1.92 — the conversation you lost, and the disk space you never got back

Released 18 August 2026, two commits after v0.1.90. This release closes two gaps
that only really hurt on the machines this project is actually built for: a
conversation you have left is now reachable, and deleting one now returns the
space to your disk instead of pretending to.

It also implements a command that had been documented for a day without existing
— which is the kind of fault no compiler catches and no test in this repository
was looking for, so a test now looks for it.

This is the same document in two registers. The plain-language track assumes no
prior knowledge and omits nothing significant. The developer track is precise
enough to audit and rebuild from. Both end with a claim-source table saying
which statements this release measured and which it inferred — see
[PHILOSOPHY.md](../PHILOSOPHY.md) for why.

---

## Plain-language track

### Why this release exists

Three things were wrong. One was reported by the owner within minutes of the
last release; the other two were found by using the program on its own real
data rather than by reading the code.

**A command told you to run something that did not exist.** v0.1.90 added a
safety net: when an expensive research run finishes collecting, its findings are
written straight to disk by the program itself, before any AI is asked to do
anything with them. The file it writes told you what to do next — *run `/osint
--recover`*. So did the research tool's own report.

That command had never been built. Typing it handed the literal text `--recover`
to a ten-helper professional dossier as the subject to investigate, in the most
expensive configuration the program offers. The model refused — "I cannot
fabricate a dossier about '--recover', that's a flag, not a question" — which was
exactly the right call, and it still cost the setup of a run to discover.

**You could not reach a conversation you had left.** The session switcher showed
titles and nothing else: no date, no size, no way to search. And "save this
conversation" could only ever save the one currently open. Both assume you are
still sitting in the session you care about.

That assumption breaks on a fifteen-year-old laptop. The battery is measured in
minutes, sometimes seconds, so the machine lives plugged in — and when the
electricity drops, the session stops mid-sentence. Sometimes the electricity does
not come back for days. By then the conversation you needed is a title you no
longer recognise in a list you cannot search.

**You could not delete anything, and the obvious fix would not have worked.**
Conversations accumulated forever, with no way to see what they were costing. For
someone with a 500 GB disk that is untidy. For someone on an early Chromebook, or
a device from before Chromebooks, running on raw SLC NAND, CompactFlash, PATA
flash or eMMC with 1 to 2 GB free in total, it is the difference between the
program working and the program being uninstallable.

### `/osint --recover`

It lists every research run that collected findings — the saved files, and the
runs that live only in the local store — showing what was asked, when, how many
lanes reported, and what it cost. Pick one and it is written up as a dossier.
Nothing is searched again, no helpers are sent out, no web pages are fetched.

The reason it works where the original run failed is arithmetic, not cleverness.
A run on 17 August spent about 850,000 tokens and died at the write-up with its
context at 145% of the model's window. But the findings themselves were never
big: the strict report format had already compressed two hours of searching down
to about 15,000 tokens. What drowned the run was everything *else* it was
carrying — raw tool output, its own reasoning, the whole conversation. So the
write-up now happens in a fresh conversation carrying only the findings.

Proven against the real thing: **five dead runs from the night before were
recovered**, roughly 1.3 million tokens of collected work that had been sitting
unusable. Their distilled findings measured between 696 and 22,448 tokens.

### `/sessions`

Every conversation you have ever had, newest first, with the date, how many
messages, and how much space it is taking up. The line at the top gives the
total.

**Type to search.** It searches inside the messages, not just the titles. That
matters more than it sounds, because titles are generated automatically and "New
Session" is a real title this program writes. Weeks later you do not remember
what the summary called that day — you remember the error you were chasing.
Searching for the error finds the conversation. On the developer's own store,
searching one word took 18 conversations down to 3, and two of those three had
nothing matching in their titles at all.

**Enter reopens one**, exactly where it stopped.

**Ctrl+E saves one to a file** — the whole thing. Every message with its time,
the model's reasoning, and every tool it ran with the input it was given and the
result that came back, including the failures. That is deliberate: when something
goes wrong you need to be able to work out *how*. How did it end up publishing
something private; how did it end up deleting something. A record of only the
visible chat cannot answer either question.

If the conversation was a research run, its helper sessions come too. This is
where the release found one of its own bugs: a run listed 275 messages and
exported 14. The other 261 were in twenty-three helper sessions, and that is
where every piece of reasoning and every tool call lived. The export now carries
all of it — 2.6 MB, 238 tool calls with their inputs and results, 119 reasoning
blocks.

**Delete erases one for good** — the conversation, the helper sessions it
spawned, and everything in them — and then returns the space to your disk, and
tells you how much came back.

### Why "delete" usually frees nothing

The conversations live in a single database file. When you delete rows from it,
the database marks that space free *for its own later use*. The file on your disk
does not get smaller. Not by one byte.

Measured, in this release's own test:

| | Size of the database file |
|---|---|
| With 40 messages of about 25 KB each | 1,073,152 bytes |
| **After deleting every one of them** | **1,073,152 bytes** — unchanged |
| After rebuilding the file | 65,536 bytes |

On a machine with 1 GB free, an erase that returns nothing is not an erase. So
erasing rebuilds the file and reports the difference the *filesystem* shows,
rather than what the database claims.

Measured live, through the real interface, erasing one research run:

- Before: **5,575,048 bytes**
- After: **2,351,104 bytes**
- On screen: **"Erased 24 sessions · 3.1 MB returned to the disk"**

Twenty-four sessions, because one conversation plus the twenty-three helpers it
had spawned. Those helpers were invisible before this release, and they held most
of the bytes.

### The number at the top is the honest one

The header shows what the database **occupies on disk**, not what the messages add
up to. On the developer's machine those were 9.4 MB and 4.6 MB — because a
write-ahead log had grown to 4.3 MB and nothing had ever truncated it. Nearly half
the space was in a file nobody was counting. You are shown the number your disk
sees.

### Two safety rules on the erase screen

- Erasing takes **two deliberate keypresses**, and anything that is not `y`
  cancels — including an arrow key you were already holding down.
- Typing goes to the search box and can **never** trigger an action. Someone
  searching for the word "delete" spells d-e-l-e-t-e, and none of those letters
  may erase anything.

Both rules have a test that was proven to fail when the rule was removed.

---

## Developer track

### Surfaces

| Command | Scope |
|---|---|
| `/osint --recover` | write up a research run that collected findings but never produced the dossier |
| `/sessions` (`/history`) | any conversation ever held: search, revive, export, erase |
| `/export` | unchanged — the conversation currently open, with a chosen folder and filename |

Keys inside `/sessions`: `↑↓` move, `enter` revive, `ctrl+e` export, `DEL` erase,
`tab` toggle date/size sort, `esc` close. Printable characters go to the search
field, which is why the actions are not on letters.

### `/osint --recover` — `internal/llm/agent/research_recover.go`

Extraction is pure Go: no tokens are spent finding the findings, because a
recovery that needed a model to locate its own material would be a second chance
to fail the same way.

- `ListRecoverableRuns` merges saved `findings-*.md` files with runs reconstructed
  from the session store, deduplicating on the normalised question so an
  already-recovered run is not listed twice (it was: eleven entries for six runs
  on the first live drive).
- Helper sessions are grouped by tool-call id via
  `^(call_[A-Za-z0-9]+)-(.+)$`; a `supervisor:<lane>` id is paired to the lane it
  judged rather than emitted as a section of its own.
- `laneReport` takes the **last** assistant message carrying prose, not the first:
  helpers narrate between tool calls, and the contract-shaped report is written
  at the end.
- A lane that produced nothing is written as `LANE UNCOVERED`, never omitted.
- `AssemblyPrompt` carries the findings **inline**. Handing the model a path would
  route the file through the tool-result path, and bulk tool results are exactly
  what put the original run at 145% of its window. If the prompt exceeds two
  thirds of the selected model's context the run is refused with the two sizes
  named, rather than truncated into a dossier that silently omits three lanes.

`ListSessions` filters to `parent_session_id IS NULL` — correctly, since helper
sessions are not conversations — which returned zero runs from a store holding
five. `internal/db/research_helpers.go` asks the question the session picker
cannot.

### Storage — `internal/db/session_storage.go`

Hand-written, deliberately not sqlc-generated, so a regeneration cannot drop it.

- `AllSessionStorage` — bytes and message counts per conversation **including its
  helpers**, in one query for the whole list. A query per row is felt when
  opening a screen on eMMC.
- `DeleteSessionTree` — helper sessions **first**, then the parent. Sessions have
  no foreign key on `parent_session_id`, so SQLite will not cascade and a plain
  delete strands seventeen orphans per supervised run. The order is what makes
  the non-transactional path safe: interrupted between the two statements leaves
  helpers gone and the parent still listed — visible, deletable again. The
  reverse would leave a deleted parent with orphans nothing can name.
- `Reclaim` — `VACUUM` then `PRAGMA wal_checkpoint(TRUNCATE)`. VACUUM alone does
  not touch the write-ahead log, which was 44% of the total here. VACUUM needs
  temporary room roughly the size of the database, which is not a given on a full
  device, so the error is returned and the UI states that the space was **not**
  reclaimed.
- `SearchSessions` — `LIKE` over title and `messages.parts`, lowercased. Not FTS:
  a full-text index is a second copy of every message on a device with 1 GB free.
  On 578 messages / 4.77 MB the scan is imperceptible.

### Export — `internal/export/write.go`

`WriteSessionTree` renders the conversation and appends every helper session in
full through the same message renderer, returning the total message count so the
UI cannot claim more than it wrote. Files are `0o600` — a transcript holds
whatever was discussed, and these machines are shared. Existing files are never
overwritten; a taken name gets a numbered suffix, because telling someone whose
power just came back that their export "already exists" is the wrong answer.
Destination is `~/Documents/Gorilla-Session-Exports/`, derived from the user's
home and never hardcoded, never the working folder (often a git repository).

### The guard for the class that started this release

`internal/commands/promised_commands_test.go` walks every non-test Go file and
fails if a `/command` named in a string literal does not resolve in the command
registry. Instructional text is a promise with no compiler behind it; this gives
it one. Proven non-vacuous by renaming the string in `research_salvage.go` and
watching the test name the file.

### Bugs found while building this, all by driving the real binary

None of these were caught by a test that existed at the time. All six now have
one.

1. **Export dropped 95% of a research run** — 14 messages written where the list
   said 275.
2. **The size column vanished whenever a title was long**, which is almost
   always, since titles are generated from the first message. The title is now
   truncated; the size never is.
3. **A row with double-width characters wrapped.** `中文` is two runes and four
   display columns; an emoji is one rune and two; a real title in the store
   contains `U+FFFC`. Truncation counted runes. It now measures display columns
   via `ansi.Truncate`. **The symptom is height, not width** — lipgloss wraps, so
   after the bug fires every resulting line measures under the frame and a
   per-line width assertion passes cleanly. That assertion was written first and
   stayed green against the bug; the test now compares rendered height against an
   ASCII baseline.
4. **The search box ignored fast typing and every paste.** bubbletea coalesces
   input into one `KeyMsg` carrying several runes, and only single-rune messages
   were accepted.
5. **Two of three action keys never arrived.** `ctrl+s` is XOFF *and* is already
   bound globally here to the session switcher, handled before any dialog sees a
   key; `ctrl+d` is EOF. Both facts were already recorded in this repository, in
   `internal/tui/provider_escape_test.go`, before this screen existed. The app
   binds `ctrl+a c d e f h k l n o r s t u x y` — there was no free control key.
   `DEL` and `tab` are unbound and unreserved;
   `TestManagerActionsAvoidReservedKeys` pins it.
6. **The dialog drew 32 rows in a 24-row window** — caught by
   `TestDialogFramesNeverExceedTheTerminal` before it ever reached a screen.

### Known and deliberately unaddressed

An Arch/CachyOS tester reports stranded lines in the frame. It has not been
reproduced here, and the terminal, font and `$TERM` needed to reproduce it have
not been supplied. It stays open and unfixed rather than being guessed at — a
speculative fix to a rendering bug nobody can reproduce is how the footer-drift
bug survived three releases of wrong diagnoses. Every dialog added in this
release is covered by the frame-width and frame-height invariants at 60×20,
80×24, 100×30 and 130×42.

---

## Claim sources

| Claim | Source |
|---|---|
| DELETE leaves the file at 1,073,152 bytes; VACUUM takes it to 65,536 | **stated in input** — `TestReclaimActuallyShrinksTheFileOnDisk`, run 2026-08-18 |
| Live erase 5,575,048 → 2,351,104 bytes, UI reported 3.1 MB | **stated in input** — `du -sb` before and after, driven through the real TUI on a copied store |
| Store on the developer's machine: 65 sessions, 578 messages, 4,774,160 bytes of parts, 5,480,448 byte database, 4,313,672 byte WAL | **stated in input** — `sqlite3` and `ls -la`, 2026-08-18 |
| Research export: 275 messages, 23 helper sessions, 2,675,779 bytes, 238 tool calls, 119 reasoning blocks | **stated in input** — live export, `ls -la` and `grep -c` on the artifact |
| Five dead runs recovered, ~1.3M tokens, findings 696–22,448 tokens | **stated in input** — measured against the real session store |
| The 2026-08-17 run spent ~850,000 tokens and died at 145% of its window | **stated in input** — measured during that run |
| Nine lane reports totalled ~15,045 tokens on that run | **stated in input** — measured |
| Search took 18 conversations to 3, two matching on content only | **stated in input** — driven live |
| A full-text index would roughly double message storage | **model inference** — not measured; the LIKE scan was chosen on that reasoning |
| The Arch stranded-lines report is a frame-width issue | **not established** — unreproduced, awaiting the tester's terminal, font and `$TERM` |


---

# Appendix A — the full storage and sessions documentation


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
| `/sessions` (`/history`) | any conversation ever held; search, revive, export, erase |

Keys inside `/sessions`: `↑↓` move, `enter` revive, `ctrl+e` export, `DEL` erase,
`tab` toggle date/size sort, `esc` close. Printable characters go to the search
field.

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
