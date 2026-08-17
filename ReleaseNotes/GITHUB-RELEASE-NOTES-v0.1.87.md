# Gorilla Opencode v0.1.87

**Sign in with ChatGPT.** OpenAI's coding models are now reachable with the ChatGPT
account you already have, instead of an API key — and a **free** ChatGPT account is
enough. No developer account, no payment method, no card.

---

### Why this matters

An API key needs an OpenAI developer account, which needs a payment method. For a lot of
people that is where the road ends. This release opens a second door: you sign in through
your browser, the way you sign in to any website, and the models appear.

There is **no per-use charge on this route at all** — which is why the model list shows no
prices next to these two entries. Your ChatGPT plan is what pays, and the free plan costs
nothing. Go over the allowance and you are paused for a while, never billed.

### How to use it

Start the program — click the icon or run `gorilla-opencode` — and the sign-in list
appears. **ChatGPT is the second row**, tagged `free`. Press Enter on it, sign in to
ChatGPT in the browser that opens, and you are done.

From a terminal, if you prefer:

```bash
gorilla-opencode login --chatgpt
```

### What you get

| Model | Used for | Notes |
|---|---|---|
| **GPT-5.5** | your actual work | 272K context, tools, images |
| **GPT-5.4 Mini** | background jobs (naming and summarising conversations) | **OpenAI retires this on 31 Aug 2026** |

**GPT-5.6 Terra and Luna are deliberately not offered.** The backend reports them as
`code_mode_only` — they expect tools handed over in a shape this program does not speak
yet. Listing them would mean you sign in, pick one, and watch it fail the first time it
tries to read a file. The sign-in screen says so on the row itself, so their absence does
not read as a bug.

### This was measured, not assumed

Every part of the wire format was an inference until it ran against a real free account:

- streamed text — deltas arriving, usage reported
- a tool call — emitted, arguments valid JSON, correctly identified
- the tool **result** fed back and used in the next answer
- the whole thing end-to-end through this binary's real agent loop: it read a file off
  disk and quoted it back

Two of those inferences were wrong and only the live run caught them. Both are written up
in the notes below.

### Known limits, stated plainly

- **Unofficial.** This speaks the Codex client's protocol, which OpenAI does not publish.
  An OpenAI-side change could break it without notice.
- **GPT-5.4 Mini dies on 31 Aug 2026.** Known and dated. Use GPT-5.5.
- **No usage meter** for ChatGPT the way `/usage` shows the Antigravity allowance.
- **The model list does not refresh itself** — it is a snapshot from 17 Aug 2026.
- The program identifies itself to OpenAI honestly as `gorilla_opencode`. It does not
  pretend to be another client.

### ⚠ Still true from v0.1.85 — read before upgrading

Your **old conversations do not carry across**. They are safe, in the old folder, and this
version does not read them. Custom slash commands and per-project settings are in the same
position. No migration exists yet.

### Also fixed

- `install.sh` no longer reports a network failure that did not happen. Piping `curl` into
  `awk` closed the pipe early, so curl exited 23 on a download that had actually worked.
- One installer, one checksum file, both exercised against the live release.

### Install

```bash
sudo apt install ./gorilla-opencode_0.1.87_amd64.deb
```

Arch/CachyOS: `sudo pacman -U gorilla-opencode-0.1.87-1-x86_64.pkg.tar.zst`
Any other distro, no sudo: extract the `.tar.gz` and put the binary in `~/.local/bin`.

Then confirm what you actually have — the binary and the package database must agree:

```bash
gorilla-opencode --version && dpkg -l gorilla-opencode | tail -1
```

### Assets

| File | What it is |
|---|---|
| `gorilla-opencode_0.1.87_amd64.deb` | Debian / Ubuntu / Mint |
| `gorilla-opencode-0.1.87-1-x86_64.pkg.tar.zst` | Arch / CachyOS |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any distro — just the binary |
| `checksums.sha256` | Checksums for all three |

```bash
sha256sum -c checksums.sha256
```

Or install in one command — it downloads, **verifies the checksum**, and sets up the
desktop launcher and icons for you:

```bash
curl -fsSL https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/main/install.sh | sh
```

### Read more

- **[Plain language](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.87/Changelogs/v0.1.87-release-notes.md)** — the whole release with no prior knowledge assumed, plus the developer track and a claim-source table saying what we measured and what we inferred.
- **[v0.1.86's notes](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.86/Changelogs/v0.1.86-release-notes.md)** — the repairs this builds on.

*Why two tracks? [PHILOSOPHY.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.87/PHILOSOPHY.md).*
