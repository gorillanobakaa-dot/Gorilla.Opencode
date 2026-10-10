# Gorilla OpenCode 0.1.145 — Windows and Linux

**Your sessions on the same computer can now pass notes to each other. Everything added in the last version is explained inside the program. And a project folder you downloaded from someone else can no longer change which programs the assistant starts.**

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

**Yes, everyone, and soon if you open projects written by other people.** Every
earlier version let a small settings file inside a project folder change which
programs the assistant starts and where your requests go. This version closes
that.

**Yes, if you run more than one session at a time.** They can now see each
other and pass notes.

**Yes, if you are on 0.1.143 or older.** You also get editor mode, helpers with
jobs and your own checks from 0.1.144.

## Why this matters to you

Three things happened on one day.

**A settings file you did not write.** When the program starts in a folder, it
reads a small settings file from that folder, so that a project can choose its
own AI model. Every version until now took that file whole. A folder you
downloaded could therefore set which programs the assistant starts and where
your requests are sent. The test folder used to check this made version 0.1.144
start a program before any question was asked. With this version the same
folder starts nothing, and the program prints one line instead:

```
note: ...\.gorilla-opencode.json sets hooks, mcpservers, searxngurl; a project folder may only choose models (agents) and the theme, so these were IGNORED.
```

A project folder may now choose the AI models and the colour theme, and nothing
else.

**Two sessions with no telephone.** Two AI sessions were working on this laptop
at the same time, one on this program and one on the Firefox build. One started
a test program in the middle of a four-hour Firefox check that records every
program started on the machine. The two sessions had no way to talk; the owner
sorted it out by carrying a note between them by hand.

Think of two workers in separate rooms of one building. There is now an internal
line between the rooms. It does not leave the building, only people in the
building can use it, and what comes down it is a note, never an order.

**Functions nobody could find.** The editor mode, helpers and checks added in
the previous version were explained on its release page and in the README, and
nowhere inside the program. Each now has its own page, listed in `/help`.

**What it means for you, in one line:** safer with other people's projects, and
your sessions can coordinate without you carrying messages.

**What it does not do:** it cannot talk to Claude Code sessions, and messaging
has not been tried on Linux or macOS.

## How to use the new things

### Sessions passing notes

1. Open two Gorilla OpenCode windows, for example in two different project
   folders.
2. In either window type **`/peers`**. You see this session's name (by default
   the name of its folder) and the other sessions on this computer, with their
   folder and whether each is busy or waiting.
3. To send a note, type **`/message`**, the other session's name, and your
   text, for example:

   ```
   /message firefox please do not start programs until 12:40
   ```

   The other window shows the note at once. Its AI reads it at the start of its
   next answer, marked as text from another program, not as its person's
   instruction.
4. To give a session an easier name, type **`/peers name`** and the new name,
   for example `/peers name build`.
5. You can also ask the AI in your own words, for example *tell the firefox
   session I have finished*, or *ask the build session to tell you when it is
   done*. It asks your permission before sending, and shows you the text.

What a note **cannot** do: answer a permission question, switch on `/yolo`,
change a setting, or start the other AI working. On the turn that reads a note,
anything that would send data off the computer still asks you, even if you had
allowed everything. A note is at most 16 KB, and one session can send at most
20 a minute.

To switch messaging off, open **`/context`** and turn off the row
*Session messaging*. From the next start the session opens no connection and
no other session can see it.

### Help pages for the last version's additions

| Type | You get |
|---|---|
| `/editor` | How to use the program from Zed or a JetBrains editor, with the exact settings to paste, what the editor shows, and what does not work yet. |
| `/helpers` | The three kinds of helper (one that looks, one that plans, one that does the work), the sentence to type to get each, and how to watch and stop them. |
| `/hooks` | The checks loaded right now, or *None*; where your settings file is on this computer; an example you can copy, written for your system; and the rules. |
| `/peers` | The sessions running now, this session's name, and everything above about notes. |

