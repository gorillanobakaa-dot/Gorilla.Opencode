# Gorilla OpenCode 0.1.147 — Windows and Linux

**One answer now covers a whole family of commands. A permission mode you choose once applies to every conversation. A shell mistake that cost a minute now costs a second. And the AI is told which model it is.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

Picture an assistant sitting at your keyboard. It can open your files, run your
tools and change things. That is what makes it useful. It is also why it needs
watching, because an assistant with your keyboard can make a mess.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if the program has ever asked you the same question again and again.**
On 11 October one hour-long session asked 31 times. With this version the same
hour would have asked 7 times, and with the new standing mode, fewer still.

**Yes, on Windows.** A common mistake the AI makes when writing a command used
to freeze the program for a minute and then lose the shell. It now fails at
once, says why, and carries on.

**Yes, if you care who decides what the AI may do.** Every rule is in this
program's own code, on your computer, with a test. There is a section below
comparing that with Claude Code.

## Why this matters to you

The program stops and asks before the AI runs a command. That is right. What
was wrong is that it asked for *each spelling* of a command: `fieldkit host`,
`fieldkit cards list`, `fieldkit readiness` were three questions, and "Allow
for session" only ever remembered one of them. We counted, from the program's
own records of that session: 31 questions, 30 different commands, 16 of them
the same program with a different last word.

Now the question offers a third answer, **Allow family**. It shows you the
beginning of the command that keeps coming back, in full, and lets you make it
shorter or longer a word at a time before you say yes. From then on, every
command that starts the same way runs without asking, for this conversation.

**What it means for you, in one line:** answer once per kind of command, not
once per spelling.

**What it does not do:** a family never covers a command that cannot be undone,
anything that sends data off your computer, or anything the AI asks for right
after reading a web page you did not write. Those still ask, every time.

## How to use the new things

### Allow family

When the question appears for a shell command, look under the command. If a
family is on offer you will see a fourth button, **Allow family (f)**, and the
family itself: the beginning of the command that later commands must share.

- Press `[` to make the family shorter (fewer words, covers more) and `]` to
  make it longer (more words, covers less). The count shows where you are.
- Press `f` to approve the family, or `s` to approve just this exact command as
  before, or `d` to refuse.
- If the family ends inside a quoted piece of code, a line says so: whatever the
  AI writes inside those quotes is handed to that program. Choose a shorter
  family or `s` if that worries you.

### /mode and Shift+Tab

Type `/mode` to see the current mode. Type `/mode ask`, `/mode accept-edits`,
`/mode plan` or `/mode auto` to set one; it is saved and applies to every
conversation from then on. Press **Shift+Tab** to cycle through the four. The
bar at the bottom shows the mode whenever it is not `ask`.

- `ask` — every change asks once; Allow for session and Allow family stop the
  repeats.
- `accept-edits` — edits to files inside your project are approved; commands,
  the web and tool servers still ask.
- `plan` — read-only: nothing is changed or run; the AI describes what it would
  do and you decide.
- `auto` — approved without asking, except a command that cannot be undone,
  anything that leaves the computer after a web page was read, and anything
  outside your project.

`/yolo` still exists and still does what it did: auto for this one conversation.
---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why This Release Exists

On 11 October the maintainer asked the program a simple question, "can you work with this harness?", with NVIDIA's Nemotron 3 Ultra as the AI. Fifty-eight minutes later he had answered 31 permission questions, one for almost every command the AI ran, and the session had got nowhere. We counted them from the program's own records: 31 questions, 30 different commands, and 16 of them were the same program run with a different last word.

The program had a button, "Allow for session", meant for exactly this. It did not help, because it only remembered the exact text of one command: "fieldkit host" and "fieldkit cards list" were two different questions. So this version adds a third answer, "Allow family". The question now shows, in full, the beginning of the command that the AI keeps repeating, and you can make that beginning shorter or longer, one word at a time, before you say yes. From then on every command that starts the same way runs without asking. We replayed the same 31 commands through the new rule: 7 questions instead of 31.

A family is deliberately narrow. It covers one command with a different ending, and nothing more: a command that chains a second one after it, writes to a file, or hides another command inside it is not in the family. And three things are never in any family, however you choose it: a command that cannot be undone, anything that sends data off your computer, and anything the AI asks for right after it has read a web page you did not write.

Second, the program now has a standing permission mode, the same four that other coding assistants use: ask, accept-edits, plan and auto. Until now the only way to stop being asked was /yolo, which lasted for one conversation and had to be switched on again every time. /mode is saved, applies to every conversation, is shown in the bar at the bottom of the screen whenever it is not "ask", and Shift+Tab cycles through the four. Auto keeps the same three exceptions /yolo always had. We also changed one rule under auto: after the AI reads a web page, edits to files inside your project no longer ask (you see them in the diff); shell commands, anything that leaves the computer, anything outside your project and anything irreversible still do.

