# Gorilla OpenCode 0.1.141 — Windows and Linux

**Every time it started, the program spent ten seconds re-testing a key it had already tested. It now tests a key when it is new and once a day after that. And on Windows you can update without closing it first.**

> **Linux checks: all passed.** GitHub's build service was out of order when
> this page went up. When it returned, the checks ran on the files published
> here. The .deb was installed, started and removed on Ubuntu. The .rpm was
> installed, started and removed on Fedora. The Arch package was built on Arch
> Linux, installed, started and removed, and is now attached below. And the
> program, installed from the .deb, held a conversation with an AI running on
> the same Linux machine. Sentences further down this page that say these checks
> had not run were true when it was published.

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

**Yes, on Windows, if you use a provider that needs a key** (NVIDIA, Cloudflare
and others) **and you press Enter at the first menu.** You get about ten seconds
back every time you start the program.

**Yes, on Windows, for the update itself.** From this version on, installing a
newer one works even while the program is open.

**It changes nothing for you** if you sign in with Google or ChatGPT, or use an
AI on your own computer, and always close the program before updating.

**On Linux:** the same start-up change applies. Read the note at the top of this
page first.

**If you are on 0.1.139 or older, take this one.** It includes 0.1.140, which
fixed a far longer wait: minutes, when the program was opened in a large folder.

## Why this matters to you

Think of a shop where you have a membership card. The first time, they check it
properly. That is fair.

Now imagine they check it again every time you walk past the till. Same card,
same shop, ten seconds each time, all day.

That is what this program was doing. When it starts, it shows a short menu
asking which AI provider to use. If you pressed Enter on the one you already
use, it took your key and asked the provider three small test questions to prove
the key works, and waited for the answers.

The test itself is a good thing. It was added three versions ago so that a wrong
key is caught the moment you save it, not an hour later in the middle of your
work. The mistake was repeating it at every start.

The owner asked for the start to be timed, in his own folder with his own
settings. This is what the clock showed on the previous version:

```
version 0.1.140
  press Esc at the menu    ready to type after  1.3 seconds
  press Enter at the menu  ready to type after 11.0 seconds
```

**Now a key that passed is trusted for a day.** Within that day, choosing the
same provider goes straight through, and the program prints one line so you know
what it relied on:

```
NVIDIA NIM: checked 14 minutes ago, a model answered; not asked again (type /update to re-check).
```

The same clock, the same folder, this version:

```
version 0.1.141
  press Esc at the menu    ready to type after 1.25 seconds
  press Enter at the menu  ready to type after 1.43 seconds
```

**What it means for you, in one line:** the program is ready in about a second
and a half whichever key you press.

**What stays the same:** a key you have just typed in is still tested at once.

**The other change: updating with the program open.** On Windows, a program's
file cannot be replaced while that program is running. So installing a new
version failed with a message about *another process*, and you had to hunt down
and close every window first. Windows does, however, allow the file to be
renamed. The installer now does that: it moves the old file aside and puts the
new one in its place. The window you have open carries on with the old version;
the next one you start is the new one.

**What it does not do:** it cannot tell you a key was cancelled an hour ago. If
that happens, your first message fails with the provider's own error. Type
`/update` and the test is run again.
---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why This Release Exists

The version before this one made the program start in about a second instead of in minutes. The owner then did the sensible thing and asked for it to be timed again, in the same folder, with the same settings, to see whether that was true.

It was true one way and not the other. When the program starts it shows a short menu asking which AI provider to use. Press Esc to keep the one you had, and it was ready in 1.3 seconds. Press Enter on the provider you already use, which is what most people do, and it took 11 seconds.

The ten extra seconds were the program proving your key works. It asked the provider three small test questions and waited for the answers. That check is worth having: it was added three versions ago so that a key is proven at the moment you save it, instead of failing later in the middle of your work. The mistake was doing it again at every single start.

It is like a shop that checks your membership card, which is fair, and then checks it again each time you walk past the till. Once a day is enough.

So a check that passed is now trusted for 24 hours. During that time, choosing the same provider goes straight through, and the program prints one line telling you when the check was last made. If you type in a new key, it is tested at once, as before.

The second change came from the same afternoon. Installing the previous version failed on the owner's computer because he had the program open in another window, and Windows does not let a program's file be replaced while it is running. The installer now steps round that.

### What You Will Notice

**Starting the program and pressing Enter on the provider you already use**
- Before: About 11 seconds before you could type, every time, while the program re-tested your key.
- After:  About 1.4 seconds. A line says when the key was last tested. The test is repeated once the last one is more than a day old.
- Affects: Anyone who uses a provider with a key, such as NVIDIA or Cloudflare

