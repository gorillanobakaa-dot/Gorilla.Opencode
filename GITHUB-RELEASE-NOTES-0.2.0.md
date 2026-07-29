## New system prompts — breathing life into an old fossil

A "system prompt" is the standing instruction sheet sent to the AI before it ever sees your question. It decides whether you get an assistant that checks its work, or one that tells you a build succeeded when it did not.

The one this program inherited from upstream OpenCode was written in 2023 and shouts. `IMPORTANT` occurs seven times; the same brevity instruction is given three times, as if the model would only believe it on the third try. That is not a dig at its authors — **writing excellent Go is a genuinely different skill from understanding how a language model behaves**, and in 2023 almost everyone wrote prompts that way. The research since then says that kind of emphasis makes things measurably worse, not better.

All four shipped prompts have been rewritten against the prompting guidance published by **the team at Anthropic behind Claude Fable 5**, who wrote down what they learned about steering an AI through long unsupervised work and published it instead of keeping it as a moat. Full credit and citation in [`system-prompts/RESEARCH-SOURCES.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/RESEARCH-SOURCES.md).

### What you'll notice

- **It stops reporting work it has not checked.** Every progress claim must now point at something that actually happened in the session. If you leave a Firefox or kernel build running overnight, this is the one that matters.
- **It stops doing things you did not ask for.** A question is explicitly not a work order.
- **It stops trailing off** on "I'll now run the build" without running the build.
- **It stops offering to hand off** just because the conversation got long.
- **Its final summary is written for someone who wasn't watching** — no shorthand, no jargon it invented three hours ago.
- **Compaction no longer erases what already failed**, so a compacted session can't retry a fix that failed an hour ago.

### What it costs

The coder prompt grew from 1,855 to 4,233 bytes — about **1,058 tokens a turn, up from 464**, on every request. The byte counts are measured; the token figures are the one-per-four-bytes estimate `/context` uses, and are labelled as estimates throughout. It is still roughly half the ~2,003-token prompt it replaced, and **every section can be switched off individually in `/context`**, with its cost and its consequence spelled out.

### Also in this release

- `system-prompts/current/` is now byte-identity tested against what actually ships. It had already drifted — it was publishing two prompts that are no longer sent and none of the ones that are.
- Five new research citations, plus a full re-audit of every arXiv ID in the file against arXiv's own metadata. All resolved; five carried a paper's informal title and were corrected.

### Not proved

No behavioural A/B against the old prompt on a real build yet. This rests on published research and Anthropic's reported testing, not our own numbers — and that stays written down until we have run it.

---

## Credit — the team at Anthropic behind Claude Fable 5

This release is built directly on prompting guidance published by **the team at Anthropic behind Claude Fable 5**, who did something the industry mostly does not: they wrote down what they learned about steering a long-horizon agent — in specific, testable, copyable language — and put it in public documentation instead of keeping it as a moat.

**Primary source:** [Prompting Claude Fable 5](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5#create-a-send-to-user-tool)

Directly sourced from that document: grounding progress claims against tool results before reporting; the "do not end a turn on a promise" rule; scope boundaries against unrequested actions; the instruction not to stop or hand off over context-budget worry; re-grounding the reader in a final summary after unattended work; delegation guidance for parallel sub-agents; and the one-lesson-per-file memory discipline. We adapted rather than copied — including one recommendation (`send_to_user`) we deliberately did **not** adopt, because this program does not have the problem it solves.

Also drawn on:
- [Prompting best practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices) — the model-agnostic companion guide
- [Effort](https://platform.claude.com/docs/en/build-with-claude/effort) — read as context for *why* the guidance reads the way it does; the parameter itself is Claude-API-specific and is not used by this fork

## The research

Every arXiv ID below was fetched and its title compared against arXiv's own metadata on 2026-07-29. None of these is a plausible-looking guess.

- **[The Compliance Gap: Why AI Systems Promise to Follow Process Instructions but Don't](https://arxiv.org/abs/2605.01771)** (2026-05-03, arXiv:2605.01771) — the most directly relevant paper to this fork's purpose. Names a third axis of AI honesty, distinct from factual truthfulness, where a model verbally agrees to a process constraint and then does something else. Its opening example is an agent told to read files individually that instead issues one batched call and reports success. **This is the paper behind "audit before reporting."**
- **[OctoBench: Benchmarking Scaffold-Aware Instruction Following in Repository-Grounded Agentic Coding](https://arxiv.org/abs/2601.10343)** (2026-01-15, arXiv:2601.10343) — 34 environments, 217 tasks, 7,098 checklist items, built to separate "solved the task" from "followed the scaffold's rules". Exactly the distinction a coding agent's system prompt lives or dies on.
- **[Natural-Language Agent Harnesses](https://arxiv.org/abs/2603.25723)** (2026-03-26, arXiv:2603.25723) — argues harness policy should be an editable natural-language document rather than logic buried in controller code, because buried logic cannot be inspected, compared, or ablated. The academic case for what `/prompts` and `/context` already do here.
- **[MAS-PromptBench: When Does Prompt Optimization Improve Multi-Agent LLM Systems?](https://arxiv.org/abs/2606.23664)** (2026-06-22, arXiv:2606.23664) — system prompts as the accessible optimization surface for multi-agent systems, and where optimizing them stops paying off. Context for the new `# delegation` section.
- **[AGENTIF: Benchmarking Instruction Following of Large Language Models in Agentic Scenarios](https://arxiv.org/abs/2505.16944)** (NeurIPS 2025 D&B spotlight, arXiv:2505.16944) — older than the six-month window and included anyway, because it is the measurement the others build on: real agentic instructions average 1,723 words and 11.9 constraints, and models follow them poorly. The reason we resisted simply appending every good idea to the prompt.

We also re-audited **every** pre-existing arXiv ID in [`RESEARCH-SOURCES.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/RESEARCH-SOURCES.md) against arXiv's metadata. All resolved. Five carried a paper's short or informal title rather than its full one and were corrected in place. No ID was wrong and nothing had been fabricated.

## Why the old prompts were the way they were

The research is consistent, and it contradicts how nearly everyone wrote prompts in 2023:

- Capitalising a word **splits it into more tokens** and dilutes attention rather than concentrating it.
- Threatening or emotionally loaded instructions measurably increase hedging and **false reports of success** — the exact failure this program exists to avoid.
- Piling on constraints makes instruction-following *worse*, not better.

None of which was obvious at the time, and none of which you would learn by being good at Go.

---

Full detail, both tracks: [`Changelogs/v0.2.0-release-notes.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/Changelogs/v0.2.0-release-notes.md)

**Install:** `sudo dpkg -i gorilla-opencode_0.2.0_amd64.deb`
