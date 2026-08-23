# Gorilla OpenCode v0.1.116: what every message costs, counted rather than guessed

Everything about this release is printed in full on this page, including the
pictures.

**No new features.** This one makes the program cheaper to run and removes three
things that were quietly untrue: a token figure reached by subtraction, an
economy setting that had stopped covering everything, and a bug fix that was a
runtime check where it should have been a type.

---

## Every message carries a price, and we had it wrong

Every time you send a message, the program also sends a description of **every
tool** the model can use. Every message, not once at the start.

That cost was written down as **about 5,000 tokens**, and the document said
plainly that the number came from subtraction rather than counting: measure a
total, take away the parts you know, call the rest tool descriptions.

Counted directly, it is **8,462**. Nearly double.

```
tool schemas, default ON      8,462   69%
base system prompt            1,791   15%
prompt blocks, default ON       130    1%
per-turn total, no CLAUDE.md 12,174

find 1,322 · research 1,007 · bash 962 · fetch 789 · edit 759 · review 759 · websearch 749
```

The subtraction had been done against a total measured on **one machine** with
several tools switched off, so the leftover described those settings rather than
what a new user pays.

**The part worth remembering: the real number was already on screen.** The
`/context` screen has been showing the cost of each tool, individually and
correctly, the whole time. Nothing was missing except somebody adding the rows
up.

It is now pinned by a test that prints the whole breakdown, so the documented
figure can be regenerated rather than trusted, and fails if it drifts by more
than 500 tokens.

You can see the per-tool prices yourself, and the running total in the status
bar:

