# The cost meter in the status bar — dual-track documentation

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the other.

It covers the work after v0.1.149 (2026-10-21): the sourced price table, the cost detail and warning in the status bar, and the Gemini and Antigravity token counts.

---

# Track one: plain language

## What happened

Every time you send a message, the program sends the whole conversation to the AI again, not only your new message. A long conversation therefore costs more with every message. In one measured 64-hour session, 72% of a $303 bill was spent sending the same old conversation again and again. Messages sent late in that session cost almost four times as much as early ones.

The status bar already showed a cost. It now also shows how much of that cost was spent re-sending old conversation, and what the last message cost. When a session passes $5 (you can change this), the cost turns red and a warning tells you to finish the job and start a fresh session.

## Honest state of play

Done and tried on the real program, on Gemini 3.6 Flash through Antigravity: the cost of a two-call session was exactly what the price page gives by hand ($0.020061).

Not tried on a real program: the warning on a long session, Anthropic's own prices (no Anthropic key on the test computer), and the re-reading share (the short test session had no re-sent conversation for Google to report). All three are covered by tests with made-up numbers.

## Worst case if something is wrong

The prices are copied from the providers' price pages on 2026-10-21. If a provider changes its prices, the meter shows the old ones until a newer build carries the new ones. It never looks prices up by itself: that would be one more connection from your computer to someone else's. Two Google prices are already known to change: Gemini 3.8 and 3.6 Flash on 1 January 2027 and Gemini 4 Argon on 1 December 2026. Those dates are in the table, so the meter switches on the right day.

A model the table does not know (for example `gemini-flash-latest`, a name that points to a different model as Google updates it) is shown as "unpriced" instead of being given a guessed price. The cost then reads lower than the truth, and the status bar says how many messages were left out.

## What changed for you

**The cost in the status bar**
- Before: `Cost: ~$4.82 est`
- After:  `Cost: ~$4.82 est (38% re-reading, last call $0.056)`. On a narrow window (80 columns) the part in brackets is left out and the cost stays.

**A warning before it hurts**
- Before: nothing.
- After:  past $5, the cost turns red and stays red, and a message says what the last message cost and that a fresh session would cost less per message. It comes again at $10, $15 and so on, once each.

**Gemini and Antigravity token counts**
- Before: the part of the conversation Google had kept from last time (its cache) was counted twice, and the AI's thinking was not counted at all.
- After:  each counted once, as Google's own documentation describes.

## What you can do now

- Change the warning: put `"costWarnUSD": 10` in your settings file for $10, or `0` to switch it off.
- Read the "% re-reading" figure: if it is high, a fresh session is the cheapest thing you can do.

## What is still missing

- Rolling model names (`gemini-flash-latest`, `gemini-pro-latest`) are unpriced. Google tells the program which real model answered, and a later change can price by that.
- The model picker and the helper planner still use the older typed prices.

## How to check that the new parts work on your computer

1. Start a conversation and send two messages.
2. The status bar shows `Cost:` and, after the first message with a price, a bracket with "last call".
3. To see the warning without spending money, set `"costWarnUSD": 0.001` in your settings and send one message. Set it back afterwards.

## Should you be concerned?

No new connection was added. Prices travel inside the program file. The settings file gains one number.

## Glossary

- **Re-reading**: the part of each message's cost that pays for sending the earlier conversation again (the provider calls it "cache reads").
- **Unpriced**: a message on a model for which no price was published; counted, not guessed.

---

# Track two: developer

## Problem Being Solved

`agent.TrackUsage` priced a call with four flat per-model numbers (`models.Model.CostPer1M*`), typed by hand, undated, without long-prompt tiers, with `CostPer1MInCached` holding the cache-write rate and `CostPer1MOutCached` the cache-read rate. The footer showed one total, with nothing to say why a session was expensive. The Gemini-shaped providers also mis-stated usage: `promptTokenCount` includes `cachedContentTokenCount` (genai v1.64.0 `types.go`), and `thoughtsTokenCount` is outside `candidatesTokenCount`.

## Approach Taken

Port of Fieldkit's `session-cost` kit (`local/kits/TokenAudit`): the same `prices.json` (embedded), the same rules and test figures.

## Before

Footer `Context: 12.8K, Cost: ~$0.02 est`; Gemini input counted cached tokens at full price plus again as cache reads; no thinking tokens.

