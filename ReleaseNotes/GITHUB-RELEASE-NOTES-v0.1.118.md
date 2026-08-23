# Gorilla OpenCode v0.1.118: stop asking about the string, start asking about the outcome

Everything about this release is printed in full on this page.

**Six fixes, one theme.** Every one of them was a number or a question put in
front of you that you could not act on.

---

## The prompt you clicked twenty times

Every web search asked permission, and the approval covered only that exact
search. Nobody runs the same search twice, so "Allow for session" covered
nothing useful, and clicking Allow every single time was the only thing that
worked. A ten-helper research run asked ten times, and answering did not help,
because the eleventh search was a different question again.

> *"This is getting ridiculous. In order to get a web search either I have to
> click ten or twenty times if the search has to restart or what?"*

**Now it asks once.** Allow covers this search. **Allow for session** covers
searching for the rest of the session. The two things people actually want are
on the two buttons, which they were not before: "allow every future search with
this exact wording" is a grant nobody has ever needed.

---

## Why the old prompt was useless, and not just annoying

It showed you the search words.

```
Search SearXNG for: ath9k rfkill regression
```

You cannot tell from that whether allowing it is wise. Reading it more carefully
does not help. It is the equivalent of a surgeon showing you their choice of
incision and asking you to approve it: the honest question was never about the
technique, it was about the risk.

So the prompt now says the things you can actually decide about:

- **what leaves the machine**: search terms, to one search service
- **what it can reach**: the open web, and nothing on your computer. No files,
  no commands, no keys, no history
- **what cannot be taken back**: a term that has left has left, and terms are
  written from what you were discussing
- **that there is no list to show you**, because the model writes each search as
  it works

That last point is the one most tools leave out. If permission is being asked
over words nobody can read in advance, the dialog should say so plainly instead
of showing one example and implying that is the deal.

**Opening pages is separate and still asks, once per site.** Sending words out
is one thing; choosing which server answers you is another, and the second is
what a poisoned search result would want to change.

---

## Research runs ask once, before they start

A single question covering searching and fetching for the whole run, showing how
many helpers are about to start.

**Deny does not cancel the run.** It falls back to asking separately for each
search and page, and the dialog says so, because Deny on a permission prompt
otherwise reads as a cancel button. The approval ends when the run ends.

---

## Nine questions you never saw were recorded as refusals

Nobody had noticed this one, and it is worse than the clicking.

The dialog holds exactly one question. When ten helpers asked at the same
moment, each question painted over the last, and **only the tenth was ever
shown**. The other nine waited for an answer that could no longer arrive, timed
out after ten minutes, and were recorded as denied.

Nothing unsafe happened: an unanswered request is refused, which is the safe
direction. The problem is honesty. A question you never saw was filed as one you
refused, and while it happened the run looked frozen for ten minutes and then
blamed the network.

Reproduced by putting the bug back:

```
showed 1 of 10 queued requests: [j]
```

Questions now queue. Answer one, the next appears, and anything your answer
already settled is approved quietly instead of asked again.

---

## `/tasks`: tab and escape work

Reported twice. With the task list open, tab and escape did nothing, and only
began working once enough helpers finished for the list to shrink.

The permission prompt was **underneath** the task list. It was taking the
keystrokes, because a blocking question outranks everything else, but it was
drawn first, so every other dialog painted over it. You were typing at a dialog
you could not see.

Finished helpers also used to vanish from `/tasks` the instant they were marked
DONE. They now stay until the run is over.

---

## The cost screen was roughly double, and left your own model out

Three errors, all in the direction of a wrong number stated confidently.

**The money was nearly double.** Helper steps were priced at the main model's
context. A helper does not run that: it carries four tools, the main model
carries thirteen.

```
helper                    5,246 tokens
main model (was used)    10,380 tokens     1.98x
```

**"Worth about N ordinary questions" had no token in its arithmetic.** It was a
step count wearing a label naming a unit it never touched. At ten helpers it
said 30. The real token model gives about 16.

**"This run" left out your own model.** The launch turn and the write-up turn run
on the main model, and the write-up turn carries everything the helpers sent
back. On a cheap-helper, expensive-main-model setup that single turn can cost
more than the whole fleet. It now has its own line, in its own model's name.

---

## The per-minute figures are measured now

Every per-minute and per-hour figure rested on an invented constant: fifteen
seconds per step. It was labelled ASSUMED on screen, which was honest, and is
not the same as knowing.

Helper durations are now recorded, and the figure comes from your machine, your
model and your connection. Until there are three samples the screen still says
ASSUMED, and it always says which of the two it is showing.

The **median** is used rather than the average, so one helper stuck on retries
does not drag the forecast above anything you will ever see.

---

## Smaller

- The research modes said "up to 4 at a time". The real cap is **11** and had
  been for a while. It reads the cap now instead of repeating a number.
- A research run had no label of its own, so the longest operation in the
  program said "Working...".

---

## Credit

The research progress line now reads:

```
Gorilla is analyzing this... with science...
```

That is a nod to a **Pete Holmes** stand-up bit, *"I'm gonna analyze this...
WITH SCIENCE!"*, and it is credited here because it earned its place rather than
because it was borrowed for a laugh.

The bit is funny because it is the sound of authority with nothing inside it.
Which is exactly what a permission dialog does when it shows you a search string
and asks you to approve it. It named the problem in this release before we did.

No special or year is cited, because that was not checked, and a confident wrong
citation is worse than none.

---

## What was checked, and what was not

Verified: full test suite green with the real exit code captured, not a piped
`echo`; every new test re-run with its bug reinstated to prove it fails.

**Not verified, said plainly:**

- **What age-based context eviction actually saves.** The rule ships and is
  tested, but nobody has measured it on a real long session, and the 8-turn
  window is untuned.
- **Whether cancelling the main agent kills its sub-agents.** Flagged, still
  open, not touched here.
- **Any paid ChatGPT plan**, unchanged since v0.1.117.

---

## Install

**Debian / Ubuntu:**

```sh
sudo apt install ./gorilla-opencode_0.1.118_amd64.deb
```

`apt` rather than `dpkg -i`, because the package depends on `lynx`, `python3`
and `ripgrep`, and `dpkg` resolves nothing.

**Arch / CachyOS**, pre-built, no Go toolchain needed:

```sh
sudo pacman -U gorilla-opencode-0.1.118-1-x86_64.pkg.tar.zst
```

Or build from source: `git clone`, `cd packaging`, `makepkg -si`. The PKGBUILD
carries a real checksum, not `SKIP`.

**Verify your download first**, in the same directory as the files:

```sh
sha256sum -c SHA256SUMS-v0.1.118.txt
```
