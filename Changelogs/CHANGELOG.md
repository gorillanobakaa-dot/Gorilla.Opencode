## v0.1.74 — 2026-08-07 — a price tag on big pages, a quota you can scroll back to, and a model that stops inventing its own methods

Full dual-track document: [v0.1.74-release-notes.md](v0.1.74-release-notes.md).

**Plain-language version:** Three fixes, all from watching it fail in front of a
user. When the assistant reads a web page, that page joins the conversation — and
the AI re-reads the whole conversation every time it replies, so a big page isn't
charged once, it's charged again on every message after it. One page quietly ate
88% of the assistant's memory and nobody mentioned it. Now a note appears saying
what it costs, and offers to shorten it on your own computer for free. Only
genuinely enormous pages get cut, and it says so clearly — the entire text of
*Romeo and Juliet* fits under the limit, so papers and manuals are untouched.

The quota display used to vanish as soon as you carried on working, and checking
it again used up more quota. It now also stays in the scroll-back history with
the time beside it, so you can see what was left earlier and work out how fast
you're burning through it, for free.

And when we asked the assistant to explain how it had searched for something, it
described settings that don't exist, checks it never did, and blamed a technical
fault that wasn't real — when the true answer was "I only tried one search word".
The explanation sounded *more* trustworthy than the original answer, because it
was well organised and admitted mistakes. It is now told that explaining its own
work is a claim like any other: read what actually happened, and if you don't
know why something failed, say so.

Not verified: nobody has yet seen the `/usage` line appear in the history — that
needs one person to type it and look. The prompt line ships on reasoning, not
measurement.

## v0.1.73 — 2026-08-07 — the token sieve: 92% less sent, and a way to find the free copy

Full dual-track document: [v0.1.73-release-notes.md](v0.1.73-release-notes.md).

**Plain-language version:** When you asked the assistant to read a web page, it
used to send the entire page to the AI service — menus, advertising scripts,
cookie banner, footer — and you were charged for every word, then charged again
each time the conversation continued. We measured it across eight real pages:
**ninety-two percent of what you were paying for was not the article.** One
GitHub file page cost 62,083 tokens to display a README whose actual content was
363. This release sends 92% less.

That matters because of who this is for. A family living on a dollar a day cannot
absorb two dollars a month, and there is no version of "it's only a few cents"
that survives that arithmetic. Send small enough requests and you never pay at
all — you stay inside the free allowances permanently. At a typical free daily
allowance the difference we measured is between sixteen pages a day and 2,754.

The assistant can also now **search** for papers and books instead of guessing
web addresses, and — the part that matters most — when it finds a paper behind a
paywall it checks whether a **legal free copy** exists elsewhere. Very often one
does, posted by the authors or their university. Measured on one query, seven of
ten results carried a free legal full text. Nobody should be told to pay $40 for
a paper that is free on the next page.

It now also says clearly when it *cannot* search, instead of inventing a source —
a real failure we observed and fixed. Long documents are shortened on your own
computer using mathematics rather than AI, and the summary always says how much
it cut and warns that the dropped parts may include the paper's own caveats. And
the download itself is 26% smaller, which is about forty minutes back on a slow
connection.

Not verified: no live test over a genuinely slow link; TextRank is unit-tested
but not yet driven end-to-end on a real full-text document; token counts are
byte/4 estimates; the prompt experiment remains pre-registered and unrun.

## v0.1.72 — 2026-08-07 — the model could always read the web, and kept telling you it couldn't

Full dual-track document: [v0.1.72-release-notes.md](v0.1.72-release-notes.md).

**Plain-language version:** The assistant could always read web pages. It just did
not know it could. The tool was called `fetch`, the word "web" appeared once in
its whole description and never in its name, and the system prompt never
mentioned the internet at all — so against a strong trained belief that "I am a
language model, I cannot browse the web", the assistant kept refusing a perfectly
ordinary request and giving a false reason for it. It is now called `web_fetch`,
its description opens by saying the capability exists, and the prompt says so too.

Two other faults in the same tool. It could have been redirected into your home
network or a cloud server's internal admin address — the check looked at the
address you typed and never at where the connection actually went; it now checks
every hop, immediately before connecting. And it asked every website for the full
rendered page when a clean text version was often available, downloading roughly
fifty times more than it needed. That is invisible on fast broadband and it is
minutes of waiting on a slow or satellite link — and because AI assistants charge
by the word and re-read the whole conversation on every reply, the navigation
menus and cookie banners were being billed to you again and again.

