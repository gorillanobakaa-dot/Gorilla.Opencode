## The agent can search the open web — if you run your own search engine

`web_search` gains `source: web`, backed by a **self-hosted SearXNG**. Set
`searxngURL` in `config.json` or export `SEARXNG_URL`. No container needed —
SearXNG runs from source; setup is in the
[full release notes](Changelogs/v0.1.75-release-notes.md#setting-up-searxng).

SearXNG is not a preference, it is what is left. As of August 2026 Google's
Custom Search JSON API is closed to new customers, Microsoft retired the Bing
Search API, Brave wants a credit card even for its free credit, and Mojeek is
sales-gated. Self-hosting is the only remaining path to key-free general web
search — and it costs nothing, needs no account, and logs nothing about you.

It also fits this project better than a paid API could. SearXNG reports
`unresponsive_engines` per query, so **"nothing matched" and "every engine was
blocked" are told apart**. The second is returned as an error, never as an empty
result set — a model told "nothing found" concludes the thing does not exist,
and that conclusion is where fabricated citations come from.

If SearXNG is not configured, the tool says so and tells the agent to ask you
for a URL. It refuses *before* the permission prompt: approving a search that
cannot happen only teaches you to approve without reading.

## The prompt stops advertising tools you switched off

Prompt lines can now carry `[[needs tool.x]]` and vanish when that component is
disabled in `/context`.

Before this, turning a tool off removed its schema but left the prompt
describing it. The worst case: with `tool.fetch` off, the agent was still told
*"never say you cannot reach a page"* — an instruction to fabricate, at exactly
the moment it genuinely could not. Low-bandwidth mode is one keypress and turns
off five tools at once, so this was reachable by accident.

## Notes

Nothing breaks. `source: web` is additive and every existing source is
unchanged. If you never configure SearXNG, the only difference is that the agent
now says web search is unavailable instead of not knowing the option exists.

Verified end-to-end against a real SearXNG (8 results, plus a live
`startpage (Suspended: CAPTCHA)` degradation caught and surfaced). **Not**
verified through the TUI or a live agent loop, and the behavioural effect of the
prompt change on a real model is unmeasured — only its assembly is tested.
