<p align="center"><img src="../internal/assets/icons/gorilla-opencode-256.png" width="96" alt="Gorilla OpenCode"></p>

<h1 align="center">What Gorilla OpenCode is, what it isn't,<br>and how far it has come since the fork</h1>

<p align="center"><em>Written 2026-09-03, from the git history rather than from memory.<br>Every number below is produced by a command, and the command is named.</em></p>

---

## In one paragraph

Gorilla OpenCode is a terminal AI coding assistant for Windows. You type what
you want in plain language, and it reads your files, runs your build, edits
code, searches the web, and reports what actually happened. It began as a fork
of **OpenCode**, a project that had been **archived and abandoned**, and it has
since been rebuilt around one idea its original never had: that the person using
it is often paying for every token by the megabyte, and deserves to be told the
truth about what a thing costs and whether it really worked.

---

# Part one — for anyone

## It started with a dead project

On **17 September 2025**, the last commit landed on OpenCode. It was a README
change, and this is what it said:

> Archived: Project has Moved. This repository is no longer maintained and has
> been archived for provenance.

The original authors moved on to a different product. The code sat there,
finished, unmaintained.

That archived commit is the fork point. The code then sat untouched for **304
days**. The first commit of this project, on **20 July 2026**, is titled
**"Revive:"**.

So this is not a fork in the usual sense of taking a healthy project and
steering it. It is a salvage. Somebody picked up an abandoned tool, got it
running again, and then rebuilt it into something the original was not -- 534 commits in
45 days.

## How much has changed

| | at the fork | today | |
|---|---|---|---|
| Commits by this project | 0 | **534** | all by one person |
| Files | 162 | **1,024** | |
| Lines of Go | 42,183 | **136,537** | 3.2x |
| **Test files** | **4** | **300** | 75x |
| Documentation files | 2 | **217** | |
| Documented commands | 0 | **31** | there was no command reference at all |

The test number is the one worth pausing on. The original had **four** test
files for forty-two thousand lines of code. That is not a criticism of people
who moved on to other work — but it does explain why so much of the work
here has been finding things that were quietly broken.

## What it can do now that it could not then

**It works on Windows properly.** The original was written for Linux and macOS.
Getting it to behave on Windows meant fixing a shell that never worked, a screen
that erased itself, a window that opened at the wrong size, and a terminal that
never noticed being resized — none of which are visible in a feature list, and
all of which made it unusable.

**It runs your models, not just paid ones.** It finds local models running on
your own machine, and it works with free tiers — Google Gemini, NVIDIA NIM,
OpenRouter, ChatGPT — as well as paid APIs.

**It tells you what a conversation costs.** There is a screen (`/context`) that
prices every part of what gets sent on every single turn, and lets you switch
pieces off. The original had no such thing, and no way to know.

**It reviews code with real tools.** `/review` drives around thirty genuine
analysers — the ones that find memory errors, injection, leaked secrets — and,
crucially, **refuses to run** if none of them are installed, because an empty
result looks exactly like a clean report.

**It ports patches between versions.** `/port` forward-ports, backports and
rebases patches, and tells you *how* each one landed rather than just whether it
did.

**It looks things up properly.** Web search across scholarly and open-access
sources, plus eleven scientific databases queried directly.

**It sends helpers to investigate.** `/research` splits a question across
several sub-agents, each on one angle, with a verifier attacking their
conclusions.

## What it is NOT

**It is not a Linux release.** It compiles for Linux and passes the checks
there, but it has never been run on Linux. The published build is Windows and
says so on the first line.

**It is not the original OpenCode, and it is not Crush.** The original authors
took their work in a different direction under a different name. This is a
separate path from the same starting point.

**It is not finished.** The known issues are published with every release, and
one feature in the current version ships switched off because it was measured
and found to fail on small models.

**It is not magic, and it will tell you so.** The rule the whole project is
built around is that the program must never claim something worked when it did
not, and must never look like it found nothing when it simply did not look.

---

# Part two — for developers

## The shape of the change

Eleven internal packages exist that did not before:

```
arsenal    assets     auth       bootstrap  commands   export
osutil     plain      plaincmd   politehttp quota
```

Those names carry the story. `commands` is a single source of truth for what
every slash command does, enforced by a drift test that reads the dispatch
switch. `politehttp` is per-host rate limiting. `quota` and `auth` are the
free-tier and OAuth work. `plain` and `plaincmd` are an alternative
non-alternate-screen interface for terminals where the TUI cannot work.
`bootstrap` is the Windows first-run path.

