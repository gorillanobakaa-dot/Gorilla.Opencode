# Gorilla OpenCode 0.1.144 — Windows and Linux

**You can now use it from inside your code editor, have it hand jobs to helpers that only look, only plan, or do the work, and put your own checks in front of its tools so a command you never want run is stopped before it starts.**

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

**Yes, if you write code in an editor** such as Zed or a JetBrains editor. The
program now works inside it.

**Yes, if you want a rule the AI cannot get round**, for example *never push my
code*. You can now write that rule yourself, and the program enforces it.

**Yes, if you use helpers.** The record printed under each answer now shows
everything a helper did, where before it hid it.

**It changes little for you** if you only chat in the terminal and do not use
helpers.

## Why this matters to you

The owner looked at Kimi Code, a coding assistant from the Chinese company
Moonshot AI, and asked a straight question: where are its tools?

Its own toolbox turned out to be small and ordinary, about fifteen tools for
reading, writing, searching and running commands. What it has is the things
around the tools. Some of those are good ideas. Some are channels back to its
maker: it reports usage by default with an identifier for your computer, it
updates itself on a schedule its maker sets for each computer, and it fetches
web pages through its maker's server first.

This version takes the three good ideas and none of the channels.

**Working inside your editor.** Until now this program lived only in a terminal
window. An editor can now start it and talk to it directly. You watch each
action as it happens, the editor asks you each permission question, and each
answer ends with the program's own record of what ran. It is the same program
with the same AI and keys; the editor is a different window onto it.

**Helpers with set jobs.** Think of a foreman. He can send one person to look
around, another to draw up a plan without touching anything, and a third to do
the job and report back exactly what was done. The program could already send
the first kind. It can now send all three. The one that does the job still asks
you before every change.

**Your own checks.** You can write a short command in the program's settings
that runs before the assistant uses a tool, and can stop it. Here is the record
the program printed when a check refused a push, on the version you download:

```
2 tool calls plus 3 by helpers, 1 did not succeed:
  agent  Append a comment line '# checked by 0.1.144' to app.py  -> ok
    | view   ...\app.py  -> ok
    | edit   ...\app.py  -> ok
    | view   ...\app.py  -> ok
  bash   git push origin main  -> refused by hook, not run
```

The indented lines are the helper's actions. Before this version they did not
appear at all: the first real test showed only *agent ... ok* while the helper
had changed a file and run a command. A record that hides changes is the false
comfort the record exists to remove, so that was fixed first.

**What it means for you, in one line:** more ways to use the program, and still
nothing happens that you cannot see or stop.

**What it does not do:** editor mode cannot yet pick up an old conversation,
take tools the editor offers, or accept pictures. It has been tested with a
script that speaks the editor protocol, not yet from Zed itself.
---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why This Release Exists

The owner looked at Kimi Code, a terminal coding assistant from Moonshot AI, and asked a fair question: where are its tools? The answer is that its own toolbox is small and generic, about fifteen tools for reading, writing, searching and running commands. Several of its channels also lead back to its maker: it reports usage by default with an identifier for your computer, it updates itself on a schedule its maker sets per computer, and it fetches web pages through its maker's server first.

Three of its ideas were worth having without any of those channels, and this version adds all three.

The first is working inside a code editor. Until now Gorilla OpenCode lived only in a terminal window. Editors such as Zed can now start it and talk to it directly: you see each action as it happens, the editor asks you each permission question, and each answer ends with the program's own record of what ran.

The second is helpers with set jobs. Think of a foreman who can send one person to look around, another to draw up a plan, and a third to do the work. The program could already send a helper to look. A helper can now also write a plan without touching anything, or make a change and report every file it changed and every command it ran. A helper that changes things still asks you first.

The third is your own checks. You can write a short command in your settings file that runs before the assistant uses a tool, and stops it. For example: never run any command containing git push. Nothing like this is set up for you; it exists only if you write it.

