# Gorilla OpenCode 0.1.152: Windows and Linux

**With an AI that runs on your own computer, you can now see where the time goes: when the first word came, how fast the rest came, and how much of it was thinking. Switching thinking off now really switches it off, and a conversation that still fits is no longer stopped as "too big".**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if you run the AI on your own computer** (LM Studio, Ollama). You get a
speed line under every answer, a warning when the instructions fill too much of
the AI's memory, and no more false "conversation too big" stops.

**Yes, if you switched thinking off.** LM Studio kept the AI thinking anyway; a
three-word greeting took 30 seconds. Now it takes about 3.

**Everyone else:** a safe update, nothing you rely on changes; the program has a
new icon, the gorilla in a cyan flame ring.

**Debian, Ubuntu, Fedora and Mac:** the files for these are built on another
machine and will be added to this page; until then 0.1.151 is the one to use.

## Why this matters to you

An AI on your own laptop costs nothing per word, but it costs time: on the
owner's laptop, over four minutes before the first word of a new conversation.
Until now the program gave no hint why. Now the answer is on the screen, under
every reply, in the same terms LM Studio uses.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why this release exists

On 24 October the owner used an AI that runs on the laptop itself (Gemma 4 12B, through LM Studio). It felt much slower in this program than in LM Studio, and nothing on screen said why. Asked how many Fieldkit tools it could use, it answered "hundreds" without counting. On the fourth message the program stopped with "This conversation has grown bigger than Gemma 4 can hold" and no answer.

LM Studio's own log showed what happened. Before its first word, the AI spent 4 minutes 17 seconds reading 12,592 tokens of instructions and tool descriptions. After that it wrote about 4 words a second, and most of those words were its thinking. The "too big" stop was a wrong estimate: the conversation still fitted, with about 3,000 tokens to spare.

### What you will notice

**A speed line under every answer**
- Before: you saw only how long an answer took in total, for example "Gemma 4 (3.0m)".
- After: you also see when the first word came and how fast the rest came, for example `first word 4m25s, 3.8 tok/s, 299 tokens (287 thinking)`, in bright red so it stands out on a dark or a white terminal. The same numbers are in a red label in the bar at the bottom, and in an exported conversation. "Thinking" is how much of the answer was the AI working things out before it replied.
- Affects: everyone; it matters most with an AI on your own computer, where time is the cost.

**"Ask the model to think out loud" switched off now works**
- Before: with the switch off, LM Studio still had the AI think, because the program never told it not to. Measured: a three-word greeting took 30 seconds, and 117 of its 142 tokens were thinking.
- After: with the switch off, the program says "no thinking". The same greeting took 3.3 seconds. With the switch on, nothing changes: you still see how the AI reasoned.
- Affects: people who switched thinking off and run the AI on their own computer.

**A conversation is no longer stopped while it still fits**
- Before: the program counted the AI's old thinking as if it were sent again. It is never sent again, so the program refused a message that would have fitted.
- After: the program uses the size the AI's own server reports. If the conversation really is full, it leaves out long tool results from earlier messages (they stay on your screen) and carries on, and it tells you it did.
- Affects: everyone using a model with a small memory (about 32,000 tokens or less).

**A warning when the instructions take too much room**
- Before: nothing said that the instructions and tools fill a large part of a small AI's memory before you type.
- After: at the start of a conversation the program says so, for example "about 12.6K of Gemma 4's 20K window (62%)", names the parts that take the most room, and points to /context. It switches nothing off for you.
- Affects: people with an AI on their own computer.

**A new icon**
- Before: a purple-cap gorilla.
- After: the owner's new gorilla in a cyan flame ring. Every size Windows uses (16 to 256 pixels) is drawn separately from a 2048-pixel original, so it stays sharp on the taskbar, in Alt-Tab and on a large desktop. The same picture is used on Linux and on the Mac.
- Affects: everyone, after installing this version.

