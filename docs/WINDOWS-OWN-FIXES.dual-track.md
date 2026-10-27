# Windows fixes in our own code — dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers job 01 of the goal "windows-capabilities" (2026-10-26): eleven Windows bugs in Gorilla OpenCode's own code, fixed without any new dependency.

---

# Track one: plain language

## What happened

On Windows, Gorilla OpenCode tripped over things a Windows user does every day. A file saved by PowerShell was refused as "binary". A copy with robocopy that worked was reported as failed. Looking at a folder with `Get-ChildItem` asked for permission every single time. Accented letters printed by git came out as garbage. A program that waited for keyboard input swallowed the next commands. None of this needed anything from Microsoft: these were our own mistakes, and this job fixed them.

## Honest state of play

Done and tested on this laptop (Windows 11, PowerShell 5.1): all eleven items below, each with a test that runs the real thing where it can (real PowerShell, real robocopy in a folder the test makes, real git and Python).

Not tried: PowerShell 7 (`pwsh`, not installed here), a real language server on Windows, a real network share (UNC paths are tested as text only), Linux and macOS (the Mac build compiles; nothing was run there).

## Worst case if something is wrong

- A command the program now treats as "only looks" could change something. The list is narrow (the `Get-*` commands, `Test-Path`, `Select-String` and a few formatting commands), every part of a pipeline must be on it, and anything with a `{ }` block, a `$( )` or a `>` still asks. A credential file still asks however it is spelled.
- A text file in another old code page (Cyrillic, Greek) is shown as Western European letters. The program says which encoding it used, so the mismatch is visible.
- The dangerous-command warnings are patterns. A spelling nobody thought of gets the ordinary permission question, not the stronger warning.

## What changed for you

**Files written by PowerShell or Notepad**
- Before: "binary file, not read" for a UTF-16 file (`>` in PowerShell 5.1, .reg exports) or an old ANSI text file.
- After: The file is shown, with one line saying which encoding it was decoded from.
- Affects: Anyone reading logs, exports or old text files on Windows

**robocopy**
- Before: Exit code 1 ("files were copied") looked like a failure to the AI, to the receipt and to the warning under the answer.
- After: The result says "robocopy: success - files were copied"; 8 and above are still failures, with their meaning.
- Affects: Anyone copying or mirroring folders

**Looking at files**
- Before: `Get-ChildItem`, `Get-Item`, `Get-Content`, `Test-Path`, `Get-FileHash`, `Get-Acl` and friends asked permission every time.
- After: They run without asking, as long as nothing in the line changes anything. `Get-ChildItem | Rename-Item ...` still asks.
- Affects: Everyone on Windows

**Dangerous commands**
- Before: Deleting registry keys, deleting restore points, wiping free space, changing the execution policy, taking ownership of system files, deleting or stopping core services, scheduled tasks, start-up (Run) keys and robocopy `/MIR` got the same plain question as `dir`.
- After: They get the strong warning, saying in one sentence what cannot be undone, and they ask even when everything else is allowed.
- Affects: Everyone on Windows

**PowerShell itself**
- Before: Scripts such as `npm`, `npx` or a Python virtual environment's `Activate.ps1` could refuse to run; accented letters came out garbled; a program waiting for input could eat the next commands.
- After: Scripts run (the execution policy is bypassed for this program's own PowerShell only; the computer's setting is never changed); text is UTF-8 throughout; a program that reads input gets "nothing" instead of the command stream.
- Affects: Everyone on Windows

## What you can do now

- Ask the AI to "show me report.txt" for a file PowerShell wrote: it reads it.
- Ask it to "mirror C:\a to D:\b with 64 threads": it gets the strong warning (files can be deleted for good), and after your yes the result says what robocopy did.
- Ask it to "list the hidden files here" or "what are the attributes of report.txt": no permission question.

## What is still missing

- The robocopy `/MIR` preview (a dry run with `/L` showing what would be deleted, before the question) is job 05.
- PowerShell 7 was not available here to test.

## How to check it works on your computer

Open PowerShell in the program's source folder and run:

