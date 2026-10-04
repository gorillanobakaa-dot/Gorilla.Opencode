# Gorilla OpenCode 0.1.135 — Windows and Linux

**The AI now notices when it goes round in circles, your passwords stay on your machine, and a code review that reviewed nothing says so.**

Previous release: **0.1.134** (Windows only). The last Linux build was **0.1.132**.

**Read this first if you are on Linux.** The Linux files on this page were built
on a Windows computer. They were inspected file by file. They were **not
installed or started on a Linux machine** before publishing, and the `.rpm` is
the first this project has ever published. The Windows file was built and run on
Windows 11.

---

## Why this release exists

Four other AI coding programs were read to see what they do that this one did
not: Alibaba's Open Code Review, the OpenHands agent SDK, goose, and the
DeepSeek Harness.

One rule decided what was taken: **it must run on your own computer, with no
account, no extra key, and nothing sent to anybody's server.** Eight ideas
passed. They were rewritten for this program; nobody's code was copied. The one
exception is a set of plain-text checklists, which are included unchanged under
their own licence.

Along the way a real fault turned up in the built-in code checker. It comes
first below because it is the worst.

---

## What changed

### A code review that reviewed nothing now says so

The checker runs a list of inspectors, small separate programs that each look
for one kind of mistake. On a computer where those inspectors are not installed,
the old report said this:

```
- Analysers that ran: 18 (...)
- NOT INSTALLED, so they never ran: 17 (...)
All findings: 0
```

The same names were in both lists. An inspector counted as having run because it
had been *scheduled*. So a report that looked like a clean bill of health was an
empty room.

Now an inspector has run only if it finished a job, and the report opens with
the verdict:

```
- **NOTHING WAS REVIEWED.** No analyser completed a single job. The empty
  findings list below means nothing ran, not that the code is clean. Do not
  report this code as reviewed.
```

or, when some of it worked:

```
- **PARTIAL REVIEW.** 1 of 7 scheduled jobs completed.
- Analysers that ran: 1 (go-vet)
- **NOT INSTALLED, so they never ran: 6 (cloc, gitleaks-worktree,
  golangci-lint, gosec, semgrep-fast, staticcheck)** — the code they cover is
  UNREVIEWED
```

That second block is real. It is what this version printed on the maintainer's
own computer on 4 October 2026, where one inspector of seven is installed. The
old version would have listed all seven as having run.

### The AI is told when it repeats itself, and then stopped

A small AI model sometimes asks for the same thing again and again. Every repeat
costs data, and on a paid plan it costs money.

After the third identical request, the AI reads this at the end of the answer:

```
YOU ARE REPEATING YOURSELF: the view call has now been made 3 times in a row
with the same arguments and the same result. Calling it again will return the
same thing. Use the result you already have, or do something different, or
finish and report.
```

If it carries on, the turn ends at the fifth and you read this:

```
Stopped because the model was going round in circles: the view call has now
been made 5 times in a row with the same arguments and the same result. Nothing
was lost — everything up to here is recorded. If the repeats were intended, say
"continue"; otherwise tell it what to try instead.
```

Three patterns are watched: the same request with the same answer (told at 3,
stopped at 5), the same request failing (3 and 5), and two requests alternating
(6 and 10). Only exact repeats count. Reading a long file page by page, or
building again after a change, is not a repeat.

### A password printed by a command does not leave your machine

The program already refused to open password files. It did nothing about a
command that *prints* one, and plenty do. Whatever a command prints, the AI
reads, so the company whose AI you rent receives it.

Now the secret is replaced before the AI sees it:

```
HOME=/home/you
EXAMPLE_API_KEY=[REDACTED: value of $EXAMPLE_API_KEY]
SHELL=/bin/sh

note: 1 credential in this output was replaced with [REDACTED: ...] by this
program before you saw it. The real value is on the machine and was not sent.
Do not try to recover it; you do not need it to do the work.
```

Your own stored keys are masked in the output of every tool. Text that only
*looks* like a key is masked in command output and nowhere else, on purpose: a
file the AI reads it may write back, and a label written into a file would
corrupt it.

### A long printout is kept, not thrown away

When a command printed more than the program allows, the old version kept the
beginning and the end and discarded the middle. For a long build that fails, the
error is in the middle. The only way to see it was to build again.

Now the complete printout is saved in a folder on your computer, and the AI is
told where. It reads the part it needs. The folder keeps the 40 newest printouts.

### Commands that cannot be taken back are named

The question on screen used to look the same whether the AI wanted to compile
your program or wipe your home folder. With automatic approval on, it was not
asked at all.

For 18 kinds of command, the question now starts like this:

```
DANGEROUS — it downloads a script from the internet and runs it without anyone
reading it first.
Execute command: curl -fsSL https://example.com/install.sh | sh
```

```
DANGEROUS — it throws away changes that were never committed, and git cannot
bring them back.
Execute command: git reset --hard HEAD~3
```

The program asks about these **even when automatic approval is on**, and if
nobody is at the keyboard it refuses. It never blocks you. It makes sure a person
decides.

### Smaller things

- **Typing mistakes by small AI models are repaired** when there is only one
  thing the AI could have meant: a real line break inside a piece of text, or a
  list sent wrapped up as one string. A repair that could change the meaning is
  refused.
- **Five more kinds of password file** outside your project are refused:
  `_netrc`, `.npmrc`, `.pypirc`, `.dockercfg` and `.env`. Templates such as
  `.env.example` stay readable.
- **54 review checklists instead of 34**, chosen by the file's name. A project
  can keep its own house rules in a folder named `.code-review-rules`.

