# Local Code Review Toolkit

A vendor-agnostic, OS-agnostic, LLM-agnostic code review / static analysis /
security audit pipeline you run entirely on your own machine. No API keys
required, no cloud dependency unless you explicitly ask for one.

Built for (but not limited to): C/C++, Python, JavaScript/TypeScript, CSS,
Go, Rust, shell, and FreeMarker (`.ftl`) templates -- i.e. exactly the mix
you get across a Firefox checkout, a Linux kernel tree, a Go project, and a
Python/Rust vector DB.

## Files

| File | What it does |
|---|---|
| `tools_registry.py` | Single source of truth: every tool, how to check/install/run/parse it |
| `install_tools.py` | Sets your machine up -- checks first, installs only what's missing |
| `code_review.py` | The orchestrator -- point it at a directory or file, it does the rest |
| `findings.py` | One normalised `Finding` shape for every tool, + the position check |
| `heuristics.py` | In-house heuristics: brace parity, C++ dependent names, Firefox unified-build pollution, kernel spinlock sleeps, blast radius |
| `rules.py` | Review know-how per language, read from `rule_docs/` |
| `local_tier.py` | Optional: a small LOCAL model doing glue work only (never reviewing) |
| `llm_client.py` | ~100 lines, stdlib only, talks to ANY OpenAI- or Anthropic-compatible endpoint |
| `PLAYBOOK.md` | The literal, step-by-step version of everything below -- read this if you want to understand or run any single piece by hand |
| `configs/` | Minimal fallback eslint/stylelint configs, only used when a target project has none of its own |
| `rule_docs/` | 34 per-language review checklists (vendored, Apache-2.0 -- see its NOTICE.md) |
| `tests/` | `python3 tests/test_parsers.py` and `tests/test_position.py` |

## Two ways to run it

**For a person** -- the default. Rolling status, then a Markdown report.

**For an agent** -- `--audience agent`. stdout becomes a single JSON document
(schema `code-review/agent/1`), all progress moves to stderr, and the
findings arrive already normalised, position-verified and de-jargoned:

```bash
python3 code_review.py ~/src/proj --diff origin/main --audience agent
```

That is the intended path for a coding agent (Claude Code, opencode, …): the
agent supplies the reading and reasoning that static analysers cannot, and this
supplies the thirty tools plus the honest account of what did and didn't run.
No second model is installed anywhere. There's a matching skill at
`~/.agents/skills/code-review/SKILL.md`.

The JSON carries `findings`, `corroborated` (same line flagged by 2+ different
tools -- computed, not guessed), `trust` (what ran, what errored, what was
missing), `review_rules`, `manual_steps` and `checklists`.

## Quick start

```bash
cd code_review_toolkit
python3 code_review.py ~/src/myproject --doctor   # can this machine even do it?
python3 install_tools.py                          # one-time setup, safe to re-run
python3 code_review.py ~/src/myproject
```

### Start with `--doctor`

**This toolkit does not review code by itself.** It drives ~30 real analysers,
and they have to be installed. On a machine without them a run inspects
*nothing* — and a report full of "MISSING" looks a lot like a report that found
no problems. That confusion is the worst way for a review tool to fail, so
`--doctor` tells you up front:

- which analysers are present and which are missing, **per language actually in
  the target** (being told eslint is missing is noise when you're reviewing a
  kernel driver)
- how many need downloading and roughly how much disk that takes
- free space on the target filesystem, and whether that's enough — *before* you
  start a 2 GB install with 300 MB free
- which install route each one uses (apt / pip / npm / cargo / go)
- the exact command to fix it

```bash
python3 code_review.py ~/src/proj --doctor   # scoped to that project
python3 doctor.py --all                      # every language this toolkit knows
```

Exit code is 3 if no analyser is installed, so CI can gate on it. The same
report is printed automatically if the preflight gate trips mid-run.

That prints a rolling status line per tool as it finishes, then writes:

```
~/src/myproject/.code_review/<timestamp>/
├── REPORT.md              <- start here
├── report.json             <- same data, machine-readable
├── <tool>__<file>.txt       <- every tool's complete, unedited output
└── ...
```

## What actually happens

1. **Detects your project type** from marker files -- a mozilla-central
   checkout, a Linux kernel tree, a Go module, a Rust crate, or a Python
   project -- and prefers each tree's own native tooling where one exists
   (`./mach lint` for Firefox, `checkpatch.pl`/`sparse`/`coccicheck` for the
   kernel) over generic tools that would just be noisier and slower.
