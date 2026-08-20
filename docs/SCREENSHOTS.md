<!-- Version: 1.3.0 · updated 26-08-20-14-23 -->
# Gorilla OpenCode — screenshots & proof

Real screenshots from a Debian 13 / GNOME 48 machine running the revived
OpenCode — the newest set on the free Antigravity tier (no API key, no
card), the older sets on an NVIDIA NIM key. **New here? Read the plain-English
[GUIDE.md](GUIDE.md)** — it explains every part of the screen, the
menus, and the not-obvious ← → arrow trick to switch to the Google
models.

Every screenshot is unscaled, at whatever size the window was captured
(1583–1600 px wide), so the terminal text stays readable — the complete
set is in [`screenshots/gallery/`](screenshots/gallery/).

## How these are published (the rule)

**Full resolution, never thumbnails.** Every image below is uploaded at the size
it was captured (~1595 px wide) and rendered as wide as the page allows, and
every one is **clickable through to the full-resolution original**. Nothing is
downscaled, cropped for tidiness, or re-encoded to save bytes. The reason is the
philosophy, not vanity: this project's claim is that a non-technical reader can
CHECK what was built, and a screenshot too small to read destroys exactly that.
Nothing is staged — the numbers on screen are real runs.

---

## v0.1.96 — when the link fails, it fails cheaply and says so

Captured 2026-08-18 on Debian 13 / GNOME 48, model **Llama 3.3 70B**, over a
**simulated bad satellite link** (a local proxy throttling to 2 KB/s, or dropping
the connection). Both are real runs — the numbers on screen are the program's own.

### A slow model is not a hung one — the screen says which

While waiting for a slow endpoint, the indicator counts (`Generating… (12s)`),
and once the wait crosses the point where a free model is usually just warming
up, a notice says so in the status bar. You can tell "warming up" from "stuck"
without guessing.

[![The waiting indicator counting "Generating… (12s)" on a slow link, with the status bar reading "Still waiting on the model — a quiet endpoint is usually warming up, not stuck. First reply…"](screenshots/gallery/v0196-cold-start-toast.png)](screenshots/gallery/v0196-cold-start-toast.png)

### When the link drops, it stops — and tells you what it spent

A dropping connection used to retry silently and upload a megabyte for one
unanswered question. Now the turn stops at a budget counted in the bytes you pay
for, and says exactly what it cost, rather than failing in silence.

[![The upload-budget stop after a dropping link: "stopped after 8 attempts: this turn had already uploaded 307 KB of the 307 KB it is allowed, and the connection kept failing. Nothing further was sent. On a slow or metered link each retry re-uploads the whole conversation, so retrying forever costs real money for no answer."](screenshots/gallery/v0196-upload-budget-error.png)](screenshots/gallery/v0196-upload-budget-error.png)

---

## v0.1.90 — `/osint`, the all-source assessment

Captured 2026-08-17 on Debian 13 / GNOME 48, model **DeepSeek V4 Flash**.

### The capability page — `/osint` with no question

Typing `/osint` on its own opens the full-screen explanation instead of
starting anything. Status line first: armed or off, and how to change it.

[![The /osint capability page, ARMED, explaining the five stages of a run: plan, collect from 985 sources, vet on two axes, gap round, product](screenshots/gallery/v0190-osint-page-armed.png)](screenshots/gallery/v0190-osint-page-armed.png)

*(Captured on the build immediately before the rename to "All-Source
Intelligence Analysis" — the page now carries the new title, the content is
otherwise as shown.)*

### The same page, scrolled: the iron rules, privacy, and the cost

Where the finished assessment is written — **outside** the working folder,
because working folders are git repositories and a private question must never
land in a commit — and what a run actually costs, in red, unhedged.

[![The lower half of the page: iron rules, the dossier written to ~/Documents/Gorilla-OSINT-Dossiers, and the cost warning in red](screenshots/gallery/v0190-osint-page-cost-and-privacy.png)](screenshots/gallery/v0190-osint-page-cost-and-privacy.png)