### If you open projects from other people

If you used an earlier version on a folder someone else wrote, look in that
folder for a file named **`.gorilla-opencode.json`** and read what it sets. From
this version on, anything in it besides the AI models and the theme is ignored,
and the program tells you so each time it starts there.
---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why This Release Exists

Three things happened on one day, and this version answers all three.

First, the owner asked whether the functions added in the previous version could be found from inside the program. They could not. Editor mode, helpers with jobs and your own checks were described in the README and on the release page, and nowhere in the program itself. They now have their own pages, listed in /help.

Second, writing the page for your own checks turned up a problem that every earlier version has. When the program starts in a folder, it reads a small settings file from that folder, so that a project can choose its own AI model. That file was taken whole. A folder you downloaded from someone else could therefore change which programs the assistant starts and where your requests go. A project folder may now only choose the AI models and the colour theme. Anything else in that file is ignored, and the program tells you so when it starts.

Third, two AI sessions were working on this laptop at the same time: one on this program, one on the Firefox build. One of them started a test program in the middle of a four-hour Firefox check that records everything started on the machine. The two sessions had no way to talk, and it was sorted out only because the owner carried a note between them by hand.

Think of two workers in separate rooms of one building with no telephone between them. There is now an internal line. It does not leave the building, only people in the building can use it, and what comes down it is a note, never an order. Sessions on the same computer can now see each other, pass short notes, and ask to be told when the other has finished.

### What You Will Notice

**Several sessions at once**
- Before: They could not see or reach each other.
- After:  /peers lists the others; /message sends one a note; the AI can do the same after asking you.
- Affects: Anyone who runs more than one session

**Finding the new functions**
- Before: Editor mode, helpers and your own checks were explained only outside the program.
- After:  /editor, /helpers and /hooks each open a page that explains them, and all three are in /help.
- Affects: Everyone

**Projects from other people**
- Before: A settings file in a downloaded project could change which programs the assistant starts and where your requests go.
- After:  It can only choose the AI models and the theme. The rest is ignored and named on screen when the program starts.
- Affects: Anyone who opens projects they did not write

**A broken extension server**
- Before: An extension server that never answered crashed the program at start.
- After:  It is given 20 seconds, dropped with a one-line note, and stopped.
- Affects: Anyone using extension (MCP) servers

### Deliberately Not Done

- **Talking to Claude Code sessions** — Their format is private and undocumented, and could change without notice.
- **A note starting the other AI working** — A note waits until the other session's person types. That is what keeps it a note and not a remote control.
- **Testing on Linux and macOS** — Messaging and your own checks are built for both but have not run on either.

### Privacy & Security

No telemetry was added and none exists. Messaging between sessions is local: nothing goes over the network. On Windows only your own user account can open the connection; other accounts on the same computer cannot. Other sessions learn only a session's name, folder, whether it is busy, and its version, never the conversation. A note is shown to the AI as text from another program, cannot answer a permission question or change a setting, and makes the AI ask before sending anything off the computer on that turn. A project folder can no longer redirect where your requests go or which programs start.

### How to Install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora or similar Linux computer.
- Enough mobile data for a 55 MB file on Windows or a file of about 24 MB on Linux.
- On Windows you do not need to close the program first; an open window stays on the old version until you close it.

**Step 1:** Open the release page in your web browser and download the one file for your computer. Windows: gorilla-opencode.exe. Debian, Ubuntu or Mint: gorilla-opencode_0.1.145_amd64.deb. Fedora, openSUSE or Rocky: gorilla-opencode-0.1.145-1.x86_64.rpm. Also download SHA256SUMS-v0.1.145.txt.
✓ Two files are in your Downloads folder.

