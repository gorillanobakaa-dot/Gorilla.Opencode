# Third-party content: review rule docs

Every `*.md` file in this directory (except this NOTICE) is vendored **unmodified**
from the OpenCodeReview project:

- Upstream: https://github.com/alibaba/open-code-review
- Path in upstream: `internal/config/rules/rule_docs/`
- Copyright: Alibaba Group
- Licence: Apache License 2.0 — full text in `LICENSE.apache-2.0` in this directory
- Vendored on: 2026-08-06 (34 files, from `main`)

## Why these are here

They are the *know-how* half of a code review, expressed as data instead of
code: what a reviewer should look for in each language. This toolkit's own
registry knows how to *run* thirty analysers; it had nothing that described
what bad code looks like. These files fill that gap.

Nothing here executes. `rules.py` reads them, keys them by language, and hands
them to whoever is doing the reading — an agent, a local model, or a person.

## Keeping them updated

They're plain Markdown with no dependency on our code, so re-vendoring is just
a re-download. To refresh:

    python3 rules.py --refresh

If you edit one locally, note it below so a refresh doesn't silently revert
your change.

### Local modifications

None.
