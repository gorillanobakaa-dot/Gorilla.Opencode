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

## The 2026-07-29 system prompt rewrite

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
increase in fixed overhead on every request.
**Re-measured 2026-08-06: it is now 5,078 bytes / 812 words / ~1,270 est.
tokens.** It gained ~845 bytes after the line above was written and nobody
re-measured, so this file understated the standing per-turn cost by about 20%
for a week. Word count barely moved, so the growth is structure rather than
new rules. On a single-digit-KB/s link that difference is real; re-measure
this figure whenever the prompt is touched rather than trusting the prose. It is still roughly half the
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

## The 2026-08-06 corrections — four things we had wrong

Everything in this section is a correction to something this directory
previously stated with confidence. It is written down because a research
directory that only records its wins is a marketing directory.

### 1. Sub-agents do not run concurrently. We told the model they did — three times.

`# delegation` read *"delegate independent subtasks: keep working while they
run."* There is no concurrency in this program. `agent-tool.go:94` is
`result := <-done`; the parent blocks until the helper finishes.

The genuinely embarrassing part is that we already knew. The `# tools` line
was corrected from "parallel" to "batching" on 2026-07-31, and the agent
tool's own description was rewritten the same day to open with "Agents run
ONE AT A TIME." Two fixes, same day, same falsehood, and the section
literally called `# delegation` was missed by both. It survived a rewrite
whose stated purpose was to explain when to delegate.

Now: *"delegate independent subtasks: saves context not time: helpers run one
at a time and block."* Costs 32 bytes a turn. Worth it, because the old line
actively encouraged spawning helpers as a latency hedge on a link where
latency is the whole problem — the single worst thing it could have taught.

### 2. The build-log filter deleted the errors it existed to preserve.

`filterBuildLog` shipped in every `.deb` with no tests. `buildNoiseRe`
matched its tool names as bare prefixes, so `ld:`, `cc1plus:`, `assertion`,
and every path under `arch/` were classified as compiler progress noise. The
noise test was also applied to the matched signal line itself, so noise beat
signal.

Net effect on a kernel or Gecko build — the two workloads it was written
for — was to strip the first line of failure and hand the model the
surrounding chatter under the header *"showing the N signal lines."*

We have a house rule that truncation must always announce itself, written
after the grep incident, on the grounds that a model handed a silent fragment
reasons about the fragment as if it were complete. This filter announced
itself impeccably. It was just announcing a lie. Fixed, and guarded by
`bash_logfilter_test.go`, which fails against the old regex.

### 3. "Capitals hurt accuracy" was never measured, by us or by anyone we cited.

See the amended item 1 under *What the research says* and the entry for
arXiv:2608.03711 in `RESEARCH-SOURCES.md`. Short version: the token-cost
argument for cutting `IMPORTANT` × 17 was always sound and still is. The
accuracy argument was extrapolated from a paper about emotional phrasing that
never studied letter case, and when someone finally measured letter case
directly, sparse capitals turned out to be the one formatting intervention
that reliably *helps*.

The prompt needed no change — two capitalised spans in 812 lowercase words
was already the right shape. We simply had the wrong reason for being right,
which is the kind of correct that stops being correct the moment conditions
change.

### 4. This file's own token figure was 20% out for a week.

It said 4,233 bytes / ~1,058 tokens. The prompt was 5,110. It had grown and
nobody re-measured, in a document whose central argument is that you should
measure things rather than assert them. Re-measured, and the roadmap
checkbox for the build-log filter — which had stayed unticked for so long
that a later session read it as outstanding and began writing a second
implementation — is now ticked.

**The pattern in all four:** every one was a claim nothing could falsify. The
prompt line had no test, the filter had no test, the research claim had no
citation, and the byte count had no re-measurement. The fixes that stuck are
the ones that came with a guard attached. Prose in a README is not a guard;
it is a promise, and this directory now contains a documented instance of
every kind of promise it makes being broken.

## 2026-08-06 — gzipped request bodies

Go's HTTP transport negotiates gzip for **responses** automatically and
transparently decompresses them. It never compresses what you **send**. So
every turn was uploading the whole conversation — system prompt, every prior
message, every tool result — as raw JSON, over a link where the uplink is
usually the scarcer direction. HTTP/2 was already compressing the headers via
HPACK; the body was the last bulk traffic going out untouched.

`internal/llm/provider/gzip_request.go` wraps the shared transport and gzips
request bodies over 1 KB. Both providers already funnel through
`resilientHTTPClient()`, so it was one line to wire in.

**Measured, on payloads shaped like real traffic:**

