# Gorilla OpenCode 0.1.142 — Windows and Linux

**A very capable AI was asked to find a file, and this program told it the file did not exist. Fifteen copies were on the disk. This version repairs the search tool, shortens the list of AIs to the ones you can use, and fixes four commands that were saying things that were not true.**

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

**Yes, everyone.** The search tool is what the AI uses to look round your
computer. When it gives a wrong answer, the AI gives you a wrong answer, however
good the AI is.

**Yes, if you use a provider with a key** such as NVIDIA. The list of AIs was
half full of entries that could not be used.

**Yes, if you use `/review`, `/arsenal`, `/research` or `/osint`.** Each of them
had faults, and several of those faults were the program telling you something
false.

**One thing to know before you rely on it:** the search tool and the list were
tested on the real program with a real AI. The repairs to `/review`,
`/research` and `/osint` were checked by the program's own tests only. A full
run of those costs money, or needs tools that are not on the test computer.

## Why this matters to you

Think of sending a capable assistant into an archive to fetch a document. The
assistant does not know the building. It works from the catalogue at the front
desk.

If the catalogue says *no such document*, the assistant comes back and tells you
there is no such document. The assistant is not being stupid. The catalogue is
wrong.

In this program the AI is the assistant and the search tool is the catalogue.

The owner asked one of the largest AIs available two plain questions, in his own
home folder. It took four minutes. He sent the whole conversation and said that
an AI of that size ought to manage this. He was right, and the fault was ours.

**First question: where is the file called gemini.md?** This is what the search
tool answered:

```
No matches found.
```

The files are named `GEMINI.md`, in capitals, and the tool was fussy about
capitals. The AI tried again, adding the kind of file, and was handed every
Markdown file on the computer: **120,765** of them. It found the right answer on
its fifth try, by guessing the capitals.

**Second question: list the folders.** The tool drew the first 40 files it
happened to walk past and stopped, without saying it had stopped. It showed the
Documents folder as holding one file. The AI reported that to the owner as his
folder structure. Documents holds more than three million files.

**Now** a name is found whatever its capitals. A name and a kind together mean
both. And a folder too large to draw is answered with its folders and how much
is in each, with a plain statement that this is not everything.

The same two questions, the same AI, the same folder, on this version: each was
answered correctly from one successful search.

**The list of AIs.** The owner's list had 82 rows. The provider goes on listing
AIs after it has switched them off, and lists tools that are not chat AIs at
all, and this program was repeating all of it, with opinions typed in months ago
printed beside them. One AI was marked as failing, and then carried his whole
conversation without trouble.

**Now** the list shows 41 rows on his computer: the ones that can be used, with
those seen working at the top. A line says how many were left out and why. An AI
that works when you use it loses its failure mark.

**Four commands that were not telling the truth.** An audit of `/review`,
`/arsenal`, `/research` and `/osint` found about fifty faults. The worst:

- `/review` with the quick option ran its security checks, then said they had
  been skipped and that it could not have found a leaked password.
- `/review` wrote its results inside your project, including any password it
  found, in full.
- `/osint` promised to plan your question into sub-questions. No such step
  existed. Every question was worked as if it were a programming problem, and
  one helper searched your own computer.
- `/research help` started a paid run researching the word *help*.
- `/arsenal` said ImageMagick was installed on every Windows computer.

All of these are fixed. The full list is further down this page.

**What it means for you, in one line:** when the AI looks for something on your
computer, what it is told is now true.

**What it does not do:** it does not make a search of a huge folder faster. The
owner's home folder holds 3.9 million files, and a search by name there still
takes about nine seconds.
---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why This Release Exists

The owner sat down with one of the largest AIs available, in his own home folder, and asked it two plain questions: list my folders, and find a file called gemini.md. It took four minutes and five attempts, and one of the two answers was wrong. He sent the whole conversation and said, fairly, that an AI of that size should manage this.

It should, and it was not the AI's fault. The AI can only see the computer through the tools this program hands it, and the search tool gave it three wrong answers in a row. Asked for gemini.md it said there was no such file, because the files are named GEMINI.md and it was fussy about capitals. Asked again with the kind of file added, it returned every Markdown file on the computer, 120,765 of them. Asked for the folders, it drew the first 40 files it happened to walk past and stopped without saying it had stopped.

It is like sending a capable assistant into an archive with a catalogue that is wrong. The assistant is not the problem. The catalogue is.

All three are fixed, and the same two questions now get correct answers on the first search.

The owner also sent a picture of the list of AIs to choose from. It had 82 rows. A third of them were AIs the provider had already switched off, and others were not chat AIs at all. The list now shows only what can be used, 41 rows on his computer, with the ones that were seen working at the top.

Last, he asked for four commands to be looked at: /review, /arsenal, /research and /osint. An audit found about fifty faults in them. Several were the program saying something that was not true, which is the worst kind. They are fixed, and the largest are listed below.

