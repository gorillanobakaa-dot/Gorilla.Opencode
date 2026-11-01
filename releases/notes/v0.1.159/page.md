# Gorilla OpenCode 0.1.159: Mac, Windows and Linux

**All 13 carried helper programs are now the same on Windows, Linux and the old Intel Mac, and the Linux re-run of the everyday jobs is done.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if you use Linux or an old Intel Mac (macOS 10.13).** Until now the carried
tools (search, file tools, 7z, video, pictures) came inside the program on
Windows, and only some of them on Linux. Now every one of the 13 comes inside
the program on all three systems, so the AI does not have to hunt for them.

**Everyone else on Windows:** nothing changes for you; this is the same program
with the same tools.

**Know before you update:** the Windows program is still not signed. The macOS
tools were pinned and checked by hash, but the everyday jobs were not run on a Mac.

## Why this matters to you

The owner wants the same abilities on every computer, with no extra steps to
install a new version. A test now fails if one system gets a tool another lacks,
and the list of tools Windows had alone is empty.

## Also in this version

- The Linux re-run of the 16 jobs that use the carried tools passed 16 of 16.
- The project's checks on a change now test only the parts it touches.

## Files

| File | Bytes | SHA-256 |
|---|---|---|
| (filled in by `fieldkit release build` after the build) | | |

Check a download: `sha256sum -c SHA256SUMS-v0.1.159.txt --ignore-missing`.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Main goal

The owner's rule is that Gorilla OpenCode has the same abilities on Windows, Linux and the old Intel Mac (macOS 10.13), and that a new version reaches both of his computers without him doing anything. This release finishes the first half for the carried tools.

### Everyday problems

1. **Linux had fewer carried tools than Windows.** Seven tools (the code searcher, the file tools, the file comparer, the speed tester, the build tool, the text replacer and the file finder) came inside the program only on Windows. They now come inside it on Linux too.
2. **The old Intel Mac had none of them.** All 13 tools are now pinned for the Mac as well, each checked against a fixed fingerprint (SHA-256), with 10.13 as the lowest macOS they may need.
3. **Checking Linux.** The 16 everyday jobs that need the carried tools were run on the Linux laptop with the ChatGPT model `gpt-5.6-luna`: 16 passed, 0 failed.

### Ideas

- **A test now stops any system from getting a tool another lacks.** It covers all three systems, and the list of exceptions is empty.
- **The project's automatic checks on a change test only the parts it touches,** so a change is checked faster; the full run still happens on the main branch and on releases.

### What you get, in numbers

- 13 of 13 carried tools on each of Windows, Linux and the Intel Mac; the exception list went from 7 to 0.
- Linux everyday jobs with the carried tools: 16 of 16 (one run per job).

### What was NOT checked

- **The Mac:** the tools are pinned and hashed, but the everyday jobs were not run on a Mac.
- **One run per job is a sample,** not a rate.
- **The Windows program is still unsigned.**

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

- Linux toolbox (#54, d699af1): `ast-grep`, `coreutils`, `difft`, `hyperfine`, `ninja`, `sd` (musl builds made on the VAIO, task T-20261027-012841-ee4400) and `pfind` pinned for linux-amd64 in `internal/toolbox/SHA256SUMS`; `parityBacklog` in `internal/toolbox/parity_test.go` is now empty.
- Darwin toolbox (#56, a46c229): all 13 helpers pinned for darwin-amd64 (Intel, 10.13 floor); the parity test covers windows-amd64, linux-amd64 and darwin-amd64. Pins in `third_party/PINS.md`.
- Linux re-run (#53, fc6e5bb): P5b/P5c/P4b on the VAIO, Luna, 16 of 16 jobs; results under `tests/acceptance/basic/results/`.
- CI (7a8084a): PRs and feat/fix pushes run `go test` for the touched packages and their dependents; main and tags run the full suite.

### Test coverage and CI

Each PR ran its CI on its own head. The release build runs vet and the whole test suite once (`fieldkit emerge`, stage `test`).

### Security posture

No new network channel and no telemetry. The new helper binaries are pinned by version and SHA-256 and embedded; none is fetched at run time.

### Deployment

```
sha256sum -c SHA256SUMS-v0.1.159.txt --ignore-missing
gorilla-opencode --version
# Expected: v0.1.159
```

### Not verified

- macOS: the darwin helpers are pinned and hashed; no job was run on a Mac.
- One run per job is a sample, not a rate.

### Known issues

- The Windows exe is unsigned.
- The write tool still allows writing one folder above the working folder (owner decision pending).
