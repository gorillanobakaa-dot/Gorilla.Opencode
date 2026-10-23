# Gorilla OpenCode 0.1.151: Mac, Windows and Linux

**With "auto" on, the program now works on its own: the questions that interrupted it every few seconds after it read a web page are gone. A question nobody answers no longer looks like a refusal or a frozen screen. When Google's servers have a passing fault, the program tries again instead of stopping.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if you use "auto".** On 0.1.150, after the AI had read any web page, it
still stopped to ask before harmless commands. A test on an old Mac that asked
33 questions on the first try now asks none.

**Yes, if you use the Google sign-in (Antigravity) or Gemini.** A passing server
error (the "HTTP 500" kind) used to end the answer. Now it is tried again, up to
four times, and the screen says so.

**Everyone else:** a safe update, nothing you rely on changes.

## Why this matters to you

Think of "auto" as telling a helper "you have my permission, get on with it".
Until now the helper still knocked on the door every few seconds after reading
anything from the internet, to ask about things that could not hurt you. You
said auto; it now means auto. It still stops and asks before anything that
cannot be undone, before using the administrator password, and before writing
outside the folder you gave it.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why this release exists

On 23 October the owner ran version 0.1.150 on his MacBook Air with "auto" on, and it still stopped to ask before a harmless command, because a web page had been read earlier. His words: auto must mean auto. The same day a question was left unanswered for ten minutes; the program then said "Permission denied" although nobody had pressed Deny, and the screen sat on "Waiting for response..." for good. And one answer ended with "Internal error encountered (HTTP 500)" from Google's servers, after which nothing happened for 40 minutes. This version fixes all three.

### What you will notice

**Questions in "auto" mode**
- Before: after the AI had read a web page, the program asked before network commands, downloads and most other tools, even with "auto" on.
- After: with "auto" on it does not ask about those. It still asks before things that cannot be undone, administrator commands (sudo, installing for the whole computer) and writing outside your folder. Measured on the MacBook Air: an install test that asked 33 questions on 0.1.149 asked none on this code.
- Affects: everyone who uses "auto".

**A question nobody answers**
- Before: after ten minutes the program said "Permission denied", left the question on screen, and pressing Allow afterwards did nothing; the screen kept saying "Waiting for response...".
- After: the question is taken off the screen when it expires, and the program says "nobody answered in time" instead of "denied". The answer stops cleanly.
- Affects: anyone who walks away from a question.

**Google server errors**
- Before: one "HTTP 500" from Google (Antigravity sign-in or Gemini) ended the answer.
- After: the program tries again up to four times, waiting a little longer each time, and shows "Google server error (HTTP 500), retry 2 of 4". It only retries before any text has appeared, so you never see the same words twice.
- Affects: everyone using Google's AI.

### Deliberately not done

- Nothing about what is asked in "ask" mode changed: there every action still asks.
- Web pages the AI reads are still marked to the AI as "data, not instructions". Only the extra questions went.
- The Mac program is still not signed by Apple (that costs a yearly fee); the first open needs right-click, Open.

### Privacy and security

Removed: the rule that, after any web page was read, made "auto" ask before anything that could send data. The owner decided this on 23 October: it asked so often that people press Allow without reading, which protects nothing. What still stops "auto": actions that cannot be undone, administrator commands, writing outside the folders you gave it. In "ask" mode everything asks, as before.

### How to install

**On a Mac (macOS 10.13 or later, Intel):** on this page, under Assets, click `gorilla-opencode-0.1.151-mac-10.13-x86_64.zip`. Open the folder Safari makes in Downloads, right-click `Install Gorilla OpenCode.command`, choose **Open**, then **Open** again. When it says DONE, double-click "Gorilla OpenCode" on the Desktop. It moves the version you had to the Trash; `Undo Gorilla OpenCode.command` brings it back.

**On Windows:** download `gorilla-opencode.exe`, then in a terminal in that folder: `.\gorilla-opencode.exe install`.

**On Debian, Ubuntu or Mint:** `sudo apt install ./gorilla-opencode_0.1.151_amd64.deb`. **Fedora, openSUSE, Rocky:** the `.rpm`. **Arch, CachyOS:** the `.pkg.tar.zst`. **Any other Linux:** `gorilla-opencode-v0.1.151-linux-amd64`, one file you make executable, or the `.tar.gz`.

