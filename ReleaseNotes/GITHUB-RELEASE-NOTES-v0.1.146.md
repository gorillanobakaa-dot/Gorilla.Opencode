# Gorilla OpenCode 0.1.146 — Windows and Linux

**Research reports are now checked by the program against the pages the helpers really opened. /osint writes a plan before it searches. And the program no longer sends the maintainer's contact address to two research sources: it sends yours, and only if you set it.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, for example *find out why this program will not start*, and an
AI does the work: it opens the files on your computer, runs commands, reads what
they print, and tells you what it found.

Picture an assistant sitting at your keyboard. It can open your files, run your
tools and change things. That is what makes it useful. It is also why it needs
watching, because an assistant with your keyboard can make a mess.

It is free. It has no account of its own. You choose which AI it uses: one that
runs entirely on your own computer, or one from a company you have a key or a
sign-in for. It is built to work on an old laptop with slow, expensive internet.

## Should you download this version?

**Yes, if you use /research or /osint.** Their reports are now checked before you
read them, and /osint plans first.

**Yes, everyone, for privacy.** Every earlier version sent the maintainer's own
contact address to Unpaywall and to the American SEC filings service whenever
research reached them. This version sends nothing there unless you give it your
own address.

**Yes, if you connect your own local tool server** and are tired of answering a
permission question for every read-only check.

## Why this matters to you

A research helper writes a report with sources. Until now the program believed
the report as written. A helper could name a web page it never opened, or quote a
sentence that is not on the page, and the final answer would carry it.

Think of an editor who used to print a reporter's story as it came in. Now the
editor takes the reporter's own notebook, the pages that reporter really opened,
and checks every footnote against it before printing. A footnote to a page
nobody opened, or a quotation that is not on the page, is marked UNSUPPORTED.

Measured on the file published here, with Gemini 3.6 Flash and an /osint question
about the 2024 CrowdStrike outage: 28 claims were checked against 33 pages the
helpers had opened, and the record of that check was saved next to the report.

**What it means for you, in one line:** what a research report cites, it opened;
what it quotes, it read.

**What it does not do:** it does not judge whether a quotation proves the claim.
That is still for the model in charge, and for you.

## How to use the new things

### Research and /osint

Nothing to switch on. Run `/research` or `/osint` as before.

- The answer now opens with the plan: the 3 to 7 questions the model in charge
  decided the answer depends on.
- Any claim the program could not match to an opened page or a real quotation is
  listed as **UNSUPPORTED** under the helper that made it.
- Next to the usual `findings-....md` you will find `findings-....jsonl`. It is the
  record of the check, in a form other tools (for example Fieldkit's
  `fieldkit evidence check`) can read again later.

### Your contact address for Unpaywall and SEC

Two sources refuse to answer without a contact address. If research reaches
them and you have not set one, that source is skipped and the report says so,
for example *SEC EDGAR required contact email setting*. To give your own
address, open your `config.json` and add a line with your address:

```json
"contactEmail": "you@example.org"
```

On Windows the file is `%USERPROFILE%\.config\gorilla-opencode\config.json`.
The address goes to those two sources and nowhere else. A project folder's
settings file cannot set it for you.

### A local tool server you trust

If you connect your own local (stdio) tool server, you may mark it in
`config.json`:

```json
"mcpServers": {
  "fieldkit": { "type": "stdio", "command": "fieldkit", "args": ["mcp"], "trust": "local" }
}
```

Its calls then ask for permission only when they say they go online. Mark only
a server you wrote or trust: the program relies on that server's own word about
whether a call went online. Servers without the mark, and servers on the
internet, ask exactly as before. `docs/FIELDKIT.md` explains it in full.
---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### Why This Release Exists

A review of /research and /osint on 11 October found twelve faults, and one bigger problem behind them: the report each helper wrote was taken as written. A helper could list a web page as its evidence without ever having opened it, or quote a sentence that is not on the page, and nothing noticed.

Think of a reporter handing in a story with footnotes. Until now the editor printed it as it came. Now the editor takes the reporter's own notebook, the pages that reporter really opened, and checks every footnote against it before the story goes to print. A footnote to a page nobody opened, or a quotation that is not on the page, is marked UNSUPPORTED, in front of the person in charge, before the final answer is written.

The same review found that two sources, Unpaywall and the American SEC filings service, were sent a fixed contact address belonging to the maintainer, built into every copy of the program up to 0.1.145. That address is gone. Those two sources now get your own address, and only if you set one.

