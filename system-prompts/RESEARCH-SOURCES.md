# Research sources — where the design came from

Per this project's Open Source Philosophy, we publish our inspiration so
you can read the primary research and judge the design for yourself. The
shipped system prompts (`current/`) and the harness roadmap in
`README.md` are built on the work below. Go read it; disagree with us if
it's wrong.

## Primary source for the 2026-07-29 rewrite — with thanks

The 2026-07-29 rewrite of all four shipped prompts is built directly on
work published by **the team at Anthropic behind Claude Fable 5**, who
did something the industry mostly does not: they wrote down what they
learned about steering a long-horizon agent, in specific, testable,
copyable language, and put it in public documentation instead of keeping
it as a moat.

- **Prompting Claude Fable 5** — Anthropic (2026). The section that
  named the `send_to_user` pattern, and the guide as a whole:
  https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5#create-a-send-to-user-tool
  Full guide:
  https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5
  *Directly sourced from this document: grounding progress claims against
  tool results before reporting; the "do not end a turn on a promise"
  rule; scope boundaries against unrequested actions; the instruction not
  to stop or hand off over context-budget worry; re-grounding the reader
  in a final summary after unattended work; delegation guidance for
  parallel sub-agents; and the one-lesson-per-file memory discipline.
  We adapted rather than copied — see `README.md` for what we changed,
  and for the one recommendation (`send_to_user`) we deliberately did
  not adopt because this program does not have the problem it solves.*
- **Prompting best practices** — Anthropic (2026), the model-agnostic
  companion guide:
  https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices
- **Effort** — Anthropic (2026). Read as context for *why* the guidance
  reads the way it does; the parameter itself is Claude-API-specific and
  is not used by this fork:
  https://platform.claude.com/docs/en/build-with-claude/effort

*All four URLs fetched and confirmed live on 2026-07-29.*

## Instruction following and compliance in agentic settings (2026)

The four papers below are the recent evidence base for the rewrite. Each
arXiv ID was fetched and its title confirmed against arXiv on
2026-07-29 — none of these is a plausible-looking guess.

- **The Compliance Gap: Why AI Systems Promise to Follow Process
  Instructions but Don't** — (2026-05-03). arXiv:2605.01771 —
  https://arxiv.org/abs/2605.01771
  *The most directly relevant paper to this fork's purpose. Names a third
  axis of AI honesty — distinct from factual truthfulness — where a model
  verbally agrees to a process constraint and then does something else.
  Its opening example is an agent told to read files individually that
  instead issues one batched call and reports success. This is why the
  `# honesty` section demands a tool result rather than an assurance.*
- **OctoBench: Benchmarking Scaffold-Aware Instruction Following in
  Repository-Grounded Agentic Coding** — (2026-01-15). arXiv:2601.10343 —
  https://arxiv.org/abs/2601.10343
  *34 environments, 217 tasks, 7,098 checklist items, built to separate
  "solved the task" from "followed the scaffold's rules". Exactly the
  distinction a coding agent's system prompt lives or dies on.*
- **Natural-Language Agent Harnesses** — (2026-03-26). arXiv:2603.25723 —
  https://arxiv.org/abs/2603.25723
  *Argues harness policy should be an editable natural-language document
  rather than logic buried in controller code, because buried logic
  cannot be inspected, compared, or ablated. This is the academic case
  for what `/prompts` and `/context` already do here.*
- **MAS-PromptBench: When Does Prompt Optimization Improve Multi-Agent
  LLM Systems?** — (2026-06-22). arXiv:2606.23664 —
  https://arxiv.org/abs/2606.23664
  *System prompts as the accessible optimization surface for multi-agent
  systems, and where optimizing them stops paying off. Context for the
  `# delegation` section and for the sub-agent leash in `/context`.*
- **AGENTIF: Benchmarking Instruction Following of Large Language Models
  in Agentic Scenarios** — Qi et al. (2025), NeurIPS 2025 Datasets &
  Benchmarks spotlight. arXiv:2505.16944 —
  https://arxiv.org/abs/2505.16944
  *Older than the six-month window and included anyway, because it is the
  measurement everything above builds on: real agentic instructions
  average 1,723 words and 11.9 constraints, and models follow them
  poorly. The reason we resisted simply appending every good idea to the
  prompt.*

## Prompt hygiene — letter case and typographic emphasis

