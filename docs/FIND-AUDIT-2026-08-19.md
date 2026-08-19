<!-- Version: 1.0.0 · updated 26-08-19-12-48 -->
# Search-strategy audit — the find tool, 19 August 2026

*Prompted by a real failure and a good question: `find` replaced grep, glob and
ls, so it now runs on nearly every turn. Its parameter surface is large. What
else can go wrong the way today's bug went wrong?*

---

## The one shape worth hunting

Today's failure was not "the search was slow". It was that **a filter which
misfired looked exactly like an empty result** — and an empty result is a claim
about your code.

> "No matches found" is a statement about the WORLD.
> "The filter did not work" is a statement about the CALL.
>
> A tool that returns the first when it means the second makes the model report
> a false fact, confidently, and never retry.

So the audit did not look for crashes. It looked for **every way this tool can
say "there is none of that here" when it means something else.**

## Method

The parameters were exercised directly against the real engine — twelve
combinations covering hidden paths, ignored paths, invalid filters, conflicting
flags, missing paths, relative paths, path-is-a-file, and each `view`. What
follows is what came back, not what was expected.

## Findings

### 1. Hidden directories were invisible — including `.github` ✅ FIXED

`glob=".github/**"` returned **"No matches found"** on a repository whose
`.github/workflows/` holds `build.yml`, `ci.yml` and `release.yml`.

A model asked *"does this project have CI?"* is told no. It answers no.

Skipping hidden files is correct as a **default** — nobody wants `.git` objects
and caches in every result. It is indefensible when **the request names the
hidden thing**. Typing `.github/**` is not ambiguous.

Fixed: a path or glob that explicitly names a dot-path now passes `--hidden
--no-ignore-vcs`. Intent-driven, not a blanket flag — an ordinary search still
skips hidden files, and there is a test asserting it still does.

### 2. An unknown `type` silently returned nothing ✅ FIXED

`pfind -t notalanguage` exits **0 with no output**. So `type="pyton"` produced
"No matches found" — which reads as *"this project has no Python in it"*.

A typo in a filter became a factual claim about the codebase.

Fixed: unknown types are refused before the search runs, with the nearest
suggestions and the full valid list. The message says explicitly that the
search **did not happen**.

### 3. The type list I wrote was itself wrong ✅ FIXED

The guard test for finding #2 immediately failed — against **my own list**. I
had hand-written it from memory and invented six languages the engine has never
heard of: `protobuf`, `r`, `rb`, `rs`, `svelte`, `vue`.

That is finding #2 committed while fixing finding #2. A type we accept but the
engine does not know returns nothing — *"there is none of that language here"*.

Fixed properly: the list is now **read from the engine** at first use, not
declared. A hand-maintained mirror of somebody else's list rots, silently, in
the direction of a false answer.

*(And the first parser for that leaked the header row in as a language called
"language". The drift test caught that too.)*

### 4. `modified_only` outside a git repository ✅ FIXED

Returned **"No matches found"** — identical to the answer inside a repository
with a clean tree.

Different facts. One says "nothing has been edited"; the other says "this
question cannot be asked here". Fixed: it now says which.

### 5. A content search over a whole home directory ✅ FIXED EARLIER TODAY

`query` reads **inside** every file. Rooted at `$HOME` that is every project,
cache and archive on the machine — it timed out at 30s and the model fell back
to guessing paths through bash.

Refused instantly now, naming `glob` as the fix for a name search.

## Checked and found sound

| Case | Behaviour |
|---|---|
| Exact dotfile glob (`.gitignore`) | found ✅ |
| `path` is a file, not a directory | searched it ✅ |
| `path` does not exist | clear error naming the path ✅ |
| Relative path | resolved against the working directory, error names the resolved path ✅ |
| Exclude-only glob (`!*.go`) | listed everything else ✅ |
| `regex` + `fuzzy` together | no error, regex wins — harmless |
| `files_only` + `view=tree` | tree wins — harmless |
| Truncation | already reported, already documented |

## Still open

- **`view=code` on a large tree times out at 30 seconds.** It aggregates
  line counts across everything, so it is genuinely expensive on a repository
  this size. Not yet fixed; it fails with the improved message rather than
  silently.
- **Nothing warns when results were cut by `.gitignore`.** If a search matches
  files inside `node_modules` or `build/`, the answer is silently smaller. The
  fix is a count of skipped files, which the engine does not currently report.

## The rule this audit produced

**Any filter that can fail must fail LOUDLY, because the alternative is
indistinguishable from a fact about the user's code.**

That covers all five findings and, on today's evidence, most of what is left to
find. It generalises past this tool: an agent's tools are believed, so a tool
that hedges its failures teaches the agent falsehoods.