Also fixed: a page cut off at the 5MB limit was silently handed over as if
complete, so the assistant would confidently summarise a document it had only
partly read; failed requests threw away the server's explanation; non-UTF-8 pages
arrived as gibberish; PDFs came back as binary noise; and `format` was a required
argument, so the obvious way to call the tool failed outright.

Not verified: no live fetch over a genuinely slow link, no measured hit rate for
the markdown negotiation, and no measurement of the prompt line's effect on
behaviour — that last one has a pre-registered experiment that still has not
been run.

## v0.1.71 — 2026-08-07 — providers that said "ready" and then refused, and a log filter that ate the error

Full dual-track document: [v0.1.71-release-notes.md](v0.1.71-release-notes.md).

**Plain-language version:** Four things were quietly broken, and each one made the
app misreport its own state. It showed a provider as "ready" and then refused to
use it, because the startup picker and the thing that validates your choice had
two different ideas of what "configured" means — providers you had never opened
worked, ones you had worked on did not. Conversations with Cloudflare stopped
dead at the first tool the assistant used and never recovered, because a bad
message stays in the history forever; session titles failed for a separate reason
in the same family. The filter that trims noisy build output was throwing away
the error line itself, on exactly the big kernel and browser builds it exists
for. And if the provider you picked at startup turned out not to work, there was
no way back to that screen — now `/providers` reopens it. Two models in the list
also gained a note saying they held their ground when a user insisted they were
wrong, with the number of times we checked printed next to it, because two checks
is not many.

Fixes: environment-variable provider keys no longer hidden by a stale
`disabled:true`; `"content": null` and `"tools": []` no longer sent, which
unblocks Cloudflare Workers AI for tool use, session titles and compaction;
`filterBuildLog` no longer discards the signal line it matched on; `/providers`
reopens the launch picker mid-session; `openai/gpt-oss-20b` added to the NIM
catalogue; coder prompt gains a `# precedence` section (~160 tokens, switchable
in `/context`).

Not verified: no interactive TUI run — the `/providers` flow wants one human
confirmation. The prompt precedence work has a pre-registered experiment that has
not been run.

## v0.1.70 — 2026-08-05 — the error you needed to read was being deleted, and the input box ignored the window

Full dual-track document: [v0.1.70-release-notes.md](v0.1.70-release-notes.md).

**Plain-language version:** v0.1.68 promised that a failed turn would leave the
full explanation in your conversation. It did not work, and this release is
mostly about admitting that and fixing it properly — the reason was recorded
correctly and then deleted a fraction of a second later by a different piece of
code, so you saw "Canceled — no answer was produced" for a failure that had
nothing to do with cancelling. Four more fixes ride along. Errors now stay on
screen forty seconds instead of ten, because a provider failure is a sentence you
must read and act on, not a "copied to clipboard" toast. The app no longer
guesses that your request was "too large" when it already knows the real cause —
that guess was appearing directly above a message contradicting it, with the
context reading 0%. Switching model from `/models` no longer strands the three
helper agents (session titles, summarising, sub-tasks) on the old model, where
the only clue was a recurring "failed to generate title" and the real failures
waited until later. And the input box no longer outgrows the window: it had been
ignoring its row allotment entirely — handed 1, 2, 3 or 5 rows it drew 16 every
time — so on a short terminal a long prompt appeared stuck on one line, scrolling
"from the last word", while the same build wrapped perfectly in a taller window.
It now respects the space available and says so (`▲ N more lines`) when holding
text back. Two flaky tests were also repaired, one of which guards the
footer-width invariant behind the old marching-footer bug.

## v0.1.69 — 2026-08-04 — the provider menu finds your setup whatever you named it

Full dual-track document: [v0.1.69-release-notes.md](v0.1.69-release-notes.md).

