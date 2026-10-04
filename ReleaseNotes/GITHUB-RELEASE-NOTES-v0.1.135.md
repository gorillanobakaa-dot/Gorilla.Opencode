# Gorilla OpenCode 0.1.135 — Windows and Linux

**Five protections for a program that lets an AI use your computer: it stops an AI that repeats itself, keeps your passwords from leaving your machine, saves long printouts instead of throwing half away, names the commands that cannot be undone, and admits when a code check checked nothing.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

Picture an assistant sitting at your keyboard. It can open your files, run your
tools and change things. That is what makes it useful. It is also why it needs
watching, because an assistant with your keyboard can make a mess, and can tell
you a job is done when it is not.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one you have a key for. It is built to
work on an old laptop with slow, expensive internet.

## Should you download this version?

**No. Download 0.1.137 instead.** It was published the same day. It contains
everything on this page, and it fixes four faults in this version that were
found when it was finally tried with a real AI.

This version said on its own page that **nothing in it had been tested with a
real AI**. That was true, and the testing that followed found problems. This
page stays up so that anyone can read what was added and why.

**Wait, if you are on Linux and a failed download would cost you data you cannot
spare.** The Linux files were built on a Windows computer and checked file by
file. They were not installed on a Linux machine before being published.

## Why this matters to you

Think about the fuse box in a house.

A fuse does not know what went wrong. It does not know whether a kettle is
faulty or a wire is damaged. It knows one thing: too much current is flowing, and
if that continues the house may burn. So it cuts the power, every time, without
being asked and without needing to understand.

Nobody is grateful for a fuse on an ordinary day. You only find out whether you
have one on the day something goes wrong.

An AI working on your computer needs fuses for the same reason. It is fast and
usually right. When it is wrong, it is wrong quickly, and it does not notice.
This version adds five:

- **A fuse for going round in circles.** A small AI sometimes asks for the same
  thing again and again. Each repeat costs you data, and money if you pay for
  the AI. The program now tells the AI it is repeating itself, and if it carries
  on, stops it.
- **A fuse for your passwords.** If a command prints a password or key, the AI
  reads it, and so the company providing the AI receives it. The program now
  replaces it with a label before the AI sees anything.
- **A fuse for long printouts.** When a command prints too much, the program used
  to keep the start and the end and throw the middle away. It now keeps the whole
  thing in a file on your computer.
- **A fuse for commands that cannot be undone.** Wiping a folder, running a
  script straight from the internet, throwing away unsaved work: the program now
  says in plain words what will happen, and asks a person, even if you had told
  it to stop asking.
- **A fuse for false reassurance.** The built-in code checker used to say that 18
  inspectors had run when none were installed, and that it had found no
  problems. It now says, in capitals, when it reviewed nothing.

All five run on your own computer. None needs an account or sends anything to
anyone.

**What it means for you, in one line:** fewer ways for an AI to waste your data,
leak your passwords, or destroy your work without anyone deciding to let it.

> **If you are on Linux, read this before downloading.** The Linux files on this
> page were built on Windows and inspected. They were **not installed or started
> on a Linux machine**. The `.rpm` file is the first this project has published.
---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why This Release Exists

Think of the AI as a new assistant working at your desk. It is clever and quick, and it has three bad habits. It sometimes repeats the same action again and again without noticing, like someone pressing a lift button that is already lit. It reads anything you put in front of it aloud to the company whose AI you rent, including a password a program happened to print. And when a job produces a long printout, the old version kept the first and last pages and threw the middle in the bin.

This release deals with all three. The maintainer read four other AI helper programs to see what they do that this one did not. The rule for copying an idea was strict: it must work on your own computer, with no account, no extra key, and nothing sent to anybody's server. Eight ideas passed that test.

