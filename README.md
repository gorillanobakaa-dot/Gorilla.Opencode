<p align="center"><img src="internal/assets/icons/gorilla-opencode-256.png" width="128" alt="Gorilla OpenCode"></p>

# Gorilla OpenCode

**The original OpenCode, revived.** A terminal AI coding agent — MIT
licensed, no telemetry, no accounts, no vendor funnel. Bring your own
API keys or run models on your own machine.

![Gorilla OpenCode at work: the quota meter, the banana ladder, and the agent running lynx-powered web research on the Antigravity free tier](docs/screenshots/gallery/v0180-agent-at-work.png)

> New to a tool like this? A plain-English walkthrough of the screen
> and menus (incl. how to reach the Google models): **[docs/GUIDE.md](docs/GUIDE.md)**.
> Every command, with what it does and what it costs: **[docs/COMMANDS.md](docs/COMMANDS.md)**.
> Lost a session to a power cut, or run out of disk? **[docs/SESSIONS-AND-STORAGE.md](docs/SESSIONS-AND-STORAGE.md)**.
> Thirty static-analysis and security tools, built in: **[docs/CODE-REVIEW.md](docs/CODE-REVIEW.md)**.
> Will it run on my machine? Measured RAM, disk and network: **[docs/FOOTPRINT.md](docs/FOOTPRINT.md)**.
>
> What happens when the connection drops, in plain language: **[docs/SATELLITE.md](docs/SATELLITE.md)**.
> Screenshots & proof: **[docs/SCREENSHOTS.md](docs/SCREENSHOTS.md)**.
>
> **⏱️ Why does an AI model feel slow?** A free, hands-on lesson built from
> our own measurements — what "tokens per second" means, why a 550-billion
> model can beat a 70-billion one, and a 60-line script so you can prove it
> yourself: **[docs/BENCHMARKS.md](docs/BENCHMARKS.md)**.
>
> **🔒 Does it phone home?** No — and you don't have to trust us. A
> reproducible network audit (`ss`/`tshark`/`strace`) proving it connects
> only to the provider you choose: **[SECURITY.md](SECURITY.md)**.
>
> **💸 What am I actually paying for — and how do I control it?** A free,
> plain-English lesson on how AI tools bill you by the token, why agents multiply
> requests, and exactly what our cost/pace/agent controls do (with the source
> files so you can recreate them): **[docs/CONTROL-AND-COST.md](docs/CONTROL-AND-COST.md)**.
>
> **🤖 OpenAI models with no API key — a *free* ChatGPT account is enough.**
> An API key needs a developer account, which needs a card. Signing in with the
> ChatGPT account you already have does not. No per-use charge at all; go over
> your plan's allowance and you are paused, never billed:
> **[docs/CHATGPT-LOGIN.md](docs/CHATGPT-LOGIN.md)**.
>
> **🔑 Free Gemini with no API key.** Sign in with your Gmail
> (`gorilla-opencode login`) to use Google's Code Assist free tier — the same
> login Gemini CLI/Antigravity use, so your quota lasts: **[docs/GOOGLE-LOGIN.md](docs/GOOGLE-LOGIN.md)**.
>
> **🔐 "Why did GitHub block our push over a secret that isn't secret?"** A
> real story from building this, turned into a lesson on OAuth logins, client
> secrets, and telling a real leak from a false alarm: **[docs/CLIENT-SECRETS-EXPLAINED.md](docs/CLIENT-SECRETS-EXPLAINED.md)**.
>
> The design draws on published research; we cite our sources so you can
> read them and judge for yourself: **[system-prompts/RESEARCH-SOURCES.md](system-prompts/RESEARCH-SOURCES.md)**.

