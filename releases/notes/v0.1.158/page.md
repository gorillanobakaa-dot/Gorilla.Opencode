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

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Main goal

The owner said Gorilla OpenCode on Windows "seems to lack a lot of basic capabilities". One local model could not see or delete a file; another searched the web for how to run `tail -f`. Rather than guess what to add, 35 everyday jobs were written down and the program was run on each one for real. This release is what that measurement led to: the program now carries the helper programs those jobs needed, on Windows and Linux alike, and its instructions to the AI say which ones are there.

### Everyday problems

1. **Opening 7z archives, converting video and audio.** The jobs "make a 7z", "unpack a 7z", "length of a video" and "pull the audio out of a video" passed only because the AI hunted the whole disk for a program that happened to be installed on the owner's laptop. One unpack took 158 to 218 seconds. On a computer without those programs they would likely fail. The program now carries three of them itself: `7za` (7z archives), `ffmpeg` (video and audio) and `ffprobe` (reads what is inside a video).
2. **Pictures.** It now also carries `cwebp` (makes WebP pictures), `avifenc` (makes AVIF pictures) and `oxipng` (makes PNG files smaller without changing a pixel). For JPEG it still needs ImageMagick, which is not carried.
3. **The AI did not know the tools were there.** The instructions now list the helpers that really exist on your computer, and point to a short "everyday" card to read before searching the web.
4. **Web pages built by script.** Some pages show no text until a browser runs their code. When a page gives almost no text, the program now asks a browser already on your computer to draw it and reads the result. It sends only the web address, nothing from your sessions. It can be switched off (`webFetchBrowserOff`, or `GORILLA_OPENCODE_WEB_FETCH_BROWSER=0`).

### Ideas

- **The Windows and Linux versions carry the same helpers.** A test now fails if one side gets a helper the other lacks. Seven Windows-only helpers remain and may only get fewer.
- **A test suite of 35 everyday jobs** (list a folder, delete to the Recycle Bin, zip, 7z, tail a log, download, convert a picture, read the text in a picture...). A check on the disk or the answer decides pass or fail; one AI never grades another.

### What you get, in numbers

All from `docs/ACCEPTANCE-BASIC.md`, with the ChatGPT model `gpt-5.6-luna`, one run per job.

