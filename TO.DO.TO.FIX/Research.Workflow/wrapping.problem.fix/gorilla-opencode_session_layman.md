# Your words stop vanishing as you type — and the program stops answering to someone else's name

> **Gorilla Opencode v0.1.85** · released 16 August 2026 · replaces v0.1.84
> Plain-language track. A developer twin covers the same release in technical detail.
>
> ⚠ **Read "What you will lose" before you upgrade.** This release does not carry your
> old conversations across. Nothing is deleted, but nothing is moved either, and the
> program will not tell you that when you start it.

---

## The story

Two things were wrong. One you could see every time you typed. The other you could not
see at all, until it nearly cost somebody a day's work.

### One: you could not read your own sentence

Type a long sentence into the chat box. When it reached the right-hand edge of the
screen it did not drop onto a second line, the way text does in every other program you
have ever used. The box stayed exactly one line tall, and your sentence slid sideways —
the beginning of it disappearing off the left edge while you were still typing the end.

Your words were never lost. The program had every character, and pressing Enter sent the
whole thing correctly. But you could not read back what you had written before sending
it. If you write more than a few words at a time, that is the same as broken.

The sentence that finally pinned it down was 171 characters long, on a screen 176
characters wide. At the 172nd character, everything before it went dark.

### Two: two different programs were labelling their files with the same name

This program is a **fork**: somebody took a copy of an existing program called OpenCode
and developed it in a different direction. That is normal and legal and how a great deal
of software gets made. But the copy kept the original's habits, and one of those habits
was the name it wrote on everything it saved — its settings, its saved conversations,
its scratch files.

So on a computer that had both programs, there were two sets of folders with the same
name, holding different things, belonging to different software.

**This is not a hypothetical risk.** During development, exactly the predictable thing
happened: while clearing out the old program's leftovers, the folders belonging to *this*
program were very nearly thrown away with them — including a fix that existed in no
other copy anywhere. It was caught, and recovered. That near-miss is why this release
exists.

Now everything this program creates carries its own name, `gorilla-opencode`, and lives
in the places Linux sets aside for exactly this purpose.

---

## The picture that explains the typing bug

Imagine a tailor measuring a jacket.

The proper way is to measure the customer. This program was measuring **a mannequin
standing next to the customer** — and the mannequin was still set to the last customer's
size. Every measurement was taken carefully and honestly. Every measurement described
somebody else.

The mannequin was set to *one line tall*. So no matter how much you typed, the answer
came back "one line". And when a text box is one line tall and the text is longer than
the line, the only thing it can do is slide sideways — which is exactly what you saw.

The fix: before measuring, take a **spare copy** of the box, open it out to full size,
lay *this* sentence into it, and count the lines it actually fills. Then throw the spare
copy away and set the real box to that height.

That is all `measuredRows()` does. Reset the mannequin, put the customer's own cloth on
it, then reach for the tape.

### Why three earlier attempts failed

This is worth knowing, because it looks like carelessness and it was not.

The symptom — text sliding off to the left — *looks like* a width problem. So the first
attempts fixed width problems. They found real bugs and fixed them: the box had been
capped at half the screen even when there was room to spare, and the box had not been
told how wide the terminal actually was. Both genuine. Neither one touched this.

You cannot give a box more room and expect it to grow if the box keeps measuring itself
as one line tall.

Worse, the tests written to catch it **passed while the bug was still there**. A test
builds a fresh, clean text box — and a fresh box has no stale measurement lying around to
reuse, so it measures correctly and the test goes green. Three times, that green tick
said "fixed" when nothing was fixed.

It was only found by running the real program with a counter printed on screen: the text
grew past 600 characters while the measurement sat at 1, frame after frame. Once you can
see those two numbers disagree, the cause names itself.

---

## What you will notice

**Typing a long message**
- **Before:** the box stayed one line; the start of your sentence scrolled out of sight.
- **After:** the box grows downwards and everything you typed stays visible.
- **Affects:** everyone, most severely on narrow windows.

