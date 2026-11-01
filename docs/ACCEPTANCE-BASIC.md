# Basic acceptance suite - dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers plan row P5a (2026-10-31): a set of 35 everyday jobs, run against the real program with real models, so that what is missing is measured before anything is added (rows P5b and P5c).

---

# Track one: plain language

## What happened

The owner said Gorilla OpenCode on Windows "seems to lack a lot of basic capabilities": a local Qwen model could not see or delete a file, and a model searched the web for how to run `tail -f`. Instead of guessing which tools to add, 35 everyday jobs were written down (list a folder, delete a file to the Recycle Bin, zip, 7z, tail, grep, download, convert a picture, read the text in a picture, probe a video, and so on). Each job has a prepared folder, the sentence a person would type, and a check that looks at the disk or at the answer. A model never grades another model.

Then the program was run for real on each job with `chatgpt.gpt-5.6-luna` (ChatGPT sign-in), and on a few jobs with the local `qwen3.6-35b-a3b` in LM Studio.

## Honest state of play

- **ChatGPT Luna passed 34 of 35 jobs in each of two runs** on this Windows laptop. The failure was a different job in each run (audio extraction in run 1, sort-and-deduplicate in run 2); each passed in the other run. One run per cell is a sample, not a rate.
- **The pass rate hides the cost.** Several jobs passed only because the model hunted the disk for a program that is not on the system PATH and not inside Gorilla: 7-Zip (158 to 218 s and 10 to 15 tool calls; it ran a 7zr.exe from the Lenovo Vantage folder in one run and one from a Firefox build tree in the other), ImageMagick (8 to 13 calls), ffmpeg (found in the scoop folder in one run, not found in the other). On a machine where nobody installed these, the same jobs would likely fail. This laptop is not such a machine, so "pass" here means "this machine has the tool somewhere".
- **The tools Gorilla already carries were not the gap.** On the jobs about files and text, Luna used the built-in find, view, files, write and archive tools, almost never a shell command, and never the embedded coreutils by name. They passed in 1 to 5 calls and mostly under 20 seconds.
- **A model that can see needs no OCR program.** Luna read the text in the picture with its own eyes (1 call), and read the photo's camera and date through PowerShell and Windows' own image library. A model without vision would have no way; that was not tested.
- **Local Qwen was too slow to measure fully.** Every Qwen job that finished took 292 to 355 s, even with a single tool call. It passed all 4 it finished (list, read, delete, delete to the Recycle Bin: the two things the owner said it could not do worked here). The fifth job (a hidden file) hit the 6-minute limit before the model made any call. The other 30 jobs were not run: the run was stopped after 5 jobs because 35 jobs at 5 minutes each did not fit the time budget. Qwen is **not** compared with Luna in this report.
- **Linux was run on 2026-10-31** (Sony VAIO, same commit `ee19710`): ChatGPT Luna passed **33 of 35**. Both failures wrote their result one folder above the job folder (a copy and a 7z archive); the model said it was done, the disk says the file is in the wrong place. The same habit failed `sort_uniq` once on Windows. The suite needed no change to run on Linux.

## Worst case if something is wrong

- The suite works in a scratch folder. It leaves tiny files in the real Recycle Bin (named `junk.tmp` and `acc-recycle-8841.txt`; there are 4 and 3 now, all from this job). The model, however, can touch anything it is allowed to: in run 2 it wrote a file one folder above its working folder, and the program let it.
- Model runs spend ChatGPT allowance (free plan). A full run of 35 jobs took 11 to 15 minutes.

## What changed for you

Nothing in the program. Only a test suite and this report were added.
- Before: nobody knew which everyday jobs fail.
- After: `tests/acceptance/basic` lists them and measures them; this page is the first reading.
- Affects: whoever decides what tools to add or to explain to the model next (rows P5b and P5c).

---

# Track two: developer reference

## Method

