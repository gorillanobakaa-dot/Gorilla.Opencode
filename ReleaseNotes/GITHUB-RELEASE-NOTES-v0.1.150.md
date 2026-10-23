# Gorilla OpenCode 0.1.150: Mac, Windows and Linux

**The first version you can download and install on an old Mac (macOS 10.13 High Sierra and later, Intel). On a Mac's Terminal the screen no longer fills with copies of the prompt, the replies are readable instead of dim grey, and help opens with a key every keyboard has. The permission questions ask far less, and when they ask, they show what is being asked.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if you have a Mac.** This is the first version with a Mac download on this
page: one file, unpacked and installed with a double-click, no password, nothing
installed for the whole computer.

**Yes, everyone else too.** The permission questions stop asking about harmless
things (looking at a file, asking a program for its version), and the question
box now shows the command or the address it is asking about. On every system the
input line no longer reaches the last column of the window, which is what broke
the screen on the Mac.

## Why this matters to you

Think of the screen as a whiteboard the program wipes and redraws many times a
second. On a Mac's Terminal, writing in the very last column of the board moves
the pen down a line. The program used to write right up to the edge, so every
redraw started one line lower than it thought, and the old drawing stayed behind:
two, three, four prompt lines and cursors, and the name at the top split as
"G" on one line and "orilla OpenCode" on the next. Now it never writes in the
last column.

The grey text was a colour code the old Terminal did not know. It read part of
it as "draw this faintly". The program now sends colours in the form the
terminal says it understands.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why this release exists

On 22 and 23 October the program was run on a 2011 MacBook Air with macOS 10.13 and a small screen (1366 by 768), the kind of computer this project is for. The owner photographed every fault he saw: 30 pictures. The faults came from two causes. The program wrote right up to the last column of the window, which the Mac's Terminal treats as "go to the next line", so every redraw of the screen landed in the wrong place. And the program sent colours in a form the old Terminal does not know, so it showed the answers in a faint grey. Both are fixed. The permission questions were reworked at the same time, from a test on the same Mac where installing one program raised 33 questions, none of them about anything risky.

### What you will notice

**The input line on a Mac**
- Before: two to four `>` lines stacked above the bottom bar, with two cursors, and the bottom bar sometimes missing.
- After: one `>` line and one cursor. Proved on the real Mac (picture below).
- Affects: Mac users; on Linux and Windows the last column is now left empty too.

**The name at the top**
- Before: "orilla OpenCode", with the "G" alone at the right edge of the line above.
- After: "Gorilla OpenCode", whole, from the first column.
- Affects: Mac users.

**Colours of the answers**
- Before: on the Mac the AI's answers were a faint grey, hard to read, and some lines in italics looked different from Linux.
- After: the same colours as elsewhere, in the 256 colours the Mac's Terminal has.
- Affects: anyone whose terminal has 256 colours or fewer.

**Changes to files (the side-by-side view)**
- Before: on the Mac, grey blocks of different widths behind the lines.
- After: on a terminal with 256 colours or fewer there are no background blocks; added and removed lines are marked by a green `+` or red `-` and their line number.
- Affects: anyone whose terminal has 256 colours or fewer.

**Opening help**
- Before: the bottom bar said "ctrl+? help", a key the Mac's Terminal cannot send.
- After: it says "? help". Press `?` when the input line is empty. F1 also works, and ctrl+? still works where the terminal sends it.
- Affects: everyone.

**Long lines**
- Before: a long question you typed was cut in the middle of words ("su" / "do"); the /usage text broke at 80 columns with half the window empty; one notice ran off the right edge.
- After: they break between words, at the width of your window.
- Affects: everyone.

**Permission questions**
- Before: one install on the Mac raised 33 questions, 13 of them with an empty Command field, so you were asked to approve something you could not see.
- After: looking at things (reading a file, listing a folder, asking a program its version) never asks. With "auto-approve" on, the program stops only for things that cannot be undone, for administrator commands, for writing outside your project, and, after reading something from the internet, for sending to a new place. The question shows the full command or address first. "Deny" is an answer the AI reads and works around.
- Affects: everyone.

**A Mac download**
- Before: there was no Mac file on the release page.
- After: one zip file for Intel Macs with macOS 10.13 or later.
- Affects: Mac users.

