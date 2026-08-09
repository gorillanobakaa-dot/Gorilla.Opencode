## Web search that needs no setup at all

v0.1.75 made web search possible. v0.1.76 taught the assistant to offer to set it
up. This one just works.

`source: web` now falls back to **lynx** — the text browser that's been around
since 1992 — so search works with no account, no API key, no card, no service and
nothing to configure. The resolution order is:

```
SearXNG   if configured        best quality
lynx      if present           zero setup
refuse    only if both absent
```

## Is it built in? No, and that's deliberate

lynx is **not bundled** in the download. It's a `Recommends:`, pulled from
Debian's own repository — which means Debian ships its security updates, not us.

| | Size |
|---|---|
| Download grows by | **12 KB** |
| lynx from Debian, if absent | 641 KB |
| Package total | 19.2 MB |

About **0.06%**. Bundling SearXNG — a better version of the same feature — would
have added ~300 MB.

Install with `sudo apt install ./gorilla-opencode_0.1.77_amd64.deb`, **not**
`dpkg -i`: dpkg does no dependency resolution at all, so it silently skips lynx.
(Double-clicking the .deb is fine — the graphical installer handles it.)

## Measured, not assumed

External result URLs per engine through lynx:

```
marginalia 43 · brave 28 · ecosia 27 · mojeek 19
duckduckgo 0 · google 0 · startpage 0
```

DuckDuckGo and Google are permanently excluded. Google refuses text browsers
outright — *"Your browser isn't supported any more"*, with no search box at all.

**The user agent is left honest on purpose.** Claiming to be Chrome raises the
hit rate and makes failure quieter:

| | Blocked response |
|---|---|
| Honest UA | 157 bytes, **exit 1** |
| Chrome UA | 1,122 bytes, **exit 0** — a CAPTCHA reading *"Select all squares containing a duck"* |

A model handed the second summarises the CAPTCHA. So success is measured as *did
any external result URL come out* — the only check that survives every observed
failure, since a rate-limited DuckDuckGo also returns 12 KB at exit 0 with zero
results.

## Two bugs found by running it

Both produced output that looked correct. Every result came out titled *"More on
reddit.com"* — engine furniture points at genuine external URLs, so no host
filter catches it. Worse, a real title got attached to the **wrong URL**: lynx
wrapped a link label across two lines, so the parser searched forward for a title
and landed in the next result's heading. A real URL, a real title, about
different things.

Fixed structurally rather than with a longer blocklist: a marker with text
already carries its label, and looking elsewhere is never allowed. Both are
pinned by regression tests built from verbatim captures.

## Notes

Nothing breaks. Without lynx, behaviour is exactly as in v0.1.76.

Search engines rate-limit, so expect intermittent failures — reported **as
failures**, never as "no results found". Not verified: engine reliability over
time, Ecosia/Marginalia layouts outside the live path, the `sudo -n` fallback on
a machine that actually prompts, and nothing has been driven through the TUI.