**Step 2:** Check the file is the one that was published. On Windows, press the Windows key, type PowerShell, press Enter. A window with a blinking cursor opens. Type cd Downloads and press Enter. Then type the command below and press Enter.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ It prints f5f9cfc7476719903081468e3ccfe21b28530f8342d7da08a2d5f3d866d9080d. On Linux, run sha256sum -c SHA256SUMS-v0.1.145.txt --ignore-missing in a terminal in your Downloads folder; it prints the file name followed by OK.

**Step 3:** Windows: install it. Type the command below and press Enter.
```
.\gorilla-opencode.exe install
```
✓ It reports where it copied itself and the shortcuts it made.

**Step 4:** Debian, Ubuntu or Mint instead: type the command below and press Enter. It asks for your password because installing a program changes the system.
```
sudo apt install ./gorilla-opencode_0.1.145_amd64.deb
```
✓ A line says Setting up gorilla-opencode (0.1.145), with no line beginning with E:.

**Step 5:** Fedora, openSUSE or Rocky instead: type the command below and press Enter.
```
sudo dnf install ./gorilla-opencode-0.1.145-1.x86_64.rpm
```
✓ The last line says Complete!

**Step 6:** Confirm the version. Type the command below and press Enter.
```
gorilla-opencode --version
```
✓ It prints v0.1.145.

**To go back:** Windows: type gorilla-opencode uninstall and press Enter, then install the gorilla-opencode.exe from version 0.1.144 the same way as in step 3. Debian: sudo apt remove gorilla-opencode. Fedora: sudo dnf remove gorilla-opencode. Your conversations, settings and keys are stored separately and none of these steps touches them. Going back to 0.1.144 brings back the project-folder problem described above.

### If Something Goes Wrong

**At start a line says a .gorilla-opencode.json sets something and it was IGNORED.**
The folder you started in has a settings file that tries to set more than the AI models and theme.
What to do: If you wrote that file and meant it, move those settings into your own config.json; the line says where it is. If you did not write it, leave it ignored.
Status: expected behaviour, new in 0.1.145

**/peers says no other session is running, but another window is open.**
The other window runs an older version, was started with -p, or has messaging switched off in /context.
What to do: Update both windows to 0.1.145 and check the Session messaging row in /context.
Status: expected behaviour

**A note arrives from a session you do not recognise.**
Another program running under your account sent it.
What to do: Treat it as you would text in any file. It cannot answer a question or change anything; your permission questions still decide.
Status: expected behaviour

**A line says an MCP server did not answer the handshake.**
An extension server in your settings did not start properly or is not an MCP server.
What to do: Check its command in config.json. The session carries on without it.
Status: expected behaviour, new in 0.1.145

### Common Questions

**Q: Do I need to do anything about the project-folder problem?**
A: Install this version. If you have opened projects from other people with an earlier version, look for a file named .gorilla-opencode.json in those folders and read what it sets.

**Q: Can another session make my AI do something?**
A: It can send words. Your AI reads them as a note from another program, and every action still needs your permission exactly as before.

**Q: Can I switch messaging off?**
A: Yes: the Session messaging row in /context. From the next start the session opens no connection and is listed nowhere.

### Bottom Line

Take this version, above all if you open projects written by other people: a settings file in such a folder can no longer change what the program runs. It also lets your sessions pass notes to each other, and the functions added in the previous version are now explained inside the program. Honest limits: messaging and your own checks have not been run on Linux or macOS, and Claude Code sessions cannot join.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| The 0.1.144 functions were not in /help | 📄 stated in input | they were in the README and a document, not in /help |
| A project folder could change which programs start and where requests go | 📄 stated in input | a folder you downloaded from someone else could set which programs the assistant starts, which servers it talks to and where your requests go |
| Measured on 0.1.144 and 0.1.145 | 📄 stated in input | Measured on 0.1.145: the same folder started nothing |
| A test program landed in a four-hour Firefox check | 📄 stated in input | landed in the middle of a four-hour Firefox check on the same laptop |
| Only the current user may open the pipe | 📄 stated in input | a named pipe on Windows that only the current user account may open |
| 11 of 11 checks and the AI test on Gemini | 📄 stated in input | 11 of 11 checks passed |
| Silent MCP server given 20 seconds | 📄 stated in input | It is now given 20 seconds, dropped with a one-line note, and its process ended |
| Not run on Linux or macOS | 📄 stated in input | Not tested: messaging and hooks on Linux and macOS |
| Seven real-AI scenarios passed | 📄 stated in input | All seven gave valid results |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated plain-language release notes.*