2. **Figures out what's in scope** -- git-tracked files by default (so a
   700,000-file Firefox tree doesn't turn into 700,000 jobs), or exactly
   your changes with `--diff origin/main` / `--diff HEAD~1`.
3. **Runs everything relevant in parallel**, staged: fast linters first,
   then static analysis/security, then -- only if something looked
   security-shaped -- a deeper conditional pass on just the flagged files.
4. **Never pretends to auto-run what it can't safely automate.** Things
   like a full Firefox/kernel rebuild under `scan-build`, or `valgrind`
   against a specific compiled binary, get printed as exact copy-paste
   commands instead of a guessed, probably-wrong invocation.
5. **Distinguishes "clean" from "broken."** If a tool errors out (blocked
   network, bad config) instead of actually running, that's flagged
   explicitly -- it's never silently reported the same as "found nothing."
6. **Optionally hands the whole aggregate to an LLM** -- local (Ollama,
   llama.cpp, LM Studio) or remote, any vendor -- for a prioritized,
   plain-English synthesis. Entirely optional; nothing leaves your machine
   unless you pass `--llm-endpoint` yourself.

## Common commands

```bash
# just your uncommitted-vs-upstream changes
python3 code_review.py ~/src/proj --diff origin/main

# force the deep/security-focused stage on everything, not just auto-flagged files
python3 code_review.py ~/src/proj --deep

# a single file you just patched
python3 code_review.py ~/src/firefox/dom/canvas/WebGLContext.cpp

# enable clang-tidy (needs a compile database)
bear -- make -j$(nproc)                       # generates compile_commands.json
python3 code_review.py ~/src/proj --compile-commands-dir .

# hand it to a local model afterwards
python3 code_review.py ~/src/proj \
  --llm-endpoint http://localhost:11434/v1/chat/completions \
  --llm-model llama3 --llm-api-style openai
```

See `PLAYBOOK.md` for the full literal walkthrough, including exact
per-project recipes for Firefox and the kernel specifically, and the
troubleshooting section (PEP 668 / PATH / conflicting installs).

## Design choices worth knowing about

- **No hardcoded LLM vendor, ever.** `llm_client.py` is ~100 lines of
  standard-library `urllib` code. It works identically with a 0.2B model on
  a Raspberry Pi and a frontier cloud model -- same JSON in, same text out.
- **Every raw tool log is kept, untouched.** The Markdown/JSON reports and
  any LLM synthesis are summaries *on top of* the logs, never a replacement
  for them.
- **Escalation is evidence-based, not blanket.** Stage 3 (the expensive,
  noisy, "run everything at maximum strictness" tier) only fires on files
  where something already looked security-relevant, or when you pass
  `--deep` yourself.
- **Findings are data, not prose.** Every tool has a `parse_output` function in
  the registry, so `report.json` contains the findings themselves -- not just
  how many there were and a path to a log. That one change is what lets an
  agent consume this directly, makes "two tools agree" a set operation instead
  of an LLM's impression, and makes the position check possible at all.
- **Position check, no model required.** Every finding must point at a line that
  really exists, and it carries that line's source when it ships. Findings that
  can't be located are dropped. If a finding arrives *with* a claimed snippet
  (which is how a model reports), the claim is checked against the real source
  and a small drift is corrected rather than discarded.
- **Severity means something.** flake8's `E501` is style; its `F821` is an
  error. Scoring both the same buries the bug that crashes under three hundred
  long lines, so the parsers encode each tool's own severity scheme rather than
  guessing from a letter.
- **Know-how lives in `rule_docs/`, not in a prompt.** The registry knows how to
  *run* thirty analysers; the rule docs describe what to *look for* that no
  analyser checks. Plain Markdown, so a human can edit it and any reader --
  agent, local model, person -- gets the same text.
- **The local model never reviews.** `--local-glue` uses a small local model
  (default `qwen2.5-coder:1.5b` via ollama) for exactly two jobs: rewriting tool
  jargon into plain English, and flagging probable noise. It cannot find, drop
  or reorder a finding. A 1.5B model's disagreement is not evidence against a
  real analyser, so its triage is advisory and the finding still ships.
- **Preflight gate — refuses to run against a bare machine.** Before doing
  any work, it checks whether the tools that *would* run for this project are
  actually installed. If **none** of the analysis tools are present, it stops
  with exit code 3 and tells you to run `install_tools.py` first — instead of
  producing an all-"MISSING" report that a low-reasoning caller could mistake
  for "no problems found" (the worst possible false negative). Override with
  `--skip-preflight` only if you know what you're doing.
