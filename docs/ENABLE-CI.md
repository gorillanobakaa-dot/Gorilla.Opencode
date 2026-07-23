# Enabling GitHub Actions CI/CD

**Status: ✅ GitHub Actions is already enabled and CI runs on every push.**

The `ci.yml` workflow runs `go build ./...`, `go vet ./...`, and `go test ./...` on every push to `main` and on every PR. It passes (see recent runs on the Actions tab).

---

## What still needs doing for automated releases

The `release.yml` workflow (triggered on git tags `v*`) fails because it needs **two repository secrets** that only a repo admin can add:

| Secret | Purpose | How to get it |
|--------|---------|---------------|
| `HOMEBREW_GITHUB_TOKEN` | Publishes to Homebrew tap (`opencode-ai/homebrew-tap`) | Create a **classic PAT** at `github.com/settings/tokens` with `repo` scope, or a fine-grained PAT with `Contents: Read & Write` on the tap repo. Add it as `HOMEBREW_GITHUB_TOKEN` in **Settings → Secrets → Actions**. |
| `AUR_KEY` | Publishes to AUR (`opencode-ai-bin`) | Generate an SSH key (`ssh-keygen -t ed25519`), add the **public key** to the AUR package maintainer keys, add the **private key** as secret `AUR_KEY` in **Settings → Secrets → Actions**. |

### To add secrets (repo admin only):

1. Go to **Settings → Secrets and variables → Actions → New repository secret**
2. Add `HOMEBREW_GITHUB_TOKEN` (classic PAT with `repo` scope, or fine-grained with Contents R/W on the tap repo)
3. Add `AUR_KEY` (private SSH key for AUR push access)

Once both secrets exist, pushing a tag `v0.1.xx` will:
1. Build Linux/macOS binaries (amd64/arm64)
2. Create a GitHub Release with tarballs + checksums
3. Publish `.deb`/`.rpm` via nfpm
4. Submit to Homebrew tap (`opencode-ai/homebrew-tap`)
5. Submit to AUR (`opencode-ai-bin`)

---

## Current CI status

| Workflow | Trigger | Status |
|----------|---------|--------|
| `ci.yml` | push/PR to `main` | ✅ **Passing** — runs build, vet, test |
| `build.yml` | push to `main` | ✅ **Passing** — builds snapshot binaries via goreleaser |
| `release.yml` | push tag `v*` | ❌ **Blocked** — needs the two secrets above |

---

## One-line summary for the next maintainer

> **CI is live.** Add `HOMEBREW_GITHUB_TOKEN` + `AUR_KEY` as repo secrets (Settings → Secrets → Actions) and tag a release (`git tag v0.1.xx && git push origin v0.1.xx`) to get automatic .deb/.rpm/Homebrew/AUR releases.