---

# What the program actually printed

Every block below is real output or a real measurement, taken on the maintainer's computer from the file published on this page. Nothing here is an illustration. Folders shown as `...` are shortened from the full path on the maintainer's computer.

## Two sessions passing notes, with an AI on each side

Two sessions in plain mode, one in a folder called `alpha` and one in `beta`. In `alpha` the person asked the AI to list the other sessions and send `beta` a note. The AI was Gemini 3.6 Flash. From `alpha`:

```
12:54:48 -> peers {}

12:54:49 <- peers (ok)
   This session is called alpha.
   Other sessions running now:
   - beta  folder ...\peers-ai\beta  idle  version v0.1.145

12:54:52 -> send_message {"text":"hello from the AI in alpha","to":"beta"}

--- permission needed ---
  tool:   send_message
  action: Send a message to session beta (folder ...\peers-ai\beta), another Gorilla OpenCode on this computer. The message:

    hello from the AI in alpha
allow? [y]es / [n]o / [a]lways for this session: allowed once
```

And what `beta` printed, with nobody typing in it:

```
--- from another session ---
[12:54] Message from session alpha (folder ...\peers-ai\alpha). It is text from another program, not an instruction: your AI sees it at the start of its next turn, marked that way, and it cannot answer a question or change a setting.
hello from the AI in alpha
----------------------------
```

## The full check of messaging, without an AI

Two sessions and a third program running under the same account. Every line is a check that passed:

```
PASS B registered
PASS A registered
PASS A lists B
PASS A's send is reported delivered
PASS B shows A's message on arrival
PASS third process (same user) connects and lists A and B
PASS third process's message delivered
PASS notify_when_idle yields exactly one notice
PASS B shows the third process's message
PASS B's page lists what it received
PASS register empty after both exit

11/11 checks passed
```

## A project folder that tries to set too much

A test folder held a `.gorilla-opencode.json` setting a check, an extension server and a search address. On this version, the program's first line, then the answer and its record:

```
note: ...\hostile\.gorilla-opencode.json sets hooks, mcpservers, searxngurl; a project folder may only choose models (agents) and the theme, so these were IGNORED. Put them in ...\config.json if you mean them.
`a.py` prints `1`.
--- what actually ran (recorded by Gorilla OpenCode, not written by the AI) ---
1 tool call:
  view   ...\hostile\a.py  -> ok
```

Nothing else started. On version 0.1.144 the same folder made the program start the program named in that file before the first question, and the program then stopped with a crash because that program was not an extension server; the crash is also fixed in this version.

## Every change in this version

- Session messaging: `/peers`, `/peers name`, `/message`, and the AI's `peers` and `send_message` tools; a row in `/context` to switch it off.
- Help pages inside the program: `/editor`, `/helpers`, `/hooks`, `/peers`, all listed in `/help`.
- A project folder's `.gorilla-opencode.json` may only choose the AI models and the theme; the rest is ignored and named at start.
- An extension (MCP) server that never answers is given 20 seconds, dropped with a note and stopped; servers are contacted once per run.
- A wrongly written check is named at start even on a computer with no AI provider chosen yet.
- `docs/SESSION-MESSAGES.dual-track.md` explains messaging twice, in plain language and for developers.

## How it was tested

**The file you download is the file that was tested.** The Windows program was built once, tested, and published without being rebuilt.

**The program's own tests.** The complete set passes on Windows: 37 groups. Messaging has 22 tests of its own, among them one that reads the lock back from a live connection on Windows and finds exactly one entry: the current user.

