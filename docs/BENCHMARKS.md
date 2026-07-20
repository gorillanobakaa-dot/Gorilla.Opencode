<p align="center"><img src="../internal/assets/icons/gorilla-opencode-256.png" width="96" alt="Gorilla OpenCode"></p>

<h1 align="center">Why does this AI model feel slow?</h1>

<p align="center"><em>A small, honest experiment you can repeat yourself — and everything it teaches about how AI models are served.</em></p>

<p align="center">Measured 2026-07-20 · NVIDIA NIM free tier · one laptop, one API key · <a href="../scripts/benchmark-models.py">the script is in this repo</a></p>

---

## Start here (no jargon)

You open the app, pick a model, ask it something, and the answer comes
out **s l o w l y** — the words trickle in like water from a nearly-closed
tap. You switch to another model and the same answer floods out instantly.
Same app, same question. **What changed?**

This page answers that. We took a real question ("count from 1 to 20") and
sent it to a shelf of different AI models through the exact same code, and
we timed them. Then we explain *why* the fast ones are fast and the slow
ones are slow — because once you understand it, you can pick the right
model for your task and your budget instead of guessing.

You don't need a degree for this. You need two ideas.

### The only two numbers that matter

When you send a prompt, two separate things decide how it *feels*:

| The number | Plain meaning | The feeling when it's bad |
|---|---|---|
| **First-token latency** | How long you stare at a blank screen before the *first* word appears. | "Did it freeze? Is it broken?" |
| **Throughput** (tokens/sec) | Once words start, how fast they keep coming. | "The... letters... arrive... one... by... one..." |

A restaurant analogy that's actually accurate:

- **First-token latency** = how long after you order before the *first*
  plate reaches your table. The kitchen has to notice your order, and if
  it's cold it has to fire up the stove.
- **Throughput** = how fast the following plates keep arriving once the
  kitchen is going.

A model can be great at one and terrible at the other. A slow-to-start,
fast-to-stream model *feels* frozen and then dumps everything at once. A
quick-to-start, slow-to-stream model answers instantly and then dribbles.
Both are annoying, for opposite reasons. Keep these two numbers separate in
your head and model speed stops being mysterious.

---

## What we found

Same code, same prompt, sent to each model in turn. Sorted fastest-streaming
first. **These are real measurements, not estimates.**

| # | Model | First token | Throughput | How it feels |
|--:|---|--:|--:|---|
| 1 | `nvidia/nemotron-3-nano-30b-a3b` | 0.89 s | **77.5 tok/s** | ⚡ instant |
| 2 | `nvidia/nemotron-3-super-120b-a12b` | 0.55 s | **62.4 tok/s** | ⚡ instant |
| 3 | `openai/gpt-oss-120b` | 0.57 s | **61.2 tok/s** | ⚡ instant |
| 4 | `meta/llama-3.3-70b-instruct` | 15.24 s | 30.4 tok/s | 🐢 long freeze, then fast |
| 5 | `nvidia/nemotron-3-ultra-550b-a55b` | 1.57 s | 27.1 tok/s | 🙂 snappy |
| 6 | `deepseek-ai/deepseek-v4-flash` | 1.40 s | 6.9 tok/s | 😐 slow trickle *(see note)* |
| 7 | `mistralai/mixtral-8x7b-instruct-v0.1` | 0.29 s | 8.8 tok/s | 😐 quick start, slow trickle |
| 8 | `minimaxai/minimax-m3` | 7.90 s | 7.3 tok/s | 🐢 slow both ways |
| 9 | `mistralai/mistral-large-3-675b-instruct` | 5.39 s | 4.6 tok/s | 🐢 slow both ways |
| 10 | `nvidia/llama-3.3-nemotron-super-49b-v1.5` | 13.56 s | 2.5 tok/s | 🐢🐢 painful |

> Row 6 (`deepseek-v4-flash`) was measured on an earlier run; on the batch
> run for this table the same model returned **HTTP 503 (overloaded)** and
> gave no output at all. That is not a typo — it's the single most important
> lesson on this page, and we come back to it below.

**Read the extremes.** The fastest model here streams **31× faster** than
the slowest (77.5 vs 2.5 tokens/sec). For a 500-word answer that's the
difference between **~4 seconds** and **over two minutes**. Nobody tells you
this when you pick a model from a list — so we measured it.

### The models that didn't answer at all

Eight of the models we tried never produced output. This is not us hiding
failures — it's *data*, and learning to read it is part of the point.