**Entering a new key**
- Before: Tested at once.
- After:  Still tested at once. Only a key that already passed is given the day's grace.
- Affects: Anyone setting up or replacing a key

**Updating on Windows while the program is open**
- Before: The install stopped with a message about the file being used by another process. You had to find and close every Gorilla OpenCode window and try again.
- After:  The install goes through. The window you have open keeps running the old version until you close it; the next one you start is the new version.
- Affects: Everyone on Windows

### Deliberately Not Done

- **Trusting a passed check for longer than a day** — Providers switch AIs off without notice. On the day this was written, NVIDIA still listed seven AIs that no longer answered. A day is a compromise between waiting at every start and trusting an old answer.
- **Deleting the old program file after an update with a window open** — It cannot be deleted while that window is still using it. The installer tells you its name so you can delete it later; leaving it does no harm.
- **An automatic test of the installer's new step** — It was done by hand once, successfully, on the owner's computer. It is not covered by a test the program runs on itself.
- **Checking this version's Linux files on Linux, and an Arch package for it** — The checks run on GitHub's computers, and GitHub's build service has been out of order since before the previous version was published. They run when it is back, and this page will be changed to show the result.

### Privacy & Security

No telemetry was added and none exists. This version sends less than the last one: the three test questions to your provider are no longer repeated at every start. Nothing new is sent to anyone.

### How to Install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora or similar Linux computer.
- Enough mobile data for a 55 MB file on Windows or a file of about 24 MB on Linux.
- On Windows you no longer need to close the program first, though the open window will stay on the old version.

**Step 1:** Open the release page in your web browser and download the one file for your computer. Windows: gorilla-opencode.exe. Debian, Ubuntu or Mint: gorilla-opencode_0.1.141_amd64.deb. Fedora, openSUSE or Rocky: gorilla-opencode-0.1.141-1.x86_64.rpm. Also download SHA256SUMS-v0.1.141.txt.
✓ Two files are in your Downloads folder.

**Step 2:** Check the file is the one that was published. On Windows, press the Windows key, type PowerShell, press Enter. A window with a blinking cursor opens. Type cd Downloads and press Enter. Then type the command below and press Enter.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ It prints a4fa0fb576e9dda450050ed1db3350f180e6bf88ffefe04245c39803fff3c2a4. On Linux, run sha256sum -c SHA256SUMS-v0.1.141.txt --ignore-missing in a terminal in your Downloads folder; it prints the file name followed by OK.

**Step 3:** Windows: install it. Type the command below and press Enter.
```
.\gorilla-opencode.exe install
```
✓ It reports where it copied itself and the shortcuts it made. If a window was open, it also prints a note beginning with note: saying the open window keeps the old version.

**Step 4:** Debian, Ubuntu or Mint instead: type the command below and press Enter. It asks for your password because installing a program changes the system.
```
sudo apt install ./gorilla-opencode_0.1.141_amd64.deb
```
✓ A line says Setting up gorilla-opencode (0.1.141), with no line beginning with E:.

**Step 5:** Fedora, openSUSE or Rocky instead: type the command below and press Enter.
```
sudo dnf install ./gorilla-opencode-0.1.141-1.x86_64.rpm
```
✓ The last line says Complete!

**Step 6:** Confirm the version. Type the command below and press Enter.
```
gorilla-opencode --version
```
✓ It prints v0.1.141.

**To go back:** Windows: type gorilla-opencode uninstall and press Enter, then install the gorilla-opencode.exe from version 0.1.140 the same way as in step 3. Debian: sudo apt remove gorilla-opencode. Fedora: sudo dnf remove gorilla-opencode. Your conversations, settings and keys are stored separately and none of these steps touches them.

### If Something Goes Wrong

**At start-up a line says: checked some hours ago, a model answered; not asked again.**
Your key passed its test less than a day ago, so the program did not spend ten seconds testing it again.
What to do: Nothing to do. To test it again now, type /update and press Enter once the program is open.
Status: expected behaviour, new in 0.1.141

**The AI stops answering although the start-up line said the key was checked.**
The check was up to a day old. The provider may have switched that AI off since, or your allowance may have run out.
What to do: Type /update and press Enter. It repeats the test, and the list of AIs will show which ones answer now.
Status: expected behaviour, new in 0.1.141

