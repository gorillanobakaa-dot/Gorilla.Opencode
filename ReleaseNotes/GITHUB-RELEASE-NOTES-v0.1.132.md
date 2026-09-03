# Gorilla OpenCode 0.1.132 — the Linux build

**0.1.130 was the Windows build.** It compiled for Linux and passed the static
checks there, and that was the entire extent of the evidence. Nobody had run it.

This is that same work, executed on Linux for the first time, plus what the first
run turned up. Previous Linux release: **0.1.119**.

---

## Why this release exists

A branch that cross-compiles is not a branch that works. `GOOS=linux go build`
succeeding tells you the types line up. It tells you nothing about whether a
shell wrapper written against PowerShell can report an exit code through bash,
or whether a test that passed on a bare Windows box passes on a laptop that
already has the tools installed.

So the first thing that happened on Linux was the test suite, and it failed in
six places.

**Not one of them was a bug in the program.** Every failure was a test or a test
harness asserting something that could only ever be true on Windows. The product
code needed no changes at all. That is a good result, but it is only knowable by
running it, which is the point.

Three of the six are worse than a plain failure: they were passing for the wrong
reason, and had been for weeks.

---

## What the first Linux run found

**A shell test that could not have passed here.** It ran `exit 7` and expected
the exit code back. But the shell is persistent, so `exit` ends the session
rather than a subshell, the status file is never written, and the program
deliberately returns 1 with an explanation. The Windows half of the test never
hit this because `cmd /c "exit 7"` is already a subprocess. The POSIX equivalent
is `(exit 7)`, a subshell, and the round trip works.

**A path test that asserted a Windows fact.** `~\Documents\my-project` must
expand on Windows. On Linux a backslash is an ordinary character in a filename,
so that string is one segment genuinely named `\Documents\my-project`, and
resolving it any other way would break anyone who has a backslash in a name. The
program was already right; the test was asserting for both platforms what is
true of one.

**Two `/arsenal` tests that assumed a bare machine.** The helper picked the first
package the system could install without asking whether it was already
installed. Selecting an installed package is deliberately a no-op, so on a
machine that already had the tool the test selected nothing, the install plan
correctly refused to open for an empty selection, and the failure described the
symptom. It would have failed the same way on a Windows box with the tools
already present.

**Three smoke checks that were grading an error message.** This is the one worth
reading twice. The checks invoked the program as:

```sh
env -i HOME=... PATH=... TERM=dumb runbin "$BIN" -p hi -q
```

`env` executes a *program*. `runbin` is a shell *function*, so `env` could not
see it. Every one of those runs started nothing at all and exited 127, and the
checks graded `env`'s own complaint: "did it exit non-zero?" passed on 127, "is
the cryptic error gone?" and "is there no usage dump?" both passed because
`env: 'runbin': No such file or directory` contains neither. Only the one check
asserting that expected text was *present* failed, and that is what exposed it.

It stayed hidden on Windows because a local model server was listening, which
made the enclosing block skip. The fix is to nest the other way round, so the
timeout wrapper is outermost. The same check now reports the program's real exit
status of 1.

**A packaging check demanding the return of a bug.** It searched
`build-deb.sh` for an inlined desktop-entry line. That inlined copy is exactly
what v0.1.44 deleted, because keeping the launcher in three places is how the
package once shipped without its plain-mode action. The check now resolves the
tracked `.desktop` file the script actually installs and asserts on that.

---

## The low-bandwidth mode line

Pressing `l` in `/context` applies the low-bandwidth preset. It was reported as
doing nothing.

It was doing exactly what it promises: seven components switched off, written to
disk, undo recorded. But of those seven, six sat below the visible window of a
forty-two row list, so nothing on the screen being watched changed. The only
acknowledgement was a message that appears once and fades.

A message that has already gone cannot tell you what state you are still in.
`/context` now carries a persistent line, in the theme's error colour, for as
long as the preset is applied:

```
LOW-BANDWIDTH MODE: 7 components switched off. Press u to put them back.
```

It names the count and the way out. It costs a row only while the preset is
applied, and it deliberately survives on a short terminal, where the
explanatory header lines are dropped first: a cramped screen shows fewer rows,
so that is precisely where the least of the change is visible.

This is the third instance of one mistake in this part of the program, after two
fixes in August. Stated generally, so the fourth is recognised: **when the effect
of a key can land off-screen, say what happened, do not merely do it.**

---

## The permission gate in front of your git tree

`patch_port` rewrites git trees. One line decides whether it asks first:

```go
if op != "inspect" {
```

`inspect` is read-only. It reaches the tree through exactly one call,
`git apply --check`, which asks git whether a patch *would* apply and writes
nothing. Everything else, `forward-port`, `backport`, `rebase`, `refresh` and
`port-series`, can rewrite a checkout, and each one asks first and names the
tree and the operation in the prompt.

That was all correct, and **nothing tested any of it.** The file had no tests at
all. Refactor that condition the wrong way and the whole suite still passes while
five operations start rewriting trees unannounced.

Six tests now pin it: `inspect` does not ask, an omitted operation defaults to
the safe side, all five modifying operations ask exactly once with the tree named,
denying actually aborts, the operation approved is the operation that runs, and
an unrecognised operation is refused without asking about something that will
never happen.