One more fault turned up along the way, and it is the most embarrassing one. The built-in code checker has a list of inspectors, small separate programs that each look for one kind of mistake. On a computer where those inspectors are not installed, the checker reported that 18 of them had done their work and, two lines later, that 17 of them were not installed. Both lists held the same names. Underneath it said: findings, 0. A report like that looks like a clean bill of health. It was an empty room.

### What You Will Notice

**The AI repeating itself**
- Before: The AI asks for the same thing again and again. Each repeat costs you data and, on a paid plan, money. Nothing stops it until you notice and press Esc.
- After:  After the third identical request the program tells the AI, in the answer it reads, that it is repeating itself and what to do instead. If it carries on, the program stops the turn at the fifth repeat and tells you why in one sentence. Type the word continue if the repeats were what you wanted.
- Affects: Everyone, and most of all people using small AI models that run on their own computer

**Passwords and keys shown on screen by a command**
- Before: If a command prints a password or an access key, the AI reads it, and so the company providing the AI receives it. The program also saves it in your conversation history as ordinary text.
- After:  The program replaces the secret with a label such as [REDACTED: value of $EXAMPLE_API_KEY] before the AI sees anything. A short note tells the AI that the real value stayed on your machine and that it does not need it.
- Affects: Everyone who keeps a key or password on the computer they work on

**Long printouts**
- Before: When a command prints more than the program allows, the program keeps the beginning and the end and discards the middle. For a long build that fails, the error is usually in the middle, so the only way to see it is to run the whole build again.
- After:  The program saves the complete printout in a file on your computer and tells the AI where it is. The AI reads the part it needs. Nothing runs twice.
- Affects: Anyone who builds large programs or runs commands that print a lot

**Commands that cannot be taken back**
- Before: The question on screen looks the same whether the AI wants to compile your program or wipe your home folder. If you switched on automatic approval, the program does not ask at all.
- After:  For 18 kinds of command that cannot be undone, the question begins with the word DANGEROUS and one plain sentence saying what will happen. The program asks even when automatic approval is on. If nobody is at the keyboard, it refuses.
- Affects: Everyone, and most of all people who use automatic approval

**The code checker's report**
- Before: The report lists an inspector as having run when it was only scheduled to run. On a computer with no inspectors installed, it says 18 ran, 17 were not installed, and 0 problems were found.
- After:  The report opens with a verdict in capitals: NOTHING WAS REVIEWED, or PARTIAL REVIEW with the programming languages nobody looked at. An inspector counts as having run only if it finished at least one job.
- Affects: Anyone who types /review

**Small AI models making typing mistakes**
- Before: A small AI model sometimes writes a request with a real line break in the wrong place, or sends a list wrapped up as one piece of text. The program rejects the request, the AI sends the same request again, and the two go back and forth.
- After:  The program repairs those two mistakes when there is only one thing the AI could have meant. If a repair could change the meaning, the program refuses it and reports the original mistake.
- Affects: People using small or local AI models

**Files that hold secrets**
- Before: The program refuses to read some password files outside your project folder, but not the ones named _netrc, .npmrc, .pypirc, .dockercfg or .env.
- After:  It refuses those too. Template files meant for copying, such as .env.example, stay readable. Inside your own project nothing changes.
- Affects: Everyone

**Checklists for reviewing code**
- Before: The checker carries 34 checklists, one per programming language, and has no way to hand over a checklist for a language its inspectors do not cover.
- After:  It carries 54, and it picks the right one from the file's name. You can also keep your own house rules in a folder named .code-review-rules inside your project.
- Affects: Anyone who types /review on a project

### Deliberately Not Done

- **Letting the AI run a tool by writing the request as ordinary text** — Two of the other programs allow this, and it helps weak AI models. This project decided against it earlier and the reason still holds: a web page with hidden instructions could then choose which tool runs on your computer.
- **Testing this release with a real AI model** — No part of this release was run against an AI model. The automatic tests pass, and they test the program's own logic. How a real model reacts to being told it is repeating itself is not measured.
- **Installing the Linux packages before publishing them** — The maintainer built the Debian and RPM packages on a Windows computer. They were inspected file by file. They were not installed or started on a Linux computer. The RPM package is the first this project has published.
- **A ready-made package for Arch Linux** — Not built for this release. Arch users can still build from source with the PKGBUILD file.
- **macOS** — Never built, never run.