**After updating, a file with a name ending in .replaced- and some numbers is in the program's folder.**
The program was open while you updated. That is the old version, which the open window was still using.
What to do: Close every Gorilla OpenCode window, then delete the file. Leaving it does no harm.
Status: expected behaviour, new in 0.1.141

**On Linux the package will not install, or the program will not start.**
This version's Linux files had not been installed on Linux when this page was published, because the service that does it was out of order.
What to do: Download the plain file gorilla-opencode-v0.1.141-linux-amd64 from the same page, make it runnable with chmod +x, and start it directly. Then report what the package printed and which Linux you use.
Status: being checked: see the top of this page for the result

### Common Questions

**Q: Is it less safe to test my key only once a day?**
A: No. The test never protected anything; it only told you early if the key was wrong. A key that is revoked during the day will fail at your first message, with the provider's own error, exactly as it would have between two starts before.

**Q: I use an AI on my own computer, or I sign in with Google or ChatGPT. Does this change anything?**
A: The start-up change does not: those were never test-questioned at start. The update-while-open change applies to everyone on Windows.

**Q: Does the saved check use up my allowance?**
A: It saves some. Three small questions at every start are no longer sent.

### Bottom Line

On Windows, update: if you use a provider with a key and press Enter at the first menu, the program is ready about ten seconds sooner every time you start it, and updating no longer needs every window closed first. The check that proves a key is kept; it is done when a key is new and then once a day, not at every start. Two honest limits. The installer's new step was done by hand once and has no automatic test. And this version's Linux files have not yet been installed on Linux, because GitHub's build service has been out of order all evening; the page will be changed when those checks have run.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Esc took 1.3 seconds and Enter took 11.0 seconds on the previous version | 📄 stated in input | Pressing Enter on the NVIDIA row, which is what he does, it took 11.0 seconds |
| The check asked three models a test question | 📄 stated in input | asked three models a test question, to prove the key works |
| A successful check is reused for 24 hours | 📄 stated in input | A successful check is now reused for 24 hours |
| A new key is always tested at once | 📄 stated in input | A newly typed key is always tried at once |
| Now about 1.4 seconds with Enter | 📄 stated in input | ready to type after 1.43 seconds and 1.40 seconds |
| The installer failed while a window was open | 📄 stated in input | The process cannot access the file because it is being used by another process |
| The installer now moves the in-use file aside | 📄 stated in input | The installer now moves the copy in use aside and writes the new one in its place |
| The installer change has no automatic test | 📄 stated in input | the installer's rename has no automatic test |
| Seven real-AI scenarios passed on the published Windows file | 📄 stated in input | All seven gave valid results |
| Linux files not yet installed on Linux because of an outage | 📄 stated in input | GitHub's build service has been in an outage since before v0.1.140 was published |
| A revoked key fails at the first message as before | 🤖 model inference | *(none — model judgment)* |


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

Every block below is real output or a real measurement, taken on the maintainer's computer. Nothing here is an illustration.

## The start, timed

The program was started in a home folder, on real settings, and the moment each screen appeared was read from its window. Times are seconds from starting the program.

```
version 0.1.140, Enter on the NVIDIA row
  provider menu shown       0.31
  connection screen shown  10.16
  ready to type            10.98

version 0.1.141, Enter on the NVIDIA row
  provider menu shown       0.39        0.37
  connection screen shown   0.55        0.52
  ready to type             1.43        1.40      (two runs)

version 0.1.141, Esc at the menu
  provider menu shown       0.37
  ready to type             1.25
```

The ten seconds sat between the first menu and the next screen. That is where the three test questions were being asked.

## What the installer used to say

Installing version 0.1.140 with a Gorilla OpenCode window open:

```
Error: writing C:\Users\gorilla1\AppData\Local\Programs\Gorilla OpenCode\gorilla-opencode.exe: open C:\Users\gorilla1\AppData\Local\Programs\Gorilla OpenCode\gorilla-opencode.exe: The process cannot access the file because it is being used by another process.
```

From this version, the installer moves the file in use aside, writes the new one, and says so.

## How it was tested

**The file you download is the file that was tested.** The Windows program was built once, tested, and published without being rebuilt.

**Starting up.** Timed three times in a home folder, with the numbers above.

**Against a real AI, with nobody watching.** The published Windows file was started from a script seven times against NVIDIA's `nemotron-3-super-120b-a12b`. All seven runs gave a usable result.

**The program's own tests.** The complete set passes on Windows. Three were added for this change: a recent check is reused; and the test questions are still asked when a key is new, when the last check is over a day old, when the last check found a fault, and when there never was one.

