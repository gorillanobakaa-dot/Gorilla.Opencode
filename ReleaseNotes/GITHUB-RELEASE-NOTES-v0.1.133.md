# Gorilla OpenCode 0.1.133 — Windows

**This is the Windows build.** Previous Windows release: **0.1.130**, seventeen
commits ago.

0.1.132 existed and you could not use it. It shipped a `.deb`, an Arch package
and a Linux binary, and no `.exe` — so anyone on Windows landing on the latest
release found three things they could not install and left. Everything fixed in
that release had been compiled for Linux only. This closes that gap.

---

## Why this release exists

Two reasons, and the first is an apology.

**The Windows build was three fixes behind and nothing said so.** The work
below was written, tested and released for Linux on 3 September. On Windows the
program carried on showing a command list in a narrow panel and a cost screen
cut off mid-sentence, because nobody had built it here.

**Then a run on Linux found six broken tests, and three of them had been
passing for weeks while testing nothing at all.** No product code needed
changing. The tests were asserting things that could only ever be true on
Windows. That is worth a release on its own: a test that cannot fail is not
protecting anything.

---

## What you will see

### The command list uses the whole window

Before: a panel about 84 columns wide showing roughly a dozen of the 31
commands, with nothing to say the rest existed. If you did not already know the
list scrolled, the bottom row looked like the end of what this program can do.

After: two balanced columns across the whole terminal, with a count at the top.
On an ordinary screen every command is visible at once.

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does and showing 37 of 37 lines, 31 commands, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.133/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.133/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

### Low-bandwidth mode now says it is on

Pressing `l` switches seven things off. Six of them sat below the visible part
of the list, so nothing on screen changed and it looked exactly like a key that
did nothing. A red line now stays at the top for as long as the mode is on,
saying how many things are off and which key puts them back.

This is the same failure that produced 0.1.130: a switch that silently removed
capability and never mentioned it again.