The first real test of the helpers found something wrong with the record printed under each answer. A helper had changed a file and run a command, and the record said only that a helper ran. That is fixed: the record now lists every action a helper took.

### What You Will Notice

**Where you can use it**
- Before: Only in a terminal window.
- After:  Also inside Zed, a JetBrains editor or any editor that speaks the Agent Client Protocol, with the same AI, keys and permission questions.
- Affects: Anyone who writes code in an editor

**Helpers**
- Before: A helper could only look through files.
- After:  A helper can look, write a plan, or make a change. The one that makes changes runs on your main AI and asks you before each change.
- Affects: Everyone

**The record under each answer**
- Before: A helper's work showed as one line saying a helper ran, even when it had changed a file.
- After:  Every action the helper took is listed, indented under it, and counted on the first line.
- Affects: Everyone

**Your own checks**
- Before: Not possible.
- After:  A command in your settings file can stop a tool before it runs, or note each action in a log. None is set up unless you write it.
- Affects: Anyone who wants a hard rule the AI cannot get round

**The research cost screen in a small window**
- Before: In some window sizes it was taller than the window, 48 rows in a 40-row window, and the line naming the keys was cut off.
- After:  It switches to its short form whenever the full form does not fit.
- Affects: Anyone using /research in a small window

### Deliberately Not Done

- **Taking Kimi Code's telemetry, automatic updates or vendor web fetching** — Each one is a channel to the maker. This program has none of them and adds none.
- **Resuming an old conversation from the editor** — Not built yet. Each time the editor starts the program, a new conversation begins.
- **Pictures and editor-supplied tools in editor mode** — Not built yet. The editor is told they are not supported.
- **Testing with Zed itself** — Editor mode was tested by a script that speaks the same protocol as an editor, on the real program with a real AI. It has not yet been used from Zed or a JetBrains editor.
- **Testing your own checks on Linux** — That part is built and checked by the compiler, but it has not run on a Linux computer.

### Privacy & Security

No telemetry was added and none exists. Editor mode adds no account, no key and no new connection: the editor talks to the program on your computer, and the program talks to the same AI provider it uses in the terminal. A helper that changes files asks your permission exactly as the main assistant does. Your own checks are commands you write; they run as you, on your computer, and the program adds none. A check that is broken, slow or cannot start stops the action it guards, so a broken guard does not let the action through.

### How to Install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora or similar Linux computer.
- Enough mobile data for a 55 MB file on Windows or a file of about 24 MB on Linux.
- On Windows you do not need to close the program first; an open window stays on the old version until you close it.

**Step 1:** Open the release page in your web browser and download the one file for your computer. Windows: gorilla-opencode.exe. Debian, Ubuntu or Mint: gorilla-opencode_0.1.144_amd64.deb. Fedora, openSUSE or Rocky: gorilla-opencode-0.1.144-1.x86_64.rpm. Also download SHA256SUMS-v0.1.144.txt.
✓ Two files are in your Downloads folder.

**Step 2:** Check the file is the one that was published. On Windows, press the Windows key, type PowerShell, press Enter. A window with a blinking cursor opens. Type cd Downloads and press Enter. Then type the command below and press Enter.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ It prints cdb631d37e284f09e3cddd801c1ff546ae4f705f8d6215cb53d4f4d415ef14f6. On Linux, run sha256sum -c SHA256SUMS-v0.1.144.txt --ignore-missing in a terminal in your Downloads folder; it prints the file name followed by OK.

**Step 3:** Windows: install it. Type the command below and press Enter.
```
.\gorilla-opencode.exe install
```
✓ It reports where it copied itself and the shortcuts it made.

**Step 4:** Debian, Ubuntu or Mint instead: type the command below and press Enter. It asks for your password because installing a program changes the system.
```
sudo apt install ./gorilla-opencode_0.1.144_amd64.deb
```
✓ A line says Setting up gorilla-opencode (0.1.144), with no line beginning with E:.

