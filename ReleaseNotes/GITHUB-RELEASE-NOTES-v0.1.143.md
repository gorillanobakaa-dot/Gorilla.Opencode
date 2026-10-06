# Gorilla OpenCode 0.1.143 — Windows and Linux

**The previous version repaired three commands and admitted nobody had run them for real. This version runs them for real, on a small Gemini model, and fixes what the runs showed. Every item the previous page listed as still wrong is dealt with.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

Picture an assistant sitting at your keyboard. It can open your files, run your
tools and change things. That is what makes it useful. It is also why it needs
watching, because an assistant with your keyboard can make a mess.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if you use `/review`, `/research` or `/osint`.** Those three are the
subject of this version. They have now been driven from start to finish with a
real AI, and the faults that showed up are fixed.

**Yes, if you are on 0.1.141 or older.** You get this and the previous version,
which repaired the search tool and the list of AIs.

**It changes little for you** if you only chat and edit files. The search tool
and the list of AIs were already fixed in 0.1.142.

## Why this matters to you

A garage can service a car thoroughly and still not have driven it. The
previous version of this program was in that state: four commands had been
taken apart, about fifty faults repaired, every part tested on the bench, and
the release page said so, in those words. Nobody had typed `/review` into the
real program and watched it work.

The owner's instruction was short: fix everything, and run them for real, on a
cheap AI through his subscription. So the car went round the block.

**`/review`** was pointed at a small test project with three faults planted on
purpose: a fake access key in a settings file, a dangerous shell call in a
Python file, and a result thrown away in a Go file. It found all three and named
the file and line of each. The key's value was kept out of the report.

**`/research`** was asked a precise question about Go's memory settings. Four
helpers went out and came back with the right answer, citing the Go source code
by line number, the release notes and the original design proposal.

**`/osint`** was asked for a dossier on the 2024 CrowdStrike outage. Four of the
research lanes added in the previous version ran, and each came back with graded
sources: the vendor's own reports, Microsoft's statement, a regulatory filing,
two peer-reviewed studies, and the lane whose job is to argue the other side.

The drive still found things. The worst: after that `/review` run, which had
found everything, the program's own record under the answer read

```
1 tool call, 1 did not succeed:
  review ...\sample  -> partial: 6 of 17 jobs completed
```

Eleven of the seventeen were checking tools that are not installed on that
computer. The record was counting them as failures. It now reads

```
1 tool call:
  review ...\sample  -> ok, partial: 6 analysers ran, 11 not installed
```

and a review is called a problem only when a tool broke, or a language was left
with no tool at all.

Also found and fixed: two `/review` settings that were meant to differ sent the
same instructions; the cost shown before a research run was still an estimate
that real runs had proved low; one cost screen was a row too tall for a small
window, which cut off the line that tells you which keys to press.

**What it means for you, in one line:** the three commands work, and the
program now tells the truth about how well each run went.

**What it does not do:** it does not make a name search of a folder holding
millions of files faster. The search engine alone takes that long.
---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why This Release Exists

The previous version repaired four commands and said, in so many words, that three of them had been checked only by the program's own tests. Nobody had sat the real program down with a real AI and typed /review, /research or /osint. The owner's reply was short: fix everything, and run them for real, on a cheap Gemini model through his subscription.

So that is what happened. A test repository was built with three planted faults: a fake key in a settings file, a dangerous shell call in a Python file, and a thrown-away result in a Go file. /review found all three and named the file and line of each. /research was asked a precise question about Go's memory settings and came back with the right answer, citing the Go source code by line. /osint was asked for a dossier on the 2024 CrowdStrike outage and produced one with graded sources: the vendor's own reports, Microsoft's statement, a regulatory filing, two peer-reviewed studies and the arguments against.

It is like a garage that has serviced a car and then, for the first time, drives it round the block. The service was good. The drive still found things.

The program's own record of the /review call said it had not succeeded, when it had found everything: eleven of the seventeen jobs were analysers that are not installed, and the record counted them as failures. Two /review settings that were supposed to differ sent the same instructions. The cost shown before a research run was still an estimate that real runs had proved low. One cost screen was a row too tall for a small window. All of that is fixed in this version, together with the four items the previous page listed as still wrong.

### What You Will Notice

**The record under a /review answer**
- Before: A review that found every planted fault was recorded as "did not succeed", because analysers that are not installed were counted as failures.
- After:  The record says how many analysers ran and how many are not installed. A review is a problem only when an analyser broke or a language had no analyser at all.
- Affects: Anyone using /review

