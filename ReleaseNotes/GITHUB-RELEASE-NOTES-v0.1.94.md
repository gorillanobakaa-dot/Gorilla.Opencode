# Gorilla OpenCode v0.1.94 — a professional code review, built in

**Everything you need to judge this release is on this page.** Not behind a
link, not "see the docs" — the complete plain-language explanation and the
complete technical one are printed below in full.

> *"Open source gave the world the recipe. It forgot to teach people how to cook."*

---

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.94_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.94_amd64.deb` |
| `gorilla-opencode-0.1.94-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer |
| `SHA256SUMS-v0.1.94.txt` | `sha256sum -c` |

Use `apt`, not `dpkg -i`. And restart the program if it is already running.

---

## In one paragraph

`/review` runs around thirty real static-analysis and security tools over your
code — `cppcheck`, `gosec`, `bandit`, `semgrep`, `clippy`, `shellcheck`,
`gitleaks` — picking whichever suit the languages actually present. They are
embedded in the binary, so nothing is downloaded when you run it. The answer
always begins with **what did not run**, because a review that found nothing
because nothing was installed looks exactly like a review of clean code, and if
no analyser is installed for your language it refuses to run rather than hand
you a reassuring blank. No other terminal coding agent ships this.

---

## v0.1.94 — 2026-08-18 — /review: thirty static-analysis and security tools, built in

**Plain-language version:** Type `/review` and the program runs around thirty
real code-analysis and security tools over your code — `cppcheck`, `gosec`,
`bandit`, `semgrep`, `clippy`, `shellcheck`, `gitleaks` and the rest — picking
whichever suit the languages you actually have. They find the mechanical faults:
memory errors, injection holes, leaked credentials, errors nobody checked. The
tools live inside the program; nothing is downloaded when you run it.

**The part that makes it trustworthy.** A review that found nothing because
nothing was installed looks exactly like a review that found nothing because
your code is fine. That is the worst way a review tool can fail, and no small
model can spot it. So every answer *starts* with which tools ran, which are
missing and which failed — before a single finding — and says the code a missing
tool covers is **UNREVIEWED**, in those words. If nothing is installed for your
language it refuses to run at all rather than hand you a comforting blank page.

It also flags every line that two or more **different** tools objected to
independently. Not "the AI thinks this matters" — two separate programs, written
by different people, disagreeing with the same line. Those are computed, and
they are never truncated.

**What it will not do**, and it says so every time: find wrong logic, a broken
assumption, or an error quietly swallowed. No static tool finds those. This is
half a review, and `/review` instructs the model to read the changed code itself
and tell you it did.

**Measured, on a real run over 50 files:** 17 analysers, 42 seconds, **739,476
bytes of raw JSON returned as a 7,315-byte summary** — a 99% reduction, because
every tool result is re-sent on every later turn. The trust block and the
corroborated findings are never cut; the long tail is capped at 60, sorted
most-severe-first, with the real total always stated.

The whole toolkit is embedded in the binary: **444 KB of payload, +480 KB of
binary**, about fifty seconds on an 8 KB/s line. "Install this other thing
first" is a wall, not a step, for the people this is built for — the same
reasoning that made `lynx` a hard dependency.

**A test this change had to fix, worth naming.** The loadout calibration test
asserted that a row's measured token count *differs* from its hand-written
estimate — a proxy for "calibration ran". `tool.review` measured 475 tokens, the
estimate was corrected from 320 to 475, and the test promptly declared the
correct figure a guess. It now stamps a sentinel no real schema can produce and
asserts calibration overwrote it. Same failure class as a limit counted in the
wrong unit: a proxy breaks exactly in the case it was meant to reward.

Also in this release: the first live run flagged credentials in two OAuth files.
They are installed-app client credentials publicly embedded in the open-source
Gemini and Antigravity CLIs, and were never confidential. The toolkit reported
their location and **withheld every value from the report**, which is the rule
that matters more than the finding.

Full detail, both tracks: [docs/CODE-REVIEW.md](../docs/CODE-REVIEW.md).

