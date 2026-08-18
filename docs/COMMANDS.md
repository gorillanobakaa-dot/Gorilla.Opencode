# Every command, and what it does

Generated from `internal/commands/registry.go` by `go run ./cmd/commands-doc`.
Do not edit by hand — a test compares this file against the registry, so a
command cannot be added without appearing here.

You can read the same list inside the program with **`/help`**, where the
explanation for whichever command you have highlighted appears underneath it.
Type a command by starting a message with `/`.

**Nothing here costs money except where it says so.** Two commands spend real
tokens on your behalf — `/research` and `/osint` — and both say what they will
cost before they start. Everything else is local.

## At a glance

| Type this | What happens |
|---|---|
| `/clear` · `/new` | Start a fresh conversation. |
| `/plain` · `/copy` `/copyable` | Switch to the interface you can select and copy. |
| `/resume` · `/continue` `/handoff` | Pick up work that stopped, or work another model started. |
| `/sessions` · `/history` | Every past conversation: search, reopen, save, erase. |
| `/export` | Save this conversation to a file. |
| `/compact` · `/summarize` `/summarise` | Squeeze the conversation down so it keeps working. |
| `/cd [folder]` | Switch to working in one folder. |
| `/add-dir` · `/adddir` `/dirs` `/roots` | Work in more than one folder at once. |
| `/init` | Write the project notes file the AI reads first. |
| `/model` · `/models` | Choose which AI answers you. |
| `/connect` · `/connections` | Add or manage your AI accounts and keys. |
| `/providers` · `/provider` `/switch` | Switch to a different AI provider. |
| `/login` | Sign in with your Google account. |
| `/logout` | Sign out of your Google account. |
| `/usage` | Show your quota and balances — how many bananas are left. |
| `/context` · `/loadout` `/tokens` | Turn features off to spend less. |
| `/settings` · `/config` `/prefs` | Every option, what it accepts, and its default. |
| `/prompts` · `/prompt` | Read or change the AI's standing instructions. |
| `/reset` · `/defaults` | Put things back the way they shipped. |
| `/research <question>` | Send helper agents to investigate, each on one angle. |
| `/osint <question>` · `/dossier` | The serious one. Professional dossier. Burns real money. |
| `/review [--quick|--security|--full] [--diff REF] [folder]` · `/audit` `/codereview` | Run 30 real analysers over your code and report honestly. |
| `/yolo` · `/auto` `/autopilot` `/goal` | Approve everything for this conversation. No more prompts. |
| `/tasks` · `/task` `/agents` `/kill` | See and stop background helpers. |
| `/help` · `/commands` `/?` | This list. |

## Your conversation

### `/clear`

*Also: `/new`*

**Start a fresh conversation.**

The AI forgets everything said so far. Use this when you move to a different task — a long conversation costs more on every message, because the whole history is sent each time.

### `/plain`

*Also: `/copy`, `/copyable`*

**Switch to the interface you can select and copy.**

This interface draws on a screen your terminal keeps no history of, which is why Ctrl+A selects nothing here. Plain mode writes ordinary text instead, so you can select, copy and search the whole conversation with your terminal's own keys. It has fewer commands. This takes effect next time you start the program — the current screen is already running. Switch back in /settings, or right-click the desktop icon for a one-off.

### `/resume`

*Also: `/continue`, `/handoff`*

**Pick up work that stopped, or work another model started.**

For when the job is not finished: the power went, the connection dropped, the model ran out of room, or you want a different model to take over. It opens the same list as /sessions — press Ctrl+R on the one you want.

This is NOT the same as reopening the conversation. Reopening loads every message back in, which is right for a short chat and wrong for a long job — and a long job is exactly what gets interrupted. Putting a thousand messages back into a small model is what stopped the work in the first place.

Instead it writes a short brief and starts a FRESH conversation with just that: everything you asked for, word for word, in order; which files were changed and which commands were run; what went wrong; and where it stopped. The brief is built by the program itself, so it costs nothing and cannot fail the way the original did.

It also says plainly what it does NOT know — whether any of the work was correct, and whether it was finished. That matters most when you hand it to a different model, which has no way to tell a finished job from an abandoned one and would otherwise assume the best.

### `/sessions`

*Also: `/history`*

**Every past conversation: search, reopen, save, erase.**

The one to reach for when a session ended without you — the power went, the connection dropped, the machine was closed. It lists every conversation you have ever had, newest first, with the date, how many messages, and how much space it is taking up.

Type to search. It looks inside the messages as well as the titles, because titles are generated and are often useless weeks later — you remember the error you were chasing, not what the summary called that day.