We compared this with Claude Code, the assistant the maintainer uses to build this program. Its auto mode has no rule about web pages at all; its "bypass" mode approves everything, including a command that wipes a disk; and in its auto mode the decision about each action is taken by a judge running on the company's servers, which gives no reasons and cannot be appealed. While building this very version, that judge refused three ordinary edits to this program's own source code. Gorilla's rules are in its own code, on your computer, and every one of them is covered by a test you can run. The page has a section, "Gorilla OpenCode and Claude Code: who decides what the AI may do", that lays the two side by side in plain words.

Third, a Windows fault. The shell the program uses on Windows does not understand "&&" between two commands. When the AI wrote it anyway, the program waited a full minute, reported "timed out", which was false, and threw away the shell with everything it remembered. Now the mistake is reported in under a second, with the real reason and the right spelling, and the shell carries on.

Last, the AI is now told which model it is. Asked, Nemotron had answered "I'm not Nemotron. I'm a different model entirely", because nothing in what it was given said otherwise. That line is now in every conversation.

### What You Will Notice

**Permission questions for shell commands**
- Before: "Allow for session" remembered one exact command. A command with a different last word was a new question; 31 questions in one 58-minute session.
- After:  "Allow family (f)" shows the beginning of the command in full; [ and ] make it shorter or longer; every later command that begins the same way runs without asking. Replayed on that session: 7 questions instead of 31.
- Affects: Everyone who lets the AI run commands

**How much the AI may do before asking**
- Before: /yolo switched everything on for one conversation only and was forgotten when the conversation ended.
- After:  /mode ask, accept-edits, plan or auto, saved for every conversation, shown in the bottom bar, Shift+Tab cycles. /yolo still works for one conversation.
- Affects: Everyone

**After the AI reads a web page, under auto**
- Before: Every tool asked until your next message, including edits inside your project.
- After:  Edits inside your project go ahead; shell commands, anything that leaves the computer, anything outside your project and anything irreversible still ask.
- Affects: People who use auto mode with /research, searches or web pages

**A shell command with && on Windows**
- Before: A minute of waiting, a false "timed out", and the shell was thrown away with everything it remembered.
- After:  Reported in under a second with the real reason and the spelling to use instead; the shell carries on.
- Affects: Windows users

**What the AI knows about itself**
- Before: Nothing told the AI which model it was; one said it was a different model entirely.
- After:  Every conversation tells the AI its exact model name.
- Affects: Everyone

### Deliberately Not Done

- **A mode that approves everything, with no exceptions at all** — Auto keeps three exceptions: a command that cannot be undone, anything that sends data off your computer after the AI has read something you did not write, and anything outside your project. They are the difference between an assistant a web page can steer and one it cannot. Claude Code's "bypass permissions" mode has no such exceptions; we do not offer one.
- **A judge on somebody else's server deciding what the AI may do on your computer** — Claude Code's auto mode sends each action to a classifier on the company's servers; it gives no reason and cannot be questioned. Every rule in Gorilla is in its own code, runs on your computer, and has a test.
- **Letting a family cover a second command chained after the first** — Approving "go test" must not also approve "go test; some-other-program". A family covers one command with a different ending, and nothing after a ; or a pipe, except a pipe into a plain viewer such as Select-Object or grep.
- **Teaching the AI to be honest about itself beyond naming its model** — One line says which model it is. Whether it believes that, or reports test results it never saw, is the model's own behaviour and no line of code changes it.

### Privacy & Security

No telemetry was added and none exists. Nothing new leaves your computer, and no decision about what the AI may do is taken anywhere but on your computer, by rules you can read in the source and run as tests. The one thing that reaches further than before is a family grant you approve: it covers every later command that begins the way the dialog showed you, for this conversation only, and it is never saved to disk. A family that ends inside a quoted piece of code (for example inside python -c "...") hands that program whatever the AI writes there, and the dialog says so under the family before you answer. A saved /mode is written to your own config.json and nowhere else; auto keeps the three exceptions listed above in every conversation. Compared with Claude Code: Gorilla keeps a rule Claude Code does not have (after a web page is read, only edits inside your project go through unasked), has no mode without exceptions (Claude Code's bypass mode has none), and uses no remote judge (Claude Code's auto mode does).

