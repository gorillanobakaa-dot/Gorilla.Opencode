# Contributing to Gorilla OpenCode

## Quick start

```bash
go build ./...
go vet ./...
go test ./... -count=1
```

## CI / CD

- **CI runs on every push/PR** (`.github/workflows/ci.yml`) — build, vet, test.
- **Build runs on every push to main** (`.github/workflows/build.yml`) — snapshot binaries via goreleaser.
- **Release runs on tag push** (`.github/workflows/release.yml`) — creates GitHub Release, .deb/.rpm, Homebrew tap, AUR package.

### For maintainers: enabling automated releases

The release workflow needs two repository secrets (Settings → Secrets → Actions):

| Secret | Purpose |
|--------|---------|
| `HOMEBREW_GITHUB_TOKEN` | PAT with `repo` scope (or fine-grained Contents R/W on `opencode-ai/homebrew-tap`) |
| `AUR_KEY` | SSH private key with push access to `ssh://aur@aur.archlinux.org/opencode-ai-bin.git` |

Once both secrets are added, tag a release:

```bash
git tag v0.1.XX && git push origin v0.1.XX
```

See `docs/ENABLE-CI.md` for the full story.

## Code style

- Go ≥ 1.24
- `go vet ./...` must pass
- `go test ./... -count=1` must pass
- Every change carries a `// GORILLA OVERRIDE:` comment explaining *what* and *why*

## Reporting bugs

Use the issue template. Include:
- Gorilla version (`gorilla-opencode version`)
- Provider + model
- Minimal repro steps
- Relevant logs (`OPENCODE_DEBUG=1`)