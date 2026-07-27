# Working on gorilla-opencode

A Go fork of OpenCode: a Bubble Tea TUI coding agent. Target machine is a Sony VAIO
SVE (i7-3632QM, 16 GB, Intel HD 4000, Debian 13) — assume modest hardware and a
possibly high-latency link.

Mark every intentional divergence from upstream with a `GORILLA OVERRIDE:` comment
that says **why**, not what. Future readers need the reason; the diff already shows
the change.

## Releasing — the checklist

Follow it in order. Steps 6 and 7 exist because they were forgotten and it cost
real confusion; they are not optional.

1. **Tag first, then build**, so the version stamp is real:
   ```sh
   git tag -a vX.Y.Z -m "…"
   go build -ldflags "-X github.com/opencode-ai/opencode/internal/version.Version=vX.Y.Z" -o gorilla-opencode .
   ./gorilla-opencode --version    # must print vX.Y.Z
   ```
2. **Dual-track docs** via `dual_track.py` (see "Local-only tooling" below):
   `release prep` → fill both JSONs → `release render --validate`. **`--validate` is
   opt-in**, so it is easy to skip by accident — never skip it. Mark claims honestly:
   `stated_in_input` needs real evidence, everything else is `model_inference`.
   State what was **not** verified.
3. **Prepend to `Changelogs/CHANGELOG.md`** (newest first), including a
   `**Plain-language version:**` paragraph.
4. **Build and inspect the .deb**:
   ```sh
   scripts/build-deb.sh X.Y.Z
   dpkg-deb -c gorilla-opencode_X.Y.Z_amd64.deb | grep release-notes   # this release's notes must be there
   ```
5. **Checksums** over every artifact, then `sha256sum -c` to prove they verify.
6. **Install the .deb — do not `install` the binary by hand.** A manual copy to
   `/usr/bin` leaves dpkg's database reporting the *old* version, so
   `gorilla-opencode --version` and `dpkg -l gorilla-opencode` disagree and nobody
   can tell what is actually installed. Verify both agree:
   ```sh
   sudo dpkg -i gorilla-opencode_X.Y.Z_amd64.deb
   gorilla-opencode --version && dpkg -l gorilla-opencode | tail -1
   ```
7. **Fast-forward `main` to the release tag, always.**
   ```sh
   git checkout main && git merge --ff-only vX.Y.Z && git push origin main
   ```
   `main` is the default branch, so it is what GitHub displays and what `git clone`
   hands out. Releasing from a feature branch without doing this leaves the shop
   window showing old stock: on 2026-07-27 `main` sat 43 commits behind at v0.1.37
   while v0.1.41 was published, so anyone building from source got something four
   releases old. Downloads still worked (assets attach to *tags*), which is exactly
   why the drift went unnoticed. If the merge is not a clean fast-forward, stop and
   ask — do not resolve it silently.
8. **GitHub release** with all artifacts attached, then **download them back** and
   re-verify the checksums. Publishing is not proof of a good upload.
9. Link release notes by **tag** (`blob/vX.Y.Z/…`), never by `main` — a `main` link
   breaks the moment the file moves.

## Verification standards

- **Never claim a fix works without exercising it.** Drive the real path and show
  the output.
- **Tests must be non-vacuous**: restore the old behaviour and confirm the test
  fails. A test that passes against the bug is worse than none.
- **TUI work needs headless render assertions** — build the component in-package,
  call `View()`, assert `lipgloss.Width()` per line and the row count. Never fix a
  visual bug by guessing at a screenshot.
- **Interactive TUIs cannot be driven from an agent shell here.** A *minimal* Bubble
  Tea program also fails to receive piped-PTY input, so it is environmental. Assert
  layout and logic headlessly, and tell the user plainly which interactive parts
  they must confirm themselves.
- **Never test config-writing code by running the built binary against the live
  config.** That has overwritten the user's real `~/.config/gorilla-opencode/`
  twice. Unit-test against an isolated `XDG_CONFIG_HOME` with a guard that panics
  if the path is not the temp one (`internal/config/main_test.go`).
- **Mask credentials in diagnostic output** — print a length and a short prefix,
  never the value. A live API key once went into a transcript in full.

## Traps that have already cost rework

- **Nothing may write to stdout/stderr while Bubble Tea owns the screen.** The text
  is painted over the frame with no record in the renderer, so no redraw can ever
  clear it. Route it through a `tea.Msg`, or log it. Grep `fmt.Print` under
  `internal/` before releasing; the only legitimate survivors are the `-p` output
  path and the deliberate mouse-mode escape sequences.
- **Container chrome is SUBTRACTED from the terminal size, never added to a content
  size.** Adding it shipped an invisible input box and four over-wide dialogs.
- **viper reads `mapstructure` tags, not `json` tags.** `Config.WorkingDir` had only
  `json:"wd"`, so the field was written and never read back — a write-only setting.
  Field names also match case-insensitively, which is why `additionalDirs` worked
  and `wd` did not.
- **`omitempty` on a bool discards the "off" choice.** Store the negative when the
  default is true (`skipWorkspacePrompt`, not `askWorkspaceOnStartup`).
- **In the loadout state map an ABSENT key means ENABLED.** Never flip the raw map
  value — the zero value makes the first press a no-op. Read with that rule first.
- **A successful `/v1/models` listing does not prove a credential works.** NVIDIA
  answers it unauthenticated. Only an auth-gated call proves a key.
- **Endpoints sharing a `baseURL` overwrite each other's model routes**, last one
  wins. Collapse by URL, prefer a keyed entry, keep the first of two.
- **Scroll that follows the selection makes non-selectable trailing rows
  unreachable.**
- **Loadout/registry globals leak between tests in a package.** Helpers must replace
  the rows they own and restore both registry and state; assert persistence by
  reading the file, not by clearing global state. `sync.Once` cannot be reset —
  replace it.

## Local-only tooling (deliberately untracked)

`/scripts/` and `/Documentation.Scripts/` are in `.gitignore` by an earlier security
decision. **Do not commit them.** They exist on the developer's machine:

- `scripts/build-deb.sh` — packaging
- `scripts/setup-lsps.py` — language-server installer
- `Documentation.Scripts/Documentation.Writing.Scripts/dual_track.py` — release docs

`gorilla-opencode` (the built binary) and `*.deb` are also ignored; they are build
outputs and must never be committed.

The developer keeps deliberate redundant backups and vaults as a defence against
destructive tooling. Never treat backup redundancy as cruft to clean up.
