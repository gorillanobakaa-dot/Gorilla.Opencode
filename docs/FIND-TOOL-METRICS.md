# The find tool — every measured number, and what each one costs a real person
<!-- Version: 1.2.0 · updated 26-08-17-17-53 -->

This file is the source of record for the release documentation, both tracks.
Every figure below was **measured during the 2026-08-17 build session**, not
estimated. Where a figure came from a live incident, the incident is named.
Quote these verbatim; do not round them into vagueness.

---

## 1. The scam this tool exists to end: paying twice for one question

The old toolset (`ls`, `glob`, `grep` — what nearly every AI coding product
ships) answers "where is this code?" with **a list of file paths and nothing
else**. The model then has to open a whole file to actually see anything.
Measured on this repository:

| Step | Tokens |
|---|---|
| `grep` finds the file (paths only) | 16 |
| The forced follow-up: reading the whole 7,317-char file | **1,829** |
| **Plus** a second full round trip of standing context | **~9,935** |

Standing context — system prompt, tool descriptions, conversation history — is
re-sent on EVERY message. That ~9,935-token figure is from this machine's own
`/context` screen. So the two-step search pattern doesn't cost 16 + 1,829
tokens; it costs a whole extra turn. **Every failed or half-answered search a
model relaunches is roughly 10,000 tokens of someone's money.**

The `find` tool returns the matching lines WITH surrounding context:
**132 tokens, one turn**, for the same question that cost 16 + 1,829 + a
second turn. That is the entire design.

**Layman's version:** imagine directory assistance that will only tell you the
street, never the house number — so every call becomes two calls, and you pay
for both. Careless companies ship search tools that work exactly like that,
and the meter runs on your account, not theirs.

## 2. What the three old tools cost just by existing

Tool descriptions ride on every single message, whether used or not.

| | chars | ≈ tokens/turn |
|---|---|---|
| `ls` + `glob` + `grep` descriptions & schemas | 5,554 | **1,388** |
| `find`, first minimal version | 3,032 | 758 |
| `find`, full arsenal (fuzzy, recency, git-dirty, tree/long/code views) | 5,117 | **1,279** |

The full arsenal — everything pfind can do, exposed — still costs **less than
the three crippled tools it replaced**. A test
(`TestFindDescriptionCostsLessThanTheThreeItReplaced`) enforces that boundary
forever, with the owner's reasoning recorded in its comment: a relaunched
failed search costs ~10k tokens; schema tokens are cheap by comparison.

## 3. Why small/free-tier models failed before, measured

- 2026-08-14 (recorded in `internal/llm/agent/toolname.go`): a 10-helper
  research run on `local.meta/muse-glimmer-30b` returned **nothing** — 30 of
  44 tool calls failed with `Tool not found: ls<|message|>`, `grep<|message|>`
  etc. Every failed call was still billed.
- Three overlapping tools = a tool-choice decision on every search. One tool
  named `find`, whose description opens with "THIS TOOL REPLACES grep, glob,
  ls, rg, ripgrep, list_dir…", removes the decision entirely.
- A model that still asks for `grep` now gets an error that TEACHES:
  *"searching, file finding and directory listing are all done by the 'find'
  tool… retry using find."* The mistake costs one small error message, not a
  dead investigation.

## 4. Speed on real trees (the workloads that used to fail)

Kernel checkout: **3.4 GB, 119,980 files.** Engine: pfind 3.2.0 → two ripgrep
legs (name + content) at `--threads 8`, SIMD/mmap, fused by RRF ranking
(weights: name 2.0, content 1.0, git 0.8, recency 0.5; k=60 — the score
printed beside each result IS the fusion score).

| Query | Time | Result size |
|---|---|---|
| `ath9k_hw_init`, `type="c"` | ~1.2 s | 1,763 tokens, answered with code |
| `"static int"` broad, paths only | ~1.6 s | 435 tokens **+ honest cap marker** |
| Folder listing (depth-limited) | ~1.6 s | 472 tokens |
| Broken regex | instant | 37-token honest **FAILURE** (never "no matches") |
| Whole home directory, no narrowing | 27.8 s | capped at 32 KB (see §5) |

Raw `rg` alone on the same query: 0.31 s. pfind's overhead buys the ranking,
the name-leg, the caps and the honesty markers.

## 5. The caps, and the incident that justifies each one

Every tool result is re-sent on every later turn. An unbounded result is a
**recurring** bill, not a one-off. Layered bounds, each from a real incident:

| Cap | Value | The incident behind it |
|---|---|---|
| Bytes per result | 32 KB (~8k tokens) | Old grep: capped *matches* at 100, still returned **2,438,026 bytes** — one turn took a conversation from 15.9K to 675K tokens (2026-07-30) |
| Matches per file | 3 (with context) | One kernel file with 36 hits emitted all 36: 35,165 B → 16,579 B after the cap |
| Files per result | 15 (40 tree / 20 long) | The listing path once printed a kernel tree's every path — a 14 MB dump |
| Chars per line | 500 + omission marker | Crocodile test: a single spell-checker wordlist line ate budget that could have held ten more files |
| Wall clock | 30 s | The unnarrowed whole-home search measured 27.8 s |
| Absolute backstop | 400 KB | `NewTextResponse`, catches the tool nobody thought about |