*Section renamed 2026-08-06. It was previously headed "the case against
shouting" and carried exactly one citation, about agent documentation
practices, which argued nothing of the sort. The header asserted a position
with no source under it. The correction is below and it partly goes against
us.*

- **Attention is Case-Sensitive** — Dillitzer, Sohn, Corso & Auerbach
  (2026-08-04). arXiv:2608.03711 — https://arxiv.org/abs/2608.03711
  *The direct measurement of the thing this project asserted without one.
  Holds semantics and word order fixed and varies only letter case, across
  15 schemes, 13 models and 3 tokenizer families (BPE, SentencePiece,
  byte-level BPE), measuring attention mass and downstream accuracy as
  separate quantities. Four results that bear on our prompt:*
  - *Uppercase target spans against lowercase context (their TE1) are the
    only reliably **productive** intervention: +1.85 pp accuracy on the
    discovery model, up to +8.95 pp across the set.*
  - *`aLtErNaTiNg` case pulls the **most** attention (+2.77 pp) and **loses**
    accuracy (−2.88 pp mean, −13.96 pp on LLaMA-3.1-8B). Attention
    concentration and task performance are not the same axis — the paper's
    central point, and a trap for anyone optimising a prompt by eye.*
  - *Global uniform casing — all-caps or all-lowercase throughout — does
    close to nothing (≤1.16 pp). Emphasis is a contrast effect. Applied
    everywhere it is applied nowhere.*
  - *Reasoning models show <±0.5 pp sensitivity; the deliberation phase
    filters typographic salience. The effect is universal in the nine
    non-reasoning models. **This fork mostly drives local and open-weight
    models, so the effect applies to us more than it applies to
    Claude-backed tooling.***

  *What it does NOT license: reintroducing seventeen `IMPORTANT`s. The
  productive regime is sparse caps in a lowercase field, which is what
  `current/coder-modern.md` already has (2 spans / 816 words). Their
  finding explains why that shape works; it does not argue for more.*
  *Caveat: Latin-script only, no causal mediation analysis, and no
  pre-registration behind a 15 × 13 × 6 sweep.*

- **Large Language Models Understand and Can be Enhanced by Emotional
  Stimuli** ("EmotionPrompt") — Cheng Li et al. (2023). arXiv:2307.11760
  — https://arxiv.org/abs/2307.11760
  *Listed again here, under the section it was actually used to support,
  because that use was wrong. This paper is about emotional and
  motivational phrasing ("this is very important to my career"). We
  extended it to letter case, which it never studied, and stated the
  extension as a research finding in `REPORT.md` and `README.md`. Both are
  corrected as of 2026-08-06. The token-fragmentation argument for cutting
  caps was always separate and still stands.*

- **The 2025 AI Agent Index: Documenting Technical and Safety Features of
  Deployed Agentic AI Systems** — (2026-02-19). arXiv:2602.17753 —
  https://arxiv.org/abs/2602.17753
  *Survey of 30 deployed agents and how inconsistently they document what
  they actually do. Part of why this directory exists — and this section is
  a worked example of the failure it describes.*


## Agents for repository-scale code & system compilation

- **SWE-agent: Agent-Computer Interfaces Enable Automated Software
  Engineering** — Yang et al. (2024). arXiv:2405.15793 —
  https://arxiv.org/abs/2405.15793
- **CodePlan: Repository-Level Code Editing via Planning and LLMs** —
  Bairi et al. (2024), ACM TOSEM. arXiv:2309.12499 —
  https://arxiv.org/abs/2309.12499
- **CodeR: Issue Resolving with Multi-Agent System via Task
  Decomposition** — Chen et al. (2024). arXiv:2406.01304 —
  https://arxiv.org/abs/2406.01304
- **Adaptive Graph of Thoughts (AGoT): Test-Time Adaptive Reasoning** —
  Pandey et al. (2025). arXiv:2502.05078 —
  https://arxiv.org/abs/2502.05078

## Token cost, formatting bloat, and "agent anxiety"

- **LLMLingua-2: Data Distillation for Efficient and Faithful
  Task-Agnostic Prompt Compression** — Pan et al. (2024).
  arXiv:2403.12968 — https://arxiv.org/abs/2403.12968 (and the original
  LLMLingua, Jiang et al. 2023, arXiv:2310.05736)
- **The Prompt Report: A Systematic Survey of Prompt Engineering
  Techniques** — Schulhoff et al. (2024). arXiv:2406.06608 —
  https://arxiv.org/abs/2406.06608 *(title + author verified)*