**Where your files live**
- **Before:** settings, database and scratch files were named after the other program, and
  a new folder appeared inside *every project folder you ever opened*.
- **After:** one set of folders, named `gorilla-opencode`, in the four standard Linux
  locations — settings in `~/.config`, saved conversations in `~/.local/share`, downloaded
  model lists in `~/.cache`, logs in `~/.local/state`.
- **Affects:** everyone. This is the change that requires you to read the next section.

**Crash reports**
- **Before:** if the program crashed it dropped a log file into whatever folder you happened
  to be in — including, potentially, a folder you then uploaded to GitHub.
- **After:** crash logs go to one place set aside for them, out of your project folders.
- **Affects:** everyone, silently and for the better.

---

## What you will lose

This is the honest part, and the previous version of this document did not mention it at
all. **Nothing here is deleted. But nothing is carried across either.**

The program used to keep your saved conversations in a hidden folder inside each project.
It now keeps them in one place under your home folder, under a new filename. There is no
code in this release that moves the old ones over. So:

| What | What happens on first launch |
|---|---|
| **Your saved conversations** | The history list is empty. The old ones sit untouched in the old folders. |
| **Custom slash commands you wrote** | Gone from the menu. The files are still on disk in the old location. |
| **Per-project settings** (a `.opencode.json` inside a project) | Silently ignored. If you had pinned a cheaper model for a project, it quietly goes back to the default one — which can cost money without saying so. |
| **Your `/context` on-off switches** | Reset to defaults. |

Your old data is safe. It is simply not read. Recovering it means moving files by hand,
and the developer document explains where from and where to.

**Two further consequences worth knowing**, both side-effects rather than decisions:

- **Every project now shares one conversation history.** Previously each project had its
  own. Now work on one project and work on another appear in the same list.
- **The "shall I set this project up?" question is now asked once, ever.** Answer it in
  your first project and no later project will offer it again.

If either of those is wrong for how you work, say so — they are consequences of a path
change rather than choices anyone made deliberately.

---

## What it costs

Nothing extra to run, and no new data leaves your machine. Two small, real costs:

- **The download is about 18.5 MB.** On a single-digit KB/s connection that is roughly
  forty minutes. There is no smaller path to this release; it is one static file with
  nothing to install alongside it.
- **The model list re-downloads once**, about 650 KB, because it moved to a new folder
  along with everything else. Roughly eighty seconds on a slow line, once.

The measuring fix does slightly more work per keystroke than before — it draws the box
invisibly to count the lines. On the 2012 laptop this project is built for, that has not
been measured. It has not been observed to lag; that is an observation, not a number.

---

## How to install it

**Step 1 — download the package.** From the releases page, take the file ending `.deb`
for Debian, Ubuntu and Mint, or the one ending `.pkg.tar.zst` for Arch and CachyOS.

**Step 2 — install it.** Open a terminal in your Downloads folder and run:

```bash
sudo apt install ./gorilla-opencode_0.1.85_amd64.deb
```

✓ It should finish without asking anything further. Use `apt`, not `dpkg -i` — `apt`
also fetches the small text browser the web search needs, and `dpkg` does not.

**Step 3 — confirm what you actually have.**

```bash
gorilla-opencode --version
```

✓ Prints `v0.1.85`. If it prints anything else, the old version is still the one being
run and the install did not take.

**Step 4 — check the typing fix yourself.** Start the program and type a sentence long
enough to reach the right-hand edge of your window.

✓ The box grows downwards and the start of your sentence stays on screen. If it slides
sideways instead, the fix is not in the copy you are running.

**To go back:** `sudo apt install ./gorilla-opencode_0.1.84_amd64.deb` if you kept the
previous file. Your old conversations, which this version cannot see, become visible
again — they were never moved.

---

## If something goes wrong

**"All my conversations are gone."**
They are not gone; this version looks in a new place. See "What you will lose" above.
**Status:** working as built in v0.1.85 — no automatic move exists yet.