### Deliberately not done

- The program is not signed by Apple. Apple charges a yearly fee for that, and the project is for people without a card. The first time, the Mac asks you to confirm you want to open it (steps below).
- There is no build for Macs with Apple chips (M1 and later) on this page yet. The Intel build may run there through Apple's translator; that has not been tried.
- The shadow under the other dialog boxes (not the permission question) can still hide one line under the box in the scrolling view. It is a separate change.

### Privacy and security

Nothing new is sent anywhere. The Mac installer works only inside your own home folder: it never asks for a password and changes nothing for other users. It moves the version you had to the Trash (where "Put Back" restores it), and the Undo file next to it brings that version back. It also turns off Terminal's habit of reopening old windows, after writing down how it was, and Undo restores that too.

The permission rework relaxes some questions. Each relaxed rule, the fear it was there for, and what still covers that fear, is written in `docs/PERMISSIONS.md`.

### How to install

**On a Mac (macOS 10.13 or later, Intel):**

1. On this page, under Assets, click `gorilla-opencode-0.1.150-mac-10.13-x86_64.zip`. Safari unpacks it into a folder in Downloads.
2. Open that folder. Right-click (or Control-click) `Install Gorilla OpenCode.command` and choose **Open**, then **Open** again. A double-click alone shows "unidentified developer" the first time, because the program is not signed by Apple.
3. A Terminal window says what it does at each step and stops at the first problem. When it says DONE, double-click "Gorilla OpenCode" on your Desktop.

To go back: right-click `Undo Gorilla OpenCode.command` in the same folder and choose **Open**.

**On Windows:** download `gorilla-opencode.exe`, then in a terminal in that folder: `.\gorilla-opencode.exe install`.

**On Debian, Ubuntu or Mint:** download `gorilla-opencode_0.1.150_amd64.deb` and install it with `sudo apt install ./gorilla-opencode_0.1.150_amd64.deb` (apt, not dpkg, so the web search helper comes with it).

**On Fedora, openSUSE or Rocky:** `gorilla-opencode-0.1.150-1.x86_64.rpm`. **On Arch or CachyOS:** `gorilla-opencode-0.1.150-1-x86_64.pkg.tar.zst`. **Anywhere else on Linux:** `gorilla-opencode-v0.1.150-linux-amd64`, a single file you make executable and run.

To check a download is the file that was published, compare it with `SHA256SUMS-v0.1.150.txt` (on a Mac: `shasum -a 256 FILE`; on Linux: `sha256sum -c SHA256SUMS-v0.1.150.txt --ignore-missing`).

### If something goes wrong

- **"cannot be opened because it is from an unidentified developer"** on a Mac: use right-click, **Open**, as in step 2.
- **The installer says STOPPED**: it names what was wrong and has not replaced your current version. The most common one is a download that did not finish; download the zip again.
- **The screen still shows several `>` lines on a Mac**: check the version on the top line says v0.1.150. If an older one starts, the Desktop icon points at the old app; run the installer again.
- **Help does not open with `?`**: the input line must be empty. F1 works at any time (on a Mac laptop, hold fn).

### Common questions

**Why is the Mac file a zip?** The installer needs the program and its checksum in the same folder, and needs to stay "runnable". A single zip keeps all of that; loose files lose it in the browser.

**Do I need Homebrew, Xcode or a password?** No. Everything goes into your own home folder.

**Will this work on macOS 10.12 or older?** No. 10.13 is the oldest version it was built and tested for.

### Bottom line

If you have an old Intel Mac, this is the first version you can install from this page, and the first one whose screen stays clean in the Mac's own Terminal. On every system, permission questions now ask about real risks and show you what they are asking about.

---

## Measured on the published files

Built on 2026-10-23 on Debian (Go 1.24) from main after PR #4 and PR #5 were merged. The Mac program is built with scripts/build-mac-zip.sh, the Windows one cross-compiled on the same machine.