> **Provenance, stated plainly:** this is the original Go OpenCode by
> [Kujtim Hoxha](https://github.com/kujtimiihoxha), archived in 2025
> when its development continued as [Crush](https://github.com/charmbracelet/crush)
> under Charm (FSL license). It is unrelated to
> [SST's opencode](https://github.com/sst/opencode), which reuses the
> name. This fork revives the archived MIT original — the fossil the
> living species evolved from — and keeps it working with
> the AI providers of 2026. The full reasoning, and everything that was
> changed, is documented for both humans and developers in
> [DOCUMENTATION.dual-track.md](Changelogs/DOCUMENTATION.dual-track.md), per this
> project's [Open Source Philosophy](PHILOSOPHY.md).

## Who this is built for

**Old, slow, cheap hardware. Bad internet. No credit card.**

That is the whole design brief. Gorilla OpenCode is built for people running
2012 laptops on connections measured in single-digit kilobytes per second —
often young, often with no money for subscriptions, often on a machine someone
else threw away. The same is true of every Gorilla project: the
[kernel](https://github.com/gorillanobakaa-dot/debian-kernel),
the [Firefox fork](https://github.com/gorillanobakaa-dot/firefox-154), this.

It is not charity and it is not a stripped-down edition. It is the actual
target, and it shapes everything:

- **The download is kept small** because 18 MB at 8 KB/s is forty minutes of
  your life. Release builds are stripped for exactly this reason.
- **You can see what every turn costs you**, before you spend it — `/context`
  prices every piece of what gets sent, and you can switch any of it off.
- **The free paths come first.** Sign in with a Gmail and use Google's free
  tier with no API key at all; run models on your own machine with Ollama; use
  free models where they exist. Paid keys work, but they are never the only door.
- **No accounts, no telemetry, no vendor funnel.** Don't take our word for it —
  [SECURITY.md](SECURITY.md) is a reproducible network audit you can run yourself.
- **Everything is explained twice**, once for developers and once in plain
  English, because software you cannot verify is software you have to trust.

If you have a fast machine and a good connection, it works fine there too. It
just was not built with you in mind first.

## Copying a whole session

The normal interface draws in the terminal's *alternate* screen, which has no
scrollback — so `Ctrl+A` has nothing to select. That is how alternate screens work,
not something that can be fixed inside the interface.

For a session you can select, copy and search with your terminal's own keys, pick
whichever suits how you start the program:

- **Clicking the icon?** Right-click it and choose **Plain mode (selectable and
  copyable)**. No typing.
- **Want it every time?** `/settings` → *Which interface to start* → `plain`. The
  choice sticks however you launch, including from the icon.
- **Already inside?** Type `/plain`. It applies next launch — the current screen is
  already running.
- **From a shell, once:** `gorilla-opencode --plain`

The flag is only one of four routes on purpose: the desktop entry runs
`gorilla-opencode launch` with no arguments, so a mode reachable only by typing a
flag would be a mode most people never get.

Plain mode has no panels and no redraws. Every byte is ordinary terminal output.
It carries a smaller set of commands (`/help` lists them); anything else needs the
full interface. `/export` works in both and writes the complete record —
timestamps, reasoning, tool calls and their results.

## Built to work where the internet barely does

This is the part that matters most to us, so it goes near the top.

Most AI tools are written and tested on fat office fibre, by people who have
never watched a page load one line at a time. So they quietly assume the network
is always fast, always there. Point one of them at a **satellite phone uplink
pushing a few kilobytes a second** — a mountainside in Afghanistan, a village
deep in the Nigerian bush, a fishing boat, a disaster zone, a refugee camp — and
it falls over. Which means it fails *exactly* the people who have the least and
could use the help the most: not only service members deployed somewhere austere,
but the kids for whom a crawling, dropping, few-KB/s connection isn't an
emergency — it's just **Tuesday**.

If you've never heard a dial-up modem screech its handshake, never rationed
megabytes to the end of the month, never sat watching a single reply trickle in —
you don't think to check whether your software survives on a bad line. We did.
So we went looking, and we found two bugs that made Gorilla give up on a weak
connection — **and we didn't write either of them. We inherited them** from the
upstream code, where they'd sat unfixed because the low-bandwidth path was never
walked and nothing ever ran the tests (`git blame` receipts and the full story
are in [`Errors.in.the.code.txt`](Errors.in.the.code.txt)).

What we fixed (plain-language):

- **It never hung up the phone.** Every question to the AI opens a connection —
  like dialling a number. The app answered, got its reply, and set the receiver
  down *without hanging up*. On city broadband you'd never notice. But a real
  task asks dozens of questions, so the open lines stacked up — 1, 2, … 46 —
  until the provider (NVIDIA NIM) said *"you're using all your lines"* and refused
  to talk. The whole session died with a cryptic `ResourceExhausted` error. Now we
  hang up after every call.
- **It assumed a fast line, and quit at the first sign of trouble.** It would
  redial from scratch constantly (brutal when a single handshake takes *seconds*
  over satellite), abort a slow reply the instant a timer expired, and give up the
  moment the server said *"busy, try later"*. Now it **keeps one line warm and
  reuses it**, **never hangs up on a slow answer**, prefers the connection-frugal
  HTTP/2, honours a satellite terminal's proxy, and when the server says *"busy"*
  it **waits a beat and tries again** instead of quitting.

**What it means for you:** on a connection most modern apps refuse to even
attempt, you can hold a real back-and-forth with an AI and actually get your
problem solved — fix the code, understand the error, draft the message, learn the
thing — from the edge of the map, on the kind of link the rest of the industry
forgot exists. That was the whole point.

*(Fixed in v0.1.32–v0.1.33. Full technical write-up:
[`Changelogs/`](Changelogs/); every bug and its provenance:
[`Errors.in.the.code.txt`](Errors.in.the.code.txt).)*

## Why this exists

An AI coding tool is billed by the **token** — the small chunks of text it sends
to and from the model. Nearly every cost you pay, and every free-tier limit you
hit, comes down to *how much* text is sent, *how often*, and *how many times* the
agent loops or spawns helper agents to finish a job. Most tools keep all of that
under the hood. This one puts it in your hands, in one menu (`/context`):

- **See the bill.** Every block of context sent each turn is listed with its
  token cost *and* its dollar cost at your model's real price — so "what does it
  cost just to say *yo*?" has a number, not a shrug. Free/flat tiers show `$0.00`
  honestly rather than a fake estimate.
- **Set the request pace.** A user-adjustable speed limit spaces calls to the
  provider so you glide *under* an undocumented, moving free-tier ceiling (NVIDIA
  NIM advertises only "up to 40/min") instead of slamming into it and triggering
  retry storms.
- **Hold the leash on agents.** The main agent can spawn helper agents that each
  run their own request loop — fine on a paid plan, punishing on a metered one.
  One dial caps how many it may spawn, from unlimited down to the **🦍 Gorilla
  Nuclear Option**: all agents/subagents off, main agent works solo, fewest
  possible calls.
- **Strip the loadout.** Every tool and prompt block is a switch; turn off what
  you don't need and its cost leaves every future turn.

None of this is exotic — it's a few hundred lines. It's *uncommon* because the
incentives usually run the other way: a service **paid by the token** has little
reason to ship you dials whose whole purpose is to send fewer of them. That's not
a claim about anyone's motives — just the shape of the incentive. This project's
bias is the opposite one, stated plainly in its [Philosophy](PHILOSOPHY.md):
**measure it, show the user, and give them the switch.** Your key, your machine,
your call — every turn.

### The token sieve: we don't send you the whole page

The same bias decides how the web tools work. An API can only bill you for what
you send it, so we send the least that answers the question. One arXiv abstract
page, measured:

| what gets sent | ~tokens | vs raw |
|---|---|---|
| raw HTML, as "URL context" features send it | ~10,744 | 100% |
| converted and stripped locally | ~1,794 | 16.7% |
| via the arXiv export API | ~734 | 6.8% |
| the abstract alone | **~328** | **3.1%** |

Thirty-two times less, for strictly more useful content — the rest was
navigation, cookie banner and tracking scripts, billed to you and then re-billed
on every later turn of the conversation.

`web_fetch` therefore walks a ladder and reports which rung it used: a
structured API first, then the source document (`Accept: text/markdown`, a
`raw.githubusercontent.com` rewrite, a `.md` companion), then local HTML
conversion, and it says plainly when it has handed you a reconstruction rather
than a source.

This matters most where it is least visible. On fibre with a corporate card it
is a rounding error. On a single-digit-KB/s link, for a household that cannot
absorb two dollars a month, it is the difference between a tool that works and
one that doesn't. See [Philosophy, Part Seven](PHILOSOPHY.md#part-seven-the-token-sieve--why-we-refuse-to-send-the-whole-page).

Every one of these knobs exists *somewhere* — but buried in a config file, an
environment variable, or an external gateway, and set once at startup. What we
haven't found anywhere else is having them **live and together**: token *and*
dollar cost, a request pace-setter, and an agent leash with a real off-switch,
all adjustable from **one terminal menu, mid-session, with the arrow keys**, in a
self-contained tool.

| Control *(as of July 2026 — corrections welcome via an issue)* | **Gorilla OpenCode** | Claude Code | Codex CLI | aider |
| --- | :--: | :--: | :--: | :--: |
| Per-turn cost shown in **dollars**, in-app | ✅ tokens **+ $** | ✅ session $ | ❌ dashboard only | ~ estimate |
| **Live** requests/min pace dial (mid-session) | ✅ | ❌ | ❌ | ❌ |
| Cap on **agents/subagents** spawned | ✅ | ✅ env var (dflt 200) | ✅ config (dflt 6) | — (no subagents) |
| **True off-switch** for agents (fully disable) | ✅ Nuclear | ❌ "can't be turned off" | ❌ | — |
| Adjustable **in-UI, no config/env edit** | ✅ arrow keys | ❌ | ❌ (`config.toml`) | ❌ |

<sub>Basis: Claude Code exposes `CLAUDE_CODE_MAX_SUBAGENTS_PER_SESSION` (default 200) and states the limit **can't be turned off**; Codex CLI sets `agents.max_threads` (default 6) in `config.toml` and has **no built-in in-CLI usage/cost command**; aider reports token/cost **estimates** but "never enforces" limits — pacing is left to the provider. Every competitor sets these once, in config/env, before launch. Spot an error? Open an issue and we'll fix the cell.</sub>

## Install

**One command** (the binary installs itself: PATH, icons, desktop entry):

```sh
curl -fsSL https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/main/install.sh | sh
# or:  wget -qO- https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/main/install.sh | sh
```

**Debian / Ubuntu package** — from the [releases page](../../releases):

```sh
sudo apt install ./gorilla-opencode_*_amd64.deb

# or, with no terminal at all: right-click the .deb → Open With → GDebi
# (both resolve dependencies; `dpkg -i` does not, and will refuse until they
#  are installed)
```

**From source:**

```sh
go build -ldflags "-X github.com/opencode-ai/opencode/internal/version.Version=vX.Y.Z" -o gorilla-opencode .   # Go ≥ 1.24
./gorilla-opencode install       # optional: icons + desktop entry, no sudo
```

`gorilla-opencode uninstall` removes exactly what `install` created.

## Use

```sh
# NVIDIA NIM (your key, NVIDIA's prices)
LOCAL_ENDPOINT=https://integrate.api.nvidia.com/v1 \
LOCAL_ENDPOINT_API_KEY=nvapi-... gorilla-opencode

# Google AI Studio (Gemini 3, free tier works)
GEMINI_API_KEY=... gorilla-opencode

# ...or sign in with Google (free Code Assist tier, no API key) — see below
gorilla-opencode login

# Local models via Ollama (no key, no cloud)
LOCAL_ENDPOINT=http://localhost:11434/v1 gorilla-opencode
```

Non-interactive: `gorilla-opencode -p "your task" -q`. Pin models per
project in `.opencode.json`:

```json
{ "agents": { "coder": { "model": "local.deepseek-ai/deepseek-v4-flash" } } }
```

All original providers (Anthropic, OpenAI, Groq, OpenRouter, Azure,
Bedrock, Vertex, Copilot) remain wired as upstream left them.

## See it in action

New to this kind of tool? The plain-English **[GUIDE](docs/GUIDE.md)**
explains every part of the screen. Here's the short version.

**The model picker — a ranked leaderboard, with the full catalog behind it.**
We pinged every model on your key with a one-token message and ranked the
good ones for coding — numbered best-first (1 = best), each with a plain
description of its size and strength (here: NVIDIA NIM, DeepSeek V4 Pro at #1,
118 models deep). The rest still follow below, unranked — nothing is hidden.
Your key, your call.

![The ranked NVIDIA NIM model picker](docs/screenshots/gallery/v0130-picker-nim.png)

**One tool, every provider — switch with the ← → arrows.** Your models are
grouped by provider; up/down moves through a list, **left/right pages between
providers.** Gemini (2.0 → **3.6 Flash**), Groq (Llama 4, Qwen QwQ), Cerebras
(GLM 4.7 on wafer-scale silicon), NVIDIA NIM — the same terminal reaches all of
them. Here's the Gemini page; the counter (e.g. `10/14 →`) shows where you are.

![Reaching the Google/Gemini models with the arrow key](docs/screenshots/gallery/v0130-picker-gemini.png)

More providers (Groq, Cerebras, NIM) side by side:
**[docs/SCREENSHOTS.md](docs/SCREENSHOTS.md#one-tool-every-provider--the-latest-models-v0130)**.

**`/usage` — your quota as a meter, in bananas.** No more guessing whether
"96%" means left or spent. Each model group gets a thermometer bar — red at the
left end, green at the right — that shrinks from the green end as your week
burns down, with both numbers in words: *"71% left, 29% used · resets in 2d"*.
The gorilla narrates the descent: 🍌🍌🍌 *"Loaded up on bananas... let's go
nuts."* through *"You're halfway through your bananas..."* down to 🦍 *"Zero
bananas. Even the peel is gone."* If you have a DeepSeek or OpenRouter key,
your balance shows in the same panel — and a free-tier key is told the truth
("no credits purchased — free models only") instead of an empty-tank guilt trip.

![The /usage panel: gradient meters, plain-language numbers, and the banana verdict](docs/screenshots/gallery/v0180-usage-panel-healthy.png)

**And you don't have to ask.** After each response the meter is re-checked
quietly; the moment a group crosses a banana threshold, the crossing is
announced — timestamped in the scrollback, echoed on the status bar. Watch a
heavy session ride the whole ladder down:

![9% left: Banana emergency! Scraping the peel...](docs/screenshots/gallery/v0180-usage-9pct-emergency.png)

![The live alert the moment the barrel empties: Zero bananas. Even the peel is gone.](docs/screenshots/gallery/v0180-alert-zero-bananas-live.png)

Every reading is printed into the terminal history with a timestamp, because a
quota figure without a time is not a measurement — two dated readings are what
give you a burn rate. (This morning's burn rate, for the record: one
overambitious job-hunting agent, 100% → 0% before 8am. The gorilla warned us
at every step.)

**The `/context` menu — see (and control) exactly what every message costs.**
The top **🦍 GORILLA CONTROLS** section holds two live dials you drive with the
arrow keys: an **AI-server request pace-setter** (requests/min, to glide under
free-tier limits) and a **GORILLA AGENTS/SUBAGENTS leash** (cap helper agents,
right down to the ☢ Nuclear Option). Below, every tool and prompt block is a
switch, each with its **token *and* dollar cost**; the `⚠` marks what the
assistant can't work without. Turning off the big ones drops the number — and the
bill — immediately.

![The context loadout menu with the Gorilla controls](docs/screenshots/gallery/v0180-context-loadout.png)

**Lost? `/help` explains every command in plain language.** Grouped by what you
are trying to do rather than alphabetically — because someone who does not know a
command's name cannot look it up alphabetically. The selected command's full
explanation, including what it costs, shows in place; `/` searches the
descriptions as well as the names.

[![The /help command reference](docs/screenshots/gallery/v0180-help.png)](docs/screenshots/gallery/v0180-help.png)

**Two interfaces, one program — and no flags to remember.** Launch the icon
normally for the full interface above. **Right-click the icon → "Plain mode
(copyable output)"** and the same program runs with no interface at all: every
line is ordinary terminal output, so `Ctrl+A` / `Ctrl+Shift+C` lifts a five-hour
session straight into a text editor. Here they are running side by side — same
machine, same key, both with timestamps and the model's thinking on show.

[![Plain mode and the full interface running side by side](docs/screenshots/gallery/v0146-plain-and-tui-thinking.png)](docs/screenshots/gallery/v0146-plain-and-tui-thinking.png)

Plain mode carries a smaller command set and says so; the sidebar on the right
accounts for the session without guessing — input/output tokens, MCP servers,
how many language servers are off, which files were touched.

[![The plain-mode command list beside the full interface sidebar](docs/screenshots/gallery/v0146-plain-help-and-sidebar.png)](docs/screenshots/gallery/v0146-plain-help-and-sidebar.png)

More full-resolution screenshots and captions:
**[docs/SCREENSHOTS.md](docs/SCREENSHOTS.md)** ·
**[docs/GUIDE.md](docs/GUIDE.md)**.

## What this fork adds

- **Runs on 2026 providers**: NVIDIA NIM (your key, curated + ranked
  models), Google Gemini 3 — up to **3.6 Flash / 3.5 Flash-Lite**
  (1M context, thought-signature support) — and local Ollama.
- **Navigable model picker**: 100+ discovered models shown with curated
  names + capability descriptions ("DeepSeek V4 Pro — 1.6T MoE, 1M ctx,
  80.6% SWE-bench"), ranked best-coder-first, with a position counter.
- **Slash commands**: `/model` `/models` (picker), `/export` (session →
  Markdown in the cwd), `/clear` (fresh session), `/context` (loadout),
  `/tasks` (live helper-agent monitor + kill switch).
- **Agent transparency & kill switch** (`/tasks`): whenever the main agent
  spawns helper sub-agents, a `🦍 N helper(s) · /tasks` badge lights up in
  the status bar and a toast announces each spawn. `/tasks` lists every live
  helper — kill one (`enter`/`x`), or the Nuclear Option `X`: *kill 'em all,
  their tasks, and the horse they rode in on.* (This **terminates** running
  helpers; the `/context` Nuclear dial **prevents** them from starting.)
- **Context loadout** (`/context`): a transparent, total-control menu
  showing exactly what's sent to the model every turn — now with its
  **dollar cost** at your model's live price, not just tokens — plus a
  switch for every tool and prompt block. Strip it to the bone at your
  own risk; one key resets defaults.
- **Request pace-setter & agents/subagents leash** (`/context`, arrow
  keys): a user-adjustable requests-per-minute limiter that spaces calls
  to glide under undocumented free-tier ceilings, and a cap on how many
  helper agents the main agent may spawn — down to the 🦍 Gorilla Nuclear
  Option (all agents/subagents off). Both persist and apply mid-session.
- **Prompt caching** (opt-in, `GORILLA_OPENCODE_PROMPT_CACHE=1`) for endpoints
  that support it; Anthropic caching always on. See the changelog for
  the honest note on NIM.
- **Desktop-native**: embedded icons, self-installer, `.deb`, one-line
  curl install; the app-grid icon reads keys from
  `~/.config/gorilla-opencode/env`.

Full history: [CHANGELOG.md](CHANGELOG.md). Deep explanations, both
plain-language and developer: [DOCUMENTATION.dual-track.md](Changelogs/DOCUMENTATION.dual-track.md).

## What the revival changed

It started as six files to get the fossil talking to 2026 providers; it
has since grown into **~80 files changed across 25 releases** — roughly
**+4,000 lines**, **96 `// GORILLA OVERRIDE:` markers in 36 source
files**. Every single change carries one of those comments saying what
changed and why, so `grep -rn "GORILLA OVERRIDE" .` is the complete,
honest audit trail.

Headline work:

- **Providers**: authenticated OpenAI-compatible endpoints (NVIDIA NIM),
  Google Gemini 3 with thought-signature support (genai SDK v1.3→v1.64),
  local Ollama, and native Groq + Cerebras.
- **Bug fixes**: two segfaults masking real API errors, an upstream
  operator-precedence bug, a rate-limit retry storm (2→256s backoff), and
  a concurrent-request bug that tripped free-tier limits on a plain "yo".
- **UX**: a ranked, probe-verified model picker; `/model` `/context`
  `/export` `/clear` slash commands; the `/context` token-loadout menu;
  mouse-wheel scrolling; a modern, lean system prompt.
- **Packaging**: embedded icons + self-installer, `.deb`, one-line curl
  install, and all config unified under `~/.config/gorilla-opencode/`.

Blow-by-blow with dates: [CHANGELOG.md](CHANGELOG.md). Details,
verification results, and honest limitations:
[DOCUMENTATION.dual-track.md](Changelogs/DOCUMENTATION.dual-track.md).

## License

MIT, unchanged from the original. © 2025 Kujtim Hoxha (original OpenCode),
© 2026 gorillanobakaa (the revival and subsequent work), same license.

The `LICENSE` file names both holders. The original notice stays exactly where
it is - MIT requires it to be retained, and it is the reason this fork can exist
at all.
