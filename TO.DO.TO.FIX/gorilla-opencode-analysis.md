# Gorilla-Opencode — Full Internals Analysis

> Source: [Gorilla.Opencode](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/)
> Process 311984 · PID running from `/home/gorilla` · 16 threads · 154% CPU

---

## 1. Prompt System — What It Sends to the LLM

### 1.1 Prompt Loading: Compiled Into the Binary

The active system prompt is **embedded at compile time** via Go's `//go:embed` directive:

```go
//go:embed coder-modern.txt
var baseModernCoderPrompt string
```

Source: [coder.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/prompt/coder.go) — this is where the prompt assembly happens.

The embedded file is [coder-modern.txt](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/prompt/coder-modern.txt), which is identical to the `proposed/coder-modern.md` in the system-prompts directory (~924 tokens). This is the **proposed** prompt you wrote — the neutral, imperative, anti-hallucination one. It's already shipping as the default.

### 1.2 Prompt Assembly Pipeline

When a request goes out, `GetAgentPrompt()` in [prompt.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/prompt/prompt.go) builds the full system prompt in layers:

```
┌─────────────────────────────────────────────┐
│ 1. Base system prompt (coder-modern.txt)    │  ~924 tokens
├─────────────────────────────────────────────┤
│ 2. Environment block (if loadout enabled)   │  ~50-100 tokens
│    - CWD, OS, date, git branch              │
│    - Depth-1 dir listing (max 25 entries)   │
│    - git status --short (max 10 lines)      │
├─────────────────────────────────────────────┤
│ 3. LSP information (if loadout enabled)     │  ~30 tokens
│    - How diagnostics tags appear in tools   │
├─────────────────────────────────────────────┤
│ 4. Project instruction files (if found)     │  variable
│    - .github/copilot-instructions.md        │
│    - CLAUDE.md, .cursorrules                │
│    - OPENCODE.md, opencode.md               │
│    Deduplicated, joined as "# From:<path>"  │
├─────────────────────────────────────────────┤
│ 5. Conversation history (from SQLite)       │  variable
├─────────────────────────────────────────────┤
│ 6. Tool definitions (JSON Schema)           │  ~2000+ tokens
│    - Filtered by active context loadout     │
└─────────────────────────────────────────────┘
```

### 1.3 The Four Agent Roles & Their Prompts

| Agent | Prompt Source | Purpose |
|---|---|---|
| **`coder`** | `coder-modern.txt` (embedded) | Main coding agent — the one you interact with |
| **`task`** | [task.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/prompt/task.go) | Read-only sub-agent for search/exploration. Concise 1-line answers |
| **`summarizer`** | [summarizer.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/prompt/summarizer.go) | Conversation compaction for context window management |
| **`title`** | [title.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/prompt/title.go) | Generates <50 char titles. MaxTokens forced to 80 |

### 1.4 The Legacy Prompts (Still in Source Tree)

The `current/` directory holds the **old** prompts that shipped before your redesign:

| File | Tokens | Used When |
|---|---|---|
| [coder-anthropic.md](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/system-prompts/current/coder-anthropic.md) | ~2,003 | Was used for Anthropic + local providers |
| [coder-openai.md](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/system-prompts/current/coder-openai.md) | ~1,048 | Was used for OpenAI provider |

These are **no longer compiled into the binary** — `coder-modern.txt` replaced them. They're kept in `system-prompts/current/` for reference/comparison only.

> [!IMPORTANT]
> The key difference: the old Anthropic prompt was verbose, used ALL-CAPS threats ("VERY IMPORTANT", "NEVER"), and repeated "answer in <4 lines" three times. Your redesigned `coder-modern.txt` is 69% smaller, neutral/imperative, and adds build-failure loop discipline. The research notes in [README.md](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/system-prompts/README.md) document why.

---

## 2. The File Scanner — Why You See 46k `getdents64` Calls

### 2.1 There Is No Background Indexer