[![The context loadout screen with the low bandwidth preset applied, showing a red line reading LOW-BANDWIDTH MODE 7 components switched off press u to put them back, above the per-turn cost of 8,441 tokens and the list of tools with their individual costs](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.133/docs/screenshots/gallery/v0132-low-bandwidth-mode-line.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.133/docs/screenshots/gallery/v0132-low-bandwidth-mode-line.png)

### The cost screen finishes its sentences

`/context` was capped at a fixed size, so on a wide screen the column explaining
what switching a tool off would cost you was cut off mid-sentence. That column
is the one thing on that screen you actually need before deciding. It now uses
the whole window, and the footer rows are reserved so the banner and title stop
scrolling away.

### Patch porting still tells you HOW a patch landed

Unchanged in behaviour, verified on a second platform. `/port` asks permission
before it touches a git tree, and asks for nothing when only inspecting.

[![The patch port tool running an inspect operation and reporting inspect completed, naming the tree, its kind and branch, and the single patch found with its hunk and file count, with no permission dialog anywhere on screen](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.133/docs/screenshots/gallery/v0132-patch-port-inspect-no-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.133/docs/screenshots/gallery/v0132-patch-port-inspect-no-prompt.png)

[![The same patch port tool asked to forward-port instead, stopped on a Permission Required dialog naming the tool, the full path of the git tree it would modify, the operation and the patch series, offering Allow, Allow for session and Deny](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.133/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.133/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)

---

## The AI's rulebook grew, and part of it is an experiment

The instructions sent to the AI on every message gained eleven lines. Six are
about honesty and are not negotiable. Five are about restraint and **are new,
unmeasured, and switched on**.

### The six that stay on

**First, a word you will see a lot below.** A **token** is roughly three-quarters
of a word — it is the unit AI services count and charge by, and the unit your
data allowance disappears in. Everything the AI is told gets re-sent on every
single message, so a line added to its instructions is not a one-off cost. It is
a standing charge, on every message, forever. That is why this page prices each
change instead of just listing it.

Three of the six concern what the AI writes down about you: open a file before
claiming not to have the answer; never record its own guess as though you had
said it; one passing mention of something is not a preference.

One concerns what happens when you push back: correct a real error and move on,
rather than folding and abandoning a correct answer because you sounded annoyed.

Two concern verification, and both name a bug this program has actually shipped:

- **Check the thing itself, not the message about it.** When you run a command
  and feed its output through a second command, the "did it work?" answer you
  get back belongs to the *second* one. So a build can fail while the terminal
  cheerfully reports success. That has printed **"BUILD OK" over a failed
  build** in this repository more than once. The AI was already told to verify
  its work; it was never told *where to look*. Now it is: open the file and see
  if the thing is actually there.
- **Missing means missing.** If something the AI expected is not there, it must
  say so — never quietly write an empty stand-in file so the job can carry on.
  That converts a problem you would have noticed into one you would not.

Cost: 84 tokens per message for the verification pair, 161 for the rest.

### The five that are an experiment

`/context` has a new row, **Scope restraint**, costing **212 tokens on every
message**. In plain terms, it tells the AI to stay on the job you gave it:

- A file being nearby, or already open, or mentioned by the file you asked
  about, does not make it part of the task.
- "It would save time later" is not permission.
- Do not go reading through a whole folder tree nobody asked about.
- Move something to a clearly labelled holding place rather than deleting it.
- Assume the machine you actually have — its speed, its screen, its connection —
  rather than an imagined average one.

**Read this part carefully, because it is the honest bit.**

These five lines have **never been tested**. Not once. We have no evidence that
an AI reads around less with them than without them — only a reasonable belief
that it should.

They are switched on anyway, for one reason: a rule nobody ever runs can never
be proven or disproven, so leaving it off would have kept it a guess forever.
212 tokens on top of roughly 2,250 was judged a fair price for one release in
exchange for finding out.

**If it goes wrong, it will go wrong by being too careful — not too reckless.**
Watch for the AI:

- asking permission for things it used to simply do,
- running commands one at a time that it used to send together, making it
  slower and costing you more,
- or telling you it is stuck when it could have looked in the next folder along.

Any of those, and it is probably these five lines.

**It is one keypress to switch off.** Open `/context`, find *Scope restraint*,
press space. Nothing else changes, and you get the 212 tokens back on every
message from then on.

That is the same deal deferred tool loading got in 0.1.130, and it is the deal
anything unmeasured should get: shipped where it can be observed, priced in
public, and removable by one key.

---

## Install

1. Close Gorilla OpenCode if it is running.
2. Download `gorilla-opencode.exe` below.
3. In a terminal, from wherever you saved it:

```
.\gorilla-opencode.exe install
```

It copies itself into your programs folder and adds Desktop and Start menu
shortcuts. Then check it:

```
gorilla-opencode --version
```

You should see `v0.1.133`.

**To go back:** `gorilla-opencode uninstall`, then install your previous `.exe`
the same way. Your conversations, settings and API keys live elsewhere and are
untouched by either step.

**Verify the download** (optional):

```
certutil -hashfile gorilla-opencode.exe SHA256
```

```
2513f4ccfd8c8143fc2523f3d35eddef067541291c65c5a0da0b0fac4d199eeb
```

---

## Known issues

| What you see | Why | What to do |
|---|---|---|
| The AI asks permission for things it used to just do | The new Scope restraint rules, overreaching | `/context`, find *Scope restraint*, press space |
| The AI says a tool does not exist | Something switched it off, usually the low-bandwidth preset | `/context`, press `u` to undo the preset |
| `/review` refuses to run and lists missing analysers | The analysers are separate programs and are not installed. An empty result would look exactly like a clean report | Read the list — it names each one and how to install it |
| A patch reports **applied WITH FUZZ** | It did not fit where it claimed; surrounding lines were used to place it | Read the diff. If those lines appear twice in the file, it can land in the wrong place |
| Deferred loading on, and a tool stops being used | Small models do not always realise they should search for it first | Switch it back off in `/context` |

---

## Not done, and said rather than implied

### macOS

Never built for, never run on it. Nothing here changes that.

### A saving we found, tested, and could not make safe

There is a switch called **Deferred tool loading**, and it is still off.

Here is what it is. The AI has tools — searching the web, reviewing code,
porting patches, looking up scientific records. It does not know they exist
unless it is told, so a written description of every tool is sent along with
your message, **every single time you press enter**. Those descriptions are the
bulk of what gets sent. You pay for them whether or not the AI uses a single one.

The switch holds back the descriptions of the specialist tools and sends a short
index instead, letting the AI ask for the full description only when it wants
one. That saves roughly **2,900 tokens per conversation** — genuinely worth
having if you are paying by the megabyte or living inside a free daily limit.

**We ran it 26 times against a small AI on an ordinary laptop.** Two of its four
specialist tools were never reached — the AI never thought to ask for them. And
it did not stop and say "I can't do this." It answered from whatever it could
read, and sounded completely confident doing it.

**We know why it happens:** a small AI does not reliably work out that it should
go looking for a tool before deciding it does not have one. **We do not yet know
how to make it reliably do that.** Until we do, a saving that occasionally
removes an ability without telling you is not a saving worth switching on for
everyone.

If you use a larger AI it may well work fine. Turn it on in `/context` and watch
how yours copes. It cannot cost you more than leaving it off — that part is
guaranteed and tested.

### The new Scope restraint rules have not been proven to do anything

Said in the section above and repeated here, because this is the part most
likely to affect you and the last place you might look before downloading.

**"Unmeasured" means exactly what it sounds like.** We have not run a single
test showing that these five lines change how the AI behaves. They might make it
tidier. They might make it timid. They might do nothing at all and cost you 212
tokens a message for the privilege.

They are switched on so that we — and you — can find out, on a version where
that is written on the box rather than discovered later. If the AI starts asking
permission for things it used to just get on with, that is these rules, and
`/context` → *Scope restraint* → space removes them.

---

## Privacy

No telemetry was added and none exists. Nothing is sent anywhere except to the
AI provider you configured.

---

## Full notes

Both tracks are in the repository:

- [`v0.1.133-release-notes.layman.md`](Changelogs/v0.1.133-release-notes.layman.md) — plain English
- [`v0.1.133-release-notes.developer.md`](Changelogs/v0.1.133-release-notes.developer.md) — architecture, measurements, file by file