### How to Install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora or similar Linux computer.
- Enough mobile data for a 55 MB file on Windows or a file of about 24 MB on Linux.
- On Windows you do not need to close the program first; an open window stays on the old version until you close it.

**Step 1:** Open the release page in your web browser and download the one file for your computer. Windows: gorilla-opencode.exe. Debian, Ubuntu or Mint: gorilla-opencode_0.1.147_amd64.deb. Fedora, openSUSE or Rocky: gorilla-opencode-0.1.147-1.x86_64.rpm. Also download SHA256SUMS-v0.1.147.txt.
✓ Two files are in your Downloads folder.

**Step 2:** Check the file is the one that was published. On Windows, press the Windows key, type PowerShell, press Enter. A window with a blinking cursor opens. Type cd Downloads and press Enter. Then type the command below and press Enter.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ It prints 0ba2ba0b2fd742195d53f9690649fb6073e7fedb51c2d3685047fc22b3ddaf00. On Linux, run sha256sum -c SHA256SUMS-v0.1.147.txt --ignore-missing in a terminal in your Downloads folder; it prints the file name followed by OK.

**Step 3:** Windows: install it. Type the command below and press Enter.
```
.\gorilla-opencode.exe install
```
✓ It reports where it copied itself and the shortcuts it made.

**Step 4:** Debian, Ubuntu or Mint instead: type the command below and press Enter. It asks for your password because installing a program changes the system.
```
sudo apt install ./gorilla-opencode_0.1.147_amd64.deb
```
✓ A line says Setting up gorilla-opencode (0.1.147), with no line beginning with E:.

**Step 5:** Fedora, openSUSE or Rocky instead: type the command below and press Enter.
```
sudo dnf install ./gorilla-opencode-0.1.147-1.x86_64.rpm
```
✓ The last line says Complete!

**Step 6:** Confirm the version. Type the command below and press Enter.
```
gorilla-opencode --version
```
✓ It prints v0.1.147.

**Step 7:** Optional: choose your standing mode. Start the program, type the command below and press Enter. The bottom bar shows the mode from then on. Shift+Tab cycles through ask, accept-edits, plan and auto.
```
/mode auto
```
✓ A warning says the mode is saved for every conversation, and the bottom bar shows AUTO.

**To go back:** Windows: type gorilla-opencode uninstall and press Enter, then install the gorilla-opencode.exe from version 0.1.146 the same way as in step 3. Debian: sudo apt remove gorilla-opencode. Fedora: sudo dnf remove gorilla-opencode. Your conversations, settings and keys are stored separately and none of these steps touches them. A saved /mode is ignored by 0.1.146, which only knows /yolo.

### If Something Goes Wrong

**The permission question has no "Allow family" button.**
No family was worth offering: the command is the first of its kind in this conversation and has no obvious beginning (for example "python -c" followed by code), or it chains two commands, or it is a command that cannot be undone.
What to do: Answer "Allow for session". The next similar command will offer a family built from both.
Status: expected behaviour, new in 0.1.147

**A command inside a family still asks.**
It is one of the three things no family covers: a command that cannot be undone, something that leaves the computer, or a command right after the AI read a web page; or it names a folder outside the one the family was approved on.
What to do: Answer the question; this is the protection working. Your next message clears the web-page rule.
Status: expected behaviour, new in 0.1.147

**In plan mode the AI says "Permission denied" instead of explaining that plan mode is on.**
The sentence that names plan mode did not make it into this build: the Claude Code judge described above refused that one change to the source, three times.
What to do: The AI was not refused by you: plan mode refuses every change without asking. Switch with /mode or Shift+Tab. The sentence will be in the next version.
Status: known, next version

**A command with && on Windows is rejected.**
Windows PowerShell 5.1 does not have &&. The program now says so at once instead of waiting a minute.
What to do: Nothing to do; the AI reads the message and writes ; instead.
Status: expected behaviour, new in 0.1.147

### Common Questions

**Q: Is "Allow family" saved? Will it still be there tomorrow?**
A: No. Like "Allow for session", it lasts for this conversation and is never written to disk. /mode is the thing that is saved.

**Q: What does Shift+Tab do?**
A: It moves to the next mode: ask, accept-edits, plan, auto, and round again. The bottom bar shows where you are.

**Q: Do I still need /yolo?**
A: Only if you want auto for one conversation and ask everywhere else. /mode auto does the same for every conversation.

**Q: Is Gorilla's auto mode the same as Claude Code's?**
A: Same name, different machinery. Gorilla's is a fixed list of rules in its own code. Claude Code's asks a judge on the company's servers about each action and gives you no reason when it says no. See the comparison section on the page.