Gorilla-opencode does **not** run a persistent background file watcher or indexer daemon. The directory walks you see in strace are triggered **on-demand** by the LLM agent using its tools. Specifically:

### 2.2 What Triggers Directory Walks

Every time the LLM agent calls one of these tools, a filesystem walk happens:

| Tool | File | Walk Type |
|---|---|---|
| **`ls`** | [ls.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/ls.go) | Lists directory entries at a given path |
| **`glob`** | [glob.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/glob.go) | `doublestar.GlobWalk` — recursive pattern match |
| **`grep`** | [grep.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/grep.go) | Shells out to `rg` (ripgrep) |
| **`agent`** (sub-agent) | [agent-tool.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/agent/agent-tool.go) | Spawns a task sub-agent that itself uses ls/glob/grep |
| **`projectSummary()`** | [coder.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/prompt/coder.go) | Depth-1 listing (max 25) + git status at prompt assembly time |

### 2.3 What You Told It To Do

You said you fed it the strace/lsof analysis and asked it to investigate its own source code. That means the LLM is actively:

1. Calling `glob` and `grep` to search the source tree at `/home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/`
2. Calling `ls` to explore directories
3. Possibly spawning `agent` sub-agents to do parallel searches
4. Each tool call triggers a filesystem walk → that's the `getdents64` + `openat` + `close` storm

**But the CWD is `/home/gorilla`**, so whenever the LLM asks for a broad search without a specific path, the tools default to walking from the CWD — your entire home directory.

### 2.4 Ignore Patterns (Built-In)

The `SkipHidden()` function in [fileutil.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/fileutil/fileutil.go) filters out:

```
.opencode, node_modules, vendor, dist, build, target, .git,
.idea, .vscode, __pycache__, bin, obj, out, coverage, tmp,
temp, logs, generated, bower_components, jspm_packages
```

Plus any path starting with `.` (hidden files/dirs).

> [!NOTE]
> This skip list does **not** include `firefox-source`, `Documents`, `chroma_db`, `.cache`, `.local`, or any of the other massive trees under your home directory. So when a glob or grep walks from CWD (`/home/gorilla`), it traverses everything not in that short skip list.

---

## 3. Tools Available to the LLM Agent

### 3.1 Coder Agent Tools (Full Set)

| Tool | Source | Description |
|---|---|---|
| `bash` | [bash.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/bash.go) | Execute shell commands with permission checks, timeout, streaming |
| `edit` | [edit.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/edit.go) | In-place search-and-replace within files + LSP diagnostic check |
| `write` | [write.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/write.go) | Create/overwrite entire files |
| `patch` | [patch.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/patch.go) | Apply multi-hunk unified diffs |
| `view` | [view.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/view.go) | Read file contents with line-range slicing |
| `ls` | [ls.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/ls.go) | Directory listing |
| `grep` | [grep.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/grep.go) | Ripgrep-powered regex search |
| `glob` | [glob.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/glob.go) | Pattern-based file finder |
| `fetch` | [fetch.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/fetch.go) | Download web content (SSRF-guarded) |
| `diagnostics` | [diagnostics.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/diagnostics.go) | LSP linter/typecheck errors |
| `agent` | [agent-tool.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/agent/agent-tool.go) | Spawn read-only sub-agents (leashed by `MaxSubAgents`) |
| `sourcegraph` | [sourcegraph.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/tools/sourcegraph.go) | Public code search |
| MCP tools | [mcp-tools.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/agent/mcp-tools.go) | Dynamic tools from stdio/sse MCP servers |

### 3.2 Task Agent Tools (Sub-Agent, Read-Only)

Sub-agents get a restricted set: `view`, `ls`, `grep`, `glob`, `fetch`, `sourcegraph`. No `bash`, `edit`, `write`, `patch`.

### 3.3 Tool Filtering via Context Loadout

Tools can be toggled on/off via the loadout system in [loadout.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/config/loadout.go). Each tool is gated by `loadoutOn("tool.<name>")`. This lets you disable expensive tools to save context tokens.

