# Gorilla OpenCode v0.1.104 — it knows where your pictures are

**Everything you need is on this page**, printed in full.

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.104_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.104_amd64.deb` |
| `gorilla-opencode-0.1.104-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer |
| `SHA256SUMS-v0.1.104.txt` | `sha256sum -c` |

Use `apt`, not `dpkg -i`. Restart the program if it is already running.

---

## Screenshots - the failure that prompted this

*Click any image for the full-resolution original. Unscaled, uncropped, nothing staged.*

**Before.** Asked for "my screenshots folder", it searched the whole home directory and got back documentation images from unrelated projects.

[![The agent searching the entire home directory for anything named screenshot and getting back matches from inside unrelated documentation trees in SECOND.BRAIN and Microsoft Learn folders](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.104/docs/screenshots/gallery/v0104-before-home-wide-search-noise.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.104/docs/screenshots/gallery/v0104-before-home-wide-search-noise.png)

**Before.** The second attempt gave up after thirty seconds, and the advice it offered was true and useless.

[![The find tool reporting - Error: search timed out after 30s. Narrow it with path or glob - after a second attempt to search the home directory](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.104/docs/screenshots/gallery/v0104-before-find-timed-out.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.104/docs/screenshots/gallery/v0104-before-find-timed-out.png)

**Before.** Third attempt: guessing the path through a shell command, which correctly stopped and asked. Two minutes, three tool calls, for a folder the operating system can name instantly.

[![A permission dialog for the bash tool showing the command - for d in /home/gorilla/Pictures - as the agent falls back to guessing where the folder is](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.104/docs/screenshots/gallery/v0104-before-fell-back-to-guessing-a-path.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.104/docs/screenshots/gallery/v0104-before-fell-back-to-guessing-a-path.png)

**After.** The identical request now takes two tool calls, no timeout, and goes straight to `Pictures/Screenshots` - see the developer track below for the run.

---

## Plain-language track

### What happened

Someone asked it: *"have a look in my screenshots folder and let me know what's
in there."*

It searched the entire home directory for anything called `*screenshot*`, got
back a page of results from inside other projects' documentation folders, tried
a second search that **gave up after thirty seconds**, and finally guessed at
`/home/gorilla/Pictures` by running a shell command — which quite rightly
stopped and asked permission.

**Two minutes and three attempts** to find a folder the operating system could
have named instantly.

### Whose fault that is

The obvious reading is that the model is not very clever. The fairer reading is
that **the program never told it.**

Your computer already knows where your pictures live. Every Linux desktop writes
it down in `~/.config/user-dirs.dirs`. The agent gets a short description of
your machine at the start of every conversation — your folder, your operating
system, today's date — and that description simply never mentioned it.

This matters more than it looks, because **guessing is worse than useless on
most of the world's computers.** If your desktop is in German the folder is
`Bilder`. In Spanish, `Imágenes`. In Japanese, `画像`. An assistant guessing
"Pictures" in English fails on exactly the machines this project exists to
serve.

### What changed

The description now includes three folders, taken from your own configuration
rather than assumed:

```
This machine's standard folders:
  Documents: /home/gorilla/Documents
  Downloads: /home/gorilla/Downloads
  Pictures (screenshots land here): /home/gorilla/Pictures
```

Only folders that actually exist are listed. Telling the assistant about a
folder it cannot open is worse than saying nothing.

**Three, not six.** The full set — adding Videos, Music and Desktop — measured
66 tokens on every single message. These three cost about 41. Videos and Music
are not a coding assistant's business, and if it ever needs them it can ask,
which costs nothing until it happens. Anything that rides every message is money
taken from you repeatedly, not once, so it is worth being mean about. You can
switch the whole block off in `/context`.

### And the error message that helped nobody

When the search gave up, it said: *"Narrow it with path or glob."*

True, and useless. The assistant had just told the program the one thing that
would have let it help — **where it was looking.** Searching a whole home
directory means walking every project on the machine, and there is a specific,
obvious thing to say about that.

It now says so, and points at the folder list above.

### Does it work

The identical request, against a live model:

> The **Pictures/Screenshots** folder contains **19 files** (all PNG
> screenshots). The newest one is `Screenshot From 2026-08-19 11-45-11.png`
> (modified 1 minute ago, ~153 KiB).

**Two tool calls instead of three, no timeout, straight to the right folder.**

---

## Developer track

### `userDirs()` in the environment block

Reads `~/.config/user-dirs.dirs` directly rather than shelling out to
`xdg-user-dir`: no process per prompt render, and it works where `xdg-utils` is
not installed. Expands `$HOME`, `os.Stat`s each path and drops any that is not a
directory.

Three keys — `XDG_DOCUMENTS_DIR`, `XDG_DOWNLOAD_DIR`, `XDG_PICTURES_DIR`.
Measured: **66 tokens for the full set, 41 for these three.**
`TestTheEnvironmentBlockNamesTheStandardFolders` fails above 60, because a block
riding every turn is a recurring bill (directive 8).
`TestOnlyDirectoriesThatExistAreClaimed` walks the rendered output and stats
every path in it.

Deliberately **facts, not instructions**: it states where things are and says
nothing about what to do with them. That is what keeps it cheap and what keeps
it out of the behaviour-shaping part of the prompt. It rides the `prompt.env`
loadout row.

### The find timeout

`find.go` returned `"search timed out after 30s. Narrow it with path or glob"`
regardless of what was searched. It now detects a search rooted at the home
directory itself — the case that reliably times out, because it walks every
project on the machine — and says so, pointing at the folder list.

`withinHome` compares cleaned absolute paths and matches only the home
directory itself, not anything beneath it. Tested against `""`, `/etc`,
`~/Documents` and `~`.

### Verification

Live, against `deepseek-v4-flash`, with the exact request that failed: **2 tool
calls, no timeout**, correct file count and newest filename, plus an unprompted
note distinguishing `Pictures` from `Pictures/Screenshots`.

### Claim Sources

| Claim | Basis | Evidence |
|---|---|---|
| Home-wide search, timeout, bash fallback | 📄 stated in input | User screenshots, embedded above. |
| 66 tokens full set / 41 for three | 📄 stated in input | Measured with a probe test against the rendered block. |
| Two tool calls after the fix | 📄 stated in input | Live run; tool-call count read from the session database. |
| Localised folder names break guessing | 🤖 model inference | Well-established XDG behaviour; not tested on a localised install here. |
| These three are the right three | 🤖 model inference | A judgement about what a coding agent needs against a per-turn cost. |
