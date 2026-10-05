# Gorilla OpenCode 0.1.139 — Windows and Linux

**An alarm this project had wired up itself had been ringing for four releases, and nobody was reading it. This version reads it, fixes what it found, and puts the Linux files on real Linux for the first time.**

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

**Yes, if you use Linux.** This is the first version whose Linux files were
installed and started on real Linux computers before reaching you, and it fixes
a search fault that mostly affected Linux.

**Yes, if you are on Windows and have never installed a program called
ripgrep.** The same search fault applies to you. If you do not know whether you
have it, you probably do not.

**It is a small update otherwise.** On Windows with ripgrep, you gain one thing:
the program now removes a duplicate connection by itself instead of only telling
you about it.

**If you are still on 0.1.137 or older, take this one.** It contains everything
in 0.1.138, which fixed the lists of AIs and closed 23 faults in the part that
guards your computer.

## Why this matters to you

Imagine a smoke alarm in your kitchen, wired in properly, working exactly as it
should. It has been going off for a month. Everyone in the house has learned to
walk past it.

This project has such an alarm. Every time its code is published, a computer at
GitHub that runs Linux builds the program and runs its tests, with nobody having
to remember. For four releases in a row that check came back red:

```
v0.1.135   failure
v0.1.136   failure
v0.1.137   failure
v0.1.138   failure
```

And on each of those four release pages there was this sentence: *the Linux
files were built on Windows and inspected, not installed or started on Linux.*
That read as an honest admission. It was worse than it sounded. The tests **had**
been run on Linux. They had failed, and nobody had looked.

**What the alarm was trying to say.** One of the failures was a real fault in the
program. When the AI needs to find a file, it uses a search tool. That tool has
two ways of working: a fast way that needs a small helper program called
ripgrep, and a plain way for computers that do not have it. The plain way was
broken. Ask it to look inside a named folder, and it answered:

```
No matches found.
```

whether or not anything was there. So you could ask the AI *does this project
have automated checks?*, the AI would search the folder where those are kept,
be told there was nothing, and tell you **no**. Confidently, and wrongly.

Every computer this program was written on has ripgrep. So the broken half only
ever ran on other people's computers. Most Linux computers do not have ripgrep.

**What changed.**

1. The search works both ways now, and a test runs every question through both.
2. The alarm is green: the whole test suite passes on Linux.
3. A new automatic check takes the Linux files from the release page, installs
   them on Ubuntu and on Fedora, starts the program, runs its check-up, and
   removes it again. This is what it printed:

```
deb-ubuntu | Setting up gorilla-opencode (0.1.138) ...
deb-ubuntu | v0.1.138
deb-ubuntu | Nothing wrong found.
rpm-fedora | Complete!
rpm-fedora | v0.1.138
rpm-fedora | Nothing wrong found.
```

Those lines are from the previous version's files, which are built the same
way. The same check runs by itself on this version's files the moment they are
published, and anyone can read the result: on the project's GitHub page, open
the tab called **Actions** and look for **linux-install**.

**What it means for you, in one line:** on Linux, you are no longer the first
person to install this.

**What it does not do:** the automatic check has no key for any AI, and must not
have one. It proves the files install and the program starts. It does not prove
that a conversation with an AI works on Linux.
---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why This Release Exists

The version before this one ended with an honest list of what had not been checked. This version exists because the owner said: close all of them. Closing them turned up something worse than any item on the list.

Every time this program's code is published, a computer at GitHub that runs Linux builds it and runs its tests. It is like a smoke alarm wired into the house: nobody has to remember to check, it goes off by itself. That alarm had been going off for four releases in a row. Nobody was reading it. And on each of those four release pages there was a sentence saying nothing had been run on Linux. It had been run. It had failed.