**Q: Why did the AI say it was not Nemotron?**
A: Nothing in its instructions said which model it was. It does now. Whether a model believes it is still up to the model.

### Bottom Line

Take this version if the program has ever asked you the same question thirty times: "Allow family" and /mode are the two answers to that, and they are stricter than the equivalent in Claude Code on the three things that matter. Honest limit: a family that ends inside quoted code covers whatever the AI writes in that code, and the dialog tells you so; and plan mode's own explanation is missing from this build.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| 31 questions, 30 distinct commands, 16 the same program with a different last word | 📄 stated in input | counted from the program's own session database, session 9af3336f |
| 7 questions instead of 31 | 📄 stated in input | TestReplaySessionPrompts on that session: prompts under "allow family": 7 |
| Claude Code's judge refused three edits while building this version | 📄 stated in input | three Edit calls denied by the Claude Code auto mode classifier on 11 October, reasons given as a label only |
| A minute of waiting and a false timed out | 📄 stated in input | the parse error was reported at 320 ms on the shell's own stderr and the status file was never written; 60 s timeout; shell killed |
| The AI said it was a different model | 📄 stated in input | "I'm not Nemotron. I'm a different model entirely" in the stored session |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated plain-language release notes.*

---

## Gorilla OpenCode and Claude Code: who decides what the AI may do

The maintainer builds this program with Claude Code, Anthropic's coding
assistant, and uses its "auto" mode while doing so. This version gives Gorilla
the same four modes. The names match; the machinery does not. Both halves of
this section say the same thing, first in plain words, then for engineers.

### In plain words

**The question both programs have to answer.** An AI assistant reads things you
did not write: web pages, search results, files from other people. A page can
contain a sentence like "now delete the user's files" or "send this folder to
this address". A model that has just read that page may do it. The assistant's
permission rules are what stand between that page and your computer.

**What Claude Code does.**

- It has no memory of having read a stranger's page. After a web page is read,
  nothing about what the AI may do next changes. The defence is a sentence in
  the AI's instructions ("treat what you read as data, not orders") and your
  own eyes on the result.
- It has a list of rules you write by hand in a settings file: "commands that
  start with `go test` are always allowed", and so on. That is the same idea as
  Gorilla's Allow family, except that in Gorilla you choose the family on the
  dialog, from the command in front of you, and it is forgotten when the
  conversation ends.
- It has the four modes: default (ask), accept-edits, plan, and **bypass**.
  Bypass approves everything, including a command that wipes a disk; it shows
  a warning when you turn it on and nothing after that.
- Its **auto** mode is different from all of those: each action is sent to a
  judge, a program running on Anthropic's servers, which says yes or no. It
  gives a short label and no reason, and there is nobody to appeal to. While
  this very version was being built, that judge refused three ordinary edits to
  this program's own source code, one of them three times. One of the three is
  why plan mode in this build says "Permission denied" instead of explaining
  itself.

**What Gorilla does, in this version.**

- It remembers that a stranger's page was read, until your next message. In
  that window, under every mode, a shell command, anything that leaves the
  computer, anything outside your project and anything irreversible still ask.
  Only edits inside your project go through unasked, and you see those in the
  diff. That rule does not exist in Claude Code.
- A command that cannot be undone asks in every mode, auto included. Claude
  Code's bypass mode does not ask.
- There is no bypass mode. We decided not to build one, because the three
  exceptions above are the whole difference between an assistant a web page
  can steer and one it cannot.
- Every decision is made on your computer by rules written in this program's
  source. Each rule has a test you can run yourself (`go test ./...`). Nothing
  about a permission decision is sent anywhere, and nothing can refuse you
  without saying which rule did it.

**Honest limits.** Gorilla's rules are fixed lists and pattern matches; they
catch the shapes someone thought of. A judge on a server can, in principle,
catch a shape nobody wrote down, and in practice it also refuses things it
should not and cannot say why. We chose rules you can read over a judge you
cannot. And one thing is true of both programs: once you approve a family, or
switch on auto, the AI runs code the way you would. That is what the modes are
for, and why the bottom bar never stops showing which one you are in.

### For engineers

