# v0.1.160: toolbox pinned for linux-amd64 and darwin-amd64, parityBacklog emptied, Linux P5b/P5c/P4b re-run, CI tests touched packages

**Date:** 2026-11-01
**Previous version:** v0.1.158
**Commit range:** `v0.1.158..52965be` (v0.1.159 was tagged without its Changelogs notes and never published) (PRs #53, #54, #56; the CI change 7a8084a is carried on main with them)

---

## Summary

- Linux toolbox (#54, d699af1): `ast-grep`, `coreutils`, `difft`, `hyperfine`, `ninja`, `sd` (musl builds made on the VAIO, task T-20261027-012841-ee4400) and `pfind` pinned for linux-amd64 in `internal/toolbox/SHA256SUMS`; `parityBacklog` in `internal/toolbox/parity_test.go` is now empty.
- Darwin toolbox (#56, a46c229): all 13 helpers pinned for darwin-amd64 (Intel, 10.13 floor); the parity test covers windows-amd64, linux-amd64 and darwin-amd64. Pins in `third_party/PINS.md`.
- Linux re-run (#53, fc6e5bb): P5b/P5c/P4b on the VAIO, Luna, 16 of 16 jobs; results under `tests/acceptance/basic/results/`.
- CI (7a8084a): PRs and feat/fix pushes run `go test` for the touched packages and their dependents; main and tags run the full suite.

## Test coverage and CI

Each PR ran its CI on its own head. The release build runs vet and the whole test suite once (`fieldkit emerge`, stage `test`).

## Security posture

No new network channel and no telemetry. The new helper binaries are pinned by version and SHA-256 and embedded; none is fetched at run time.

## Deployment

```
sha256sum -c SHA256SUMS-v0.1.160.txt --ignore-missing
gorilla-opencode --version
# Expected: v0.1.160
```

## Not verified

- macOS: the darwin helpers are pinned and hashed; no job was run on a Mac.
- One run per job is a sample, not a rate.

## Known issues

- The Windows exe is unsigned.
- The write tool still allows writing one folder above the working folder (owner decision pending).