[![The context loadout screen listing each tool with its individual per-message token cost, and the status bar at the bottom reporting the running total of about 8,138 tokens per turn](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.116/docs/screenshots/gallery/v0116-loadout-cost-in-status-bar.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.116/docs/screenshots/gallery/v0116-loadout-cost-in-status-bar.png)

---

## The economy setting had a hole in it

`/context` has a key (**`l`**) that switches off everything not needed for
ordinary editing and building. It exists for people on slow or metered links.

That list was written on **14 August**. The code-review tool arrived on
**18 August**, switched on by default, costing **759 tokens per message**, and
nobody went back to add it. So the setting built for somebody on a satellite link
was still shipping a thirty-analyser review tool with every single message.

Nothing looked broken. It still ran, still switched things off, still showed a
smaller number. It was simply leaving 759 tokens behind, and only counting the
schemas directly made it visible.

**From default settings it now saves 37%**, from 12,178 tokens per message down
to 7,713.

### And the key itself was wrong

`l` was written as a **preset**, not a subtraction. It did two things: switch off
everything on its list, and switch everything **else** back to the factory
default. The second half undid your own economies.

Here it is on a hand-trimmed setup, before, at ~8,802:

[![The context loadout before pressing the low bandwidth key, showing about 8,802 tokens per turn with the bash, code review, diagnostics, edit and environment rows switched off and the web tools switched on](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.116/docs/screenshots/gallery/v0116-context-before-low-bandwidth.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.116/docs/screenshots/gallery/v0116-context-before-low-bandwidth.png)

After, at ~8,138. Only about 8%, because the web tools went away but bash, the
edit tool, environment info and four language servers all came **back**:

[![The same context loadout after pressing the low bandwidth key, showing about 8,138 tokens per turn with the web tools now off but the bash tool, edit tool, environment info and four language servers switched back on](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.116/docs/screenshots/gallery/v0116-context-after-low-bandwidth.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.116/docs/screenshots/gallery/v0116-context-after-low-bandwidth.png)

Run against the old code from a more trimmed starting point, the test says it
plainly:

```
the low-bandwidth key RAISED the per-turn cost: 6125 -> 7020
tool.bash was switched off by hand and the low-bandwidth key turned it back ON
tool.edit was switched off by hand and the low-bandwidth key turned it back ON
```

**895 tokens more per message, from a button labelled low bandwidth.** It only
shows if you have already trimmed harder than the defaults, which is exactly what
somebody on a bad connection does. Anywhere else it looked like it worked.

**Fixed. `l` now only ever subtracts.** It switches off its list and leaves every
other choice alone, so it can never raise your bill. `r` already existed for
restoring the shipped defaults. Pinned by a test that fails if the cost goes up,
or if a component you disabled by hand comes back.

### The ambiguity is gone either way

A tool missing from that drop-list used to mean either "we decided to keep it" or
"nobody looked", and from outside those are the same thing. There are now **two**
lists: things dropped, and things **deliberately kept with the reason written
down**. A tool in neither fails the build.

---

## Files stopped being sent over and over

Read a file at ten in the morning, and the program was still paying to send it at
eleven, in every message in between.

An old read is now replaced with a short note saying the content was dropped and
where to find it.

**The note matters more than the saving.** A model that simply cannot see
something fills the gap from memory. Being told plainly that content was removed,
and that it should look again, is the difference between a re-read and an
invention.

Only things cheap to fetch again are dropped: reading a file, searching, checking
diagnostics. **Nothing from the web** is touched, because re-downloading costs
you bandwidth and may not return the same page. Nothing recent is touched at all.

---

## The petrol gauge cannot read the wrong tank any more

v0.1.115 fixed the symptom: `/usage` on a ChatGPT session used to show a
different service's allowance, and a check now stops that.

A check is something somebody has to remember to write.

This release makes it impossible instead. A reading now carries the account it
belongs to **as one thing rather than two**, so it cannot be shown under another
name, and the screen that draws the bars no longer knows anything about sign-ins.

The same fault turned out to exist in the low-quota warnings, and nobody had
noticed: they were filed under the name of the limit, so two accounts both
reporting a "Weekly Limit" would have warned about each other.

---

## One of ours, written down

The file-dropping feature shipped in its first draft with a safety rule that read
as sensible and was really an **off switch**.

The rule was "always keep the most recent read of each file". A file read **once**
is trivially the most recent read of itself, so it could never be dropped, and a
file read once then forgotten is exactly what the feature was built for. It did
nothing at all.

It passed every test asking "did we break anything". It failed the one asking
"did it actually work".

**A guard that makes a feature safe by making it inert is not a safe feature, it
is a disabled one.**

---

## What was checked, and what was not

Verified: full test suite green; **every new guard checked non-vacuously** by
reinstating the real bug and confirming it fails; the panel layout compared
against a rendered sample with all 14 existing panel tests passing; the packaged
binary's SHA-256 equal to the built binary and to the installed copy.

**Not verified, said plainly:**

- **How much context the file-dropping actually saves in a real session.** The
  mechanism is tested. The saving is not measured. That claim is a mechanism, not
  a number, and it was the same gap in v0.1.115.
- **The 8-turn window** before a read counts as idle. Chosen because evicting too
  early is worse than evicting too late, not tuned against real sessions.
- **Any paid provider.** The meter work still runs only against a free plan and
  the free Antigravity tier.
- **Whether anyone relied on the old reset behaviour.** `l` no longer restores
  defaults; `r` does, and always did.

---

## Install

**Debian / Ubuntu:**

```sh
sudo apt install ./gorilla-opencode_0.1.116_amd64.deb
```

`apt` rather than `dpkg -i`, because the package depends on `lynx`, `python3`
and `ripgrep`, and `dpkg` resolves nothing.

**Arch / CachyOS**, pre-built, no Go toolchain needed:

```sh
sudo pacman -U gorilla-opencode-0.1.116-1-x86_64.pkg.tar.zst
```

Or build from source: `git clone`, `cd packaging`, `makepkg -si`. The PKGBUILD
carries a real checksum, not `SKIP`, so `makepkg` verifies what it fetched.

**Verify your download first**, in the same directory as the files:

```sh
sha256sum -c SHA256SUMS-v0.1.116.txt
```