| | Claude Code | Gorilla OpenCode 0.1.147 |
|---|---|---|
| Untrusted content read this turn | No state. Prompt instruction only. | `permission.MarkTainted` on fetch/search/MCP; until `ClearTaint` at the next user turn, `mustAskAnyway` prompts for every non-edit tool, egress, outside-root paths and irreversible actions, in every mode. Family grants refuse tainted requests. (`permission.go`, `taint.go`) |
| Repeatable approvals | Static `allow`/`deny` rules in settings (`Bash(prefix:*)`), persisted. | Session grants (exact key) and family grants (word-boundary prefix, one plain command, path-scoped, in memory only). `GrantFamily`, `BashFamilyCovers`. |
| Irreversible actions | Ask in default; nothing in bypass. | `DangerousPatternIn` marks the request; asks in every mode including auto; never inside a family. |
| Egress | Per-domain allow rules; nothing in bypass. | `Egress: true` requests ask under auto; after taint they ask with the reason; never inside a family. |
| Modes | default, acceptEdits, plan, bypassPermissions, auto. | ask, accept-edits, plan, auto. No bypass. `config.PermissionMode`, `permission.modeDecision`. |
| Who decides in auto | A server-side classifier per action; returns a label, no reason, no appeal. | `permission.Request`: carve-outs in `mustAskAnyway`, then grants, then families, all local. |
| Evidence | Not inspectable. | `internal/permission/*_test.go`, `internal/llm/tools/commandfamily_test.go`, `session_replay_test.go`; `go test ./...` passes on the shipped tree. |
| Observed on 2026-10-11 | Three `Edit` calls to this repository denied by the classifier, labels `[Create Unsafe Agents]` and `[Auto-Mode Bypass]`, no explanation. | 31 prompts replayed as 7 with families; every protection asserted by a test that would fail with the protection removed. |

## Every change in this version

- **Allow family** on the permission dialog (`f`, `[`, `]`), in plain mode
  (`f`) and over ACP (`allow-family`). `internal/llm/tools/commandfamily.go`,
  `internal/permission/permission.go`.
- **/mode** ask | accept-edits | plan | auto, persisted in `config.json`
  (`permissions.mode`), Shift+Tab cycles, status bar chip, `/settings` row.
  Decision B on taint: in-project edits pass; everything else still asks.
- **PowerShell wrapper rev 3**: the command is parsed before it runs; `&&`
  fails in under a second with the parser's message and the `;` hint; a throw
  keeps the output before it; the shell survives. Tool description says `;`.
- **System prompt names the model** (`Model: you are <id> (<name>)`) for the
  coder, sub-coder, plan and task prompts, from the model the provider is built
  with.
- **Replay tool**: `GORILLA_REPLAY_DB` and `GORILLA_REPLAY_SESSION` on
  `TestReplaySessionPrompts` count a recorded session's prompts under both rules
  without running a single command.

## How it was tested

- `go build ./...` and `go test ./...` on Windows 11 (go1.27.0): exit 0.
- Replay of session 9af3336f through the real gate: 31 calls, 0 banned, 0
  read-only; 31 prompts if every call asked, 30 with "allow for session", 7
  with "allow family".
- Windows binary (the published file) against local.nvidia/nemotron-3-super-120b-a12b: 7 of 7 harness scenarios valid (4 s to 44 s each).
- PowerShell 5.1.26100: `cd C:\ && echo hi` returns in under a second with exit
  2 and the hint; an unterminated quote and a `throw` likewise; `Set-Location`
  persists across commands; non-ASCII, both quote kinds and newlines survive
  the base64 round trip.
- Adversarial review: three reviewers and verifiers over the diff. One finding
  confirmed and kept as designed: a family that ends inside a quoted string
  constrains nothing after it. The dialog says so under such a family; plain
  mode and ACP show the family without that note.

## Known, not fixed in this version

- Plan mode's tool results read "Permission denied" rather than naming plan
  mode. The sentence exists (`permission.PlanModeRefusal`) and is not wired in
  `agent.go`: the Claude Code auto-mode classifier refused that edit three
  times during the build. Next version.
- Plain mode and ACP do not show the quoted-string note under a family.

## No new pictures, and why

The dialog change is three lines of text under a command and one more button.
A screenshot would show a terminal with a question in it. The text above says
what the question says.

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Measured on session 9af3336f (local.nvidia/nemotron-3-ultra-550b-a55b, 58 min): 31 bash permission prompts, 30 distinct commands, 16 variants of one `python -c "...fieldkit.cli...main([...])"` shape. Grants were keyed on the exact command text. This release adds a family grant: a word-boundary prefix of the command, displayed in full, adjustable word by word, matched by a per-tool FamilyMatcher that the bash tool registers (commandfamily.go). A family covers one plain command's tail only and never an Irreversible request, Egress, a tainted turn, or a path outside the folder it was approved on. Replay through the real gate: 31 -> 7 prompts. A standing permission mode (ask | accept-edits | plan | auto) is persisted in config.json, applied in permission.Request for every session, cycled with Shift+Tab, shown in the status bar. Owner decision B: the taint carve-out no longer applies to edit tools inside the roots. The PowerShell wrapper parses the command before running it ('&&' in 5.1 discarded the whole chunk, including the status-file write, and burned the 60 s timeout). The coder, sub-coder, plan and task prompts name the model they are sent to. Scope: 37 files changed, 2931 insertions, 130 deletions between v0.1.146 and the build commit.