One of the failures was a real fault. On a computer without a small helper program called ripgrep, the search tool that the AI uses to look for files answered No matches found for a whole class of questions, whether or not the files were there. Ask it does this project have automated checks, and it would look, find nothing, and tell you no. Every computer the program was written on has that helper, so the broken part only ever ran on other people's computers. Most Linux computers do not have it.

That is fixed, and the alarm is now green. The Linux install files were also put on real Linux computers for the first time, installed, started, and removed again. And the items on the original list are closed: a stand-in AI was built so that whole conversations can be tested without a real one, the three commands were typed into the real program and the screen read back, and the duplicate connection the program used to only complain about is now removed by it.

### What You Will Notice

**Searching for files on a computer without ripgrep**
- Before: Asking the AI to look inside a named folder, for example everything under src or the hidden folder where a project keeps its automated checks, came back with No matches found even when the files existed.
- After:  It finds them. The search now behaves the same with or without that helper program.
- Affects: Anyone whose computer does not have ripgrep installed, which is most Linux computers and many Windows ones

**The Linux install files**
- Before: Built on a Windows laptop, looked at, and published. Never installed on Linux by anyone before you.
- After:  An automatic check installs the .deb on Ubuntu and the .rpm on Fedora, starts the program, runs its check-up, and uninstalls it. It passed on both for the previous version's files, and it runs again by itself on this version's files when they are published.
- Affects: Everyone on Linux

**Two saved connections to the same place**
- Before: The program told you about them and left them. Choosing LM Studio in the provider menu could even create the second one.
- After:  The menu no longer creates a second one, and when you update, the check-up removes the spare and keeps the one in use. It prints a line saying which one it removed.
- Affects: Anyone who uses an AI on their own computer through LM Studio or a similar program

**Saying no to one of several actions**
- Before: Fixed in the last version, but checked only by reading the code.
- After:  Checked by a test that plays out the whole conversation: three actions asked for, the first refused, the other two do not happen. With the old fault put back, the test fails.
- Affects: Everyone

**The update, model and provider commands**
- Before: Checked through their parts, never typed into the running program.
- After:  Typed into the real program on real settings and the screen read back. All three did what they should.
- Affects: Everyone

### Deliberately Not Done

- **A conversation with an AI on Linux** — The automatic Linux check has no key and must not have one. It proves the files install and the program starts and checks itself. It does not prove an AI conversation works there.
- **Installing this version's own Linux files before publishing them** — The check works on published files. It was run before publishing on the previous version's files, which are built the same way, and runs by itself on this version's the moment they go out. If it fails, this page will be changed to say so.
- **An Arch package; macOS** — Not built.

### Privacy & Security

No telemetry was added and none exists. This version makes no new kind of request. The automatic Linux check runs on GitHub's own computers using the files anyone can download; it holds no key, contacts no AI, and has nothing of yours.

### How to Install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora or similar Linux computer.
- Enough mobile data for a 55 MB file on Windows or a file of about 23 MB on Linux.
- Gorilla OpenCode closed, if it is open.

**Step 1:** Close Gorilla OpenCode if it is open.
✓ No window titled Gorilla OpenCode is open.

**Step 2:** Open the release page in your web browser and download the one file for your computer. Windows: gorilla-opencode.exe. Debian, Ubuntu or Mint: gorilla-opencode_0.1.139_amd64.deb. Fedora, openSUSE or Rocky: gorilla-opencode-0.1.139-1.x86_64.rpm. Also download SHA256SUMS-v0.1.139.txt.
✓ Two files are in your Downloads folder.

**Step 3:** Check the file is the one that was published. On Windows, press the Windows key, type PowerShell, press Enter. A window with a blinking cursor opens. Type cd Downloads and press Enter. Then type the command below and press Enter.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ It prints 64d1f1f66e16cc5112f4f18354332abbfa65ac810020c1c0e6647b4aa075e161. On Linux, run sha256sum -c SHA256SUMS-v0.1.139.txt --ignore-missing in a terminal in your Downloads folder; it prints the file name followed by OK.