**Step 5:** Fedora, openSUSE or Rocky instead: type the command below and press Enter.
```
sudo dnf install ./gorilla-opencode-0.1.144-1.x86_64.rpm
```
✓ The last line says Complete!

**Step 6:** Confirm the version. Type the command below and press Enter.
```
gorilla-opencode --version
```
✓ It prints v0.1.144.

**Step 7:** Optional, to use it from Zed: open Zed's settings.json and add the lines below inside the outer braces, then open the agent panel and choose Gorilla OpenCode. Choose your AI provider once in the terminal program first.
```
"agent_servers": { "Gorilla OpenCode": { "type": "custom", "command": "gorilla-opencode", "args": ["acp"] } }
```
✓ Asking it to read a file shows the read as an action, then the answer, then a record beginning --- what actually ran.

**To go back:** Windows: type gorilla-opencode uninstall and press Enter, then install the gorilla-opencode.exe from version 0.1.143 the same way as in step 3. Debian: sudo apt remove gorilla-opencode. Fedora: sudo dnf remove gorilla-opencode. Your conversations, settings and keys are stored separately and none of these steps touches them.

### If Something Goes Wrong

**The editor says no AI provider is configured.**
Editor mode uses the provider you chose in the terminal program, and none has been chosen yet.
What to do: Start gorilla-opencode once in a terminal, choose a provider, then try the editor again.
Status: expected behaviour, new in 0.1.144

**The program will not start and names a hook.**
One of the checks in your settings file is written wrongly: an unknown event, an empty command, or a time limit over 600 seconds.
What to do: Fix or remove that entry in config.json; the message names it by number and shows its command.
Status: expected behaviour, new in 0.1.144

**An action shows as refused by hook, not run.**
One of your own checks stopped it, took too long, or could not start.
What to do: Read the check's output in the answer. If the check is wrong, fix it in config.json.
Status: expected behaviour, new in 0.1.144

**The record lists lines starting with a vertical bar.**
Those are actions a helper took.
What to do: Nothing to do.
Status: expected behaviour, new in 0.1.144

### Common Questions

**Q: Is Kimi Code better than this program?**
A: Its own tools are basic. Its strengths are that its maker also trains the AI it runs on, and that it is a polished, widely installed product. It also reports usage to its maker by default and updates itself on its maker's schedule. This version takes its three useful ideas and none of those channels.

**Q: Can a coder helper do something I did not approve?**
A: It asks you before each change, as the main assistant does. If you have told the program to allow everything for the session, that permission covers the helper too.

**Q: Do I need to set up any checks?**
A: No. None is set up. They exist only if you write them in config.json.

### Bottom Line

Take this version if you work in an editor, want the assistant to hand jobs to helpers, or want a rule the AI cannot get round. It changes nothing for you otherwise, except that the record under each answer now shows what helpers did. Two honest limits: editor mode was tested by a script that speaks the editor protocol, not yet from Zed itself, and your own checks have not been run on Linux.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Kimi Code's core tool set is about fifteen generic tools | 📄 stated in input | Its core tool set is about fifteen generic tools |
| Kimi Code reports usage by default with a device id | 📄 stated in input | its telemetry is on by default and carries a device id |
| Kimi Code updates on its maker's schedule per device | 📄 stated in input | it updates itself in staged rollouts its maker chooses for each device |
| Editor mode wrote a file after the client allowed it | 📄 stated in input | Allow was sent, and the file was written containing "acp on 0.1.144" |
| The coder helper runs on the main AI | 📄 stated in input | coder runs on your main model |
| The record said only agent ok before | 📄 stated in input | the receipt under the answer said only "agent ... -> ok" |
| A check that is slow or cannot start stops the tool | 📄 stated in input | A command that takes too long or cannot start also stops the tool |
| The research screen was 48 rows in a 40-row window | 📄 stated in input | the cost screen was 48 rows tall, so in a 120-by-40 window |
| Editor mode not tested from Zed | 📄 stated in input | The protocol was exercised by the client script, not by an editor |
| Hooks not run on Linux | 📄 stated in input | has not run on a Linux computer |
| Seven real-AI scenarios passed on the published Windows file | 📄 stated in input | All seven gave valid results |
| Its maker also trains the AI it runs on | 🤖 model inference | *(none — model judgment)* |


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

