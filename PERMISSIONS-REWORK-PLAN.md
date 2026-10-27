# Permissions rework: safe without being unusable

Version: 1.0.0 · updated 26-10-23-02-05

Owner, 2026-10-23, watching Gorilla OpenCode work on the MacBook Air (translated from Romanian):
"yet another security dialog ... you will have to write a project that removes this nonsense and these
hallucinations produced by an over-cautious Fable. You cannot work with a coding agent like this. long horizon
tasks? On gorilla.opencode? forget about it." Earlier: "it is so safe and asks for so many prompts that it becomes
almost unusable ... safer than Claude Desktop or the Claude Code terminal."

## Evidence (measured 2026-10-23, v0.1.149-dev, macOS 10.13, Gemini 3.8 Flash High, auto-approve ON)

One stranger task ("install Fieldkit into ~/Stranger-Test") raised a dialog for every step after the first web page:

| # | Tool | What it really did | Reason printed | Field shown in the dialog |
|---|---|---|---|---|
| 1 | bash | read `/Library/Developer/CommandLineTools` (a probe) | "outside every workspace root" | `Command` EMPTY |
| 2 | web_fetch | read the project's README on GitHub | "this sends a request off the machine" | `URL` EMPTY |
| 3 | web_fetch | read `INSTALL.md` on the same site | "already read untrusted content" | `URL` EMPTY |
| 4 | bash | `python3 --version && git --version` | "already read untrusted content" | `Command` EMPTY |

Also seen: after "Deny" on dialog 1 the agent froze at "Waiting for response..." and never continued (the owner
had to type something else). Every dialog said "Asking even though auto-approve is on".

The owner's screenshot and the harness's screenshot were identical: the empty field is not a capture artefact.

## What is wrong (model inference, to confirm in internal/permission/ and dialog/permission.go)

1. **The dialog hides the one thing a person needs** (the command, the URL). A prompt you cannot judge trains
   "Allow" by reflex: less safe, not more.
2. **Taint is too wide** (taint.go): after any web page, every later tool call asks, including read-only
   commands with no network and no write. Taint should guard the exfiltration path (network sends, writes outside
   the root, shell with network tools), not `--version`.
3. **Reads outside the workspace ask.** Reading `/Library/...`, `which`, `ls`, `xcode-select -p` is how an agent
   learns the machine. Reads should not ask; writes, deletes, installs, sudo and new network destinations should.
4. **Auto-approve does not mean auto-approve.** Four carve-outs override it, so the mode the person chose is not
   the mode they get.
5. **Deny freezes the turn** instead of returning "denied by the person" to the model so it can carry on (the
   stranger was told to write the question in QUESTIONS.txt and continue).

## Target behaviour (proposal)

- **Never ask** for: read-only file access anywhere the user can read; read-only shell (an allowlist of verbs:
  ls, cat, head, which, type, file, stat, find without -exec/-delete, grep, git status/log/diff, `--version`,
  `--help`, uname, sw_vers, sysctl -n, df, xcode-select -p); web GETs to the same host the user named.
- **Ask** for: writes/deletes outside the workspace roots; installs (pip/npm/brew/apt/...), sudo, chmod/chown
  outside the root; network SENDS (POST/PUT, curl -d/-T, scp/rsync/ssh to a new host); a new network host after
  untrusted content was read (that is the real taint case).
- **The dialog always shows the full command or URL**, wrapped, plus one plain line of what it will do
  ("reads a version number", "downloads 4 MB from github.com into Stranger-Test").
- **"Allow for session" remembers the class and the place** (e.g. "read-only shell", "GETs to github.com",
  "writes inside ~/Stranger-Test"), not one exact string.
- **Deny returns to the model** with "the person denied this; do not retry it, ask in text or continue".
- **Auto-approve** keeps exactly one hard stop: the destructive/exfiltration list above. Everything else runs.
- The rules are a table (data) with a test per row, and a counter: a measured task that asked N times before must
  ask at most M times after (target for the stranger task: 0 dialogs until something is installed).

## Done when

- The stranger install task on the Air runs with auto-approve and no dialog for reads, versions or README fetches,
  and still stops for a system-wide install or sudo.
- Every dialog that remains shows its command or URL in full (screenshot proof, full size, directive 13).
- Deny no longer freezes a turn (a test).
- The four rows of the evidence table become regression tests.

Before starting: this checkout is 10 commits behind origin (sync first), and read ROADMAP.md and the project's
LESSONS/ store: the current rules came from a deliberate hardening; record which earlier fear each relaxed rule
answered and why the new rule still covers it.

## Evidence, run 2: every dialog of one install (2026-10-23 02:03-02:15, auto-approve ON)

The stranger installed Fieldkit successfully (zip + lean, READY). Before that run's log started it had
already met 7 dialogs (README fetch, INSTALL.md fetch, `python3 --version && git --version`, install.sh
fetch, a second fetch, a GitHub API check, the zip download); web fetches were then set to 'Allow for
session'. The 26 below came one per bash command. Every one ran inside ~/Stranger-Test; none needed a
person. 'not shown' = the dialog's Command field was empty.

| # | time | command |
|---|---|---|
| 1 | 02:06:09 | `python3 -m zipfile -e /Users/gorilla/Stranger-Test/fieldkit.zip /Users/gorilla/Stranger-Test/` |
| 2 | 02:06:46 | `PYTHONPATH="/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install" python3 -m fieldkit.core.doctor --probe` |
| 3 | 02:07:00 | `PYTHONPATH="/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install" python3 -m fieldkit.core.doctor --lean` |
| 4 | 02:07:14 | `PYTHONPATH="/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install" python3 -m fieldkit.core.doctor` |
| 5 | 02:07:44 | (not shown) |
| 6 | 02:08:13 | (not shown) |
| 7 | 02:08:27 | (not shown) |
| 8 | 02:08:41 | (not shown) |
| 9 | 02:08:54 | `/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install/.venv/bin/fieldkit doctor --lean` |
| 10 | 02:09:40 | (not shown) |
| 11 | 02:10:01 | (not shown) |
| 12 | 02:10:15 | `/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install/.venv/bin/fieldkit cards list` |
| 13 | 02:10:45 | `/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install/.venv/bin/fieldkit agent discover "privacy scan"` |
| 14 | 02:11:14 | `/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install/.venv/bin/fieldkit agent discover --include-drafts  "privacy scan"` |
| 15 | 02:11:28 | `/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install/.venv/bin/fieldkit privacy scan  /Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install` |
| 16 | 02:11:42 | `/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install/.venv/bin/fieldkit macair status` |
| 17 | 02:11:56 | `/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install/.venv/bin/fieldkit mac --help` |
| 18 | 02:12:17 | (not shown) |
| 19 | 02:12:31 | (not shown) |
| 20 | 02:12:45 | (not shown) |
| 21 | 02:12:59 | `/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install/.venv/bin/python -m fieldkit.core.doctor --probe` |
| 22 | 02:13:20 | `/Users/gorilla/Stranger-Test/Gorilla.Fieldkit-feat-stranger-install/.venv/bin/python -m pip install pytest` |
| 23 | 02:13:34 | (not shown) |
| 24 | 02:13:47 | (not shown) |
| 25 | 02:14:41 | (not shown) |
| 26 | 02:14:55 | (not shown) |

Total for one install: 33 security dialogs, 0 of them for anything risky.