---

## 4. Provider & Model Configuration

### 4.1 Provider Auto-Detection Order

When no explicit model is configured, [config.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/config/config.go) detects credentials in this priority:

| Priority | Provider | Default Model |
|---|---|---|
| 1 | GitHub Copilot | `CopilotGPT4o` |
| 2 | Anthropic | `Claude4Sonnet` / `Claude37Sonnet` |
| 3 | OpenAI | `GPT41` |
| 4 | Google Gemini | `Gemini36Flash` |
| 5 | Groq | `Llama3.3-70B` |
| 6 | Cerebras | `CerebrasGLM47` |
| 7 | OpenRouter | `OpenRouterClaude37Sonnet` |
| 8 | xAI | `XAIGrok3Beta` |
| 9 | AWS Bedrock | `BedrockClaude37Sonnet` |
| 10 | Azure OpenAI | `AzureGPT41` |
| 11 | Vertex AI | `VertexAIGemini25` |

### 4.2 Rate Limiting

Proactive RPM pacing in [ratelimit.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/llm/provider/ratelimit.go) — sliding window that sleeps between API calls to prevent 429s. Configurable via `RateLimitRPM` presets in [config/ratelimit.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/config/ratelimit.go).

---

## 5. CPU Usage Explained (Updated Diagnosis)

Now that we know the architecture, here's the refined picture:

| What You See in Strace | What's Actually Happening |
|---|---|
| **46k `getdents64`/3s** | The LLM agent is actively calling `glob`/`grep`/`ls` tools to explore the source code. Because CWD = `/home/gorilla`, broad searches walk your entire home tree |
| **23k `openat`+`close`/3s** | Each file discovered by `doublestar.GlobWalk` or `rg` gets opened/stated/closed |
| **7k `futex` (74% wall time)** | Go runtime goroutine synchronization — 16 threads coordinating tool execution, SQLite writes, and LLM streaming. Normal for active Go concurrency |
| **6.5k `nanosleep` (10%)** | Go runtime scheduler + rate limiter pacing between API calls |
| **1.3k `epoll_pwait` (10%)** | Network I/O — waiting on the HTTPS connection to the LLM endpoint |

**The CPU is legitimate work** — the agent is doing what you asked (analysing its own source code). The 154% CPU is the cost of a Go process with 16 threads doing aggressive filesystem walks on a directory rooted at `/home/gorilla` while simultaneously streaming LLM responses.

### 5.1 Where the Waste Is

The waste isn't in the agent being active — it's in **what it's scanning**. The `SkipHidden` filter in [fileutil.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/fileutil/fileutil.go) only skips 17 hardcoded directory names. Your home directory has massive trees that aren't in that list:

| Directory | Approx Size | Skipped? |
|---|---|---|
| `firefox-source/` | 400k+ files | ❌ No |
| `Documents/Second Brain/chroma_db/` | 6.6 GB | ❌ No |
| `.cache/` | Hidden → skipped | ✅ Yes |
| `.local/` | Hidden → skipped | ✅ Yes |
| `Documents/` (everything else) | Huge | ❌ No |

### 5.2 Optimization Opportunities (Code-Level)

1. **Expand `SkipHidden()` in [fileutil.go](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/internal/fileutil/fileutil.go)** — add a user-configurable ignore file (`.opencodeignore` or similar) that gets loaded at startup, rather than a hardcoded list.

2. **Scope tools to project root** — when the agent calls `glob` or `grep` without an explicit path, default to the **project root** (detected from `.git` or `.opencode.json`) instead of CWD. This is the single biggest win.

3. **Build-log filter** — already on your roadmap in [system-prompts/README.md](file:///home/gorilla/Documents/ai-tooling/opencode/Gorilla.Opencode/system-prompts/README.md): strip `CC`/`CXX`/`AR` progress noise from bash tool output, cap at ~800 tokens.

4. **Loop guard** — also on your roadmap: hash `(tool, args, stderr-snippet)` and intercept repeats.
