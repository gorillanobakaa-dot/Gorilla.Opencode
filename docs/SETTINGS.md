# Settings reference

Generated from `internal/config/settings.go` by `go run ./cmd/settings-doc`.
Do not edit by hand — a test compares this file against the registry.

Open the same list in the app with **`/settings`**, where every row shows its
current value, what it accepts, and its default.

## Conversation

### Auto-summarise long chats

What happens when a conversation grows too long to fit in the AI's memory.

- **ON:** the program automatically summarises the older part and carries on
- **OFF:** you get an error when the chat is full and must start a new one with /clear yourself
- **Setting:** `autoCompact`
- **Type:** on/off
- **Accepts:** on or off
- **Default:** `ON`

### Longest single AI reply

The most the AI may write in one reply. Higher lets it produce bigger files in one go, but each reply can cost more and take longer.

- **Setting:** `agents.coder.maxTokens`
- **Type:** number
- **Accepts:** 512-32768
- **Default:** `4096`
- **Unit:** tokens

### Summariser reply length

How much room the summariser gets when it compresses an old conversation.

- **Setting:** `agents.summarizer.maxTokens`
- **Type:** number
- **Accepts:** 512-32768
- **Default:** `4096`
- **Unit:** tokens

### Helper AI reply length

How much room a helper sub-agent gets for its answer.

- **Setting:** `agents.task.maxTokens`
- **Type:** number
- **Accepts:** 512-32768
- **Default:** `4096`
- **Unit:** tokens

### Session title length

How long a generated session name may be.

- **Setting:** `agents.title.maxTokens`
- **Type:** number
- **Accepts:** 80-80
- **Default:** `80`
- **Unit:** tokens
- **Read-only:** fixed at 80 — titles must be one short line, and the program overwrites this on every launch

### Thinking effort

How long the AI may think before answering. Only some models support this; on the rest it is ignored. Higher is slower but usually better on hard bugs.

- **Setting:** `agents.coder.reasoningEffort`
- **Type:** choice
- **Accepts:** low / medium / high
- **Default:** `medium`

## Network and pace

### Requests per minute

How fast this program is allowed to talk to the AI provider. Free accounts cut you off if you go too fast. Lower is slower but avoids cut-offs. 0 means no limit at all.

- **Setting:** `ratelimit.rpm`
- **Type:** preset
- **Accepts:** 0=unlimited 40 35 30 25 20 15 12 10 8 6 5 4 3 2
- **Default:** `25`
- **Unit:** requests/min

### Helper AIs allowed

How many helper AIs the main AI may start. Each helper is a whole extra conversation you pay for. 0 switches helpers off completely; -1 means no limit.

- **Setting:** `subagents.max`
- **Type:** preset
- **Accepts:** -1=unlimited 100 50 25 12 6 3 1 0=off
- **Default:** `-1`

## Files and shell

### Shell used for commands

Which shell runs the commands the AI asks for. Change this only if your shell lives somewhere unusual — a wrong value breaks every command the AI tries to run.

- **Setting:** `shell.path`
- **Type:** text
- **Accepts:** text
- **Default:** `/bin/bash`

### Shell flags

Extra flags for that shell. -l makes it a login shell, so your PATH and aliases are loaded.

- **Setting:** `shell.args`
- **Type:** list
- **Accepts:** comma-separated list
- **Default:** `-l`

### Project instruction files

Filenames the program looks for in each of your folders and feeds to the AI as project instructions. Removing entries means less is sent every turn, but the AI then knows less about your conventions.

- **Setting:** `contextPaths`
- **Type:** list
- **Accepts:** comma-separated list
- **Default:** `.github/copilot-instructions.md, .cursorrules, .cursor/rules/, CLAUDE.md, CLAUDE.local.md, opencode.md, opencode.local.md, OpenCode.md, OpenCode.local.md, OPENCODE.md, OPENCODE.local.md`

### Ask which folder to work in at startup

Whether the program asks you to pick a working folder each time it starts.

- **ON:** you pick the folder on launch, so clicking the desktop icon does not scope the AI to your whole home folder
- **OFF:** it starts silently in the folder you last used, and you change it with /cd
- **Setting:** `askWorkspaceOnStartup`
- **Type:** on/off
- **Accepts:** on or off
- **Default:** `ON`

### Program data folder

Folder inside your project where this program keeps its database and logs.

- **Setting:** `data.directory`
- **Type:** text
- **Accepts:** text
- **Default:** `.opencode`
- **Takes effect:** next launch

## Appearance

### Colour scheme

How the program looks. Cosmetic only — changes nothing about what the AI does or costs.

- **Setting:** `tui.theme`
- **Type:** choice
- **Accepts:** (no options registered)
- **Default:** `opencode`

## Diagnostics

### Debug logging

Extra logging to help diagnose problems with this program itself.

- **ON:** writes a detailed log; it can get large
- **OFF:** normal logging only
- **Setting:** `debug`
- **Type:** on/off
- **Accepts:** on or off
- **Default:** `OFF`
- **Takes effect:** next launch

### Language-server logging

Whether the raw chatter from language servers is shown live in your terminal.

- **ON:** very noisy, and it will interleave with the interface — only for chasing a language-server bug
- **OFF:** that chatter goes to the log file instead of over the top of the interface
- **Setting:** `debugLSP`
- **Type:** on/off
- **Accepts:** on or off
- **Default:** `OFF`

## Owned by other commands

These are deliberately not in `/settings`, so there is one owner per setting:

- **AI model** — which model each agent uses. Use `/model`.
- **Provider keys and endpoints** — API keys, local endpoints, Google login. Use `/connect`.
- **Tools and prompt sections** — what rides every turn, with token costs. Use `/context`.
- **System prompt text** — edit or reset the prompts themselves. Use `/prompts`.
- **Workspace roots** — which directories are part of the workspace. Use `/add-dir`.

