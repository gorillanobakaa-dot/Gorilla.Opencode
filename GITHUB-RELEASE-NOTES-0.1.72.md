The assistant could always read web pages. It kept telling you it couldn't.

**Plain-language version:** The tool was called `fetch`, the word "web" appeared
once in its entire description and never in its name, and the system prompt never
mentioned the internet at all. Against a strong trained belief that "I am a
language model, I cannot browse the web", the assistant kept refusing an ordinary
request and giving a false reason for it.

### Fixed

- **`fetch` → `web_fetch`, and the prompt now says the capability exists.** The
  description opens with *"YOU DO HAVE INTERNET ACCESS through this tool"*, and
  one line was added under `# tools` in the coder prompt. The old name reinforced
  the wrong belief: `fetch` reads like `git fetch`.
- **SSRF: the guard moved from the URL string to the connection.** `http.Client`
  follows redirects by default, so a permitted URL could `302` into
  `169.254.169.254` and the string check never saw the second URL; a hostname
  resolving to a private IP was dialled without complaint. A dialer `Control`
  hook now validates the address about to be connected to, and `CheckRedirect`
  re-validates every hop. `TestBlockedFetchTarget` passed throughout the
  vulnerable period because it tested the layer that was never broken.
- **Silent truncation.** A page cut at the 5MB limit was handed over as if
  complete, so the assistant would confidently summarise a document it had only
  partly read. It now says so.
- Non-200 responses discarded the body, losing `Retry-After` and error JSON;
  `application/xhtml+xml` was not treated as HTML; responses were cast to string
  assuming UTF-8; PDFs came back as binary noise; `format` was a required
  argument, so `web_fetch(url=…)` hard-failed.

### Added — content negotiation

The tool sent **no `Accept` header at all**, so every site returned HTML which
was then reconstructed into markdown locally — lossy, and roughly fifty times
more data than needed. It now:

- sends `Accept: text/markdown, text/plain;q=0.9, text/html;q=0.8, …`
- rewrites `github.com/…/blob/…` to `raw.githubusercontent.com`
- tries the `.md` companion once when a docs URL answers HTML
- re-uses `ETag` / `If-Modified-Since` within a session
- strips nav, header, footer, form and script chrome before converting
- **reports which path produced the result**, so the model knows whether it read
  a source document or a reconstruction

This is invisible on fibre and worth minutes per page on a metered or
single-digit-KB/s link — and because assistants are billed by the token and
re-read the whole conversation each turn, the navigation menus were being charged
for repeatedly. The release notes explain that reasoning in full for
non-technical readers.

### Not verified

- No live fetch over a genuinely slow link; content negotiation is covered by
  unit tests, not measured end-to-end.
- Hit rates for `Accept: text/markdown` and the `.md` companion are unmeasured.
- The prompt line's effect on behaviour is reasoning, not evidence. The
  pre-registered experiment at `system-prompts/EXPERIMENT-PREREG-2026-08-04.md`
  has still not been run.

### Note on stricter blocking

Loopback, link-local, private-LAN and multicast addresses are now refused
including via redirect. A local development server on `127.0.0.1` can no longer
be fetched by the agent. That is deliberate.

Full dual-track detail: [Changelogs/v0.1.72-release-notes.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.72/Changelogs/v0.1.72-release-notes.md)
