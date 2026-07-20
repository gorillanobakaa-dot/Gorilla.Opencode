# Gorilla OpenCode — Dual-Track Documentation

**Project:** the revived original OpenCode (Go), continued as "Gorilla OpenCode"
**Repository:** https://github.com/gorillanobakaa-dot/Gorilla.Opencode
**Revival date:** 2026-07-19 / 2026-07-20
**License:** MIT (unchanged from the original)

This document follows the [Gorilla Open Source Philosophy](PHILOSOPHY.md):
every significant piece of work is explained twice — once in plain language
for any human being, once with technical precision for developers. Both
tracks cover the same facts. Neither is a summary of the other.

---

# TRACK ONE — THE HUMAN TRACK

*Plain language. No assumed knowledge.*

## What is this?

This is a program that puts an AI coding assistant inside your terminal
(the black window where you type commands). You tell it, in ordinary
English, what you want done to the files in a folder — "find the bug in
this script", "write a small program that does X" — and it reads files,
writes files, and runs commands to do it, asking your permission along
the way.

## Where did it come from? (the honest history)

In 2025 a developer named Kujtim Hoxha wrote this program and called it
**OpenCode**. Three things then happened to that name, and the
distinction matters:

1. **This code** was archived — frozen, development stopped — when its
   author joined a company called Charm.
2. At Charm, the same code was continued under a new name, **Crush**.
   Crush is bigger and more polished, but it is no longer under a fully
   free license, and by default it steers you toward Charm's paid
   AI-access service.
3. A different company (SST) released an unrelated program that reuses
   the **OpenCode** name. It shares nothing with this code but the name.

This project takes path 1 — the frozen original, whose license (MIT)
means it belongs to everyone, forever — and brings it back to life. We
call the revival **Gorilla OpenCode**: it is, quite literally, the fossil
the living species evolved from.

## Why bother?

Because the original is the last version of this program that is
genuinely free, contains no advertising for anyone's paid service, sends
no usage statistics to anyone, and is small enough that one person can
read and understand all of it. Those properties were worth rescuing.

## What did the revival actually change?

The frozen 2025 code no longer worked with the AI services of 2026. We
fixed exactly that — nothing more. In plain terms:

- **It can now talk to NVIDIA's AI service (NIM)** using your personal
  key, so you can use powerful models at NVIDIA's rates instead of
  paying a middleman.
- **It can now talk to Google's current AI models (Gemini 3).** Google
  changed the rules of conversation in late 2025 — replies now carry a
  kind of tamper-proof seal that must be shown back to Google on the
  next message. The old code didn't know about seals; now it does.
- **It no longer crashes** in two situations where the old code, upon
  receiving an error from the AI service, tripped over itself and died
  instead of telling you what the error was. Those errors are now shown
  to you in readable form.
- **It works with AI models running on your own computer** (via a free
  program called Ollama) — including newer versions of Ollama that
  rejected the old code's requests.
- **It can install itself.** Download one file, run
  `gorilla-opencode install`, and it puts itself where programs live, adds
  its icon, and appears in your applications menu like any normal app.
  `gorilla-opencode uninstall` removes every file it created — the exact
  list is printed as it happens. No hidden leftovers.

## What does it do with your data?

- Your prompts, and the contents of files the assistant reads, are sent
  to **whichever AI service you configured** (NVIDIA, Google, or your
  own machine via Ollama) — that is the entire point of the program, but
  you should know it happens.
- Your API keys are read from the environment when the program starts.
  They are **not** written into the project's files by this program.
- Conversations are saved in a small database file **on your own
  computer** (a folder named `.opencode` where you ran the program).
- It sends **no statistics, telemetry, or crash reports to anyone.**
  There is no server of ours; there is no "us" to phone home to.

## What it cannot do, and honest warnings

- **If you use Ollama with a very small model** (like a 1.5-billion-
  parameter model), the assistant will be clumsy — it may describe what
  it would do instead of doing it. That is the small model's limitation,
  not a defect in the program. Bigger models work properly.
- **Google's free tier is rate-limited.** If you use it heavily it will
  make you wait. That is Google throttling you, not the program failing.