### What You Will Notice

**Finding a file by name**
- Before: Capitals had to match exactly. Asked for gemini.md, it said no such file existed while 15 files named GEMINI.md were on the disk.
- After:  Capitals do not matter. The answer also states how many files were found, so nobody has to count.
- Affects: Everyone

**Finding a file by name and by kind together**
- Before: It returned files matching either one: 120,765 files for a name that matches 15.
- After:  A file must match both. The same search returns 15.
- Affects: Everyone

**Asking for the folders of a very large folder**
- Before: It drew the first 40 files it met and stopped silently. It showed Documents as holding one file.
- After:  It shows the folders with the number of files inside each, and says plainly that this is not the whole tree.
- Affects: Anyone who starts the program in a home folder or another large folder

**The list of AIs from a provider with a key**
- Before: 82 rows, including AIs the provider had switched off and entries that cannot hold a conversation, with old typed opinions beside them and no sensible order.
- After:  41 rows on the owner's computer. Working AIs first. A line says how many were left out and why. The typed opinions are gone.
- Affects: Anyone using NVIDIA, Cloudflare or another provider with a key

**The cost shown for a free provider**
- Before: The bar at the bottom said $0.11 for a conversation that cost nothing.
- After:  A provider that reports no price shows no cost.
- Affects: Anyone using a free provider

**/review with the quick option**
- Before: It ran the security checks and then said they had been skipped and that it could not have found a leaked password.
- After:  Quick runs only the fast style checks, and says the security checks were skipped only when that is true.
- Affects: Anyone using /review

**Where /review leaves its results**
- Before: Inside the folder being reviewed, including any password it found, written out in full.
- After:  In the program's own private folder, with found passwords blanked out.
- Affects: Anyone using /review

**/arsenal on Windows**
- Before: It said ImageMagick was installed on every Windows computer, because Windows has an unrelated tool with the same name. It said LibreOffice was missing when it was installed.
- After:  The Windows tool is recognised and not counted. LibreOffice is found where it really is.
- Affects: Everyone on Windows

**/osint**
- Before: The page promised that your question is planned into sub-questions. In fact every question was worked as if it were a programming problem, and one helper searched your own computer.
- After:  A question is worked in ten fixed lanes suited to research, such as official sources, published studies, news reporting and opposing views. No helper can search your computer. The page describes what really happens.
- Affects: Anyone using /osint

**/research help and /osint help**
- Before: The word help was taken as the question, and pressing Enter started a paid run researching the word help.
- After:  Help words show the help.
- Affects: Anyone using /research or /osint

### Deliberately Not Done

- **Making a name search of a huge folder faster** — The owner's home folder holds 3.9 million files. Walking past that many takes about 9 seconds even for the search engine on its own. The answer is now right; it is not quicker.
- **Running the four repaired commands from start to finish with a paid AI** — A /research or /osint run costs real money and many minutes, and none of the checking tools that /review drives is installed on the owner's computer. Their repairs were checked by the program's own tests. This is the main limit of this release.
- **Proving again that Ctrl+V works in the key box** — The check needs the Windows clipboard, and Windows refuses the clipboard while the screen is locked, which it was. The same check failed the same way on the previous version, which had passed it earlier, and that part of the program has not changed.
- **The whois entry in /arsenal on Windows** — The Windows package manager this program uses has no whois package, so that entry stays unavailable there.
- **Correcting the money estimate for /research** — It is still lower than what finished runs really used. The screen now shows the measured size beside the estimate, so you can see the gap. The estimate itself is not yet corrected.

### Privacy & Security

No telemetry was added and none exists. Three things are better for privacy. /review no longer leaves passwords it finds written inside your project folder. One of the checking tools /review drives used to report usage figures to its maker; that is switched off. And two screens that used to stay silent now say what leaves your computer: the /review permission question names the tools that use the internet, and the /osint page says that your question and search words go to the AI provider and to the search services.

### How to Install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora or similar Linux computer.
- Enough mobile data for a 55 MB file on Windows or a file of about 24 MB on Linux.
- On Windows you do not need to close the program first; an open window stays on the old version until you close it.

**Step 1:** Open the release page in your web browser and download the one file for your computer. Windows: gorilla-opencode.exe. Debian, Ubuntu or Mint: gorilla-opencode_0.1.142_amd64.deb. Fedora, openSUSE or Rocky: gorilla-opencode-0.1.142-1.x86_64.rpm. Also download SHA256SUMS-v0.1.142.txt.
✓ Two files are in your Downloads folder.

**Step 2:** Check the file is the one that was published. On Windows, press the Windows key, type PowerShell, press Enter. A window with a blinking cursor opens. Type cd Downloads and press Enter. Then type the command below and press Enter.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ It prints 8046e42a7e92990881716fd9fda97a8da29045ac8c362d6024ec41290f1f8987. On Linux, run sha256sum -c SHA256SUMS-v0.1.142.txt --ignore-missing in a terminal in your Downloads folder; it prints the file name followed by OK.