| Model | What happened | What that means |
|---|---|---|
| `deepseek-ai/deepseek-v4-pro` | HTTP 400 — *"DEGRADED function cannot be invoked"* | The #1-ranked model was **deployed but unhealthy** on the provider's side. Not your fault, not the app's. |
| `deepseek-ai/deepseek-v4-flash` | HTTP 503 | **Overloaded right now.** It worked minutes earlier (row 6). Come back later. |
| `qwen/qwen3.5-122b-a10b` | HTTP 410 | **Gone / retired.** The provider removed this model from the endpoint. |
| `z-ai/glm-5.2` | connection failed | Couldn't even establish a response in time. |
| `qwen/qwen3.5-397b-a17b` | timed out (>70 s) | Deployed, but so backed-up or slow it never produced a first token. |
| `meta/llama-4-maverick-17b-128e-instruct` | timed out (>70 s) | Same. |
| `qwen/qwen3-next-80b-a3b-instruct` | timed out (>70 s) | Same. |
| `minimaxai/minimax-m2.7` | timed out (>70 s) | Same. |

**Learn to read the number, not the vibe.** An error code is a sentence,
not a shrug:

- **400 / "degraded"** → the model exists but the server hosting it is sick. Wait, or pick another.
- **410 Gone** → it's been retired. Stop trying; update your list.
- **503** → too many people are using it *this second*. Retry in a bit.
- **timeout** → it may be alive but is queued behind everyone else on the free tier.

None of these mean "the software is broken." They mean the *shared kitchen
is busy or closed*. A tool that hides these behind a generic spinner teaches
you nothing; one that shows you the code lets you make a decision.

---

## The big lesson: bigger is **not** slower (and the name tells you why)

Look again at the top of the results table. The **550-billion-parameter**
model (row 5) streams faster than the **70-billion** one (row 4). The
**30-billion** nano (row 1) is the fastest thing on the board. If "bigger =
slower" were true, the table would be upside-down. It isn't. Why?

Because most modern models are **Mixture-of-Experts (MoE)**, and the model's
own name quietly tells you the trick — if you know how to read it.

```
nvidia/nemotron-3-ultra-550b-a55b
                       │     │
                       │     └── "a55b" = 55 billion ACTIVE parameters
                       └──────── 550 billion TOTAL parameters
```

A MoE model is like a **hospital full of specialists**. It has 550 billion
parameters' worth of doctors on staff (total), but for any one question it
only wakes up the handful it needs — 55 billion worth (active). You pay in
**memory** for the whole hospital, but you pay in **speed** only for the
doctors who actually see you. **Active parameters, not total, predict how
fast it streams.**

Watch the NVIDIA Nemotron family prove it, cleanly, in our own numbers:

| Model | Total params | **Active** params | Throughput |
|---|--:|--:|--:|
| nemotron-3-nano | 30 B | **3 B** (`a3b`) | 77.5 tok/s ⚡ |
| nemotron-3-super | 120 B | **12 B** (`a12b`) | 62.4 tok/s |
| nemotron-3-ultra | 550 B | **55 B** (`a55b`) | 27.1 tok/s |

More active parameters → slower streaming, in near-perfect order.
The "550B" label that looks scary is mostly *staff on payroll*, not *staff
in the room*. This is why a well-designed big model can feel snappier than a
small old one: `mixtral-8x7b` (row 7) is a small MoE, but it's from an
older generation and less optimised on this endpoint, so it streams at a
mere 8.8 tok/s despite its size. **Architecture and optimisation beat raw
size.**

> **The takeaway you can use today:** when choosing a model, look for the
> `aNNb` in the name. That "active" number is your speed dial. A `...-a3b`
> will almost always feel faster than a `...-a55b`, whatever the big number
> in front says.

### Speed is not quality — they're different axes

Here's the catch, and this page would be dishonest without it: **the fast
models are not automatically the smart ones.** Streaming speed measures how
quickly text comes out, *not whether the text is correct*. The little
`a3b` nano that won the speed race will lose the *reasoning* race to a
`deepseek-v4-pro` on a hard bug — when the pro model is actually up.

So the real skill isn't "find the fastest model." It's **matching the model
to the job**:

- **Quick edits, boilerplate, "rename this everywhere"** → pick a fast
  low-active-param model. You want throughput; the task isn't hard.
- **A subtle concurrency bug, architecture design, a tricky algorithm** →
  pick a big high-active-param model and *accept* the slower trickle. You're
  paying speed for intelligence, and it's worth it.