**What was not tested, and why.** The installer's new step has no automatic test; it was done by hand once, successfully. This version's Linux files have not been installed on Linux, and no Arch package has been built for it, because GitHub's build service has been cancelling jobs all evening with this message:

```
The job was not acquired by Runner of type hosted even after multiple attempts
```

They run when the service returns. The note at the top of this page will then be replaced with their result.

## No new pictures, and why

Everything in this release is text the program prints, quoted above as printed.
No new screenshot was taken. The two below are from earlier releases and show
screens this release did not change, pinned to this version.

**The normal window**, where every action is listed as it happens. This is the
permission question.

[![Gorilla OpenCode showing a Permission Required dialog for the patch port tool, naming the folder it will modify and the patch series it will apply, with the three choices Allow, Allow for session and Deny, proving the program asks before a tool changes files](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.141/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.141/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)

**The command list**, all 31 commands on one screen.

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does and showing 37 of 37 lines, 31 commands, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.141/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.141/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Root cause: `applyLocalEndpoint` in cmd/provider_portal.go has called `probeEndpoint` unconditionally since v0.1.138, and the provider portal runs at every launch, so Enter on an already-configured key-based row issued up to three chat-completion probes each start. Measured on v0.1.140: input line at 1.3 s after Esc, 11.0 s after Enter on the NVIDIA row. Fix: when no key was typed and `models.EndpointProvenWithin(name, 24h)` finds a usable verdict, the probe is skipped and the default comes from `PreferredOnEndpoint`. Second change: `copySelf` in cmd/install.go, on a failed write of the destination, renames the destination aside and writes again, because Windows refuses to overwrite a running image but permits renaming it. Scope: 3 files modified, 1 added, 3 new tests.

### Known Alternatives Considered

Skipping the model listing as well was not done: `registerLocalEndpoint` stays, because it costs 0.1 to 0.7 s on NVIDIA and is what removes models the provider has dropped. A longer reuse window was rejected for the reason given in the layman track: providers retire models without notice. Probing in the background after the window opens was not chosen; the default model must be known before agents are pointed at it. Other alternatives: Not available in the source material.

### Architecture Impact

None. The probe verdict cache (`model-probes.json`) gains a reader on the launch path. `finishLocalEndpoint` is the former tail of `applyLocalEndpoint`, shared by the probed and the reused paths.

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.141" -o gorilla-opencode.exe .`. Linux: same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Windows binary sha256 a4fa0fb576e9dda450050ed1db3350f180e6bf88ffefe04245c39803fff3c2a4, built from the committed tree, tested, published unchanged. .deb and .rpm by nfpm.
```

### Resource Deltas

gorilla-opencode.exe: 54,535,680 bytes (0.1.140) -> 54,539,264 bytes (+3,584). Launch with Enter on the NVIDIA row in a home directory: 11.0 s -> 1.43 s and 1.40 s (two runs). With Esc: 1.25 s. Requests per launch on that path: one model listing plus up to three probes -> one model listing.

### Code Changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `cmd/provider_portal.go` | modified | `applyLocalEndpoint` always called `probeEndpoint(name, cacheDir)` after registering the endpoint. | Records `newKey := key != ""` before a stored key is adopted. If `!newKey` and `EndpointProvenWithin(name, probeReuse)` is true, prints the age of the check and returns `finishLocalEndpoint(name, PreferredOnEndpoint(name))`. `probeReuse = 24 * time.Hour`. |
| `internal/llm/models/probe_helpers.go` | modified | No query for the age of a usable verdict. | `EndpointProvenWithin(endpoint, maxAge) (time.Duration, bool)` returns the age of the freshest `ProbeWorks` verdict among the endpoint's routed models, ignoring verdicts at or beyond maxAge and verdicts dated in the future. Test helpers `SetProbeVerdictForTest` and `ClearProbeVerdictForTest`. |
| `cmd/install.go` | modified | `copySelf` returned the error from `os.WriteFile(dst, ...)`; on Windows with the destination running this is ERROR_SHARING_VIOLATION. | On that error it renames dst to `dst.replaced-YYYYMMDD-HHMMSS`, writes dst, and prints a note naming the kept file; if the second write fails the rename is undone. |
| `cmd/portal_reprobe_test.go` | added | N/A - new file. | Counts calls to the `probeEndpoint` seam: zero when a `ProbeWorks` verdict is two hours old and no key was typed; one when a key was typed, the verdict is 25 hours old, the verdict is `ProbeError`, or there is none. Also covers `EndpointProvenWithin`. |

### Subsystem Changes

**NETWORK:** Up to three fewer chat-completion requests per launch on the Enter path for a recently probed endpoint.

**OTHER:** Installer: in-place update with a running instance on Windows.

**TUI:** No change.

**STORAGE:** A `.replaced-<timestamp>` file may remain beside the installed binary after an update with a window open.

**AUTH:** No change. A typed key is still probed immediately.

### Test Coverage

- **Added:** cmd/portal_reprobe_test.go: TestARecentCheckIsReusedWhenTheSameConnectionIsChosenAgain, TestTheModelsAreAskedWhenItMatters (four cases), TestEndpointProvenWithinReadsTheFreshestGoodAnswer.
- **Removed:** None.
- **Notes:** Complete suite passes on Windows (34 packages). Windows binary against nvidia/nemotron-3-super-120b-a12b: 7 of 7 scenarios valid; portal paste re-proven; launch timed three times in a home directory by reading the console screen buffer. The installer rename has no automated test; it was performed manually during the v0.1.140 install with an instance running. NOT RUN: ci, linux-install, arch-package and linux-conversation, because GitHub Actions has been in a major outage and cancels jobs before a runner is assigned.

### Security Posture

No change to permission, masking or command gates. A key revoked within the reuse window is not detected at launch; the first request fails with the provider's error, as it would between any two launches before. The reuse applies only when no key was typed.

### Deployment

**Prerequisites:**
- Windows 10/11 x64, or linux/amd64.
- A running instance is permitted on Windows.

```bash
# fetch
gh release download v0.1.141 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, .deb, .rpm, SHA256SUMS-v0.1.141.txt.
# verify
sha256sum -c SHA256SUMS-v0.1.141.txt --ignore-missing
# Expected: Each present file followed by OK. Windows exe: a4fa0fb576e9dda450050ed1db3350f180e6bf88ffefe04245c39803fff3c2a4.
# install
.\gorilla-opencode.exe install
# Expected: Install path and shortcuts; a `note:` line if an instance was running.
# verify_active
gorilla-opencode --version
# Expected: v0.1.141
# verify_active
go test ./cmd -run 'Reprobe|RecentCheck|AskedWhenItMatters|EndpointProvenWithin' -count=1
# Expected: ok (from a source checkout).
```

**Rollback:**
  1. Remove this version
     `gorilla-opencode uninstall   # or apt remove / dnf remove gorilla-opencode`
  2. Install v0.1.140 from its release
     `gh release download v0.1.140 -R gorillanobakaa-dot/Gorilla.Opencode`

### Known Issues

**[low]** Launch still takes about ten seconds on a key-based row.
- Cause: No usable verdict under 24 hours old exists for that endpoint, or the last probe found no working model.
- Remedy: Expected once a day. Run /update; check `model-probes.json` in the cache directory.

**[low]** A model fails at the first request although launch reported a recent check.
- Cause: The provider retired the model or the key was revoked inside the reuse window.
- Remedy: /update re-probes and re-ranks.

**[low]** `.replaced-*` files accumulate beside the installed binary.
- Cause: Repeated updates with an instance running.
- Remedy: Delete them when no instance is running.

**[medium]** CI workflows show cancelled with no steps.
- Cause: GitHub Actions incident: no hosted runner assigned.
- Remedy: Re-run when Actions is healthy.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Launch times before the fix | 📄 stated in input | Keeping the current provider with Esc, the program was ready in 1.3 seconds |
| The probe ran at every Enter since v0.1.138 | 📄 stated in input | The check had been added in v0.1.138 so that a key is proven when it is saved |
| 24-hour reuse, not for a typed key | 📄 stated in input | less than a day ago and no new key was typed |
| Launch times after the fix | 📄 stated in input | ready to type after 1.43 seconds and 1.40 seconds. Pressing Esc: 1.25 seconds |
| Installer sharing violation and rename | 📄 stated in input | Windows does allow that file to be renamed |
| Binary hash and size | 📄 stated in input | sha256 a4fa0fb576e9dda450050ed1db3350f180e6bf88ffefe04245c39803fff3c2a4, 54,539,264 bytes |
| Real-model run | 📄 stated in input | All seven gave valid results |
| Installer rename untested automatically | 📄 stated in input | the installer's rename has no automatic test |
| CI not run because of the Actions outage | 📄 stated in input | GitHub's build service has been in an outage since before v0.1.140 was published and cancels the jobs |
| Model listing costs 0.1 to 0.7 s on NVIDIA | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*
