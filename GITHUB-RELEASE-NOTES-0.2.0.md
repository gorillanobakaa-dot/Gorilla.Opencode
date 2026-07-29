## New system prompts — research-grounded, low-overhead agent governance

A "system prompt" is the standing instruction sheet sent to the AI before it ever sees your question. It decides whether you get an assistant that checks its work, or one that lies and tells you a build succeeded when it did not even look.

The system prompt inherited from upstream OpenCode was written in 2023 and relied heavily on shouting (`IMPORTANT` occurred seven times, along with all-caps emphasis). Research since 2024 shows that shouting in system prompts actually dilutes model attention, increases hedging, and leads to false reports of success.

All four shipped prompts have been rewritten based on research from **Anthropic (Claude Fable 5 guidance)**, **arXiv:2605.01771 (*The Compliance Gap*)**, **arXiv:2606.23664 (*MAS-PromptBench*)**, and **arXiv:2603.25723 (*Natural-Language Agent Harnesses*)**. Full research details and citations are documented in [`system-prompts/RESEARCH-SOURCES.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/RESEARCH-SOURCES.md).

---

## Layperson Summary: What You'll Notice & Real-World Constraints

### 1. What Changes for the User
- **No More Pretend Success:** Every progress claim must point to an actual tool result from the session. The AI can no longer claim a build passed without checking.
- **No Unrequested Work:** Describing a problem or asking a question is strictly an assessment request, not permission to create unwanted backup branches, drafts, or extra files.
- **No Ending on Promises:** The AI is forbidden from ending a turn with *"I will now run the build..."* without actually running it.
- **No Length-Anxiety Handoffs:** The AI will not try to end the session or yield a handoff summary just because conversation context is getting long.
- **Leashed Helper Agents:** Helper sub-agents are strictly capped to prevent them from spawning out of control and draining your token budget.

### 2. Built for Satellite Internet & Constrained API Limits
We designed these prompts with extreme real-world constraints in mind:
- **Remote & Emergency Field Operations:** For emergency crews, rescue teams, or field engineers using satellite internet connections measured in **single-digit KB/s**, every token counts.
- **Rate-Limited API Keys:** For users operating on NVIDIA NIM or custom API endpoints capped at **20 to 40 requests per minute**, prompt overhead must be kept to an absolute minimum.
- **Shallow Directory Summaries:** Legacy OpenCode dumped thousands of directory paths into every prompt (10,000–30,000 tokens per turn). We replaced this with a shallow top-level summary (~50 tokens), saving significant bandwidth and cost.

---

## Detailed Research Inclusions, Omissions, and Rationale

| Feature / Research Source | Included or Omitted? | Location / Rationale |
| :--- | :--- | :--- |
| **Grounded Claims** (*The Compliance Gap*, arXiv:2605.01771) | **INCLUDED** | `system-prompts/current/coder-modern.md` (L23-24). Forces tool evidence before reporting success. |
| **No End-on-Promise** (*Anthropic Fable 5 Guide*) | **INCLUDED** | `coder-modern.md` (L57). Prevents AI from trailing off with unexecuted plans. |
| **Sub-Agent Leashing** (*MAS-PromptBench*, arXiv:2606.23664) | **INCLUDED** | `coder-modern.md` (L37) & `internal/llm/agent/subagent_guard.go`. Prevents runaway helper spawns. |
| **Modular Harness Toggles** (*Agent Harnesses*, arXiv:2603.25723) | **INCLUDED** | `internal/llm/prompt/sections.go`. Parses prompt sections dynamically so users can toggle them in `/context`. |
| **`send_to_user` Tool** (*Anthropic Fable 5 Guide*) | **OMITTED** | **Omitted to Save Tokens:** Terminal output is native in Opencode. Adding a custom `send_to_user` tool schema would waste ~300 tokens per turn for zero operational gain. |
| **Full File Tree Dumps** (2023 Legacy Approach) | **OMITTED** | **Omitted to Save Bandwidth:** Replaced 10k–30k token tree dumps with shallow depth-1 summaries (`coder.go` L94–101) to protect satellite connections and low-RPM API keys. |

---

## Developer & Auditor Forensic Reference

For security auditors and software engineers, here are the absolute file paths and line numbers where these research principles are implemented in the repository:

1. **Grounded Honesty (`arXiv:2605.01771`):**
   - [`system-prompts/current/coder-modern.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/current/coder-modern.md) (Line 23: `audit before reporting: every progress claim must point to a tool result...`)
   - [`internal/llm/prompt/summarizer.txt`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/prompt/summarizer.txt) (Line 13: `do not promote attempts to successes: unverified in, unverified out`)
2. **Autonomous Turn Conduct (Anthropic Fable 5):**
   - [`system-prompts/current/coder-modern.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/system-prompts/current/coder-modern.md) (Line 31: `# scope`, Line 49: `# output`, Line 57: `# conduct`)
3. **Sub-Agent Guard & Process Registry (`arXiv:2606.23664`):**
   - [`internal/llm/agent/subagent_guard.go`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/agent/subagent_guard.go) (Lines 28–40: `reserveSubAgentSpawn`)
   - [`internal/llm/agent/agent-tool.go`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/agent/agent-tool.go) (Lines 62–69: limit check & `SubAgentsNuclear` intercept)
   - [`internal/llm/agent/subagent_registry.go`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/agent/subagent_registry.go) (Lines 57–74: `RegisterSubAgent`, Lines 133–147: `KillAllSubAgents`)
4. **Dynamic Section Parsing (`arXiv:2603.25723`):**
   - [`internal/llm/prompt/sections.go`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.2.0/internal/llm/prompt/sections.go) (Lines 52–99: `ParseSections`, Lines 141–156: `assembleCoderPrompt`)

---

**Install:** `sudo dpkg -i gorilla-opencode_0.2.0_amd64.deb`