---

# Appendix A — code review, in full


*Dual-track. The first half is the complete explanation in plain language — not
a simplification. The second half is the developer record. Every number was
measured on 18 August 2026 and says which command produced it.*

---

## Part one: in plain language

### What it is

Type `/review` and the program runs about **thirty real code-analysis and
security tools** over your code, then tells you what they found — and, just as
importantly, what they could not check.

These are not AI guesses. They are the standard tools professionals use:
`cppcheck` and `clang-tidy` for C and C++, `gosec` and `golangci-lint` for Go,
`bandit` and `pylint` for Python, `clippy` for Rust, `shellcheck` for shell
scripts, `semgrep` for patterns across all of them, and `gitleaks` for
credentials accidentally committed. They find the mechanical faults: memory
errors, injection holes, leaked secrets, errors nobody checked.

The tools themselves live **inside the program**. Nothing is downloaded when you
run it.

### The part that matters most

**A review that found nothing because nothing was installed looks exactly like a
review that found nothing because your code is fine.**

That is the worst way a review tool can fail, and it is easy to fall for. So this
one is built the other way round. Every answer *starts* with which tools ran,
which are missing, and which failed — before a single finding. And if none of
the tools for your language are installed, it **refuses to run at all** rather
than hand you a comforting blank page.

If a tool is missing, the answer says the code it covers is **UNREVIEWED**, in
those words.

### What to read first

The answer flags every line that **two or more different tools complained about
independently**. Not "the AI thinks this is important" — two separate programs,
written by different people, objecting to the same line. Those are computed, and
they are where to start.

### What it cannot do

It will not find wrong logic. It will not find a broken assumption, an error
quietly swallowed, or code that is technically fine and completely wrong for
what you are building. **No static tool finds those.**

So this is half a review, and it says so every time. The AI still has to read
the code, and is instructed to tell you plainly that it did and what it found.
A review that claims to be complete having only run the tools is lying to you.

### If tools are missing

The answer names them and gives you the command that installs them. The full
toolkit is around 30 programs; you only ever need the ones for the languages you
actually use, and the program works out which those are by looking at your code
rather than installing everything.

---

## Part two: the developer record

### Shape

| | |
|---|---|
| `/review [path]` | routed through the agent, not called directly |
| `review` tool | `path`, `diff`, `deep` |
| Loadout row | `tool.review`, **475 tokens**, on by default |
| Vendored at | `internal/llm/tools/codereview/toolkit/` |
| Payload | 444 KB, `go:embed all:toolkit` |
| Binary growth | **+480 KB** measured |

### Why the command goes through the agent

`/review` does not print findings and stop. It sends the agent an instruction to
run the tool, read the trust block first, start from the corroborated findings,
**and then read the changed code itself** for what static analysis cannot see —
and say so explicitly. A command that dumped analyser output would produce
exactly the "looks complete, is half" failure the tool's own description warns
about.

### Why the toolkit is embedded rather than depended on

The audience is on single-digit KB/s. "Install this other thing first" is a wall,
not a step — the same reasoning that made `lynx` a `Depends:` rather than a
`Recommends:`, and the same reasoning that embedded `pfind`. 444 KB against a
19 MB package is about fifty seconds on an 8 KB/s line.

What is embedded is the **orchestrator**, not the analysers: the part that knows
which tool suits which language, how to normalise thirty different output
formats into one shape, how to verify a reported line actually says what the
tool claims, and how to report what did not run. The analysers themselves are
never embedded and never will be.

Unpacking is content-addressed: `Version()` is a SHA-256 over every embedded
file, and that hash names the unpack directory, so a binary upgrade unpacks
beside the old copy rather than mixing versions. A `.complete` marker is written
**last**, so an extraction killed halfway is redone rather than silently used.

### Output bounding

A real review of this repository's `codereview` package produced **739,476 bytes
of JSON**. The tool returns **7,315 bytes** — a 99% reduction — because every
tool result is re-sent on every later turn. That is the grep lesson in a
different tool: a limit must be expressed in the unit of the resource it
protects.

