# Signing in with your Claude account

> Dual track. The first half is the whole explanation in plain language. The
> second half is the detail needed to rebuild it. Neither is a summary of the
> other.

---

## Plain language

### What this is

Gorilla OpenCode can now use Anthropic's Claude models through the Claude
subscription you already pay for, instead of asking you for an API key.

It does this by starting the `claude` program, Anthropic's own Claude Code, which
is already installed and signed in on your computer, and talking to it. Your
answers come back from Claude. Nothing is billed per message: the work counts
against your Claude plan, exactly as it would if you had typed into Claude Code
yourself.

### What you have to do once

Install Claude Code if you do not have it, then in a terminal:

```
claude login
```

Choose your Claude account. That is the whole setup. After that the row
**"Claude sign-in"** appears in Gorilla OpenCode's connect screen and you pick it
like any other.

### What Gorilla OpenCode knows about your account

Nothing. That is not a figure of speech.

This program never asks for your password, never receives a token, never stores
one, and has nothing to refresh. Your sign-in lives inside Anthropic's own
program, which you signed in to yourself. Gorilla OpenCode starts that program
and reads what it prints.

If someone sent you this program's settings file, there would be no Claude
credential in it, because there never was one.

### Why not just log in directly, like the ChatGPT row does?

This is the question worth answering honestly, because the two look like they
should work the same way and they do not.

A Claude Pro or Max plan is not an API key. Anthropic says, in writing, that
signing in with a subscription is for their own programs, and that other people's
software should use a paid API key instead. Programs that reached a subscription
anyway did it by pretending, on the network, to be Claude Code. Anthropic started
blocking that on 9 January 2026, and OpenCode, the project Gorilla OpenCode is
built from, was one of the programs that broke that day.

So this route does not pretend to be anything. It runs the real Claude Code, the
one you installed, unmodified. That is the route Anthropic's own terms describe,
and it is the reason this will still work next month.

### What it costs

No money per message. Your Claude plan is the cost, and you are already paying it.

What you run out of instead is your plan's **usage window**. Claude plans refill
on a five-hour cycle and a seven-day cycle. When you use up the window, Claude
stops until it resets. That is not a bill arriving and there is nothing to pay to
make it go away: you wait, or you switch to another provider for a while.

This is worth understanding before you start, because it is a completely
different shape of limit from the pay-per-use providers. With those you run out
of money. With this you run out of time.

### Which tools Claude gets

Gorilla OpenCode's own, and only those.

The `claude` program arrives with its own file reader, editor and command runner.
Those are all switched off. What Claude can reach instead is exactly the tool set
Gorilla OpenCode gives every other model: the same tools, running inside Gorilla
OpenCode, asking you for permission in the same way, and appearing in the same
receipt.

This matters more than it sounds. If Claude used its own tools, you would be
testing Claude Code with a Gorilla OpenCode window around it. The whole point of
this route is to run **your** agent with a better model, not to run somebody
else's agent.

You can see it working. Ask it to read a file, and the receipt underneath the
answer names the tools that really ran:

```
--- what actually ran (recorded by Gorilla OpenCode, not written by the AI) ---
2 tool calls:
  find   note.txt              -> ok
  view   .../work/note.txt...  -> ok
```

`find` and `view` are Gorilla OpenCode's tools. Claude Code has no tools by those
names.

### When something goes wrong

| What you see | What it means |
|---|---|
| "not signed in" | Run `claude login` in a terminal and pick your Claude account. |
| "could not find the claude program" | Claude Code is not installed, or not on your PATH. |
| "your Claude plan's usage window is full" | Not a bill. Wait for the reset, or switch provider meanwhile. |
| "does not recognise this model" | Your installed Claude Code is older than the model name. Update it. |

---

## Developer track

### Shape

`internal/llm/provider/claudecli.go` implements `ProviderClient` (`send` and
`stream`) by spawning a child process. It is the only provider in the package
that is not an HTTP client.

```
claude -p <prompt>
  --output-format stream-json      newline-delimited JSON on stdout
  --include-partial-messages       token-level deltas
  --verbose                        REQUIRED, see below
  --tools ""                       strip every built-in Claude Code tool
  --strict-mcp-config              ignore the user's own MCP servers
  --permission-prompts none        a child process has no terminal to ask at
  --model <alias>                  opus | fable | sonnet | haiku
  --system-prompt <ours>           replaces Claude Code's own prompt
  --session-id <uuid>   (first turn)
  --resume <uuid>       (every turn after)
```