| File | Bytes | SHA-256 |
|---|---|---|
| `gorilla-opencode.exe` | 55,019,008 | `26ac367c35b58c2c90a0fe5ded99c911b5c91dfb5b357dd56e70ecea57c55fbe` |
| `gorilla-opencode-v0.1.150-linux-amd64` | 53,715,236 | `064d5805c71e797eb1823a951ce5d8090440b36d711ceaad138e273b7d3dc2e8` |
| `gorilla-opencode_0.1.150_amd64.deb` | 20,550,624 | `27c0469501e54f0915eb117dab816e38e22c318d055201c9cc56baa7f9da604c` |
| `gorilla-opencode-0.1.150-1.x86_64.rpm` | 25,101,254 | `3b752a6defa5272ac95e4d20dfa8c8c8249245d91b3e1a98b0312801958407f0` |
| `gorilla-opencode-0.1.150-1-x86_64.pkg.tar.zst` | 21,637,374 | `d3aaf38b33345f5830a8ea6db49aca24d205e97f340f84be7f5e53a27635ecf9` |
| `gorilla-opencode-0.1.150-mac-10.13-x86_64.zip` | 20,782,879 | `ffac63e5283c284815de1b4e35105b6501d847a3d4eeedca199d82cc0bfeb603` |

## The pictures: the fixed layout on the real MacBook Air

Taken on 2026-10-23 on the MacBook Air (macOS 10.13.6, Terminal.app, 169x37) with a build of the layout branch (f8acf24) before it was merged: same layout code as this release, before the colour, help-key and diff fixes, which is why the answer is still grey and the hint still says ctrl+?.

### One input line, one cursor, the name whole

