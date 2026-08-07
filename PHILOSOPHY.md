# Gorilla Open Source Philosophy
### A Manifesto for Radical Transparency, Universal Access, and the End of Knowledge Gatekeeping

---

> *"The gorilla does not ask permission to use its full strength.  
> Neither should any human being when seeking knowledge."*

---

## What This Document Is

This is a living reference document — a manifesto and a methodology — written so that its author does not have to re-explain the same foundational ideas at the start of every conversation, collaboration, or project review.

It is written for **everyone**: the developer reading the source code, the layperson trying to understand what was built and why, the AI assistant being asked to help, and the human reviewer checking whether any of this raises a flag. You are all equally welcome here, and this document is designed so that none of you needs to translate it for the other.

That, in itself, is the point.

---

## Part One: The Problem With Open Source As It Currently Exists

Open source software changed the world. The philosophy behind it — that knowledge, code, and tools should be freely available and not locked behind corporate walls — is one of the most important ideas in the history of computing.

And yet open source, as it is most commonly practised, **fails the majority of human beings on Earth.**

Here is why.

When a project publishes its code on GitHub, it does something remarkable: it makes the *instructions* available to anyone. But instructions written for machines, or for the engineers who programme them, are not actually available to a person who does not already speak that language. A repository full of Python files and a `README.md` that begins with `pip install` is not accessible to someone who has never used a terminal. It is transparent in theory. It is a closed door in practice.

**Open source gave the world the recipe. It forgot to teach people how to cook.**

The result is a two-tier knowledge system disguised as equality. Developers can read the source, fork the project, understand exactly what is happening. Everyone else must simply trust that what they were told about the software is true. They have been given freedom in principle while retaining dependence in practice.

This is not a failure of intention. It is a failure of **communication methodology.**

---

## Part Two: The Gorilla Unleashed Philosophy

The name comes from a simple image.

A gorilla in a cage is not limited by its intelligence or its strength. It is limited by the structure around it. Open the cage, and the animal does not become more capable — it already was. What changes is its access to the full expression of what it already is.

Most people walk around as caged gorillas. Not because they lack intelligence. Not because they lack curiosity. Because the structures around knowledge — the jargon, the assumed expertise, the culture of "if you don't know this already you probably shouldn't be here" — function exactly like bars.

**The Gorilla Open Source Philosophy is not about software. It is about structural access to knowledge.**

It holds three things to be true:

**1. Hardware and capability are only the beginning.**  
Unleashing the full performance of a machine, a model, or a system matters — but it is the smallest part of the work. The larger work is making what that capability *does* understandable to every person it affects.

**2. Transparency must be legible, not merely visible.**  
Publishing code satisfies a legal and ethical standard of openness. But openness without comprehension is a formality. True transparency means that any person — regardless of their technical background — can understand *what* a system does, *why* it was built that way, and *what consequences* it may have for them. If they cannot read it, it is not truly open.

**3. Education is not optional. It is the product.**  
Every project built under this philosophy treats documentation, explanation, and accessible language as first-class deliverables — not afterthoughts, not README files written at midnight before a launch. The explanation is part of what is being built.

---

## Part Three: The Dual-Track Documentation Standard

The practical methodology that emerges from the philosophy above is called **Dual-Track Documentation.**

The idea is straightforward: every significant piece of work produces two parallel explanations, written simultaneously, covering the same material.

### Track One: The Human Track

Written in plain language. No assumed knowledge. Explains *what* the system does from the perspective of the person who will use it or be affected by it. Uses analogies, examples, and real-world consequences. Answers the question a non-technical person would actually ask: *"What does this do to me, or for me?"*

This track is not a dumbed-down version of the technical document. It is a complete and honest explanation in a different language. It does not omit complexity — it translates it.

### Track Two: The Developer Track

Written with technical precision. Explains *how* the system works: the architecture, the data flows, the design decisions, the trade-offs made and why. Uses correct terminology. Assumes programming literacy but not familiarity with this specific project. Answers the question a technical reviewer would ask: *"How does this actually work, and could I audit, modify, or improve it?"*

### Why Both Must Exist Simultaneously

The two tracks are not alternatives. They are not "pick one based on your audience." They are always produced together, because:

- **Accountability requires both.** A system that only has technical documentation can be audited by developers but not by the people it affects. A system that only has plain-language documentation cannot be independently verified. Only when both exist is the system genuinely transparent.

