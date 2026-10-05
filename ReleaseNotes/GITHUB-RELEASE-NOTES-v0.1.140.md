# Gorilla OpenCode 0.1.140 — Windows and Linux

**Opened in a big folder, the program read the name of every file in it before it would show its window: minutes of waiting, gigabytes of memory. It now starts in about a second. And Arch Linux has a package again.**

> **About the Linux files on this page.** The automatic checks that install them
> on real Linux run on GitHub's computers, and GitHub's build service was out of
> order on the evening this was published. The checks had not run when this page
> went up. They run when the service is back, and this note will be replaced by
> what they printed. The previous version's Linux files, built the same way,
> passed the same checks earlier the same day.

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

**Yes, on Windows.** If you have ever started the program and watched it sit
there doing nothing, this is very likely why, and this version fixes it.

**Yes, on Arch Linux, CachyOS or Manjaro.** This is the first version with a
package for you since 0.1.132.

**On other Linux: yes, with one thing to know.** The fix is the same. But read
the note at the top of this page: this version's Linux files had not yet been
through their install check when the page was published. If a failed download
would cost you data you cannot spare, wait until that note changes.

**If you only ever start the program inside a small project folder,** you will
not notice a difference. There is no harm in updating.

## Why this matters to you

Imagine walking into a library to ask one question. Before answering, the
librarian says: *one moment, I will first write down the title of every book in
this building.*

In a library with one shelf, you would not notice. In a real one, you would
leave before the list was finished.

That is what this program did every time it started. When you type the `@` sign
in a message, it offers you a list of files to choose from. To have that list
ready, it made it at the very beginning, before its window appeared, and it made
it by going through **every file in the folder it was opened in, and every
folder inside that, to the bottom**.

Open it in a small project and that is a few hundred names. Open it in your
home folder, the one that holds your documents, your downloads and years of
everything else, and this is what was measured on the owner's own computer:

```
version 0.1.139, opened in the home folder
  after 150 seconds:   still not ready
  memory used:         3,157 MB
  processor time:      125 seconds
```

The window showed three lines and then nothing. It looked broken. It was busy
writing down the title of every book.

Nobody had caught it, because every test of the program started it in a small,
empty folder.

**Now the librarian hands you the first shelf and asks what you are looking
for.** The list stops at a screenful. If you type part of a name, it searches
for that, and gives up after a second and a half instead of never. The same
folder, the same computer:

```
version 0.1.140, opened in the home folder
  first menu appears:  after 0.42 seconds
  ready to type:       0.95 seconds after the last key press
  memory used:         80 MB
```

**What it means for you, in one line:** the program starts in about a second
wherever you open it.

**What it costs:** in a folder with more than 60,000 files, a file buried deep
may not appear in the `@` list until you type more of its path.

**One more reason a window can freeze on Windows, which is not this program's
doing.** If you click inside the black window, Windows starts selecting text,
the word **Select** appears at the start of the title bar, and Windows pauses
the program until you finish. Press **Esc** once and it carries on.
---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why This Release Exists

This version exists because the owner opened the program in his home folder and nothing happened. The window showed three lines and then sat there for minutes, with the computer's fan running. He asked the obvious question: why does it take so long to start?

Here is why. When you type the @ sign in this program, it offers you a list of files to mention. To have that list ready, the program was making it at the very start, before its window appeared. And it was making it the slow way: it went through every file in the folder it was opened in, and every folder inside that, to the bottom, and wrote all the names down first.

Think of walking into a library to ask one question, and the librarian saying: one moment, I will first write out the title of every book in the building. In a small room that takes a second. In a home folder, which holds your documents, your downloads, your programs' private files and years of everything else, it took more than two minutes and three gigabytes of memory, and the window still was not ready.

Nobody had noticed, because every test of the program started it in a small empty folder.

Now the librarian hands you the first shelf and waits to hear what you are looking for. The list stops at a screenful, or after a second and a half, whichever comes first. In the same home folder the program is now ready in about a second.

Two other things are in this version. People who use Arch Linux were told to download a package that had not existed for seven versions, because the instructions for building it were broken; it is built again, on a real Arch computer. And a real conversation with an AI was held in the owner's own window, with him watching, to check that asking, refusing and allowing all work.

### What You Will Notice