## Tools: fewer, and different

| at fork | today |
|---|---|
| agent, bash, diagnostics, edit, fetch, **glob, grep, ls**, patch, **sourcegraph**, view, write | agent, bash, **bio_lookup**, edit, fetch, **find**, patch, **patch_port**, **research**, **review**, **sparse**, view, **websearch**, write |

Twelve to fourteen, but the composition changed more than the count. `glob`,
`grep` and `ls` were replaced by a single `find`: three descriptions repeating
WHEN TO USE / HOW TO USE / LIMITATIONS cost roughly 1,485 tokens on every turn,
and `grep` returned paths only, so answering anything cost a second turn and a
whole-file read.

Measured afterwards, `find` costs 1,322 tokens against those ~1,485 — a saving
of about 160, not the two-thirds originally claimed. **The comment asserting
"roughly a third" was corrected rather than the number quietly adjusted.** The
change was still right, on grounds that do not depend on that figure: one tool
instead of three removes a tool-choice mistake small models kept making, and
returning matching lines rather than paths removes an entire round trip.

## Providers

At the fork: `anthropic, azure, bedrock, copilot, gemini, openai, vertexai`.

Today the provider directory carries not just more providers (`antigravity`,
`chatgpt`, `code_assist`) but an infrastructure layer that did not exist:
`ratelimit`, `stallguard`, `reasoning`, `uploadbudget`, `evict_age`,
`supersede`, `gzip_request`, `httpclient`. That is where the free-tier work
lives — pacing, stall detection, upload budgeting, and request supersession.

## The system prompt

| | at fork | today |
|---|---|---|
| Structure | two hardcoded constants (`baseOpenAICoderPrompt`, `baseAnthropicCoderPrompt`) | one file, 13 sections |
| Size | ~2,374 tokens | **~2,073 tokens** |
| Switchable | no | every section is a `/context` row |
| Editable | recompile | a text file, overridable at runtime |

It is **smaller than the original** while covering considerably more, and every
section can be switched off individually by someone who does not want it.

The original was 2023-era prompting: ALL CAPS emphasis, "IMPORTANT:" stacking, a
threat-toned register. The current one is declarative, sectioned on `# headers`
that auto-register as loadout rows, with `[[needs tool.x]]` markers that drop
individual lines when a tool is switched off.

## The testing culture is the real difference

Four test files became three hundred, but the count matters less than the kind.
Several exist specifically to stop silent drift:

- every switchable component must have a low-bandwidth decision, in one list or
  the other — absence from both is indistinguishable from nobody having looked
- every command that can be typed must be documented, and vice versa
- every deferrable tool must map to a real `/context` row
- recorded token costs are checked against measured ones
- a new guard is **mutation-tested**: broken deliberately to confirm it fails

That last one has earned its place. Two guards written in the most recent
release passed against deliberately broken code before being fixed. A test that
cannot fail is decoration.

## What a maintainer should know before touching it

**Measure, do not estimate.** Nine recorded component costs were found wrong by
up to 182%. The calibration path overwrites them at startup, so the running
program was honest while the source lied to anyone reading it.

**CRLF will waste your afternoon.** This is a Windows checkout. Four separate
investigations in one week were spent on diffs that showed whole files as
changed when nothing had changed. `diff --strip-trailing-cr` before believing
anything.

**`$?` after a pipe is the last command's status.** `cmd | head` reports
`head`'s success. This has produced a "build OK" printed over a failed build,
and a tool reported as passing when it exited 1.

**Read `TO.DO.TO.FIX/`** if you have it. It is gitignored and holds the measured
evidence behind decisions the code only summarises.

---

## An honest assessment

The work is 45 days old, it is one person's, and it has more
tests than most things this size. Its distinguishing quality is not any single
feature — it is that the program is built to refuse to overstate itself: a
review that will not run rather than return a misleading blank, a cost screen
that reports what actually goes on the wire, a patch tool that distinguishes
"applied" from "applied somewhere plausible".

Its weaknesses are equally plain. It is Windows-tested and Linux-unverified. It
carries a year of one person's decisions with no second reviewer. And some of it
is very new — the most recent release shipped a feature switched off because
measurement contradicted the intuition behind it.

For the people it is built for — no credit card, an old laptop, data sold by the
megabyte — the trade it makes is the right one. A tool that quietly wastes your
quota is worse than a tool that tells you it cannot help.