- **Each track improves the other.** Writing a plain-language explanation forces the developer to confront whether they truly understand what they built. Translating something into everyday language reveals assumptions and gaps that technical documentation can hide. The discipline of writing for non-experts makes the technical documentation more honest.

- **Neither audience should have to trust the other.** A layperson should not have to trust that the developer's summary of the technical document is accurate. A developer should not have to translate the plain-language document into something auditable. Both should be able to read what was written for them directly.

---

## Part Four: What This Looks Like in Practice

A project built under this philosophy will always include:

**For the layperson:**
- A plain-language description of what the tool does
- An honest account of what data it uses, and what it does with that data
- A clear statement of what the tool cannot do, and where its judgement should not be trusted
- Real examples of inputs and outputs, explained in plain English
- A description of any risks or limitations the user should know about

**For the developer:**
- Full source code, version-controlled and readable
- Architecture documentation explaining why decisions were made, not only what they are
- A description of every external dependency and why it was chosen
- An honest account of known failure modes and edge cases
- A clear statement of what the code does not yet do

**For both, in the same document when possible:**
- A changelog written so that both a layperson and a developer can follow how the project evolved
- A statement of intent: what problem this is trying to solve, and for whom

---

## Part Five: On AI Assistants and This Philosophy

This philosophy was developed in the context of working with AI language models as collaborative tools — specifically in the construction of systems where the stakes for non-technical users are real (legal disputes, tenancy rights, financial decisions).

When an AI assistant is asked to help build something under this philosophy, it is being asked to hold two audiences in mind simultaneously. The code it writes must be technically sound. The explanations it produces must be genuinely accessible. Neither task is subordinate to the other.

This document also serves a secondary purpose: context for AI systems participating in these conversations. When an AI assistant reads this document, it understands that:

- The person it is working with is committed to making their work understandable to everyone, not just experts
- Documentation and explanation are part of the deliverable, not optional additions
- Plain language is not simplification — it is a parallel form of rigour
- The aim is not to impress technically sophisticated readers; it is to leave no reader behind

---

## Part Six: On Transparency and Reviewers

This document, and the projects built under this philosophy, are intentionally written to be readable by anyone reviewing the work — including reviewers at AI companies checking conversations for safety and policy compliance.

That is not a concession. It is a feature.

If a system cannot be explained clearly to someone who does not already know how it works, that is a warning sign about the system — not about the reviewer's expertise. The commitment to transparency here is not performative. It is structural.

Reviewers, human or automated, are part of the intended audience. The same dual-track principle applies: the technical details are visible and auditable; the intent and consequences are explained in plain language.

There is nothing here that benefits from being hidden.

---

## Part Seven: The Token Sieve — Why We Refuse to Send the Whole Page

### The principle

**An API can only bill you for what you send it.**

That single sentence is the whole of this section. Everything below is its
consequences.

The modern AI ecosystem is built on an incentive that runs directly against the
user: maximise the context window, ingest as much raw data as possible, bill per
token. The "URL context" feature built into most LLM frameworks fetches a page
and sends the entire blob — navigation, cookie banner, sidebar, footer, tracking
scripts, the lot — and charges you for every word of it. Then it answers your
question using about two percent of what you paid for.

We treat that as a defect, not a feature.

### Why this is a moral question and not a performance one

This project is built for people who are not the intended customer of any of
this. A household of two parents and two children living on a dollar a day
cannot absorb two dollars a month. There is no version of "it's only a few
cents per query" that survives contact with that arithmetic.

So token waste here is not inefficiency. It is a barrier to entry, and removing
it is the point of the work.

Send few enough tokens and something changes in kind rather than degree: you
fit inside the free tiers permanently. A provider offering a fixed number of
free requests per day is generous if each request is small and useless if each
request carries a web page. **Staying small is what makes the tool free
forever**, and free forever is what makes it usable by someone who cannot enter
a card number.

### The measurement

One arXiv abstract page, measured on 2026-08-07:

| what you send | bytes | ~tokens | vs raw |
|---|---|---|---|
| raw HTML, as URL-context tools send it | 42,979 | ~10,744 | 100% |
| HTML converted and stripped locally | 7,177 | ~1,794 | 16.7% |
| the arXiv export API, full XML | 2,936 | ~734 | 6.8% |
| the abstract alone | 1,312 | ~328 | 3.1% |

**Thirty-two times less, for strictly more useful content.** The raw page is
mostly furniture; the abstract is the part that answers the question.

Note carefully *where* the saving comes from. Going from raw HTML to the API
captures 93% of it. That matters, because it tells you which engineering is
worth doing.