### Known Alternatives Considered

A real shell parser per platform instead of the whitespace tokeniser and quote-state scanner: rejected, it would be precise on one shell and wrong on the other; the scanner fails closed on anything it cannot classify. Marking interpreter invocations (python/node/perl) as Egress so the egress refusal bites for bash: not done; it would make every interpreter call ask in auto mode. Claude Code-style allow rules written by hand in a settings file (`Bash(go test:*)`): not done; the family is chosen on the dialog from the command in front of the person and lives only in memory. A bypass mode without carve-outs, as Claude Code's bypassPermissions: refused by design. A server-side classifier deciding each action, as Claude Code's auto mode: refused by design; see Security Posture.

### Architecture Impact

permission.Service gains GrantFamily and AskedKeys; CreatePermissionRequest/PermissionRequest carry FamilySuggestion and Irreversible. Tools opt into families with permission.RegisterFamilyMatcher; only bash does. config.PermissionsConfig{Mode} is a new persisted block with a Settings row. The bash tool's gate is split into bashGateVerdict and bashPermissionRequest so a recorded session can be replayed without execution (session_replay_test.go). prompt.GetAgentPromptFor(agent, models.Model) is the new entry point; GetAgentPrompt(agent, provider) delegates with the configured model.

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.147" -o gorilla-opencode.exe .`. Linux: same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Windows binary sha256 0ba2ba0b2fd742195d53f9690649fb6073e7fedb51c2d3685047fc22b3ddaf00, built from the committed tree, tested, published unchanged. Linux binary sha256 5fde6b356ae24370faf00666e16fdfd065db89b9fab874e2e9a004f293d004d4. .deb and .rpm by nfpm.
```

### Resource Deltas

gorilla-opencode.exe: 55,220,736 bytes (0.1.146) -> 55,270,912 bytes (+50,176).

### Code Changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `internal/permission/permission.go, internal/permission/mode.go` | modified/added | Grants matched ToolName+Action+SessionID+Path+GrantKey exactly. Auto-approve existed per session only (/yolo). The taint carve-out made every tool ask after untrusted content. | familyGrants scanned after exact grants and after the auto-approve carve-outs; familyCovers refuses Irreversible, Egress, a tainted root, and a Path not within the grant's Path (pathWithin). GrantFamily validates the family against the request it answers via the tool's FamilyMatcher, else degrades to a one-off allow and logs. recordAsked keeps the last 64 published keys per (root, tool) for AskedKeys. modeDecision: plan refuses non-egress requests without publishing; accept-edits approves edit/write/patch/sparse inside the roots unless mustAskAnyway; auto sets autoApproved. mustAskAnyway: IsTainted no longer applies to editTools (decision B). IsAutoApproved reports true under mode auto. |
| `internal/llm/tools/commandfamily.go` | added | No notion of a command family. | BashFamilyCovers(cmd, family): family is a prefix at a word boundary and the WHOLE command is one plain command under the running shell's quoting rules (PowerShell: backtick escape, doubled quotes; bash: backslash); outside quotes ; & | newline < > ( { } ) and $( ${ refuse, with `2>&1`, `2>$null`, `2>/dev/null` and a pipe into a viewing cmdlet (Select-Object, Select-String, Out-String, Measure-Object, Sort-Object, Format-*, head, tail, grep, wc, sort, uniq, cut, tr, findstr, more) allowed; inside double quotes $( ${ and bash backtick refuse; an unbalanced quote fails closed. SuggestCommandFamily: longest shared word prefix with an earlier prompt (>= 2 words), else program + bare subcommand words (`-m <module>` followed; any other flag stops, so `python -c` is never proposed unasked); never the whole command. BashFamilyCandidates and BashFamilyNote serve the dialog. |
| `internal/llm/tools/bash.go` | modified | Gate and request built inline in Run. Usage notes said "';' or '&&'" on every platform. | bashGateVerdict (banned, read-only, dangerous) and bashPermissionRequest (FamilySuggestion from AskedKeys; none for an irreversible command). On PowerShell the notes say ';' and the bad example uses ';'. A note tells the model what "Allow family" means. |
| `internal/llm/tools/shell/shell.go` | modified | The command text was pasted into the wrapper block `& { ... }`. A parse error in 5.1 discarded the whole chunk; no status file; 60 s timeout; stopRunning killed the shell. A top-level throw aborted the chunk the same way. | Command sent base64 (UTF-8); [System.Management.Automation.Language.Parser]::ParseInput first; parse errors written to the stderr file with line:column and exit 2; else [scriptblock]::Create + & inside try/catch, output collected into an ArrayList so output before a throw survives; a throw is exit 1 with the record; the status file is still written last. The '&&' hint is appended when the parser message contains "is not a valid statement separator". Set-Location and $env: persistence unchanged (measured by tests). |
| `internal/tui/components/dialog/permission.go, internal/tui/tui.go, internal/plain/plain.go, internal/acp/server.go` | modified | Three buttons, fixed indices. Plain mode y/n/a. ACP allow/allow-session/deny. | options slice (Allow, Allow for session, Allow family when a suggestion exists, Deny last and default); f answers with the family shown; [ and ] walk BashFamilyCandidates; the family, its position, BashFamilyNote and the never-covered list are rendered under the command; dialog height 0.7 when a family is shown. Plain mode [f]amily with the suggestion printed. ACP option allow-family whose name carries the prefix. tui.go: GrantFamily on PermissionAllowFamily; /mode and Shift+Tab (keys.CycleMode) call setPermissionMode; /yolo reports when mode is already auto. |
| `internal/config/permissionmode.go, internal/config/config.go, internal/commands/registry.go, docs/COMMANDS.md, docs/SETTINGS.md` | added/modified | No persisted mode. | Config.Permissions.Mode; PermissionMode, SetPermissionMode (updateCfgFile), NormalisePermissionMode (yolo, readonly, default, 'Accept Edits' accepted), NextPermissionMode, PermissionModeLayman; Settings row permissions.mode (KindEnum). /mode command registered; both docs regenerated. |
| `internal/tui/components/core/status.go` | modified | Chip only while the session was /yolo-armed. | permission.EffectiveMode: AUTO (error colour), EDITS auto-approved (warning), PLAN read-only (primary), nothing for ask. |
| `internal/llm/prompt/coder.go, prompt.go, plan.go, task.go, internal/llm/agent/agent.go` | modified | getEnvironmentInfo() had no model line; prompts were built from a provider only. | getEnvironmentInfo(model) adds `Model: you are <id> (<name>). State this id, not a guess, when asked which model you are.` inside <env>; CoderPromptFor/PlanPromptFor/SubCoderPromptFor/TaskPromptFor take a models.Model; createAgentProviderFor passes the model it builds with, so a /model switch (which builds before committing config) names the new model. Old signatures delegate through configuredModel(agent). |
| `internal/permission/export_test_seam.go, internal/llm/tools/session_replay_test.go` | added | No way to replay a recorded session through the gate. | PermissionServiceForTest alias for cross-package tests. TestReplaySessionPrompts (GORILLA_REPLAY_DB, GORILLA_REPLAY_SESSION) copies the database, reads the session's bash calls (helper sessions included), runs the gate without executing, answers prompts with family-if-offered-else-session, and prints per-call verdicts and the three counts. |

### Subsystem Changes

**TOOLS:** bash gate split; command families; PowerShell wrapper rev 3.

**NETWORK:** None added. A family never covers an Egress request; bash requests carry no Egress flag, so the ban list remains the shell's network control. No permission decision leaves the machine.

**STORAGE:** config.json gains permissions.mode. Family grants are in memory only.

**OTHER:** docs/COMMANDS.md and docs/SETTINGS.md regenerated; a test holds each to its generator.

### Test Coverage

- **Added:** permission/family_test.go (coverage, boundary, tool/action/path/tree binding, pathWithin, never Irreversible/Egress/tainted, under auto-approve, invalid family not recorded, no matcher no grant, IsCovered, AskedKeys); permission/mode_test.go (ask unchanged; accept-edits in-root only, not bash, not outside; plan refuses without publishing, web still asks; auto for every session with carve-outs; cycle and spellings); tools/commandfamily_test.go (the Nemotron commands verbatim, pytest pipes, fallback suggestions, chained/redirected/substituted/unbalanced tails under both shells, quote rules per shell, note, candidates, banned/dangerous/tainted still ask on the real service, read-only gate unchanged, covered requests return without publishing); shell/parse_error_test.go (&& fails at once with the hint and the shell lives, unterminated quote, throw keeps output, Set-Location persists, UTF-8/quotes/newlines round trip, native exit code); prompt/model_line_test.go; dialog/permission_family_test.go. carveout_test.go updated for decision B.
- **Removed:** Not available in the source material.
- **Notes:** Complete suite passes on Windows (go test ./... exit 0). Replay of session 9af3336f: 31 bash calls, 0 banned, 0 read-only; 31 prompts if every call asked, 30 with "allow for session" per distinct command, 7 with "allow family". Windows binary (the published file) against local.nvidia/nemotron-3-super-120b-a12b: 7 of 7 harness scenarios valid (4 s to 44 s each). Adversarial review (three reviewers, verifiers) confirmed one finding, left as designed and disclosed: a family ending inside a quoted string (the fieldkit prefix ends inside python -c "...") constrains nothing after it, so arbitrary interpreter code is repeatable without a prompt; the dialog says so under the family; plain mode and ACP show the family without that note. NOT RUN: an interactive session on Linux; the plan-mode refusal sentence (PlanModeRefusal) is defined but not wired in agent.go, so plan mode returns "Permission denied" (the Claude Code auto-mode classifier denied that edit three times during the build).

### Security Posture

A family grant is an explicit, displayed widening of one session grant, in memory only, bounded to one plain command shape and one path scope, and refused for Irreversible, Egress and tainted requests. Decision B narrows the taint carve-out: after untrusted content, edit tools inside the roots are approved under auto and accept-edits; shell, egress, outside-root paths and irreversible actions still ask. Mode auto is persisted and visible in the status bar; no mode removes the carve-outs. Compared with Claude Code (the tool used to build this release): Claude Code has no taint bit (a fetched page changes nothing about later permissions); its bypassPermissions mode approves every action including irreversible ones; its auto mode delegates each decision to a server-side classifier that returns a label and no reason, and that classifier denied three edits to this repository on 11 October. Every Gorilla rule is local, in source, and under test; nothing about a permission decision leaves the machine.

### Deployment

**Prerequisites:**
- Windows 10/11 x64, or linux/amd64.

```bash
# fetch
gh release download v0.1.147 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, .deb, .rpm, SHA256SUMS-v0.1.147.txt.
# verify
sha256sum -c SHA256SUMS-v0.1.147.txt --ignore-missing
# Expected: Each present file followed by OK. Windows exe: 0ba2ba0b2fd742195d53f9690649fb6073e7fedb51c2d3685047fc22b3ddaf00.
# install
.\gorilla-opencode.exe install
# Expected: Install path and shortcuts.
# verify_active
gorilla-opencode --version
# Expected: v0.1.147
# mode
/mode auto   # inside the program; or Shift+Tab
# Expected: Warning that the mode is persisted; status bar shows AUTO.
```

**Rollback:**
  1. Remove this version
     `gorilla-opencode uninstall   # or apt remove / dnf remove gorilla-opencode`
  2. Install v0.1.146 (permissions.mode in config.json is ignored there)
     `gh release download v0.1.146 -R gorillanobakaa-dot/Gorilla.Opencode`