**Starting the program in a large folder**
- Before: Opened in a home folder: still not ready after two and a half minutes, using 3,157 MB of memory and a whole processor core. The window showed its first lines and nothing else.
- After:  Opened in the same folder: the first menu appears in under half a second and the place where you type appears about a second after your last key press, using 80 MB.
- Affects: Anyone who starts the program in a folder with a great many files: a home folder, a Documents folder, a whole disk

**The list of files offered when you type @**
- Before: Every file under the folder, however many.
- After:  A screenful of 200 to begin with. Type a few letters of the name and it searches for that. A search looks at up to 60,000 names and stops after a second and a half.
- Affects: Everyone

**The package for Arch Linux**
- Before: None for seven versions. The build instructions pointed at a file that was no longer there, so they could not be followed by anyone.
- After:  Built on a real Arch computer, installed there, started, removed again, and placed on the release page.
- Affects: Anyone on Arch Linux, CachyOS, Manjaro or similar

**A window that seems frozen for no reason, on Windows**
- Before: Not explained anywhere.
- After:  Explained below: clicking inside the window can pause the program. This is Windows, not a fault in the program, and one key press ends it.
- Affects: Everyone on Windows

### Deliberately Not Done

- **Checking this version's Linux files on Linux before publishing** — The checks run on GitHub's computers, and GitHub's build service was out of order on the evening this was released. It cancelled the jobs before they began. They run when the service is back, and this page will be changed to show what they printed.
- **A conversation with an AI on Linux** — The check for it is written: it runs a small free AI on the Linux computer itself, so no key is needed. It could not run for the same reason. It is not claimed here as done.
- **Searching every file when you type a name after @** — In a folder with more than 60,000 files, a file far down may not be offered. Type more of its path, or open the program in the project's own folder. That is the price of starting in a second instead of in minutes.
- **A macOS version** — Not built, by the owner's decision.

### Privacy & Security

No telemetry was added and none exists. This version makes no new kind of request. The new Linux conversation check does not contact any company's AI: the AI it talks to runs on the same computer as the check, and no key is involved.

### How to Install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora, Arch or similar Linux computer.
- Enough mobile data for a 55 MB file on Windows or a file of about 24 MB on Linux.
- Gorilla OpenCode closed, if it is open.

**Step 1:** Close Gorilla OpenCode if it is open.
✓ No window titled Gorilla OpenCode is open.

**Step 2:** Open the release page in your web browser and download the one file for your computer. Windows: gorilla-opencode.exe. Debian, Ubuntu or Mint: gorilla-opencode_0.1.140_amd64.deb. Fedora, openSUSE or Rocky: gorilla-opencode-0.1.140-1.x86_64.rpm. Arch, CachyOS or Manjaro: gorilla-opencode-0.1.140-1-x86_64.pkg.tar.zst. Also download SHA256SUMS-v0.1.140.txt.
✓ Two files are in your Downloads folder.

**Step 3:** Check the file is the one that was published. On Windows, press the Windows key, type PowerShell, press Enter. A window with a blinking cursor opens. Type cd Downloads and press Enter. Then type the command below and press Enter.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ It prints 4fd3a75c6332f45ca415b27a087aa1481207eddbcb825820c87df409b13eaaca. On Linux, run sha256sum -c SHA256SUMS-v0.1.140.txt --ignore-missing in a terminal in your Downloads folder; it prints the file name followed by OK.

**Step 4:** Windows: install it. Type the command below and press Enter.
```
.\gorilla-opencode.exe install
```
✓ It reports where it copied itself and the shortcuts it made.

**Step 5:** Debian, Ubuntu or Mint instead: type the command below and press Enter. It asks for your password because installing a program changes the system.
```
sudo apt install ./gorilla-opencode_0.1.140_amd64.deb
```
✓ A line says Setting up gorilla-opencode (0.1.140), with no line beginning with E:.

**Step 6:** Fedora, openSUSE or Rocky instead: type the command below and press Enter.
```
sudo dnf install ./gorilla-opencode-0.1.140-1.x86_64.rpm
```
✓ The last line says Complete!

**Step 7:** Arch, CachyOS or Manjaro instead: type the command below and press Enter.
```
sudo pacman -U ./gorilla-opencode-0.1.140-1-x86_64.pkg.tar.zst
```
✓ It asks to proceed with installation; after you answer y, it ends without a line beginning with error:.

**Step 8:** Confirm the version. Type the command below and press Enter.
```
gorilla-opencode --version
```
✓ It prints v0.1.140.