The set of modifying operations is declared in the test, so adding a seventh
operation without gating it fails the suite until somebody gates it.

**On proving a negative.** Two of those tests assert that permission was *not*
requested, and the permission check sits after two steps that can return early.
So "nobody asked" is also true if the tool died before reaching the gate, and
both tests would pass having exercised nothing. Neutering the gate proves only
that the positive tests work. The mutation that matters is the opposite one:
force the gate to fire when it should not, and confirm the negative tests go red.
They did.

---

## The command reference uses the window now

`/help` capped itself at 84 columns. On a 200-column terminal that is a narrow
panel floating in the middle of the display, showing about a dozen of
thirty-one commands, with nothing to say the list continued. Reported plainly:
someone who does not already know a reference is scrollable reads the bottom row
as the end of what the program can do.

The answer is to show the commands rather than announce them. Full width, two
columns, and on a normal-sized terminal **every command is on screen at once**
with no scrolling at all.

- **tab**, and the left and right arrows, move between the columns, the way the
  old Slackware installer moved between panes.
- Columns are balanced while everything fits, and only fill top-to-bottom once
  there is more than a screenful. Filling in order first put every row in the
  left column and left the right half empty, which is the same wasted window
  moved to the other side of the screen.
- The header now counts what exists, so the size of the list is a fact on the
  page rather than something you discover by scrolling.
- The border is gone. It drew a rounded box around the whole screen, spending
  two rows and two columns of a fixed budget to put a line just inside an edge
  the terminal already has. Those are also box-drawing characters, which are
  East Asian Ambiguous and measure two columns instead of one on a terminal
  configured for CJK: the one piece of decoration here that could change width
  on somebody else's machine.

---

## Screenshots

<!-- SCREENSHOTS PENDING.
     This section is deliberately empty and the release guard is deliberately
     failing because of it. Directive 13: screenshots are part of the
     deliverable, and a release page without them is incomplete work.

     Each one goes in as a linked image, pinned to the tag v0.1.132, pointing at
     docs/screenshots/gallery/NAME.png on raw.githubusercontent.com, wrapped in a
     link to that same URL so it opens full size. Never a width attribute, never
     a /main/ path, and the alt text describes what the picture PROVES.

     Three to capture, and what each has to prove:

       v0132-low-bandwidth-mode-line.png
         /context with the preset applied, the red LOW-BANDWIDTH MODE line
         visible at the top and the tool list below it.

       v0132-patch-port-inspect-no-prompt.png
         patch_port running an inspect and reporting how each patch would apply,
         with no permission prompt, because it cannot change anything.

       v0132-patch-port-permission-prompt.png
         the same tool asked for a forward-port, stopping on the permission
         dialog, with the tree path and the operation named in it.

       v0132-help-two-columns-full-window.png
         /help on a wide terminal, filling it, both columns populated, the
         header line showing the command count. The point it proves is that
         the whole reference is visible without scrolling.

     The A/B of the last two is the argument: the gate is not "does it prompt",
     it is "it prompts for exactly the operations that can write".
-->

---

## Install

**Debian, Ubuntu, Mint** — use `apt`, not `dpkg -i`. The package depends on
`lynx`, `python3` and `ripgrep`, and `dpkg -i` resolves no dependencies at all,
which would leave you with a program whose search does not work.

```sh
sudo apt install ./gorilla-opencode_0.1.132_amd64.deb
```

**Arch, CachyOS** — either the pre-built package:

```sh
sudo pacman -U gorilla-opencode-0.1.132-1-x86_64.pkg.tar.zst
```

or from source, which needs a Go toolchain:

```sh
git clone https://github.com/gorillanobakaa-dot/Gorilla.Opencode
cd Gorilla.Opencode/packaging
makepkg -si
```

**Any Linux, no package manager** — the stripped binary runs as it is:

```sh
chmod +x gorilla-opencode-v0.1.132-linux-amd64
./gorilla-opencode-v0.1.132-linux-amd64
```

Verify what you downloaded against `SHA256SUMS-v0.1.132.txt`:

```sh
sha256sum -c SHA256SUMS-v0.1.132.txt
```

---

## What was measured, and what was not

Everything below was produced by a command on the machine that built this
release, a 2012 quad-core laptop running current Debian.

| claim | evidence |
|---|---|
| Full Go test suite green | `go test ./internal/... -count=1`, exit 0 |
| Patch porting works against distro `patch` | `test_patch_port.py`, 4 of 4, GNU patch 2.8 |
| Smoke suite green | `tests/smoke.sh`, 13 of 13 against the installed binary |
| Installed binary is the built binary | `sha256sum` of `/usr/bin/gorilla-opencode` and the build output match |
| Package version matches the binary inside it | `dpkg-deb -f` and the inner binary's `--version` both read 0.1.132 |
| Every new test can fail | each mutated deliberately, then reverted |

**Not verified.** macOS, which has never been built for or run. Any 32-bit
target. The `/review` path against a full analyser set on a machine other than
this one. Whether the deferred tool loading behaves the same on a model larger
than the one it was measured against, which remains the largest open question in
this branch and is unchanged by this release.

Token and byte figures shown inside `/context` are estimates from schema bytes
divided by four. Measured against a real tokeniser they run about 10 percent
high. They are good enough to choose which rows to switch off and are not good
enough to quote as a bill, and the screen says so itself.