### Load-bearing flags, and what breaks without them

- **`--verbose`.** Without it no `stream_event` lines are emitted at all. The
  answer still arrives, on the `result` line, so the finished transcript looks
  perfect and nothing streams on screen. `TestClaudeCLIArgsCarryTheLoadBearingFlags`
  pins it; breaking it was verified to fail the test, not just to look wrong.
- **`--tools ""`.** Otherwise the child silently uses Claude Code's own Read,
  Edit, Bash and WebFetch. Everything appears to work and the user is testing the
  wrong program.
- **`--strict-mcp-config`.** Without it the user's own configured MCP servers
  join the session uninvited.

### Wire format

Measured against claude 2.1.296 on 2026-10-12. One JSON object per line:

```
system/hook_started, system/hook_response   housekeeping, ignored
system/init                                 session id, resolved model, apiKeySource
stream_event/message_start
stream_event/content_block_start            content_block.type: text | thinking | tool_use
stream_event/content_block_delta            delta.type: text_delta | thinking_delta | input_json_delta
stream_event/content_block_stop
stream_event/message_delta                  delta.stop_reason, usage
stream_event/message_stop
assistant                                   the assembled message, after the deltas
rate_limit_event                            subscription window utilisation
system/post_turn_summary
result/success                              is_error, result, usage, total_cost_usd
```

The objects under `stream_event` are **verbatim Anthropic Messages API SSE
events**, which is why the parser looks like an SSE reader with no SSE present.

A captured run is committed at
`internal/llm/provider/testdata/claudecli_stream.jsonl`, scrubbed of machine
paths. The tests replay it rather than hand-written JSON, because a fixture
someone invented only proves the parser agrees with its author.

### History, and why it is carried by session resumption

`--input-format stream-json` cannot be used to seed a conversation. **Measured
2026-10-12:** three input messages (user, assistant, user) produced **two
separate `result` events**, the first of which actually answered a question from
the middle of the transcript. It replays each user message as a real turn.

So history is carried the way the CLI carries it. `cliSession` records how much
of the gorilla-opencode transcript has been handed over; the first turn opens a
named session with `--session-id`, every later turn resumes it and sends only the
new messages.

The rejected alternative, re-flattening the whole transcript into every prompt,
is recorded here because the reason matters: on this route the scarce resource is
the plan's five-hour window, not money, and re-sending the conversation every turn
spends that window on text the CLI already holds.

`sess.sent` is advanced only after a turn completes. Advancing it earlier would
make a failed turn look delivered, and the next turn would resume a session that
never received the message.

### Usage accounting

`applyCLIUsage` **replaces, never adds**. The CLI reports cumulative turn totals
on both `message_delta` and `result`; summing them double-counts.
`TestClaudeCLIUsageIsNotSummed` pins this, and the same trap is already recorded
in this package as `usage_not_summed_test.go`.

`total_cost_usd` from the `result` line is deliberately **not** written into the
model's cost fields. It is what the work would have cost on the API. On a
subscription it is not charged, and showing it would tell the user they are
paying for something their plan covers.

### Models

Four, in `internal/llm/models/claudecli.go`. `APIModel` is the **alias**, not the
resolved id. Probed 2026-10-12:

| alias | resolved to |
|---|---|
| `fable` | `claude-fable-5-1` |
| `opus` | `claude-opus-5-5` |
| `sonnet` | `claude-sonnet-5-5` |
| `haiku` | `claude-haiku-5-5` |

Sending the alias means the list cannot rot: when Anthropic ships the next
Sonnet, `sonnet` follows it with no change here and no release. This is why the
provider is in the fixed-list category in `catalogue_fetch.go` rather than in
`LiveCatalogues`. There is no list to fetch, because there are only four names
and they are permanent.

The resolved ids above are shown to the user and never sent.

**`ClaudeCLIContextWindow` is 200,000 and is a FLOOR, not a measurement.**
Claim sourcing: nothing measured the real window of the 5.5 generation. 200K is
the smallest any current Claude model has, so sizing against it can only compact
earlier than necessary, never send a request refused for length. The CLI also
auto-compacts. Raise it against a measurement, not a documentation page.