**Plain-language version:** the startup menu asked for an NVIDIA NIM key that was
already saved, and showed the NVIDIA row as not set up while Groq, Cerebras and
xAI showed `(ready)`. Retyping the key would have looked like a fix and then it
would have asked again on the next launch, forever. The key was never lost — the
menu looked for your NVIDIA connection **by name**, searching for an entry called
exactly `NVIDIA NIM`, so one named anything else (`Gorilla.FREE.NVIDIA.NIM`) was
invisible. A connection is identified by where it points, not by what you called
it. The same mistake had a quieter second symptom: choosing NVIDIA also wrote the
app's fixed name back, creating a **second** entry beside yours on the same
address — and two entries on one address fight over which serves the models,
which is how a config ended up with two NVIDIA connections and zero usable models
between them. Both halves now match by address; re-picking a provider updates
your existing entry and carries your saved key across, so pressing Enter on the
row can never wipe a working credential. Notably, model *registration* already
resolved by address and was name-agnostic — so the endpoint worked fine for
inference while the menu insisted it was missing. If this was already broken for
you it repairs itself on the next launch; no action needed.

## v0.1.68 — 2026-08-04 — when something fails, you can finally read why

Full dual-track document: [v0.1.68-release-notes.md](v0.1.68-release-notes.md).

