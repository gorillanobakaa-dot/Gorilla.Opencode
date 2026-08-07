**An API can only bill you for what you send it.** This release sends 92% less.

**Plain-language version:** When you asked the assistant to read a web page, it
used to send the whole page to the AI service — menus, ad scripts, cookie
banner, footer — and you were charged for every word, then charged again each
time the conversation continued. Measured across eight real pages, **ninety-two
percent of what you were paying for was not the article.**

### The measurement

| page | tokens before | tokens now | cut |
|---|---|---|---|
| GitHub file page | 62,083 | 885 | 99% |
| MDN documentation page | 44,219 | 1,568 | 96% |
| A large search page | 34,759 | 3,931 | 89% |
| Wikipedia article | 25,595 | 3,928 | 85% |
| arXiv abstract page | 10,744 | 1,769 | 84% |
| **total** | **179,100** | **13,781** | **92%** |

A GitHub file page cost **62,083 tokens** to display a README whose actual
content is 363. It is a JavaScript application shell wrapped around a text file.

`internal/llm/tools/fetch_tokencost_test.go` re-runs this and **fails if the cut
drops below 50%**, so a regression shows up as a red test rather than a larger
bill.

### Who this is for

A household of two parents and two children on a dollar a day cannot absorb two
dollars a month. There is no version of "it's only a few cents per query" that
survives that arithmetic.

Send few enough tokens and it changes in kind, not degree: **you stay inside the
free tiers permanently.** At a nominal one-million-token daily allowance, the
difference measured here is between **16 GitHub pages a day and 2,754**.

Nobody set out to overcharge anyone. The incentive simply runs one way — a
service paid by the token has little reason to build the dials that send fewer
of them — and the people writing these tools work on fast connections where
62,000 tokens to read a README is invisible. Some providers do offer real free
tiers, and that deserves saying. But a free tier you exhaust in sixteen requests
is a technicality, not access.

### Added

- **Source ladder with provenance.** Structured API → source document → local
  conversion → refuse. New rewrites: `arxiv.org/abs` → export API,
  `pubmed.ncbi.nlm.nih.gov` → E-utilities XML, `wikipedia.org/wiki` → REST
  summary (25,595 → 600 tokens). The tool reports which rung it used.
- **`web_search` with seven keyless sources** — `scholar` (OpenAlex),
  `medical` (Europe PMC / MEDLINE), `crossref`, `openaccess`, `books`,
  `reference`, `all`.
- **Free legal copies surfaced inline.** OpenAlex was already telling us where an
  open-access version lives and we discarded the field. Measured: **7 of 10**
  scholar results now carry a `FREE LEGAL FULL TEXT` link. Give `openaccess` a
  DOI and Unpaywall answers the only question that matters at a paywall: *is
  there a legal free copy, and where?*
- **TextRank**, in Go, no dependencies. Full text only — it **refuses** below
  8,000 characters, and every summary declares its compression ratio, that it is
  extractive, and that centrality is not importance so caveats and negative
  results may have been dropped.
- **26% smaller download**: 66.1 MB → 48.6 MB. About forty minutes back on a
  single-digit-KB/s link.

### Said out loud rather than hidden

There is **no general web search** here. DuckDuckGo answers `200` with a block
page every time; public SearxNG instances returned 403/429; Marginalia returned
nothing. Shipping any of them would be worse than shipping nothing — a model
cannot tell a block page from a result and will summarise it in good faith. The
tool now states the gap, so it **asks you for a URL instead of inventing a
source**. That fixes a real failure we observed and recorded.

### Not verified

- No live fetch over a genuinely slow link; measurements are byte counts, not
  timings.
- Hit rates for `Accept: text/markdown` and the `.md` companion are unmeasured.
- TextRank is unit-tested and wired, not yet driven end-to-end on a real
  full-text document.
- Prompt-line effects are reasoning, not evidence; the pre-registered experiment
  remains unrun.
- Token counts are byte/4 estimates, good to roughly ±15%. The 92% ratio is
  unaffected — both sides use the same estimate.

Full dual-track detail: [Changelogs/v0.1.73-release-notes.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.73/Changelogs/v0.1.73-release-notes.md) ·
Reasoning: [PHILOSOPHY.md Part Seven](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.73/PHILOSOPHY.md)
