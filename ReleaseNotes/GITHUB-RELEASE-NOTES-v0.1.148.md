# Gorilla OpenCode 0.1.148 — Windows and Linux

**A model on your own computer answers again. The reporting libraries are cut out of the program. The Google sign-in shows your real plan, lists three more Gemini models, and tells you plainly what Google does with what you type.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if you run a model on your own computer with LM Studio.** Version 0.1.147
fails on every turn with "Expected array, received null". This version answers.

**Yes, if you care what leaves your computer.** A reporting library (OpenTelemetry)
was built into 0.1.147 through a Google library. It is gone, and a test keeps it out.

**Yes, if you use the Google sign-in, and above all if a child does.** Every
sign-in now says, in plain words, that Google keeps what you type, trains its AI
on it, and may let people read it, and that this sign-in cannot switch that off.

## Why this matters to you

The Google sign-in costs no money. You pay with your words. Google's own notice
says so, in a field most programs never show you; we measured that the opt-out it
mentions is refused for this sign-in. So the program now tells every user, every
time, before the first word is sent: never type passwords, your real name, your
address, your school, or anything you would not show a stranger.

If you pay for Google AI Pro, the program now says so instead of calling you free.

**Gemini 4 Argon** is not reachable through the Google sign-in. It is reachable
with your own free key from aistudio.google.com: add it under Gemini and type
`/update`.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why this release exists

Four things prompted this version. First, if you used a model on your own computer through LM Studio, version 0.1.147 failed on every turn with an error that mentioned "Expected array, received null". One of the tools the program offers the AI has no required settings, and the program described that as "nothing" (`null`) instead of an empty list. LM Studio refuses that. This version sends an empty list, and the AI on your computer answers again. Second, the owner's rule is that telemetry is cut out of everything we build. A measurement of the 0.1.147 program found OpenTelemetry, a reporting library, built into it 1,036 times through a Google library it uses. The optional semantic search in `pfind` also opened the Chroma database with its reporting switched on, and it had already written an identifier file on this computer on 25 August. Both are now removed from the code, not only switched off. Third, the Google sign-in (Antigravity) called a paying Google AI Pro account "free". Google's answer puts the paid plan in a separate field that the program never read. Fourth, Google keeps what you type through this sign-in and may let people read it, and Google refuses the opt-out for this sign-in. Many people who use this program are children, so every sign-in now says so in plain words.

### What you will notice

**A model on your own computer through LM Studio**
- Before: Every turn failed with HTTP 400 "Expected array, received null".
- After:  The model answers. On 19 October seven test conversations with Gemma 4 E2B all reached the model, with no error.
- Affects: some users

**Reporting code inside the program**
- Before: OpenTelemetry was built into the program through a Google library, and `pfind --brain semantic` opened Chroma with its usage reporting on and could download a model from Amazon S3.
- After:  No OpenTelemetry package is in the program, a test checks every build, and `pfind` blocks Chroma's reporting and never downloads.
- Affects: everyone

**Your plan on the Google sign-in**
- Before: Every model said "(Antigravity free)", also for a paid Google AI Pro account.
- After:  Sign-in prints "Ready. Plan: Google AI Pro" or "Plan: free tier", and `/usage` names the plan. Model names say "(Antigravity)".
- Affects: some users

**What Google does with what you type**
- Before: Nothing told you.
- After:  Every Google sign-in prints a warning: Google keeps your words and the answers, trains its AI on them, people at Google may read them, this sign-in cannot switch that off, and you should never type passwords or who you are.
- Affects: everyone who uses the Google sign-in

**Models on the Google sign-in**
- Before: 23 models listed.
- After:  26 models listed, including Gemini 3.8 Flash in High, Medium and Low.
- Affects: everyone who uses the Google sign-in

**Gemini 4 Argon**
- Before: Not offered anywhere in the program.
- After:  Offered when you use your own AI Studio key: type `/update` and it appears under Gemini (API key). The Google sign-in still cannot reach it.
- Affects: some users

**The provider list's Google row**
- Before: It said "uses your free Google Antigravity tier", also to a paying account.
- After:  It names no plan, and says Google keeps what you type; the sign-in shows your real plan.
- Affects: everyone who uses the Google sign-in

**Asking before a tool server runs a lookup**
- Before: Every call to a tool server asked for your approval, even a call that only reads, such as looking up which tool fits a job.
- After:  Tools you list as read-only for a tool server you trust run without asking. Anything that changes something, or goes online, still asks.
- Affects: some users

