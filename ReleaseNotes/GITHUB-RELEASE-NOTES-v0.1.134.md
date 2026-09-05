# Gorilla OpenCode 0.1.134 — Windows

**A two-sentence fix to the AI's rulebook, and the audit that found it.**

Previous Windows release: **0.1.133**, published yesterday. If you are running it,
this replaces it. Nothing else changed — same features, same tools, same install.

---

## What was wrong

Version 0.1.133 shipped five new rules telling the AI to stay on the job it was
given: don't wander into nearby files, don't read through whole folders uninvited,
don't delete things, assume the machine you actually have.

They were switched on, and the release page said plainly that they had **never been
measured**. This release is what happened when they were.

Two of them were broken. Not subtly.

### It could not read the file with the answer in it

One rule said a file being nearby, imported, or already open does not make it part
of your task. Sensible. But it did not distinguish **looking** from **touching**.

> You say *"fix the build error in `src/parser.c`"*. The compiler complains that
> `FOO_MAX` is undefined. `parser.c` includes `limits_config.h`, which is where
> `FOO_MAX` lives.
>
> The rule marked that header as out of scope. **So the AI could not open the file
> containing the answer.**

Another shape of the same fault:

> You say *"the build is broken, fix `render.cpp`"*. The first real error is a
> missing semicolon in `geometry.h`, cascading into forty errors in `render.cpp`.
> The rule marked the actual fix site as off-limits.

This program is built for large C and C++ codebases where the fault is almost never
in the file you named. The restraint landed squarely on the thing it exists to do.

### It could not delete, and could not do the alternative either

A second rule said: don't delete things, move them to a clearly labelled holding
folder instead.

An older rule, which outranks it, says: don't create files nobody asked for.

So the AI could not delete, and could not create the approved holding folder. It
would simply stall.

---

## What changed

Two sentences. **No rule was switched off.**

**Looking is not touching.** The first rule now restrains *changing* — editing,
deleting, moving, running — and says outright that reading along an include or call
chain to find the fault is part of the job, not a scope violation.

**The holding folder is part of the delete.** No longer a forbidden extra file.

That is the entire code change: four lines in one text file.

---

## How this was found, and how much to trust it

Forty-four AI agents examined the rulebook from three angles at once: one group
classifying every rule, one hunting for conflicts, one predicting behaviour on
realistic tasks. Then ten more whose only job was to prove the first lot wrong.

**Three of those groups independently landed on the same rule** without being
pointed at it. That convergence is why these two fixes were made and the other
three rules were left alone.

**Now the honest part, because it cuts the other way.**

The suspicion that started the audit was that the rulebook had become lopsided —
too many "stop" rules against too few "go" rules, roughly 15 to 6. That figure was
a hand-count and it was **wrong**. Counted properly: **20 restraint, 18 completion,
20 honesty, 16 neutral.** Balanced. The real defect was two sentences, not a ratio.

**Every one of the ten demolition agents succeeded** in refuting the finding it was
given. They were instructed to assume a finding was wrong unless proven, so a high
strike rate is partly built into the method — and only about a tenth of the serious
findings were ever checked before the audit ran out of budget. So the findings are
softer than they look, and are not disproved either.

**Eight of thirteen test scenarios changed behaviour with no harm at all.** The
rules were never broadly breaking things. Two specific sentences were.

**And no agent ran a real task.** Every scenario is one AI predicting what another
would do. That is careful reasoning, not measurement, and it should not be read as
measurement.

---

## Still not measured, still shipped on

The other three restraint rules are unchanged and still switched on. One of them,
*"no scanning or stockpiling"*, drew twenty-six findings in the audit with eight
rated serious, and barely any were checked.

They stay on for the same reason as before: a rule nobody runs produces no
evidence. If the AI starts asking permission for things it used to simply do, that
is them, and **one keypress removes all five** — open `/context`, find *Scope
restraint*, press space. You get 212 tokens per message back.

---

## Install

1. Close Gorilla OpenCode if it is running.
2. Download `gorilla-opencode.exe` below.
3. In a terminal, from wherever you saved it:

```
.\gorilla-opencode.exe install
```

Then check it:

```
gorilla-opencode --version
```

You should see `v0.1.134`.

**To go back:** `gorilla-opencode uninstall`, then install your previous `.exe` the
same way. Conversations, settings and API keys live elsewhere and are untouched by
either step.

**Verify the download** (optional):

```
certutil -hashfile gorilla-opencode.exe SHA256
```

```
9c3bd04f8ea8c06c8de1ec781be2e7d70bca94939bfd1ed970e0045dac3b4840
```

---

## Everything else from 0.1.133 is unchanged

Including the parts most worth knowing about if you are arriving here first:

**The command list uses the whole window.** Two columns, all 31 commands visible at
once, instead of about a dozen in a narrow panel.

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does and showing 37 of 37 lines, 31 commands, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.134/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.134/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

**Low-bandwidth mode says it is on**, for as long as it is on, instead of
announcing itself once and fading.

[![The context loadout screen with the low bandwidth preset applied, showing a red line reading LOW-BANDWIDTH MODE 7 components switched off press u to put them back, above the per-turn cost of 8,441 tokens and the list of tools with their individual costs](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.134/docs/screenshots/gallery/v0132-low-bandwidth-mode-line.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.134/docs/screenshots/gallery/v0132-low-bandwidth-mode-line.png)

**The research tool refuses rather than overspending.** Ask for more helpers than
your limit allows and the run does not start — and the AI reports the refusal
instead of inventing a result. Note the spend at the bottom of that screen.

[![Gorilla OpenCode refusing a research run that asked for ten helper agents when the configured limit is three, showing the error that the helper-leash allows three and how to raise it, followed by the model explicitly declining to produce a partial or speculative dossier, with total spend showing zero dollars](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.134/docs/screenshots/gallery/v0133-research-leash-refuses-ten-agents.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.134/docs/screenshots/gallery/v0133-research-leash-refuses-ten-agents.png)

---

## Known issues

| What you see | Why | What to do |
|---|---|---|
| The AI asks permission for things it used to just do | The three remaining restraint rules, overreaching | `/context`, find *Scope restraint*, press space |
| The AI says a tool does not exist | Something switched it off, usually the low-bandwidth preset | `/context`, press `u` to undo the preset |
| `/review` refuses and lists missing analysers | Those analysers are separate programs and are not installed. An empty result would look exactly like a clean report | Read the list — it names each one and how to install it |
| A patch reports **applied WITH FUZZ** | It did not fit where it claimed; surrounding lines were used to place it | Read the diff. If those lines appear twice, it can land in the wrong place |

---

## Not done, and said rather than implied

- **macOS.** Never built, never run.
- **Linux.** The last Linux build is 0.1.132 and does not contain this fix.
- **The remaining three restraint rules are unmeasured**, and one of them has
  serious unchecked findings against it.
- **Deferred tool loading still ships off.** Tested over 26 runs against a small
  AI, it silently cost two of four specialist tools without saying so.

## Privacy

No telemetry was added and none exists. Nothing is sent anywhere except to the AI
provider you configured.

## Full notes

- [`v0.1.134-release-notes.layman.md`](Changelogs/v0.1.134-release-notes.layman.md) — plain English
- [`v0.1.134-release-notes.developer.md`](Changelogs/v0.1.134-release-notes.developer.md) — the audit method, evidence weights, and what it cost
