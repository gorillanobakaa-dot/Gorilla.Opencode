# v0.1.158: web_fetch browser fallback, toolbox parity test, 7za/ffmpeg/ffprobe and image codecs on both systems, helpers named in the system prompt, basic acceptance suite

**Date:** 2026-11-01
**Previous version:** v0.1.157
**Commit range:** `v0.1.157..e1d7700` (PRs #42-#49; #41 is the v0.1.157 PKGBUILD follow-up)

---

## Summary

- web_fetch (#42, #43): when the HTTP body holds no readable text, `web_fetch` runs `fieldkit web fetch URL --json` once (if `fieldkit` is on PATH) and returns the rendered text, fenced as untrusted and labelled "rendered by a local browser". #43: the trigger is under 200 characters of HTTP text, not only an empty page (measured: discord.com/app gave its title only, so the fallback never fired); a failed fallback keeps the short HTTP text with the reason. Off with `"webFetchBrowserOff": true` in config.json or `GORILLA_OPENCODE_WEB_FETCH_BROWSER=0`. Files: `internal/llm/tools/fetch.go`, `fetch_browser.go`, `internal/config/config.go`, `docs/FIELDKIT.md`.
- Toolbox parity (#44): `internal/toolbox/parity_test.go` requires Windows and Linux to carry the same helpers; the 7 Windows-only helpers are a frozen backlog that may only shrink (owner, 2026-10-31).
- Basic acceptance suite (#45, #46): `tests/acceptance/basic`, 35 everyday jobs (`jobs.yaml`), deterministic checks (`absent`, `exists`, `content`, `answer`, `magic`, `zip_has`, `dims`, `same_tree`, `recycled`, `size_min`; no model grades a model), runner, report renderer, unit tests. Report: `docs/ACCEPTANCE-BASIC.md`. A fresh-profile variant is included.
- P5b (#47): `7za`, `ffmpeg`, `ffprobe` pinned in the embedded toolbox on Windows and Linux (official releases, `internal/toolbox/SHA256SUMS`, `third_party/PINS.md`), `scripts/fetch-toolbox-extras.sh` and `scripts/build-toolbox.sh`, how-to cards for each, `TestEveryPinnedToolboxHelperHasACard`.
- P5c (#48): the system prompt carries a "Helpers here" block (under 1.5 KB, built from hostprobe, the toolbox and PATH, same shape on both systems, absent helpers not named) and a how-to card `everyday` that the block tells the model to read before any web search.
- P4b (#49): `cwebp` (libwebp 1.6.0), `avifenc` (libavif v1.4.2) and `oxipng` (v10.2.1) pinned in the toolbox on both systems; cards `image`, `cwebp`, `avifenc`, `oxipng`. JPEG has no toolbox encoder (mozjpeg ships no official binary for both systems); the card says to use `magick`.

## Measured (docs/ACCEPTANCE-BASIC.md; model `chatgpt.gpt-5.6-luna`, one run per cell)

- P5a, Windows: 34 of 35 in each of two runs (failures: `extract_audio` in run 1 of the clean-PATH run, `sort_uniq` in the fresh-profile run; each passed in the other). Linux (VAIO, `ee19710`): 33 of 35 (`copy_folder`, `sevenzip_make`: output written one folder above the job folder). Local `qwen3.6-35b-a3b`: 4 of the 5 jobs started (the 5th hit the 6-minute limit); every finished job took 292-355 s; 30 jobs not run; not compared with Luna.
- P5b, Windows, toolbox embedded, clean PATH: 6 of 6. `sevenzip_extract` 20 s, 3 calls (was 158 s, 15 calls and 218 s, 10 calls); `video_length` 14 s, 1 call; `extract_audio` 25 s, 3 calls; `image_resize` 211 s, 8 calls (slower than before).
- P5c, Windows, host PATH: 6 of 6. `sevenzip_extract` 16 s, 3 calls; `png_to_webp` 9 s, 2 calls; `video_length` 9 s, 1 call; `extract_audio` 16 s, 4 calls.
- P4b, Windows, image cards: 4 of 4 (`png_to_webp` 13 s, 3 calls; `image_resize` 130 s, 8 calls; `ocr_picture` 7 s, 1 call; `exif_read` 22 s, 5 calls). Record: `tests/acceptance/basic/results/p4b/`.
- The same write-outside-the-working-folder habit failed `sort_uniq` (Windows) and `copy_folder` and `sevenzip_make` (Linux). The write tool allows it; whether that is intended is an owner decision, not changed here.

## Test coverage and CI

Each PR ran build-test (Linux), build-test-windows and toolbox-windows on its own head. Locally only the touched packages (`internal/llm/tools`, `internal/config`, `internal/toolbox`, `internal/howto`, `internal/llm/prompt`, `tests/acceptance/basic`). The model runs are measurements, not CI.

## Security posture

New downloaded programs: `7za`, `ffmpeg`, `ffprobe`, `cwebp`, `avifenc`, `oxipng`, each pinned by version and SHA-256 in `third_party/PINS.md`, from the vendor's official release, embedded in the toolbox; none is fetched at run time. The web_fetch browser fallback sends only the URL to a local `fieldkit` process; no cookies, headers or session data. No telemetry added.

## Deployment

```
sha256sum -c SHA256SUMS-v0.1.158.txt --ignore-missing
gorilla-opencode --version
# Expected: v0.1.158
```

## Not verified

- The Linux re-run of P5b, P5c and P4b (running on the VAIO at the time of writing; no "Linux re-run (2026-11-01" section was on origin/main when this was written).
- macOS: not run.
- One run per job is a sample, not a rate. The P5c "before" runs used a clean PATH and the "after" run the host PATH, so part of the gain may be the helpers being on PATH rather than the prompt block (not separated).
- A bare machine with none of these helpers; Qwen on all but 5 jobs; whether `image_resize` is slow because of the tool or the model.
- The P4b record has no per-call receipt, so which program each job called was not read.

## Known issues

- JPEG compression still needs ImageMagick (`magick`), which is not in the toolbox.
- Carried: the Windows exe is unsigned. The 7 Windows-only toolbox helpers remain a frozen backlog.