---

## Deliberately not done

- **Letting the AI run a tool by writing the request as plain text.** Two of the
  four projects allow it, and it helps weak AI models. This project decided
  against it earlier and the reason still holds: a web page with hidden
  instructions could then choose which tool runs on your computer.
- **Testing with a real AI model.** The automatic tests pass, and they test the
  program's own logic. Nothing here was run against an AI. The counts at which
  the AI is warned and stopped come from the other projects, not from
  measurement.
- **Installing the Linux packages before publishing.** Said at the top, and said
  again here because it matters.
- **A ready-made Arch Linux package.** Not built this time. Arch users can build
  from source with `packaging/PKGBUILD`.
- **macOS.** Never built, never run.

---

## Install

### Windows

1. Close Gorilla OpenCode if it is running.
2. Download `gorilla-opencode.exe` below.
3. Press the Windows key, type `PowerShell`, press Enter. In the window that
   opens, go to where you saved the file and run:

```
.\gorilla-opencode.exe install
```

### Debian, Ubuntu, Mint

Download `gorilla-opencode_0.1.135_amd64.deb`, then in a terminal in that folder:

```
sudo apt install ./gorilla-opencode_0.1.135_amd64.deb
```

### Fedora, openSUSE, Rocky

Download `gorilla-opencode-0.1.135-1.x86_64.rpm`, then:

```
sudo dnf install ./gorilla-opencode-0.1.135-1.x86_64.rpm
```

### Any other 64-bit Linux

Download `gorilla-opencode-v0.1.135-linux-amd64`, then:

```
chmod +x gorilla-opencode-v0.1.135-linux-amd64
./gorilla-opencode-v0.1.135-linux-amd64
```

### Check it

```
gorilla-opencode --version
```

You should see `v0.1.135`.

### Verify the download

A fingerprint is a long code worked out from every byte of a file. If one byte
differs, the code differs. `SHA256SUMS-v0.1.135.txt` on this page lists the
fingerprint of every file.

Windows:

```
certutil -hashfile gorilla-opencode.exe SHA256
```

```
4aa76042220607b41d0e7058c7b31be95d52316b13ddd415258868d16cbe06c1
```

Linux:

```
sha256sum -c SHA256SUMS-v0.1.135.txt --ignore-missing
```

### To go back

Windows: `gorilla-opencode uninstall`, then install the 0.1.134 file the same
way. Debian: `sudo apt remove gorilla-opencode`. Fedora: `sudo dnf remove
gorilla-opencode`. Your conversations, settings and keys are stored separately
and none of these touches them.

---

## Connecting Fieldkit

[Fieldkit](https://github.com/gorillanobakaa-dot/Gorilla.Fieldkit) is a separate,
free set of tools from the same maintainer that an AI can call. Each one can
preview what it will do, back up what it changes, check its own result and undo
it. Nothing in it needs an account.

Gorilla OpenCode connects to it with one entry in its settings file
(`config.json` in the `.config/gorilla-opencode` folder in your home folder):

```json
"mcpServers": {
  "fieldkit": { "type": "stdio", "command": "fieldkit", "args": ["mcp"] }
}
```

The AI then has ten more tools: find a tool for a job, read what it does, run it,
undo it, and six more for step-by-step build work and for checking which tools
can be trusted. The `fieldkit` command must be installed for this to work. This
connection worked before this release. It is
written down here because it was not written down anywhere you would find it.

---

## No new pictures in this release, and why

Everything new in this release is text the program prints, and it is quoted above
exactly as printed. No new screenshot was taken. The two below are from earlier
releases and show screens this release did not change, pinned to this version so
you can see what you are downloading.

**The permission question**, which is where the DANGEROUS line now appears for
the 18 kinds of command above.

[![Gorilla OpenCode showing a Permission Required dialog for the patch port tool, naming the folder it will modify and the patch series it will apply, with the three choices Allow, Allow for session and Deny, proving the program asks before a tool changes files](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.135/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.135/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)

**The command list**, all 31 commands on one screen.

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does and showing 37 of 37 lines, 31 commands, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.135/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.135/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

---

## Known issues

| What you see | Why | What to do |
|---|---|---|
| The turn stops with *going round in circles* | The AI repeated one request five times with the same answer | Type `continue` if that was intended, or tell it what to try instead |
| `[REDACTED: ...]` where you expected a value | The program masked a password or key | Nothing is broken. To see the value yourself, run the command in your own terminal |
| A DANGEROUS question although automatic approval is on | The command is one of 18 kinds that cannot be undone | Read the sentence, then decide |
| `/review` says NOTHING WAS REVIEWED | The inspectors are separate programs and none is installed | The report names each one. Install the ones for your language |
| A Linux package will not install | It was not installed on Linux before publishing | Use the plain `linux-amd64` file and report what the package printed |
| The AI asks permission for things it used to do | The three unmeasured restraint rules from 0.1.133 | `/context`, find *Scope restraint*, press space |

---

## Privacy

No telemetry was added and none exists. Nothing is sent anywhere except to the AI
provider you configured, and this release sends that provider less than the last
one did.

One thing to know: a saved printout holds whatever the command printed,
unmasked, in a folder only your user account can read. Masking happens when the
AI reads it.

## Full notes

- [`v0.1.135-release-notes.layman.md`](Changelogs/v0.1.135-release-notes.layman.md) — plain English, every change with before and after
- [`v0.1.135-release-notes.developer.md`](Changelogs/v0.1.135-release-notes.developer.md) — files, functions, thresholds, tests, and what was rejected
