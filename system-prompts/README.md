# System prompts — study, research, and redesign

This directory exists to make the agent's instructions **auditable and
improvable**. Nothing here is hidden: `current/` is exactly what ships
today, `historical/` is what came before and why it was replaced, and
`RESEARCH-SOURCES.md` is the primary literature the design rests on.

**In plain terms:** a "system prompt" is the standing set of instructions
this program sends to the AI before it ever sees your question. It is the
difference between an assistant that checks its work and one that tells you
a build succeeded when it did not. Most tools treat it as a trade secret.
We publish ours, because you cannot trust what you cannot read.

## Layout

- `current/` — the prompts the built binary uses right now, byte-identical
  to the embedded originals in `internal/llm/prompt/`. This is enforced by
  `internal/llm/prompt/studycopy_test.go`, not by good intentions; the two
  had already drifted once before the guard existed.
  - `coder-modern.md` — the main prompt, sent on every chat turn.
  - `task.md` — helper sub-agents (read-only: search and read, no builds).
  - `summarizer.md` — `/clear` compaction and session summaries.
  - `title.md` — naming new sessions.
- `historical/` — superseded prompts, kept because the reasoning is the
  point:
  - `coder-anthropic-2023-upstream.md`, `coder-openai-2023-upstream.md` —
    the two prompts inherited from upstream OpenCode. Dead code since the
    constants were removed from `coder.go`; retained as the "before"
    picture. Read them next to `current/coder-modern.md`.
  - `coder-lean-draft.md`, `coder-modern-prose-draft.md` — drafts from the
    2026-07 redesign. Neither is what shipped.

## What you can do about it

You are not stuck with our judgement. In the app:

- `/prompts` — read, edit, or reset any of the four. Your edit is stored
  separately; the shipped default is compiled into the binary and cannot be
  damaged by an edit, so "reset" is always a real reset.
- `/context` — switch off individual **sections** of the coder prompt, with
  the token cost of each and a plain-language line telling you what you lose.
  Two sections are marked critical (`preamble` and `honesty`); turning those
  off makes the AI *more* likely to claim success it never observed, and the
  menu says so rather than quietly letting you do it.

## The 2026-07-29 system prompt rewrite (shipped in v0.1.49)

All four prompts were reworked against Anthropic's published guidance for
**Claude Fable 5** — see the credit and citation in `RESEARCH-SOURCES.md`.
That guidance is written for exactly the workload this fork is built for:
long-horizon runs where the model works for hours and nobody is watching.

What changed, and why:

| Change | Why |
| --- | --- |
| **New `# scope` section** | The model was free to take actions nobody asked for. Asking a question is now explicitly not a work order, and a state-changing command has to be justified by evidence for *that* action. |
| **New `# delegation` section** | Sub-agents existed but the prompt never said when to use them. It now does — and says they are read-only, so the model stops trying to delegate a build. |
| **New `# memory` section** | Project context files were read but never treated as authoritative, and nothing was ever written down for next time. |
| **`# honesty` — audit before reporting** | Every progress claim must now point at a tool result from the session. In Anthropic's testing this near-eliminated fabricated status reports; it is the single most valuable line here for anyone leaving a build running overnight. |
| **`# conduct` — do not end on a promise** | "I'll now run the build" as the last thing in a turn, with no build. The rule is: if your final paragraph is a plan, do the plan. |
| **`# conduct` — context is not a reason to stop** | Long sessions produced unprompted offers to summarise and hand off. |
| **`# output` — re-ground the reader** | After hours of unattended work, the summary is the user's *first* look. Working shorthand and invented labels get dropped. |
| **`summarizer` — what was ruled out survives** | Approaches already tried and the errors they gave now survive compaction, so a freshly-compacted context cannot cheerfully retry them. |
| **`task` — "one word answers" removed** | It was costing the parent agent the evidence behind the answer. |

**The honest cost:** the coder prompt grew from ~464 to ~1058 estimated
tokens per turn (1,855 → 4,233 bytes, measured, not guessed). That is a real
increase in fixed overhead on every request. It is still roughly half the
~2,003-token 2023-era prompt it replaced, and every new section can be
switched off individually in `/context`.