### Privacy & Security

No telemetry was added and none exists. This program sends nothing anywhere except to the AI provider you configured. Three changes reduce what that provider receives: passwords and keys printed by commands are replaced before the AI sees them; five more kinds of credential file outside your project are refused; and the complete copy of a long printout is kept in a folder on your own computer, readable only by your user account, instead of being sent whole. That folder keeps the 40 newest printouts and deletes older ones. One thing to know: a saved printout holds whatever the command printed, unmasked, because masking happens when the AI reads it, not when the file is written.

### How to Install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora or similar Linux computer.
- About 60 MB of free disk space and enough mobile data to download a 54 MB file on Windows or a file of about 20 MB on Linux.
- Gorilla OpenCode closed, if you already have it installed.

**Step 1:** Close Gorilla OpenCode if it is open. On Windows, look at the taskbar at the bottom of the screen and close any window titled Gorilla OpenCode.
✓ No window titled Gorilla OpenCode is open.

**Step 2:** Open the release page in your web browser and download the one file for your computer. Windows: gorilla-opencode.exe. Debian, Ubuntu or Mint: gorilla-opencode_0.1.135_amd64.deb. Fedora, openSUSE or Rocky: gorilla-opencode-0.1.135-1.x86_64.rpm. Also download SHA256SUMS-v0.1.135.txt, a short list of fingerprints.
✓ Two files are in your Downloads folder.

**Step 3:** Check that the file you received is the file that was published. A fingerprint is a long code worked out from every byte of a file; if one byte differs, the code differs. On Windows, press the Windows key, type PowerShell, press Enter, and a window with a blinking cursor opens. Type cd Downloads and press Enter. Then type the command below and press Enter.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ It prints 4aa76042220607b41d0e7058c7b31be95d52316b13ddd415258868d16cbe06c1. On Linux, open a terminal in your Downloads folder and run: sha256sum -c SHA256SUMS-v0.1.135.txt --ignore-missing. It prints the file name followed by OK.

**Step 4:** Windows: install it by typing the command below in the same PowerShell window and pressing Enter.
```
.\gorilla-opencode.exe install
```
✓ It reports where it copied itself and the shortcuts it made.

**Step 5:** Debian, Ubuntu or Mint instead: in the terminal, type the command below and press Enter. It asks for your password because installing a program changes the system.
```
sudo apt install ./gorilla-opencode_0.1.135_amd64.deb
```
✓ The last lines say the package gorilla-opencode was set up, with no line beginning with E:.

**Step 6:** Fedora, openSUSE or Rocky instead: in the terminal, type the command below and press Enter.
```
sudo dnf install ./gorilla-opencode-0.1.135-1.x86_64.rpm
```
✓ The last line says Complete!

**Step 7:** Confirm which version you now have. Type the command below and press Enter.
```
gorilla-opencode --version
```
✓ It prints v0.1.135.

**To go back:** Windows: type gorilla-opencode uninstall and press Enter, then install the gorilla-opencode.exe from version 0.1.134 the same way as in step 4. Debian: sudo apt remove gorilla-opencode. Fedora: sudo dnf remove gorilla-opencode. Your conversations, settings and keys are stored in a separate folder and none of these steps touches them.

### If Something Goes Wrong

**The AI stops and the screen says: Stopped because the model was going round in circles.**
The AI made the same request five times in a row and got the same answer every time, or two requests alternated ten times. The program ended the turn so the repeats do not continue.
What to do: Read the sentence: it names the request that was repeated. If the repeats were what you wanted, type continue and press Enter. Otherwise tell the AI what to try instead. Nothing is lost; the conversation is saved up to that point.
Status: expected behaviour, new in 0.1.135

