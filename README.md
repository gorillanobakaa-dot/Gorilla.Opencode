<p align="center"><img src="internal/assets/icons/gorilla-opencode-256.png" width="128" alt="Gorilla OpenCode"></p>

# Gorilla OpenCode

**The original OpenCode, revived.** A terminal AI coding agent — MIT
licensed, no telemetry, no accounts, no vendor funnel. Bring your own
API keys or run models on your own machine.

> **Provenance, stated plainly:** this is the original Go OpenCode by
> [Kujtim Hoxha](https://github.com/kujtimiihoxha), archived in 2025
> when its development continued as [Crush](https://github.com/charmbracelet/crush)
> under Charm (FSL license). It is unrelated to
> [SST's opencode](https://github.com/sst/opencode), which reuses the
> name. This fork revives the archived MIT original — the fossil the
> living species evolved from — and keeps it working with
> the AI providers of 2026. The full reasoning, and everything that was
> changed, is documented for both humans and developers in
> [DOCUMENTATION.dual-track.md](DOCUMENTATION.dual-track.md), per this
> project's [Open Source Philosophy](PHILOSOPHY.md).

## Install

**One command** (the binary installs itself: PATH, icons, desktop entry):

```sh
curl -fsSL https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/main/install.sh | sh
# or:  wget -qO- https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/main/install.sh | sh
```

**Debian / Ubuntu package** — from the [releases page](../../releases):

```sh
sudo apt install ./gorilla-opencode_*_amd64.deb
```

**From source:**

```sh
go build -ldflags "-X github.com/opencode-ai/opencode/internal/version.Version=vX.Y.Z" -o gorilla-opencode .   # Go ≥ 1.24
./gorilla-opencode install       # optional: icons + desktop entry, no sudo
```

`gorilla-opencode uninstall` removes exactly what `install` created.

## Use

```sh
# NVIDIA NIM (your key, NVIDIA's prices)
LOCAL_ENDPOINT=https://integrate.api.nvidia.com/v1 \
LOCAL_ENDPOINT_API_KEY=nvapi-... gorilla-opencode

# Google AI Studio (Gemini 3, free tier works)
GEMINI_API_KEY=... gorilla-opencode

# Local models via Ollama (no key, no cloud)
LOCAL_ENDPOINT=http://localhost:11434/v1 gorilla-opencode
```

Non-interactive: `gorilla-opencode -p "your task" -q`. Pin models per
project in `.opencode.json`:

```json
{ "agents": { "coder": { "model": "local.deepseek-ai/deepseek-v4-flash" } } }
```

All original providers (Anthropic, OpenAI, Groq, OpenRouter, Azure,
Bedrock, Vertex, Copilot) remain wired as upstream left them.

## What the revival changed

Six files. Every change carries a `// GORILLA OVERRIDE:` comment saying
what and why — `grep -rn "GORILLA OVERRIDE" .` is the complete audit
trail. Headlines: authenticated OpenAI-compatible endpoints (NVIDIA NIM),
Gemini 3 thought-signature support (SDK v1.3→v1.64), two segfault fixes
that were masking real API errors, one upstream operator-precedence bug,
embedded icons + self-installer. Details, verification results, and
honest limitations: [DOCUMENTATION.dual-track.md](DOCUMENTATION.dual-track.md).

## License

MIT, unchanged from the original. © 2025 Kujtim Hoxha (original),
revival patches © 2026 contributors, same license.
