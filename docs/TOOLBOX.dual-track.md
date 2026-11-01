# The embedded toolbox - dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers job 04 of the goal "windows-capabilities" (2026-10-27): the native pfind merged in, and every helper program carried inside gorilla-opencode.exe.

---

# Track one: plain language

## What happened

Gorilla OpenCode's AI works better with a few good command-line tools: a fast file search (pfind), a structural code search (ast-grep), a diff that understands code (difftastic), a find-and-replace (sd), a timing tool (hyperfine), a build runner (ninja) and the usual Unix commands (uutils coreutils). On Windows none of these are there by default, and installing them one by one is exactly the kind of chore Gorilla should spare you.

A Windows build made with the "toolbox" option now carries all seven inside its one .exe file. The first time one is needed, Gorilla unpacks it into your own private cache folder and checks its fingerprint (a sha256 hash) before every single run. A file that does not match is replaced from the copy inside the .exe; a copy inside the .exe that does not match its fingerprint list is never run at all.

The new native pfind.exe (job 03) is the first of them: when it is there, the find tool uses it instead of Python.

## P5b update (2026-10-31): three more helpers, on Windows AND Linux

The acceptance suite (docs/ACCEPTANCE-BASIC.md) found three programs the AI kept hunting the disk for: 7-Zip, ffmpeg and ffprobe. They are now in the toolbox under the same names on both systems: `7za` (make and unpack .7z, .zip, .tar and more), `ffmpeg` (convert, resize and cut pictures, audio and video, including PNG to WebP and extracting the sound of a video) and `ffprobe` (read a video's length and details). The "image converter" row of the suite is covered by ffmpeg, so no fourth program was added. Rule kept: Windows never gets more than Linux; the seven older helpers are still Windows-only (the frozen list in `parity_test.go` did not grow and did not shrink: 7 before, 7 after).

Cost: each Windows build with these is about 60 MB bigger (ffmpeg and ffprobe are 165 MB each unpacked, about 35 MB each compressed).

Who can act through them: nobody on their own. The files come once, at build time, from the 7-Zip and FFmpeg (GyanD and John Van Sickle) release pages, are checked against fixed fingerprints, and sit inside the program. They make no update checks and send nothing. FFmpeg can open web addresses if the AI types one; that is the AI's command, not the helper's doing.

## Honest state of play

Done and tested on this laptop (Windows 11): a toolbox build with all seven helpers; every one unpacked and checked; pfind.exe answering the find tool; the AI's shell running `sd --version` by its plain name. The ordinary build (without the toolbox option) is unchanged and carries nothing extra.

Not done yet: Linux and macOS builds carry no helpers except the three of the P5b update (below). The jq/yq "data" action and finding Visual Studio's build tools are separate pieces still to come. The six third-party helpers were built from source on the owner's second machine, not yet rebuilt on Windows.

## Worst case if something is wrong

- If a helper's fingerprint does not match, that helper is refused and the AI is told so ("this command was NOT run ..."). Nothing else stops working.
- If your cache folder were somewhere other users can write to (outside your own user folder on Windows), nothing is unpacked there at all.
- The .exe is bigger: about 85 MB instead of about 58 MB.

## What changed for you

**Searching files (the find tool)**
- Before: Needed Python on the machine; without it, find refused to run.
- After: A toolbox build uses its own pfind.exe, about four times faster, no Python needed. The result's details name the engine that ran.
- Affects: Everyone on Windows using a toolbox build

**Commands the AI runs**
- Before: `sd`, `ast-grep`, `difft`, `hyperfine`, `ninja`, `coreutils` existed only if you had installed them yourself.
- After: In a toolbox build they are on the AI shell's PATH, by plain name. Your own terminals are not changed.
- Affects: Everyone on Windows using a toolbox build

## What you can do now

Ask the AI to restructure code with ast-grep, compare two versions with difftastic, or time a command with hyperfine, without installing anything.

## What is still missing

Linux and macOS toolbox builds, 7za, the jq/yq data action, Visual Studio build-environment discovery, a Windows rebuild of the six helpers from their saved sources.

## How to check it works on your computer

In a toolbox build, ask the AI to run `sd --version` and `pfind --version`. The helpers sit in `%LOCALAPPDATA%\gorilla-opencode\toolbox\<fingerprint>\`.

## Should you be concerned?

No new network access. The helpers make no network connections of their own (checked in their source; see Track two), and Gorilla never downloads them: they are inside the .exe you already have.

## Glossary

- **sha256 / fingerprint**: a 64-character number computed from a file's bytes; any change to the file changes it.
- **cache folder**: `%LOCALAPPDATA%`, a folder inside your own user profile that only you can write to.
- **build tag**: an option given when the program is built; `toolbox` switches the embedding on.

## Claim Sources

- Sizes and test results: the commands under "How to verify" in Track two, run on 2026-10-27.
- Speed of pfind.exe: `tools/pfind-rs/tests/run_parity.py --both` (pfind.py 12.2 s, pfind.exe 3.0 s over 84 cases).

---

# Track two: developer

## Problem Being Solved

Owner decision 2026-10-26: every helper binary is embedded in gorilla-opencode.exe, extracted on first use into the private per-user cache under a name from its sha256, and hash-checked before every exec; downloads only from the approved list, built from source.

## Approach Taken

- `internal/toolbox` (new). `SHA256SUMS` (always compiled in, `//go:embed`) is the only trust anchor: `<sha256 of the uncompressed file>  <goos>-<goarch>/<file>`. `embed_on.go` (`//go:build toolbox`) embeds `bin/`; `embed_off.go` embeds nothing. Only `bin/README.md` is committed, so a fresh clone builds with or without the tag.
- `Box.embedded` reads `<platform>/<file>` or `<file>.gz` (gunzip), refuses a helper with no pin or with other bytes; the check is cached per process (the bytes are in the binary's read-only data).
- `Box.Path` (the "before every exec" call) hashes `<UserCacheDir>/gorilla-opencode/toolbox/<sha256>/<name>.exe`; on mismatch or absence it makes each folder 0700 after `CacheDirIsPrivate` (on Windows BEFORE creating it, by location), writes a temp file, renames, and hashes again.
- The folder carries the hash and the file keeps its plain name, so PATH lookup works (`sd`, not `<hash>.exe`). Deviation from COMMON.md's "file name = content hash", for that reason.
- find.go: `GORILLA_PFIND` > `toolbox.Path("pfind")` > `pfind` on PATH > python + embedded pfind.py. `pfindEngine` fills `FindResponseMetadata.Engine` ("pfind.exe (embedded)", "pfind (<path>)", "pfind.py (python)").
- shell: `agentShellEnv` (persistent shell and ConPTY) prepends `toolbox.Dirs()` (extract + check all) to PATH of the agent's shell only. bash.go calls `toolbox.VerifyMentioned(command)` before each command: each helper named as a word in the command is re-checked (and re-extracted) first; a failure refuses the command.
- `cacheDirIsPrivate` moved to `toolbox.CacheDirIsPrivate`; find.go delegates.
- `scripts/build-toolbox.ps1`: cargo-builds pfind.exe (offline, vendored), copies the six from `-HelpersDir`, stops on any pin mismatch, gzips (default; `-NoCompress` for raw), `go build -tags toolbox -trimpath -ldflags "-s -w"`.

## P5b update (2026-10-31): 7za, ffmpeg, ffprobe on both tracks

Pins (`internal/toolbox/SHA256SUMS`, one line per system; the Linux `7za` is the official `7zz`, renamed so the name matches Windows):

| helper | version | windows-amd64 sha256 | linux-amd64 sha256 |
|---|---|---|---|
| 7za | 7-Zip 26.04 (released 2026-10-05) | abd632a86f2a3be878311a50dde3d560bc719664550d8b6a411888fbe89c3629 (7za.exe, 860,160 B) | d1bc79f00e9815b24048ab167690bdd1dbc387de7dbc47e8d75d68689e49cf4f (7zz, 2,886,216 B) |
| ffmpeg | 7.0.2, GPL (Windows full build, Linux static) | d269a313b8893bd72ec3ba4f6faea08cd9f26ccda8385cf40918f08e029a1692 (142,110,208 B) | e7e7fb30477f717e6f55f9180a70386c62677ef8a4d4d1a5d948f4098aa3eb99 (79,826,272 B) |
| ffprobe | same build | 8983059df0785bdafbdda1558234042eca3b7b7d274b6bc3a2a9ce98b0eabd37 (141,952,512 B) | 4f231a1960d83e403d08f7971e271707bec278a9ae18e21b8b5b03186668450d (79,665,792 B) |

Sources (official release files, no mirror): `https://www.7-zip.org/a/7z2604-extra.7z` (Windows 7za.exe) and `7z2604-linux-x64.tar.xz` (7zz); FFmpeg's own download page links GyanD/codexffmpeg (Windows, tag `7.0.2`, `ffmpeg-7.0.2-full_build.zip`) and johnvansickle.com (Linux, `ffmpeg-7.0.2-amd64-static.tar.xz`); both keep every version, unlike BtbN's rolling builds (deleted upstream, 2026-11-01). Archive hashes are in `scripts/fetch-toolbox-extras.sh`, which downloads, refuses any other bytes, and checks every extracted file against the pin list. Neither FFmpeg source publishes a hash, and 7-Zip does not either, so all those hashes are the ones measured on 2026-10-31 over HTTPS (trust on first use, then pinned; FFmpeg's on 2026-11-01).

Threat model. Download: once, by the release maker, over HTTPS from 7-zip.org, github.com and johnvansickle.com; never by the running program. Signature: none checked for 7-Zip (none offered); the FFmpeg builds are unsigned. Who can act: Igor Pavlov (7-Zip) and the GyanD and John Van Sickle build maintainers could have shipped other bytes at fetch time; after the pin, the only bytes ever run are these. Telemetry: neither 7-Zip nor FFmpeg has any; ffmpeg only opens network addresses the caller gives it. Rule for the tag (third_party/PINS.md): 7-Zip 26.04 is the newest release at least 14 days old (26.05 is from 2026-10-21). FFmpeg 7.0.2 is a tagged release from 2024, far older than 14 days, and the only version that both sources keep as a permanent versioned download (the Linux source's newest is 7.0.2).

Build: `bash scripts/fetch-toolbox-extras.sh OUT`, then `scripts/build-toolbox.ps1 -HelpersDir` (Windows, flat folder) or `bash scripts/build-toolbox.sh OUT` (Linux, new: embeds the linux-amd64 pins, gzip, `-tags toolbox`). Licenses: `third_party/7zip/License.txt` (LGPL + unRAR restriction), `third_party/ffmpeg/LICENSE.txt` (GPL).

Parity: `TestToolboxSameHelpersOnWindowsAndLinux` passes with the three on both systems; `parityBacklog` is unchanged at 7 (ast-grep, coreutils, difft, hyperfine, ninja, pfind, sd). Linux: the system tool is still found on PATH when the toolbox is not built in.

## Before

No helper shipped. find needed python3 (or a `pfind` on PATH). The shell found only what the user had installed.

## After

| build | gorilla-opencode.exe bytes |
|---|---|
| plain (`go build -trimpath -ldflags "-s -w"`) | 57,540,608 |
| `-tags toolbox`, helpers gzip (default) | 84,741,632 |
| `-tags toolbox`, helpers raw (`-NoCompress`) | 244,210,176 |

Gzip sizes: difft 10.8 MB (of 116.1), ast-grep 7.5 (50.2), coreutils 5.3 (12.2), pfind 1.4 (3.3), sd 0.9 (2.2), hyperfine 0.6 (1.4), ninja 0.5 (1.3). Extraction cost: first use writes 186.6 MB into the cache once per helper version.

## Known Alternatives

- Ship the helpers beside the .exe in the zip: rejected by the owner (one file).
- Name the extracted FILE by its hash: then the shell cannot call it by name without per-helper shims.
- zstd/xz instead of gzip: smaller, but not in Go's standard library (a new dependency); gzip already cuts 159.5 MB.

## Files Changed

- `internal/toolbox/` (new): toolbox.go, private.go, embed_on.go, embed_off.go, SHA256SUMS, bin/README.md, toolbox_test.go, embedded_test.go
- `internal/llm/tools/find.go`, `find_test.go`, `bash.go`, `shell/shell.go`, `shell/conpty_windows.go`, `shell/toolbox_path_test.go`
- `scripts/build-toolbox.ps1`, `.gitignore`, `.github/workflows/ci.yml` (job `toolbox-windows`), `tests/mutations-toolbox.yaml`
- `third_party/PINS.md` ("Embedded toolbox"), `third_party/{ast-grep,coreutils,difftastic,hyperfine,ninja,sd}/LICENSE*`
- Merge of feat/pfind-rs (tools/pfind-rs, docs/PFIND-RS.dual-track.md); see Decisions.

## Decisions Made

- gzip by default: 84.7 MB vs 244.2 MB exe; decompression only on first extract.
- Verify-before-exec for the shell is by name: a helper called only indirectly (a script that calls ninja) is on PATH, checked when the shell session started, not re-checked per command. Documented limit.
- Vendored sources of the six (161,981,237 bytes of `vendor-*.tar.zst`) are not committed; pinned by sha256 in third_party/PINS.md.
- The merge of feat/pfind-rs: the repository's pre-commit hook passes every new path on one command line and failed with WinError 206 on 2192 files; the vendored crates were committed byte-identical in 11 parts first, then `git merge --no-ff feat/pfind-rs`. The hook (a guard) was not changed.
- A refused embedded pfind falls through to pfind on PATH / python (the result's engine says which ran) rather than failing the search.

## Tried and Abandoned

- `-run "(?i)a|b"` in the mutation command: Go's test filter splits on top-level `|` and compiles each part alone, so `(?i)` covered only the first; 7 mutants "survived" untested. Fixed with `(?i)(a|b)`.

## ⚠ Claimed But Not Verified

- The helpers' lack of network code: from the VAIO build report (Cargo.lock and source grep), not a traffic capture.
- TOCTOU between the hash check and CreateProcess: another process running as the same user could swap the file in between. Not mitigated (such a process already has the user's rights).
- Linux/macOS: the toolbox package builds and its tests compile (GOOS=linux vet, darwin go1.24.13 build); no helper is embedded there and no Linux/macOS run was made.
- The CI job `toolbox-windows` has not run yet (pushing from a worker is refused; GitHub CI is checked by job ci-red-tests).

## Open Items

- 7-Zip 7za built with MSVC; jq/yq `data` action (gojq, yq as libraries); MSVC discovery from `state.json`; Windows rebuild script for the six from the vendor tarballs; per-crate license notices of the helpers' dependencies; Linux/macOS embeds; windows-arm64 helpers.
- Remove the job-03 worktree (`git worktree remove`) once nobody needs it (left in place on the round-8 instruction).

## How to verify this work is correct

```
fieldkit guard -- powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-toolbox.ps1 -HelpersDir <folder with the six .exe> -Output <exe>
fieldkit guard -- go test -count=1 -tags toolbox ./internal/toolbox/... -v
fieldkit guard -- go test -count=1 -tags toolbox ./internal/llm/tools/shell/ -run AgentShell -v
fieldkit guard -- fieldkit mutate --plan tests/mutations-toolbox.yaml --cwd . -- <the command on its first line>
python -I tools/pfind-rs/tests/run_parity.py --both
```

Measured 2026-10-27: 14 toolbox tests pass with the seven helpers embedded (pfind --version 4.0.0 through the checked path); the two shell tests pass (`sd --version` by plain name); mutation 16 caught, 0 survived; parity 84 cases, 0 failures, 0 mismatches.

## Technical Debt

- `Mentioned` compiles one regexp per helper per command (7 small regexps; microseconds).
- `Dirs()` at shell start extracts every helper even if none is used (once per helper version).

## Claim Sources

- Hashes: `fieldkit size hash` of each exe on Windows, equal to the build host's SHA256SUMS.
- Sizes: `scripts/build-toolbox.ps1` output, 2026-10-27.
- Build host and toolchains: third_party/PINS.md, from the build report of 2026-10-27.
