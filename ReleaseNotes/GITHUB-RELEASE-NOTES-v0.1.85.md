# Gorilla Opencode v0.1.85

This release identifies fork-owned runtime state as `gorilla-opencode` and fixes inline prompt wrapping.

## Included changes

- Routes configuration, data, cache, and state through branded XDG directories.
- Removes upstream-name runtime artifacts and legacy migration reads.
- Keeps the fork repository link at `https://github.com/gorillanobakaa-dot/Gorilla.Opencode`.
- Fixes the inline editor so Bubble Tea rebuilds wrapped rows from the current textarea value instead of reusing a stale one-row viewport.
- Adds a regression test for long prompts at the terminal-width boundary.
- Fixes the Arch package builder so it validates the compressed archive and does not publish truncated output.
- Adds plain-language and developer dual-track documentation for this work.

## Verification

- `go test ./...` passes.
- The binary reports `v0.1.85`.
- The Debian package passes `dpkg-deb` inspection.
- The Arch package passes `zstd -t` and archive listing checks.

## Assets

- `gorilla-opencode_0.1.85_amd64.deb`
- `gorilla-opencode-0.1.85-1-x86_64.pkg.tar.zst`
- `gorilla-opencode_session_layman.md`
- `gorilla-opencode_session_developer.md`
- `SHA256SUMS-v0.1.85.txt`
