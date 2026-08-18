# Your past conversations: finding them, saving them, deleting them

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