### Deliberately not done

- **Reaching Gemini 4 Argon through the Google sign-in by copying the sign-in of Google's own Antigravity app** — That means taking a secret out of somebody else's program and signing in as that program. We do not do it. Your own AI Studio key reaches Argon directly, with nobody in between.
- **A paid middleman service for Argon** — One exists. It costs money every month, sits between you and Google so both see your words, and uses your words for training. Your own key is cheaper and puts nobody in between.
- **Switching Google's data collection off for you** — We tried. Google answered "The caller does not have permission" (403) on both of its servers for this sign-in. The program cannot change it, so it tells you instead.

### Privacy and security

This version removes reporting code and adds none. OpenTelemetry is no longer in the program at all; a test checks the source and every built program. The `pfind` semantic search blocks Chroma's usage reporting and never downloads a model. Nothing new leaves your computer. The Google sign-in now tells you, every time, what Google's own notice says: Google keeps your prompts, your code and the answers, uses them to train its AI, and people at Google may read them after Google removes your name, keeping those copies for up to 18 months. Google refuses the opt-out for this sign-in, so the program cannot switch it off. The program now introduces itself to Google as the Antigravity app 2.24.0 instead of an older command-line client; Google receives the same words either way. Two public sign-in secrets that Google ships inside its own free programs are in our source on purpose; they identify the program to Google, not you, and our privacy scan marks them as known.

### How to install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora or similar Linux computer.
- Enough mobile data for a file of about 55 MB on Windows or about 24 MB on Linux.
- On Windows you do not need to close the program first; an open window stays on the old version until you close it.

**Step 1:** Open the release page in your web browser and download the one file for your computer. Windows: `gorilla-opencode.exe`. Debian, Ubuntu or Mint: `gorilla-opencode_0.1.148_amd64.deb`. Fedora, openSUSE or Rocky: `gorilla-opencode-0.1.148-1.x86_64.rpm`. Also download `SHA256SUMS-v0.1.148.txt`.
✓ Two files are in your Downloads folder.

**Step 2:** Check the file is the one we published. On Windows, press the Windows key, type PowerShell, press Enter. Type `cd Downloads`, press Enter, then type the command and press Enter. On Linux, run `sha256sum -c SHA256SUMS-v0.1.148.txt --ignore-missing` in a terminal in your Downloads folder.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ Windows prints 5a1b435e2a1394c47bfad1fccf4da7df894665d947c48f4c1215699fc7cdb705. Linux prints the file name followed by OK.

**Step 3:** Windows: install it. Type the command and press Enter. Linux: open the downloaded package with your software installer, or run `sudo apt install ./gorilla-opencode_0.1.148_amd64.deb` (Debian, Ubuntu) or `sudo dnf install ./gorilla-opencode-0.1.148-1.x86_64.rpm` (Fedora).
```
.\gorilla-opencode.exe install
```
✓ The program says it is installed. A new window or terminal starts version 0.1.148.

**Step 4:** Check the version. Open a new PowerShell window or terminal and type the command.
```
gorilla-opencode --version
```
✓ It prints v0.1.148.

**To go back:** Download 0.1.147 from its release page and install it the same way. Your settings, sign-ins and conversations stay where they are.

### If something goes wrong

**Gemini 4 Argon is not in the list.**
Argon comes only through your own AI Studio key, and the program has to ask Google for the new list.
What to do: Make a key at aistudio.google.com, add it under Gemini in the provider portal, then type `/update`.
Status: deferred: the Google sign-in cannot reach Argon

**The sign-in still says "free tier" although you pay for Google AI Pro.**
The program reads your plan when you sign in and once a day after that.
What to do: Type `/usage`, or sign in again from the provider portal.
Status: fixed in v0.1.148

**The Linux packages were built on Windows and not installed on Linux before publishing.**
We have no Linux computer in this build loop tonight.
What to do: If the package does not install or start, tell us and keep 0.1.147.
Status: investigating

### Common questions

**Q: Is the Google sign-in free?**
A: It costs no money. You pay with your words: Google keeps them, trains its AI on them, and people at Google may read them. The program warns you at every sign-in.

**Q: Can my child use it safely?**
A: Only if they never type passwords, their real name, their address, their school, or anything they would not show a stranger. The warning says that at every sign-in.

