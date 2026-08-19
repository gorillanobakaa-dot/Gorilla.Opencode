# Gorilla OpenCode v0.1.107 — the one I said was fine

**Everything you need is on this page**, printed in full.

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.107_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.107_amd64.deb` |
| `gorilla-opencode-0.1.107-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer |
| `SHA256SUMS-v0.1.107.txt` | `sha256sum -c` |

Use `apt`, not `dpkg -i`. Restart the program if it is already running.

---

## Screenshots

*Click any image for the full-resolution original. Unscaled, uncropped, nothing staged.*

**What the program can do here, and what the rest would cost** - measured against your own machine, with nothing run on your behalf.

[![The arsenal install plan showing two capabilities, four packages, 97.9 MB to download and 331.0 MB on disk, the exact apt-get command, and the line saying this program will not run it and will never ask for your password](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.107/docs/screenshots/gallery/v0103-arsenal-install-plan.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.107/docs/screenshots/gallery/v0103-arsenal-install-plan.png)

**Cost stated before it is spent.** The same honesty this release is about, applied to money instead of errors.

[![The research cost dialog in supervised mode showing the price per minute, the cost of this run, and 18 sessions across 10 helpers and 8 auditors, with ASCII rules spanning the frame](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.107/docs/screenshots/gallery/v0103-research-supervised-cost-ascii-rules.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.107/docs/screenshots/gallery/v0103-research-supervised-cost-ascii-rules.png)

---

## Plain-language track

### What this is

Yesterday's release audited four tools for one kind of mistake and fixed ten of
them. In the notes I wrote that two tools were **checked and deliberately left
alone**.

The owner's reply was, in effect: *check those too.*

One of them was fine. The other had a bug that cost **a full minute, every
time**, and then lied about what happened.

### The bug

Ask this program to run a shell command that ends with `exit` — something like
`build.sh || exit 1`, which is an ordinary thing to write — and here is what
happened:

1. The command ran. It worked. Its output was captured.
2. The shell it ran in **shut itself down**, because `exit` does that.
3. The program waited for a result that was never coming.
4. **Sixty seconds later** it gave up and reported:
   *"Command execution timed out or was interrupted."*

Every part of that message is wrong except the waiting. It did not time out. It
was not interrupted. It had already finished, successfully, before the wait
even started.

### Why that matters more than a minute

An assistant told "this timed out" does the sensible thing: **it tries again.**
Or raises the time limit. Or tells you the machine is hanging.

All of which are the wrong fix, arrived at after a minute of waiting, for a
command that had already worked.

It now comes back **instantly** and says what really happened: the shell session
ended, `exit` is the usual reason, the output above is real, and the exit code
was lost with the shell. The next command gets a fresh shell.

**Sixty seconds, down to a hundredth of one.**

### The other one really was fine

Web search distinguishes **three** situations, not two, and it turns out to be
the most careful tool in the program:

- **every source failed** → *"Search FAILED — nothing was retrieved. Tell the
  user the search failed; do not substitute remembered citations."*
- **nothing found, but some sources were down** → *"COVERAGE WAS INCOMPLETE, so
  this is not evidence that nothing exists."*
- **results found, some sources down** → *"PARTIAL: results below are
  incomplete."*

That is the standard the rest of this week's work has been trying to reach.
Nothing to change.

### The honest bit

I did not test the shell tool before saying it was fine. **I reasoned about it,
decided it made sense, and wrote that in the release notes.**

That is the second time in one day. Earlier, the test I wrote to catch a bug
caught me committing the same bug in my own list of programming languages.

Reasoning about what a program does is a guess. Running it is the answer. Both
of today's mistakes were the first kind pretending to be the second.

---

## Developer track

### Finding 11: `exit` in a command kills the persistent shell

`shell.go` runs the command through `eval`, which executes in the **current**
shell — so `exit N` terminates the persistent shell rather than a subshell. The
status file is never written; the watcher polls until the default one-minute
timeout elapses and returns `interrupted`.

The output is unaffected — stdout and stderr are redirected to files and were
already flushed. Only the exit code is genuinely lost.

Fixed in the watcher: it now checks `s.isAlive` alongside the status file and
returns immediately when the shell has gone, with a message naming `exit` as
the cause and stating that the captured output is real.

**MEASURED: 60 s → 0.01 s** (`TestACommandThatCallsExitIsNotReportedAsATimeout`
fails above 20 s).

### What was genuinely sound, now under test rather than assertion

`TestOrdinaryFailuresReportTheRealErrorAndExitCode` pins the behaviour I had
only argued for: a non-zero exit appends `Exit code N` without setting
`IsError`, because `grep`(1) `test`(1) `diff`(1) are all legitimate results.
`ls` on a missing path → real message + `Exit code 2`; unknown command →
`command not found` + `Exit code 127`; failing `go vet` → its diagnostics +
`Exit code 1`.

`TestTheShellRecoversAfterACommandCallsExit` covers the respawn.

### web_search

Re-read after being wrong about `bash`. Three states, each with distinct
wording, including an explicit instruction not to substitute remembered
citations on failure. No change.

### Claim Sources

| Claim | Basis | Evidence |
|---|---|---|
| `exit` kills the persistent shell | 📄 stated in input | `eval` runs in the current shell; reproduced directly. |
| 60 s → 0.01 s | 📄 stated in input | Timed in `TestACommandThatCallsExitIsNotReportedAsATimeout`. |
| Output survives the shell's death | 📄 stated in input | stdout redirected to a file before the shell exits; asserted in the test. |
| Ordinary non-zero exits report correctly | 📄 stated in input | Three cases run against the real tool. |
| web_search handles three states | 📄 stated in input | Read from source, quoted in the audit doc. |
| Not flagging non-zero exits is right | 🤖 model inference | A judgement about `grep`/`test`/`diff` semantics. |
| Eleven findings is most of what is there | 🤖 model inference | One shape, five tools, one day. Not exhaustive. |