**A command's output shows [REDACTED: ...] where you expected a value.**
The program recognised a password or access key and replaced it before the AI saw it. The real value is unchanged on your computer.
What to do: Nothing is broken. If you need to see the real value yourself, run the command in your own terminal window, outside Gorilla OpenCode.
Status: expected behaviour, new in 0.1.135

**The program asks permission for a command even though automatic approval is on, and the question begins with DANGEROUS.**
The command matches one of 18 kinds that cannot be undone, such as deleting your home folder or discarding work that was never saved to git.
What to do: Read the sentence after the word DANGEROUS. Answer yes only if that is what you want to happen. The program never blocks you; it makes sure a person decides.
Status: expected behaviour, new in 0.1.135

**After typing /review the report says NOTHING WAS REVIEWED.**
The inspectors are separate programs and none of them is installed on this computer, so nothing looked at your code.
What to do: The report lists each missing inspector by name. Install the ones for the programming language you use, then type /review again. Until then, treat the report as no review at all.
Status: the old report hid this; fixed in 0.1.135

**On Linux the package will not install, or the program will not start after installing.**
The Linux packages for this release were built on Windows and were not installed on a Linux computer before publishing.
What to do: Download the plain file gorilla-opencode-v0.1.135-linux-amd64 from the same page, make it runnable with chmod +x, and start it directly. Then report what the package printed in the project's issue tracker.
Status: investigating: untested before release

### Common Questions

**Q: Does any of this need an account, a key or the internet?**
A: No. Every change in this release runs on your own computer. That was the condition for including it.

**Q: Will the AI stop in the middle of real work?**
A: It should not. Only exact repeats count: the same request with the same details, returning the same answer. Reading a long file page by page, or building again after a change, produces a different request or a different answer each time. This was checked by automatic tests, not with a real AI model.

**Q: Can the masking damage my files?**
A: It is built not to. Key-shaped text is masked only in the output of commands. When the AI opens a file, the file's text is shown as it is, because the AI may write the file back and a label written into a file would corrupt it. Your own stored keys are masked everywhere, since nothing needs them.

**Q: Where are the saved printouts?**
A: In a folder named tool-output inside the program's state folder, which is .local/state/gorilla-opencode in your home folder. You can delete its contents at any time.

**Q: Did you copy another project's code?**
A: The ideas, not the code. The exception is 54 plain-text checklists for reviewing code, which come unchanged from the Alibaba project under its Apache 2.0 licence, with the notice included. Each file in this program that uses a borrowed idea names where it came from.

**Q: What is Fieldkit, and do I need it?**
A: Fieldkit is a separate, free set of tools from the same maintainer that an AI can call, each with a preview, a backup and an undo. Gorilla OpenCode can connect to it. You do not need it; this release works without it.

### Bottom Line

This release makes the program more careful, not more capable. It stops the AI repeating itself at your expense, keeps passwords from leaving your machine, saves long printouts instead of discarding them, and says in capitals when a code review reviewed nothing. Each change runs on your own computer with no account. The honest limits are these: nobody tested the release with a real AI model, so the point at which the AI is told it is repeating itself comes from other projects and not from measurement here. The Linux packages were built on Windows and not installed on Linux before publishing. If you use Windows, this replaces 0.1.134 and is worth the 54 MB. If you use Linux and cannot afford a failed download, wait for a report that the package installs.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Eight ideas were taken, all of which run locally | 📄 stated in input | it runs on the user's own machine with no API key and no remote service |
| The old review report was contradictory | 📄 stated in input | with the same names in both lists |
| No AI model was used to test the release | 📄 stated in input | No part of this release was run against an AI model |
| Linux packages were not installed before publishing | 📄 stated in input | were NOT installed or run on a |
| The turn stops at the fifth repeat | 📄 stated in input | same call, same result \| 3 \| 5 |
| A saved printout is unmasked on disk | 🤖 model inference | *(none — model judgment)* |
| Ordinary work such as paging through a file does not trigger a stop | 🤖 model inference | *(none — model judgment)* |
| Small or local AI models benefit most from the repair of mangled requests | 🤖 model inference | *(none — model judgment)* |
| A Linux user short of data should wait for an install report | 🤖 model inference | *(none — model judgment)* |
| Download sizes of about 54 MB on Windows and about 20 MB on Linux | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated plain-language release notes.*