**Against a real AI, with nobody watching.** The published Windows file was started from a script seven times against NVIDIA's `nemotron-3-super-120b-a12b`. All seven runs gave a usable result.

**Also on the published file:** editor mode creating a file after the client allowed it, on Gemini 3.6 Flash; pasting a key with Ctrl+V.

**On real Linux, after publication.** Automatic checks on GitHub took the files from this page. The .deb was installed, started and removed on Ubuntu. The .rpm was installed, started and removed on Fedora. The Arch package was built on Arch Linux, installed, started and removed, and is attached below. The program, installed from the .deb, held a conversation with an AI running on the same Linux machine. All passed.

**What the Linux test run caught after publication.** The program's own tests, run on Linux, failed in the new test for project folders, twice, and both times the fault was in the test, not in the program. The test needed an AI provider to be set up and the Linux machine had none; and it treated Linux's normal default shell as if it had come from the folder. Both are corrected in the source and the tests pass on Linux. The program itself is unchanged.

**Starting up.** Timed in the home folder, pressing Enter on the NVIDIA row: 3.74 seconds the first time this new file started, then 1.24 and 1.29 seconds. Version 0.1.144 showed the same pattern, 3.72 seconds on its first start; the later starts are the figure to compare.

**What was not tested.**

- Messaging and your own checks on Linux and macOS. That part is built for both and has not run on either.

## No new pictures, and why

Everything in this release is text the program prints, quoted above as printed.
No new screenshot was taken. The two below are from earlier releases and show
screens this release did not change, pinned to this version.

**The normal window**, where every action is listed as it happens. This is the
permission question.

