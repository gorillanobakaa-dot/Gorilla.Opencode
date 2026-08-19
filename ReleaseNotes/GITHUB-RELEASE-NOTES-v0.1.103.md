# Gorilla OpenCode v0.1.103 — /arsenal is a window, and the screenshots to prove it

**Everything you need is on this page**, printed in full.

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.103_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.103_amd64.deb` |
| `gorilla-opencode-0.1.103-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer |
| `SHA256SUMS-v0.1.103.txt` | `sha256sum -c` |

Use `apt`, not `dpkg -i`. Restart the program if it is already running.

---

## Screenshots - every one of these is a real measurement, taken on a 2012 laptop

*Click any image for the full-resolution original. Unscaled, uncropped, nothing staged.*

**/arsenal - what the agent can do here, and what the rest would cost.** The header is measured on open, never claimed. The costs come from your own package manager, counted against what is already on your disk.

[![The /arsenal series list: 25 of 33 capabilities already present on this machine, 8 not, and a measured cost of 97.9 MB to download and 331.0 MB on disk for the two selected - about 3.4 hours at 8 KB per second](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-arsenal-series-measured-cost.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-arsenal-series-measured-cost.png)

**The install plan is the whole argument on one screen.** Nothing is installed by this program. It prints the command and gets out of the way.

[![The /arsenal install plan: 2 capabilities to add, 4 packages, 97.9 MB to download and 331.0 MB on disk, the exact sudo apt-get command, and the line - This program will not run it and will never ask for your password](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-arsenal-install-plan.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-arsenal-install-plan.png)

**Every key says what it did.** Pressing space on a group that is already installed used to do nothing and say nothing, which is indistinguishable from a broken key.

[![The /arsenal page reporting - All 8 of these are already installed, nothing to select - after space was pressed on a group that is fully present](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-arsenal-already-installed.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-arsenal-already-installed.png)

**Lines that behave.** These rules used to be 56 columns of box-drawing typed at a fixed length - too long for any narrower window, so each wrapped into two physical rows. Three on one screen meant three rows of debris. They are ASCII now and sized to the frame at render time.

[![The /research cost dialog in supervised mode: ASCII rules spanning the frame exactly one row each, 0.192 dollars per minute, about 0.431 dollars for the run, 18 sessions across 10 helpers and 8 auditors](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-research-supervised-cost-ascii-rules.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-research-supervised-cost-ascii-rules.png)

The same screen priced for four helpers. Note what it volunteers: which model the helpers will actually run on, that it is not the one you are chatting with, and that parallel buys time rather than answers.

[![The /research cost dialog in parallel mode: 0.128 dollars per minute, about 0.0958 dollars for the run, and the note that parallel is the same token cost as sequential](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-research-parallel-cost-ascii-rules.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-research-parallel-cost-ascii-rules.png)

**/osint explains itself before it spends anything.**

[![The /osint capability page explaining the five-stage intelligence cycle, the 985-source registry of which 866 are free and 370 need no account, and the two-axis A-F reliability and 1-6 credibility grading](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-osint-capability-page.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-osint-capability-page.png)

**It declined to burn money on a joke.** Pointed at a deliberately silly question with 10 agents, supervised, and told "the user chose these and they decide what this costs" - it refused, priced the refusal, answered for free anyway, and asked what the real work was.

*Read the caveat: one run, one model (Deepseek V4 Flash), one question. The arithmetic - roughly 20 sessions - is ours, from the tool's own description. The decision to refuse is the model's. It has not been tested across models.*

[![The agent declining a 10-agent supervised research run, explaining it would be roughly 20 LLM sessions and would waste the user money, then answering the question directly for free](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-refuses-expensive-run-on-a-joke.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.103/docs/screenshots/gallery/v0103-refuses-expensive-run-on-a-joke.png)

---

## Plain-language track

### What this release is

Three layout corrections to `/arsenal`, and the first proper photographs of the
new features. No new capability.

All three corrections came from someone using the program and sending
screenshots. None of them were visible to the test suite, and two of them were
not bugs at all — they were decisions of mine that looked fine in a test and
wrong on a screen.

### It was taking over your terminal

Every other page in this program draws itself as a **window**: it picks a
sensible width, leaves the sidebar visible, and has a border you can see.
`/arsenal` was the only one that took the entire terminal width.

Two things followed, and both got reported as bugs:

- It **covered the sidebar**, so the program looked like it had gone somewhere
  else entirely.
- Its border landed on the very edge of the screen, where a border is
  indistinguishable from **no border**.

It is a window now, like everything else.

### It was reserving the whole screen to show nothing

The list of capability groups is eight lines long. It was being padded out to
the full height of your terminal — so eight lines of content sat in a box with
thirty blank lines underneath, covering your conversation to display nothing.

That reads as a program that stopped halfway, and several of the "it's cut off"
reports were really that.

I had done it deliberately, reasoning that a frame which is always the same
height is more predictable. The rule that actually matters is **never taller
than your terminal**, not always exactly it. The box stops when the content
does now: **23 rows instead of 52**.

### And the notice that never finished

Fixed in the previous release, and now visible in the screenshots: pressing `p`
used to say `measuring with apt...` and never stop saying it, even once the
answer was on screen.

### The screenshots

`docs/SCREENSHOTS.md` now has a section for this release: the capability list
with real measured costs, the install plan with the exact command, the research
cost dialog with its rules drawing correctly at last, the `/osint` explainer,
and the agent refusing to spend money on a joke question.

Full size, unscaled, click through to the original. Every number on them is a
real measurement taken at that moment.

### One of them needs its caveat read

One screenshot shows the agent refusing a ten-agent research run on a silly
question, on cost grounds, after being told *"the user chose these and they
decide what this costs"*.

That is real and it happened. It is also **one run, one model, one question**.
The arithmetic it used — "roughly 20 sessions" — is ours, from the tool's own
description. The decision to refuse is the model's. It has not been tested
across models, and the write-up says so rather than letting you assume.

---

## Developer track

### Three corrections, all from screenshots

**1. Width.** `arsenal.go` computed `boxW := m.width - 2` while every other
dialog uses `dialogWidth(termWidth, preferred, chrome)`. Now
`dialogWidth(m.width, 120, 6)` — 120 rather than `/osint`'s 104 because the
entries carry long plain-language descriptions and legibility is their purpose,
but capped, so the frame is a window on anything wider.

`TestThePageIsAWindowNotATakeover` asserts the rendered width is strictly less
than the terminal at 120, 150 and 200 columns, and never exceeds it at 70.

**2. Height.** `View()` padded `b` out to `budget` so the frame was a constant
size. On a 52-row terminal the series list rendered 52 rows for 23 rows of
content. Removed. The budget still derives from `m.height` — that is what keeps
the trailing notice and the overflow marker inside a known bound — but the box
ends with the content.

`TestThePageFillsTheScreenExactly` relaxed from `== size.h` to `<= size.h`, and
`TestAShortPageDoesNotReserveTheWholeScreen` added, asserting the series list is
under 45 rows on a 52-row terminal. `TestTheNoticeAndTheOverflowMarkerLiveInsideTheBudget`
still pins the overflow case at exactly 14 rows on a 14-row terminal, which is
the case the arithmetic exists for.

**3. The stuck notice** (shipped in v0.1.102, photographed here).

### Why the tests could not see any of this

All three were **correct output that looked wrong**. A render assertion checks
what a component emits; it cannot check whether emitting that is a good idea. A
box that is exactly the terminal height passes every dimensional test ever
written for it and still covers your work with blank rows.

Third instance this session of the same class — the previous two were a
component whose output was clipped downstream by `PlaceOverlay`, and a marker
whose reserved width no longer matched its real width. Filed.

### The screenshots

Seven added to `docs/screenshots/gallery/` at capture resolution, written up in
`docs/SCREENSHOTS.md`, each clickable to its original, with alt text stating
what the image proves rather than what it depicts.

### Claim Sources

| Claim | Basis | Evidence |
|---|---|---|
| The page took the full terminal width | 📄 stated in input | `boxW := m.width - 2` in the previous commit; every other dialog uses `dialogWidth`. |
| 52 rows for 23 rows of content | 📄 stated in input | `TestAShortPageDoesNotReserveTheWholeScreen` logs the figure. |
| The notice never cleared | 📄 stated in input | User screenshot, fixed in v0.1.102, visible here. |
| The refusal happened as shown | 📄 stated in input | Screenshot; the model's reasoning is legible in it. |
| "20 sessions" came from our schema | 📄 stated in input | `research-tool.go`: "supervised roughly doubles that again". |
| Refusal generalises across models | 🚫 not established | One run, one model. Explicitly not claimed. |
| A window reads better than a takeover | 🤖 model inference | A judgement, made after seeing both. |