**/review with the security setting**
- Before: Identical to the full setting; only the list shown afterwards differed.
- After:  Runs only the secret scanners, security tools and static analysis, at full depth. The full setting runs everything. The report records which one ran.
- Affects: Anyone using /review

**The cost shown before /research or /osint**
- Before: An estimate from assumptions that real runs had shown to be low by a multiple.
- After:  Once a run has finished on your computer, the estimate is priced from the measured size of a helper session. Before that it says the figure is assumed.
- Affects: Anyone using /research or /osint

**The /research cost screen on a small window**
- Before: With a priced helper AI and ten helpers it was a row too tall, and the line with the keys was cut off.
- After:  It fits. Two rows that repeated what was shown elsewhere are dropped on short screens.
- Affects: Anyone on a 32-row terminal

**The whois and DNS entry in /arsenal on Windows**
- Before: Unavailable, because the two halves were one entry and only one half has a Windows package.
- After:  Two entries. DNS can be installed on Windows; whois says plainly that it cannot.
- Affects: Everyone on Windows

### Deliberately Not Done

- **Running /research and /osint again on the exact published file** — They were run on the previous version plus this version's fixes, before the final build. The part that does the work did not change between that run and this file; only the cost screens and the review tool did. /review was run again on the published file.
- **Making a name search of a huge folder faster** — The search engine on its own takes that long to walk 3.9 million files. On this day it took 12 to 20 seconds with a cold disk, with or without this program.
- **Showing a price for a free AI** — A free quota has no price, so the cost line reads nothing. The token figures beside it are real.

### Privacy & Security

No telemetry was added and none exists. The three real runs sent their questions to Google, the AI provider, and to public web sources, which is what those commands do; they ran on a separate configuration so the owner's own settings and conversations were not touched. The only files they left in the owner's Documents folder are the dossier and the findings file, which is where the program always puts them.

### How to Install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora or similar Linux computer.
- Enough mobile data for a 55 MB file on Windows or a file of about 24 MB on Linux.
- On Windows you do not need to close the program first; an open window stays on the old version until you close it.

**Step 1:** Open the release page in your web browser and download the one file for your computer. Windows: gorilla-opencode.exe. Debian, Ubuntu or Mint: gorilla-opencode_0.1.143_amd64.deb. Fedora, openSUSE or Rocky: gorilla-opencode-0.1.143-1.x86_64.rpm. Also download SHA256SUMS-v0.1.143.txt.
✓ Two files are in your Downloads folder.

**Step 2:** Check the file is the one that was published. On Windows, press the Windows key, type PowerShell, press Enter. A window with a blinking cursor opens. Type cd Downloads and press Enter. Then type the command below and press Enter.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ It prints a25524dd59ff208624afd0774c888379765323e7188230c3b6d4133da74f4fd8. On Linux, run sha256sum -c SHA256SUMS-v0.1.143.txt --ignore-missing in a terminal in your Downloads folder; it prints the file name followed by OK.

**Step 3:** Windows: install it. Type the command below and press Enter.
```
.\gorilla-opencode.exe install
```
✓ It reports where it copied itself and the shortcuts it made.

**Step 4:** Debian, Ubuntu or Mint instead: type the command below and press Enter. It asks for your password because installing a program changes the system.
```
sudo apt install ./gorilla-opencode_0.1.143_amd64.deb
```
✓ A line says Setting up gorilla-opencode (0.1.143), with no line beginning with E:.

**Step 5:** Fedora, openSUSE or Rocky instead: type the command below and press Enter.
```
sudo dnf install ./gorilla-opencode-0.1.143-1.x86_64.rpm
```
✓ The last line says Complete!

**Step 6:** Confirm the version. Type the command below and press Enter.
```
gorilla-opencode --version
```
✓ It prints v0.1.143.

**To go back:** Windows: type gorilla-opencode uninstall and press Enter, then install the gorilla-opencode.exe from version 0.1.142 the same way as in step 3. Debian: sudo apt remove gorilla-opencode. Fedora: sudo dnf remove gorilla-opencode. Your conversations, settings and keys are stored separately and none of these steps touches them.

### If Something Goes Wrong

