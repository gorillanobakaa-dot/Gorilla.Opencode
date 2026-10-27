# Production ready: what the Windows work found, and what became of it - dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers job 09 (points 1 to 3) of the goal "windows-capabilities" (2026-10-27). The owner asked that nothing found along the way be left as "found, not fixed". Point 4 (release readiness) and point 5 (cleanup) are the next job's.

---

# Track one: plain language

## What happened

Every earlier job of this goal wrote down what it found but did not fix. All of those notes were collected (the jobs' hand-over files, the plan's rows, the owner's inbox) and each one was either fixed, with a test that fails on the old code and passes on the new, or explained as not being a fault of the program. Then six gaps from the original report were closed, and a new read-only tool for Windows facts was added.

## Honest state of play

Fixed and tested on this laptop (Windows 11):
- A command run in a folder that someone else deleted reported "no output, success". It now says the folder is gone, does not run the command, and moves the session to the nearest folder that still exists.
- Web search no longer needs the lynx program: the program can read search results pages itself. It is **off until you turn it on**, because it sends your search words to four search engines; whether it should be on by default is your decision (inbox).
- The code-review installer uses Scoop on Windows.
- The sparse checker says clearly why it is Linux-only and what to use instead.
- Pictures can be read (OCR) with the text recognition built into Windows, offline, when tesseract is not installed. Tested on a real picture made on this laptop.
- Sign-in files are encrypted for your Windows account (DPAPI). Old sign-ins keep working and are encrypted the next time they are saved.
- The release script can sign the program, once you have a code-signing certificate (inbox: what to get).
- The new `windows` tool (see docs/WINDOWS-TOOL.dual-track.md).
- On the way: the picture tests on Windows used to call Windows' own convert.exe (a tool that converts a disk from FAT to NTFS) by mistake. They no longer can.
- On macOS a conversation could wait forever for a helper program that had crashed; it now notices.

Not tested here: macOS and Linux (they are built and checked for errors, but their tests run on GitHub), a real signing certificate, the web reader's three backup search engines, and going back to an older version after the sign-in was encrypted.

## Worst case if something is wrong

- Sign-in encryption: if an older version of the program is started after the new one saved your sign-in, the older one cannot read it and asks you to sign in again. Nothing is deleted.
- The web reader is off by default, so it changes nothing until you switch it on.
- The deleted-folder fix refuses to run the command rather than run it somewhere else: you are told, and run it again.

## What changed for you

**A folder deleted under the program**
- Before: "no output (exit code 0)", as if the command had worked.
- After: "The working folder ... no longer exists ... The command was NOT run. The session is now in ...".
- Affects: everyone, mostly Windows.

**Searching the web on Windows**
- Before: "web search is not configured", and an install of lynx through Scoop.
- After: the same message names the three ways to turn it on, one of them needing no install (the built-in reader). `gorilla-opencode doctor` lists what is there and what is missing.
- Affects: Windows users without lynx or a SearXNG.

**Reading text in pictures**
- Before: only with tesseract installed.
- After: with Windows' own OCR when tesseract is not there.
- Affects: Windows users.

**Your sign-ins**
- Before: plain files any program running as you, or any administrator, could read.
- After: encrypted for your Windows account on this computer.
- Affects: Windows users.

## What you can do now

Run `gorilla-opencode doctor`: under "Tool capabilities" it says which web search route and which OCR engine this computer has, and what is missing.

## What is still missing

Your two decisions (inbox): whether the web reader is on by default, and the code-signing certificate. The release itself (the next job).

## How to check it works on your computer

`gorilla-opencode doctor` prints "Tool capabilities" with a web search line and an OCR line. View a screenshot in a chat: the answer starts "TEXT READ OUT OF THE IMAGE ... using OCR (Windows OCR)" when tesseract is not installed.

## Should you be concerned?

No new connection opens by itself: the web reader is off, Windows OCR runs offline, the windows tool reads only this computer, signing runs only with your certificate, and nothing was downloaded.

## Glossary

- **DPAPI**: Windows' built-in way to encrypt data so only your Windows account on this computer can decrypt it.
- **OCR**: reading the text in a picture.
- **Code signing**: a certificate-backed stamp on the program saying who made it, which Windows checks.
- **Scoop**: a package installer for Windows that needs no administrator rights.

## Claim Sources

The test names in the table below, run on this laptop on 2026-10-27 under `fieldkit guard`.

---

# Track two: developer

## Problem Being Solved

Job 09 point 1: every "found, not fixed", blocker or gotcha that is a defect, from `state/goals/windows-capabilities/jobs/*/HANDOFF.md`, the goal's PLAN.md rows (section "2026-10-26: Gorilla OpenCode Windows capabilities", job 07's row) and the goal's inbox items. Point 2: REPORT.md gaps #12, #13, #14, #15, #17, #24. Point 3: the read-only `windows` tool.