**Step 4:** Windows: install it. Type the command below and press Enter.
```
.\gorilla-opencode.exe install
```
✓ It reports where it copied itself and the shortcuts it made.

**Step 5:** Debian, Ubuntu or Mint instead: type the command below and press Enter. It asks for your password because installing a program changes the system.
```
sudo apt install ./gorilla-opencode_0.1.139_amd64.deb
```
✓ A line says Setting up gorilla-opencode (0.1.139), with no line beginning with E:.

**Step 6:** Fedora, openSUSE or Rocky instead: type the command below and press Enter.
```
sudo dnf install ./gorilla-opencode-0.1.139-1.x86_64.rpm
```
✓ The last line says Complete!

**Step 7:** Confirm the version. Type the command below and press Enter.
```
gorilla-opencode --version
```
✓ It prints v0.1.139.

**Step 8:** Start the program, and once it is open type /update and press Enter. This fetches today's lists and lets the check-up tidy your settings.
```
/update
```
✓ A line appears naming each provider and how many AIs it offers. If it repaired anything, a part of that line begins with fixed:.

**To go back:** Windows: type gorilla-opencode uninstall and press Enter, then install the gorilla-opencode.exe from version 0.1.138 the same way as in step 4. Debian: sudo apt remove gorilla-opencode. Fedora: sudo dnf remove gorilla-opencode. Your conversations, settings and keys are stored separately and none of these steps touches them.

### If Something Goes Wrong

**After /update you see: fixed: removed the connection, followed by a name.**
Two of your saved connections pointed at the same program on your computer. One was a spare. The check-up removed the spare and kept the one in use.
What to do: Nothing to do. Your AIs are still listed, once each instead of twice.
Status: expected behaviour, new in 0.1.139

**On an older version, the AI said a folder was empty or a project had no automated checks, and that was wrong.**
On a computer without ripgrep, the search tool could not match a pattern that named a folder.
What to do: Install this version and ask again.
Status: fixed in 0.1.139

**On Linux the package will not install, or the program will not start.**
The automatic check installs these files on Ubuntu and Fedora. Another kind of Linux may differ.
What to do: Download the plain file gorilla-opencode-v0.1.139-linux-amd64 from the same page, make it runnable with chmod +x, and start it directly. Then report what the package printed and which Linux you use.
Status: checked on Ubuntu and Fedora; other kinds not checked

### Common Questions

**Q: Should I worry that tests were failing for four releases?**
A: On Windows, no: the same tests passed there every time. On Linux, one real fault was shipping: file searches by folder failed on computers without ripgrep. That is the fault this version fixes. The other three failures were mistakes in the tests themselves.

**Q: How do I know the Linux check is real?**
A: It is public. On the project's page at GitHub, open the tab called Actions and look for linux-install. Each run shows every step and what it printed.

**Q: What is a stand-in AI, and does it affect me?**
A: It is a test tool, not part of what you use. It plays the part of the AI from a script so that the program's own safety rules can be tested the same way every time, without a real AI's moods.

**Q: I installed 0.1.138 this morning. Do I need this?**
A: On Linux, yes. On Windows without ripgrep, yes. On Windows with it, you gain the automatic tidy-up of duplicate connections and nothing else you would notice.

### Bottom Line