Every block below is real output or a real measurement, taken on the maintainer's computer from the file published on this page, unless it says otherwise. Nothing here is an illustration. The AI was Gemini 3.6 Flash through a Google Antigravity sign-in, on a separate configuration so the owner's own settings were not touched.

## Editor mode

A client script speaking the Agent Client Protocol started `gorilla-opencode acp` and asked it to create a file. These are the titles of the actions the program reported to it, in order:

```
"title": "find C:\\Users\\gorilla1\\Documents\\Gorilla.Opencode.Builds\\e2e\\run5\\sample",
"title": "write C:\\Users\\gorilla1\\Documents\\Gorilla.Opencode.Builds\\e2e\\run5\\sample\\notes2.txt",
"title": "view C:\\Users\\gorilla1\\Documents\\Gorilla.Opencode.Builds\\e2e\\run5\\sample\\notes2.txt",
```

The permission question the client was asked, before the write:

```
"title": "Create file C:\\Users\\gorilla1\\Documents\\Gorilla.Opencode.Builds\\e2e\\run5\\sample\\notes2.txt",
```

The client answered Allow. The answer ended with the program's record:

```
  find   C:\Users\gorilla1\Documents\Gorilla.Opencode.Builds\e2e\run5\sample  -> ok
  write  C:\Users\gorilla1\Documents\Gorilla.Opencode.Builds\e2e\run5\sample\notes2.txt  -> ok
  view   C:\Users\gorilla1\Documents\Gorilla.Opencode.Builds\e2e\run5\sample\notes2.txt  -> ok
```

and the file on disk read:

```
acp on 0.1.144
```

## A planning helper

On the development build of this version, asked to plan how to make a small Python file safe from shell injection. The first lines of the plan the helper returned, verbatim:

```
1. `C:\Users\gorilla1\Documents\Gorilla.Opencode.Builds\e2e\run5\sample\app.py`, lines 1 and 4-5 in function `run(cmd)`:
   Add `import shlex` at line 1. Update `run(cmd)` to eliminate `shell=True` ...
```

followed by three verification steps and a section headed `Risks and decisions for parent agent`. The record showed one call, the helper, and nothing written.

## Your own checks

Three checks were written in the separate configuration's `config.json`: one that refuses any shell command containing `git push`, one that adds a line to a log after every tool, one that adds a line at the end of every answer. The assistant was asked to have a helper edit a file and then to push. The record is shown at the top of this page. The log the two notifying checks wrote:

```
after_tool view error=0
after_tool edit error=0
after_tool view error=0
turn_end end_turn
after_tool agent error=0
turn_end end_turn
```

The refused push has no `after_tool` line because it never ran. There are two `turn_end` lines because the helper's own answer is a turn of its own.

## Every change in this version

- `gorilla-opencode acp`: the program as an Agent Client Protocol agent for editors.
- The `agent` tool takes a role: `explore`, `plan` or `coder`.
- The record under each answer lists every action a helper took.
- `hooks` in `config.json`: `before_tool` (can stop a tool), `after_tool` and `turn_end` (notify).
- The `/research` cost screen fits every window size it was tested at.
- The README has a section on editor mode; `docs/EDITOR-ROLES-HOOKS.dual-track.md` explains all of the above twice, in plain language and for developers.

## How it was tested

**The file you download is the file that was tested.** The Windows program was built once, tested, and published without being rebuilt.