- **Large Language Models Cannot Self-Correct Reasoning Yet** —
  Huang et al. (2023). arXiv:2310.01798 —
  https://arxiv.org/abs/2310.01798
- **Large Language Models Understand and Can be Enhanced by Emotional
  Stimuli** (the "EmotionPrompt" paper) — Cheng Li et al. (2023).
  arXiv:2307.11760 — https://arxiv.org/abs/2307.11760 *(title corrected
  and verified 2026-07-20; the informal name "EmotionPrompt" is the
  technique, not the paper title)*

## Agentic loops, loop detection, and exit ramps

- **Reflexion: Language Agents with Verbal Reinforcement Learning** —
  Shinn et al. (2023), NeurIPS 2023. arXiv:2303.11366 —
  https://arxiv.org/abs/2303.11366
- **When Agents Do Not Stop: Uncovering Infinite Agentic Loops in LLM
  Agents** — Hou et al. (2026). arXiv:2607.01641 — https://arxiv.org/abs/2607.01641
  *NEW 2026: Identifies IALs (Infinite Agentic Loops) as distinct from
  ordinary programming loops. Proposes static analysis via Agentic Loop
  Dependence Graph (ALDG). 91.9% precision detecting real IAL failures
  across 47 agent projects.*
- **The Dual-State Architecture for Reliable LLM Agents** — (2026).
  arXiv:2512.20660 — https://arxiv.org/abs/2512.20660
  *NEW 2026: Three-level recovery hierarchy to prevent O(R^K) retry
  explosion: context refinement, informed backtracking with stagnation
  detection, and human escalation.*
- Research on infinite agentic loops (stderr resonance, context
  blindness past ~50k tokens, missing failure primitive) informed the
  loop-guard design in `system-prompts/README.md`. The dossier that
  compiled this did not carry a single clean citation ID for that
  specific study, so it is described rather than given a fabricated
  reference — an honest description beats a fake link.

## Token compression and reasoning efficiency

- **Compressed Chain of Thought: Efficient Reasoning through Dense
  Representations** — (2024). arXiv:2412.13171 —
  https://arxiv.org/abs/2412.13171
  *NEW 2024: CCoT framework for variable-length compressed reasoning
  chains. Reduces latency while preserving accuracy through contentful
  contemplation tokens.*
- **Accelerating Chain-of-Thought Reasoning** — (2025).
  arXiv:2505.08392 — https://arxiv.org/abs/2505.08392
  *NEW 2025: Achieves 45%+ CoT token reduction with 1.6-2.0× inference
  speedup while maintaining reasoning accuracy.*
- **CtrlCoT: Dual-Granularity Chain-of-Thought Compression for
  Controllable Reasoning** — (2026). arXiv:2601.20467 —
  https://arxiv.org/abs/2601.20467
  *NEW 2026: CoT compression with preserved correctness, addressing
  latency and memory costs of verbose reasoning traces.*

## Reducing sycophancy and hallucinations

- **Bridging Mechanistic Interpretability and Prompt Engineering with
  Gradient Ascent for Interpretable Persona Control** — (2025).
  arXiv:2601.02896 —
  https://arxiv.org/abs/2601.02896
  *NEW 2025: Automatically discovered prompts reduce sycophancy from
  79.24% to 49.90% by grounding prompt discovery in mechanistically
  meaningful features.*
- **Toward Epistemic Stability: Engineering Consistent Procedures for
  Industrial LLM Hallucination Reduction** — (2026). arXiv:2603.10047 —
  https://arxiv.org/abs/2603.10047
  *NEW 2026: Five prompt engineering strategies for repeatable, grounded
  results: Iterative Similarity Convergence, Decomposed Model-Agnostic
  Prompting, Single-Task Agent Specialization, Enhanced Data Registry,
  Domain Glossary Injection.*
- **Optimizing LLM Prompt Engineering with DSPy Based Declarative
  Learning** — (2026). arXiv:2604.04869 —
  https://arxiv.org/abs/2604.04869
  *NEW 2026: Shows 30-45% improvement in factual accuracy and ~25%
  reduction in hallucination rates through declarative prompt
  optimization.*
- **How RLHF Amplifies Sycophancy** — (2026). arXiv:2602.01002 —
  https://arxiv.org/abs/2602.01002
  *NEW 2026: Documents how preference-based post-training increases
  sycophantic behavior, causing models to affirm user beliefs even when
  factually incorrect.*
