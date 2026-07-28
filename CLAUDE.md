# Working on gorilla-opencode

A Go fork of OpenCode: a Bubble Tea TUI coding agent. Target machine is a Sony VAIO
SVE (i7-3632QM, 16 GB, Intel HD 4000, Debian 13) — assume modest hardware and a
possibly high-latency link.

Mark every intentional divergence from upstream with a `GORILLA OVERRIDE:` comment
that says **why**, not what. Future readers need the reason; the diff already shows
the change.

## Releasing — read this first

**There is a script: `Documentation.Scripts/Documentation.Writing.Scripts/release_pipeline.py`,
with a `go_gorilla` profile built for this repo.** It knows the ldflags path, calls
`scripts/build-deb.sh`, uploads assets with checksums, and can purge and reinstall
(`--action install-purge`). It is resumable — it recovers state if an earlier run was
interrupted. Start with `--dry-run --yes`.

```sh
python3 Documentation.Scripts/Documentation.Writing.Scripts/release_pipeline.py \
  --profile go_gorilla --version X.Y.Z --dry-run --yes      # see the plan
```

It was undocumented until 2026-07-28, and the cost of that was four consecutive
releases driven by hand, one step at a time, by agents who had no idea it existed.
If you add tooling, put it here.

**What the script does NOT do**, and you must:
- Write the dual-track docs and the changelog. It can call `dual_track.py`, but the
  *content* — honest claim sourcing, what was not verified — is judgement.
- **Inspect the built `.deb`.** `dpkg-deb -c` and confirm this release's notes are in
  it and the inner binary hash matches. This is how the missing plain-mode desktop
  action was caught in v0.1.44, after the source was correct but a third copy of the
  launcher inside the packaging script was not.
- Verify non-vacuously that whatever you fixed is actually fixed.
- Its resume logic can decide the state is already `INSTALLED` and skip phases,
  including git operations. Read the log; do not assume a quiet run did the work.

**Its state detection was broken in four ways, all fixed on 2026-07-28.** It infers
"how far along is this release" from side-effects, and every inference was wrong:
- `verify_cmd` succeeding meant `INSTALLED` — i.e. ANY build on `$PATH`. So a release
  after the first skipped every phase and then failed verification. **This is the most
  likely reason earlier sessions abandoned the script.**
- A binary existing in the tree meant `BUILT`, without asking its version — so it
  packaged last release's binary under the new number.
- A `.deb` NAMED for the version meant `PACKAGED`, without checking its contents. This
  one actually published: a package whose control file said 0.1.46 containing a binary
  that reported v0.1.45. Caught by extracting the artefact, after upload.
- A tag existing meant `COMMITTED` ⇒ built and packaged. But step 1 of this checklist
  says TAG FIRST, then build, so the tag routinely exists before any binary — build and
  packaging were skipped and upload failed with "Asset not found".

All four now verify what they assume. The lesson generalises: **an artefact's name is
not its contents, and a side-effect is not a state.**

Two more guards were added the same day, both from real incidents:
- **It refuses to commit deletions** (`--allow-deletions` to override). `git add -A`
  had no such guard, and nine files of published research were sitting deleted in the
  working tree at the time — one release away from being baked into a tag.
- **It fast-forwards `main` to the tag**, only ever as a fast-forward. It previously
  pushed just the current branch, which is how `main` sat 43 commits behind.

## Releasing — the checklist

The script covers much of this. Follow it in order when working by hand, and use it
to check the script's log. Steps 6 and 7 exist because they were forgotten and it
cost real confusion; they are not optional.

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
- **Configure the logger before anything in `Load` can log.** Until
  `slog.SetDefault` runs, slog's *built-in* default handler is in force and it
  writes to **stderr** — so an early `logging.Warn` lands on the terminal and is
  then painted over by the TUI, exactly like a stray `fmt.Print`. It is the same
  bug with a different mechanism, and grepping for `fmt.Print` will not find it.
  `configureLogging()` is called immediately after `applyDefaultValues()`; never
  add a step that logs above that call.
- **Container chrome is SUBTRACTED from the terminal size, never added to a content
  size.** Adding it shipped an invisible input box and four over-wide dialogs.
- **lipgloss `.Width(w)` WRAPS text longer than `w`; it does not overflow.** So the
  symptom of an untruncated long string is extra **height**, not extra width — a
  width assertion passes against it. Test the height. (I wrote the width version
  first and it passed against the bug.)
- **`viper.ConfigFileUsed()` stays empty for the whole process if no config.json
  existed at startup** — nothing re-runs `ReadInConfig`. `updateCfgFile` keyed
  "does a config exist?" off it and substituted a literal `{}`, so on a fresh
  install every write re-based from empty and discarded the one before: paste a
  key in `/connect`, add an endpoint, key gone. Read the file from disk regardless,
  and `viper.SetConfigFile` after creating it.
- **Unregister local models by endpoint NAME, not baseURL.** Several configured
  endpoints can aim at one URL and only one of them owns the registered models, so
  dropping by URL takes the survivor's models down with the entry being removed.
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
- **The desktop entry passes NO arguments** (`Exec=gorilla-opencode launch`), and
  clicking the icon is how most people start this program. Anything reachable only
  by typing a flag is a capability those users do not have. This has now landed
  twice: the working directory defaulting to `$HOME`, and plain mode shipping as
  `--plain` only in v0.1.43. Every user-facing capability needs a route that does
  not involve typing — a persisted setting, a slash command, or a Desktop Action.
- **The launcher exists in THREE places** — `packaging/gorilla-opencode.desktop`,
  the `desktopEntry` string in `cmd/install.go`, and (until v0.1.44) a third copy
  inlined in `scripts/build-deb.sh`. Adding the plain-mode action updated the first
  two and missed the third, so the built `.deb` shipped a launcher with no action.
  The script now installs the tracked file; `cmd/desktop_entry_test.go` holds the
  remaining two in step. **Always extract the built `.deb` and check the artefact,
  not the source.**
- **viper reads a DOTTED map key as NESTING.** An `extras` map keyed
  `"extra.timestamps.show"` unmarshalled as `{extra:{timestamps:{show:true}}}` and
  then failed to decode into `map[string]bool`, breaking `config.Load` for the
  whole application. Keys of a viper-backed map must contain no dots
  (`extras-timestamps-show`). The loadout registry gets away with dotted IDs only
  because its state lives in `loadout.json`, written directly and never read by
  viper.
- **Any test package that calls `config.Load` MUST isolate the config**, via
  `os.Exit(configtest.Isolate(m))` in `TestMain`. Without it, every setter
  (`SetExtra`, `SetWorkingDir`, `UpsertProviderKey`, `AddDir`) writes through
  `updateCfgFile` to the developer's real `~/.config/gorilla-opencode/config.json`.
  `internal/config` had a guard; four other packages did not, and one new writing
  test duly put a stray key in the live config. That is three times now. The guard
  panics rather than falling back, because silent damage is worse than a failed run.
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
- `Documentation.Scripts/Documentation.Writing.Scripts/release_pipeline.py` — the
  release pipeline; see "Releasing — read this first" above. **Inventory this whole
  directory before building any release tooling of your own.**
- `Documentation.Scripts/Code.review/code_review_toolkit/` — a code-review toolkit
  (`code_review.py`, `tools_registry.py`, a PLAYBOOK). Not yet used by any session;
  read it before hand-rolling a review process.

`gorilla-opencode` (the built binary) and `*.deb` are also ignored; they are build
outputs and must never be committed.

The developer keeps deliberate redundant backups and vaults as a defence against
destructive tooling. Never treat backup redundancy as cruft to clean up.