If you are on Linux, take this version: it is the first whose Linux files were installed and started on real Linux before you, and it fixes a search fault that mostly affected Linux. If you are on Windows, it is a small update unless your computer lacks ripgrep. The larger point is about trust. For four releases this project told you nothing had been run on Linux while an alarm it had wired up itself was ringing. That alarm is now read, it is green, and the release page shows what it printed. What is still not checked is a real conversation with an AI on Linux, and this version's own Linux files are installed by the automatic check straight after publishing, not before.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| The Linux test run failed on four releases | 📄 stated in input | That run failed on every release from v0.1.135 to v0.1.138 |
| The search fault on computers without ripgrep | 📄 stated in input | the find tool answered "No matches found" for any pattern with a folder in it |
| The other three failures were faults in tests | 📄 stated in input | The other three were faults in tests |
| The Linux run now passes | 📄 stated in input | After the fixes the Linux run passes |
| The install check passed on Ubuntu and Fedora for the 0.1.138 files | 📄 stated in input | Run on the published v0.1.138 files, every step passed on both |
| The install check does not prove an AI conversation | 📄 stated in input | It does not prove a conversation with an AI works on Linux |
| Whole-turn tests fail when the old faults are put back | 📄 stated in input | the remaining tools RAN: second=1 third=1 |
| The three commands were typed into the real program | 📄 stated in input | were typed into the real program in a real console window on the owner's real settings |
| The check-up removed the duplicate connection | 📄 stated in input | fixed: removed the connection LM Studio |
| Seven real-AI scenarios on the published Windows file | 📄 stated in input | All seven gave valid results |
| This version's Linux files are installed after publishing | 📄 stated in input | installed by the automatic check after publishing, not before |
| Most Linux computers do not have ripgrep | 📄 stated in input | Most Linux computers do not have it |


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

Every block below is real output, copied from a run on the maintainer's computer or from GitHub's. Nothing here is an illustration.

## The update command, typed into the real program

`/update` was typed into the program in a real window, on real settings, and this is the line it wrote. The last part, beginning *fixed:*, is the check-up removing a duplicate connection by itself.

```
OpenRouter 325 usable (+0, -6) | Antigravity 26 usable | ChatGPT 4 usable | 4 configured
endpoint(s) re-asked | Gemini 18 usable | NVIDIA NIM: asked 10 model(s), 2 answered with a tool call (nvidia/nemotron-3-
super-120b-a12b, openai/gpt-oss-20b); not usable: google/gemma-4-31b-it=no-answer-in-time, ibm/granite-34b-code-
instruct=gone, meta/codellama-70b=gone, meta/llama2-70b=gone, nvidia/llama-3.1-nemotron-51b-instruct=gone, nvidia/llama-
3.1-nemotron-70b-instruct=gone, nvidia/llama3-chatqa-1.5-70b=gone, nvidia/nemotron-3-ultra-550b-a55b=provider-error |
fixed: removed the connection LM Studio: it was a second entry for the same server as lmstudio
(http://localhost:1234/v1), which is kept.
```

*gone* means the company still lists that AI but has switched it off. Seven of the ten asked were in that state.

## The list of AIs, after it

`/model`, typed the same way. Numbered from 1, and the four that share a name can be told apart:

```
22. Gemini 3.5 Flash Lite [gemini-2.5-flash] (Antigravity free)
23. Gemini 3.5 Flash Lite [gemini-2.5-flash-lite] (Antigravity free)
24. Gemini 3.5 Flash Lite [gemini-2.5-flash-thinking] (Antigravity free)
25. Gemini 3.5 Flash Lite [gemini-3.5-flash-lite] (Antigravity free)
26. Gemini 3.1 Flash Lite (Antigravity free)
```

## The tests that play out a whole conversation

A stand-in AI was written for testing. It plays the AI's part from a script, so the program's own rules can be checked the same way every time. To prove these tests can fail, each old fault was put back for a moment.

With the fault that let actions through after you said no:

```
--- FAIL: TestRefusingOneActionStopsTheOthersSentWithIt
    after a refusal the remaining tools RAN: second=1 third=1. The person said no.
```

With the fault that let a helper AI switch off the caution applied after reading a web page:

```
--- FAIL: TestOnlyAMessageFromThePersonClearsTheUntrustedContentMark
    a helper agent starting cleared the untrusted-content mark; the next action would be auto-approved
```

With the faults taken out again, both pass.

## How it was tested

**On Linux, by GitHub's computers.** The complete test suite: passed. The install check on Ubuntu and Fedora: every step passed, shown at the top of this page.

