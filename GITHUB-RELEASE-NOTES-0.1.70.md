# v0.1.70 — the error you needed to read was being deleted, and the input box ignored the window

v0.1.68 promised that a failed turn would leave the full explanation in your
conversation, where you could scroll back to it and copy it. **It did not work,
and this release is mostly about admitting that and fixing it properly.**

The reason was recorded correctly and then deleted a fraction of a second later
by a different piece of code, so what you actually saw was:

```
Canceled — no answer was produced.
```

for a failure that had nothing to do with cancelling anything.

### Fixed

- **Failures are no longer mislabelled "cancelled" with the reason deleted.**
  Every provider failure was affected. The real reason now survives and appears
  in your conversation.

- **The input box no longer outgrows the window.** On a short terminal a long
  prompt appeared stuck on one line, scrolling sideways and continuing "from the
  last word", while the *same build* wrapped perfectly in a taller window. The
  box had been ignoring its row allotment entirely — handed 1, 2, 3 or 5 rows it
  drew **16 every time** — so on a short screen the frame grew taller than the
  window, and a frame taller than the window cannot be erased and redrawn
  correctly. It now respects the space available, and says so (`▲ N more lines`)
  when it has to hold text back.

- **Errors stay readable for 40 seconds**, not the 10 they shared with notices
  like "copied to clipboard". Reported as *"it flashes by so fast I barely had
  time to read it — it took me two tries to screenshot it."*

- **No more guessing at a cause we already know.** The app suggested your request
  might have been "too large" even when the real error was known — appearing
  directly above a message contradicting it, with the context reading **0%**. The
  guess now shows only when nothing better is available.

- **Switching model no longer strands the helper agents.** `/models` changed only
  the main agent, leaving session titles, summarising and sub-tasks on the old
  one. If that model had stopped working the only clue was a recurring "failed to
  generate title", while the real failures waited for later. Helpers now follow —
  but only if they were already matching; one you deliberately set differently is
  left alone.

Also repaired two flaky tests, one of which guards the footer-width invariant
behind the old "marching footer" bug. A guard that cries wolf gets ignored.

### An honest note

The v0.1.68 notes claimed its fix worked. It didn't, and it was verified in a way
that could never have caught the problem: the tests built the data by hand and
checked it rendered, rather than running the app and watching what it recorded.
The bug was one line above the fix, in code no test touched.

It was found because a user hit a real failure, screenshotted it, and said the
message was still wrong. **A test that does not drive the real path proves
nothing about the real path.**

### Install
```sh
sha256sum -c checksums.txt
sudo dpkg -i gorilla-opencode_0.1.70_amd64.deb
gorilla-opencode --version && dpkg -l gorilla-opencode | tail -1
```
Both must say `0.1.70`. Quit any running copy first — an open session keeps the
old binary until you relaunch.

Full dual-track notes ship inside the package and are in the repo at
[`Changelogs/v0.1.70-release-notes.md`](Changelogs/v0.1.70-release-notes.md).
