# The windows tool - dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers point 3 of job 09 of the goal "windows-capabilities" (2026-10-27): one read-only tool, `windows`, that reads facts about this Windows computer.

---

# Track one: plain language

## What happened

To answer "is the Windows Update service running?", "what went wrong at the last boot?" or "which build of Windows is this?", the AI had to write a PowerShell command, wait for you to allow it, and read text made for a person. Now it has one tool that reads five things and answers in a fixed, machine-readable form (JSON):

- the **registry** (Windows' settings database): one value, or a key with its values and subkeys;
- **services** (programs Windows runs in the background): their state, how they start, which program they are;
- the **event log** (Windows' diary of errors and warnings): errors of the last day by default;
- **scheduled tasks**: their state, last and next run, and what they run;
- a short, fixed list of **system classes** (WMI/CIM): Windows version, processor, disks, memory, graphics, network addresses, running programs, installed updates, drivers and a few more.

## Honest state of play

Tested on this laptop (Windows 11, a normal user account, not administrator): every action, real data, the caps, the error messages. Not tested: Windows 10, a machine joined to a company domain, the Security event log (it needs administrator rights, and the tool says so).

## Worst case if something is wrong

The tool cannot change anything: it opens the registry and the service list with read rights only, and the other three parts run fixed scripts that only read. The worst case is a wrong or incomplete answer, or an answer that is cut short; when it cuts, it says so and says how to see more.

What it returns goes to the AI's provider, like everything else the AI reads. So it does **not** read your computer's serial number, your network cards' hardware addresses or the full command lines of running programs (which can contain passwords). It does not ask before reading, because it only reads: that was your decision. If you do not want the AI to see your event log or registry at all, switch the tool off in `/context` (row "Windows: registry, services, event log, tasks, CIM").

## What changed for you

**Questions about this computer**
- Before: a PowerShell command per question, most with a permission prompt (only reading a registry key with Get-ItemProperty was allowed without asking), and text cut to the console's width.
- After: one call, no prompt, a complete answer in JSON, 20 items at most unless the AI asks for all of them.
- Affects: everyone on Windows. On Linux and macOS the tool does not exist and costs nothing.

## What you can do now

Ask in plain words: "is the print spooler running?", "what errors were in the System log today?", "when did the backup task last run and did it work?", "how much space is left on each drive?", "which Windows build is this?", "which updates were installed this month?".

## What is still missing

Reading another computer on the network, and changing anything (starting a service, editing the registry): those stay PowerShell commands that ask first.

## How to check it works on your computer

In Gorilla OpenCode, ask: "Use the windows tool to show the EventLog service." The answer names the service "Windows Event Log", its state (Running) and how it starts (Automatic).

## Should you be concerned?

Only about what you share with your AI provider. The tool itself reads and never writes, never touches the network, and needs no administrator rights.

## Glossary

- **Registry**: the database where Windows and programs keep their settings.
- **Service**: a program Windows runs in the background, without a window.
- **Event log**: Windows' record of what happened: errors, warnings, information.
- **WMI / CIM**: Windows' built-in way of describing the computer (hardware, system, programs) to programs.
- **JSON**: a plain-text format that programs (and AIs) read reliably.

## Claim Sources

- Tests named `TestWindowsTool*` in `internal/llm/tools/winsys_windows_test.go`, `internal/llm/agent/windows_tool_windows_test.go` and `internal/howto/howto_test.go`, run on this laptop on 2026-10-27, all PASS.

---

# Track two: developer

## Problem Being Solved

REPORT.md section 3 step 4: no structured read access to the registry, services, event log, scheduled tasks or WMI. Each read was a bash (PowerShell) call through the command gate, so a permission prompt, a re-sent conversation, and console-formatted text.

## Approach Taken

`internal/llm/tools/winsys.go` (all OS: schema, validation, caps, rendering), `winsys_windows.go` (the five readers), `winsys_other.go` (stub). Registered in `agent/tools.go` only when `tools.WindowsToolAvailable()` (GOOS windows), loadout row `tool.windows` (default ON, not critical, kept by the low-bandwidth preset because it never touches the network); `calibrate.go` sets the row to 0 elsewhere.

- **registry**: `golang.org/x/sys/windows/registry`, `OpenKey(root, sub, QUERY_VALUE|ENUMERATE_SUB_KEYS)`. Roots HKLM/HKCU/HKCR/HKU/HKCC in short, long and `HKLM:\` spellings. SZ/EXPAND_SZ/MULTI_SZ/DWORD/QWORD decoded; other types as hex (64 bytes, 2,048 with full).
- **services**: `windows.OpenSCManager(SC_MANAGER_CONNECT|SC_MANAGER_ENUMERATE_SERVICE)` and `EnumServicesStatusEx(SC_ENUM_PROCESS_INFO)` for name, display name, state and PID; `OpenService(SERVICE_QUERY_CONFIG|SERVICE_QUERY_STATUS)` + `mgr.Service.Config()` for start type, delayed start, binary and account. `mgr.Connect`/`Mgr.OpenService` are not used: they ask for ALL_ACCESS, which a normal user lacks.
- **eventlog, tasks, cim**: Windows PowerShell 5.1 (System32 path), `-EncodedCommand` of a FIXED script; every input travels in an environment variable (`GORILLA_WIN_*`), never spliced into code. `Get-WinEvent -FilterHashtable` (levels 1,2 / 1,2,3 / all; StartTime; MaxEvents = cap+1); `Get-ScheduledTask` + `Get-ScheduledTaskInfo` for the shown tasks only; `Get-CimInstance -ClassName -Property` on an allowlist of classes with fixed property lists and fixed WQL filters. Dates rendered ISO 8601 in the script (PS 5.1's ConvertTo-Json writes `\/Date()\/`).
- Caps: 20 items and 300 characters per string by default; `full=true` 500 items and 4,000 characters. The `capped` field says what was cut and how many there were.
- Microsoft's `microsoft/wmi` Go module was not used: job 04 did not evaluate it, and COMMON.md allows it only if that job found it sound. PowerShell CIM needs no new dependency.

## Before

`bash` with `Get-Service`, `Get-WinEvent`, `Get-ScheduledTask`, `Get-CimInstance`, `reg query`: none of them is on the bash tool's PowerShell read-only list (`powerShellReadOnly` in `commandclass.go`; `Get-ItemProperty` is), so each asked for permission; text output at console width.

## After

`{"action":"services","name":"eventlog"}` returns `{"action":"services","items":[{"account":"NT AUTHORITY\\LocalService","binary":"C:\\WINDOWS\\System32\\svchost.exe -k LocalServiceNetworkRestricted -p","display":"Windows Event Log","name":"EventLog","pid":2924,"start":"Automatic","state":"Running"}],"shown":1,"total":1}` (seen on this laptop).

## Known Alternatives

- `microsoft/wmi` (MIT): not evaluated by job 04, so not used.
- EvtQuery (wevtapi.dll) through syscall for the event log: more code than the fixed script, and not in x/sys.
- `schtasks /query /fo csv`: headers are localized; Get-ScheduledTask is not.

## Files Changed

`internal/llm/tools/winsys.go`, `winsys_windows.go`, `winsys_other.go`, `winsys_windows_test.go`; `internal/llm/agent/tools.go`, `calibrate.go`, `schema_cost_test.go`, `windows_tool_windows_test.go`; `internal/config/loadout.go`; `internal/howto/cards/attributes-acl-registry.md`, `services-events-tasks.md`, `howto_test.go`; `docs/WHAT-A-CURIOUS-USER-COSTS.md`.

## Decisions Made

- Always allowed (no permission request): the owner's rule for a read-only tool. Read-only is enforced by the access rights and the fixed scripts, not by filtering the model's text.
- Excluded on purpose: `Win32_Product` (querying it runs a Windows Installer consistency check that can repair products), serial numbers (`Win32_BIOS.SerialNumber`), MAC addresses, `Win32_Process.CommandLine` (secrets in arguments). The output goes to the model provider.
- Not deferred under tool_search: 291 tokens, and a model that does not see it falls back to prompted PowerShell.

## Tried and Abandoned

Nothing.

## ⚠ Claimed But Not Verified

- Windows 10, domain-joined machines, non-English Windows (the event `level` text is localized by Windows itself).
- The Security log (needs administrator; the error message path is written but was not triggered here).

## Open Items

- A `windows-arm64` run.

## How to verify this work is correct

```
fieldkit guard -- go test -count=1 ./internal/llm/tools/ ./internal/llm/agent/ ./internal/howto/ -run TestWindowsTool -v
```

## Technical Debt

The CIM allowlist is a table in `winsys.go`; a class added there needs its properties chosen with the same care (nothing identifying, nothing slow).

## Claim Sources

- `go test -count=1 ./internal/llm/tools/ -run TestWindowsTool -v`: 8 PASS (2026-10-27, this laptop, not elevated: `net session` failed).
- Schema cost: `go test ./internal/llm/agent -run TestDefaultToolSchemaCost -v`: `tool.windows ON 291`, total 11,454.