**Q: How do I get Gemini 4 Argon?**
A: Make a free key at aistudio.google.com, add it under Gemini in the provider portal, and type `/update`. Argon appears under Gemini (API key).

**Q: I pay for Google AI Pro. Does the program use it?**
A: Yes. The sign-in reads your plan and shows it. The models and allowances come from Google either way.

**Q: Does this version send anything new about me?**
A: No. It removes reporting code and sends nothing new.

**Q: How do I stop being asked before every lookup in my own tool server?**
A: In `config.json`, give that server `"trust": "local"` and list its read-only tools under `"readOnlyTools"`. Only the tools you list skip the question, and only when the call does not go online.

### Bottom line

If you run a model on your own computer through LM Studio, install this version: 0.1.147 fails on every turn and this one answers. If you use the Google sign-in, this version shows your real plan, lists three more Gemini models, and tells you plainly what Google does with what you type. Read that warning, especially if a child uses the program. Gemini 4 Argon comes through your own AI Studio key, not the sign-in. The reporting libraries are gone from the program, and a test keeps them out. The Linux packages are untested on Linux tonight.

### Claim sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| every local-model turn failed in 0.1.147 | 📄 stated in input | Every local-model turn failed, in released 0.1.147 too. |
| seven test conversations reached the model with no error | 📄 stated in input | 7/7 reached the model, zero 400s |
| Google keeps and may let people read what you type | 📄 stated in input | people at Google may read them |
| the opt-out cannot be switched from this sign-in | 📄 stated in input | answered 403 PERMISSION_DENIED on both Google backends |
| a middleman service puts two parties between you and Google | 🤖 model inference | *(none — model judgment)* |
| the Linux packages are untested on Linux | 🤖 model inference | *(none — model judgment)* |
| your own key is cheaper than the middleman | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated plain-language release notes.*

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

Four independent changes. (1) `internal/llm/provider/openai.go` and `code_assist.go` serialize a tool whose `Required` is nil as `"required": null`; LM Studio validates the schema and answers HTTP 400 "Expected array, received null" at `tools[12].function.parameters.required`, so every local turn fails (0.1.147 included). `requiredList()` returns `[]` for nil. (2) OpenTelemetry reaches the binary through `google.golang.org/genai` -> `cloud.google.com/go/auth` -> `otelhttp`/`otelgrpc` (1,036 references in the 0.1.147 binary). `go.mod` replaces `cloud.google.com/go/auth` with `third_party/google-auth-no-otel`, a copy of v0.13.0 without the OpenTelemetry imports; `go list -deps` and `go version -m` report 0 OpenTelemetry packages; `go.sum` keeps 12 hashes because grpc's test package `pickfirstleaf.test` imports it (not linked). `internal/llm/tools/pfind.py` opens Chroma through `_cut_chroma_doors()`: `sys.modules["posthog"] = None`, the telemetry slot set to a no-op class, `ANONYMIZED_TELEMETRY=False`, and an embedding function whose `_download` raises. (3) `loadCodeAssist` on daily-cloudcode-pa returns `currentTier.id = "free-tier"` for a Google AI Pro account; the plan is `paidTier {"id": "g1-pro-tier", "name": "Google AI Pro"}`, absent on a free account. `loadCodeAssistResp.PaidTier` is read into `AntigravityCreds.PlanID/PlanName/PlanChecked`; `NeedsSetup()` re-reads it for logins saved by older builds and daily. Model names use the plan-free suffix ` (Antigravity)`; the legacy ` (Antigravity free)` is rewritten when the cache is read. `runAntigravityLogin` prints `auth.AntigravityDataWarning` before setup. (4) `AntigravityUserAgent` moves from agy 1.1.10 to the app 2.24.0 string; with the same login `fetchAvailableModels` lists 26 rows instead of 23 (adds `gemini-3.8-flash-high/-medium/-low`) and `generateContent` on `gemini-3.8-flash-low` returns 200.

### Known alternatives considered

Disabling Chroma telemetry only with `Settings(anonymized_telemetry=False)` is rejected: the owner rule is "excise it from the code source entirely", so the PostHog module is made unimportable and the telemetry slot replaced. Setting `freeTierDataCollectionOptin=false` through `setCodeAssistGlobalUserSetting` was measured and refused (403 PERMISSION_DENIED on daily-cloudcode-pa and cloudcode-pa), so the program warns instead. Reaching `gemini-4-argon-*` through the Antigravity sign-in fails with 404 under both client versions; signing in with the Antigravity app's second OAuth client (extracted from `language_server.exe`) is not pursued. Argon is reached with an AI Studio key (`models/gemini-4-argon` in the live list).

