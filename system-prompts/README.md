# System prompts — study, research, and redesign

This directory exists to make the agent's instructions **auditable and
improvable**. Nothing here is hidden: `current/` is exactly what ships
today, `reference/` is external material we learn from, and `proposed/`
is the redesign in progress.

## Layout

- `current/` — the prompts the built binary uses right now, extracted
  verbatim from `internal/llm/prompt/`:
  - `coder-anthropic.md` — used by the Anthropic family **and by the
    `local` provider (NVIDIA NIM, Ollama)**. ~2,003 tokens.
  - `coder-openai.md` — used only when the provider type is `openai`.
  - `task.md`, `summarizer.md`, `title.md` — sub-agent prompts.
- `reference/anthropic-leaks/` — leaked/observed production system
  prompts (incl. Claude Code) from
  [asgeirtj/system_prompts_leaks](https://github.com/asgeirtj/system_prompts_leaks),
  kept locally to learn from the best.
- `proposed/coder-lean.md` — a redesigned coder prompt: **621 tokens
  (−69%)**, neutral/imperative, compilation-specialized, with loop and
  hallucination discipline built in. Draft, for review.

## On telemetry

Audited 2026-07-20: this fork contains **no telemetry, analytics, or
metrics code of any kind** (no posthog/sentry/segment/amplitude/track
calls, no phone-home). It is the pre-commercial MIT original. The word
"telemetry" appeared once — as a *claim inside the OpenAI prompt*
("Log telemetry so sessions can be replayed") describing a capability
that does not exist. That line is token-wasting and misleading and has
been removed from the code. There is nothing to opt out of because
there is nothing being sent.

## What the research says (synthesis)

From the SOTA dossier (SWE-agent, CodePlan, CodeR, AGoT, LLMLingua,
Reflexion, and the agentic-loop studies):

1. **Formatting bloat costs accuracy, not just tokens.** ALL-CAPS and
   heavy markdown (`***`, `###`) fragment BPE tokens ("IMPORTANT" = 2–3
   tokens vs "important" = 1) and dilute attention over long compile
   logs. The current prompt repeats "IMPORTANT/VERY IMPORTANT/NEVER" and
   states "answer in <4 lines" three times. → strip it.
2. **Threat/emotional prompting backfires.** "DO NOT FAIL OR ELSE"
   shifts output toward hedging and *false success reports* (+20–35%
   filler). → neutral, declarative, imperative only.
3. **Loops come from three causes** (stderr resonance, context
   blindness past ~50k tokens, and no explicit failure primitive). The
   prompt can discourage re-running identical failed commands and can
   require recording what was tried (Reflexion), but **real loop
   eradication is harness-level**, not prompt-level.
4. **Bounded, filtered tool output** (extract only `error:` /
   `fatal error:` / `undefined reference` / `recipe ... failed`, cap at
   ~800 tokens) is the single biggest lever for build agents — raw
   `make`/`mach build` output saturates context (SWE-agent's ACI).

## Roadmap — what the prompt alone cannot do (needs code)

The `proposed/coder-lean.md` prompt covers points 1–3 at the prompt
level. The high-value remainder is harness work, tracked for a future
change:

- [ ] **Build-log filter** on the bash tool: strip `CC`/`CXX`/`AR`
  progress noise, surface only error lines + file:line, cap the tool
  response (~800 tokens). Biggest single win for kernel/Firefox builds.
- [ ] **Loop guard**: hash `(tool, args, stderr-snippet)`; on a repeat
  within a sliding window, intercept before the model call and inject a
  forced strategy-shift message.
- [ ] **`yield_failure` / `yield_success` tools**: explicit exit ramps
  so the agent can declare an unresolvable toolchain issue instead of
  looping.
- [ ] Make the lean prompt selectable (a loadout/config switch) and
  A/B it against the current one on real Firefox 154/155 and kernel
  builds before making it the default.