## Approach Taken

Each item: reproduce, test first where the defect is ours (the test was run against the old code and seen failing), fix, run the touched package. Items that are not defects of Gorilla OpenCode say why.

### Point 1: every finding

| # | Defect or finding | Source | Fix commit | Test, or "not a defect: why" |
|---|---|---|---|---|
| 1 | Windows PowerShell 5.1 in a deleted working folder: no native program starts, no `$LASTEXITCODE`; the wrapper said "no output (exit code 0)" | jobs/own-fixes HANDOFF "Found, not fixed"; PLAN job 01 row | be54386 | `TestDeletedCwdNativeCommandReportsTheRealFailure` (old code: "reported success"), `TestDeletedCwdRecoveryPicksTheNearestExistingParent` |
| 2 | Fieldkit `mutate` crashes on UTF-8 test output | jobs/own-fixes HANDOFF, mutate-handover.md | Fieldkit f48eb88 (fix/mutate-utf8, job fieldkit-mutate-utf8) | Fieldkit `tests/test_mutate_utf8.py` (4 failed before, pass after); not in this repository |
| 3 | macOS: an MCP call to a stdio server that died waits for ever (the zombie answers signal 0; the test was skipped on darwin) | jobs/local-model-acceptance HANDOFF NOT verified; PLAN job 07 row | 82a820d | `TestAnMCPServerThatExitsMidCallEndsTheCall`, skip removed; darwin check via `sysctl kern.proc.pid` P_stat == SZOMB. Compiled and vetted for darwin; not run on a Mac |
| 4 | OCR tests on Windows ran `convert` from PATH, which is System32\convert.exe (FAT to NTFS volume conversion), with ImageMagick's arguments (it exited 4, the tests skipped) | found by this job | d77a96c | `TestOCRReadsTextOutOfAnImage`, `TestAnImageWithNoTextIsAnAnswerNotAFailure`, `TestOCRWritesNothingBesideTheImage` now run on Windows (System.Drawing image; only `magick` is ImageMagick there) |
| 5 | `scripts/git-hooks/hooks.py` line 112 calls `run()` with keyword arguments it does not take | jobs/toolbox-embed HANDOFF Blockers | - | Not a defect: `mandate_gate(stdin_text, env, run=subprocess.run)` takes `run` as a parameter that shadows the module's `run()`, and its one caller (line 125) passes none, so it is `subprocess.run` |
| 6 | Pre-commit hook fails with WinError 206 on ~2,000 new files; the hook file is missing in a new worktree (gitignored) | jobs/pfind-rs, jobs/toolbox-embed | - | A defect of the repository's own hook, which sessions may not change (hard line 3); worked around with 200-file batches and a copy of the hook. Owner item in the inbox |
| 7 | pfind-rs ignore/attributes lines (5d9c57f) | jobs/pfind-rs BLOCKER, inbox | - | Not a defect: an owner decision under hard line 3, already in the inbox |
| 8 | Pushes refused (HARD LINE 1), no GitHub credential in autopilot runs, gh not logged in | jobs/ci-red-tests, ci-red-fixes, files-tool, model-awareness, windows-ci, ci-green, fieldkit-mutate-utf8; inbox #55, #56, #83, #143, #144, #145 | - | Not a product defect: the harness's push path; owner decisions in the inbox. Job ci-push publishes |
| 9 | Commit a06b212's message says 41 Files tests; there are 34 | jobs/files-tool | - | Not a defect of the program: history; amending is refused (hard line 2); the PLAN row corrects it |
| 10 | 8.3 short TEMP on the CI runner broke containment checks; Go 1.23+ `EvalSymlinks` no longer follows junctions; review report written in cp1252; Linux `\` credential paths; pwsh 7 `&&` | jobs/ci-red-fixes | 7272737 (job 08 follow-up) | `TestCacheDirIsPrivateShortPath*`, `TestSensitiveShortPath*`, `TestWorkspaceShortPath*`, `TestAReportingAnalyserDoesNotCrashTheReportOnAnANSICodePage`, `TestPowerShellCredentialPathsAskOnEveryOS_ReadOnly` (already fixed before this job) |
| 11 | Linux mirror preview did not list files inside an extra folder; `"/proc/"+pid+"/stat"` read as a slash command | jobs/ci-green | c3d91fe | `TestFilesGoEngineMirror`, `TestEverySlashCommandNamedInProseActuallyExists` (already fixed) |
| 12 | `TestTmpD65b50ccLocaleHint` skips on PowerShell 7; `TestASymlinkIsDescribedByWhereItActuallyGoes` skips without symlink rights | jobs/ci-red-fixes "Skips added" | - | Not defects: each skip names a condition the runner honestly lacks (the hint is for 5.1's `&&` parse error; the junction analogue runs) |
| 13 | MCP discover crashed on YAML boolean card inputs; the call then hung | job 07 row | ef8cf1d (hang); Fieldkit PR #145 (crash) | `TestAnMCPServerThatExitsMidCallEndsTheCall` |
| 14 | Job 07's fixes: init offer ate keys, files schema wording, PowerToys line, hidden items, YOUR TOOLS header, find depth note, ChatGPT re-send | job 07 row | a381ac7, ace8ce1, 3b92eea, e4d0cc6, 5b46cfe, 8ed0bb7, 5d502c1 | Named in docs/WINDOWS-ACCEPTANCE.dual-track.md (already fixed) |
| 15 | what_tools: the answer did not name robocopy | job 07 row | - | Not a defect: the YOUR TOOLS line for files says "robocopy" on Windows; the model did not repeat it |
| 16 | prefix_01: the model used `01_$1` and then claimed 01/02/03 | job 07 row | - | Not a defect: the tool result showed `01_` three times; the model contradicted it. VALID in round 1 on the same schema |
| 17 | 13:47 "crash" of job 07 | jobs/local-model-acceptance | - | Not a defect: the command post's `taskkill /T` and `lms unload` on the owner's order (RAM) |
| 18 | 04:14 power cut; a worker "crashed x2 on missing cwd" | PLAN command-post rows | - | Not product defects: two unguarded heavy jobs at once (now a rule), and a worker started in a missing worktree (harness) |
| 19 | ChatGPT gpt-5.4-mini retired; still the background model in `backgroundModelByProvider` | jobs/local-model-acceptance | - | Not a defect: `helperTargetFor` keeps the coder's model when the background model is not registered, and the live catalogue (`RefreshChatGPT`) replaces the list, so a retired model disappears on refresh |
| 20 | `fieldkit mutate` leaves the last mutant's binary in target/ | jobs/pfind-rs | - | Not a defect: mutate restores sources; build output belongs to the test command, and `mutate_check.py` rebuilds first |
| 21 | Not built yet from job 04's list: jq/yq `data` action (gojq, yq), 7za from source, a rebuild script for the six helpers, Linux/macOS embeds, windows-arm64 helpers | jobs/toolbox-embed "Next" | - | Not defects: unbuilt scope, handed to job production-ready. (MSVC discovery from state.json exists: internal/hostprobe/probe_windows.go) |
| 22 | The vendored code-review toolkit now differs from its development copy (Fieldkit local kits) | this job (71b4771) | - | `TestVendoredToolkitMatchesDevCopy` will name `install_tools.py` and `tools_registry.py` on a machine that has the development copy; copy them back there |

### Point 2: REPORT.md gaps

| Gap | Fix commit | Test |
|---|---|---|
| #12 web search without lynx: HTML to text in Go (lynx's dump layout, so `parseLynxDump` reads it), off by default, SearXNG unchanged; refusal and `doctor` name what is missing | ec9cc57, 49fcc59 | `TestHTMLToTextWritesLynxLayout`, `TestHTMLToTextFeedsTheLynxParser`, `TestHTMLToTextBuiltinReaderSearchesWithoutLynx`, `TestHTMLToTextReaderIsOffUntilTheUserTurnsItOn`; one live query (`TestLiveBuiltinReaderFindsResults`, opt-in): Brave answered |
| #13 code-review installer knows Scoop | 71b4771 | `TestToolkitInstallerKnowsScoop` (failed on the old installer) |
| #14 sparse stays Linux-only, says why and what to use | c45f56b | `TestSparseOnWindowsSaysWhyAndWhatToUse` |
| #15 OCR via Windows.Media.Ocr (PowerShell 5.1 WinRT, offline), tesseract first when present | d77a96c | `TestWindowsOCRReadsAGeneratedPNG`, `TestWindowsOCRIsTheFallbackWithoutTesseract` (real OCR of a generated PNG) |
| #17 DPAPI for the OAuth files | 1dc7812 | `TestDPAPIRoundTrip`, `TestDPAPIReadsAnOldPlainFile`, `TestDPAPIMigratesOnNextWrite`, `TestDPAPIForeignFileIsKept` |
| #24 signing step behind `GORILLA_SIGN_CERT_SHA1` | 53a3ebd | `TestReleaseSigningIsOffWithoutTheOwnersCertificate`, `TestReleaseSigningFailsClosed` |

Point 3: the `windows` tool, 3bd7ddf and 7525cf0, tests `TestWindowsTool*`; see docs/WINDOWS-TOOL.dual-track.md.

## Before

See the "Defect or finding" column.

## After

See the "Fix commit" and "Test" columns.

## Known Alternatives

- #12: keep web search off on Windows until lynx is installed (the old state), or turn the reader on by default (the owner's decision, inbox).
- #17: Windows Credential Manager instead of DPAPI-sealed files: one more API, and the file names other tools look for (Fieldkit's parity check) would disappear.
- #15: a WinRT binding generated with go-winmd: a code generator and a NuGet download for one call that PowerShell 5.1 makes natively.

## Files Changed

`internal/llm/tools/shell/shell.go`, `deleted_cwd_test.go`; `internal/llm/agent/mcp_server_exit_other.go`, `mcp_server_zombie_*.go`, `mcp_server_exit_test.go`; `internal/llm/tools/htmltext.go`, `htmltext_test.go`, `websearch_builtin.go`, `websearch.go`, `websearch_lynx.go`, `capabilities.go`; `internal/config/config.go`; `cmd/doctor.go`; `internal/llm/tools/codereview/toolkit/install_tools.py`, `tools_registry.py`, `installer_scoop_test.go`; `internal/llm/tools/sparse.go`, `sparse_test.go`; `internal/llm/tools/ocr.go`, `ocr_windows.go`, `ocr_other.go`, `ocr_test.go`, `ocr_windows_test.go`; `internal/auth/secretfile.go`, `dpapi_windows.go`, `dpapi_other.go`, `secretfile_windows_test.go`, `gemini_oauth.go`, `antigravity_oauth.go`, `chatgpt_oauth.go`; `scripts/build-toolbox.ps1`, `internal/version/release_signing_windows_test.go`; the windows tool's files (its own document).

## Decisions Made

- Deleted folder: the command is not run in the fallback folder (it could act on the wrong files); the model is told both folders.
- Web reader: off by default (fail closed); proposed default in the inbox.
- DPAPI: sealing failure writes the old plain form rather than nothing (a refreshed token that is not saved ends the sign-in); an unreadable sealed file is kept, never deleted.
- Signing: no timestamp server by default (it learns every signed hash); a requested signing that fails stops the script.

## Tried and Abandoned

- A separate `scripts/sign-windows.ps1`: `/scripts/*` is ignored and adding an exception changes the ignore rules (hard line 3), so signing became step 5 of `build-toolbox.ps1` (`-SignOnly` runs it alone).

## ⚠ Claimed But Not Verified

- macOS zombie detection (82a820d): darwin build and vet only.
- The web reader's Ecosia, Mojeek and Marginalia fallbacks live; only Brave answered the one live query.
- A real Authenticode signature (no certificate exists).
- Downgrading after DPAPI sealing (expected: the older version asks for a new sign-in).

## Open Items

Two owner decisions in the inbox (web reader default; code-signing certificate) and one owner item (the pre-commit hook's WinError 206 and its absence in worktrees).

## How to verify this work is correct

```
fieldkit guard -- go test -count=1 ./internal/... -run "^(TestDeletedCwd|TestHTMLToText|TestWindowsOCR|TestDPAPI|TestWindowsTool)" -v
```

## Technical Debt

The built-in reader and lynx share `lynxEngines`; an engine changing its HTML breaks both the same way, and `parseLynxDump`'s "zero results is the failure" rule reports it.

## Claim Sources

Test output seen on this laptop on 2026-10-27; commit hashes from `git log` on feat/windows-capabilities.