| Payload | Saved |
| --- | ---: |
| Conversation JSON (this repo's source as a chat log) | **77%** |
| The system prompt alone | 53% |
| Random bytes (the floor) | −0.1% |

**Honesty note on that table.** The first version of the test repeated one
identical sentence 400 times and reported **99.3% saved**. That is the best
case gzip will ever see and it would have gone straight into a release note
if nobody had looked. Even the varied synthetic payload the test now uses
flatters itself at ~90%. The 77% above is measured against real source text
and is the number to quote. The test says so in a comment, in capitals,
because this is a project that has already shipped one confidently-labelled
lie today.

**Not every provider accepts a gzipped body, and there is no way to ask.** So
it probes: compress, and if the server rejects the encoding, retry raw and
remember that host. Cost is exactly one wasted round-trip per host per
process.

The subtle part is telling *"this server hates gzip"* apart from *"that
request was simply bad"* — both can come back 400. A host is marked
unsupported **only when the uncompressed retry succeeds**. If both attempts
fail it was a genuine error, gzip stays on, and an ordinary invalid-model
typo does not silently disable compression for the rest of the session.

Opt out with `OPENCODE_NO_REQUEST_GZIP=1`. Guarded by eight tests in
`gzip_request_test.go`, plus a live probe (`gzip_live_probe_test.go`, gated
behind `GORILLA_LIVE_GZIP_PROBE=1` because it spends real quota).

### What the live probe found — including that we were wrong

Run 2026-08-06 against real endpoints. **Read this before getting excited
about the 77% above.**

| Endpoint | Result |
| --- | --- |
| NVIDIA NIM | **rejects gzip** — falls back, works |
| Ollama (local) | **rejects gzip** — falls back, works |
| Cloudflare | inconclusive (deprecated model, unrelated failure) |
| Anthropic / OpenAI | **untested — no API key on this machine** |

So on the providers this machine can actually reach, the feature currently
saves **nothing**. It is correct, it is safe, and today it is inert. That is
the honest headline and it should stay at the top of this section.

It is still worth keeping: the cost is one extra round-trip per host per
process, it disables itself per-host on first contact, and it will start
paying the moment it meets a provider that accepts the encoding. But nobody
should read the compression table above and believe their turns got faster.

**The probe also caught a bug that would have broken every NIM request.**
NIM answers a gzipped body with **HTTP 500**:

```
failed to decode json body: invalid character '\x1f' looking for beginning of value
```

`0x1f` is the first byte of the gzip magic number — NIM hands the compressed
bytes straight to a JSON parser and reports the parser's distress as a server
fault. Our fallback only recognised 415 and 400, so it never fired and the
request died. Eight httptest-based tests, all passing, had proven the logic
correct against a server we wrote ourselves; not one of them could have found
this, because we would never have written a mock that answers 500.

Fixed by adding 500 to `encodingRejected` and replacing the
supported/unsupported boolean with three states. The third state matters:
once a host has *proven* it accepts gzip, a later 500 is a real server error
and must not trigger another retry — otherwise every transient failure would
cost a doubled round-trip on the link least able to afford it.

The first version of the probe also printed **"ACCEPTS GZIP"** for NIM while
NIM was answering 500, because the verdict checked "did we mark it rejected"
rather than what had actually been learned. A diagnostic that reports success
it did not observe, in a tool built to stop exactly that — noted, and fixed.

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

1. **Formatting bloat costs tokens. Whether it costs accuracy depends on how
   much of it there is.** ALL-CAPS fragments BPE tokens ("IMPORTANT" = 2–3
   tokens vs "important" = 1), and that is a bill paid on every request. The
   upstream prompt (`historical/coder-anthropic-2023-upstream.md`, 8,015 bytes
   / ~2,003 est. tokens) contains "IMPORTANT" seven times, "NEVER" three,
   "MUST" four, and gives the brevity instruction three times — twice
   word-for-word. → stripped.
   **Amended 2026-08-06:** this item used to claim caps cost accuracy outright.
   arXiv:2608.03711 measures letter case directly across 13 models and finds
   the opposite for *sparse* caps — a few uppercase spans in lowercase context
   is the only formatting intervention that reliably raises accuracy. The
   destructive pattern is `aLtErNaTiNg` case. So the correct rule is about
   density, not capitals: emphasis works when it is rare, and seventeen
   instances is not rare. The effect is also near-zero in reasoning models and
   universal in non-reasoning ones — which matters here, because this fork is
   mostly pointed at local and open-weight models. See `RESEARCH-SOURCES.md`.
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

- [x] **Build-log filter** on the bash tool — **shipped**, `filterBuildLog` in
  `internal/llm/tools/bash.go`. This checkbox stayed unticked long after the
  code landed, and the cost was real: a later session read the roadmap, took
  it as outstanding, and started writing a second implementation.
  It shipped with no tests and a bug that inverted its purpose: `buildNoiseRe`
  matched its tool names as bare prefixes, so `ld:`, `cc1plus:`, `assertion`
  and every path under `arch/` were classified as progress noise — and the
  noise test was applied to the matched signal line itself, so noise beat
  signal. On a kernel or Gecko build it deleted the first line of failure and
  labelled the remainder *"showing the N signal lines"*. Fixed 2026-08-06;
  guarded by `bash_logfilter_test.go`, which fails against the old regex.
  **Known remaining gap:** the signal patterns miss some real first-line
  failures (`as: unrecognized option …`). Those fall through to the
  "no errors detected" tail branch, so they usually survive by luck rather
  than by design. Widening `buildSignalRe` is a separate change.
- [ ] **Loop guard**: hash `(tool, args, stderr-snippet)`; on a repeat within
  a sliding window, intercept before the model call and inject a forced
  strategy-shift message.
- [ ] **`yield_failure` / `yield_success` tools**: explicit exit ramps so the
  agent can declare an unresolvable toolchain issue instead of looping.
- [ ] **Measure the 2026-07-29 prompt rewrite** against real Firefox 154/155 and kernel
  builds. The changes above are grounded in published research and in
  Anthropic's own testing, **not** in our own A/B numbers. We have not run
  that experiment yet, and this line stays here until we have.