## After

Footer `Context: 12.8K, Cost: ~$0.02 est (last call $0.010)`; red past `costWarnUSD`; one warning per step. Gemini usage: input = prompt − cached, cache read = cached, output = candidates + thoughts.

## Known Alternatives

Fetching prices at run time (rejected: a network door, and the meter must work offline). A per-model price in the catalogue only (rejected: no date, no source, no tiers).

## Files Changed

- `internal/llm/cost/` (new): `cost.go`, `prices.json` (embedded), `cost_test.go`.
- `internal/llm/agent/callcost.go` (new): `callCost`, `addCall`; `agent.go`: both cost sites use them.
- `internal/llm/provider/gemini_usage.go` (new), `gemini.go`, `code_assist.go` (`caUsage`), `antigravity.go`.
- `internal/db/migrations/20261021120000_add_cost_ledger.sql`: `cost_reread`, `calls_priced`, `calls_unpriced`, `last_call_cost`; `sql/sessions.sql`, `sessions.sql.go`, `models.go` changed to match (sqlc v1.29.0 layout, edited by hand: sqlc needs cgo on this Windows machine).
- `internal/session/session.go`: four fields, saved and loaded.
- `internal/config/config.go`: `costWarnUSD` (default 5; no `omitempty`, so an explicit 0 survives a save).
- `internal/tui/components/core/costline.go` (new), `status.go`.

## Decisions Made

- Tables only for providers whose price page was read (`anthropic`, `gemini`, `antigravity`). A model missing from its table is unpriced, not given the catalogue's number: several catalogue numbers disagree with the page (3.6 Flash $1.50/$7.50 typed vs $0.75/$3.75 published until 2026-12-31). Other providers keep the catalogue estimate unchanged.
- Antigravity is priced at API rates, Claude there from Google Cloud's table (`API_PROVIDER_ANTHROPIC_VERTEX` in Antigravity's own descriptor).
- Every Anthropic cache write is a 5-minute write: OpenCode sends no TTL, and anthropic-sdk-go v1.4.0 reports no 5m/1h split.
- Existing session rows start at 0 in the new columns: a back-fill would be a guess.

## Tried and Abandoned

Installing sqlc to regenerate: needs cgo on Windows.

## ⚠ Claimed But Not Verified

- That the Antigravity endpoint sends `thoughtsTokenCount` and `cachedContentTokenCount`. The parser handles both (unit test); the live run did not show either field's effect.
- Anthropic pricing on a live call (no key here).

## Open Items

- Price rolling aliases by the response's model version.
- `internal/config/loadout.go` and the model dialogs still use `CostPer1M*`.
- `TestAnAmpersandChainFailsAtOnceWithTheRealReason` (shell) failed once under the full parallel suite (7.6 s) and passed alone and on a package re-run: timing-sensitive, not touched by this change.

## How to verify this work is correct

    go test ./internal/llm/cost/ ./internal/llm/agent/ ./internal/llm/provider/ ./internal/tui/components/core/ ./internal/db/
    python -m fieldkit mutate --plan %USERPROFILE%\Documents\Scripts\Fieldkit\local\kits\TokenAudit\opencode-mutations.yaml -- go test ./internal/llm/cost/ ./internal/llm/agent/ ./internal/llm/provider/ ./internal/tui/components/core/

Live: a `-p` run on `antigravity.gemini-3.6-flash-medium` with `XDG_DATA_HOME` in a temp folder wrote `cost 0.020061, calls_priced 2, cumulative 25,198 in / 310 out`; 25,198 × $0.75/M + 310 × $3.75/M = $0.020061.

## Technical Debt

`sessions.sql.go` is hand-edited in sqlc's layout; the next `sqlc generate` must produce the same file (columns in table order).

## Claim Sources

- Prices: https://platform.claude.com/docs/en/about-claude/pricing, https://cloud.google.com/vertex-ai/generative-ai/pricing, https://ai.google.dev/gemini-api/docs/pricing (fetched 2026-10-21; sha256 in `prices.json`).
- 72% / $303 / $0.189 vs $0.051 per call: Fieldkit `session_cost.py` on Claude Code session 98ea1ad9, 2026-10-21.
- Gemini usage fields: `google.golang.org/genai@v1.64.0/types.go`, `GenerateContentResponseUsageMetadata`.