**Step 3:** Windows: install it. Type the command below and press Enter.
```
.\gorilla-opencode.exe install
```
✓ It reports where it copied itself and the shortcuts it made.

**Step 4:** Debian, Ubuntu or Mint instead: type the command below and press Enter. It asks for your password because installing a program changes the system.
```
sudo apt install ./gorilla-opencode_0.1.142_amd64.deb
```
✓ A line says Setting up gorilla-opencode (0.1.142), with no line beginning with E:.

**Step 5:** Fedora, openSUSE or Rocky instead: type the command below and press Enter.
```
sudo dnf install ./gorilla-opencode-0.1.142-1.x86_64.rpm
```
✓ The last line says Complete!

**Step 6:** Confirm the version. Type the command below and press Enter.
```
gorilla-opencode --version
```
✓ It prints v0.1.142.

**To go back:** Windows: type gorilla-opencode uninstall and press Enter, then install the gorilla-opencode.exe from version 0.1.141 the same way as in step 3. Debian: sudo apt remove gorilla-opencode. Fedora: sudo dnf remove gorilla-opencode. Your conversations, settings and keys are stored separately and none of these steps touches them.

### If Something Goes Wrong

**The list of AIs is much shorter than before.**
AIs the provider has switched off, and entries that are not chat AIs, are no longer shown. The line under the title says how many were left out.
What to do: Nothing to do. Type /update and press Enter to test the list again.
Status: expected behaviour, new in 0.1.142

**The cost at the bottom of the window stays at nothing on NVIDIA or a local AI.**
Those providers do not report a price, and the program no longer makes one up.
What to do: Nothing to do.
Status: expected behaviour, new in 0.1.142

**Asking for the folders of a large folder shows folders with numbers, not every file.**
There are too many files to list. The numbers are how many files each folder holds.
What to do: Ask about one of the folders by name to see inside it.
Status: expected behaviour, new in 0.1.142

**/research or /osint refuses to start and names a switch.**
The research tool is switched off in the tools list, so the AI would not have been able to do the work.
What to do: Switch it on in the tools list the message names, then try again.
Status: expected behaviour, new in 0.1.142

**/review, /research or /osint does something wrong that this page says was fixed.**
These repairs were checked by the program's own tests, not by a full paid run.
What to do: Report what you typed and what it printed.
Status: being watched

### Common Questions

**Q: Was the AI at fault in the owner's conversation?**
A: No. It was told a file did not exist when 15 did, and it was shown 40 files as if they were the whole folder. It reported what it was shown. The one slip that was its own was saying 14 when it had listed 15, and the search now states the count so that cannot happen.

**Q: Why did the list of AIs have rows that could not be used?**
A: The provider goes on listing AIs after switching them off, and lists tools that are not chat AIs. The program used to repeat the provider's list. Now it leaves those out and tells you how many.

**Q: Does anything in this version cost me more?**
A: No. /osint now refuses a third run for one question, which used to be possible, so its cost has a real ceiling.

### Bottom Line

Take this version. If you ask the AI to find files or look round a large folder, it now gets true answers from the search tool where before it could be told that a file did not exist. The list of AIs shows only the ones you can use. Four commands that said things that were not true now say what they do. The honest limit: the search and the list were tested on the real program with a real AI, but the repairs to /review, /research and /osint were checked only by the program's own tests, because a full run of those costs money or needs tools that are not installed here.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Asked for gemini.md it said there was no such file while 15 existed | 📄 stated in input | it answered "No matches found" while 15 files named GEMINI.md existed |
| Name plus kind returned 120,765 files and now returns 15 | 📄 stated in input | returned 120,765 files. It now returns 15 |
| The tree drew the first 40 files and showed Documents as holding one file | 📄 stated in input | drew the first 40 files of the walk and stopped without saying so |
| The home folder holds 3.9 million files | 📄 stated in input | The home folder holds 3,835,116 files in 338,714 folders |
| The list went from 82 rows to 41 | 📄 stated in input | on the owner's machine it shows 41 rows |
| The bar said $0.11 for a free conversation | 📄 stated in input | the status bar said $0.11 for a conversation that cost nothing |
| /review quick ran security checks and said they were skipped | 📄 stated in input | quick ran the security analysers and then told the model they had been skipped entirely |
| /review results held found secrets inside the reviewed folder | 📄 stated in input | the raw logs there could hold any secret found, in full |
| ImageMagick was reported present on every Windows computer | 📄 stated in input | the name convert finds Windows' own disk converter |
| /osint had no planning and one lane searched the user's machine | 📄 stated in input | No planning code existed: every dossier ran the software-engineering lanes |
| Help words started a paid run | 📄 stated in input | pressing Enter started a paid run about the word help |
| A name search still takes about 9 seconds | 📄 stated in input | a name search over that home folder takes about 9 seconds |
| The four commands were not run end to end with a paid model | 📄 stated in input | none of the four audited commands was run end to end with a paid model |
| Ctrl+V could not be re-proven because the screen was locked | 📄 stated in input | Windows refused it ("Requested Clipboard operation did not succeed") because the screen was locked at the time |
| Seven real-AI scenarios passed on the published Windows file | 📄 stated in input | All seven gave valid results |
| A third dossier call is refused | 📄 stated in input | the program refuses a third dossier call for one question |


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