**Every cut announces itself** — `TRUNCATED`, `only 15 shown`, `… 34 more
match(es)`, `[… omitted end of long line]` — because a model handed a silent
fragment reasons about the fragment as if it were the whole answer.

## 6. The live proof: the crocodile search (2026-08-17, DeepSeek V4 Flash)

The model was asked "there is a file in here that contains the word crocodile.
find it" and called `find {"path":"/home/gorilla","query":"crocodile"}` — the
entire home directory, maximally careless, exactly what a small model does.

- Raw matches: **348,881 bytes** (kernel source + a wordlist)
- Delivered: 32,768 bytes + TRUNCATED marker, in 27.8 s
- The model answered **correctly from the capped result**: IBM Crocodile SAS
  adapter, `drivers/scsi/ipr.c:181`, both checked-out kernel copies identified
- Under the old grep, this exact shape was the 2.4 MB / 675K-token incident

One follow-on cost it exposed, then fixed: 32 KB ≈ 8k tokens filled 95% of a
32K-context local model's window. The per-line cap (500 chars) now keeps the
same budget full of *diverse* results instead of one wordlist line.

## 7. Found while testing, because the numbers made it visible

The crocodile turn's call and its full result were **in the database and
absent from the screen** — every tool-using turn had baked
"Waiting for response…" into the terminal's permanent history, because the
turn printed the moment the model stopped streaming, before its results
existed. Users could not see what any tool did or returned. Fixed
(`ScrollbackSettled`): a tool-using turn now prints only when its results
exist, so the transcript shows the call, its arguments, and what came back.
Transparency you cannot see is not transparency.

## 8. What the user can switch off

The whole tool is one `/context` row — "Find tool (search + list + glob)",
calibrated cost shown, toggleable like everything else. Turning it off saves
its full measured cost and blinds the agent to the tree; that trade belongs to
the user, stated in those words. Nothing here is hidden and nothing rides free.

## 9. The program file got BIGGER — here is exactly why, and why that is the right direction

Measured, binary against binary, same build flags:

| Build | Program size |
|---|---|
| 0.1.87 (the old three tools) | 51,867,940 bytes |
| 0.1.88 (the find tool, measured at the swap) | 51,999,012 bytes |
| **Growth** | **+131,072 bytes — exactly 128 KiB** |

The **shipped** v0.1.88 binary is larger again — 52,125,988 bytes — because the
release carries more than the find tool: the four data-source backends, the
embedded source atlas, the `/osint` dossier command and its capability page all
landed after the measurement above. Against v0.1.87's 51,867,940 bytes that is
**+258,048 bytes (252 KiB) for the whole release**, which is the number to
judge the download by. The paragraph below explains why a quarter-megabyte of
one-off download buys back a recurring per-message bill.

**Where the growth comes from.** Retiring `ls`/`glob`/`grep` removed roughly
10 KB of compiled code — but `find` carries the entire pfind search engine
(139,343 bytes of Python) **inside the program itself**, via Go's `embed`.
That is a deliberate portability decision: any bare downloaded binary can
search on any machine that has python3 — no package to install, no path to
configure, nothing that only exists on the developer's computer. Net result:
+128 KiB.

**Layman's version.** Think of the program as a toolbox. We threw out three
flimsy screwdrivers and put in one good power drill — and the drill comes
with its own battery, so it works the moment you pick the box up, anywhere,
without hunting for a charger. The box got 128 KiB heavier. On the slowest
connection this project designs for — 8 KB/s, a metered line, someone else's
throwaway laptop — that extra weight costs about **16 seconds of download,
once in your life**.

**What those 16 seconds buy.** Every time the AI searches your code with the
old-style tools and half-fails, it relaunches — and every relaunch re-sends
roughly 10,000 tokens of standing context at your expense (§1). The
embedded engine is what makes the one-call, answered-first-time search work
everywhere. Sixteen seconds once, against ten thousand tokens saved per
avoided retry, forever: it is the cheapest trade in the whole project.

**Why we say it grew instead of pretending it shrank.** It would be easy to
write "smaller, faster, better" in a release note — it is what careless
companies write. The program did NOT get smaller; it got 128 KiB bigger, and
this document says so with the exact byte counts, because the promise of this
project is that you can check every claim. What actually got smaller is the
thing you pay for repeatedly: **the conversation**. The always-present tool
descriptions went from 1,388 tokens to 1,279 per message; one search call
replaced the old search-then-open-the-whole-file chain; results are capped at
32 KB instead of sprawling. If the app feels lighter, that is why — the fat
moved out of every message you pay for and into a one-time 128 KiB of
download. **That is the direction you always want the fat to move: out of the
recurring bill, into the one-off.**