[![Gorilla OpenCode on the MacBook Air in Terminal.app at 169x37: the banner reads Gorilla OpenCode whole from the first column, there is one input line with one cursor above the bottom bar, and the bottom bar is fully on screen](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.150/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-23-025732-layout-f8acf24.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.150/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-23-025732-layout-f8acf24.png)

### A question and its answer, the long notice wrapped inside the window

[![Gorilla OpenCode on the MacBook Air after the question Say hello in one short sentence: the no thinking to show notice wraps onto a second line inside the window instead of running off the edge, the answer follows, and the input area is one line with one cursor](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.150/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-23-025859-layout-f8acf24-answer.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.150/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-23-025859-layout-f8acf24-answer.png)

### The permission question shows the address it asks about

[![The permission dialog on the MacBook Air: the full URL first, a plain line saying what the step does, the reason it asks, what Allow for session covers, inside an aligned ASCII border with the rows around it intact](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.150/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-23-perms-dialog-1.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.150/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-23-perms-dialog-1.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Defects 1-14 of the MacBook Air run (macOS 10.13.6 Terminal.app, TERM=xterm-256color, no COLORTERM, 167x39 and 169x33) traced to two causes. (1) Frame lines padded to exactly the terminal width put the cursor in the last column after every frame; bubbletea v1.3.5's inline renderer then repositions with CursorUp(n-1) and paints without a CR, so redraws landed a row or column off. (2) glamour (default TrueColor), chroma (`terminal16m`) and hand-written diff SGR emitted `38;2;R;G;B`; Terminal.app 10.13 reads the `2` as SGR 2 (faint). PR #5 reworks the permission rules (docs/PERMISSIONS.md). A Mac release asset is built for the first time.

### Known alternatives considered

- Fixing width per component (status bar, divider, editor) instead of once at `WindowSizeMsg`: rejected, the same reasoning as `clampToWidth` (one choke point).
- A 256-colour palette tuned for Terminal.app: not needed; the hex colours degrade correctly once the bytes follow the detected profile.
- Loose Mac release assets (tar.gz, sums, two .command files): rejected, GitHub renames files with spaces and a browser download loses the executable bit.

### Architecture impact

`appModel.Update(tea.WindowSizeMsg)` now hands every component `usableWidth(cols) = cols-1`. The frame tracker still receives the physical width for rewrap arithmetic. All `tea.Println` calls in `internal/tui` go through `printAbove` (leading `\r`), like `printLine` in `components/chat`.

### Toolchain

Go 1.24 (Debian). Mac binary: `scripts/build-darwin-legacy.sh` (patched GOROOT copy for 10.13's missing `SecTrustEvaluateWithError`), wrapped by the new `scripts/build-mac-zip.sh`. Windows exe cross-compiled on Linux with the committed `rsrc_windows_amd64.syso`.

### Resource deltas

Mac app icon `packaging/macos/gorilla.icns`: 188,011 bytes, four PNG entries (16, 32, 128, 256), written by hand because Pillow's ICNS writer always upscales to 1024 (3,061,070 bytes, measured). Other sizes are in "Measured on the published file".

### Code changes

- `internal/tui/tui.go`: `usableWidth`, `printAbove`; `ToggleHelpMsg` handling; `keys.Help` gains `f1`.
- `internal/tui/session_banner.go`: banner through `printAbove`.
- `internal/tui/components/chat/message.go`: user questions `ansi.Wrap` at words before `Hardwrap`.
- `internal/tui/components/chat/printer.go`: reasoning-off notice wrapped with `wrapReasoning(width)`.
- `internal/tui/components/chat/editor.go`: `?` as first keystroke on an empty prompt sends `ToggleHelpMsg`.
- `internal/tui/components/core/status.go`: hint `? help`.
- `internal/tui/quota_panel.go`: 80-column cap on the bar only.
- `internal/tui/styles/markdown.go`: `glamour.WithColorProfile(lipgloss.ColorProfile())`.
- `internal/diff/diff.go`: `terminalFormatter()`, `sgr()`, `rowBackgrounds()`/`withRowBg()` (no backgrounds below TrueColor), `SyntaxHighlight` treats NoColor/nil as no token background.
- PR #5: `internal/llm/tools/commandclass*.go`, `internal/permission/`, `internal/tui/components/dialog/permission.go`, `placeOverlayShadow`/`placePermissionDialog` in `tui.go`; see docs/PERMISSIONS.md.
- `scripts/build-mac-zip.sh`, `packaging/macos/gorilla.icns`: new.

### Subsystem changes

TUI inline renderer contract: no frame line reaches column `width`. Colour output: every emitter follows `lipgloss.ColorProfile()`. Permissions: read-only class never asks; auto-approve stops for irreversible, hard stops, out-of-root writes, and post-taint sends to a new place.

### Test coverage

- `internal/tui/inline/macos_air_test.go`: the real app model through bubbletea into the terminal emulator (now counting last-column writes) at 167x39 and 169x33. On v0.1.149 code: 12 last-column writes, 6 prompt rows, status bar 0 times on screen, banner at column 41. All four tests fail there; the banner test also fails with only the `\r` removed.
- `chat/question_wrap_test.go`, `TestQuotaPanelTextUsesTheWholeWidth`, `TestNoticeFitsTheWindow`, `styles/color_profile_test.go`, `diff/color_profile_test.go` (3 tests), `help_key_test.go` (tui, chat): each fails on the previous code or under a mutation removing the fix.
- PR #5: per-row command-class tests, both Air runs as regressions, 13 mutations caught.
- Full `go test ./...` on the merged tree: see Known issues.

### Security posture

The permission rework relaxes read-only commands and some auto-approve cases; each relaxation and its compensating control is in docs/PERMISSIONS.md. The Mac installer runs as the user, writes only under `$HOME`, verifies SHA-256 before replacing anything, test-runs the new binary before install, and quarantines (Trash) rather than deletes.

### Deployment

```sh
# fetch
gh release download v0.1.150 --repo gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, .deb, .rpm, .pkg.tar.zst, Mac zip, SHA256SUMS-v0.1.150.txt
# verify
sha256sum -c SHA256SUMS-v0.1.150.txt
# Expected: each downloaded file: OK
# install (Mac)
unzip gorilla-opencode-0.1.150-mac-10.13-x86_64.zip && cd gorilla-opencode-0.1.150-mac && ./"Install Gorilla OpenCode.command"
# Expected: DONE. Gorilla OpenCode 0.1.150 is installed.
# verify_active
~/.local/bin/gorilla-opencode --version
# Expected: v0.1.150
```

### Known issues

- Not signed or notarised by Apple; first open needs right-click, Open.
- No arm64 Mac build.
- Overlay drop shadow on dialogs other than the permission dialog can still cut one row in inline mode.
- `TestPfindSemanticSendsNothingAndDownloadsNothing` needs chromadb (absent on the build machine); `TestMatchingIsFast` is a timing test that fails under load (also on v0.1.149's tree).
