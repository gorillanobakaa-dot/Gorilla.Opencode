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
- Research on infinite agentic loops (stderr resonance, context
  blindness past ~50k tokens, missing failure primitive) informed the
  loop-guard design in `system-prompts/README.md`. The dossier that
  compiled this did not carry a single clean citation ID for that
  specific study, so it is described rather than given a fabricated
  reference — an honest description beats a fake link.

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
