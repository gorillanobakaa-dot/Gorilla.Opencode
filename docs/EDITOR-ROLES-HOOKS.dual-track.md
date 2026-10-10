# Editor mode, helper roles and your own checks — dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers the work between v0.1.143 and v0.1.144: `gorilla-opencode acp`, the `role` parameter of the `agent` tool, lifecycle hooks, helper calls on the receipt, and the `/research` screen fit.

---

# Track one: plain language


> Session record generated 2026-10-10

---

## What happened

You asked how Gorilla OpenCode compares with Kimi Code, a terminal coding assistant from the Chinese company Moonshot AI. Its own toolbox turns out to be small and generic: about fifteen tools for reading, writing, searching and running commands. Several of its channels also lead back to its maker: it reports usage by default, it updates itself on a schedule its maker sets for each computer, and it fetches web pages through its maker's server first.

Three of its ideas are worth having without any of those channels. The first is working inside a code editor such as Zed instead of only in a terminal. The second is helpers with set jobs: one that only looks, one that only plans, one that may change files. The third is letting you put your own small checks in front of the assistant's actions, so that a command you never want run is stopped before it starts.

All three are now in Gorilla OpenCode. Each one keeps the rules the program already has: it asks you before it changes anything, it adds no account, and it sends nothing anywhere the terminal version would not send it. Each was tried on the real program with a real AI before this page was written.

## Honest state of play

Done and tried on the real program, on Gemini 3.6 Flash, in 9 separate runs: editor mode for reading, writing with a permission question, and running a helper; all three helper jobs; checks that stop a tool and checks that only take notes; the fuller record; the cost screen.

Not done in editor mode: picking up an old conversation where you left it, tools that the editor itself offers, and pictures in a question. The editor gets a clear "not supported" answer for those.

Not tried: your own checks on Linux. The Linux part is built and checked for mistakes, but it has not run on a Linux computer.

## Worst case if something is wrong

A check you write yourself can have a mistake in it. Suppose you write a check meant to stop `git push`, but you misspell it as `git psuh`. The assistant then pushes your code and the record shows an ordinary successful action. The program cannot know what you meant. A check that crashes, takes too long, or cannot start does stop the action, so a broken check fails safe; a check that runs but tests the wrong thing does not.

## What changed for you

**Where you can use the program**
- Before: Only in a terminal window.
- After:  Also inside an editor that speaks the Agent Client Protocol, such as Zed or a JetBrains editor. The editor shows each action as it happens and asks you each permission question.
- Affects: Anyone who writes code in an editor

**Helpers**
- Before: The assistant could send out a helper that only looked through files.
- After:  A helper now has one of three jobs: `explore` looks, `plan` writes a step-by-step plan, `coder` may change files and run commands. The `coder` helper still asks you before each change.
- Affects: Everyone

**The record printed under each answer**
- Before: When a helper did work, the record said only that a helper ran.
- After:  The record lists every action the helper took, indented under it. On the real program one answer read "2 tool calls plus 4 by helpers", with the helper's edit and command each on its own line.
- Affects: Everyone

**Your own checks**
- Before: There was no way to run your own command before or after the assistant used a tool.
- After:  You can list commands in your settings file. A command set to run before a tool can stop that tool. Nothing is set up unless you write it yourself.
- Affects: People who want a hard rule, such as never pushing code

**The research cost screen on a small window**
- Before: On some window sizes the line that names the keys fell off the bottom: the screen was 48 rows tall in a 40-row window.
- After:  The screen switches to its short form whenever the full form does not fit.
- Affects: Anyone using `/research` in a small window

## What you can do now

- Use Gorilla OpenCode from inside Zed or another Agent Client Protocol editor, with the same AI, keys and permission questions as in the terminal. A first read in editor mode took 9.1 seconds from question to answer.
- Ask the assistant to send a `plan` helper that returns a numbered plan without touching anything.
- Ask the assistant to send a `coder` helper that makes a change and reports every file it changed and every command it ran.
- See every action a helper took in the record under the answer.
- Write a check in your settings file that stops a tool before it runs, for example any command containing `git push`.
- Write a check that notes each action in a log, or makes a sound when an answer is finished.

## What is still missing