- **The underlying libraries are from early 2025.** We updated the one
  that talks to Google, but a full security review of the rest has not
  been done yet. It is on the list, and until then you should treat this
  as software for enthusiasts, not for handling secrets.
- It is a terminal program. The desktop icon opens it **in a terminal**.
- Compared to its descendant Crush, it has fewer features (no plugin
  protocol maturity, fewer commands, simpler interface). That
  simplicity is partly the point.

## How to install (three ways, easiest first)

1. **One command** (downloads the program and it installs itself):
   `curl -fsSL https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/main/install.sh | sh`
   (or the same with `wget -qO- … | sh`)
2. **Debian/Ubuntu package:** download the `.deb` file from the
   releases page, then `sudo apt install ./gorilla-opencode_*.deb`
3. **From source** (for developers): `go build -o gorilla-opencode .`

Then set a key and run it — the program prints these exact lines after
installing:

```
NVIDIA NIM: LOCAL_ENDPOINT=https://integrate.api.nvidia.com/v1 LOCAL_ENDPOINT_API_KEY=nvapi-...
Google:     GEMINI_API_KEY=...
Ollama:     LOCAL_ENDPOINT=http://localhost:11434/v1
```

---

# TRACK TWO — THE DEVELOPER TRACK

*Technical precision. Assumes programming literacy, not project familiarity.*

## Provenance and scope

Fork of `github.com/opencode-ai/opencode` (Go, Bubble Tea TUI, sqlc/
SQLite sessions, MIT), archived 2025 when the codebase continued as
`charmbracelet/crush` (FSL-1.1-MIT). This revival treats the archive as
frozen upstream and applies a deliberately minimal patch set. Every
in-source change carries a `// GORILLA OVERRIDE:` marker stating what
changed and why — grep for it to audit the complete delta:

```
git diff 73ee493..HEAD          # full revival diff
grep -rn "GORILLA OVERRIDE" .   # every annotated change site
```

## Change inventory (verified 2026-07-19/20 on Debian 13, Go 1.26.5)

### 1. Local provider: authenticated OpenAI-compatible endpoints
`internal/llm/models/local.go`
- `listLocalModels()` used bare `http.Get`; any keyed endpoint returned
  401 and discovery silently yielded zero models. Now sends
  `Authorization: Bearer $LOCAL_ENDPOINT_API_KEY` when set.
- `providers.local.apiKey` viper default was the literal `"dummy"`; now
  defaults to `LOCAL_ENDPOINT_API_KEY` when present, so chat completions
  authenticate with the same key as discovery.
- `convertLocalModel()` hardcoded `CanReason: true`, which made the
  OpenAI-compat client send `reasoning_effort`; Ollama ≥2026 rejects
  that with 400 "does not support thinking" for non-thinking models.
  Now `false`.
- Context-window fallback for endpoints that publish no limits (Ollama
  `/v1/models`, NVIDIA NIM) raised 4096 → 32768 (conservative floor,
  not a measured limit), `DefaultMaxTokens` 8192.

Result: NVIDIA NIM works as a "local" provider —
`LOCAL_ENDPOINT=https://integrate.api.nvidia.com/v1` +
`LOCAL_ENDPOINT_API_KEY`. All 119 models the key serves register as
`local.<id>`; pin the coder agent in `.opencode.json`
(`"agents": {"coder": {"model": "local.deepseek-ai/deepseek-v4-flash"}}`).