---

# What the program actually printed

Every block below is real output, copied from a run on the maintainer's computer. Nothing here is an illustration.

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

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Two commits (04d3182, a6256d1) add eight behaviours taken in idea from four other agent projects, selected on one criterion: the behaviour runs on the user's machine with no API key and no remote service. Root cause of the one outright defect: `emit_agent_json` built `ran_ids` from every scheduled job, so a tool whose job ended 127 (not installed) appeared in `tools_ran` and in `tools_missing` at once. The fix gives every job exactly one terminal state and derives `tools_ran` from completed jobs. The other seven changes add a single post-tool step in the agent loop (`finishToolResults`) that masks credentials and records the call for loop detection, a spill file for output a size cap cuts, a pattern list of irreversible shell commands wired into the permission service as a carve-out from auto-approve, two lossless repairs to malformed tool arguments, five more credential file names, and 20 more vendored review rule documents with lookup by file name. Scope: 13 files modified, 13 added, plus 21 rule documents. No schema migration, no configuration key, no new dependency.

### Known Alternatives Considered

Executing tool calls parsed from model text was considered and rejected. OpenHands (`llm/mixins/fn_call_converter.py`) and goose (`providers/toolshim.rs`) both do it. `internal/llm/agent/leakedtoolcall.go` records the project's reason: "Synthesising a tool call from model TEXT and executing it is exactly the fuzzy-output-to-dispatch path internal/llm/agent/toolname.go exists to forbid". For secret masking, shape-based masking of file reads was rejected: `secretmask.go` states "Masking a shape in a source file trades a leak nobody has for a corruption somebody will." For the stuck detector, DeepSeek Harness's advisory-only reminder (remind at 3, 5 and 8, never stop) was rejected: `stuck.go` states "a reminder that can be ignored forever is paid for forever." For spill, replacing the result with only a file path (what goose and DeepSeek Harness do) was rejected: `spill.go` states "a small model that is handed only a path often stops there." Alibaba's LLM grouping, LLM filter pass, web viewer and OpenTelemetry were not taken. Recipes were not taken because custom commands with named arguments already exist.

### Architecture Impact