### The source ladder

Every fetch walks down this ladder and stops at the first rung that answers.
The tool reports which rung it used, every time.

0. **A structured API.** arXiv export, OpenAlex, Europe PMC, Crossref,
   Wikipedia REST. Kilobytes, no furniture, machine-readable.
1. **The source document.** Content negotiation (`Accept: text/markdown`), a
   `raw.githubusercontent.com` rewrite, a `.md` companion.
2. **HTML converted locally**, with navigation and scripts stripped before
   conversion.
3. **Refuse, and say why.** We do not send a raw page to a paid API and call it
   a feature.

The reporting is not decoration. A model that read a reconstruction should know
it read a reconstruction, so it can weigh its own confidence accordingly.

### Where we differ from the obvious version of this idea

Three refinements, each of which we got wrong first and corrected against
measurement.

**Do not summarise what is already small.** An abstract is ~330 tokens. Running
a local extractive summariser over it saves perhaps 100 tokens and risks
deleting the finding. Algorithmic summarisation earns its place on *full text* —
a 10,000-word paper — and nowhere else. The ladder above already captured 97% of
the available saving before any summariser runs.

**A local summariser must declare what it dropped.** TextRank and LexRank select
sentences by graph centrality, and centrality is not importance. A paper's
negative result, its caveat, its limitation section — these are frequently the
*least* central sentences in the graph and the first to be cut. Send an LLM a
silent extract and it will reason about the fragment as though it were the
whole, exactly as it would with a truncated file. **Any local summarisation
states its compression ratio and that it is extractive.** This is the same rule
as "truncation always says so", applied one layer up.

**Local processing must not import a heavier dependency than the thing it
saves.** The tempting pipeline — `xmllint`, `pup`, `html2text`, Python with
`sumy` and its numeric stack — was checked against reality on 2026-08-07: none
of those four were installed on the project's own development machine. On a
2 GB laptop, a Python runtime plus numeric libraries is a larger install than
this entire application, which ships as one static binary with no runtime at
all. So parsing and summarising belong **in-process, in Go**. TextRank is about
150 lines and no dependencies. A pipeline that cannot be installed on the
hardware it was designed for is a thought experiment.

**On local search:** SQLite with FTS5 and BM25 over a vector database, without
reservation. It is a single file, a few megabytes of RAM, no floating-point
matrix work, and it runs on hardware that predates every assumption the vector
ecosystem makes.

### The rule this gives us

> **The cloud model is a reasoning engine, not a web scraper.**
>
> It should never see a URL, never fetch a page, and never receive a byte that
> a local process could have removed. Extraction, filtering and selection happen
> on the user's own machine, on hardware they already own, at zero marginal
> cost. Only the distilled question reaches the meter.

Applied honestly this yields a system that delivers real research on a
fifteen-year-old laptop with 2 GB of RAM, no AVX2, no GPU and a single-digit
KB/s link — for nothing, permanently, inside free quotas.

That is not a performance optimisation. It is the difference between a tool that
serves the people this project was written for and one that quietly excludes
them.

---

## Summary: The Core Commitments

For anyone who has read this far and wants the short version:

1. **Open source must mean open to everyone, not only to engineers.**
2. **Every project produces two forms of documentation: one for humans, one for developers — both complete, both honest.**
3. **Explanation is not an add-on. It is the work.**
4. **The measure of transparency is not whether the code is published. It is whether a non-technical person affected by the system can understand what it does.**
5. **No one should have to trust a summary they cannot verify. Both tracks of documentation exist so that everyone can read what was written for them.**
6. **An API can only bill you for what you send it. We send the least that answers the question, so the tool stays free for people who cannot pay.**
7. **The cloud model is a reasoning engine, not a web scraper. Extraction and filtering happen locally, on hardware the user already owns.**
8. **Anything that discards content — truncation, conversion, summarisation — says so, and says how much.**

---

## Document History

| Version | Date | Notes |
|---|---|---|
| 1.0 | 2026-06 | Initial manifesto, written from accumulated context across multiple project conversations |
| 1.1 | 2026-08-07 | Added Part Seven: The Token Sieve. Written after measuring a single arXiv page at 10,744 tokens raw versus 328 as an abstract — a 32x difference that decides whether this tool is free or not. Three commitments added. |

---

*This document may be shared freely, referenced in project READMEs, included in AI assistant context, or cited in conversation. Its purpose is to make re-explanation unnecessary. Use it accordingly.*