- **Ask don't tell: Reducing sycophancy in large language models** —
  (2026). arXiv:2602.23971 — https://arxiv.org/abs/2602.23971
  *NEW 2026: Converting non-questions into questions before answering
  significantly reduces sycophancy more than simply prompting "don't be
  sycophantic".*

## The 2026-07-31 sweep — change reporting and impact assessment

Compiled for the `# change reporting` section added to `coder-modern.md` on
2026-07-31. Four arXiv API queries: chain-of-thought faithfulness, structured
output vs reasoning, coding-agent impact assessment, and self-explanation /
overclaiming.

**Two tiers of verification, stated plainly.** The twelve marked **[id-verified]**
were re-fetched individually through the arXiv `id_list` API and their exact
titles, first authors and dates compared against arXiv's own metadata. The rest
came back inside arXiv API *query* responses — so the titles are arXiv's
metadata rather than anyone's recollection, but each was not separately
re-fetched. Nothing here is a guess, and the distinction is recorded rather than
glossed.

### The case FOR mandatory impact reporting

- **TDAD: Test-Driven Agentic Development — Reducing Code Regressions in AI
  Coding Agents via Graph-Based Impact Analysis** — Alonso et al. (2026-03-18).
  arXiv:2603.17973 — https://arxiv.org/abs/2603.17973 **[id-verified]**
  *The load-bearing citation. Regressions 6.08% → 1.82% (−70%); issue resolution
  24% → 32%. Decisively: it **outperformed procedural TDD instructions** — impact
  analysis delivered as a computed artifact beat the same idea delivered as a
  prose rule. This is why the shipped section says "compute do not narrate".*
- **What Breaks When LLMs Code? Characterizing Operational Safety Failures of
  Agentic Code Assistants** — Al Hasan et al. (2026-05-29). arXiv:2605.30777 —
  https://arxiv.org/abs/2605.30777 **[id-verified]**
  *547 real failures, 33 risk types, 326 (59.6%) high or critical. Dominant
  categories: constraint violations, destructive operations, and **deceptive
  success reporting**. The empirical basis for "capabilities lost" being
  mandatory rather than optional.*
- **Capability Advertisement as a Market for Lemons: A Trust Layer for
  Heterogeneous Agent Networks** — Mittal et al. (2026-06-02). arXiv:2606.03034 —
  https://arxiv.org/abs/2606.03034 **[id-verified]**
  *Names the exact failure mode: agents that describe themselves accurately and
  behave incorrectly.*
- **SABER: Benchmarking Operational Safety of LLM Coding Agents in Stateful
  Project Workspaces** — (2026-05-31). arXiv:2606.01317 —
  https://arxiv.org/abs/2606.01317
  *>54% harmful safety-violation rates even for top models in realistic project
  environments.*
- **Trust but Verify? Uncovering the Security Debt of Autonomous Coding Agents**
  — (2026-07-14). arXiv:2607.12428 — https://arxiv.org/abs/2607.12428
  *38.9% of agent-generated PRs contain at least one security smell.*

### The case AGAINST imposing a rigid schema — why the section is tiered

These are the reason the shipped wording says *render after the work* and tiers
by blast radius, instead of demanding four fixed sections on every edit.

- **The Constraint Tax: Measuring Validity-Correctness Tradeoffs in Structured
  Outputs for Small Language Models** — Ray et al. (2026-05-20).
  arXiv:2605.26128 — https://arxiv.org/abs/2605.26128 **[id-verified]**
  *Hard schema constraints raised validity 61.5% → 100% while dropping answer
  accuracy 19.7% → 11.0%; on a deterministic tool-call task executable accuracy
  fell 91.5% → 48.0%. You buy perfectly-shaped answers that are wrong more often.
  **Directly relevant to this fork**, whose users run Ollama and small NIM models.*
- **Capacity, Not Format: Rethinking Structured Reasoning Failures** — Fan et al.
  (2026-06-08). arXiv:2606.09410 — https://arxiv.org/abs/2606.09410
  **[id-verified]**
  *JSON constraints cost Haiku 36.2pp and GPT-4o-mini 28.0pp, independent of
  token limits. **Delayed structure recovers 80–87%.** The source of "render
  after the work not instead of it".*
