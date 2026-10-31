# Fieldkit: local tools, labelled per call

[Fieldkit](https://github.com/gorillanobakaa-dot/Gorilla.Fieldkit) is a set of tested local tools served over
MCP. Examples: checking an Office file before you send it, a privacy scan, verified builds, and academic
checks (references, word count, language, plagiarism pre-check). Each tool answers in a few short lines and
ends with a `NEXT:` step, so a small model can follow it. The model reaches every tool through the same
calls: `discover`, `describe`, `run` and `undo`. Fieldkit's tools are loaded only when needed, like any
other MCP server here, so they cost almost nothing per turn.

## Connect it

Install Fieldkit (Python 3.10 or newer), then check it starts:

```
pipx install git+https://github.com/gorillanobakaa-dot/Gorilla.Fieldkit
fieldkit doctor
```

Add it to **your own** settings file, `~/.config/gorilla-opencode/config.json`. On Windows that is
`%USERPROFILE%\.config\gorilla-opencode\config.json`. A project folder's `.gorilla-opencode.json` cannot
add MCP servers.

```json
"mcpServers": {
  "fieldkit": { "type": "stdio", "command": "fieldkit", "args": ["mcp"], "trust": "local" }
}
```

Restart. In `/context` a `fieldkit` row shows what its tools cost.

## What `"trust": "local"` changes, and what it does not

Without it, every MCP server is treated as a stranger:

- each call asks as "this sends a request off the machine", even in auto-approve;
- each answer marks the conversation as having read untrusted content, so every later action asks too.

That is right for a server you did not write. For a harness of read-only checks, it means a click on almost
every step.

With `"trust": "local"`, a server you installed labels each answer, and the program uses those labels
instead:

| | Without `trust` | `"trust": "local"` |
|---|---|---|
| Asked as egress | every call | only a call whose arguments say `"network": true` |
| Conversation tainted | every answer | only an answer whose `_meta.untrusted` is not exactly `false` |
| Answer fenced as untrusted | always | when tainted |
| Server says it went online for a call that did not declare it | n/a | tainted, and logged |
| An answer with no labels | n/a | tainted (treated as today) |

What does **not** change:

- You are still asked before each call, unless you approve it for the session.
- An irreversible Fieldkit change still needs you at a terminal. The MCP door has no approval argument.

`trust` works only on `stdio` servers, and only with the exact value `"local"`.

Fieldkit's labels:
- **Untrusted:** answers built from text someone else wrote, such as `office-read`, the study-material
  search, the plagiarism pre-check, and build jobs that quote Firefox source.
- **Own:** checks of your own work and Fieldkit's own registry.
- **Going online:** a tool that goes online (for example the release check) refuses unless the call says
  `"network": true`, so you are asked first.

**Only mark a server `local` if you trust its labels.** A server can lie about `untrusted`. It cannot
hide going online for a call that did not declare it, because that taints the session anyway.

## Check it works

Ask: *use fieldkit to describe academic-refs*. The answer starts with `academic-refs:`, and later actions
in the turn do not ask "this turn has already read untrusted content".

The Go test `TestTheRealFieldkitServerAsALocalServer` covers the same ground automatically:

```
FIELDKIT_MCP_COMMAND="fieldkit mcp" go test ./internal/llm/agent/ -run TheRealFieldkit -count=1
```

## Building this program with Fieldkit

Fieldkit also builds Gorilla OpenCode itself, the same way every time:

1. It checks the Go toolchain against `go.mod`.
2. It verifies every dependency against `go.sum`.
3. It runs `go vet` and the whole test suite, and reports only the failures.
4. It builds Linux and Windows binaries, stamped with the version, with cgo off and `-trimpath`.
5. It writes `SHA256SUMS`.

```
fieldkit pipeline run gorilla-opencode --var src=PATH/TO/Gorilla.Opencode --var version=v0.1.146
```

The version flag is optional. A second run redoes only the stages whose inputs changed.

## web_fetch renders script-only pages in a local browser

When a page's HTTP body holds no readable text (its content is built by JavaScript), `web_fetch` runs `fieldkit web fetch URL --json` once, if `fieldkit` is on PATH. It drives a browser already installed on this machine (throwaway profile, no downloads). Only the URL goes to that process: no cookies, headers or session data. The text it returns is fenced as untrusted page text, like any fetched page, and the result says "rendered by a local browser". If it fails, the usual "no readable text" message says why.

- **Setting:** `"webFetchBrowserOff": true` in config.json turns it off (default: on when fieldkit is found; not read from a project folder's `.gorilla-opencode.json`)
- **Environment:** `GORILLA_OPENCODE_WEB_FETCH_BROWSER=0` does the same
- **Lookup:** `fieldkit` is looked up on PATH once per process
