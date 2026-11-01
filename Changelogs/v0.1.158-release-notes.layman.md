# Gorilla OpenCode v0.1.158: it carries its own everyday tools and knows it has them

**Date:** 2026-11-01

---

## Main goal

The owner said Gorilla OpenCode on Windows "seems to lack a lot of basic capabilities". One local model could not see or delete a file; another searched the web for how to run `tail -f`. Rather than guess what to add, 35 everyday jobs were written down and the program was run on each one for real. This release is what that measurement led to: the program now carries the helper programs those jobs needed, on Windows and Linux alike, and its instructions to the AI say which ones are there.

## Everyday problems

1. **Opening 7z archives, converting video and audio.** The jobs "make a 7z", "unpack a 7z", "length of a video" and "pull the audio out of a video" passed only because the AI hunted the whole disk for a program that happened to be installed on the owner's laptop. One unpack took 158 to 218 seconds. On a computer without those programs they would likely fail. The program now carries three of them itself: `7za` (7z archives), `ffmpeg` (video and audio) and `ffprobe` (reads what is inside a video).
2. **Pictures.** It now also carries `cwebp` (makes WebP pictures), `avifenc` (makes AVIF pictures) and `oxipng` (makes PNG files smaller without changing a pixel). For JPEG it still needs ImageMagick, which is not carried.
3. **The AI did not know the tools were there.** The instructions now list the helpers that really exist on your computer, and point to a short "everyday" card to read before searching the web.
4. **Web pages built by script.** Some pages show no text until a browser runs their code. When a page gives almost no text, the program now asks a browser already on your computer to draw it and reads the result. It sends only the web address, nothing from your sessions. It can be switched off (`webFetchBrowserOff`, or `GORILLA_OPENCODE_WEB_FETCH_BROWSER=0`).

## Ideas

- **The Windows and Linux versions carry the same helpers.** A test now fails if one side gets a helper the other lacks. Seven Windows-only helpers remain and may only get fewer.
- **A test suite of 35 everyday jobs** (list a folder, delete to the Recycle Bin, zip, 7z, tail a log, download, convert a picture, read the text in a picture...). A check on the disk or the answer decides pass or fail; one AI never grades another.

## What you get, in numbers

All from `docs/ACCEPTANCE-BASIC.md`, with the ChatGPT model `gpt-5.6-luna`, one run per job.

- **Before:** on Windows it passed 34 of 35 jobs in each of two runs (a different job failed each time). On Linux, 33 of 35. A local Qwen model finished only 4 jobs in the time allowed, each taking 292 to 355 seconds, and was not compared.
- **After the carried helpers (Windows, clean PATH):** the 6 jobs that needed those programs passed 6 of 6. Unpacking a 7z: 20 seconds in 3 steps, down from 158 and 218 seconds. Video length: 14 seconds, 1 step.
- **After the instructions name the helpers (Windows):** 6 of 6 again; unpacking a 7z took 16 seconds in 3 steps, converting a PNG to WebP 9 seconds in 2 steps.
- **Pictures with the new codecs:** 4 of 4 (convert PNG to WebP 13 s, resize 130 s, read text in a picture 7 s, read a photo's camera and date 22 s).

One failure pattern is still open: the AI sometimes saves its result one folder above the folder it was told to work in, and the program allows it. This failed one job on Windows and two on Linux. Whether that should be blocked is for the owner to decide.

## What was NOT checked

- **The Linux re-run** of the new helpers and instructions: it was still running on the Sony VAIO when this was written.
- **macOS:** not run.
- **One run per job is a sample,** not a rate. Part of the gain from the instructions may come from the helpers being on this laptop's PATH, not from the instructions themselves.
- **A computer with none of these programs installed:** not measured.