**To go back:** Windows: type gorilla-opencode uninstall and press Enter, then install the gorilla-opencode.exe from version 0.1.139 the same way as in step 4. Debian: sudo apt remove gorilla-opencode. Fedora: sudo dnf remove gorilla-opencode. Arch: sudo pacman -R gorilla-opencode. Your conversations, settings and keys are stored separately and none of these steps touches them.

### If Something Goes Wrong

**On Windows the window stops responding and the word Select has appeared at the start of its title bar.**
You clicked inside the window. In the classic Windows console that starts selecting text, and Windows pauses the program until you finish.
What to do: Click on the window and press the Esc key once. The word Select disappears and the program carries on from where it was.
Status: behaviour of Windows, not changed by this version

**After typing @ and part of a name, a file you know exists is not in the list.**
The folder holds more than 60,000 files and the search stopped before reaching it.
What to do: Type more of the path, for example the folder name and then the file name. Or close the program and start it inside the project's own folder.
Status: expected behaviour, new in 0.1.140

**An older version started slowly or not at all in a big folder.**
It listed every file under the folder before showing its window.
What to do: Install this version.
Status: fixed in 0.1.140

**On Linux the package will not install, or the program will not start.**
This version's Linux files had not been installed on Linux when this page was first published, because the service that does it was out of order.
What to do: Download the plain file gorilla-opencode-v0.1.140-linux-amd64 from the same page, make it runnable with chmod +x, and start it directly. Then report what the package printed and which Linux you use.
Status: being checked: see the top of this page for the result

### Common Questions

**Q: Was the program doing something with all my files?**
A: It read their names, to have a list ready for the @ sign. It did not open them, change them or send them anywhere. The names stayed on your computer. It now reads far fewer of them.

**Q: I always start the program inside a small project folder. Will I notice anything?**
A: Probably not. In a small folder the old way was already quick.

**Q: Why is the Arch package made after the release and not with it?**
A: Its build instructions include a fingerprint of the published source code, so that nobody can swap the source for something else. That fingerprint cannot be known until the source is published. So the package is built a few minutes later and then added to the same page.

**Q: Should I wait until the Linux checks have run?**
A: If a failed download would cost you data you cannot spare, yes. Look at the top of this page: it will say when they have passed. The previous version's Linux files passed the same checks earlier the same day.

### Bottom Line

On Windows, update: if you ever start the program in a big folder, this is the difference between a second and several minutes. On Arch Linux, this is the first package you can install since version 0.1.132. On other Linux, the fix is the same and the files are built the same way as the previous version's, which were installed and started on real Linux earlier the same day; but this version's own files had not yet been through that check when the page was published, because GitHub's build service was out of order. That is stated here, not hidden, and the page will be changed when the checks have run. The one thing given up is completeness of the @ list in enormous folders: it now stops looking after 60,000 names.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| The old version was not ready after 150 seconds and used 3,157 MB | 📄 stated in input | not ready after 150 seconds, 3,157 MB of memory, 125 seconds of processor time |
| The new version is ready in about a second in the same folder | 📄 stated in input | the provider menu appears after 0.42 seconds and the input line 0.95 seconds after the last key press; 80 MB of memory |
| The list is capped at 200 names, 60,000 looked at, 1.5 seconds | 📄 stated in input | With something typed it looks at no more than 60,000 names |
| No Arch package for seven versions | 📄 stated in input | No Arch package was published from v0.1.133 to v0.1.139 |
| The Arch package was built and installed on a real Arch system | 📄 stated in input | Run for v0.1.139, every step passed |
| Clicking in the console pauses the program | 📄 stated in input | Windows pauses the program until a key is pressed |
| A conversation was held in the owner's window | 📄 stated in input | the AI ran it and reported gorilla-live-check |
| The Linux checks could not run because of an outage | 📄 stated in input | The job was not acquired by Runner of type hosted even after multiple attempts |
| The Linux conversation check uses no key | 📄 stated in input | No key is involved |
| Seven real-AI scenarios passed on the published Windows file | 📄 stated in input | All seven gave valid results |
| The program only read file names and sent nothing | 🤖 model inference | *(none — model judgment)* |


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

Every block below is real output, copied from a run on the maintainer's computer or from GitHub's. Nothing here is an illustration.

## What it was doing while the window sat empty

These are the two helper programs the old version had started, seen while its window showed nothing. The first lists every file under the folder. The second was given an empty filter, which lets every name through.

```
rg.exe  --files -L --null C:\Users\gorilla1
fzf.exe --filter "" --read0 --print0
```