**"The install script from the README does not work."**
It does not, and that is a real bug in this release: the script asks the download site
for a file type this project does not publish. **What to do:** install the `.deb` or the
`.pkg.tar.zst` by hand as described above. **Status:** broken in v0.1.85, not yet fixed.

**"The Arch instructions gave me the wrong version."**
The instructions in the packaging file tell you to download 0.1.85 and then install
0.1.84. Trust the download line, not the install line. **Status:** broken in v0.1.85,
not yet fixed.

**"Setting an `OPENCODE_...` option no longer does anything."**
Those settings were renamed to start `GORILLA_OPENCODE_` instead. Some of the project's
own documentation still shows the old names. **Status:** the program is correct; the
docs are stale.

---

## Common questions

**Is the program faster now?**
No, and nothing here claims it is. This release fixes what you can see and where files
are kept. It does not change how fast anything runs.

**Did my API keys move or leak?**
Neither. The settings file was already correctly named and did not move. No key was
copied anywhere by this release.

**Is my old data destroyed?**
No. Every old folder is still on disk exactly as it was. The new version does not read
it, which is not the same as deleting it.

**Do I have to upgrade?**
No. v0.1.84 keeps working. If you rely on your saved conversations day to day, waiting
until an automatic move exists is a reasonable choice.

**Was the typing fix properly tested?**
Partly, and this document will not pretend otherwise. There is a test covering the
wrapping boundary, and it passes. But when the fix itself was deliberately removed and
the test re-run, **the test still passed** — so it does not actually guard the thing that
was fixed. The proof that the fix works is a person typing into a running build and
watching it behave. That is real evidence, and it is weaker than a test, and the honest
thing is to say so.

---

## The bottom line

1. **You can read what you are typing again.** Long sentences wrap downwards instead of
   sliding away.
2. **This program now owns its own files**, named after itself, in the standard places —
   so the mix-up that nearly destroyed a day's work cannot happen the same way twice.
3. **Your old conversations will not appear**, and nothing on screen will tell you why.
   They are safe; they are just not carried over.
4. **Three known rough edges ship with this release** — the install script, the Arch
   instructions, and some stale documentation — all listed above rather than discovered
   by you.
5. **The typing fix is proven by watching it work, not by a test.** The test that looks
   like it guards this bug does not.

---

## Claim sources

| Claim | Basis | Evidence |
|---|---|---|
| The box stayed one line while the text grew past 600 characters. | 📄 stated in input | live trace: `textarea_width=172, textarea_height=1, measured_rows=1`, value_bytes rising |
| The reproduction sentence was 171 characters on a 176-column screen. | 📄 stated in input | the recorded reproduction line in the session notes |
| No code moves old conversations, commands or project settings. | 🔍 verified in this session | `migrateLegacyConfig` and the loadout migration are deleted in commit 8b7c14d; a search for replacement migration code found none |
| The regression test passes with the fix removed. | 🔍 verified in this session | the fix at `editor.go:180` was reverted and `TestExactLongPromptWrapsAtTerminalWidth` still passed |
| The published package is 19,405,016 bytes and its checksum matches. | 🔍 verified in this session | recomputed sha256 of the local `.deb` matches `SHA256SUMS-v0.1.85.txt` and the published asset |
| The install script requests a file the project does not publish. | 🔍 verified in this session | `install` asks for `gorilla-opencode-linux-x86_64.tar.gz`; the release has three assets, none of them a tarball |
| Every project now shares one history, and the setup question is asked once. | 🤖 model inference | follows from `data.directory` becoming one absolute path; not separately tested |

---

**How to verify this document:**
`📄 stated in input` — the model's phrasing of something the source text said. Find the
matching line in the original to check it.
`🔍 verified in this session` — a command was run and its output read. The command is
named so you can run it yourself.
`🤖 model inference` — the model's own judgement. Treat as opinion, not measurement.

*Plain-language track. Its developer twin covers the same release in technical detail.
Neither is a summary of the other; both are complete.*
