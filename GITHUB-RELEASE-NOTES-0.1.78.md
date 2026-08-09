## Web search that needs no setup

`source: web` now falls back to **lynx** — the text browser from 1992, 641 KB,
already in Debian. No account, no API key, no card, no service to run.

lynx is **not bundled**. It is a `Recommends:`, pulled from Debian's own
repository, so Debian ships its security updates rather than us. The whole
feature costs about **13 KB** of download.

```sh
sudo apt install ./Compiled.Builds/gorilla-opencode_0.1.78_amd64.deb
```

**Use `apt`, not `dpkg -i`** — dpkg does no dependency resolution, so it silently
skips lynx.

## Every model says what it costs, and what it is good for

```
FREE — NVIDIA Nemotron 3 Ultra — open frontier-reasoning model …
$0.04/$0.14 per 1M — UNTESTED for coding work — use at your own risk
$0.8/$1.6 per 1M — shit tier for code — vendor calls it roleplay (vendor: "roleplay")
$15.0/$75.0 per 1M — CAN CODE (vendor: "agentic coding")
```

Free models were labelled before and paid ones were not, so telling them apart
meant knowing that silence means paid — and **260 of 274 entries were silent**.

The verdicts are built in four layers, each traceable:

1. **Earned** — we used it, with a citation to where that was recorded. A test
   fails the build if a verdict has no evidence.
2. **Curated** — a judgement already written for the same underlying model.
3. **Vendor's claim** — with the word that triggered the label quoted, so you can
   check it rather than trust us.
4. **Nothing known** — said plainly.

This matters more than it sounds: looking up what `inclusionai/ling-3.0-tiny` is
means a web search plus a heavy vendor page, and on a single-digit-KB/s
connection that is not inconvenient, it is impossible.

## A personal shortlist

**space** bookmarks a model, **b** jumps to your list, **space** again removes
it. It spans every provider, so what you actually use sits in one place instead
of being hunted for among hundreds.

## Fixed

- **OpenRouter was broken.** Nine of its 22 models had been retired by the
  provider and a tenth could not call tools — including the two used as defaults
  for every agent, so configuring it produced something that could not answer at
  all. Now generated from the live catalogue: 274 usable models, 67 dropped for
  being unable to call tools, 59 dropped as asynchronous batch endpoints.
- **The model list wrapped forever.** At 128 models you could scroll past the top
  without noticing. Both ends are hard now.
- **The system prompt described tools that were switched off** — including
  "never say you cannot reach a page" while the fetch tool was disabled, which is
  an instruction to fabricate.
- **The environment block showed no source code.** ASCII sort put 13
  release-notes files first and the 25-entry budget was spent before reaching
  `cmd/` or `internal/`.
- **Groq shipped Go identifiers as model names** — `Llama4Scout`,
  `DeepseekR1DistillLlama70b` — with no descriptions.

## Also

`gorilla-opencode models refresh` downloads the current catalogue yourself,
without waiting for a release. `models check` asks your providers whether the
models we list still exist.

## Notes

Nothing breaks. If you tried OpenRouter before and concluded it did not work — it
genuinely did not, and it does now.

Not verified: search-engine reliability over time (they rate-limit; failures are
reported as failures), the lynx parser against Ecosia and Marginalia layouts
outside the live path, and 251 of 274 OpenRouter models have never been run here
— which is exactly why their labels quote the vendor rather than claiming
first-hand knowledge.