```powershell
cd $HOME\Documents\Gorilla.Opencode
go test ./internal/lsp/... ./internal/llm/tools/... ./internal/app/... -run "(?i)uri|executionpolicy|utf|stdin|utf16|crlf|readonly|dangerous|robocopy|exitcode" -v
```

Every line that starts with `--- PASS` is one check that passed. A line with `--- FAIL` names the check that did not.

## Should you be concerned?

No new door was opened: nothing is downloaded, nothing is sent anywhere, no setting of the computer is changed. The execution policy bypass applies only to the PowerShell this program starts for itself.

## Glossary

- **UTF-8 / UTF-16 / Windows-1252**: ways of storing letters as bytes. Windows uses all three.
- **Execution policy**: PowerShell's rule about which script files may run.
- **robocopy**: Windows' built-in tool for copying and mirroring folders.
- **Exit code**: the number a program returns when it ends; usually 0 means success, but robocopy uses 1 to 7 for success too.
- **LSP**: the protocol through which a language server reports errors in code.

## Claim Sources

- robocopy exit codes: Microsoft, "Return codes that are used by the Robocopy utility".
- fc exit codes: measured on this laptop (Windows 11 26200): 0 same, 1 different, 2 file missing, -1 bad switch.
- Every "after" above: the tests named in Track two, run 2026-10-26.

---

# Track two: developer

## Problem Being Solved

REPORT.md (goal windows-capabilities, section 1) lists 24 Windows gaps; gaps #2-#8, #16, #18, #22 and the robocopy exit-code item are bugs in our own code. Job 01 fixes them with no new module.

## Approach Taken

One fix per gap, each with a test whose name ends in a lowercase token (`_utf16`, `_robocopy` ...). `go test -run 'A|B'` splits the pattern at every top-level `|` and compiles each part on its own, so the goal check's leading `(?i)` covers only `uri`; CamelCase names alone would not be selected.

## Before