We measured **speed** on this page because speed is the thing nobody
publishes and everybody feels. We did **not** measure quality here — and per
this project's rule, *if we didn't measure it, we don't claim it.* For
coding-quality rankings we defer to public benchmarks like
[SWE-bench](https://www.swebench.com/), which is why the model picker in the
app is ranked using those, not using the speeds on this page.

### Why the same model gives different answers on different days

Row 6 in the results is the whole lesson in one row. `deepseek-v4-flash`
streamed at 6.9 tok/s one minute and returned **503 Overloaded** the next.
Nothing about the model changed. What changed is that you're sharing a
**public, free kitchen** with thousands of other people, and kitchens get
busy.

This means **one measurement is a snapshot, not a verdict.** The honest way
to use numbers like these is:

1. Treat this table as *"here's the shape of the landscape"*, not gospel.
2. **Run the experiment yourself**, on your key, at your hour, for your
   region — which is exactly why the script is in this repo.
3. If a model you like returns 503, that's traffic, not a funeral. Retry.

A paid, dedicated endpoint smooths most of this out. A free shared one — the
kind that lets a student in Lagos or Lima learn this stuff for **zero
dollars** — trades steadiness for being free. That's a fair trade. Just know
which one you're on.

---

## Reproduce every number on this page

This is the part that separates *learning* from *being told*. Don't trust
our table — **regenerate it.** The whole experiment is ~60 lines of Python
with no libraries to install, sitting right here in the repo:
**[`scripts/benchmark-models.py`](../scripts/benchmark-models.py)**.

```sh
# 1. Point it at your endpoint's key (NVIDIA NIM shown; any
#    OpenAI-compatible endpoint works — Groq, Cerebras, local Ollama).
export NIM_KEY=nvapi-your-key-here

# 2. Run the built-in list...
python3 scripts/benchmark-models.py

# 3. ...or test any models you like, by their ids:
python3 scripts/benchmark-models.py \
    nvidia/nemotron-3-nano-30b-a3b \
    deepseek-ai/deepseek-v4-flash
```

You'll get a table like ours. **It will not match ours exactly** — your
region, your hour, and the shared load will differ. That mismatch *is the
lesson from the section above*, felt first-hand. That's the point.

### How the measurement actually works (the honest details)

No magic, no hidden weighting. In plain terms, for each model the script:

1. Opens a **streaming** chat request (so we see tokens as they arrive, not
   all at once at the end).
2. Starts a stopwatch. Records the clock the instant the **first** chunk of
   content arrives → that's **first-token latency**.
3. Counts every chunk until the model stops, divides by the seconds spent
   streaming → that's **throughput**.
4. Gives up after a timeout (45 s in the main run, 70 s for the retry) and
   records the failure honestly instead of pretending.

The prompt is deliberately trivial — *"Count from 1 to 20 in words"* — so we
measure the **plumbing** (queuing, cold-start, streaming), not how hard the
model has to think. A trivial prompt is the fair way to compare serving
speed across models of wildly different sizes.

**What this does not measure**, stated plainly so no one is misled:
answer quality, reasoning ability, tool-use skill, long-context behaviour,
or cost. Only raw serving speed, on one key, on one day. If you need the
other things, measure *those* — and now you know how to build the tool that
does.

---

## The five things to walk away with

1. **Two numbers, kept separate:** first-token latency (the wait) and
   throughput (the trickle). Almost all "this feels slow" complaints are one
   or the other.
2. **Read the `aNNb`**, not the big number. Active parameters set the
   speed; total parameters set the memory bill. Bigger is not slower.
3. **Speed ≠ quality.** Match the model to the job: fast+small for easy
   work, big+slow for hard thinking. Don't pay for reasoning you don't need,
   don't starve a hard problem of it.
4. **Error codes are sentences.** 503 = busy, 410 = retired, 400/degraded =
   provider-side sick, timeout = queued. None mean "your software is
   broken."
5. **One run is a snapshot.** Shared free endpoints vary by the minute.
   Reproduce it yourself — the script is right here, and running it teaches
   more than reading this ever will.

---

<p align="center"><em>This experiment costs nothing to repeat. That was the point.<br>
Part of <a href="../README.md">Gorilla OpenCode</a> — a terminal AI coding agent that shows you what it's doing instead of hiding it.<br>
No telemetry, no accounts, MIT licensed. See our <a href="../PHILOSOPHY.md">Open Source Philosophy</a>.</em></p>