- **Drop the Act: Probe-Filtered RL for Faithful Chain-of-Thought Reasoning** —
  Parekh et al. (2026-05-12). arXiv:2605.11467 —
  https://arxiv.org/abs/2605.11467 **[id-verified]**
  *Coins **"post-commitment theater"**: deliberative-looking steps contributing
  nothing to correctness. A mandatory four-section form is a theater generator
  unless every section is falsifiable — hence the falsifiability rule.*
- **DisasterBench: Benchmarking LLM Planning under Typed Tool Interface
  Constraints** — Chen et al. (2026-05-27). arXiv:2605.27957 —
  https://arxiv.org/abs/2605.27957 **[id-verified]**
  *Verbose intermediate reasoning creates instruction clash with structured
  output requirements; semantic reasoning and execution-grounded coordination are
  distinct bottlenecks.*
- **GraphRAG on Consumer Hardware: Local LLM EHR Schema Retrieval** —
  (2026-05-20). arXiv:2605.20815 — https://arxiv.org/abs/2605.20815
  *Models below ~7B cannot reliably produce valid structured output at all. The
  hardware floor this fork actually targets.*
- **Schema Key Wording as an Instruction Channel in Constrained Decoding** —
  (2026-04-28). arXiv:2604.14862 — https://arxiv.org/abs/2604.14862
  *Schema keys act as an implicit instruction channel; effects are
  model-dependent. Section headings are not neutral packaging.*

### Self-reports are weak evidence — why every claim needs a pointer

- **DEMM-Bench: A Cross-Regime Benchmark for Agent-Runtime Governance-Evidence
  Sufficiency** — Solozobov et al. (2026-05-30). arXiv:2606.20634 —
  https://arxiv.org/abs/2606.20634 **[id-verified]**
  *The most important number in this sweep: **trace-present and schema-present
  baselines overclaim on 75% of cases**, ledger-present on 50%. Having the
  evidence available does not stop overclaiming; only property-level scoring
  reached zero. Availability is not verification.*
- **Self-CTRL: Self-Consistency Training with Reinforcement Learning** — Pres et
  al. (2026-06-16). arXiv:2606.18327 — https://arxiv.org/abs/2606.18327
  **[id-verified]**
  *Baseline correlation between a model's self-report and its actual behaviour:
  R²=0.24, rising to 0.64 only after consistency training. An unverified
  self-report is close to noise.*
- **Can we trust LLM Self-Explanations for Entity Resolution?** — Teofili et al.
  (2026-05-31). arXiv:2606.01210 — https://arxiv.org/abs/2606.01210
  **[id-verified]**
  *Self-explanations are "unstable, weakly faithful, and poorly aligned with
  counterfactual evidence".*
- **Agent-Safety Evaluations as Load-Bearing Evidence: A Vendor-Neutral,
  Cross-Harness Reconstructability Metric** — Solozobov et al. (2026-07-14).
  arXiv:2607.12469 — https://arxiv.org/abs/2607.12469 **[id-verified]**
  *Evidence sufficiency spans 0.458–0.833 across four inputs that read
  identically on the surface. The formal version of "point at the tool result".*
- **Why Models Know But Don't Say: Chain-of-Thought Faithfulness Divergence
  Between Thinking Tokens and Answers in Open-Weight Reasoning Models** — Young
  et al. (2026-03-27). arXiv:2603.26410 — https://arxiv.org/abs/2603.26410
  **[id-verified]**
  *55.4% thinking-vs-answer divergence; model variation from 94.7% down to 19.6%.
  What the model concluded internally is not what it tells you.*
- **Stability vs. Manipulability: Evaluating Robustness Under Post-Decision
  Interaction in LLM Judges** — (2026-06-03). arXiv:2606.05384 —
  https://arxiv.org/abs/2606.05384
  *Post-decision justifications show low overlap with the original reasoning —
  post-hoc rationalization under challenge.*
- **The Fragility of Chain-of-Thought Monitoring Across Typologically Diverse
  Languages** — (2026-05-27). arXiv:2605.27901 —
  https://arxiv.org/abs/2605.27901
  *95.9% unfaithfulness including strategic post-hoc rationalization.*
- **From Plausible to Actionable: A Position on LLM Self-Explanations** —
  (2026-07-17). arXiv:2607.15957 — https://arxiv.org/abs/2607.15957
  *The counterweight: self-explanations can be "plausible, questionably faithful,
  and yet highly actionable". Included because it argues against the strongest
  reading of the papers above.*
