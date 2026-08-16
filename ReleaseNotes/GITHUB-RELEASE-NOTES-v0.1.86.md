# Gorilla Opencode v0.1.86

**Repairs only. No new features.** v0.1.85 fixed two real problems and shipped with loose
ends; this ties them off.

---

### ⚠ Still true from v0.1.85 — read before upgrading

Your **old conversations do not carry across**. They are safe, in the old folder, and this
version does not read them. Nothing on screen will tell you why the history list is empty.
Custom slash commands and per-project settings are in the same position. No migration
exists yet.

---

### What's fixed

- **The one-line install works.** It never did — it asked for a file this project had never
  published. That file now exists, and the script that fetches it was rewritten: it used to
  install a binary under the wrong name, check the version of a *different* program, and
  treat a "not found" web page as if it were the download.
- **A third download for everyone else.** Alongside the `.deb` and the Arch package there is
  now a plain `.tar.gz` — one binary, no sudo, any distro.
- **The Arch instructions stopped contradicting themselves** — they said download 0.1.86,
  install 0.1.84.
- **The packages contain their own release notes again.** v0.1.85's shipped notes for every
  version *except* v0.1.85.
- **The settings screen stopped saying your data lives "inside your project"** — since
  v0.1.85 it is one folder for the whole machine.
- **Our docs stopped naming options that no longer exist** (`OPENCODE_…` → `GORILLA_OPENCODE_…`).
- **File search no longer walks into your abandoned conversation history.** v0.1.85 orphaned
  the old data folder and, in the same change, removed it from the skip list.
- **The published settings reference no longer contains the build machine's username.**

### Known, unfixed, and stated plainly

- No migration for old conversations, commands or per-project settings.
- The typing fix from v0.1.85 still has no honest test — remove the fix and the test still
  passes. A real one is outstanding.

### Install

```bash
sudo apt install ./gorilla-opencode_0.1.86_amd64.deb
```

Arch/CachyOS: `sudo pacman -U gorilla-opencode-0.1.86-1-x86_64.pkg.tar.zst`
Any other distro, no sudo: extract the `.tar.gz` and put the binary in `~/.local/bin`.

Then confirm what you actually have:

```bash
gorilla-opencode --version
```

### Assets

| File | What it is |
|---|---|
| `gorilla-opencode_0.1.86_amd64.deb` | Debian / Ubuntu / Mint |
| `gorilla-opencode-0.1.86-1-x86_64.pkg.tar.zst` | Arch / CachyOS |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any distro — just the binary |
| `SHA256SUMS-v0.1.86.txt` | Checksums |

```bash
sha256sum -c SHA256SUMS-v0.1.86.txt
```

### Read more

- **[Plain language](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/main/Changelogs/v0.1.86-release-notes.md)** — what each repair means for you, no prior knowledge assumed.
- **[v0.1.85's notes](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/main/Changelogs/v0.1.85-release-notes.md)** — the two changes this release cleans up after.

*Why two tracks? [PHILOSOPHY.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/main/PHILOSOPHY.md).*