**What we deliberately did not adopt:** Anthropic's guide recommends a
`send_to_user` tool so an asynchronous agent can push content to a user
verbatim without ending its turn. It solves a problem this program does not
have — the model's text already streams straight into your terminal, and
nothing summarises it in between. Adding the tool would have been cargo
cult. The effort-level and `refusal`-fallback guidance is likewise
Claude-API-specific and does not generalise to the local and open-weight
models this fork is mostly pointed at.

## On telemetry

Audited 2026-07-20, re-checked 2026-07-29: this fork contains **no
telemetry, analytics, or metrics code of any kind** (no posthog/sentry/
segment/amplitude/track calls, no phone-home). It is the pre-commercial MIT
original. The word "telemetry" appeared once — as a *claim inside the OpenAI
prompt* ("Log telemetry so sessions can be replayed") describing a capability
that does not exist. That line was token-wasting and misleading and was
removed. There is nothing to opt out of because there is nothing being sent.

Primary citations and links for everything below:
**[RESEARCH-SOURCES.md](RESEARCH-SOURCES.md)**.

## What the research says (synthesis)

From the original SOTA dossier (SWE-agent, CodePlan, CodeR, AGoT, LLMLingua,
Reflexion, and the agentic-loop studies), plus the 2026 additions:

1. **Formatting bloat costs accuracy, not just tokens.** ALL-CAPS and heavy
   markdown fragment BPE tokens ("IMPORTANT" = 2–3 tokens vs "important" = 1)
   and dilute attention over long compile logs. The upstream prompt
   (`historical/coder-anthropic-2023-upstream.md`, 8,015 bytes / ~2,003 est.
   tokens) contains "IMPORTANT" seven times, "NEVER" three, "MUST" four, and
   gives the brevity instruction three times — twice word-for-word. → stripped.
2. **Threat and emotional prompting backfires.** "DO NOT FAIL OR ELSE" shifts
   output toward hedging and *false success reports*. → neutral, declarative,
   imperative only.
3. **Saying a rule is not following a rule.** *The Compliance Gap* (2026)
   names this directly: models agree to a process constraint and then violate
   it, and the violation is invisible to anything that only reads the text.
   This is why the honesty section demands a *tool result*, not a promise.
4. **Instruction-following degrades with prompt length and constraint count.**
   AgentIF and OctoBench both measure it. This is the argument against simply
   appending every good idea to the prompt — and the argument for `/context`
   letting you drop what a given job does not need.
5. **Loops come from three causes** (stderr resonance, context blindness past
   ~50k tokens, and no explicit failure primitive). The prompt can discourage
   re-running identical failed commands and can require recording what was
   tried, but **real loop eradication is harness-level**, not prompt-level.
6. **Bounded, filtered tool output** (extract only `error:` / `fatal error:` /
   `undefined reference` / `recipe ... failed`, cap at ~800 tokens) is the
   single biggest lever for build agents — raw `make`/`mach build` output
   saturates context (SWE-agent's ACI).

## Roadmap — what the prompt alone cannot do (needs code)

The high-value remainder is harness work, tracked for a future change:

- [ ] **Build-log filter** on the bash tool: strip `CC`/`CXX`/`AR` progress
  noise, surface only error lines + file:line, cap the tool response (~800
  tokens). Biggest single win for kernel/Firefox builds.
- [ ] **Loop guard**: hash `(tool, args, stderr-snippet)`; on a repeat within
  a sliding window, intercept before the model call and inject a forced
  strategy-shift message.
- [ ] **`yield_failure` / `yield_success` tools**: explicit exit ramps so the
  agent can declare an unresolvable toolchain issue instead of looping.
- [ ] **Measure the 2026-07-29 prompt rewrite** against real Firefox 154/155 and kernel
  builds. The changes above are grounded in published research and in
  Anthropic's own testing, **not** in our own A/B numbers. We have not run
  that experiment yet, and this line stays here until we have.