- **Before:** on Windows it passed 34 of 35 jobs in each of two runs (a different job failed each time). On Linux, 33 of 35. A local Qwen model finished only 4 jobs in the time allowed, each taking 292 to 355 seconds, and was not compared.
- **After the carried helpers (Windows, clean PATH):** the 6 jobs that needed those programs passed 6 of 6. Unpacking a 7z: 20 seconds in 3 steps, down from 158 and 218 seconds. Video length: 14 seconds, 1 step.
- **After the instructions name the helpers (Windows):** 6 of 6 again; unpacking a 7z took 16 seconds in 3 steps, converting a PNG to WebP 9 seconds in 2 steps.
- **Pictures with the new codecs:** 4 of 4 (convert PNG to WebP 13 s, resize 130 s, read text in a picture 7 s, read a photo's camera and date 22 s).

One failure pattern is still open: the AI sometimes saves its result one folder above the folder it was told to work in, and the program allows it. This failed one job on Windows and two on Linux. Whether that should be blocked is for the owner to decide.

### What was NOT checked

- **The Linux re-run** of the new helpers and instructions: it was still running on the Sony VAIO when this was written.
- **macOS:** not run.
- **One run per job is a sample,** not a rate. Part of the gain from the instructions may come from the helpers being on this laptop's PATH, not from the instructions themselves.
- **A computer with none of these programs installed:** not measured.

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

- web_fetch (#42, #43): when the HTTP body holds no readable text, `web_fetch` runs `fieldkit web fetch URL --json` once (if `fieldkit` is on PATH) and returns the rendered text, fenced as untrusted and labelled "rendered by a local browser". #43: the trigger is under 200 characters of HTTP text, not only an empty page (measured: discord.com/app gave its title only, so the fallback never fired); a failed fallback keeps the short HTTP text with the reason. Off with `"webFetchBrowserOff": true` in config.json or `GORILLA_OPENCODE_WEB_FETCH_BROWSER=0`. Files: `internal/llm/tools/fetch.go`, `fetch_browser.go`, `internal/config/config.go`, `docs/FIELDKIT.md`.
- Toolbox parity (#44): `internal/toolbox/parity_test.go` requires Windows and Linux to carry the same helpers; the 7 Windows-only helpers are a frozen backlog that may only shrink (owner, 2026-10-31).
- Basic acceptance suite (#45, #46): `tests/acceptance/basic`, 35 everyday jobs (`jobs.yaml`), deterministic checks (`absent`, `exists`, `content`, `answer`, `magic`, `zip_has`, `dims`, `same_tree`, `recycled`, `size_min`; no model grades a model), runner, report renderer, unit tests. Report: `docs/ACCEPTANCE-BASIC.md`. A fresh-profile variant is included.
- P5b (#47): `7za`, `ffmpeg`, `ffprobe` pinned in the embedded toolbox on Windows and Linux (official releases, `internal/toolbox/SHA256SUMS`, `third_party/PINS.md`), `scripts/fetch-toolbox-extras.sh` and `scripts/build-toolbox.sh`, how-to cards for each, `TestEveryPinnedToolboxHelperHasACard`.
- P5c (#48): the system prompt carries a "Helpers here" block (under 1.5 KB, built from hostprobe, the toolbox and PATH, same shape on both systems, absent helpers not named) and a how-to card `everyday` that the block tells the model to read before any web search.
- P4b (#49): `cwebp` (libwebp 1.6.0), `avifenc` (libavif v1.4.2) and `oxipng` (v10.2.1) pinned in the toolbox on both systems; cards `image`, `cwebp`, `avifenc`, `oxipng`. JPEG has no toolbox encoder (mozjpeg ships no official binary for both systems); the card says to use `magick`.

### Measured (docs/ACCEPTANCE-BASIC.md; model `chatgpt.gpt-5.6-luna`, one run per cell)

- P5a, Windows: 34 of 35 in each of two runs (failures: `extract_audio` in run 1 of the clean-PATH run, `sort_uniq` in the fresh-profile run; each passed in the other). Linux (VAIO, `ee19710`): 33 of 35 (`copy_folder`, `sevenzip_make`: output written one folder above the job folder). Local `qwen3.6-35b-a3b`: 4 of the 5 jobs started (the 5th hit the 6-minute limit); every finished job took 292-355 s; 30 jobs not run; not compared with Luna.
- P5b, Windows, toolbox embedded, clean PATH: 6 of 6. `sevenzip_extract` 20 s, 3 calls (was 158 s, 15 calls and 218 s, 10 calls); `video_length` 14 s, 1 call; `extract_audio` 25 s, 3 calls; `image_resize` 211 s, 8 calls (slower than before).
- P5c, Windows, host PATH: 6 of 6. `sevenzip_extract` 16 s, 3 calls; `png_to_webp` 9 s, 2 calls; `video_length` 9 s, 1 call; `extract_audio` 16 s, 4 calls.
- P4b, Windows, image cards: 4 of 4 (`png_to_webp` 13 s, 3 calls; `image_resize` 130 s, 8 calls; `ocr_picture` 7 s, 1 call; `exif_read` 22 s, 5 calls). Record: `tests/acceptance/basic/results/p4b/`.
- The same write-outside-the-working-folder habit failed `sort_uniq` (Windows) and `copy_folder` and `sevenzip_make` (Linux). The write tool allows it; whether that is intended is an owner decision, not changed here.

### Test coverage and CI

Each PR ran build-test (Linux), build-test-windows and toolbox-windows on its own head. Locally only the touched packages (`internal/llm/tools`, `internal/config`, `internal/toolbox`, `internal/howto`, `internal/llm/prompt`, `tests/acceptance/basic`). The model runs are measurements, not CI.

### Security posture

New downloaded programs: `7za`, `ffmpeg`, `ffprobe`, `cwebp`, `avifenc`, `oxipng`, each pinned by version and SHA-256 in `third_party/PINS.md`, from the vendor's official release, embedded in the toolbox; none is fetched at run time. The web_fetch browser fallback sends only the URL to a local `fieldkit` process; no cookies, headers or session data. No telemetry added.

### Deployment

```
sha256sum -c SHA256SUMS-v0.1.158.txt --ignore-missing
gorilla-opencode --version
# Expected: v0.1.158
```

### Not verified

- The Linux re-run of P5b, P5c and P4b (running on the VAIO at the time of writing; no "Linux re-run (2026-11-01" section was on origin/main when this was written).
- macOS: not run.
- One run per job is a sample, not a rate. The P5c "before" runs used a clean PATH and the "after" run the host PATH, so part of the gain may be the helpers being on PATH rather than the prompt block (not separated).
- A bare machine with none of these helpers; Qwen on all but 5 jobs; whether `image_resize` is slow because of the tool or the model.
- The P4b record has no per-call receipt, so which program each job called was not read.

### Known issues

- JPEG compression still needs ImageMagick (`magick`), which is not in the toolbox.
- Carried: the Windows exe is unsigned. The 7 Windows-only toolbox helpers remain a frozen backlog.