**The record under a /review answer says "ok, partial" with a number of analysers not installed.**
The review ran every analyser you have. The others are not on your computer.
What to do: Nothing to do, unless you want more coverage: install the named analysers.
Status: expected behaviour, new in 0.1.143

**The cost before a research run changed between yesterday and today.**
A run finished on your computer, and the estimate is now priced from its measured size instead of an assumption.
What to do: Nothing to do. The screen says which basis is in use.
Status: expected behaviour, new in 0.1.143

**The cost line before a research run says nothing, or $0.00, on Google Antigravity or NVIDIA.**
A free quota has no price. The token figures are still real and still count against your quota.
What to do: Nothing to do.
Status: expected behaviour

### Common Questions

**Q: Why run them on a cheap AI?**
A: Because the point was to test the program, not the AI, and these runs cost quota. A small fast model found all three planted faults and answered the research question correctly, which says the tools and the lanes do their job.

**Q: Does the measured cost basis come from my computer or from somebody else's?**
A: Yours. The program keeps a small record of your own finished runs and uses their middle value. Until you have one, it says the figure is assumed.

### Bottom Line

Take this version. The three commands that the previous page admitted were untested have now been run for real and did their job, and the faults those runs exposed are fixed: a review that worked is no longer recorded as a failure, the security setting really is a security setting, and the cost you are shown before a research run comes from your own measured runs. The honest limit: /research and /osint were run on the build before this one plus these fixes, not on the published file itself; /review was.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| All three planted faults were found with file and line | 📄 stated in input | All three faults were found and reported with file and line |
| /research answered correctly citing the Go source by line | 📄 stated in input | They cited the Go runtime source files by line, the Go 1.19 release notes and the original design proposal, and the answer was correct |
| /osint produced a graded dossier with the vendor's reports, Microsoft, the SEC filing and two peer-reviewed studies | 📄 stated in input | the vendor's own post-incident review and root cause analysis, Microsoft's statement, the SEC filing, two peer-reviewed studies with their DOIs |
| The record said did not succeed although the review found everything | 📄 stated in input | the program's own record of the call still said "did not succeed: partial: 6 of 17 jobs completed" |
| security and full sent the same instructions | 📄 stated in input | /review "security" and "full" sent the same instructions to the toolkit |
| The cost estimate was proved low by real runs | 📄 stated in input | an estimate that finished runs had proved low by a multiple |
| The screen was 33 rows on a 32-row terminal | 📄 stated in input | the /research screen was 33 rows on a 32-row terminal and the key line was cut off |
| whois and DNS split into two entries | 📄 stated in input | it is split into a DNS entry (dig and host, which scoop's bind package provides) and a whois entry |
| /research and /osint were run before the final build | 📄 stated in input | the /research and /osint runs above were made on the 0.1.142 program plus the uncommitted fixes of this version, before the final build |
| A name search still takes 12 to 20 seconds on a cold disk | 📄 stated in input | measured 12 to 20 seconds on this day with a cold disk cache |
| Seven real-AI scenarios passed on the published Windows file | 📄 stated in input | All seven gave valid results |
| The runs used a separate configuration | 📄 stated in input | a separate configuration folder was used |


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

Every block below is real output or a real measurement, taken on the maintainer's computer. Nothing here is an illustration. The AI in the three runs was Gemini 3.6 Flash, reached through a Google Antigravity sign-in; the program ran on a separate configuration so the owner's own settings were not touched.

## /review on the test project

Three faults were planted: a fake access key (the secret scanner's own documented example, not a real credential), `shell=True` in a Python file, and an unused result in a Go file. The program's record after the run with the **security** setting, on this version:

```
--- what actually ran (recorded by Gorilla OpenCode, not written by the AI) ---
1 tool call:
  review C:\Users\gorilla1\Documents\Gorilla.Opencode.Builds\e2e\run5\sample  -> ok, partial: 5 analysers ran, 5 not installed
```

With the **full** setting:

```
1 tool call:
  review C:\Users\gorilla1\Documents\Gorilla.Opencode.Builds\e2e\run5\sample  -> ok, partial: 6 analysers ran, 11 not installed
```

What the AI reported from the tool's output on the full run, verbatim, first lines:

```
### 1. Analysers That Ran (6)
* `bandit`
* `bandit-deep`
* `gitleaks-history`
* `gitleaks-worktree`
* `go-vet`
* `staticcheck`
```

And the findings it listed from the first run, before this version's receipt fix, which are the three planted faults:

```
* `app.py:5` `[bandit+bandit-deep]` — subprocess call with `shell=True` identified, security issue.
* `main.go:6` `[go-vet+staticcheck]` — result of `fmt.Sprintf` call not used.
* `settings.ini:1` `[gitleaks-history+gitleaks-worktree]` — Possible secret detected (rule: `aws-access-token`) — value withheld from report on purpose.
```

The same run's record on 0.1.142, which this version corrects:

```
1 tool call, 1 did not succeed:
  review C:\Users\gorilla1\Documents\Gorilla.Opencode.Builds\e2e\run2\sample  -> partial: 6 of 17 jobs completed
```

## /research

The question was what Go's `GOGC` variable controls and what `GOMEMLIMIT` adds. The record:

```
2 tool calls, 1 did not succeed:
  research {"agents":4,"mode":"standard","question":"What does Go's GOGC environment variable control...  -> failed
  research {"agents":4,"doctrine":"standard","mode":"parallel","question":"What does Go's GOGC enviro...  -> ok
```

The first call failed because the test asked for a mode that does not exist; the tool said so and the AI corrected itself. The four lanes were LOCAL, PRIOR ART, PRIMARY SOURCE and REQUIREMENT. The primary-source lane cited, among others:

```
* Go standard library source: `src/runtime/mgc.go` (lines 81–86)
* Go GC pacer runtime source: `src/runtime/mgcpacer.go` (lines 648–779, 878–923)
* Go 1.19 Release Notes: `https://go.dev/doc/go1.19`
```

Recorded size of the run, from the program's own record of finished runs:

```
sessions 4, tokens in 506,693, tokens out 3,847, tool calls 27, 108.9 seconds
```

## /osint

The question was the 2024 CrowdStrike Falcon outage: cause, scale and the vendor's stated remediation. The record:

```
2 tool calls:
  research {"agents":4,"doctrine":"dossier","mode":"parallel","question":"What is known about the 202...  -> ok
  write  C:\Users\gorilla1\Documents\Gorilla-OSINT-Dossiers\dossier-26-10-06-crowdstrike-falcon-out...  -> ok
```

The four lanes were `official`, `scholarly`, `reporting` and `dissent`. The findings file the program saved before the AI assembled anything begins:

```
# Raw findings — What is known about the 2024 CrowdStrike Falcon sensor outage: cause, scale, and the vendor's stated remediation?

Saved automatically at 2026-10-06 04:36:09, before any model was asked to assemble them.

Doctrine: dossier
```

The cost block the tool returned:

```
helpers                 4  (4 sessions incl. supervisors)
tool calls              27
tokens processed (in)   274,703
tokens written (out)    6,168
ratio (in : out)        45 : 1
TOTAL COST              $0.00
```

The cost is nothing because a free quota has no price. The tokens are real and count against it.

## The cost screen, after those two runs

The line both cost screens now print on the owner's computer:

```
BASIS: 98.9K tokens per helper session, measured from 2 runs on this computer.
```

Before any run has finished on a computer, the same line reads:

```
BASIS: 21.8K tokens per helper session, assumed; no run measured yet.
```

## Every fault that was fixed

- A review whose only shortfall is uninstalled analysers is recorded as "ok, partial" with the numbers, not as a failure.
- The review summary states how many jobs were not installed, failed, and timed out.
- `/review` security runs the secret scanners, security tools and static analysis only; full runs everything; the report records which, and the summary claims only what the report confirms.
- The cost before `/research` and `/osint` is priced from the measured median helper session once a run has finished on your computer; before that it says the basis is assumed. Both screens say the same thing.
- The priced, supervised `/research` screen fits a 32-row terminal.
- The `/arsenal` entry for whois and DNS is two entries; DNS can be installed on Windows.
- The commit check used by the maintainer no longer crashes on a byte outside the Windows code page.
- The recorded size of the tool descriptions was re-measured and the document quoting it corrected.

## How it was tested

**The file you download is the file that was tested.** The Windows program was built once, tested, and published without being rebuilt.

**The program's own tests.** The complete set passes on Windows: 33 groups, no failures.

**Against a real AI, with nobody watching.** The published Windows file was started from a script seven times against NVIDIA's `nemotron-3-super-120b-a12b`. All seven runs gave a usable result.

**The three commands, end to end.** `/review` was run on the published file in all three settings with Gemini 3.6 Flash. `/research` and `/osint` were run with the same AI on the previous version's program plus this version's fixes, before the final build; the part of the program that does that work is identical in this file, byte for byte in its source. They were not run again on the published file.

**The real screens.** `/update`, `/model` and `/provider` were typed into the real program by a script and read back. Ctrl+V in the key box was proven on the published file, and on the installed 0.1.142, whose page had said it could not be proven while the screen was locked.

**Starting up.** Timed twice in the home folder, pressing Enter on the NVIDIA row: ready to type after 1.6 and 1.3 seconds.

**On real Linux, after publication.** Automatic checks on GitHub take the files from this page and install them on Ubuntu, Fedora and Arch Linux, and hold a conversation with an AI on Linux. This paragraph is replaced with their result when they have run.

**What is still true.** A search by name over a folder holding millions of files takes as long as the search engine takes to walk it: 12 to 20 seconds on this day with a cold disk.

## No new pictures, and why

Everything in this release is text the program prints, quoted above as printed.
No new screenshot was taken. The two below are from earlier releases and show
screens this release did not change, pinned to this version.

**The normal window**, where every action is listed as it happens. This is the
permission question.

[![Gorilla OpenCode showing a Permission Required dialog for the patch port tool, naming the folder it will modify and the patch series it will apply, with the three choices Allow, Allow for session and Deny, proving the program asks before a tool changes files](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.143/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.143/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)

**The command list.**

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.143/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.143/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

The three commands repaired in v0.1.142 were run end to end on the real binary with Gemini 3.6 Flash via the owner's Antigravity sign-in, on an isolated XDG configuration, by a new rig (scripts/model-tests/run_commands_e2e.py, private). /review on a three-fault sample repository with gitleaks, bandit and staticcheck installed found all three; /research (4 helpers) answered a GOGC/GOMEMLIMIT question citing runtime sources; /osint (4 dossier lanes) produced a graded dossier. The runs exposed: the receipt counted missing analysers as a failed call; focus security and full both sent --deep; the research forecast still used the assumed basis; the priced supervised research dialog overflowed 32 rows. All fixed, plus the whois/dns split and a UTF-8 fix in the pre-commit hook. Scope: 28 files changed, 1366 insertions, 306 deletions since v0.1.142.

### Known Alternatives Considered

The receipt could have kept treating any partial review as a problem; rejected because a run that executed every installed analyser and found the planted faults is not a failure, and the receipt's job is to be read as a verdict. The forecast could have kept the assumed basis and shown the measured figure beside it (the v0.1.142 state); rejected because two numbers for one quantity is what the owner objected to. Other alternatives: Not available in the source material.

### Architecture Impact

Run history moved from internal/llm/agent/research_measure.go to internal/config/research_runs.go (config cannot import agent); the agent file keeps thin wrappers. The review toolkit gains --security beside --quick, with depth_mode() as the single definition read by the scheduler, depth_of, network_report and preflight.

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.143" -o gorilla-opencode.exe .`. Linux: same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Windows binary sha256 a25524dd59ff208624afd0774c888379765323e7188230c3b6d4133da74f4fd8, built from the committed tree, tested, published unchanged. .deb and .rpm by nfpm.
```

### Resource Deltas

gorilla-opencode.exe: 54,704,128 bytes (0.1.142) -> 54,721,536 bytes (+17,408). Default tool schema cost re-measured: 9,733 -> 9,947 tokens. Launch with Enter on the NVIDIA row: 1.6 s and 1.3 s. End-to-end runs on Gemini 3.6 Flash: /research 4 helpers, 506,693 tokens in, 109 s; /osint 4 lanes, 274,703 tokens in, 97 s; /review security 25.9 s, full 14.8 s on the sample repository.

### Code Changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `internal/llm/tools/review.go` | modified | focusArgs: security and full both --deep. PARTIAL REVIEW line gave completed/planned only. | quick -> --quick, security -> --security, full -> --deep. Summary states DEPTH: security (linters and formatters skipped, from the report's depth block) or 'did not confirm'. PARTIAL REVIEW line carries (N not installed, N failed, N timed out). |
| `internal/llm/tools/codereview/toolkit/code_review.py, tools_registry.py` | modified | No security-only mode. | SECURITY_CATEGORIES = (recon, secrets, security, static-analysis); --security forces stage 3 on every file, filtered by category (vulture excluded); --quick with --deep or --security exits 2; depth_of reports mode security with categories and tools skipped. |
| `internal/app/receipt.go` | modified | Any PARTIAL REVIEW counted as a problem: 'partial: 6 of 17 jobs completed' under 'did not succeed'. | With the breakdown present, 0 failed, 0 timed out and no unreviewed language: 'ok, partial: N analysers ran, M not installed', not counted as a problem. Otherwise unchanged. |
| `internal/config/research_runs.go` | added | N/A - new file (moved from agent/research_measure.go). | RecordResearchRun, MeasuredRunSize, ResearchHelperSessionTokens() (in, out, runs, measured): median tokens per session over up to 20 recorded runs, split by the history's input share; assumed basis when none. |
| `internal/config/loadout.go` | modified | ResearchCost and ResearchPaidEquivalent priced StepsPerHelper x helper context + OutputPerStep. | helperSessionForecast(model, inFlight) shared by both; measured path prices the median session; ResearchQuotaMultiple and ResearchOrchestratorTokens use the same basis. Review loadout row no longer types 'A RUN is 4-10'. |
| `internal/tui/components/dialog/research.go, osint.go` | modified | 'expect about 1.6x the money shown' row; both screens could disagree on basis; priced supervised dialog 33 rows at 100x32. | forecastBasisPhrase() shared: 'BASIS: 98.9K tokens per helper session, measured from 2 runs on this computer.' or 'assumed; no run measured yet.' On compact screens the per-step context row is dropped once tokens are measured, the PUBLISHED row is dropped, and the warning loses its vertical padding. |
| `internal/arsenal/manifest.json, arsenal.go` | modified | whoisdns: whois+dig+host all-mode, no scoop package. Comment claimed 24 of 33 entries had a scoop package (measured 23). | dns (dig, host; apt dnsutils, pacman bind, scoop bind) and whois (apt, pacman; none on scoop, caveat says so). 34 entries, 24 with scoop. Comment corrected. |
| `internal/tui/reviewargs.go, internal/commands/registry.go, docs/COMMANDS.md` | modified | Prompts and help described security and full identically. | Each says exactly what it runs. |
| `internal/llm/agent/schema_cost_test.go, docs/WHAT-A-CURIOUS-USER-COSTS.md` | modified | Recorded 9,733 tokens; drift reached +501 against a 500 allowance. | Recorded 9,947; the document's table re-measured (9,947 / 2,314 / 369 = 12,630). |
| `scripts/git-hooks/hooks.py` | modified | subprocess text decoding in the console code page; crashed on byte 0x81 in a diff. | encoding utf-8, errors replace. |

### Subsystem Changes

**TOOLS:** review: --security mode, breakdown in the coverage line, depth confirmation for all three focuses.

**TUI:** Research and osint cost screens priced from the measured basis; compact research dialog shorter by two rows; arsenal dns/whois rows.

**STORAGE:** research-runs.json now written by internal/config (same path, same bound of 20).

**NETWORK:** No change.

**OTHER:** Receipt outcome 'ok, partial'. Pre-commit hook UTF-8.

### Test Coverage

- **Added:** internal/config/research_runs_test.go (3 tests); internal/tui/components/dialog/research_fit_priced_test.go (priced supervised dialog at 176x48, 120x40, 100x32, 90x30); TestBothCostScreensNameTheSameForecastBasis; review_run_test.go: TestSecurityAndFullClaimsAreOnlyMadeWhenTheRunConfirmsThem, TestARealSecurityRunSchedulesNoLinterAndReachesTheDeepStage; reviewargs_diff_test.go: TestTheSecurityAndFullPromptsSayWhatEachReallyRuns; arsenal_test.go: TestDnsAndWhoisAreSeparateEntries; receipt_test.go: three 'ok, partial' cases; toolkit tests/test_depth.py +61 lines.
- **Removed:** TestResearchScreenShowsMeasuredRunSizeBesideTheForecast (its premise, a forecast low by a multiplier, no longer holds); replaced by TestResearchScreenPricesTheMeasuredBasis.
- **Notes:** Complete suite passes on Windows: 33 packages, 0 failures. Windows binary against nvidia/nemotron-3-super-120b-a12b: 7 of 7 scenarios valid. /review run end to end on the published binary in quick, security and full with Gemini 3.6 Flash (Antigravity), on an isolated XDG configuration. /research and /osint were run end to end on the v0.1.142 binary plus this version's uncommitted fixes, before the final build; the research engine (internal/llm/agent/research-tool.go) has no diff between that run and this binary. Portal paste proven on the installed v0.1.142 and on this file. /update, /model, /provider driven in the real TUI. NOT RUN on this file: /research and /osint. Linux checks run on GitHub after publication.

### Security Posture

No change to permission, masking or command gates. The receipt change relaxes only the 'problem' count for reviews whose missing analysers are the sole shortfall; failures, timeouts and unreviewed languages still count. The end-to-end rig copies the Antigravity OAuth file into an isolated config directory; the rig is private (gitignored).

### Deployment

**Prerequisites:**
- Windows 10/11 x64, or linux/amd64.
- Python 3 on PATH for the find tool's fallback and for /review; gitleaks, bandit, staticcheck or other analysers for /review coverage.

```bash
# fetch
gh release download v0.1.143 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, .deb, .rpm, SHA256SUMS-v0.1.143.txt.
# verify
sha256sum -c SHA256SUMS-v0.1.143.txt --ignore-missing
# Expected: Each present file followed by OK. Windows exe: a25524dd59ff208624afd0774c888379765323e7188230c3b6d4133da74f4fd8.
# install
.\gorilla-opencode.exe install
# Expected: Install path and shortcuts.
# verify_active
gorilla-opencode --version
# Expected: v0.1.143
# verify_active
go test ./internal/config -run 'ForecastBasis' -count=1
# Expected: ok (from a source checkout).
```

**Rollback:**
  1. Remove this version
     `gorilla-opencode uninstall   # or apt remove / dnf remove gorilla-opencode`
  2. Install v0.1.142 from its release
     `gh release download v0.1.142 -R gorillanobakaa-dot/Gorilla.Opencode`

### Known Issues

**[low]** A /review receipt reads 'partial: N of M jobs completed' (counted as a problem).
- Cause: An analyser errored or timed out, or a language in scope had no analyser complete.
- Remedy: Read the Trust block: Failed to run / Timed out / Languages with NO completed analyser.

**[low]** focus=security reports fewer analysers than focus=full.
- Cause: Linters and formatters are excluded by design in security mode.
- Remedy: Expected. Use full for everything.

**[low]** The research forecast differs from the previous version's.
- Cause: research-runs.json holds at least one finished run; the median session is now the basis.
- Remedy: Expected; the BASIS line states the source. Delete research-runs.json in the cache directory to return to the assumed basis.

**[low]** Cost line shows $0.00 on a free provider.
- Cause: No price for the model.
- Remedy: Read the token figures; they count against quota.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| All three planted faults found | 📄 stated in input | All three faults were found and reported with file and line |
| Research run size and duration | 📄 stated in input | The run processed 506,693 tokens and took 109 seconds |
| Dossier run size and duration | 📄 stated in input | The run processed 274,703 tokens and took 97 seconds |
| Receipt said did not succeed | 📄 stated in input | still said "did not succeed: partial: 6 of 17 jobs completed" |
| security and full identical before | 📄 stated in input | sent the same instructions to the toolkit |
| security ran 5, full ran 6 | 📄 stated in input | security ran 5 analysers and full ran 6 |
| Measured basis 98.9K on the owner's machine | 📄 stated in input | the basis on the owner's computer reads 98.9K tokens per helper session |
| Dialog 33 rows at 32 | 📄 stated in input | the /research screen was 33 rows on a 32-row terminal |
| Schema cost 9,733 to 9,947 | 📄 stated in input | 9,947 tokens with the default tools on, up from 9,733 |
| Hook crash on a non-code-page byte | 📄 stated in input | The commit check script crashed on a byte outside Windows' default code page |
| Binary hash and size | 📄 stated in input | sha256 a25524dd59ff208624afd0774c888379765323e7188230c3b6d4133da74f4fd8, 54,721,536 bytes |
| Suite and scenarios | 📄 stated in input | All seven gave valid results |
| Launch times | 📄 stated in input | ready to type after 1.6 seconds and 1.3 seconds |
| research and osint not re-run on the final file | 📄 stated in input | the /research and /osint runs above were made on the 0.1.142 program plus the uncommitted fixes of this version, before the final build |
| Scope | 📄 stated in input | 28 files changed, 1366 insertions, 306 deletions since v0.1.142 |
| The research engine has no diff between the run and the binary | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*