Every block below is real output or a real measurement, taken on the maintainer's computer. Nothing here is an illustration. Where lines were left out, it says so.

## The search tool, before

From the owner's conversation on version 0.1.141. The question was where a file called gemini.md is. First search, by name:

```
No matches found.
```

Second search, the same name with the kind of file added. These are the first lines of the answer, and its last line:

```
C:\Users\gorilla1\scoop\apps\zig\0.16.0\README.md
C:\Users\gorilla1\go\pkg\mod\software.sslmate.com\src\go-pkcs12@v0.7.1\README.md
C:\Users\gorilla1\scoop\buckets\versions\README.md
… and 120750 more file(s) not shown (raise --limit)
```

None of those is the file. The last line also tells the AI to change a setting it has no way to reach.

## The search tool, now

The same search by name, in the same folder. The last lines of the answer:

```
C:\Users\gorilla1\Documents\Build.Work\firefox\155.0.1\media\libyuv\libyuv\GEMINI.md
C:\Users\gorilla1\Documents\Build.Work\firefox\155.0.1\third_party\libwebrtc\GEMINI.md
[15 files found; this list is complete]
```

Asked for the folders of the home folder. The first line and some of the 71 lines that follow; the owner's project folders are left out of this page:

```
C:\Users\gorilla1: 3,840,385 files in 335,740 folders. That is too many to list one by one, so this shows FOLDERS ONLY, 2 level(s) deep, each with the number of files inside it (sub-folders included). Hidden and git-ignored files are not counted. This is NOT the whole tree.
AppData\  (249,905 files)
Desktop\  (5 files)
Documents\  (3,392,451 files)
Downloads\  (2 files)
Pictures\  (70 files)
scoop\  (148,215 files)
(5 file(s) sit directly in C:\Users\gorilla1, not listed here.)
To see inside one folder, call find again with path set to that folder.
```

## The same questions, asked of a real AI on this version

The built program was started without a window, in the home folder, on NVIDIA's Nemotron 3 Ultra. After each answer the program prints its own record of what ran. That record is written by the program, not by the AI.

The gemini.md question. The first call was a search inside every file, which the program refuses in a home folder and says why; the AI then searched by name and listed all fifteen files:

```
--- what actually ran (recorded by Gorilla OpenCode, not written by the AI) ---
2 tool calls, 1 did not succeed:
  find   gemini.md  -> failed
  find   {"glob":"**/gemini.md","files_only":true}  -> ok
```

The folders question:

```
--- what actually ran (recorded by Gorilla OpenCode, not written by the AI) ---
1 tool call:
  find   C:\Users\gorilla1  -> ok
```

## The list of AIs

Read off the screen of the real program after typing `/model`. Before: 82 rows. Now the counter at the foot of the list reads:

```
1/41  down ->
```

The first rows, as drawn:

```
[NVIDIA NIM] Nemotron 3 Ultra 550B  (answered with a tool call on 2026-10-05)
[NVIDIA NIM] Nemotron 3 Super 120B  (answered with a tool call on 2026-10-05)
[NVIDIA NIM] Nemotron 3.5 Lightning  (answered with a tool call on 2026-10-05)
[NVIDIA NIM] GPT-OSS 20B  (answered with a tool call on 2026-10-05)
```

On version 0.1.141 the first of those rows read `provider error 500 on 2026-10-05`.

## Every fault that was fixed

**Search tool**

- A name is found whatever its capitals.
- A name and a kind of file together mean both.
- A folder too large to draw is shown as folders with file counts, and says it is not the whole tree.
- A list that was cut short says so, and says how to narrow it, without naming a setting the AI cannot reach.
- A complete list states how many files it holds.

**List of AIs**

- AIs the provider has switched off are left out, and counted.
- Entries that are not chat AIs are left out, and counted.
- AIs seen working come first.
- Opinions typed into the program long ago are no longer shown. A verdict from this project's own testing is kept and marked "tested here".
- A provider that reports no price shows no cost.
- A server error is tried once more before an AI is marked as failing.
- A failure that may pass says "may work now".
- An AI that works in real use loses its failure mark.

**/review**

