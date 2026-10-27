# The files tool - dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers job 05 of the goal "windows-capabilities" (2026-10-27): one tool, `files`, for the everyday file operations a person asks for in plain words.

---

# Track one: plain language

## What happened

The owner tried Gorilla OpenCode with local models and found it could not do the most basic file work: it did not know Windows can copy with robocopy on up to 128 threads at once, it did not know how to look at or change a file's attributes (read-only, hidden ...), and renaming many files meant writing PowerShell by hand.

The AI now has one tool for all of it. It can copy, mirror and move whole folders (with robocopy on Windows), make a file read-only or hidden and back, rename many files at once (replace text in the names, number them 01_, 02_ ...), show a file's details (size, dates, owner, who may open it, whether it was downloaded from the Internet), list a folder including hidden items, and delete to the Recycle Bin.

## Honest state of play

Done and tested on this laptop (Windows 11), in temporary folders only: every action, every line of the owner's list of examples, 2,000 files copied with 128 threads, names with Romanian and other accented letters, paths longer than Windows' old 260-character limit, read-only files, a folder link that points back at itself, and one real trip to the Recycle Bin.

Not tested here: Linux and macOS (they use a built-in copier instead of robocopy; their tests compile but only run on GitHub), network shares, and a real AI conversation using the tool.

## Worst case if something is wrong

- **Mirror, purge and move delete files for good.** That is what those robocopy switches do, and the owner decided they should be real. So before any of them runs you see a dry run listing what will be copied AND what will be deleted, and you are asked. Auto-approve does not answer for you here.
- If the dry run shows any error (a drive that is not there, a folder it cannot read), nothing runs and nothing is asked.
- Mirroring onto a whole drive or onto your user folder, or between two folders where one is inside the other, is refused outright.
- A rename that would give two files the same name, or take the name of a file that stays, is refused before any file moves.

## What changed for you

**Copying and backing up folders**
- Before: The AI wrote robocopy or Copy-Item commands itself, often single-threaded, with no preview of deletions.
- After: "Mirror C:\a to D:\b with 64 threads" previews, asks, then runs robocopy with up to 128 threads (default: one per processor thread). The result says what robocopy's exit code means.
- Affects: Everyone, mostly on Windows

**Attributes**
- Before: The AI had to guess PowerShell syntax.
- After: "Make report.txt read-only", "unhide this folder", "show the attributes" work directly, for one file or a whole folder.
- Affects: Everyone on Windows (on Linux and macOS only read-only can be changed)

**Renaming many files**
- Before: PowerShell pipelines written by the model, with no check for name clashes.
- After: "Prefix every file here with 01_, 02_ ..." or "replace oldText with newText in every name" shows the new names first and asks.
- Affects: Everyone

**Deleting**
- Before: A delete through the shell was permanent.
- After: The files tool sends to the Recycle Bin unless told "permanent" (and refuses "permanent" if you asked for the Recycle Bin).
- Affects: Everyone

## What you can do now

Ask in plain words: "mirror my Photos folder to E:\Backup", "move everything from Downloads\old to D:\archive", "make every .log file in here read-only", "number these photos 01_, 02_ ...", "is this file from the Internet?", "show the hidden files in this folder", "delete all .tmp files here".

## What is still missing