- **Training Large Language Models for Self-Explanation Faithfulness** —
  (2026-07-23). arXiv:2607.21090 — https://arxiv.org/abs/2607.21090
- **From Forecasting Leaderboards to Deployment Decisions** — (2026-06-23).
  arXiv:2606.24996 — https://arxiv.org/abs/2606.24996
  *Certification protocol blocking overclaiming via locked audits; eliminated 155
  apparent winner inversions.*

### Chain-of-thought faithfulness — the wider 2025–2026 base

- **CASE: Causal Alignment and Structural Enforcement for Improving
  Chain-of-Thought Faithfulness** — (2026-07-21). arXiv:2607.18820 —
  https://arxiv.org/abs/2607.18820
- **Investigating the Interplay between Contextual and Parametric
  Chain-of-Thought Faithfulness under Optimization** — (2026-05-24).
  arXiv:2605.24960 — https://arxiv.org/abs/2605.24960
- **Counterfactual Simulation Training for Chain-of-Thought Faithfulness** —
  (2026-02-24). arXiv:2602.20710 — https://arxiv.org/abs/2602.20710
- **GeoFaith: A Spatio-Temporal Dual View of Faithful Chain-of-Thought** —
  (2026-05-26). arXiv:2605.26893 — https://arxiv.org/abs/2605.26893
- **FRIT: Using Causal Importance to Improve Chain-of-Thought Faithfulness** —
  (2025-09-10). arXiv:2509.13334 — https://arxiv.org/abs/2509.13334
  *Reasoning steps "often fail to causally influence the final answer".*
- **A Closer Look at Bias and Chain-of-Thought Faithfulness of Large (Vision)
  Language Models** — (2025-05-29). arXiv:2505.23945 —
  https://arxiv.org/abs/2505.23945
- **Measuring Chain of Thought Faithfulness by Unlearning Reasoning Steps** —
  (2025-02-20). arXiv:2502.14829 — https://arxiv.org/abs/2502.14829
- **ConsisGuard: Aligning Safety Deliberation with Policy Enforcement in LLM
  Guardrails** — (2026-05-29). arXiv:2605.31073 —
  https://arxiv.org/abs/2605.31073
  *Models that recognise a problem in reasoning and then emit the safe label
  anyway — the same divergence in a safety setting.*
- **Universal Activation Verbalizer** — (2026-05-25). arXiv:2605.25903 —
  https://arxiv.org/abs/2605.25903

### Coding agents in the field — context for what the harness is up against

- **Don't Blame the Large Language Model: How Agent Harness Evolution Shapes
  Coding Agent Quality** — (2026-07-04). arXiv:2607.03691 —
  https://arxiv.org/abs/2607.03691
  *Harness updates, not just model changes, drive measurable quality swings. The
  direct argument for versioning and publishing prompts the way this directory
  does.*
- **Do AI Coding Agents Log Like Humans? An Empirical Study** — (2026-04-10).
  arXiv:2604.09409 — https://arxiv.org/abs/2604.09409
  *Agents change logging less than humans; explicit logging instructions are rare
  **and ineffective**; humans fix 72.5% of observability issues. A caution that
  telling an agent to report more does not automatically make it report better.*
- **How Do AI Coding Agents Contribute to Software Development? An Empirical
  Study of Agentic Pull Requests** — (2026-07-23). arXiv:2607.21832 —
  https://arxiv.org/abs/2607.21832
- **AIDev: Studying AI Coding Agents on GitHub** — (2026-02-09).
  arXiv:2602.09185 — https://arxiv.org/abs/2602.09185
  *932,791 agentic PRs across 116,211 repositories.*
- **Adoption and Impact of Command-Line AI Coding Agents: A Study of Microsoft's
  Early 2026 Rollout** — (2026-07-01). arXiv:2607.01418 —
  https://arxiv.org/abs/2607.01418
- **From Conversation to Contribution: Characterizing Coding Agents in
  Open-Source Software** — (2026-07-06). arXiv:2607.05677 —
  https://arxiv.org/abs/2607.05677
  *Developers perceive AI-generated code as harder to maintain.*
- **Cross-Model Cross-Language AI Coding Agent Performance: Accuracy and Speed of
  Parallel CLRS Algorithms** — (2026-07-26). arXiv:2607.26083 —
  https://arxiv.org/abs/2607.26083