- `internal/lsp` built `file://` + path by hand (`file://C:\x`), stripped it the same way, and `DocumentUri.Path()` panicked on a bad URI.
- The shell started `powershell -NoProfile -Command -`: no execution policy bypass, OEM code page for native output, and native children inherited the command pipe as stdin.
- `view` refused any file with a NUL byte or invalid UTF-8.
- `diff.TextToPatch` split CRLF files on `\n`, leaving `\r` on every line and inserting LF lines.
- No PowerShell cmdlet was on the read-only list; Unix-only names were.
- `dangerous.go` had no registry, backup, service, task or robocopy shapes.
- Any non-zero exit was failure in the bash result, `receipt.go` and the claim guard.
- `bashRequestPath` resolved only the home folder; `HKLM:\` fell back to the working directory.

## After

| # | Gap | Change | Tests |
|---|---|---|---|
| 1 | #2 LSP URIs | `protocol.URIFromPath` / `DocumentUri.Path() (string, error)` everywhere; UNC as `file://server/share`; server-sent UNC keeps its host | `TestURIFromPathWindows`, `TestURIRoundTripWindows`, `TestURIPathBadURINoPanic`, `TestURIApplyWorkspaceEditRealPath`, `TestURINoHandBuiltFileURIs`, `TestURIDiagnosticsSameFilePath` |
| 2 | #3, #18 | `shell.PowerShellArgs` adds `-ExecutionPolicy Bypass` (process scope); hooks use it and `shell.HookPowerShell()` (pwsh when the bash tool drives pwsh) | `TestExecutionPolicyBypassArgs_executionpolicy`, `TestExecutionPolicyLiveSession_executionpolicy`, `TestExecutionPolicyHookShell_executionpolicy` |
| 3 | #4 UTF-8 | `psSessionSetup` sets `[Console]::OutputEncoding`, `InputEncoding`, `$global:OutputEncoding` to UTF-8 (no BOM); child env gets `PYTHONUTF8=1`, `PYTHONIOENCODING=utf-8` unless set | `TestUTF8ChildEnv_utf`, `TestUTF8NativeOutputIsNotMojibake_utf` (real git and python) |
| 4 | #5 stdin | `GorillaStdin` (Add-Type, kernel32 `SetStdHandle`) swaps STD_INPUT for an inheritable NUL handle inside the wrapper's try/finally | `TestStdinNativeReaderDoesNotEatCommands_stdin` |
| 5 | #6 view | `textenc.go`: BOM decides UTF-8-BOM / UTF-16LE / UTF-16BE; invalid UTF-8 without NUL, undefined bytes or >1 % control bytes is Windows-1252; the result names the encoding | `TestViewDecodes*_utf16`, `TestViewDecodes*_utf`, `TestViewControlBytesStayBinary_utf`, `TestDetectTextEncodingCutRune_utf` |
| 6 | #16 patch | `TextToPatch` and `Identify*` normalize CRLF; `PatchToCommit` writes the file's own ending back | `TestPatchKeepsCRLF_crlf`, `TestPatchCRLFPatchTextLFFile_crlf` |
| 7 | #7 read-only | `powerShellReadOnly` (Get-ChildItem, Get-Item, Get-ItemProperty, Get-Content, Test-Path, Get-Command, Get-Location, Resolve-Path, Get-FileHash, Get-Acl, Select-String, Measure-Object, Select/Sort/Format-*, aliases); `notInPowerShell` drops Unix-only names; `(cmd).Prop` segments unwrapped; `namesSensitivePath` expands `$env:` | `TestPowerShellReadOnlyPipelines_readonly`, `TestUnwrapPropertyReads_readonly`, `TestPosixListUnchanged_readonly`, `TestReadOnlyCommandsNeverAsk` |
| 8 | #8 dangerous | 11 shapes: delete-registry, delete-backups, wipe-free-space, execution-policy, take-ownership, system-permissions, delete-service, stop-critical-service, scheduled-task, autorun-key, robocopy-deletes | `TestWindowsDangerousCommands_dangerous` (33 flagged rows, 12 benign rows) |
| 9 | exit codes | `ExitCodeMeaning`: robocopy 0-7 success (bit flags), 8/16 failure; fc 1; diff/cmp 1; program = last non-cmdlet segment; bash result adds the meaning line; `outcomeForCall` in receipt and claim guard | `TestRobocopyExitCodes_robocopy`, `TestRobocopyLiveExitCodes_robocopy`, `TestRobocopyBashToolSaysSuccess_robocopy`, `TestExitCodeMeaningPrograms_exitcode`, `TestRobocopyReceiptCountsSuccess_robocopy`, `TestRobocopyClaimGuardSeesSuccess_robocopy`, `TestExitCodeOtherProgramsStayFailures_exitcode` |
| 10 | #22 paths | `expandShellPathWord` ($env:NAME, ${env:NAME}, %NAME%, ~, $HOME); `registryPath` returned as written | `TestBashRequestPathResolvesEnvAndRegistrySecurity` |
| 11 | stale docs | `bugs.txt` "Linux x86_64 only" corrected; `TO.DO.TO.FIX/BUG-ALTSCREEN-ERASE.md` (untracked) marked fixed in 0bc8ae1 | - |

## Known Alternatives