Compressing or encrypting files (Windows' compact and cipher), changing who may open a file (only reading it is done), and a real AI-driven test of the owner's examples (job 07).

## How to check it works on your computer

Ask the AI to mirror a test folder to another test folder with `dry_run`: it shows what would be copied and deleted, and changes nothing.

## Should you be concerned?

No new network access: the tool only touches files on this computer. The deletes of mirror, purge and move are real and permanent; that is why they are always shown first and always asked.

## Glossary

- **robocopy**: Windows' built-in "robust file copy" program.
- **Mirror**: make the destination an exact copy of the source, including deleting what is not in the source.
- **Recycle Bin**: where deleted files wait until you empty it; they can be put back.
- **Attribute**: a flag on a file, such as read-only or hidden.
- **Junction / link**: a folder entry that points somewhere else; the tool never follows one.
- **Mark of the Web**: a note Windows attaches to downloaded files (Zone.Identifier).

## Claim Sources

- The owner's examples: Autopilot/windows-capabilities-2026-10-26/OWNER-EXAMPLES.md.
- Test results: `go test -count=1 ./internal/llm/tools/ -run '(?i)files' -v` on Windows 11 26200, 2026-10-27.

---

# Track two: developer

## Problem Being Solved

No tool covered tree copies, attributes, bulk rename, item details or recycling. The model fell back to bash with hand-written robocopy/PowerShell: single-threaded copies, no preview of /MIR deletions (dangerous.go only flagged them), PowerShell syntax errors in Rename-Item scripts, and `Remove-Item` as a permanent delete (trash.go).

## Approach Taken

One tool, `files` (internal/llm/tools/files*.go), ten actions: copy, mirror, purge, move, move_files, attributes, rename, info, list, delete. One schema instead of eight: 470 tokens per turn (trimmed from 516), default ON, critical, never deferred (the complaint was a model that did not know the capability existed).

- **Tree actions** (files.go, files_robocopy.go, files_windows.go): robocopy from System32 (never PATH), log via `/UNILOG:<temp>` (UTF-16, so non-ASCII names survive any console code page), parsed by `parseRobocopyLog`. Preview = the same command with `/L`; real run adds `/MT:n` (1..128, default `runtime.NumCPU()` capped at 128) and optional `/Z`. Always `/XJ /R:2 /W:2 /FP /BYTES /NP /NJH /NJS`. purge = `/E /PURGE /XC /XN /XO /XL` (measured: deletes extras, copies nothing). move = `/E /MOVE`, move_files = `/E /MOV` (measured: /MOV leaves "same" files in the source, so the preview's delete list is the copy list). Exit code decoded with job 01's `robocopyExit`.
- **Go engine** (files_gosync.go): the same plan by size + mtime (2 s tolerance), extras for mirror/purge, links skipped (`fs.ModeSymlink|fs.ModeIrregular`), copies on `threads` goroutines via temp file + rename, read-only destinations replaced. Used outside Windows; tested directly on Windows too.
- **Order**: refuse (roots, nesting) -> dry run -> any dry-run ERROR stops -> `dry_run` returns -> permission prompt with the preview as Description -> run -> result with counts, first 20 paths per list (`full=true` all), exit meaning, errors, kept log path.
- **Permission**: `Irreversible` set when anything is deleted or overwritten, so auto-approve asks anyway and unattended runs refuse (permission.go mustAskAnyway). Reads (info, list, attributes get) never ask.
- **Attributes** (files_attr.go): Get/SetFileAttributesW (`\\?\` past 248 chars); settable ReadOnly, Hidden, System, Archive, Temporary, Offline, NotContentIndexed; `Normal` clears all; Compressed/Encrypted refused with a pointer to compact/cipher. Recursive = Get-ChildItem -Recurse (the folder itself excluded) with a preview count. Elsewhere only ReadOnly (write bits); Hidden read from a leading dot.
- **Rename** (files_rename.go): regex find/replace on the whole name, then case (lower/upper/title), then template `{n}`, `{n:Dk}`, `{name}`, `{stem}`, `{ext}`. All names checked first (duplicates, existing names not being moved, invalid Windows names, reserved devices). Two-phase rename through temp names (swaps and case-only renames work); a failure puts every name back.
- **Info**: size, times (creation via GetFileInformationByHandle), attributes, hard-link count, reparse target, owner + DACL summary (GetNamedSecurityInfo, rights named like Explorer), alternate data streams (FindFirstStreamW), Zone.Identifier decoded, SHA-256 on request.
- **List**: the Get-ChildItem view (`kind`, `hidden`, `recurse`, `pattern`), 200 lines unless `full`. Kept in files rather than find because find deliberately skips hidden and ignored files; searching stays in find.
- **Delete** (files_delete.go): SHFileOperationW `FO_DELETE` with `FOF_ALLOWUNDO|FOF_WANTNUKEWARNING`, after SHQueryRecycleBinW proves the volume has a bin; paths >= MAX_PATH refused (the shell API cannot take them). `permanent=true`: irreversible prompt, read-only cleared first, refused when `TrashAsked(session)`. Elsewhere `gio trash` / `trash-put` / Finder; none -> refused, never a silent permanent delete.

## Before

`robocopy a b /MIR` through bash: one irreversible-delete warning (dangerous.go "robocopy-deletes"), no list of what would go, one thread unless the model knew /MT.

## After

`files action=mirror path=a dest=b threads=64`: /L preview listing copies and deletes -> irreversible prompt -> `/MIR /MT:64 /XJ` -> decoded exit code and the log path.

## Known Alternatives

- IFileOperation (COM) for the Recycle Bin: richer, but needs COM plumbing without cgo; SHFileOperationW does the job with one syscall.
- Routing list to find: rejected, find hides hidden/ignored files by design (see find.go) and has no attributes.
- A Go copier on Windows too: robocopy is in every Windows, handles long paths, ACL-preserving modes and /Z restart; the owner asked for it by name.

## Files Changed

- internal/llm/tools/files.go, files_robocopy.go, files_gosync.go, files_attr.go, files_rename.go, files_delete.go, files_windows.go, files_other.go (new)
- internal/llm/tools/files_test.go, files_windows_test.go, files_other_test.go (new)
- internal/config/loadout.go (FilesComponentID row), internal/llm/agent/tools.go, calibrate.go (registration)
- internal/llm/agent/schema_cost_test.go, docs/WHAT-A-CURIOUS-USER-COSTS.md (11,141)
- tests/mutations-files-tool.yaml (new)

## Decisions Made

- Real /MIR, /PURGE, /MOVE, /MOV after a mandatory /L preview and an irreversible prompt (owner, 2026-10-26).
- Any ERROR in the dry run stops the action: measured, robocopy /L to a missing drive exits 1 while logging `ERROR 3 ... Getting File System Type of Destination`; the exit code alone would have let it through.
- Root refusal applies to the side that loses files: destination for mirror/purge, source for move/move_files. Nesting is refused for every tree action.
- `FOF_WANTNUKEWARNING`: when an item cannot go to the bin, Windows shows a warning instead of deleting for good silently. In a headless run that dialog waits; a waiting run is better than a lost file.

## Tried and Abandoned

- `SHQUERYRBINFO.cbSize = 20`: E_INVALIDARG on 64-bit; the struct is naturally aligned there (24). Fixed with `unsafe.Sizeof`.
- Including the folder itself in a recursive attribute change: it changed a folder the pattern excluded. Now Get-ChildItem -Recurse semantics.

## ⚠ Claimed But Not Verified

- Linux and macOS runtime: test binaries compile (`GOOS=linux go test -run XXX_NONE -exec echo`), darwin go1.24.13 build + vet pass; nothing ran there.
- The `gio`/`trash-put`/Finder trash path: never run.
- UNC shares, ReFS, FAT drives without a Recycle Bin (the refusal path is coded, not exercised).
- The real-run robocopy exit >= 8 branch after a clean dry run.

## Open Items

- Compress/encrypt (compact.exe, cipher.exe) and ACL changes (only reading is done).
- Job 07: the owner's examples as plain-language requests to a real model.

## How to verify this work is correct

```
go test -count=1 ./internal/llm/tools/ -run '(?i)files' -v        # must print --- PASS: TestFilesMirrorPreviewsAsksThenMirrors
fieldkit mutate --plan tests/mutations-files-tool.yaml --cwd . -- go test -count=1 ./internal/llm/tools/ -run "(?i)files"
GORILLA_TEST_REAL_RECYCLE=1 go test -count=1 ./internal/llm/tools/ -run TestFilesRealRecycleBin -v   # puts one probe file in YOUR bin
go test -count=1 ./internal/llm/agent -run TestDefaultToolSchemaCost -v   # tool.files 470
```

## Technical Debt

- `runRename`'s undo walks every step twice; correct, not elegant.
- The preview and the run are two robocopy passes: a file changed between them is handled by robocopy's own rules, not re-shown.

## Claim Sources

- Robocopy switches and exit codes: learn.microsoft.com/windows-server/administration/windows-commands/robocopy and the return-codes page cited in exitcodes.go.
- Measurements: probes of the robocopy.exe shipped with Windows 11 Home 10.0.26200, 2026-10-27 (log format, purge-only flags, /MOV and "same" files, /L to a missing drive).