To check a download is the published file, compare it with `SHA256SUMS-v0.1.151.txt` (Mac: `shasum -a 256 FILE`; Linux: `sha256sum -c SHA256SUMS-v0.1.151.txt --ignore-missing`).

### If something goes wrong

- **The program still asks in "auto" mode:** it is one of the kept cases (cannot be undone, administrator, outside your folder). The question says which.
- **"Google server error ... retry 4 of 4" and then it stops:** Google's service is having a longer fault. Wait a few minutes and ask again.
- **On a Mac, "unidentified developer":** right-click, **Open**.

### Common questions

**Is "auto" now unsafe?** It does what you told it to do in your folder, and still stops before the things that cannot be taken back. If you want a question for every action, use "ask".

**Why ten minutes before a question expires?** It is long enough to come back to the computer and short enough that a run left alone does not hang forever.

### Bottom line

With "auto" on, the program now gets on with the job; with nobody at the keyboard it says plainly that nobody answered; and a passing fault at Google no longer ends your answer.

---

## Measured on the published files

Every file below was built on 2026-10-23 on one Linux machine (Debian, Go 1.24.4) from a clean clone of commit b3cdf25 (main d1a5571 plus the notes and the build-script mode fix), by `fieldkit pipeline run gorilla-opencode`: modules verified against go.sum, vet clean, then the Linux and Windows programs built and built a second time in another folder, time zone and locale, **byte for byte identical**; then the macOS 10.13 program and the four Linux packages. The Mac zip was made by `scripts/build-mac-zip.sh` from the same clone; the program inside it is byte for byte the pipeline's Mac program.