One new invariant: every tool result passes through `finishToolResults` (internal/llm/agent/aftertool.go) before it is stored or sent. It is called once, at the `out:` label of `streamAndHandleEvents`, so it covers the normal path and the cancellation path. A new `ran []bool` slice marks calls that were attempted; cancelled and permission-denied calls are excluded from masking and from loop counting. A turn can now end with `*StuckError`, returned after the tool message is stored so the history stays whole. The stuck tracker is a package-level value keyed by session ID and reset in `processGeneration` beside `permission.ClearTaint`. `permission.CreatePermissionRequest` gains one field, `Irreversible string`; `mustAskAnyway` checks it first, and in unattended mode `Request` returns false for it where the other carve-outs log and proceed. The review report schema `code-review/agent/1` gains two top-level keys, `coverage` and `review_rule_sources`; `agentReport.Coverage` is a pointer so a report from an older toolkit still renders.

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.135" -o gorilla-opencode.exe .` (CGO_ENABLED=1, gcc 15.2.0, same as 0.1.134). Linux: the same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Both report vcs.revision a6256d1767f99cb70e2209a29947f5425cff9ed6 and vcs.modified=true, because Changelogs/release.meta.json was being rewritten for this release when they were built; 0.1.134 was built the same way. The .deb and .rpm are produced by nfpm from one configuration, reproducing the file layout of the published 0.1.132 .deb.
```

### Resource Deltas

gorilla-opencode.exe: 54,121,984 bytes (0.1.134) -> 54,353,920 bytes (+231,936). Linux binary: 52,519,204 bytes (0.1.132) -> 53,039,264 bytes (+520,060, across three releases). RSS and cold start: not measured. Per-turn token cost: the stuck warning and the mask notice add text to a tool result only when they fire; not measured.

### Code Changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `internal/llm/tools/codereview/toolkit/code_review.py` | modified | `ran_ids = {r.job.tool_id for r in all_results}`: every tool with a job counted as having run, including jobs ending 127, 124, or erroring. | `job_state()` gives each job one of completed, missing, errored, timed_out. `coverage_of()` returns terminal_state (complete, partial, nothing-ran), job counts, tools_completed, tools_never_completed, and each in-scope language as reviewed, unreviewed or no-analyser. `tools_ran` is `coverage["tools_completed"]`. `review_rules` now merges `rules.rules_for_files(files)` with the per-language rules. |
| `internal/llm/tools/review.go` | modified | The trust block began with "Analysers that ran: N". | When the report carries `coverage`, the trust block begins with a verdict line: NOTHING WAS REVIEWED, PARTIAL REVIEW with completed/planned counts and the unreviewed languages, or a coverage line for a complete run. |
| `internal/llm/tools/codereview/toolkit/rules.py, rule_docs/` | modified | 34 documents, looked up by registry language only; read with the platform default encoding (cp1252 on Windows). | 54 documents (upstream a758d9c), read as UTF-8. `doc_for_file()` resolves by file name, by `EXTENSION_TO_DOC`, by path pattern for .github YAML and mapper/DAO XML, and by first-line sniff for `.m`. `set_project()` layers `<project>/.code-review-rules/<doc>.md` over the vendored text; `<!-- replace -->` on the first line replaces it. `sources()` reports system, project or system+project per document. |
| `internal/llm/tools/toolinput.go` | modified | After a failed strict decode: lone-backslash repair, then quoted-scalar coercion. | Adds `escapeBareControls` (bytes below 0x20 inside string literals), tried alone and combined with the backslash repair. `coerceFields` replaces `coerceScalarStrings` and adds `containerLiteral`: a slice, map or struct field that arrives as a JSON string is unwrapped; an unparseable one is repaired by `repairSerializedContainer` and accepted only if `repairedContainerAcceptable` finds no unknown key and no string with an odd quote count; one bare string for a `[]string` field becomes a one-element list. On refusal the strict error is returned. |
| `internal/llm/tools/sensitive.go` | modified | Basenames refused outside the workspace did not include _netrc, .npmrc, .pypirc, .dockercfg or .env files. | Those four basenames are refused, and `isDotEnv` refuses `.env` and `.env.*` except .env.example, .env.sample and .env.template. The workspace exemption is unchanged. |
| `internal/llm/agent/stuck.go, aftertool.go, agent.go` | added | No loop detection. A model repeating a call continued until the user cancelled. | `stuckTracker.Record` keeps the last 16 calls per session as (tool + normalised arguments, sha256 prefix of result, isError). Thresholds warn/stop: repeat 3/5, error 3/5, ping-pong 6/10. The warning is appended to the tool result once, at the warn count exactly. At the stop count `streamAndHandleEvents` returns `*StuckError`. Arguments are normalised by a JSON round trip (sorted keys, no insignificant whitespace). The result is recorded before any note is appended. |
| `internal/llm/tools/secretmask.go` | added | Tool output reached the provider and the session database unaltered. | `MaskSecrets(toolName, content)` replaces known values (configured provider API keys; values of environment variables whose name matches `(?i)(?:^|_)(?:KEY|APIKEY|API_KEY|TOKEN|SECRET|PASSWORD|PASSWD|PWD|CREDENTIAL|CREDENTIALS|AUTH)(?:_|$)`, minimum 12 characters, not a path, no spaces) in every tool, and 11 issuer-prefixed token shapes plus PEM private-key blocks in `bash` output only. Known values are gathered once per process. |
| `internal/llm/tools/spill.go, bash.go, tools.go` | added | `truncateOutput` (30,000 bytes) and `clampToolContent` (409,600 bytes) discarded what they cut. | Before cutting, the whole text is written to `StateBase()/tool-output/<timestamp>-<label>-<8 hex>.txt`, mode 0600, at most 64 MB, newest 40 files kept. The notice names the path. A failed write falls back to the old notice. |
| `internal/llm/tools/dangerous.go, internal/permission/permission.go, bash.go` | added | One permission prompt for every non-read-only command; auto-approve covered all of them inside the workspace. | `DangerousPatternIn` matches 18 regular expressions (15 names) against the whole command line. A match sets `Irreversible` on the permission request, prefixes the description with DANGEROUS and the reason, and forces `isSafeReadOnly` false. `mustAskAnyway` returns a reason for it, so auto-approve asks; unattended mode refuses. |

### Subsystem Changes

**OTHER:** Agent loop: one post-tool step for masking and loop detection; a new turn-ending error type.

**AUTH:** Permission service: `Irreversible` carve-out from auto-approve; refusal in unattended mode.

**STORAGE:** New directory `tool-output/` under the state directory, mode 0700, files 0600, pruned to 40. Nothing is written unless a cap truncates.

**NETWORK:** No new network access. Tool results sent to the provider may now contain [REDACTED: ...] placeholders in place of credentials.

**TUI:** No change.

### Test Coverage

- **Added:** internal/llm/agent/stuck_test.go (8 tests); internal/llm/tools: toolinput_repair_test.go (11), dangerous_test.go (2 tests over 32 dangerous and 35 ordinary commands), secretmask_test.go (5), spill_test.go (5), 3 added to sensitive_test.go, 4 added to review_test.go; internal/permission/irreversible_test.go (3); toolkit tests/test_coverage.py (19 checks) and tests/test_rules.py (16 checks), run from Go by TestToolkitCoverageAccountingHolds.
- **Removed:** None.
- **Notes:** Mutation check: eight guarded lines were broken one at a time and each break failed a named test. One (the acceptance check in `containerLiteral`) survived at first; TestAValidLookingButWrongRepairIsRefused was added and it is now killed. `go test ./internal/...` passes. `go test ./cmd` has one failure, TestChatGPTModelsAreRegisteredAndRoutable, which fails when the whole package runs and passes alone, identically on v0.1.134. No test in this release runs against an AI model. internal/llm/tools/main_test.go now redirects the spill directory: the first run of the response-cap tests wrote two 3 MB files into the developer's real state directory.

### Security Posture

No CVE. Attack surface: reduced in three places and widened in one. Reduced: credentials in tool output are masked before egress; five more credential file names are refused outside the workspace; irreversible commands are excluded from auto-approve and refused when unattended. Widened: full copies of truncated output now persist on disk under the state directory (0600), unmasked, because masking is applied when a result is delivered, not when the spill is written. The dangerous-command list is a recognition aid, not a boundary: it matches text, so a command built at run time or hidden in a script file is not recognised. The argument repairs run only after a strict decode fails and never change a string headed for a string field. Telemetry: none added, none present.

### Deployment

**Prerequisites:**
- Windows 10 or 11 amd64, or Linux amd64 with python3, lynx, ripgrep and one of xclip, xsel or wl-clipboard (declared as package dependencies).
- The program is not running.

```bash
# fetch
gh release download v0.1.135 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: gorilla-opencode.exe, gorilla-opencode-v0.1.135-linux-amd64, gorilla-opencode_0.1.135_amd64.deb, gorilla-opencode-0.1.135-1.x86_64.rpm, SHA256SUMS-v0.1.135.txt and the two release-notes files.
# verify
sha256sum -c SHA256SUMS-v0.1.135.txt
# Expected: Each file name followed by OK. gorilla-opencode.exe is 4aa76042220607b41d0e7058c7b31be95d52316b13ddd415258868d16cbe06c1; the Linux binary is 9fd2b1cbc395587b4aa2f92fa7f7dadb60ec066cfe88fcebd45bbe714863fe09.
# install
.\gorilla-opencode.exe install   |   sudo apt install ./gorilla-opencode_0.1.135_amd64.deb   |   sudo dnf install ./gorilla-opencode-0.1.135-1.x86_64.rpm
# Expected: Windows: the install path and shortcuts. apt: the package is set up. dnf: Complete!
# verify_active
gorilla-opencode --version
# Expected: v0.1.135
# rollback
gorilla-opencode uninstall   |   sudo apt remove gorilla-opencode   |   sudo dnf remove gorilla-opencode
# Expected: The program is removed. Configuration, sessions and keys under the config and data directories are left in place.
```

**Rollback:**
  1. Remove 0.1.135 with the rollback command for your platform.
     `gorilla-opencode uninstall`
  2. Install the previous build. Windows: 0.1.134. Linux: 0.1.132, the last Linux build before this one.
     `.\gorilla-opencode.exe install`
  3. Optionally delete the spilled output, which older versions never read.
     `rm -r ~/.local/state/gorilla-opencode/tool-output`

### Known Issues

**[low]** A turn ends with "Stopped because the model was going round in circles".
- Cause: Five identical calls with identical results, five identical failing calls, or ten alternating calls since the last user message.
- Remedy: Send "continue" to reset the count, or redirect the model. The thresholds are constants in internal/llm/agent/stuck.go; there is no configuration key.

**[medium]** An edit fails to match text the model copied from command output containing [REDACTED: ...].
- Cause: The model took a masked value from `bash` output (for example `cat file`) instead of reading the file with `view`, which is not shape-masked.
- Remedy: Have the model read the file with `view`. Known live values are masked in every tool by design.

**[low]** A legitimate command prompts with DANGEROUS under auto-approve.
- Cause: It matches one of the 18 expressions in dangerous.go, for example `git clean -f` or `git push --force`.
- Remedy: Approve it. If the match is wrong, add the command to the ordinary-work list in dangerous_test.go and narrow the expression.

**[high]** The Linux package fails to install or the binary fails to start. (deferred to next release)
- Cause: The Linux artefacts were cross-built on Windows and not installed or executed on Linux before publishing. The .rpm is the first this project has published.
- Remedy: Run the bare binary gorilla-opencode-v0.1.135-linux-amd64 and report the package manager's output.

**[low]** go test ./cmd reports TestChatGPTModelsAreRegisteredAndRoutable failing. (deferred to unscheduled)
- Cause: Test-order dependence on models.SupportedModels; it passes alone. Present on v0.1.134.
- Remedy: Not fixed in this release.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| tools_ran counted scheduled jobs | 📄 stated in input | listed every tool that had |
| Selection criterion was local execution | 📄 stated in input | it runs on the user's own machine with no API key and no remote service |
| Thresholds are borrowed, not measured | 📄 stated in input | not measured here |
| Eight mutations, all killed after one added test | 📄 stated in input | eight guarded lines were broken one at a time |
| Spilled output is unmasked on disk and so widens the surface | 🤖 model inference | *(none — model judgment)* |
| The dangerous-command list is not a security boundary | 🤖 model inference | *(none — model judgment)* |
| The cmd test failure is caused by test-order dependence | 🤖 model inference | *(none — model judgment)* |
| A masked value copied from bash output can break a later edit | 🤖 model inference | *(none — model judgment)* |
| Binary size deltas | 🤖 model inference | *(none — model judgment)* |
| Linux package failure is rated high severity because it is untested | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*
