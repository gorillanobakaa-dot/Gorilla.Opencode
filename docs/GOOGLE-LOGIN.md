<p align="center"><img src="../internal/assets/icons/gorilla-opencode-256.png" width="96" alt="Gorilla OpenCode"></p>

<h1 align="center">Sign in with Google — free Gemini, no API key</h1>

<p align="center"><em>The same "Login with Google" that Gemini CLI and Antigravity use, so anyone with a Gmail account can code with Gemini on the free tier — and stay in control of their quota.</em></p>

---

## For anyone (why this exists)

Google gives Gemini away two ways. One is an **API key**, and on many free
keys the coding models now answer with `HTTP 429 "limit: 0"` — i.e. *no free
quota at all* unless you attach billing. The other is **"Login with
Google"**, which reaches Google's *Code Assist* free tier through your normal
Gmail account. That second path is what the Gemini CLI used to give you
before it got heavy — hours of coding help a day, for free.

This fork is deliberately lean (it makes few, small calls instead of
swarming dozens of sub-agents to read one folder), so paired with the free
login your daily allocation lasts the way it used to. If you *do* pay Google,
the same leanness makes your money go much further. Either way, **you** decide
which model to spend your quota on — drop from Pro to Flash to Flash-Lite and
keep working instead of being stuck.

That is the whole point: putting quota control back in the hands of people who
can't spend $20/month, so they can learn.

### How to use it

```sh
gorilla-opencode login
```

1. Pick **1. Google OAuth** (personal Gmail, free tier).
2. Your browser opens Google's normal consent screen — approve it.
3. The terminal prints `Signed in as you@gmail.com` and a project id.

Then start the app, open the model picker (`/models`), press **→** until you
reach **"Gemini (Google login)"**, and choose a model. Recommended:

- **Gemini 2.5 Flash / Flash-Lite** — generous free quota, fast. Use these for
  most work.
- **Gemini 2.5 Pro / 3.x** — smarter but a *small* free allocation; you will
  hit `capacity exhausted on this model` quickly. Save it for hard problems.

If a model says it's out of capacity, that's Google's per-model free limit,
not a bug — switch to a lighter model or come back later. Your logged-in cost
shows as **$0.00** because the free tier isn't billed.

Sign out by deleting `~/.config/gorilla-opencode/gemini-oauth.json`.

---

## For developers (how it works)

Two pieces: an OAuth package and a provider.

**Auth — `internal/auth/gemini_oauth.go`**
- OAuth 2.0 **loopback** flow: starts a localhost server on a random port
  (`/oauth2callback`, overridable with `OAUTH_CALLBACK_PORT`), opens the
  browser to `accounts.google.com/o/oauth2/v2/auth` with `access_type=offline`
  + `prompt=consent` (to always get a refresh token), validates `state`, and
  exchanges the code at `oauth2.googleapis.com/token`.
- Credentials (access + refresh token, expiry, email, project, tier) are
  stored `0600` at `~/.config/gorilla-opencode/gemini-oauth.json` and the
  access token is refreshed automatically ~60s before expiry.
- **Onboarding** (`SetupCodeAssist`): POSTs `:loadCodeAssist` to discover the
  tier + any project, then polls `:onboardUser` (a long-running op) until it
  returns the auto-provisioned free-tier `cloudaicompanionProject` id.
- Client id/secret and scopes are copied verbatim from the open-source Gemini
  CLI (see the transparency note below).

**Provider — `internal/llm/provider/code_assist.go`**
- Not OpenAI-compatible and not the genai SDK. It hand-builds the Code Assist
  envelope: `POST {CODE_ASSIST_ENDPOINT}/v1internal:streamGenerateContent?alt=sse`
  with body `{ "model", "project", "request": { contents, systemInstruction,
  tools, generationConfig } }` and `Authorization: Bearer <token>`.
- Streaming replies arrive as SSE `data:` lines; the real Gemini payload is
  nested under a **`response`** field per chunk (`chunk.response.candidates[…]`).
- Message/tool conversion mirrors the SDK-based `gemini.go`, including echoing
  the **Gemini-3 thought signature** back on replayed `functionCall` parts
  (Google 400s without it).
- Registered as provider **`gemini-oauth`**; the live Gemini model list is
  cloned onto it (`internal/llm/models/gemini_codeassist.go`), and the
  background agents (title/summarizer/task) default to it when logged in
  (`internal/config/config.go`) so the whole app runs on the free tier.

Reproduce the endpoints yourself: they are the public constants in
`internal/auth/gemini_oauth.go`.

---

## Transparency & Google ToS (stated, not hidden)

This reuses the Gemini CLI's **public** OAuth client id/secret (an
installed-app "secret" is not truly secret — it ships in an Apache-2.0
open-source binary) and Google's **private** Code Assist backend from a
third-party client. It is how every "free Gemini login" tool works, and it is
a Google **Terms-of-Service gray area**. We state that plainly here rather
than bury it, per this project's [philosophy](../PHILOSOPHY.md). Use your own
judgement; if you need a contractually clean path, use an API key or a paid
Google Cloud project (`gorilla-opencode login --project YOUR_PROJECT_ID`).

No telemetry is added by this feature. The only hosts it contacts are
Google's own OAuth and Code Assist endpoints — verify with the audit in
[SECURITY.md](../SECURITY.md).