## A real conversation, in a window on the desktop

The owner opened the program himself and left it on screen. A conversation was then held in that window with a real AI (NVIDIA's Nemotron 3 Super 120B), without touching his keyboard, and the screen was read back.

Asked to run a command and report it:

```
 The command echo gorilla-live-check printed exactly:
 gorilla-live-check
 Nemotron 3 Super 120B (2s)
```

Asked to create a file. The program stopped and asked first:

```
Permission Required
Tool: write
Path: C:\Users\gorilla1
File: C:\Users\gorilla1\gorilla-live-check.txt
        Allow (a)    Allow for session (s)    Deny (d)
```

**Deny** was pressed. No file was created. The program's own record of the conversation:

```
CALL    write {"file_path":"C:\\Users\\gorilla1\\gorilla-live-check.txt","content":"ok"}
RESULT  ERR   Permission denied
```

Asked again, the question appeared again. This time the owner pressed **Allow** himself, and the file was written.

## The Arch package, built and installed on Arch Linux

The steps of the automatic job, each with its result. This run was for the previous version; the same job builds this version's package once the release exists.

```
success  the recipe is for this release
success  build as an ordinary user (makepkg refuses to run as root)
success  what is in the package
success  install it
success  the installed program starts and reports its version
success  the model doctor and the search tool run
success  it uninstalls cleanly
success  attach it to the release, with its checksum
```

## How it was tested

**The file you download is the file that was tested.** The Windows program was built once, tested, and published without being rebuilt.

**Starting up.** Timed in an empty folder and in a home folder, with the numbers shown at the top of this page.

**Against a real AI, with nobody watching.** The published Windows file was started from a script seven times against NVIDIA's `nemotron-3-super-120b-a12b`. All seven runs gave a usable result.

**The program's own tests.** The complete set passes on Windows. On Linux it passed for the change that contains this fix, before GitHub's service went out of order.

**What was not tested, and why.** This version's Linux files have not been installed on Linux, and the new check that holds a conversation with an AI on Linux has not run. GitHub's build service cancelled those jobs three times that evening with this message:

```
The job was not acquired by Runner of type hosted even after multiple attempts
```

They run when the service returns. The note at the top of this page will then be replaced with their result.

## No new pictures, and why

Everything in this release is text the program prints, quoted above as printed.
No new screenshot was taken. The two below are from earlier releases and show
screens this release did not change, pinned to this version.

**The normal window**, where every action is listed as it happens. This is the
permission question quoted above.

[![Gorilla OpenCode showing a Permission Required dialog for the patch port tool, naming the folder it will modify and the patch series it will apply, with the three choices Allow, Allow for session and Deny, proving the program asks before a tool changes files](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.140/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.140/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)

**The command list**, all 31 commands on one screen.

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does and showing 37 of 37 lines, 31 commands, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.140/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.140/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Root cause: `dialog.NewCompletionDialogCmp` calls `GetChildEntries("")` while the chat page is constructed, and `filesAndFoldersContextGroup.getFiles` ran `rg --files -L --null <roots>` piped into `fzf --filter ""`, buffering the complete output before returning. An empty filter passes every path, so start-up cost was proportional to the size of the tree under the working directory. Measured on v0.1.139 in a home directory: not ready after 150 s, 3,157 MB RSS, 125 s CPU. Fix: `getFiles` now streams names from ripgrep (or `filepath.WalkDir` when ripgrep is absent) through `scanFileNames`, which stops on the first of: enough results (200 for an empty query), 60,000 names scanned, or a 1.5 s budget, and kills the lister. fzf is no longer used there. Separately, `packaging/PKGBUILD` installed `packaging/setup-searxng.sh` from the release tarball, which has not contained it since `*.sh` was git-ignored; it is now a local second source with its own checksum. Scope: 3 files modified, 4 added, 5 new tests, 2 new workflows.

### Known Alternatives Considered

Making the initial listing asynchronous was not chosen: an unbounded walk of a home directory would still consume gigabytes in the background. Keeping fzf for non-empty queries was rejected in the code comment: "fzf is no longer used here: it cannot be told to stop early, and the fuzzy match it did is done in Go on the names already read." Shipping the prebuilt Linux binary in the Arch package was not chosen; the recipe still builds from the tagged tarball with a verified checksum. Other alternatives: Not available in the source material.

### Architecture Impact

None outside `internal/completions`. `getFiles` has a hard upper bound on time and memory. Completion results for queries are now `fuzzy.Find` over at most 60,000 names, where it was fzf over the whole tree, so ranking differs slightly and files beyond the scan limit are not offered. CI gains `arch-package.yml` (dispatch, after the tarball checksum is committed) and `linux-conversation.yml` (dispatch and `release: published`).

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.140" -o gorilla-opencode.exe .`. Linux: same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Windows binary sha256 4fd3a75c6332f45ca415b27a087aa1481207eddbcb825820c87df409b13eaaca, built from the committed tree, tested, published unchanged. .deb and .rpm by nfpm; .pkg.tar.zst by makepkg in an archlinux container.
```

### Resource Deltas

gorilla-opencode.exe: 54,543,872 bytes (0.1.139) -> 54,535,680 bytes (-8,192). Start-up in C:\Users\<home>: not ready at 150 s / 3,157 MB / 125 s CPU -> provider menu at 0.42 s, input line 0.95 s after the last key, 80 MB. Start-up in an empty directory: unchanged (menu at 0.47 s).

### Code Changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `internal/completions/files-folders.go` | modified | `getFiles` ran rg to completion into fzf and parsed all NUL-separated output; fallbacks used an unbounded doublestar glob. `processNullTerminatedOutput` held the whole listing. | `scanFileNames(deadline, enough)` reads rg's stdout with a bufio.Reader, calls `take` per name, and on stop kills the process and waits for it; a timer kills it at the deadline regardless. Without rg it uses `filepath.WalkDir` with `SkipDir` for hidden directories and `SkipAll` on stop. Constants `completionShown = 200`, `completionScan = 60000`; `completionBudget = 1500 ms` is a variable for tests. |
| `packaging/PKGBUILD` | modified | One source (the tag tarball); `package()` installed `packaging/setup-searxng.sh` from the tarball, which does not contain it; makepkg's default produced an empty -debug split package. | `source=(tarball "setup-searxng.sh")` with two sha256sums; installs `${srcdir}/setup-searxng.sh`; `options=('!debug' '!strip')`. |
| `packaging/setup-searxng.sh, .gitignore` | added | N/A - the script existed only in an untracked directory and inside the binary packages. | Tracked, with a `.gitignore` exception. It is a product file shipped in every .deb and .rpm. |
| `internal/version/pkgbuild_test.go` | modified | Parsed a single-line `sha256sums=(...)` holding one sum. | Requires one 64-hex checksum per source, none SKIP; `TestPKGBUILDLocalSourceMatchesItsChecksum` hashes the LF form of setup-searxng.sh and requires it in the recipe. |
| `.github/workflows/arch-package.yml` | added | N/A - new file. | In `archlinux:latest`: checks pkgver against the tag, `makepkg --syncdeps` as a non-root user, `pacman -Qip/-Qlp`, `pacman -U`, version equals tag, `models doctor`, `pfind --help`, `pacman -R`, then uploads the package and an updated SHA256SUMS to the release. |
| `.github/workflows/linux-conversation.yml` | added | N/A - new file. | Starts `ollama/ollama` with `OLLAMA_CONTEXT_LENGTH=16384`, pulls `qwen2.5:3b`, writes a config with an Ollama local endpoint, runs `gorilla-opencode -p` up to three times and requires a receipt line showing a bash call that ended ok. Builds from source on dispatch, installs the release .deb when given a tag. |

### Subsystem Changes

**TUI:** @-completion: bounded listing; start-up no longer blocks on a full tree walk.

**OTHER:** Arch packaging restored. Two CI workflows added.

**STORAGE:** No change.

**NETWORK:** No change.

**AUTH:** No change.

### Test Coverage

- **Added:** internal/completions/bounded_test.go (4): an empty query over 3,000 files returns at most 200 names within budget; a typed query finds a deep file and is capped; a 30 ms budget returns promptly; hidden paths are excluded. internal/version: TestPKGBUILDLocalSourceMatchesItsChecksum.
- **Removed:** None. TestPKGBUILDChecksumIsNotSKIP rewritten for a multi-source array.
- **Notes:** Complete suite passes on Windows (34 packages). The `ci` workflow passed on ubuntu for commits 12f6645 and f75d403, which contain the fix. arch-package passed every step for v0.1.139 and attached gorilla-opencode-0.1.139-1-x86_64.pkg.tar.zst (24,547,406 bytes). Windows binary against nvidia/nemotron-3-super-120b-a12b: 7 of 7 scenarios valid; portal paste re-proven; start-up timed in an empty and a home directory. Interactive conversation held in a user-opened console by AttachConsole and WriteConsoleInput: bash tool call, permission Deny (no file), re-prompt on retry. NOT RUN: linux-install and linux-conversation for v0.1.140, and the last ci run, because GitHub Actions cancelled the jobs during an incident ("The job was not acquired by Runner of type hosted even after multiple attempts").

### Security Posture

No change to permission, masking or command gates. The completion lister reads names only; hidden paths remain excluded by `fileutil.SkipHidden`. `arch-package.yml` has `contents: write` to upload the package and is dispatch-only. `linux-conversation.yml` has `contents: read` and uses no secret.

### Deployment

**Prerequisites:**
- Windows 10/11 x64, or linux/amd64.
- No running gorilla-opencode process.

```bash
# fetch
gh release download v0.1.140 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, .deb, .rpm, SHA256SUMS-v0.1.140.txt; the .pkg.tar.zst once arch-package has run.
# verify
sha256sum -c SHA256SUMS-v0.1.140.txt --ignore-missing
# Expected: Each present file followed by OK. Windows exe: 4fd3a75c6332f45ca415b27a087aa1481207eddbcb825820c87df409b13eaaca.
# install
.\gorilla-opencode.exe install
# Expected: Install path and shortcuts. Linux: apt install ./...deb, dnf install ./...rpm, or pacman -U ./...pkg.tar.zst.
# verify_active
gorilla-opencode --version
# Expected: v0.1.140
# verify_active
python scripts/model-tests/startup_time.py <exe> <large-folder> 60
# Expected: ready_after_s of a few seconds and memory under 150 MB (the helper script is not in the public tree).
# verify_active
gh workflow run arch-package.yml -f tag=v0.1.140
# Expected: Every step succeeds and the package appears on the release.
```

**Rollback:**
  1. Remove this version
     `gorilla-opencode uninstall   # or apt remove / dnf remove / pacman -R gorilla-opencode`
  2. Install v0.1.139 from its release
     `gh release download v0.1.139 -R gorillanobakaa-dot/Gorilla.Opencode`

### Known Issues

**[low]** @-completion does not offer a file in a very large tree.
- Cause: The scan stopped at 60,000 names or 1.5 s.
- Remedy: Type more of the path, or start in the project root. `completionScan` and `completionBudget` are the limits.

**[low]** Console window frozen with a title starting "Select".
- Cause: conhost QuickEdit selection blocks the process's console writes.
- Remedy: Press Esc in the window.

**[medium]** makepkg fails with a checksum mismatch.
- Cause: The recipe on the branch predates the commit that records the tag tarball's sha256.
- Remedy: Use the recipe from main after the "real sha256" commit for that version.

**[medium]** linux-install or linux-conversation shows cancelled with no steps.
- Cause: No hosted runner was assigned (GitHub Actions incident).
- Remedy: Re-run the workflow when Actions is healthy.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Start-up cost before the fix | 📄 stated in input | not ready after 150 seconds, 3,157 MB of memory, 125 seconds of processor time |
| The command that was run | 📄 stated in input | rg --files -L --null on the folder, piped into fzf --filter "" |
| Limits of the new listing | 📄 stated in input | Either way it stops after 1.5 seconds, and the lister is stopped, not left running |
| Start-up after the fix | 📄 stated in input | the provider menu appears after 0.42 seconds and the input line 0.95 seconds after the last key press; 80 MB of memory |
| PKGBUILD could not build since the script left the source | 📄 stated in input | that file has not been in the source since shell scripts were excluded from the repository |
| arch-package passed for v0.1.139 | 📄 stated in input | gorilla-opencode-0.1.139-1-x86_64.pkg.tar.zst, 24,547,406 bytes, was attached to that release |
| Binary hash and size | 📄 stated in input | sha256 4fd3a75c6332f45ca415b27a087aa1481207eddbcb825820c87df409b13eaaca, 54,535,680 bytes |
| Real-model run | 📄 stated in input | All seven gave valid results |
| Linux jobs not run because of an Actions incident | 📄 stated in input | The job was not acquired by Runner of type hosted even after multiple attempts |
| ci passed on Linux for the fix commit | 📄 stated in input | The Linux tests of the source passed on GitHub for the commit that contains the fix, before the outage |
| Ranking differs slightly from fzf | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*