### Credential handling

The provider holds none. `CheckClaudeCLI` shells out to `claude auth status`
(237ms measured, zero tokens) and decodes **only** `loggedIn`, `authMethod` and
`subscriptionType`.

The real output also carries the account's email address, organisation name and
organisation id. None of it is needed, so none of it is decoded, so none of it can
reach a log, a crash report or a screenshot by accident. Directive §7: the safest
handling of a value is not to hold it.

`authMethod == "claude.ai"` distinguishes a subscription sign-in from a
configured API key. The portal refuses the row for a key and points at the
Anthropic row instead.

### Portal placement

The row sits at the **head of the paid block**, not with the free sign-ins. It
never asks for a card, a key or anything else, but a Claude plan is bought with
one, and grouping it with the free routes would put a paid option above the
no-card ones. That is the exact thing `TestPortalRowOrder` exists to prevent
(directive §8). It is the best paid route, because no credential is typed into
this program at all.

Background agents run `haiku`: a Claude plan is one pool, so this is the cheap
model against the same five-hour window rather than a separate quota.

### What is verified, and how

Replayed against the committed fixture, no network, no account:

- content deltas arrive, and the final content equals the concatenated deltas
- usage is reported and is not double-counted
- the load-bearing flags are present
- turn 1 opens a session, turn 2 resumes it
- only unseen messages are forwarded
- a signed-out CLI produces an error naming `claude login`
- a full window is not described as a billing problem
- no model carries a per-token price

Live, against the real subscription (`CLAUDECLI_LIVE=1`):

```
TestClaudeCLILiveText                       content="PONG" deltas=1
                                            usage={InputTokens:2 OutputTokens:5
                                            CacheCreationTokens:23085} finish=end_turn
TestClaudeCLILiveRemembersTheConversation   turn 1 "OK", turn 2 "8713"
TestClaudeCLILiveCancellationStopsTheChild  child killed, stream closed
```

End to end through the built binary, its own agent loop and an isolated config:

```
$ gorilla-test -c work -p "Reply with exactly: GORILLA-OK"
GORILLA-OK

--- what actually ran (recorded by Gorilla OpenCode, not written by the AI) ---
No tool was called. The answer above was written without running, reading or checking anything.
```

**Non-vacuity.** Four behaviours were broken on purpose and each produced a test
failure rather than a compile error: dropping `--verbose`, summing usage instead
of replacing it, suppressing content deltas, and deleting the portal row. The row
deletion kept `ccReady` alive with `_ = ccReady` specifically so the build still
succeeded, because a compile error proves nothing.

Note that suppressing the deltas still produced the correct final answer, from the
`result` line. That is the failure the test exists to catch: right in the
transcript, broken on screen.

### The tool bridge

`--tools ""` strips every built-in Claude Code tool, and this program's own are
served back over a loopback MCP server on `127.0.0.1`, with a bearer token
generated per turn and never written to disk.

SSE rather than stdio, deliberately: an MCP stdio server is a command the CHILD
spawns, so the tools would run in a third process with no route back to the
terminal, losing the permission prompt the user answers and the receipt. Over
SSE this program is the server, so `Run()` executes here, on the same permission
system every other provider goes through.

Without `--allowedTools` the child DENIES every bridged tool. Measured against
claude 2.1.296: the call comes back "the session has no approval surface, so the
permission prompt was denied automatically" and lands in the result's
`permission_denials`, and the model then explains the failure instead of doing
the work. The allowlist is built from our own tool names, so a tool this program
did not supply cannot be on it.

Calls the child already ran come back as `ExecutedToolCalls`, not `ToolCalls`,
and are marked `message.ToolCall.Executed` by the agent. They are RECORDED, so
the transcript and the receipt show them, and never DISPATCHED, because the
child has already run them.

### Not done yet

**Attachments.** The models declare `SupportsAttachments: true` on the strength of
the underlying models supporting images. The `-p` path has a `--file` flag that
has not been wired up or tested here, so this is **unverified** and should be
treated as not working until someone proves otherwise.

**The rate-limit meter.** `rate_limit_event` carries exactly what the footer
wants: `five_hour` and `seven_day` utilisation with reset timestamps, plus
`overageStatus`. It is currently logged at debug level and nothing displays it.
`internal/auth/meters.go` already does this job for the other sign-in providers.