**The program's own tests.** The complete set passes on Windows. The editor-mode tests pass three times in a row under Go's race detector.

**Against a real AI, with nobody watching.** The published Windows file was started from a script seven times against NVIDIA's `nemotron-3-super-120b-a12b`. All seven runs gave a usable result.

**On the published file with Gemini 3.6 Flash:** editor mode creating a file with the permission answered; a `coder` helper editing a file; a check refusing a push; the two notifying checks. Pasting a key with Ctrl+V was proven again.

**Starting up.** Timed in the home folder, pressing Enter on the NVIDIA row: 1.29, 1.29 and 1.25 seconds. One reading of 3.72 seconds was taken while the seven scenarios were running on the same machine.

**What was not tested.**

- Editor mode from Zed or a JetBrains editor. The protocol was exercised by a script, not by an editor.
- Your own checks on Linux. That part is built and checked by the compiler for Linux but has not run on a Linux computer.
- The Linux files are installed on real Linux by automatic checks that run on GitHub after this page is published. This paragraph is replaced with their result when they have run.

## No new pictures, and why

Everything in this release is text the program prints, quoted above as printed.
No new screenshot was taken. The two below are from earlier releases and show
screens this release did not change, pinned to this version.

**The normal window**, where every action is listed as it happens. This is the
permission question.