And /osint now starts with a plan: three to seven questions the answer depends on, most important first, which every helper works from and the report opens with. Testing this release found that the first build did not ask /osint for that plan, only /research; that was fixed and everything was tested again on the file published here.

Last, for people who connect their own local tool server to the program (the maintainer's is called Fieldkit): such a server can now be marked as trusted in your settings, and its read-only checks then stop asking for permission at every step.

### What You Will Notice

**Research and OSINT reports**
- Before: A helper's sources and quotations were trusted as written.
- After:  Every source is checked against the pages that helper really opened, and every quotation against the text it got back. Failures are marked UNSUPPORTED before the answer is written. A record of the check is saved next to the report.
- Affects: Anyone who uses /research or /osint

**/osint**
- Before: The helpers started without an agreed list of what to find out.
- After:  The model in charge writes 3 to 7 questions first; every helper gets them and says which ones it could not answer.
- Affects: Anyone who uses /osint or /research

**Unpaywall and SEC sources**
- Before: Every copy of the program sent the maintainer's address to these two services.
- After:  They get your address, from your own settings, or they refuse and tell you which setting to fill in.
- Affects: Anyone whose research reaches these two sources

**Your own local tool server**
- Before: Every call asked for permission as if it went on the internet, even read-only checks.
- After:  A server you mark as trusted asks only for calls that say they go online.
- Affects: Only people who connect their own local tool server

### Deliberately Not Done

- **Trusting every tool server** — Only a local server you mark yourself, in your own settings file, is trusted. A server on the internet, or one you did not mark, asks exactly as before, and a project folder cannot add a server at all.
- **Checking that a page says what the helper claims it means** — The program checks that the page was opened and that the quotation is on it. Whether the quotation supports the claim is still for the person in charge, and for you, to judge.
- **Fixing the flood of permission questions when an AI runs many slightly different shell commands** — Seen in a long session with NVIDIA's Nemotron on 11 October: each new wording of a command is a new question. It needs its own careful change and is not in this version.

### Privacy & Security

No telemetry was added and none exists. Up to 0.1.145, every copy of the program sent the maintainer's own contact address to Unpaywall and SEC EDGAR when research reached them. That address is no longer in the program. Those two sources now receive your address only if you set it, and are skipped with a clear message if you do not; a project folder's settings file cannot set it for you. A local tool server asks less only when you mark it as trusted in your own config.json; remote servers and unmarked servers ask exactly as before. Mark only a server you wrote or trust, because the program then relies on that server's own word about whether a call went online.

### How to Install

**Before you start:**
- Windows 10 or 11, or a 64-bit Debian, Ubuntu, Fedora or similar Linux computer.
- Enough mobile data for a 55 MB file on Windows or a file of about 24 MB on Linux.
- On Windows you do not need to close the program first; an open window stays on the old version until you close it.

**Step 1:** Open the release page in your web browser and download the one file for your computer. Windows: gorilla-opencode.exe. Debian, Ubuntu or Mint: gorilla-opencode_0.1.146_amd64.deb. Fedora, openSUSE or Rocky: gorilla-opencode-0.1.146-1.x86_64.rpm. Also download SHA256SUMS-v0.1.146.txt.
✓ Two files are in your Downloads folder.

**Step 2:** Check the file is the one that was published. On Windows, press the Windows key, type PowerShell, press Enter. A window with a blinking cursor opens. Type cd Downloads and press Enter. Then type the command below and press Enter.
```
certutil -hashfile gorilla-opencode.exe SHA256
```
✓ It prints 6eb3debd2e402a3d777223706bb0928d079dc2eb43a28088e854081f32548307. On Linux, run sha256sum -c SHA256SUMS-v0.1.146.txt --ignore-missing in a terminal in your Downloads folder; it prints the file name followed by OK.

**Step 3:** Windows: install it. Type the command below and press Enter.
```
.\gorilla-opencode.exe install
```
✓ It reports where it copied itself and the shortcuts it made.

**Step 4:** Debian, Ubuntu or Mint instead: type the command below and press Enter. It asks for your password because installing a program changes the system.
```
sudo apt install ./gorilla-opencode_0.1.146_amd64.deb
```
✓ A line says Setting up gorilla-opencode (0.1.146), with no line beginning with E:.

**Step 5:** Fedora, openSUSE or Rocky instead: type the command below and press Enter.
```
sudo dnf install ./gorilla-opencode-0.1.146-1.x86_64.rpm
```
✓ The last line says Complete!

**Step 6:** Confirm the version. Type the command below and press Enter.
```
gorilla-opencode --version
```
✓ It prints v0.1.146.

**To go back:** Windows: type gorilla-opencode uninstall and press Enter, then install the gorilla-opencode.exe from version 0.1.145 the same way as in step 3. Debian: sudo apt remove gorilla-opencode. Fedora: sudo dnf remove gorilla-opencode. Your conversations, settings and keys are stored separately and none of these steps touches them. Going back to 0.1.145 brings back the maintainer's address being sent to Unpaywall and SEC.

### If Something Goes Wrong

**A research run says a source requires a contact email.**
Unpaywall or SEC was asked, and you have not given the program an address to send them.
What to do: Add "contactEmail": "you@example.org" to your config.json, with your own address, or set GORILLA_OPENCODE_CONTACT_EMAIL. The address goes to that source and nowhere else. Without it, the rest of the research still runs.
Status: expected behaviour, new in 0.1.146

**A research report lists claims as UNSUPPORTED.**
A helper cited a page it never opened, or quoted text that is not on the page it got.
What to do: Treat those claims as unproven. The rest of the report is not affected.
Status: expected behaviour, new in 0.1.146

**A file named findings-...jsonl appears next to findings-...md.**
It is the record of the check, in a form other tools can read.
What to do: Keep it with the report, or delete it; nothing else uses it unless you run a checker on it.
Status: expected behaviour, new in 0.1.146

### Common Questions

**Q: Do I have to set a contact address?**
A: Only if you want research to use Unpaywall or SEC EDGAR. Everything else works without it.

**Q: Does UNSUPPORTED mean the claim is false?**
A: No. It means the helper did not show the program the evidence it named. The claim may be true; it is not proven by that report.

**Q: I do not use a local tool server. Does the trust setting change anything for me?**
A: No. Without the mark in your own settings, nothing changes.

### Bottom Line

Take this version if you use /research or /osint: their reports are now checked against what the helpers really read, /osint plans before it searches, and the program no longer sends the maintainer's address to two research sources. Honest limit: the program checks that evidence was opened and quoted correctly, not that it proves the claim.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Sources and quotations are checked against what the helper fetched | 📄 stated in input | the program checks every helper's report against what that helper actually fetched |
| The maintainer's address was compiled in up to 0.1.145 | 📄 stated in input | were sent a fixed address belonging to the maintainer, compiled into every build up to 0.1.145 |
| The first build of 0.1.146 did not ask /osint for a plan; fixed and retested | 📄 stated in input | on the first build of 0.1.146, /osint did not ask the model for a plan |
| Trusted local server asks only for calls that say they go online | 📄 stated in input | a call counts as going on the internet only when its arguments say "network": true |
| Permission flood seen with Nemotron | 📄 stated in input | the permission question appearing for almost every shell command |
| Seven scenarios valid; /osint plan of five, 28 claims against 33 pages | 📄 stated in input | All seven gave valid results |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated plain-language release notes.*

---

# What the program actually printed

Every block below is real output, taken on the maintainer's computer from the file published on this page. Nothing here is an illustration.

## /osint opens with its plan

Gemini 3.6 Flash, four helpers, the question *What is known about the 2024 CrowdStrike Falcon sensor outage: cause, scale, and the vendor's stated remediation?* The answer began:

```
### Research Plan (Sub-Questions)

Before running the inquiry, the investigation plan was structured into five prioritized sub-questions:
1. **Direct Mechanism**: What specific technical defect or mechanism in the CrowdStrike Falcon sensor update caused Windows systems to crash (BSOD) on July 19, 2024?
2. **Global Scale**: What was the global scale of the outage in terms of affected Windows devices and impacted sectors?
3. **Pipeline Root Cause**: What root cause in CrowdStrike's validation and deployment pipeline permitted the faulty Channel File 291 to reach production systems?
4. **Immediate Remediation**: What immediate remediation steps did CrowdStrike and Microsoft provide to restore affected systems?
5. **Vendor Architecture Commitments**: What architectural and quality assurance changes did CrowdStrike commit to in its Post-Incident Review (PIR) to prevent recurrence?
```

## A source with no contact address is skipped, and says so

From the same run, one helper's list of what it tried:

```
- web_search sec 'CrowdStrike Falcon July 19 2024 Channel 291': failed (SEC EDGAR required contact email setting)
```

## The record of the check

`findings-26-10-11-19-40-13-what-is-known-about.jsonl`, saved next to the report, counted by record type:

```
{'run': 1, 'lane': 4, 'claim': 28, 'fetched': 33}
```

## Every change in this version

- Research and /osint: each helper's sources and quotations are checked against the pages it really opened; failures are listed as UNSUPPORTED; the check is saved as `findings-*.jsonl`.
- A plan of 3 to 7 questions before the helpers start, in /research and /osint.
- Twelve fixes from the 11 October review of /research and /osint, among them: the maintainer's address is no longer sent to Unpaywall and SEC EDGAR.
- The list of sources the helpers are shown is generated from the source register, and three disagreements were corrected.
- A local tool server marked `"trust": "local"` asks only for calls that say they go online.
- Smaller: the settings schema regenerated and held to its generator; two tests that failed for reasons unrelated to the program fixed.

## How it was tested

**The file you download is the file that was tested.** The Windows program was built once from the final source, tested, and published without being rebuilt.

**The program's own tests.** The complete set passes on Windows. It had only run on Linux before this release; on Windows it found one test checking Unix file permissions, which Windows does not have, and that test was corrected.

**Against a real AI, with nobody watching.** The published Windows file was started from a script seven times against NVIDIA's `nemotron-3-super-120b-a12b`. All seven runs gave a usable result.

**/osint on the published file** with Gemini 3.6 Flash: the plan, the skipped source and the record shown above. 264.5 seconds.

**Found while testing this release.** The first build of 0.1.146 did not ask /osint for a plan: the sentence that asks for it had been added to /research only, while the /osint page told the user the model writes the parts first. A dossier run on that build called the research tool with no plan. The sentence now goes to both, a test requires it in both, and every test on this page was run again on the rebuilt file.

**On real Linux, after publication.** Automatic checks on GitHub took the files from this page. The .deb was installed, started and removed on Ubuntu. The .rpm was installed, started and removed on Fedora. The Arch package was built on Arch Linux, installed, started and removed, and is attached below. The program, installed from the .deb, held a conversation with an AI running on the same Linux machine. All passed.

**What the automatic checks caught after publication.** The program's own test run on GitHub failed once, on this page: a test requires every release page to show its pictures, and the first version of this page had none. The section below was added; the program itself is unchanged.

**What was not tested.**

- The `"trust": "local"` setting against a real Fieldkit server on this computer. Its tests use a stand-in server.
- Unpaywall and SEC with an address set. Without one, the refusal above was seen on the published file; with one, only the program's own tests.

## Known, not fixed in this version

In a long session with NVIDIA's Nemotron 3 Ultra on 11 October, the permission question appeared for almost every shell command. "Allow for session" covers one exact command, and the model wrote each command slightly differently. This needs its own careful change.

## No new pictures, and why

Everything in this release is text the program prints, quoted above as printed.
No new screenshot was taken. The two below are from earlier releases and show
screens this release did not change, pinned to this version.

**The normal window**, where every action is listed as it happens. This is the
permission question.

[![Gorilla OpenCode showing a Permission Required dialog for the patch port tool, naming the folder it will modify and the patch series it will apply, with the three choices Allow, Allow for session and Deny, proving the program asks before a tool changes files](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.146/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.146/docs/screenshots/gallery/v0132-patch-port-permission-prompt.png)

**The command list.**

[![The command reference filling a 200 column terminal in two balanced columns, headed Commands what each one does, with every command from slash clear through to slash help visible at once and no scrolling needed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.146/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.146/docs/screenshots/gallery/v0132-help-two-columns-full-window.png)

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

research_evidence.go checks each lane's reply against that lane's own fetched pages before synthesis: cited-not-opened, quote-not-found and the output contract; failures are listed as UNSUPPORTED and the run is saved as findings-*.jsonl for `fieldkit evidence check`. The research tool takes a `plan` of 3-7 sub-questions (more refused); /research and /osint both ask for it. Twelve fixes from the 2026-10-11 research/OSINT review, including removal of the compiled-in maintainer address sent to Unpaywall and SEC EDGAR (now contactEmail / GORILLA_OPENCODE_CONTACT_EMAIL, refused by name when unset). The source atlas is rendered by cmd/atlas from the registry. MCP stdio servers marked "trust": "local" decide egress and taint from per-call labels. Scope: 50 files changed, 2648 insertions, 300 deletions between v0.1.145 and the build commit.

### Known Alternatives Considered

Checking that a quotation supports the claim (semantic entailment) is not attempted; only presence of the page and of the quotation is checked. The MCP local-trust decision rests on the server's own _meta labels; a lying server is detected after the fact only when it reports egress for an undeclared call. Other alternatives: Not available in the source material.

### Architecture Impact

New internal/llm/agent/research_evidence.go and cmd/atlas; source-atlas.txt becomes a generated artefact of docs/source-registry.json and source-atlas.yaml. MCP tool calls split into callMCP (transport) and runTool/runLocalTool (policy). golang.org/x/text and gopkg.in/yaml.v3 become direct dependencies at their existing versions.

### Toolchain

```
go1.27.0 windows/amd64. Windows: `go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.146" -o gorilla-opencode.exe .`. Linux: same flags with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Windows binary sha256 6eb3debd2e402a3d777223706bb0928d079dc2eb43a28088e854081f32548307, built from the committed tree, tested, published unchanged. .deb and .rpm by nfpm.
```

### Resource Deltas

gorilla-opencode.exe: 55,099,904 bytes (0.1.145) -> 55,220,736 bytes (+120,832).

### Code Changes

| File | Change | Old Behavior | New Behavior |
|------|--------|--------------|--------------|
| `internal/llm/agent/research_evidence.go` | added | Lane replies trusted as written. | Per lane: EVIDENCE URLs not fetched by that helper (failed fetch excluded, redirect target counted) -> cited-not-opened; QUOTE not in fetched text (NFKC-folded) -> quote-not-found; contract headings, BASIS, CONFIDENCE, SOURCES TRIED, tiers and two-axis grades. Listed as UNSUPPORTED and counted in the synthesis duty. findings-*.jsonl written beside findings-*.md (run, lane, claim, fetched records). |
| `internal/llm/agent/research-tool.go, internal/tui/researchroute.go` | modified | No plan; /research and /osint prompts did not ask for one. | `plan` parameter (max ResearchMaxPlan = 7, more refused); THE PLAN in every helper brief; NOT ESTABLISHED names unanswered parts. planInstruction shared by researchPrompt and osintPrompt (the osint prompt lacked it in PR 1; fixed during release testing, 71e773d). |
| `internal/llm/agent/research-tool.go (selectRolesFor, peerDigest), research_recover.go, research_salvage.go` | modified | Mandatory lanes could be cut by roles; second wave did not see first-wave audits; recovery read context occupancy; saved findings matched by question only. | Mandatory lanes first and never cut, unfitted roles named; peerDigest carries audits; spend from cumulative token fields; findings matched on call id. |
| `internal/config/loadout.go (ResearchContactEmail), internal/config/config.go (ContactEmail), internal/llm/tools/websearch_data_sources.go` | modified | Unpaywall email= and SEC User-Agent used a compiled-in maintainer address. | GORILLA_OPENCODE_CONTACT_EMAIL, else config.json contactEmail; validated; empty -> the backend refuses with ErrNoContactEmail naming the setting. Not settable from .gorilla-opencode.json (allow-list). |
| `internal/llm/tools/biodata.go, websearch_lynx.go, websearch_data_sources.go, utf8cut.go` | modified/added | bio_lookup unfenced; lynx outside the politeness limiter; World Bank in map order ignoring n; year-only dates; byte-index truncation. | Fenced and tainting; limiter applied; API order and n respected; full dates on GDELT/HDX/SEC/World Bank; cutAtRune. |
| `cmd/atlas, internal/llm/agent/source-atlas.{txt,yaml}, docs/source-registry.json` | added/modified | Hand-typed atlas beside the registry. | Rendered; a test fails on drift. bioRxiv/medRxiv A->B, HDX A->B, ReliefWeb address corrected. |
| `internal/config/config.go (MCPServer.Trust), internal/llm/agent/mcp-tools.go` | modified | Every MCP call Egress: true and tainting. | Stdio + trust "local": Egress only when args carry "network": true; taint unless _meta.untrusted == false; _meta.egress true on an undeclared call -> tainted and logged. Unmarked and sse servers unchanged. |
| `cmd/schema, opencode-schema.json` | modified | Model enum in map order; committed schema 77 entries behind. | Sorted; regenerated; a test holds the file to the generator. |
| `internal/completions/bounded_test.go, internal/llm/agent/research_evidence_test.go` | modified | A clock-dependent test failed about one run in two under load; a Unix file-mode assertion failed on Windows. | Budget lifted for the scan test only; mode asserted only where permission bits exist. |

### Subsystem Changes

**TOOLS:** research plan and evidence check; data-source fixes; MCP local trust.

**NETWORK:** None added. Unpaywall and SEC now send the user's address or are not called.

**STORAGE:** findings-*.jsonl beside findings-*.md.

**OTHER:** Generated source atlas and schema, each held by a test.

### Test Coverage

- **Added:** research_evidence_test.go, research_recover_test.go, biodata_test.go, utf8cut_test.go, websearch_data_sources_test.go, websearch_lynx_test.go, mcp_local_trust_test.go, mcp_fieldkit_live_test.go (skipped unless FIELDKIT_MCP_COMMAND), cmd/atlas and cmd/schema tests; researchroute_test requires the plan in both prompts.
- **Removed:** Not available in the source material.
- **Notes:** Complete suite passes on Windows. Windows binary against nvidia/nemotron-3-super-120b-a12b: 7 of 7 scenarios valid. /osint on Gemini 3.6 Flash: plan of five sub-questions passed and opening the answer; findings-*.jsonl with 28 claims against 33 fetched pages; 264.5 s. The first 0.1.146 build sent /osint without a plan; fixed, rebuilt, all of the above rerun on the rebuilt file. NOT RUN: MCP local trust against a live Fieldkit server; Unpaywall/SEC with and without the address outside unit tests.

### Security Posture

Removes a compiled-in personal identifier sent to two third-party services from every build. Research answers now carry a code-level provenance check. MCP local trust narrows the egress prompt for marked stdio servers only and relies on the server's labels; config.json only.

### Deployment

**Prerequisites:**
- Windows 10/11 x64, or linux/amd64.
- For Unpaywall/SEC: contactEmail in config.json or GORILLA_OPENCODE_CONTACT_EMAIL.

```bash
# fetch
gh release download v0.1.146 -R gorillanobakaa-dot/Gorilla.Opencode
# Expected: exe, linux-amd64 binary, .deb, .rpm, SHA256SUMS-v0.1.146.txt.
# verify
sha256sum -c SHA256SUMS-v0.1.146.txt --ignore-missing
# Expected: Each present file followed by OK. Windows exe: 6eb3debd2e402a3d777223706bb0928d079dc2eb43a28088e854081f32548307.
# install
.\gorilla-opencode.exe install
# Expected: Install path and shortcuts.
# verify_active
gorilla-opencode --version
# Expected: v0.1.146
```

**Rollback:**
  1. Remove this version
     `gorilla-opencode uninstall   # or apt remove / dnf remove gorilla-opencode`
  2. Install v0.1.145 (reintroduces the compiled-in address)
     `gh release download v0.1.145 -R gorillanobakaa-dot/Gorilla.Opencode`

### Known Issues

**[low]** 'this source requires a contact email'
- Cause: Unpaywall or SEC EDGAR reached with no address configured.
- Remedy: Set contactEmail in config.json or GORILLA_OPENCODE_CONTACT_EMAIL.

**[low]** Claims listed as UNSUPPORTED
- Cause: cited-not-opened or quote-not-found for that lane.
- Remedy: Treat as unproven; rerun or check the source by hand.

**[medium]** A shell command asks for permission again after 'allow for session'
- Cause: Session grants are keyed on the exact command text.
- Remedy: Known; not changed in this version.

### Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| Evidence check rules | 📄 stated in input | a web address listed as evidence that the helper never opened is marked UNSUPPORTED |
| Compiled-in maintainer address removed | 📄 stated in input | That address is gone |
| osint plan fixed during release testing | 📄 stated in input | /osint did not ask the model for a plan |
| 7 of 7 scenarios | 📄 stated in input | All seven gave valid results |
| Binary hash and size | 📄 stated in input | sha256 6eb3debd2e402a3d777223706bb0928d079dc2eb43a28088e854081f32548307, 55,220,736 bytes |
| Scope | 📄 stated in input | 50 files changed, 2648 insertions, 300 deletions |
| Session grants keyed on exact command text | 📄 stated in input | a session-wide "allow" covers one exact command |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Auto-generated DITA-structured technical release notes.*