What is never truncated: the trust block, and the corroborated findings. Both
are small, and both are what stop a reader drawing a false conclusion. The long
tail is capped at 60, sorted most-severe-first, and the real total is always
stated along with how many were left out.

### Tests

- `TestTrustBlockComesBeforeAnyFinding` — ordering is structural, not stylistic.
- `TestNoFindingsIsNotReportedAsClean`
- `TestFindingsAreBoundedAndTruncationIsAnnounced`
- `TestMostSevereFindingsComeFirst`
- `TestCorroboratedFindingsAreNeverTruncated`
- `TestVendoredToolkitMatchesDevCopy` — per-file SHA-256 against the working copy
  in `Scripts.For.Work`, and a check that every Python module there is embedded.
  A new module that is not vendored kills the shipped toolkit at import time.
- `TestEveryModuleTheToolkitImportsIsEmbedded` — closes the import graph without
  needing the development copy at all.

Two were proven non-vacuous by reverting the behaviour they assert.

### One test this change had to fix

`TestCalibrationCoversEveryComponentWithNoLSPClients` asserted that a row's
calibrated token count **differs** from its hand-written estimate. That is a
proxy for "calibration ran", and it fails precisely when the estimate is
correct: `tool.review` measured 475, the estimate was corrected to 475, and the
test then declared the figure a guess.

It now stamps a sentinel value no real schema can produce and asserts
calibration overwrote it — measuring the thing itself rather than a side effect.
Same failure class as a limit counted in the wrong unit.

### Measurements

| What | Value | Source |
|---|---|---|
| Vendored payload | 444 KB | `du -sh` on the embedded tree |
| Binary growth | +480 KB | `stat` before and after |
| Schema cost | 475 tokens | `toolTokens()`, measured |
| First hand-written estimate | 320 tokens — **48% under** | corrected |
| Live run: analysers that ran | 17 | real run, 2026-08-18 |
| Live run: raw JSON | 739,476 bytes | same run |
| Live run: returned summary | 7,315 bytes | same run |
| Live run: wall clock | 42 s over 50 files | same run |

All **stated in input** — measured by the command named. Nothing estimated.

### A note on what the first live run found

`gitleaks-history` flagged credentials in `internal/auth/antigravity_oauth.go`
and `internal/auth/gemini_oauth.go`. These are **installed-app OAuth client
credentials, publicly embedded in the open-source Gemini CLI and Antigravity
CLI** — the source comments have always said so. Per the estate's rule on
secrets, a value that ships inside downloadable software was never confidential.
The remaining hits are deliberate fake keys in test fixtures.

Worth recording because the toolkit behaved correctly: it reported the location
and **withheld every detected value from the report**, which is the rule that
matters more than the finding.


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
| `/review [folder or file]` · `/audit` `/codereview` | Run 30 real analysers over your code and report honestly. |
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

### `/review [folder or file]`

*Also: `/audit`, `/codereview`*

**Run 30 real analysers over your code and report honestly.**

A professional static-analysis and security review, built in. Point it at a folder, a file, or your changes and it runs around thirty real analysers — the ones that find memory errors, injection, leaked secrets, unchecked errors — picking whichever suit the languages actually present. C, C++, Go, Python, JavaScript, TypeScript, Rust, shell and more.

With no arguments it reviews your current folder. Add a path for somewhere else. The tools live inside the program; nothing is downloaded when you run it.

**It tells you what did NOT run.** That is the part that matters. Those thirty analysers have to be installed on your machine, and if they are missing they simply find nothing — which looks exactly like a clean report. So the answer always starts with which tools ran, which are missing, and which failed; and if none of them are installed it refuses to run at all rather than hand you a reassuring blank.

It also flags every line that two or more DIFFERENT tools complained about independently. Those are the ones worth reading first.

What it cannot do: find wrong logic, a broken assumption, or an error quietly ignored. No static tool can. This is half a review and it says so — the AI still has to read the code, and should tell you it did.

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
