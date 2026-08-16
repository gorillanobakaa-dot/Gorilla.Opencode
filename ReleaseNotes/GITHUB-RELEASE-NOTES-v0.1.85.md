# Gorilla Opencode v0.1.85

**Two fixes: you can read what you're typing again, and the program now owns its own files.**

---

### ⚠ Before you upgrade — read this one paragraph

This version keeps your saved conversations in a **new place**, and it does **not** move
your old ones across. Nothing is deleted — every old file stays exactly where it is — but
when you start v0.1.85 your history list will be **empty**, and nothing on screen will
explain why. Custom slash commands and per-project settings are in the same position. If
you rely on your saved conversations day to day, staying on v0.1.84 is a reasonable choice
until an automatic move exists.

---

### What's fixed

**You can see what you type.** Before, a long sentence hit the right-hand edge of the
screen and slid sideways instead of wrapping onto a second line — the start of it
disappearing off the left while you typed the end. Your words were never lost, but you
couldn't read them back. Now the box grows downwards and everything stays visible.

**The program stopped using another program's name for its files.** This is a fork of a
project called OpenCode, and it had inherited the original's habit of naming everything it
saved after *that* project. On a machine with both, two different programs were writing
folders with the same name — and during development that very nearly got this project's
work thrown out with the other one's leftovers. Every file is now named
`gorilla-opencode` and lives in the standard Linux locations for settings, data, cache and
logs, instead of dropping a folder into every project you open.

**Crash logs stopped landing in your project folders**, where they could be committed by
accident. They now go to one place set aside for them.

### Known rough edges shipping with this release

- **The one-line `install` script does not work** — it asks the download page for a file
  type this project doesn't publish. Install the `.deb` or `.pkg.tar.zst` below instead.
- **The Arch packaging instructions contradict themselves** — they tell you to download
  0.1.85 and then install 0.1.84. Trust the download line.
- **Options beginning `OPENCODE_` were renamed to `GORILLA_OPENCODE_`.** Some of our own
  documentation still shows the old names; the program is right, the docs are stale.

### Install

```bash
sudo apt install ./gorilla-opencode_0.1.85_amd64.deb
```

Use `apt`, not `dpkg -i` — `apt` also fetches the small text browser that makes web search
work with no setup. On Arch or CachyOS use `sudo pacman -U` with the `.pkg.tar.zst` file.

Then check what you actually have:

```bash
gorilla-opencode --version
```

It must print `v0.1.85`. Anything else means the old binary is still the one being run.

### Assets

| File | What it is |
|---|---|
| `gorilla-opencode_0.1.85_amd64.deb` | Debian / Ubuntu / Mint package — 18.5 MB |
| `gorilla-opencode-0.1.85-1-x86_64.pkg.tar.zst` | Arch / CachyOS package — 19.5 MB |
| `SHA256SUMS-v0.1.85.txt` | Checksums, so you can prove the download arrived intact |

To check your download before installing it:

```bash
sha256sum -c SHA256SUMS-v0.1.85.txt
```

### Read more

Both documents cover this release completely — neither is a summary of the other.

- **[Plain language](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/main/TO.DO.TO.FIX/Research.Workflow/wrapping.problem.fix/gorilla-opencode_session_layman.md)** — what changed, what you'll lose, how to check it, and why three earlier fixes failed. No prior knowledge assumed.
- **[Developer](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/main/TO.DO.TO.FIX/Research.Workflow/wrapping.problem.fix/gorilla-opencode_session_developer.md)** — the stale-viewport mechanism, the measurement probe, file-by-file changes and the honest state of test coverage.

*Why two documents? [PHILOSOPHY.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/main/PHILOSOPHY.md) — publishing code that only engineers can read is transparent in theory and a closed door in practice.*