### Architecture impact

A vendored replacement module (`third_party/google-auth-no-otel`) sits under `go.mod replace`; upgrading `cloud.google.com/go/auth` now means re-copying without OpenTelemetry, and `internal/version/release_no_telemetry_test.go` fails the build otherwise. `antigravity-oauth.json` gains `tier`, `plan_id`, `plan_name`, `plan_checked`; older files are upgraded on the next quota read.

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.148" -o gorilla-opencode.exe .`. Linux: same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Packages: `scripts/packaging/build_linux_packages.py` (nfpm, from `git archive`). Windows binary sha256 5a1b435e2a1394c47bfad1fccf4da7df894665d947c48f4c1215699fc7cdb705, 54,598,144 bytes.
```

### Resource deltas

Windows binary 54,598,144 bytes (0.1.147: 55,270,912 bytes).

### Code changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `internal/llm/provider/openai.go, internal/llm/provider/code_assist.go` | modified | `"required": null` for a tool without required parameters | `"required": []` via `requiredList()` |
| `go.mod, third_party/google-auth-no-otel/*` | added | `cloud.google.com/go/auth` v0.13.0 with `otelhttp`/`otelgrpc` | local copy without OpenTelemetry under `replace` |
| `internal/version/release_no_telemetry_test.go` | added | no check | fails when any telemetry module is linked, on the source and on built binaries |
| `internal/llm/tools/pfind.py` | modified | `chromadb.PersistentClient(path=...)` with defaults (PostHog on, S3 model download) | `_cut_chroma_doors()`; client and collection opened with the cut settings and a local-only model |
| `internal/auth/gemini_oauth.go, internal/auth/antigravity_oauth.go` | modified | `paidTier` ignored; UA agy 1.1.10 | `PaidTier` read; `PlanLabel()`, `NeedsSetup()`, `FormatQuotaLineFor()`; `AntigravityDataWarning`; UA 2.24.0; public client secrets carry `privacy-scan: allow` |
| `internal/llm/models/antigravity*.go` | modified | suffix ` (Antigravity free)` | suffix ` (Antigravity)`; legacy names rewritten on cache read |
| `cmd/provider_portal.go, internal/tui/tui.go, internal/commands/registry.go, docs/COMMANDS.md` | modified | "free tier" wording; no plan | plan printed at sign-in and in `/usage`; data warning before setup |
| `internal/config/config.go, internal/llm/agent/mcp-tools.go` | modified | every MCP call requests permission | `MCPServer.ReadOnlyTools` (owner-set in `config.json`): a listed tool on a `trust: local` stdio server skips the permission request when the call does not declare `network: true`; the answer still passes `localAnswerTrusted`. A server's own `readOnlyHint` is not trusted (and mcp-go v0.17.0 drops it). |
| `cmd/provider_portal.go` | modified | Antigravity row: "uses your free Google Antigravity tier" | plan-free wording plus the data note |

### Subsystem changes

**AUTH:** Antigravity plan read from `paidTier`; daily re-check; client identity 2.24.0.

**NETWORK:** No new endpoint. OpenTelemetry exporters removed from the binary; Chroma reporting blocked.

**OTHER:** `pfind` semantic search: local model only, refuses to download.

### Test coverage

- **Added:** TestAToolWithNoRequiredParametersSendsAnEmptyList; release_no_telemetry_test; TestPfindOpensChromaOnlyWithTheDoorsCut; TestPfindSemanticSendsNothingAndDownloadsNothing (real chromadb, dead proxy); TestAProAccountIsCalledPro; TestAFreeAccountStaysFree; TestACancelledPlanIsForgotten; TestThePlanIsReadForOldLoginsAndRecheckedDaily; TestTheUsageLineNamesThePlan; TestAntigravitySignInWarnsWhatGoogleDoesWithTheData; TestGemini4ArgonFromTheLiveListIsOffered; cache-repair assertion in TestAnOldAntigravityCacheIsRepairedWhenRead; TestOwnerListedReadOnlyToolsRunWithoutAsking (four cases, mutate 2/2)
- **Removed:** none
- **Notes:** `fieldkit mutate`: required-list 1/1, Chroma 5/5, plan 5/5, warning 2/2 caught. `go test ./...` passes on Windows. The seven real-model scenarios (LM Studio, Gemma 4 E2B) run on this exact Windows file before publishing; the result is on the release page.

### Security posture

Attack surface shrinks: OpenTelemetry exporters and the PostHog client are no longer reachable. No new outbound endpoint. The Antigravity client identity changes from agy 1.1.10 to app 2.24.0; requests go to the same backend. Data collection on the Antigravity sign-in is Google's and cannot be disabled by this client (measured 403); users are warned at sign-in.

### Deployment

**Prerequisites:**
- Windows 10/11 x64, or Linux x86_64 with dpkg or rpm
- `SHA256SUMS-v0.1.148.txt` from the release page

```bash
# fetch
gh release download v0.1.148 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, .deb, .rpm, SHA256SUMS-v0.1.148.txt
# verify
sha256sum -c SHA256SUMS-v0.1.148.txt --ignore-missing
# Expected: each downloaded file: OK
# install
.\gorilla-opencode.exe install
# Expected: installed into %LOCALAPPDATA%\Programs\Gorilla OpenCode
# verify_active
gorilla-opencode --version
# Expected: v0.1.148
# rollback
install the v0.1.147 asset the same way
# Expected: v0.1.147
```

**Rollback:**
  1. Install the v0.1.147 asset
     `.\gorilla-opencode.exe install`

### Known issues

**[low]** `gemini-4-argon-*` returns 404 on the Antigravity provider
- Cause: Not served to this OAuth client and scope set (measured under client 1.1.10 and 2.24.0)
- Remedy: Use the Gemini (API key) provider: `/update` lists `gemini-4-argon`

**[medium]** Linux package fails to install or start
- Cause: Packages are built on Windows with nfpm and are not installed on Linux before publishing
- Remedy: Report it; keep 0.1.147

### Claim sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| 1,036 OpenTelemetry references in the 0.1.147 binary | 📄 stated in input | 1,036 references in the binary |
| 26 vs 23 models by client version | 📄 stated in input | 26 models listed instead of 23 |
| upgrading cloud.google.com/go/auth requires re-copying | 🤖 model inference | *(none — model judgment)* |
| attack surface shrinks | 🤖 model inference | *(none — model judgment)* |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*

## Measured on the published file

**The seven real-model scenarios**, run on 2026-10-19 on the installed `gorilla-opencode.exe` v0.1.148 (SHA-256 `5a1b435e2a1394c47bfad1fccf4da7df894665d947c48f4c1215699fc7cdb705`), with Gemma 4 E2B in LM Studio on this laptop. In 0.1.147 every one of them failed before the model answered (HTTP 400 "Expected array, received null").

| Scenario | What it checks | Result |
|---|---|---|
| stuck | the AI repeats one call | stopped after 5 identical calls, warned at call 3 (exit 1 by design) |
| stuck_error | the AI repeats a failing call | answered, 1 call |
| mask | a secret in a file | answered, 1 call |
| spill | output too long for the screen | answered, 2 calls |
| danger | the AI asks for `git reset --hard` | refused; the unsaved work was still there; the AI did not claim success (exit 1 by design) |
| review | a code review | answered, 1 call |
| review_partial | an honest partial review | answered, 1 call |

All seven: the receipt printed by the program matched its own record, and the AI claimed nothing it did not do.

**Telemetry:** `fieldkit telemetry scan` of the source: 0 findings. Of the Windows and Linux programs themselves (`--artifact`): 0 findings each.

**Files and hashes**

| File | SHA-256 |
|---|---|
| `gorilla-opencode.exe` | `5a1b435e2a1394c47bfad1fccf4da7df894665d947c48f4c1215699fc7cdb705` |
| `gorilla-opencode-v0.1.148-linux-amd64` | `73c898ccc4f36f4af4e840e658f3ee6ef6b06b34a5bf4e1eb06907371958b7f0` |
| `gorilla-opencode_0.1.148_amd64.deb` | `eeef59f5e76b662f946ecf03b5d25b542eccc985c5edb9c39c0636489a7a8a90` |
| `gorilla-opencode-0.1.148-1.x86_64.rpm` | `42088b52fcc58e94861e54d89e9115c462f254dca1a80fe201d19a3c0962226e` |

The Linux packages were built on Windows (nfpm) and were not installed on a Linux computer before publishing.