# What You're Paying For — and How to Control It

*A free, standalone lesson. No prior knowledge assumed in the plain-language
track. Everything here is real and reproducible: file names, exact settings, and
the reasoning, so you can **recreate it yourself** — or check that we did what we
say.*

This follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): every
significant thing is explained **twice** — once in plain language for any human
being, once with developer precision. Both cover the same facts.

---

## Table of contents

1. [How an AI coding tool actually spends your money](#1-how-an-ai-coding-tool-actually-spends-your-money)
2. [The tools, and which ones "phone out"](#2-the-tools-and-which-ones-phone-out)
3. [What we built, and exactly how](#3-what-we-built-and-exactly-how)
4. [How to use the controls (`/context`)](#4-how-to-use-the-controls-context)
5. [Is this really new? (with sources)](#5-is-this-really-new-with-sources)
6. [Recreate it yourself](#6-recreate-it-yourself)

---

# 1. How an AI coding tool actually spends your money

## Plain language

Picture the AI as **a very knowledgeable person on the other end of a phone line
who cannot touch anything in your house.** You're in the house with the phone.
They can *talk* — that's all. When people say "the AI read my file" or "the AI
ran a command," what really happened is: the AI *asked*, and **your** program
(this tool, on your laptop) did it and read the result back down the line.

So what do you pay for? **The phone call — measured in words.** The industry
calls a chunk of text a **token** (roughly ¾ of a word). You pay a little per
word going in, a bit more per word coming back. That's essentially the whole
bill.

Three things quietly run the meter:

1. **The menu is re-read every call.** Every time the tool phones the AI, it must
   re-send the *whole list* of things it can do ("I can read files, here's how
   you ask…"). That list is words. You pay for it **on every single turn**, even
   if the AI never uses a tool that turn.
2. **The AI's requests are words.** "Please read `main.go`" is text it generated —
   billed (at the higher outbound rate).
3. **Answers get re-paid, over and over.** The AI has **no memory between calls**.
   So every new step re-sends the *entire conversation so far* — including that
   big file you read out loud three steps ago. A large file or build log gets
   **paid for again on every turn** until something trims it out.

That third point is why long sessions and busy agents get expensive: it's not a
per-tool fee, it's the conversation **re-sent in full, every hop**.

## Developer track

"Tool use" (function calling) is a **client-side request/response loop**, not
remote execution. Per user request:

1. **Client → API**: system prompt + **tool schemas** (JSON) + full history.
2. **API → Client**: a normal completion, or a structured `tool_call` (name +
   args) — model **output** tokens.
3. **Client executes the tool locally.** The provider is never involved.
4. **Client → API (next turn)**: the `tool_result` is appended and the whole
   conversation re-POSTed — now **input** tokens.

The model is **stateless** between calls, so cost scales with the *integral of
context size over turns*, not the number of tools. Three token cost sites, all
tokens: schemas (input, every turn), the tool-call (output), and the tool result
(input, every subsequent turn until pruned). **There is no per-invocation fee for
local tools** — which is why this project's model table (`internal/llm/models/models.go`)
has `CostPer1MIn/Out/InCached/OutCached` and *no* "cost per tool call" field.

**Round-trip multiplier:** one user request that triggers *k* tool calls ≈ *k+1*
API requests, each re-sending the growing context. This is the real driver of
agent cost — and the thing the controls in §3 attack.

---

# 2. The tools, and which ones "phone out"

## Plain language

We audited all 13 things the AI can ask for. **Good news: not one charges a
hidden fee.** You pay for words, full stop — this tool ships **no** "the-provider-
runs-it-for-you" tools that bill extra.

- **11 of 13 never leave your laptop** — reading, writing, editing, listing,
  searching, running commands. Private, and free beyond the words.
- **2 reach the internet:** *Fetch* ("go to this web address") and *Sourcegraph*
  ("search public code for me").

### 🔎 Sidebar: what is "Sourcegraph"?

A **giant search engine for source code** — like Google, but it searches the
actual code inside millions of public open-source projects. The AI can use it to
see **how thousands of other programmers really used** some library, instead of
guessing. Handy for an obscure or brand-new library; **unnecessary most of the
time**, which is why it ships **off by default**. When on, your search query
leaves your machine (to sourcegraph.com) — so we made it **ask permission first**,
like every other tool that reaches outside. Off = nothing leaves, and its chunky
tool description (~1,000 words) stops riding every turn.

## Developer track

Only `internal/llm/tools/fetch.go` and `sourcegraph.go` open sockets; the rest
(`bash`, `edit`, `write`, `patch`, `view`, `ls`, `glob`, `grep`, `diagnostics`)
run locally. Cost model = **100% tokens**; no provider-billed hosted tools ship.
`tool.agent` (sub-agent) isn't a network tool but a **token multiplier** — it
runs a full nested request loop (see §3).

---

# 3. What we built, and exactly how

Everything below is a real change. Each carries a `// GORILLA OVERRIDE:` comment
in the source, so `grep -rn "GORILLA OVERRIDE" .` is the complete audit trail.

## 3.1 Killed the biggest hidden cost: the recursive file dump

**Plain:** the tool used to cram a list of **up to 1,000 files** — walking every
sub-folder — into the background of *every* message. In a big project that was
**tens of thousands of words per turn**, before you typed anything. We swapped it
for a **quick glance**: just the top-level names + a short `git status`. On a big
tree it dropped from **~10,000–30,000 tokens to ~76.**

**Developer:** `internal/llm/prompt/coder.go` — `getEnvironmentInfo()` no longer
runs the recursive `LsTool` (`MaxLSFiles=1000`). New `projectSummary()` =
depth-1 `os.ReadDir` (cap 25) + `git status --short` (cap 10, 2s timeout, silent
on failure). Tests in `internal/llm/prompt/env_test.go`.

## 3.2 Made the bill legible: tokens **and dollars**

**Plain:** the `/context` menu showed how many words each message costs — but not
the **money**. Now it shows both, priced at your model's real rate. Free/flat
tiers honestly show `$0.00`.

**Developer:** `config.LoadoutCost()` in `internal/config/loadout.go` prices
`LoadoutActiveTokens()` at the active model's `CostPer1MIn` using the *same*
formula the agent bills a turn with. `internal/tui/components/dialog/loadout.go`
renders it; unknown models show `unpriced`, not a fake `$0.00`.

## 3.3 The pace-setter: stop slamming free-tier limits

**Plain:** free tiers cap how often you may call (NVIDIA NIM says "up to 40/min" —
in practice a moving target). The tool used to fire as fast as the work demanded
and smash into the ceiling, triggering "rate limited… retrying" churn. Now a
**speed limit you set yourself** spaces the calls so you glide *under* it.

**Developer:** `internal/llm/provider/ratelimit.go` — a tiny dependency-free
`paceRequest(ctx)` (mutex + next-slot timestamp, spacing = `time.Minute/rpm`)
wired into `baseProvider.SendMessages`/`StreamResponse` — the one chokepoint all
providers (and title/summarize/sub-agent traffic) share. Setting persists in
`internal/config/ratelimit.go` (`ratelimit.json`, default **25/min**); read live,
so changes apply mid-session. Tests in `ratelimit_test.go`.

## 3.4 The agents/subagents leash — and the 🦍 Gorilla Nuclear Option

**Plain:** the main AI can summon **helper AIs (sub-agents)**, each of which runs
its *own* full back-and-forth — great on a paid plan, brutal on a free one. We
checked the code: helpers **can't** summon their own helpers (no runaway tree),
and they run **one at a time** (no stampede) — but there was **no cap** on how
many, and the main loop had **no stop-counter**. So we added a dial: from
unlimited down to **☢ Gorilla Nuclear** — all agents/subagents off, main AI works
solo, fewest possible calls.

**Developer:** `internal/config/subagents.go` (`MaxSubAgents`: `-1` unlimited,
`0` Nuclear, else cap; ladder `100…1`). `internal/llm/agent/subagent_guard.go` —
per-coder-session counter, reset each turn; `agentTool.Run` refuses over-cap
spawns as a **normal tool result** so the model adapts. On Nuclear,
`CoderAgentTools` omits `tool.agent` entirely so its schema tokens vanish too.
Verified: `TaskAgentTools` excludes `tool.agent` (depth capped at 1) and tool
calls run sequentially. Tests in `subagent_guard_test.go`.

## 3.5 Two security fixes we did along the way

**Plain:** *Fetch* would visit **any** web address the AI named — including sneaky
"inside" addresses pointing back at your own machine or network. Now it refuses
those automatically. And *Sourcegraph* used to search **without asking**; now it
asks first, like everything else.

**Developer:** `fetch.go` — `blockedFetchTarget()` rejects loopback / link-local
(incl. cloud metadata `169.254.169.254`) / private / unspecified hosts before the
prompt (`fetch_ssrf_test.go`; known gap: pre-DNS host only). `sourcegraph.go` —
added a `permission.Request(...)` gate, threaded through the sub-agent toolset.

---

# 4. How to use the controls (`/context`)

**Plain language.** Open the menu by typing **`/context`**. The top section is
**"🦍 GORILLA CONTROLS."** You drive it entirely with the **arrow keys** (they
work on every keyboard layout):

- **↑ / ↓** — move the highlight to a line.
- **← / →** — change the highlighted dial (left = less, right = more).
- **space** — toggle a highlighted feature on/off.

```
🦍 GORILLA CONTROLS — tune for your connection / free tier  (↑↓ pick · ←→ change):
  AI SERVER requests — pace-setter   ‹ ←/→ ›  25/min (spaces calls ~2.4s apart) — lower if you get "rate limited"
  GORILLA AGENTS/SUBAGENTS — leash   ‹ ←/→ ›  unlimited — each agent adds AI-server requests
```

**Finding your free-tier sweet spot (e.g. NVIDIA NIM):** start the speed around
**20–25/min**; if it's smooth, nudge **→** up; if you see "rate limited," nudge
**←** down. Busy time of day? Drop it, raise it later. On a tight budget, walk the
leash down to **1** or **☢ Gorilla Nuclear** to keep request counts lowest.

---

# 5. Is this really new? (with sources)

The individual pieces exist elsewhere — but almost always as **developer-facing
config-file / env-var knobs set once at startup**, not a live, in-terminal
control panel. As of July 2026 (corrections welcome via an issue):

| Control | **Gorilla OpenCode** | Claude Code | Codex CLI | aider |
| --- | :--: | :--: | :--: | :--: |
| Per-turn cost in **dollars**, in-app | ✅ tokens **+ $** | ✅ session $ | ❌ dashboard only | ~ estimate |
| **Live** requests/min pace dial (mid-session) | ✅ | ❌ | ❌ | ❌ |
| Cap on **agents/subagents** | ✅ | ✅ env (dflt 200) | ✅ config (dflt 6) | — none |
| **True off-switch** for agents | ✅ Nuclear | ❌ "can't be turned off" | ❌ | — |
| Adjustable **in-UI, no config/env edit** | ✅ arrows | ❌ | ❌ | ❌ |

What's uncommon isn't any single knob — it's having them **live and together**,
adjustable from one terminal menu, mid-session, in a self-contained MIT tool.

**Sources**
- Claude Code — subagents (`CLAUDE_CODE_MAX_SUBAGENTS_PER_SESSION`, cannot disable): <https://code.claude.com/docs/en/sub-agents>
- Claude Code sub-agents — context, cost, parallelism: <https://www.mindstudio.ai/blog/claude-code-sub-agents-explained>
- OpenAI Codex — configurable sub-agent limits (`agents.max_threads`): <https://github.com/openai/codex/issues/16183>
- Codex CLI cost tracking (no in-CLI usage command): <https://whoburnedmore.com/guides/codex-cli-cost>
- aider — token limits are reported, not enforced: <https://aider.chat/docs/troubleshooting/token-limits.html>
- Configurable per-minute rate limiting is still a feature request in agents: <https://github.com/NousResearch/hermes-agent/issues/31802>

The design also draws on published research we cite so you can judge for
yourself: [system-prompts/RESEARCH-SOURCES.md](../system-prompts/RESEARCH-SOURCES.md).

---

# 6. Recreate it yourself

Nothing here is magic. To reproduce the whole thing from source:

```sh
git clone https://github.com/gorillanobakaa-dot/Gorilla.Opencode
cd Gorilla.Opencode

# 1. See every change we made, with a one-line reason on each:
grep -rn "GORILLA OVERRIDE" internal | less

# 2. Prove the env block shrank (the biggest win):
go test ./internal/llm/prompt/ -run TestProjectSummary -v

# 3. Prove the pace-setter spaces requests and the leash counts spawns:
go test ./internal/llm/provider/ -run TestPaceRequest -v
go test ./internal/llm/agent/  -run TestHelperLeash  -v

# 4. Build it and try the dials:
go build -o gorilla-opencode . && ./gorilla-opencode
#   then type  /context  and use the arrow keys.
```

The files that matter, by change:

| Change | Files |
| --- | --- |
| Shallow env (§3.1) | `internal/llm/prompt/coder.go`, `env_test.go` |
| Dollar cost (§3.2) | `internal/config/loadout.go`, `internal/tui/components/dialog/loadout.go` |
| Pace-setter (§3.3) | `internal/llm/provider/{ratelimit.go,provider.go}`, `internal/config/ratelimit.go` |
| Agents/subagents leash (§3.4) | `internal/config/subagents.go`, `internal/llm/agent/{subagent_guard.go,agent-tool.go,tools.go,agent.go}` |
| Security (§3.5) | `internal/llm/tools/{fetch.go,sourcegraph.go}` |
| The `/context` UI | `internal/tui/components/dialog/loadout.go` |

Full dated history: [CHANGELOG.md](../CHANGELOG.md).

---

## Why this exists — a note from the author

> I spent a lot of my own money building and testing this. I wanted it to count.
> If a kid in Lima, or Port Harcourt, or Nairobi — anyone who was handed a slow
> laptop and an expensive, throttled connection — gets to learn from it and use
> it, then it counted.
>
> This is what **true open source** should look like: giving back to the community
> that taught you, and teaching the next person too. Not just the code, but the
> *understanding* — what we did, where, how, and why — so you can do it yourself,
> better.
>
> — *Gorilla*

MIT licensed. Take it, learn from it, improve it, pass it on.