- **IssueTrojanBench: Benchmarking AI Coding Agents Against Malicious Issue
  Requests** — (2026-07-22). arXiv:2607.20759 —
  https://arxiv.org/abs/2607.20759 *(66.5% of malicious issues penetrate all
  guardrails)*
- **Agent Data Injection Attacks are Realistic Threats to AI Agents** —
  (2026-07-06). arXiv:2607.05120 — https://arxiv.org/abs/2607.05120
- **MOSAIC: Knowledge-Guided CLI Command Composition Attack in LLM Coding
  Agents** — (2026-07-03). arXiv:2607.02857 — https://arxiv.org/abs/2607.02857
  *Benign CLI commands composing into dangerous state relations.*

### Verification and evidence design — adjacent, kept for the next revision

- **SEVA: Self-Evolving Verification Agent with Process Reward** — (2026-06-29).
  arXiv:2606.29713 — https://arxiv.org/abs/2606.29713
- **Distributional Energy-Based Models for Uncertainty-Aware Structured
  Reasoning** — (2026-05-15). arXiv:2605.18871 —
  https://arxiv.org/abs/2605.18871
  *"Structural verification wins when constraints are checkable" — the principle
  behind preferring a computed blast radius to a narrated one.*
- **Think Through Uncertainty: Long-Form Factuality via Reasoning Calibration** —
  (2026-04-13). arXiv:2604.12046 — https://arxiv.org/abs/2604.12046
  *Pairing each claim with a confidence estimate improved factuality by up to
  39.9%.*
- **Representation Matters: Program Representations for LLM Vulnerability
  Reasoning** — (2026-06-24). arXiv:2606.25356 —
  https://arxiv.org/abs/2606.25356
  *Graph representations 83.2% vs raw source 53.5%; documents a "context dilution
  effect" when raw source is added to structural evidence.*
- **A Deterministic Agentic Workflow for HS Tariff Classification** —
  (2026-05-14). arXiv:2605.14857 — https://arxiv.org/abs/2605.14857
  *Stage-wise structured outputs giving interpretability by design.*
- **Finetuning with Scientific Data Increases Hallucinations** — (2026-06-19).
  arXiv:2606.21359 — https://arxiv.org/abs/2606.21359
- **Do LLMs Build World Models? MentalMap Multilingual Spatial Reasoning** —
  (2026-05-27). arXiv:2605.28277 — https://arxiv.org/abs/2605.28277
  *A universal "reasoning cliff" with structured-output failures at the cliff
  edge.*

*Queries also returned domain-specific hits — medical imaging, speech
recognition, anomaly detection, tariff and IPO document analysis — that use
structured output incidentally rather than studying it. Those were read and
excluded as off-topic rather than padded into this list; the two that carry a
transferable finding (2605.14857, 2604.12046) are kept above and labelled with
what that finding is.*

*Sweep performed 2026-07-31 via the arXiv API. What this sweep did NOT do:
measure any of it on this fork. The `# change reporting` section's tiering and
its "render after the work" wording are reasoned from the papers above, not
tested here. If a small local model starts producing well-formatted wrong
answers, that section is the first suspect and `/context` turns it off.*

## Reference prompts we learned from

- **asgeirtj/system_prompts_leaks** — observed production system prompts
  (including Claude Code) used to study modern agentic prompt structure.
  https://github.com/asgeirtj/system_prompts_leaks
  Kept **local for study, not redistributed** in this repo — see
  `system-prompts/README.md`.

## Provenance of the code itself

- Gorilla OpenCode (this fork) —
  https://github.com/gorillanobakaa-dot/Gorilla.Opencode
- Crush (Charm, FSL) — https://github.com/charmbracelet/crush
- SST opencode (unrelated project, same name) —
  https://github.com/sst/opencode

---

*Citations reflect the research dossier compiled 2026-07. arXiv IDs for
The Prompt Report and the Emotional Stimuli paper were verified against
arXiv on 2026-07-20.*

*Re-audited 2026-07-29 for the prompt rewrite: every arXiv ID in
this file was fetched and its title compared against arXiv's own
metadata. All resolved. Five entries had the paper's short or informal
title rather than its full one and were corrected in place — 2607.01641,
2601.20467, 2603.10047, 2601.02896 and 2602.23971. No ID was wrong and
nothing was fabricated. The five Anthropic documentation URLs were
fetched the same day and returned 200.*

*If a link rots or an ID is wrong, open an issue — accurate sourcing is
the whole point.*