Enter reopens a conversation exactly where it stopped. Ctrl+E saves it to a file — the whole thing: every message with its time, the model's reasoning, and every tool it ran with the result that came back, including the failures. That is what lets you work out afterwards how something ended up published, or deleted.

Ctrl+D erases one for good, along with the helper sessions it spawned, and returns the space to your disk — really returns it, and tells you how much came back. Deleting alone frees nothing on this kind of database; the file only shrinks when it is rebuilt, which is why this reports the actual before-and-after. Ctrl+S sorts by size, so the conversations worth deleting are the ones at the top.

### `/export`

**Save this conversation to a file.**

Asks you which folder and what to call it, then writes the whole session out as text: every message with its date and time, how far into the session it happened, which model answered, the model's reasoning, and every tool it ran with the result that came back — including the ones that failed. Use it when you need to know exactly what happened and when.

This one saves the conversation you are in, and lets you name the file. To save a conversation you have LEFT — after a power cut, or from last week — use /sessions instead, which can also reach the helper sessions a research run spawned.

### `/compact`

*Also: `/summarize`, `/summarise`*

**Squeeze the conversation down so it keeps working.**

Every message you send carries the whole conversation with it, and each model has a limit on how much it can hold. Approach that limit and answers get worse, then stop. This writes a summary of everything so far and continues from that instead, so the thread survives while the bulk goes.

Use it when a long session starts to drift, or before starting a big job in an old conversation. It costs one model call to write the summary. Models with small windows need this often — the status bar shows how full you are. It also runs by itself at 95% full if you leave that setting on in /settings.

## Which files the AI can see

### `/cd [folder]`

**Switch to working in one folder.**

This is the important one. The AI searches and reads inside your working folder, so pointing it at one project instead of your whole home folder is the difference between a handful of files and a million. Fewer files means faster answers and far less of your quota spent. Typing it with no folder opens a chooser.

### `/add-dir`

*Also: `/adddir`, `/dirs`, `/roots`*

**Work in more than one folder at once.**

Adds a second (or third) folder alongside your main one — useful when a change spans two projects. Each folder you add is more for the AI to search, so add only what you need. To move to a single folder instead of adding one, use /cd.

### `/init`

**Write the project notes file the AI reads first.**

Looks through the project and writes a short file describing how to build, test and work in it, in the house style of this codebase. Every future conversation in this folder reads that file before anything else, so the AI starts knowing your conventions instead of guessing at them. Run it once per project, and again after big changes.

## Models and accounts

### `/model`

*Also: `/models`*

**Choose which AI answers you.**

Bigger models are better at hard problems and cost more; small ones are cheap and fast. Models running on your own machine cost nothing to use. Each is listed with the connection it comes from.

### `/connect`

*Also: `/connections`*

**Add or manage your AI accounts and keys.**

Where you paste an API key, add a local server such as Ollama or NVIDIA, or turn a connection off without deleting it. Adding a connection makes its models appear in /model. The list shows the servers you have added as well as the ones on offer; press d to remove one of yours for good, or space to just switch it off.

### `/providers`

*Also: `/provider`, `/switch`*

**Switch to a different AI provider.**

Reopens the same picker you saw when the app started, with the free options marked. Use it when the provider you chose does not work — a key refused, a model not included in your plan — instead of quitting and starting again. Esc leaves everything as it is.

### `/login`

**Sign in with your Google account.**

Opens your browser. Lets you use Google's models through your account instead of pasting an API key.

### `/logout`

**Sign out of your Google account.**

Removes the stored sign-in. Any API keys you typed are untouched.

### `/usage`

**Show your quota and balances — how many bananas are left.**

Shows what you have left to spend, in plain words. If you signed in with the Antigravity free tier: how much of your weekly allowance remains — Gemini has a separate pool from Claude and GPT-OSS — and when each resets. If you have a DeepSeek or OpenRouter key: your remaining balance there too. A one-line summary also appears on its own at the start of each session.

## Cost, speed and behaviour

### `/context`

*Also: `/loadout`, `/tokens`*

**Turn features off to spend less.**

Everything the AI can do is described to it on every single message, and you pay for that description each time. Switching off what you are not using makes every message cheaper. This is also where you turn language servers off — press L for all of them at once, or pick them one by one.

### `/settings`

*Also: `/config`, `/prefs`*

**Every option, what it accepts, and its default.**

One list of every setting with a plain-language description, the range it accepts and what it shipped as, so you can always get back to a known state.

### `/prompts`

*Also: `/prompt`*

**Read or change the AI's standing instructions.**

