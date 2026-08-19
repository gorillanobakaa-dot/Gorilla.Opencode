# Gorilla OpenCode v0.1.100 — /arsenal says what it did

**Everything you need is on this page**, printed in full.

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.100_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.100_amd64.deb` |
| `gorilla-opencode-0.1.100-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer |
| `SHA256SUMS-v0.1.100.txt` | `sha256sum -c` |

Use `apt`, not `dpkg -i`. Restart the program if it is already running.


## Plain-language track

### What was wrong

v0.1.99 added `/arsenal`. Within minutes of it shipping, the first person to
use it reported that pressing **space** selected nothing and pressing **p**
showed no costs.

Both keys were working perfectly. That was the problem.

The page opens with the cursor on the first group — "The minimum" — and on that
machine all eight of those tools were **already installed**. So space correctly
selected nothing, because there was nothing to select. Then `p` correctly
priced an empty selection, because nothing had been selected.

Two keys in a row doing exactly the right thing, and looking completely broken.

### Why this is the same bug this project keeps writing about

There is a rule in this project's own instructions: **silence and success must
never look alike.** A program that does the right thing and says nothing is
indistinguishable from a program that is broken — and the user is left guessing
which one they have.

The behaviour was right. The missing feedback was the bug.

Now every keypress reports itself:

- *"All 8 of these are already installed — nothing to select."*
- *"Nothing selected yet — space takes what the cursor is on, a takes everything."*
- *"selected 2 (3 already installed, skipped) — press p to measure the cost."*

And then the cost appears, as it always did: **97.9 MB to download, 331.0 MB on
disk, about 3.4 hours** for the two document tools that machine was missing.

### The part nobody would ever have noticed

While fixing that, a second defect turned up that nobody reported and no test
covered.

Four of the keys were written in a way where the program **might** have thrown
away the work they had just done, depending on the compiler. It happened to work
correctly with the compiler in use — which is the worst kind of working:
right by accident, on one machine, with nothing able to tell you when that
stops being true.

Written explicitly now, with a test.

### Also

The tool-detection fix from v0.1.99 is visible in the numbers: this machine now
reports **25 capabilities present** instead of 24, and the coding group reads
4 of 4 instead of 3 of 4 — because a tool that answers to two different names
is no longer reported as missing when it is installed under the other one.

---

## Developer track

### Scope

One commit. Feedback in `internal/tui/components/dialog/arsenal.go`, no change
to what any key actually does.

### The reported defect

`toggle()` skipped entries where `status[id].Present` and returned silently.
The first series is `minimum`, 8/8 present on the reporter's machine, so the
first keypress of a brand-new feature hit the one path with no output.

`price()` had the same shape: an empty selection returned `nil` with no notice.

Both now set `m.notice`, distinguishing three cases — nothing selectable
because everything is present, nothing selected yet, and a real selection with
a count and how many were skipped.

**Why no test caught it:** every existing test called `toggle()` on entries
deliberately chosen to be *missing*, because that is the interesting case. The
uninteresting case was the one the user hit first. Three tests added, all of
which fail against the previous commit.

### The latent defect

```go
return m, m.price()
```

The Go spec orders function *calls* left to right within a return statement but
does not pin down when a plain operand such as `m` is read. `m` could therefore
be copied before `price()` mutates it, discarding every field the method set.
`gc` evaluated it in the working order; nothing guaranteed that, and no test
could observe it. Four sites rewritten as an explicit two-statement form, with
`TestKeyHandlersMutationsSurviveTheReturn`.

### Full screen, and two width bugs found by going there

My first fix reserved rows for the trailing notice. The owner's answer was
better: *"you can always make the window either bigger or full screen. That
solves the problem."*

It does, and it removes the question rather than managing it — the row budget
is fixed and known before anything renders, and the content scrolls inside it.
It also reads far better: this page is a map of long descriptions, and a
centred box two thirds the width of the terminal was throwing away the space
that makes it legible.

Going full screen immediately exposed two real width bugs that the
content-sized box had hidden:

1. **The box was told the wrong width.** lipgloss counts padding INSIDE
   `Width()`, so passing the *text* width to the box made it 4 columns too
   narrow for its own content — and lipgloss WRAPS rather than overflowing, so
   every line silently became two. Measured at 60×20: a **27-row frame in a
   20-row terminal**.
2. **Truncation counted runes, not columns.** An em-dash is one rune and one
   column; CJK and emoji are one rune and *two*. `truncateToWidth` now measures
   with `lipgloss.Width`. The notice line was not truncated at all, which put
   the frame one row over on its own.

`TestThePageFillsTheScreenExactly` asserts the frame is *exactly* the terminal
height — not merely "not taller" — across four views and five sizes down to
60×20, with a notice set and content overflowing.

### Verification

Live, in a detached screen against the real binary: space on the fully
installed series prints *"All 8 of these are already installed — nothing to
select."*; down-arrow to Documents and space prints *"selected 2 (3 already
installed, skipped)"*; `p` then reports 97.9 MB / 331.0 MB / about 3.4 hours at
8 KB/s. Full suite green.

### Claim Sources

| Claim | Basis | Evidence |
|---|---|---|
| space and p appeared broken | 📄 stated in input | User report against v0.1.99. |
| Both were behaving correctly | 📄 stated in input | Reproduced headlessly; the first series is 8/8 present. |
| 97.9 MB / 331.0 MB / ~3.4 hours | 📄 stated in input | Live run, apt-get --print-uris via the page. |
| 25 present, coding 4/4 | 📄 stated in input | Live run header after the detection fix. |
| The return statement could discard mutations | 🤖 model inference | Read from the Go spec's evaluation-order rules; not observed failing on this toolchain. |
| 27-row frame in a 20-row terminal | 📄 stated in input | Measured with a debug render harness before the width fix. |
| The page is exactly the terminal height | 📄 stated in input | `TestThePageFillsTheScreenExactly`, 4 views × 5 sizes. |