**Plain-language version:** an error used to read `failed to process events: POST
"https://…/v1/chat/completions": 404 Not Found`, which looks like the app or the
network is broken. It was neither — the key was fine and the provider was simply
refusing to run that one model for that account, and working that out took an
evening. Errors are now written in English (*"Llama 3.3 70B isn't enabled for your
account (HTTP 404 — your key is fine). Pick another with /models."*) with the raw
machine error kept alongside, never instead — a translation that throws away the
evidence is worse than none when the translation is wrong. 401 and 404 give
deliberately different advice, because sending you to regenerate a perfectly good
key is its own waste. Failures now also land in the transcript, where they can be
scrolled, selected and copied, instead of flashing past in a status bar that cuts
off at ~100 characters and is wiped by the next message. The jargon prefix
"failed to process events" is gone. A turn where the model runs a command instead
of talking is no longer labelled "Finished without output" as though it had
crashed. And a text-mangling bug was found while fixing the above: the status bar
shortened messages by counting bytes rather than characters, so any dash or
accented letter on the cut point was sliced in half — harmless for plain English,
but the new error messages contain "—" and "⟨⟩", so the fix above would have
started triggering it. Provider error text is now stored locally in your session
database; inspected errors carry only method, URL and status, though this has not
been audited across every provider.

## v0.1.67 — 2026-08-04 — the provider picker stops cutting names in half

Full dual-track document: [v0.1.67-release-notes.md](v0.1.67-release-notes.md).

**Plain-language version:** the every-launch provider menu and the extras screen
squeezed themselves into 76 characters regardless of terminal size, so provider
names and descriptions were chopped off mid-word on a wide window. The 76 was a
legitimate *fallback* for before the terminal reports its size, but it was also
being used as a *ceiling*, so the real width was ignored once known. It now
applies only when the width is genuinely unknown. Display-only; no settings, keys
or sessions touched. **These notes were written after the fact, during the
v0.1.68 release — v0.1.67 shipped with no changelog entry and no notes inside its
package. That was an oversight, and this entry exists so the release is not a hole
in the record.**

## v0.1.66 — 2026-08-04 — the free Claude/GPT tier now actually works when you use it

Full dual-track document: [v0.1.66-release-notes.md](v0.1.66-release-notes.md).

**Plain-language version:** v0.1.65 shipped free Claude/GPT-OSS/Gemini and then three
bugs made it fall over the moment you actually used it. First, signing in to
Antigravity *looked* like it worked but silently ran Gemini instead of the model you
picked — the app forgot to record the sign-in until the next restart, so it decided
the provider "wasn't configured" and fell back (`agent "coder" model
"antigravity.claude-sonnet-4-6" is unusable ... falling back to
"gemini-flash-latest"`). Second, typing `/usage` said "Unknown command". Third — the
big one — Claude and GPT-OSS crashed the instant they used a tool
(`invalid_request_error: ...tool_use.id: Field required`), which for a coding
assistant means they could chat but couldn't actually help. All three are fixed and
each has a test that fails without the fix. Gemini was fine throughout and is left
untouched.

### Fixed

- **Signing in to Antigravity now uses the model you chose, first session included.**
  The provider is registered in-session the instant login succeeds
  (`UpsertProviderKey` with the `oauth-login` sentinel), before agent models are set —
  so `validateAgent` no longer silently reverts every agent to Gemini. Same fix
  applied to the Google-only and GCP login paths.
- **Claude and GPT-OSS can use tools again.** Their native (Anthropic/OpenAI) format
  requires a tool-call `id`; we were sending the Gemini shape, which has none, so any
  conversation containing a tool call 400'd. Tool-call ids are now emitted on the
  Antigravity path only (Gemini matches by name and is unchanged), and the backend's
  own id is preserved from responses. This is what made Claude/GPT usable for real
  coding rather than chat-only.
- **`/usage` works when typed**, not just from the command palette, and now appears in
  `/help`.

### Note (not a bug)

- Hot-swapping models mid-conversation can make the assistant misidentify itself
  (claim to be Claude, then admit it's Gemini). That's each model reading the shared
  history; the one actually on Gemini corrected itself. Nothing changed here.

## v0.1.65 — 2026-08-03 — Claude, GPT-OSS and Gemini for free, through your own Google account

Full dual-track document: [v0.1.65-release-notes.md](v0.1.65-release-notes.md).

**Plain-language version:** two things. First, the free Gemini sign-in had stopped
working — Google quietly closed the free "Code Assist" tier to every program except
their own new app, Antigravity, and answered our requests with "migrate to
Antigravity". Having the program introduce itself the way Google now expects (a
one-word change to how it identifies itself, on the sign-in step only) opens that
door again. Second — and this is the treat — every Gmail account already has a
generous **free** Antigravity allowance that includes **Claude (Sonnet and Opus
4.6), GPT-OSS, and Gemini**, and until now the only way to spend it was Google's own
tool. This release adds an **"Antigravity free tier"** sign-in that unlocks all of
them at no cost, using the allowance already attached to your own Google account.
On top of that, a **provider menu now appears on every launch** (one press of Enter
keeps what you had), and **`/usage`** shows how much of your weekly free allowance is
left — it also appears on its own at the start of each session. The Antigravity route
is unofficial: it works by speaking to Google the way Google's Antigravity tool does,
so Google could change something and break it — that is stated plainly and kept
isolated so nothing else depends on it. The Gemini fix uses Google's supported login
and carries no such risk.

### Added

- **"Antigravity free tier" provider** — sign in with Google and use your own free
  Antigravity allowance: Claude Sonnet 4.6, Claude Opus 4.6 (Thinking), GPT-OSS 120B,
  and Gemini, at cost 0. New OAuth identity (`internal/auth/antigravity_oauth.go`) and
  transport (`internal/llm/provider/antigravity.go`), reusing ~90% of the existing
  Code Assist client. Protocol captured from the installed Antigravity CLI, not
  guessed; transport proven end-to-end through the program's own code (Claude replied).
  **Unofficial and brittle** — Google can change it; the risk is isolated to this
  provider.
- **Every-launch provider portal** — a startup menu listing every way to connect, with
  the cursor on your current provider so Enter continues in one keystroke. Reachable
  from the desktop icon, not just a typed flag. Replaces the old "edit the env file and
  relaunch" first-run dead-end. Keys are entered masked.
- **`/usage`** — shows your Antigravity weekly quota as one line, on demand and
  automatically at session start (silent for non-Antigravity users). Wire shape
  unit-tested against the captured response so a Google-side change fails a test
  rather than blanking the view.

### Fixed

- **The free Gemini "Login with Google" tier works again.** Google discontinued the
  free Code Assist tier for non-Antigravity clients (`UNSUPPORTED_CLIENT`). Sending an
  `antigravity` product token in the `User-Agent` on the onboarding calls restores
  free-tier eligibility and project provisioning. Root-caused live; the header is sent
  on onboarding only — generation rejects it with 403. The earlier HTTP 500 was a blank
  project, not the generation call.

### Also included (pre-existing)

- A **DeepSeek** provider added in an earlier development session
  (`internal/llm/models/deepseek.go` and related edits) ships in this tag by the
  maintainer's decision. Not part of this release's work; noted for completeness.

## v0.1.48 → v0.1.64 — 2026-07-31 — The conversation no longer stops dead at the first tool call

Sixteen builds made between 28 and 31 July 2026, none of which were ever
published. This entry covers all of them. Full documents:
[layman](v0.1.48-v0.1.64-LAYMAN.md) · [developer](v0.1.48-v0.1.64-DEVELOPER.md).

**Plain-language version:** for three days this program had a bug that made it
close to unusable, and it hid itself well. When the AI used a tool — searching
your files, running a command — the answer arrived, was saved, and was never
shown to you. The screen sat on "Waiting for response…". On 30 July a command
finished in two seconds, the AI wrote its full answer, and the screen showed
nothing for fifteen minutes; that is indistinguishable from a stuck connection,
so you wait, then restart, then blame your provider. Every conversation that
used a tool was cut off at its first tool call. Alongside it, one search could
return 2.4 megabytes in a single result and quietly wreck your token budget, the
context meter read about two hundred times too high, Escape did not stop the AI,
and the bar at the bottom of the screen crawled down the window and jumped back
up. All fixed. Every one was found by measuring, not by reasoning about the code.

### Fixed

- **The transcript no longer halts at the first tool result** (v0.1.63). The
  biggest fix here. `ScrollbackReady` returned false for tool messages to stop
  double-printing, but `printPending` breaks on the first not-ready message — so
  every later message, including the model's finished answer, was generated,
  persisted, and never displayed. **"Ready" means "will not change again", not
  "has something to show".** Duplicate suppression moved to
  `RenderForScrollback` returning `""` for that role.
- **Every tool result is bounded by SIZE at one choke point** (v0.1.62). grep
  capped matches at 100 and returned **2,438,026 bytes**, because it matched
  inside files where a whole source file is one escaped string — 80 lines over
  10 KB, longest 66,438. That one result took a conversation from 15.9K to 675K
  tokens in a single turn, and tool results are re-sent every turn afterwards.
  Now 400 KB in `NewTextResponse`. **A limit must be expressed in the unit of
  the resource it protects.**
- **No frame line may exceed the terminal width** (v0.1.57) — the real cause of
  the marching footer. The inline renderer erases by *logical* line count, so an
  over-wide line occupies two physical rows, counts as one, and under-erases by
  a row per render. Enforced centrally by `clampToWidth`.
- **The context meter was inflated ~200×** (v0.1.55); it displayed 387%. Failed
  turns now say why they failed instead of printing nothing.
- **Escape actually stops the model** (v0.1.54), and streamed reasoning wraps at
  word boundaries.
- **It says why there is no thinking** when you asked to see thinking (v0.1.60).

### Added

- Up and Down recall previous messages (v0.1.64).
- Reasoning streams into scrollback; the preview pane is gone (v0.1.58–v0.1.61).

### Changed

- All four system prompts rewritten (2026-07-29) against Anthropic's published
  Claude Fable 5 guidance, with the research cited in
  [`system-prompts/RESEARCH-SOURCES.md`](../system-prompts/RESEARCH-SOURCES.md).
  Coder prompt 1,855 → 4,233 bytes (~464 → ~1,058 tokens/turn). Every section is
  switchable in `/context` with its cost and what you lose; two are marked
  critical because disabling them increases unverified success claims.

### Known issues

- **The footer is still reported to jump.** Two hypotheses are dead, both with
  permanent tests: height oscillation, and the 20-row editor collapse. Diagnose
  with a real byte capture replayed through `internal/tui/inline/terminal_test.go`
  — not from a screenshot.
- **The v0.1.57 width fix is verified headlessly only**, not across a long
  interactive session. It bites hardest near 80 columns.

### Corrections to the record

- **v0.1.56 was shipped on a wrong diagnosis.** Frame-height oscillation was
  blamed for the marching footer; a headless test shows 3↔4 rows and a 20→1
  collapse both render correctly. The change was kept (constant height is still
  more predictable) but its commit message states a cause that is not real.
  Three independent sources reached that same wrong answer.
- **v0.1.59 shipped with a failing test.** A shell chain did not gate on the
  exit code and printed "all green" while a test was red. A pipe returns the
  last command's status, not the test's.
- **The v0.2.0 / v0.1.49 version numbers were never real.** They were invented
  in error, never approved, and the release carrying them had no downloadable
  assets. Both have been removed. The documents written under them were good
  work and are kept; only the numbers are gone.

## v0.1.46 — 2026-07-28 — Undoing a slowdown I caused, and giving the mouse back

Three complaints, three real causes — but only one was what it looked like.

**Plain-language version:** the interface genuinely had got slower, by code I added in
v0.1.45; it is now slightly quicker than before that release. The *models* were never
slower — v0.1.45 stopped under-reporting their time by a factor of a thousand, so an
84-second reply that used to read `84ms` now reads what it always was. And dragging to
select text typed garbage into the input box because the program had quietly taken the
mouse away from your terminal; it no longer does.

### Fixed

- **Dialog redraws were 2–3× more expensive** (`internal/tui/layout/fit.go`) —
  `FitHeight` re-ran its whole render-and-measure search on every `View()`, and Bubble
  Tea calls `View()` on every keystroke **and** every streamed token. Instrumented at
  **3 internal renders per frame** for `/context`. `layout.Fitter` now caches the row
  count that last fitted, verify-then-reuse. Measured like-for-like at 100×30:

  | | v0.1.44 | v0.1.45 | now |
  |---|---|---|---|
  | `/context` | 2.33 ms | **6.65 ms** | **2.05 ms** |
  | `/help` | 1.28 | 2.70 | 1.35 |

  The first version of that cache keyed on terminal size alone and only asked *"does
  the remembered count still fit"*, never *"could more fit now"* — so one cramped
  selection locked in a small list and **two commands became unreachable** while
  scrolling `/help`. An existing reachability test caught it, not me.

- **Dragging to select typed raw escape codes into the editor** (`cmd/root.go`) —
  reported as `[<32;71;41M`. The program requested cell-motion mouse tracking, which
  takes the mouse from the terminal (Shift then needed to select) and reports **one
  event per cell crossed**, so a single drag fires hundreds and stalls the loop until
  the input parser spills raw codes. Dropping non-wheel events in `Update` was too
  late — the cost is upstream of any handler. Mouse reporting is now **opt-in**, with a
  `/settings` row that states the trade. Verified on the real binary under a pty:
  **`?1002h` emitted 0 times off, once on.**

- **One `/context` figure was still a guess** (`internal/llm/agent/calibrate.go`) —
  token costs are measured from real tool schemas at startup, except `diagnostics`,
  which was guarded on having LSP clients. A schema is static, so with every language
  server off — supported, and this developer's setup — that one row showed an estimate.
  Now measured unconditionally, with a test asserting no component still reports its
  declared value.

### Changed

- **One prompt rule relaxed, six kept** (`coder-modern.txt`) — Anthropic's Claude 5
  context-engineering guidance reports removing 80%+ of their coding agent's system
  prompt with no eval loss, and its worked example is a rule we also had. Ours now
  reads `comments: match surrounding density and idiom` instead of `never explain
  WHAT/WHY`. The other six `do not`/`never` lines were reviewed and **kept**: five are
  verification and honesty rules (*never claim unobserved success*, *do not invent
  paths*), and the guidance is about trusting judgement on **style**, not about
  trusting an agent's account of its own work.

- **The release tooling refuses to commit deletions** (`release_pipeline.py`) — it ran
  `git add -A` unguarded. **Nine files of published research under `system-prompts/`
  were sitting deleted in the working tree while this was being written**, unnoticed
  for hours, one release from being written permanently into a tag. Now it stops and
  lists them. It also fast-forwards `main` to the tag, which it never did — the
  omission behind `main` once sitting 43 commits behind.

- **`CLAUDE.md` now documents `release_pipeline.py`**, which has a `go_gorilla` profile
  built for this repo and was undocumented. That cost four consecutive releases driven
  by hand by sessions that never knew it existed.

### Known issues

- **The display corruption reported alongside the mouse leak was never reproduced.**
  Message rendering, the reasoning block, a 4,000-character unbreakable paste and the
  split layout all produce uniform widths headlessly. Attributing it to the mouse flood
  is reasoning, not proof — if it survives this release, that was wrong.
- The size sweep covers **8 of ~15** dialog surfaces; the rest may overflow undetected.
- **Tool descriptions are ~3,680 tokens against the prompt's ~464** and are the real
  cost centre — but there is zero duplication and no prescriptive language left in
  them, so the safe cuts are spent. Trimming further without an eval risks a quietly
  worse agent. Not attempted.
- `layout.Fitter`'s cache key is caller-supplied; nothing enforces completeness.
- The main interface still cannot be selected or copied.

