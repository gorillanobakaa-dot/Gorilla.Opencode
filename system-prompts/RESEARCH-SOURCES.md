# Research sources — where the design came from

Per this project's Open Source Philosophy, we publish our inspiration so
you can read the primary research and judge the design for yourself. The
modern system prompt (`proposed/coder-modern.md`, shipped as the base
prompt) and the harness roadmap in `README.md` are built on the work
below. Go read it; disagree with us if it's wrong.

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
- **Uncovering Infinite Agentic Loops in LLM Agents** — Hou et al.
  (2026). arXiv:2607.01641 — https://arxiv.org/abs/2607.01641
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
- **Dual-Granularity Chain-of-Thought Compression for Controllable
  Reasoning** — (2026). arXiv:2601.20467 —
  https://arxiv.org/abs/2601.20467
  *NEW 2026: CoT compression with preserved correctness, addressing
  latency and memory costs of verbose reasoning traces.*

## Reducing sycophancy and hallucinations

- **Bridging Mechanistic Interpretability and Prompt Engineering with
  Gradient Ascent** — (2025). arXiv:2601.02896 —
  https://arxiv.org/abs/2601.02896
  *NEW 2025: Automatically discovered prompts reduce sycophancy from
  79.24% to 49.90% by grounding prompt discovery in mechanistically
  meaningful features.*
- **Engineering Consistent Procedures for Industrial LLM Hallucination
  Reduction** — (2026). arXiv:2603.10047 —
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
- **Reducing sycophancy in large language models** — (2026).
  arXiv:2602.23971 — https://arxiv.org/abs/2602.23971
  *NEW 2026: Converting non-questions into questions before answering
  significantly reduces sycophancy more than simply prompting "don't be
  sycophantic".*

## Reference prompts we learned from

- **asgeirtj/system_prompts_leaks** — observed production system prompts
  (including Claude Code) used to study modern agentic prompt structure.
  https://github.com/asgeirtj/system_prompts_leaks
  Kept **local for study, not redistributed** in this repo — see
  `system-prompts/README.md`.

## Provenance of the code itself

- Original OpenCode (MIT), Kujtim Hoxha —
  https://github.com/opencode-ai/opencode
- Crush (Charm, FSL) — https://github.com/charmbracelet/crush
- SST opencode (unrelated project, same name) —
  https://github.com/sst/opencode

---

*Citations reflect the research dossier compiled 2026-07. arXiv IDs for
The Prompt Report and the Emotional Stimuli paper were verified against
arXiv on 2026-07-20. If a link rots or an ID is wrong, open an issue —
accurate sourcing is the whole point.*
