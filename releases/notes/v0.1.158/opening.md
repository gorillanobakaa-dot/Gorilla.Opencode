# Gorilla OpenCode 0.1.158: Mac, Windows and Linux

**The program now carries its own 7z, video, audio and picture tools on Windows and Linux, tells the AI which helpers it has, and can read web pages that only draw themselves in a browser.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if you work with 7z archives, video, audio or pictures.** Before, the AI
hunted your disk for programs that might not be installed. Now `7za`, `ffmpeg`,
`ffprobe`, `cwebp`, `avifenc` and `oxipng` come inside the program. Unpacking a
7z took 158 to 218 seconds before and 20 seconds with the carried tool.

**Yes, if the AI reads web pages for you.** A page that shows no text until its
script runs is now drawn by a browser already on your computer, and the AI
reads that.

**Everyone else:** nothing breaks; the AI is also told which helpers exist, so
it wastes fewer steps.

**Know before you update:** the Windows program is still not signed. The new
helpers were re-run on Windows only. The Linux re-run was not finished when this
page was written, and macOS was not run.

## Why this matters to you

The owner noticed that on Windows the program "seems to lack a lot of basic
capabilities". Instead of guessing, 35 everyday jobs were written down and run
for real. The ChatGPT model `gpt-5.6-luna` passed 34 of 35 on Windows, but
several only because it found a program that happened to be installed on that
laptop. On a computer without it, those jobs would likely fail.

After this release, on Windows with a clean PATH, the six jobs that needed those
programs passed 6 of 6, each calling the carried helper by name. One run per
job is a sample, not a rate.

## Also in this version

- The Windows and Linux builds are required to carry the same helpers; a test
  fails if one side gets one the other lacks.
- A suite of 35 everyday jobs with checks that look at the disk or the answer,
  in `docs/ACCEPTANCE-BASIC.md`.
- Open: the AI sometimes saves its result one folder above the folder it was
  told to use, and the program allows it. It failed one job on Windows and two
  on Linux.

## Files

| File | Bytes | SHA-256 |
|---|---|---|
| (filled in by `fieldkit release build` after the build) | | |

Check a download: `sha256sum -c SHA256SUMS-v0.1.158.txt --ignore-missing`.
