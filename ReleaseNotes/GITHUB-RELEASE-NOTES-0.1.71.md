Four faults where the application misreported its own state, plus an escape hatch
for the one screen that had no way back.

**Plain-language version:** The app told you a provider was ready and then
refused to use it. Talking to Cloudflare stopped working the moment the assistant
used a tool. The filter that trims noisy build output was throwing away the error
message itself. And if you picked the wrong provider at startup there was no way
back except quitting.

### Fixed

- **Environment-variable provider keys were hidden by a stale `disabled` flag.**
  A provider that had ever been written to config was reverted on selection even
  with its `*_API_KEY` set and working — while the startup picker showed it as
  "(ready)". The picker and the validator held two different definitions of
  "configured".
- **Cloudflare Workers AI was unusable for tool-using conversations.** An
  assistant turn with no text serialised as `"content": null`, which Cloudflare
  rejects; because the message stays in history, a conversation survived exactly
  until its first tool call and failed on every turn after. Separately,
  `"tools": []` was sent when there were none, which broke session titles and
  would have broken compaction.
- **The build-log filter deleted the first line of the failure.** Unanchored tool
  names classified `ld:`, `cc1plus:` and `assertion` as progress noise, and the
  noise test was applied to the signal line itself — so `ld: cannot find -lssl`
  was discarded despite being the whole reason to keep anything.

### Added

- **`/providers`** (also `/provider`, `/switch`) reopens the launch-time provider
  picker mid-session. `/connect` keeps the detailed manager.
- **`openai/gpt-oss-20b`** in the NIM catalogue — served by the endpoint but
  previously absent entirely.
- **Verified honesty notes** on `openai/gpt-oss-20b` and
  `nvidia/nemotron-3-ultra-550b-a55b`: both held tool-sourced facts under
  sustained user pressure, 2/2 runs. Sample size is printed because two
  observations is thin — repeating the same test elsewhere changed 4 of 13
  verdicts. Evidence and every transcript:
  <https://github.com/gorillanobakaa-dot/model-eval>
- **`# precedence`** section in the coder prompt, ~160 tokens, switchable off in
  `/context`.

### Not verified

- ~~No interactive TUI run.~~ **Confirmed working by a human on 2026-08-07**:
  `/providers` reopens the launch picker as intended. Build, vet and the full
  test suite also pass.
- The Cloudflare fixes were measured against the live API on 2026-08-05 and not
  re-measured for this release — the free tier's daily allocation was exhausted.
- The prompt precedence work has a pre-registered experiment
  (`system-prompts/EXPERIMENT-PREREG-2026-08-04.md`) that has **not** been run.
  The section ships on reasoning, not on measured effect.

Full dual-track detail: [Changelogs/v0.1.71-release-notes.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.71/Changelogs/v0.1.71-release-notes.md)