- Quick runs only the fast checks, and says the security checks were skipped only when that is true.
- Results are kept in the program's own private folder, not inside your project.
- Passwords it finds are blanked out in its logs.
- The permission question names the checking tools that use the internet. One of them no longer reports usage figures to its maker.
- On Windows, checking tools installed through npm are found. Before, they were reported as not installed.
- Reviews of C and C++ code no longer fail at the end with "output could not be read".
- Nothing to review is reported as nothing to review, not as a failure.
- A stopped run carries the reason.
- `/review --diff` followed by a folder name treats it as a folder.
- A wrong value for an option is blamed on the value, not on the option.
- A typed count of checking tools was removed. The help says Python 3 is needed.
- On Windows, checking tools still running when the time limit is reached are stopped with it.

**/arsenal**

- A Windows tool that shares a name with ImageMagick is no longer counted as ImageMagick. The same was fixed for one tool on Linux.
- LibreOffice installed off the usual search path is found, with a warning.
- A selection that cannot be fetched is reported as not measured. Before, it said "nothing to download".
- On Windows, where the package manager cannot say how big a download is, the screen no longer offers to measure it.
- An answer from the package manager that cannot be read is reported as not measured.
- Leaving the install plan returns to the screen it was opened from.
- The key that sends your selection to the AI, which costs one turn, is shown on screen.
- Tools that cannot exist on your system are not counted against you, and a group with nothing for your system is not listed.
- The download time uses your measured link speed when there is one, and says when it is an assumption.
- Install commands are shown one per line.
- Clearing the selection says so. A selection file from someone else can be loaded.
- The help no longer claims a transcription tool that is not there.
- Long lines are folded to the window, and long lists scroll.

**/research and /osint**

- Both refuse at once, naming the switch, when the research tool is off.
- Help words show help.
- `/osint` works a question in ten research lanes; no helper can search your computer; the page says so.
- The follow-up round is described as optional, and a third run for one question is refused.
- Figures typed into the `/osint` page were removed or are now counted from what a helper is given.
- Run sizes shown before you start come from finished runs on your computer, or the screen says there is no measurement yet.
- The time shown for a run uses measured helper times when there are any.
- Rules that are only instructions to the AI are no longer listed as guarantees.
- The `/osint` page says what leaves your computer.
- The warning about the supervised mode no longer says every lane is checked twice, which was false for larger runs.
- The screen points to the key that changes the helper AI. Before, it pointed to a settings file.
- Findings files are private to you and are never overwritten.
- An interrupted run can be recovered whichever provider it ran on, and is put back together as the kind of run it was.
- A helper that was cancelled is waited for before the run reports its spend.
- With web search not set up, the instructions fit the system you are on.
- The helpers' source list no longer points at a file that is not shipped.

## How it was tested

**The file you download is the file that was tested.** The Windows program was built once, tested, and published without being rebuilt.

**The program's own tests.** The complete set passes on Windows: 33 groups, no failures. New tests cover each search fault on both search engines, the list of AIs, and the repairs to the four commands.

**Against a real AI, with nobody watching.** The published Windows file was started from a script seven times against NVIDIA's `nemotron-3-super-120b-a12b`. All seven runs gave a usable result. The two questions from the owner's conversation were asked again, as shown above.

**The real screens.** `/update`, `/model` and `/provider` were typed into the real program by a script, and the list of AIs was read off its screen.

**On real Linux, after publication.** Automatic checks on GitHub took the files from this page. The .deb was installed, started and removed on Ubuntu. The .rpm was installed, started and removed on Fedora. The Arch package was built on Arch Linux, installed, started and removed, and is attached below. The program, installed from the .deb, held a conversation with an AI running on the same Linux machine. All passed.

**Starting up.** Timed twice in the home folder, pressing Enter on the NVIDIA row: ready to type after 1.56 and 1.93 seconds.

**What was not tested, and why.**

- `/review`, `/research` and `/osint` were not run from start to finish with an AI. A research run costs money and many minutes, and none of the checking tools `/review` drives is installed on the test computer. Their repairs rest on the program's own tests.
- Pasting a key with Ctrl+V could not be proven again on this file. The check needs the Windows clipboard, and Windows answered `Requested Clipboard operation did not succeed` because the screen was locked. The same check failed in the same way on version 0.1.141, which had passed it before. That part of the program has not changed since.
- The reading of package-manager answers on Linux was tested with typed samples, not with answers captured from a real Linux computer.

**What is still wrong.**

- A search by name over a folder holding millions of files takes about nine seconds.
- The money estimate for `/research` is lower than finished runs really used. The measured size is now shown beside it.
- The whois entry in `/arsenal` is unavailable on Windows.

## No new pictures, and why

Everything in this release is text the program prints, quoted above as printed.
No new screenshot was taken. The two below are from earlier releases and show
screens this release did not change, pinned to this version.

**The normal window**, where every action is listed as it happens. This is the
permission question.

