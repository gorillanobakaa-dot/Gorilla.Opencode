# Windows acceptance run (job 07) - dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers job 07 of the goal "windows-capabilities" (2026-10-27): the owner's everyday Windows file operations, typed as a person would type them into the real program, each scored from what ended up on the disk.

---

# Track one: plain language

## What happened

The owner tried Gorilla OpenCode on Windows and found it did not know its own tools: robocopy's 128 threads, file attributes, PowerShell. Jobs 01 to 06 added those tools. This job checked that an AI model actually uses them correctly. It typed 35 requests into the program (copy, mirror, move, rename, list, attributes, delete, "what tools do you have", "build this with MSVC", "which files came from the internet") and checked the result on disk each time, never trusting what the model said.

The run started on the local model Qwen3.6-35B-A3B in LM Studio, as the brief asked. Three requests passed, but each took 4 to 7 minutes. The owner then stopped the local model because the laptop's memory was 88% full and ordered: no local model, use ChatGPT, in a window he can watch. Everything after that ran on ChatGPT's GPT-6-Luna (the ChatGPT sign-in's default and lightest model) in a visible window.

## Honest state of play

- **Final result: 33 of 35 VALID (94%)** on the fixed program, checked on disk.
- The first ChatGPT pass found 8 failures (27 of 35). Seven had a cause in our own code or in the test itself. All seven are fixed and tested (see "What changed for you").
- **Two remain INVALID, both the model's own choice:**
  - "Prefix every file here with 01_, 02_, 03_": the model gave every file the prefix 01_, then claimed it had numbered them 01, 02 and 03. The tool's own result showed 01_ three times. The same request passed in the first pass. The program reported the truth; the model misreported it.
  - "What tools do you have?": after the fix it listed its tools correctly, including the files tool and PowerShell, but never said "robocopy", which the check requires. The brief allows one fix round per failure, and this one used its round.
- The local model was not tested beyond its first three requests (all VALID). That was the owner's decision; nothing on that model was scored INVALID.

## Worst case if something is wrong

- The run worked only in scratch folders it created. Mirror, purge and move delete files for good. In this run a person never had to approve, because the test program answered "allow" for them. In real use you see the preview and decide.
- In the first pass the model swapped source and destination on a purge and deleted one file from the source. That happened in a scratch folder. The preview had said so in plain words ("DELETED for good from ...\src"), and a person reading it would have refused. The tool's wording that misled the model is fixed.

## What changed for you

**Typing while the program starts**
- Before: if the "Initialize project?" offer was on screen, the first letters you typed could vanish. Typing at an ordinary pace could lose the whole message: the test sent "What is in the folder Folder?" and the AI received "t is in the folder Folder?".
- After: every key you type reaches the chat box. Only y, n, q, Enter and Esc answer the offer.
- Affects: everyone, on every system.

**A tool server that crashes**
- Before: if an add-on tool server (an "MCP server", here Fieldkit) crashed during a call, the chat waited forever ("Waiting for tool response... 320s").
- After: the call ends at once and says the server stopped.
- Affects: anyone who uses MCP servers.

**ChatGPT over a weak connection**
- Before: one failed connection (for example "TLS handshake timeout" on a phone hotspot) ended the turn.
- After: a request that never reached the server is sent again, twice (after 2 and 4 seconds). A request that did reach the server is never sent twice.
- Affects: people who use the ChatGPT sign-in.

**Lists of folders**
- Before: "what is in this folder" could leave out a subfolder whose files were all deeper down, without saying so. "Show the attributes of every file" left out hidden files.
- After: a listing names every folder it did not open, and hidden files are listed and marked as hidden.
- Affects: everyone.

**What the AI is told about this computer**
- Before: asked "is PowerToys installed?" the AI said it could not tell. Asked to build with MSVC it got the quoting wrong twice.
- After: it is told "PowerToys: not installed" (or where it is installed), and it is told exactly how to start the MSVC compiler from PowerShell.
- Affects: Windows.

**Copy, purge and move**
- Before: the tool's description did not say which way "purge" deletes, or that "move" also takes the folders away.
- After: it says both. The model now picks the right one: purge and "move the files but leave the folders" both pass.
- Affects: everyone.

## What you can do now

Ask in plain words for the operations on the owner's list ("mirror src to dst with 64 threads", "make report.txt read-only", "delete the .tmp files") and expect them done with the program's own tools, with a preview before anything is deleted for good.

## What is still missing

- A full run on the local model (stopped by the owner for memory; see above).
- A run with the program's own settings left alone. This run used the owner's real settings with three overrides (see Track two), so the Fieldkit add-on tools were loaded.
- Linux and macOS: nothing here ran on them. The changed code compiles for both.

## How to check it works on your computer

Start Gorilla OpenCode in a folder you have not used before, so the "Initialize project?" offer appears. Type a sentence straight away, without waiting: every letter must arrive. Then ask "Is PowerToys installed?" and "What is in this folder?" in a folder that has a subfolder of subfolders.

## Should you be concerned?

No. Every operation ran in throw-away folders, and those folders are now in the Recycle Bin. The memory the local model used was freed: it was unloaded, and LM Studio was stopped.

## Glossary

- **VALID / INVALID**: what the disk shows after the request. VALID means the files are exactly as asked; for a question, the answer matches what is on disk.
- **MCP server**: an add-on program that offers the AI extra tools.
- **robocopy**: Windows' built-in copier; it can copy with up to 128 threads at once.
- **Mark of the Web**: the tag Windows puts on a file downloaded from the internet.

## Claim Sources

- Results: the runner's per-scenario result files and logs (local, `%USERPROFILE%\Documents\Scripts\job07-acceptance\`, recycled at the end; the tables below are generated from them by `tools/table.py`).
- Fixes: commits ef8cf1d, a381ac7, ace8ce1, 3b92eea, e4d0cc6, 5b46cfe, 8ed0bb7, 5d502c1 on feat/windows-capabilities.

---

# Track two: developer

## Problem Being Solved

Brief 07 asks whether a real model drives the Windows capabilities of jobs 01-06 correctly: every OWNER-EXAMPLES.md operation plus "what tools do you have", "how many robocopy threads", "is PowerToys installed", a repo search with Python removed from PATH, an MSVC build, "attributes of every file here" and Mark of the Web. Each is scored from the file system, and timings and tokens are recorded per call.

## Approach Taken

- Build: the helpers job 04 staged were already in place, so plain `fieldkit guard -- go build -tags toolbox -trimpath -ldflags "-s -w" -o gorilla-opencode.exe .` (about 84.96 MB with the 7 embedded helpers). The exe was rebuilt after each fix; the final scores come from the build at 8ed0bb7, and the copy64/msvc rerun from the build at 5d502c1.
- Runner: `scripts/model-tests/windows_acceptance.py`. It lives in an ignored folder: like the repo's other model-test scripts it stays local. One scratch folder per scenario, real files, scored from disk. Two modes:
  - `-p` headless (permission mode auto), used for the Qwen part.
  - `--console --visible` for all ChatGPT runs: the real TUI in its own visible console window, the prompt typed with WriteConsoleInput, and each permission dialog saved and then answered "a". Headless mode refuses irreversible actions by design (permission.go, unattended carve-out). The console mode lets mirror, purge and move run after the preview.
- Model and config, ChatGPT: the sign-in lives in the real config folder, and a hard line forbids reading, copying or printing credential files. So the runner leaves XDG_CONFIG_HOME alone and sets three viper overrides through the environment: `GORILLA_OPENCODE_AGENTS.<AGENT>.MODEL=chatgpt.gpt-6-luna`, `GORILLA_OPENCODE_DATA.DIRECTORY=<scratch>`, `GORILLA_OPENCODE_PERMISSIONS.MODE=auto`. config.json was not edited.
  - Because the owner's real config.json was used, the Fieldkit MCP server was loaded too, and its "matching Fieldkit tools" hint was added to each prompt. That is why the prompt is about 21,000 tokens per call on ChatGPT, against about 13,800 on the isolated Qwen config.
- Model choice: the owner asked for `chatgpt.gpt-5.4-mini`. That model is retired: the 2026-10-19 catalogue cache lists only gpt-6-luna, gpt-5.6-terra and gpt-5.6-luna. A headless smoke run on gpt-5.6-luna (the catalogue's "cheapest") passed `rename` in 9 s. In the TUI the session ran on gpt-6-luna, and the owner, re-signing in at 13:56, confirmed gpt-6-luna as the default ("you've got enough credit for luna"). Every scored ChatGPT run used gpt-6-luna. The runner marks a scenario INVALID if any reply came from a different model.
- Fix rounds: one per failure, as the brief says. Round 1 is the full set, then the 8 failures were rerun after fixes, then a full final run on the fixed build. copy64 failed in the final run on a network error, before the model saw the prompt. That led to the retry fix, and copy64 (with msvc, to apply the stricter MSVC check) was rerun on that build.

## Before

| Run | Model, build | VALID |
|---|---|---|
| Qwen, headless (stopped by owner order) | local Qwen3.6-35B-A3B, 1ed8595 | 3/3 |
| ChatGPT round 1, console | GPT-6-Luna, ef8cf1d | 27/35 (77%) |

## After

| Run | Model, build | VALID |
|---|---|---|
| ChatGPT fix round (the 8 failures) | GPT-6-Luna, 5b46cfe | 6/8 (list then failed on the depth-2 listing, fixed in 8ed0bb7) |
| **ChatGPT final, all 35** | GPT-6-Luna, 8ed0bb7 | 32/35 as run; **33/35 (94%)** with copy64 rerun on 5d502c1 |

Final run totals (35 scenarios): 111 model calls, 2,340,975 prompt tokens and 7,072 output tokens. Per call that is 21,090 tokens in and 64 out; the median call took 3 s, the slowest 31 s. Wall time per scenario is 25-68 s. Most of it is the runner: 5 s of start-up, typing at 30 ms a key, and 15 s before it first checks for a finished turn. The model's own time is the "Seconds per call" column.

## Every INVALID, explained

| Scenario | Round 1 cause | Ours? | Fix | After |
|---|---|---|---|---|
| purge | Model called purge with path=dst, dest=src. The schema said only "purge=only delete extras". It deleted c_new.txt from src in scratch, then claimed success. | Yes (schema) | ace8ce1: "purge=delete from dest what path lacks", path "left as it is by purge" | VALID (fix round and final) |
| mov_files | Model used move (/MOVE, folders go too), not move_files (/MOV) | Yes (schema) | ace8ce1: "move=/MOVE (folders go too), move_files=/MOV (folders stay)" | VALID (fix round and final) |
| list | (a) Input lost: the model got "t is in the folder Folder?" (b) In the fix round: answer without sub2 | Yes, both | (a) a381ac7: the init-offer key gate. (b) 8ed0bb7: the depth-2 plain listing names the folders it did not enter | VALID (final) |
| item_props | Answer correct ("Modified: <true time>"), but the check demanded the word "LastWriteTime" | Yes (runner) | The check now requires size 720 AND the true modified time read from disk (stricter on facts) | VALID (fix round and final) |
| what_tools | Model called Fieldkit's MCP readiness tool and answered "1,499 Fieldkit tools" | Partly (prompt) | 5b46cfe: the YOUR TOOLS header says to answer from it | Still **INVALID**: correct list, but no "robocopy" (one round used) |
| powertoys | "I can't determine..." (no tool call) | Yes (prompt) | 3b92eea: `PowerToys: not installed` in <env> on Windows | VALID (fix round and final) |
| msvc | Two failed attempts to quote `cmd /c "...VsDevCmd.bat..."` from PowerShell 5.1 | Yes (prompt) | 3b92eea: <env> names VsDevCmd.bat with the .cmd-file recipe | VALID (fix round, final and rerun); the rerun also passes the Rich-header check |
| attrs_all | The listing hid the two hidden files; the model noted it and stopped | Yes (tool) | e4d0cc6: list shows hidden/system items, marked; hidden folders are entered only with hidden=true | VALID (fix round and final) |
| copy64 (final run only) | `net/http: TLS handshake timeout` on the first request; turn ended before the model saw the prompt | Yes (provider) | 5d502c1: re-send on a never-connected error (dial, DNS, TLS handshake timeout), 2 s then 4 s | VALID (rerun) |
| prefix_01 (final run only) | Model used find `^(.*)$` / replace `01_$1` (not the numbering template), then claimed 01/02/03; the tool result showed 01_ three times | No (model) | none: VALID in round 1 on the same tool schema | **INVALID** |

Not counted against Gorilla OpenCode: in round 1, before the restart, the Fieldkit MCP server crashed on `discover`. The cause was a TypeError in fieldkit/agent.py: card input names `yes`/`on`/`n` were read by YAML as booleans. It is fixed in Fieldkit PR #145 (command post). Our side of it, the call that waited forever, is ef8cf1d.

## Files Changed

- `internal/llm/agent/mcp-tools.go`, `mcp_server_exit.go`, `mcp_server_exit_windows.go`, `mcp_server_exit_other.go` (+ test): callMCP watches the stdio server's process. On Windows it reads the exit code; on Linux it uses signal 0 and the zombie state. The call is cancelled with "the MCP server's process exited before it answered".
- `internal/tui/init_offer.go` (+ `startup_keys_test.go`): a held key the offer has no use for withdraws the offer and goes to the chat. h and l move the offer but are replayed to the chat if typing follows.
- `internal/llm/tools/files.go`: purge/move/move_files wording; `hidden` now means "enter hidden/system folders".
- `internal/llm/tools/files_attr.go` (+ test): list shows hidden/system items by default.
- `internal/llm/tools/find.go`, `find_depthnote.go` (+ test): depth-cut note naming the folders; a folder of only folders no longer reads "No matches found."
- `internal/llm/tools/tooltable.go` (+ test): header line.
- `internal/hostprobe/hostprobe.go`, `probe_windows.go` (+ tests): `PowerToys: not installed`; `MSVCDevCmd` and the `MSVC build:` line.
- `internal/llm/provider/chatgpt.go`, `chatgpt_connect_retry.go` (+ test): never-connected retry.

## Decisions Made

- **List shows hidden items by default.** This reverses job 05's Get-ChildItem default (hidden only with hidden=true). The attribute view is where hidden matters. Hidden folders are still not entered on a recursive list, so .git cannot flood it.
- **PowerToys is named when absent.** This reverses job 06's "only when found". It costs about 5 tokens a turn, and a missing line reads as "not checked".
- **Retry only before the connection.** Errors after the connection (reset, cut stream) are not retried here, because the backend may already have the request. openai.go's isTransientStreamError is broader on purpose, for its own stream-resume path; it was not reused.
- **Model under test is GPT-6-Luna** (see Approach). gpt-5.4-mini is retired.

## Tried and Abandoned

- Copying chatgpt-oauth.json into an isolated config: refused. It is a credential file (hard line), and a refresh there could rotate the owner's refresh token.
- A per-folder `.gorilla-opencode.json` to choose the model: it would be a file inside each scenario folder, and the listing, rename and attribute checks would see it.

## ⚠ Claimed But Not Verified

- macOS: the MCP exit watch cannot see a zombie child there (it still answers signal 0), so on macOS a crashed server still hangs a call, as before. The new test is skipped on darwin. The Linux branch compiles but did not run here (CI).
- The ChatGPT retry was unit-tested with a fake transport only. A real second TLS timeout followed by success was not observed. The copy64 rerun simply did not hit one.
- The "Initialize project" fix was tested in the model (unit tests) and seen working in the real exe: the final `list` prompt arrived whole. The h/l replay path was tested only in unit tests.

## Open Items

- A local-model run, when the owner has memory for it. The runner supports it: `--model local.qwen3.6-35b-a3b`, isolated config.
- what_tools: whether to name robocopy in the files tool's table line (it does on Windows: "Copy/mirror/move via robocopy (1-128 threads)..."; the model did not repeat it).
- prefix_01: the model misreporting a tool result. No tool change can stop a model from contradicting its own tool output.

## How to verify this work is correct

```
fieldkit guard -- go test -count=1 ./internal/llm/agent/... ./internal/tui/... ./internal/llm/tools/ ./internal/hostprobe/... ./internal/llm/provider/
fieldkit guard -- go test -count=1 -run "ExitsMidCall|KeysTypedOneByOne|PlainListingNames|NeverConnected|TLSHandshake" -v ./internal/llm/agent/ ./internal/tui/ ./internal/llm/tools/ ./internal/llm/provider/
```

`TestAnMCPServerThatExitsMidCallEndsTheCall` and `TestKeysTypedOneByOneIntoTheOpenOfferAllLandInTheBox` were run against the old code, and both FAILED there:
- the MCP test hit its 15 s limit;
- the box held `" the folder Folder?"`, `""` and `""`.

Cross-OS: `go vet` on Windows and Linux, compilation of the Linux test binaries, and a darwin build with go1.24.13 all exited 0 for every touched package.

## Technical Debt

- The runner's 15 s minimum before it checks for a finished turn inflates wall times. The per-call seconds are the real numbers.
- chatgpt.go still uses http.DefaultClient, not resilientHTTPClient (httpclient.go).

## Processes, models and cleanup (evidence)

| When | What | Who started it | How it ended |
|---|---|---|---|
| 13:13:39 | LM Studio 17036 6792 6052 8016 11408 + llama-server 7172 (Qwen, 41.5 GB) | this job (`lms`, earlier session) | stopped 13:47:06-23 by the command post (Zero), owner order: RAM 88%. This killed the job's session too: the 13:47 "crash" was this stop, not a runner fault (ROOM.log lines 3788-3789; LM Studio log 13:47:04 "Client disconnected") |
| 13:47:06 | LM Studio 7452 11952 15060 16920 17456 | Zero's own `lms unload --all` woke the service | stopped by Zero 13:51:49 (not mine, not touched) |
| 13:50:30 | llama-server 17768, Qwen3.6-35B-A3B reloaded | this job, before the owner's order reached it | `lms unload qwen3.6-35b-a3b` at 13:51; process gone; `lms ps`: no models loaded |
| 13:53-14:55 | runner python + one gorilla-opencode.exe per scenario | this job | each exe killed by the runner after its turn; a stuck run (python 17948, exe 2960) stopped by PID 13:59 |
| 14:57:35 | LM Studio 14364 4820 15684 304 17976 | this job: `lms ps` woke the service while checking | `taskkill /PID 14364 /T` 14:58; all five gone |

At the end: no gorilla-opencode, llama-server or LM Studio process running; `lms ps` reported no model loaded; 49.6 GB of 63.7 GB free. The scratch folder `job07-acceptance` (exe builds, run folders, logs) was moved to the Recycle Bin with `fieldkit size recycle`; see the last line of this document.

## Results

Prompts as typed (OWNER-EXAMPLES.md robocopy, rename, listing, attribute, move, copy and delete blocks, then the brief's extras):

| Scenario | Prompt typed |
|---|---|
| attrs_all | Show the attributes of every file here. |
| attrs_one | What attributes does report.txt have? |
| copy64 | Copy everything in the folder src (with its subfolders) into a new folder called backup, using robocopy with 64 threads. |
| copy_dir | Copy the folder SourceDir to BackupDir, with everything in it. |
| copy_file | Copy File.txt to backup\File.txt. |
| del_file | Delete File.txt. |
| del_folder | Delete the folder Folder and everything in it. |
| del_locked | Delete LockedFile.txt. It is read-only; force it. |
| del_tmp | Delete all the .tmp files in Folder. |
| inspect | Show me the details of report.txt: size, dates and attributes. |
| item_props | Show all the properties of report.txt. |
| list | What is in the folder Folder? |
| list_dirs | List only the subfolders of Folder. |
| list_files | List only the files in Folder, not the folders. |
| list_hidden | Show me the hidden files in Folder. |
| list_recurse | List everything in Folder, including what is inside its subfolders. |
| make_normal | Clear every attribute on locked.txt so it is a normal file. |
| make_ro | Make report.txt read-only. |
| mir16_z | Mirror SourceDir to DestDir with robocopy, 16 threads, restartable mode. |
| mirror64 | Mirror src to dst with 64 threads, so dst ends up exactly like src: delete whatever src no longer has. |
| motw | Which files in this folder came from the internet? |
| mov_files | Move all the files from src (including the ones in its subfolders) into dest, keeping the folder layout, but leave src's folders where they are, just empty. |
| move_file | Move notes.txt into the dest folder. |
| move_force | Move notes.txt into dest, replacing the notes.txt that is already there. |
| move_tree | Move the folder src, files and subfolders, into archive\src. src should be gone afterwards. |
| msvc | Build hello.c with MSVC into hello.exe and run it. |
| powertoys | Is PowerToys installed? |
| prefix_01 | Prefix every file here with a counter in alphabetical order: 01_, 02_, 03_ and so on. |
| purge | Delete from dst every file that is no longer in src. Do not copy anything new over, only remove the old ones. |
| rename | Rename OldName.txt to NewName.txt. |
| replace_names | In this folder, rename every file so that 'oldText' in its name becomes 'newText'. |
| robo_threads | How many robocopy threads can you use here? |
| search_nopy | Search this repo for frobnicate and tell me which files have it. |
| unset_ro | report.txt is read-only. Make it writable again. |
| what_tools | What tools do you have? |

### Tables per run (generated from the result files)

Columns: wall seconds for the whole scenario; model calls; seconds per call (from the message timestamps); prompt and output tokens per call (session totals / calls); the tools the model called; what the check measured on disk.

**Qwen3.6-35B-A3B, headless (local, LM Studio)**: 3/3 VALID (100%)

| Scenario | Result | Wall s | Model calls | Seconds per call | Prompt tok/call | Output tok/call | Tools called | Checked on disk |
|---|---|---|---|---|---|---|---|---|
| copy64 | VALID | 393 | 8 | 230, 14, 13, 13, 16, 22, 39, 44 | 14,470 | 136 | files, files, bash, bash, bash, bash, bash | backup == src: True; 64 threads asked for in a call: True |
| rename | VALID | 242 | 2 | 235, 7 | 13,816 | 86 | files | files now ['NewName.txt'] |
| replace_names | VALID | 314 | 3 | 257, 30, 25 | 13,965 | 147 | files, files | files now ['newText_report.txt', 'notes.txt', 'summary_newText.md'] |

**ChatGPT GPT-6-Luna, round 1**: 27/35 VALID (77%)

| Scenario | Result | Wall s | Model calls | Seconds per call | Prompt tok/call | Output tok/call | Tools called | Checked on disk |
|---|---|---|---|---|---|---|---|---|
| attrs_all | **INVALID** | 28 | 5 | 2, 4, 2, 3, 2 | 23,228 | 65 | find, fieldkit_discover, fieldkit_discover, files | {'ro.txt readonly': True, 'hid.txt hidden': False, 'both.txt hidden': False, 'plain.txt listed': True} |
| attrs_one | VALID | 25 | 2 | 2, 3 | 17,712 | 36 | files | tool calls: 1; missing from the answer: []; wrongly listed: [] |
| copy64 | VALID | 35 | 3 | 2, 3, 2 | 13,561 | 63 | find, files | backup == src: True; 64 threads asked for in a call: True |
| copy_dir | VALID | 26 | 3 | 2, 2, 2 | 24,195 | 75 | files, files | BackupDir == SourceDir (12 files, subfolders): True |
| copy_file | VALID | 25 | 4 | 2, 2, 3, 3 | 22,711 | 58 | find, files, files | backup/File.txt is a copy and File.txt still there: True |
| del_file | VALID | 24 | 4 | 3, 1, 3, 2 | 20,997 | 45 | find, files, files | files now ['keep.txt'] |
| del_folder | VALID | 25 | 4 | 2, 2, 2, 2 | 20,380 | 48 | find, files, files | Folder gone: True; keep.txt kept: True |
| del_locked | VALID | 25 | 5 | 2, 1, 6, 3, 3 | 20,774 | 56 | find, view, files, find | files now ['keep.txt'] |
| del_tmp | VALID | 25 | 4 | 4, 2, 2, 3 | 22,815 | 56 | find, files, files | left in Folder: ['keep.log', 'keep.txt'] (sub/c.tmp counts: the request says all .tmp files in Folder) |
| inspect | VALID | 29 | 3 | 14, 2, 2 | 16,198 | 54 | find, files | tool calls: 2; missing from the answer: []; wrongly listed: [] |
| item_props | **INVALID** | 31 | 3 | 12, 3, 4 | 22,693 | 108 | find, files | tool calls: 2; missing from the answer: ['LastWriteTime']; wrongly listed: [] |
| list | **INVALID** | 25 | 1 | 3 | 22,525 | 31 | (none) | no tool call at all |
| list_dirs | VALID | 28 | 2 | 2, 2 | 22,594 | 30 | find | tool calls: 1; missing from the answer: []; wrongly listed: [] |
| list_files | VALID | 27 | 2 | 3, 3 | 17,812 | 30 | find | tool calls: 1; missing from the answer: []; wrongly listed: [] |
| list_hidden | VALID | 25 | 3 | 2, 3, 2 | 22,813 | 43 | find, files | tool calls: 2; missing from the answer: []; wrongly listed: [] |
| list_recurse | VALID | 28 | 2 | 3, 2 | 17,736 | 52 | find | tool calls: 1; missing from the answer: []; wrongly listed: [] |
| make_normal | VALID | 29 | 6 | 2, 4, 3, 4, 1, 2 | 23,540 | 67 | files, files, files, files, files | locked.txt attributes 0x80 (want 0x80 = Normal) |
| make_ro | VALID | 25 | 3 | 2, 3, 2 | 19,430 | 75 | find, bash | report.txt attributes 0x21 (ReadOnly set: True) |
| mir16_z | VALID | 28 | 4 | 2, 3, 3, 2 | 21,355 | 82 | find, files, files | DestDir == SourceDir: True; 16 threads in a call: True |
| mirror64 | VALID | 30 | 4 | 2, 3, 4, 2 | 19,307 | 92 | find, files, files | dst is an exact copy of src (extra file and folder gone): True |
| motw | VALID | 25 | 4 | 4, 2, 2, 2 | 18,163 | 44 | find, fieldkit_discover, fieldkit_describe | both MOTW files named: True; others named as downloaded: [] |
| mov_files | **INVALID** | 35 | 7 | 2, 3, 2, 3, 3, 2, 2 | 19,237 | 61 | find, find, files, files, find, find | dest has 12 files: True; files left in src: 0; src folders kept: False |
| move_file | VALID | 25 | 4 | 2, 3, 2, 2 | 20,304 | 60 | find, files, files | notes.txt moved into dest: True |
| move_force | VALID | 26 | 4 | 2, 4, 2, 2 | 22,789 | 74 | find, files, files | dest/notes.txt replaced by the new one and source gone: True |
| move_tree | VALID | 27 | 4 | 2, 3, 3, 2 | 21,094 | 62 | find, find, files | archive/src has all 12 files: True; src removed: True |
| msvc | **INVALID** | 31 | 6 | 2, 3, 3, 4, 3, 2 | 22,832 | 105 | find, find, view, bash, bash | hello.exe was not built |
| powertoys | **INVALID** | 25 | 1 | 2 | 22,530 | 35 | (none) | PowerToys installed on disk: False; answer says not installed: False |
| prefix_01 | VALID | 28 | 3 | 2, 2, 2 | 16,271 | 57 | find, files | files now ['01_alpha.txt', '02_bravo.txt', '03_charlie.txt'] |
| purge | **INVALID** | 36 | 7 | 3, 2, 4, 2, 4, 2, 3 | 16,788 | 121 | find, files, files, files, files, files | dst now ['a.txt', 'b.txt', 'old1.txt', 'old2.txt'] (want a.txt, b.txt: old ones deleted, c_new.txt NOT copied) |
| rename | VALID | 25 | 4 | 3, 3, 3, 1 | 18,566 | 46 | find, files, files | files now ['NewName.txt'] |
| replace_names | VALID | 28 | 5 | 3, 2, 2, 1, 3 | 17,641 | 58 | find, files, files, find | files now ['newText_report.txt', 'notes.txt', 'summary_newText.md'] |
| robo_threads | VALID | 25 | 1 | 2 | 22,535 | 23 | (none) | answer says 128: True |
| search_nopy | VALID | 26 | 2 | 4, 2 | 22,609 | 42 | find | both files named: True; a call used python: False; notes.txt named: False |
| unset_ro | VALID | 25 | 4 | 2, 3, 3, 4 | 22,744 | 47 | find, files, files | report.txt attributes 0x20 (ReadOnly cleared: True) |
| what_tools | **INVALID** | 43 | 2 | 2, 2 | 22,644 | 28 | fieldkit_readiness | {'files tool': False, 'robocopy': False, 'shell is PowerShell': False, 'pfind or find': False} |

**ChatGPT GPT-6-Luna, fix round (the 8 round-1 failures)**: 6/8 VALID (75%)

| Scenario | Result | Wall s | Model calls | Seconds per call | Prompt tok/call | Output tok/call | Tools called | Checked on disk |
|---|---|---|---|---|---|---|---|---|
| attrs_all | VALID | 25 | 2 | 4, 2 | 17,896 | 47 | files | {'ro.txt readonly': True, 'hid.txt hidden': True, 'both.txt hidden': True, 'plain.txt listed': True} |
| item_props | VALID | 25 | 3 | 1, 1, 4 | 19,572 | 120 | find, files | size 720 in answer: True; true modified time in answer: True |
| list | **INVALID** | 25 | 2 | 2, 2 | 12,992 | 39 | find | tool calls: 1; missing from the answer: ['sub2']; wrongly listed: [] |
| mov_files | VALID | 32 | 6 | 2, 3, 3, 3, 2, 2 | 14,372 | 81 | find, find, files, files, find | dest has 12 files: True; files left in src: 0; src folders kept: True |
| msvc | VALID | 59 | 4 | 33, 2, 6, 2 | 18,031 | 128 | find, view, bash | hello.exe runs and prints the line: True; MSVC marks in the PE: False |
| powertoys | VALID | 25 | 1 | 13 | 22,644 | 17 | (none) | PowerToys installed on disk: False; answer says not installed: True |
| purge | VALID | 30 | 6 | 3, 2, 3, 3, 2, 2 | 15,726 | 65 | find, find, files, files, find | dst now ['a.txt', 'b.txt'] (want a.txt, b.txt: old ones deleted, c_new.txt NOT copied) |
| what_tools | **INVALID** | 25 | 1 | 4 | 22,643 | 179 | (none) | {'files tool': True, 'robocopy': False, 'shell is PowerShell': True, 'pfind or find': True} |

**ChatGPT GPT-6-Luna, final run on the fixed build**: 32/35 VALID (91%)

| Scenario | Result | Wall s | Model calls | Seconds per call | Prompt tok/call | Output tok/call | Tools called | Checked on disk |
|---|---|---|---|---|---|---|---|---|
| attrs_all | VALID | 37 | 3 | 2, 15, 8 | 19,584 | 71 | find, files | {'ro.txt readonly': True, 'hid.txt hidden': True, 'both.txt hidden': True, 'plain.txt listed': True} |
| attrs_one | VALID | 25 | 2 | 2, 2 | 13,048 | 36 | files | tool calls: 1; missing from the answer: []; wrongly listed: [] |
| copy64 | **INVALID** | 28 | 1 | 10 | 0 | 0 | (none) | no tool call at all |
| copy_dir | VALID | 26 | 4 | 2, 3, 3, 2 | 20,910 | 70 | find, files, files | BackupDir == SourceDir (12 files, subfolders): True |
| copy_file | VALID | 25 | 5 | 3, 2, 3, 2, 2 | 22,875 | 73 | find, find, files, files | backup/File.txt is a copy and File.txt still there: True |
| del_file | VALID | 24 | 3 | 3, 2, 1 | 22,746 | 43 | find, files | files now ['keep.txt'] |
| del_folder | VALID | 25 | 3 | 2, 2, 7 | 24,128 | 46 | find, files | Folder gone: True; keep.txt kept: True |
| del_locked | VALID | 25 | 5 | 3, 3, 3, 2, 2 | 22,884 | 56 | find, files, files, files | files now ['keep.txt'] |
| del_tmp | VALID | 25 | 3 | 2, 3, 4 | 19,588 | 49 | find, files | left in Folder: ['keep.log', 'keep.txt'] (sub/c.tmp counts: the request says all .tmp files in Folder) |
| inspect | VALID | 26 | 2 | 3, 2 | 22,779 | 50 | files | tool calls: 1; missing from the answer: []; wrongly listed: [] |
| item_props | VALID | 25 | 2 | 2, 3 | 22,799 | 150 | files | size 720 in answer: True; true modified time in answer: True |
| list | VALID | 25 | 2 | 8, 4 | 22,704 | 48 | find | tool calls: 1; missing from the answer: []; wrongly listed: [] |
| list_dirs | VALID | 25 | 2 | 2, 6 | 22,708 | 30 | find | tool calls: 1; missing from the answer: []; wrongly listed: [] |
| list_files | VALID | 25 | 2 | 3, 3 | 17,928 | 30 | find | tool calls: 1; missing from the answer: []; wrongly listed: [] |
| list_hidden | VALID | 25 | 3 | 3, 2, 3 | 22,954 | 46 | find, files | tool calls: 2; missing from the answer: []; wrongly listed: [] |
| list_recurse | VALID | 26 | 2 | 3, 2 | 17,850 | 50 | find | tool calls: 1; missing from the answer: []; wrongly listed: [] |
| make_normal | VALID | 26 | 4 | 3, 3, 3, 1 | 20,422 | 76 | files, files, files | locked.txt attributes 0x80 (want 0x80 = Normal) |
| make_ro | VALID | 25 | 3 | 2, 3, 2 | 22,792 | 82 | find, bash | report.txt attributes 0x21 (ReadOnly set: True) |
| mir16_z | VALID | 26 | 3 | 4, 2, 4 | 20,225 | 90 | files, files | DestDir == SourceDir: True; 16 threads in a call: True |
| mirror64 | VALID | 30 | 3 | 7, 2, 2 | 20,269 | 87 | files, files | dst is an exact copy of src (extra file and folder gone): True |
| motw | VALID | 43 | 5 | 12, 3, 2, 6, 3 | 21,568 | 66 | find, fieldkit_discover, fieldkit_discover, bash | both MOTW files named: True; others named as downloaded: [] |
| mov_files | VALID | 29 | 4 | 3, 2, 2, 4 | 24,093 | 58 | find, find, files | dest has 12 files: True; files left in src: 0; src folders kept: True |
| move_file | VALID | 25 | 4 | 6, 4, 2, 2 | 22,865 | 73 | find, files, files | notes.txt moved into dest: True |
| move_force | VALID | 68 | 7 | 5, 3, 31, 4, 3, 4, 2 | 22,083 | 60 | find, view, view, files, files, view | dest/notes.txt replaced by the new one and source gone: True |
| move_tree | VALID | 36 | 5 | 2, 2, 3, 10, 2 | 21,933 | 66 | find, find, files, find | archive/src has all 12 files: True; src removed: True |
| msvc | VALID | 25 | 3 | 2, 3, 2 | 19,570 | 94 | view, bash | hello.exe runs and prints the line: True; MSVC marks in the PE: False |
| powertoys | VALID | 25 | 1 | 2 | 12,915 | 13 | (none) | PowerToys installed on disk: False; answer says not installed: True |
| prefix_01 | **INVALID** | 26 | 3 | 2, 1, 1 | 16,383 | 47 | find, files | files now ['01_alpha.txt', '01_bravo.txt', '01_charlie.txt'] |
| purge | VALID | 39 | 6 | 6, 2, 3, 5, 2, 3 | 20,131 | 83 | find, find, files, files, find | dst now ['a.txt', 'b.txt'] (want a.txt, b.txt: old ones deleted, c_new.txt NOT copied) |
| rename | VALID | 25 | 3 | 3, 2, 2 | 22,746 | 43 | find, files | files now ['NewName.txt'] |
| replace_names | VALID | 26 | 4 | 2, 3, 3, 2 | 20,508 | 54 | find, files, files | files now ['newText_report.txt', 'notes.txt', 'summary_newText.md'] |
| robo_threads | VALID | 25 | 1 | 3 | 22,648 | 21 | (none) | answer says 128: True |
| search_nopy | VALID | 26 | 2 | 2, 2 | 17,860 | 42 | find | both files named: True; a call used python: False; notes.txt named: False |
| unset_ro | VALID | 25 | 5 | 3, 3, 2, 3, 2 | 23,520 | 58 | find, files, files, find | report.txt attributes 0x20 (ReadOnly cleared: True) |
| what_tools | **INVALID** | 25 | 1 | 4 | 22,642 | 234 | (none) | {'files tool': True, 'robocopy': False, 'shell is PowerShell': True, 'pfind or find': True} |

**ChatGPT GPT-6-Luna, rerun on 5d502c1 (copy64 after the network failure; msvc with the Rich-header check)**: 2/2 VALID (100%)

| Scenario | Result | Wall s | Model calls | Seconds per call | Prompt tok/call | Output tok/call | Tools called | Checked on disk |
|---|---|---|---|---|---|---|---|---|
| copy64 | VALID | 28 | 3 | 3, 2, 1 | 23,359 | 77 | files, files | backup == src: True; 64 threads asked for in a call: True |
| msvc | VALID | 38 | 6 | 1, 6, 3, 2, 4, 3 | 22,186 | 126 | view, bash, view, bash, bash | hello.exe runs and prints the line: True; MSVC Rich header in the PE: True |