**"How many Fieldkit tools do I have?"**
- Before: the AI repeated "hundreds" from a description.
- After: the description tells it which Fieldkit tool counts, and it answers with the number measured at that moment (Fieldkit pull request #101). Tried with Gemma 4 12B: it called the counting tool and gave the number.

### Deliberately not done

- Thinking is not switched off or limited for you. The owner keeps it on to see how the AI reasons and where it goes wrong.
- No tool is turned off on your behalf when the window is crowded. The warning says what you could turn off; the choice is yours.
- This page has the Windows program, the Linux program and the Arch package. The Debian/Ubuntu, Fedora and Mac files are built on another machine and will be added to this page.

### Privacy and security

Nothing new leaves your computer. The speed line comes from the times at which words arrive, measured inside the program. The "no thinking" request goes only to the AI server you chose.

### How to install

**On Windows:** download `gorilla-opencode.exe`, then in a terminal in that folder: `.\gorilla-opencode.exe install`.

**On Arch or CachyOS:** the `.pkg.tar.zst`. **Any other Linux:** `gorilla-opencode-v0.1.152-linux-amd64`, one file you make executable, or the `.tar.gz`.

**Debian, Ubuntu, Mint, Fedora, Mac:** until their files appear on this page, version 0.1.151 is the latest for those systems.

To check a download is the published file, compare it with `SHA256SUMS-v0.1.152.txt` (Linux: `sha256sum -c SHA256SUMS-v0.1.152.txt --ignore-missing`; Windows: `Get-FileHash gorilla-opencode.exe`).

### If something goes wrong

- **The first word takes minutes with an AI on your own computer:** the AI is reading the instructions and tools. Open /context and turn off what you do not use, or give the AI a larger memory in LM Studio or Ollama.
- **"left out ... old tool output(s)":** the conversation was full; the AI can run the tool again if it needs that output. /compact or /new give it room.
- **No speed line:** answers from before this version have none.

### Common questions

**Why is the first answer slow and the next ones fast?** The AI reads the instructions once; LM Studio keeps what it read. Measured: 4m27s for the first word of a new conversation, 11 seconds when the next conversation started.

**Does "tok/s" match LM Studio?** It is measured the same way, from the first word to the last. With a long conversation in the AI's memory, each word costs a little more, so it can be lower than in a short LM Studio chat.

### Bottom line

You can now see where the time goes with an AI on your own computer, "thinking off" means off, and a conversation that still fits is no longer stopped.

---

# What the program actually printed

Every block below is real output, copied from a run on the maintainer's computer. Nothing here is an illustration.

## Measured on the published files

The Windows program and the Linux program were built on 2026-10-25 on one Windows 11 laptop (Go 1.27.0) from commit 71d8223 of the release branch, by `fieldkit pipeline run gorilla-opencode`: modules verified against go.sum, vet clean, the full test suite passed (44 packages, no exception), then both programs built and built a second time in another folder, time zone and locale, **byte for byte identical**. One earlier run of the same suite failed one test, `TestOrdinaryFailuresReportTheRealErrorAndExitCode` (PowerShell lost its own error message once under the full suite's load); it passed in 5 of 6 runs and is recorded as unstable to fix. The pipeline also built a macOS 10.13 program on this laptop; it is not on this page, because nobody has started that build on a Mac yet. The Linux `.tar.gz` holds that Linux program unchanged (one file, mode 0755); it was made with the layout of `scripts/build-tarball.sh`, and because a Windows machine cannot run the Linux program, the check is that the packed file is byte for byte the reproduced program and carries the v0.1.152 stamp. The Arch package is built from the tagged source by the `arch-package` workflow on GitHub and attached with its own checksum. The `.deb`, `.rpm` and Mac zip are not on this page yet.

| File | Bytes | SHA-256 |
|---|---|---|
| `gorilla-opencode.exe` | 57,307,136 | `bc5057d09562d75d8a7aec3dfb2793dcb314722d2c33faec59ce6234adfda583` |
| `gorilla-opencode-v0.1.152-linux-amd64` | 55,218,336 | `942c2339554c1f7fcbf8e2b6645526d14653142dfec7a7bc9732e2585be71fd4` |
| `gorilla-opencode-linux-x86_64.tar.gz` | 21,679,463 | `620be165f7799d9da0548f3a753ff6ca28bafe5028ca05afff371d099bbdaaee` |

Before publishing, the exact `gorilla-opencode.exe` above was run headless against Gemma 4 12B QAT in LM Studio with the Fieldkit tools connected (`run_fieldkit_mcp_scenarios.py`, scenario "count"). Asked "How many tools do I have access to through Fieldkit? Count them, do not guess.", it called `fieldkit_readiness` and answered "You have 1,445 tools accessible through Fieldkit. Of those, 12 are verified, 44 are tested, 319 are carded, and 1,070 have been gathered only." Both calls were stored with their speed. That run used the processor only: that night the laptop's Intel Iris Xe graphics stopped with "device lost" (Vulkan) on every large request after the first fault at 00:50, so the model was loaded with the graphics switched off. The earlier runs on the same code, on the graphics, are the pictures below.


---

## The pictures

### The speed meter on a model running on this laptop

Version 0.1.152 on Windows 11, Gemma 4 12B QAT served by LM Studio on the same laptop, "think out loud" switched on, asked "Say hello in five words." in a new conversation. Under the answer and in the bar at the bottom: 4 minutes 27 seconds before the first word (the AI reading 13,000 tokens of instructions and tools), then 3.8 words per second, 357 tokens, of which 345 were its thinking. The thinking itself is the cyan text above the answer.

[![Gorilla OpenCode v0.1.152 on Windows with Gemma 4 12B in LM Studio: under the answer and in a red label in the bottom bar the speed reads first word 4m27s, 3.8 tok/s, 357 tokens (345 thinking), with the model's cyan reasoning above](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.152/docs/screenshots/gallery/v0152-speed-meter-gemma.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.152/docs/screenshots/gallery/v0152-speed-meter-gemma.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Measured against the LM Studio 0.4.26 server log with google/gemma-4-12b-qat (20,224-token window): 12,592 prompt tokens read in 257 s before the first token, ~3.9 tok/s generation, and a refused request whose real size was ~14.1K of a 17.2K budget.

- `message.Finish` gains `Speed *Speed` (`omitempty`): `first_ms`, `stream_ms`, `prompt_tokens`, `output_tokens`, `reasoning_tokens`. `agent.streamClock` stamps the request and the first `thinking_delta`/`content_delta`/`tool_use_start`; `recordSpeed` stores it after `EventComplete` and keeps the newest per session for the status bar (`agent.LastSpeed`). Rendered by `speedSuffix` (chat footer, tool-use finishes included), `status.go`, and `export/transcript.go`.
- `provider.TokenUsage.ReasoningTokens` from `completion_tokens_details.reasoning_tokens`.
- `thinkingRequestOptions(modelID, local)`: with `extras-reasoning-generate` OFF and `Provider == ProviderLocal`, sends `reasoning_effort: "none"`. Rejection handling shared with `chat_template_kwargs` (`isParameterRejection` matches either; remembered per model; retried once before any content).
- `EstimateRequestTokens` no longer counts `ReasoningContent` (no provider in `internal/llm/provider` sends it back). `MeasuredNextRequest` = previous call's reported prompt + (output - reasoning) + estimated new tool results; used by `overflowAfterTools` when the server reported usage.
- On a real overflow after a tool round, `elideOldToolResults` replaces tool outputs > 400 chars from messages before the current turn with a sized stub, in the copy that is sent only; the turn continues with a persisted warning.
- Speed rendering: `styles.SpeedColor` (adaptive #FF3333 / #D70000, not from the theme) bold in the chat footer; a white-on-red chip in the status bar, kept on narrow screens. The speed is noted (`noteLastSpeed`) before `processEvent`, whose `TrackUsage` session save redraws the bar.
- Icon: the owner's cyan flame gorilla, rendered per size from the 2048 px source by `iconkit from-art` (alpha keyed from black outside an opaque core): winres 21 frames 16-256, the 112-240 steps added so the .exe has no gap between 128 and 256 (`rsrc_windows_*.syso` rebuilt with go-winres), assets PNG 128-1024, a 21-size .ico, the SVG's embedded master, the macOS .icns.
- `crowdedLoadoutNotice`: `config.LoadoutCrowdsTheContext` (share >= 0.33) had no caller; now announced once per session at the first coder message with `LoadoutBiggestCuts(3)`.

### Known alternatives considered

- Switching reasoning off or capping it by default on slow local models: rejected by the owner, who keeps it on to read how the model reasons.
- Shrinking the loadout automatically below a window size: rejected; the loadout is the person's choice, the notice reports it.
- Prefill tok/s from `prompt_tokens / first_ms`: not shown. LM Studio reports the whole prompt in `prompt_tokens` while serving most of it from its prefix cache, so the ratio would be wrong after the first call.

### Architecture impact

One more round-trip write of the assistant message per call (`recordSpeed` updates it after the finish). The elided history exists only in the turn's local `msgHistory`; the database is unchanged. Changing earlier messages invalidates the server's prefix cache once.

### Toolchain

Go 1.27.0 (Windows). Windows .exe and Linux amd64 binary by `fieldkit pipeline run gorilla-opencode` on Windows (modules, vet, 44 test packages passed, build, byte-identical reproducibility rebuild, checksums); tarball with the layout of `scripts/build-tarball.sh` (one member, 0755, gzip -9), checked by byte equality with the reproduced Linux binary and its version stamp instead of running it; Arch package by `arch-package.yml`. The .deb, .rpm and macOS 10.13 zip need the Linux build machine (dpkg-deb, rpmbuild, patched Go 1.24) and are added afterwards.

### Resource deltas

File sizes are in "Measured on the published files". No new dependency. Status bar line longer by the speed summary (~45 characters).

### Code changes

- `internal/message/speed.go` (new), `content.go` (`Finish.Speed`).
- `internal/llm/agent/speed.go`, `overflow.go`, `crowded.go` (new); `agent.go` (clock, overflow fallback, notice); `contextbudget.go` (`MeasuredNextRequest`, reasoning not counted).
- `internal/llm/provider/reasoning.go`, `openai.go`, `provider.go`.
- `internal/tui/components/chat/message.go`, `internal/tui/components/core/status.go`, `internal/export/transcript.go`.
- `internal/llm/tools/commandclass_test.go`: the `type` row asserts the PowerShell verdict on Windows.
- `internal/tui/styles/styles.go` (`SpeedColor`, `Speed`).
- `winres/`, `internal/assets/icons/`, `packaging/macos/gorilla.icns`, `rsrc_windows_*.syso`: new icon.

### Subsystem changes

Providers: OpenAI-compatible user endpoints receive an explicit reasoning-off. Agent: overflow decided on measured usage. TUI/export: per-call speed.

### Test coverage

- `speed_test.go`: the clock split, reasoning as first token, no invented rate. `overflow_test.go`: the refused Gemma request fits when measured; old reasoning not counted; only earlier turns lose tool output; stored messages untouched. `reasoning_off_test.go`: off sent to a local endpoint, on never sends none, rejection falls back and is remembered, cloud unchanged, reasoning tokens read from stream usage. `crowded_test.go`.
- `TestTheStatusBarHasTheSpeedWhenTheSessionIsSaved`: every session save happens with the speed already known.
- `fieldkit mutate`: 7/7 caught on PR #13, 1/1 on the commandclass rule, 2/2 on the status-bar ordering.
- Field: `run_fieldkit_mcp_scenarios.py` 1.1.0 on the PR binary with Gemma 4 12B QAT: speeds stored per call (first token 267 s cold, 11.3 s with the cache warm); "how many tools" -> `fieldkit_readiness` -> measured count.

### Security posture

Unchanged. No new network destination; the reasoning field goes to the endpoint the person configured. `fieldkit telemetry scan`: 0 findings.

### Deployment

```sh
# fetch
gh release download v0.1.152 --repo gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, tar.gz, .pkg.tar.zst, SHA256SUMS-v0.1.152.txt (.deb, .rpm, Mac zip added later)
# verify
sha256sum -c SHA256SUMS-v0.1.152.txt --ignore-missing
# Expected: each downloaded file: OK
# verify_active
gorilla-opencode --version
# Expected: v0.1.152
```

### Known issues

- Not driven end to end on a real model: the overflow fallback (unit tests only) and the crowded notice on screen.
- `reasoning_effort: "none"` measured on LM Studio only; other servers rely on the reject-and-retry path.
- .deb, .rpm and the Mac zip are not on the page at publication.