[![Gorilla OpenCode showing a Permission Required dialog for the patch port tool, naming the folder it will modify and the patch series it will apply, with the three choices Allow, Allow for session and Deny, proving the program asks before a tool changes files](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.142/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.142/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)

**The command list.**

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.142/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.142/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Three defects in the find tool were reproduced from an owner transcript and fixed: name globs were case-sensitive (now passed as --iglob); a glob combined with a type was sent to ripgrep as two -g includes, which ripgrep ORs (type is now applied to the returned names when a caller glob is present); the tree view truncated the walk at 40 files without notice (an oversized tree is now rendered as folders with cumulative file counts). The picker for endpoint-served models now drops 404/410 verdicts and non-chat ids, orders by probe verdict, and carries no typed description or price. ProbeChat retries one 5xx; a tool-call answer in real use overwrites a failed probe verdict. An audit of /review, /arsenal, /research and /osint found about fifty defects; the fixes are listed under code_changes. Scope: 48 files changed, 8026 insertions, 667 deletions since v0.1.141.

### Known Alternatives Considered

Intersecting glob and type inside ripgrep is not possible: an override whitelist match is accepted before the type matcher is consulted, so the type is applied in pfind after the walk. Hiding retired models is a deliberate reversal of the earlier rule that the picker never hides a model; that rule is kept for taste (small or old models) and dropped for models the provider answers 404/410 for. A real planning stage for /osint was not built; a fixed dossier role set was chosen and the texts changed to match. A gap-round engine was not built; the ceiling on dossier calls is enforced in Go. Other alternatives: Not available in the source material.

### Architecture Impact

New files: internal/llm/models/picker_order.go, internal/tui/researchroute.go, internal/llm/agent/research_measure.go. The agent loop writes to the probe verdict cache (model-probes.json) on a tool-call completion, at most once per 12 hours per model. /review results move from <target>/.code_review to the cache directory. The review toolkit gains --quick and --network-report. The arsenal manifest gains detect.impostors, detect.paths and platforms.

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.142" -o gorilla-opencode.exe .`. Linux: same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Windows binary sha256 8046e42a7e92990881716fd9fda97a8da29045ac8c362d6024ec41290f1f8987, built from the committed tree, tested, published unchanged. .deb and .rpm by nfpm.
```

### Resource Deltas

gorilla-opencode.exe: 54,539,264 bytes (0.1.141) -> 54,704,128 bytes (+164,864). find glob=**/gemini.md type=md over a 3.9M-file home directory: 120,765 results -> 15. Picker rows for the owner's endpoints: 82 -> 41. Launch with Enter on the NVIDIA row: 1.56 s and 1.93 s (two runs; 0.1.141 measured 1.43 s and 1.40 s with no other load). A name search over that directory remains about 9 s, which is the ripgrep walk.

### Code Changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `internal/llm/tools/find.go` | modified | Caller glob passed as --glob (case-sensitive). | Passed as --iglob. Parameter description states that capitals do not matter and that glob with type means both. |
| `internal/llm/tools/pfind.py` | modified | Type globs and caller globs both emitted as -g includes (OR). Tree view sliced the file list to --limit before rendering. Cut lists printed '(raise --limit)'. | _split_type_filter withholds type/ext includes from ripgrep when a caller include exists and filters returned paths by basename; applied in rg_list_files and rg_content. render_folder_overview prints folders with cumulative counts when the list exceeds the limit. Cut lists say INCOMPLETE and how to narrow; complete glob listings state their count. |
| `internal/llm/models/picker_order.go` | added | N/A - new file. | PickerOrder(ids) returns shown ids in candidateOrder plus counts of ProbeGone and chatTier 9 ids left out. NoteAnsweredInUse(id, cacheDir) records ProbeWorks for an endpoint-routed model unless a usable verdict under 12 h old exists. |
| `internal/llm/models/probe.go` | modified | One request per model; a 5xx recorded as provider-error. Labels for 5xx, 429 and timeout read as final. | ProbeChat repeats the request once after probeRetryPause (2 s) on a 5xx. Those three labels end '; may work now'. |
| `internal/llm/models/local.go` | modified | convertLocalModel copied Description and four cost fields from metadata/nim.json. | Description is empty unless EarnedVerdict matches ('tested here: ...'). Cost fields are zero. A typed name is cut at ' ('. |
| `internal/tui/components/dialog/models.go` | modified | ProviderLocal rows in coding-heuristic order, all registry entries. | Rows from models.PickerOrder; the connection line states the counts left out. |
| `internal/llm/agent/agent.go` | modified | No feedback from use to the verdict record. | On EventComplete with at least one tool call, models.NoteAnsweredInUse is called. |
| `internal/llm/tools/review.go` | modified | quick mapped to --no-stage3 (stages 0-2, including security analysers, still ran) while the summary stated security stages were skipped. cmd.Output discarded stderr. Results under <target>/.code_review. manual_steps decoded as []string; results_dir read where the toolkit writes logs_dir. | quick sends --quick; the skipped statement is printed only when the report says mode quick. stdout and stderr captured separately; exit 0 with empty stdout is 'nothing in scope'. --results-dir under CacheBase()/code-review/results, 0700, pruned to 20. Permission text built from --network-report. Struct fields corrected. On Windows the process tree is killed on timeout. |
| `internal/llm/tools/codereview/toolkit/*.py` | modified | No lint-only mode. subprocess.run with a bare name (fails on .cmd shims). semgrep metrics on; gitleaks -v. | --quick (recon, lint, format categories), --network-report, Tool.network field, resolve_argv via shutil.which, semgrep --metrics=off, gitleaks --redact. |
| `internal/tui/reviewargs.go` | modified | --diff consumed the next word as the ref unconditionally; a bad --focus value was reported as an unknown option; quotes kept in paths. | A word after --diff that is a path and not a resolvable ref is the path. Bad values are blamed on the value. Quote-aware tokeniser. |
| `internal/arsenal/arsenal.go, manifest.json` | modified | exec.LookPath hit counted as present (system32\convert.exe satisfied imagemagick). MeasureCost(nil) and a regex miss returned measured 0 bytes. Entries impossible on the platform counted against a series. scoop commands joined with ';'. | detect.impostors rejects named impostors; detect.paths finds off-PATH installs; platforms excludes inapplicable entries and empty series; zero packages or an unparsed answer is not measured; CanMeasure gates the price hints; one command per line. |
| `internal/tui/researchroute.go, tui.go` | added | /research and /osint did not check tool.research; help words were sent as the question. | Route function: help words first, then requireTool checks; recovery needs neither switch. |
| `internal/llm/agent/research-tool.go` | modified | One role set for every doctrine, including a lane that searches the local machine. runWave returned on cancel without wg.Wait. No limit on dossier calls. RunShape used an assumed duration only. | dossierRoles (ten lanes) with web tools only; standard role ids refused on a dossier run. launchWave always waits. reserveDossierCall allows two dossier calls per user message. RunShape uses MeasuredSecondsPerHelper when present. Source and lane counts computed. |
| `internal/llm/agent/research_recover.go, research_salvage.go, research_measure.go` | modified | Helper ids parsed by ^(call_[A-Za-z0-9]+)-(.+)$ (failed for UUID and toolu_ ids). Findings 0644, minute-resolution names, no doctrine recorded. | splitHelperID splits on the known role-id suffix. Findings 0600 in a 0700 directory, seconds in the stamp, O_EXCL with a numeric suffix, Doctrine line; AssemblyPrompt branches on it. Finished run sizes recorded in research-runs.json. |
| `internal/llm/tools/websearch.go` | modified | Unconfigured-search refusal gave apt and a Linux path on every OS, with typed size and time. | searxngSetupFor(goos); one source list feeds the enum, the texts and the count. |
| `internal/tui/components/dialog/osint.go, osintpage.go, research.go` | modified | Typed registry counts and a typed tokens-per-minute constant; a planning stage and a gap round described as code; prompt instructions listed as iron rules; no statement of egress. | Computed counts or none; measured run size or 'NO MEASUREMENT YET'; fixed lanes; optional follow-up with its ceiling; instructions labelled as such; a LEAVES paragraph. |

### Subsystem Changes

**TOOLS:** find: iglob, glob AND type, folder overview, stated counts. review: real quick mode, results location, egress disclosure, stderr reporting. web_search: OS-specific setup text.

**TUI:** Model picker for endpoints filtered and ordered by verdict. /research and /osint routing. Arsenal, research and osint dialogs corrected.

**NETWORK:** semgrep metrics off. One extra probe request after a 5xx. No new destinations.

**STORAGE:** Review results under the cache directory (0700). Findings 0600/0700. New cache file research-runs.json. model-probes.json written from the agent loop.

**OTHER:** Pre-commit hook decodes git output as UTF-8 (it crashed on a non-cp1252 byte in a diff).

### Test Coverage

- **Added:** internal/llm/tools/find_names_test.go (5 tests, both engines); internal/llm/models/picker_order_test.go (5 tests); internal/llm/tools/review_run_test.go; toolkit tests/test_depth.py; internal/tui/reviewargs_diff_test.go; internal/tui/researchroute_test.go; internal/llm/agent/research_audit_20261005_test.go; internal/tui/components/dialog/research_audit_20261005_test.go; additions to internal/arsenal/arsenal_test.go and dialog/arsenal_test.go.
- **Removed:** TestEmptySelectionCostsNothingAndKnowsIt (asserted the defect; replaced by TestZeroPackagesIsNotMeasuredNeverFree).
- **Notes:** Complete suite passes on Windows: 33 packages, 0 failures. Windows binary against nvidia/nemotron-3-super-120b-a12b: 7 of 7 scenarios valid. Both transcript questions re-asked on the built binary against Nemotron 3 Ultra. /update, /model and /provider driven in the real TUI through the console input buffer; picker read from the screen buffer. NOT RUN: the portal paste probe (Windows refused clipboard access with the session locked; the same probe failed identically on v0.1.141; internal/tui/startup has no diff since v0.1.141). NOT RUN: /review, /research and /osint end to end against a model; no /review analyser is installed on the test machine. apt and pacman cost parsers are tested with typed fixtures, not captured output. The Windows process-tree kill in review.go has no test. Mutation checks were made for the /review quick flag only.

### Security Posture

/review: raw analyser logs no longer land in the reviewed tree and gitleaks output is redacted; semgrep metrics disabled; the permission prompt sets Egress and names networked analysers. Dossier helpers lose find and view (web tools only). Findings files 0600. Dossier calls capped at two per user message. No change to the command gate, masking or permission flow.

### Deployment

**Prerequisites:**
- Windows 10/11 x64, or linux/amd64.
- Python 3 on PATH for the find tool's fallback and for /review.

```bash
# fetch
gh release download v0.1.142 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, .deb, .rpm, SHA256SUMS-v0.1.142.txt.
# verify
sha256sum -c SHA256SUMS-v0.1.142.txt --ignore-missing
# Expected: Each present file followed by OK. Windows exe: 8046e42a7e92990881716fd9fda97a8da29045ac8c362d6024ec41290f1f8987.
# install
.\gorilla-opencode.exe install
# Expected: Install path and shortcuts.
# verify_active
gorilla-opencode --version
# Expected: v0.1.142
# verify_active
go test ./internal/llm/tools -run 'Capitals|BothNotEither|TooLargeToDraw' -count=1
# Expected: ok (from a source checkout).
```

**Rollback:**
  1. Remove this version
     `gorilla-opencode uninstall   # or apt remove / dnf remove gorilla-opencode`
  2. Install v0.1.141 from its release
     `gh release download v0.1.141 -R gorillanobakaa-dot/Gorilla.Opencode`

### Known Issues

**[low]** A model expected in the endpoint picker is absent.
- Cause: Its last probe returned 404/410, or its id matches a non-chat family in chatTier.
- Remedy: /update re-probes within its budget; inspect model-probes.json in the cache directory.

**[low]** find view=tree returns folder counts where a tree is expected.
- Cause: The file list exceeds the tree limit (40 from the tool).
- Remedy: Call find with path set to a sub-folder.

**[low]** /review reports 'nothing in scope'.
- Cause: --diff against a ref with no changed files, or an empty path.
- Remedy: Expected. Review a path or another ref.

**[low]** A third /osint tool call in one turn is refused.
- Cause: reserveDossierCall ceiling.
- Remedy: Expected. Send a new message to start another run.

**[medium]** Cost meter reads zero on an endpoint that does bill.
- Cause: Endpoint models carry no price because the endpoint reports none.
- Remedy: None in this version.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Name search was case-sensitive | 📄 stated in input | A name search was case-sensitive |
| glob with type returned 120,765 files, now 15 | 📄 stated in input | returned 120,765 files. It now returns 15 |
| Tree view cut at 40 files silently | 📄 stated in input | drew the first 40 files of the walk and stopped without saying so |
| Picker 82 rows to 41 | 📄 stated in input | on the owner's machine it shows 41 rows |
| A 5xx is asked once more; use replaces a failed label | 📄 stated in input | A server error is now asked once more before it goes on record |
| quick ran security analysers and reported them skipped | 📄 stated in input | quick ran the security analysers and then told the model they had been skipped entirely |
| C and C++ reviews failed to decode | 📄 stated in input | Every C or C++ review ran to completion and then failed with "output could not be read" |
| convert resolved to the Windows disk converter | 📄 stated in input | the name convert finds Windows' own disk converter |
| No tool.research guard | 📄 stated in input | Neither command checked that the research tool was switched on |
| Dossier ran engineering lanes; now ten dossier lanes | 📄 stated in input | A dossier now runs its own ten lanes |
| Recovery failed for some providers' call ids | 📄 stated in input | runs made through Gemini, Claude or Antigravity could not be recovered from the session store |
| Binary hash and size | 📄 stated in input | sha256 8046e42a7e92990881716fd9fda97a8da29045ac8c362d6024ec41290f1f8987, 54,704,128 bytes |
| Suite result | 📄 stated in input | passes, 33 packages, 0 failures |
| Real-model run | 📄 stated in input | All seven gave valid results |
| Launch times | 📄 stated in input | ready to type after 1.56 seconds and 1.93 seconds |
| Paste probe not run | 📄 stated in input | Ctrl+V in the key box could not be re-proven on this file |
| Scope | 📄 stated in input | 48 files changed, 8026 insertions, 667 deletions since v0.1.141 |
| ripgrep accepts an override whitelist match before consulting the type matcher | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*
