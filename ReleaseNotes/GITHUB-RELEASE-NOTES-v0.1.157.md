# Gorilla OpenCode 0.1.157: Mac, Windows and Linux

**Claude models answer again on an Anthropic API key, the program now says what a message will cost before you send it, and the status bar is one readable red bar.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if you use Claude with an Anthropic API key.** In 0.1.156 and earlier,
every message to a Claude model failed with `400 Bad Request: "temperature" is
deprecated for this model`. Nothing you typed got an answer. This version
answers.

**Yes, if you use any paid API key** (Anthropic, OpenAI, xAI, DeepSeek,
OpenRouter). The first time you choose one, a screen tells you what it costs
and waits until you type *I understand*.

**Everyone else:** the status bar is easier to read, and the warning that the
AI is approving actions without asking is now visible on every screen.

**Know before you update:** the Windows program is still not signed.

## Why this matters to you

The owner upgraded the program on an 11-inch MacBook Air with an Anthropic key
holding $99.80, typed *comms check*, and got an error. Every model, every
message. Anthropic stopped accepting a setting (`temperature`) that the program
sent with every request; since 21 October 2026 it refuses it on all models.

[![Before, on version 0.1.156: comms check sent to Claude Sonnet 5 fails with 400 Bad Request, temperature is deprecated for this model, and the status bar has a black gap](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.157/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-28-before-comms-check-400-black-gap.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.157/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-28-before-comms-check-400-black-gap.png)

The same words on this version:

[![After, on this version: comms check answered by Claude Sonnet 5 in under a second for 5 cents, and the status bar red from end to end](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.157/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-28-after-comms-check-answered.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.157/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-28-after-comms-check-answered.png)

### An API key is a bank card

An API key bills a card for every message, and every message pays again for
the whole conversation so far. The first time you pick a paid provider you now
see this, with the most each Claude model can cost per message and per hour:

[![The paid API key warning: your card pays for every message, a table of the most each Claude model can cost per message in a new chat and a long chat and per hour of the AI working alone, the free options with no card, and a prompt to type I understand](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.157/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-28-after-api-key-cost-warning.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.157/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-28-after-api-key-cost-warning.png)

After that, each session on a paid key opens with one red line, and the status
bar shows what the next message can cost. For this *comms check* it said "up to
$0.055"; Anthropic billed $0.0493.

[![A session on an Anthropic key: a red line saying the card pays for every message and Claude Sonnet 5 costs up to 5.5 cents for the first message, and the status bar showing next up to 0.055 dollars](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.157/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-28-after-api-key-start-line-and-next-cost.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.157/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-28-after-api-key-start-line-and-next-cost.png)

### The status bar

The empty part of the bar was black, and on the Mac the chip that says *AUTO -
approving without asking* was drawn black on black: invisible. The bar is now
red from end to end, the chip is bold white, and messages appear in fixed
colours that stay readable (white on dark blue, black on yellow, white on black
for errors).

[![The status bar at rest on the MacBook Air: red from the help chip to the blue model name, with AUTO approving without asking and No diagnostics readable in bold white](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.157/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-28-after-red-bar-auto-readable.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.157/docs/screenshots/gallery/macos-air-2026-10/air-2026-10-28-after-red-bar-auto-readable.png)

## Also fixed

- Conversation titles were never made on current Claude models ("failed to
  generate title"), and summaries of long conversations would have failed the
  same way.
- Messages containing the word *think* asked current Claude models for thinking
  in a form they refuse.
- `/usage` in an Anthropic session talked about Google Antigravity. It now names
  Anthropic and says where the balance is (console.anthropic.com, Billing): an
  API key cannot read its own balance.

## Files

| File | Bytes | SHA-256 |
|---|---|---|
| `gorilla-opencode-v0.1.157-linux-amd64` | 55,324,856 | `f3b697bb008f8b706b4e5d06d77c61644f895434d574189ad051a5257c6179bf` |
| `gorilla-opencode.exe` | 84,823,552 | `06ca66b48f9c70085324a191e3344ada7cd4981a4c83ff8fc551ef4d14dfa3a2` |
| `gorilla-opencode-0.1.157-mac-10.13-x86_64.zip` | 24,377,087 | `c59e22cf39da4c3b496e711ba54fcdac40d7d2f9b59863117efee99c6d575c13` |
| `gorilla-opencode-linux-x86_64.tar.gz` | 21,595,519 | `11ab9db66234e37a3c647ea3a5a366fb2490578a760c89249c961a7ca1e218b0` |
| `gorilla-opencode-0.1.157-1.x86_64.rpm` | 25,223,736 | `19d0871e3c3f098a002ecd76c55a881eda7e12adf4d3a002a6f266662ead4e59` |
| `gorilla-opencode_0.1.157_amd64.deb` | 17,512,536 | `699e53be0b13cdb349fb5cabbad10b48add983a2966a3b74a0ab2cff6bc8be9f` |
| `gorilla-opencode-0.1.157-1-x86_64.pkg.tar.zst` | 26,822,313 | `aa8d6fc8f848a1491c098e3e9205139fc8dc07b947449766c2033a553c42eb1f` |

Check a download: `sha256sum -c SHA256SUMS-v0.1.157.txt --ignore-missing`. The Arch package was built by the arch-package workflow from `packaging/PKGBUILD`.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why this release exists

On the morning of 28 October the owner upgraded Gorilla OpenCode on an 11-inch
MacBook Air, signed in with an Anthropic API key, and typed *comms check*. The
answer was an error. Every Claude model gave the same one, for every message.

### What was wrong

1. **Claude refused every message.** The program sent a setting called
   `temperature` with each request. Anthropic stopped accepting it on its newer
   models, and since 21 October 2026 refuses it on all of them. The program
   now leaves it out.
2. **A message with the word "think" in it** asked the newer models to think in
   an older way they refuse. It now asks the way they accept.
3. **Conversation titles failed** ("failed to generate title"). When you change
   model, the small helper that writes the title took on the new model's full
   output limit, and the library that talks to Anthropic refuses a request
   that large unless it is streamed. The title now keeps its small limit, and
   such requests are kept under the size the library accepts. Summaries of long
   conversations had the same problem and are fixed with it.
4. **`/usage` talked about the wrong company.** In an Anthropic session it
   only said that Google Antigravity was not in use. It now names Anthropic.
   It cannot show your balance: Anthropic does not let an API key read it. It
   says where to find it: console.anthropic.com, under Billing.

### New: what an API key really costs

An API key is tied to a bank card. Every message is paid for, and every
message pays again for everything already in the conversation. A short reply
adds little. A file the AI reads adds its whole size to every message after
it. A pause of more than five minutes makes the next message cost full price
again.

- **The first time you choose a paid provider** (Anthropic, OpenAI, xAI,
  DeepSeek, OpenRouter) a screen explains this. For Claude it shows the most
  each model can cost: per message at the start of a conversation, per message
  in a long one, and per hour of the AI working on its own. For example Claude
  Sonnet 5: 5.5 cents, 51 cents, 121 dollars. You type *I understand* to go on.
  It is asked once per provider.
- **Every session on a paid key** opens with one red line naming the model and
  what the first message can cost.
- **The status bar** shows what the next message can cost ("next up to
  $0.055"), and after a minute how fast the session is spending ("$0.10/h").

These are estimates at list price, and they are the most a message can cost:
the program saves the repeated part of the conversation for five minutes, so a
quick reply usually costs less. Two real bills on the Air (4.85 and 4.93
cents) were both under the 5.5 cents shown. For providers whose prices the
program does not have, the screen says so instead of guessing.

### The status bar

The empty part of the bar was black. On the Mac the chip that says *AUTO -
approving without asking* was drawn black on black, so you could not see that
the AI was acting without asking. The bar is now red from end to end, with that
chip in bold white, and messages appear in colours chosen to be read: white on
dark blue, black on yellow, and white on black for errors (so an error never
looks like the bar at rest).

### What was checked

On the MacBook Air: the old version refused *comms check*, this one answered
it. The warning screen, the red start line, the "next up to" figure and the
bar were photographed. Testing used about 20 cents of the owner's key ($99.80 before, $99.65 at 09:51, then one more 4.93-cent message).

Not checked: OpenAI, xAI, DeepSeek and OpenRouter keys with this version (their
warning has no prices); a real long conversation with the spending pace; the
bar on Windows and Linux screens (same code, not photographed).

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

- `internal/llm/provider/anthropic.go`: `preparedMessages` no longer sets `Temperature` (was 0, 1 with thinking). Evidence: `400 invalid_request_error "temperature is deprecated for this model"` on every turn, v0.1.152 and v0.1.156, Opus 5.5, Fable 5.1, Sonnet 5 (owner's screenshots, 2026-10-28); Anthropic rejects temperature/top_p/top_k on every model since 2026-10-21.
- Thinking: `anthropicThinking(apiModel, maxTokens)` sends `{"type":"adaptive"}` via `param.Override` (SDK v1.4.0 has no adaptive type); `budget_tokens` kept only for `legacyBudgetThinking` prefixes (claude-3, 4.0, 4.1, 4.5).
- Non-streaming cap: `capNonStreaming` sets `max_tokens` to 16,000 when larger (`send()` is used by title and summarizer only). The SDK's `CalculateNonStreamingTimeout` refuses above ~21,333 ("streaming is strongly recommended"). `config.UpdateAgentModel` keeps the title agent at 80 tokens when it follows the coder (it took `DefaultMaxTokens`, 64K+).
- `/usage`: `anthropicUsageLine` when the coder is Anthropic; "nothing sent yet, $0.00 spent" when no session row exists.
- Status bar (`components/core/status.go`): idle fill, mode chips (AUTO/EDITS/PLAN, long and short) and diagnostics chip on `styles.SpeedColor` with bold white; info/warn/error messages on fixed `StatusInfoBg` #1E3A8A (white), `StatusWarnBg` #FFD400 (black), `StatusErrorBg` #000000 (white). The theme's `Error()`/`Background()` pair rendered black on black on Terminal.app 10.13.
- Paid-key warning: `ProviderRow.CostWarning/CostAccepted`, `ProviderChoice.CostAccepted`, acking state in `startup/provider.go` (exact phrase "I understand", case-insensitive; Esc back). Persisted as `providers.<id>.costWarningAccepted`. Text and table in `cmd/cost_warning.go`; rows for anthropic, openai, xai, deepseek, openrouter.
- Forecast (`internal/llm/cost/forecast.go`): `MessageCost = ctx*in*1.25/1e6 + 500*out/1e6` (1.25 = 5-minute cache write, so the figure is an upper bound: measured 0.0485 and 0.0493 against 0.055 for 19.4K/19.6K on Sonnet 5); `WorkingHourCost` = long (200K) message x 4 calls/min x 60. Rates from `prices.json` via `ListRates` (anthropic and gemini tables only).
- Status chip: `paidForecast` appends "next up to $X" (context floored at 20K) and "$Y/h" (session cost / age, after 1 minute). Session start: `paidKeyStartLine` as an error-kind quota line.

### Test coverage and CI

Locally only the touched packages: `internal/llm/provider`, `internal/config`, `internal/tui`, `internal/tui/components/core`, `internal/tui/styles`, `internal/tui/startup`, `internal/llm/cost`, `cmd`, `internal/version`, `internal/tui/inline`. Every new test was run against the old code and failed (temperature: 8/8 cases; status bar: 3 + 1; title limit; non-streaming cap; /usage). CI on PR #39 head 32dd918: build-test, build-test-windows, toolbox-windows, all pass.

### Security posture

No new network code, dependency or downloaded program. The cost warning reads only the embedded price table. No key is displayed or logged.

### Deployment

```
sha256sum -c SHA256SUMS-v0.1.157.txt --ignore-missing
gorilla-opencode --version
# Expected: v0.1.157
go version -m gorilla-opencode.exe | grep tags
# Expected: build	-tags=toolbox
```

### Not verified

- OpenAI, xAI, DeepSeek, OpenRouter with this version; their warning has no table (no prices in `prices.json`).
- `$/h` pace in a real long session; adaptive thinking on a real "think" message beyond one headless run on the Air.
- The summarizer path on a real long Anthropic conversation (only the cap is unit-tested).
- Status bar colours on Windows Terminal and Linux terminals (photographed on Terminal.app 10.13 only).

### Known issues

- `costWarningModels` in `cmd/cost_warning.go` is a hand list beside `prices.json`; a new Claude model needs a line there.
- Carried: the Windows exe is unsigned.

<!-- helpers:begin -->

### What is built in

Carried inside the program and checked by hash before every run. Nothing to install. A dash means that system does not carry it. On macOS, ffmpeg and ffprobe are 8.0 and need macOS 11 (third_party/PINS.md).

| Helper | Version | Windows | Linux | macOS |
|---|---|---|---|---|
| 7za | 26.04 | yes | yes | yes |
| ast-grep | 0.50.0 | yes | yes | yes |
| avifenc | v1.4.2 | yes | yes | yes |
| coreutils | 0.12.0 | yes | yes | yes |
| cwebp | 1.6.0 | yes | yes | yes |
| difft | 0.71.0 | yes | yes | yes |
| ffmpeg | 7.0.2 | yes | yes | yes |
| ffprobe | 7.0.2 | yes | yes | yes |
| hyperfine | v2.0.0 | yes | yes | yes |
| ninja | v1.13.2 | yes | yes | yes |
| oxipng | v10.2.1 | yes | yes | yes |
| pfind | 4.0.0 | yes | yes | yes |
| sd | v1.1.0 | yes | yes | yes |

Not carried, but known: `/arsenal` finds them on your machine and prints the line to install them.

| Program | Windows | Linux | macOS |
|---|---|---|---|
| Read text out of images and screenshots (OCR) (tesseract) | `winget install UB-Mannheim.TesseractOCR` | `sudo apt install tesseract-ocr tesseract-ocr-eng` | `brew install tesseract` |
| Prepare an image so OCR can actually read it (imagemagick) | `winget install ImageMagick.ImageMagick` | `sudo apt install imagemagick` | `brew install imagemagick` |
| PowerToys: screen text grabber, window layouts, file search (powertoys) | `winget install Microsoft.PowerToys` | not available on Linux | not available on macOS |

<!-- helpers:end -->