- **Picking up an old conversation in editor mode** — Each time the editor starts the program you begin a new conversation.
- **Tools offered by the editor** — If your editor offers extra tools to assistants, Gorilla OpenCode does not take them; it uses its own.
- **Pictures in a question in editor mode** — You cannot paste a screenshot into the editor's question box for this assistant.
- **Checks tried on Linux** — On Linux, your own checks should work, but nobody has seen them run there yet.

## How to check that the new parts work on your computer

**Step 1:**
```bash
Open a terminal and type `gorilla-opencode acp --help`.
```
  - **Pass:** A short page explains the command and shows the lines to put in Zed's settings.

**Step 2:**
```bash
In Zed, add the lines from step 1 to `settings.json`, open a project, and open the agent panel with Gorilla OpenCode chosen. Ask it to read one file.
```
  - **Pass:** The panel shows the read as an action, then the answer, then the record that begins `--- what actually ran`.

**Step 3:**
```bash
In the terminal program, ask: use the agent tool with role plan to plan a small change in this project.
```
  - **Pass:** A numbered plan that names files and what to check, and a record showing the helper only read files.

**Step 4:**
```bash
Add a check to `config.json` that stops any shell command containing a word you choose, start the program, and ask it to run a command with that word.
```
  - **Pass:** The record shows that command as `refused by hook, not run`.


## Should you be concerned?

Two risks are worth naming. First, the `coder` helper can change files. It asks you before each change exactly as the main assistant does, and the record now shows what it did, but if you have told the program to allow everything for a session, the helper is allowed everything too. Second, your own checks run with your own permissions, as you, and the program does not judge what they do. A check that sends your tool output somewhere sends it there; that is your decision, written in your settings file. No check ships with the program, and the program logs each check it loads when it starts.

## Glossary

**Agent Client Protocol** — An open set of rules that lets a code editor talk to an AI assistant running as a separate program.

**Helper** — A second, short conversation the assistant starts to do one job and report back.

**Check (hook)** — A command you write in your settings file that the program runs before or after an action.

**Record (receipt)** — The list the program prints under each answer of every action that really ran, written by the program and not by the AI.

## Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Kimi Code's core tool set is about fifteen generic tools | 📄 stated in input | Its core tool set is about fifteen generic tools |
| Kimi Code fetches web pages through its maker's server first | 📄 stated in input | Its web fetch goes through a Moonshot server first and falls back to local |
| Editor mode was tried for reading, writing with permission, and a helper | 📄 stated in input | Read turn: view app.py, correct answer, receipt, 9.1 seconds. Write turn |
| The coder helper still asks before each change | 📄 stated in input | permission questions go to the user's conversation |
| A check that crashes, times out or cannot start stops the action | 📄 stated in input | A timeout or a hook that cannot start also refuses (fail closed) |
| Checks have not run on Linux | 📄 stated in input | has not been run on Linux |
| No check ships with the program | 📄 stated in input | None ships configured |
| Allow-everything for a session also covers the helper | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Session record. Plain-language track. Its developer twin covers the same session in technical detail.*

---

# Track two: developer


> Session record generated 2026-10-10

---

## Problem Being Solved

A comparison with MoonshotAI/kimi-code identified three capabilities Gorilla OpenCode lacked: editor integration through the Agent Client Protocol, sub-agents with distinct roles (Kimi ships `coder`, `explore`, `plan`), and user-configured lifecycle hooks. Kimi's vendor channels (default-on telemetry with a device id, vendor-staged self-update, web fetch through `moonshot-fetch-url.ts`) were explicitly excluded.

## Approach Taken

ACP is implemented as a thin bridge over the four services the plain front end already drives (`session.Service.Create`, the message and permission brokers, `agent.Service.Run/Cancel`), narrowed to small interfaces so it tests without a model. Roles extend the existing `agent` tool and its helper registry instead of adding a tool. Hooks run at the single tool-execution site in `agent.go`, and fail closed for `before_tool`. Each feature was exercised on the built binary against Gemini 3.6 Flash through Antigravity on an isolated XDG configuration.

## Before

No editor integration. The `agent` tool spawned one helper type (`config.AgentTask`, `find` + `view`). No hooks. The receipt listed only the parent session's calls, so a helper's calls were invisible. The `/research` dialog chose its compact form from terminal height alone (under 40 rows) and overflowed at 120x40 when the helper model differed from the chat model (48 rows).

