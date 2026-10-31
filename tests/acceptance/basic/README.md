# Basic acceptance suite (plan row P5a)

35 everyday jobs a person would type, run against the real program with a real
model, each checked on the disk or in the answer text. No model judges a model.
Results and the reading of them: `docs/ACCEPTANCE-BASIC.md`.

| file | what |
|---|---|
| `jobs.yaml` | the jobs: setup (files, fixtures, zips, a helper service), prompt, checks |
| `fixtures/` | committed binaries: `ocr.png`, `photo.png`, `exif.jpg`, `clip.mp4` (7 s, with sound), `sample.7z` |
| `spec.go`, `setup.go`, `check.go` | load, make the scratch folder, run the checks |
| `runner.go` | build the exe once, run `gorilla-opencode -p PROMPT -c FOLDER -f json -q`, read the receipt |
| `report.go` | tables from `results/*.json` and `results/causes.yaml` |
| `spec_test.go` | ordinary unit tests (no model, no network) |
| `acceptance_test.go` | the model runs, behind the build tag `acceptance` |

## Run

```
fieldkit guard -- go test ./tests/acceptance/basic                      # unit tests only
fieldkit guard -- go test -tags acceptance ./tests/acceptance/basic -run 'TestAcceptance$' -v -timeout 3h
go test -tags acceptance ./tests/acceptance/basic -run TestRenderReport # results/tables.md
```

Environment (all optional): `ACC_MODELS` (comma list, default
`chatgpt.gpt-5.6-luna`), `ACC_JOBS`, `ACC_ROOT` (scratch root; use
`fieldkit scratch new` on Windows), `ACC_EXE` + `ACC_EMBEDDED=1` (reuse a
build), `ACC_HELPERS`, `ACC_PATH` (`clean` default, or `host`), `ACC_VARIANT`
(`fresh-profile`), `ACC_TIMEOUT`.

## How it works

- **Permissions.** `-p` already approves every ordinary action for its session
  and refuses the carve-outs (irreversible commands, outside-folder writes) with
  a logged reason, because nobody can answer. The suite changes no permission
  default. A refusal shows in the receipt as `refused, not run` and is scored
  "tool refused by permission or guard".
- **Model.** Pinned per run in the job folder's `.gorilla-opencode.json` (the
  only per-folder setting a folder may make). Your own config is not touched.
  That file is visible to the model as a hidden file in the job folder.
- **PATH.** `clean` (default): only the Windows system folders, plus what the
  program itself puts on its shell's PATH (the embedded toolbox). `host`: this
  machine's PATH.
- **Variants.** `fresh-profile` moves USERPROFILE, HOME, APPDATA and LOCALAPPDATA
  to an empty folder so scoop/winget/pip installs under the profile are not
  where the model looks. Config (login, MCP servers) is kept via
  `XDG_CONFIG_HOME`. Programs under `C:\Program Files` can still be found by
  full path.
- **Record per job.** Pass/fail with the reason of every check; the program's own
  receipt (tool name, the one key argument clipped to 90 characters, outcome);
  wall time; exit code; stderr tail; which `needs` programs were on PATH; a
  first-guess cause. Causes are confirmed by hand in `results/causes.yaml`
  (`MODEL/JOB: {cause, note}`), which overrides the guess.
- **Cause labels.** `tool missing` / `model did not know the tool exists` /
  `tool refused by permission or guard` / `model error`.
- **Linux.** The same file runs there: `go test -tags acceptance` builds a plain
  exe (no toolbox exists on Linux), `hidden:` needs a dot name (the jobs use one),
  `recycled` looks in `~/.local/share/Trash`.

## Leftovers

`delete_to_bin` puts one tiny file named `acc-recycle-8841.txt` in the real
Recycle Bin / trash per run. The scratch root is cleaned with
`fieldkit scratch clean`.
