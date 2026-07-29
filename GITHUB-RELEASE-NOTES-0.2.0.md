# Comprehensive Dual-Track Forensic Audit & Research Analysis

**Target Release:** `Gorilla.Opencode` (`v0.2.0`)  
**Methodology:** Gorilla Dual-Track Standard (Layman Track + Developer Track)  
**Audit Scope:** Research Foundations, System Prompts, Go Engine Mechanics, Operational Trade-offs, & Omissions Analysis

---

# TRACK ONE: THE LAYMAN TRACK
*Written in plain language for users, field teams, and non-technical reviewers. No jargon assumed.*

---

## 1. What Is This Software and Why Does It Need System Prompts?

When you ask an AI coding assistant to help you write or fix software, it doesn't start with a blank mind. Before your prompt is ever processed, the application quietly sends a secret instruction sheet called a **System Prompt**.

This system prompt acts as the AI's "operating manual." It tells the AI:
- Who it is (e.g., a systems software engineer).
- How it must behave (e.g., read files before editing them).
- What it is allowed and forbidden to do (e.g., do not delete files on a hunch).
- How it should report its progress to you.

If the system prompt is poorly written, the AI acts foolishly: it might promise that a complex build succeeded when it didn't even check, or it might get stuck in an endless loop retrying the exact same broken step over and over.

In version `v0.2.0` of `Gorilla.Opencode`, all system prompts were completely rewritten based on cutting-edge research from 2024–2026.

---

## 2. Real-World Constraints: Token Costs, Remote Field Teams & Satellite Connections

Every word sent to or from an AI costs **tokens** (computational word fragments). Designing system prompts is an extreme balancing act dictated by three severe real-world constraints:

1. **Token Expenditure & Financial Cost:**  
   Every single character in a system prompt is re-sent to the AI on *every single turn* of the conversation. If a prompt is too long, users burn through API budgets in minutes.
2. **Bandwidth Limits for Emergency, Rescue & Remote Field Teams:**  
   Many people using this software are operating in high-risk, extreme, or remote environments (such as disaster recovery, maritime, or military operations). They rely on satellite internet connections measured in **single-digit kilobytes per second (KB/s)**. If we send massive 50,000-token system prompts, the connection stalls and the software becomes unusable.
3. **API Rate Limits (NVIDIA NIM, Open-Router, etc.):**  
   Many users operate under strict rate limits (e.g., 20 to 40 requests per minute). Piling unnecessary text into the prompt causes severe throttling, timeouts, and system crashes.

**Therefore: We CANNOT simply append every good idea from research into the system prompt. Every single line in the prompt must justify its existence in real-world performance, bandwidth savings, and token cost.**

---

## 3. What Was Included from the Research, What Was Omitted, and Why

Here is an honest breakdown of what research ideas were included in the system prompt, what ideas were **deliberately omitted**, and the practical rationale behind those choices.

### Summary Table of Research Inclusions & Omissions

| Research Source & Idea | Included or Omitted? | Plain English Rationale & Practical Trade-off |
| :--- | :--- | :--- |
| **Grounding Progress Claims** (*The Compliance Gap*, arXiv:2605.01771) | **INCLUDED** | **Crucial:** Forces AI to cite tool evidence. Stops the AI from lying and claiming a build passed without actually checking. |
| **"Do Not End on a Promise"** (*Anthropic Fable 5 Guide*) | **INCLUDED** | **Crucial:** Stops the AI from ending its turn saying *"I will now run the build..."* without actually running it. |
| **Sub-Agent Leashing** (*MAS-PromptBench*, arXiv:2606.23664) | **INCLUDED** | **Crucial:** Prevents the main AI from spawning dozens of helper sub-agents that drain your token budget and crash API rate limits. |
| **`send_to_user` Tool** (*Anthropic Fable 5 Guide*) | **OMITTED** | **Omitted to Save Tokens:** Anthropic recommended creating a custom tool for the AI to message users. But terminal output is native in Opencode. Adding a custom tool schema wastes ~300 tokens per turn for zero gain. |
| **Full File Tree Dumps** (2023 Legacy Approach) | **OMITTED** | **Omitted to Save Bandwidth:** Replaced 10k–30k token tree dumps with shallow depth-1 summaries (`coder.go` L94–101) to protect satellite connections and low-RPM API keys. |