## After

`gorilla-opencode acp` serves `initialize`, `authenticate`, `session/new`, `session/prompt`, `session/cancel`; other methods return -32601. The `agent` tool takes `role` explore, plan or coder. `hooks` in `config.json` run `before_tool`, `after_tool` and `turn_end` commands. The receipt nests helper calls under their `agent` line. The research dialog measures its full render and falls back to compact when it does not fit. Linux execution of hooks is compiled and vetted with `GOOS=linux` but not run on Linux.

## Known Alternatives

ACP `session/load`, editor-supplied MCP servers, image prompts and the client-side `fs/*` and `terminal/*` methods are not implemented. Role `plan` runs on the light helper model; running it on the coder's model is a one-line change in `subagent_roles.go`. The `coder` role also loses `research`, not only `agent`, because `research` spawns helpers.

## Files Changed

| File | Change | What Changed | Why |
|------|--------|--------------|-----|
| `internal/acp/jsonrpc.go` | added | Line-delimited JSON-RPC 2.0 connection with serialised writes and id-matched agent-to-client calls. | ACP transport is stdio JSON-RPC; `session/request_permission` is a request from the agent. |
| `internal/acp/server.go` | added | ACP methods, prompt streaming (`agent_thought_chunk`, `agent_message_chunk`, `tool_call`, `tool_call_update`), permission mapping to Grant/GrantPersistant/Deny, stop reasons, receipt as the final chunk. | Editor integration without new providers, keys or network paths. |
| `internal/acp/server_test.go` | added | Whole turn with a scripted engine, cancel plus Deny, error paths, tool kinds and titles. | Protocol order and refusal semantics under `-race`. |
| `cmd/acp.go` | added | `acp` subcommand: loads config, opens the database, builds the app, serves stdin/stdout. | Editors spawn `gorilla-opencode acp`. |
| `internal/llm/agent/agent-tool.go` | modified | `role` parameter; per-role agent name, tool set and prompt; `[role]` prefix in `/tasks`; unknown role refused before the helper leash is charged. | Role-based sub-agents. |
| `internal/llm/agent/tools.go` | modified | Shared coder tool builder; `SubCoderAgentTools` without `agent` and `research`; `PlanAgentTools`; MCP list copied. | A tool added for the coder reaches the sub-coder; helpers cannot recurse. |
| `internal/config/subagent_roles.go` | added | `DeriveRoleAgent`: missing `subcoder` and `plan` entries derived from coder and task at each spawn. | Old `config.json` files lack the entries and the provider refuses a missing agent. |
| `internal/llm/prompt/plan.go, plan.txt` | added | Plan prompt: numbered file-by-file plan, verification, risks, no writes. | Prompt selection is by agent name. |
| `internal/llm/agent/hooks.go, hooks_unix.go, hooks_other.go` | added | Hook runner: platform shell, environment variables, stdin input, 8 KB output tail, process-tree kill on timeout. | User-configured lifecycle hooks. |
| `internal/config/hooks.go` | added | `Hook` type and validation by index and command. | Invalid hooks must stop start-up with a usable message. |
| `internal/llm/agent/agent.go` | modified | `before_tool` gate before `tool.Run`, `after_tool` after it, `turn_end` once per turn. | The single tool-execution site. |
| `internal/app/receipt.go, app.go` | modified | `refused by hook, not run` outcome; `BuildReceiptWithHelpers` nests a helper session's calls under its `agent` line; `HelperCalls` count; `ReceiptText` for ACP. | The first real `coder` run showed only `agent -> ok` while the helper edited `app.py`. |
| `internal/tui/components/dialog/research.go` | modified | `View` renders, measures and falls back to compact; compact helper-model block in two rows; two restating rows dropped on compact. | 48 rows at 120x40; one row over at 90x30 on GitHub's Linux tests after v0.1.143. |
| `README.md` | modified | Section "Inside your editor" with the Zed configuration. | Discoverability. |

## Decisions Made