The full test suite ran in the pipeline: every package passed except one test, `TestPfindSemanticSendsNothingAndDownloadsNothing`, which needs the Python library chromadb and that library is not installed on the build machine (it fails the same way on main without this release's changes).

| File | Bytes | SHA-256 |
|---|---|---|
| `gorilla-opencode-0.1.151-1-x86_64.pkg.tar.zst` | 21,605,488 | `c0d3c1dd4fe86ae9b0da86e6d86736c4f5f6b8dd248891d27e2f3a8e4a673ace` |
| `gorilla-opencode-0.1.151-1.x86_64.rpm` | 25,061,486 | `cc1b5f4659f69bab9de0ffaf855a549e054528f592fe346ff0fb190febfa39f0` |
| `gorilla-opencode_0.1.151_amd64.deb` | 20,527,480 | `47c9b14b3f336db3b5761d550dd220c15dcadb25b86fbde3a6def99963c751a6` |
| `gorilla-opencode-0.1.151-mac-10.13-x86_64.zip` | 20,781,074 | `fcde781b23887bd4b9877ed48176673c1b96a631a10bc8dba9eb1fd7acd29851` |
| `gorilla-opencode.exe` | 55,016,448 | `75c16380e22bff1111a4bc4463ddcb6e258bad799b487797113296aa1b0fcdf9` |
| `gorilla-opencode-linux-x86_64.tar.gz` | 20,287,924 | `68edc9e129a6505317788fb329751c12890e576e70bcaeb7ad1a74701ea76501` |
| `gorilla-opencode-v0.1.151-linux-amd64` | 53,604,536 | `510c46e4823a749a7cdd3334b229e6012061e0c91e3d56fd39b3daa7d6572b31` |

## The pictures

### The MacBook Air, the layout this release keeps

Version 0.1.150 installed from its release page on the MacBook Air (macOS 10.13.6, 167x39), used by the owner. 0.1.151 does not change the screen layout; a picture of 0.1.151 itself, installed from this page, is added below once taken.

[![Gorilla OpenCode v0.1.150 on the MacBook Air, installed from its release page: one input line with one cursor, the bottom bar reads ? help and the version in full, the answer is readable white and the italic note is yellow](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/35cfafcc6a6335dc27319c75fdc3c4d4c3b0046b/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-23-v0150-from-release-page.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/35cfafcc6a6335dc27319c75fdc3c4d4c3b0046b/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-23-v0150-from-release-page.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

PR #8 removes every branch of the permission path that keyed on untrusted content ("taint"): the egress and tool branches in `permission.mustAskAnyway`, the taint refusal in `permission.familyCovers`, the taint store (`internal/permission/taint.go`, deleted), its recording calls in fetch, websearch, biodata, MCP and peers, and the new-host bookkeeping that only served it. Auto now stops only for irreversible commands (`dangerous.go`), hard stops (`HardStopIn`) and paths outside every workspace root. It also publishes a `DeletedEvent` when a question is settled, so the TUI takes the dialog down, words a `PermissionWait` expiry as "nobody answered" rather than a denial, and marks the message stopped only after its tool results are stored. PR #9 retries Google HTTP 500/502/503/504 up to 4 times (2 s doubling plus 20%) in the Antigravity and Code Assist clients (`serverretry.go`) and in the Gemini API client's `shouldRetry`, before any text has streamed.

### Known alternatives considered

- Keeping the taint rule with a "remember for this session" answer: rejected by the owner, the question frequency itself was the fault.
- Retrying after text had started streaming: rejected, the person would see the same words twice.

### Architecture impact

The permission service no longer holds per-session taint state. `Request` publishes `DeletedEvent` on settle; consumers that only acted on `CreatedEvent` (plain mode, ACP) are unchanged.

### Toolchain

Go 1.24 (Debian). All files from one Linux machine by `fieldkit pipeline run gorilla-opencode` (toolchain, modules, vet, full test suite, build, reproducibility rebuild, macOS 10.13 binary, packages, checksums); the Mac zip by `scripts/build-mac-zip.sh`.

### Resource deltas

File sizes are in "Measured on the published files". No new dependency.

### Code changes

- `internal/permission/`: `mustAskAnyway`, `familyCovers`, `taint.go` removed, `Request` settle events, timeout wording.
- `internal/llm/tools/`: taint marking removed from fetch, websearch, biodata, untrusted (MCP), mcp-tools.
- `internal/llm/agent/`: `ClearTaint` per turn and `markPeerTaint` removed; stopped-message ordering.
- `internal/tui/`: `settlePermission` takes a settled dialog down.
- `internal/llm/provider/serverretry.go` (new), Antigravity and Code Assist `send`/`stream`, Gemini `shouldRetry`.
- `scripts/build-darwin-legacy.sh`: mode 755 in git.

### Subsystem changes

Permissions: auto asks only for irreversible, hard stop, outside-root. Providers: Google 5xx retried before streaming, notice on screen.

### Test coverage

- `TestAutoAfterAFetchedPageAsksNothingForANetworkCommand`: under auto, after a real fetched page, a new host, the owner's `urllib ok` command, a `urlretrieve`, `scp` and `git push`: 0 dialogs on this code, 5 on 0732750.
- `TestASettledQuestionIsAnnouncedAndATimeoutIsNotADeny`, `TestASettledQuestionTakesItsDialogDown`, `TestAnUnansweredQuestionSaysSoAndStoresResultsFirst`.
- `serverretry_test.go`: the real Antigravity `stream` against canned 500/502/503/504 responses, the cap, and no retry after text.
- Field: the MacBook Air stranger install test, run 5 on 6029d9a: 0 permission dialogs (run 1 on 0.1.149-dev: 33); Fieldkit installed, `fieldkit doctor --lean` READY. Two Antigravity HTTP 500s in that run were before PR #9.

### Security posture

Relaxed: auto no longer asks after untrusted content before egress or other tools. Kept: irreversible commands, hard stops, writes outside the roots, the "data, not instructions" fence around fetched text, everything in ask mode.

### Deployment

```sh
# fetch
gh release download v0.1.151 --repo gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, tar.gz, .deb, .rpm, .pkg.tar.zst, Mac zip, SHA256SUMS-v0.1.151.txt
# verify
sha256sum -c SHA256SUMS-v0.1.151.txt
# Expected: each downloaded file: OK
# install (Mac, as a new user would, from this page)
fieldkit mac release-install NAME --tag v0.1.151
# Expected: [ok] v0.1.151 installed from the release page
# verify_active
~/.local/bin/gorilla-opencode --version
# Expected: v0.1.151
```

### Known issues

- Mac build unsigned; no arm64 Mac build.
- `TestPfindSemanticSendsNothingAndDownloadsNothing` needs chromadb (absent on the build machine); `TestMatchingIsFast` is a timing test that fails under load.
