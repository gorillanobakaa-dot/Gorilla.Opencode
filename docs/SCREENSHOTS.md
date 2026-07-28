# Gorilla OpenCode — screenshots & proof

Real screenshots from a Debian 13 / GNOME 48 machine running the revived
OpenCode on an NVIDIA NIM key. **New here? Read the plain-English
[GUIDE.md](GUIDE.md)** — it explains every part of the screen, the
menus, and the not-obvious ← → arrow trick to switch to the Google
models.

Every screenshot is unscaled, at whatever size the window was captured
(1583–1600 px wide), so the terminal text stays readable — the complete
set is in [`screenshots/gallery/`](screenshots/gallery/).

## v0.1.46 — two interfaces, one program

Gorilla OpenCode has **two front ends**, and these four shots are the same
machine running both at once. Left window: **plain mode** — every line is
ordinary terminal output, so `Ctrl+A` / `Ctrl+Shift+C` copies a five-hour
session into a text editor. Right window: the **full interface**, with the
sidebar, live token counter and running cost.

You do not need a command line flag for either. Plain mode is a **right-click
action on the desktop icon** ("Plain mode (copyable output)"); launching
normally gives the full interface.

### Plain mode and the full interface, side by side

Plain mode announces what it is and what it costs you: the folder, the model,
and `showing: 4 on, 0 off — thinking is ON and uses extra tokens`. Both windows
carry **timestamps** (`19:47:08`) and both show the model's **thinking** before
its answer — the display toggles added in v0.1.44, on by default except the one
that actually costs money.

[![Plain mode and the full interface running side by side](screenshots/gallery/v0146-plain-and-tui-thinking.png)](screenshots/gallery/v0146-plain-and-tui-thinking.png)

### `/help` in plain mode, and the sidebar's live accounting

Plain mode carries a deliberately smaller command set and says so —
*"Anything not listed here needs the full interface."* On the right, the sidebar
accounts for the session with no guessing: `Input 9.1K / Output 61`, `MCP: no
MCP servers`, `LSP: all 9 off (/context to change)`, `Modified Files: none`.

[![The plain-mode command list beside the full interface sidebar](screenshots/gallery/v0146-plain-help-and-sidebar.png)](screenshots/gallery/v0146-plain-help-and-sidebar.png)

### Both interfaces answering the same question

A non-coding question, asked of a local model served over an OpenAI-compatible
endpoint (MiniMax M3). Plain mode's answer is four lines; the full interface's
is a structured comparison with the elapsed time attached — `MiniMax M3 (2.4m)`.
Same program, same key, two ways to read the result.

[![The same question answered in plain mode and the full interface](screenshots/gallery/v0146-plain-and-tui.png)](screenshots/gallery/v0146-plain-and-tui.png)

### What showing the thinking also shows you — an honest caveat

Turn the reasoning display on and you see everything the model reasoned,
including the parts it should have kept to itself. In the shot above, MiniMax M3
quotes **our own instructions back at the user**, mangled:

> `Per my conduct guidelines: "match answer: simple question gets direct sentence."`

That string is a garbled paraphrase of a line in our prompt. No prompt text is
secret — [`system-prompts/`](../system-prompts/) is in this repository — but it
is worth knowing that a reasoning window is a window into the *whole* request,
not a curated summary. That is the trade the `/extras` toggle exists to let you
make deliberately, in either direction.

[![The full interface mid-generation, thinking visible](screenshots/gallery/v0146-tui-generating.png)](screenshots/gallery/v0146-tui-generating.png)

## v0.1.42 — the release the screenshots caught bugs in

Every shot below is 1600x900, unscaled. **Click any image for full resolution** —
terminal screenshots become unreadable the moment they are downscaled to fit.

Two of these fixes exist *because* of screenshots. The `/help` list was hiding
whichever command the cursor sat on, and the sign-in box was printing an
un-clearable URL across the interface. Neither was caught by a test; both were
obvious the moment someone photographed the screen.

### `/help` — every command, in plain language

Grouped by what you are trying to do rather than alphabetically, because someone
who does not know a command's name cannot look it up alphabetically. The selected
row's full explanation shows in place, and `/` searches the descriptions as well
as the names.

[![The /help command reference](screenshots/gallery/v0142-help.png)](screenshots/gallery/v0142-help.png)

This is also the fix: the highlighted row used to render as a blank line, because
the row's highlight background was being overwritten with the panel background,
leaving dark text on dark. Whichever command you were reading about was the one
that vanished.

### The sign-in box — dismissible, and a URL you can actually read

[![The Google sign-in overlay](screenshots/gallery/v0142-login-overlay.png)](screenshots/gallery/v0142-login-overlay.png)