---

# TRACK TWO: THE DEVELOPER & AUDITOR TRACK
*Written with technical precision for software engineers, security auditors, and prompt architects.*

---

## SECTION 1: Absolute Repository Paths & Core Architecture

All prompt engineering and sub-agent controls in `v0.2.0` reside in the following canonical files:

- **Active System Prompts:**
  - Coder Prompt: [`system-prompts/current/coder-modern.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/current/coder-modern.md)
  - Summarizer Prompt: [`internal/llm/prompt/summarizer.txt`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/prompt/summarizer.txt)
  - Subagent / Task Prompt: [`system-prompts/current/task.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/current/task.md)
- **Go Engine Mechanics:**
  - Section Parser & Dynamic Assembler: [`internal/llm/prompt/sections.go`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/prompt/sections.go)
  - Source Reader & Validation: [`internal/llm/prompt/source.go`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/prompt/source.go)
  - Sub-agent Guard & Rate Limiter: [`internal/llm/agent/subagent_guard.go`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/agent/subagent_guard.go)
  - Sub-agent Registry & Termination: [`internal/llm/agent/subagent_registry.go`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/agent/subagent_registry.go)

---

## SECTION 2: Forensic Line-by-Line Research Mapping

### 1. Paper 1: "The Compliance Gap" (arXiv:2605.01771)
- **Finding:** Verbal-only reporting obscures non-compliance. System prompts must enforce Process Compliance over verbal claims.
- **Code Mapping:**
  - [`system-prompts/current/coder-modern.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/current/coder-modern.md) (Line 23): `audit before reporting: every progress claim must point to a tool result...`
  - [`internal/llm/prompt/summarizer.txt`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/prompt/summarizer.txt) (Line 13): `do not promote attempts to successes: unverified in, unverified out`

### 2. Paper 2: "Prompting Claude Fable 5" (Anthropic 2026)
- **Finding:** Never end on a promise; re-ground reader after long unattended executions; enforce strict scope boundaries.
- **Code Mapping:**
  - [`system-prompts/current/coder-modern.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/current/coder-modern.md) (Line 57): `finish task: do not yield a plan instead of the work: do not end on a promise ("I'll now..."): if your last paragraph is a plan... do that work now`
  - [`system-prompts/current/coder-modern.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/current/coder-modern.md) (Line 49): `re-ground the reader: after long unattended work your summary is their first look...`

### 3. Paper 3: "MAS-PromptBench" (arXiv:2606.23664)
- **Finding:** Un-leashed sub-agents degrade swarm accuracy and exhaust context buffers rapidly.
- **Code Mapping:**
  - [`system-prompts/current/coder-modern.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/current/coder-modern.md) (Line 37): `sub-agents: use task tool only for focused, read-only sub-tasks...`
  - [`internal/llm/agent/subagent_guard.go`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/agent/subagent_guard.go) (Lines 28–40): Reserve & guard sub-agent concurrency slots.
  - [`internal/llm/agent/subagent_registry.go`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/agent/subagent_registry.go) (Lines 57–74, 133–147): Full registry tracking and nuclear teardown.

---

# SECTION 3: PHILOSOPHY & METHODOLOGY
*The Dual-Track Standard & Radical Transparency*

Every piece of engineering documentation in `Gorilla.Opencode` is governed by the **Gorilla Open Source Philosophy**:
1. **Legible Transparency:** Code visibility is useless without comprehension. Every technical feature must have a plain-language translation.
2. **Dual-Track Standard:** Track One (Human/Layman) and Track Two (Developer/Auditor) are written simultaneously in the exact same report so that neither audience is forced to blindly trust the other.
3. **Low-Overhead Engineering:** Documentation and prompts are engineered for real-world constraints, including low-bandwidth satellite links and strict API rate limits.