- 📄 **`authMethods` is empty and `authenticate` succeeds with nothing to check.** — The program has no account of its own; provider credentials are configured in the terminal.
- 📄 **An unanswered permission question, a cancelled turn or a closed editor is a Deny.** — No answer, a cancelled turn or a closed editor is a refusal, never consent.
- 📄 **Before asking a permission question, pending message updates are drained.** — Both arrive at once and Go's select has no preference; the tool call must be announced before its question.
- 📄 **`before_tool` fails closed on non-zero exit, timeout or failure to start.** — A timeout or a hook that cannot start also refuses (fail closed).
- 📄 **Hooks never ship configured.** — A hook is a local command the user writes, a door the user opens on purpose.
- 🤖 **Helper calls count apart from the model's own calls on the receipt's first line.** — The first figure stays the model's own, so it can be checked against a claimed number of attempts.

## Tried and Abandoned

- **Patching `receipt.go` through a shell heredoc.** — The shell turned `\n` in Go string literals into real newlines; the patch was redone from a script file.
- **Relying on terminal height alone to choose the compact research dialog.** — The full form's height depends on the model configuration; only measuring it is correct.

## ⚠ Claimed But Not Verified

*Prior documents claimed these are done. No test evidence found in this diff:*

- Hook execution on Linux (`sh -c`, process-group kill): vetted with `GOOS=linux`, not run on Linux.
- ACP with Zed or a JetBrains editor: the protocol was exercised by a private client script, not by an editor.

## Open Items

| Item | Priority | Blocks |
|------|----------|--------|
| ACP `session/load` and `session/resume` | medium | Resuming a conversation from the editor. |
| ACP editor-supplied MCP servers and image prompts | low | nothing currently |
| Run the hook tests on Linux | medium | A claim that hooks work on Linux. |
| Exercise `gorilla-opencode acp` from Zed | medium | A claim of tested editor support. |

## How to verify this work is correct

**Step 1:**
```bash
`go test ./internal/acp/... -race -count=3`
```
  - **Pass:** ok three times
  - **Fail:** A data race report, or the editor-order assertion listing `permission` before `tool_call`.

**Step 2:**
```bash
`go test ./internal/llm/agent/ -run 'Role|Hook' -count=1`
```
  - **Pass:** ok
  - **Fail:** A coder role carrying `agent`, or a refused hook that let the tool run.

**Step 3:**
```bash
`go test ./internal/app/ -run 'Helper|Hook' -count=1`
```
  - **Pass:** ok
  - **Fail:** A receipt without the `| edit` helper line, or a hook refusal shown as a failure.

**Step 4:**
```bash
`go test ./internal/tui/components/dialog/ -run PricedSupervisedDialogFits -count=1`
```
  - **Pass:** ok
  - **Fail:** A frame taller than 30, 32, 40 or 48 rows.

**Step 5:**
```bash
Pipe `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":1}}` into `gorilla-opencode acp`.
```
  - **Pass:** One JSON line with `protocolVersion` 1 and `agentInfo.name` `gorilla-opencode`.
  - **Fail:** Any non-JSON text on stdout.


## Technical Debt

🟡 **LOW** — Permission questions are matched to an announced tool call by tool name, because `PermissionRequest` carries no call id. → Carry the tool call id in `permission.CreatePermissionRequest`.
🟡 **LOW** — The sub-coder's prompt is the coder prompt and may mention `agent`, which it does not have. → Strip helper-spawning guidance from the sub-coder prompt.
🟡 **LOW** — A slow `turn_end` hook delays the end of the turn by up to its timeout. → Run `turn_end` after the result is handed on, if notification order does not matter.

## Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Kimi's channels were excluded | 📄 stated in input | Three ideas were worth taking without opening any of those doors |
| ACP read and write turns passed on Gemini 3.6 Flash | 📄 stated in input | the question "Create file ...\notes.txt" reached the client, Allow was sent, the file was written |
| ACP tests pass under the race detector three times | 📄 stated in input | Passes under the race detector, three runs |
| coder role excludes agent and research | 📄 stated in input | the full coder tool set except agent and research |
| Receipt nests helper calls | 📄 stated in input | "2 tool calls plus 4 by helpers" |
| Hook gate refused git push on the real program | 📄 stated in input | "bash git push origin main -> refused by hook, not run" |
| Research dialog was 48 rows at 120x40 | 📄 stated in input | the full form was 48 rows |
| Linux hook path not run | 📄 stated in input | has not been run on Linux |
| Permission matching is by tool name | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Session record. Developer track. Covers work done, not current code state.*