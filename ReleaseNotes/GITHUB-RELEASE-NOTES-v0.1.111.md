# Gorilla OpenCode v0.1.111 — the meter got its colours back, and a 204 KB file that could eat 420 MB

Everything about this release is printed in full on this page, including the
pictures.

---

## What this release is

Three repairs. Two of them undo damage done by earlier passes that looked
helpful, and the third closes a memory hole nobody had noticed.

No new features. If v0.1.110 works for you, this one fixes things you may not
have known were broken.

---

## The quota meter is back

Two days ago a tidy-up pass swept 81 files replacing anything that "looked like
a box character" with plain keyboard symbols. The request behind it had been
narrow — fix some misaligned lines in the prompt given to the model. The sweep
went far past that, and one of the things it flattened was the quota meter.

`[████░░░░]` became `[####....]`. A solid block reads as a **meter**; a row of
`#` reads as **text**. The colour maths was never touched — it was still
computing a red-to-green gradient, cell by cell, into a body that no longer
existed to show it.

[![The quota panel showing two weekly limits as solid horizontal bars, the top one full and running smoothly from red through yellow to green across its whole length, the lower one at 29.71 percent showing only the red and orange portion of the same gradient](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.111/docs/screenshots/gallery/v0180-usage-30pct-few-bananas.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.111/docs/screenshots/gallery/v0180-usage-30pct-few-bananas.png)

[![The same panel with the second group down to 4.88 percent, its bar reduced to a single red cell beside the message that the last banana has been spotted and nobody should make any sudden prompts](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.111/docs/screenshots/gallery/v0180-usage-5pct-last-banana.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.111/docs/screenshots/gallery/v0180-usage-5pct-last-banana.png)

**These two captures are from before the sweep, and that is the claim being
made.** The bar-drawing code at this tag was diffed against the version in those
pictures and is identical in behaviour — same filled glyph, same trough glyph,
same per-cell colour. The meter you get today is the meter in those photographs.

### Why it was allowed to happen, and why the rule did not apply here

The project genuinely does avoid box-drawing characters in most places: they are
East Asian *Ambiguous* width, so a terminal set up for CJK renders them two
columns wide, and the layout library then mis-measures a frame and **wraps** it —
damage that appears as unexplained height somewhere else entirely.

That hazard is real for chrome that wraps content. This panel is printed to
scrollback, never into the frame, and works out its own width. Worst case on a
CJK terminal is one wrapped line that scrolls away.

And the trough character, `░`, turns out to be East Asian **Neutral**. It never
carried a width risk at all.

### It is now nailed down

Three tests that fail loudly if this happens again:

| Test | What it refuses to allow |
|---|---|
| the bar keeps its solid body | the glyphs, and that `#`/`.` are absent |
| the gradient still runs red to green | the endpoints **and** that cells differ, so the ramp cannot be flattened to one colour while still being called a gradient |
| all nine banana rungs survive | every rung word for word, and that at least seven stay reachable, so the ladder cannot be collapsed while the strings sit unused in the file |

Both locks were verified **by breaking them on purpose**. The first draft of one
of them was a no-op — it searched for the literal glyph while the source stores
the escape — which is exactly how a guard ends up looking like it works.

A future change can still alter any of this. It just has to delete the lock
deliberately and say why. An accident cannot; only a decision can.

---

## A compression bomb, closed

A 204 KB archive can be built to expand into 200 MB of data. The search tool's
archive mode decompressed members straight into memory with no limit.

**Measured, on a real bomb file:**

| | decompressed | peak memory |
|---|---|---|
| before | 200.0 MB | **420.7 MB** |
| after | 64.0 MB | **148.9 MB** |

420 MB of memory from a 204 KB file. On the 4 GB laptops this program is built
for, that is a swap storm. A bigger bomb is an out-of-memory kill.

The existing file-size guard did not help: it checks the size **on disk**, which
for a compressed file is the small number.

**The budget is shared across the whole archive, not per file inside it.** A
per-file cap stops one huge member and does nothing about ten thousand small
ones — verified against a 2000-member tar, which the shared budget handles
correctly.

**And it is loud when it bites.** It names the file and the limit that stopped
it. Truncating quietly would make "found nothing" and "I stopped reading"
identical, which is the failure this project cares most about.

Default 256 MB. `PFIND_MAX_DECOMPRESSED_MB` overrides it; `0` turns it off for
someone who means it. Normal archives are unaffected.

### Stated plainly so this is not mistaken for an emergency

**The model cannot reach this.** Archive search is off by default and the tool
interface never exposes it. Only a person running the search script directly with
`--search-zip` could hit it. It was fixed anyway, because the standalone copy of
that script is exactly that person's tool and had the identical hole. All three
copies are now hash-verified identical.

### Two bugs in the fix, both caught by testing rather than reading

- The budget charged for bytes it was **offered** instead of bytes it actually
  **read**, so a 1 MB read attempt against a 100 KB archive member burned 1 MB.
  A 64 MB budget cut a perfectly legitimate archive off after 3 MB.
- The warning printed the module's default limit while a smaller override was in
  force — naming a number the run never used. A message that lies about its own
  cause is worse than no message.

---

## Six screenshots that had been broken links since 17 August

A commit in v0.1.90 deleted six gallery images and added eight unrelated ones in
the same change. The gallery page still referenced four of the six, so it has
been showing broken images on GitHub for three days.

Recovered byte-for-byte from history and verified as valid PNGs. The whole
gallery was audited afterwards: no other image is missing and no other document
carries a broken reference.

Found by the owner noticing, not by any test. A guard that fails when a
documentation image reference does not resolve would have caught it the day it
happened.

---

## Files that were never closed

Four places opened a file and trusted the language to close it. Three leak a
handle. The fourth is a **write**, which can exit with data still in a buffer —
and a half-written catalogue looks exactly like a complete one.

All four now close explicitly.

The audit covered all twenty Python files. The only one that actually ships —
embedded in the binary and in the package — was already clean: no unclosed
handles, no un-timed subprocess calls, every whole-file read inside a `with`
block.

---

## What was deliberately NOT changed

Six commands that run other programs still have no time limit on them. A blanket
fix there would be the **same mistake as the sweep that flattened the meter** —
right-looking, applied by pattern, wrong exactly where it matters.

Decided one at a time instead:

| Where | Decision |
|---|---|
| runs a just-built binary | **60s** — a broken build waiting on input would hang the release forever |
| local archive extraction | **180s** |
| local version probe | **60s** |
| documentation round-trip test | **300s** |
| the general command runner | **optional, off by default** — it runs `git push` and uploads a 20 MB release asset, which on a single-digit-KB/s link legitimately takes many minutes |
| installs system packages under `sudo` | **none** — `sudo` may be waiting for you to type a password, which is a human thinking rather than a hang, and killing a package installation halfway through breaks the system's package database |

---

## A packaging bug found while cutting this release

Both packaging scripts had quietly stopped putting the current release's notes
**inside the package**. The dual-track documents were split into two files a few
releases ago, and the pattern that collects them was still anchored to the old
single-file name — so it matched neither.

It stayed invisible because the folder still looked full: sixty older releases'
notes were all sitting there. Checking that *something* was present passed.
Checking that *this release* was present did not.

Fixed in both the Debian and the Arch package, and verified in the built
artifacts. **This is the second time that one line has dropped the current
release's prose** — the comment above it already recorded the first. A naming
convention changed underneath a pattern anchored to the old one, and the pattern
kept passing.

### The Arch package was missing a working search tool

Diffing the two built packages file by file turned up a second thing: the Arch
package shipped no `pfind` at all, and declared only one of its three
dependencies.

The missing dependency is the one that matters. The `find` tool — which replaced
`ls`, `grep` and `glob` — **refuses to run without Python**, and the Arch package
never said it needed it. `pacman` would report a clean install and the agent
would then have no search at all.

The Debian dependency list could not simply be copied across: Arch calls that
package `python`, not `python3`. Copying it verbatim would have turned a silent
gap into a package that will not install.

Fixed, and verified by downloading the published package back and running its
search engine on a real file.

**The Debian package was never affected.** If you are on Debian or Ubuntu,
nothing here changes for you.

---

## What is NOT verified

- The meter was confirmed by diffing the drawing code against the pre-sweep
  version and by headless tests. It has not been re-photographed since the
  restore; the two pictures above are the pre-sweep captures.
- The decompression cap was measured against a purpose-built bomb, a
  2000-member tar, a raw gzip stream, the opt-out, and a normal small archive.
  It has not been run against a large real-world archive in the field.
- One unrelated test fails when tests run in random order. It fails the same way
  on the previous release.

---

## Install

**Debian / Ubuntu**

```sh
sudo apt install ./gorilla-opencode_0.1.111_amd64.deb
```

`apt` rather than `dpkg -i`: the package depends on `lynx`, which is what makes
web search work with no setup, and `dpkg -i` resolves no dependencies at all.

**Arch / CachyOS**

```sh
sudo pacman -U gorilla-opencode-0.1.111-1-x86_64.pkg.tar.zst
```

Verify what you downloaded against `checksums.txt` before installing.