**On Windows.** The complete test suite passes. The search tool's tests were also run with ripgrep switched off, which is how the fault was reproduced on a computer that has it.

**The file you download is the file that was tested.** The Windows program was built once, tested, and published without being rebuilt. It was started from a script seven times against a real AI from NVIDIA (`nemotron-3-super-120b-a12b`). All seven runs gave a usable result: a repeating action was warned at the third try and stopped at the fifth, a secret was shown to the AI only as a placeholder, a command that would have thrown away unsaved work was refused, and a review that could not run was reported as not having run.

**A tool from outside the program, called through it.** The AI was asked to use a tool supplied by Fieldkit, a separate set of tools on the maintainer's computer:

```
1 tool call:
  fieldkit_readiness {}  -> ok
```

**What was not tested.** A conversation with an AI on Linux. This version's own Linux files before publishing: the automatic check installs them just after.

## No new pictures, and why

Everything in this release is text the program prints, quoted above as printed.
No new screenshot was taken. The two below are from earlier releases and show
screens this release did not change, pinned to this version.

**The normal window**, where every action is listed as it happens. This is the
permission question, the one the first failing test above is about.

[![Gorilla OpenCode showing a Permission Required dialog for the patch port tool, naming the folder it will modify and the patch series it will apply, with the three choices Allow, Allow for session and Deny, proving the program asks before a tool changes files](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.139/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.139/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)

**The command list**, all 31 commands on one screen.

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does and showing 37 of 37 lines, 31 commands, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.139/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.139/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Root cause of the headline fault: the `ci` workflow (ubuntu-latest, `go test ./... -count=1`) had failed on every push to main since v0.1.135 and was not read. Three of its four failures predate v0.1.138. The one product defect: in `pfind.py` with ripgrep absent, `_apply_user_globs` matched a slash-containing glob with `fnmatch` against the whole joined path, so `.github/**` and `src/**` matched nothing, and four of five `py_fallback_files` call sites did not pass `include_hidden`, so `--hidden` was ignored. Fix: match the normalised path against the pattern and against `*/pattern`; pass `include_hidden` everywhere; `PFIND_NO_RG=1` forces the fallback so it can be tested where ripgrep exists. Also: `sparse` now validates the file and the session before `exec.LookPath`; the portal and `applyLocalEndpoint` compare `CanonicalEndpointURL`; `doctorEndpoints(fix)` removes duplicate endpoints; a scripted provider makes whole-turn agent tests possible. Scope: 8 files modified, 2 added, 6 new tests, one new workflow.

### Known Alternatives Considered

Skipping the two sparse tests when sparse is absent was not chosen; the tool's check order was changed instead, because the tests assert properties (a missing session is a hard error, a missing file is named) that should hold whether or not the binary is installed. Installing ripgrep on the CI runner to make the find test pass was not chosen: it would have hidden the defect that users without ripgrep have. Building release-candidate packages for the install check was attempted and abandoned: `build_linux_packages.py` refuses a package without its own release notes ("the package would ship without its own release notes"), so the pre-publish check ran on the v0.1.138 packages. Other alternatives: Not available in the source material.

### Architecture Impact

None in the product. Test architecture: `internal/llm/agent/loop_harness_test.go` adds `scriptedProvider` (implements `provider.Provider`, replies from a script), `memMessages`, `memSessions` and `scriptedTool`; `newLoopAgent` builds a real `*agent` around them so `Run`, `processGeneration`, `streamAndHandleEvents` and the tool loop execute unmodified. CI: `linux-install.yml` runs on `release: published` and on dispatch with `tag` and optional `expect` inputs.

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.139" -o gorilla-opencode.exe .`. Linux: same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Windows binary sha256 64d1f1f66e16cc5112f4f18354332abbfa65ac810020c1c0e6647b4aa075e161, built from the committed tree, tested, published unchanged. .deb and .rpm by nfpm.
```

### Resource Deltas

gorilla-opencode.exe: 54,539,776 bytes (0.1.138) -> 54,543,872 bytes (+4,096). No runtime request added. RSS and cold start: not measured.

### Code Changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `internal/llm/tools/pfind.py` | modified | `HAVE_RG = shutil.which("rg") is not None`. Fallback glob match: `fnmatch.fnmatch(path, pattern)` on the joined path for patterns containing `/`. `py_fallback_files` called without `include_hidden` at four sites. | `PFIND_NO_RG=1` forces the fallback. Slash globs match `path.replace(os.sep, '/')` against `pattern` and `*/pattern`. All call sites pass `include_hidden=args.hidden or almost_all`. |
| `internal/llm/tools/sparse.go` | modified | Order: parse, empty path, Windows refusal, `LookPath("sparse")`, stat file, session check. | Order: parse, empty path, Windows refusal, stat file, session check, `LookPath`. A missing file and a missing session are reported whether or not sparse is installed. |
| `cmd/provider_portal.go` | modified | `endpointFor` and `applyLocalEndpoint` matched `e.BaseURL != baseURL` textually; 127.0.0.1 and localhost were distinct, so selecting the LM Studio row appended a second endpoint. | Both compare `models.CanonicalEndpointURL`; an existing entry is adopted with its own name and its own spelling of the address. |
| `internal/config/doctor.go` | modified | `doctorEndpoints()` reported duplicates. | `doctorEndpoints(fix bool)`; with fix (a non-nil AgentSetter) it keeps the endpoint that owns registered routes (`models.EndpointHasModels`), else the keyed one, else the first, and calls `RemoveLocalEndpoint` on the rest. |
| `internal/llm/agent/loop_harness_test.go` | added | N/A - new file. No test could run a turn. | Five loop-level tests: ordinary turn; denial stops remaining calls in the message (finding 3); only a coder turn clears taint (finding 4); a stream closed without completion is `FinishReasonError` (finding 14); an orphaned tool call is answered and the session continues (finding 19). |
| `.github/workflows/linux-install.yml` | added | N/A - new file. Linux artefacts were never installed. | Jobs `deb-ubuntu` and `rpm-fedora` (fedora:latest container): `gh release download`, `sha256sum -c`, install, `--version` equals the expected tag, `models doctor` exits 0 or 3 on an empty HOME, the pfind wrapper has an LF shebang, plain binary reports its version, uninstall leaves no command. |

### Subsystem Changes

**OTHER:** find tool: fallback file lister corrected; behaviour with ripgrep unchanged.

**STORAGE:** `config.json` loses duplicate local endpoints when the doctor runs with repairs (on /update, /model, the portal).

**TUI:** No change.

**NETWORK:** No change.

**AUTH:** No change.

### Test Coverage

- **Added:** loop_harness_test.go (5); TestFolderPatternsWorkWithAndWithoutRipgrep in find_test.go, which runs each question through both listers; doctor_test.go extended to cover the endpoint repair, report-only behaviour and idempotence.
- **Removed:** None. TestAShellCommandNamesThePathItReachesOutsideTheProject now adds its PowerShell case on Windows only.
- **Notes:** Mutation check: with `goto out` reverted to `break`, TestRefusingOneActionStopsTheOthersSentWithIt fails ("the remaining tools RAN: second=1 third=1"); with the coder guard removed, TestOnlyAMessageFromThePersonClearsTheUntrustedContentMark fails. `PFIND_NO_RG=1 go test ./internal/llm/tools` passes. CI run on ubuntu-latest: success (build, vet, full suite). linux-install on the published v0.1.138 artefacts: every step succeeded on Ubuntu and Fedora. Windows binary against nvidia/nemotron-3-super-120b-a12b: 7 of 7 scenarios valid. /update, /model, /provider driven in a real console by WriteConsoleInput on the real configuration; screens recorded. Portal paste re-proven on this binary. Not covered: an LLM turn on Linux; the v0.1.139 packages are installed by the release-triggered run, after publication.

### Security Posture

No new attack surface in the product. The find fix does not widen what find may read: `RefuseSensitiveRead` and `refuseSensitiveGlob` run before listing in both engines. `linux-install.yml` has `contents: read` and uses the job token only to download release assets.

### Deployment

**Prerequisites:**
- Windows 10/11 x64, or linux/amd64 (static binary, CGO disabled).
- No running gorilla-opencode process.

```bash
# fetch
gh release download v0.1.139 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: gorilla-opencode.exe, the linux-amd64 binary, .deb, .rpm and SHA256SUMS-v0.1.139.txt.
# verify
sha256sum -c SHA256SUMS-v0.1.139.txt --ignore-missing
# Expected: Each present file followed by OK. Windows exe: 64d1f1f66e16cc5112f4f18354332abbfa65ac810020c1c0e6647b4aa075e161.
# install
.\gorilla-opencode.exe install
# Expected: Install path and shortcuts. Linux: `sudo apt install ./gorilla-opencode_0.1.139_amd64.deb` or `sudo dnf install ./gorilla-opencode-0.1.139-1.x86_64.rpm`.
# verify_active
gorilla-opencode --version
# Expected: v0.1.139
# verify_active
PFIND_NO_RG=1 go test ./internal/llm/tools -run FolderPatterns
# Expected: ok (from a source checkout: the fallback lister on a machine that has ripgrep).
# verify_active
gh workflow run linux-install -f tag=v0.1.139
# Expected: Both jobs succeed.
```

**Rollback:**
  1. Remove this version
     `gorilla-opencode uninstall   # or: sudo apt remove gorilla-opencode / sudo dnf remove gorilla-opencode`
  2. Install v0.1.138 from its release
     `gh release download v0.1.138 -R gorillanobakaa-dot/Gorilla.Opencode`

### Known Issues

**[low]** find returns different results with and without ripgrep for a slash glob.
- Cause: The fallback matches unanchored (`*/pattern`); ripgrep anchors a glob containing a slash to the search root unless it begins with `**/`.
- Remedy: Use a pattern that is explicit about depth, or install ripgrep. Report the case.

**[low]** A local endpoint disappeared from config.json after /update.
- Cause: It shared a canonical URL with another endpoint and the doctor removed the spare.
- Remedy: Expected. The kept endpoint is named in the `fixed:` line of the /update summary.

**[high]** linux-install fails on a release.
- Cause: A packaging regression, or a version stamp that does not match the tag.
- Remedy: Read the failing step in the Actions run; correct the release page to say so until fixed.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| CI failed from v0.1.135 to v0.1.138 | 📄 stated in input | That run failed on every release from v0.1.135 to v0.1.138 |
| Fallback glob defect | 📄 stated in input | the find tool answered "No matches found" for any pattern with a folder in it |
| Linux run passes after the fixes | 📄 stated in input | After the fixes the Linux run passes: build, checks and the complete test suite on Ubuntu |
| Install check passed on the v0.1.138 artefacts | 📄 stated in input | Run on the published v0.1.138 files, every step passed on both |
| Mutation results | 📄 stated in input | a helper agent starting cleared the untrusted-content mark |
| Commands driven in a real console | 📄 stated in input | by writing key presses into the window's input and reading the screen back |
| Binary hash and size | 📄 stated in input | sha256 64d1f1f66e16cc5112f4f18354332abbfa65ac810020c1c0e6647b4aa075e161, 54,543,872 bytes |
| Real-model run | 📄 stated in input | All seven gave valid results |
| v0.1.139 packages are installed after publication | 📄 stated in input | installed by the automatic check after publishing, not before |
| Fieldkit verified through the program | 📄 stated in input | the receipt recorded 1 tool call, ok |
| The fallback is unanchored where ripgrep anchors | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*
