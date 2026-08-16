## The assistant now offers to set up web search

v0.1.75 made web search possible. Almost nobody was going to act on it — it
required knowing to run your own SearXNG.

Now the assistant offers. Ask it something it needs the web for and, instead of
"not configured, here are the instructions", it says it can set it up for you:
no account, no API key, no card, entirely on your own machine, a couple of
minutes. Say yes and it runs one installer. Say no and it asks you for a link,
exactly as before.

## The installer is shipped, not improvised

`/usr/share/gorilla-opencode/setup-searxng.sh`. The refusal text explicitly tells
the model **not** to improvise the steps or retype them from memory.

That isn't caution for its own sake. Handed an earlier version of those
instructions as plain text and asked to relay them, a model dropped `pyyaml` from
one pip line — one of only two traps in the whole procedure — while *merely
paraphrasing*. A model performing the install would make the same class of
mistake and then report success, and a half-working SearXNG is worse than none:
it answers, and the agent summarises whatever it answered with.

The script encodes both traps (`pip install -e .` fails before msgspec/pyyaml
exist; `json` is absent from `search.formats` by default, which is why public
instances return 403), installs a systemd user service, needs no root, and writes
`searxngURL` preserving every other config key.

Then it **verifies with a live query** and requires real results back — exiting
non-zero naming the failed step rather than reporting "probably fine".

## Also in this release

**A test-isolation bug the installer exposed.** `internal/llm/tools`' `TestMain`
called `config.Load` without redirecting `XDG_CONFIG_HOME`, so `config.Get()` had
been reading the developer's real config all along. Harmless until a config value
changed behaviour — the moment `searxngURL` was written, four stubbed tests
stopped using their `httptest` servers and queried the live instance, failing
with "want 2 hits, got 8". Had it returned two, the suite would have stayed green
while testing nothing.

**A negative result, recorded.** An opt-in probe establishes that Antigravity
does not support Google Search grounding — flash returns 200 with no
`groundingMetadata`, pro returns 400 — and that the request envelope is not the
cause, since gemini-cli places `tools` identically.

## Notes

Nothing breaks. If you already set `searxngURL` by hand, nothing changes.

Verified end to end on one machine: 34 results on the installer's own check,
service enabled and active, config keys intact, then 8 real hits through
`web_search`. **Not** verified on other distributions, Python versions or
non-systemd sessions, and the `python3-venv` apt branch is unexercised. Whether
models reliably *act* on the offer is unmeasured — only the text is tested.