### The model weighing whether the question is worth the money

Unstaged, and the most revealing shot here. The user demanded a full 10-agent
supervised run on a frivolous question; the model's own reasoning is visible,
arguing with itself about waste — *"Running a full 10-agent dossier on this is a
waste of money"* — while still respecting an explicit instruction. The cost
discipline is not a slogan in a document; it is in the model's reasoning.

[![The model's thinking, visible on screen, weighing an expensive run against a frivolous question](screenshots/gallery/v0190-osint-model-weighs-the-spend.png)](screenshots/gallery/v0190-osint-model-weighs-the-spend.png)

### Eight helpers running, each in its own lane — and killable

Every helper is named by the lane it owns: adversary, prior art, primary source,
requirement, local, history, sidestep, cost. `X` kills all of them. You are
never locked out of your own spending.

[![The /tasks monitor: eight research helpers running, each labelled with its lane, with kill and nuclear-kill keys](screenshots/gallery/v0190-osint-eight-helpers-running.png)](screenshots/gallery/v0190-osint-eight-helpers-running.png)

### Permission asked before a helper reaches the web

A helper searching medical sources stops and asks first. The query it wants to
send is shown in full, so you approve the actual text, not a category.

[![The permission dialog showing the exact medical search query a helper wants to run, with allow / allow-for-session / deny](screenshots/gallery/v0190-osint-permission-medical-search.png)](screenshots/gallery/v0190-osint-permission-medical-search.png)

---

## v0.1.80 — the banana meter, from full barrel to empty peel

All shots below are one real morning (2026-08-11) on the Antigravity free
tier: a heavy agent session burned the weekly Claude/GPT pool from 100% to
0% before 8am, and the meter narrated every step of the descent. Nothing is
staged — the percentages fall because the work was real.

### `/usage` — the panel

Each model group gets a thermometer bar (red at the left end, green at the
right) that shrinks from the green end as the week burns. Both numbers are
spelled out — *"71% left, 29% used · resets in 2d"* — because "96%" alone
forces you to guess which direction it runs. Paid providers with a real
balance endpoint appear below: OpenRouter here, truthfully reporting a
free-tier key as "no credits purchased — free models only".

[![The quota panel, healthy: 100% and 71%, triple bananas](screenshots/gallery/v0180-usage-panel-healthy.png)](screenshots/gallery/v0180-usage-panel-healthy.png)

### The descent

The gorilla's verdicts escalate as the barrel empties — the exact numbers
always printed right next to the mood:

[![59% left — and the one-line quota reading echoed on the status bar](screenshots/gallery/v0180-usage-59pct-footer-toast.png)](screenshots/gallery/v0180-usage-59pct-footer-toast.png)

[![30% left: "Yeah... just a few bananas left."](screenshots/gallery/v0180-usage-30pct-few-bananas.png)](screenshots/gallery/v0180-usage-30pct-few-bananas.png)

[![9% left: "Banana emergency! Scraping the peel..."](screenshots/gallery/v0180-usage-9pct-emergency.png)](screenshots/gallery/v0180-usage-9pct-emergency.png)

[![5% left: "Last banana spotted. Nobody make any sudden prompts."](screenshots/gallery/v0180-usage-5pct-last-banana.png)](screenshots/gallery/v0180-usage-5pct-last-banana.png)

[![4% left — the red sliver](screenshots/gallery/v0180-usage-4pct.png)](screenshots/gallery/v0180-usage-4pct.png)

[![0%: "Zero bananas. Even the peel is gone."](screenshots/gallery/v0180-usage-zero-bananas.png)](screenshots/gallery/v0180-usage-zero-bananas.png)

[![The empty barrel, clean shot](screenshots/gallery/v0180-usage-zero-clean.png)](screenshots/gallery/v0180-usage-zero-clean.png)

### Live crossing alerts — you don't have to ask

After each response the meter is re-checked quietly (one small request every
half minute at most, only for signed-in Antigravity users). The moment a
group crosses a banana threshold, the crossing is announced: timestamped in
the scrollback with the gorilla, echoed on the status bar without him (the
status bar is redrawn in place, where emoji width quirks can smear a frame —
so the words travel, the gorilla stays in the history).

[![The status bar and scrollback announcing: "Last banana spotted. Nobody make any sudden prompts. — Claude and GPT models: 5% left"](screenshots/gallery/v0180-alert-last-banana-footer.png)](screenshots/gallery/v0180-alert-last-banana-footer.png)

[![The same alert arriving mid-session, between tool calls](screenshots/gallery/v0180-alert-last-banana-session.png)](screenshots/gallery/v0180-alert-last-banana-session.png)

[![07:42:41 — the zero-crossing announced live: "Zero bananas. Even the peel is gone."](screenshots/gallery/v0180-alert-zero-bananas-live.png)](screenshots/gallery/v0180-alert-zero-bananas-live.png)

### The agent at work

The hero shot from the README: quota panel above, a challenge below, and the
agent reaching for `lynx` — the 641 KB text browser that ships as a hard
dependency — to research job listings with no API key and no card.

[![The agent running lynx-powered web research, quota panel above](screenshots/gallery/v0180-agent-at-work.png)](screenshots/gallery/v0180-agent-at-work.png)

Every file write asks first, with a full diff — here the agent proposing a
`profile.json` template (placeholders, not invented personal data):

[![The permission dialog: tool, path, and the full diff of what would be written](screenshots/gallery/v0180-permission-dialog.png)](screenshots/gallery/v0180-permission-dialog.png)

### The dialogs, as of v0.1.80

[![The provider portal: Antigravity free tier, Code Assist, NIM, Ollama, Cloudflare, and the API-key providers — free paths first](screenshots/gallery/v0180-provider-portal.png)](screenshots/gallery/v0180-provider-portal.png)

[![/help — commands grouped by what you are trying to do, including "/usage — how many bananas are left"](screenshots/gallery/v0180-help.png)](screenshots/gallery/v0180-help.png)

[![/context — the Gorilla controls: request pace-setter, agent leash, and every tool priced in tokens](screenshots/gallery/v0180-context-loadout.png)](screenshots/gallery/v0180-context-loadout.png)

[![/settings — every option, what it accepts, and its default, with plain-English consequences](screenshots/gallery/v0180-settings.png)](screenshots/gallery/v0180-settings.png)

[![/prompts — the system prompts the AI is given, with token counts and a trustworthy reset](screenshots/gallery/v0180-prompts.png)](screenshots/gallery/v0180-prompts.png)

[![The coder prompt split into sections — each behavioural block can be switched off, with an honest warning about what that costs you](screenshots/gallery/v0180-prompt-sections.png)](screenshots/gallery/v0180-prompt-sections.png)

[![The summarizer, task and title prompts — small, and all visible](screenshots/gallery/v0180-prompts-summarizer.png)](screenshots/gallery/v0180-prompts-summarizer.png)

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

# v0.1.103 — /arsenal, and lines that behave

*Captured 19 August 2026 on the machine this project is built for: a Sony VAIO
SVE, 1600x900, Debian 13. Unscaled, uncropped, click any image for the
full-resolution original. The numbers on screen are real measurements taken at
that moment, not mock-ups.*

## /arsenal — what the agent can do here, and what the rest would cost

The feature exists because of a capability that was sitting unused: a model was
asked to read a screenshot and said it could not read images, while tesseract
was installed on the same machine and had been for months. Nobody stumbles onto
`binwalk` or `sleuthkit` or `ssdeep`, and you cannot ask for a thing you do not
know exists. The barrier was never bandwidth. It was the map.

The header is measured on open, never claimed — **25 capabilities present, 8
not**, and the package manager it actually found.

[![The /arsenal series list showing 25 of 33 capabilities present on this machine, with a measured cost of 97.9 MB to download and 331.0 MB on disk for the two selected, about 3.4 hours at 8 KB per second](screenshots/gallery/v0103-arsenal-series-measured-cost.png)](screenshots/gallery/v0103-arsenal-series-measured-cost.png)

Every key says what it did. Pressing space on a group that is already fully
installed used to do nothing and say nothing, which is indistinguishable from a
broken key — the line at the bottom is the fix.

[![The /arsenal page reporting "All 8 of these are already installed - nothing to select" after space was pressed on a fully installed group](screenshots/gallery/v0103-arsenal-already-installed.png)](screenshots/gallery/v0103-arsenal-already-installed.png)

**The install plan is the whole argument on one screen.** The costs come from
your own package manager, counted against what is already on your disk — so
they are true for your machine rather than a worst case from a table. And
nothing is installed by this program: it prints the command and gets out of the
way.

[![The /arsenal install plan listing 2 capabilities, 4 packages, 97.9 MB to download and 331.0 MB on disk, the exact apt-get command, and the line "This program will not run it and will never ask for your password"](screenshots/gallery/v0103-arsenal-install-plan.png)](screenshots/gallery/v0103-arsenal-install-plan.png)

## Lines that behave — the rule that used to draw itself twice

A tester reported continuous rules rendering doubled, stacked and misaligned.
The cause was rules typed at a fixed length: 56 columns of box-drawing chosen to
look right in one window, too long in any narrower one, wrapping into two
physical rows each. Three of them on one screen meant three rows of debris at
once.

They are ASCII now, and **sized to the box at render time rather than typed** —
the `-- WHAT THIS COSTS ----` and `-- QUOTA ----` rules below span the frame
exactly, one row each, at any width.

[![The /research cost dialog in supervised mode showing ASCII rules spanning the frame exactly, with $0.192 per minute, $0.431 for the run, and 18 sessions across 10 helpers and 8 auditors](screenshots/gallery/v0103-research-supervised-cost-ascii-rules.png)](screenshots/gallery/v0103-research-supervised-cost-ascii-rules.png)

The same screen in parallel mode, priced for four helpers. Note what it tells
you without being asked: which model the helpers will actually run on, that it
is not the one you are chatting with, and that parallel buys time and not
answers.

[![The /research cost dialog in parallel mode showing $0.128 per minute, about $0.0958 for the run, and the note that parallel is the same token cost as sequential](screenshots/gallery/v0103-research-parallel-cost-ascii-rules.png)](screenshots/gallery/v0103-research-parallel-cost-ascii-rules.png)

## /osint — the page that explains itself before it spends anything

[![The /osint capability page explaining the five-stage intelligence cycle, the 985-source registry of which 866 are free and 370 need no account, and the two-axis A-F and 1-6 grading system](screenshots/gallery/v0103-osint-capability-page.png)](screenshots/gallery/v0103-osint-capability-page.png)

## It declined to burn money on a joke

Not a designed feature, and worth being precise about. The research tool's
description carries the arithmetic — *"6 helpers is still 6 LLM sessions"*,
*"supervised roughly doubles that again"*, *"over-spawning is the main way this
tool wastes the user's money"* — so the model has the numbers before it chooses.

Here it was pointed at a deliberately silly question with **10 agents,
supervised**, and told *"the user chose these and they decide what this costs"*.
It refused, priced the refusal, answered the question for free anyway, and asked
what the real work was.

[![The agent declining a 10-agent supervised research run, explaining it would be roughly 20 LLM sessions and would waste the user money, then answering the question directly for free](screenshots/gallery/v0103-refuses-expensive-run-on-a-joke.png)](screenshots/gallery/v0103-refuses-expensive-run-on-a-joke.png)

**Read the caveat before quoting this.** One run, one model (Deepseek V4 Flash),
one question. The number 20 is ours — it is not inferable without the doubling
rule in our schema — but the decision to refuse under an explicit instruction to
comply is the model's, and it has not been tested across models. The full split
of what is the program and what is the model is in
[CONTROL-AND-COST.md](CONTROL-AND-COST.md).


---

## v0.1.108 — the connection profile picker

The program is built for people on satellite phone links and weak mobile
signals. Everything needed to survive a bad connection already existed and had
been measured; none of it could be found, because every setting was an
environment variable. These five profiles put them on one screen.

You are asked once, on first run, and after that only if your measured speed
disagrees with your chosen profile by two rungs or more. One rung is inside the
noise of a single measurement and must never interrupt you.

[![The connection picker with Austere environment selected, showing the 1-9 KB/s band, Iridium Short Burst and 2G examples, and the full plain-language explanation of why the typing effect is switched off including the 22,256 against 834 byte measurement](screenshots/gallery/v0108-connection-picker-austere-explains-non-streaming.png)](screenshots/gallery/v0108-connection-picker-austere-explains-non-streaming.png)

Austere is the reason the feature exists. Note what the screen does **not** do:
it has no measurement yet on first run, so it says *"Nothing measured yet, so
there is no suggestion below"* instead of inventing a recommendation. Nothing is
ever downloaded to test the line — at 2 KB/s a 100 KB probe costs about 50
seconds of a metered allowance to report what the user already knows, and a
one-to-two-second satellite round trip would make it measure delay rather than
speed.

[![The connection picker with Constrained selected, showing the 10-60 KB/s band with Iridium Certus and EDGE examples, an 8 minute wait limit, 1.5 MB per message and 3 retries](screenshots/gallery/v0108-connection-picker-constrained-profile.png)](screenshots/gallery/v0108-connection-picker-constrained-profile.png)

[![The connection picker with Modest selected and marked as current, the shipped default covering 60-250 KB/s with Inmarsat BGAN and UMTS examples, 4 minute wait and 4 MB per message](screenshots/gallery/v0108-connection-picker-modest-current-default.png)](screenshots/gallery/v0108-connection-picker-modest-current-default.png)

Modest is the shipped default, and the cursor starts on whichever profile is
already current — so Enter always means "keep what I have" for someone who
opened this by accident.

[![The connection picker with Unconstrained selected for 5 MB/s and up, showing Starlink and 5G examples, a 60 second wait limit, and text confirming answers still arrive a word at a time on fast links](screenshots/gallery/v0108-connection-picker-unconstrained-keeps-live-typing.png)](screenshots/gallery/v0108-connection-picker-unconstrained-keeps-live-typing.png)

The two slowest profiles receive the answer in one piece rather than word by
word. Measured on the same reply: **22,256 bytes streamed against 834 bytes in
one piece**, with **identical token accounting** — 106 either way, read from
each response's own usage block. The AI bill does not move; the data bill does.
The fast profiles keep live typing, and every row says which behaviour it has
and why.

[![Gorilla OpenCode v0.1.108 running a web search tool call against Llama 3.3 70B, showing the search results block with untrusted content markers and the model composing its follow-up](screenshots/gallery/v0108-agent-tool-call-working-on-v0108.png)](screenshots/gallery/v0108-agent-tool-call-working-on-v0108.png)

**Read the caveat before quoting this.** These profiles have never run on a
genuinely slow connection — they were built and tested on a fast one, against
measurements taken on 2026-08-18 with a deliberately broken link
([SATELLITE.md](SATELLITE.md)). The byte counts are real and repeatable; the
choice of timeout values for each band is judgement, not measurement. The retry ceiling is
now driven by the profile — two attempts on Austere, five on a fast line.

---

*The design draws on published research — sources cited in
[system-prompts/RESEARCH-SOURCES.md](../system-prompts/RESEARCH-SOURCES.md).*