This used to be five `fmt.Println` calls straight to the terminal while the
interface was drawing to the same screen — so the URL was painted over the top
with no record of it in the renderer, and **no redraw could ever remove it**. It
stayed for the rest of the session. It is now part of the frame: `esc` hides it,
and sign-in carries on regardless.

### `/context` — what every message costs, and the bulk LSP switch

[![The context loadout menu](screenshots/gallery/v0142-context-loadout.png)](screenshots/gallery/v0142-context-loadout.png)

Note `L all LSPs` in the footer. Nine configured language servers meant nine
separate toggles to get a quiet session. Also fixed here: the **first** press on
any of those rows used to do nothing at all, because an absent entry reads as
"enabled" everywhere else but the toggle was flipping the map's zero value.

### The sidebar tells the truth about language servers

[![The sidebar showing all 9 language servers off](screenshots/gallery/v0142-sidebar-lsp-off.png)](screenshots/gallery/v0142-sidebar-lsp-off.png)

`all 9 off (/context to change)`. This panel used to list every configured server
whether or not it was running, which reads exactly like a switch that does
nothing. They were being disabled the whole time — measured: with all nine off,
**zero** language-server processes start; with them on, one clangd, two gopls and
five Node servers. The panel was the liar, not the switch.

### `/settings` — every option, what it accepts, its default

[![The settings list](screenshots/gallery/v0142-settings.png)](screenshots/gallery/v0142-settings.png)

Including *Ask which folder to work in at startup*, which exists because clicking
the desktop icon used to start the agent in your home folder — on this machine,
over a million files in scope before typing anything.

### `/connect` — accounts that coexist

[![The connect dialog](screenshots/gallery/v0142-connect.png)](screenshots/gallery/v0142-connect.png)

Adding one never disables another. NVIDIA NIM and a local Ollama can both be live;
`/model` then labels each model with the connection serving it. In v0.1.41 those
models registered successfully and were **unselectable**, because the picker built
its provider list from saved accounts and environment keys, and a local endpoint is
neither.


## The model picker, full width (v0.1.16)

118 models, each with a capability description, sorted best-for-coding
first, with a position counter. Up/down moves through models; **left/
right switches provider**.

![Model picker](screenshots/gallery/10-09-02-16.png)

## Reaching the Google (Gemini) models — press the → arrow

Your models are grouped by provider. Press **→** in the picker to page
from NVIDIA to **"Select Gemini Model"** — the `1/4 →` at the bottom
shows you're on the Gemini page. Bottom-left shows the context down to
**6.9K** (it used to be ~15K).

![Gemini model page](screenshots/gallery/15-09-12-23.png)

## One tool, every provider — the latest models (v0.1.30)

The same terminal, the same key-per-provider setup, reaching four different
backends. Each picker is ranked best-for-coding first, with plain capability
notes, and you page between providers with the **← →** arrows.

**NVIDIA NIM** — the full ranked catalog (DeepSeek V4 Pro #1, GLM 5.2, MiniMax M3,
Nemotron…), 118 models deep, best-coder-first with a position counter.

![NVIDIA NIM model picker](screenshots/gallery/v0130-picker-nim.png)

**Google Gemini** — the current lineup, Gemini 2.0 through **3.6 Flash** (1M
context, the newest workhorse), plus the rolling `latest` aliases.

![Gemini model picker](screenshots/gallery/v0130-picker-gemini.png)

**Groq** — DeepSeek-R1-Distill-Llama-70B, Llama 4 Maverick / Scout, Qwen QwQ,
Llama 3.3 70B, at Groq's signature speed.

![Groq model picker](screenshots/gallery/v0130-picker-groq.png)

**Cerebras** — GLM 4.7 on wafer-scale silicon, GPT-OSS 120B, Gemma 4 31B —
"extremely fast" inference.

![Cerebras model picker](screenshots/gallery/v0130-picker-cerebras.png)

## The context loadout & Gorilla controls — every token (and dollar) accounted for

`/context` shows exactly what's sent to the model every turn — with its **token
*and* dollar cost** — and lets you switch any of it off. The top **🦍 GORILLA
CONTROLS** section adds two arrow-key dials: an **AI-server request pace-setter**
(requests/min, to stay under free-tier limits) and a **GORILLA AGENTS/SUBAGENTS
leash** (cap helper agents down to the ☢ Nuclear Option). Full write-up:
[CONTROL-AND-COST.md](CONTROL-AND-COST.md); how-to in the
[GUIDE](GUIDE.md#the-context-loadout-context--your-token-budget).

![The context loadout with Gorilla controls](screenshots/02-context-loadout.png)

---

*The design draws on published research — sources cited in
[system-prompts/RESEARCH-SOURCES.md](../system-prompts/RESEARCH-SOURCES.md).*
