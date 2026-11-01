# What "built in" means in Gorilla OpenCode

The full definition lives in Fieldkit `docs/INTEGRATION.md`. A capability (a helper program, a command, a feature) is
**built in** only when all six hold. Otherwise it is not done.

1. **Carried.** It ships inside the program, pinned by version and sha256 (`internal/toolbox/SHA256SUMS`,
   `third_party/PINS.md`). It is never downloaded at run time.
2. **Known to the AI.** The "Helpers here" prompt block (`internal/hostprobe/helpers.go`) and a how-to card name it.
   The AI never scans the disk or the web to find it.
3. **Visible.** `/arsenal` opens on "BUILT IN (ready now)" with one line per helper (name, version, what it is for);
   `/help` names `/arsenal` and the everyday card; `/context` shows "N built-in helpers, M installable".
4. **On the release page.** The "What is built in" table is generated (Fieldkit `fieldkit/build/relbuiltin.py`) from
   `SHA256SUMS` + `internal/toolbox/helpers.json` + `PINS.md`; it is never typed by hand.
5. **On every system.** Windows, Linux and macOS, or the closest equivalent named. Helpers that need macOS 11 or newer
   (ffmpeg, ffprobe) are said plainly.
6. **Tested.** One test enumerates `internal/toolbox/SHA256SUMS` and fails if a helper is missing from the arsenal
   page, the `/context` count, `helpers.json` (the page's source) or the everyday card
   (`TestEveryPinnedHelperIsVisible`).

Adding a helper: pin it, describe it in `internal/toolbox/helpers.json`, name it in PINS.md; the screens and the page
follow. `fieldkit integration check --repo . --changed origin/main` must exit 0 before a pull request.