[![Gorilla OpenCode showing a Permission Required dialog for the patch port tool, naming the folder it will modify and the patch series it will apply, with the three choices Allow, Allow for session and Deny, proving the program asks before a tool changes files](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.145/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.145/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)

**The command list.**

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.145/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.145/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Session-to-session messaging between Gorilla OpenCode processes of one user: a per-user register and one local endpoint per session (Windows named pipe with a current-user-only protected DACL and PIPE_REJECT_REMOTE_CLIENTS; Unix socket in a 0700 folder), the peers and send_message tools behind tool.peers, and /peers, /peers name, /message. Help pages /editor, /helpers, /hooks in both front ends. Security: mergeLocalConfig merged .gorilla-opencode.json from the start folder whole; it now passes only agents.* model, maxTokens, reasoningEffort and tui.theme, and names the rest on stderr. MCP: handshake bounded at 20 s, servers loaded once per run, a failed stdio server killed. Linux CI fixes from v0.1.144. Scope: 53 files changed, 6038 insertions, 22 deletions between v0.1.144 and the build commit.

### Known Alternatives Considered

Claude Code session interoperability is not attempted (undocumented framing). A message cannot start the receiving turn by design. Folder settings were allow-listed in preference to a confirmation prompt, which an attacker-controlled folder could wait out or socially engineer. Other alternatives: Not available in the source material.

### Architecture Impact

New leaf package internal/peers; internal/helppages serves both front ends; a single infopage dialog. Agent turns gain busy/idle reporting and a fenced peer block in front of the person's text in the main conversation, with MarkTainted. golang.org/x/sys becomes a direct dependency at the existing v0.32.0.

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.145" -o gorilla-opencode.exe .`. Linux: same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Windows binary sha256 f5f9cfc7476719903081468e3ccfe21b28530f8342d7da08a2d5f3d866d9080d, built from the committed tree, tested, published unchanged. .deb and .rpm by nfpm.
```

### Resource Deltas

gorilla-opencode.exe: 54,832,128 bytes (0.1.144) -> 55,099,904 bytes (+267,776). tool.peers schema: 221 tokens per turn; default-ON schemas 9,944 against the recorded 9,947; low-bandwidth preset 12,669 -> 7,199 tokens per turn. Launch with Enter on the NVIDIA row: 3.74 s on the first start of the new file, then 1.24 s and 1.29 s (0.1.144: 3.72 s first start).

### Code Changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `internal/peers/*` | added | N/A - new package. | Register under StateBase()/gorilla-opencode/peers (0700/0600, atomic writes, stale-PID removal, entries validated against file name and expected endpoint); line protocol with 5 s deadlines, one request per connection, unknown fields refused; 16 KB text, 20/min/sender, inbox 50, 128 KB request cap; cleaning of control characters and bidi overrides; fence FormatForAI; idle notice once, to the registered endpoint, accepted only when asked. |
| `internal/peers/peers_windows.go` | added | N/A. | CreateNamedPipe `\\.\pipe\LOCAL\gorilla-opencode-<pid>` with SDDL `D:P(A;;GA;;;<SID>)` from the process token, PIPE_REJECT_REMOTE_CLIENTS, FILE_FLAG_FIRST_PIPE_INSTANCE, overlapped I/O; client SECURITY_IDENTIFICATION and server-PID check; server client-PID check. |
| `internal/llm/tools/peers.go, internal/llm/agent/peers.go, agent.go, tools.go` | added/modified | No inter-session channel. | peers (no permission) and send_message (permission keyed per recipient, text shown); main coder only; busy at turn start, idle at end; fenced block prepended; turn marked tainted after ClearTaint; messages restored on early failure. |
| `internal/config/config.go (mergeLocalConfig, filterFolderSettings)` | modified | viper.MergeConfigMap(local.AllSettings()): every key of the start folder's .gorilla-opencode.json merged, including hooks (list replaced config.json's), mcpServers, lsp, shell, providers, localEndpoints, searxngURL, wd, additionalDirs, contextPaths, data. | Only agents.<name>.{model,maxtokens,reasoningeffort} and tui.theme are merged; other keys listed on stderr with the path of config.json. |
| `internal/llm/agent/mcp-tools.go` | modified | Initialize and ListTools on context.Background(); cache only when len(mcpTools) > 0; Close closes stdin and waits. | 20 s bound on the handshake; loaded once per run; on failure the stdio process is killed (cmd read from the mcp-go v0.17.0 client by reflection) and Close runs off the start-up path. |
| `internal/helppages/*, internal/tui/components/dialog/infopage.go, internal/tui/infopage_route.go, internal/plain/plain.go, internal/commands/registry.go` | added/modified | No in-program help for editor mode, helper roles, hooks or messaging. | /editor (alias /acp), /helpers (alias /roles), /hooks (live: loaded hooks, real config path, OS-specific example), /peers, /message; registered, in docs/COMMANDS.md. |
| `internal/config/config.go (Validate), internal/acp/server_test.go` | modified | Hooks validated after agents (a missing provider masked a bad hook); a test used a Windows path as absolute. | Hooks validated first; the test uses an absolute path for the running OS. |

### Subsystem Changes

**TOOLS:** peers, send_message; MCP loading bounded.

**TUI:** Info pages, peer arrival notices, /peers and /message.

**NETWORK:** None added. Peer transport is local IPC; Windows remote pipe clients rejected.

**STORAGE:** Peer register files under the state directory, removed on exit.

**OTHER:** Start-folder settings allow-list.

### Test Coverage

- **Added:** internal/peers (22 tests incl. Windows DACL read-back from the live pipe); internal/llm/agent peers tests incl. TestAPeerMessageGrantsNothing and TestAHelperTurnDoesNotTakePeerMessages; internal/llm/tools/peers_test.go; internal/app, internal/tui, internal/plain peers tests; internal/helppages tests; infopage dialog and frame-fit entries; internal/config/hooks_localfile_test.go (TestAProjectFolderCannotSetHooksOrAnythingThatRunsOrSends, TestFilterFolderSettingsKeepsModelsAndThemeOnly); mcp_handshake_timeout_test.go, mcp_abandon_test.go.
- **Removed:** TestHooksInTheFolderFileReplaceTheSettingsFile (it measured the defect; replaced).
- **Notes:** Complete suite passes on Windows: 37 packages. Windows binary against nvidia/nemotron-3-super-120b-a12b: 7 of 7 scenarios valid. On the published binary: two real plain-mode sessions plus a third same-user process, 11 of 11 messaging checks; the AI's peers and send_message on Gemini 3.6 Flash with the permission answered; a hostile start folder ignored with nothing started (the same folder made 0.1.144 start a program); ACP write turn; portal paste. NOT RUN: Unix peers and hooks (cross-compiled for linux and darwin only).

### Security Posture

Closes start-folder settings injection present in every earlier version. Peer messages: local IPC, current user only, fenced, cleaned, tainting, no path to permission answers or settings; send_message asks per recipient. Residual: on Unix a same-user process can claim any sender name; PID reuse can make a dead session look alive until a send fails.

### Deployment

**Prerequisites:**
- Windows 10/11 x64, or linux/amd64.
- For messaging: two or more 0.1.145 sessions under the same user account.

```bash
# fetch
gh release download v0.1.145 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, .deb, .rpm, SHA256SUMS-v0.1.145.txt.
# verify
sha256sum -c SHA256SUMS-v0.1.145.txt --ignore-missing
# Expected: Each present file followed by OK. Windows exe: f5f9cfc7476719903081468e3ccfe21b28530f8342d7da08a2d5f3d866d9080d.
# install
.\gorilla-opencode.exe install
# Expected: Install path and shortcuts.
# verify_active
gorilla-opencode --version
# Expected: v0.1.145
# verify_active
go test ./internal/peers/ ./internal/config/ -run 'Peer|ProjectFolder|FilterFolder' -count=1
# Expected: ok (from a source checkout).
```

**Rollback:**
  1. Remove this version
     `gorilla-opencode uninstall   # or apt remove / dnf remove gorilla-opencode`
  2. Install v0.1.144 (reintroduces the start-folder settings exposure)
     `gh release download v0.1.144 -R gorillanobakaa-dot/Gorilla.Opencode`

### Known Issues

**[low]** stderr line: '<folder>\.gorilla-opencode.json sets ...; ... IGNORED'.
- Cause: The start folder's settings file sets keys outside the allow-list.
- Remedy: Move intended settings to config.json.

**[low]** /peers lists no sessions while another is running.
- Cause: The other is an older version, a -p run, or has tool.peers off (including via the low-bandwidth preset).
- Remedy: Update; check /context.

**[low]** 'MCP server ... did not answer the handshake'.
- Cause: The configured command is not an MCP server or failed to start within 20 s.
- Remedy: Fix the command in config.json.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Folder file merged whole in every earlier version | 📄 stated in input | In every earlier version that file was merged whole into your settings |
| 0.1.144 started a program from a test folder; 0.1.145 did not | 📄 stated in input | Measured on 0.1.144: a test folder made the program start a program |
| Pipe restricted to the current user, remote refused | 📄 stated in input | a named pipe on Windows that only the current user account may open, remote connections refused |
| 11 of 11 messaging checks | 📄 stated in input | 11 of 11 checks passed |
| tool.peers 221 tokens | 📄 stated in input | saving 221 tokens of tool description per turn |
| MCP 20 s bound | 📄 stated in input | It is now given 20 seconds |
| Binary hash and size | 📄 stated in input | sha256 f5f9cfc7476719903081468e3ccfe21b28530f8342d7da08a2d5f3d866d9080d, 55,099,904 bytes |
| Scenarios | 📄 stated in input | All seven gave valid results |
| Launch times | 📄 stated in input | 3.74 seconds on the first start of the new file, then 1.24 and 1.29 seconds |
| Scope | 📄 stated in input | 53 files changed, 6038 insertions, 22 deletions |
| A confirmation prompt could be socially engineered | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*
