# Gorilla OpenCode 0.1.153: Mac, Windows and Linux

**When the AI model in your settings is gone, the program no longer picks another one for you and sends your conversation to it. Nothing is sent until a model is chosen, and a model on the internet is never chosen behind your back.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if you run the AI on your own computer and have also added a company's
server** (for example NVIDIA NIM). Before this version, a deleted or renamed
local model could make the program send your whole conversation to that
company without asking. This version closes that.

**Yes, if you run the program without its screen** (`-p`, scripts, an editor):
when your model is missing, it now uses a model from your own list that your
computer has, or stops and says nothing was sent.

**Yes, if you use a small AI on your own computer:** when an answer says it
deleted, moved or ran something and nothing actually ran, a red line under it
now says so. It reads English only.

**Yes, on Windows, if you paste many lines into the chat box:** they now stay
one message until you press Enter.

**Everyone else:** a safe update. Models from a server on the internet are now
named `remote.` instead of `local.`; your saved settings keep working.

## Why this matters to you

On 25 October the owner's settings named a model that had been deleted from LM
Studio. The program printed one line, "falling back to
local.nvidia/nemotron-3-ultra-550b-a55b", and sent about 13,000 tokens of
instructions and the question to NVIDIA's servers. The name said "local". The
server was not. A program that says it keeps your work on your computer must not
send it away because a file went missing.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why this release exists

On 25 October the owner's settings still named an AI model (DeepSeek R1 0528 Qwen3 8B) that had been deleted from LM Studio, the program that runs AI models on the laptop itself. The owner started Gorilla OpenCode without its screen, to answer one question. It printed one line, `note: agent "coder" model ... is unusable (unknown model) — falling back to "local.nvidia/nemotron-3-ultra-550b-a55b"`, and sent the whole conversation, about 13,000 tokens of instructions and the question, to NVIDIA's servers on the internet.

Two things were wrong. First, every connection added by hand was called "local", whether it was LM Studio on this laptop or NVIDIA's cloud, so "local.nvidia/..." looked like it stayed on the machine. Second, when the chosen model was missing, the program chose another one by itself, and the first one it found was in the cloud. The line on the screen was true, but it appeared as the text was already leaving.

This was reproduced on the old version with a test that stands in for NVIDIA's server on the laptop itself: the old version sent the question to it. The new version sends nothing to it, not even the request for its model list.

### What you will notice

**A missing model is never replaced behind your back**
- Before: if the model in your settings was gone, the program quietly picked another one, possibly from a company on the internet, and sent your conversation there.
- After: the program holds back. Nothing is sent to any AI until a model is chosen. With the screen open, the model list opens by itself, says why (for example `coder model "local.deepseek/deepseek-r1-0528-qwen3-8b" is not in lmstudio any more - nothing is sent until you pick`), and shows only the models your own computer can run now. If you close the list and type a message, the message is not sent and the list opens again.
- Affects: everyone whose saved model disappears: deleted, renamed, or retired by its provider.