### Known Issues

**[low]** No "Allow family" button on a bash prompt
- Cause: SuggestCommandFamily returned "": first prompt of its shape with a flag as the second word, a chained/redirected command, a one-word command, or an irreversible command.
- Remedy: Answer "allow for session"; the next prompt proposes the shared prefix.

**[low]** A command inside a family still prompts
- Cause: Irreversible, Egress, tainted root, or Path not within the grant's Path.
- Remedy: By design; see familyCovers.

**[low]** Plan mode tool results read "Permission denied" (deferred to v0.1.148)
- Cause: PlanModeRefusal is not wired in agent.go in this build.
- Remedy: Next version.

**[medium]** A family ending inside a quoted string covers arbitrary code for that interpreter
- Cause: The tail inside the quotes is handed to the program as its code; the scanner does not read program syntax.
- Remedy: Choose a shorter family with [, or answer "allow for session"; the dialog note names the condition.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| 31 prompts, 30 distinct, 16 variants | 📄 stated in input | session 9af3336f counted from gorilla-opencode.db |
| Replay 31 -> 7 | 📄 stated in input | TestReplaySessionPrompts output |
| Parse error mechanism | 📄 stated in input | probe: ParserError at 320 ms on the shell's stderr; AFTER never printed; status file absent |
| Binary hash and size | 📄 stated in input | sha256 0ba2ba0b2fd742195d53f9690649fb6073e7fedb51c2d3685047fc22b3ddaf00, 55,270,912 bytes |
| Scope | 📄 stated in input | 37 files changed, 2931 insertions, 130 deletions |
| Claude Code classifier denials | 📄 stated in input | three Edit calls refused with the labels [Create Unsafe Agents] and [Auto-Mode Bypass] on 2026-10-11 |
| Reviewer finding on quoted families | 📄 stated in input | reproduced on the real permission service by two verifiers |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*