[![Gorilla OpenCode showing a Permission Required dialog for the patch port tool, naming the folder it will modify and the patch series it will apply, with the three choices Allow, Allow for session and Deny, proving the program asks before a tool changes files](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.144/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.144/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)

**The command list.**

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.144/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.144/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Three capabilities taken from a comparison with MoonshotAI/kimi-code, without its vendor channels (default-on telemetry with a device id, vendor-staged self-update, web fetch via a Moonshot server). (1) `gorilla-opencode acp`: an Agent Client Protocol agent over stdio, bridging the plain front end's four services. (2) `role` on the `agent` tool: explore, plan, coder. (3) `hooks` in config.json: before_tool (gating, fail closed), after_tool, turn_end. A real coder run exposed that the receipt hid sub-agent calls; it now nests them. The research cost dialog falls back to compact whenever the full render exceeds the terminal. Scope: 34 files changed, 3705 insertions, 91 deletions since v0.1.143.

### Known Alternatives Considered

ACP session/load, session/resume, editor-supplied MCP servers, image prompts and client fs/terminal methods are not implemented and return -32601. Role plan runs on the light helper model. The coder role excludes research as well as agent, because research spawns helpers. Other alternatives: Not available in the source material.

### Architecture Impact

New package internal/acp (jsonrpc.go, server.go) depending on narrow interfaces (SessionCreator, PermissionDesk, Runner, message Suscriber). New agent names subcoder and plan, derived in memory from coder and task when unconfigured (internal/config/subagent_roles.go). Hook runner internal/llm/agent/hooks*.go called from the single tool-execution site in agent.go. Receipt gains HelperCalls and nested lines (BuildReceiptWithHelpers); helper session id equals the agent tool-call id.

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.144" -o gorilla-opencode.exe .`. Linux: same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Windows binary sha256 cdb631d37e284f09e3cddd801c1ff546ae4f705f8d6215cb53d4f4d415ef14f6, built from the committed tree, tested, published unchanged. .deb and .rpm by nfpm.
```

### Resource Deltas

gorilla-opencode.exe: 54,721,536 bytes (0.1.143) -> 54,832,128 bytes (+110,592). Launch with Enter on the NVIDIA row: 1.29 s, 1.29 s, 1.25 s on an idle machine; 3.72 s once while seven scenarios ran concurrently. ACP read turn on Gemini 3.6 Flash: 9.1 s.

### Code Changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `internal/acp/jsonrpc.go, internal/acp/server.go, cmd/acp.go` | added | N/A - new files. | initialize (protocolVersion 1, embeddedContext true, image/audio/loadSession/MCP false, authMethods []), authenticate, session/new (absolute cwd, adopted as -c), session/prompt (agent_thought_chunk, agent_message_chunk as suffixes, tool_call with kind/location/rawInput, tool_call_update completed|failed with a 4,000-character clip, receipt as final chunk; stopReason end_turn|cancelled|max_tokens|refusal), session/cancel. session/request_permission with allow_once, allow_always, reject_once; an unanswered or cancelled question is Deny. Pending updates drained before a question so the tool call precedes it. |
| `internal/llm/agent/agent-tool.go, tools.go` | modified | One helper type: AgentTask with find + view. | role explore|plan|coder; plan adds diagnostics when an LSP client exists; coder gets the shared coder tool set minus agent and research; unknown role refused before the leash is charged; /tasks label prefixed with [role]. |
| `internal/config/subagent_roles.go, config.go, doctor.go, loadout.go` | added/modified | N/A. | AgentSubCoder (main model) and AgentPlan (helper model); derived per spawn when unconfigured; a configured entry is never touched; FollowCoderModel moves configured entries only. |
| `internal/llm/prompt/plan.go, plan.txt, prompt.go` | added/modified | N/A. | Plan prompt; subcoder maps to the coder prompt plus SubCoderNote; both get project context. |
| `internal/config/hooks.go, internal/llm/agent/hooks.go, hooks_unix.go, hooks_other.go, agent.go` | added/modified | No hooks. | before_tool runs before tool.Run; non-zero exit, timeout or failure to start returns 'Refused by the user's before_tool hook ...' with IsError and the tool is not run. after_tool after every result; turn_end once per turn. Environment GORILLA_HOOK_*; full input on stdin; input variable cut at 24 KB; result masked and cut at 8 KB; process tree killed on timeout. Not run for title or summarizer. |
| `internal/app/receipt.go, app.go` | modified | Only parent-session calls; a hook refusal would have read as a failure. | BuildReceiptWithHelpers nests a helper session's calls under its agent line with '|'; first line 'N tool calls plus M by helpers'; outcome 'refused by hook, not run'; ReceiptText exported for ACP. |
| `internal/tui/components/dialog/research.go` | modified | compact chosen by height < 40; 48 rows at 120x40 with a differing helper model; 31 rows at 90x30 after v0.1.143. | View renders, measures, and falls back to compact; compact helper-model block in two rows; two restating rows dropped on compact. |
| `README.md, docs/EDITOR-ROLES-HOOKS.dual-track.md` | modified/added | N/A. | Editor section with the Zed configuration; dual-track feature document. |

### Subsystem Changes

**TOOLS:** agent tool role parameter; hooks around every tool for coder, sub-agents and research helpers.

**TUI:** Research dialog fit.

**NETWORK:** No change. ACP is stdio only; no new destinations.

**AUTH:** No change. ACP advertises no auth methods.

**OTHER:** New acp subcommand; receipt nesting.

### Test Coverage

- **Added:** internal/acp/server_test.go (4 tests, -race x3); internal/llm/agent/agent_roles_test.go; internal/config/subagent_roles_test.go; internal/llm/prompt/role_prompts_test.go; internal/llm/agent/hooks_test.go (8 tests, real platform shell); internal/config/hooks_test.go (4 tests); internal/app/receipt_hook_test.go; internal/app/receipt_helper_test.go; research_fit_priced_test.go extended to a differing helper model at 176x48, 120x40, 100x32, 90x30, measured and unmeasured.
- **Removed:** None.
- **Notes:** Complete suite passes on Windows. Windows binary against nvidia/nemotron-3-super-120b-a12b: 7 of 7 scenarios valid. On the published binary with Gemini 3.6 Flash via Antigravity, isolated XDG configuration: ACP write turn through a client script (permission answered Allow, file written); a coder helper edit; a before_tool hook refusing 'git push origin main'; after_tool and turn_end log lines. Portal paste proven on the published binary. NOT RUN: ACP from Zed or JetBrains (client script only); hooks on Linux (vetted with GOOS=linux only).

### Security Posture

No change to permission, masking or command gates. Sub-agents with role coder hold write and shell tools; every call passes the same permission service, routed to the parent conversation, and a session-wide allow covers them. Hooks are user-authored local commands executed with the user's privileges; none ships; before_tool fails closed. ACP treats an unanswered permission question as Deny and writes nothing but protocol to stdout.

### Deployment

**Prerequisites:**
- Windows 10/11 x64, or linux/amd64.
- For editor mode: an ACP-capable editor and a provider chosen once in the terminal.

```bash
# fetch
gh release download v0.1.144 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, .deb, .rpm, SHA256SUMS-v0.1.144.txt.
# verify
sha256sum -c SHA256SUMS-v0.1.144.txt --ignore-missing
# Expected: Each present file followed by OK. Windows exe: cdb631d37e284f09e3cddd801c1ff546ae4f705f8d6215cb53d4f4d415ef14f6.
# install
.\gorilla-opencode.exe install
# Expected: Install path and shortcuts.
# verify_active
gorilla-opencode --version
# Expected: v0.1.144
# verify_active
echo '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":1}}' | gorilla-opencode acp
# Expected: One JSON line with protocolVersion 1 and agentInfo.name gorilla-opencode.
```

**Rollback:**
  1. Remove this version
     `gorilla-opencode uninstall   # or apt remove / dnf remove gorilla-opencode`
  2. Install v0.1.143 from its release
     `gh release download v0.1.143 -R gorillanobakaa-dot/Gorilla.Opencode`

### Known Issues

**[medium]** The editor shows the agent exited at once.
- Cause: No coder agent configured: the acp command refuses without a provider.
- Remedy: Run gorilla-opencode once in a terminal and choose a provider.

**[low]** Start-up fails naming hooks[N].
- Cause: Invalid event, empty command, empty tools entry or timeout outside 1-600.
- Remedy: Correct the named entry in config.json.

**[low]** A tool shows 'refused by hook, not run' unexpectedly.
- Cause: The before_tool command exited non-zero, timed out, or could not start (for example the working folder no longer exists).
- Remedy: Read the hook output in the tool result; run the command by hand with the same GORILLA_HOOK_* variables.

**[low]** Editor reports method not found.
- Cause: session/load, session/resume, MCP from the editor, or client fs/terminal methods.
- Remedy: Not implemented in this version.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Kimi Code vendor channels | 📄 stated in input | Its web fetch goes through a Moonshot server first and falls back to local |
| ACP write turn on the published file | 📄 stated in input | the file was written containing "acp on 0.1.144" |
| ACP read turn 9.1 s | 📄 stated in input | a read turn answered in 9.1 seconds |
| Coder excludes helpers of its own | 📄 stated in input | it cannot start helpers of its own |
| Receipt hid helper calls | 📄 stated in input | the receipt under the answer said only "agent ... -> ok" |
| Hook refusal on the published file | 📄 stated in input | bash   git push origin main  -> refused by hook, not run |
| Research dialog 48 rows at 120x40 | 📄 stated in input | the cost screen was 48 rows tall, so in a 120-by-40 window |
| Binary hash and size | 📄 stated in input | sha256 cdb631d37e284f09e3cddd801c1ff546ae4f705f8d6215cb53d4f4d415ef14f6, 54,832,128 bytes |
| Scenarios | 📄 stated in input | All seven gave valid results |
| Launch times | 📄 stated in input | 1.29, 1.29 and 1.25 seconds |
| Not tested from Zed; hooks not run on Linux | 📄 stated in input | The protocol was exercised by the client script, not by an editor |
| Scope | 📄 stated in input | 34 files changed, 3705 insertions, 91 deletions since v0.1.143 |
| Session-wide allow covers sub-agents | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*
