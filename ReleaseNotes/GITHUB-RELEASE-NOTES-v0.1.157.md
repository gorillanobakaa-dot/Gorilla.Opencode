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

Plain-language notes: `v0.1.157-release-notes.layman.md`. Developer notes:
`v0.1.157-release-notes.developer.md`. Both are attached.