- Jobs: `tests/acceptance/basic/jobs.yaml` (35). Each: `files`/`dirs`/`lines`/`zips`/`fixtures` (made in `<root>/<job id>/`), an optional helper `service` (`http` serving a folder outside the job folder, `port` listening in the runner, `growing_log` appending a `tick-NNN` line every 700 ms), the `prompt`, and `checks`. Checks are deterministic: `absent`, `exists`, `content`, `answer`, `magic`, `zip_has`, `dims`, `same_tree`, `recycled` (the Recycle Bin / trash holds the unique name), `size_min`. Unknown YAML fields and unknown check kinds are rejected at load.
- Runner: builds the exe once (`go build -tags toolbox` on Windows; the six pinned helpers are taken from the installed copy's cache folder, sha256-checked against `internal/toolbox/SHA256SUMS`, embedded gzip-compressed, removed after the build), then per job `gorilla-opencode -p PROMPT -c FOLDER -f json -q` with the folder as the process's working directory.
- Permissions: nothing was weakened. `-p` calls `AutoApproveSession` and `SetUnattended(true)` (`internal/app/app.go`): ordinary actions are approved, carve-outs are refused with a logged reason. No receipt of any run contains `refused`, so "refused by permission or guard" is 0 in this reading.
- Model pin: `.gorilla-opencode.json` in the job folder (`agents.coder/task/title/summarizer.model`), the only per-folder setting the program accepts. The user's config is untouched. The file is visible to the model as a dotfile.
- Environment: PATH is the Windows system folders only (`clean`); the program adds the embedded toolbox to its own shell's PATH. Run 1 ("this machine") keeps the user's profile. Run 2 (`fresh-profile`) points USERPROFILE, HOME, APPDATA and LOCALAPPDATA at an empty folder and keeps the config folder through `XDG_CONFIG_HOME`; login and the Fieldkit MCP server therefore stayed available in both runs (`fieldkit_discover/describe/run` appear in the receipts).
- Record per job (`results/*.json`): checks with reasons, the program's own receipt (tool, one key argument clipped to 90 characters, outcome; other arguments such as the `files` action are not in the receipt), wall time, exit code, stderr tail, whether each `needs` program was on PATH, and an automatic cause guess. `results/causes.yaml` holds the cause settled by reading the receipt and overrides the guess.
- Not CI: the model runs are the measurement. The unit tests (`go test ./tests/acceptance/basic`) cover the loader, setup, passing and failing checks, services, receipt parsing, the cause guess and the table.

## Result

`PASS (seconds, tool calls)`. Models: `chatgpt.gpt-5.6-luna` (two environments) and `local.qwen3.6-35b-a3b` (LM Studio, context 32768, GPU max; 5 jobs of a 15-job selection, stopped for time). OS: Windows 11 on the L15, exe built from this branch with the toolbox embedded.

| job | chatgpt.gpt-5.6-luna [fresh-profile] / windows | chatgpt.gpt-5.6-luna / windows | local.qwen3.6-35b-a3b / windows |
|---|---|---|---|
| list_folder | PASS (4 s, 1 calls) | PASS (8 s, 1 calls) | PASS (306 s, 1 calls) |
| read_file | PASS (4 s, 1 calls) | PASS (5 s, 1 calls) | PASS (292 s, 1 calls) |
| hidden_file | PASS (8 s, 3 calls) | PASS (10 s, 3 calls) | FAIL (360 s, 0 calls) |
| file_type | PASS (5 s, 2 calls) | PASS (8 s, 2 calls) | not run |
| file_hash | PASS (4 s, 1 calls) | PASS (5 s, 1 calls) | not run |
| create_file | PASS (9 s, 3 calls) | PASS (9 s, 3 calls) | not run |
| make_folders | PASS (10 s, 2 calls) | PASS (9 s, 2 calls) | not run |
| delete_file | PASS (9 s, 3 calls) | PASS (8 s, 3 calls) | PASS (304 s, 1 calls) |
| delete_to_bin | PASS (9 s, 3 calls) | PASS (15 s, 4 calls) | PASS (355 s, 1 calls) |
| rename_file | PASS (7 s, 2 calls) | PASS (6 s, 2 calls) | not run |
| move_file | PASS (9 s, 5 calls) | PASS (11 s, 5 calls) | not run |
| copy_folder | PASS (8 s, 3 calls) | PASS (11 s, 4 calls) | not run |
| replace_many | PASS (25 s, 9 calls) | PASS (21 s, 10 calls) | not run |
| sort_uniq | FAIL (8 s, 4 calls) | PASS (17 s, 4 calls) | not run |
| zip_make | PASS (10 s, 4 calls) | PASS (11 s, 3 calls) | not run |
| zip_extract | PASS (17 s, 6 calls) | PASS (20 s, 3 calls) | not run |
| sevenzip_make | PASS (11 s, 5 calls) | PASS (9 s, 3 calls) | not run |
| sevenzip_extract | PASS (158 s, 15 calls) | PASS (218 s, 10 calls) | not run |
| tail_30 | PASS (5 s, 1 calls) | PASS (10 s, 4 calls) | not run |
| follow_log | PASS (14 s, 3 calls) | PASS (21 s, 3 calls) | not run |
| count_lines | PASS (4 s, 1 calls) | PASS (8 s, 1 calls) | not run |
| find_by_name | PASS (4 s, 1 calls) | PASS (9 s, 1 calls) | not run |
| grep_text | PASS (4 s, 1 calls) | PASS (4 s, 1 calls) | not run |
| json_query | PASS (4 s, 1 calls) | PASS (4 s, 1 calls) | not run |
| diff_files | PASS (5 s, 2 calls) | PASS (6 s, 2 calls) | not run |
| count_by_ext | PASS (3 s, 1 calls) | PASS (4 s, 1 calls) | not run |
| disk_usage | PASS (11 s, 2 calls) | PASS (5 s, 1 calls) | not run |
| download_file | PASS (78 s, 10 calls) | PASS (125 s, 9 calls) | not run |
| port_process | PASS (7 s, 1 calls) | PASS (55 s, 4 calls) | not run |
| png_to_webp | PASS (34 s, 8 calls) | PASS (98 s, 13 calls) | not run |
| image_resize | PASS (27 s, 7 calls) | PASS (30 s, 6 calls) | not run |
| ocr_picture | PASS (5 s, 1 calls) | PASS (4 s, 1 calls) | not run |
| exif_read | PASS (17 s, 2 calls) | PASS (14 s, 3 calls) | not run |
| video_length | PASS (61 s, 6 calls) | PASS (44 s, 7 calls) | not run |
| extract_audio | PASS (43 s, 6 calls) | FAIL (57 s, 10 calls) | not run |
| **passed** | **34 of 35** | **34 of 35** | **4 of 35** |

| model / OS | job | cause | why it failed |
|---|---|---|---|
| chatgpt.gpt-5.6-luna [fresh-profile] / windows | sort_uniq | model error | Model wrote unique.txt one folder ABOVE the working folder (write path C:\fk-scratch\p5a-acc\unique.txt). The write tool let it; the same job passed in the other run. |
| chatgpt.gpt-5.6-luna / windows | extract_audio | tool missing | ffmpeg is not on the system PATH and not in the embedded toolbox (it is only in scoop here). The model looked, asked the fieldkit videokit tool, which needs a download that needs approval, and stopped to ask. |
| local.qwen3.6-35b-a3b / windows | hidden_file | model error | Killed at the 6-minute job limit with no tool call made: every Qwen job that finished took 292 to 355 s here, even with one tool call, so the limit ended the run while the model was still working (why a turn is that slow was not measured). Not a tool gap. Not re-run (time budget). |

| cause | failures |
|---|---|
| model error | 2 |
| tool missing | 1 |

The cause column uses the four labels of the brief. `model error` for Qwen/hidden_file is a time limit, not a wrong answer.

## What the table says, for P5b and P5c

1. **Tools missing from PATH and from the toolbox, found only by searching the disk** (P5b candidates):
   - 7-Zip: the `archive` tool fails on `.7z` in both runs (`archive ... failed`), then the model hunts the shell; 158 to 218 s.
   - An image converter: `png_to_webp` took 8 to 13 calls; `image_resize` fell back to `System.Drawing` through PowerShell.
   - ffmpeg/ffprobe: `extract_audio` failed once and passed once (via the scoop folder); `video_length` fell back to a Windows shell property. Both runs tried the Fieldkit `videokit` tool first, which needs a download approval.
2. **Model did not know a tool exists** (P5c): `sevenzip_make` passed through the `files` tool (it made a valid 7z), while `archive` cannot extract one; in `png_to_webp` the model asked `fieldkit_discover` before looking at the machine. No job showed a model searching the web for a basic command.
3. **No permission refusals** in the 75 job-runs recorded.
4. **The embedded coreutils were not used by name once.** `tail_30`, `count_lines`, `file_hash`, `disk_usage`, `find_by_name` and `grep_text` passed with `view`, `find` and `files`.
5. **A write outside the working folder is allowed** (`sort_uniq`, run 2: `write C:\fk-scratch\p5a-acc\unique.txt`). Whether that is intended is the owner's decision; it is reported here, not changed.
6. **Qwen locally: 292 to 355 s per job.** The cause of the slowness was not measured (prompt size, offload and thinking were not separated).

## Limits of this reading

- One run per model per environment: flaky cells (`extract_audio`, `sort_uniq`) show run-to-run variance. Nothing here is a rate.
- This laptop has 7z, ImageMagick, ffmpeg, tesseract and Python with Pillow (scoop and Program Files). `fresh-profile` hid only the profile variables; models still found tools by absolute path (`find C:\`). A truly bare machine was not measured, so "tool missing" is inferred from the PATH and the embedded list, not observed on a bare PC.
- Qwen: 5 of 35 jobs.
- The receipt records one argument per call.
- The OCR, EXIF and video checks accept the model's own vision or PowerShell route; they do not require the named program.
- Linux: one run, one model (ChatGPT Luna, 33 of 35), the default clean PATH `/usr/local/bin:/usr/bin:/bin`. This VAIO has 7z, ffmpeg and ImageMagick installed, so Linux passes say "installed", not "carried by Gorilla". Qwen was not run on Linux. macOS: not run.

## Linux result (2026-10-31, Sony VAIO, exe built from `ee19710`, no toolbox on Linux)

`chatgpt.gpt-5.6-luna`, default model, `ACC_ROOT` inside a scratch job, 521 s for 35 jobs. Unit tests: `ok github.com/opencode-ai/opencode/tests/acceptance/basic`. Per-job table: `tests/acceptance/basic/results/tables.md` (column `linux`) and `results/chatgpt.gpt-5.6-luna-linux.json`.

| model / OS | passed | failures |
|---|---|---|
| chatgpt.gpt-5.6-luna / linux | 33 of 35 | copy_folder, sevenzip_make |

| job | cause | why |
|---|---|---|
| copy_folder | model error | `photos-backup` was created one folder above the job folder; the answer claimed success. Whether the model passed a wrong path or the `files` tool resolved a relative path elsewhere was not determined (the receipt clips arguments). |
| sevenzip_make | model error | `docs.7z` was written one folder above the job folder. 7z was on PATH; the runner's automatic guess ("model did not know the tool exists") was wrong and is overridden in `causes.yaml`. |

Reading: no Linux-shaped defect in the suite (PATH, fixtures, helpers, hidden files and the trash check all worked), so no code fix was made. The two failures and the Windows `sort_uniq` failure are one pattern, a write outside the working folder, which is a program behavior for the owner to decide (point 5 above). Not re-run: one run per cell is a sample.

## P5c re-run: the prompt names the helpers (2026-10-31, Windows, ChatGPT Luna)

Change: the system prompt now carries a "Helpers here" block (under 1.5 KB, built from the toolbox and PATH, same shape on Windows and Linux, absent helpers not named) and a how-to card `everyday`, which the block tells the model to read before any web search. Cards `7za`, `ffmpeg`, `ffprobe` were added for the P5b helpers.

Re-run: the six jobs whose first-run notes said the model hunted for or asked about a tool, one run each, `ACC_PATH=host` (this laptop's PATH has 7z, ffmpeg and ImageMagick; P5b is not in this branch, so the embedded toolbox was not used). Record: `results/p5c/chatgpt.gpt-5.6-luna-windows-host-path.json`.

| job | before (run 1 / run 2) | after |
|---|---|---|
| sevenzip_make | PASS 11 s, 5 calls / PASS 9 s, 3 calls | PASS 15 s, 5 calls |
| sevenzip_extract | PASS 158 s, 15 calls / PASS 218 s, 10 calls | PASS 16 s, 3 calls |
| png_to_webp | PASS 34 s, 8 calls / PASS 98 s, 13 calls | PASS 9 s, 2 calls |
| image_resize | PASS 27 s, 7 calls / PASS 30 s, 6 calls | PASS 26 s, 5 calls |
| video_length | PASS 61 s, 6 calls / PASS 44 s, 7 calls | PASS 9 s, 1 call |
| extract_audio | PASS 43 s, 6 calls / FAIL 57 s, 10 calls | PASS 16 s, 4 calls |

6 of 6 passed. Not verified: one run per job (a sample, not a rate); the "before" runs used the clean PATH, the "after" run the host PATH, so part of the gain may be the helpers being on PATH rather than the block (not separated); Qwen and Linux not re-run; the prompt's effect on a machine with none of these helpers was only unit-tested.

## Files

- `tests/acceptance/basic/`: `jobs.yaml`, `fixtures/`, `spec.go`, `setup.go`, `check.go`, `runner.go`, `report.go`, `spec_test.go`, `acceptance_test.go`, `README.md`, `results/`.

## P5b re-run: the "tool missing" jobs after the toolbox grew (2026-10-31, Windows, ChatGPT Luna)

Toolbox build with 7za, ffmpeg and ffprobe embedded (docs/TOOLBOX.dual-track.md), clean PATH, same model as P5a. Records: `tests/acceptance/basic/results/p5b/`. The six jobs that used the missing programs, **6 of 6 passed**, each with the helper called by plain name from the toolbox:

| job | P5a Luna (run 1 / run 2) | P5b |
|---|---|---|
| sevenzip_make | PASS / PASS (158 to 218 s class, 10 to 15 calls, disk hunt) | PASS, 15 s, 4 calls |
| sevenzip_extract | PASS / PASS | PASS, 20 s, 3 calls |
| png_to_webp | PASS (34 s, 8 calls) / PASS (98 s, 13 calls) | PASS, 18 s, 2 calls (`ffmpeg -y -i ...`) |
| image_resize | PASS (27 s, 7 calls) / PASS (30 s, 6 calls) | PASS, 211 s, 8 calls (slower: asked fieldkit_discover first, see P5c) |
| video_length | PASS (61 s, 6 calls) / PASS (44 s, 7 calls) | PASS, 14 s, 1 call |
| extract_audio | PASS (43 s, 6 calls) / FAIL (57 s, 10 calls) | PASS, 25 s, 3 calls |

One run per cell, so this is a sample. The earlier numbers for 7z are from the report text above. Not verified: a Linux re-run (Zero sends this to the VAIO: build with `bash scripts/fetch-toolbox-extras.sh OUT` then `bash scripts/build-toolbox.sh OUT`, run `ACC_JOBS=sevenzip_make,sevenzip_extract,png_to_webp,image_resize,video_length,extract_audio` with `ACC_EXE` set to that build and `ACC_EMBEDDED=1`; note `BuildExe` embeds only on Windows today); a bare machine; whether `image_resize` is slow because of the tool or the model.