### 2. Gemini: SDK six-generations bump + Gemini 3 protocol
`go.mod`: `google.golang.org/genai` v1.3.0 → v1.64.0 (compiled without
source changes; the SDK's iterator API is unchanged).

`internal/message/content.go`: `ToolCall` gains
`ThoughtSignature string` (base64, `json:"thought_signature,omitempty"`)
— persisted transparently through the existing JSON/SQLite session
store.

`internal/llm/provider/gemini.go`:
- Thought signatures captured on receive (`send`, `stream`, and
  `toolCalls` paths: `base64(part.ThoughtSignature)`) and decoded back
  onto replayed `functionCall` parts in `convertMessages()`. Gemini 3
  rejects replayed function calls without them (400
  `INVALID_ARGUMENT: missing thought_signature`).
- Tool results were emitted with the obsolete `"function"` role;
  genai v1.64 validates roles and the current API expects
  `functionResponse` parts in a `"user"` turn. Changed accordingly.
- `Part.Thought == true` text (Gemini 3 reasoning summaries) filtered
  out of visible content in both receive paths.
- Crash fix 1: in the stream retry path, a yielded error ends the
  iterator with `resp == nil`; the original fell through to
  `resp.Candidates` → SIGSEGV that masked the underlying API error.
- Crash fix 2: both `Chats.Create` call sites discarded the error
  (`chat, _ :=`); a nil chat segfaults inside the SDK on first use.
  Errors now surface through the provider event channel / return.

`internal/llm/models/gemini.go`: dead 2025 preview aliases
(`gemini-2.5-flash-preview-04-17`, `-pro-preview-05-06`) replaced with
rolling aliases `gemini-flash-latest` / `gemini-pro-latest`. Rationale:
versioned 2.x aliases are account-gated ("not available to new users")
and the free tier for `gemini-2.0-flash` is `limit: 0` on new accounts;
rolling aliases track whatever Google currently serves (resolved to
Gemini 3.5 Flash at verification time).

### 3. Config: operator-precedence bug (upstream)
`internal/config/config.go`: `model.CanReason && provider == OpenAI ||
provider == Local` parsed as `(a && b) || c`, forcing a reasoning-effort
default onto every local-provider model. Parenthesized to the intended
meaning.

### 4. Embedded icons + self-install (new code, no upstream equivalent)
- `internal/assets/`: `go:embed icons/*` (PNG 128/256/512/1024 +
  scalable SVG; the SVG embeds the raster — a painting cannot be
  honestly vectorized, and the file says so in a comment).
- `cmd/install.go`: `install` / `uninstall` cobra subcommands.
  User scope: `~/.local/{bin,share/icons/hicolor,share/applications}`;
  root scope: `/usr/local/…`. Copies `os.Executable()` onto PATH,
  unpacks icons, writes the desktop entry (`Terminal=true`), refreshes
  caches best-effort (`gtk-update-icon-cache`,
  `update-desktop-database`). Uninstall removes exactly the emitted
  path list and prints each removal.

## Verification method and results

Non-interactive end-to-end task (`-p "Create answer.py printing 6*7,
run it, report output" -q`), asserting on created file + executed
output:

| Provider | Model | Result |
|---|---|---|
| NVIDIA NIM | `local.deepseek-ai/deepseek-v4-flash` | ✅ file written, executed, "42" (~40 s) |
| Google AI Studio (free tier) | `gemini-2.5-flash` id → `gemini-flash-latest` → Gemini 3.5 Flash | ✅ file written, executed, "42" |
| Ollama (local) | `qwen2.5-coder:1.5b` | plumbing ✅ (discovery, chat, 16.3 tok/s); model too weak to emit tool calls — emitted them as prose |

## Known failure modes and what this code does not yet do

- **429 handling:** Google free-tier throttling triggers retry with
  server-provided backoff; on quota `limit: 0` models this looks like a
  hang (retries × long waits). No user-facing progress indication in
  non-interactive mode.
- `gemini-pro-latest` is wired but was **not** exercised end-to-end.
- Thought-signature support exists **only** in the gemini provider;
  anthropic/openai/bedrock/etc. paths are untouched 2025 code.
- **Dependency vintage:** everything except `genai` is early-2025.
  `govulncheck` audit is the top of the roadmap; until then do not
  treat this as hardened.
- Anthropic prompt caching (inherited) applies to the Anthropic
  provider only; NIM/Gemini/Ollama requests re-send full context every
  turn. Flat-rate or local providers make this a latency cost, not a
  monetary one.
- The `local` provider assumes an OpenAI-compatible `/chat/completions`
  under `LOCAL_ENDPOINT` — include the `/v1` suffix in the URL.
- Module path remains `github.com/opencode-ai/opencode`: renaming would
  churn every import for zero functional gain and blur provenance.

## Build and release recipe

```sh
go build -o gorilla-opencode .            # Go ≥ 1.24
./gorilla-opencode install                # user-scope install
scripts/build-deb.sh 0.1.0             # produces gorilla-opencode_0.1.0_amd64.deb
```

---

## Desktop launches and the key file (added in v0.1.1)

**Human track:** apps started from the applications menu do not see the
settings from your terminal, so the desktop icon used to open a window
that closed instantly — the program couldn't find any AI service and
the error vanished with the window. Now the desktop entry starts the
program through a small wrapper that (1) reads your keys from one file,
`~/.config/gorilla-opencode/env` (created for you at install time, with
instructions inside, readable only by you), and (2) if anything still
goes wrong, keeps the window open until you press Enter so you can
actually read the message.

**Developer track:** `cmd/launch.go` adds a hidden `launch` subcommand
used by the desktop entry (`Exec=gorilla-opencode launch`). It parses
KEY=VALUE lines from the env file (process env wins on conflict) and
re-executes the real binary as a child with the augmented environment —
re-exec is required because `LOCAL_ENDPOINT` is read at package-init
time, before main(). Nonzero child exit → error + hint printed, window
held open on stdin read. `install` writes the commented template 0600,
never overwriting an existing file.

# CHANGELOG (one track — written for both audiences)

## v0.1.2 — 2026-07-20 — "Package parity"

The v0.1.1 desktop-launch fix reached only the self-installer; users who
installed the **.deb** (the normal path) still got the flash-die,
because `scripts/build-deb.sh` wrote the old bare `Exec=gorilla-opencode`
and shipped no key file. Reproduced clean-room by installing the
published .deb as a fresh user and launching from the app grid.

- `.deb` desktop entry now uses `Exec=gorilla-opencode launch`, same as
  the self-installer.
- `launch` self-heals: on first run it creates the key-file template
  (`~/.config/gorilla-opencode/env`) and shows a held-open welcome
  naming the file, so a `.deb` user who never ran `install` is onboarded
  instead of flash-died. Shared `ensureEnvTemplate` removes the drift.
- Two new smoke checks assert both install paths use `launch` and that
  `launch` creates the key file — the class of bug that let the two
  paths diverge unnoticed.

## v0.1.1 — 2026-07-20 — "Community review hardening"

An independent review by another AI model (MiniMax M3, run by the
project owner) found five real defects in v0.1.0. All five are fixed,
and a smoke test (`tests/smoke.sh`) now guards each one:

- Desktop icon appeared to crash: apps launched from the menu don't
  inherit shell environment variables, so no API key was found and the
  window closed before the error could be read. Fixed with the `launch`
  wrapper + key file (see section above).
- No configured provider now prints an actionable message naming the
  exact variables to set, instead of the cryptic "agent coder not found".
- Runtime errors (bad key, unreachable endpoint) no longer dump the
  entire usage/help text after the error, which buried it — in scripts
  and CI this made failures look like usage mistakes.
- `--version` now reports the real release version. (Root cause: Go
  ≥1.22 stamps VCS pseudo-versions for plain `go build`, silently
  overriding the release stamp — precedence inverted, ldflags wins.)
- Help text said `opencode`; it now consistently says
  `gorilla-opencode`. The FZF warning was demoted to debug — it printed
  on every non-interactive run where it is irrelevant.

## v0.1.0 — 2026-07-20 — "The fossil breathes"

- Revived the archived original OpenCode; verified working coding runs
  against NVIDIA NIM (your key, NVIDIA's prices), Google Gemini 3.5
  Flash (free-tier key), and local Ollama.
- Fixed two crashes that used to hide real error messages behind a
  program abort. Errors are now shown readably.
- Taught it Google's 2026 conversation rules (thought signatures).
- The binary now contains its own icons and can install/uninstall
  itself: PATH, app-grid icon, desktop entry — one file, one command.
- Nothing added beyond that. No telemetry, no accounts, no defaults
  that cost you money.
