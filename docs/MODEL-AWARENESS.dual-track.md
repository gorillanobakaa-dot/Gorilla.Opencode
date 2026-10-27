# The model knows its tools and its machine - dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers job 06 of the goal "windows-capabilities" (2026-10-27).

---

# Track one: plain language

## What happened

The owner tried Gorilla OpenCode with local models and found they did not know Gorilla's own tools, the tools on the computer, that robocopy can copy on up to 128 threads, which PowerShell they were typing into, PowerToys, or how to look at file attributes. The reason was simple: the instructions the AI receives said only "Platform: windows".

Now, at the start, the program looks at the computer once (without going on the Internet and without starting any program) and tells the AI in a few short lines: which Windows and edition, whether it runs as administrator, which PowerShell it is typing into and its version, how many processor threads there are and that robocopy takes up to 128, whether long paths and Developer Mode are on, whether git, Visual Studio's C++ compiler and Windows Terminal are installed, PowerToys and Sysinternals only if they are there, and which helper programs are built into this copy of Gorilla.

The AI also gets a short table of its own tools, one line each, made from the real list of tools so it can never be out of date.

Longer how-to notes (robocopy's switches and what its exit numbers mean, PowerShell file commands, attributes and permissions, services and event logs, each built-in helper) are NOT sent every time. The AI reads one when it needs it.

## Honest state of play

Done and tested on this laptop (Windows 11 Home, Windows PowerShell 5.1, Visual Studio Build Tools 2022): the printed instructions show every line above, and 25 deliberate breakages of the new rules were each caught by the tests.

Not tested here: a machine with PowerShell 7 as the shell, with PowerToys installed, or with Sysinternals on the PATH (the tests use made-up folders for those); Linux and macOS (their tests compile here but only run on GitHub); and a real conversation with a local model using the new information (that is job 07).

## Worst case if something is wrong

- A wrong line about the machine (for example, an old version number) misleads the AI until the program restarts: the look is taken once per run.
- The instructions are now about 520 tokens longer on every turn. On a small local model with a small memory that is real. It can be switched off with the "Environment info" row in /context (that also removes the folder listing).

## What changed for you

**What the AI is told about this computer**
- Before: "Platform: windows".
- After: a few lines: Windows edition and build, administrator or not, the PowerShell edition, CPU threads and robocopy's limit, what is installed, the built-in helpers.
- Affects: Everyone on Windows; on Linux and macOS the same lines without the Windows facts

**The AI's own tools**
- Before: only the technical definitions; small models often did not connect them to a request.
- After: "YOUR TOOLS", one line per tool, made from the real tool list.
- Affects: Everyone

**How-to notes**
- Before: none; the AI guessed robocopy switches and PowerShell syntax.
- After: the AI reads a note on demand with the files tool (`action=howto`). PowerToys and Sysinternals notes appear only when they are installed.
- Affects: Everyone on Windows

**PowerShell 7 users**
- Before: told never to use `&&`, which PowerShell 7 accepts.
- After: the warning is only given for Windows PowerShell 5.1.
- Affects: People who installed PowerShell 7

## How to check it works on your computer

In PowerShell, in the Gorilla.Opencode folder:

```powershell
cd C:\Users\gorilla1\Documents\Gorilla.Opencode; & 'C:\Program Files\Go\bin\go.exe' build -o C:\Users\gorilla1\Documents\Gorilla.Opencode\gorilla-opencode.exe .; & C:\Users\gorilla1\Documents\Gorilla.Opencode\gorilla-opencode.exe system-prompt
```

It prints the full instructions the AI would receive from this folder. Look for the lines under "Platform: windows" and the "YOUR TOOLS" table. Add `--mcp` to include tools from your MCP servers (that starts them). If no model server is running it prints a one-line note first and the instructions anyway.

## Should you be concerned?

No new network access, no data sent anywhere: everything is read from this computer's registry, files and PATH. The look happens once per run.

## Glossary

- **System prompt**: the instructions the AI receives before your message, every turn.
- **Token**: the unit AI models count text in, about four characters.
- **robocopy**: the copy program built into Windows; it can copy with many threads at once.
- **PowerShell 5.1 / 7**: the PowerShell built into Windows, and the newer one you can install.

## Claim Sources

- "about 520 tokens longer": measured with `gorilla-opencode system-prompt` on this laptop, 12,087 bytes after; ~10,015 before (the same output minus the new lines).
- "25 deliberate breakages ... caught": `fieldkit mutate --plan tests/mutations-model-awareness.yaml`, 25 caught, 0 survived.

---

# Track two: developer

## Problem Being Solved

`prompt/coder.go` rendered `Platform: %s` from `runtime.GOOS` and nothing else about the machine (REPORT.md gaps #18/#19), the coder prompt claimed "Windows internals" while carrying only Linux hints (gap #20), the bash tool described itself as plain "PowerShell" with a 5.1-only `&&` ban on every edition, and nothing told the model about the files tool's robocopy, the embedded toolbox or installed tools.

## Approach Taken

- `internal/hostprobe`: `Get()` probes once (`sync.Once`) and `Facts.Lines()` renders. Windows (`probe_windows.go`): `HKLM\...\Windows NT\CurrentVersion` (ProductName, DisplayVersion, CurrentBuildNumber, UBR; build >= 22000 renames "Windows 10" to "Windows 11"), token elevation (`windows.GetCurrentProcessToken().IsElevated()`), `LongPathsEnabled`, `AllowDevelopmentWithoutDevLicense`, PowerShell versions from `HKLM\SOFTWARE\Microsoft\PowerShell\3\PowerShellEngine` and `...\PowerShellCore\InstalledVersions\*\SemanticVersion`, MSVC from `%ProgramData%\Microsoft\VisualStudio\Packages\_Instances\*\state.json` (the files vswhere reads; an instance counts only with `VC\Tools\MSVC`), `wt.exe` on PATH, PowerToys under `%LOCALAPPDATA%\PowerToys` or `%ProgramFiles%\PowerToys` (CLIs listed from the installed files: names containing "cli", plus `PowerToys.Awake.exe`), Sysinternals names on PATH, `toolbox.Names()`. No process is started.
- `shell.ShellPath()` (new, exported) is the bash tool's program without starting it; `hostprobe.ShellEdition` names it.
- `internal/howto`: `cards/*.md` embedded; `Available()` filters by OS, PowerToys, Sysinternals and the embedded toolbox; `files` tool `action=howto` serves them (`path` = card name; empty lists them).
- `tools.ToolTableBlock` builds "YOUR TOOLS" from the coder's tool set in `agent.CoderAgentTools` (`prompt.SetToolTable`); deferred tools stay in the existing deferred catalogue. A tool may implement `Summarizer`; bash and files do.
- `[[needs os.<goos>]]` line gates in `prompt/sections.go`; the ext4 and distro-package lines are `os.linux`.
- `gorilla-opencode system-prompt [--cwd DIR] [--mcp]` prints `agent.CoderSystemPrompt`.

## Before

`<env>` had Working directory, git repo yes/no, `Platform: windows`, the model line. bash: "Executes a given PowerShell command", "NEVER '&&'" on every edition.

## After

Measured on this laptop:

```
Platform: windows
OS: Windows 11 Home 25H2 (build 26200.9711), amd64; not administrator
Shell (the bash tool): Windows PowerShell 5.1.26100.8875 (powershell.exe)
CPUs: 12 logical; robocopy /MT:n takes 1-128 threads (default 8), the files tool drives it
Windows: long paths on, Developer Mode off
Installed: git, MSVC (Visual Studio Build Tools 2022 17.14.40), Windows Terminal (wt)
Embedded toolbox: none in this build
How-to cards (read one with the files tool: action=howto, path=NAME): robocopy, powershell-files, attributes-acl-registry, services-events-tasks
```

followed (after the project summary) by `YOUR TOOLS (loaded now; call them by these names)` and one line per tool, e.g. `bash  Runs Windows PowerShell 5.1.26100.8875 (powershell.exe); not bash`. A `-tags toolbox` build lists its helpers and adds one card per helper.

## Known Alternatives

- Shelling out to `vswhere.exe`, `pwsh -v`, `reg query`: rejected, a process per prompt render (or per start) and vswhere is not on every machine; the owner preferred pure Go for state.json.
- Putting the cards in the prompt: rejected, ~3,000 tokens on every turn for knowledge most turns do not need.
- A separate `howto` tool: rejected, one more schema on every turn; the files tool is always loaded and never deferred, and tool_search is off by default.

## Files Changed

- `internal/hostprobe/{hostprobe.go,probe_windows.go,probe_other.go}` + tests
- `internal/howto/howto.go`, `internal/howto/cards/*.md` (13) + test
- `internal/llm/tools/{tooltable.go,bash.go,files.go}`, `internal/llm/tools/shell/shell.go` (`ShellPath`) + `machine_awareness_test.go`
- `internal/llm/prompt/{coder.go,sections.go,coder-modern.txt}`, `system-prompts/current/coder-modern.md` + `machine_awareness_test.go`, `storage_unified_test.go` (9259 -> 9285)
- `internal/llm/agent/{tools.go,systemprompt.go,schema_cost_test.go}` + `systemprompt_test.go`
- `cmd/system_prompt.go`
- `tests/mutations-model-awareness.yaml`, `docs/WHAT-A-CURIOUS-USER-COSTS.md`

## Decisions Made

- The table and the probe ride `prompt.env`: one switch for "orientation", and `EnvironmentInfoBlock` (what /context prices) includes the table once the agent has set it.
- Absent registry values for long paths and Developer Mode are reported "off" (the Windows default).
- PowerToys CLIs are read from the install folder, never from a list in the code (the brief: "verify from the installed files, never from memory").
- The Sysinternals card tells the model to ask before `-accepteula` (it writes HKCU on the user's behalf).
- "Windows internals" became "Windows builds (MSVC, PowerShell, robocopy)": what the program now really gives.

## Tried and Abandoned

- A 90-character cap per table line: ~1,760 bytes for 18 tools; cut to 72 (1,500 bytes) with summaries for bash and files so the important words survive the cut.

## ⚠ Claimed But Not Verified

- PowerShell 7 as the shell (registry `SemanticVersion`, the 7-specific `&&` text): not on this laptop; logic covered by unit tests only.
- PowerToys and Sysinternals detection on a real install: tested with synthetic folders only; PowerToys is not installed here.
- Linux and macOS: compiled (`GOOS=linux go vet`, test binaries built; darwin go1.24.13 build); their tests run only in CI.
- Whether local models now USE the tools and cards: job 07.
- The /context price of prompt.env includes the table only after the agent built its tool set.

## Open Items

- A 7za card when 7-Zip joins the embedded toolbox (not pinned yet).

## How to verify this work is correct

```
go build -o gorilla-opencode.exe . ; .\gorilla-opencode.exe system-prompt
go test -count=1 ./internal/hostprobe/... ./internal/howto/... ./internal/llm/prompt/... ./internal/llm/agent/...
fieldkit mutate --plan tests/mutations-model-awareness.yaml --cwd . -- go test -count=1 ./internal/hostprobe/ ./internal/howto/ ./internal/llm/prompt/ ./internal/llm/agent/ ./internal/llm/tools/ -run "(?i)(carriestheprobe|windowslines|linuxlines|shelledition|getiscached|windows11|msvc|powertoys|card|robocopy|carriesthetooltable|osgated|renderedsystem|howto|chaining|namestheedition|tooltablelines)"
```

## Technical Debt

- `GatedLineTokens("prompt.localtools")` still counts the `os.linux` lines on Windows (an over-estimate of ~60 tokens in /context for a default-off row).

## Claim Sources

- Prompt text and sizes: `gorilla-opencode.exe system-prompt` on the L15 laptop, 2026-10-27.
- Mutation result: 25 caught, 0 survived, 0 refused (same date).
- Schema figure: `go test ./internal/llm/agent -run TestDefaultToolSchemaCost -v`, 11,143.