The instructions the AI is given before it sees your message — how careful to be, how to report what it did. You can switch sections off or rewrite them. Advanced: changing these changes how the AI behaves everywhere.

### `/reset`

*Also: `/defaults`*

**Put things back the way they shipped.**

Undoes your changes, in whichever area you pick — settings, instructions, or feature switches. Use this when something is behaving oddly and you no longer remember what you changed.

## Background helpers

### `/research <question>`

**Send helper agents to investigate, each on one angle.**

The everyday investigation tool: four to ten helpers, each given ONE angle, collecting with the same intelligence-cycle discipline as /osint but in a single pass. A verifier attacks their conclusions. Each helper is a full model session, so the dialog shows the cost before anything starts. Worth it when being wrong is expensive; waste when a single search would answer. For the full professional dossier — rounds, graded sources, the works — see /osint.

### `/osint <question>`

*Also: `/dossier`*

**The serious one. Professional dossier. Burns real money.**

A professional intelligence assessment, not a chat answer: plans your question into sub-questions, collects from hundreds of free primary sources (scholarly APIs, SEC filings, World Bank, humanitarian data, global news), grades every claim on two axes like a real intelligence shop, hunts its own gaps, and tells you plainly what it could NOT establish. OFF by default — arm it in /context. Every run starts with a warning showing the burn rate in money, because 4-10 helpers is 4-10 full model sessions. Type /osint alone for the full explanation page.

/osint --recover writes up a run that collected its findings but never produced the dossier — the usual outcome when a connection drops or the model runs out of room at the very last step. It costs nothing to look: the findings are already on disk and in the local store, and it lists every past run so you can pick one. The write-up happens in a fresh conversation carrying only those findings, which is exactly why it succeeds where the original run ran out of room. Nothing is collected again and no helpers are sent out.

### `/review [--quick|--security|--full] [--diff REF] [folder]`

*Also: `/audit`, `/codereview`*

**Run 30 real analysers over your code and report honestly.**

A professional static-analysis and security review, built in. Point it at a folder, a file, or your changes and it runs around thirty real analysers — the ones that find memory errors, injection, leaked secrets, unchecked errors — picking whichever suit the languages actually present. C, C++, Go, Python, JavaScript, TypeScript, Rust, shell and more.

With no arguments it reviews your current folder. Add a path for somewhere else. The tools live inside the program; nothing is downloaded when you run it.

**How deep:**
  /review                    the normal pass — fast checks and static analysis, and it escalates to the deep security tools ON ITS OWN for any file that looks security-shaped. This is usually the one you want.
  /review --quick            linters and formatters only, seconds. Skips the security stages entirely, and says so.
  /review --security         forces the deep security pass over everything and reports only security findings.
  /review --full             every stage over every file.
  /review --diff HEAD        only what you changed. Add a ref for something else: --diff origin/main.

These combine, and the order does not matter: /review --security --diff HEAD internal/auth

**It tells you what did NOT run.** That is the part that matters. Those thirty analysers have to be installed on your machine, and if they are missing they simply find nothing — which looks exactly like a clean report. So the answer always starts with which tools ran, which are missing, and which failed; and if none of them are installed it refuses to run at all rather than hand you a reassuring blank.

It also flags every line that two or more DIFFERENT tools complained about independently. Those are the ones worth reading first.

What it cannot do: find wrong logic, a broken assumption, or an error quietly ignored. No static tool can. This is half a review and it says so — the AI still has to read the code, and should tell you it did.

### `/yolo`

*Also: `/auto`, `/autopilot`, `/goal`*

**Approve everything for this conversation. No more prompts.**

Normally the program stops and asks before it edits a file, runs a command, or reaches the internet. This turns that off for the conversation you are in: every tool call is approved automatically, including every research helper — which is the point, because a ten-helper run otherwise asks you the same question ten times.

What you are handing over: file edits, shell commands and web access, unattended. Use it when you have told the agent to get on with a job and you do not want to babysit it. Do not use it in a folder you cannot afford to have changed.

It lasts only as long as this conversation and is never written to disk, so it cannot silently follow you into tomorrow. Type /yolo again to turn it off. /tasks still stops helpers at any time.

### `/tasks`

*Also: `/task`, `/agents`, `/kill`*

**See and stop background helpers.**

The AI can start helpers to work on parts of a job. Each one costs quota of its own, so this is where you check what is running and stop anything you do not want.

## Help

### `/help`

*Also: `/commands`, `/?`*

**This list.**

Every command, what it does, and what it costs or changes.

---

*This page is generated. If a command is missing from it, that is a bug in
the program rather than in the documentation — the registry and this file are
held together by a test.*
