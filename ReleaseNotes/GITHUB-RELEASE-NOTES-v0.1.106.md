# Gorilla OpenCode v0.1.106 — ten ways it could have lied to you

**Everything you need is on this page**, printed in full.

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.106_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.106_amd64.deb` |
| `gorilla-opencode-0.1.106-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer |
| `SHA256SUMS-v0.1.106.txt` | `sha256sum -c` |

Use `apt`, not `dpkg -i`. Restart the program if it is already running.

---

## Screenshots

*Click any image for the full-resolution original. Unscaled, uncropped, nothing staged.*

**The install plan** - measured against your own machine by your own package manager, with the exact command and nothing run on your behalf.

[![The /arsenal install plan showing 2 capabilities, 4 packages, 97.9 MB to download and 331.0 MB on disk, the exact apt-get command, and the line saying this program will not run it and will never ask for your password](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.106/docs/screenshots/gallery/v0103-arsenal-install-plan.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.106/docs/screenshots/gallery/v0103-arsenal-install-plan.png)

**Reading a screenshot.** Since v0.1.105 this is one `view` call rather than the model building its own OCR pipeline through the shell.

[![The agent reading the ARSENAL page text out of a PNG screenshot using tesseract, showing the capability list and package manager line transcribed from the image](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.106/docs/screenshots/gallery/v0105-before-ocr-through-bash-worked-slowly.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.106/docs/screenshots/gallery/v0105-before-ocr-through-bash-worked-slowly.png)

**Cost before you spend it**, and rules that draw correctly at any width.

[![The research cost dialog in supervised mode showing ASCII rules spanning the frame exactly one row each, with 0.192 dollars per minute and 18 sessions across 10 helpers and 8 auditors](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.106/docs/screenshots/gallery/v0103-research-supervised-cost-ascii-rules.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.106/docs/screenshots/gallery/v0103-research-supervised-cost-ascii-rules.png)

---

## Plain-language track

### The question that started it

The search tool in this program replaced three older ones, so it now runs on
almost every message. Yesterday it failed in a small, specific way, and the
obvious question followed: **what else can go wrong like that?**

So the whole tool got audited — not for crashes, but for **one particular kind
of lie.**

### The lie

> **"No matches found"** is a statement about *your code*.
> **"The filter didn't work"** is a statement about *the request*.

If a tool says the first when it means the second, the assistant believes it,
tells you your project doesn't have the thing, and never tries again. **You get
a confident wrong answer and no sign anything went wrong.**

Ten of those were found. Here are the ones worth knowing about.

### It said your project has no CI

Asking for `.github/**` returned **"No matches found"** — on a project with
three GitHub Actions files sitting right there.

Hidden folders are skipped by default, which is right; nobody wants `.git`
internals in every search. But when you *literally type* `.github/**`, you are
not being ambiguous. Now it finds them, while ordinary searches still skip
hidden files.

### A typo became a fact about your code

Filtering by language with a typo — `pyton` instead of `python` — returned
nothing. Not "unknown language". **Nothing.** Which reads as *"this project has
no Python in it."*

Now the search refuses to run and tells you the valid list.

### And then it caught me doing exactly that

The test I wrote to catch that bug failed immediately — **against my own work.**
I had typed the list of valid languages from memory and invented six that don't
exist. That is the same bug, committed while fixing the bug.

The list is now **read from the tool itself** rather than written down. A
hand-copied list of someone else's data goes stale silently, and always in the
direction of a wrong answer.

### It was reading your binaries out loud

Viewing a binary file printed its **raw bytes** into the conversation as if they
were source code. The size limit is 5 MB — so one mistake could dump five
megabytes of garbage into the conversation, **and it gets re-sent with every
message after that.**

Now it refuses, says what the file actually is (an executable, a `.deb`, a PDF,
a database), and points at the right tools for looking inside.

### Small ones that mattered

- **An empty file** and **a failed read** looked identical. Now the empty one
  says *"the read succeeded; there is nothing in the file."*
- **Reading past the end of a file** looked like an empty file. Now it tells you
  how long the file actually is.
- **Asking which files have uncommitted changes**, outside a git repository,
  said "no matches" — same as a clean repository. Different facts.
- **A web page whose content is built by JavaScript** came back blank, which
  reads as "this page is empty". Now it says the fetch worked, the text could
  not be extracted, and what to try instead.
- **The language-breakdown view timed out** on this project. It was counting
  lines inside 1.1 GB of release packages. **30 seconds → 1.6 seconds.**

### Two things deliberately left alone

**A shell command that fails** reports its exit code in the text without being
flagged as a tool error. That looks like the same bug and isn't: `grep` exits 1
for "no match", `test` exits 1 for "false". Flagging every non-zero exit as a
failure would turn correct answers into errors.

**Web search** already tells "every source failed" apart from "nothing found".
Fixed in an earlier release; noted so nobody re-derives it.

---

## Developer track

### Method

Hunt one shape, not crashes: **every path where a tool can report a filter
failure using the words of an empty result.** Twelve parameter combinations for
`find` exercised against the real engine; `view`, `fetch`, `bash` and
`web_search` reviewed the same way. Full record in
[`docs/FIND-AUDIT-2026-08-19.md`](docs/FIND-AUDIT-2026-08-19.md).

### find

| # | Finding | Fix |
|---|---|---|
| 1 | `glob=".github/**"` → "No matches found" with three workflow files present | `wantsHidden` — a path or glob naming a dot-path passes `--hidden --no-ignore-vcs`; intent-driven, ordinary searches unchanged, tested both ways |
| 2 | `pfind -t notalanguage` exits **0 with no output** | `checkType` refuses pre-search with suggestions and the valid list |
| 3 | My hand-written type list contained six languages the engine never had | Read from `--type-list`, cached, with a drift test |
| 4 | `modified_only` outside a git repo = "No matches found" | `insideGitRepo` (handles worktrees, where `.git` is a file) |
| 5 | Content search rooted at `$HOME` timed out at 30 s | `doomedContentSearch` refuses instantly, names `glob` |
| 6 | `view=code` timed out — counting lines in 1.1 GB of `.deb` | `--max-filesize 2M`; **30 s → 1.6 s** |

### view

| # | Finding | Fix |
|---|---|---|
| 7 | Binary files dumped raw bytes; 5 MB limit, re-sent every turn | `binaryFileKind` — magic numbers + NUL byte; names the format, points at `file`/`strings`/`xxd`/`7z l`/`readelf`/`binwalk` |
| 8 | Empty file = failed read = `<file>\n\n</file>` | Explicit "0 bytes, the read succeeded" |
| 9 | Offset past end = empty file | Reports the real length via `countFileLines`, because `readTextFile` resets its counter to the requested offset |

### fetch

| # | Finding | Fix |
|---|---|---|
| 10 | JS-rendered page = blank page | Reports status, byte count, the usual cause and three routes onward |

### Not changed, with reasoning

`bash` reports a non-zero exit in the text without `IsError`. `grep`/`test`/
`diff` all exit non-zero for legitimate results; flagging them would convert
correct answers into errors. `web_search` already separates backend failure
from absence.

### Verification

Full suite green. Capability chain proven deterministically: `find` → `view` →
write → `go run`, printing `42`. A live model run confirmed `edit` (rewriting
`add()` to subtract) before the shared endpoint went cold; the remaining live
steps were blocked by provider warm-up rather than by any change here, and the
suite covers them.

### Claim Sources

| Claim | Basis | Evidence |
|---|---|---|
| `.github/**` returned no matches | 📄 stated in input | Probe against the real engine, before and after. |
| `pfind -t notalanguage` exits 0, no output | 📄 stated in input | Run directly at the shell. |
| Six invented languages in my own list | 📄 stated in input | The drift test's failure output. |
| 30 s → 1.6 s on `view=code` | 📄 stated in input | Timed both ways; 1.1 GB measured with `du`. |
| Binary bytes reached the conversation | 📄 stated in input | Probe output, quoted in the audit doc. |
| Capability chain intact | 📄 stated in input | Deterministic test printing 42. |
| Ten findings is most of what is there | 🤖 model inference | One afternoon, one shape, four tools. Not exhaustive. |
| The `bash` exit-code decision | 🤖 model inference | A judgement, stated so it can be argued with. |
