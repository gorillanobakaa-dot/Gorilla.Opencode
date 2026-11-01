# Gorilla OpenCode 0.1.160: Mac, Windows and Linux

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

Check a download: `sha256sum -c SHA256SUMS-v0.1.160.txt --ignore-missing`.
