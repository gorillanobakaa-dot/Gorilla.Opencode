## A reply with nothing in it should cost a turn, not the session

One crash, fixed. That is the whole release.

### What was wrong

Sometimes an AI service answers your question with nothing at all. Not an
error, not a refusal, just a technically valid reply that contains no words. A
safety filter removed the answer, a quota ran out mid-sentence, or a proxy in
the middle timed out and handed back an empty envelope.

Gorilla OpenCode did not know that was allowed. It reached for the first
sentence of the reply, found there wasn't one, and crashed: not the answer, not
the question, the whole program, closing the window and taking the conversation
with it.

```
panic: runtime error: index out of range [0] with length 0
  provider.(*openaiClient).stream.func1() internal/llm/provider/openai.go:478
```

If a run has ever died on you for no visible reason just as an answer was about
to appear, this may well have been why. It looked like a network fault. It was
not one. The reply arrived perfectly. The program could not cope with it being
empty.

### What changes for you

That turn now fails with a message and your session survives.

If part of the answer had already reached your screen, it is kept. It arrived,
and you paid for it, so discarding it to be tidy would be its own kind of loss.

### How it was found, which is the interesting part

We spent the morning checking whether Gorilla OpenCode could be built for
Windows. It can: there is no cgo anywhere in the tree and the SQLite driver is
pure Go, so `GOOS=windows go build` produces a working executable with no
source change. Under Wine on a Debian laptop, that build printed its version,
parsed its CLI, refused correctly with no provider configured, created its
database, ran its migrations and wrote a real session row.

To drive it without spending money on a real AI service, we stood up a fake one
on localhost. The fake was slightly wrong: it answered a streaming request with
an ordinary JSON body.

That mistake is what triggered the crash, and the crash turned out to have been
in the normal Linux build the whole time, reachable from every
OpenAI-compatible service this program supports.

An experiment on a platform we do not ship to paid for itself before producing
anything shippable. Unfamiliar ground hands a program inputs its usual
surroundings never do.

### Under the hood

`internal/llm/provider/openai.go` read `Choices[0]` without checking the list
was non-empty, on **both** the streaming and the non-streaming paths. An empty
list arrives three ways: an explicit `"choices": []`, a stream that ends before
the first chunk, and an HTTP 200 whose body is actually an error page. An
unrecovered panic in a goroutine terminates the process, so there was no
degraded mode.

`ErrEmptyCompletion` now names the condition once in `provider.go`. The
streaming repair splits the two situations that the empty list was conflating:

- Nothing streamed: the turn produced nothing, so report the error and let the
  caller decide.
- Tokens streamed and only the closing bookkeeping chunk is missing: the answer
  is on screen and billed, so keep it and report a normal end of turn.

The default-then-override shape is copied from `gemini.go:262`, which already
handled the equivalent case. Every `[0]` index on a wire-derived list in the
provider package was enumerated before writing the fix: `gemini.go`,
`antigravity.go` and `code_assist.go` all guard every `Candidates[0]` read.
Only the OpenAI client was exposed.

Cover is three end-to-end `httptest` cases in `empty_completion_test.go`, not
unit tests on the guard, because the bug was never in the arithmetic. It was in
what the wire is allowed to contain, so the test has to put those things on a
wire. Verified non-vacuous by reinstating both reads and confirming the
original panic returns. Full suite exit 0.

### Evidence

[![A terminal showing the Windows build of gorilla-opencode v0.1.119 running on Debian under Wine. file reports a PE32+ x86-64 Windows console executable, the exe prints v0.1.119, and a run against a local stub server ends with the line Error: agent processing failed: the provider returned a response with no content in it, which is the fix reporting the exact input that used to panic. Below it sqlite3 lists the four tables the Windows binary created inside the Wine C drive, files, goose_db_version, messages and sessions, and selects a real session row titled Non-interactive: say hello](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.119/docs/screenshots/gallery/v0119-windows-build-under-wine-sqlite-works.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.119/docs/screenshots/gallery/v0119-windows-build-under-wine-sqlite-works.png)

The Windows build of this release, running here under Wine against a local stub.
The line `Error: agent processing failed: the provider returned a response with
no content in it` is the fix doing its job: that is the precise input that used
to end the process. Below it, the SQLite database the Windows binary created,
with its migrations run and a real session row in it.

[![A terminal proving the three new tests are not vacuous. The tests pass as shipped, then a script removes the two guards from openai.go, and the same test command reproduces the original panic, runtime error index out of range 0 with length 0, at openai.go line 492, followed by FAIL. The fix is restored with git checkout and the tests pass again](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.119/docs/screenshots/gallery/v0119-tests-not-vacuous-panic-reproduced.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.119/docs/screenshots/gallery/v0119-tests-not-vacuous-panic-reproduced.png)

A test that cannot fail proves nothing, so here it is failing. The guards are
removed from `openai.go` in a throwaway worktree, the same test command
reproduces the original panic on demand, and the fix is restored.

### Verification

All four artifacts were extracted and their inner binary hashed against the
binary that passed the test suite:

```
repo build      815a54e7204944ad
deb inner       815a54e7204944ad
arch pkg inner  815a54e7204944ad
raw binary      815a54e7204944ad
```

Identical, and all four report `v0.1.119`.

### Install

Debian, Ubuntu and derivatives:

```
sudo apt install ./gorilla-opencode_0.1.119_amd64.deb
```

Arch, CachyOS and derivatives:

```
sudo pacman -U gorilla-opencode-0.1.119-1-x86_64.pkg.tar.zst
```

Anything else: take the raw binary, `chmod +x`, put it on your PATH.

Checksums for all artifacts are in `SHA256SUMS-v0.1.119.txt`.