- stdin: `$null | & $sb` and `@() | & $sb` were measured on 5.1 and do not help (the native command inside the block is not the pipeline's first element). Starting the shell with stdin = NUL and feeding commands through a file or named pipe would work too, but rewrites the transport.
- view: `golang.org/x/text/encoding/charmap` decodes every code page; Windows-1252 alone is a 32-entry table and adds no import.

## Files Changed

- `internal/lsp/protocol/uri.go`, `pattern_interfaces.go`, `internal/lsp/client.go`, `watcher/watcher.go`, `util/edit.go`, `internal/llm/tools/diagnostics.go`
- `internal/llm/tools/shell/shell.go`, `internal/llm/agent/hooks.go`
- `internal/llm/tools/textenc.go` (new), `view.go`
- `internal/diff/patch.go`
- `internal/llm/tools/commandclass.go`, `commandgate.go`, `shellpaths.go` (new), `bash_outside.go`
- `internal/llm/tools/dangerous.go`
- `internal/llm/tools/exitcodes.go` (new), `bash.go`, `internal/app/receipt.go`, `claimguard.go`
- `bugs.txt`
- Tests: listed in the table; mutation plan `tests/mutations-windows-own-fixes.yaml`

## Decisions Made

- The stdin swap must sit inside the one statement the wrapper is: PowerShell 5.1 fetches its own stdin handle on every line it reads, so a swap on its own line made the shell read EOF and exit (measured).
- Setting `[Console]::InputEncoding` mid-stream was measured not to lose buffered command lines.
- `type` is read-only again under PowerShell (as the Get-Content alias it is) because `namesSensitivePath` now expands `$env:`; before, `$env:USERPROFILE\.aws\credentials` was judged as a relative path inside the workspace, so even `cat` read it unasked.
- Unix-only names are dropped from the PowerShell list rather than mapped to text: a name that does not exist there can only resolve to whatever `.exe` of that name a PATH folder holds.
- robocopy `/MIR` and friends are "dangerous" (strong warning, asks under auto-approve); the `/L` preview is job 05.

## Tried and Abandoned

- `$null |` / `@() |` in front of the script block for stdin (no effect on 5.1).
- Detaching stdin on its own wrapper line (PowerShell then read EOF and exited silently).

## ⚠ Claimed But Not Verified

- pwsh 7: `HookPowerShell` picks it and the session setup is written for both editions, but pwsh is not installed on this laptop.
- UNC: tested as text (path to URI to path), not against a real share.
- A real language server (gopls) on Windows was not run against the new URIs.
- Linux and macOS: the darwin build compiles; no test ran there.

## Open Items

- Job 05: robocopy `/L` preview before `/MIR`, `/PURGE`, `/MOVE`, `/MOV`.
- Job 06 owns tool descriptions and the system prompt; nothing here changed their wording.
- `internal/acp/server.go` still trims `file://` for display only.
- Found, not fixed (outside this job): when the shell's working directory has been deleted, Windows PowerShell 5.1 starts no native program at all, sets no `$LASTEXITCODE` and records no error, so the wrapper reports "no output (exit code 0)". Measured 2026-10-26 with plain `powershell -Command -` (no wrapper): `whoami` printed nothing, `$LASTEXITCODE` empty. `TestAFailingBuildShowsItsDiagnostics` leaves the shared test shell in such a folder; `TestRobocopyBashToolSaysSuccess_robocopy` enters its own folder first.

## How to verify this work is correct

```powershell
cd $HOME\Documents\Gorilla.Opencode
go test ./internal/lsp/... ./internal/llm/tools/... ./internal/app/... -run "(?i)uri|executionpolicy|utf|stdin|utf16|crlf|readonly|dangerous|robocopy|exitcode" -v
$env:PYTHONUTF8 = "1"; python -m fieldkit mutate --plan tests/mutations-windows-own-fixes.yaml --cwd . -- go test ./internal/lsp/... ./internal/llm/tools/... ./internal/app/ ./internal/llm/agent/ -count=1 -run "(?i)uri|executionpolicy|utf|stdin|crlf|readonly|dangerous|robocopy|exitcode|EnvAndRegistry|ReadOnlyCommandsNeverAsk"
```

The second command breaks each of the 61 rules on purpose and must report every one CAUGHT. `PYTHONUTF8=1` is needed because a failing test prints UTF-8 and `fieldkit mutate` otherwise decodes it as cp1252 and crashes. The first run left 4 SURVIVED (this console was already on UTF-8, a blocked `findstr` timed out without failing, robocopy codes of 32 and above were decoded); the tests were tightened (`forceOEMConsole`, a time limit on the stdin reader, an empty meaning above 31) and all 4 were then CAUGHT.

## Technical Debt

- The Windows-1252 heuristic guesses for files without a BOM; other ANSI code pages show as Western European letters, labeled.
- The informational exit-code table is deliberately small (robocopy, fc, diff, cmp).

## Claim Sources

- learn.microsoft.com/troubleshoot/windows-server/backup-and-storage/return-codes-used-robocopy-utility
- gnu.org/software/diffutils/manual/html_node/Invoking-diff.html (diff/cmp exit status)
- RFC 8089 appendix E.3 (UNC in file URIs)
- PEP 540 (PYTHONUTF8)
- Measurements on this laptop, 2026-10-26: stdin probe (python read the next commands before the fix; '' after), fc exit codes, robocopy exit codes (TestRobocopyLiveExitCodes_robocopy).