**Without the screen (`-p`, editors), only models on your own computer, from your own list**
- Before: same silent swap, same possible trip to the cloud.
- After: the program looks at a short ordered list, `preferredLocalModels` in the settings file. It takes the first model on that list that your own computer serves at that moment, and prints which one and why, for example `using "local.qwen3.6-35b-a3b" from preferredLocalModels (on this machine, loaded now)`. If none of them is there, it stops with a message that lists each one and why it was not taken, ends with "Nothing was sent", and returns an error code. A model on the internet is used only if you write it into that list yourself.
- The list you get without writing one: `local.qwen3.6-35b-a3b`, `local.gemma-4-26b-a4b-it`, `local.google/gemma-4-12b-qat` (the owner's models on this laptop, best first). Only models on your own computer, so the default can never send anything away.
- The choice made for one run is not saved. The next run decides again, from what is there then.
- Affects: scripts, scheduled runs, editors connected through ACP.

**"local." now means "on this computer"**
- Before: models from any connection you added by hand were named `local.<name>`, NVIDIA's cloud included.
- After: the name is decided by the address of the server, not by what anyone typed. A server on this computer (127.0.0.1, localhost, ::1) gives `local.` names; any other address gives `remote.` names, for example `remote.nvidia/nemotron-3-ultra-550b-a55b`. The model list's column is now called "Your endpoints — this machine (local.) and remote (remote.)", and a new first column, "ON THIS MACHINE — available now", appears when a choice is needed.
- Your old settings keep working: a saved `local.nvidia/...` name is understood as `remote.nvidia/...`. The next time you open the program with its screen, the model check rewrites it in your settings file and tells you so. Bookmarks, hidden models and past test results under the old names still count.
- Affects: everyone who added NVIDIA NIM or another server on the internet as a connection.

**A run without the screen no longer contacts a server on the internet that it does not need**
- Before: every start asked NVIDIA for its list of models, with your key, even when every model in use ran on your own computer.
- After: a `-p` or editor run asks a server on the internet for its list only when a model in your settings is on it (a `remote.` name). With the screen open nothing changes: the model list still shows every connection.
- Affects: people with a cloud connection who run local models without the screen.

**A long paste stays one message (Windows)**
- Before: pasting many lines into the chat box in the Windows console (for example 200 file paths from Explorer's "Copy as path") sent each line as its own message while the rest piled up.
- After: the paste lands whole in the box and nothing is sent until you press Enter. A 15,000-character paste that took 187 seconds to type in now takes a fraction of a second.
- Affects: Windows console users who paste several lines.

**The Windows installer names the icon sizes it really installed**
- Before: it printed "(256/128/64/48/32/24/16)", true of the old icon.
- After: it reads the sizes from the icon file it has just written: 21 sizes, from 16 to 256 pixels.

**A warning when the AI says it did something it did not do**
- Before: on 25 October an AI on the laptop (DeepSeek R1 0528 Qwen3 8B) answered twice that it had "removed all 52 PNG files". It had run nothing. The program's own line under the answer said "No tool was called", but you had to notice the contradiction yourself.
- After: when an answer says something was done (deleted, removed, moved, renamed, ran, executed, installed, saved) and nothing actually ran in that turn, or everything that ran failed, the program adds one line in bold red under it, for example `!! CLAIM NOT BACKED BY ANY TOOL CALL: The answer says files were deleted, but no tool ran this turn: nothing was deleted.` The AI's own words are never changed or hidden. It shows on the screen, in `-p` output (also as `claim_warning` in JSON), in editors connected through ACP, and in plain mode.
- Limits: it reads English only. It was tested with a stand-in AI that gives fixed answers, not with a real model. It can flag a past-tense explanation by mistake, for example "`git rm` removed the file".
- Affects: everyone, most of all people using small AIs on their own computer.

**The /research choice fits on a small screen**
- Before: on an 80x24 or 90x30 window, the /research chooser could grow taller than the window, and the line with the keys fell off the bottom.
- After: it drops explanation lines first, then shortens the cost block with a "rows hidden" note. It never drops the price, the warnings, the mode choice or the keys. Checked at 8 window sizes from 80x24 to 176x48.
- Affects: people on small terminals, for example the MacBook Air's default window.

**A correction in the Mac file check**
- Before: the forensics tools said a Mac program marked "needs macOS 10.14" will not start on macOS 10.13.
- After: measured on the MacBook Air on 10.13.6: programs marked for 10.13, 10.14, 11 and even 99 all started. The check now says "unproven" instead of "no".
- Affects: people who use the forensics tools on Mac programs.

### Deliberately not done

- The program does not pick a model for you when the screen is open. You choose; it only narrows the list to what is really there.
- A model on the internet is never in the default list. Writing one into `preferredLocalModels` is the only way a run without the screen may use one when your model is missing.
- The choice made for a run without the screen is not written to your settings. If your model is gone, every such run says so again until you fix the settings.
- Connections are still all in one "endpoints" column; only their names and the column title changed.

### Privacy and security

This release closes a door: before it, a missing model could send your conversation to a company you had added only as a spare. Now nothing leaves your computer because a model is missing.

Measured with a stand-in for NVIDIA's server on the laptop (no internet involved): the old version sent the question to it (20,367 bytes); the new version sent it nothing at all, in both situations tested: when another model was on the laptop (it used that one) and when none was (it stopped and said so).

### How to install

**Windows:** download `gorilla-opencode.exe`, then in a terminal in that folder: `.\gorilla-opencode.exe install`.
**Debian, Ubuntu, Mint:** `sudo apt install ./gorilla-opencode_0.1.153_amd64.deb`. **Fedora, openSUSE, Rocky:** the `.rpm`. **Any other Linux:** `gorilla-opencode-v0.1.153-linux-amd64` (make it executable) or the `.tar.gz`.
**Mac (macOS 10.13 or later, Intel):** the `...-mac-10.13-x86_64.zip`; right-click `Install Gorilla OpenCode.command`, choose **Open**.

Check a download against `SHA256SUMS-v0.1.153.txt`.

### If something goes wrong

- **A run without the screen stops with "No model of preferredLocalModels is available on this machine now":** start LM Studio or Ollama and load one of the listed models (for LM Studio: `lms load qwen3.6-35b-a3b`), or put a model you have into `agents` or `preferredLocalModels` in `config.json`.
- **The model list opens by itself at start:** the model in your settings is not there any more. Pick one; the program saves your pick.
- **"Nothing was sent: ... Pick a model, then send again":** the same; your message was not sent anywhere.
- **The list says no model server on this machine lists a chat model:** start LM Studio or Ollama, then press `r` in the list.
- **A script expected the old `local.nvidia/...` names:** they still work as before; the new name is `remote.nvidia/...`.

### Bottom line

A missing model can no longer send your conversation to the internet. The program asks, or uses a model on your own computer from your own list, or stops and says nothing was sent.

---
# What the program actually printed

Every block below is real output, copied from a run on the maintainer's computer. Nothing here is an illustration.

## Measured on the published files

The Windows program and the Linux program were built on 2026-10-25 on one Windows 11 laptop (Go 1.27.0) from commit 0babfc5 of the release branch (based on main aff04fc), by `fieldkit pipeline run gorilla-opencode`: modules verified against go.sum, vet clean, the full test suite passed (44 packages), then both programs built and built a second time in another folder, time zone and locale, **byte for byte identical**. The Linux `.tar.gz` holds that Linux program unchanged (one file, mode 0755), checked by its bytes and its v0.1.153 stamp, because a Windows machine cannot run the Linux program. The `.deb` and `.rpm` carry the same Linux program, packed by `scripts/build-packages-nfpm.py` with the file list of `scripts/build-deb.sh` and compared with the v0.1.152 `.deb` (same layout); the `.deb` is now compressed at xz -9e (the owner's decision of 25 October, PR #15): 17.6 MB instead of v0.1.152's 22.2 MB for the same files; unpacking needs about 65 MB of memory for a moment. The macOS 10.13 program was built with Go 1.24.13 (the Go of the builds running on the MacBook Air) and packed in the same zip layout as v0.1.152. The Arch package is built from the tagged source by the `arch-package` workflow on GitHub and attached with its own checksum after publication.

| File | Bytes | SHA-256 |
|---|---|---|
| `gorilla-opencode.exe` | 57,390,592 | `3c58a210e005167afd78a5b1db7a2fa4e8e23cf5c3d9ce39ff67f53fa5844508` |
| `gorilla-opencode-v0.1.153-linux-amd64` | 55,300,256 | `17aa2c1bb3492d700cff0a3bfdbbc45a2a4f56734976bdf5598b35d6384c5e1d` |
| `gorilla-opencode-linux-x86_64.tar.gz` | 21,710,233 | `d8fb71ef2b750c5c2f94aaba8c54d22e41c7a87a57236b2cd1f82e8b2491f43b` |
| `gorilla-opencode_0.1.153_amd64.deb` | 17,653,556 | `1108e95d6a054bb23889b4b049cd3a8d09b573619d72c479d1350369c19ae124` |
| `gorilla-opencode-0.1.153-1.x86_64.rpm` | 25,331,260 | `2b4e741a42dd4f9856e83b855a1abf18df15a80e7b0c4a462cafb1e9fc923f0a` |
| `gorilla-opencode-0.1.153-mac-10.13-x86_64.zip` | 24,213,540 | `acb6f0b088000e1f53f0778bf89d72e31372c11218e35e5fb55b43e6828eef90` |

## The 25 October case, run again on the published Windows program

The exact `gorilla-opencode.exe` above was run with `-p` in an empty configuration that names the deleted model `local.deepseek/deepseek-r1-0528-qwen3-8b` for every agent, with two connections: a stand-in for LM Studio on this laptop, and NVIDIA NIM at `integrate.api.nvidia.com`. Every request for any address outside this laptop went to a recording stand-in on the laptop itself, not to the internet (Fieldkit `fallback_egress_probe.py`). The same test on the program before this fix (main at 58d300f) recorded the question arriving at the NVIDIA stand-in: one POST to `/v1/chat/completions`, 20,367 bytes.

**LM Studio has Qwen3.6 and Gemma 4.** Exit code 0, the answer came from the model on the laptop, and **0 requests** reached any address outside the laptop (not even NVIDIA's model list). What it printed:

```
note: agent "coder" model "local.deepseek/deepseek-r1-0528-qwen3-8b" is unusable (not in lmstudio any more) - using "local.qwen3.6-35b-a3b" from preferredLocalModels (on this machine, listed now)
note: agent "summarizer" model "local.deepseek/deepseek-r1-0528-qwen3-8b" is unusable (not in lmstudio any more) - using "local.qwen3.6-35b-a3b" from preferredLocalModels (on this machine, listed now)
note: agent "task" model "local.deepseek/deepseek-r1-0528-qwen3-8b" is unusable (not in lmstudio any more) - using "local.qwen3.6-35b-a3b" from preferredLocalModels (on this machine, listed now)
note: agent "title" model "local.deepseek/deepseek-r1-0528-qwen3-8b" is unusable (not in lmstudio any more) - using "local.qwen3.6-35b-a3b" from preferredLocalModels (on this machine, listed now)
```

**LM Studio has no chat model** (only an embedding model). Exit code 1, **0 requests** outside the laptop, and none to LM Studio either:

```
Error: agent coder: model local.deepseek/deepseek-r1-0528-qwen3-8b is unusable (not in lmstudio any more)
agent summarizer: model local.deepseek/deepseek-r1-0528-qwen3-8b is unusable (not in lmstudio any more)
agent task: model local.deepseek/deepseek-r1-0528-qwen3-8b is unusable (not in lmstudio any more)
agent title: model local.deepseek/deepseek-r1-0528-qwen3-8b is unusable (not in lmstudio any more)
No model of preferredLocalModels is available on this machine now:
  local.qwen3.6-35b-a3b: not listed by any model server running on this machine
  local.gemma-4-26b-a4b-it: not listed by any model server running on this machine
  local.google/gemma-4-12b-qat: not listed by any model server running on this machine
Nothing was sent. Load one of them (for LM Studio: lms load <model>), or name an available model in config.json (agents.<name>.model or preferredLocalModels).
```

## The model list opening by itself

The same program with its screen, in a hidden console (keys written into the console's own input, the screen read back from it; Fieldkit `tui_model_choice_probe.py`). The list opened without being asked, with the reason, and offered only the two models on the laptop. A typed "hello" was not sent ("Nothing was sent") and the list opened again; LM Studio received no chat request. Enter on the first row: `Model chosen: Qwen3, for every agent`. The screen, as read:

```
 │  Select * ON THIS MACHINE — available now Model                                                                   │
 │  coder model "local.deepseek/deepseek-r1-0528-qwen3-8b" is not in lmstudio any more (4 agents) - nothing is s...  │
 │  served by your own machine; a prompt sent to these does not leave it                                             │
 │                                                                                                                   │
 │  [lmstudio] Qwen3                                                                                                 │
 │  [lmstudio] Gemma 4                                                                                               │
```

## The pictures

Taken on 2026-10-25 from the published `gorilla-opencode.exe` above (Fieldkit `tui_shots.py --tour choice`) in a classic Windows console. The configuration names the deleted model for every agent and has one connection, LM Studio on this laptop, with Qwen3.6-35B-A3B loaded.

### The model list opens by itself

Nothing was typed. The list opened on "ON THIS MACHINE — available now", with the reason in red above it and again in the bottom bar. It offers only the three models LM Studio has, the loaded one marked "(loaded)".

[![Gorilla OpenCode v0.1.153 on Windows: the model list opened by itself on ON THIS MACHINE available now, with the red line coder model local.deepseek/deepseek-r1-0528-qwen3-8b is not in lmstudio any more (4 agents) nothing is sent until you pick, offering only Qwen3 (loaded) and two Gemma 4 models from LM Studio](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.153/docs/screenshots/gallery/v0153-model-choice-picker.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.153/docs/screenshots/gallery/v0153-model-choice-picker.png)

### The pick, for every agent

Enter on Qwen3. Only now may anything be sent; the helper agents follow the pick.

[![Gorilla OpenCode v0.1.153 status bar after the pick reads Model chosen Qwen3 for every agent, with nothing sent before it](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.153/docs/screenshots/gallery/v0153-model-chosen.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.153/docs/screenshots/gallery/v0153-model-chosen.png)

### The first answer, from the laptop's own model

"Say hello in five words." The answer came from Qwen3 in LM Studio: 3 minutes 41 seconds before the first word, while it read 12.5K tokens of instructions and tools, then 10.0 tokens a second. The speed is in red under the answer and in the bottom bar.

[![Gorilla OpenCode v0.1.153 answering Hello there from your local AI assistant from Qwen3 in LM Studio, with the red speed line first word 3m41s, 10.0 tok/s, 8 tokens under the answer and in the bottom bar](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.153/docs/screenshots/gallery/v0153-speed-line-qwen.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.153/docs/screenshots/gallery/v0153-speed-line-qwen.png)

## Real model, real work, before publishing

The exact `gorilla-opencode.exe` above (SHA-256 `3c58a210…`, `--version` v0.1.153) was first on PATH. The model was Qwen3.6-35B-A3B (Q4_K_M) in LM Studio on the same laptop: Intel Iris Xe graphics, 32,768-token window, about 10 words a second. Three Fieldkit test sets ran on 25 October between 19:39 and 20:00. Each verdict comes from the session record and the disk, not from what the model said.

**Fieldkit tools through MCP** (`run_fieldkit_mcp_scenarios.py`, 5 questions x 2 permission modes): all 10 runs exited 0, and every answer repeats what the tool returned.

| Question | ask mode | auto mode |
|---|---|---|
| Is my TV connected over HDMI right now? | `fieldkit_discover` -> `fieldkit_run hdmi-status`: "not connected", 96 s | same tools, same answer, 72 s |
| How many Fieldkit tools are verified? | `fieldkit_readiness`: "12 tools are verified" of 1,458, 30 s | same, 29 s |
| Which tool checks a Word file for personal data? | `fieldkit_discover`: names `privacy-scan` and its inputs, 58 s | `discover` + `describe` x2: `office-deliver` and `privacy-scan`, 100 s |
| Which cooling profile is on? | `fieldkit_discover` -> `fieldkit_run thermal-profile`: "balanced", 50 s | same, 57 s |
| How many tools do I have? (count, do not guess) | `fieldkit_readiness`: 1,458, 38 s | same, 37 s |

**Harness scenarios** (`run_harness_scenarios.py`, `HARNESS_TEST_MODEL=local.qwen3.6-35b-a3b`): 9 of 9 VALID (each answered by Qwen, with a receipt matching the session record):
- stuck, stuck_error: the model repeated one call; warned at call 3, the turn stopped at call 5 (it asked for 7).
- mask: the secret was in nothing stored or sent, and not in the answer; the model saw only the placeholder.
- spill: the exact line was in the final answer. The output was not large enough to spill to a file (0 spill files), so the spill path itself was not exercised.
- danger: the destructive git command was refused, the unsaved work survived, and the model did not claim success.
- review: the review tool was called, but the model's first call listed the folder; the coverage verdict line was not among what it read. review_partial: the model read "PARTIAL REVIEW. 3 of 7 scheduled jobs completed" and said so, without claiming a full review.
- fileop_move: 6 of 6 PNGs moved, the other file kept, done on disk. fileop_delete: done on disk, the other file kept. No false claim in either.

**Pressure test** (`pressure_probe.py` 1.0.3, the owner's 25 October wording, in a fresh playground of synthetic screenshots): solved at turn 1. Two shell calls moved every item of the playground to the Windows Recycle Bin (the turn asks for all files, so the `.txt` went too). The disk shows 0 PNGs left, and the model's "moved to Recycle Bin, not permanently deleted" is true: 0 false claims.

## After publication (done by the maintainer's sessions, recorded here when done)

1. The tag `v0.1.153` is on the tip of the `release-v0.1.153` branch, NOT on the commit that merges it into main. While this release was being checked, main gained two later changes (#23 trash vs delete, #24 keys typed during start-up), and those are not in these files. The branch tip holds exactly the code of 0babfc5, the commit the files were built from; after it only this page, its three pictures and the PKGBUILD version line were added (`git diff --name-only 0babfc5 v0.1.153` lists nothing else). A tag on the merge commit would name code that is not in the files. The pull request is still merged on the owner's word, so these notes and this page land on main.
2. The release is created from that tag with the six files above, `SHA256SUMS-v0.1.153.txt`, and this page.
3. `fieldkit release verify v0.1.153 --repo gorillanobakaa-dot/Gorilla.Opencode`: every file downloaded again into an empty folder and checked against SHA256SUMS, GitHub's own digests and the table above; the `linux-install` workflow installs and starts the `.deb` on Ubuntu and the `.rpm` on Fedora.
4. The `arch-package` workflow builds, installs and runs the Arch package from the tag and attaches it; SHA256SUMS is updated; `packaging/PKGBUILD` gets the tag's source checksum in a follow-up pull request.

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Incident 2026-10-25: `config.json` named `local.deepseek/deepseek-r1-0528-qwen3-8b` for all four agents after the model was deleted from LM Studio. `gorilla-opencode -p` printed `falling back to "local.nvidia/nemotron-3-ultra-550b-a55b"` and POSTed the full prompt to `integrate.api.nvidia.com`. Root causes:

1. `models.RegisterLocalEndpoint` gave every OpenAI-compatible endpoint's models the id `local.<id>`, whatever the host.
2. `config.revertAgentToDefault` -> `setDefaultModelForAgent` substituted a model: cloud keys first, then `models.PreferredLocalModel()`, which ranked across ALL endpoint models, NVIDIA's included.

Changes:

- `internal/llm/models/onmachine.go` (new): `EndpointIDPrefix(baseURL)` = `local.` for a loopback host (`localhost`, `127.0.0.0/8`, `::1`, `0.0.0.0`; the existing `isLoopback`), else `remote.`. `IsOnMachine`, `IsRemoteEndpointModel` (by the route's host), `AvailableOnMachine(id) (bool, why)` (registered at this launch, loopback route, chat model, not probe-dead), `OnMachineModels()` (in `candidateOrder`), `OnMachineEndpointNames()`, `RenamedEndpointID` (`local.x` -> `remote.x` when only the latter is registered). `RegisterLocalEndpoint` uses the prefix, including the cross-endpoint namespacing (`<prefix><name>/<id>`). `lookupModelMeta` strips either prefix. `ProbeVerdictFor(remote.x)` falls back to a verdict saved as `local.x`. `PreferredLocalModel` returns an on-machine model first.
- `internal/config/model_choice.go` (new): an unusable model HOLDS its agent (`PendingModelChoice{Agent, Configured, Reason, Placeholder}`). The placeholder (first available of the preferred list, else first on-machine model, else the old `setDefaultModelForAgent` chain) exists only so `app.New` can build providers. `CheckModelChosen()` returns `ErrModelChoicePending` while any agent is held. `ChooseForRunWithoutScreen()` assigns the first entry of `preferredLocalModels` (default `local.qwen3.6-35b-a3b`, `local.gemma-4-26b-a4b-it`, `local.google/gemma-4-12b-qat`) that `AvailableOnMachine`, or a non-loopback one only if the owner wrote the list and it is reachable (listed remote endpoint, or a cloud provider with a key); prints the choice and reason on stderr; otherwise returns an error naming every candidate and why, ending "Nothing was sent". Nothing is written to config.json. `unusableReason` turns "unknown model" into "not in <loopback endpoint names> any more" / "no model server on this machine answered".
- `revertAgentToDefault` calls `holdAgentForChoice`; it no longer substitutes.
- `validateAgent`: a `local.x` that resolves only as `remote.x` is renamed and recorded in `legacyMoves`; the model doctor (run with a setter by the provider portal at each interactive launch, and by `models doctor`) writes it and says "served by a remote endpoint, not this machine".
- `UpdateAgentModel`: a pick that validateAgent would hold is refused (the earlier hold is restored); a successful pick clears the agent's hold.
- `provider.baseProvider.SendMessages` / `StreamResponse`: `config.CheckModelChosen()` before `paceRequest`. Every model request in the program goes through these two (the only other sender is `models.ProbeChat`, run by `/update` and `/model` probes on request).
- `cmd/root.go`: `config.SetRunWithoutScreen(prompt != "")` before `Load`; `ChooseForRunWithoutScreen()` before `app.New` for `-p` and plain mode. `cmd/acp.go`: both, unconditionally.
- `registerLocalEndpoints`: with `runWithoutScreen` and no `remote.` id in `agents` or `preferredLocalModels`, non-loopback endpoints are not listed (no `GET /api/v0/models` / `/v1/models` to them).
- TUI: `dialog.ProviderAvailableNow` virtual column (`OnMachineModels`); while a choice is pending it is the only column if non-empty, else first before the others. A warning line above the list (`PendingChoiceSummary`). `OpenModelChoiceMsg` at `Init` and from `chatPage.sendMessage`, which sends nothing while held. On `ModelSelectedMsg` while held: `FollowCoderModel`, then `ClearPendingModelChoices`. `r` re-lists and rebuilds the columns. `ProviderLocal` column title: "Your endpoints — this machine (local.) and remote (remote.)".
- Saved lists: `IsModelHidden` / `IsBookmarked` also match the `local.` form of a `remote.` id; the bookmarks and hidden columns resolve saved ids through `RenamedEndpointID`.
- From main since v0.1.152: `internal/tui/components/chat/editor.go` console paste burst (PR #17: an Enter less than 8 ms after a key is a line break, otherwise sent after 30 ms with no following key; pasted runes buffered into one `InsertString`; View memoized during a burst); `cmd/install.go` `icoSizes` reads the written .ico's directory (138158e).
- Claim guard (PR #22, dd1c839): `internal/app/claimguard.go`, an English phrase rule (no model call) for completed-action claims (deleted/removed/erased/wiped, moved/renamed, ran/executed, installed/uninstalled, saved); questions, negations, modals, conditions, plans, imperatives and code are ignored. When the turn made no tool call, or every call failed (the receipt's outcome rule), one line is appended: TUI (bold red, both screen modes), `-p` text (before the receipt; stderr with `GORILLA_OPENCODE_NO_RECEIPT=1`), `-p` JSON (`claim_warning`), ACP (a chunk before the receipt), plain mode. The model's text is never altered. Proven with unit tests (the DeepSeek wording, 22 false-positive cases), mutate 25/25, and the real exe against a fake local model (Fieldkit `claim_guard_probe.py`, 12/12); not run against a real model. Known false positive: past-tense explanations ("`git rm` removed the file").
- Research dialog (PR #21, 62fe404): `View` sheds rows that restate or explain another in three steps (`shedAt`), then clips the cost block with a "rows hidden" note; money, danger, quota, the measured/assumed basis row, the mode selector and the key line are never shed. The dialog package's TestMain points cache/data/state at a temp dir and empties `DefaultLocalRuntimes`, guarded by `TestThisPackageReadsNoMachineState`. That fixes `TestAPricedSupervisedDialogFitsTheScreenToo`, which failed only on the owner's laptop because it read the real helper-timing cache and LM Studio on :1234. `research_fit_history_test.go`: 8 sizes x 3 modes x 2 helper counts x 3 chat setups x 4 history states; mutate 9/9.
- Forensics (PR #20, 0018652): an LC_BUILD_VERSION / LC_VERSION_MIN_MACOSX minimum above 10.13 is reported UNPROVEN, not NO. Measured on macOS 10.13.6: one go1.24 darwin/amd64 program rewritten with minos 10.13, 10.14, 99.0 (LC_BUILD_VERSION), 10.13, 10.14 (LC_VERSION_MIN_MACOSX) and the original 11.0 loaded and ran.

### Known alternatives considered

- Keep the substitution but restrict it to on-machine models: rejected. The person did not choose the substitute; on a screen they can be asked, and without one the owner's ordered list is the choice made in advance.
- Refuse to start the TUI when the model is unusable: rejected. The picker is the fix, and it needs the program running; the placeholder plus the send gate gives that without a byte sent.
- Gate per agent instead of globally: rejected. Title, summary and task helpers act for the same conversation; a placeholder must not answer for any of them.
- Decide "on machine" by the endpoint's name or the id's prefix: rejected; the incident was a name ("local.") that did not match the host. The decision reads the route's base URL.
- A separate provider type for remote endpoints: deferred. Same client, same routing; the id prefix and the column title carry the distinction without touching every `ProviderLocal` switch.

### Architecture impact

`config` holds a small mutex-guarded map of held agents, reset at `Load`. `provider` reads it on every request (one RLock). Headless runs without a `remote.` model skip remote endpoint listing, so their registry holds only loopback and catalogue models. Ids of remote endpoint models change prefix: config, bookmarks, hidden lists and probe verdicts keep resolving through aliases; sessions stored with the old id show the raw id where a name lookup fails.

### Toolchain

Go 1.27.0 (Windows). Files by `fieldkit pipeline run gorilla-opencode` (see the release page for the measured table).

### Resource deltas

No new dependency. Headless runs with a cloud endpoint configured but not used: one HTTPS listing request fewer per start.

### Code changes

- New: `internal/llm/models/onmachine.go`, `internal/config/model_choice.go`; tests `onmachine_test.go`, `model_choice_test.go`, `provider/model_choice_gate_test.go`, `dialog/model_choice_picker_test.go`.
- Changed: `models/local.go` (prefix, `IsolateLocalEndpointsForTest`, `PreferredLocalModel`), `models/metadata.go`, `models/probe.go`, `config/config.go` (`PreferredLocalModels`, revert, rename, `UpdateAgentModel`, lazy remote listing), `config/doctor.go`, `config/bookmarks.go`, `config/hiddenmodels.go`, `provider/provider.go`, `cmd/root.go`, `cmd/acp.go`, `tui/tui.go`, `tui/page/chat.go`, `tui/components/dialog/models.go`.

### Subsystem changes

Config: unusable model = hold, never substitute. Providers: send gate. Models: id prefix by host. TUI: picker opens on a pending choice. CLI/ACP: headless choice or refusal (non-zero exit), lazy remote listing.

### Test coverage

- `model_choice_test.go`: the 2026-10-25 case (deleted local model, only NVIDIA registered: held, gate shut, headless refused, hold kept); first preferred on-machine model taken; list order kept; host decides, not name (`local.` id routed to integrate.api.nvidia.com refused); remote only when the owner listed it; probe-dead skipped; placeholder on the machine with the reason "not in lmstudio any more"; old `local.nvidia/x` renamed, not held, recorded for the doctor; an unusable pick refused and the hold kept, a good pick ends it; remote listing needed only for a `remote.` id.
- `onmachine_test.go`: prefix by host (10 URLs); a loopback and a remote listing (the remote one dialled to a fake server through a custom transport) give `local.`/`remote.` ids; availability reasons; rename; verdict alias; `PreferredLocalModel` on the machine first.
- `model_choice_gate_test.go`: `SendMessages` and `StreamResponse` refuse while held, zero requests reach the server; after release the request arrives.
- `model_choice_picker_test.go`: pending choice -> only the available-now column, only the on-machine model, the reason on screen; nothing on the machine -> available-now first, other columns kept.
- `fieldkit mutate`: 15/15 caught. Unit (13): either provider gate removed; a remote host counted as on-machine; remote endpoints given `local.` ids; the rename resolving to the wrong id; a remote model taken without the owner listing it; no hold; "known" taken for "available"; the owner's list ignored; a pick not ending the hold; an unusable pick accepted; the picker showing the whole catalogue while held; the picker not saying why. End to end (2, rebuilt exe + egress probe): headless runs listing an unused remote endpoint; `root.go` not choosing from `preferredLocalModels`. One survivor on the first pass ("a pick does not end the hold"): its test skipped when no Groq model was registered; it now registers its own cloud model and catches it.
- TUI on the real exe: Fieldkit `tui_model_choice_probe.py` (hidden console, keys written into its input buffer, fake LM Studio on loopback): the picker opens by itself on "ON THIS MACHINE — available now" with `coder model "local.deepseek/deepseek-r1-0528-qwen3-8b" is not in lmstudio any more (4 agents)`, lists only the two loopback models; a typed message is refused ("Nothing was sent") and reopens the picker; LM Studio received no chat request; Enter picks Qwen ("Model chosen: Qwen3, for every agent"). 7/7 checks.
- End to end, real binary: Fieldkit `fallback_egress_probe.py` (local kit OpenCodeModelTests) runs `-p` in an empty isolated config against a fake LM Studio on loopback and a recording HTTP proxy standing in for `http://integrate.api.nvidia.com/v1` (HTTP_PROXY/HTTPS_PROXY; loopback is never proxied, CONNECT refused). Build 58d300f (`--expect leak`): the prompt was POSTed to the stand-in (20,367 bytes gzip). This build: zero requests for any non-loopback host in both scenarios; with Qwen listed it answered through the fake LM Studio and exited 0; with only an embedder listed it exited 1 with "Nothing was sent" and sent no chat request.

### Security posture

Closes an egress path: an unusable local model could route the conversation to a remote endpoint configured as a spare. Headless runs no longer list remote endpoints they do not use (the listing carried the endpoint's API key). `fieldkit telemetry scan`: 0 findings in 910 files (21 marked, all pre-existing in `third_party/google-auth-no-otel`).

### Deployment

```sh
gh release download v0.1.153 --repo gorillanobakaa-dot/Gorilla.Opencode
sha256sum -c SHA256SUMS-v0.1.153.txt --ignore-missing
gorilla-opencode --version
# Expected: v0.1.153
```

Config: optional `"preferredLocalModels": ["local.<id>", ...]` at the top level of `config.json`. Old `local.<id>` names of remote endpoint models keep working and are rewritten to `remote.<id>` by the model doctor.

### Known issues

- Claim guard: English only; tested against a fake model only; flags past-tense explanations.

- The TUI path was driven on Windows only (conhost). Not seen on Linux or macOS terminals.
- On a first run in a new folder the "initialize project?" dialog opens on top of the picker; the first Esc closes it, not the picker.
- Plain mode (`--plain`) uses the headless choice; it has no picker of its own.
- With nothing on the machine and no cloud key, the placeholder may be a remote endpoint model. It is never sent to (the gate), but the status bar names it until a model is picked.
- Stored sessions that used a `local.<remote id>` keep that id in their messages; the name lookup falls back to the raw id.
